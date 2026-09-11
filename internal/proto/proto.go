// Package proto carries the nivis-tunnel wire protocol.
//
// This package is the reason relay, agent and tunnel live in one repository:
// all three speak it, and splitting them across repos would mean releasing
// three things in lockstep for every protocol change.
//
// # Shape
//
// A session has two phases. First a rendezvous [Frame] is sent to the relay,
// announcing a protocol version, a [StreamID] and a [Role]. The relay pairs one
// agent with one orchestrator per id and then copies bytes between them.
//
// Second, orchestrator and agent complete a Noise handshake through the relay,
// which carries it as opaque payload. Everything after that is encrypted and
// authenticated end to end, so a relay operator sees only ciphertext it cannot
// attribute. That is a design property, not a policy: it is what makes a hosted
// relay safe to run and honest to offer.
//
// Once the handshake completes the stream is bytes and ssh takes over. The
// protocol's job ends at exactly that point — it does not frame, multiplex or
// interpret the payload.
//
// # Why Noise_XK, with the agent initiating
//
// The requirement is asymmetric. The agent must talk to exactly one
// orchestrator; the target proves nothing about itself, and is identified by
// the address the cloud API returned — the trust model ssh-keyscan already
// relies on.
//
// Which party dials first is a relay-layer concern: both dial outward. So the
// Noise roles are free to be assigned by who must be sure of whom, and the
// agent — which holds only a public key — is the side that has to verify.
// Noise_XK puts it there:
//
//	K   the orchestrator's static key, known to the agent in advance because it
//	    is baked into the boot image. Handshake message 1 is encrypted to it, so
//	    a peer without the matching private key cannot read it, cannot reply,
//	    and is refused.
//	X   the agent's static key, transmitted during the handshake. This build
//	    does not verify it. It is carried so that pinning it later is a
//	    configuration change rather than a protocol change.
//
// Noise_KX with the orchestrator initiating was tried first and rejected. It
// authenticates just as strongly, but the responder gets no signal at handshake
// time: the agent would complete its half believing it had a session and
// discover otherwise only when transport failed to decrypt, holding a
// connection that can never carry a byte.
//
// Noise_KK was rejected too: it needs the agent's static key distributed in
// advance, and the agent does not exist when the boot image is built — the same
// chicken-and-egg this project exists to remove.
//
// One consequence is inherent rather than chosen: a peer that stays silent
// produces a timeout rather than a named refusal, because there is nothing to
// refuse. That is why the handshake carries a deadline.
package proto

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// Version is the protocol version this build speaks and announces.
//
// It exists from the first commit on purpose. The agent's half of this protocol
// ships inside the boot image, and the boot image is one of the few things a
// closure push cannot replace — so a protocol change that cannot be negotiated
// is a fleet-wide machine replacement. Making that expensive to need is cheaper
// than retrofitting it.
const Version uint16 = 1

// supportedVersions is every version this build can serve, lowest first.
var supportedVersions = []uint16{1}

// SupportsVersion reports whether this build can serve the given version.
func SupportsVersion(v uint16) bool {
	for _, s := range supportedVersions {
		if s == v {
			return true
		}
	}
	return false
}

// SupportedVersionsString renders the supported set for an error message. A
// version refusal names both what was offered and what is understood, so an
// operator is not left comparing two builds by hand.
func SupportedVersionsString() string {
	parts := make([]string, 0, len(supportedVersions))
	for _, v := range supportedVersions {
		parts = append(parts, strconv.Itoa(int(v)))
	}
	return strings.Join(parts, ",")
}

// MaxStreamIDLen bounds a stream id. It is small enough that a decoder can
// allocate for it without consulting the peer's intentions.
const MaxStreamIDLen = 63

// StreamID identifies a rendezvous. Both sides announce the same id and the
// relay splices them together.
//
// In practice this is the cloud's own instance id, because that is a value the
// orchestrator already holds in its state and the agent can read locally. It is
// an identifier and never a credential: anyone may claim one, and all authority
// comes from the handshake.
type StreamID string

// streamIDPattern matches cloud instance identifiers conservatively: the
// character set is what AWS, Hetzner and a hand-written test fixture all fit
// inside, and nothing here may reach a shell, a filesystem path or a log line.
var streamIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,62}$`)

// ErrInvalidStreamID reports a stream id that is empty, over-long, or carries
// characters outside the permitted set.
var ErrInvalidStreamID = errors.New("proto: invalid stream id")

// Validate reports whether the id is well formed.
//
// Callers must validate before the id is used for anything at all — including
// before it is logged. An id arrives from an unauthenticated peer, and routing
// on it is the relay's whole job.
func (s StreamID) Validate() error {
	if !streamIDPattern.MatchString(string(s)) {
		return ErrInvalidStreamID
	}
	return nil
}
