// Package tunnel is the orchestrator-side client: what ssh runs as its
// ProxyCommand to reach a machine that cannot be dialled.
//
// The contract is narrow. ssh hands the command stdin and stdout and treats
// whatever appears there as the network, so one stray byte on stdout — a
// warning, a progress line, a debug print — corrupts the ssh protocol and
// produces a failure that looks like anything but its cause.
//
// That narrowness is also why the project works. Because the interface is two
// file descriptors, nix-copy-closure, switch-to-configuration, deploy-rs and
// plain interactive ssh all run over this transport with no modification.
package tunnel

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nivis-project/nivis-tunnel/internal/proto"
)

// DefaultTimeout bounds dialling, announcing and handshaking.
//
// ssh hangs for as long as its ProxyCommand does, so an unbounded wait here is
// an ssh that never returns.
const DefaultTimeout = 60 * time.Second

// Failure stages, distinguished because this client is the only place the real
// cause is known. An ssh session that fails with "ProxyCommand failed" and
// nothing else is undiagnosable.
var (
	// ErrRelayUnreachable reports that the relay could not be dialled.
	ErrRelayUnreachable = errors.New("tunnel: relay unreachable")

	// ErrAnnounceFailed reports that the relay refused the rendezvous
	// announcement — most often because the stream id already has an
	// orchestrator.
	ErrAnnounceFailed = errors.New("tunnel: relay refused the announcement")

	// ErrCounterpartAbsent reports that the agent for this stream id never
	// arrived. Usually the machine is still booting, or is not running the
	// agent at all.
	ErrCounterpartAbsent = errors.New("tunnel: the agent for this stream id did not arrive")
)

// Connect dials the relay, announces as the orchestrator for id, and completes
// the handshake. The returned connection is encrypted and authenticated end to
// end; the relay cannot read it.
//
// It also returns the agent's static public key. This build does not verify it
// — the target proves nothing, and is identified by the address the cloud API
// returned — but returning it means pinning later is a caller's decision rather
// than a protocol change.
func Connect(relayAddr string, id proto.StreamID, local proto.Keypair, timeout time.Duration) (net.Conn, []byte, error) {
	// Validate before dialling, and never echo a rejected id: the error will be
	// logged, and echoing it performs the injection validation exists to stop.
	if err := id.Validate(); err != nil {
		return nil, nil, err
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	conn, err := net.DialTimeout("tcp", relayAddr, timeout)
	if err != nil {
		return nil, nil, fmt.Errorf("%w at %s: %w", ErrRelayUnreachable, relayAddr, err)
	}

	frame, err := proto.Frame{
		Version:  proto.Version,
		Role:     proto.RoleOrchestrator,
		StreamID: id,
	}.MarshalBinary()
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}

	if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("tunnel: setting write deadline: %w", err)
	}
	if _, err := conn.Write(frame); err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("%w: %w", ErrAnnounceFailed, err)
	}
	if err := conn.SetWriteDeadline(time.Time{}); err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("tunnel: clearing write deadline: %w", err)
	}

	// From here the relay is parking us until the agent arrives. A handshake
	// that times out means the counterpart never came, which is a different
	// diagnosis from a refused peer and deserves to say so.
	secure, agentKey, err := proto.ConnectAsOrchestrator(conn, local, timeout)
	if err != nil {
		_ = conn.Close()
		if isTimeout(err) {
			return nil, nil, fmt.Errorf("%w (stream id %s, waited %s)", ErrCounterpartAbsent, id, timeout)
		}
		return nil, nil, err
	}

	return secure, agentKey, nil
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// Splice copies between the stream and a local reader/writer pair until either
// direction ends.
//
// It takes an io.Reader and io.Writer rather than reaching for os.Stdin and
// os.Stdout directly, so the copy loop can be tested without a terminal.
func Splice(stream net.Conn, in io.Reader, out io.Writer) error {
	var (
		wg      sync.WaitGroup
		once    sync.Once
		copyErr error
	)

	// Each direction closes the stream when it finishes, which makes the other
	// direction fail with a closed-connection error. That is how the splice
	// ends, not a fault, so those errors are not reported.
	record := func(err error) {
		switch {
		case err == nil,
			errors.Is(err, io.EOF),
			errors.Is(err, io.ErrClosedPipe),
			errors.Is(err, net.ErrClosed),
			errors.Is(err, os.ErrClosed):
			return
		}
		once.Do(func() { copyErr = err })
	}

	wg.Add(2)

	go func() {
		defer wg.Done()
		_, err := io.Copy(out, stream)
		record(err)
		// Unblock the opposite direction: ssh closing its end must end the
		// process, not leave it holding descriptors.
		_ = stream.Close()
	}()

	go func() {
		defer wg.Done()
		_, err := io.Copy(stream, in)
		record(err)
		_ = stream.Close()
	}()

	wg.Wait()
	return copyErr
}

// WritePrivateKey saves a private key so that only its owner can read it.
//
// It refuses to overwrite. A key file that already exists is either in use or
// evidence of a mistake, and silently replacing it would lock out every agent
// configured with the matching public half.
func WritePrivateKey(path string, key proto.Keypair) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("tunnel: creating key directory: %w", err)
		}
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("tunnel: %s already exists; refusing to overwrite it", path)
		}
		return fmt.Errorf("tunnel: creating key file: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintln(f, proto.EncodePublicKey(key.Private)); err != nil {
		return fmt.Errorf("tunnel: writing key file: %w", err)
	}
	return nil
}

// ReadPrivateKey loads a private key written by WritePrivateKey and recovers
// the public half from it.
func ReadPrivateKey(path string) (proto.Keypair, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return proto.Keypair{}, fmt.Errorf("tunnel: reading key file: %w", err)
	}

	priv, err := proto.DecodePrivateKey(trimLine(string(raw)))
	if err != nil {
		return proto.Keypair{}, fmt.Errorf("tunnel: %s: %w", path, err)
	}
	return priv, nil
}

func trimLine(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}
