// Command nivis-tunnel-tunnel is a placeholder binary.
//
// It exists so that `nix flake check` is green from the first commit and the
// build gate is real before any behaviour is written. See .beans for the
// milestone that replaces it.
package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	fmt.Fprintf(os.Stderr, "nivis-tunnel-tunnel %s: not implemented yet\n", version)
	os.Exit(1)
}
