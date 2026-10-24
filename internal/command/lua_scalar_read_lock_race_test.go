package command

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: seven Lua script bindings read guarded collections with NO lock.
//
//	internal/command/lua.go
//	  HGET/HMGET/HEXISTS/HLEN  read hv.Fields     lockcalls=0
//	  SISMEMBER/SCARD          read sv.Members    lockcalls=0
//	  ZSCORE                   read zset.Members  lockcalls=0
//
// while cmdHSET/cmdHDEL, cmdSADD/cmdSREM/cmdSMOVE and cmdZADD/cmdZREM mutate the
// very same maps under their value write lock. A concurrent map read and map
// write is a PROCESS-FATAL runtime error reachable from any client via EVAL.
//
// This is the scalar-lookup half of the Lua access path; the map ITERATORS and
// the Lua WRITERS were fixed in earlier rounds, these single-key lookups were
// the remainder.
//
// HMGET takes the read lock across its whole lookup loop; the others read a
// single value under the lock. No Store call happens inside any of them, so
// none can self-deadlock.

func luaScalarTry(s *store.Store, router *Router, cmd string, args ...string) string {
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
		return "ERR:" + err.Error()
	}
	return buf.String()
}

func luaScalarRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
	t.Helper()
	out := luaScalarTry(s, router, cmd, args...)
	if len(out) > 4 && out[:4] == "ERR:" {
		t.Fatalf("%s: Execute returned error: %s", cmd, out)
	}
	return out
}

// luaScalarRouter rebinds the package-global scriptEngine to this test's store;
// otherwise a later EVAL would silently run against a previous test's store.
func luaScalarRouter(t *testing.T, s *store.Store) *Router {
	t.Helper()
	router := NewRouter()
	RegisterHashCommands(router)
	RegisterSetCommands(router)
	RegisterSortedSetCommands(router)
	RegisterStringCommands(router)
	RegisterListCommands(router)
	RegisterScriptCommands(router)
	// scriptEngine is a package global lazily bound to the FIRST store it sees.
	// Save and restore it, or a later script test would silently drive this
	// test's discarded store.
	savedEngine := scriptEngine
	scriptEngine = nil
	t.Cleanup(func() { scriptEngine = savedEngine })
	InitScriptEngine(NewScriptEngine(s))
	return router
}

func luaScalarSeed(t *testing.T, s *store.Store, router *Router, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		luaScalarRun(t, s, router, "HSET", "h", fmt.Sprintf("f%03d", i), "v")
		luaScalarRun(t, s, router, "SADD", "st", fmt.Sprintf("m%03d", i))
		luaScalarRun(t, s, router, "ZADD", "z", fmt.Sprintf("%d", i), fmt.Sprintf("zm%03d", i))
	}
}

// TestLuaScalarHashReadsAreRaceFree covers HGET/HMGET/HEXISTS against cmdHSET.
func TestLuaScalarHashReadsAreRaceFree(t *testing.T) {
	s := store.NewStore()
	router := luaScalarRouter(t, s)
	luaScalarSeed(t, s, router, 200)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			luaScalarTry(s, router, "HSET", "h", fmt.Sprintf("f%03d", i%200), "w")
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			luaScalarTry(s, router, "EVAL", "return redis.call('HGET', redis.KEYS[1], redis.ARGV[1])", "1", "h", "f001")
			luaScalarTry(s, router, "EVAL", "return redis.call('HMGET', redis.KEYS[1], redis.ARGV[1])", "1", "h", "f001")
			luaScalarTry(s, router, "EVAL", "return redis.call('HEXISTS', redis.KEYS[1], redis.ARGV[1])", "1", "h", "f001")
		}
	}()

	wg.Wait()
}

// TestLuaScalarSetReadsAreRaceFree covers SISMEMBER against cmdSADD/cmdSREM.
func TestLuaScalarSetReadsAreRaceFree(t *testing.T) {
	s := store.NewStore()
	router := luaScalarRouter(t, s)
	luaScalarSeed(t, s, router, 200)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			luaScalarTry(s, router, "SADD", "st", fmt.Sprintf("m%03d", i%200))
			luaScalarTry(s, router, "SREM", "st", fmt.Sprintf("m%03d", (i+1)%200))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			luaScalarTry(s, router, "EVAL", "return redis.call('SISMEMBER', redis.KEYS[1], redis.ARGV[1])", "1", "st", "m001")
		}
	}()

	wg.Wait()
}

// TestLuaScalarSortedSetReadIsRaceFree covers ZSCORE against cmdZADD.
func TestLuaScalarSortedSetReadIsRaceFree(t *testing.T) {
	s := store.NewStore()
	router := luaScalarRouter(t, s)
	luaScalarSeed(t, s, router, 200)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			luaScalarTry(s, router, "ZADD", "z", "1", fmt.Sprintf("zm%03d", i%200))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			luaScalarTry(s, router, "EVAL", "return redis.call('ZSCORE', redis.KEYS[1], redis.ARGV[1])", "1", "z", "zm001")
		}
	}()

	wg.Wait()
}

// TestLuaScalarControlsUncontended is the control: against collections nobody
// mutates, every guarded binding must still return its documented reply. This
// proves the harness really reaches the Lua bindings, so a race FAIL is the
// defect and not a broken setup.
func TestLuaScalarControlsUncontended(t *testing.T) {
	s := store.NewStore()
	router := luaScalarRouter(t, s)
	luaScalarRun(t, s, router, "HSET", "h", "f1", "v")
	luaScalarRun(t, s, router, "SADD", "st", "m1")
	luaScalarRun(t, s, router, "ZADD", "z", "5", "zm1")

	cases := []struct {
		script string
		key    string
		arg    string
		want   string
	}{
		{"return redis.call('HGET', redis.KEYS[1], redis.ARGV[1])", "h", "f1", "$1\r\nv\r\n"},
		{"return redis.call('HMGET', redis.KEYS[1], redis.ARGV[1])", "h", "f1", "*1\r\n$1\r\nv\r\n"},
		{"return redis.call('HEXISTS', redis.KEYS[1], redis.ARGV[1])", "h", "f1", ":1\r\n"},
		{"return redis.call('HLEN', redis.KEYS[1])", "h", "", ":1\r\n"},
		{"return redis.call('SISMEMBER', redis.KEYS[1], redis.ARGV[1])", "st", "m1", ":1\r\n"},
		{"return redis.call('SCARD', redis.KEYS[1])", "st", "", ":1\r\n"},
		// ZSCORE's Lua binding returns an INTEGER reply, not a bulk string.
		{"return redis.call('ZSCORE', redis.KEYS[1], redis.ARGV[1])", "z", "zm1", ":5\r\n"},
	}
	for _, c := range cases {
		args := []string{c.script, "1", c.key}
		if c.arg != "" {
			args = append(args, c.arg)
		}
		if got := luaScalarRun(t, s, router, "EVAL", args...); got != c.want {
			t.Fatalf("Lua %q (key=%q arg=%q) = %q, want %q", c.script, c.key, c.arg, got, c.want)
		}
	}

	// Boundary: a missing field is a Lua nil, not an error.
	if got := luaScalarRun(t, s, router, "EVAL", "return redis.call('HGET', redis.KEYS[1], 'nope')", "1", "h"); got != "$-1\r\n" {
		t.Fatalf("HGET of a missing field = %q, want a null reply", got)
	}
	if got := luaScalarRun(t, s, router, "EVAL", "return redis.call('SISMEMBER', redis.KEYS[1], 'nope')", "1", "st"); got != ":0\r\n" {
		t.Fatalf("SISMEMBER of a missing member = %q, want :0", got)
	}
}
