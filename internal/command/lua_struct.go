package command

import (
	"bytes"
	"encoding/binary"
	"math"

	lua "github.com/cachestorm/cachestorm/third_party/gopherlua"
)

// structMaxIntSize and structMaxAlign are lua_struct.c's MAXINTSIZE and the
// alignment of a double on the 64-bit platforms Redis runs on.
const (
	structMaxIntSize = 32
	structMaxAlign   = 8
)

type structHeader struct {
	little bool
	align  int
}

// openStruct installs Roberto Ierusalimschy's struct library as Redis
// bundles it: struct.pack, struct.unpack and struct.size.
func openStruct(L *lua.LState) {
	mod := L.NewTable()
	mod.RawSetString("pack", L.NewFunction(structPack))
	mod.RawSetString("unpack", L.NewFunction(structUnpack))
	mod.RawSetString("size", L.NewFunction(structSize))
	L.SetGlobal("struct", mod)
}

func structGetNum(L *lua.LState, f string, i *int, df int) int {
	if *i >= len(f) || f[*i] < '0' || f[*i] > '9' {
		return df
	}
	a := 0
	for *i < len(f) && f[*i] >= '0' && f[*i] <= '9' {
		d := int(f[*i] - '0')
		if a > math.MaxInt32/10 || a*10 > math.MaxInt32-d {
			L.RaiseError("integral size overflow")
		}
		a = a*10 + d
		*i++
	}
	return a
}

func structOptSize(L *lua.LState, opt byte, f string, i *int) int {
	switch opt {
	case 'B', 'b', 'x':
		return 1
	case 'H', 'h':
		return 2
	case 'f':
		return 4
	case 'L', 'l', 'T', 'd':
		return 8
	case 'c':
		return structGetNum(L, f, i, 1)
	case 'i', 'I':
		sz := structGetNum(L, f, i, 4)
		if sz > structMaxIntSize {
			L.RaiseError("integral size %d is larger than limit of %d", sz, structMaxIntSize)
		}
		return sz
	}
	return 0
}

func structToAlign(n int, h *structHeader, opt byte, size int) int {
	if size == 0 || opt == 'c' {
		return 0
	}
	if size > h.align {
		size = h.align
	}
	return (size - (n & (size - 1))) & (size - 1)
}

func structControl(L *lua.LState, opt byte, f string, i *int, h *structHeader) {
	switch opt {
	case ' ':
	case '>':
		h.little = false
	case '<':
		h.little = true
	case '!':
		a := structGetNum(L, f, i, structMaxAlign)
		if a == 0 || a&(a-1) != 0 {
			L.RaiseError("alignment %d is not a power of 2", a)
		}
		h.align = a
	default:
		L.ArgError(1, "invalid format option '"+string(opt)+"'")
	}
}

func structIsInt(opt byte) bool {
	switch opt {
	case 'b', 'B', 'h', 'H', 'l', 'L', 'T', 'i', 'I':
		return true
	}
	return false
}

func structPack(L *lua.LState) int {
	f := L.CheckString(1)
	h := structHeader{little: true, align: 1}
	arg := 2
	// lua_struct.c pushes a nil mark after the arguments, so the first
	// missing one is reported as nil.
	L.Push(lua.LNil)
	var b bytes.Buffer
	for i := 0; i < len(f); {
		opt := f[i]
		i++
		size := structOptSize(L, opt, f, &i)
		for pad := structToAlign(b.Len(), &h, opt, size); pad > 0; pad-- {
			b.WriteByte(0)
		}
		switch {
		case structIsInt(opt):
			n := float64(L.CheckNumber(arg))
			arg++
			var v uint64
			if n < 0 {
				v = uint64(int64(n))
			} else {
				v = uint64(n)
			}
			buf := make([]byte, size)
			for j := 0; j < size && j < 8; j++ {
				if h.little {
					buf[j] = byte(v >> (8 * j))
				} else {
					buf[size-1-j] = byte(v >> (8 * j))
				}
			}
			b.Write(buf)
		case opt == 'x':
			b.WriteByte(0)
		case opt == 'f':
			b.Write(structOrder(h).AppendUint32(nil, math.Float32bits(float32(L.CheckNumber(arg)))))
			arg++
		case opt == 'd':
			b.Write(structOrder(h).AppendUint64(nil, math.Float64bits(float64(L.CheckNumber(arg)))))
			arg++
		case opt == 'c' || opt == 's':
			s := L.CheckString(arg)
			arg++
			if size == 0 {
				size = len(s)
			}
			if len(s) < size {
				L.ArgError(arg, "string too short")
			}
			b.WriteString(s[:size])
			if opt == 's' {
				b.WriteByte(0)
			}
		default:
			structControl(L, opt, f, &i, &h)
		}
	}
	L.Push(lua.LString(b.String()))
	return 1
}

type structByteOrder interface {
	binary.ByteOrder
	binary.AppendByteOrder
}

func structOrder(h structHeader) structByteOrder {
	if h.little {
		return binary.LittleEndian
	}
	return binary.BigEndian
}

func structUnpack(L *lua.LState) int {
	f := L.CheckString(1)
	data := L.CheckString(2)
	ld := len(data)
	pos := int(L.OptInt64(3, 1)) - 1
	if pos < 0 {
		L.ArgError(3, "offset must be 1 or greater")
	}
	h := structHeader{little: true, align: 1}
	n := 0
	for i := 0; i < len(f); {
		opt := f[i]
		i++
		size := structOptSize(L, opt, f, &i)
		pos += structToAlign(pos, &h, opt, size)
		if size > ld || pos > ld-size {
			L.ArgError(2, "data string too short")
		}
		switch {
		case structIsInt(opt):
			var v uint64
			for j := 0; j < size; j++ {
				c := data[pos+size-1-j]
				if !h.little {
					c = data[pos+j]
				}
				v = v<<8 | uint64(c)
			}
			res := float64(v)
			if opt >= 'a' && opt <= 'z' {
				if size < 8 && size > 0 && v&(1<<(size*8-1)) != 0 {
					v |= ^uint64(0) << (size * 8)
				}
				res = float64(int64(v))
			}
			L.Push(lua.LNumber(res))
			n++
		case opt == 'x':
		case opt == 'f':
			L.Push(lua.LNumber(math.Float32frombits(structOrder(h).Uint32([]byte(data[pos : pos+4])))))
			n++
		case opt == 'd':
			L.Push(lua.LNumber(math.Float64frombits(structOrder(h).Uint64([]byte(data[pos : pos+8])))))
			n++
		case opt == 'c':
			if size == 0 {
				prev, ok := L.Get(-1).(lua.LNumber)
				if n == 0 || !ok {
					L.RaiseError("format 'c0' needs a previous size")
				}
				size = int(prev)
				L.Pop(1)
				n--
				if size < 0 || size > ld || pos > ld-size {
					L.ArgError(2, "data string too short")
				}
			}
			L.Push(lua.LString(data[pos : pos+size]))
			n++
		case opt == 's':
			e := bytes.IndexByte([]byte(data[pos:]), 0)
			if e < 0 {
				L.RaiseError("unfinished string in data")
			}
			size = e + 1
			L.Push(lua.LString(data[pos : pos+e]))
			n++
		default:
			structControl(L, opt, f, &i, &h)
		}
		pos += size
	}
	L.Push(lua.LNumber(pos + 1))
	return n + 1
}

func structSize(L *lua.LState) int {
	f := L.CheckString(1)
	h := structHeader{little: true, align: 1}
	pos := 0
	for i := 0; i < len(f); {
		opt := f[i]
		i++
		size := structOptSize(L, opt, f, &i)
		pos += structToAlign(pos, &h, opt, size)
		if opt == 's' {
			L.ArgError(1, "option 's' has no fixed size")
		} else if opt == 'c' && size == 0 {
			L.ArgError(1, "option 'c0' has no fixed size")
		}
		if !isAlnum(opt) {
			structControl(L, opt, f, &i, &h)
		}
		pos += size
	}
	L.Push(lua.LNumber(pos))
	return 1
}

func isAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
