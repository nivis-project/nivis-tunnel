// Package proto carries the nivis-tunnel wire protocol.
//
// This package is the reason relay, agent and tunnel live in one repository:
// all three speak it, and splitting them across repos would mean releasing
// three things in lockstep for every protocol change.
//
// The protocol is deliberately small. The relay matches two connections by
// stream id and splices bytes between them; it performs no cryptography and
// authenticates nobody. Confidentiality and authenticity come from a Noise
// handshake that runs end to end between orchestrator and agent, so a relay
// operator sees only ciphertext it cannot attribute.
package proto

import (
	"errors"
	"regexp"
)

// Version is the protocol version announced during rendezvous.
//
// It exists from the first commit on purpose: without a negotiated version,
// every protocol change is a breaking change, and every breaking change means
// rebuilding the boot image of every machine in the fleet. Making that
// expensive to need is cheaper than retrofitting it later.
const Version = 1

// StreamID identifies a rendezvous. Both sides announce the same id and the
// relay splices them together.
//
// In the PoC this is the cloud's own instance id, because that is a value the
// orchestrator already holds in its state and the agent can read locally. It
// is an identifier, never a credential: anyone may claim one, and the trust
// that matters comes from the Noise handshake.
type StreamID string

// streamIDPattern matches cloud instance identifiers conservatively: the
// character set is what AWS, Hetzner and a hand-written test fixture all fit
// inside, and nothing here may reach a shell or a filesystem path.
var streamIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,62}$`)

// ErrInvalidStreamID reports a stream id that is empty, over-long, or carries
// characters outside the permitted set.
var ErrInvalidStreamID = errors.New("proto: invalid stream id")

// Validate reports whether the id is well formed. The relay must reject a
// malformed id before it reaches any map key or log line.
func (s StreamID) Validate() error {
	if !streamIDPattern.MatchString(string(s)) {
		return ErrInvalidStreamID
	}
	return nil
}
