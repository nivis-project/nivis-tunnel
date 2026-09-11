package proto

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/flynn/noise"
	"golang.org/x/crypto/curve25519"
)

// cipherSuite is Noise_XK_25519_ChaChaPoly_BLAKE2s.
var cipherSuite = noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s)

// maxNoiseMessage is the Noise framework's hard limit on one message.
const maxNoiseMessage = 65535

// maxPlaintextChunk leaves room for the AEAD tag inside one Noise message.
const maxPlaintextChunk = maxNoiseMessage - 16

// ErrPeerRefused reports a handshake that did not complete because the peer was
// not the party expected. For the agent this is the entire access control
// mechanism: it talks to exactly one orchestrator and to no other.
var ErrPeerRefused = errors.New("proto: handshake refused: peer is not the expected party")

// Keypair is a Noise static keypair. Only the public half ever leaves the
// machine that generated it, which is what lets a boot image carry key material
// while carrying no secret.
type Keypair struct {
	Private []byte
	Public  []byte
}

// GenerateKeypair produces a fresh static keypair.
func GenerateKeypair() (Keypair, error) {
	k, err := cipherSuite.GenerateKeypair(rand.Reader)
	if err != nil {
		return Keypair{}, fmt.Errorf("proto: generating keypair: %w", err)
	}
	return Keypair{Private: k.Private, Public: k.Public}, nil
}

// EncodePublicKey renders a public key for configuration files. Standard base64
// so it can sit in a NixOS module option, a flag, or a public repository.
func EncodePublicKey(pub []byte) string {
	return base64.StdEncoding.EncodeToString(pub)
}

// DecodePublicKey parses a public key produced by EncodePublicKey.
func DecodePublicKey(s string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("proto: decoding public key: %w", err)
	}
	if len(b) != cipherSuite.DHLen() {
		return nil, fmt.Errorf("proto: public key is %d bytes, want %d", len(b), cipherSuite.DHLen())
	}
	return b, nil
}

// DecodePrivateKey parses a private key and recovers its public half.
//
// Only the private half is stored. Keeping one value on disk removes the whole
// class of failure where a pair drifts apart and an operator is left comparing
// two files to find out which half is stale.
func DecodePrivateKey(s string) (Keypair, error) {
	priv, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return Keypair{}, fmt.Errorf("proto: decoding private key: %w", err)
	}
	if len(priv) != cipherSuite.DHLen() {
		return Keypair{}, fmt.Errorf("proto: private key is %d bytes, want %d", len(priv), cipherSuite.DHLen())
	}

	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return Keypair{}, fmt.Errorf("proto: deriving public key: %w", err)
	}
	return Keypair{Private: priv, Public: pub}, nil
}

// setHandshakeDeadline applies timeout, treating a non-positive value as no
// deadline at all.
//
// The agent needs that: it parks at the relay until an orchestrator arrives,
// and a machine may legitimately wait days between deploys. The relay's own
// rendezvous timeout is what bounds that wait, not this one.
func setHandshakeDeadline(conn net.Conn, timeout time.Duration) error {
	if timeout <= 0 {
		return nil
	}
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("proto: setting handshake deadline: %w", err)
	}
	return nil
}

// ConnectAsAgent runs the agent's half of the handshake over conn.
//
// The agent is the Noise initiator. That is not an accident of who dialled
// first — both parties dial the relay — but a consequence of which side has to
// be sure of the other. The agent holds only a public key and must refuse
// anyone who is not the orchestrator, so it has to be the party that verifies,
// and in Noise_XK the initiator verifies a responder whose static key it knows
// in advance.
//
// The alternative, Noise_KX with the orchestrator initiating, authenticates
// just as strongly in the cryptographic sense — an impostor learns nothing and
// can forge nothing — but the agent gets no signal at handshake time and only
// discovers the problem when transport messages fail to decrypt. It would sit
// holding a session that can never carry a byte. Refusing at the handshake is
// what the specification asks for and what a small machine can afford.
func ConnectAsAgent(conn net.Conn, local Keypair, expectedOrchestrator []byte, timeout time.Duration) (net.Conn, error) {
	if len(expectedOrchestrator) != cipherSuite.DHLen() {
		return nil, fmt.Errorf("proto: expected orchestrator key is %d bytes, want %d",
			len(expectedOrchestrator), cipherSuite.DHLen())
	}

	hs, err := noise.NewHandshakeState(noise.Config{
		CipherSuite:   cipherSuite,
		Random:        rand.Reader,
		Pattern:       noise.HandshakeXK,
		Initiator:     true,
		StaticKeypair: noise.DHKey{Private: local.Private, Public: local.Public},
		PeerStatic:    expectedOrchestrator,
	})
	if err != nil {
		return nil, fmt.Errorf("proto: agent handshake state: %w", err)
	}

	if err := setHandshakeDeadline(conn, timeout); err != nil {
		return nil, err
	}

	// -> e, es
	msg, _, _, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("proto: writing handshake message 1: %w", err)
	}
	if err := writeNoiseMessage(conn, msg); err != nil {
		return nil, err
	}

	// <- e, ee
	//
	// Only a peer holding the orchestrator's private key can produce a message
	// that decrypts here, so this read is where an impostor is refused.
	in, err := readNoiseMessage(conn)
	if err != nil {
		return nil, err
	}
	if _, _, _, err := hs.ReadMessage(nil, in); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPeerRefused, err)
	}

	// -> s, se
	msg, csSend, csRecv, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("proto: writing handshake message 3: %w", err)
	}
	if csSend == nil || csRecv == nil {
		return nil, errors.New("proto: handshake did not complete after message 3")
	}
	if err := writeNoiseMessage(conn, msg); err != nil {
		return nil, err
	}

	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, fmt.Errorf("proto: clearing handshake deadline: %w", err)
	}

	return &secureConn{Conn: conn, send: csSend, recv: csRecv}, nil
}

// ConnectAsOrchestrator runs the orchestrator's half of the handshake over conn.
//
// It returns a secured connection and the agent's static public key. This build
// does not verify that key — the target proves nothing about itself, and is
// identified by the address the cloud API returned, which is the trust model
// ssh-keyscan already relies on. It is returned so that pinning it later is a
// caller's decision rather than a protocol change.
func ConnectAsOrchestrator(conn net.Conn, local Keypair, timeout time.Duration) (net.Conn, []byte, error) {
	hs, err := noise.NewHandshakeState(noise.Config{
		CipherSuite:   cipherSuite,
		Random:        rand.Reader,
		Pattern:       noise.HandshakeXK,
		Initiator:     false,
		StaticKeypair: noise.DHKey{Private: local.Private, Public: local.Public},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("proto: orchestrator handshake state: %w", err)
	}

	if err := setHandshakeDeadline(conn, timeout); err != nil {
		return nil, nil, err
	}

	// -> e, es
	in, err := readNoiseMessage(conn)
	if err != nil {
		return nil, nil, err
	}
	if _, _, _, err := hs.ReadMessage(nil, in); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrPeerRefused, err)
	}

	// <- e, ee
	msg, _, _, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("proto: writing handshake message 2: %w", err)
	}
	if err := writeNoiseMessage(conn, msg); err != nil {
		return nil, nil, err
	}

	// -> s, se
	in, err = readNoiseMessage(conn)
	if err != nil {
		return nil, nil, err
	}
	_, csRecv, csSend, err := hs.ReadMessage(nil, in)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrPeerRefused, err)
	}
	if csSend == nil || csRecv == nil {
		return nil, nil, errors.New("proto: handshake did not complete after message 3")
	}

	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, nil, fmt.Errorf("proto: clearing handshake deadline: %w", err)
	}

	// csRecv carries agent->orchestrator, csSend carries orchestrator->agent.
	return &secureConn{Conn: conn, send: csSend, recv: csRecv}, hs.PeerStatic(), nil
}

// writeNoiseMessage frames one Noise message with a uint16 length prefix.
func writeNoiseMessage(w io.Writer, msg []byte) error {
	if len(msg) > maxNoiseMessage {
		return fmt.Errorf("proto: noise message is %d bytes, limit is %d", len(msg), maxNoiseMessage)
	}
	buf := make([]byte, 2+len(msg))
	binary.BigEndian.PutUint16(buf[:2], uint16(len(msg)))
	copy(buf[2:], msg)
	if _, err := w.Write(buf); err != nil {
		return fmt.Errorf("proto: writing noise message: %w", err)
	}
	return nil
}

// readNoiseMessage reads one length-prefixed Noise message. The prefix is a
// uint16, so a peer cannot induce an allocation larger than the framework's own
// message limit.
func readNoiseMessage(r io.Reader) ([]byte, error) {
	var lenBuf [2]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, fmt.Errorf("proto: reading noise message length: %w", err)
	}
	n := binary.BigEndian.Uint16(lenBuf[:])
	msg := make([]byte, n)
	if _, err := io.ReadFull(r, msg); err != nil {
		return nil, fmt.Errorf("proto: reading noise message body: %w", err)
	}
	return msg, nil
}

// secureConn is the post-handshake stream: a net.Conn whose bytes are encrypted
// and authenticated end to end.
//
// Callers splice it without knowing about Noise, which is the whole point —
// above this line the transport is just a connection, and ssh runs over it
// unchanged.
type secureConn struct {
	net.Conn
	send *noise.CipherState
	recv *noise.CipherState

	// pending holds plaintext decrypted but not yet consumed by a short Read.
	pending []byte
}

func (c *secureConn) Read(p []byte) (int, error) {
	if len(c.pending) == 0 {
		msg, err := readNoiseMessage(c.Conn)
		if err != nil {
			return 0, err
		}
		plain, err := c.recv.Decrypt(nil, nil, msg)
		if err != nil {
			return 0, fmt.Errorf("proto: decrypting stream: %w", err)
		}
		c.pending = plain
	}

	n := copy(p, c.pending)
	c.pending = c.pending[n:]
	return n, nil
}

func (c *secureConn) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		chunk := p
		if len(chunk) > maxPlaintextChunk {
			chunk = chunk[:maxPlaintextChunk]
		}
		sealed, err := c.send.Encrypt(nil, nil, chunk)
		if err != nil {
			return written, fmt.Errorf("proto: encrypting stream: %w", err)
		}
		if err := writeNoiseMessage(c.Conn, sealed); err != nil {
			return written, err
		}
		written += len(chunk)
		p = p[len(chunk):]
	}
	return written, nil
}
