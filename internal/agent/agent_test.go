package agent

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/nivis-project/nivis-tunnel/internal/proto"
	"github.com/nivis-project/nivis-tunnel/internal/relay"
	"github.com/nivis-project/nivis-tunnel/internal/tunnel"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func mustKeypair(t *testing.T) proto.Keypair {
	t.Helper()
	k, err := proto.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() = %v", err)
	}
	return k
}

// startRelay brings up a real relay and returns its address.
func startRelay(t *testing.T) string {
	t.Helper()

	srv, err := relay.New(relay.Config{Logger: quiet()})
	if err != nil {
		t.Fatalf("relay.New() = %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _ = srv.Serve(ctx, ln) }()
	t.Cleanup(func() { cancel(); <-done })

	return ln.Addr().String()
}

// startEchoService stands in for sshd: the local thing the agent splices onto.
func startEchoService(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); _, _ = io.Copy(c, c) }()
		}
	}()

	return ln.Addr().String()
}

// runAgent starts an agent and returns when the test ends.
func runAgent(t *testing.T, cfg Config) {
	t.Helper()

	cfg.Logger = quiet()
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Run() = %v, want nil on cancellation", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("Run() did not return after cancellation")
		}
	})
}

// exchange connects as the orchestrator and round-trips a payload.
func exchange(t *testing.T, relayAddr string, id proto.StreamID, orchKey proto.Keypair, payload string) error {
	t.Helper()

	stream, _, err := tunnel.Connect(relayAddr, id, orchKey, 15*time.Second)
	if err != nil {
		return err
	}
	defer stream.Close()

	if err := stream.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	if _, err := stream.Write([]byte(payload)); err != nil {
		return err
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(stream, got); err != nil {
		return err
	}
	if !bytes.Equal(got, []byte(payload)) {
		return errors.New("payload did not survive the agent")
	}
	return nil
}

func TestNewRejectsUnusableConfiguration(t *testing.T) {
	orch := mustKeypair(t)
	base := Config{
		RelayAddr:             "127.0.0.1:1",
		StreamID:              "poc-target-01",
		OrchestratorPublicKey: orch.Public,
		LocalAddr:             "127.0.0.1:22",
	}

	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{"no relay", func(c *Config) { c.RelayAddr = "" }},
		{"malformed stream id", func(c *Config) { c.StreamID = "../etc/passwd" }},
		{"no orchestrator key", func(c *Config) { c.OrchestratorPublicKey = nil }},
		{"no local address", func(c *Config) { c.LocalAddr = "" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			cfg.Logger = quiet()
			tc.mutate(&cfg)
			if _, err := New(cfg); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("New() = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

func TestAgentCarriesBytesToTheLocalService(t *testing.T) {
	relayAddr := startRelay(t)
	local := startEchoService(t)
	orch := mustKeypair(t)
	const id proto.StreamID = "poc-target-01"

	runAgent(t, Config{
		RelayAddr:             relayAddr,
		StreamID:              id,
		OrchestratorPublicKey: orch.Public,
		LocalAddr:             local,
	})

	if err := exchange(t, relayAddr, id, orch, "SSH-2.0-OpenSSH_10.5\r\n"); err != nil {
		t.Fatalf("session: %v", err)
	}
}

func TestAgentIsReachableAgainAfterASession(t *testing.T) {
	// A machine must be reachable immediately after a deploy without anything
	// restarting it — there is no second channel to restart it through.
	relayAddr := startRelay(t)
	local := startEchoService(t)
	orch := mustKeypair(t)
	const id proto.StreamID = "poc-target-01"

	runAgent(t, Config{
		RelayAddr:             relayAddr,
		StreamID:              id,
		OrchestratorPublicKey: orch.Public,
		LocalAddr:             local,
	})

	if err := exchange(t, relayAddr, id, orch, "first session"); err != nil {
		t.Fatalf("first session: %v", err)
	}
	if err := retryExchange(t, relayAddr, id, orch, "second session"); err != nil {
		t.Fatalf("second session: %v", err)
	}
}

func TestAgentStaysAvailableAfterRefusingAnImpostor(t *testing.T) {
	// Refusing a peer is not a fault to back off from. A machine that could be
	// taken out of reach by anyone who connects to it would be worse than one
	// with an open port.
	relayAddr := startRelay(t)
	local := startEchoService(t)
	orch, impostor := mustKeypair(t), mustKeypair(t)
	const id proto.StreamID = "poc-target-01"

	runAgent(t, Config{
		RelayAddr:             relayAddr,
		StreamID:              id,
		OrchestratorPublicKey: orch.Public,
		LocalAddr:             local,
	})

	// The impostor cannot complete; it must not take the agent with it.
	if err := exchange(t, relayAddr, id, impostor, "should not work"); err == nil {
		t.Fatal("an impostor established a session")
	}

	if err := retryExchange(t, relayAddr, id, orch, "legitimate"); err != nil {
		t.Fatalf("the agent did not survive refusing an impostor: %v", err)
	}
}

func TestAgentWaitsForARelayThatIsNotUpYet(t *testing.T) {
	// A target boots before the relay is necessarily reachable, and nobody can
	// intervene: there is no other way in.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close() // nothing is listening there yet

	local := startEchoService(t)
	orch := mustKeypair(t)
	const id proto.StreamID = "poc-target-01"

	runAgent(t, Config{
		RelayAddr:             addr,
		StreamID:              id,
		OrchestratorPublicKey: orch.Public,
		LocalAddr:             local,
		MinBackoff:            20 * time.Millisecond,
		MaxBackoff:            100 * time.Millisecond,
	})

	// Give the agent a few failed attempts, then bring the relay up on that
	// very address.
	time.Sleep(150 * time.Millisecond)

	srv, err := relay.New(relay.Config{Logger: quiet()})
	if err != nil {
		t.Fatal(err)
	}
	ln2, err := net.Listen("tcp", addr)
	if err != nil {
		t.Skipf("could not re-bind %s: %v", addr, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _ = srv.Serve(ctx, ln2) }()
	t.Cleanup(func() { cancel(); <-done })

	if err := retryExchange(t, addr, id, orch, "after the relay appeared"); err != nil {
		t.Fatalf("the agent never found the relay once it appeared: %v", err)
	}
}

func TestRunStopsPromptlyMidBackoff(t *testing.T) {
	// Cancellation must not wait out the current interval.
	a, err := New(Config{
		RelayAddr:             "127.0.0.1:1",
		StreamID:              "poc-target-01",
		OrchestratorPublicKey: mustKeypair(t).Public,
		LocalAddr:             "127.0.0.1:22",
		MinBackoff:            time.Hour,
		MaxBackoff:            time.Hour,
		Logger:                quiet(),
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	time.Sleep(100 * time.Millisecond) // let it fail once and enter backoff
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() waited out an hour-long backoff instead of stopping")
	}
}

func TestBackoffRisesThenPlateaus(t *testing.T) {
	// Bounded on purpose: a relay coming back up must be found promptly, and a
	// machine that waits ten minutes to notice is a machine nobody can deploy
	// to.
	const max = 8 * time.Second
	got := []time.Duration{}
	d := time.Second
	for i := 0; i < 6; i++ {
		got = append(got, d)
		d = nextBackoff(d, max)
	}

	want := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, max, max, max}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("backoff sequence = %v, want %v", got, want)
		}
	}
}

// retryExchange allows for the agent needing a moment to re-announce between
// sessions; the assertion is that it gets there, not that it is instant.
func retryExchange(t *testing.T, relayAddr string, id proto.StreamID, orch proto.Keypair, payload string) error {
	t.Helper()

	var last error
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if last = exchange(t, relayAddr, id, orch, payload); last == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return last
}
