// Package agent is the half that runs on the target.
//
// It dials outward and waits, which is the whole reason a machine with no
// inbound port can be reached at all. It listens on nothing.
//
// It also ships inside the boot image, and the boot image is one of the few
// things a closure push cannot replace — partition layout, filesystem, boot
// mode and this agent are the only things that genuinely force a rebuild. So
// the rule here is narrower than a feature list: the agent only has to be good
// enough to accept one push, after which a newer one arrives through the live
// configuration like any other package. Keep it dull.
package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/nivis-project/nivis-tunnel/internal/proto"
)

// Backoff bounds. The relay is often not reachable when a machine finishes
// booting, and the agent must find it without anyone intervening — because
// there is no way to intervene.
const (
	// DefaultMinBackoff is the first retry interval.
	DefaultMinBackoff = 500 * time.Millisecond

	// DefaultMaxBackoff caps the retry interval. It is deliberately short: a
	// relay coming back up must be found promptly, and a machine that waits
	// ten minutes to notice is a machine nobody can deploy to.
	DefaultMaxBackoff = 30 * time.Second

	// DefaultHandshakeTimeout bounds dialling, announcing and handshaking.
	DefaultHandshakeTimeout = 30 * time.Second
)

// ErrInvalidConfig reports an Agent built with unusable settings.
var ErrInvalidConfig = errors.New("agent: invalid configuration")

// Config settings for an Agent.
type Config struct {
	// RelayAddr is the rendezvous relay, host:port.
	RelayAddr string
	// StreamID is the rendezvous id this agent announces.
	StreamID proto.StreamID
	// OrchestratorPublicKey is the only peer this agent will talk to.
	OrchestratorPublicKey []byte
	// LocalAddr is where an accepted stream is spliced, normally localhost:22.
	LocalAddr string
	// Static is this agent's own keypair. Generated per run when unset: the
	// orchestrator does not verify it, so it need not persist.
	Static proto.Keypair

	MinBackoff       time.Duration
	MaxBackoff       time.Duration
	HandshakeTimeout time.Duration

	Logger *slog.Logger
}

// Agent dials a relay, waits, and splices one session at a time.
type Agent struct {
	cfg Config
	log *slog.Logger
}

// New validates the configuration and builds an Agent.
func New(cfg Config) (*Agent, error) {
	if cfg.RelayAddr == "" {
		return nil, fmt.Errorf("%w: no relay address", ErrInvalidConfig)
	}
	if err := cfg.StreamID.Validate(); err != nil {
		// Not echoed: the id came from configuration, but the same discipline
		// applies everywhere it could reach a log.
		return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}
	if len(cfg.OrchestratorPublicKey) == 0 {
		return nil, fmt.Errorf("%w: no orchestrator public key", ErrInvalidConfig)
	}
	if cfg.LocalAddr == "" {
		return nil, fmt.Errorf("%w: no local address to splice onto", ErrInvalidConfig)
	}

	if cfg.MinBackoff <= 0 {
		cfg.MinBackoff = DefaultMinBackoff
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = DefaultMaxBackoff
	}
	if cfg.HandshakeTimeout <= 0 {
		cfg.HandshakeTimeout = DefaultHandshakeTimeout
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	if len(cfg.Static.Private) == 0 {
		// The orchestrator does not verify the agent's key, so there is nothing
		// to persist and nothing for an operator to manage. A per-run key keeps
		// the boot image free of state.
		k, err := proto.GenerateKeypair()
		if err != nil {
			return nil, err
		}
		cfg.Static = k
	}

	return &Agent{cfg: cfg, log: cfg.Logger}, nil
}

// Run serves sessions until ctx is cancelled.
//
// It never returns because of a failed connection: a target that gives up is a
// target nobody can reach, and there is no second channel to fix it through.
func (a *Agent) Run(ctx context.Context) error {
	backoff := a.cfg.MinBackoff

	for {
		if ctx.Err() != nil {
			return nil
		}

		err := a.serveOne(ctx)
		switch {
		case ctx.Err() != nil:
			return nil
		case err == nil:
			// A session completed. Go straight back to waiting, so the machine
			// is reachable again immediately after a deploy.
			a.log.Info("session ended; waiting for the next")
			backoff = a.cfg.MinBackoff
			continue
		default:
			a.log.Warn("connection attempt failed", "error", err.Error(), "retry_in", backoff.String())
		}

		if !sleepCtx(ctx, backoff) {
			return nil
		}
		backoff = nextBackoff(backoff, a.cfg.MaxBackoff)
	}
}

// nextBackoff doubles the interval up to a ceiling.
func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
}

// sleepCtx waits for d, returning false if ctx was cancelled first. A cancelled
// agent must stop now, not at the end of the current backoff.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// serveOne runs a single rendezvous, handshake and session.
func (a *Agent) serveOne(ctx context.Context) error {
	dialer := net.Dialer{Timeout: a.cfg.HandshakeTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", a.cfg.RelayAddr)
	if err != nil {
		return fmt.Errorf("dialling relay %s: %w", a.cfg.RelayAddr, err)
	}
	defer conn.Close()

	// Stop waiting as soon as the agent is asked to.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	frame, err := proto.Frame{
		Version:  proto.Version,
		Role:     proto.RoleAgent,
		StreamID: a.cfg.StreamID,
	}.MarshalBinary()
	if err != nil {
		return err
	}
	if err := conn.SetWriteDeadline(time.Now().Add(a.cfg.HandshakeTimeout)); err != nil {
		return err
	}
	if _, err := conn.Write(frame); err != nil {
		return fmt.Errorf("announcing to relay: %w", err)
	}
	if err := conn.SetWriteDeadline(time.Time{}); err != nil {
		return err
	}

	a.log.Info("waiting for an orchestrator", "stream", string(a.cfg.StreamID), "relay", a.cfg.RelayAddr)

	// The relay parks us here until an orchestrator arrives. No deadline: a
	// machine may legitimately wait days between deploys, and the relay's own
	// rendezvous timeout is what bounds the wait.
	secure, err := proto.ConnectAsAgent(conn, a.cfg.Static, a.cfg.OrchestratorPublicKey, 0)
	if err != nil {
		if errors.Is(err, proto.ErrPeerRefused) {
			// Not a fault to back off from. Someone who is not the orchestrator
			// tried; the agent goes back to waiting.
			a.log.Warn("refused a peer that is not the configured orchestrator")
			return nil
		}
		return fmt.Errorf("handshake: %w", err)
	}
	defer secure.Close()

	a.log.Info("orchestrator connected", "stream", string(a.cfg.StreamID))

	local, err := net.Dial("tcp", a.cfg.LocalAddr)
	if err != nil {
		return fmt.Errorf("dialling local service %s: %w", a.cfg.LocalAddr, err)
	}
	defer local.Close()

	splice(secure, local)
	return nil
}

// splice copies in both directions until either side ends.
func splice(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	copyOne := func(dst, src net.Conn) {
		defer wg.Done()
		_, _ = io.Copy(dst, src)
		// Closing both unblocks the opposite direction, so one side hanging up
		// ends the session rather than leaking a goroutine on a small machine.
		_ = dst.Close()
		_ = src.Close()
	}

	go copyOne(a, b)
	go copyOne(b, a)
	wg.Wait()
}
