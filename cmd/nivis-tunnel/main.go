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
	os.Exit(execute(os.Args[1:]))
}

// execute runs a subcommand and reports its failure, returning the exit status.
//
// Separated from main so that tests exercise the real reporting path. What
// reaches stdout and stderr IS the contract here, and a test that called run
// directly would be checking a path no operator ever takes.
func execute(args []string) int {
	if err := run(args); err != nil {
		// stderr, always: a byte of this on stdout would corrupt ssh's protocol
		// and produce a failure that looks like anything but its cause.
		fmt.Fprintf(os.Stderr, "nivis-tunnel: %v\n", err)
		return 1
	}
	return 0
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

	// Go's flag package stops parsing at the first non-flag argument, so
	// `connect <id> --relay ... --key ...` would silently swallow the flags as
	// positionals. That is exactly the ProxyCommand line this tool documents,
	// so flags have to work on either side of the stream id.
	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(positional) != 1 {
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

	stream, _, err := tunnel.Connect(*relayAddr, proto.StreamID(positional[0]), key, *timeout)
	if err != nil {
		return err
	}
	defer stream.Close()

	return tunnel.Splice(stream, os.Stdin, os.Stdout)
}

// parseInterspersed parses flags that appear before, after or around positional
// arguments, and returns the positionals.
//
// Go's flag package stops at the first non-flag argument by design. That is
// wrong for this command: the natural ProxyCommand line puts the stream id
// first and the connection options after it, and a silently ignored --relay is
// a failure an operator would have no way to read.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	rest := args

	for {
		if err := fs.Parse(rest); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, fs.Arg(0))
		rest = fs.Args()[1:]
	}
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
