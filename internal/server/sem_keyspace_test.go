package server

import (
	"strings"
	"testing"
	"time"
)

// SEM and SEMAPHOREX semaphores and SESSIONX sessions are keys: permits
// taken and sessions with their data survive a restart.
func TestSemaphoresAndSessionXAreKeys(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	ok := func(args ...string) {
		t.Helper()
		if v := c.do(args...); v.Type == '-' {
			t.Fatalf("%v: %v", args, v.Err)
		}
	}
	ok("SEM.CREATE", "pool", "3")
	ok("SEM.ACQUIRE", "pool", "2")
	ok("SEMAPHOREX.CREATE", "px", "4")
	ok("SEMAPHOREX.ACQUIRE", "px", "3")
	id := string(c.do("SESSIONX.CREATE", "alice", "3600000").Bulk)
	ok("SESSIONX.SET", id, "cart", "3 items")
	if got := c.do("SESSIONX.SET", id, "__sessionx_user", "mallory"); got.Type != '-' {
		t.Fatalf("SET of a reserved field: %v", got)
	}
	c.do("SET", "plain", "v")
	for _, args := range [][]string{{"SEM.ACQUIRE", "plain", "1"}, {"SEMAPHOREX.ACQUIRE", "plain"}, {"SESSIONX.GET", "plain"}, {"SEM.TRYACQUIRE", "px", "1"}} {
		if got := c.do(args...); got.Type != '-' || !strings.HasPrefix(got.Err, "WRONGTYPE") {
			t.Fatalf("%v: %v", args, got)
		}
	}
	stopReplServer(s)

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	if got := c.do("SEM.TRYACQUIRE", "pool", "2"); got.Int != 0 {
		t.Fatalf("SEM.TRYACQUIRE of taken permits after restart: %v", got)
	}
	if got := c.do("SEM.VALUE", "pool"); got.Array[1].Int != 1 {
		t.Fatalf("SEM.VALUE after restart: %+v", got)
	}
	if got := c.do("SEMAPHOREX.ACQUIRE", "px", "2"); got.Int != 0 {
		t.Fatalf("SEMAPHOREX.ACQUIRE past the max after restart: %v", got)
	}
	g := c.do("SESSIONX.GET", id)
	if len(g.Array) != 12 || string(g.Array[3].Bulk) != "alice" || len(g.Array[11].Array) != 2 || string(g.Array[11].Array[1].Bulk) != "3 items" {
		t.Fatalf("SESSIONX.GET after restart: %+v", g)
	}
	if got := c.do("PTTL", id); got.Int <= 0 {
		t.Fatalf("session lost its expiry: %v", got)
	}
	if got := c.do("SESSIONX.LIST"); len(got.Array) != 1 || string(got.Array[0].Bulk) != id {
		t.Fatalf("SESSIONX.LIST: %+v", got)
	}

	// A blocked SEM.ACQUIRE takes the permits once another client releases
	// them.
	done := make(chan int64, 1)
	go func() {
		w := dialRepl(t, s)
		done <- w.do("SEM.ACQUIRE", "pool", "3", "5000").Int
	}()
	time.Sleep(50 * time.Millisecond)
	if got := c.do("SEM.VALUE", "pool"); got.Array[5].Int != 1 {
		t.Fatalf("waiters while blocked: %+v", got)
	}
	ok("SEM.RELEASE", "pool", "2")
	select {
	case got := <-done:
		if got != 1 {
			t.Fatalf("blocked SEM.ACQUIRE: %d", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("SEM.ACQUIRE stayed blocked after the release")
	}
	if got := c.do("SEM.ACQUIRE", "pool", "1", "30"); got.Int != 0 {
		t.Fatalf("SEM.ACQUIRE with no permits: %v", got)
	}
	if got := c.do("SESSIONX.DELETE", id); got.Int != 1 {
		t.Fatalf("SESSIONX.DELETE: %v", got)
	}
}
