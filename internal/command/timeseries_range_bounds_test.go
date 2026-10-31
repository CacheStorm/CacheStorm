package command

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: TS.RANGE / TS.REVRANGE / TS.DEL parsed their endpoints with
// parseInt64, which returns 0 for any non-numeric input:
//
//	func parseInt64(s string) int64 {
//	    v, err := strconv.ParseInt(s, 10, 64)
//	    if err != nil { return 0 }
//	    return v
//	}
//
// Redis defines "-" as the minimum possible timestamp and "+" as the maximum.
// Because both fell through to 0, the single most common query form
// `TS.RANGE key - +` silently collapsed to Range(0, 0), and
// internal/store/timeseries.go's `s.Timestamp >= from && s.Timestamp <= to`
// matched only a sample at exactly instant 0 -- so it always returned empty.
// TS.DEL key - + silently deleted NOTHING while replying with a count.
//
// All three commands are pinned here against actually-stored samples. Note the
// argument order is fromTimestamp-toTimestamp for BOTH TS.RANGE and TS.REVRANGE
// (Redis differs only in output ordering), so `TS.REVRANGE key - +` is the
// whole-key form -- see internal/command/ts_revrange_options_test.go, which
// already pinned the low-to-high convention.

func tsRangeRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
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

func tsRangeRouter(t *testing.T) *Router {
	t.Helper()
	router := NewRouter()
	RegisterTSCommands(router)
	RegisterKeyCommands(router)
	return router
}

func tsRangeSeed(t *testing.T, s *store.Store, router *Router, key string) {
	t.Helper()
	tsRangeRun(t, s, router, "TS.CREATE", key)
	for _, ts := range []int64{1000, 2000, 3000} {
		out := tsRangeRun(t, s, router, "TS.ADD", key, strconv.FormatInt(ts, 10), "1")
		if !bytes.HasPrefix([]byte(out), []byte(":")) {
			t.Fatalf("TS.ADD %s %d failed: %q", key, ts, out)
		}
	}
}

func tsRangeLen(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) int {
	t.Helper()
	out := tsRangeRun(t, s, router, cmd, args...)
	v, err := resp.NewReader(bytes.NewReader([]byte(out))).ReadValue()
	if err != nil {
		t.Fatalf("unparsable %s reply %q: %v", cmd, out, err)
	}
	if v.Type != resp.TypeArray {
		t.Fatalf("%s %v returned %q, want an array", cmd, args, out)
	}
	return len(v.Array)
}

// TestTSRangeMinusPlusReturnsAllSamples is the primary case for the endpoint
// sentinel: "-" and "+" mean the whole key.
func TestTSRangeMinusPlusReturnsAllSamples(t *testing.T) {
	router := tsRangeRouter(t)
	s := store.NewStore()
	tsRangeSeed(t, s, router, "range")

	for _, c := range []struct {
		name string
		cmd  string
		args []string
		want int
	}{
		{"TS.RANGE - +", "TS.RANGE", []string{"range", "-", "+"}, 3},
		{"TS.RANGE 0 +", "TS.RANGE", []string{"range", "0", "+"}, 3},
		{"TS.RANGE - 3000", "TS.RANGE", []string{"range", "-", "3000"}, 3},
		{"TS.RANGE 1000 +", "TS.RANGE", []string{"range", "1000", "+"}, 3},
		{"TS.REVRANGE - +", "TS.REVRANGE", []string{"range", "-", "+"}, 3},
		{"TS.REVRANGE 0 +", "TS.REVRANGE", []string{"range", "0", "+"}, 3},
		{"TS.REVRANGE - 3000", "TS.REVRANGE", []string{"range", "-", "3000"}, 3},
		{"TS.REVRANGE 1000 +", "TS.REVRANGE", []string{"range", "1000", "+"}, 3},
	} {
		if n := tsRangeLen(t, s, router, c.cmd, c.args...); n != c.want {
			t.Fatalf("%s returned %d samples, want %d; parseInt64 turned \"-\" and \"+\" "+
				"into 0, so the range collapsed to the single instant 0", c.name, n, c.want)
		}
	}
}

// TestTSDELMinusPlusDeletesEverything covers the third site: TS.DEL shares the
// endpoint parsing, so `TS.DEL key - +` silently deleted nothing.
func TestTSDELMinusPlusDeletesEverything(t *testing.T) {
	router := tsRangeRouter(t)
	s := store.NewStore()
	tsRangeSeed(t, s, router, "del")

	if out := tsRangeRun(t, s, router, "TS.DEL", "del", "-", "+"); out != ":3\r\n" {
		t.Fatalf("TS.DEL del - + = %q, want :3; parseInt64 turned \"-\"/\"+\" into 0, so "+
			"nothing matched and the samples silently survived", out)
	}
	if out := tsRangeRun(t, s, router, "TS.INFO", "del"); !bytes.Contains([]byte(out), []byte(":0\r\n")) {
		t.Fatalf("TS.DEL del - + left samples behind: TS.INFO = %q", out)
	}
}

// TestTSRangeControlsUncontended is the control: explicit low-to-high bounds must
// still filter exactly, an empty window and a missing key must return 0, and a
// single-sample series must be returned whole. None of these touch "-" or "+",
// so they pass before and after the fix.
func TestTSRangeControlsUncontended(t *testing.T) {
	router := tsRangeRouter(t)
	s := store.NewStore()
	tsRangeSeed(t, s, router, "ctrl")

	for _, c := range []struct {
		from, to string
		want     int
	}{
		{"1000", "3000", 3},
		{"1500", "2500", 1},
		{"2000", "2000", 1},
		{"5000", "6000", 0},
		{"1", "999", 0},
	} {
		if n := tsRangeLen(t, s, router, "TS.RANGE", "ctrl", c.from, c.to); n != c.want {
			t.Fatalf("CONTROL FAIL: TS.RANGE ctrl %s %s returned %d samples, want %d",
				c.from, c.to, n, c.want)
		}
	}

	s2 := store.NewStore()
	if got := tsRangeRun(t, s2, router, "TS.RANGE", "nope", "-", "+"); got != "-ERR TSDB: the key does not exist\r\n" {
		t.Fatalf("CONTROL FAIL: TS.RANGE on a missing key returned %q", got)
	}

	s3 := store.NewStore()
	tsRangeRun(t, s3, router, "TS.CREATE", "one")
	tsRangeRun(t, s3, router, "TS.ADD", "one", "1000", "1")
	if n := tsRangeLen(t, s3, router, "TS.RANGE", "one", "0", "1000"); n != 1 {
		t.Fatalf("CONTROL FAIL: single-sample TS.RANGE 0 1000 returned %d, want 1", n)
	}
}
