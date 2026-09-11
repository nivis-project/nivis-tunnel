// Command nivis-tunnel-agent runs on the target.
//
// It dials outward to a relay and waits, which is what lets a machine with no
// inbound port be reached at all. It listens on nothing.
//
// It ships inside the boot image, and the boot image is one of the few things a
// closure push cannot replace. So it is deliberately dull: it only has to be
// good enough to accept one push, after which a newer agent arrives through the
// live configuration like any other package.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nivis-project/nivis-tunnel/internal/agent"
	"github.com/nivis-project/nivis-tunnel/internal/proto"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "nivis-tunnel-agent: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	relayAddr := flag.String("relay", "",
		"rendezvous relay to dial out to, host:port")
	streamID := flag.String("stream-id", "",
		"the rendezvous id this host announces; normally the cloud's instance id")
	orchKey := flag.String("orchestrator-key", "",
		"base64 public key of the only orchestrator this agent will talk to")
	localAddr := flag.String("local", "127.0.0.1:22",
		"local address an accepted stream is spliced onto")
	minBackoff := flag.Duration("min-backoff", agent.DefaultMinBackoff,
		"first retry interval when the relay cannot be reached")
	maxBackoff := flag.Duration("max-backoff", agent.DefaultMaxBackoff,
		"ceiling on the retry interval, so a recovering relay is found promptly")
	logLevel := flag.String("log-level", "info", "debug, info, warn or error")
	showVersion := flag.Bool("version", false, "print the version and exit")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"nivis-tunnel-agent %s — dial out and wait, so this host needs no inbound port.\n\n"+
				"The orchestrator key is a PUBLIC key. That is what lets a boot image\n"+
				"carry key material while carrying no secret, and why this configuration\n"+
				"is safe in a public repository and in the world-readable Nix store.\n\nOptions:\n",
			version)
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

	if *orchKey == "" {
		return fmt.Errorf("no orchestrator key: pass --orchestrator-key")
	}
	pub, err := proto.DecodePublicKey(*orchKey)
	if err != nil {
		return err
	}

	a, err := agent.New(agent.Config{
		RelayAddr:             *relayAddr,
		StreamID:              proto.StreamID(*streamID),
		OrchestratorPublicKey: pub,
		LocalAddr:             *localAddr,
		MinBackoff:            *minBackoff,
		MaxBackoff:            *maxBackoff,
		Logger:                log,
	})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("agent starting", "version", version, "relay", *relayAddr,
		"stream", *streamID, "local", *localAddr)

	if err := a.Run(ctx); err != nil {
		return err
	}

	log.Info("agent stopped")
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
