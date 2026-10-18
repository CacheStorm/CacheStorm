package command

// Regression coverage for silent-zero numeric parsing in the TS counter
// commands.
//
// cmdTSINCRBY and cmdTSDECRBY routed their increment/decrement through
// parseJSONFloat, a hand-rolled parser that returns 0 for anything it cannot
// read and reports no error. Round 16 fixed the identical defect in
// cmdTSADD/cmdTSMADD but left these two behind, so a typo'd increment became
// "add nothing": the counter kept its old value and the handler still replied
// with a success-shaped timestamp. A monitoring counter that silently stops
// advancing is a wrong answer, not a crash.
//
// The contract is the one this repo already owns: ErrNotFloat ("ERR value is
// not a valid float", internal/command/context.go:19), used by the GEO,
// sortedset, JSON and TS.ADD/TS.MADD families.
//
// The TIMESTAMP argument is deliberately NOT validated: 0 is this handler's
// documented "use now" sentinel, so tightening it is a different contract
// question.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func newTSCounterRouter() *Router {
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterTSCommands(router)
	return router
}

// All TS handlers share the package-level tsManager, so isolation comes from
// a unique series key per test rather than from swapping the manager.
var tsCounterStore = store.NewStore()

func runTSCounter(t *testing.T, router *Router, args ...string) string {
	t.Helper()
	raw := make([][]byte, len(args))
	for i, a := range args {
		raw[i] = []byte(a)
	}
	var out bytes.Buffer
	ctx := NewContext(args[0], raw[1:], tsCounterStore, resp.NewWriter(&out))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%v: unexpected execute error: %v", args, err)
	}
	return out.String()
}

// tsCounterLatest reads the newest sample's value out of TS.GET, which answers
// [timestamp, value].
func tsCounterLatest(t *testing.T, router *Router, key string) string {
	t.Helper()
	fields := strings.Fields(runTSCounter(t, router, "TS.GET", key, "+"))
	if len(fields) == 0 {
		t.Fatalf("TS.GET %s + returned no fields", key)
	}
	return fields[len(fields)-1]
}

// A non-numeric increment must be rejected and must NOT move the counter.
// Before the fix it replied with a timestamp and left the value unchanged.
func TestTSIncrByRejectsNonNumericIncrement(t *testing.T) {
	router := newTSCounterRouter()
	runTSCounter(t, router, "TS.ADD", "ts:incr:garbage", "1000", "10")

	before := tsCounterLatest(t, router, "ts:incr:garbage")
	if before != "10" {
		t.Fatalf("seed value = %s, want 10", before)
	}

	reply := runTSCounter(t, router, "TS.INCRBY", "ts:incr:garbage", "abc")
	if !strings.HasPrefix(reply, "-") {
		t.Fatalf("TS.INCRBY with increment %q = %q, want a float error", "abc", reply)
	}
	if !strings.Contains(reply, "not a valid float") {
		t.Errorf("error %q does not name the float contract", reply)
	}

	if after := tsCounterLatest(t, router, "ts:incr:garbage"); after != before {
		t.Fatalf("counter moved from %s to %s on a rejected increment — a typo must not "+
			"silently freeze a monitoring counter", before, after)
	}
}

// TSDECRBY feeds the same parser and must reject rather than subtract 0.
func TestTSDecrByRejectsNonNumericDecrement(t *testing.T) {
	router := newTSCounterRouter()
	runTSCounter(t, router, "TS.ADD", "ts:decr:garbage", "1000", "10")

	reply := runTSCounter(t, router, "TS.DECRBY", "ts:decr:garbage", "zzz")
	if !strings.HasPrefix(reply, "-") {
		t.Fatalf("TS.DECRBY with decrement %q = %q, want a float error", "zzz", reply)
	}
	if after := tsCounterLatest(t, router, "ts:decr:garbage"); after != "10" {
		t.Fatalf("counter = %s after a rejected decrement, want 10", after)
	}
}

// Control: well-formed increments and decrements keep working, and the
// counter actually advances in both directions. One step per case: INCRBY
// without TIMESTAMP stamps "now", so the series can no longer be re-seeded at
// a fixed past timestamp.
func TestTSIncrDecrByAcceptsValidValues(t *testing.T) {
	cases := []struct {
		name   string
		cmd    string
		amount string
		want   string
	}{
		{name: "integer increment", cmd: "TS.INCRBY", amount: "5", want: "15"},
		{name: "negative increment", cmd: "TS.INCRBY", amount: "-3", want: "7"},
		{name: "fractional decrement", cmd: "TS.DECRBY", amount: "0.5", want: "9.5"},
		{name: "exponent increment", cmd: "TS.INCRBY", amount: "1e1", want: "20"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := newTSCounterRouter()
			key := "ts:counter:" + tc.name
			runTSCounter(t, router, "TS.ADD", key, "1000", "10")

			reply := runTSCounter(t, router, tc.cmd, key, tc.amount)
			if strings.HasPrefix(reply, "-") {
				t.Fatalf("%s %s was rejected: %q", tc.cmd, tc.amount, reply)
			}
			if got := tsCounterLatest(t, router, key); got != tc.want {
				t.Fatalf("counter = %s, want %s", got, tc.want)
			}
		})
	}
}

// The TIMESTAMP argument keeps its lenient parse: 0 means "use now".
func TestTSIncrByKeepsNowTimestampSentinel(t *testing.T) {
	router := newTSCounterRouter()
	runTSCounter(t, router, "TS.ADD", "ts:auto", "1000", "10")

	if reply := runTSCounter(t, router, "TS.INCRBY", "ts:auto", "5", "TIMESTAMP", "0"); strings.HasPrefix(reply, "-") {
		t.Fatalf("TIMESTAMP 0 (the use-now sentinel) was rejected: %q", reply)
	}
}
