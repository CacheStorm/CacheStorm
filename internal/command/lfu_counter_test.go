package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// TestLFUCounterFollowsRedis pins OBJECT FREQ under an LFU policy with
// lfu-log-factor 0, where every access counts: a new key starts at 5, a
// lookup adds one, TYPE and TTL do not count, an overwrite keeps the counter
// and a key that moves to a new name starts afresh, as in Redis 7.4.
func TestLFUCounterFollowsRedis(t *testing.T) {
	h := newLuaHarness(t)
	RegisterStringCommands(h.router)
	RegisterKeyCommands(h.router)
	RegisterDebugCommands(h.router)
	h.s.ConfigureMemory(0, store.EvictionAllKeysLFU, 70, 90, 5)
	store.SetLFUParams(0, 1)
	t.Cleanup(func() { store.SetLFUParams(10, 1) })
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"SET", "k", "v"}, ""},
		{[]string{"OBJECT", "FREQ", "k"}, ":5\r\n"},
		{[]string{"GET", "k"}, ""},
		{[]string{"GET", "k"}, ""},
		{[]string{"TYPE", "k"}, ""},
		{[]string{"TTL", "k"}, ""},
		{[]string{"OBJECT", "FREQ", "k"}, ":7\r\n"},
		{[]string{"SET", "k", "w"}, ""},
		{[]string{"OBJECT", "FREQ", "k"}, ":8\r\n"},
		{[]string{"APPEND", "k", "x"}, ""},
		{[]string{"OBJECT", "FREQ", "k"}, ":9\r\n"},
		{[]string{"RENAME", "k", "r"}, ""},
		{[]string{"OBJECT", "FREQ", "r"}, ":5\r\n"},
		{[]string{"OBJECT", "IDLETIME", "r"}, "-ERR An LFU maxmemory policy is selected, idle time not tracked."},
	} {
		got := h.do(c.args...)
		if c.want != "" && !strings.Contains(got, c.want) {
			t.Errorf("%v: %q, want %q", c.args, got, c.want)
		}
	}
}
