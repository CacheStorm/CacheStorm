package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: URL.DECODE silently produced WRONG BYTES for an invalid
// percent-escape instead of rejecting it.
//
// hexToByte had no else branch, so a non-hex digit contributed nothing to the
// accumulator while still shifting it:
//
//	"%zz" -> 0x00  (a NUL byte)
//	"%4z" -> 0x40  ('@')
//
// and a truncated escape such as "ab%4" was passed through literally. The
// (string, error) result of urlDecode had always been dead code — cmdURLDECODE
// wrote that error back to the client, but the body could never produce one —
// so a malformed escape never surfaced as an error at all.
//
// The guard is strict: a '%' must be followed by exactly two hex digits, as in
// Redis. A bare trailing '%' is therefore an error too.

func urlDecodeRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) *resp.Value {
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

func urlDecodeRouter() *Router {
	router := NewRouter()
	RegisterEncodingCommands(router)
	return router
}

// TestURLDecodeRejectsInvalidPercentEscapes is the primary case: an escape
// whose digits are not hex must be an error, never a silently wrong byte.
func TestURLDecodeRejectsInvalidPercentEscapes(t *testing.T) {
	s := store.NewStore()
	router := urlDecodeRouter()

	bad := []string{
		"%zz", // both digits invalid -> was NUL
		"%4z", // low digit invalid -> was '@'
		"%z4", // high digit invalid
		"%g0",
		"%0g",
		"%gH",
		"ab%4", // truncated escape
		"%4",
		"%",    // bare percent
		"100%", // bare trailing percent in otherwise ordinary text
	}

	for _, in := range bad {
		v := urlDecodeRun(t, s, router, "URL.DECODE", in)
		if v.Type != resp.TypeError {
			t.Fatalf("FAIL: URL.DECODE(%q) returned type %v bytes %q, want an error —\n"+
				"hexToByte had no else branch, so an invalid digit contributed\n"+
				"nothing while still shifting the accumulator", in, v.Type, v.Bulk)
		}
	}
}

// TestURLDecodeRejectsInvalidEscapeInsideText is the neighbouring-input case: a
// bad escape embedded in otherwise valid text must be rejected rather than
// corrupting only part of the output.
func TestURLDecodeRejectsInvalidEscapeInsideText(t *testing.T) {
	s := store.NewStore()
	router := urlDecodeRouter()

	for _, in := range []string{
		"hello%zzworld",
		"a+b%zz",
		"%41%42%zz",
		"prefix%4zsuffix",
		"one%gHtwo",
	} {
		v := urlDecodeRun(t, s, router, "URL.DECODE", in)
		if v.Type != resp.TypeError {
			t.Fatalf("FAIL: URL.DECODE(%q) returned %q, want an error", in, v.Bulk)
		}
	}
}

// TestURLDecodeControls is the control: every valid escape must still decode
// exactly, '+' must still become a space, and plain text must pass through.
// A rejection above is therefore the fix and not an over-broad guard.
func TestURLDecodeControls(t *testing.T) {
	s := store.NewStore()
	router := urlDecodeRouter()

	cases := []struct {
		in  string
		out string
	}{
		{"%41", "A"},
		{"%2F", "/"},
		{"%2f", "/"},               // lowercase hex digits are valid
		{"%7e", "~"},               // unreserved character
		{"%20", " "},               // escaped space
		{"%FF", "\xff"},            // high byte value
		{"a+b", "a b"},             // form-encoded space
		{"plain", "plain"},         // no escapes at all
		{"%00", "\x00"},            // a VALID escape for NUL must still decode
		{"a-b_c.d~e", "a-b_c.d~e"}, // unreserved passthrough
		{"%41%42%43", "ABC"},       // consecutive escapes
	}

	for _, tc := range cases {
		v := urlDecodeRun(t, s, router, "URL.DECODE", tc.in)
		if v.Type == resp.TypeError {
			t.Fatalf("CONTROL FAIL: URL.DECODE(%q) was rejected, want %q", tc.in, tc.out)
		}
		if string(v.Bulk) != tc.out {
			t.Fatalf("CONTROL FAIL: URL.DECODE(%q) = %q, want %q", tc.in, string(v.Bulk), tc.out)
		}
	}

	// Round trip: everything URL.ENCODE emits must decode back unchanged.
	for _, orig := range []string{"hello world", "a/b?c=d&e", "100%\x00", "ünïcødé", "~-_.", ""} {
		enc := urlDecodeRun(t, s, router, "URL.ENCODE", orig)
		if enc.Type == resp.TypeError {
			t.Fatalf("CONTROL FAIL: URL.ENCODE(%q) was rejected", orig)
		}
		back := urlDecodeRun(t, s, router, "URL.DECODE", string(enc.Bulk))
		if back.Type == resp.TypeError {
			t.Fatalf("CONTROL FAIL: round trip of %q failed: URL.ENCODE gave %q, which URL.DECODE rejected",
				orig, string(enc.Bulk))
		}
		if string(back.Bulk) != orig {
			t.Fatalf("CONTROL FAIL: round trip of %q gave %q (via %q)",
				orig, string(back.Bulk), string(enc.Bulk))
		}
	}
}
