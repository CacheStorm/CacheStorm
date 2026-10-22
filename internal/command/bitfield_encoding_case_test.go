package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: BITFIELD accepted its type-encoding argument
// case-insensitively but interpreted it case-SENSITIVELY.
//
//	parseEncoding()  : strings.ToLower(encoding)      -> "I8" accepted, 8 bits
//	bitfieldGet()    : strings.HasPrefix(encoding, "i") -> "I8" is NOT "i..."
//	bitfieldIncr()   : strings.HasPrefix(encoding, "i") -> "I8" is NOT "i..."
//
// Redis BITFIELD encodings are case-insensitive (i8/I8/u8/U8 are equivalent).
// Here the same stored bits returned different values depending only on the
// case of the encoding argument:
//
//	BITFIELD k SET i8 0 -1 ; BITFIELD k GET i8 0 -> -1   (signed, correct)
//	BITFIELD k SET I8 0 -1 ; BITFIELD k GET I8 0 -> 255  (read as unsigned)
//
// Impact: silent data corruption, and a signed saturation floor that depends
// on the spelling of the encoding.

func bitfieldCaseRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
	t.Helper()
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("%s: Execute returned error: %v", cmd, err)
	}
	return buf.String()
}

// bitfieldCaseGet runs BITFIELD <key> GET <enc> <off> and returns the integer.
func bitfieldCaseGet(t *testing.T, s *store.Store, router *Router, key, enc string, off int) int64 {
	t.Helper()
	out := bitfieldCaseRun(t, s, router, "BITFIELD", key, "GET", enc, fmtInt(off))
	v, err := resp.NewReader(bytes.NewReader([]byte(out))).ReadValue()
	if err != nil {
		t.Fatalf("unparsable BITFIELD GET %s reply %q: %v", enc, out, err)
	}
	if v.Type != resp.TypeArray || len(v.Array) != 1 || v.Array[0].Type != resp.TypeInteger {
		t.Fatalf("BITFIELD GET %s returned %q, want a 1-element integer array", enc, out)
	}
	return v.Array[0].Int
}

func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func bitfieldCaseRouter() *Router {
	router := NewRouter()
	RegisterBitmapCommands(router)
	return router
}

// TestBitfieldEncodingIsCaseInsensitive is the decode symptom: identical stored
// bits must decode identically regardless of the encoding's case.
func TestBitfieldEncodingIsCaseInsensitive(t *testing.T) {
	s := store.NewStore()
	router := bitfieldCaseRouter()

	if out := bitfieldCaseRun(t, s, router, "BITFIELD", "low", "SET", "i8", "0", "-1"); out != "*1\r\n:0\r\n" {
		t.Fatalf("SET i8 0 -1 returned %q, want old value 0", out)
	}
	if out := bitfieldCaseRun(t, s, router, "BITFIELD", "up", "SET", "I8", "0", "-1"); out != "*1\r\n:0\r\n" {
		t.Fatalf("SET I8 0 -1 returned %q, want old value 0", out)
	}

	lower := bitfieldCaseGet(t, s, router, "low", "i8", 0)
	upper := bitfieldCaseGet(t, s, router, "up", "I8", 0)

	if lower != -1 {
		t.Fatalf("CONTROL FAIL: GET i8 after SET i8 -1 = %d, want -1", lower)
	}
	if upper != lower {
		t.Fatalf("BITFIELD GET I8 = %d but GET i8 = %d for the same stored bits; "+
			"the type encoding must be case-insensitive", upper, lower)
	}
}

// TestBitfieldEncodingCaseAffectsOverflow covers SAT and WRAP, whose signed
// bounds must not depend on the case of the encoding.
func TestBitfieldEncodingCaseAffectsOverflow(t *testing.T) {
	s := store.NewStore()
	router := bitfieldCaseRouter()

	bitfieldCaseRun(t, s, router, "BITFIELD", "low", "SET", "i8", "0", "100")
	bitfieldCaseRun(t, s, router, "BITFIELD", "up", "SET", "I8", "0", "100")

	loOut := bitfieldCaseRun(t, s, router, "BITFIELD", "low", "OVERFLOW", "SAT", "INCRBY", "i8", "0", "-1000")
	upOut := bitfieldCaseRun(t, s, router, "BITFIELD", "up", "OVERFLOW", "SAT", "INCRBY", "I8", "0", "-1000")

	if !strings.Contains(loOut, ":-128\r\n") {
		t.Fatalf("CONTROL FAIL: INCRBY i8 SAT -1000 from 100 = %q, want -128", loOut)
	}
	if !strings.Contains(upOut, ":-128\r\n") {
		t.Fatalf("INCRBY I8 SAT -1000 from 100 = %q, want -128; the signed "+
			"saturation floor must not depend on the encoding's case", upOut)
	}

	// WRAP past the signed maximum must land on the negative twin.
	bitfieldCaseRun(t, s, router, "BITFIELD", "upw", "SET", "I8", "0", "127")
	bitfieldCaseRun(t, s, router, "BITFIELD", "upw", "OVERFLOW", "WRAP", "INCRBY", "I8", "0", "1")
	if v := bitfieldCaseGet(t, s, router, "upw", "I8", 0); v != -128 {
		t.Fatalf("WRAP INCRBY I8 past 127 = %d, want -128; a signed encoding "+
			"must wrap into the negative range", v)
	}
}

// TestBitfieldEncodingCaseControls covers only paths that do NOT depend on the
// signed/unsigned branch, so they pass both before and after the fix. Any
// assertion using an uppercase "i" encoding belongs in a defect test, not here.
func TestBitfieldEncodingCaseControls(t *testing.T) {
	s := store.NewStore()
	router := bitfieldCaseRouter()

	// Unsigned stays unsigned in every spelling.
	bitfieldCaseRun(t, s, router, "BITFIELD", "ul", "SET", "u8", "0", "-1")
	bitfieldCaseRun(t, s, router, "BITFIELD", "uu", "SET", "U8", "0", "-1")
	if v := bitfieldCaseGet(t, s, router, "ul", "u8", 0); v != 255 {
		t.Fatalf("CONTROL FAIL: GET u8 after SET u8 -1 = %d, want 255", v)
	}
	if v := bitfieldCaseGet(t, s, router, "uu", "U8", 0); v != 255 {
		t.Fatalf("CONTROL FAIL: GET U8 after SET U8 -1 = %d, want 255", v)
	}

	// Lowercase signed spellings keep working.
	bitfieldCaseRun(t, s, router, "BITFIELD", "w", "SET", "i8", "0", "127")
	bitfieldCaseRun(t, s, router, "BITFIELD", "w", "OVERFLOW", "WRAP", "INCRBY", "i8", "0", "1")
	if v := bitfieldCaseGet(t, s, router, "w", "i8", 0); v != -128 {
		t.Fatalf("CONTROL FAIL: WRAP INCRBY i8 past 127 = %d, want -128", v)
	}

	bitfieldCaseRun(t, s, router, "BITFIELD", "s16", "SET", "i16", "0", "-2")
	if v := bitfieldCaseGet(t, s, router, "s16", "i16", 0); v != -2 {
		t.Fatalf("CONTROL FAIL: GET i16 after SET i16 -2 = %d, want -2", v)
	}
}
