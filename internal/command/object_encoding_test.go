package command

import (
	"strings"
	"testing"
)

// TestObjectEncodingMatchesRedis pins OBJECT ENCODING where Redis' answer
// depends on how a value was built: strings changed in place are raw and
// INCRBYFLOAT's result is never an int, and a sorted set that became a
// skiplist stays one.
func TestObjectEncodingMatchesRedis(t *testing.T) {
	h := newLuaHarness(t)
	RegisterStringCommands(h.router)
	RegisterHyperLogLogCommands(h.router)
	RegisterSortedSetCommands(h.router)
	RegisterGeoCommands(h.router)
	RegisterKeyCommands(h.router)
	RegisterDebugCommands(h.router)
	long := strings.Repeat("q", 80)
	for _, c := range [][]string{
		{"APPEND", "new", "123"},
		{"SET", "app", "hi"}, {"APPEND", "app", "1"},
		{"SET", "rng", "hi"}, {"SETRANGE", "rng", "0", "x"},
		{"SET", "rng0", "hi"}, {"SETRANGE", "rng0", "0", ""},
		{"INCRBYFLOAT", "flt", "2"},
		{"PFADD", "hll", "a"},
		{"COPY", "app", "cp"},
		{"SET", "num", "5"}, {"APPEND", "num", "1"}, {"INCR", "num"},
		{"ZADD", "z", "1", "x", "1", long}, {"ZREM", "z", long},
		{"ZADD", "zx", "1", "x"}, {"ZADD", "zx", "XX", "1", long},
		{"GEOADD", "g", "1", "1", long}, {"GEOADD", "g", "1", "1", "b"}, {"ZREM", "g", long},
	} {
		h.do(c...)
	}
	for key, want := range map[string]string{
		"new": "int", "app": "raw", "rng": "raw", "rng0": "embstr", "flt": "embstr",
		"hll": "raw", "cp": "raw", "num": "int", "z": "skiplist", "zx": "listpack", "g": "skiplist",
	} {
		if got := h.do("OBJECT", "ENCODING", key); !strings.Contains(got, want+"\r\n") || strings.Contains(got, "embstr") != (want == "embstr") {
			t.Errorf("%s: %q, want %s", key, got, want)
		}
	}
	h.do("ZADD", "zd", "1", "x")
	args := []string{"ZADD", "zd"}
	for range 129 {
		args = append(args, "1", "x")
	}
	h.do(args...)
	if got := h.do("OBJECT", "ENCODING", "zd"); !strings.Contains(got, "skiplist") {
		t.Errorf("129 pairs on a listpack: %q", got)
	}
}
