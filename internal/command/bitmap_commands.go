package command

import (
	"strconv"

	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterBitmapCommands(router *Router) {
	router.Register(&CommandDef{Name: "SETBIT", Handler: cmdSETBIT})
	router.Register(&CommandDef{Name: "GETBIT", Handler: cmdGETBIT})
	router.Register(&CommandDef{Name: "BITCOUNT", Handler: cmdBITCOUNT})
	router.Register(&CommandDef{Name: "BITPOS", Handler: cmdBITPOS})
	router.Register(&CommandDef{Name: "BITOP", Handler: cmdBITOP})
	router.Register(&CommandDef{Name: "BITFIELD", Handler: cmdBITFIELD})
}

type BitmapValue struct {
	Data []byte
}

func (v *BitmapValue) Type() store.DataType { return store.DataTypeString }
func (v *BitmapValue) SizeOf() int64        { return int64(len(v.Data)) + 24 }
func (v *BitmapValue) String() string       { return string(v.Data) }
func (v *BitmapValue) Clone() store.Value {
	cloned := make([]byte, len(v.Data))
	copy(cloned, v.Data)
	return &BitmapValue{Data: cloned}
}

func getOrCreateBitmap(ctx *Context, key string) *BitmapValue {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		bm := &BitmapValue{Data: make([]byte, 0)}
		ctx.Store.Set(key, bm, store.SetOptions{})
		return bm
	}

	switch v := entry.Value.(type) {
	case *BitmapValue:
		return v
	case *store.StringValue:
		return &BitmapValue{Data: v.Data}
	default:
		return nil
	}
}

func getBitmap(ctx *Context, key string) *BitmapValue {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return nil
	}

	switch v := entry.Value.(type) {
	case *BitmapValue:
		return v
	case *store.StringValue:
		return &BitmapValue{Data: v.Data}
	default:
		return nil
	}
}

func cmdSETBIT(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	offset, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	bit, err := strconv.Atoi(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	if bit != 0 && bit != 1 {
		return ctx.WriteError(ErrInvalidArg)
	}

	bm := getOrCreateBitmap(ctx, key)
	if bm == nil {
		return ctx.WriteError(store.ErrWrongType)
	}

	byteIndex := int(offset / 8)
	bitIndex := uint(offset % 8)

	if byteIndex >= len(bm.Data) {
		newData := make([]byte, byteIndex+1)
		copy(newData, bm.Data)
		bm.Data = newData
	}

	oldBit := 0
	if bm.Data[byteIndex]&(1<<bitIndex) != 0 {
		oldBit = 1
	}

	if bit == 1 {
		bm.Data[byteIndex] |= (1 << bitIndex)
	} else {
		bm.Data[byteIndex] &^= (1 << bitIndex)
	}

	return ctx.WriteInteger(int64(oldBit))
}

func cmdGETBIT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	offset, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	bm := getBitmap(ctx, key)
	if bm == nil {
		return ctx.WriteInteger(0)
	}

	byteIndex := int(offset / 8)
	bitIndex := uint(offset % 8)

	if byteIndex >= len(bm.Data) {
		return ctx.WriteInteger(0)
	}

	bit := 0
	if bm.Data[byteIndex]&(1<<bitIndex) != 0 {
		bit = 1
	}

	return ctx.WriteInteger(int64(bit))
}

func cmdBITCOUNT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	start := 0
	end := -1

	if ctx.ArgCount() >= 3 {
		var err error
		start, err = strconv.Atoi(ctx.ArgString(1))
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
		end, err = strconv.Atoi(ctx.ArgString(2))
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
	}

	bm := getBitmap(ctx, key)
	if bm == nil {
		return ctx.WriteInteger(0)
	}

	data := bm.Data
	if end < 0 {
		end = len(data) + end
	}
	if start < 0 {
		start = len(data) + start
	}
	if start < 0 {
		start = 0
	}
	if end >= len(data) {
		end = len(data) - 1
	}
	if start > end {
		return ctx.WriteInteger(0)
	}

	count := 0
	for i := start; i <= end; i++ {
		b := data[i]
		for b != 0 {
			count += int(b & 1)
			b >>= 1
		}
	}

	return ctx.WriteInteger(int64(count))
}

func cmdBITPOS(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	bit, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	if bit != 0 && bit != 1 {
		return ctx.WriteError(ErrInvalidArg)
	}

	start := 0
	end := -1

	if ctx.ArgCount() >= 3 {
		start, err = strconv.Atoi(ctx.ArgString(2))
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
	}
	if ctx.ArgCount() >= 4 {
		end, err = strconv.Atoi(ctx.ArgString(3))
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
	}

	bm := getBitmap(ctx, key)
	if bm == nil {
		return ctx.WriteInteger(-1)
	}

	data := bm.Data
	if len(data) == 0 {
		if bit == 1 {
			return ctx.WriteInteger(-1)
		}
		return ctx.WriteInteger(0)
	}

	if end < 0 {
		end = len(data) + end
	}
	if start < 0 {
		start = len(data) + start
	}
	if start < 0 {
		start = 0
	}
	if end >= len(data) {
		end = len(data) - 1
	}
	if start > end {
		return ctx.WriteInteger(-1)
	}

	target := byte(0)
	if bit == 1 {
		target = 1
	}

	for i := start; i <= end; i++ {
		b := data[i]
		for j := 0; j < 8; j++ {
			if (b & (1 << uint(j))) != 0 {
				if target == 1 {
					return ctx.WriteInteger(int64(i*8 + j))
				}
			} else {
				if target == 0 {
					return ctx.WriteInteger(int64(i*8 + j))
				}
			}
		}
	}

	if bit == 0 {
		return ctx.WriteInteger(int64(len(data) * 8))
	}

	return ctx.WriteInteger(-1)
}

func cmdBITOP(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	op := ctx.ArgString(0)
	destKey := ctx.ArgString(1)
	srcKeys := make([]string, 0, ctx.ArgCount()-2)
	for i := 2; i < ctx.ArgCount(); i++ {
		srcKeys = append(srcKeys, ctx.ArgString(i))
	}

	var result []byte
	for i, k := range srcKeys {
		bm := getBitmap(ctx, k)
		if bm == nil {
			continue
		}

		if i == 0 {
			result = make([]byte, len(bm.Data))
			copy(result, bm.Data)
			continue
		}

		if len(bm.Data) > len(result) {
			newResult := make([]byte, len(bm.Data))
			copy(newResult, result)
			result = newResult
		}

		for j := 0; j < len(bm.Data) && j < len(result); j++ {
			switch op {
			case "AND":
				result[j] &= bm.Data[j]
			case "OR":
				result[j] |= bm.Data[j]
			case "XOR":
				result[j] ^= bm.Data[j]
			case "NOT":
				if i == 0 {
					result[j] = ^bm.Data[j]
				}
			}
		}
	}

	if result == nil {
		return ctx.WriteInteger(0)
	}

	ctx.Store.Set(destKey, &BitmapValue{Data: result}, store.SetOptions{})
	return ctx.WriteInteger(int64(len(result)))
}

func cmdBITFIELD(ctx *Context) error {
	return ctx.WriteError(ErrUnknownCommand)
}
