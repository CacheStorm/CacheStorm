package command

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/bits"
	"strconv"
	"strings"
	"sync"

	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterBitmapCommands(router *Router) {
	router.Register(&CommandDef{Name: "SETBIT", Handler: cmdSETBIT})
	router.Register(&CommandDef{Name: "GETBIT", Handler: cmdGETBIT})
	router.Register(&CommandDef{Name: "BITCOUNT", Handler: cmdBITCOUNT})
	router.Register(&CommandDef{Name: "BITPOS", Handler: cmdBITPOS})
	router.Register(&CommandDef{Name: "BITOP", Handler: cmdBITOP})
	router.Register(&CommandDef{Name: "BITFIELD", Handler: cmdBITFIELD})
	router.Register(&CommandDef{Name: "BITFIELD_RO", Handler: cmdBITFIELD})
}

type BitmapValue struct {
	mu   sync.RWMutex
	Data []byte
}

func (v *BitmapValue) Type() store.DataType { return store.DataTypeString }
func (v *BitmapValue) SizeOf() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return int64(len(v.Data)) + 48
}
func (v *BitmapValue) String() string { v.mu.RLock(); defer v.mu.RUnlock(); return string(v.Data) }
func (v *BitmapValue) RedisStringBytes() ([]byte, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return append([]byte(nil), v.Data...), nil
}
func (v *BitmapValue) Clone() store.Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	cloned := make([]byte, len(v.Data))
	copy(cloned, v.Data)
	return &BitmapValue{Data: cloned}
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

// bitmapWrongType reports whether key holds a value that is not a string, so
// the bitmap readers reply WRONGTYPE instead of treating it as empty.
func bitmapWrongType(ctx *Context, key string) bool {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return false
	}
	switch entry.Value.(type) {
	case *store.StringValue, *BitmapValue:
		return false
	default:
		return true
	}
}

func cmdSETBIT(ctx *Context) error {
	ctx.writeEffectsHandled = true
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	offset, ok := parseBitmapNumber(ctx.Arg(1), ctx.Replaying || ctx.LegacyReplay)
	if !ok || offset < 0 || offset > 4294967295 {
		return ctx.WriteError(bitfieldOffsetError())
	}
	bit, ok := parseBitmapNumber(ctx.Arg(2), ctx.Replaying || ctx.LegacyReplay)
	if !ok || (bit != 0 && bit != 1) {
		return ctx.WriteError(fmt.Errorf("ERR bit is not an integer or out of range"))
	}
	return bitmapSetBit(ctx, offset, byte(bit))
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
	if offset < 0 || offset > 4294967295 {
		return ctx.WriteError(errors.New("ERR bit offset is not an integer or out of range"))
	}

	if bitmapWrongType(ctx, key) {
		return ctx.WriteError(store.ErrWrongType)
	}
	bm := getBitmap(ctx, key)
	if bm == nil {
		return ctx.WriteInteger(0)
	}
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	byteIndex := int(offset / 8)
	bitIndex := uint(7 - offset%8)

	if byteIndex >= len(bm.Data) {
		return ctx.WriteInteger(0)
	}

	bit := 0
	if bm.Data[byteIndex]&(1<<bitIndex) != 0 {
		bit = 1
	}

	return ctx.WriteInteger(int64(bit))
}

var errBitArgument = errors.New("ERR The bit argument must be 1 or 0.") //nolint:staticcheck // Redis reply text

// bitmapBytes returns a copy of key's string bytes and whether the key
// exists; err is WRONGTYPE for a key of another type.
func bitmapBytes(ctx *Context, key string) ([]byte, bool, error) {
	entry, ok := ctx.Store.Get(key)
	if !ok {
		return nil, false, nil
	}
	switch v := entry.Value.(type) {
	case *BitmapValue:
		v.mu.RLock()
		defer v.mu.RUnlock()
		return append([]byte(nil), v.Data...), true, nil
	case *store.StringValue:
		return v.Data, true, nil
	}
	return nil, true, store.ErrWrongType
}

// bitRange resolves a BITCOUNT/BITPOS start/end pair over totlen bytes, as
// Redis does: in BIT mode the range is in bits and the edge bytes get masks
// for the bits outside it.
func bitRange(start, end, totlen int64, isBit bool) (s, e int64, firstMask, lastMask byte) {
	if isBit {
		totlen <<= 3
	}
	if start < 0 {
		start += totlen
	}
	if end < 0 {
		end += totlen
	}
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}
	if end >= totlen {
		end = totlen - 1
	}
	if isBit && start <= end {
		firstMask = ^byte((1<<(8-(start&7)))-1) & 0xFF
		lastMask = byte((1 << (7 - (end & 7))) - 1)
		start >>= 3
		end >>= 3
	}
	return start, end, firstMask, lastMask
}

func parseBitUnit(arg string) (bool, error) {
	switch strings.ToUpper(arg) {
	case "BIT":
		return true, nil
	case "BYTE":
		return false, nil
	}
	return false, ErrSyntaxError
}

func cmdBITCOUNT(ctx *Context) error {
	n := ctx.ArgCount()
	if n < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if n != 1 && n != 3 && n != 4 {
		return ctx.WriteError(ErrSyntaxError)
	}
	var start, end int64
	isBit := false
	if n >= 3 {
		var err error
		if start, err = strconv.ParseInt(ctx.ArgString(1), 10, 64); err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
		if end, err = strconv.ParseInt(ctx.ArgString(2), 10, 64); err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
		if n == 4 {
			if isBit, err = parseBitUnit(ctx.ArgString(3)); err != nil {
				return ctx.WriteError(err)
			}
		}
	}
	data, ok, err := bitmapBytes(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if !ok {
		return ctx.WriteInteger(0)
	}
	var firstMask, lastMask byte
	if n >= 3 {
		if start < 0 && end < 0 && start > end {
			return ctx.WriteInteger(0)
		}
		start, end, firstMask, lastMask = bitRange(start, end, int64(len(data)), isBit)
	} else {
		start, end = 0, int64(len(data))-1
	}
	if start > end {
		return ctx.WriteInteger(0)
	}
	count := 0
	for _, b := range data[start : end+1] {
		count += bits.OnesCount8(b)
	}
	if firstMask != 0 {
		count -= bits.OnesCount8(data[start] & firstMask)
	}
	if lastMask != 0 {
		count -= bits.OnesCount8(data[end] & lastMask)
	}
	return ctx.WriteInteger(int64(count))
}

func cmdBITPOS(ctx *Context) error {
	n := ctx.ArgCount()
	if n < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	bit, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	if bit != 0 && bit != 1 {
		return ctx.WriteError(errBitArgument)
	}
	var start, end int64 = 0, -1
	isBit, endGiven := false, false
	switch {
	case n >= 3 && n <= 5:
		if start, err = strconv.ParseInt(ctx.ArgString(2), 10, 64); err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
		if n == 5 {
			if isBit, err = parseBitUnit(ctx.ArgString(4)); err != nil {
				return ctx.WriteError(err)
			}
		}
		if n >= 4 {
			if end, err = strconv.ParseInt(ctx.ArgString(3), 10, 64); err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			endGiven = true
		}
	case n != 2:
		return ctx.WriteError(ErrSyntaxError)
	}
	data, ok, err := bitmapBytes(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if !ok {
		if bit == 1 {
			return ctx.WriteInteger(-1)
		}
		return ctx.WriteInteger(0)
	}
	start, end, firstMask, lastMask := bitRange(start, end, int64(len(data)), isBit)
	if start > end {
		return ctx.WriteInteger(-1)
	}
	want := bit == 1
	for i := start; i <= end; i++ {
		b := data[i]
		var outside byte
		if i == start {
			outside |= firstMask
		}
		if i == end {
			outside |= lastMask
		}
		if want {
			b &^= outside
		} else {
			b |= outside
		}
		for j := 0; j < 8; j++ {
			if (b&(0x80>>j) != 0) == want {
				return ctx.WriteInteger(i*8 + int64(j))
			}
		}
	}
	if want || endGiven {
		return ctx.WriteInteger(-1)
	}
	return ctx.WriteInteger((end + 1) * 8)
}

func cmdBITOP(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	op := strings.ToUpper(ctx.ArgString(0))
	switch op {
	case "AND", "OR", "XOR", "NOT":
	default:
		return ctx.WriteError(ErrSyntaxError)
	}
	if op == "NOT" && ctx.ArgCount() != 3 {
		return ctx.WriteError(errors.New("ERR BITOP NOT must be called with a single source key.")) //nolint:staticcheck // Redis reply text
	}
	destKey := ctx.ArgString(1)
	srcs := make([][]byte, 0, ctx.ArgCount()-2)
	maxLen := 0
	for i := 2; i < ctx.ArgCount(); i++ {
		data, _, err := bitmapBytes(ctx, ctx.ArgString(i))
		if err != nil {
			return ctx.WriteError(err)
		}
		srcs = append(srcs, data)
		maxLen = max(maxLen, len(data))
	}
	if maxLen == 0 {
		ctx.Store.Delete(destKey)
		return ctx.WriteInteger(0)
	}
	result := make([]byte, maxLen)
	for j := range result {
		var v byte
		for i, src := range srcs {
			var b byte
			if j < len(src) {
				b = src[j]
			}
			switch {
			case op == "NOT":
				v = ^b
			case i == 0:
				v = b
			case op == "AND":
				v &= b
			case op == "OR":
				v |= b
			default:
				v ^= b
			}
		}
		result[j] = v
	}
	if err := ctx.Store.Set(destKey, &BitmapValue{Data: result}, store.SetOptions{}); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(maxLen))
}

func cmdBITFIELD(ctx *Context) error {
	ctx.writeEffectsHandled = true
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return bitmapBitfield(ctx)
}

func bitfieldGet(data []byte, encoding string, offset int64) int64 {
	bits := parseEncoding(encoding)
	if bits == 0 {
		return 0
	}

	byteOffset := int(offset / 8)
	bitOffset := int(offset % 8)

	if byteOffset >= len(data) {
		return 0
	}

	var result int64
	remaining := bits
	currentByte := byteOffset
	currentBit := bitOffset

	for remaining > 0 && currentByte < len(data) {
		bitsToRead := 8 - currentBit
		if bitsToRead > remaining {
			bitsToRead = remaining
		}

		mask := byte((1 << bitsToRead) - 1)
		bitsRead := (data[currentByte] >> (8 - currentBit - bitsToRead)) & mask
		result = (result << bitsToRead) | int64(bitsRead)

		remaining -= bitsToRead
		currentBit += bitsToRead
		if currentBit >= 8 {
			currentBit = 0
			currentByte++
		}
	}

	result <<= remaining

	if strings.HasPrefix(encoding, "i") && bits > 0 {
		signBit := int64(1 << (bits - 1))
		if result&signBit != 0 {
			result |= ^((1 << bits) - 1)
		}
	}

	return result
}

func bitfieldSet(bm *BitmapValue, encoding string, offset int64, value int64) int64 {
	bits := parseEncoding(encoding)
	if bits == 0 {
		return 0
	}

	byteOffset := int(offset / 8)
	bitOffset := int(offset % 8)

	// Grow to exactly the bytes the field spans. A field of `bits` width
	// starting at bit `bitOffset` ends at bit bitOffset+bits-1, so it needs
	// ceil((bitOffset+bits)/8) bytes from byteOffset onwards. The previous
	// "byteOffset + (bits+7)/8 + 1" appended a phantom trailing byte to every
	// BITFIELD SET, so the stored string was one byte longer than Redis's and
	// BITOP reported a too-large destination length.
	totalBytes := byteOffset + (bitOffset+bits+7)/8
	if totalBytes > len(bm.Data) {
		newData := make([]byte, totalBytes)
		copy(newData, bm.Data)
		bm.Data = newData
	}

	oldValue := bitfieldGet(bm.Data, encoding, offset)

	mask := int64((1 << bits) - 1)
	valueToSet := value & mask

	remaining := bits
	currentByte := byteOffset
	currentBit := bitOffset

	for remaining > 0 {
		bitsToWrite := 8 - currentBit
		if bitsToWrite > remaining {
			bitsToWrite = remaining
		}

		shift := remaining - bitsToWrite
		bitsValue := byte((valueToSet >> shift) & ((1 << bitsToWrite) - 1))
		clearMask := ^byte(((1 << bitsToWrite) - 1) << (8 - currentBit - bitsToWrite))
		bm.Data[currentByte] = (bm.Data[currentByte] & clearMask) | (bitsValue << (8 - currentBit - bitsToWrite))

		remaining -= bitsToWrite
		currentBit += bitsToWrite
		if currentBit >= 8 {
			currentBit = 0
			currentByte++
		}
	}

	return oldValue
}

func bitfieldIncr(bm *BitmapValue, encoding string, offset int64, increment int64, overflow string) (int64, bool) {
	current := bitfieldGet(bm.Data, encoding, offset)
	bits := parseEncoding(encoding)
	if bits == 0 {
		return 0, false
	}

	maxVal := int64(1<<bits - 1)
	minVal := int64(0)

	if strings.HasPrefix(encoding, "i") {
		minVal = -(1 << (bits - 1))
		maxVal = (1 << (bits - 1)) - 1
	}

	newValue := current + increment
	positiveOverflow := increment > 0 && newValue < current
	negativeOverflow := increment < 0 && newValue > current

	switch overflow {
	case "SAT":
		if positiveOverflow || newValue > maxVal {
			newValue = maxVal
		} else if negativeOverflow || newValue < minVal {
			newValue = minVal
		}
	case "FAIL":
		if positiveOverflow || negativeOverflow || newValue > maxVal || newValue < minVal {
			return current, true
		}
	default:
		newValue &= (1 << bits) - 1
		if strings.HasPrefix(encoding, "i") && newValue&(1<<(bits-1)) != 0 {
			newValue |= ^((1 << bits) - 1)
		}
	}

	bitfieldSet(bm, encoding, offset, newValue)
	return newValue, false
}

func parseEncoding(encoding string) int {
	encoding = strings.ToLower(encoding)
	if len(encoding) < 2 || (encoding[0] != 'i' && encoding[0] != 'u') {
		return 0
	}
	for _, c := range encoding[1:] {
		if c < '0' || c > '9' {
			return 0
		}
	}
	bits, err := strconv.Atoi(encoding[1:])
	if err != nil || bits < 1 || bits > 64 {
		return 0
	}
	return bits
}

var _ = binary.BigEndian
