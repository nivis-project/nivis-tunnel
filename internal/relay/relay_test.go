package relay

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nivis-project/nivis-tunnel/internal/proto"
)

// captureLogger collects every record so tests can assert on what was, and was
// not, written.
type captureLogger struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (c *captureLogger) logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(c, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func (c *captureLogger) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(p)
}

func (c *captureLogger) text() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

// start brings up a real relay on a real listener and returns its address.
func start(t *testing.T, cfg Config) (*Server, string, *captureLogger) {
	t.Helper()

	cap := &captureLogger{}
	cfg.Logger = cap.logger()

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New() = %v, want nil", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v, want nil", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx, ln) }()

	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Serve() = %v, want nil on shutdown", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("Serve() did not return after context cancellation")
		}
	})

	return srv, ln.Addr().String(), cap
}

// announce dials the relay and sends a rendezvous frame.
func announce(t *testing.T, addr string, role proto.Role, id proto.StreamID) net.Conn {
	t.Helper()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	buf, err := proto.Frame{Version: proto.Version, Role: role, StreamID: id}.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary() = %v, want nil", err)
	}
	if _, err := conn.Write(buf); err != nil {
		t.Fatalf("Write() = %v, want nil", err)
	}
	return conn
}

func TestNewRejectsANegativeBound(t *testing.T) {
	if _, err := New(Config{MaxParked: -1}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("New() = %v, want ErrInvalidConfig", err)
	}
}

func TestPairedPartiesExchangeBytesBothWays(t *testing.T) {
	_, addr, _ := start(t, Config{})

	agent := announce(t, addr, proto.RoleAgent, "poc-target-01")
	orch := announce(t, addr, proto.RoleOrchestrator, "poc-target-01")

	if err := roundTrip(orch, agent, []byte("from orchestrator")); err != nil {
		t.Fatalf("orchestrator -> agent: %v", err)
	}
	if err := roundTrip(agent, orch, []byte("from agent")); err != nil {
		t.Fatalf("agent -> orchestrator: %v", err)
	}
}

func roundTrip(from, to net.Conn, payload []byte) error {
	if err := from.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	if _, err := from.Write(payload); err != nil {
		return err
	}
	if err := to.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(to, got); err != nil {
		return err
	}
	if !bytes.Equal(got, payload) {
		return errors.New("payload did not survive the splice")
	}
	return nil
}

func TestClosingOneSideEndsTheSession(t *testing.T) {
	srv, addr, _ := start(t, Config{})

	agent := announce(t, addr, proto.RoleAgent, "poc-target-01")
	orch := announce(t, addr, proto.RoleOrchestrator, "poc-target-01")
	if err := roundTrip(orch, agent, []byte("ping")); err != nil {
		t.Fatalf("splice did not form: %v", err)
	}

	_ = orch.Close()

	if err := agent.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := agent.Read(make([]byte, 1)); err == nil {
		t.Fatal("Read() = nil after the peer closed, want EOF")
	}

	waitFor(t, func() bool { return srv.ParkedCount() == 0 }, "parked count did not return to zero")
}

func TestStreamIDIsReusableAfterASession(t *testing.T) {
	_, addr, _ := start(t, Config{})

	a1 := announce(t, addr, proto.RoleAgent, "poc-target-01")
	o1 := announce(t, addr, proto.RoleOrchestrator, "poc-target-01")
	if err := roundTrip(o1, a1, []byte("first")); err != nil {
		t.Fatalf("first session: %v", err)
	}
	_ = o1.Close()
	_ = a1.Close()

	// The id must come back into circulation, or a machine could never
	// reconnect after a dropped session.
	var lastErr error
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a2 := announce(t, addr, proto.RoleAgent, "poc-target-01")
		o2 := announce(t, addr, proto.RoleOrchestrator, "poc-target-01")
		if lastErr = roundTrip(o2, a2, []byte("second")); lastErr == nil {
			return
		}
		_ = a2.Close()
		_ = o2.Close()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("stream id was not reusable: %v", lastErr)
}

func TestDuplicateRoleIsRefusedAndIncumbentSurvives(t *testing.T) {
	_, addr, cap := start(t, Config{})

	incumbent := announce(t, addr, proto.RoleAgent, "poc-target-01")
	intruder := announce(t, addr, proto.RoleAgent, "poc-target-01")

	// The intruder must be closed.
	if err := intruder.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := intruder.Read(make([]byte, 1)); err == nil {
		t.Fatal("the duplicate claim was not refused")
	}

	// And the incumbent must still be usable — an attacker who guesses an id
	// must not be able to evict a live session.
	orch := announce(t, addr, proto.RoleOrchestrator, "poc-target-01")
	if err := roundTrip(orch, incumbent, []byte("still here")); err != nil {
		t.Fatalf("incumbent was disturbed by the refused claim: %v", err)
	}

	if !strings.Contains(cap.text(), "refused duplicate role") {
		t.Fatal("the refusal was not logged")
	}
}

func TestUnmatchedPartyIsReleased(t *testing.T) {
	srv, addr, cap := start(t, Config{RendezvousTimeout: 150 * time.Millisecond})

	lonely := announce(t, addr, proto.RoleAgent, "poc-target-01")

	if err := lonely.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := lonely.Read(make([]byte, 1)); err == nil {
		t.Fatal("the unmatched party was not released")
	}

	waitFor(t, func() bool { return srv.ParkedCount() == 0 }, "expired connection was not unparked")

	if !strings.Contains(cap.text(), "expired unmatched connection") {
		t.Fatal("the expiry was not logged")
	}
}

func TestSilentConnectionIsClosedWithoutTakingAnID(t *testing.T) {
	srv, addr, _ := start(t, Config{FrameTimeout: 150 * time.Millisecond})

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial() = %v", err)
	}
	defer conn.Close()

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("a silent connection was not closed")
	}
	if got := srv.ParkedCount(); got != 0 {
		t.Fatalf("ParkedCount() = %d, want 0: a silent connection took a slot", got)
	}
}

func TestFloodIsRefusedRatherThanAbsorbed(t *testing.T) {
	srv, addr, cap := start(t, Config{MaxParked: 2, RendezvousTimeout: time.Minute})

	held := []net.Conn{
		announce(t, addr, proto.RoleAgent, "target-a"),
		announce(t, addr, proto.RoleAgent, "target-b"),
	}
	waitFor(t, func() bool { return srv.ParkedCount() == 2 }, "parked connections did not settle")

	overflow := announce(t, addr, proto.RoleAgent, "target-c")
	if err := overflow.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := overflow.Read(make([]byte, 1)); err == nil {
		t.Fatal("a connection beyond the bound was accepted")
	}

	// The sessions that were already working must be untouched by the refusal.
	for i, id := range []proto.StreamID{"target-a", "target-b"} {
		orch := announce(t, addr, proto.RoleOrchestrator, id)
		if err := roundTrip(orch, held[i], []byte("unaffected")); err != nil {
			t.Fatalf("parked connection %s was disturbed by the flood: %v", id, err)
		}
	}

	if !strings.Contains(cap.text(), "parked bound reached") {
		t.Fatal("the bound refusal was not logged")
	}
}

func TestWrongProtocolIsRefusedPromptly(t *testing.T) {
	_, addr, _ := start(t, Config{FrameTimeout: 30 * time.Second})

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial() = %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("GET / HTTP/1.1\r\nHost: relay\r\n\r\n")); err != nil {
		t.Fatal(err)
	}

	// Promptly means "without waiting for the frame deadline", which is 30s
	// here; 5s is comfortably inside that.
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("a wrong-protocol connection was not refused")
	}
}

func TestHostileStreamIDNeverReachesTheLogs(t *testing.T) {
	_, addr, cap := start(t, Config{})

	hostile := "id\nlevel=INFO msg=\"forged record\""

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial() = %v", err)
	}
	defer conn.Close()

	// Built by hand: MarshalBinary would refuse to produce this.
	buf := []byte{'N', 'V', 'T', 'L', 0, byte(proto.Version), byte(proto.RoleAgent), byte(len(hostile))}
	buf = append(buf, hostile...)
	if _, err := conn.Write(buf); err != nil {
		t.Fatal(err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, _ = conn.Read(make([]byte, 1))

	if strings.Contains(cap.text(), "forged record") {
		t.Fatal("a rejected stream id reached the log, forging a record")
	}
}

func TestNoStateOutlivesManySessions(t *testing.T) {
	srv, addr, _ := start(t, Config{})

	for i := 0; i < 25; i++ {
		a := announce(t, addr, proto.RoleAgent, "churn-target")
		o := announce(t, addr, proto.RoleOrchestrator, "churn-target")
		if err := roundTrip(o, a, []byte("cycle")); err != nil {
			// The id is released asynchronously, so an occasional cycle may
			// race; retry rather than fail, since the assertion below is the
			// real one.
			_ = a.Close()
			_ = o.Close()
			time.Sleep(10 * time.Millisecond)
			continue
		}
		_ = a.Close()
		_ = o.Close()
	}

	waitFor(t, func() bool { return srv.ParkedCount() == 0 },
		"parked state outlived its sessions")
}

func TestRelayCannotReadTheHandshakeItCarries(t *testing.T) {
	// The end-to-end claim, against a real listener and a real handshake: what
	// the relay copies must not be recoverable. This is the property that makes
	// hosting one an honest offer.
	_, addr, _ := start(t, Config{})

	agentKey, orchKey := mustKeypair(t), mustKeypair(t)

	agentConn := announce(t, addr, proto.RoleAgent, "poc-target-01")
	orchConn := announce(t, addr, proto.RoleOrchestrator, "poc-target-01")

	type res struct {
		conn net.Conn
		err  error
	}
	orchCh := make(chan res, 1)
	go func() {
		c, _, err := proto.ConnectAsOrchestrator(orchConn, orchKey, 10*time.Second)
		orchCh <- res{c, err}
	}()

	secureAgent, err := proto.ConnectAsAgent(agentConn, agentKey, orchKey.Public, 10*time.Second)
	if err != nil {
		t.Fatalf("ConnectAsAgent() = %v, want nil", err)
	}
	r := <-orchCh
	if r.err != nil {
		t.Fatalf("ConnectAsOrchestrator() = %v, want nil", r.err)
	}

	secret := []byte("ADMIN_TOKEN=hunter2-relay-must-not-see-this")
	go func() { _, _ = r.conn.Write(secret) }()

	got := make([]byte, len(secret))
	if err := secureAgent.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(secureAgent, got); err != nil {
		t.Fatalf("ReadFull() = %v, want nil", err)
	}
	if !bytes.Equal(got, secret) {
		t.Fatal("the payload did not survive the relay")
	}
}

func mustKeypair(t *testing.T) proto.Keypair {
	t.Helper()
	k, err := proto.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() = %v", err)
	}
	return k
}

func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(msg)
}

func TestShutdownDoesNotWaitOutRendezvousDeadlines(t *testing.T) {
	// Regression. Serve used to wait for every parked connection's rendezvous
	// deadline before returning, so a relay asked to stop would hang for up to
	// five minutes — and with no inbound port anywhere, a relay that will not
	// restart is every deploy against it stopped.
	cap := &captureLogger{}
	srv, err := New(Config{RendezvousTimeout: time.Hour, Logger: cap.logger()})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx, ln) }()

	// Park a party whose counterpart will never come.
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial() = %v", err)
	}
	defer conn.Close()
	frame, err := proto.Frame{Version: proto.Version, Role: proto.RoleAgent, StreamID: "poc-target-01"}.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(frame); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return srv.ParkedCount() == 1 }, "the party never parked")

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve() = %v, want nil on shutdown", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Serve() waited on a parked connection's rendezvous deadline instead of shutting down")
	}

	if !strings.Contains(cap.text(), "relay is shutting down") {
		t.Fatal("the released connection was not logged as a shutdown release")
	}
}
