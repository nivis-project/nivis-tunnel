// Command nivis-tunnel is the orchestrator-side client.
//
// Its main use is as an ssh ProxyCommand:
//
//	ssh -o ProxyCommand='nivis-tunnel connect %h' root@i-0abc123def456789
//
// ssh hands the command stdin and stdout and treats whatever appears there as
// the network, so nix-copy-closure, switch-to-configuration, deploy-rs and
// plain interactive ssh all work over this transport with no modification. That
// one line is the entire integration surface of the project.
//
// Because stdout IS the protocol, nothing but stream bytes may ever be written
// there. Every diagnostic goes to stderr.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/nivis-project/nivis-tunnel/internal/proto"
	"github.com/nivis-project/nivis-tunnel/internal/tunnel"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		// stderr, always: a byte of this on stdout would corrupt ssh's protocol
		// and produce a failure that looks like anything but its cause.
		fmt.Fprintf(os.Stderr, "nivis-tunnel: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `nivis-tunnel %s — reach a machine that cannot be dialled.

Usage:
  nivis-tunnel connect <stream-id> [options]
  nivis-tunnel keygen [options]

As an ssh ProxyCommand:
  ssh -o ProxyCommand='nivis-tunnel connect %%h --relay relay.example:7843 --key ~/.config/nivis-tunnel/orchestrator.key' \
      -o StrictHostKeyChecking=accept-new root@i-0abc123def456789

  accept-new rather than no: this tunnel does not authenticate the target, so
  ssh's own host key check is doing real work here. It pins the key on first
  contact and refuses an impostor on every connection after it.

Run a subcommand with --help for its options.
`, version)
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("no subcommand given")
	}

	switch args[0] {
	case "connect":
		return runConnect(args[1:])
	case "keygen":
		return runKeygen(args[1:])
	case "-h", "--help", "help":
		usage()
		return nil
	case "--version", "-version", "version":
		fmt.Fprintln(os.Stderr, version)
		return nil
	default:
		usage()
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func runConnect(args []string) error {
	fs := flag.NewFlagSet("connect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	relayAddr := fs.String("relay", envOr("NIVIS_TUNNEL_RELAY", ""),
		"relay address, host:port (or NIVIS_TUNNEL_RELAY)")
	keyPath := fs.String("key", envOr("NIVIS_TUNNEL_KEY", ""),
		"path to the orchestrator private key (or NIVIS_TUNNEL_KEY)")
	timeout := fs.Duration("timeout", tunnel.DefaultTimeout,
		"how long to wait for the relay, the agent and the handshake")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"nivis-tunnel connect <stream-id> — splice a stream to stdin and stdout.\n\n"+
				"The stream id is the cloud's own instance id, which the orchestrator\n"+
				"already holds in its state and the agent announces from the target.\n\nOptions:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("connect takes exactly one stream id")
	}
	if *relayAddr == "" {
		return errors.New("no relay address: pass --relay or set NIVIS_TUNNEL_RELAY")
	}
	if *keyPath == "" {
		return errors.New("no orchestrator key: pass --key or set NIVIS_TUNNEL_KEY")
	}

	key, err := tunnel.ReadPrivateKey(*keyPath)
	if err != nil {
		return err
	}

	stream, _, err := tunnel.Connect(*relayAddr, proto.StreamID(fs.Arg(0)), key, *timeout)
	if err != nil {
		return err
	}
	defer stream.Close()

	return tunnel.Splice(stream, os.Stdin, os.Stdout)
}

func runKeygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	out := fs.String("out", "orchestrator.key", "path to write the private key to")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"nivis-tunnel keygen — generate an orchestrator keypair.\n\n"+
				"The private half is written to --out, readable only by you. The public\n"+
				"half is printed to stdout: put it in the agent's NixOS configuration as\n"+
				"services.nivis-tunnel-agent.orchestratorPublicKey. It is a public key, so\n"+
				"it is safe in a public repository and in the Nix store — which is what\n"+
				"lets a boot image carry key material while carrying no secret.\n\nOptions:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	key, err := proto.GenerateKeypair()
	if err != nil {
		return err
	}
	if err := tunnel.WritePrivateKey(*out, key); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "private key written to %s\n", *out)
	fmt.Println(proto.EncodePublicKey(key.Public))
	return nil
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
