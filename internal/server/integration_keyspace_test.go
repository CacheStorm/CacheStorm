package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ARRAY, OBJECT, SEQUENCE, CAPTCHA and the CACHE locks are keys, and the
// CACHE bulk deletes are recorded as DEL: all survive a restart through an
// AOF holding only Redis commands.
func TestIntegrationNamespacesAreKeys(t *testing.T) {
	cfg := aofTxConfig(t)
	s := startToggleServer(t, cfg)
	c := dialRepl(t, s)
	ok := func(args ...string) string {
		t.Helper()
		v := c.do(args...)
		if v.Type == '-' {
			t.Fatalf("%v: %v", args, v.Err)
		}
		return string(v.Bulk)
	}
	ok("ARRAY.PUSH", "arr", "c", "a", "b", "a")
	ok("ARRAY.UNSHIFT", "arr", "x", "y")
	ok("ARRAY.UNIQUE", "arr")
	ok("ARRAY.SORT", "arr", "DESC")
	ok("ARRAY.SPLICE", "arr", "1", "1", "new")
	ok("ARRAY.PUSH", "arr2", "a", "z")
	ok("ARRAY.MERGE", "arr2", "arr")
	ok("OBJECT.FROMENTRIES", "obj", "a", "1", "b", "2", "c", "3")
	ok("OBJECT.SET", "obj", "d", "4")
	ok("OBJECT.OMIT", "obj", "a")
	ok("OBJECT.PICK", "obj", "b", "d")
	ok("SEQUENCE.NEXT", "seq")
	ok("SEQUENCE.NEXT", "seq", "10")
	ok("SEQUENCE.SET", "seq2", "100")
	if r := c.do("CACHE.LOCK", "lk", "alice", "3600000"); r.Int != 1 {
		t.Fatalf("CACHE.LOCK: %+v", r)
	}
	if r := c.do("CACHE.LOCK", "lk", "bob", "3600000"); r.Int != 0 {
		t.Fatalf("CACHE.LOCK by another holder: %+v", r)
	}
	capt := c.do("CAPTCHA.GENERATE", "4")
	if len(capt.Array) != 4 || len(capt.Array[3].Bulk) != 4 {
		t.Fatalf("CAPTCHA.GENERATE: %+v", capt)
	}
	capID, capAnswer := string(capt.Array[1].Bulk), string(capt.Array[3].Bulk)
	ok("SET", "tmp:1", "a")
	ok("SET", "tmp:2", "b")
	if r := c.do("CACHE.BULKDEL", "tmp:*"); r.Int != 2 {
		t.Fatalf("CACHE.BULKDEL: %+v", r)
	}

	ok("SET", "plain", "v")
	for _, args := range [][]string{{"ARRAY.PUSH", "plain", "x"}, {"OBJECT.SET", "plain", "a", "b"}, {"SEQUENCE.NEXT", "obj"},
		{"CACHE.LOCK", "plain", "h", "1000"}, {"ARRAY.REVERSE", "obj"}, {"OBJECT.KEYS", "arr"}} {
		if r := c.do(args...); r.Type != '-' || !strings.HasPrefix(r.Err, "WRONGTYPE") {
			t.Fatalf("%v: %v", args, r)
		}
	}
	for _, args := range [][]string{{"ARRAY.SLICE", "arr", "x", "1"}, {"ARRAY.SORT", "arr", "UP"}, {"SEQUENCE.SET", "seq", "abc"},
		{"CAPTCHA.GENERATE", "0"}, {"CACHE.LOCK", "lk2", "h", "0"}} {
		if r := c.do(args...); r.Type != '-' {
			t.Fatalf("%v accepted: %v", args, r)
		}
	}
	stopReplServer(s)

	data, err := os.ReadFile(filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ARRAY.", "OBJECT.", "SEQUENCE.", "CAPTCHA.", "CACHE."} {
		if strings.Contains(string(data), name) {
			t.Fatalf("the AOF records %s commands", name)
		}
	}

	s = startToggleServer(t, cfg)
	defer stopReplServer(s)
	c = dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if r := renderReply(c.do(args...), ","); r != want {
			t.Fatalf("%v after restart = %q, want %q", args, r, want)
		}
	}
	expect("y,new,c,b,a", "LRANGE", "arr", "0", "-1")
	expect("1", "ARRAY.INDEXOF", "arr", "new")
	expect("a,z,y,new,c,b,a", "ARRAY.SLICE", "arr2", "0", "10")
	expect("b,2,d,4", "OBJECT.ENTRIES", "obj")
	expect("2", "HGET", "obj", "b")
	expect("11", "SEQUENCE.CURRENT", "seq")
	expect("101", "SEQUENCE.NEXT", "seq2")
	expect("1", "CACHE.LOCKED", "lk")
	expect("0", "CACHE.LOCK", "lk", "bob", "1000")
	expect("1", "CACHE.UNLOCK", "lk", "alice")
	expect("0", "EXISTS", "tmp:1")
	expect("0", "CAPTCHA.VERIFY", capID, "wrong")
	expect("1", "CAPTCHA.VERIFY", capID, capAnswer)
	expect("0", "CAPTCHA.VERIFY", capID, capAnswer)
	if r := c.do("PTTL", "lk"); r.Int != -2 {
		t.Fatalf("PTTL of a released lock: %+v", r)
	}
	ok("CACHE.CLEAR", "*")
	expect("0", "DBSIZE")
}
