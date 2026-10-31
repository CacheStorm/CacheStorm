package command

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: TS.CREATE / TS.ADD / TS.ALTER multiplied the RETENTION option by
// time.Millisecond WITHOUT bounding it, so a large value overflowed int64
// nanoseconds and wrapped NEGATIVE.
//
//	internal/command/timeseries_commands.go (before all three sites)
//	  ms := parseInt64(ctx.ArgString(i))          // or ParseInt
//	  retention = time.Duration(ms) * time.Millisecond      // unchecked
//
// Every trim site in internal/store/timeseries.go gates on `if v.Retention > 0`
// (:106 AddSample, :161 AddWithLabels, :193 Add), so a wrapped-negative retention
// silently DISABLES trimming and the series grows without bound — unbounded
// memory growth from a single command, behind a successful reply.
//
//	RETENTION 9223372036854775807 -> -1000000 ns           (negative)
//	RETENTION 10000000000000      -> -8446744073709551616 ns (negative)
//
// maxExpireAtMillis (MaxInt64/time.Millisecond) is the largest value whose
// product still fits in int64. Redis also rejects a negative retention.
//
// Trimming runs on every Add, so the effect is observable immediately in
// TS.INFO's totalSamples — no waiting, no sleeps. TS.RANGE is deliberately not
// used: on this build it returns an empty array regardless of stored samples
// (a separate, unrelated defect).

func tsRetRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
	t.Helper()
	buf := &bytes.Buffer{}
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("%s: Execute returned error: %v", cmd, err)
	}
	return buf.String()
}

func tsRetRouter(t *testing.T) *Router {
	t.Helper()
	router := NewRouter()
	RegisterTSCommands(router)
	RegisterKeyCommands(router)
	return router
}

// tsTotalSamples reads TS.INFO's totalSamples, which reflects trimming because
// trimming runs on every Add.
func tsTotalSamples(t *testing.T, s *store.Store, router *Router, key string) int {
	t.Helper()
	out := tsRetRun(t, s, router, "TS.INFO", key)
	v, err := resp.NewReader(bytes.NewReader([]byte(out))).ReadValue()
	if err != nil {
		t.Fatalf("unparsable TS.INFO %q reply %q: %v", key, out, err)
	}
	for i := 0; i+1 < len(v.Array); i += 2 {
		if string(v.Array[i].Bulk) == "totalSamples" {
			return int(v.Array[i+1].Int)
		}
	}
	t.Fatalf("TS.INFO %q returned no totalSamples field: %q", key, out)
	return -1
}

// tsStale is a timestamp 24h in the past — older than every retention used
// here, so a correctly-bounded retention must drop it.
func tsStale() string {
	return strconv.FormatInt(time.Now().Add(-24*time.Hour).UnixMilli(), 10)
}

// TestTSRetentionOverflowIsRejected is the primary case: an out-of-range
// RETENTION must be refused, and must never disable trimming.
func TestTSRetentionOverflowIsRejected(t *testing.T) {
	router := tsRetRouter(t)

	for _, c := range []struct{ retention, name string }{
		{"9223372036854775807", "MaxInt64"},
		{"10000000000000", "1e13"},
		{"-1", "negative"},
	} {
		s := store.NewStore()
		if out := tsRetRun(t, s, router, "TS.CREATE", "ts:k", "RETENTION", c.retention); strings.HasPrefix(out, "-") {
			continue // correctly rejected
		}
		tsRetRun(t, s, router, "TS.ADD", "ts:k", tsStale(), "1")
		if n := tsTotalSamples(t, s, router, "ts:k"); n != 0 {
			t.Fatalf("TS.CREATE RETENTION %s (%s): a 24h-old sample SURVIVED "+
				"(totalSamples=%d); the unchecked multiplication wrapped negative, so "+
				"`v.Retention > 0` is false and trimming never runs", c.retention, c.name, n)
		}
	}
}

// TestTSRetentionOverflowOnAddAndAlter covers the same defect on the other two
// sites: TS.ADD's and TS.ALTER's RETENTION options.
func TestTSRetentionOverflowOnAddAndAlter(t *testing.T) {
	router := tsRetRouter(t)

	// TS.ADD RETENTION creates the series with that retention.
	s := store.NewStore()
	if out := tsRetRun(t, s, router, "TS.ADD", "ts:add", tsStale(), "1", "RETENTION", "9223372036854775807"); strings.HasPrefix(out, "-") {
		return // rejected
	}
	if n := tsTotalSamples(t, s, router, "ts:add"); n != 0 {
		t.Fatalf("TS.ADD RETENTION MaxInt64: a 24h-old sample SURVIVED "+
			"(totalSamples=%d); the overflowing retention disabled trimming", n)
	}

	// TS.ALTER RETENTION applies to an existing series.
	s2 := store.NewStore()
	tsRetRun(t, s2, router, "TS.CREATE", "ts:alter")
	tsRetRun(t, s2, router, "TS.ADD", "ts:alter", tsStale(), "1")
	if n := tsTotalSamples(t, s2, router, "ts:alter"); n != 1 {
		t.Fatalf("CONTROL FAIL: pre-condition — with no retention the stale sample "+
			"should be retained, got totalSamples=%d", n)
	}
	if out := tsRetRun(t, s2, router, "TS.ALTER", "ts:alter", "RETENTION", "9223372036854775807"); strings.HasPrefix(out, "-") {
		return // rejected
	}
	tsRetRun(t, s2, router, "TS.ADD", "ts:alter", tsStale(), "1")
	if n := tsTotalSamples(t, s2, router, "ts:alter"); n != 0 {
		t.Fatalf("TS.ALTER RETENTION MaxInt64: stale samples SURVIVED "+
			"(totalSamples=%d); an overflowing retention must not keep stale data forever", n)
	}
}

// TestTSRetentionControlsUncontended is the control: a normal retention still
// trims, RETENTION 0 still means "keep everything", a fresh sample survives, and
// TS.ALTER RETENTION 0 still clears. These pass before and after the fix.
func TestTSRetentionControlsUncontended(t *testing.T) {
	router := tsRetRouter(t)

	s := store.NewStore()
	tsRetRun(t, s, router, "TS.CREATE", "ts:small", "RETENTION", "1000")
	tsRetRun(t, s, router, "TS.ADD", "ts:small", tsStale(), "1")
	if n := tsTotalSamples(t, s, router, "ts:small"); n != 0 {
		t.Fatalf("CONTROL FAIL: a 1s retention left totalSamples=%d, want 0", n)
	}

	s2 := store.NewStore()
	tsRetRun(t, s2, router, "TS.CREATE", "ts:zero", "RETENTION", "0")
	tsRetRun(t, s2, router, "TS.ADD", "ts:zero", tsStale(), "1")
	if n := tsTotalSamples(t, s2, router, "ts:zero"); n != 1 {
		t.Fatalf("CONTROL FAIL: RETENTION 0 must keep all samples, got %d", n)
	}

	s3 := store.NewStore()
	tsRetRun(t, s3, router, "TS.CREATE", "ts:recent", "RETENTION", "3600000")
	tsRetRun(t, s3, router, "TS.ADD", "ts:recent", "*", "1")
	if n := tsTotalSamples(t, s3, router, "ts:recent"); n != 1 {
		t.Fatalf("CONTROL FAIL: a fresh sample must survive, got %d", n)
	}

	s4 := store.NewStore()
	tsRetRun(t, s4, router, "TS.CREATE", "ts:clear", "RETENTION", "1000")
	tsRetRun(t, s4, router, "TS.ALTER", "ts:clear", "RETENTION", "0")
	tsRetRun(t, s4, router, "TS.ADD", "ts:clear", tsStale(), "1")
	if n := tsTotalSamples(t, s4, router, "ts:clear"); n != 1 {
		t.Fatalf("CONTROL FAIL: TS.ALTER RETENTION 0 must keep all samples, got %d", n)
	}

	// The largest in-range retention (maxExpireAtMillis) must still be accepted.
	s5 := store.NewStore()
	if out := tsRetRun(t, s5, router, "TS.CREATE", "ts:max", "RETENTION",
		strconv.FormatInt(maxExpireAtMillis, 10)); strings.HasPrefix(out, "-") {
		t.Fatalf("CONTROL FAIL: RETENTION maxExpireAtMillis (%d) must be accepted, got %q",
			maxExpireAtMillis, out)
	}
}
