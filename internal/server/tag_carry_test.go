package server

import (
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// In-place writes change a value without replacing the key, so they keep
// its tags as they keep its TTL; SET and GETSET replace it.
func TestTagsSurviveInPlaceWritesX(t *testing.T) {
	c := dialRepl(t, startReplServer(t, nil))
	for _, cmd := range [][]string{
		{"APPEND", "k", "x"}, {"SETRANGE", "k", "0", "Z"}, {"SETBIT", "k", "3", "1"},
		{"INCR", "k"}, {"INCRBY", "k", "2"}, {"DECR", "k"}, {"DECRBY", "k", "2"}, {"INCRBYFLOAT", "k", "1.5"},
		{"EXPIRE", "k", "100"}, {"GETEX", "k", "EX", "100"},
	} {
		c.do("DEL", "k")
		c.do("SETTAG", "k", "10", "t")
		if got := c.do(cmd...); got.Type == resp.TypeError {
			t.Fatalf("%v: %s", cmd, got.Err)
		}
		if got := c.str("TAGKEYS", "t"); got != "[k]" {
			t.Fatalf("after %v TAGKEYS t = %s, want [k]", cmd, got)
		}
		if got := c.str("TAGS", "k"); got != "[t]" {
			t.Fatalf("after %v TAGS k = %s, want [t]", cmd, got)
		}
	}
	for _, cmd := range [][]string{{"SET", "k", "v"}, {"GETSET", "k", "v"}} {
		c.do("SETTAG", "k", "10", "t")
		c.do(cmd...)
		if got := c.str("TAGS", "k"); got != "[]" {
			t.Fatalf("after %v TAGS k = %s, want []", cmd, got)
		}
	}
}

func TestRestoreTagsX(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, nil)
	rc := replicate(t, replica, master)
	c := dialRepl(t, master)
	c.do("SET", "src", "v")
	payload := c.do("DUMP", "src")
	if got := c.str("RESTORE", "k", "0", string(payload.Bulk), "TAGS", "2", "a", "b c"); got != "OK" {
		t.Fatalf("RESTORE TAGS = %s", got)
	}
	if got := c.str("TAGS", "k"); got != "[a b c]" {
		t.Fatalf("TAGS k = %s", got)
	}
	for _, bad := range [][]string{{"TAGS"}, {"TAGS", "3", "a"}, {"TAGS", "-1"}, {"TAGS", "1", "a", "TAGS", "1", "b"}} {
		args := append([]string{"RESTORE", "x", "0", string(payload.Bulk)}, bad...)
		if got := c.do(args...); got.Type != resp.TypeError {
			t.Fatalf("RESTORE %v = %s, want an error", bad, str(got))
		}
	}
	waitSynced(t, replica, master)
	if got := rc.str("TAGKEYS", "b c"); got != "[k]" {
		t.Fatalf("replica TAGKEYS = %s", got)
	}
}

func TestMigrateCarriesTagsX(t *testing.T) {
	src := startReplServer(t, nil)
	dst := startReplServer(t, nil)
	c, d := dialRepl(t, src), dialRepl(t, dst)
	port := strconv.Itoa(dst.testPort())
	c.do("SETTAG", "k1", "v", "t", "u")
	c.do("SET", "plain", "v")
	if got := c.str("MIGRATE", "127.0.0.1", port, "k1", "0", "5000"); got != "OK" {
		t.Fatalf("MIGRATE = %s", got)
	}
	// The second MIGRATE reuses the connection, whose answer is cached.
	c.do("SETTAG", "k2", "v", "t")
	if got := c.str("MIGRATE", "127.0.0.1", port, "", "0", "5000", "KEYS", "k2", "plain"); got != "OK" {
		t.Fatalf("MIGRATE KEYS = %s", got)
	}
	if got := d.str("TAGKEYS", "t"); got != "[k1 k2]" && got != "[k2 k1]" {
		t.Fatalf("destination TAGKEYS t = %s", got)
	}
	if got := d.str("TAGS", "k1"); got != "[t u]" {
		t.Fatalf("destination TAGS k1 = %s", got)
	}
	if got := d.str("GET", "plain"); got != "v" {
		t.Fatalf("destination GET plain = %s", got)
	}
	// The probe left nothing behind.
	if got := d.str("DBSIZE"); got != "3" {
		t.Fatalf("destination DBSIZE = %s, want 3", got)
	}
}

func TestInvalidateAnnouncesDeletesX(t *testing.T) {
	s := startReplServer(t, nil)
	b, sub := dialRepl(t, s), dialRepl(t, s)
	b.do("SETTAG", "k1", "v", "t")
	b.do("SETTAG", "k2", "v", "t")
	b.do("CONFIG", "SET", "notify-keyspace-events", "Eg")
	sub.do("SUBSCRIBE", "__keyevent@0__:del")
	if got := b.do("INVALIDATE", "t"); got.Int != 2 {
		t.Fatalf("INVALIDATE = %s", str(got))
	}
	var keys []string
	for len(keys) < 2 {
		v, ok := sub.next(3 * time.Second)
		if !ok {
			t.Fatalf("got del events for %v only", keys)
		}
		keys = append(keys, str(v.Array[2]))
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != "k1,k2" {
		t.Fatalf("del events for %v", keys)
	}
}
