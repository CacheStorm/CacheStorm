package server

import (
	"testing"
	"time"
)

// A transaction or a script runs at one time, as in Redis: a key alive when
// it started does not expire halfway through it. When it did, the expiry's
// DEL was recorded before the transaction, so the AOF replayed the
// transaction's first commands without the key.
func TestTransactionsAndScriptsRunAtOneTime(t *testing.T) {
	const wait = `local t0 = redis.call('TIME')
local s0 = t0[1] * 1000000 + t0[2]
while true do
  local t = redis.call('TIME')
  if t[1] * 1000000 + t[2] - s0 > 60000 then break end
end`
	cfg := aofRewriteConfig(t, t.TempDir(), nil)
	master := startReplServerCfg(t, cfg)
	replica := startReplServer(t, nil)
	replicate(t, replica, master)
	c := dialRepl(t, master)

	c.do("RPUSH", "j", "a", "b")
	c.do("PEXPIRE", "j", "30")
	c.str("MULTI")
	c.do("LMOVE", "j", "x", "LEFT", "LEFT")
	c.do("DEBUG", "SLEEP", "0.06")
	c.do("RPUSH", "j", "y")
	if got := str(c.do("EXEC")); got != "[a OK 2]" {
		t.Fatalf("EXEC = %s, want [a OK 2] as Redis 8.2 answers", got)
	}

	c.do("RPUSH", "k", "a", "b")
	c.do("PEXPIRE", "k", "30")
	script := "redis.call('LMOVE', 'k', 'z', 'LEFT', 'LEFT')\n" + wait + "\nreturn redis.call('RPUSH', 'k', 'y')"
	if got := c.do("EVAL", script, "0").Int; got != 2 {
		t.Fatalf("EVAL = %d, want 2 as Redis 8.2 answers", got)
	}

	time.Sleep(50 * time.Millisecond)
	state := func(c *replClient) string {
		return str(c.do("LRANGE", "x", "0", "-1")) + str(c.do("LRANGE", "z", "0", "-1")) + str(c.do("EXISTS", "j", "k"))
	}
	want := "[a][a]0"
	if got := state(c); got != want {
		t.Fatalf("master: %s, want %s", got, want)
	}
	waitSynced(t, replica, master)
	if got := state(dialRepl(t, replica)); got != want {
		t.Fatalf("replica: %s, want %s", got, want)
	}
	stopReplServer(master)
	restarted := startReplServerCfg(t, cfg)
	if got := state(dialRepl(t, restarted)); got != want {
		t.Fatalf("after an AOF restart: %s, want %s", got, want)
	}
}
