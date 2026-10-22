package command

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: three Lua script bindings read ListValue.Elements with NO lock.
//
//	internal/command/lua.go
//	  LLEN     reads lv.Elements        lockcalls=0
//	  LRANGE   reads lv.Elements[i]     lockcalls=0
//	  LINDEX   reads lv.Elements[i]     lockcalls=0
//
// while cmdRPUSH/cmdLPUSH/cmdLPOP/cmdRPOP mutate that same slice under the list
// write lock. A brace-matched lock census over lua.go showed the siblings in
// the SAME file already locking (LPOP/RPOP 3 calls, LPUSH/RPUSH 2 calls), so
// only these three read paths were missing it.
//
// This is the slice-ELEMENT half of the Lua access path, and a different
// failure mode from the map races fixed earlier: an append under the write
// lock can reallocate or extend the slice while a script reads len() or an
// indexed element, so the script can observe a stale length or torn bytes.
//
// LRANGE copies its window out under the lock rather than keeping a sub-slice:
// cmdLPOP can shift the backing array in place, which would corrupt a
// sub-slice held across the unlock.

// luaSliceTry never calls t.Fatal, so it is safe from a spawned goroutine.
func luaSliceTry(s *store.Store, router *Router, cmd string, args ...string) string {
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

func luaSliceRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
	t.Helper()
	out := luaSliceTry(s, router, cmd, args...)
	if strings.HasPrefix(out, "ERR:") {
		t.Fatalf("%s: Execute returned error: %s", cmd, out)
	}
	return out
}

// luaSliceRouter rebinds the package-global scriptEngine to this test's store and
// restores it afterwards: it is lazily bound to the FIRST store it ever sees, so
// without this a later script test would drive a previous test's store.
func luaSliceRouter(t *testing.T, s *store.Store) *Router {
	t.Helper()
	router := NewRouter()
	RegisterListCommands(router)
	RegisterScriptCommands(router)
	savedEngine := scriptEngine
	scriptEngine = nil
	t.Cleanup(func() { scriptEngine = savedEngine })
	InitScriptEngine(NewScriptEngine(s))
	return router
}

func luaSliceSeed(t *testing.T, s *store.Store, router *Router, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		luaSliceRun(t, s, router, "RPUSH", "l", fmt.Sprintf("e%04d", i))
	}
}

// TestLuaLlenIsRaceFreeWithListWriters is the primary case: LLEN reads
// len(lv.Elements) while cmdRPUSH appends under the list write lock.
func TestLuaLlenIsRaceFreeWithListWriters(t *testing.T) {
	s := store.NewStore()
	router := luaSliceRouter(t, s)
	luaSliceSeed(t, s, router, 256)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			luaSliceTry(s, router, "RPUSH", "l", fmt.Sprintf("w%04d", i%128))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			luaSliceTry(s, router, "EVAL", "return redis.call('LLEN', redis.KEYS[1])", "1", "l")
		}
	}()

	wg.Wait()
}

// TestLuaLrangeIsRaceFreeWithListWriters covers LRANGE, which indexes elements
// by computed bounds while the slice is appended to and popped from.
func TestLuaLrangeIsRaceFreeWithListWriters(t *testing.T) {
	s := store.NewStore()
	router := luaSliceRouter(t, s)
	luaSliceSeed(t, s, router, 256)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			luaSliceTry(s, router, "RPUSH", "l", fmt.Sprintf("w%04d", i%128))
			luaSliceTry(s, router, "LPOP", "l")
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			luaSliceTry(s, router, "EVAL", "return redis.call('LRANGE', redis.KEYS[1], 0, -1)", "1", "l")
		}
	}()

	wg.Wait()
}

// TestLuaLindexIsRaceFreeWithListWriters covers LINDEX.
func TestLuaLindexIsRaceFreeWithListWriters(t *testing.T) {
	s := store.NewStore()
	router := luaSliceRouter(t, s)
	luaSliceSeed(t, s, router, 256)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			luaSliceTry(s, router, "RPUSH", "l", fmt.Sprintf("w%04d", i%128))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			luaSliceTry(s, router, "EVAL", "return redis.call('LINDEX', redis.KEYS[1], 5)", "1", "l")
		}
	}()

	wg.Wait()
}

// TestLuaListSliceControlsUncontended is the control: against a list nobody
// mutates, every guarded binding must still return its documented reply. This
// proves the harness really reaches the Lua bindings, so a race FAIL is the
// defect and not a broken setup.
func TestLuaListSliceControlsUncontended(t *testing.T) {
	s := store.NewStore()
	router := luaSliceRouter(t, s)
	luaSliceRun(t, s, router, "RPUSH", "l", "a", "b", "c")

	for _, c := range []struct{ script, want string }{
		{"return redis.call('LLEN', redis.KEYS[1])", ":3\r\n"},
		{"return redis.call('LRANGE', redis.KEYS[1], 0, 1)", "*2\r\n$1\r\na\r\n$1\r\nb\r\n"},
		{"return redis.call('LINDEX', redis.KEYS[1], 1)", "$1\r\nb\r\n"},
		// Negative indices count from the end.
		{"return redis.call('LINDEX', redis.KEYS[1], -1)", "$1\r\nc\r\n"},
		// An empty window returns an empty array, not an error.
		{"return redis.call('LRANGE', redis.KEYS[1], 5, 9)", "*0\r\n"},
	} {
		if out := luaSliceRun(t, s, router, "EVAL", c.script, "1", "l"); out != c.want {
			t.Fatalf("CONTROL FAIL: %q = %q, want %q", c.script, out, c.want)
		}
	}

	// Boundary: LINDEX out of range is a Lua nil, not an error.
	if out := luaSliceRun(t, s, router, "EVAL", "return redis.call('LINDEX', redis.KEYS[1], 99)", "1", "l"); !strings.HasPrefix(out, "_") {
		t.Fatalf("CONTROL FAIL: LINDEX out of range = %q, want a null reply", out)
	}
}
