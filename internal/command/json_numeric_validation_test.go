package command

// Regression coverage for the silent-zero numeric parsing defect.
//
// parseJSONFloat returned 0 for any input it could not read, and the JSON
// NUMINCRBY/NUMMULTBY handlers fed their numeric argument straight through it.
// A typo therefore became a silent no-op increment, or — for NUMMULTBY — a
// multiplier of 0 that multiplied the stored number and persisted the result,
// destroying the value while replying with an ordinary bulk string.
//
// This repo already owns the correct contract: ErrNotFloat ("ERR value is not a
// valid float"), which the GEO and sortedset families return for exactly this
// situation. These tests pin the same contract for the JSON family.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func newJSONArithRouter() (*Router, *store.Store) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterJSONCommands(router)
	return router, s
}

func runJSONArith(t *testing.T, router *Router, s *store.Store, args ...string) string {
	t.Helper()
	raw := make([][]byte, len(args))
	for i, a := range args {
		raw[i] = []byte(a)
	}
	var out bytes.Buffer
	ctx := NewContext(args[0], raw[1:], s, resp.NewWriter(&out))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%v: unexpected execute error: %v", args, err)
	}
	return out.String()
}

// A non-numeric multiplier must be rejected, and must leave the stored value
// untouched. Before the fix the value was multiplied by 0 and persisted.
func TestJSONNumMultByRejectsNonNumericMultiplier(t *testing.T) {
	router, s := newJSONArithRouter()
	runJSONArith(t, router, s, "JSON.SET", "k", "$", "10")

	reply := runJSONArith(t, router, s, "JSON.NUMMULTBY", "k", "$", "abc")
	if !strings.HasPrefix(reply, "-") {
		t.Fatalf("JSON.NUMMULTBY k $ abc = %q, want a float error", reply)
	}
	if !strings.Contains(reply, "expected value") {
		t.Errorf("error %q does not report the bad number as RedisJSON does", reply)
	}

	if got := runJSONArith(t, router, s, "JSON.GET", "k", "."); got != "$2\r\n10\r\n" {
		t.Fatalf("stored value = %q, want %q — the failed multiply must not write", got, "$2\\r\\n10\\r\\n")
	}
}

// The same helper feeds NUMINCRBY, which silently added 0 instead of erroring.
func TestJSONNumIncrByRejectsNonNumericIncrement(t *testing.T) {
	router, s := newJSONArithRouter()
	runJSONArith(t, router, s, "JSON.SET", "k", "$", "10")

	reply := runJSONArith(t, router, s, "JSON.NUMINCRBY", "k", "$", "xyz")
	if !strings.HasPrefix(reply, "-") {
		t.Fatalf("JSON.NUMINCRBY k $ xyz = %q, want a float error", reply)
	}
	if got := runJSONArith(t, router, s, "JSON.GET", "k", "."); got != "$2\r\n10\r\n" {
		t.Fatalf("stored value = %q, want %q — the failed increment must not write", got, "$2\\r\\n10\\r\\n")
	}
}

// A valid multiplier must keep working: this is the control that proves the
// fix rejects only malformed input.
func TestJSONNumMultByStillMultipliesValidInput(t *testing.T) {
	router, s := newJSONArithRouter()
	runJSONArith(t, router, s, "JSON.SET", "ctl", "$", "10")

	if reply := runJSONArith(t, router, s, "JSON.NUMMULTBY", "ctl", "$", "2"); reply != "$4\r\n[20]\r\n" {
		t.Fatalf("JSON.NUMMULTBY ctl $ 2 = %q, want %q", reply, "$4\\r\\n[20]\\r\\n")
	}
	if got := runJSONArith(t, router, s, "JSON.GET", "ctl", "."); got != "$2\r\n20\r\n" {
		t.Fatalf("stored value = %q, want %q", got, "$2\\r\\n20\\r\\n")
	}
}

// Well-formed floats that the hand-rolled parser used to mangle must survive:
// a leading sign, a decimal fraction, and exponent notation. parseJSONFloat
// read "1e5" as 1 and silently ignored trailing garbage such as "12abc".
func TestJSONNumOpsAcceptWellFormedFloats(t *testing.T) {
	cases := []struct {
		name     string
		op       string
		arg      string
		seed     string
		wantBack string
	}{
		{name: "negative multiplier", op: "JSON.NUMMULTBY", arg: "-2", seed: "10", wantBack: "$3\r\n-20\r\n"},
		{name: "fractional increment", op: "JSON.NUMINCRBY", arg: "0.5", seed: "10", wantBack: "$4\r\n10.5\r\n"},
		{name: "exponent multiplier", op: "JSON.NUMMULTBY", arg: "1e2", seed: "2", wantBack: "$5\r\n200.0\r\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, s := newJSONArithRouter()
			runJSONArith(t, router, s, "JSON.SET", "k", "$", tc.seed)

			reply := runJSONArith(t, router, s, tc.op, "k", "$", tc.arg)
			if strings.HasPrefix(reply, "-") {
				t.Fatalf("%s %s was rejected: %q", tc.op, tc.arg, reply)
			}
			if got := runJSONArith(t, router, s, "JSON.GET", "k", "."); got != tc.wantBack {
				t.Fatalf("stored value = %q, want %q", got, tc.wantBack)
			}
		})
	}
}
