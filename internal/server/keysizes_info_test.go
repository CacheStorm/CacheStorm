package server

import (
	"strings"
	"testing"
)

// INFO keysizes counts each database's strings by length and collections
// by element count in power-of-two bins, as Redis 8 does, following every
// way a key is written, emptied, moved, expired and flushed.
func TestInfoKeysizes(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	keysizes := func() string {
		info := str(c.do("INFO", "keysizes"))
		if !strings.HasPrefix(info, "# Keysizes\r\n") {
			t.Fatalf("INFO keysizes: %q", info)
		}
		return strings.TrimSpace(strings.TrimPrefix(info, "# Keysizes\r\n"))
	}
	expect := func(step, want string) {
		t.Helper()
		if got := keysizes(); got != strings.ReplaceAll(want, "\n", "\r\n") {
			t.Fatalf("%s:\n%s\nwant\n%s", step, got, want)
		}
	}
	expect("empty", "")
	c.str("SET", "s0", "")
	c.str("SET", "s1", "a")
	c.str("SET", "s5", "12345")
	c.do("INCRBY", "n", "123456789")
	c.do("SETBIT", "bm", "16383", "1")
	c.do("PFADD", "hll", "a", "b")
	expect("strings", "db0_distrib_strings_sizes:0=1,1=1,4=1,8=1,16=1,2K=1")

	c.do("APPEND", "s1", "bcd")
	c.do("RPUSH", "l", "a", "b", "c")
	c.do("SADD", "st", "1", "2")
	c.do("ZADD", "z", "1", "a")
	c.do("GEOADD", "g", "13.36", "38.11", "p", "13.37", "38.12", "q")
	c.do("HSET", "h", "f", "v", "g", "w", "x", "y", "z", "q")
	expect("collections", "db0_distrib_strings_sizes:0=1,4=2,8=1,16=1,2K=1\n"+
		"db0_distrib_lists_items:2=1\ndb0_distrib_sets_items:2=1\ndb0_distrib_zsets_items:1=1,2=1\ndb0_distrib_hashes_items:4=1")

	c.do("LPOP", "l", "3")
	c.do("SREM", "st", "1")
	c.do("ZADD", "z", "2", "b", "3", "c")
	c.do("HDEL", "h", "f")
	c.do("DEL", "s0", "bm", "hll", "g")
	c.do("RENAME", "n", "n2")
	expect("in place", "db0_distrib_strings_sizes:4=2,8=1\n"+
		"db0_distrib_sets_items:1=1\ndb0_distrib_zsets_items:2=1\ndb0_distrib_hashes_items:2=1")

	c.str("MULTI")
	c.do("SADD", "st", "2", "3", "4")
	c.do("EVAL", "redis.call('rpush', KEYS[1], 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h') return 1", "1", "l2")
	c.do("EXEC")
	c.do("MOVE", "z", "1")
	c.do("PEXPIRE", "s1", "1")
	waitFor(t, "s1 to expire", func() bool { return str(c.do("EXISTS", "s1")) == "0" })
	expect("multi, move and expiry", "db0_distrib_strings_sizes:4=1,8=1\n"+
		"db0_distrib_lists_items:8=1\ndb0_distrib_sets_items:2=1\ndb0_distrib_hashes_items:2=1\n"+
		"db1_distrib_zsets_items:2=1")

	c.str("SWAPDB", "0", "1")
	expect("swapdb", "db0_distrib_zsets_items:2=1\n"+
		"db1_distrib_strings_sizes:4=1,8=1\ndb1_distrib_lists_items:8=1\ndb1_distrib_sets_items:2=1\ndb1_distrib_hashes_items:2=1")
	c.str("FLUSHALL")
	expect("flushall", "")

	if all := str(c.do("INFO")); !strings.Contains(all, "# Keysizes\r\n") {
		t.Fatal("default INFO lacks the Keysizes section")
	}
}

// Redis 8's INFO fields and Threads section are all there.
func TestInfoRedis8Fields(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	info := str(c.do("INFO", "everything"))
	for _, want := range []string{
		"\r\nused_memory_peak_time:", "\r\nmem_replica_full_sync_buffer:0\r\n",
		"\r\nio_threaded_total_prefetch_batches:0\r\n", "\r\nio_threaded_total_prefetch_entries:0\r\n",
		"# Persistence\r\n", "# Threads\r\nio_thread_0:clients=", "# Stats\r\n",
	} {
		if !strings.Contains(info, want) {
			t.Fatalf("INFO lacks %q", want)
		}
	}
	if strings.Contains(info, "used_memory_peak_time:0\r\n") {
		t.Fatal("used_memory_peak_time is 0")
	}
	if strings.Index(info, "# Threads") > strings.Index(info, "# Stats") {
		t.Fatal("Threads comes before Stats")
	}
}
