package lua

import (
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"strings"
)

/* basic functions {{{ */

func OpenBase(L *LState) int {
	global := L.Get(GlobalsIndex).(*LTable)
	L.SetGlobal("_G", global)
	L.SetGlobal("_VERSION", LString(LuaVersion))
	L.SetGlobal("_GOPHER_LUA_VERSION", LString(PackageName+" "+PackageVersion))
	basemod := L.RegisterModule("_G", baseFuncs)
	global.RawSetString("ipairs", L.NewClosure(baseIpairs, L.NewFunction(ipairsaux)))
	global.RawSetString("pairs", L.NewClosure(basePairs, L.NewFunction(pairsaux)))
	L.Push(basemod)
	return 1
}

var baseFuncs = map[string]LGFunction{
	"assert":         baseAssert,
	"collectgarbage": baseCollectGarbage,
	"dofile":         baseDoFile,
	"error":          baseError,
	"getfenv":        baseGetFEnv,
	"getmetatable":   baseGetMetatable,
	"load":           baseLoad,
	"loadfile":       baseLoadFile,
	"loadstring":     baseLoadString,
	"next":           baseNext,
	"pcall":          basePCall,
	"print":          basePrint,
	"rawequal":       baseRawEqual,
	"rawget":         baseRawGet,
	"rawset":         baseRawSet,
	"select":         baseSelect,
	"_printregs":     base_PrintRegs,
	"setfenv":        baseSetFEnv,
	"setmetatable":   baseSetMetatable,
	"tonumber":       baseToNumber,
	"tostring":       baseToString,
	"type":           baseType,
	"unpack":         baseUnpack,
	"xpcall":         baseXPCall,
	// loadlib
	"module":  loModule,
	"require": loRequire,
	// hidden features
	"newproxy": baseNewProxy,
}

func baseAssert(L *LState) int {
	if !L.ToBool(1) {
		L.RaiseError("%s", L.OptString(2, "assertion failed!"))
		return 0
	}
	return L.GetTop()
}

func baseCollectGarbage(L *LState) int {
	runtime.GC()
	return 0
}

func baseDoFile(L *LState) int {
	src := L.ToString(1)
	top := L.GetTop()
	fn, err := L.LoadFile(src)
	if err != nil {
		L.Push(LString(err.Error()))
		L.Panic(L)
	}
	L.Push(fn)
	L.Call(0, MultRet)
	return L.GetTop() - top
}

func baseError(L *LState) int {
	obj := L.Get(1)
	level := L.OptInt(2, 1)
	if num, ok := obj.(LNumber); ok && level > 0 {
		obj = LString(num.String())
	}
	L.Error(obj, level)
	return 0
}

func baseGetFEnv(L *LState) int {
	var value LValue
	if L.GetTop() == 0 {
		value = LNumber(1)
	} else {
		value = L.Get(1)
	}

	if fn, ok := value.(*LFunction); ok {
		if !fn.IsG {
			L.Push(fn.Env)
		} else {
			L.Push(L.G.Global)
		}
		return 1
	}

	if number, ok := value.(LNumber); ok {
		level := int(float64(number))
		if level <= 0 {
			L.Push(L.Env)
		} else {
			cf := L.currentFrame
			for i := 0; i < level && cf != nil; i++ {
				cf = cf.Parent
			}
			if cf == nil || cf.Fn.IsG {
				L.Push(L.G.Global)
			} else {
				L.Push(cf.Fn.Env)
			}
		}
		return 1
	}

	L.Push(L.G.Global)
	return 1
}

func baseGetMetatable(L *LState) int {
	L.Push(L.GetMetatable(L.CheckAny(1)))
	return 1
}

func ipairsaux(L *LState) int {
	tb := L.CheckTable(1)
	i := L.CheckInt(2)
	i++
	v := tb.RawGetInt(i)
	if v == LNil {
		return 0
	} else {
		L.Pop(1)
		L.Push(LNumber(i))
		L.Push(LNumber(i))
		L.Push(v)
		return 2
	}
}

func baseIpairs(L *LState) int {
	tb := L.CheckTable(1)
	L.Push(L.Get(UpvalueIndex(1)))
	L.Push(tb)
	L.Push(LNumber(0))
	return 3
}

func loadaux(L *LState, reader io.Reader, chunkname string) int {
	if fn, err := L.Load(reader, chunkname); err != nil {
		L.Push(LNil)
		L.Push(LString(err.Error()))
		return 2
	} else {
		L.Push(fn)
		return 1
	}
}

func baseLoad(L *LState) int {
	fn := L.CheckFunction(1)
	chunkname := L.OptString(2, "?")
	top := L.GetTop()
	buf := []string{}
	for {
		L.SetTop(top)
		L.Push(fn)
		L.Call(0, 1)
		ret := L.reg.Pop()
		if ret == LNil {
			break
		} else if LVCanConvToString(ret) {
			str := ret.String()
			if len(str) > 0 {
				buf = append(buf, string(str))
			} else {
				break
			}
		} else {
			L.Push(LNil)
			L.Push(LString("reader function must return a string"))
			return 2
		}
	}
	return loadaux(L, strings.NewReader(strings.Join(buf, "")), chunkname)
}

func baseLoadFile(L *LState) int {
	var reader io.Reader
	var chunkname string
	var err error
	if L.GetTop() < 1 {
		reader = os.Stdin
		chunkname = "<stdin>"
	} else {
		chunkname = L.CheckString(1)
		reader, err = os.Open(chunkname)
		if err != nil {
			L.Push(LNil)
			L.Push(LString(fmt.Sprintf("can not open file: %v", chunkname)))
			return 2
		}
		defer reader.(*os.File).Close()
	}
	return loadaux(L, reader, chunkname)
}

func baseLoadString(L *LState) int {
	return loadaux(L, strings.NewReader(L.CheckString(1)), L.OptString(2, "<string>"))
}

func baseNext(L *LState) int {
	tb := L.CheckTable(1)
	index := LNil
	if L.GetTop() >= 2 {
		index = L.Get(2)
	}
	if index != LNil {
		kv, isNum := index.(LNumber)
		inArray := isNum && isInteger(kv) && kv >= 1 && int(kv) <= len(tb.array)
		if _, known := tb.k2i[index]; !inArray && !known {
			L.RaiseError("invalid key to 'next'")
		}
	}
	key, value := tb.Next(index)
	if key == LNil {
		L.Push(LNil)
		return 1
	}
	L.Push(key)
	L.Push(value)
	return 2
}

func pairsaux(L *LState) int {
	tb := L.CheckTable(1)
	key, value := tb.Next(L.Get(2))
	if key == LNil {
		return 0
	} else {
		L.Pop(1)
		L.Push(key)
		L.Push(key)
		L.Push(value)
		return 2
	}
}

func basePairs(L *LState) int {
	tb := L.CheckTable(1)
	L.Push(L.Get(UpvalueIndex(1)))
	L.Push(tb)
	L.Push(LNil)
	return 3
}

func basePCall(L *LState) int {
	L.CheckAny(1)
	v := L.Get(1)
	if v.Type() != LTFunction && L.GetMetaField(v, "__call").Type() != LTFunction {
		L.Push(LFalse)
		L.Push(LString("attempt to call a " + v.Type().String() + " value"))
		return 2
	}
	nargs := L.GetTop() - 1
	if err := L.PCall(nargs, MultRet, nil); err != nil {
		L.Push(LFalse)
		if aerr, ok := err.(*ApiError); ok {
			L.Push(aerr.Object)
		} else {
			L.Push(LString(err.Error()))
		}
		return 2
	} else {
		L.Insert(LTrue, 1)
		return L.GetTop()
	}
}

func basePrint(L *LState) int {
	top := L.GetTop()
	for i := 1; i <= top; i++ {
		fmt.Print(L.ToStringMeta(L.Get(i)).String())
		if i != top {
			fmt.Print("\t")
		}
	}
	fmt.Println("")
	return 0
}

func base_PrintRegs(L *LState) int {
	L.printReg()
	return 0
}

func baseRawEqual(L *LState) int {
	if L.CheckAny(1) == L.CheckAny(2) {
		L.Push(LTrue)
	} else {
		L.Push(LFalse)
	}
	return 1
}

func baseRawGet(L *LState) int {
	L.Push(L.RawGet(L.CheckTable(1), L.CheckAny(2)))
	return 1
}

func baseRawSet(L *LState) int {
	tb := L.CheckTable(1)
	key, value := L.CheckAny(2), L.CheckAny(3)
	if tb.readonly {
		L.readonlyError()
	}
	L.RawSet(tb, key, value)
	return 0
}

func baseSelect(L *LState) int {
	L.CheckTypes(1, LTNumber, LTString)
	switch lv := L.Get(1).(type) {
	case LNumber:
		idx := int(lv)
		num := L.GetTop()
		if idx < 0 {
			idx = num + idx
		} else if idx > num {
			idx = num
		}
		if 1 > idx {
			L.ArgError(1, "index out of range")
		}
		return num - idx
	case LString:
		if string(lv) != "#" {
			L.ArgError(1, "invalid string '"+string(lv)+"'")
		}
		L.Push(LNumber(L.GetTop() - 1))
		return 1
	}
	return 0
}

func baseSetFEnv(L *LState) int {
	var value LValue
	if L.GetTop() == 0 {
		value = LNumber(1)
	} else {
		value = L.Get(1)
	}
	env := L.CheckTable(2)

	if fn, ok := value.(*LFunction); ok {
		if fn.IsG {
			L.RaiseError("cannot change the environment of given object")
		} else {
			fn.Env = env
			L.Push(fn)
			return 1
		}
	}

	if number, ok := value.(LNumber); ok {
		level := int(float64(number))
		if level <= 0 {
			L.Env = env
			return 0
		}

		cf := L.currentFrame
		for i := 0; i < level && cf != nil; i++ {
			cf = cf.Parent
		}
		if cf == nil || cf.Fn.IsG {
			L.RaiseError("cannot change the environment of given object")
		} else {
			cf.Fn.Env = env
			L.Push(cf.Fn)
			return 1
		}
	}

	L.RaiseError("cannot change the environment of given object")
	return 0
}

func baseSetMetatable(L *LState) int {
	L.CheckTable(1)
	obj := L.Get(1)
	mt := L.Get(2)
	if mt != LNil && mt.Type() != LTTable {
		L.ArgError(2, "nil or table expected")
	}
	if m := L.metatable(obj, true); m != LNil {
		if tb, ok := m.(*LTable); ok && tb.RawGetString("__metatable") != LNil {
			L.RaiseError("cannot change a protected metatable")
		}
	}
	L.SetMetatable(obj, mt)
	L.SetTop(1)
	return 1
}

func baseToNumber(L *LState) int {
	base := L.OptInt(2, 10)
	if base == 10 {
		lv := L.CheckAny(1)
		if num, ok := lv.(LNumber); ok {
			L.Push(num)
			return 1
		}
		if str, ok := lv.(LString); ok {
			if num, err := parseNumber(string(str)); err == nil {
				L.Push(num)
				return 1
			}
		}
		L.Push(LNil)
		return 1
	}
	str := L.CheckString(1)
	if base < 2 || base > 36 {
		L.ArgError(2, "base out of range")
	}
	s := strings.TrimLeft(str, cSpace)
	neg := false
	if s != "" && (s[0] == '-' || s[0] == '+') {
		neg = s[0] == '-'
		s = s[1:]
	}
	if base == 16 && len(s) > 2 && (s[:2] == "0x" || s[:2] == "0X") {
		s = s[2:]
	}
	digits := 0
	var n uint64
	overflow := false
	for ; digits < len(s); digits++ {
		c := s[digits]
		var d int
		switch {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'z':
			d = int(c-'a') + 10
		case c >= 'A' && c <= 'Z':
			d = int(c-'A') + 10
		default:
			d = 99
		}
		if d >= base {
			break
		}
		next := n*uint64(base) + uint64(d)
		if next/uint64(base) != n {
			overflow = true
		}
		n = next
	}
	if digits == 0 || strings.TrimLeft(s[digits:], cSpace) != "" {
		L.Push(LNil)
		return 1
	}
	if overflow {
		n = math.MaxUint64
	} else if neg {
		n = -n
	}
	L.Push(LNumber(float64(n)))
	return 1
}

func baseToString(L *LState) int {
	v1 := L.CheckAny(1)
	L.Push(L.ToStringMeta(v1))
	return 1
}

func baseType(L *LState) int {
	L.Push(LString(L.CheckAny(1).Type().String()))
	return 1
}

func baseUnpack(L *LState) int {
	tb := L.CheckTable(1)
	start := L.OptInt(2, 1)
	end := L.OptInt(3, tb.Len())
	for i := start; i <= end; i++ {
		L.Push(tb.RawGetInt(i))
	}
	ret := end - start + 1
	if ret < 0 {
		return 0
	}
	return ret
}

func baseXPCall(L *LState) int {
	fn := L.CheckFunction(1)
	errfunc := L.CheckFunction(2)

	top := L.GetTop()
	L.Push(fn)
	if err := L.PCall(0, MultRet, errfunc); err != nil {
		L.Push(LFalse)
		if aerr, ok := err.(*ApiError); ok {
			L.Push(aerr.Object)
		} else {
			L.Push(LString(err.Error()))
		}
		return 2
	} else {
		L.Insert(LTrue, top+1)
		return L.GetTop() - top
	}
}

/* }}} */

/* load lib {{{ */

func loModule(L *LState) int {
	name := L.CheckString(1)
	loaded := L.GetField(L.Get(RegistryIndex), "_LOADED")
	tb := L.GetField(loaded, name)
	if _, ok := tb.(*LTable); !ok {
		tb = L.FindTable(L.Get(GlobalsIndex).(*LTable), name, 1)
		if tb == LNil {
			L.RaiseError("name conflict for module: %v", name)
		}
		L.SetField(loaded, name, tb)
	}
	if L.GetField(tb, "_NAME") == LNil {
		L.SetField(tb, "_M", tb)
		L.SetField(tb, "_NAME", LString(name))
		names := strings.Split(name, ".")
		pname := ""
		if len(names) > 1 {
			pname = strings.Join(names[:len(names)-1], ".") + "."
		}
		L.SetField(tb, "_PACKAGE", LString(pname))
	}

	caller := L.currentFrame.Parent
	if caller == nil {
		L.RaiseError("no calling stack.")
	} else if caller.Fn.IsG {
		L.RaiseError("module() can not be called from GFunctions.")
	}
	L.SetFEnv(caller.Fn, tb)

	top := L.GetTop()
	for i := 2; i <= top; i++ {
		L.Push(L.Get(i))
		L.Push(tb)
		L.Call(1, 0)
	}
	L.Push(tb)
	return 1
}

var loopdetection = &LUserData{}

func loRequire(L *LState) int {
	name := L.CheckString(1)
	loaded := L.GetField(L.Get(RegistryIndex), "_LOADED")
	lv := L.GetField(loaded, name)
	if LVAsBool(lv) {
		if lv == loopdetection {
			L.RaiseError("loop or previous error loading module: %s", name)
		}
		L.Push(lv)
		return 1
	}
	loaders, ok := L.GetField(L.Get(RegistryIndex), "_LOADERS").(*LTable)
	if !ok {
		L.RaiseError("package.loaders must be a table")
	}
	messages := []string{}
	var modasfunc LValue
	for i := 1; ; i++ {
		loader := L.RawGetInt(loaders, i)
		if loader == LNil {
			L.RaiseError("module %s not found:\n\t%s, ", name, strings.Join(messages, "\n\t"))
		}
		L.Push(loader)
		L.Push(LString(name))
		L.Call(1, 1)
		ret := L.reg.Pop()
		switch retv := ret.(type) {
		case *LFunction:
			modasfunc = retv
			goto loopbreak
		case LString:
			messages = append(messages, string(retv))
		}
	}
loopbreak:
	L.SetField(loaded, name, loopdetection)
	L.Push(modasfunc)
	L.Push(LString(name))
	L.Call(1, 1)
	ret := L.reg.Pop()
	modv := L.GetField(loaded, name)
	if ret != LNil && modv == loopdetection {
		L.SetField(loaded, name, ret)
		L.Push(ret)
	} else if modv == loopdetection {
		L.SetField(loaded, name, LTrue)
		L.Push(LTrue)
	} else {
		L.Push(modv)
	}
	return 1
}

/* }}} */

/* hidden features {{{ */

func baseNewProxy(L *LState) int {
	ud := L.NewUserData()
	L.SetTop(1)
	if L.Get(1) == LTrue {
		L.SetMetatable(ud, L.NewTable())
	} else if d, ok := L.Get(1).(*LUserData); ok {
		L.SetMetatable(ud, L.GetMetatable(d))
	}
	L.Push(ud)
	return 1
}

/* }}} */

//
