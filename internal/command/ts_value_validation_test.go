package command

// Regression coverage for silent-zero numeric parsing in the TS family.
//
// The value argument of TS.ADD and TS.MADD was routed through parseJSONFloat,
// a hand-rolled parser that returns 0 for anything it cannot read and reports
// no error. A typo'd value therefore wrote a bogus zero sample into the series
// and replied with the resolved timestamp, exactly as if the write had
// succeeded.
//
// The contract is the one this repo already owns: ErrNotFloat ("ERR value is
// not a valid float", internal/command/context.go:19), which the GEO and
// sortedset families return for exactly this input.
//
// The timestamp argument is deliberately NOT validated: "*" is the documented
// auto-timestamp sentinel, and parseInt64("*") == 0. TestTSAddKeepsAutoTimestamp
// pins that so the strictness is not widened by accident later.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func newTSValueRouter() *Router {
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterTSCommands(router)
	return router
}

func runTSValue(t *testing.T, router *Router, args ...string) string {
	t.Helper()
	raw := make([][]byte, len(args))
	for i, a := range args {
		raw[i] = []byte(a)
	}
	var out bytes.Buffer
	ctx := NewContext(args[0], raw[1:], tsValueStore, resp.NewWriter(&out))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%v: unexpected execute error: %v", args, err)
	}
	return out.String()
}

// tsValueStore backs the helper above. The TS handlers share the package-level
// tsManager, so isolation comes from a unique key per test rather than from
// swapping the manager.
var tsValueStore = store.NewStore()

// withFreshTSManager gives the test its own time-series manager. The TS
// handlers share the package-level tsManager, so a series written by an
// earlier test (or by an earlier -count iteration) is still there: TS.ADD
// then reports a duplicate timestamp, and a counter starts from the value
// the previous iteration left. The previous manager is restored afterwards.
func withFreshTSManager(t *testing.T) {
	t.Helper()
	previous := tsManager
	tsManager = store.NewTimeSeriesManager()
	t.Cleanup(func() { tsManager = previous })
}

// A non-numeric value must be rejected and must NOT leave a sample behind.
// Before the fix TS.ADD replied ":1000" and stored a zero sample.
func TestTSAddRejectsNonNumericValue(t *testing.T) {
	withFreshTSManager(t)
	router := newTSValueRouter()

	reply := runTSValue(t, router, "TS.ADD", "ts:add:garbage", "1000", "abc")
	if !strings.HasPrefix(reply, "-") {
		t.Fatalf("TS.ADD with value %q = %q, want a float error", "abc", reply)
	}
	if !strings.Contains(reply, "not a valid float") {
		t.Errorf("error %q does not name the float contract", reply)
	}

	// The rejected write must not have created the series or a zero sample.
	if got := runTSValue(t, router, "TS.RANGE", "ts:add:garbage", "-", "+"); !strings.HasPrefix(got, "*0") {
		t.Fatalf("TS.RANGE after the rejected add = %q, want an empty array — no sample may be written", got)
	}
}

// TS.MADD feeds the same parser and must reject rather than store 0.
func TestTSMAddRejectsNonNumericValue(t *testing.T) {
	withFreshTSManager(t)
	router := newTSValueRouter()

	reply := runTSValue(t, router, "TS.MADD", "ts:madd:garbage", "1000", "zzz")
	if !strings.HasPrefix(reply, "-") {
		t.Fatalf("TS.MADD with value %q = %q, want a float error", "zzz", reply)
	}
	if got := runTSValue(t, router, "TS.RANGE", "ts:madd:garbage", "-", "+"); !strings.HasPrefix(got, "*0") {
		t.Fatalf("TS.RANGE after the rejected madd = %q, want an empty array", got)
	}
}

// Control: well-formed values keep working, so the fix rejects only garbage.
// Each case gets its own series key: TS.ADD rejects a duplicate timestamp
// (round 8's BLOCK policy), so reusing one key would fail on the policy, not
// on the value parsing this test is about.
func TestTSAddAcceptsValidValues(t *testing.T) {
	withFreshTSManager(t)
	for _, tc := range []struct{ name, value, want string }{
		{name: "positive", value: "5", want: "5"},
		{name: "negative", value: "-2", want: "-2"},
		{name: "fractional", value: "0.5", want: "0.5"},
		{name: "exponent", value: "1e2", want: "100"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := newTSValueRouter()
			key := "ts:ok:" + tc.name
			if reply := runTSValue(t, router, "TS.ADD", key, "1000", tc.value); strings.HasPrefix(reply, "-") {
				t.Fatalf("TS.ADD with value %q was rejected: %q", tc.value, reply)
			}
			if got := runTSValue(t, router, "TS.GET", key, "1000"); !strings.Contains(got, tc.want) {
				t.Fatalf("TS.GET = %q, want it to contain %q", got, tc.want)
			}
		})
	}
}

// The timestamp keeps its "*" auto-timestamp sentinel. parseInt64("*") is 0,
// so the timestamp must stay unvalidated even though the value is now strict.
func TestTSAddKeepsAutoTimestamp(t *testing.T) {
	withFreshTSManager(t)
	router := newTSValueRouter()

	// This series name is unique to this test. tsManager is a package global
	// shared by every test in the package, so reusing a series another test has
	// written to means "*" resolves against a non-empty series: ts_counter_
	// validation_test.go's TS.INCRBY ... TIMESTAMP 0 leaves a sample at now-ms,
	// and "*" then resolves to 0, which the duplicate-timestamp guard rejects as
	// non-increasing. That failure was about shared state, not about the
	// sentinel, so the fix is an isolated series rather than a relaxed assertion.
	reply := runTSValue(t, router, "TS.ADD", "ts:autosentinel", "*", "5")
	if strings.HasPrefix(reply, "-") {
		t.Fatalf("TS.ADD with timestamp %q was rejected: %q — the auto-timestamp sentinel must keep working", "*", reply)
	}
}
