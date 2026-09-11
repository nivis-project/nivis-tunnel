package proto

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func mustMarshal(t *testing.T, f Frame) []byte {
	t.Helper()
	b, err := f.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary() = %v, want nil", err)
	}
	return b
}

func TestFrameRoundTrip(t *testing.T) {
	cases := []Frame{
		{Version: Version, Role: RoleAgent, StreamID: "i-0abc123def456789"},
		{Version: Version, Role: RoleOrchestrator, StreamID: "112233445"},
		{Version: Version, Role: RoleAgent, StreamID: "poc-target-01"},
		{Version: Version, Role: RoleOrchestrator, StreamID: StreamID(strings.Repeat("a", MaxStreamIDLen))},
	}

	for _, want := range cases {
		t.Run(string(want.StreamID), func(t *testing.T) {
			got, err := ReadFrame(bytes.NewReader(mustMarshal(t, want)))
			if err != nil {
				t.Fatalf("ReadFrame() = %v, want nil", err)
			}
			if got != want {
				t.Fatalf("round trip = %+v, want %+v", got, want)
			}
		})
	}
}

func TestFrameRejectsWrongProtocol(t *testing.T) {
	// The usual cause of this is connecting to the wrong port. It must fail
	// immediately and say so, rather than hanging until a timeout while each
	// side waits for the other to say something it understands.
	_, err := ReadFrame(strings.NewReader("GET / HTTP/1.1\r\nHost: example\r\n\r\n"))
	if !errors.Is(err, ErrWrongProtocol) {
		t.Fatalf("ReadFrame() = %v, want ErrWrongProtocol", err)
	}
	if !strings.Contains(err.Error(), "NVTL") {
		t.Fatalf("error %q does not name the expected magic", err)
	}
}

func TestFrameRejectsInvalidRole(t *testing.T) {
	buf := mustMarshal(t, Frame{Version: Version, Role: RoleAgent, StreamID: "poc-target-01"})
	buf[6] = 9 // a role outside the defined set

	_, err := ReadFrame(bytes.NewReader(buf))
	if !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("ReadFrame() = %v, want ErrInvalidRole", err)
	}
	if !strings.Contains(err.Error(), "9") {
		t.Fatalf("error %q does not name the offending value", err)
	}
}

// hostileIDs are ids that must never be accepted, and — just as importantly —
// never echoed back into an error, because errors are logged.
var hostileIDs = []string{
	"../../etc/passwd",
	"id;rm -rf /",
	"id\nSECOND",
	"-leading-dash",
	"id with space",
}

func TestFrameRejectsHostileStreamID(t *testing.T) {
	for _, id := range hostileIDs {
		t.Run(id, func(t *testing.T) {
			// Marshal refuses to produce it, so build the frame by hand: this
			// is what an attacker puts on the wire, not what our code writes.
			buf := make([]byte, 0, FrameHeaderLen+len(id))
			buf = append(buf, Magic[:]...)
			buf = binary.BigEndian.AppendUint16(buf, Version)
			buf = append(buf, uint8(RoleAgent), uint8(len(id)))
			buf = append(buf, id...)

			_, err := ReadFrame(bytes.NewReader(buf))
			if !errors.Is(err, ErrInvalidStreamID) {
				t.Fatalf("ReadFrame() = %v, want ErrInvalidStreamID", err)
			}
		})
	}
}

func TestRejectedStreamIDNeverReachesALogLine(t *testing.T) {
	// Errors get logged. An error that echoes a hostile id puts that id in the
	// log, which is exactly the injection the validation exists to prevent — so
	// the refusal must not quote what it refused.
	for _, id := range hostileIDs {
		if err := StreamID(id).Validate(); err == nil {
			t.Fatalf("Validate(%q) = nil, want an error", id)
		} else if strings.Contains(err.Error(), strings.TrimSpace(id)) {
			t.Fatalf("error %q echoes the rejected id %q", err, id)
		}
	}
}

func TestFrameRejectsOverLongStreamID(t *testing.T) {
	buf := mustMarshal(t, Frame{Version: Version, Role: RoleAgent, StreamID: "ok"})
	buf[7] = MaxStreamIDLen + 1

	_, err := ReadFrame(bytes.NewReader(buf))
	if !errors.Is(err, ErrInvalidStreamID) {
		t.Fatalf("ReadFrame() = %v, want ErrInvalidStreamID", err)
	}
}

func TestMarshalRefusesWhatDecodeWouldRefuse(t *testing.T) {
	// A bug must not be able to put a malformed id on the wire.
	if _, err := (Frame{Version: Version, Role: RoleAgent, StreamID: "../x"}).MarshalBinary(); !errors.Is(err, ErrInvalidStreamID) {
		t.Fatalf("MarshalBinary() = %v, want ErrInvalidStreamID", err)
	}
	if _, err := (Frame{Version: Version, Role: 42, StreamID: "ok"}).MarshalBinary(); !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("MarshalBinary() = %v, want ErrInvalidRole", err)
	}
}

func TestUnsupportedVersionIsRefusedLegibly(t *testing.T) {
	// A peer from the future must be told so, rather than failing later as a
	// malformed handshake at an arbitrary offset.
	buf := mustMarshal(t, Frame{Version: Version, Role: RoleAgent, StreamID: "poc-target-01"})
	binary.BigEndian.PutUint16(buf[4:6], Version+1)

	_, err := ReadFrame(bytes.NewReader(buf))
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("ReadFrame() = %v, want ErrUnsupportedVersion", err)
	}

	msg := err.Error()
	if !strings.Contains(msg, "2") {
		t.Fatalf("error %q does not name the offered version", msg)
	}
	if !strings.Contains(msg, SupportedVersionsString()) {
		t.Fatalf("error %q does not name the supported range %q", msg, SupportedVersionsString())
	}
}

func TestVersionIsCheckedBeforeAnythingElse(t *testing.T) {
	// The order is load-bearing. A frame that is both too new AND otherwise
	// malformed must report the version, because that is the actionable fact.
	buf := mustMarshal(t, Frame{Version: Version, Role: RoleAgent, StreamID: "poc-target-01"})
	binary.BigEndian.PutUint16(buf[4:6], 9999)
	buf[6] = 9 // also an invalid role

	_, err := ReadFrame(bytes.NewReader(buf))
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("ReadFrame() = %v, want ErrUnsupportedVersion to win", err)
	}
}

func TestEncodedFrameCarriesANonZeroVersion(t *testing.T) {
	// Guards the decision recorded in proto.go: the agent's half of this
	// protocol ships inside the boot image, which a closure push cannot
	// replace. A protocol change that cannot be negotiated is a fleet-wide
	// machine replacement, so the version must be on the wire from the first
	// release — not added once it is first needed.
	buf := mustMarshal(t, Frame{Version: Version, Role: RoleAgent, StreamID: "poc-target-01"})

	if got := binary.BigEndian.Uint16(buf[4:6]); got == 0 {
		t.Fatal("encoded frame announces version 0")
	}
	if !SupportsVersion(Version) {
		t.Fatalf("SupportsVersion(%d) = false, want true", Version)
	}
}

func TestReadFrameWithDeadlineDoesNotWaitForever(t *testing.T) {
	// A connection that opens and then says nothing must not hold a relay slot.
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	done := make(chan error, 1)
	go func() {
		_, err := ReadFrameWithDeadline(server, 50*time.Millisecond)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("ReadFrameWithDeadline() = nil, want a timeout error")
		}
		if !errors.Is(err, io.ErrUnexpectedEOF) && !isTimeout(err) {
			t.Fatalf("ReadFrameWithDeadline() = %v, want a timeout", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ReadFrameWithDeadline() did not return; the deadline was not enforced")
	}
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func TestRoleOther(t *testing.T) {
	if got := RoleAgent.Other(); got != RoleOrchestrator {
		t.Fatalf("RoleAgent.Other() = %v, want orchestrator", got)
	}
	if got := RoleOrchestrator.Other(); got != RoleAgent {
		t.Fatalf("RoleOrchestrator.Other() = %v, want agent", got)
	}
}
