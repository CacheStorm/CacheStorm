package command

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	lua "github.com/cachestorm/cachestorm/third_party/gopherlua"

	"github.com/cachestorm/cachestorm/internal/store"
)

const (
	// luaScriptTimeout is the maximum execution time for a Lua script
	luaScriptTimeout = 5 * time.Second
)

type ScriptEngine struct {
	mu      sync.RWMutex
	scripts map[string]string
	store   *store.Store
	// dbs, set on the per-call engine of a script (scriptEngineFor), lets
	// SELECT inside the script switch that script's database.
	dbs *store.Databases
	// evalOrder lists, oldest first, the cached scripts that only EVAL put in
	// the cache. Like Redis 7.4, at most maxEvalScripts of them are kept;
	// SCRIPT LOAD pins a script.
	evalOrder []string
	evalOnly  map[string]bool
}

// maxEvalScripts is Redis 7.4's cap on scripts cached by EVAL alone.
const maxEvalScripts = 500

func NewScriptEngine(s *store.Store) *ScriptEngine {
	return &ScriptEngine{
		scripts:  make(map[string]string),
		store:    s,
		evalOnly: make(map[string]bool),
	}
}

// cacheEvalScript adds a script run by EVAL to the cache, evicting the
// oldest EVAL-only script beyond maxEvalScripts.
func (e *ScriptEngine) cacheEvalScript(script string) {
	sha := ScriptSHA(script)
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.scripts[sha]; ok {
		return
	}
	e.scripts[sha] = script
	e.evalOnly[sha] = true
	e.evalOrder = append(e.evalOrder, sha)
	for len(e.evalOnly) > maxEvalScripts && len(e.evalOrder) > 0 {
		oldest := e.evalOrder[0]
		e.evalOrder = e.evalOrder[1:]
		if e.evalOnly[oldest] {
			delete(e.evalOnly, oldest)
			delete(e.scripts, oldest)
		}
	}
}

// CreateState builds a Lua state with no ACL guard: scripts run outside an
// ACL user (the permissive default user) keep the unrestricted behaviour.
func (e *ScriptEngine) CreateState(keys []string, args []string) *lua.LState {
	return e.createState(keys, args, nil)
}

func (e *ScriptEngine) createState(keys []string, args []string, guard ScriptCallGuard) *lua.LState {
	return e.newState(keys, args, guard, nil)
}

// newState builds the sandboxed Lua state for one script run. With a
// scriptRun, redis.call / redis.pcall dispatch through the router with Redis
// reply and error semantics; without one they use the engine's built-in
// command subset (the Go API: Eval, EvalSHA, CreateState).
func (e *ScriptEngine) newState(keys []string, args []string, guard ScriptCallGuard, run *scriptRun) *lua.LState {
	L := lua.NewState(lua.Options{
		SkipOpenLibs: true,
	})

	// Open only safe libraries
	for _, pair := range []struct {
		name string
		fn   lua.LGFunction
	}{
		{lua.LoadLibName, lua.OpenPackage},
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	} {
		L.Push(L.NewFunction(pair.fn))
		L.Push(lua.LString(pair.name))
		L.Call(1, 0)
	}

	// Remove dangerous functions
	L.SetGlobal("dofile", lua.LNil)
	L.SetGlobal("loadfile", lua.LNil)
	L.SetGlobal("require", lua.LNil)

	openRedisLibs(L)

	redisTable := L.NewTable()
	L.SetGlobal("redis", redisTable)
	if run == nil || run.chunk == luaChunkFunction {
		// The Redis 7 registration convention: redis.register_function(name,
		// fn) declares redis.<name> = fn, so library code using either
		// convention resolves through the same lookup at call time.
		L.SetField(redisTable, "register_function", redisRegisterFunctionAlias(L))
	}
	installRedisHelpers(L, redisTable)

	if run != nil {
		shapeScriptGlobals(L)
		L.SetField(redisTable, "call", L.NewFunction(run.luaCallFunc(true)))
		L.SetField(redisTable, "pcall", L.NewFunction(run.luaCallFunc(false)))
		L.SetField(redisTable, "acl_check_cmd", L.NewFunction(run.luaACLCheckCmd))
		L.SetField(redisTable, "setresp", L.NewFunction(run.luaSetResp))
	} else {
		e.installLegacyCall(L, redisTable, guard)
	}

	keysTable := L.CreateTable(len(keys), 0)
	for _, k := range keys {
		keysTable.Append(lua.LString(k))
	}
	argvTable := L.CreateTable(len(args), 0)
	for _, a := range args {
		argvTable.Append(lua.LString(a))
	}
	L.SetGlobal("KEYS", keysTable)
	L.SetGlobal("ARGV", argvTable)
	// redis.KEYS / redis.ARGV are this engine's original spelling; they stay
	// as aliases of the Redis globals.
	L.SetField(redisTable, "KEYS", keysTable)
	L.SetField(redisTable, "ARGV", argvTable)

	return L
}

// installLegacyCall installs redis.call / redis.pcall over the engine's
// built-in command subset, which reports command errors as strings.
func (e *ScriptEngine) installLegacyCall(L *lua.LState, redisTable *lua.LTable, guard ScriptCallGuard) {
	// scriptArgs converts the Lua call arguments (index 2..n) to command
	// arguments. ONE implementation serves both redis.call and redis.pcall:
	// strings pass through, numbers use %v, booleans become "1"/"0" — so a
	// value produces identical arguments regardless of which binding the
	// author picked (pcall's old bare L.ToString loop silently turned
	// booleans into empty strings).
	scriptArgs := func(L *lua.LState, n int) []string {
		cmdArgs := make([]string, 0, n-1)
		for i := 2; i <= n; i++ {
			arg := L.Get(i)
			switch v := arg.(type) {
			case lua.LString:
				cmdArgs = append(cmdArgs, string(v))
			case lua.LNumber:
				cmdArgs = append(cmdArgs, fmt.Sprintf("%v", float64(v)))
			case lua.LBool:
				if bool(v) {
					cmdArgs = append(cmdArgs, "1")
				} else {
					cmdArgs = append(cmdArgs, "0")
				}
			default:
				cmdArgs = append(cmdArgs, L.ToString(i))
			}
		}
		return cmdArgs
	}

	L.SetField(redisTable, "call", L.NewFunction(func(L *lua.LState) int {
		n := L.GetTop()
		if n == 0 {
			L.Push(lua.LNil)
			return 1
		}

		cmd := L.CheckString(1)
		cmdArgs := scriptArgs(L, n)

		result := e.executeCommand(guard, L, cmd, cmdArgs)
		L.Push(result)
		return 1
	}))

	L.SetField(redisTable, "pcall", L.NewFunction(func(L *lua.LState) int {
		n := L.GetTop()
		if n == 0 {
			L.Push(lua.LNil)
			return 1
		}

		cmd := L.CheckString(1)
		cmdArgs := scriptArgs(L, n)

		// Consult the ACL guard BEFORE the recover below is registered: a
		// permission violation must abort the whole script and reach the
		// client, not be turned into a string the script can swallow.
		if err := e.checkScriptGuard(guard, cmd, cmdArgs); err != nil {
			panic(aclScriptError{err})
		}

		defer func() {
			if r := recover(); r != nil {
				L.Push(lua.LString(fmt.Sprintf("err: %v", r)))
			}
		}()

		result := e.executeCommand(guard, L, cmd, cmdArgs)
		L.Push(result)
		return 1
	}))
}

// ScriptCallGuard is consulted for every command a script invokes through
// redis.call / redis.pcall. A non-nil error aborts the script with that error.
type ScriptCallGuard func(cmd string, args []string) error

// aclScriptError wraps a guard violation so the script boundary can tell it
// apart from ordinary Lua errors and report it to the client instead of
// letting redis.pcall swallow it: a permission violation aborts the script
// regardless of pcall.
type aclScriptError struct{ err error }

func (e aclScriptError) Error() string { return e.err.Error() }

func (e *ScriptEngine) checkScriptGuard(guard ScriptCallGuard, cmd string, args []string) error {
	if guard == nil {
		return nil
	}
	return guard(cmd, args)
}

func (e *ScriptEngine) executeCommand(guard ScriptCallGuard, L *lua.LState, cmd string, args []string) lua.LValue {
	// ACL: every key a script touches — declared or not — must satisfy the
	// invoking user's key patterns. enforceACL only checks what is named on
	// the EVAL/EVALSHA line; redis.call dispatches straight to the store, so
	// without this guard a user confined to ~user:* could read and write any
	// other key from inside a script.
	if err := e.checkScriptGuard(guard, cmd, args); err != nil {
		panic(aclScriptError{err})
	}
	// Redis command names are case-insensitive and lowercase is the
	// conventional Lua spelling (redis.call('get', k)). The switch below only
	// has uppercase labels, so a lowercase name matched no arm and fell out of
	// the switch as nil WITHOUT RUNNING ANYTHING — a mutating call such as
	// redis.call('del', k) or redis.call('incr', k) silently left the store
	// untouched while the script reported success. Normalise once, here, so
	// redis.call and redis.pcall (both of which route through this function)
	// are fixed together.
	switch strings.ToUpper(cmd) {
	case "GET":
		if len(args) < 1 {
			return lua.LNil
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNil
		}
		if sv, ok := entry.Value.(*store.StringValue); ok {
			return lua.LString(string(sv.Data))
		}
		return lua.LNil

	case "SET":
		if len(args) < 2 {
			return lua.LNil
		}
		e.store.Set(args[0], &store.StringValue{Data: []byte(args[1])}, store.SetOptions{})
		return lua.LString("OK")

	case "DEL":
		if len(args) < 1 {
			return lua.LNumber(0)
		}
		deleted := e.store.Delete(args[0])
		if deleted {
			return lua.LNumber(1)
		}
		return lua.LNumber(0)

	case "EXISTS":
		if len(args) < 1 {
			return lua.LNumber(0)
		}
		_, exists := e.store.Get(args[0])
		if exists {
			return lua.LNumber(1)
		}
		return lua.LNumber(0)

	case "INCR":
		if len(args) < 1 {
			return lua.LNil
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			e.store.Set(args[0], &store.StringValue{Data: []byte("1")}, store.SetOptions{})
			return lua.LNumber(1)
		}
		if sv, ok := entry.Value.(*store.StringValue); ok {
			var val int
			fmt.Sscanf(string(sv.Data), "%d", &val)
			val++
			e.store.Set(args[0], &store.StringValue{Data: []byte(fmt.Sprintf("%d", val))}, store.SetOptions{})
			return lua.LNumber(val)
		}
		return lua.LNil

	case "DECR":
		if len(args) < 1 {
			return lua.LNil
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			e.store.Set(args[0], &store.StringValue{Data: []byte("-1")}, store.SetOptions{})
			return lua.LNumber(-1)
		}
		if sv, ok := entry.Value.(*store.StringValue); ok {
			var val int
			fmt.Sscanf(string(sv.Data), "%d", &val)
			val--
			e.store.Set(args[0], &store.StringValue{Data: []byte(fmt.Sprintf("%d", val))}, store.SetOptions{})
			return lua.LNumber(val)
		}
		return lua.LNil

	case "HGET":
		if len(args) < 2 {
			return lua.LNil
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNil
		}
		if hv, ok := entry.Value.(*store.HashValue); ok {
			// cmdHSET/cmdHDEL mutate Fields under the hash write lock.
			hv.RLock()
			val, found := hv.Fields[args[1]]
			hv.RUnlock()
			if found {
				return lua.LString(string(val))
			}
		}
		return lua.LNil

	case "HSET":
		if len(args) < 3 {
			return lua.LNil
		}
		entry, exists := e.store.Get(args[0])
		var hv *store.HashValue
		if !exists {
			hv = &store.HashValue{Fields: make(map[string][]byte)}
			e.store.Set(args[0], hv, store.SetOptions{})
		} else {
			var ok bool
			hv, ok = entry.Value.(*store.HashValue)
			if !ok {
				return lua.LNil
			}
		}
		hv.Lock()
		hv.SetField(args[1], []byte(args[2]))
		hv.Unlock()
		e.store.Reaccount(args[0])
		return lua.LNumber(1)

	case "HGETALL":
		if len(args) < 1 {
			return lua.LNil
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNil
		}
		if hv, ok := entry.Value.(*store.HashValue); ok {
			// Snapshot under the read lock, then build the table after
			// releasing it. HSET/HDEL mutate Fields under the write lock,
			// and a map iteration concurrent with a map write is a fatal
			// runtime error ("concurrent map iteration and map write").
			hv.RLock()
			pairs := make([][2][]byte, 0, len(hv.Fields))
			for k, v := range hv.Fields {
				pairs = append(pairs, [2][]byte{[]byte(k), v})
			}
			hv.RUnlock()
			tbl := L.NewTable()
			for _, p := range pairs {
				tbl.Append(lua.LString(string(p[0])))
				tbl.Append(lua.LString(string(p[1])))
			}
			return tbl
		}
		return lua.LNil

	case "LPUSH":
		if len(args) < 2 {
			return lua.LNumber(0)
		}
		entry, exists := e.store.Get(args[0])
		var lv *store.ListValue
		if !exists {
			lv = &store.ListValue{Elements: make([][]byte, 0)}
			e.store.Set(args[0], lv, store.SetOptions{})
		} else {
			var ok bool
			lv, ok = entry.Value.(*store.ListValue)
			if !ok {
				return lua.LNumber(0)
			}
		}
		lv.Lock()
		lv.PushFront([]byte(args[1]))
		n := len(lv.Elements)
		lv.Unlock()
		e.store.Reaccount(args[0])
		return lua.LNumber(n)

	case "RPUSH":
		if len(args) < 2 {
			return lua.LNumber(0)
		}
		entry, exists := e.store.Get(args[0])
		var lv *store.ListValue
		if !exists {
			lv = &store.ListValue{Elements: make([][]byte, 0)}
			e.store.Set(args[0], lv, store.SetOptions{})
		} else {
			var ok bool
			lv, ok = entry.Value.(*store.ListValue)
			if !ok {
				return lua.LNumber(0)
			}
		}
		lv.Lock()
		lv.PushBack([]byte(args[1]))
		n := len(lv.Elements)
		lv.Unlock()
		e.store.Reaccount(args[0])
		return lua.LNumber(n)

	case "LPOP":
		if len(args) < 1 {
			return lua.LNil
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNil
		}
		if lv, ok := entry.Value.(*store.ListValue); ok {
			lv.Lock()
			if len(lv.Elements) > 0 {
				val := lv.PopFront(1)[0]
				lv.Unlock()
				e.store.Reaccount(args[0])
				return lua.LString(string(val))
			}
			lv.Unlock()
		}
		return lua.LNil

	case "RPOP":
		if len(args) < 1 {
			return lua.LNil
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNil
		}
		if lv, ok := entry.Value.(*store.ListValue); ok {
			lv.Lock()
			if len(lv.Elements) > 0 {
				val := lv.PopBack(1)[0]
				lv.Unlock()
				e.store.Reaccount(args[0])
				return lua.LString(string(val))
			}
			lv.Unlock()
		}
		return lua.LNil

	case "SADD":
		if len(args) < 2 {
			return lua.LNumber(0)
		}
		entry, exists := e.store.Get(args[0])
		var sv *store.SetValue
		if !exists {
			sv = &store.SetValue{Members: make(map[string]struct{})}
			e.store.Set(args[0], sv, store.SetOptions{})
		} else {
			var ok bool
			sv, ok = entry.Value.(*store.SetValue)
			if !ok {
				return lua.LNumber(0)
			}
		}
		sv.Lock()
		if _, ok := sv.Members[args[1]]; !ok {
			sv.AddMember(args[1])
			sv.Unlock()
			e.store.Reaccount(args[0])
			return lua.LNumber(1)
		}
		sv.Unlock()
		return lua.LNumber(0)

	case "SISMEMBER":
		if len(args) < 2 {
			return lua.LNumber(0)
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNumber(0)
		}
		if sv, ok := entry.Value.(*store.SetValue); ok {
			// cmdSADD/cmdSREM/cmdSMOVE mutate Members under the set write lock.
			sv.RLock()
			_, found := sv.Members[args[1]]
			sv.RUnlock()
			if found {
				return lua.LNumber(1)
			}
		}
		return lua.LNumber(0)

	case "SCARD":
		if len(args) < 1 {
			return lua.LNumber(0)
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNumber(0)
		}
		if sv, ok := entry.Value.(*store.SetValue); ok {
			sv.RLock()
			n := len(sv.Members)
			sv.RUnlock()
			return lua.LNumber(n)
		}
		return lua.LNumber(0)

	case "TYPE":
		if len(args) < 1 {
			return lua.LString("none")
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LString("none")
		}
		return lua.LString(entry.Value.Type().String())

	case "EXPIRE":
		if len(args) < 2 {
			return lua.LNumber(0)
		}
		sec, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return lua.LNumber(0)
		}
		if e.store.SetTTL(args[0], time.Duration(sec)*time.Second) {
			return lua.LNumber(1)
		}
		return lua.LNumber(0)

	case "TTL":
		if len(args) < 1 {
			return lua.LNumber(-2)
		}
		ttl := e.store.TTL(args[0])
		if ttl < 0 {
			return lua.LNumber(int64(ttl))
		}
		return lua.LNumber(int64(ttl.Seconds()))

	case "MGET":
		if len(args) < 1 {
			return lua.LNil
		}
		tbl := L.NewTable()
		for _, key := range args {
			entry, exists := e.store.Get(key)
			if !exists {
				tbl.Append(lua.LNil)
			} else if sv, ok := entry.Value.(*store.StringValue); ok {
				tbl.Append(lua.LString(string(sv.Data)))
			} else {
				tbl.Append(lua.LNil)
			}
		}
		return tbl

	case "MSET":
		if len(args) < 2 {
			return lua.LString("OK")
		}
		for i := 0; i+1 < len(args); i += 2 {
			e.store.Set(args[i], &store.StringValue{Data: []byte(args[i+1])}, store.SetOptions{})
		}
		return lua.LString("OK")

	case "HEXISTS":
		if len(args) < 2 {
			return lua.LNumber(0)
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNumber(0)
		}
		if hv, ok := entry.Value.(*store.HashValue); ok {
			hv.RLock()
			_, found := hv.Fields[args[1]]
			hv.RUnlock()
			if found {
				return lua.LNumber(1)
			}
		}
		return lua.LNumber(0)

	case "HDEL":
		if len(args) < 2 {
			return lua.LNumber(0)
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNumber(0)
		}
		if hv, ok := entry.Value.(*store.HashValue); ok {
			hv.Lock()
			deleted := 0
			for i := 1; i < len(args); i++ {
				if _, ok := hv.Fields[args[i]]; ok {
					hv.DeleteField(args[i])
					deleted++
				}
			}
			hv.Unlock()
			return lua.LNumber(deleted)
		}
		return lua.LNumber(0)

	case "HLEN":
		if len(args) < 1 {
			return lua.LNumber(0)
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNumber(0)
		}
		if hv, ok := entry.Value.(*store.HashValue); ok {
			hv.RLock()
			n := len(hv.Fields)
			hv.RUnlock()
			return lua.LNumber(n)
		}
		return lua.LNumber(0)

	case "LLEN":
		if len(args) < 1 {
			return lua.LNumber(0)
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNumber(0)
		}
		if lv, ok := entry.Value.(*store.ListValue); ok {
			// cmdRPUSH/cmdLPUSH/cmdLPOP mutate Elements under the list
			// write lock, so even the length must be read under it.
			lv.RLock()
			n := len(lv.Elements)
			lv.RUnlock()
			return lua.LNumber(n)
		}
		return lua.LNumber(0)

	case "LRANGE":
		if len(args) < 3 {
			return lua.LNil
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNil
		}
		start, err1 := strconv.Atoi(args[1])
		stop, err2 := strconv.Atoi(args[2])
		if err1 != nil || err2 != nil {
			return lua.LNil
		}
		if lv, ok := entry.Value.(*store.ListValue); ok {
			// Copy the requested window out under the read lock, then build
			// the Lua table after releasing it. Copying (rather than keeping
			// a sub-slice) matters: cmdLPOP can shift the backing array in
			// place, which would otherwise corrupt a sub-slice held across
			// the unlock. No Store call happens here, so holding the lock
			// across the copy cannot self-deadlock.
			lv.RLock()
			length := len(lv.Elements)
			if start < 0 {
				start = length + start
			}
			if stop < 0 {
				stop = length + stop
			}
			if start < 0 {
				start = 0
			}
			if stop >= length {
				stop = length - 1
			}
			var window [][]byte
			if start <= stop {
				window = append([][]byte(nil), lv.Elements[start:stop+1]...)
			}
			lv.RUnlock()

			tbl := L.NewTable()
			for _, el := range window {
				tbl.Append(lua.LString(string(el)))
			}
			return tbl
		}
		return lua.LNil

	case "ZADD":
		if len(args) < 3 {
			return lua.LNumber(0)
		}
		score, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return lua.LNumber(0)
		}
		entry, exists := e.store.Get(args[0])
		var zset *store.SortedSetValue
		if !exists {
			zset = &store.SortedSetValue{Members: make(map[string]float64)}
			e.store.Set(args[0], zset, store.SetOptions{})
		} else {
			zset = entry.Value.(*store.SortedSetValue)
		}
		// cmdZADD mutates Members under zset.Lock(); an unguarded write here
		// races it, and concurrent map writes can abort the process.
		zset.Lock()
		_, already := zset.Members[args[2]]
		zset.Add(args[2], score)
		zset.Unlock()
		e.store.Reaccount(args[0])
		if already {
			return lua.LNumber(0)
		}
		return lua.LNumber(1)

	case "ZSCORE":
		if len(args) < 2 {
			return lua.LNil
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNil
		}
		if zset, ok := entry.Value.(*store.SortedSetValue); ok {
			// cmdZADD/cmdZREM mutate Members under the zset write lock.
			zset.RLock()
			score, found := zset.Members[args[1]]
			zset.RUnlock()
			if found {
				return lua.LNumber(score)
			}
		}
		return lua.LNil

	case "ZCARD":
		if len(args) < 1 {
			return lua.LNumber(0)
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNumber(0)
		}
		if zset, ok := entry.Value.(*store.SortedSetValue); ok {
			return lua.LNumber(len(zset.Members))
		}
		return lua.LNumber(0)

	case "ZREM":
		if len(args) < 2 {
			return lua.LNumber(0)
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNumber(0)
		}
		if zset, ok := entry.Value.(*store.SortedSetValue); ok {
			// cmdZREM deletes from Members under zset.Lock(); deleting here
			// without it races that write.
			zset.Lock()
			_, present := zset.Members[args[1]]
			if present {
				zset.Remove(args[1])
			}
			zset.Unlock()
			if present {
				return lua.LNumber(1)
			}
		}
		return lua.LNumber(0)

	case "SELECT":
		return e.scriptSelect(args)

	case "DBSIZE":
		return lua.LNumber(e.store.KeyCount())

	case "FLUSHDB":
		e.store.Flush()
		return lua.LString("OK")

	case "ZRANGE":
		if len(args) < 3 {
			return lua.LNil
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return L.NewTable()
		}
		if zset, ok := entry.Value.(*store.SortedSetValue); ok {
			start, err1 := strconv.Atoi(args[1])
			stop, err2 := strconv.Atoi(args[2])
			if err1 != nil || err2 != nil {
				return L.NewTable()
			}
			entries := zset.GetSortedRange(start, stop, false, false)
			tbl := L.NewTable()
			for _, e := range entries {
				tbl.Append(lua.LString(e.Member))
			}
			return tbl
		}
		return L.NewTable()

	case "HKEYS":
		if len(args) < 1 {
			return L.NewTable()
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return L.NewTable()
		}
		if hv, ok := entry.Value.(*store.HashValue); ok {
			hv.RLock()
			keys := make([]string, 0, len(hv.Fields))
			for k := range hv.Fields {
				keys = append(keys, k)
			}
			hv.RUnlock()
			tbl := L.NewTable()
			for _, k := range keys {
				tbl.Append(lua.LString(k))
			}
			return tbl
		}
		return L.NewTable()

	case "HVALS":
		if len(args) < 1 {
			return L.NewTable()
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return L.NewTable()
		}
		if hv, ok := entry.Value.(*store.HashValue); ok {
			hv.RLock()
			vals := make([][]byte, 0, len(hv.Fields))
			for _, v := range hv.Fields {
				vals = append(vals, v)
			}
			hv.RUnlock()
			tbl := L.NewTable()
			for _, v := range vals {
				tbl.Append(lua.LString(string(v)))
			}
			return tbl
		}
		return L.NewTable()

	case "SMEMBERS":
		if len(args) < 1 {
			return L.NewTable()
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return L.NewTable()
		}
		if sv, ok := entry.Value.(*store.SetValue); ok {
			sv.RLock()
			members := make([]string, 0, len(sv.Members))
			for member := range sv.Members {
				members = append(members, member)
			}
			sv.RUnlock()
			tbl := L.NewTable()
			for _, member := range members {
				tbl.Append(lua.LString(member))
			}
			return tbl
		}
		return L.NewTable()

	case "SREM":
		if len(args) < 2 {
			return lua.LNumber(0)
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNumber(0)
		}
		if sv, ok := entry.Value.(*store.SetValue); ok {
			// cmdSADD/cmdSREM/cmdSMOVE mutate Members under set.Lock();
			// deleting here without it races those writes.
			sv.Lock()
			removed := 0
			for i := 1; i < len(args); i++ {
				if _, ok := sv.Members[args[i]]; ok {
					sv.RemoveMember(args[i])
					removed++
				}
			}
			sv.Unlock()
			return lua.LNumber(removed)
		}
		return lua.LNumber(0)

	case "LINDEX":
		if len(args) < 2 {
			return lua.LNil
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNil
		}
		if lv, ok := entry.Value.(*store.ListValue); ok {
			idx, err := strconv.Atoi(args[1])
			if err != nil {
				return lua.LNil
			}
			// Length and the element read must come from one locked view.
			lv.RLock()
			length := len(lv.Elements)
			if idx < 0 {
				idx = length + idx
			}
			var val []byte
			if idx >= 0 && idx < length {
				val = lv.Elements[idx]
			}
			lv.RUnlock()
			if val != nil {
				return lua.LString(string(val))
			}
		}
		return lua.LNil

	case "APPEND":
		if len(args) < 2 {
			return lua.LNumber(0)
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			e.store.Set(args[0], &store.StringValue{Data: []byte(args[1])}, store.SetOptions{})
			return lua.LNumber(len(args[1]))
		}
		if sv, ok := entry.Value.(*store.StringValue); ok {
			newData := make([]byte, 0, len(sv.Data)+len(args[1]))
			newData = append(newData, sv.Data...)
			newData = append(newData, []byte(args[1])...)
			e.store.Set(args[0], &store.StringValue{Data: newData}, store.SetOptions{})
			return lua.LNumber(len(newData))
		}
		return lua.LNumber(0)

	case "STRLEN":
		if len(args) < 1 {
			return lua.LNumber(0)
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNumber(0)
		}
		if sv, ok := entry.Value.(*store.StringValue); ok {
			return lua.LNumber(len(sv.Data))
		}
		return lua.LNumber(0)

	case "INCRBY":
		if len(args) < 2 {
			return lua.LNil
		}
		incr, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return lua.LNil
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			e.store.Set(args[0], &store.StringValue{Data: []byte(args[1])}, store.SetOptions{})
			return lua.LNumber(incr)
		}
		if sv, ok := entry.Value.(*store.StringValue); ok {
			var val int64
			fmt.Sscanf(string(sv.Data), "%d", &val)
			val += incr
			e.store.Set(args[0], &store.StringValue{Data: []byte(fmt.Sprintf("%d", val))}, store.SetOptions{})
			return lua.LNumber(val)
		}
		return lua.LNil

	case "DECRBY":
		if len(args) < 2 {
			return lua.LNil
		}
		decr, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return lua.LNil
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			e.store.Set(args[0], &store.StringValue{Data: []byte(fmt.Sprintf("%d", -decr))}, store.SetOptions{})
			return lua.LNumber(-decr)
		}
		if sv, ok := entry.Value.(*store.StringValue); ok {
			var val int64
			fmt.Sscanf(string(sv.Data), "%d", &val)
			val -= decr
			e.store.Set(args[0], &store.StringValue{Data: []byte(fmt.Sprintf("%d", val))}, store.SetOptions{})
			return lua.LNumber(val)
		}
		return lua.LNil

	case "RENAME":
		if len(args) < 2 {
			return lua.LString("ERR")
		}
		_, err := e.store.Transfer(args[0], e.store, args[1], store.TransferOptions{Replace: true})
		if err == store.ErrKeyNotFound {
			return lua.LString("ERR no such key")
		}
		if err != nil {
			L.RaiseError("%s", err.Error())
			return lua.LNil
		}
		return lua.LString("OK")

	case "PERSIST":
		if len(args) < 1 {
			return lua.LNumber(0)
		}
		if e.store.Persist(args[0]) {
			return lua.LNumber(1)
		}
		return lua.LNumber(0)

	case "GETSET":
		if len(args) < 2 {
			return lua.LNil
		}
		entry, exists := e.store.Get(args[0])
		var oldVal []byte
		if exists {
			if sv, ok := entry.Value.(*store.StringValue); ok {
				oldVal = sv.Data
			}
		}
		e.store.Set(args[0], &store.StringValue{Data: []byte(args[1])}, store.SetOptions{})
		if oldVal != nil {
			return lua.LString(string(oldVal))
		}
		return lua.LNil

	case "SETNX":
		if len(args) < 2 {
			return lua.LNumber(0)
		}
		if _, exists := e.store.Get(args[0]); !exists {
			e.store.Set(args[0], &store.StringValue{Data: []byte(args[1])}, store.SetOptions{})
			return lua.LNumber(1)
		}
		return lua.LNumber(0)

	case "HMGET":
		if len(args) < 2 {
			return L.NewTable()
		}
		tbl := L.NewTable()
		entry, exists := e.store.Get(args[0])
		if !exists {
			for i := 1; i < len(args); i++ {
				tbl.Append(lua.LNil)
			}
			return tbl
		}
		if hv, ok := entry.Value.(*store.HashValue); ok {
			// cmdHSET/cmdHDEL mutate Fields under the hash write lock. The
			// lock spans the whole lookup loop: no Store call happens inside,
			// so it cannot self-deadlock.
			hv.RLock()
			for i := 1; i < len(args); i++ {
				if val, ok := hv.Fields[args[i]]; ok {
					tbl.Append(lua.LString(string(val)))
				} else {
					tbl.Append(lua.LNil)
				}
			}
			hv.RUnlock()
		}
		return tbl

	case "HMSET":
		if len(args) < 3 || len(args)%2 == 0 {
			return lua.LString("ERR")
		}
		entry, exists := e.store.Get(args[0])
		var hv *store.HashValue
		if !exists {
			hv = &store.HashValue{Fields: make(map[string][]byte)}
		} else {
			hv = entry.Value.(*store.HashValue)
		}
		// cmdHSET mutates Fields under hash.Lock(); writing here without it
		// races that. The lock is released before the store.Set below, which
		// itself takes the value lock and would otherwise self-deadlock.
		hv.Lock()
		for i := 1; i+1 < len(args); i += 2 {
			hv.SetField(args[i], []byte(args[i+1]))
		}
		hv.Unlock()
		if !exists {
			e.store.Set(args[0], hv, store.SetOptions{})
		} else {
			e.store.Reaccount(args[0])
		}
		return lua.LString("OK")

	default:
		return lua.LString("ERR unknown command '" + cmd + "' called from Lua script")
	}
}

func (e *ScriptEngine) WithState(keys []string, args []string, fn func(*lua.LState) error) error {
	L := e.CreateState(keys, args)
	defer L.Close()

	ctx, cancel := context.WithTimeout(context.Background(), luaScriptTimeout)
	defer cancel()
	L.SetContext(ctx)

	return fn(L)
}

func (e *ScriptEngine) Eval(script string, keys []string, args []string) (interface{}, error) {
	return e.eval(script, keys, args, nil)
}

// EvalGuarded runs the script with an ACL guard applied to every command the
// script invokes through redis.call / redis.pcall. A nil guard behaves exactly
// like Eval.
func (e *ScriptEngine) EvalGuarded(script string, keys []string, args []string, guard ScriptCallGuard) (interface{}, error) {
	return e.eval(script, keys, args, guard)
}

func (e *ScriptEngine) eval(script string, keys []string, args []string, guard ScriptCallGuard) (interface{}, error) {
	L := e.createState(keys, args, guard)
	defer L.Close()

	// Enforce execution timeout to prevent infinite loops / DoS
	ctx, cancel := context.WithTimeout(context.Background(), luaScriptTimeout)
	defer cancel()
	L.SetContext(ctx)

	if err := e.doScriptString(L, script); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("ERR script timeout exceeded %.0f seconds", luaScriptTimeout.Seconds())
		}
		return nil, err
	}

	return e.convertResult(L), nil
}

// doScriptString runs the script through protected.
func (e *ScriptEngine) doScriptString(L *lua.LState, script string) (err error) {
	return e.protected(func() error { return L.DoString(script) })
}

// protected runs fn and turns an ACL guard violation — raised as
// aclScriptError to be uncatchable by redis.pcall — back into an ordinary
// error for the command layer. Any other panic keeps the surrounding
// behaviour.
func (e *ScriptEngine) protected(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if aclErr, ok := r.(aclScriptError); ok {
				err = aclErr.err
				return
			}
			panic(r)
		}
	}()
	return fn()
}

func (e *ScriptEngine) EvalSHA(sha string, keys []string, args []string) (interface{}, error) {
	return e.EvalSHAGuarded(sha, keys, args, nil)
}

// EvalSHAGuarded is EvalSHA with an ACL guard applied to the script's calls.
func (e *ScriptEngine) EvalSHAGuarded(sha string, keys []string, args []string, guard ScriptCallGuard) (interface{}, error) {
	e.mu.RLock()
	script, exists := e.scripts[sha]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("NOSCRIPT No matching script. Please use EVAL")
	}

	return e.eval(script, keys, args, guard)
}

func (e *ScriptEngine) ScriptLoad(script string) string {
	sha := ScriptSHA(script)
	e.mu.Lock()
	e.scripts[sha] = script
	delete(e.evalOnly, sha)
	e.mu.Unlock()
	return sha
}

func (e *ScriptEngine) ScriptExists(sha string) bool {
	e.mu.RLock()
	_, exists := e.scripts[sha]
	e.mu.RUnlock()
	return exists
}

func (e *ScriptEngine) ScriptFlush() {
	e.mu.Lock()
	e.scripts = make(map[string]string)
	e.evalOnly = make(map[string]bool)
	e.evalOrder = nil
	e.mu.Unlock()
}

func (e *ScriptEngine) convertResult(L *lua.LState) interface{} {
	if L.GetTop() == 0 {
		return nil
	}

	val := L.Get(-1)
	return luaToGo(val)
}

func luaToGo(val lua.LValue) interface{} {
	switch v := val.(type) {
	case *lua.LNilType:
		return nil
	case lua.LBool:
		return bool(v)
	case lua.LNumber:
		return float64(v)
	case lua.LString:
		return string(v)
	case *lua.LTable:
		result := make([]interface{}, 0)
		v.ForEach(func(key, value lua.LValue) {
			result = append(result, luaToGo(value))
		})
		return result
	default:
		return v.String()
	}
}

func ScriptSHA(script string) string {
	h := sha1.Sum([]byte(script))
	return hex.EncodeToString(h[:])
}

var LPool = sync.Pool{
	New: func() interface{} {
		return &lua.LTable{}
	},
}
