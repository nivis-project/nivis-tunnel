package tunnel

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nivis-project/nivis-tunnel/internal/proto"
	"github.com/nivis-project/nivis-tunnel/internal/relay"
)

// StartRelay brings up a real relay on a real listener for a test and returns
// its address.
func StartRelay(t *testing.T, cfg relay.Config) string {
	t.Helper()

	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	srv, err := relay.New(cfg)
	if err != nil {
		t.Fatalf("relay.New() = %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Serve(ctx, ln)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	return ln.Addr().String()
}

// StartAgent announces as the agent for id and serves the handshake, handing
// the secured connection to serve. It stands in for the real agent, which does
// not exist yet.
func StartAgent(t *testing.T, relayAddr string, id proto.StreamID, key proto.Keypair, orchPub []byte, serve func(net.Conn)) {
	t.Helper()

	conn, err := net.Dial("tcp", relayAddr)
	if err != nil {
		t.Fatalf("agent Dial() = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	frame, err := proto.Frame{Version: proto.Version, Role: proto.RoleAgent, StreamID: id}.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary() = %v", err)
	}
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("agent announce: %v", err)
	}

	go func() {
		secure, err := proto.ConnectAsAgent(conn, key, orchPub, 15*time.Second)
		if err != nil {
			return
		}
		serve(secure)
	}()
}

// echoService copies everything back, standing in for the local ssh endpoint
// the real agent splices onto.
func echoService(c net.Conn) {
	defer c.Close()
	_, _ = io.Copy(c, c)
}

func mustKeypair(t *testing.T) proto.Keypair {
	t.Helper()
	k, err := proto.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() = %v", err)
	}
	return k
}

func TestConnectReachesTheAgentThroughTheRelay(t *testing.T) {
	addr := StartRelay(t, relay.Config{})
	agentKey, orchKey := mustKeypair(t), mustKeypair(t)
	const id proto.StreamID = "poc-target-01"

	StartAgent(t, addr, id, agentKey, orchKey.Public, echoService)

	stream, agentPub, err := Connect(addr, id, orchKey, 15*time.Second)
	if err != nil {
		t.Fatalf("Connect() = %v, want nil", err)
	}
	defer stream.Close()

	if !bytes.Equal(agentPub, agentKey.Public) {
		t.Fatal("Connect() did not return the agent's static key for later pinning")
	}

	want := []byte("SSH-2.0-OpenSSH_10.5\r\n")
	if _, err := stream.Write(want); err != nil {
		t.Fatalf("Write() = %v", err)
	}
	got := make([]byte, len(want))
	if err := stream.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(stream, got); err != nil {
		t.Fatalf("ReadFull() = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("round trip = %q, want %q", got, want)
	}
}

func TestMalformedStreamIDIsRefusedBeforeDialling(t *testing.T) {
	// 127.0.0.1:1 would refuse a connection; reaching it at all would prove the
	// id was not checked first.
	hostile := proto.StreamID("id\nlevel=INFO msg=\"forged\"")

	_, _, err := Connect("127.0.0.1:1", hostile, mustKeypair(t), time.Second)
	if !errors.Is(err, proto.ErrInvalidStreamID) {
		t.Fatalf("Connect() = %v, want ErrInvalidStreamID", err)
	}
	if strings.Contains(err.Error(), "forged") {
		t.Fatalf("error %q echoes the rejected id", err)
	}
}

func TestUnreachableRelayNamesTheAddress(t *testing.T) {
	// Bind and immediately release, so the address is almost certainly dead.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dead := ln.Addr().String()
	_ = ln.Close()

	_, _, err = Connect(dead, "poc-target-01", mustKeypair(t), 2*time.Second)
	if !errors.Is(err, ErrRelayUnreachable) {
		t.Fatalf("Connect() = %v, want ErrRelayUnreachable", err)
	}
	if !strings.Contains(err.Error(), dead) {
		t.Fatalf("error %q does not name the relay address; an operator cannot act on it", err)
	}
}

func TestAbsentCounterpartIsReportedAsSuch(t *testing.T) {
	// A relay that parks us happily, with no agent ever arriving. This must not
	// look like a refused peer: the diagnosis is "the machine is not there",
	// which is a different thing to go and check.
	addr := StartRelay(t, relay.Config{RendezvousTimeout: time.Minute})

	_, _, err := Connect(addr, "poc-target-01", mustKeypair(t), 300*time.Millisecond)
	if !errors.Is(err, ErrCounterpartAbsent) {
		t.Fatalf("Connect() = %v, want ErrCounterpartAbsent", err)
	}
	if !strings.Contains(err.Error(), "poc-target-01") {
		t.Fatalf("error %q does not name the stream id waited on", err)
	}
}

func TestSpliceCopiesBothWaysAndReturnsWhenClosed(t *testing.T) {
	local, remote := net.Pipe()

	// A pipe rather than a fixed string: ssh keeps stdin open for the life of
	// the session, and Splice ends the stream when stdin closes. Handing it an
	// already-exhausted reader would model a session that ends immediately.
	inR, inW := io.Pipe()
	var out lockedBuffer

	done := make(chan error, 1)
	go func() { done <- Splice(local, inR, &out) }()

	if _, err := inW.Write([]byte("towards the stream")); err != nil {
		t.Fatal(err)
	}

	// What the client sent must arrive at the far end.
	got := make([]byte, len("towards the stream"))
	if _, err := io.ReadFull(remote, got); err != nil {
		t.Fatalf("ReadFull() = %v", err)
	}
	if string(got) != "towards the stream" {
		t.Fatalf("stream received %q", got)
	}

	// And what the far end sends must reach the writer.
	if _, err := remote.Write([]byte("towards the client")); err != nil {
		t.Fatal(err)
	}
	_ = remote.Close()
	_ = inW.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Splice() = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Splice() did not return after the far end closed")
	}

	if !strings.Contains(out.String(), "towards the client") {
		t.Fatalf("writer received %q", out.String())
	}
}

func TestPrivateKeyRoundTripsAndIsOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "orchestrator.key")

	key := mustKeypair(t)
	if err := WritePrivateKey(path, key); err != nil {
		t.Fatalf("WritePrivateKey() = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("key file mode is %04o, want 0600: it is the only secret in the system", perm)
	}

	got, err := ReadPrivateKey(path)
	if err != nil {
		t.Fatalf("ReadPrivateKey() = %v", err)
	}
	if !bytes.Equal(got.Private, key.Private) {
		t.Fatal("private half did not survive the round trip")
	}
	// The public half is derived, not stored, so a pair cannot drift apart.
	if !bytes.Equal(got.Public, key.Public) {
		t.Fatal("derived public half does not match the generated one")
	}
}

func TestWritePrivateKeyRefusesToOverwrite(t *testing.T) {
	// Replacing a key silently would lock out every agent configured with the
	// matching public half.
	path := filepath.Join(t.TempDir(), "orchestrator.key")

	first := mustKeypair(t)
	if err := WritePrivateKey(path, first); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := WritePrivateKey(path, mustKeypair(t)); err == nil {
		t.Fatal("WritePrivateKey() = nil, want a refusal to overwrite")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("the existing key file was modified by the refused write")
	}
}

// lockedBuffer is a bytes.Buffer that a copy goroutine and the test body may
// touch at the same time.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
