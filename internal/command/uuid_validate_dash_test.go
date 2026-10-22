package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: isValidUUID accepted a '-' at any HEX position, so UUID.VALIDATE
// reported malformed strings as valid.
//
// The hex branch excused '-' explicitly:
//
//	if c != '-' && (c < '0' || c > '9') && ... { return false }
//
// so a character had to be outside every hex range AND not a dash to be
// rejected. The four separator positions are enforced by the branch above, so
// no '-' may legitimately appear elsewhere: "-2345678-e89b-12d3-a456-426614174000"
// validated as a UUID.
//
// Impact is not cosmetic. UUID.VERSION reads uuid[14] and returns
// `uuid[14] - '0'`, so a dash there produced a nonsense version (253) instead
// of an error, and anything trusting UUID.VALIDATE accepted malformed input.

func uuidValidRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) *resp.Value {
	t.Helper()
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("%s: Execute returned error: %v", cmd, err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", cmd, buf.String(), err)
	}
	return v
}

func uuidValidRouter() *Router {
	router := NewRouter()
	RegisterEncodingCommands(router)
	return router
}

// TestUUIDValidateRejectsDashAtHexPositions is the primary case: a '-' in any
// hex position must make the string invalid.
func TestUUIDValidateRejectsDashAtHexPositions(t *testing.T) {
	s := store.NewStore()
	router := uuidValidRouter()

	cases := []struct {
		name string
		uuid string
	}{
		{"dash at hex position 0", "-2345678-e89b-12d3-a456-426614174000"},
		{"dash at hex position 7", "123e456--e89b-12d3-a456-426614174000"},
		{"dash at hex position 35", "123e4567-e89b-12d3-a456-42661417400-"},
		{"dash inside the last group", "123e4567-e89b-12d3-a456-42661-174000"},
	}

	for _, tc := range cases {
		if len(tc.uuid) != 36 {
			t.Fatalf("FAIL: fixture %q is %d chars, want 36", tc.uuid, len(tc.uuid))
		}
		v := uuidValidRun(t, s, router, "UUID.VALIDATE", tc.uuid)
		if v.Type != resp.TypeInteger {
			t.Fatalf("FAIL: %s: UUID.VALIDATE returned type %v, want an integer", tc.name, v.Type)
		}
		if v.Int != 0 {
			t.Fatalf("FAIL: %s: UUID.VALIDATE(%q) = %d, want 0 — a dash was\n"+
				"accepted as a hex digit", tc.name, tc.uuid, v.Int)
		}
	}
}

// TestUUIDVersionRejectsDashAtHexPositions covers the second command that
// depends on the same predicate: UUID.VERSION reads uuid[14] and subtracts '0',
// so a dash there must be an error rather than a nonsense version number.
func TestUUIDVersionRejectsDashAtHexPositions(t *testing.T) {
	s := store.NewStore()
	router := uuidValidRouter()

	// Position 14 is the first character of the version nibble.
	uuid := "123e4567-e89b--2d3-a456-426614174000"
	if len(uuid) != 36 {
		t.Fatalf("FAIL: fixture is %d chars, want 36", len(uuid))
	}

	v := uuidValidRun(t, s, router, "UUID.VERSION", uuid)
	if v.Type != resp.TypeError {
		t.Fatalf("FAIL: UUID.VERSION(%q) = %d, want an error — the malformed UUID\n"+
			"passed validation, so uuid[14] is a '-' and the nibble arithmetic\n"+
			"produced %d instead of failing", uuid, v.Int, v.Int)
	}
	if v.Int < 0 {
		t.Fatalf("FAIL: UUID.VERSION produced a negative version %d from a malformed UUID", v.Int)
	}
}

// TestUUIDValidateStillAcceptsCanonicalUUIDs is the control: canonical UUIDs
// are accepted and report the correct version, so the new guard is not
// over-broad.
//
// The version nibble is decimal-only here because UUID.VERSION computes
// `uuid[14] - '0'`; a hex-letter nibble is a separate pre-existing deviation
// and is deliberately not asserted.
func TestUUIDValidateStillAcceptsCanonicalUUIDs(t *testing.T) {
	s := store.NewStore()
	router := uuidValidRouter()

	valid := []struct {
		uuid string
		ver  int64
	}{
		{"123e4567-e89b-12d3-a456-426614174000", 1},
		{"123e4567-e89b-42d3-a456-426614174000", 4},
		{"123e4567-e89b-52d3-a456-426614174000", 5},
		{"00000000-0000-0000-0000-000000000000", 0},
		{"ffffffff-ffff-4fff-bfff-ffffffffffff", 4}, // hex elsewhere, decimal version
		{"ABCDEF01-2345-6789-ABCD-EF0123456789", 6},
	}
	for _, tc := range valid {
		if v := uuidValidRun(t, s, router, "UUID.VALIDATE", tc.uuid); v.Type != resp.TypeInteger || v.Int != 1 {
			t.Fatalf("CONTROL FAIL: UUID.VALIDATE(%q) = type %v %d, want :1", tc.uuid, v.Type, v.Int)
		}
		if v := uuidValidRun(t, s, router, "UUID.VERSION", tc.uuid); v.Type == resp.TypeError || v.Int != tc.ver {
			t.Fatalf("CONTROL FAIL: UUID.VERSION(%q) = type %v %d, want %d", tc.uuid, v.Type, v.Int, tc.ver)
		}
	}

	// Genuinely invalid strings must still be rejected.
	invalid := []struct {
		name string
		uuid string
	}{
		{"too short", "123e4567-e89b-12d3-a456-42661417400"},
		{"too long", "123e4567-e89b-12d3-a456-4266141740000"},
		{"non-hex letter g", "123e4567-e89b-12d3-a456-42661417400g"},
		{"non-hex letter z", "z23e4567-e89b-12d3-a456-426614174000"},
		{"space inside", "123e4567-e89b-12d3-a456-4266141740 0"},
		{"missing a separator", "123e4567xe89b-12d3-a456-426614174000"},
		{"separator in wrong place", "123e456-78e89b-12d3-a456-42661417400"},
		{"empty", ""},
	}
	for _, tc := range invalid {
		if v := uuidValidRun(t, s, router, "UUID.VALIDATE", tc.uuid); v.Type != resp.TypeInteger || v.Int != 0 {
			t.Fatalf("CONTROL FAIL: %s: UUID.VALIDATE(%q) = type %v %d, want :0",
				tc.name, tc.uuid, v.Type, v.Int)
		}
	}
}
