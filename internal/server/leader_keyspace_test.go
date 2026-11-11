package server

import (
	"strings"
	"testing"
)

// LEADER elections and LOCKX locks are hash keys: a leader and its term,
// and a lock and its holder, survive a restart and reach replicas. LOCKX
// shares LOCK's key, so LOCK.INFO reads a LOCKX lock.
func TestLeaderAndLockXAreKeys(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	elect := func(name, who, ttl string) (elected, term int64, leader string) {
		t.Helper()
		v := c.do("LEADER.ELECT", name, who, ttl)
		if v.Type == '-' || len(v.Array) != 4 {
			t.Fatalf("LEADER.ELECT %s %s: %+v", name, who, v)
		}
		if v.Array[1].Int == 1 {
			return 1, v.Array[3].Int, who
		}
		return 0, 0, string(v.Array[3].Bulk)
	}
	if ok, term, _ := elect("svc", "a", "3600000"); ok != 1 || term != 1 {
		t.Fatalf("first election: %d term %d", ok, term)
	}
	if ok, _, leader := elect("svc", "b", "3600000"); ok != 0 || leader != "a" {
		t.Fatalf("election while a leads: %d %q", ok, leader)
	}
	if got := c.do("LEADER.RESIGN", "svc", "a"); got.Int != 1 {
		t.Fatalf("RESIGN: %v", got)
	}
	if ok, term, _ := elect("svc", "b", "3600000"); ok != 1 || term != 2 {
		t.Fatalf("election after RESIGN: %d term %d", ok, term)
	}
	if got := c.do("LEADER.RENEW", "svc", "a", "1000"); got.Int != 0 {
		t.Fatalf("RENEW by a non-leader: %v", got)
	}
	if got := c.do("LEADER.RENEW", "missing", "a"); got.Type != '-' {
		t.Fatalf("RENEW of a missing election: %v", got)
	}
	if got := c.do("LEADER.ELECT", "svc", "c", "0"); got.Type != '-' {
		t.Fatalf("ELECT with ttl 0: %v", got)
	}
	if got := c.do("LOCKX.ACQUIRE", "res", "w1", "3600000"); got.Int != 1 {
		t.Fatalf("LOCKX.ACQUIRE: %v", got)
	}
	if got := c.do("LOCKX.ACQUIRE", "res", "w2", "3600000"); got.Int != 0 {
		t.Fatalf("LOCKX.ACQUIRE by another holder: %v", got)
	}
	if got := c.do("LOCKX.EXTEND", "res", "w2", "1000"); got.Int != 0 {
		t.Fatalf("LOCKX.EXTEND by another holder: %v", got)
	}
	if got := c.do("LOCKX.EXTEND", "free", "w1", "1000"); got.Int != 0 {
		t.Fatalf("LOCKX.EXTEND of a free lock: %v", got)
	}
	c.do("SET", "plain", "v")
	for _, args := range [][]string{{"LEADER.ELECT", "plain", "a"}, {"LOCKX.ACQUIRE", "plain", "a", "1000"}, {"LEADER.CURRENT", "res"}} {
		if got := c.do(args...); got.Type != '-' || !strings.HasPrefix(got.Err, "WRONGTYPE") {
			t.Fatalf("%v: %v", args, got)
		}
	}
	stopReplServer(s)

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	cur := c.do("LEADER.CURRENT", "svc")
	if len(cur.Array) != 6 || string(cur.Array[1].Bulk) != "b" || cur.Array[3].Int != 2 {
		t.Fatalf("LEADER.CURRENT after restart: %+v", cur)
	}
	hist := c.do("LEADER.HISTORY", "svc")
	if len(hist.Array) != 2 || string(hist.Array[0].Bulk) != "a" || string(hist.Array[1].Bulk) != "b" {
		t.Fatalf("LEADER.HISTORY after restart: %+v", hist)
	}
	st := c.do("LOCKX.STATUS", "res")
	if len(st.Array) != 6 || st.Array[1].Int != 1 || string(st.Array[3].Bulk) != "w1" {
		t.Fatalf("LOCKX.STATUS after restart: %+v", st)
	}
	if got := c.do("LOCK.INFO", "res"); len(got.Array) == 0 || string(got.Array[1].Bulk) != "w1" {
		t.Fatalf("LOCK.INFO of a LOCKX lock: %+v", got)
	}
	if got := c.do("LOCKX.RELEASE", "res", "w1"); got.Int != 1 {
		t.Fatalf("LOCKX.RELEASE: %v", got)
	}
	if got := c.do("LOCKX.STATUS", "res"); len(got.Array) != 2 || got.Array[1].Int != 0 {
		t.Fatalf("LOCKX.STATUS after RELEASE: %+v", got)
	}
}

// LEADER.HISTORY keeps the last hundred terms.
func TestLeaderHistoryIsBounded(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	defer stopReplServer(s)
	c := dialRepl(t, s)
	for i := range 120 {
		who := "n" + strings.Repeat("x", i%3)
		if v := c.do("LEADER.ELECT", "e", who); v.Array[1].Int != 1 {
			t.Fatalf("election %d: %+v", i, v)
		}
		c.do("LEADER.RESIGN", "e", who)
	}
	if got := c.do("LEADER.HISTORY", "e"); len(got.Array) != 100 {
		t.Fatalf("history holds %d terms", len(got.Array))
	}
	if got := c.do("HLEN", "e"); got.Int != 103 {
		t.Fatalf("HLEN: %v", got)
	}
}
