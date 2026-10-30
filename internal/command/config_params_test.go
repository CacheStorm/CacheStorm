package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// TestConfigParamsMatchRedis pins CONFIG GET and SET for the parameters kept
// only for Redis compatibility: defaults, aliases, Redis' validation errors,
// and the ones that drive CacheStorm.
func TestConfigParamsMatchRedis(t *testing.T) {
	h := newLuaHarness(t)
	RegisterConfigCommands(h.router)
	RegisterSetCommands(h.router)
	RegisterDebugCommands(h.router)
	saved := globalConfig.params
	t.Cleanup(func() {
		h.do("CONFIG", "SET", "set-max-listpack-entries", "128", "hll-sparse-max-bytes", "3000", "acllog-max-len", "128", "client-output-buffer-limit", defaultOutputBufferLimit)
		globalConfig.mu.Lock()
		globalConfig.params = saved
		globalConfig.mu.Unlock()
	})
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"CONFIG", "GET", "hz"}, "10"},
		{[]string{"CONFIG", "GET", "slave-priority"}, "100"},
		{[]string{"CONFIG", "SET", "hz", "1000"}, "+OK"},
		{[]string{"CONFIG", "GET", "hz"}, "500"},
		{[]string{"CONFIG", "SET", "slave-priority", "x"}, "argument 'slave-priority') - argument couldn't be parsed into an integer"},
		{[]string{"CONFIG", "SET", "replica-priority", "7"}, "+OK"},
		{[]string{"CONFIG", "GET", "slave-priority"}, "7"},
		{[]string{"CONFIG", "SET", "repl-diskless-load", "x"}, "must be one of the following: disabled, on-empty-db, swapdb"},
		{[]string{"CONFIG", "SET", "io-threads", "4"}, "can't set immutable config"},
		{[]string{"CONFIG", "SET", "proto-max-bulk-len", "5"}, "argument must be between 1048576 and 9223372036854775807 inclusive"},
		{[]string{"CONFIG", "SET", "active-defrag-ignore-bytes", "2kb"}, "+OK"},
		{[]string{"CONFIG", "GET", "active-defrag-ignore-bytes"}, "2048"},
		{[]string{"CONFIG", "SET", "client-output-buffer-limit", "pubsub 1mb 2kb 3"}, "+OK"},
		{[]string{"CONFIG", "GET", "client-output-buffer-limit"}, "normal 0 0 0 slave 268435456 67108864 60 pubsub 1048576 2048 3"},
		{[]string{"CONFIG", "SET", "shutdown-on-sigint", "save nosave"}, "SAVE and NOSAVE can't be used simultaneously"},
		{[]string{"CONFIG", "SET", "maxmemory-clients", "10%"}, "+OK"},
		{[]string{"CONFIG", "GET", "maxmemory-clients"}, "10%"},
		{[]string{"CONFIG", "SET", "acllog-max-len", "1"}, "+OK"},
		{[]string{"CONFIG", "SET", "set-max-listpack-entries", "2"}, "+OK"},
		{[]string{"SADD", "s", "a", "b", "c"}, ":3"},
		{[]string{"OBJECT", "ENCODING", "s"}, "hashtable"},
	} {
		if got := h.do(c.args...); !strings.Contains(got, c.want) {
			t.Errorf("%v: %q, want %q", c.args, got, c.want)
		}
	}
	if aclLogMaxLenValue.Load() != 1 {
		t.Error("acllog-max-len not applied")
	}
	if got := h.do("CONFIG", "GET", "*"); strings.Count(got, "\r\n$") < 2*len(configParams) {
		t.Errorf("CONFIG GET * lists %d bulk strings", strings.Count(got, "\r\n$"))
	}
}

func TestConfigOutputBufferLimitDrivesSubscribers(t *testing.T) {
	h := newLuaHarness(t)
	RegisterConfigCommands(h.router)
	t.Cleanup(func() { h.do("CONFIG", "SET", "client-output-buffer-limit", defaultOutputBufferLimit) })
	if got := h.do("CONFIG", "SET", "client-output-buffer-limit", "pubsub 1000 0 0"); got != "+OK\r\n" {
		t.Fatalf("CONFIG SET = %q", got)
	}
	if _, hard := store.SetSubscriberLimits(0, 0); hard != 1000 {
		t.Fatalf("subscriber hard limit = %d, want 1000", hard)
	}
	sub := store.NewSubscriber(1)
	if sub.Send(make([]byte, 1001)) || !sub.IsOverflowed() {
		t.Fatal("frame past the configured pubsub hard limit accepted")
	}
}
