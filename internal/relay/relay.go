// Package relay is the rendezvous point where two parties that both dialled
// outward meet.
//
// It is deliberately incapable. It performs no cryptography, authenticates
// nobody, and cannot read what it carries, because the Noise handshake between
// agent and orchestrator runs end to end through it. Those absences are the
// product: they are what make a relay safe to host, trivial to self-host, and
// honest to offer as a service.
//
// What remains is a service that accepts connections from anyone and holds
// state between two arrivals that may never both come. Everything here that is
// not about pairing is about surviving that.
package relay

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

// Defaults chosen to be survivable rather than generous. A relay with no
// inbound-port alternative is a total outage when it falls over, so its bounds
// are a reliability property rather than a hardening detail.
const (
	// DefaultFrameTimeout bounds how long a connection may stay silent after
	// being accepted. It is short: announcing yourself costs one small write.
	DefaultFrameTimeout = 10 * time.Second

	// DefaultRendezvousTimeout bounds how long a party waits for its
	// counterpart. It is generous, because the usual wait is a machine
	// finishing its boot.
	DefaultRendezvousTimeout = 5 * time.Minute

	// DefaultMaxParked bounds concurrently parked connections.
	DefaultMaxParked = 1024
)

// ErrInvalidConfig reports a Server built with unusable settings.
var ErrInvalidConfig = errors.New("relay: invalid configuration")

// Config settings for a Server. The zero value is not usable; use New.
type Config struct {
	// FrameTimeout bounds reading the rendezvous frame.
	FrameTimeout time.Duration
	// RendezvousTimeout bounds how long a parked party waits to be paired.
	RendezvousTimeout time.Duration
	// MaxParked bounds concurrently parked connections.
	MaxParked int
	// Logger receives pair, refuse and expire records.
	Logger *slog.Logger
}

// waiting is a party parked under a stream id, waiting for its counterpart.
type waiting struct {
	conn net.Conn
	role proto.Role
	// paired is closed when this party has been handed to a splice, so the
	// expiry timer knows to leave it alone.
	paired chan struct{}
	once   sync.Once
}

// markPaired reports whether this call was the one that claimed the party.
// Pairing and expiry race by construction; exactly one must win.
func (w *waiting) markPaired() bool {
	claimed := false
	w.once.Do(func() {
		close(w.paired)
		claimed = true
	})
	return claimed
}

// Server is a relay. Create one with New and run it with Serve.
type Server struct {
	cfg Config
	log *slog.Logger

	mu     sync.Mutex
	parked map[proto.StreamID]*waiting
}

// New builds a Server, filling unset settings with defaults.
func New(cfg Config) (*Server, error) {
	if cfg.FrameTimeout == 0 {
		cfg.FrameTimeout = DefaultFrameTimeout
	}
	if cfg.RendezvousTimeout == 0 {
		cfg.RendezvousTimeout = DefaultRendezvousTimeout
	}
	if cfg.MaxParked == 0 {
		cfg.MaxParked = DefaultMaxParked
	}
	if cfg.MaxParked < 0 {
		return nil, fmt.Errorf("%w: MaxParked is %d, want a positive bound", ErrInvalidConfig, cfg.MaxParked)
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &Server{
		cfg:    cfg,
		log:    cfg.Logger,
		parked: make(map[proto.StreamID]*waiting),
	}, nil
}

// Serve accepts connections until ctx is cancelled or the listener fails.
//
// It returns nil on a clean shutdown, so a cancelled context is not an error:
// stopping is a thing an operator does, not a fault.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	var wg sync.WaitGroup
	defer wg.Wait()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("relay: accept: %w", err)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			s.handle(conn)
		}()
	}
}

// ParkedCount reports how many connections are currently waiting to be paired.
// Exported for tests, which assert it returns to zero.
func (s *Server) ParkedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.parked)
}

// handle takes one accepted connection through announcement and pairing.
func (s *Server) handle(conn net.Conn) {
	frame, err := proto.ReadFrameWithDeadline(conn, s.cfg.FrameTimeout)
	if err != nil {
		// The stream id is not logged here: it either does not exist yet or
		// failed validation, and echoing an unvalidated id into a log is the
		// injection that validation exists to prevent.
		s.log.Info("refused connection", "reason", err.Error(), "remote", conn.RemoteAddr().String())
		_ = conn.Close()
		return
	}

	w := &waiting{conn: conn, role: frame.Role, paired: make(chan struct{})}

	s.mu.Lock()
	if existing, ok := s.parked[frame.StreamID]; ok {
		if existing.role == frame.Role {
			// A taken role is never displaced. Stream ids are guessable, so
			// displacing an incumbent would let anyone reachable evict a live
			// session.
			s.mu.Unlock()
			s.log.Info("refused duplicate role",
				"stream", string(frame.StreamID), "role", frame.Role.String())
			_ = conn.Close()
			return
		}

		// The counterpart is here. Claim it, if expiry has not already.
		delete(s.parked, frame.StreamID)
		s.mu.Unlock()

		if !existing.markPaired() {
			// Expiry won the race and is closing it; this party has nobody.
			s.log.Info("counterpart expired during pairing", "stream", string(frame.StreamID))
			_ = conn.Close()
			return
		}

		s.log.Info("paired", "stream", string(frame.StreamID),
			"role", frame.Role.String(), "with", existing.role.String())
		s.splice(frame.StreamID, existing.conn, conn)
		return
	}

	if len(s.parked) >= s.cfg.MaxParked {
		// Refuse rather than absorb: an unmatched flood must not be able to
		// take the relay down for the sessions that are working.
		s.mu.Unlock()
		s.log.Warn("refused connection: parked bound reached",
			"stream", string(frame.StreamID), "bound", s.cfg.MaxParked)
		_ = conn.Close()
		return
	}

	s.parked[frame.StreamID] = w
	s.mu.Unlock()

	s.log.Info("parked", "stream", string(frame.StreamID), "role", frame.Role.String())
	s.expireAfter(frame.StreamID, w)
}

// expireAfter closes a parked party if it is not claimed before the rendezvous
// deadline. It blocks, running on the connection's own goroutine.
func (s *Server) expireAfter(id proto.StreamID, w *waiting) {
	timer := time.NewTimer(s.cfg.RendezvousTimeout)
	defer timer.Stop()

	select {
	case <-w.paired:
		// Claimed; the splice owns the connection now.
		return
	case <-timer.C:
	}

	if !w.markPaired() {
		// A pairing claimed it between the timer firing and this line.
		return
	}

	s.mu.Lock()
	// Only remove the entry if it is still ours: the id may have been reused.
	if cur, ok := s.parked[id]; ok && cur == w {
		delete(s.parked, id)
	}
	s.mu.Unlock()

	s.log.Info("expired unmatched connection", "stream", string(id), "role", w.role.String())
	_ = w.conn.Close()
}

// splice copies bytes in both directions until either side ends, then closes
// both. Everything here is opaque: the relay has no idea what it is carrying.
func (s *Server) splice(id proto.StreamID, a, b net.Conn) {
	defer func() {
		_ = a.Close()
		_ = b.Close()
		s.log.Info("session ended", "stream", string(id))
	}()

	done := make(chan struct{}, 2)

	copyOne := func(dst, src net.Conn) {
		defer func() { done <- struct{}{} }()
		_, _ = io.Copy(dst, src)
		// Closing the far side unblocks the opposite copy, so one party
		// hanging up ends the session rather than leaking a goroutine.
		_ = dst.Close()
		_ = src.Close()
	}

	go copyOne(a, b)
	go copyOne(b, a)

	<-done
	<-done
}
