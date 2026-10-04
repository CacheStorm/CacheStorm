package command

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// xtrimRun executes a command through the real router. The XLEN probes below
// live in the key family, so a stream-only router would fail them.
func xtrimRun(t *testing.T, s *store.Store, args ...string) *resp.Value {
	t.Helper()
	router := NewRouter()
	RegisterStreamCommands(router)
	RegisterStringCommands(router)
	RegisterKeyCommands(router)

	var buf bytes.Buffer
	ctx := NewContext(args[0], xtrimBytes(args[1:]), s, resp.NewWriter(&buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: Execute returned error: %v", args[0], err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", args[0], buf.String(), err)
	}
	return v
}

func xtrimBytes(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

// xtrimSeed builds a stream with n entries at ids 1-0 .. n-0.
func xtrimSeed(t *testing.T, n int) *store.Store {
	t.Helper()
	s := store.NewStore()
	for i := 1; i <= n; i++ {
		xtrimRun(t, s, "XADD", "st", strconv.Itoa(i)+"-0", "f", "v")
	}
	return s
}

// MAXLEN is a count of entries to keep, so a negative value is a client error.
// The handler never checked, and StreamValue.Trim then computed
// remove = Length-maxLen, overshot the entry count and sliced out of range.
func TestXTRIMRejectsNegativeMaxlen(t *testing.T) {
	for _, arg := range []string{"-1", "-5", "-100", "-9223372036854775808"} {
		t.Run(arg, func(t *testing.T) {
			s := xtrimSeed(t, 5)

			reply := xtrimRun(t, s, "XTRIM", "st", "MAXLEN", arg)

			if reply.Type != resp.TypeError {
				t.Fatalf("XTRIM st MAXLEN %s returned type %v, want an error", arg, reply.Type)
			}
			if n := xtrimRun(t, s, "XLEN", "st").Int; n != 5 {
				t.Fatalf("XLEN after rejected XTRIM = %d, want 5 — the rejected XTRIM must not trim", n)
			}
		})
	}
}

// Control: valid MAXLEN values still trim, including the MAXLEN 0 boundary
// (which is legitimate, not an error) and a MAXLEN beyond the stream length.
func TestXTRIMValidMaxlenControls(t *testing.T) {
	s := xtrimSeed(t, 5)
	if r := xtrimRun(t, s, "XTRIM", "st", "MAXLEN", "2"); r.Type != resp.TypeInteger || r.Int != 3 {
		t.Fatalf("XTRIM st MAXLEN 2 returned %v/%d, want integer 3", r.Type, r.Int)
	}
	if n := xtrimRun(t, s, "XLEN", "st").Int; n != 2 {
		t.Fatalf("XLEN after XTRIM = %d, want 2", n)
	}

	s0 := xtrimSeed(t, 3)
	if r := xtrimRun(t, s0, "XTRIM", "st", "MAXLEN", "0"); r.Type != resp.TypeInteger || r.Int != 3 {
		t.Fatalf("XTRIM st MAXLEN 0 returned %v/%d, want integer 3", r.Type, r.Int)
	}
	if n := xtrimRun(t, s0, "XLEN", "st").Int; n != 0 {
		t.Fatalf("XLEN after XTRIM 0 = %d, want 0", n)
	}

	sBig := xtrimSeed(t, 3)
	if r := xtrimRun(t, sBig, "XTRIM", "st", "MAXLEN", "100"); r.Type != resp.TypeInteger || r.Int != 0 {
		t.Fatalf("XTRIM st MAXLEN 100 returned %v/%d, want integer 0", r.Type, r.Int)
	}
	if n := xtrimRun(t, sBig, "XLEN", "st").Int; n != 3 {
		t.Fatalf("XLEN after XTRIM 100 = %d, want 3", n)
	}
}

// Control: XTRIM against a missing stream returns 0, and the approximate "~"
// form is unaffected.
func TestXTRIMOtherPathsControls(t *testing.T) {
	s := store.NewStore()
	if r := xtrimRun(t, s, "XTRIM", "nostream", "MAXLEN", "5"); r.Type != resp.TypeInteger || r.Int != 0 {
		t.Fatalf("XTRIM on missing stream returned %v/%d, want integer 0", r.Type, r.Int)
	}

	s2 := xtrimSeed(t, 4)
	if r := xtrimRun(t, s2, "XTRIM", "st", "MAXLEN", "~", "2"); r.Type != resp.TypeInteger {
		t.Fatalf("XTRIM st MAXLEN ~ 2 returned type %v, want an integer", r.Type)
	}
	if n := xtrimRun(t, s2, "XLEN", "st").Int; n != 2 {
		t.Fatalf("XLEN after approximate XTRIM = %d, want 2", n)
	}
}
