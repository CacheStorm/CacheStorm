package server

import (
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func fieldInt(t *testing.T, v *resp.Value, name string) int64 {
	t.Helper()
	for i := 0; i+1 < len(v.Array); i += 2 {
		if string(v.Array[i].Bulk) == name {
			return v.Array[i+1].Int
		}
	}
	t.Fatalf("no %s in %+v", name, v)
	return 0
}

// CIRCUITX and CIRCUITBREAKER breakers are hash keys: an open breaker stays
// open across a restart, with its counters, and a key holding anything else
// is refused.
func TestCircuitXBreakersAreKeys(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	ok := func(args ...string) *resp.Value {
		t.Helper()
		v := c.do(args...)
		if v.Type == '-' {
			t.Fatalf("%v: %v", args, v.Err)
		}
		return v
	}
	ok("CIRCUITX.CREATE", "payments", "2", "3600000")
	ok("CIRCUITX.RECORD", "payments", "SUCCESS")
	ok("CIRCUITX.RECORD", "payments", "FAILURE")
	if got := ok("CIRCUITX.RECORD", "payments", "FAILURE"); string(got.Bulk) != "open" {
		t.Fatalf("state after two failures: %+v", got)
	}
	if got := ok("CIRCUITX.ALLOW", "payments"); got.Int != 0 {
		t.Fatalf("ALLOW while open: %v", got)
	}
	ok("CIRCUITBREAKER.CREATE", "legacy", "1", "3600000")
	ok("CIRCUITBREAKER.TRIP", "legacy")
	ok("SET", "plain", "v")
	for _, args := range [][]string{{"CIRCUITX.ALLOW", "plain"}, {"CIRCUITX.CREATE", "plain", "1", "1"}, {"CIRCUITBREAKER.STATE", "plain"}} {
		if got := c.do(args...); got.Type != '-' || !strings.HasPrefix(got.Err, "WRONGTYPE") {
			t.Fatalf("%v: %v", args, got)
		}
	}
	if got := c.do("CIRCUITX.ALLOW", "missing"); got.Type != '-' {
		t.Fatalf("ALLOW of a missing breaker: %v", got)
	}
	stopReplServer(s)

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	st := ok("CIRCUITX.STATUS", "payments")
	if string(st.Array[3].Bulk) != "open" || fieldInt(t, st, "retry_after") <= 0 {
		t.Fatalf("STATUS after restart: %+v", st)
	}
	m := ok("CIRCUITX.METRICS", "payments")
	if fieldInt(t, m, "total_failures") != 2 || fieldInt(t, m, "total_successes") != 1 || fieldInt(t, m, "rejected") != 1 {
		t.Fatalf("METRICS after restart: %+v", m)
	}
	if got := ok("CIRCUITBREAKER.STATE", "legacy"); string(got.Array[3].Bulk) != "open" {
		t.Fatalf("CIRCUITBREAKER.STATE after restart: %+v", got)
	}
	ok("CIRCUITX.HALFOPEN", "payments")
	if got := ok("CIRCUITX.RECORD", "payments", "SUCCESS"); string(got.Bulk) != "closed" {
		t.Fatalf("success while half-open: %+v", got)
	}
	ok("CIRCUITX.RESET", "payments")
	if m := ok("CIRCUITX.METRICS", "payments"); fieldInt(t, m, "total_failures") != 0 {
		t.Fatalf("METRICS after RESET: %+v", m)
	}
	if got := ok("CIRCUITX.DELETE", "payments"); got.Int != 1 {
		t.Fatalf("DELETE: %v", got)
	}
}

// Failures recorded concurrently are all counted.
func TestCircuitXCountsConcurrentFailures(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	defer stopReplServer(s)
	c := dialRepl(t, s)
	c.do("CIRCUITX.CREATE", "svc", "1000000", "1000")
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := dialRepl(t, s)
			for range 40 {
				w.do("CIRCUITX.RECORD", "svc", "FAILURE")
			}
		}()
	}
	wg.Wait()
	if got := fieldInt(t, c.do("CIRCUITX.METRICS", "svc"), "total_failures"); got != 320 {
		t.Fatalf("total_failures = %d after 320 concurrent failures", got)
	}
}
