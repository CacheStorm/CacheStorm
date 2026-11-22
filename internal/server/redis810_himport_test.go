package server

import (
	"context"
	"testing"
)

// Redis 8.10's HIMPORT: fieldsets prepared per connection, hashes created
// from their values.
func TestHImport(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	expect := func(c *replClient, want string, args ...string) {
		t.Helper()
		if got := c.str(args...); got != want {
			t.Fatalf("%v = %q, want %q", args, got, want)
		}
	}
	expect(c, "OK", "HIMPORT", "PREPARE", "fs", "name", "age", "id")
	expect(c, "ERR:ERR duplicate field name in fieldset", "HIMPORT", "PREPARE", "bad", "a", "b", "a")
	expect(c, "ERR:ERR wrong number of arguments for 'himport|prepare' command", "HIMPORT", "PREPARE", "x")
	expect(c, "ERR:ERR unknown subcommand 'BOGUS'. Try HIMPORT HELP.", "HIMPORT", "BOGUS")
	c.do("SET", "h", "x", "EX", "100")
	expect(c, "ERR:WRONGTYPE Operation against a key holding the wrong kind of value", "HIMPORT", "SET", "h", "fs", "1", "2", "3")
	c.do("DEL", "h")
	expect(c, "ERR:ERR no such fieldset", "HIMPORT", "SET", "h", "nofs", "1")
	expect(c, "ERR:ERR value count does not match fieldset field count", "HIMPORT", "SET", "h", "fs", "1", "2")
	expect(c, "OK", "HIMPORT", "SET", "h", "fs", "ann", "30", "7")
	// Fields in Redis' template order: shorter first, then bytewise.
	expect(c, "[id 7 age 30 name ann]", "HGETALL", "h")
	c.do("HSET", "h2", "extra", "1")
	c.do("EXPIRE", "h2", "100")
	expect(c, "OK", "HIMPORT", "SET", "h2", "fs", "bob", "40", "8")
	expect(c, "-1", "TTL", "h2")
	expect(c, "[id 8 age 40 name bob]", "HGETALL", "h2")

	// Fieldsets belong to the connection.
	other := dialRepl(t, s)
	expect(other, "ERR:ERR no such fieldset", "HIMPORT", "SET", "h3", "fs", "a", "b", "c")
	expect(other, "0", "HIMPORT", "DISCARD", "fs")
	expect(c, "OK", "HIMPORT", "PREPARE", "fs2", "z")
	expect(c, "1", "HIMPORT", "DISCARD", "fs2")
	expect(c, "0", "HIMPORT", "DISCARD", "fs2")
	c.do("HIMPORT", "PREPARE", "fs2", "z")
	expect(c, "2", "HIMPORT", "DISCARDALL")
	expect(c, "ERR:ERR no such fieldset", "HIMPORT", "SET", "h", "fs", "a", "b", "c")
	c.do("HIMPORT", "PREPARE", "fs", "a")
	c.do("RESET")
	expect(c, "ERR:ERR no such fieldset", "HIMPORT", "SET", "h", "fs", "a")

	// Inside MULTI the fieldset prepared before EXEC is used.
	c.do("MULTI")
	c.do("HIMPORT", "PREPARE", "m", "f")
	c.do("HIMPORT", "SET", "mk", "m", "v")
	expect(c, "[OK OK]", "EXEC")
	expect(c, "v", "HGET", "mk", "f")
}

// HIMPORT SET reaches replicas and the AOF as RESTORE ... REPLACE.
func TestHImportPersists(t *testing.T) {
	cfg := aofRewriteConfig(t, t.TempDir(), nil)
	master := startReplServerCfg(t, cfg)
	replica := startReplServer(t, nil)
	mc := dialRepl(t, master)
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	mc.do("HIMPORT", "PREPARE", "fs", "b", "a")
	mc.do("HSET", "k", "old", "1")
	mc.do("HIMPORT", "SET", "k", "fs", "B", "A")
	waitSynced(t, replica, master)
	want := "[a A b B]"
	if got := rc.str("HGETALL", "k"); got != want {
		t.Fatalf("replica = %s, want %s", got, want)
	}
	if got := rc.str("HIMPORT", "PREPARE", "fs", "x"); got != "OK" {
		t.Fatalf("PREPARE on a replica = %s", got)
	}
	if got := rc.str("HIMPORT", "SET", "k", "fs", "x"); got != "ERR:READONLY You can't write against a read only replica." {
		t.Fatalf("SET on a replica = %s", got)
	}
	stopReplServer(master)
	restarted, err := newOwnedTestServer(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := dialRepl(t, restarted).str("HGETALL", "k"); got != want {
		t.Fatalf("after restart = %s, want %s", got, want)
	}
}
