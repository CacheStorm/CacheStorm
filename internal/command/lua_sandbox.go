package command

import (
	"time"

	lua "github.com/cachestorm/cachestorm/third_party/gopherlua"
)

// shapeScriptGlobals makes the global environment of a client script look
// like Redis 7.4's sandbox: the same library set, Redis' pcall, os.clock
// only, and no print/getfenv/setfenv/module/package/newproxy.
func shapeScriptGlobals(L *lua.LState) {
	L.Push(L.NewFunction(lua.OpenCoroutine))
	L.Push(lua.LString(lua.CoroutineLibName))
	L.Call(1, 0)

	for _, name := range []string{"print", "getfenv", "setfenv", "module", "newproxy", "package", "_GOPHER_LUA_VERSION", "_printregs"} {
		L.SetGlobal(name, lua.LNil)
	}

	start := time.Now()
	osTable := L.NewTable()
	osTable.RawSetString("clock", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(time.Since(start).Seconds()))
		return 1
	}))
	L.SetGlobal("os", osTable)
	L.SetGlobal("gcinfo", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(0))
		return 1
	}))

	L.SetGlobal("pcall", L.NewFunction(redisPcall))

	if tbl, ok := L.GetGlobal("table").(*lua.LTable); ok {
		tbl.RawSetString("foreach", L.NewFunction(func(L *lua.LState) int {
			return tableForeach(L, false)
		}))
		tbl.RawSetString("foreachi", L.NewFunction(func(L *lua.LState) int {
			return tableForeach(L, true)
		}))
		tbl.RawSetString("setn", L.NewFunction(func(L *lua.LState) int {
			L.RaiseError("'setn' is obsolete")
			return 0
		}))
	}
}

// tableForeach implements Lua 5.1's table.foreach / table.foreachi: call f
// for each entry and stop at the first non-nil result, which is returned.
func tableForeach(L *lua.LState, indexed bool) int {
	t := L.CheckTable(1)
	f := L.CheckFunction(2)
	call := func(k, v lua.LValue) lua.LValue {
		L.Push(f)
		L.Push(k)
		L.Push(v)
		L.Call(2, 1)
		ret := L.Get(-1)
		L.Pop(1)
		return ret
	}
	if indexed {
		for i := 1; i <= t.Len(); i++ {
			if ret := call(lua.LNumber(i), t.RawGetInt(i)); ret != lua.LNil {
				L.Push(ret)
				return 1
			}
		}
		return 0
	}
	for k, v := t.Next(lua.LNil); k != lua.LNil; k, v = t.Next(k) {
		if ret := call(k, v); ret != lua.LNil {
			L.Push(ret)
			return 1
		}
	}
	return 0
}

// redisPcall is Redis' replacement for Lua's pcall: an error raised as a
// Redis error table ({err=...}, as redis.call raises) is returned as its
// message string.
func redisPcall(L *lua.LState) int {
	n := L.GetTop()
	if n == 0 {
		L.Push(lua.LFalse)
		L.Push(lua.LString("attempt to call a boolean value"))
		return 2
	}
	for i := 1; i <= n; i++ {
		L.Push(L.Get(i))
	}
	if err := L.PCall(n-1, lua.MultRet, nil); err != nil {
		var obj lua.LValue = lua.LString(err.Error())
		if apiErr, ok := err.(*lua.ApiError); ok && apiErr.Object != nil {
			obj = apiErr.Object
		}
		if tbl, ok := obj.(*lua.LTable); ok {
			if msg, ok := tbl.RawGetString("err").(lua.LString); ok {
				obj = msg
			}
		}
		L.Push(lua.LFalse)
		L.Push(obj)
		return 2
	}
	results := L.GetTop() - n
	L.Insert(lua.LTrue, n+1)
	return results + 1
}

// protectGlobals makes reading an undefined global an error and, as Redis'
// luaSetTableProtectionRecursively does, every table reachable from the
// globals readonly, except KEYS and ARGV.
func protectGlobals(L *lua.LState) {
	globals := L.Get(lua.GlobalsIndex).(*lua.LTable)
	mt := L.NewTable()
	mt.RawSetString("__index", L.NewFunction(func(L *lua.LState) int {
		L.RaiseError("Script attempted to access nonexistent global variable '%s'", L.ToString(2))
		return 0
	}))
	L.SetMetatable(globals, mt)
	skip := map[*lua.LTable]bool{}
	for _, name := range []string{"KEYS", "ARGV"} {
		if tbl, ok := globals.RawGetString(name).(*lua.LTable); ok {
			skip[tbl] = true
		}
	}
	setReadOnlyRecursively(globals, skip)
	if smt, ok := L.GetMetatable(lua.LString("")).(*lua.LTable); ok {
		setReadOnlyRecursively(smt, skip)
	}
}

func setReadOnlyRecursively(tbl *lua.LTable, skip map[*lua.LTable]bool) {
	if tbl.ReadOnly() || skip[tbl] {
		return
	}
	tbl.SetReadOnly(true)
	tbl.ForEach(func(_, v lua.LValue) {
		if t, ok := v.(*lua.LTable); ok {
			setReadOnlyRecursively(t, skip)
		}
	})
	if mt, ok := tbl.Metatable.(*lua.LTable); ok {
		setReadOnlyRecursively(mt, skip)
	}
}
