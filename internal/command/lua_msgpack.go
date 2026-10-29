package command

import (
	"bytes"

	"math"
	"sort"

	lua "github.com/cachestorm/cachestorm/third_party/gopherlua"
)

// cmsgpackMaxNesting is lua-cmsgpack's LUACMSGPACK_MAX_NESTING: deeper
// tables encode as nil.
const cmsgpackMaxNesting = 16

// openCMsgpack installs the lua-cmsgpack library Redis bundles.
func openCMsgpack(L *lua.LState) {
	mod := L.NewTable()
	mod.RawSetString("_NAME", lua.LString("cmsgpack"))
	mod.RawSetString("_VERSION", lua.LString("lua-cmsgpack 0.4.0"))
	mod.RawSetString("_COPYRIGHT", lua.LString("Copyright (C) 2012, Salvatore Sanfilippo"))
	mod.RawSetString("_DESCRIPTION", lua.LString("MessagePack C implementation for Lua"))
	mod.RawSetString("pack", L.NewFunction(func(L *lua.LState) int {
		n := L.GetTop()
		if n == 0 {
			L.RaiseError("bad argument #0 to 'pack' (MessagePack pack needs input.)")
		}
		var buf bytes.Buffer
		for i := 1; i <= n; i++ {
			luaMsgpackEncode(&buf, L.Get(i), 0)
		}
		L.Push(lua.LString(buf.String()))
		return 1
	}))
	unpack := func(L *lua.LState, limit int, offset int) int {
		s := L.CheckString(1)
		d := &luaMsgpackDecoder{L: L, b: []byte(s), pos: offset}
		var values []lua.LValue
		for d.pos < len(d.b) && (limit <= 0 || len(values) < limit) {
			v, err := d.decode()
			if err != "" {
				L.RaiseError("%s", err)
			}
			values = append(values, v)
		}
		if limit > 0 {
			next := d.pos
			if d.pos >= len(d.b) {
				next = -1
			}
			L.Push(lua.LNumber(next))
			for _, v := range values {
				L.Push(v)
			}
			return len(values) + 1
		}
		for _, v := range values {
			L.Push(v)
		}
		return len(values)
	}
	mod.RawSetString("unpack", L.NewFunction(func(L *lua.LState) int {
		return unpack(L, 0, 0)
	}))
	mod.RawSetString("unpack_one", L.NewFunction(func(L *lua.LState) int {
		return unpack(L, 1, L.OptInt(2, 0))
	}))
	mod.RawSetString("unpack_limit", L.NewFunction(func(L *lua.LState) int {
		return unpack(L, L.CheckInt(2), L.OptInt(3, 0))
	}))
	L.SetGlobal("cmsgpack", mod)
}

func luaMsgpackEncode(buf *bytes.Buffer, v lua.LValue, depth int) {
	switch val := v.(type) {
	case lua.LString:
		luaMsgpackString(buf, string(val))
	case lua.LBool:
		if bool(val) {
			buf.WriteByte(0xc3)
		} else {
			buf.WriteByte(0xc2)
		}
	case lua.LNumber:
		f := float64(val)
		if f == math.Trunc(f) && f >= math.MinInt64 && f < math.MaxInt64 {
			luaMsgpackInt(buf, int64(f))
		} else {
			luaMsgpackDouble(buf, f)
		}
	case *lua.LTable:
		if depth >= cmsgpackMaxNesting {
			buf.WriteByte(0xc0)
			return
		}
		if n, ok := luaMsgpackArrayLen(val); ok {
			luaMsgpackHeader(buf, n, 0x90, 0xdc, 0xdd, 16)
			for i := 1; i <= n; i++ {
				luaMsgpackEncode(buf, val.RawGetInt(i), depth+1)
			}
			return
		}
		type kv struct {
			key []byte
			k   lua.LValue
			v   lua.LValue
		}
		var pairs []kv
		val.ForEach(func(k, x lua.LValue) {
			var kb bytes.Buffer
			luaMsgpackEncode(&kb, k, depth+1)
			pairs = append(pairs, kv{kb.Bytes(), k, x})
		})
		sort.Slice(pairs, func(i, j int) bool { return bytes.Compare(pairs[i].key, pairs[j].key) < 0 })
		luaMsgpackHeader(buf, len(pairs), 0x80, 0xde, 0xdf, 16)
		for _, p := range pairs {
			buf.Write(p.key)
			luaMsgpackEncode(buf, p.v, depth+1)
		}
	default:
		buf.WriteByte(0xc0)
	}
}

// luaMsgpackArrayLen reports whether t is a sequence 1..n without holes, the
// test lua-cmsgpack uses to choose between an array and a map.
func luaMsgpackArrayLen(t *lua.LTable) (int, bool) {
	count, maxKey := 0, 0
	isArray := true
	t.ForEach(func(k, _ lua.LValue) {
		n, ok := k.(lua.LNumber)
		if !ok || float64(n) < 1 || float64(n) != math.Trunc(float64(n)) {
			isArray = false
			return
		}
		count++
		if int(n) > maxKey {
			maxKey = int(n)
		}
	})
	return count, isArray && count == maxKey
}

// putBE appends the low size bytes of v in big-endian order.
func putBE(buf *bytes.Buffer, v uint64, size int) {
	for i := size - 1; i >= 0; i-- {
		buf.WriteByte(byte(v >> (8 * uint(i))))
	}
}

func luaMsgpackHeader(buf *bytes.Buffer, n int, fix, b16, b32 byte, fixMax int) {
	switch {
	case n < fixMax:
		buf.WriteByte(fix | byte(n))
	case n <= 0xffff:
		buf.WriteByte(b16)
		putBE(buf, uint64(n), 2)
	default:
		buf.WriteByte(b32)
		putBE(buf, uint64(n), 4)
	}
}

func luaMsgpackString(buf *bytes.Buffer, s string) {
	n := len(s)
	switch {
	case n < 32:
		buf.WriteByte(0xa0 | byte(n))
	case n <= 0xff:
		buf.WriteByte(0xd9)
		buf.WriteByte(byte(n))
	case n <= 0xffff:
		buf.WriteByte(0xda)
		putBE(buf, uint64(n), 2)
	default:
		buf.WriteByte(0xdb)
		putBE(buf, uint64(n), 4)
	}
	buf.WriteString(s)
}

func luaMsgpackInt(buf *bytes.Buffer, n int64) {
	if n >= 0 {
		switch {
		case n <= 127:
			buf.WriteByte(byte(n))
		case n <= 0xff:
			buf.WriteByte(0xcc)
			buf.WriteByte(byte(n))
		case n <= 0xffff:
			buf.WriteByte(0xcd)
			putBE(buf, uint64(n), 2)
		case n <= 0xffffffff:
			buf.WriteByte(0xce)
			putBE(buf, uint64(n), 4)
		default:
			buf.WriteByte(0xcf)
			putBE(buf, uint64(n), 8)
		}
		return
	}
	switch {
	case n >= -32:
		buf.WriteByte(byte(int8(n)))
	case n >= -128:
		buf.WriteByte(0xd0)
		buf.WriteByte(byte(int8(n)))
	case n >= -32768:
		buf.WriteByte(0xd1)
		putBE(buf, uint64(n), 2)
	case n >= -2147483648:
		buf.WriteByte(0xd2)
		putBE(buf, uint64(n), 4)
	default:
		buf.WriteByte(0xd3)
		putBE(buf, uint64(n), 8)
	}
}

func luaMsgpackDouble(buf *bytes.Buffer, f float64) {
	if f32 := float32(f); float64(f32) == f {
		buf.WriteByte(0xca)
		putBE(buf, uint64(math.Float32bits(f32)), 4)
		return
	}
	buf.WriteByte(0xcb)
	putBE(buf, math.Float64bits(f), 8)
}

type luaMsgpackDecoder struct {
	L   *lua.LState
	b   []byte
	pos int
}

const (
	msgpackMissing = "Missing bytes in input."
	msgpackBad     = "Bad data format in input."
)

func (d *luaMsgpackDecoder) take(n int) ([]byte, string) {
	if n < 0 || d.pos+n > len(d.b) {
		return nil, msgpackMissing
	}
	out := d.b[d.pos : d.pos+n]
	d.pos += n
	return out, ""
}

func (d *luaMsgpackDecoder) uint(n int) (uint64, string) {
	b, err := d.take(n)
	if err != "" {
		return 0, err
	}
	var v uint64
	for _, c := range b {
		v = v<<8 | uint64(c)
	}
	return v, ""
}

func (d *luaMsgpackDecoder) decode() (lua.LValue, string) {
	cb, err := d.take(1)
	if err != "" {
		return nil, err
	}
	c := cb[0]
	switch {
	case c <= 0x7f:
		return lua.LNumber(c), ""
	case c >= 0xe0:
		return lua.LNumber(int8(c)), ""
	case c&0xe0 == 0xa0:
		return d.str(int(c & 0x1f))
	case c&0xf0 == 0x90:
		return d.array(int(c & 0x0f))
	case c&0xf0 == 0x80:
		return d.mapN(int(c & 0x0f))
	}
	switch c {
	case 0xc0:
		return lua.LNil, ""
	case 0xc2:
		return lua.LFalse, ""
	case 0xc3:
		return lua.LTrue, ""
	case 0xcc, 0xcd, 0xce, 0xcf:
		v, err := d.uint(1 << (c - 0xcc))
		return lua.LNumber(float64(v)), err
	case 0xd0, 0xd1, 0xd2, 0xd3:
		size := 1 << (c - 0xd0)
		v, err := d.uint(size)
		shift := 64 - 8*size
		return lua.LNumber(float64(int64(v<<shift) >> shift)), err
	case 0xca:
		v, err := d.uint(4)
		return lua.LNumber(float64(math.Float32frombits(uint32(v)))), err
	case 0xcb:
		v, err := d.uint(8)
		return lua.LNumber(math.Float64frombits(v)), err
	case 0xd9, 0xda, 0xdb, 0xc4, 0xc5, 0xc6:
		size := map[byte]int{0xd9: 1, 0xda: 2, 0xdb: 4, 0xc4: 1, 0xc5: 2, 0xc6: 4}[c]
		n, err := d.uint(size)
		if err != "" {
			return nil, err
		}
		return d.str(int(n))
	case 0xdc, 0xdd:
		n, err := d.uint(2 << (c - 0xdc))
		if err != "" {
			return nil, err
		}
		return d.array(int(n))
	case 0xde, 0xdf:
		n, err := d.uint(2 << (c - 0xde))
		if err != "" {
			return nil, err
		}
		return d.mapN(int(n))
	}
	return nil, msgpackBad
}

func (d *luaMsgpackDecoder) str(n int) (lua.LValue, string) {
	b, err := d.take(n)
	if err != "" {
		return nil, err
	}
	return lua.LString(b), ""
}

func (d *luaMsgpackDecoder) array(n int) (lua.LValue, string) {
	tbl := d.L.NewTable()
	for i := 1; i <= n; i++ {
		v, err := d.decode()
		if err != "" {
			return nil, err
		}
		tbl.RawSetInt(i, v)
	}
	return tbl, ""
}

func (d *luaMsgpackDecoder) mapN(n int) (lua.LValue, string) {
	tbl := d.L.NewTable()
	for i := 0; i < n; i++ {
		k, err := d.decode()
		if err != "" {
			return nil, err
		}
		v, err := d.decode()
		if err != "" {
			return nil, err
		}
		if k != lua.LNil {
			tbl.RawSet(k, v)
		}
	}
	return tbl, ""
}
