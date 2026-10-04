package command

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// bitposRun executes a command through the real router. BITPOS on a SETBIT
// key needs both the string and bitmap families registered.
func bitposRun(t *testing.T, s *store.Store, args ...string) *resp.Value {
	t.Helper()
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterBitmapCommands(router)
	RegisterKeyCommands(router)

	var buf bytes.Buffer
	ctx := NewContext(args[0], bitposBytes(args[1:]), s, resp.NewWriter(&buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: Execute returned error: %v", args[0], err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", args[0], buf.String(), err)
	}
	return v
}

func bitposBytes(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

// bitposSetBits sets each listed bit to v.
func bitposSetBits(t *testing.T, s *store.Store, from, to int, v string) {
	t.Helper()
	for i := from; i < to; i++ {
		bitposRun(t, s, "SETBIT", "bm", strconv.Itoa(i), v)
	}
}

// When an explicit end offset is supplied, Redis confines the search to that
// range and answers -1 when the requested bit is absent from it. The handler
// previously answered len(data)*8 — a position past the end of the whole
// bitmap, and outside the range the caller asked about.
func TestBITPOSZeroBitInExplicitRangeReportsNotFound(t *testing.T) {
	s := store.NewStore()
	bitposSetBits(t, s, 0, 8, "1") // one all-ones byte: no 0 bit anywhere

	if got := bitposRun(t, s, "BITPOS", "bm", "0", "0", "-1").Int; got != -1 {
		t.Fatalf("BITPOS bm 0 0 -1 = %d, want -1 — the bounded range holds no 0 bit", got)
	}
}

// The same defect in its most confusing form: the reply points past the end of
// the entire bitmap and outside the requested range.
func TestBITPOSZeroBitNarrowRangeReportsNotFound(t *testing.T) {
	s := store.NewStore()
	bitposSetBits(t, s, 0, 8, "1")  // byte 0 all ones
	bitposSetBits(t, s, 8, 16, "0") // byte 1 all zeros

	if got := bitposRun(t, s, "BITPOS", "bm", "0", "0", "0").Int; got != -1 {
		t.Fatalf("BITPOS bm 0 0 0 = %d, want -1 — byte 0 is all ones", got)
	}
}

// Control: with no end offset, answering past the last bit IS correct Redis
// behaviour and must be preserved.
func TestBITPOSZeroBitNoRangeReturnsEndPosition(t *testing.T) {
	s := store.NewStore()
	bitposSetBits(t, s, 0, 8, "1")

	if got := bitposRun(t, s, "BITPOS", "bm", "0").Int; got != 8 {
		t.Fatalf("BITPOS bm 0 = %d, want 8 (one byte of all-ones)", got)
	}
}

// Control: a 0 bit present inside an explicit range is still found.
func TestBITPOSZeroBitFoundInRangeControl(t *testing.T) {
	s := store.NewStore()
	bitposSetBits(t, s, 0, 8, "1")
	bitposSetBits(t, s, 8, 16, "0")

	if got := bitposRun(t, s, "BITPOS", "bm", "0", "0", "-1").Int; got != 8 {
		t.Fatalf("BITPOS bm 0 0 -1 = %d, want 8", got)
	}
}

// Control: BITPOS for a 1 bit and BITCOUNT are unaffected.
func TestBITPOSOneBitAndBitCountControls(t *testing.T) {
	s := store.NewStore()
	bitposSetBits(t, s, 0, 8, "1")

	if got := bitposRun(t, s, "BITPOS", "bm", "1").Int; got != 0 {
		t.Fatalf("BITPOS bm 1 = %d, want 0", got)
	}
	if got := bitposRun(t, s, "BITPOS", "bm", "1", "0", "-1").Int; got != 0 {
		t.Fatalf("BITPOS bm 1 0 -1 = %d, want 0", got)
	}
	if got := bitposRun(t, s, "BITCOUNT", "bm").Int; got != 8 {
		t.Fatalf("BITCOUNT bm = %d, want 8", got)
	}
}
