package command

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: the Lua scripting engine MUTATED guarded collections with no lock,
// while the command handlers mutate the same maps under their write lock:
//
//	case "ZADD":  zset.Members[args[2]] = score      (lua.go)
//	case "ZREM":  delete(zset.Members, args[1])
//	case "SREM":  delete(sv.Members, args[i])
//	case "HMSET": hv.Fields[args[i]] = []byte(...)
//
// cmdZADD/cmdZREM/cmdSREM/cmdHSET mutate those exact maps under their locks.
// Concurrent map writes are undefined behaviour and can abort the process.
//
// An earlier round fixed the Lua *iterators* (HGETALL/HKEYS/HVALS/SMEMBERS);
// these are the Lua *writers*.

// luaWriteRouter binds the package-global script engine to THIS test's store.
// scriptEngine is lazily bound to the first store cmdEVAL ever sees, so without
// an explicit rebind a later test would drive an earlier test's store and every
// race assertion here would be vacuous.
func luaWriteRouter(t *testing.T, s *store.Store) *Router {
	t.Helper()
	saved := scriptEngine
	t.Cleanup(func() { scriptEngine = saved })
	scriptEngine = NewScriptEngine(s)

	router := NewRouter()
	RegisterSortedSetCommands(router)
	RegisterSetCommands(router)
	RegisterHashCommands(router)
	RegisterStringCommands(router)
	RegisterScriptCommands(router)
	return router
}

func luaWriteRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
	t.Helper()
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("%s: Execute returned error: %v", cmd, err)
	}
	return buf.String()
}

// luaWriteEval runs a script. KEYS/ARGV live on the redis table, not globals.
func luaWriteEval(t *testing.T, s *store.Store, router *Router, script, key string) string {
	t.Helper()
	return luaWriteRun(t, s, router, "EVAL", script, "1", key)
}

// luaWriteSeed pre-populates so Lua takes the existing-key branch that mutates
// the shared live value.
func luaWriteSeed(t *testing.T, s *store.Store, router *Router) {
	t.Helper()
	for i := 0; i < 128; i++ {
		luaWriteRun(t, s, router, "ZADD", "z", fmt.Sprintf("%d", i), fmt.Sprintf("seed%d", i))
		luaWriteRun(t, s, router, "SADD", "st", fmt.Sprintf("seed%d", i))
		luaWriteRun(t, s, router, "HSET", "h", fmt.Sprintf("f%d", i), "v")
	}
}

// TestLuaZADDWriteIsRaceFree is the primary case: the Lua ZADD write against the
// locked cmdZADD writer on the same sorted set.
func TestLuaZADDWriteIsRaceFree(t *testing.T) {
	s := store.NewStore()
	router := luaWriteRouter(t, s)
	luaWriteSeed(t, s, router)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			luaWriteRun(t, s, router, "ZADD", "z", fmt.Sprintf("%d", 10000+i), fmt.Sprintf("w%d", i%64))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			luaWriteEval(t, s, router, "return redis.call('ZADD', redis.KEYS[1], 1, 'lua')", "z")
		}
	}()

	wg.Wait()
}

// TestLuaZREMWriteIsRaceFree covers the sorted-set delete.
func TestLuaZREMWriteIsRaceFree(t *testing.T) {
	s := store.NewStore()
	router := luaWriteRouter(t, s)
	luaWriteSeed(t, s, router)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			luaWriteRun(t, s, router, "ZREM", "z", fmt.Sprintf("seed%d", i%64))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			luaWriteEval(t, s, router, "return redis.call('ZREM', redis.KEYS[1], 'lua')", "z")
		}
	}()

	wg.Wait()
}

// TestLuaSREMWriteIsRaceFree covers the set delete against both the additive and
// the deleting command writers.
func TestLuaSREMWriteIsRaceFree(t *testing.T) {
	s := store.NewStore()
	router := luaWriteRouter(t, s)
	luaWriteSeed(t, s, router)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			luaWriteRun(t, s, router, "SADD", "st", fmt.Sprintf("w%d", i%64))
			luaWriteRun(t, s, router, "SREM", "st", fmt.Sprintf("seed%d", i%64))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			luaWriteEval(t, s, router, "return redis.call('SREM', redis.KEYS[1], 'lua')", "st")
		}
	}()

	wg.Wait()
}

// TestLuaHMSETWriteIsRaceFree covers the hash writer.
func TestLuaHMSETWriteIsRaceFree(t *testing.T) {
	s := store.NewStore()
	router := luaWriteRouter(t, s)
	luaWriteSeed(t, s, router)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			luaWriteRun(t, s, router, "HSET", "h", fmt.Sprintf("w%d", i%64), "x")
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			luaWriteEval(t, s, router, "return redis.call('HMSET', redis.KEYS[1], 'lua', 'v')", "h")
		}
	}()

	wg.Wait()
}

// TestLuaWriteControlsUncontended is the control: against collections nobody
// mutates, each Lua writer must still return its documented reply and take
// effect. A race failure above is therefore the defect, not a broken harness.
func TestLuaWriteControlsUncontended(t *testing.T) {
	s := store.NewStore()
	router := luaWriteRouter(t, s)
	luaWriteSeed(t, s, router)

	// ZADD reports 1 for a new member and 0 for an existing one.
	if out := luaWriteEval(t, s, router, "return redis.call('ZADD', redis.KEYS[1], 5, 'fresh')", "z"); out != ":1\r\n" {
		t.Fatalf("CONTROL FAIL: Lua ZADD new member returned %q, want :1", out)
	}
	if out := luaWriteEval(t, s, router, "return redis.call('ZADD', redis.KEYS[1], 5, 'fresh')", "z"); out != ":0\r\n" {
		t.Fatalf("CONTROL FAIL: Lua ZADD existing member returned %q, want :0", out)
	}
	// The write must be observable through the command path.
	if out := luaWriteRun(t, s, router, "ZSCORE", "z", "fresh"); out != "$1\r\n5\r\n" {
		t.Fatalf("CONTROL FAIL: ZSCORE after Lua ZADD = %q, want 5", out)
	}

	// ZREM reports 1 when it removed the member, 0 otherwise.
	if out := luaWriteEval(t, s, router, "return redis.call('ZREM', redis.KEYS[1], 'fresh')", "z"); out != ":1\r\n" {
		t.Fatalf("CONTROL FAIL: Lua ZREM existing member returned %q, want :1", out)
	}
	if out := luaWriteEval(t, s, router, "return redis.call('ZREM', redis.KEYS[1], 'absent')", "z"); out != ":0\r\n" {
		t.Fatalf("CONTROL FAIL: Lua ZREM absent member returned %q, want :0", out)
	}

	// SREM counts what it removed.
	if out := luaWriteEval(t, s, router, "return redis.call('SREM', redis.KEYS[1], 'seed1', 'seed2')", "st"); out != ":2\r\n" {
		t.Fatalf("CONTROL FAIL: Lua SREM two members returned %q, want :2", out)
	}

	// HMSET's status reply reaches Lua as {ok='OK'}, which the script returns
	// as a +OK status, as in Redis.
	if out := luaWriteEval(t, s, router, "return redis.call('HMSET', redis.KEYS[1], 'lua', 'v')", "h"); out != "+OK\r\n" {
		t.Fatalf("CONTROL FAIL: Lua HMSET returned %q, want +OK", out)
	}
	if out := luaWriteRun(t, s, router, "HGET", "h", "lua"); out != "$1\r\nv\r\n" {
		t.Fatalf("CONTROL FAIL: HGET after Lua HMSET = %q, want v", out)
	}

	// HMSET must still create the key when it does not exist, which exercises
	// the branch that calls store.Set after releasing the value lock.
	fresh := store.NewStore()
	freshRouter := luaWriteRouter(t, fresh)
	if out := luaWriteEval(t, fresh, freshRouter, "return redis.call('HMSET', redis.KEYS[1], 'a', '1')", "newhash"); out != "+OK\r\n" {
		t.Fatalf("CONTROL FAIL: Lua HMSET creating a key returned %q", out)
	}
	if out := luaWriteRun(t, fresh, freshRouter, "HGET", "newhash", "a"); out != "$1\r\n1\r\n" {
		t.Fatalf("CONTROL FAIL: HGET after Lua HMSET create = %q, want 1", out)
	}
}
