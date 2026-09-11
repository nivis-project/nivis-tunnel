// Command nivis-tunnel-relay is the rendezvous point where two parties that
// both dialled outward meet.
//
// It matches connections by stream id and copies bytes between them. It holds
// no key material, authenticates nobody, and cannot read what it carries — the
// Noise handshake between agent and orchestrator runs end to end through it.
// Those absences are deliberate: they are what make a relay safe to host and
// trivial to self-host.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/nivis-project/nivis-tunnel/internal/relay"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "nivis-tunnel-relay: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	listen := flag.String("listen", ":7843",
		"address to accept rendezvous connections on")
	frameTimeout := flag.Duration("frame-timeout", relay.DefaultFrameTimeout,
		"how long a connection may stay silent after being accepted before it is closed")
	rendezvousTimeout := flag.Duration("rendezvous-timeout", relay.DefaultRendezvousTimeout,
		"how long a party waits for its counterpart before it is released")
	maxParked := flag.Int("max-parked", relay.DefaultMaxParked,
		"how many connections may wait to be paired at once; further ones are refused")
	logLevel := flag.String("log-level", "info", "debug, info, warn or error")
	showVersion := flag.Bool("version", false, "print the version and exit")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"nivis-tunnel-relay %s — rendezvous for parties that cannot be dialled.\n\n"+
				"The agent on a target and the orchestrator both dial this relay and\n"+
				"announce the same stream id. It pairs them and copies bytes between\n"+
				"them. It cannot read what it carries.\n\nOptions:\n", version)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return nil
	}

	level, err := parseLevel(*logLevel)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	srv, err := relay.New(relay.Config{
		FrameTimeout:      *frameTimeout,
		RendezvousTimeout: *rendezvousTimeout,
		MaxParked:         *maxParked,
		Logger:            log,
	})
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", *listen, err)
	}

	// A relay is the only way in to a machine with no inbound port, so an
	// outage stops every deploy against it. Shut down deliberately rather than
	// abruptly: Serve stops accepting and waits for live sessions to finish.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("relay listening",
		"addr", ln.Addr().String(),
		"version", version,
		"max_parked", *maxParked,
		"rendezvous_timeout", rendezvousTimeout.String())

	if err := srv.Serve(ctx, ln); err != nil {
		return err
	}

	log.Info("relay stopped")
	return nil
}

func parseLevel(s string) (slog.Level, error) {
	switch s {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q: want debug, info, warn or error", s)
	}
}
