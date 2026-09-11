package proto

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

const testTimeout = 5 * time.Second

func mustKeypair(t *testing.T) Keypair {
	t.Helper()
	k, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() = %v, want nil", err)
	}
	return k
}

// tapRelay stands in for the real relay: it copies messages between two parties
// and records everything it sees — exactly as much as a relay operator could.
//
// It is message-aware rather than a byte pump so that a test can corrupt one
// message precisely, which is how tampering is exercised below.
type tapRelay struct {
	mu       sync.Mutex
	observed bytes.Buffer

	// corrupt, when non-nil, is consulted for each message travelling from the
	// orchestrator towards the agent and may alter it.
	corrupt func(index int, msg []byte) []byte
}

// pair wires an agent end and an orchestrator end together through the relay.
func (tr *tapRelay) pair(t *testing.T) (agentSide, orchSide net.Conn) {
	t.Helper()

	agentEnd, relayAgent := net.Pipe()
	relayOrch, orchEnd := net.Pipe()

	t.Cleanup(func() {
		_ = agentEnd.Close()
		_ = orchEnd.Close()
	})

	go tr.pump(relayAgent, relayOrch, nil)
	go tr.pump(relayOrch, relayAgent, tr.corrupt)

	return agentEnd, orchEnd
}

func (tr *tapRelay) pump(src, dst net.Conn, corrupt func(int, []byte) []byte) {
	defer func() {
		_ = src.Close()
		_ = dst.Close()
	}()

	for i := 0; ; i++ {
		msg, err := readNoiseMessage(src)
		if err != nil {
			return
		}

		tr.mu.Lock()
		tr.observed.Write(msg)
		tr.mu.Unlock()

		if corrupt != nil {
			msg = corrupt(i, msg)
		}
		if err := writeNoiseMessage(dst, msg); err != nil {
			return
		}
	}
}

func (tr *tapRelay) seen() []byte {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return bytes.Clone(tr.observed.Bytes())
}

// handshakeThrough runs both halves concurrently and returns the two secured
// connections.
func handshakeThrough(t *testing.T, tr *tapRelay, agent, orch Keypair, agentExpects []byte) (agentConn, orchConn net.Conn) {
	t.Helper()

	agentSide, orchSide := tr.pair(t)

	type result struct {
		conn net.Conn
		peer []byte
		err  error
	}
	orchCh := make(chan result, 1)
	go func() {
		c, peer, err := ConnectAsOrchestrator(orchSide, orch, testTimeout)
		orchCh <- result{c, peer, err}
	}()

	agentConn, err := ConnectAsAgent(agentSide, agent, agentExpects, testTimeout)
	if err != nil {
		t.Fatalf("ConnectAsAgent() = %v, want nil", err)
	}

	r := <-orchCh
	if r.err != nil {
		t.Fatalf("ConnectAsOrchestrator() = %v, want nil", r.err)
	}
	if !bytes.Equal(r.peer, agent.Public) {
		t.Fatal("ConnectAsOrchestrator() did not return the agent's static key")
	}

	return agentConn, r.conn
}

func TestHandshakeCompletesAndCarriesBytes(t *testing.T) {
	agent, orch := mustKeypair(t), mustKeypair(t)
	tr := &tapRelay{}

	agentConn, orchConn := handshakeThrough(t, tr, agent, orch, orch.Public)

	want := []byte("SSH-2.0-OpenSSH_10.5\r\n")
	go func() { _, _ = orchConn.Write(want) }()

	got := make([]byte, len(want))
	if _, err := io.ReadFull(agentConn, got); err != nil {
		t.Fatalf("ReadFull() = %v, want nil", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("carried %q, want %q", got, want)
	}
}

func TestAgentRefusesAnUnknownOrchestrator(t *testing.T) {
	// The agent's entire access control mechanism: it talks to exactly one
	// orchestrator and to no other.
	//
	// An impostor cannot get as far as answering. Handshake message 1 is
	// encrypted to the expected orchestrator's static key, so a peer without
	// the matching private key cannot read it and therefore cannot produce a
	// valid reply. The property under test is that the agent never ends up
	// holding a session it believes in — which is exactly what the earlier
	// Noise_KX arrangement got wrong.
	realOrch, impostor, agent := mustKeypair(t), mustKeypair(t), mustKeypair(t)
	tr := &tapRelay{}
	agentSide, orchSide := tr.pair(t)

	go func() {
		// A real party that fails a handshake closes its connection; model
		// that, so the agent is not left waiting on a deadline.
		defer orchSide.Close()
		if _, _, err := ConnectAsOrchestrator(orchSide, impostor, testTimeout); err == nil {
			t.Error("ConnectAsOrchestrator() = nil for an impostor, want failure: it cannot decrypt message 1")
		}
	}()

	conn, err := ConnectAsAgent(agentSide, agent, realOrch.Public, testTimeout)
	if err == nil {
		t.Fatal("ConnectAsAgent() = nil, want a refusal for an unknown orchestrator")
	}
	if conn != nil {
		t.Fatal("ConnectAsAgent() returned a connection despite refusing the peer")
	}
}

func TestAgentRefusesAResponderThatCannotProveItself(t *testing.T) {
	// The explicit-refusal path: a peer that answers with something it did not
	// derive from the orchestrator's private key is rejected during the
	// handshake, named as such, before any payload is carried.
	realOrch, agent := mustKeypair(t), mustKeypair(t)
	agentSide, otherSide := net.Pipe()
	t.Cleanup(func() { _ = agentSide.Close() })

	go func() {
		defer otherSide.Close()
		if _, err := readNoiseMessage(otherSide); err != nil {
			return
		}
		// A well-formed frame carrying bytes the agent cannot authenticate.
		_ = writeNoiseMessage(otherSide, make([]byte, 48))
	}()

	_, err := ConnectAsAgent(agentSide, agent, realOrch.Public, testTimeout)
	if !errors.Is(err, ErrPeerRefused) {
		t.Fatalf("ConnectAsAgent() = %v, want ErrPeerRefused", err)
	}
}

func TestConfiguredOrchestratorIsAccepted(t *testing.T) {
	agent, orch := mustKeypair(t), mustKeypair(t)
	tr := &tapRelay{}

	agentConn, orchConn := handshakeThrough(t, tr, agent, orch, orch.Public)
	if agentConn == nil || orchConn == nil {
		t.Fatal("handshake did not yield two secured connections")
	}
}

func TestPayloadLargerThanOneNoiseMessageRoundTrips(t *testing.T) {
	// ssh and nix-copy-closure both push far more than 64 KiB, so a payload
	// spanning several Noise messages is the normal case, not the edge.
	agent, orch := mustKeypair(t), mustKeypair(t)
	tr := &tapRelay{}
	agentConn, orchConn := handshakeThrough(t, tr, agent, orch, orch.Public)

	want := make([]byte, 3*maxPlaintextChunk+1234)
	if _, err := rand.Read(want); err != nil {
		t.Fatalf("rand.Read() = %v", err)
	}

	go func() { _, _ = orchConn.Write(want) }()

	got := make([]byte, len(want))
	if _, err := io.ReadFull(agentConn, got); err != nil {
		t.Fatalf("ReadFull() = %v, want nil", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("payload spanning several Noise messages did not survive the round trip")
	}
}

func TestRelayCannotReadWhatItCarries(t *testing.T) {
	// The relay being unable to read its traffic is what makes it safe to host
	// and honest to offer as a service. Assert it against what the relay
	// actually saw, not against a claim about the cipher suite.
	agent, orch := mustKeypair(t), mustKeypair(t)
	tr := &tapRelay{}
	agentConn, orchConn := handshakeThrough(t, tr, agent, orch, orch.Public)

	secret := []byte("ADMIN_TOKEN=hunter2-do-not-leak-this")
	go func() { _, _ = orchConn.Write(secret) }()

	got := make([]byte, len(secret))
	if _, err := io.ReadFull(agentConn, got); err != nil {
		t.Fatalf("ReadFull() = %v, want nil", err)
	}
	if !bytes.Equal(got, secret) {
		t.Fatal("the payload did not arrive intact")
	}

	if bytes.Contains(tr.seen(), secret) {
		t.Fatal("the relay observed the plaintext it was carrying")
	}
}

func TestTamperingIsDetected(t *testing.T) {
	// An intermediary that alters ciphertext must cause a failure, not delivery
	// of altered bytes. In the orchestrator->agent direction, message index 0
	// is handshake message 2; index 1 is the first transport message.
	agent, orch := mustKeypair(t), mustKeypair(t)
	tr := &tapRelay{
		corrupt: func(index int, msg []byte) []byte {
			if index == 1 && len(msg) > 0 {
				out := bytes.Clone(msg)
				out[0] ^= 0xff
				return out
			}
			return msg
		},
	}

	agentConn, orchConn := handshakeThrough(t, tr, agent, orch, orch.Public)

	go func() { _, _ = orchConn.Write([]byte("this will be altered in flight")) }()

	buf := make([]byte, 64)
	if _, err := agentConn.Read(buf); err == nil {
		t.Fatal("Read() = nil, want an authentication error for tampered ciphertext")
	}
}

func TestAgentRejectsAWrongLengthOrchestratorKey(t *testing.T) {
	agent := mustKeypair(t)
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	if _, err := ConnectAsAgent(client, agent, []byte("short"), testTimeout); err == nil {
		t.Fatal("ConnectAsAgent() = nil, want an error for a wrong-length key")
	}
}

func TestPublicKeyEncodingRoundTrips(t *testing.T) {
	// Public keys sit in NixOS module options and in a public repository, so
	// they need a textual form that survives being copied by hand.
	k := mustKeypair(t)

	got, err := DecodePublicKey(EncodePublicKey(k.Public))
	if err != nil {
		t.Fatalf("DecodePublicKey() = %v, want nil", err)
	}
	if !bytes.Equal(got, k.Public) {
		t.Fatal("public key did not survive the encoding round trip")
	}
}

func TestDecodePublicKeyRejectsWrongLength(t *testing.T) {
	if _, err := DecodePublicKey(EncodePublicKey([]byte("too short"))); err == nil {
		t.Fatal("DecodePublicKey() = nil, want an error for a wrong-length key")
	}
}

func TestNoiseMessageLengthPrefixIsBounded(t *testing.T) {
	// The prefix is a uint16, so a peer cannot induce an allocation larger than
	// the Noise framework's own message limit.
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, uint16(maxNoiseMessage)); err != nil {
		t.Fatalf("binary.Write() = %v", err)
	}
	buf.Write(make([]byte, maxNoiseMessage))

	msg, err := readNoiseMessage(&buf)
	if err != nil {
		t.Fatalf("readNoiseMessage() = %v, want nil", err)
	}
	if len(msg) != maxNoiseMessage {
		t.Fatalf("len = %d, want %d", len(msg), maxNoiseMessage)
	}

	if err := writeNoiseMessage(io.Discard, make([]byte, maxNoiseMessage+1)); err == nil {
		t.Fatal("writeNoiseMessage() = nil, want an error beyond the message limit")
	}
}

func TestZeroTimeoutMeansNoDeadline(t *testing.T) {
	// The agent parks at the relay until an orchestrator arrives, and a machine
	// may legitimately wait days between deploys. A zero timeout must mean "no
	// deadline", not "a deadline of now" — which is what SetDeadline(now+0)
	// would produce, failing every handshake instantly.
	agent, orch := mustKeypair(t), mustKeypair(t)
	tr := &tapRelay{}
	agentSide, orchSide := tr.pair(t)

	orchCh := make(chan error, 1)
	go func() {
		_, _, err := ConnectAsOrchestrator(orchSide, orch, 0)
		orchCh <- err
	}()

	if _, err := ConnectAsAgent(agentSide, agent, orch.Public, 0); err != nil {
		t.Fatalf("ConnectAsAgent(timeout=0) = %v, want nil", err)
	}
	if err := <-orchCh; err != nil {
		t.Fatalf("ConnectAsOrchestrator(timeout=0) = %v, want nil", err)
	}
}
