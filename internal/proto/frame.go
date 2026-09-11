package proto

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// Magic prefixes every rendezvous frame.
//
// It exists so that a connection to the wrong port fails immediately and says
// why, instead of hanging until a timeout while each side waits for the other
// to say something it understands.
var Magic = [4]byte{'N', 'V', 'T', 'L'}

// Role says which half of a rendezvous a party is. The relay pairs exactly one
// of each per stream id.
type Role uint8

const (
	// RoleAgent is the party on the target. It dials out and waits.
	RoleAgent Role = 1
	// RoleOrchestrator is the party that initiates work against the target.
	RoleOrchestrator Role = 2
)

func (r Role) String() string {
	switch r {
	case RoleAgent:
		return "agent"
	case RoleOrchestrator:
		return "orchestrator"
	default:
		return fmt.Sprintf("role(%d)", uint8(r))
	}
}

// Valid reports whether r is one of the two defined roles.
func (r Role) Valid() bool { return r == RoleAgent || r == RoleOrchestrator }

// Other returns the role a party of this role must be paired with.
func (r Role) Other() Role {
	if r == RoleAgent {
		return RoleOrchestrator
	}
	return RoleAgent
}

// Frame is the rendezvous announcement, sent to the relay before any payload.
//
// The encoding is fixed and positional rather than structured:
//
//	magic    4 bytes   "NVTL"
//	version  uint16    big-endian
//	role     uint8     1 = agent, 2 = orchestrator
//	idLen    uint8     1..MaxStreamIDLen
//	id       idLen bytes
//
// Version negotiation has to work when everything else about the peer is
// unknown — including whether it shares your idea of what a message looks like.
// Putting the version at a fixed offset means "your peer is too new" is a
// legible error rather than a parse failure at an arbitrary offset.
type Frame struct {
	Version  uint16
	Role     Role
	StreamID StreamID
}

// FrameHeaderLen is the fixed portion of an encoded frame: magic, version, role
// and the id length byte.
const FrameHeaderLen = len(Magic) + 2 + 1 + 1

// MaxFrameLen bounds a frame, so a decoder never allocates on a peer's say-so.
const MaxFrameLen = FrameHeaderLen + MaxStreamIDLen

var (
	// ErrWrongProtocol reports a stream that does not begin with Magic. The
	// usual cause is connecting to the wrong port, not a corrupted peer.
	ErrWrongProtocol = errors.New("proto: not a nivis-tunnel stream")

	// ErrUnsupportedVersion reports a peer announcing a protocol version this
	// build cannot serve.
	ErrUnsupportedVersion = errors.New("proto: unsupported protocol version")

	// ErrInvalidRole reports a role byte outside the defined set.
	ErrInvalidRole = errors.New("proto: invalid role")
)

// MarshalBinary encodes the frame. It refuses to encode a frame it would refuse
// to decode, so a bug cannot put a malformed id on the wire.
func (f Frame) MarshalBinary() ([]byte, error) {
	if err := f.StreamID.Validate(); err != nil {
		return nil, err
	}
	if !f.Role.Valid() {
		return nil, fmt.Errorf("%w: %d", ErrInvalidRole, uint8(f.Role))
	}

	buf := make([]byte, 0, FrameHeaderLen+len(f.StreamID))
	buf = append(buf, Magic[:]...)
	buf = binary.BigEndian.AppendUint16(buf, f.Version)
	buf = append(buf, uint8(f.Role), uint8(len(f.StreamID)))
	buf = append(buf, f.StreamID...)
	return buf, nil
}

// ReadFrame decodes a frame from r, checking each field before interpreting the
// next. The order is load-bearing: magic, then version, then everything else.
// A peer announcing a version this build cannot serve is reported as exactly
// that, rather than as whatever the later fields happen to decode to.
func ReadFrame(r io.Reader) (Frame, error) {
	head := make([]byte, FrameHeaderLen)
	if _, err := io.ReadFull(r, head); err != nil {
		return Frame{}, fmt.Errorf("proto: reading frame header: %w", err)
	}

	if [4]byte(head[0:4]) != Magic {
		return Frame{}, fmt.Errorf("%w: expected magic %q", ErrWrongProtocol, Magic)
	}

	version := binary.BigEndian.Uint16(head[4:6])
	if !SupportsVersion(version) {
		return Frame{}, fmt.Errorf("%w: peer offered %d, this build supports %s",
			ErrUnsupportedVersion, version, SupportedVersionsString())
	}

	role := Role(head[6])
	if !role.Valid() {
		return Frame{}, fmt.Errorf("%w: %d", ErrInvalidRole, head[6])
	}

	idLen := int(head[7])
	if idLen == 0 || idLen > MaxStreamIDLen {
		return Frame{}, fmt.Errorf("%w: length %d", ErrInvalidStreamID, idLen)
	}

	idBuf := make([]byte, idLen)
	if _, err := io.ReadFull(r, idBuf); err != nil {
		return Frame{}, fmt.Errorf("proto: reading stream id: %w", err)
	}

	// Validate before the id is retained anywhere — including before it could
	// reach a log line or a routing table.
	id := StreamID(idBuf)
	if err := id.Validate(); err != nil {
		return Frame{}, err
	}

	return Frame{Version: version, Role: role, StreamID: id}, nil
}

// ReadFrameWithDeadline decodes a frame under a deadline, so a connection that
// opens and then says nothing cannot hold a relay slot indefinitely.
//
// The deadline is cleared before returning: it governs the rendezvous, not the
// session that follows it.
func ReadFrameWithDeadline(conn net.Conn, timeout time.Duration) (Frame, error) {
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return Frame{}, fmt.Errorf("proto: setting read deadline: %w", err)
	}
	defer func() { _ = conn.SetReadDeadline(time.Time{}) }()

	return ReadFrame(conn)
}
