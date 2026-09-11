package proto

import (
	"errors"
	"strings"
	"testing"
)

func TestStreamIDValidate(t *testing.T) {
	cases := []struct {
		name string
		id   StreamID
		ok   bool
	}{
		{"aws instance id", "i-0abc123def456789", true},
		{"hetzner numeric id", "112233445", true},
		{"test fixture", "poc-target-01", true},
		{"dotted", "host.example", true},
		{"empty", "", false},
		{"leading dash", "-nope", false},
		{"path traversal", "../../etc/passwd", false},
		{"shell metacharacter", "id;rm -rf /", false},
		{"whitespace", "id with space", false},
		{"newline injection", "id\nSECOND", false},
		{"too long", StreamID(strings.Repeat("a", 64)), false},
		{"longest allowed", StreamID(strings.Repeat("a", 63)), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.id.Validate()
			if tc.ok && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if !tc.ok && !errors.Is(err, ErrInvalidStreamID) {
				t.Fatalf("Validate() = %v, want ErrInvalidStreamID", err)
			}
		})
	}
}

func TestVersionIsAnnouncedFromTheStart(t *testing.T) {
	// Guards the decision recorded in proto.go: a negotiated version exists
	// from the first commit so that a protocol change never forces a
	// fleet-wide image rebuild.
	if Version < 1 {
		t.Fatalf("Version = %d, want >= 1", Version)
	}
}
