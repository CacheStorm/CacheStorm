package command

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
	lua "github.com/yuin/gopher-lua"
)

type ScriptEngine struct {
	mu      sync.RWMutex
	scripts map[string]string
	store   *store.Store
}

func NewScriptEngine(s *store.Store) *ScriptEngine {
	return &ScriptEngine{
		scripts: make(map[string]string),
		store:   s,
	}
}

func (e *ScriptEngine) CreateState(keys []string, args []string) *lua.LState {
	L := lua.NewState()

	redisTable := L.NewTable()
	L.SetGlobal("redis", redisTable)

	L.SetField(redisTable, "call", L.NewFunction(func(L *lua.LState) int {
		n := L.GetTop()
		if n == 0 {
			L.Push(lua.LNil)
			return 1
		}

		cmd := L.CheckString(1)
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

		result := e.executeCommand(L, cmd, cmdArgs)
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
		cmdArgs := make([]string, 0, n-1)
		for i := 2; i <= n; i++ {
			cmdArgs = append(cmdArgs, L.ToString(i))
		}

		defer func() {
			if r := recover(); r != nil {
				L.Push(lua.LString(fmt.Sprintf("err: %v", r)))
			}
		}()

		result := e.executeCommand(L, cmd, cmdArgs)
		L.Push(result)
		return 1
	}))

	L.SetField(redisTable, "error_reply", L.NewFunction(func(L *lua.LState) int {
		msg := L.CheckString(1)
		tbl := L.NewTable()
		L.SetField(tbl, "err", lua.LString(msg))
		L.Push(tbl)
		return 1
	}))

	L.SetField(redisTable, "status_reply", L.NewFunction(func(L *lua.LState) int {
		msg := L.CheckString(1)
		tbl := L.NewTable()
		L.SetField(tbl, "ok", lua.LString(msg))
		L.Push(tbl)
		return 1
	}))

	L.SetField(redisTable, "log", L.NewFunction(func(L *lua.LState) int {
		n := L.GetTop()
		if n < 2 {
			return 0
		}
		level := L.CheckString(1)
		msg := L.CheckString(2)
		fmt.Printf("[%s] %s\n", level, msg)
		return 0
	}))

	keysTable := L.NewTable()
	for i, k := range keys {
		L.SetTable(keysTable, lua.LNumber(i+1), lua.LString(k))
	}
	L.SetField(redisTable, "KEYS", keysTable)

	argvTable := L.NewTable()
	for i, a := range args {
		L.SetTable(argvTable, lua.LNumber(i+1), lua.LString(a))
	}
	L.SetField(redisTable, "ARGV", argvTable)

	return L
}

func (e *ScriptEngine) executeCommand(L *lua.LState, cmd string, args []string) lua.LValue {
	switch cmd {
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
			if val, ok := hv.Fields[args[1]]; ok {
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
			hv = entry.Value.(*store.HashValue)
		}
		hv.Fields[args[1]] = []byte(args[2])
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
			tbl := L.NewTable()
			for k, v := range hv.Fields {
				tbl.Append(lua.LString(k))
				tbl.Append(lua.LString(string(v)))
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
			lv = entry.Value.(*store.ListValue)
		}
		lv.Elements = append([][]byte{[]byte(args[1])}, lv.Elements...)
		return lua.LNumber(len(lv.Elements))

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
			lv = entry.Value.(*store.ListValue)
		}
		lv.Elements = append(lv.Elements, []byte(args[1]))
		return lua.LNumber(len(lv.Elements))

	case "LPOP":
		if len(args) < 1 {
			return lua.LNil
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNil
		}
		if lv, ok := entry.Value.(*store.ListValue); ok && len(lv.Elements) > 0 {
			val := lv.Elements[0]
			lv.Elements = lv.Elements[1:]
			return lua.LString(string(val))
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
		if lv, ok := entry.Value.(*store.ListValue); ok && len(lv.Elements) > 0 {
			val := lv.Elements[len(lv.Elements)-1]
			lv.Elements = lv.Elements[:len(lv.Elements)-1]
			return lua.LString(string(val))
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
			sv = entry.Value.(*store.SetValue)
		}
		if _, ok := sv.Members[args[1]]; !ok {
			sv.Members[args[1]] = struct{}{}
			return lua.LNumber(1)
		}
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
			if _, ok := sv.Members[args[1]]; ok {
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
			return lua.LNumber(len(sv.Members))
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
			if _, ok := hv.Fields[args[1]]; ok {
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
			deleted := 0
			for i := 1; i < len(args); i++ {
				if _, ok := hv.Fields[args[i]]; ok {
					delete(hv.Fields, args[i])
					deleted++
				}
			}
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
			return lua.LNumber(len(hv.Fields))
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
			return lua.LNumber(len(lv.Elements))
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
			tbl := L.NewTable()
			if start <= stop {
				for i := start; i <= stop; i++ {
					tbl.Append(lua.LString(string(lv.Elements[i])))
				}
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
		if _, exists := zset.Members[args[2]]; !exists {
			zset.Members[args[2]] = score
			return lua.LNumber(1)
		}
		zset.Members[args[2]] = score
		return lua.LNumber(0)

	case "ZSCORE":
		if len(args) < 2 {
			return lua.LNil
		}
		entry, exists := e.store.Get(args[0])
		if !exists {
			return lua.LNil
		}
		if zset, ok := entry.Value.(*store.SortedSetValue); ok {
			if score, ok := zset.Members[args[1]]; ok {
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
			if _, ok := zset.Members[args[1]]; ok {
				delete(zset.Members, args[1])
				return lua.LNumber(1)
			}
		}
		return lua.LNumber(0)

	case "DBSIZE":
		return lua.LNumber(e.store.KeyCount())

	case "FLUSHDB":
		e.store.Flush()
		return lua.LString("OK")

	default:
		return lua.LNil
	}
}

func (e *ScriptEngine) Eval(script string, keys []string, args []string) (interface{}, error) {
	L := e.CreateState(keys, args)
	defer L.Close()

	if err := L.DoString(script); err != nil {
		return nil, err
	}

	return e.convertResult(L), nil
}

func (e *ScriptEngine) EvalSHA(sha string, keys []string, args []string) (interface{}, error) {
	e.mu.RLock()
	script, exists := e.scripts[sha]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("NOSCRIPT No matching script. Please use EVAL")
	}

	return e.Eval(script, keys, args)
}

func (e *ScriptEngine) ScriptLoad(script string) string {
	sha := ScriptSHA(script)
	e.mu.Lock()
	e.scripts[sha] = script
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
