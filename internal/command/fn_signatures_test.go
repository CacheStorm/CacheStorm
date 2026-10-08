package command

import (
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Round r40 proof: FUNCTION-language functions are called with NO
// parameters. Redis passes the declared keys and args as two table
// parameters — `function(keys, args)` — while CacheStorm's callFunction
// uses L.CallByParam with zero params and exposes KEYS/ARGV only as fields
// of the redis table. A Redis-style parameterized function therefore sees
// nil for both parameters and cannot read the keys it was invoked with.

const fnSigLibrary = `
redis = redis or {}

redis.sig = function(keys, args)
	if type(keys) ~= 'table' then
		return 'ERR keys param is ' .. type(keys)
	end
	return (keys[1] or 'nokey') .. '|' .. (args[1] or 'noarg')
end

redis.legacy = function()
	return (redis.KEYS and redis.KEYS[1]) or 'nokeys'
end
`

func fnSigSetup(t *testing.T) (*FunctionRegistry, *store.Store) {
	t.Helper()
	functionOnce = sync.Once{}
	functionRegistry = nil
	s := store.NewStore()
	r := GetFunctionRegistry(s)
	if err := r.CreateLibrary("sigtest", fnSigLibrary, false); err != nil {
		t.Fatalf("FUNCTION LOAD failed: %v", err)
	}
	return r, s
}

func fnSigCall(t *testing.T, r *FunctionRegistry, s *store.Store, dottedName string, fnArgs ...string) string {
	t.Helper()
	router := NewRouter()
	RegisterFunctionCommands(router)
	functionRegistry = r

	ctx, buf := bufCtx("FCALL", bytesArgs(append([]string{dottedName}, fnArgs...)...), s)
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("FCALL %s failed: %v", dottedName, err)
	}
	return buf.String()
}

// THE DEFECT: a Redis-style parameterized function must receive the declared
// keys and args as parameters.
func TestProofFCALLPassesKeysAndArgsAsParameters(t *testing.T) {
	r, s := fnSigSetup(t)

	reply := fnSigCall(t, r, s, "sigtest.sig", "1", "user:1", "hello")
	if !strings.Contains(reply, "user:1|hello") {
		t.Fatalf("FAIL: the function must receive keys[1]=user:1 and args[1]=hello as parameters, got %q", reply)
	}
}

// BOUNDARY: numkeys=0 yields empty-but-present tables.
func TestProofFCALLEmptyKeysStillPassesTables(t *testing.T) {
	r, s := fnSigSetup(t)

	reply := fnSigCall(t, r, s, "sigtest.sig", "0")
	if !strings.Contains(reply, "nokey|noarg") {
		t.Fatalf("FAIL: with numkeys=0 the function must receive empty tables, got %q", reply)
	}
}

// CONTROL: the old redis-table convention keeps working — a zero-param
// function reading redis.KEYS[1] is unaffected by the new parameters.
func TestProofControlLegacyRedisTableConventionStillWorks(t *testing.T) {
	r, s := fnSigSetup(t)

	reply := fnSigCall(t, r, s, "sigtest.legacy", "1", "user:1")
	if !strings.Contains(reply, "user:1") {
		t.Fatalf("control failed: the redis.KEYS convention must keep working, got %q", reply)
	}
}
