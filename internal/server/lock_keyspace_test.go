package server

import (
	"testing"
	"time"
)

// A LOCK is a key: it survives a restart with the deadline it was given,
// belongs to the selected database, and LOCK.ACQUIRE waits for a release
// made by another client.
func TestLocksAreKeys(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	integer := func(c *replClient, want int64, args ...string) {
		t.Helper()
		if got := c.do(args...); got.Type != ':' || got.Int != want {
			t.Fatalf("%v: %+v, want %d", args, got, want)
		}
	}
	integer(c, 1, "LOCK.TRY", "job", "w1", "t1", "60000")
	integer(c, 0, "LOCK.TRY", "job", "w2", "t2", "60000")
	integer(c, 1, "LOCK.TRY", "job", "w1", "t1", "60000") // renews
	integer(c, 1, "LOCK.TRY", "short", "w1", "t1", "1")
	if got := c.do("HGET", "job", "holder"); string(got.Bulk) != "w1" {
		t.Fatalf("HGET job holder: %v", got)
	}
	c.do("SET", "plain", "v")
	if got := c.do("LOCK.TRY", "plain", "w1", "t1", "1000"); got.Type != '-' {
		t.Fatalf("LOCK.TRY on a string: %v", got)
	}
	if got := c.do("LOCK.TRY", "job", "w1", "t1", "0"); got.Type != '-' {
		t.Fatalf("LOCK.TRY with ttl 0: %v", got)
	}
	c.do("SELECT", "1")
	integer(c, 0, "LOCK.ISLOCKED", "job")
	c.do("SELECT", "0")
	before := c.do("PEXPIRETIME", "job").Int
	stopReplServer(s)

	time.Sleep(5 * time.Millisecond)
	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	integer(c, 1, "LOCK.ISLOCKED", "job")
	integer(c, 0, "LOCK.ISLOCKED", "short")
	if after := c.do("PEXPIRETIME", "job").Int; after != before {
		t.Fatalf("deadline %d after restart, was %d", after, before)
	}
	if got := c.do("LOCK.INFO", "job"); len(got.Array) != 8 || string(got.Array[1].Bulk) != "w1" || got.Array[7].Int != 1 {
		t.Fatalf("LOCK.INFO: %+v", got)
	}
	integer(c, 0, "LOCK.RELEASE", "job", "w2", "t2")

	// A waiter gets the lock when its holder releases it.
	w := dialRepl(t, s)
	done := make(chan int64, 1)
	go func() { done <- w.do("LOCK.ACQUIRE", "job", "w2", "t2", "60000", "5000").Int }()
	time.Sleep(30 * time.Millisecond)
	integer(c, 1, "LOCK.RELEASE", "job", "w1", "t1")
	select {
	case got := <-done:
		if got != 1 {
			t.Fatalf("LOCK.ACQUIRE after release: %d", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("LOCK.ACQUIRE did not get the released lock")
	}
	if got := c.do("HGET", "job", "holder"); string(got.Bulk) != "w2" {
		t.Fatalf("holder after ACQUIRE: %v", got)
	}
	// And gives up at its timeout.
	start := time.Now()
	integer(c, 0, "LOCK.ACQUIRE", "job", "w3", "t3", "1000", "50")
	if d := time.Since(start); d < 50*time.Millisecond || d > 2*time.Second {
		t.Fatalf("LOCK.ACQUIRE timed out after %v", d)
	}
}
