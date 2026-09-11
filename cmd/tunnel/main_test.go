package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nivis-project/nivis-tunnel/internal/proto"
)

// captureStdio replaces os.Stdout and os.Stderr with pipes and returns a
// function that restores them and yields what each received.
//
// stdout is the ssh protocol when this command runs as a ProxyCommand, so
// testing what lands there is testing the contract, not an implementation
// detail.
func captureStdio(t *testing.T) func() (stdout, stderr string) {
	t.Helper()

	origOut, origErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = outW, errW

	return func() (string, string) {
		_ = outW.Close()
		_ = errW.Close()
		os.Stdout, os.Stderr = origOut, origErr

		var outBuf, errBuf bytes.Buffer
		_, _ = io.Copy(&outBuf, outR)
		_, _ = io.Copy(&errBuf, errR)
		return outBuf.String(), errBuf.String()
	}
}

func TestKeygenWritesAnOwnerOnlyKeyAndPrintsThePublicHalf(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orchestrator.key")

	restore := captureStdio(t)
	err := run([]string{"keygen", "--out", path})
	stdout, stderr := restore()

	if err != nil {
		t.Fatalf("run(keygen) = %v, want nil", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("key file mode is %04o, want 0600", perm)
	}

	// The public half goes to stdout so it can be piped straight into a config;
	// the human-facing note about where the private half landed goes to stderr.
	pub, err := proto.DecodePublicKey(strings.TrimSpace(stdout))
	if err != nil {
		t.Fatalf("stdout did not carry a usable public key: %v (stdout=%q)", err, stdout)
	}
	if len(pub) == 0 {
		t.Fatal("decoded public key is empty")
	}
	if !strings.Contains(stderr, path) {
		t.Fatalf("stderr does not say where the private key was written: %q", stderr)
	}
}

func TestKeygenRefusesToOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orchestrator.key")

	restore := captureStdio(t)
	firstErr := run([]string{"keygen", "--out", path})
	restore()
	if firstErr != nil {
		t.Fatalf("first keygen = %v", firstErr)
	}

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	restore = captureStdio(t)
	secondErr := run([]string{"keygen", "--out", path})
	restore()

	if secondErr == nil {
		t.Fatal("run(keygen) = nil over an existing key, want a refusal")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("the existing key was modified by the refused generation")
	}
}

func TestDiagnosticsNeverReachStandardOutput(t *testing.T) {
	// One stray byte on stdout corrupts ssh's protocol and produces a failure
	// that looks like anything but its cause. Every failing path must keep
	// stdout clean.
	cases := []struct {
		name string
		args []string
	}{
		{"no subcommand", nil},
		{"unknown subcommand", []string{"wat"}},
		{"connect without a stream id", []string{"connect", "--relay", "127.0.0.1:1", "--key", "/nonexistent"}},
		{"connect without a relay", []string{"connect", "poc-target-01", "--key", "/nonexistent"}},
		{"connect with an unreadable key", []string{"connect", "poc-target-01", "--relay", "127.0.0.1:1", "--key", "/nonexistent"}},
		{"help", []string{"--help"}},
		{"version", []string{"--version"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// execute rather than run: what main() actually does, including
			// reporting the returned error. Testing run() alone would check a
			// path no operator ever takes.
			restore := captureStdio(t)
			_ = execute(tc.args)
			stdout, stderr := restore()

			if stdout != "" {
				t.Fatalf("stdout carried %q; it must carry only stream bytes", stdout)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Fatal("nothing was written to stderr; the failure would be undiagnosable")
			}
		})
	}
}

func TestFlagsAreAcceptedOnEitherSideOfTheStreamID(t *testing.T) {
	// Regression. Go's flag package stops parsing at the first non-flag
	// argument, so `connect <id> --relay ... --key ...` used to swallow both
	// flags as positionals — and that is the exact ProxyCommand line this tool
	// documents. It failed with "connect takes exactly one stream id" while
	// looking, to an operator, like a correct invocation.
	//
	// Both orders must reach the same point: a real attempt that fails on the
	// unreadable key, not on argument parsing.
	orders := [][]string{
		{"connect", "poc-target-01", "--relay", "127.0.0.1:1", "--key", "/nonexistent"},
		{"connect", "--relay", "127.0.0.1:1", "--key", "/nonexistent", "poc-target-01"},
		{"connect", "--relay", "127.0.0.1:1", "poc-target-01", "--key", "/nonexistent"},
	}

	for _, args := range orders {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			restore := captureStdio(t)
			err := run(args)
			_, stderr := restore()

			if err == nil {
				t.Fatal("run() = nil, want a failure on the unreadable key")
			}
			if strings.Contains(err.Error(), "exactly one stream id") {
				t.Fatalf("the flags were swallowed as positionals: %v", err)
			}
			if !strings.Contains(err.Error(), "reading key file") {
				t.Fatalf("run() = %v, want it to have got as far as reading the key", err)
			}
			if strings.Contains(stderr, "Options:") {
				t.Fatal("usage was printed; the arguments were misparsed")
			}
		})
	}
}
