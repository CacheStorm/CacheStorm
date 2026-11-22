package server

import (
	"strings"
	"testing"
	"time"
)

// frames reads n pub/sub messages as "channel payload" pairs.
func frames(t *testing.T, c *replClient, n int) []string {
	t.Helper()
	var out []string
	for len(out) < n {
		v, ok := c.next(3 * time.Second)
		if !ok {
			t.Fatalf("got %d frames %q, want %d", len(out), out, n)
		}
		if len(v.Array) != 4 || string(v.Array[0].Bulk) != "pmessage" {
			t.Fatalf("unexpected frame %+v", v)
		}
		out = append(out, string(v.Array[2].Bulk)+" "+string(v.Array[3].Bulk))
	}
	return out
}

// Redis 8.10's "o" and "c" classes: a key replaced wholesale announces
// "overwritten", and "type_changed" when its type differs.
func TestKeyspaceOverwrittenAndTypeChanged(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	if got := c.str("CONFIG", "SET", "notify-keyspace-events", "Eoc"); got != "OK" {
		t.Fatalf("CONFIG SET: %s", got)
	}
	if got := c.str("CONFIG", "GET", "notify-keyspace-events"); got != "[notify-keyspace-events ocE]" {
		t.Fatalf("CONFIG GET: %s", got)
	}
	c.do("CONFIG", "SET", "notify-keyspace-events", "AEoc")
	if got := c.str("CONFIG", "GET", "notify-keyspace-events"); got != "[notify-keyspace-events AE]" {
		t.Fatalf("A hides o and c in CONFIG GET? %s", got)
	}
	c.do("CONFIG", "SET", "notify-keyspace-events", "Egoc$")
	sub := dialRepl(t, s)
	events := keyEvents(t, sub)

	c.do("SET", "s", "1")
	sameEvents(t, events(1), "set s")
	c.do("SET", "s", "2")
	sameEvents(t, events(2), "overwritten s", "set s")
	c.do("RPUSH", "l", "x")
	c.do("SET", "l", "v")
	sameEvents(t, events(3), "overwritten l", "type_changed l", "set l")
	c.do("MSET", "m", "1", "m", "2", "s", "3")
	sameEvents(t, events(5), "set m", "overwritten m", "set m", "overwritten s", "set s")
	c.do("GETSET", "s", "4")
	sameEvents(t, events(2), "overwritten s", "set s")
	c.do("RENAME", "m", "s")
	sameEvents(t, events(3), "rename_from m", "rename_to s", "overwritten s")
	c.do("SADD", "set", "a")
	c.do("COPY", "set", "s", "REPLACE")
	sameEvents(t, events(3), "copy_to s", "overwritten s", "type_changed s")
	dump := c.do("DUMP", "l")
	c.do("RESTORE", "s", "0", string(dump.Bulk), "REPLACE")
	sameEvents(t, events(3), "restore s", "overwritten s", "type_changed s")
	c.do("SUNIONSTORE", "s", "set")
	sameEvents(t, events(2), "overwritten s", "type_changed s")
	// A write in place is no overwrite.
	c.do("APPEND", "l", "x")
	c.do("INCR", "n")
	c.do("INCR", "n")
	sameEvents(t, events(3), "append l", "incrby n", "incrby n")
}

// Redis 8.10's subkey channels carry the hash fields an event touched.
func TestKeyspaceSubkeyChannels(t *testing.T) {
	s := startReplServer(t, nil)
	c := dialRepl(t, s)
	if got := c.str("CONFIG", "SET", "notify-keyspace-events", "hSTIV"); got != "OK" {
		t.Fatalf("CONFIG SET: %s", got)
	}
	if got := c.str("CONFIG", "GET", "notify-keyspace-events"); got != "[notify-keyspace-events hSTIV]" {
		t.Fatalf("CONFIG GET: %s", got)
	}
	sub := dialRepl(t, s)
	if got := str(sub.do("PSUBSCRIBE", "__sub*")); !strings.HasPrefix(got, "[psubscribe ") {
		t.Fatalf("PSUBSCRIBE: %s", got)
	}
	c.do("HSET", "h", "f1", "a", "field2", "b")
	got := frames(t, sub, 5)
	want := []string{
		"__subkeyspace@0__:h hset|2:f1,6:field2",
		"__subkeyevent@0__:hset 1:h|2:f1,6:field2",
		"__subkeyspaceitem@0__:h\nf1 hset",
		"__subkeyspaceitem@0__:h\nfield2 hset",
		"__subkeyspaceevent@0__:hset|h 2:f1,6:field2",
	}
	if strings.Join(got, "#") != strings.Join(want, "#") {
		t.Fatalf("frames\n%q\nwant\n%q", got, want)
	}
	c.do("CONFIG", "SET", "notify-keyspace-events", "hT")
	c.do("HDEL", "h", "nope", "field2")
	c.do("HINCRBY", "h", "n", "1")
	c.do("HEXPIRE", "h", "100", "FIELDS", "2", "f1", "n")
	c.do("HPERSIST", "h", "FIELDS", "1", "n")
	c.do("HGETDEL", "h", "FIELDS", "1", "f1")
	got = frames(t, sub, 5)
	want = []string{
		"__subkeyevent@0__:hdel 1:h|6:field2",
		"__subkeyevent@0__:hincrby 1:h|1:n",
		"__subkeyevent@0__:hexpire 1:h|2:f1,1:n",
		"__subkeyevent@0__:hpersist 1:h|1:n",
		"__subkeyevent@0__:hdel 1:h|2:f1",
	}
	if strings.Join(got, "#") != strings.Join(want, "#") {
		t.Fatalf("frames\n%q\nwant\n%q", got, want)
	}
	// Expired fields are named too.
	c.do("HSET", "e", "x", "1", "y", "2")
	c.do("HPEXPIRE", "e", "1", "FIELDS", "1", "x")
	got = frames(t, sub, 2)
	if got[1] != "__subkeyevent@0__:hexpire 1:e|1:x" {
		t.Fatalf("frames %q", got)
	}
	got = frames(t, sub, 1)
	if got[0] != "__subkeyevent@0__:hexpired 1:e|1:x" {
		t.Fatalf("frames %q", got)
	}
	// Events without subkeys reach no subkey channel.
	c.do("DEL", "e")
	noFrame(t, sub)
}
