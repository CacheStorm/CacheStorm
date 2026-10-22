package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: UUID.VERSION decoded the version nibble with `uuid[14] - '0'`.
// uuid[14] is the FIRST hex digit after the second '-', so it is one hex
// nibble: '0'-'9' happen to decode correctly by coincidence, but 'a'-'f' gave
// 49..54 and 'A'-'F' gave 17..22 instead of 10..15.
//
//	internal/command/encoding_commands.go (before)
//	  version := uuid[14]
//	  return ctx.WriteInteger(int64(version - '0'))
//
// isValidUUID accepts any hex digit at non-separator positions, so a hex-letter
// version is valid input and must decode to its numeric value.

func uuidVersionRun(t *testing.T, s *store.Store, router *Router, uuid string) string {
	t.Helper()
	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext("UUID.VERSION", [][]byte{[]byte(uuid)}, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("UUID.VERSION %q: Execute returned error: %v", uuid, err)
	}
	return buf.String()
}

func uuidVersionInt(t *testing.T, s *store.Store, router *Router, uuid string) int64 {
	t.Helper()
	out := uuidVersionRun(t, s, router, uuid)
	v, err := resp.NewReader(bytes.NewReader([]byte(out))).ReadValue()
	if err != nil {
		t.Fatalf("unparsable UUID.VERSION reply %q: %v", out, err)
	}
	if v.Type != resp.TypeInteger {
		t.Fatalf("UUID.VERSION %q replied %q, want an integer", uuid, out)
	}
	return v.Int
}

func uuidVersionRouter() *Router {
	router := NewRouter()
	RegisterEncodingCommands(router)
	return router
}

// TestUUIDVersionDecodesHexLetterNibble is the defect: a hex-letter version
// nibble must decode to 10..15, not to its ASCII offset.
func TestUUIDVersionDecodesHexLetterNibble(t *testing.T) {
	s := store.NewStore()
	router := uuidVersionRouter()

	cases := []struct {
		uuid string
		want int64
	}{
		{"123e4567-e89b-a2b3-a456-426614174000", 10},
		{"123e4567-e89b-b2b3-a456-426614174000", 11},
		{"123e4567-e89b-f2b3-a456-426614174000", 15},
		{"123e4567-e89b-A2B3-a456-426614174000", 10},
		{"123e4567-e89b-F2B3-a456-426614174000", 15},
	}
	for _, c := range cases {
		if got := uuidVersionInt(t, s, router, c.uuid); got != c.want {
			t.Fatalf("UUID.VERSION %q = %d, want %d; uuid[14] is a hex nibble, "+
				"not an ASCII decimal digit", c.uuid, got, c.want)
		}
	}
}

// TestUUIDVersionControls covers the paths that already worked and must keep
// working: decimal nibbles, the nil UUID, UUID.VALIDATE agreement, and
// malformed-input rejection.
func TestUUIDVersionControls(t *testing.T) {
	s := store.NewStore()
	router := uuidVersionRouter()

	for _, c := range []struct {
		uuid string
		want int64
	}{
		{"123e4567-e89b-12d3-a456-426614174000", 1},
		{"123e4567-e89b-42d3-a456-426614174000", 4},
		{"123e4567-e89b-52d3-a456-426614174000", 5},
		{"123e4567-e89b-02d3-a456-426614174000", 0},
		{"00000000-0000-0000-0000-000000000000", 0},
	} {
		if got := uuidVersionInt(t, s, router, c.uuid); got != c.want {
			t.Fatalf("CONTROL FAIL: UUID.VERSION %q = %d, want %d", c.uuid, got, c.want)
		}
	}

	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext("UUID.VALIDATE",
		[][]byte{[]byte("123e4567-e89b-a2b3-a456-426614174000")}, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("UUID.VALIDATE: %v", err)
	}
	if buf.String() != ":1\r\n" {
		t.Fatalf("CONTROL FAIL: UUID.VALIDATE with hex-letter version = %q, want :1", buf.String())
	}

	for _, bad := range []string{
		"123e4567e89b12d3a456426614174000",
		"123e4567-e89b-12d3-a456-42661417400",
		"123e4567-e89b-12d3-a456-4266141740000",
		"123e4567-e89b-1g2b-a456-426614174000",
	} {
		if out := uuidVersionRun(t, s, router, bad); !strings.HasPrefix(out, "-") {
			t.Fatalf("CONTROL FAIL: UUID.VERSION %q = %q, want an error reply", bad, out)
		}
	}
}
