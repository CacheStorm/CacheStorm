package command

import (
	"encoding/binary"
	"errors"
	"fmt"
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

	if bitmapWrongType(ctx, key) {
		return ctx.WriteError(store.ErrWrongType)
	}
	bm := getBitmap(ctx, key)
	if bm == nil {
		return ctx.WriteInteger(0)
	}
	bm.mu.RLock()
	defer bm.mu.RUnlock()

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
	endGiven := false

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
		endGiven = true
	}

	if bitmapWrongType(ctx, key) {
		return ctx.WriteError(store.ErrWrongType)
	}
	bm := getBitmap(ctx, key)
	if bm == nil {
		return ctx.WriteInteger(-1)
	}
	bm.mu.RLock()
	defer bm.mu.RUnlock()

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
			if (b & (1 << uint(7-j))) != 0 {
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
		// Redis confines the search to an explicitly supplied end offset, so a
		// 0 bit absent from that range is "not found". Answering with a position
		// past the last bit is only correct when no end offset was given.
		if endGiven {
			return ctx.WriteInteger(-1)
		}
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

	// NOT is unary, not a combining op: Redis takes exactly one source and
	// stores its inversion. It used to be handled by a `case "NOT"` inside
	// the combining loop below, but that switch is only reached from the
	// i > 0 iterations (i == 0 copies and continues), so the branch was
	// unreachable and NOT silently stored an unmodified copy of the source.
	for _, k := range srcKeys {
		if bitmapWrongType(ctx, k) {
			return ctx.WriteError(store.ErrWrongType)
		}
	}
	if op == "NOT" {
		if len(srcKeys) != 1 {
			return ctx.WriteError(errors.New("ERR BITOP NOT must be called with a single source key"))
		}
		bm := getBitmap(ctx, srcKeys[0])
		if bm == nil {
			return ctx.WriteInteger(0)
		}
		bm = bm.Clone().(*BitmapValue)
		inverted := make([]byte, len(bm.Data))
		for j := range bm.Data {
			inverted[j] = ^bm.Data[j]
		}
		if err := ctx.Store.Set(destKey, &BitmapValue{Data: inverted}, store.SetOptions{}); err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteInteger(int64(len(inverted)))
	}

	// A source key that does not exist "counts as an empty string" (Redis
	// BITOP). An empty string is the IDENTITY element for OR and XOR, but the
	// ANNIHILATOR for AND: ANDing with an all-zero operand can only clear
	// every bit. The old `continue` skipped the missing source outright, so
	// `BITOP AND dest k1 missing` stored a byte-for-byte COPY of k1 — which is
	// what OR produces — and AND was not even commutative, because seeding was
	// keyed on the loop INDEX (i == 0) rather than on whether the accumulator
	// had been seeded yet.
	var result []byte
	seeded := false
	sawMissing := false

	for _, k := range srcKeys {
		bm := getBitmap(ctx, k)
		if bm == nil {
			sawMissing = true
			continue
		}

		bm = bm.Clone().(*BitmapValue)
		if !seeded {
			result = make([]byte, len(bm.Data))
			copy(result, bm.Data)
			seeded = true
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
			}
		}
		if op == "AND" {
			clear(result[len(bm.Data):])
		}
	}

	// Apply the missing operand's zero bits to an AND result.
	if op == "AND" && sawMissing && result != nil {
		for j := range result {
			result[j] = 0
		}
	}

	if result == nil {
		return ctx.WriteInteger(0)
	}

	if err := ctx.Store.Set(destKey, &BitmapValue{Data: result}, store.SetOptions{}); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(len(result)))
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
