package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
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

// A command holding the whole write gate runs at one time too: a key alive
// when it started does not expire under it. RENAME a b looked b up again
// after moving a there, and a key expiring in between was deleted and its
// DEL recorded before the RENAME.
func TestExclusiveCommandsRunAtOneTime(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	c.do("SET", "b", "v", "PX", "20")
	ctx := command.NewContext("RENAME", nil, s.dbs.DB(0), nil)
	thaw := s.freezeCommandTime(ctx)
	time.Sleep(40 * time.Millisecond)
	if _, ok := s.dbs.DB(0).Peek("b"); !ok {
		thaw()
		t.Fatal("b expired while the command's time stood still")
	}
	thaw()
	if _, ok := s.dbs.DB(0).Peek("b"); ok {
		t.Fatal("b still there once the command's time went on")
	}
}

// ADDTAG and REMTAG on an expired key expire it, recording its DEL, and
// record nothing themselves: replayed where the key was still stored (a
// replica, the AOF), they gave it tags the master's key never had.
func TestTagChangeOnExpiredKeyRecordsItsExpiry(t *testing.T) {
	cfg := aofRewriteConfig(t, t.TempDir(), nil)
	master := startReplServerCfg(t, cfg)
	replica := startReplServer(t, nil)
	replicate(t, replica, master)
	c := dialRepl(t, master)
	c.do("SET", "k", "v", "PX", "30", "TAGS", "a")
	c.do("SET", "keep", "v", "TAGS", "a")
	time.Sleep(60 * time.Millisecond)
	if n := c.do("ADDTAG", "k", "b").Int; n != 0 {
		t.Fatalf("ADDTAG on an expired key = %d", n)
	}
	if n := c.do("REMTAG", "k", "a").Int; n != 0 {
		t.Fatalf("REMTAG on an expired key = %d", n)
	}
	c.do("ADDTAG", "keep", "a")
	c.do("REMTAG", "keep", "zz")
	waitSynced(t, replica, master)
	r := dialRepl(t, replica)
	if n := r.do("EXISTS", "k").Int; n != 0 {
		t.Fatal("the replica still has the expired key")
	}
	if got := str(r.do("TAGKEYS", "b")); got != "[]" {
		t.Fatalf("replica TAGKEYS b = %s", got)
	}
	stopReplServer(master)
	b, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	if aof := string(b); strings.Contains(aof, "ADDTAG") || strings.Contains(aof, "REMTAG") {
		t.Fatalf("an ADDTAG or REMTAG that changed nothing was recorded:\n%q", aof)
	} else if !strings.Contains(aof, "DEL\r\n$1\r\nk\r\n") {
		t.Fatalf("the expired key's DEL was not recorded:\n%q", aof)
	}
}
