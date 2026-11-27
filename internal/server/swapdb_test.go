package server

import (
	"strconv"
	"testing"
	"time"
)

// SWAPDB wakes a client blocked in either database, which then pops from
// the data its database holds now, as Redis' "SWAPDB awakes blocked client".
func TestSwapdbWakesBlockedClient(t *testing.T) {
	s := startReplServer(t, nil)
	blocked := dialRepl(t, s)
	c := dialRepl(t, s)
	c.str("SELECT", "1")
	c.do("RPUSH", "k", "x")
	_, _ = blocked.conn.Write(encodeArgs("BLPOP", "k", "5"))
	waitBlocked(t, c, 1)
	c.str("SWAPDB", "0", "1")
	_ = blocked.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	v, err := blocked.rd.ReadValue()
	if err != nil || str(v) != "[k x]" {
		t.Fatalf("BLPOP after SWAPDB = %s, %v; want [k x]", str(v), err)
	}
	if n := c.do("LLEN", "k").Int; n != 0 {
		t.Fatalf("db1 (the old db0) has %d elements in k", n)
	}
}

// A key that expired before the swap does not serve the blocked client,
// which keeps waiting in its database ("SWAPDB wants to wake blocked client,
// but the key already expired").
func TestSwapdbLeavesBlockedClientOnExpiredKey(t *testing.T) {
	s := startReplServer(t, nil)
	blocked := dialRepl(t, s)
	c := dialRepl(t, s)
	c.str("DEBUG", "SET-ACTIVE-EXPIRE", "0")
	t.Cleanup(func() { c.do("DEBUG", "SET-ACTIVE-EXPIRE", "1") })
	c.str("SELECT", "1")
	c.do("RPUSH", "k", "x")
	c.do("PEXPIRE", "k", "50")
	_, _ = blocked.conn.Write(encodeArgs("BLPOP", "k", "5"))
	waitBlocked(t, c, 1)
	time.Sleep(100 * time.Millisecond)
	c.str("SWAPDB", "0", "1")
	c.str("SELECT", "0")
	c.do("RPUSH", "k", "y")
	_ = blocked.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	v, err := blocked.rd.ReadValue()
	if err != nil || str(v) != "[k y]" {
		t.Fatalf("BLPOP = %s, %v; want [k y]", str(v), err)
	}
}

// waitBlocked waits until n clients are blocked.
func waitBlocked(t *testing.T, c *replClient, n int) {
	t.Helper()
	want := strconv.Itoa(n)
	deadline := time.Now().Add(3 * time.Second)
	for c.info("clients")["blocked_clients"] != want {
		if time.Now().After(deadline) {
			t.Fatalf("blocked_clients never reached %d", n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
