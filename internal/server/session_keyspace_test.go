package server

import (
	"testing"
	"time"
)

// A SESSION is a hash key that expires with the session: its data and
// deadline survive a restart, it belongs to the selected database, and it
// goes away at its deadline.
func TestSessionsAreKeys(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	ok := func(args ...string) {
		t.Helper()
		if got := c.do(args...); got.Type == '-' {
			t.Fatalf("%v: %v", args, got.Err)
		}
	}
	ok("SESSION.CREATE", "sess:1", "600000")
	ok("SESSION.SET", "sess:1", "user", "ada")
	ok("SESSION.SET", "sess:1", "cart", "3")
	ok("SESSION.DELETE", "sess:1", "cart")
	ok("SESSION.CREATE", "sess:empty", "600000")
	ok("SESSION.CREATE", "sess:short", "50")
	ok("SESSION.CREATE", "sess:gone", "600000")
	ok("SESSION.DELETE", "sess:gone")
	ok("HSET", "plain", "f", "v")
	if got := c.do("SESSION.SET", "plain", "f", "v"); got.Type != '-' {
		t.Fatalf("SESSION.SET on a plain hash: %v", got)
	}
	if got := c.do("SESSION.SET", "missing", "f", "v"); got.Type != '-' {
		t.Fatalf("SESSION.SET without a session: %v", got)
	}
	before := c.do("PEXPIRETIME", "sess:1").Int
	stopReplServer(s)
	time.Sleep(60 * time.Millisecond)

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	if got := c.do("SESSION.GET", "sess:1", "user"); string(got.Bulk) != "ada" {
		t.Fatalf("SESSION.GET after restart: %v", got)
	}
	if got := c.do("SESSION.ALL", "sess:1"); len(got.Array) != 2 {
		t.Fatalf("SESSION.ALL: %+v", got)
	}
	if after := c.do("PEXPIRETIME", "sess:1").Int; after != before {
		t.Fatalf("deadline %d after restart, was %d", after, before)
	}
	for id, want := range map[string]int64{"sess:empty": 1, "sess:short": 0, "sess:gone": 0, "plain": 0} {
		if id == "plain" {
			if got := c.do("SESSION.EXISTS", id); got.Type != '-' {
				t.Fatalf("SESSION.EXISTS on a plain hash: %v", got)
			}
			continue
		}
		if got := c.do("SESSION.EXISTS", id); got.Int != want {
			t.Fatalf("SESSION.EXISTS %s: %v, want %d", id, got, want)
		}
	}
	if got := c.do("SESSION.COUNT"); got.Int != 2 {
		t.Fatalf("SESSION.COUNT: %v", got)
	}
	ok("SESSION.CLEAR", "sess:1")
	if got := c.do("SESSION.ALL", "sess:1"); len(got.Array) != 0 {
		t.Fatalf("SESSION.ALL after CLEAR: %+v", got)
	}
	if got := c.do("PEXPIRETIME", "sess:1").Int; got != before {
		t.Fatalf("CLEAR changed the deadline: %d, was %d", got, before)
	}
	ok("SELECT", "1")
	if got := c.do("SESSION.COUNT"); got.Int != 0 {
		t.Fatalf("SESSION.COUNT in database 1: %v", got)
	}
}
