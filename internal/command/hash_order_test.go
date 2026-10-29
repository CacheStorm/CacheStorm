package command

import (
	"strings"
	"testing"
)

// TestHashOrderMatchesRedis pins the field order Redis 7.4 lists for small
// hashes: insertion order, fields with an expiry first by expiry, ties placed
// before, a field whose expiry is removed or overwritten moved last.
func TestHashOrderMatchesRedis(t *testing.T) {
	h := newLuaHarness(t)
	RegisterDebugCommands(h.router)
	keys := func(key string) string {
		t.Helper()
		got := h.do("HKEYS", key)
		var out []string
		for _, line := range strings.Split(got, "\r\n") {
			if line != "" && line[0] != '*' && line[0] != '$' {
				out = append(out, line)
			}
		}
		return strings.Join(out, " ")
	}
	h.do("HSET", "h", "a", "1", "b", "2", "c", "3", "d", "4")
	for _, step := range []struct {
		args []string
		want string
	}{
		{[]string{"HPEXPIREAT", "h", "9000000000000", "FIELDS", "1", "c"}, "c a b d"},
		{[]string{"HPEXPIREAT", "h", "8000000000000", "FIELDS", "1", "d"}, "d c a b"},
		{[]string{"HPERSIST", "h", "FIELDS", "1", "d"}, "c a b d"},
		{[]string{"HSET", "h", "e", "5"}, "c a b d e"},
		{[]string{"HSET", "h", "c", "9"}, "a b d e c"},
		{[]string{"HPEXPIREAT", "h", "9000000000000", "FIELDS", "1", "a"}, "a b d e c"},
		{[]string{"HINCRBY", "h", "a", "1"}, "a b d e c"},
		{[]string{"HPEXPIREAT", "h", "9000000000000", "FIELDS", "1", "b"}, "b a d e c"},
		{[]string{"HPEXPIREAT", "h", "9000000000001", "FIELDS", "1", "e"}, "b a e d c"},
		{[]string{"HDEL", "h", "a"}, "b e d c"},
	} {
		h.do(step.args...)
		if got := keys("h"); got != step.want {
			t.Fatalf("after %v: %q, want %q", step.args, got, step.want)
		}
	}
	h.do("HSET", "h2", "x", "1", "y", "2", "x", "3", "z", "4")
	if got := keys("h2"); got != "x y z" {
		t.Fatalf("duplicate field in HSET: %q", got)
	}
	h.do("HSET", "h3", "a", "1", "b", "2", "c", "3")
	h.do("HPEXPIREAT", "h3", "9000000000000", "FIELDS", "1", "a")
	h.do("HSET", "h3", "a", "7", "q", "1", "a", "8")
	if got := keys("h3"); got != "b c a q" {
		t.Fatalf("HSET over a field with an expiry: %q", got)
	}
	dump := h.do("DUMP", "h3")
	payload := strings.SplitN(dump, "\r\n", 2)[1]
	payload = payload[:len(payload)-2]
	h.do("RESTORE", "h4", "0", payload)
	if got := keys("h4"); got != "b c a q" {
		t.Fatalf("DUMP and RESTORE changed the order: %q", got)
	}
	h.do("HSET", "big", "f", strings.Repeat("x", 65), "g", "1")
	h.do("HSET", "big", "f", "1")
	if got := h.do("OBJECT", "ENCODING", "big"); got != "$9\r\nhashtable\r\n" {
		t.Fatalf("a hash that held a long value stays a hash table: %q", got)
	}
}
