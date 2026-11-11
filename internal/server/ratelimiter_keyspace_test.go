package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A RATELIMITER is a key: its recorded requests survive a restart, each
// request adds a few bytes to the AOF whatever the limit, and RATELIMITER.WAIT
// waits for the oldest request to leave the window.
func TestRateLimitersAreKeys(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	try := func(want int64) {
		t.Helper()
		got := c.do("RATELIMITER.TRY", "api")
		if len(got.Array) != 3 || got.Array[0].Int != want {
			t.Fatalf("RATELIMITER.TRY: %+v, want allowed=%d", got, want)
		}
	}
	if got := c.do("RATELIMITER.CREATE", "api", "3", "60000"); got.Str != "OK" {
		t.Fatalf("CREATE: %+v", got)
	}
	path := filepath.Join(cfg.Persistence.DataDir, "appendonly.aof")
	before, _ := os.Stat(path)
	try(1)
	try(1)
	if got := c.do("RATELIMITER.TRY", "api"); got.Array[1].Int != 0 {
		t.Fatalf("remaining after the third request: %+v", got)
	}
	try(0)
	after, _ := os.Stat(path)
	if grown := after.Size() - before.Size(); grown > 3*120 {
		t.Fatalf("three requests grew the AOF by %d bytes", grown)
	}
	c.do("SET", "plain", "v")
	if got := c.do("RATELIMITER.TRY", "plain"); got.Type != '-' || !strings.HasPrefix(got.Err, "WRONGTYPE") {
		t.Fatalf("TRY on a string: %+v", got)
	}
	if got := c.do("RATELIMITER.TRY", "missing"); got.Type != '-' {
		t.Fatalf("TRY on a missing limiter: %+v", got)
	}
	stopReplServer(s)

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	try(0)
	if got := c.do("RATELIMITER.STATUS", "api"); len(got.Array) != 8 || got.Array[5].Int != 3 {
		t.Fatalf("STATUS after restart: %+v", got)
	}
	if got := c.do("RATELIMITER.RESET", "api"); got.Str != "OK" {
		t.Fatalf("RESET: %+v", got)
	}
	try(1)

	// WAIT takes the slot the oldest request frees.
	c.do("RATELIMITER.CREATE", "slow", "1", "150")
	c.do("RATELIMITER.TRY", "slow")
	start := time.Now()
	if got := c.do("RATELIMITER.WAIT", "slow", "2000"); got.Type != ':' {
		t.Fatalf("WAIT: %+v", got)
	}
	if d := time.Since(start); d < 100*time.Millisecond || d > 1500*time.Millisecond {
		t.Fatalf("WAIT returned after %v", d)
	}
	if got := c.do("RATELIMITER.WAIT", "slow", "20"); !got.IsNull {
		t.Fatalf("WAIT past its timeout: %+v", got)
	}
	if got := c.do("RATELIMITER.DELETE", "slow"); got.Int != 1 {
		t.Fatalf("DELETE: %+v", got)
	}
	if got := c.do("EXISTS", "slow"); got.Int != 0 {
		t.Fatal("DELETE left the key")
	}
}
