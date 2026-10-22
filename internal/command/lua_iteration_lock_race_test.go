package command

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: the Lua scripting engine iterated HashValue.Fields and
// SetValue.Members WITHOUT holding the value's read lock, while the command
// handlers (HSET, HDEL, SADD, SREM, SMOVE, ...) mutate the very same maps
// under their write lock.
//
// The affected bindings were the map ITERATORS:
//
//	case "HGETALL":  for k, v := range hv.Fields    (lua.go:347)
//	case "HKEYS":    for k := range hv.Fields        (lua.go:758)
//	case "HVALS":    for _, v := range hv.Fields    (lua.go:775)
//	case "SMEMBERS": for member := range sv.Members  (lua.go:792)
//
// A map iteration concurrent with a map write is a PROCESS-FATAL runtime error
// ("fatal error: concurrent map iteration and map write") — it does not merely
// return a wrong value, it takes the whole server down. The command-handler
// side of these same values was locked in an earlier round; the Lua engine is
// a second, independent access path that round never covered.
//
// Each binding now snapshots the map under RLock and builds the Lua table after
// releasing it, so the value lock is never held across a Lua call.

// luaRaceRouter registers the Lua bindings and the locked writers they race,
// and binds the package-global script engine to THIS test's store. cmdEVAL
// lazily binds scriptEngine to the first store it ever sees, so without an
// explicit rebind a later test would drive an earlier test's store and every
// race assertion below would be vacuous.
//
// The previous global is saved and restored in t.Cleanup: leaving it pointed
// at this test's discarded store would make a LATER script test silently
// operate on a dead store instead of its own.
func luaRaceRouter(t *testing.T, s *store.Store) *Router {
	t.Helper()
	saved := scriptEngine
	t.Cleanup(func() { scriptEngine = saved })
	scriptEngine = NewScriptEngine(s)
	router := NewRouter()
	RegisterHashCommands(router)
	RegisterSetCommands(router)
	RegisterScriptCommands(router)
	return router
}

func luaRaceSeedHash(t *testing.T, s *store.Store, router *Router, key string, n int) {
	t.Helper()
	fields := make([]string, 0, 2*n)
	for i := 0; i < n; i++ {
		fields = append(fields, fmt.Sprintf("seed%03d", i), fmt.Sprintf("v%03d", i))
	}
	luaRaceRun(t, s, router, "HSET", append([]string{key}, fields...)...)
}

func luaRaceSeedSet(t *testing.T, s *store.Store, router *Router, key string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		luaRaceRun(t, s, router, "SADD", key, fmt.Sprintf("seed%03d", i))
	}
}

// luaRaceRun drives a command through the real router. It returns the raw reply
// so callers can inspect the RESP shape.
func luaRaceRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
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

// luaRaceReply runs a command and parses the RESP reply, failing on setup
// problems (which are NOT the defect under test).
func luaRaceReply(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) *resp.Value {
	t.Helper()
	raw := luaRaceRun(t, s, router, cmd, args...)
	v, err := resp.NewReader(bytes.NewReader([]byte(raw))).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", cmd, raw, err)
	}
	return v
}

// luaRaceEval runs a script through EVAL. KEYS/ARGV live on the redis table
// (redis.KEYS), not as globals.
func luaRaceEval(t *testing.T, s *store.Store, router *Router, script string, key string) *resp.Value {
	t.Helper()
	return luaRaceReply(t, s, router, "EVAL", script, "1", key)
}

// TestLuaHGETALLIsRaceFreeWithHashWriters is the primary case: the Lua HGETALL
// iteration against the locked HSET writer on the same hash.
func TestLuaHGETALLIsRaceFreeWithHashWriters(t *testing.T) {
	s := store.NewStore()
	router := luaRaceRouter(t, s)
	luaRaceSeedHash(t, s, router, "h", 256)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			luaRaceRun(t, s, router, "HSET", "h", fmt.Sprintf("w%d", i%64), "x")
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			luaRaceEval(t, s, router, "return redis.call('HGETALL', redis.KEYS[1])", "h")
		}
	}()

	wg.Wait()
}

// TestLuaSMEMBERSIsRaceFreeWithSetWriters is the sibling set path, and the
// deleting writer SREM is the variant most likely to trip the runtime's
// map-corruption abort.
func TestLuaSMEMBERSIsRaceFreeWithSetWriters(t *testing.T) {
	s := store.NewStore()
	router := luaRaceRouter(t, s)
	luaRaceSeedSet(t, s, router, "st", 256)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			luaRaceRun(t, s, router, "SADD", "st", fmt.Sprintf("w%d", i%64))
			luaRaceRun(t, s, router, "SREM", "st", fmt.Sprintf("seed%03d", i%128))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			luaRaceEval(t, s, router, "return redis.call('SMEMBERS', redis.KEYS[1])", "st")
		}
	}()

	wg.Wait()
}

// TestLuaHKEYSAndHVALSAreRaceFreeWithHashWriters covers the two remaining
// iteration bindings on the hash.
func TestLuaHKEYSAndHVALSAreRaceFreeWithHashWriters(t *testing.T) {
	s := store.NewStore()
	router := luaRaceRouter(t, s)
	luaRaceSeedHash(t, s, router, "h", 256)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			luaRaceRun(t, s, router, "HSET", "h", fmt.Sprintf("w%d", i%64), "x")
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			luaRaceEval(t, s, router, "return redis.call('HKEYS', redis.KEYS[1])", "h")
			luaRaceEval(t, s, router, "return redis.call('HVALS', redis.KEYS[1])", "h")
		}
	}()

	wg.Wait()
}

// TestLuaIterationControlsUncontended is the control: against a collection
// nobody mutates, each binding must still return the correct element count.
// This proves the harness reaches real Lua behaviour, so a race failure above
// is the defect and not a broken setup.
func TestLuaIterationControlsUncontended(t *testing.T) {
	s := store.NewStore()
	router := luaRaceRouter(t, s)
	luaRaceSeedHash(t, s, router, "h", 4)
	luaRaceSeedSet(t, s, router, "st", 4)

	// HGETALL returns a flat [field, value, ...] table: 4 fields -> 8 entries.
	v := luaRaceEval(t, s, router, "return redis.call('HGETALL', redis.KEYS[1])", "h")
	if v.Type != resp.TypeArray || len(v.Array) != 8 {
		t.Fatalf("CONTROL FAIL: Lua HGETALL returned %d entries, want 8 (4 field/value pairs)", len(v.Array))
	}

	// HKEYS and HVALS return one entry per field.
	if v = luaRaceEval(t, s, router, "return redis.call('HKEYS', redis.KEYS[1])", "h"); len(v.Array) != 4 {
		t.Fatalf("CONTROL FAIL: Lua HKEYS returned %d entries, want 4", len(v.Array))
	}
	if v = luaRaceEval(t, s, router, "return redis.call('HVALS', redis.KEYS[1])", "h"); len(v.Array) != 4 {
		t.Fatalf("CONTROL FAIL: Lua HVALS returned %d entries, want 4", len(v.Array))
	}

	// SMEMBERS returns one entry per member.
	if v = luaRaceEval(t, s, router, "return redis.call('SMEMBERS', redis.KEYS[1])", "st"); len(v.Array) != 4 {
		t.Fatalf("CONTROL FAIL: Lua SMEMBERS returned %d entries, want 4", len(v.Array))
	}
}
