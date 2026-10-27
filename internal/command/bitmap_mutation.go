package command

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

type bitfieldOperation struct {
	name     string
	encoding string
	offset   int64
	value    int64
	overflow string
}

func parseBitfieldOperations(ctx *Context) ([]bitfieldOperation, int, error) {
	var ops []bitfieldOperation
	length := 0
	overflow := "WRAP"
	for i := 1; i < ctx.ArgCount(); {
		name := strings.ToUpper(ctx.ArgString(i))
		if name == "OVERFLOW" {
			if i+1 >= ctx.ArgCount() {
				return nil, 0, ErrSyntaxError
			}
			overflow = strings.ToUpper(ctx.ArgString(i + 1))
			if overflow != "WRAP" && overflow != "SAT" && overflow != "FAIL" {
				return nil, 0, fmt.Errorf("ERR Invalid OVERFLOW type specified")
			}
			i += 2
			continue
		}
		count := 4
		if name == "GET" {
			count = 3
		} else if name != "SET" && name != "INCRBY" {
			return nil, 0, ErrSyntaxError
		}
		if i+count > ctx.ArgCount() {
			return nil, 0, ErrSyntaxError
		}
		encoding := ctx.ArgString(i + 1)
		historical := ctx.Replaying || ctx.LegacyReplay
		if historical {
			encoding = strings.ToLower(encoding)
		}
		if len(encoding) < 2 || (encoding[0] != 'i' && encoding[0] != 'u') {
			return nil, 0, bitfieldTypeError()
		}
		bits, ok := parseRedisInt([]byte(encoding[1:]))
		if !ok && historical {
			digits := true
			for _, c := range encoding[1:] {
				if c < '0' || c > '9' {
					digits = false
					break
				}
			}
			if digits {
				var parseErr error
				bits, parseErr = strconv.ParseInt(encoding[1:], 10, 64)
				ok = parseErr == nil
			}
		}
		if !ok || bits < 1 || bits > 64 || (encoding[0] == 'u' && bits == 64 && !historical) {
			return nil, 0, bitfieldTypeError()
		}
		raw := ctx.ArgString(i + 2)
		scaled := strings.HasPrefix(raw, "#")
		if scaled {
			raw = raw[1:]
		}
		offset, ok := parseBitmapNumber([]byte(raw), historical)
		if !ok || offset < 0 {
			return nil, 0, bitfieldOffsetError()
		}
		if scaled {
			if offset > math.MaxInt64/bits {
				return nil, 0, bitfieldOffsetError()
			}
			offset *= bits
		}
		if offset >= int64(store.MaxValueSize)*8 {
			return nil, 0, bitfieldOffsetError()
		}
		op := bitfieldOperation{name: name, encoding: encoding, offset: offset, overflow: overflow}
		if name != "GET" {
			op.value, ok = parseBitmapNumber(ctx.Arg(i+3), historical)
			if !ok {
				return nil, 0, ErrNotInteger
			}
			length = max(length, int((offset+bits+7)/8))
		}
		ops = append(ops, op)
		i += count
	}
	if ctx.Command == "BITFIELD_RO" && length > 0 {
		return nil, 0, fmt.Errorf("ERR BITFIELD_RO only supports the GET subcommand")
	}
	return ops, length, nil
}

func bitfieldTypeError() error {
	return fmt.Errorf("ERR %s", "Invalid bitfield type. Use something like i16 u8. Note that u64 is not supported but i64 is.")
}
func bitfieldOffsetError() error {
	return fmt.Errorf("ERR bit offset is not an integer or out of range")
}

func prepareBitmapValue(value store.Value, prepare func([]byte) (store.ValuePlan, error)) (store.ValuePlan, error) {
	switch v := value.(type) {
	case nil:
		return prepare(nil)
	case *store.StringValue:
		return prepare(v.Data)
	case *BitmapValue:
		v.mu.RLock()
		defer v.mu.RUnlock()
		return prepare(v.Data)
	default:
		return store.ValuePlan{}, store.ErrWrongType
	}
}

func bitmapValuePlan(current store.Value, data []byte, length int, apply func([]byte)) store.ValuePlan {
	return store.ValuePlan{Size: int64(length) + 48, Apply: func() store.Value {
		bm, existing := current.(*BitmapValue)
		if existing {
			bm.mu.Lock()
			defer bm.mu.Unlock()
		} else {
			bm = &BitmapValue{}
		}
		if !existing || len(bm.Data) < length {
			grown := make([]byte, length)
			copy(grown, data)
			bm.Data = grown
		}
		apply(bm.Data)
		return bm
	}}
}

func bitmapSetBit(ctx *Context, offset int64, bit byte) error {
	ctx.writeEffectsHandled = true
	previous := int64(0)
	changed := false
	err := ctx.Store.ApplyValuePlan(ctx.ArgString(0), func(current store.Value) (store.ValuePlan, error) {
		changed = false
		return prepareBitmapValue(current, func(data []byte) (store.ValuePlan, error) {
			index := int(offset / 8)
			mask := byte(1 << uint(7-offset%8))
			old := byte(0)
			if index < len(data) {
				old = data[index]
			}
			previous = int64((old & mask) >> uint(7-offset%8))
			next := old &^ mask
			if bit != 0 {
				next |= mask
			}
			length := max(len(data), index+1)
			if length == len(data) && old == next {
				return store.ValuePlan{}, nil
			}
			changed = true
			return bitmapValuePlan(current, data, length, func(target []byte) { target[index] = next }), nil
		})
	})
	if err != nil {
		return ctx.WriteError(err)
	}
	if !changed {
		ctx.PropagateAs()
	}
	return ctx.WriteInteger(previous)
}

func bitmapBitfield(ctx *Context) error {
	ctx.writeEffectsHandled = true
	ops, growTo, err := parseBitfieldOperations(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	var results []*resp.Value
	var propagated [][]byte
	changed := false
	err = ctx.Store.ApplyValuePlan(ctx.ArgString(0), func(current store.Value) (store.ValuePlan, error) {
		results = results[:0]
		propagated = [][]byte{ctx.Arg(0)}
		changed = false
		return prepareBitmapValue(current, func(data []byte) (store.ValuePlan, error) {
			length := max(len(data), growTo)
			changed = length > len(data)
			growthCovered := !changed
			var growthOp bitfieldOperation
			var growthValue int64
			if changed {
				for _, op := range ops {
					if op.name != "GET" && int((op.offset+int64(parseEncoding(op.encoding))+7)/8) == growTo {
						growthOp = op
						growthValue = bitfieldGet(data, op.encoding, op.offset)
						break
					}
				}
			}
			patches := make(map[int]byte)
			for _, op := range ops {
				start := int(op.offset / 8)
				inner := op.offset % 8
				var scratch [9]byte
				for j := range scratch {
					index := start + j
					if index < len(data) {
						scratch[j] = data[index]
					}
					if b, ok := patches[index]; ok {
						scratch[j] = b
					}
				}
				old := bitfieldGet(scratch[:], op.encoding, inner)
				if op.name == "GET" {
					results = append(results, resp.IntegerValue(old))
					continue
				}
				bm := BitmapValue{Data: scratch[:]}
				var next int64
				var failed bool
				if op.name == "INCRBY" {
					next, failed = bitfieldIncr(&bm, op.encoding, inner, op.value, op.overflow)
				} else {
					if ctx.Replaying || ctx.LegacyReplay {
						next = op.value
						failed = false
					} else {
						next, failed = bitfieldSetValue(op.encoding, op.value, op.overflow)
					}
					if !failed {
						bitfieldSet(&bm, op.encoding, inner, next)
					}
				}
				if failed {
					results = append(results, resp.NullValue())
					continue
				}
				propagated = appendBitfieldSet(propagated, op, next)
				if int((op.offset+int64(parseEncoding(op.encoding))+7)/8) == growTo {
					growthCovered = true
				}
				if op.name == "SET" {
					results = append(results, resp.IntegerValue(old))
				} else {
					results = append(results, resp.IntegerValue(next))
				}
				if old != next {
					changed = true
				}
				size := int((inner + int64(parseEncoding(op.encoding)) + 7) / 8)
				for j := 0; j < size; j++ {
					patches[start+j] = scratch[j]
				}
			}
			if !changed {
				if current == nil && (ctx.Replaying || ctx.LegacyReplay) && ctx.Command == "BITFIELD" {
					changed = true
					return bitmapValuePlan(current, data, length, func([]byte) {}), nil
				}
				return store.ValuePlan{}, nil
			}
			if !growthCovered {
				prefix := appendBitfieldSet([][]byte{ctx.Arg(0)}, growthOp, growthValue)
				propagated = append(prefix, propagated[1:]...)
			}
			return bitmapValuePlan(current, data, length, func(target []byte) {
				for index, b := range patches {
					target[index] = b
				}
			}), nil
		})
	})
	if err != nil {
		return ctx.WriteError(err)
	}
	if !changed {
		ctx.PropagateAs()
	} else if len(propagated) == 1 {
		ctx.propagateSetX(ctx.ArgString(0), nil, 0)
	} else {
		ctx.PropagateAs(ExecutedCommand{Name: "BITFIELD", Args: propagated, DB: ctx.DB})
	}
	return ctx.WriteArray(results)
}

func bitfieldSetValue(encoding string, value int64, overflow string) (int64, bool) {
	bits := parseEncoding(encoding)
	minimum := int64(0)
	maximum := int64((uint64(1) << bits) - 1)
	signed := encoding[0] == 'i'
	if signed {
		minimum = -(int64(1) << uint(bits-1))
		maximum = (int64(1) << uint(bits-1)) - 1
	}
	positive := uint64(value) > uint64(maximum)
	negative := false
	if signed {
		positive = value > maximum || (bits != 64 && int64(uint64(maximum)-uint64(value)) < 0)
		negative = value < minimum || (bits != 64 && int64(uint64(minimum)-uint64(value)) > 0)
	}
	if positive || negative {
		if overflow == "FAIL" {
			return 0, true
		}
		if overflow == "SAT" {
			if !positive {
				return minimum, false
			}
			return maximum, false
		}
	}
	wrapped := value & int64((uint64(1)<<bits)-1)
	if signed && bits < 64 && wrapped&(int64(1)<<uint(bits-1)) != 0 {
		wrapped |= ^int64((uint64(1) << bits) - 1)
	}
	return wrapped, false
}

func parseBitmapNumber(arg []byte, historical bool) (int64, bool) {
	n, ok := parseRedisInt(arg)
	if ok || !historical {
		return n, ok
	}
	n, err := strconv.ParseInt(string(arg), 10, 64)
	return n, err == nil
}

func appendBitfieldSet(args [][]byte, op bitfieldOperation, value int64) [][]byte {
	bits := parseEncoding(op.encoding)
	sign := op.encoding[0]
	if bits == 64 {
		sign = 'i'
	}
	encoding := strconv.AppendInt([]byte{sign}, int64(bits), 10)
	return append(args, []byte("SET"), encoding, strconv.AppendInt(nil, op.offset, 10), strconv.AppendInt(nil, value, 10))
}
