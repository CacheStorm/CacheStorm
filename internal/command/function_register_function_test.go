package command

import (
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Round r42 proof: Redis 7's redis.register_function(name, fn) registration
// convention must work in FUNCTION libraries. CacheStorm's legacy carry
// declares functions as redis.<name> = function ... fields; the registration
// API was unsupported (the load failed with "attempt to call a nil value").

const registerFunctionLibrary = `redis = redis or {}
redis.register_function('regtest', function(keys, args)
	return (keys[1] or 'nokey') .. '|' .. (args[1] or 'noarg')
end)
redis.legacy = function()
	return (redis.KEYS and redis.KEYS[1]) or 'nokeys'
end`

// The control's library uses ONLY the legacy carry, so it loads both before
// and after the fix and isolates the defect to the register_function call.
const legacyOnlyLibrary = `redis = redis or {}
redis.legacy = function()
	return (redis.KEYS and redis.KEYS[1]) or 'nokeys'
end`

func registerFunctionSetup(t *testing.T) (*FunctionRegistry, *store.Store, *Router) {
	t.Helper()
	functionOnce = sync.Once{}
	functionRegistry = nil
	s := store.NewStore()
	r := NewRouter()
	RegisterServerCommands(r)
	RegisterFunctionCommands(r)
	return GetFunctionRegistry(s), s, r
}

func fcallRegisterFunction(t *testing.T, router *Router, s *store.Store, dottedName string, fnArgs ...string) string {
	t.Helper()
	ctx, buf := bufCtx("FCALL", bytesArgs(append([]string{dottedName}, fnArgs...)...), s)
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("FCALL %s failed: %v", dottedName, err)
	}
	return buf.String()
}

// THE DEFECT: a library using the Redis 7 registration convention loads.
func TestProofRegisterFunctionDeclarationIsSupported(t *testing.T) {
	registry, s, router := registerFunctionSetup(t)
	if err := registry.CreateLibrary("reglib", registerFunctionLibrary, false); err != nil {
		t.Fatalf("FAIL: FUNCTION LOAD with redis.register_function must succeed, got %v", err)
	}
	if _, ok := registry.GetLibrary("reglib"); !ok {
		t.Fatal("FAIL: the library was not stored")
	}
	got := fcallRegisterFunction(t, router, s, "reglib.regtest", "1", "user:1", "hello")
	if !strings.Contains(got, "user:1|hello") {
		t.Fatalf("FAIL: the registered function must receive keys[1]=user:1 and args[1]=hello as parameters, reply %q", got)
	}
	// Coexistence: the legacy carry in the same library still works.
	carry := fcallRegisterFunction(t, router, s, "reglib.legacy", "1", "user:1")
	if !strings.Contains(carry, "user:1") {
		t.Fatalf("FAIL: the legacy carry must coexist with register_function, reply %q", carry)
	}
}

// CONTROL: a legacy-only library loads and runs on the pre-fix engine too.
func TestProofControlLegacyCarryStillWorks(t *testing.T) {
	registry, s, router := registerFunctionSetup(t)
	if err := registry.CreateLibrary("legacylib", legacyOnlyLibrary, false); err != nil {
		t.Fatalf("control failed: %v", err)
	}
	got := fcallRegisterFunction(t, router, s, "legacylib.legacy", "1", "user:1")
	if !strings.Contains(got, "user:1") {
		t.Fatalf("FAIL control: the redis.<name> carry must keep working, reply %q", got)
	}
}
