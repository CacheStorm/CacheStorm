package command

import (
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// stringValueOf returns the raw bytes of any value that Redis models as the
// string type.
//
// CacheStorm keeps bitmap bits in a *BitmapValue, which is a SECOND Go
// representation of the same Redis type: BitmapValue.Type() reports
// store.DataTypeString, so TYPE answers "string" and GET (which already
// bridged the two) returns the bytes. Every other string command used to
// type-assert *store.StringValue alone and answer WRONGTYPE, so the server
// would report a key's type and then refuse to serve that key — a
// self-contradiction, and a compatibility break: in Redis a bit is not a
// distinct type, so SETBIT/BITCOUNT/BITFIELD all operate on an ordinary
// string and STRLEN/APPEND/GETRANGE must all work on the result.
//
// Values that are genuinely not strings (lists, hashes, ...) still return
// false and keep their WRONGTYPE reply.
func stringValueOf(v store.Value) ([]byte, bool) {
	switch val := v.(type) {
	case *store.StringValue:
		return val.Data, true
	case *BitmapValue:
		return val.Data, true
	}
	return nil, false
}

func RegisterStringCommands(router *Router) {
	router.Register(&CommandDef{Name: "SET", Handler: cmdSET})
	router.Register(&CommandDef{Name: "GET", Handler: cmdGET})
	router.Register(&CommandDef{Name: "DEL", Handler: cmdDEL})
	router.Register(&CommandDef{Name: "EXISTS", Handler: cmdEXISTS})
	router.Register(&CommandDef{Name: "MSET", Handler: cmdMSET})
	router.Register(&CommandDef{Name: "MGET", Handler: cmdMGET})
	router.Register(&CommandDef{Name: "INCR", Handler: cmdINCR})
	router.Register(&CommandDef{Name: "DECR", Handler: cmdDECR})
	router.Register(&CommandDef{Name: "INCRBY", Handler: cmdINCRBY})
	router.Register(&CommandDef{Name: "DECRBY", Handler: cmdDECRBY})
	router.Register(&CommandDef{Name: "INCRBYFLOAT", Handler: cmdINCRBYFLOAT})
	router.Register(&CommandDef{Name: "APPEND", Handler: cmdAPPEND})
	router.Register(&CommandDef{Name: "STRLEN", Handler: cmdSTRLEN})
	router.Register(&CommandDef{Name: "GETRANGE", Handler: cmdGETRANGE})
	router.Register(&CommandDef{Name: "SETRANGE", Handler: cmdSETRANGE})
	router.Register(&CommandDef{Name: "SETNX", Handler: cmdSETNX})
	router.Register(&CommandDef{Name: "SETEX", Handler: cmdSETEX})
	router.Register(&CommandDef{Name: "PSETEX", Handler: cmdPSETEX})
	router.Register(&CommandDef{Name: "MSETNX", Handler: cmdMSETNX})
	router.Register(&CommandDef{Name: "GETSET", Handler: cmdGETSET})
	router.Register(&CommandDef{Name: "GETDEL", Handler: cmdGETDEL})
	router.Register(&CommandDef{Name: "GETEX", Handler: cmdGETEX})
	router.Register(&CommandDef{Name: "LCS", Handler: cmdLCS})
	router.Register(&CommandDef{Name: "SUBSTR", Handler: cmdGETRANGE})
}

func cmdSET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	value := ctx.Arg(1)

	flags, expireArg, err := parseStringOptions(ctx.Args[2:], true)
	if err != nil {
		return ctx.WriteError(err)
	}
	var whenMs int64
	if expireArg != nil {
		if whenMs, err = stringOptionsExpireMs(flags, expireArg, "set"); err != nil {
			return ctx.WriteError(err)
		}
	}
	return setGeneric(ctx, key, value, flags, expireArg != nil, whenMs)
}

// setGeneric is Redis' setGenericCommand: with GET the old value (which must
// be a string) is the reply whether or not the write happens, an NX/XX abort
// otherwise replies nil, and an expire instant that already passed deletes the
// key instead of storing it.
func setGeneric(ctx *Context, key string, value []byte, flags int, hasExpire bool, whenMs int64) error {
	var old []byte
	oldFound := false
	if flags&strOptGET != 0 {
		if entry, exists := ctx.Store.Get(key); exists {
			data, ok := stringValueOf(entry.Value)
			if !ok {
				return ctx.WriteError(store.ErrWrongType)
			}
			old = make([]byte, len(data))
			copy(old, data)
			oldFound = true
		}
	}
	reply := func() error {
		if flags&strOptGET == 0 {
			return ctx.WriteOK()
		}
		if oldFound {
			return ctx.WriteBulkBytes(old)
		}
		return ctx.WriteNullBulkString()
	}
	abort := func() error {
		if flags&strOptGET != 0 {
			return reply()
		}
		return ctx.WriteNull()
	}

	if hasExpire && whenMs <= time.Now().UnixMilli() {
		found := ctx.Store.Exists(key)
		if (flags&strOptNX != 0 && found) || (flags&strOptXX != 0 && !found) {
			return abort()
		}
		ctx.Store.Delete(key)
		return reply()
	}

	opts := store.SetOptions{
		NX:      flags&strOptNX != 0,
		XX:      flags&strOptXX != 0,
		KeepTTL: flags&strOptKEEPTTL != 0,
	}
	if hasExpire {
		opts.TTL = expireMsToTTL(whenMs)
	}
	if err := ctx.Store.Set(key, &store.StringValue{Data: value}, opts); err != nil {
		if errors.Is(err, store.ErrKeyExists) || errors.Is(err, store.ErrKeyNotFound) {
			return abort()
		}
		return ctx.WriteError(err)
	}
	return reply()
}

func cmdGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteNullBulkString()
	}

	data, ok := stringValueOf(entry.Value)
	if !ok {
		return ctx.WriteError(store.ErrWrongType)
	}

	return ctx.WriteBulkBytes(data)
}

func cmdDEL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	deleted := int64(0)
	for i := 0; i < ctx.ArgCount(); i++ {
		if ctx.Store.Delete(ctx.ArgString(i)) {
			deleted++
		}
	}

	return ctx.WriteInteger(deleted)
}

func cmdEXISTS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	count := int64(0)
	for i := 0; i < ctx.ArgCount(); i++ {
		if ctx.Store.Exists(ctx.ArgString(i)) {
			count++
		}
	}

	return ctx.WriteInteger(count)
}

func cmdMSET(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount()%2 != 0 {
		return ctx.WriteError(arityError("mset"))
	}

	// MSET is atomic ("all given keys are set at once"): validate every pair
	// and pre-check capacity before applying anything — a mid-application
	// failure would leave a partially applied MSET. The padding keeps the
	// estimate conservative against the tracker's per-entry overhead.
	var estimated int64
	for i := 0; i < ctx.ArgCount(); i += 2 {
		k := ctx.ArgString(i)
		if len(k) == 0 || len(k) > store.MaxKeySize {
			return ctx.WriteError(store.ErrInvalidKey)
		}
		if len(ctx.Arg(i+1)) > store.MaxValueSize {
			return ctx.WriteError(store.ErrValueTooLarge)
		}
		estimated += int64(len(k) + len(ctx.Arg(i+1)) + 128)
	}
	if mt := ctx.Store.MemoryTracker(); mt != nil && !mt.CanAllocate(estimated) {
		return ctx.WriteError(store.ErrMemoryLimit)
	}

	for i := 0; i < ctx.ArgCount(); i += 2 {
		if err := ctx.Store.Set(ctx.ArgString(i), &store.StringValue{Data: ctx.Arg(i + 1)}, store.SetOptions{}); err != nil {
			// Reachable only under concurrent pressure after the pre-flight:
			// report it rather than hiding a partial application behind +OK.
			return ctx.WriteError(err)
		}
	}
	return ctx.WriteOK()
}

func cmdMGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	results := make([]*resp.Value, ctx.ArgCount())
	for i := 0; i < ctx.ArgCount(); i++ {
		entry, exists := ctx.Store.Get(ctx.ArgString(i))
		if !exists {
			results[i] = resp.NullBulkString()
			continue
		}
		data, ok := stringValueOf(entry.Value)
		if !ok {
			results[i] = resp.NullBulkString()
			continue
		}
		results[i] = resp.BulkBytes(data)
	}

	return ctx.WriteArray(results)
}

func cmdINCR(ctx *Context) error {
	return incrBy(ctx, 1)
}

func cmdDECR(ctx *Context) error {
	return incrBy(ctx, -1)
}

func cmdINCRBY(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	incr, ok := parseRedisInt(ctx.Arg(1))
	if !ok {
		return ctx.WriteError(ErrNotInteger)
	}
	return incrBy(ctx, incr)
}

func cmdDECRBY(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	decr, ok := parseRedisInt(ctx.Arg(1))
	if !ok {
		return ctx.WriteError(ErrNotInteger)
	}
	if decr == math.MinInt64 {
		return ctx.WriteError(errDecrementOverflow)
	}
	return incrBy(ctx, -decr)
}

var errDecrementOverflow = errors.New("ERR decrement would overflow")

func incrBy(ctx *Context, incr int64) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)

	if exists {
		data, ok := stringValueOf(entry.Value)
		if !ok {
			return ctx.WriteError(store.ErrWrongType)
		}
		cur, ok := parseRedisInt(data)
		if !ok {
			return ctx.WriteError(ErrNotInteger)
		}
		if (incr > 0 && cur > math.MaxInt64-incr) || (incr < 0 && cur < math.MinInt64-incr) {
			return ctx.WriteError(ErrOverflow)
		}
		newVal := cur + incr
		if err := ctx.Store.Set(key, &store.StringValue{Data: []byte(strconv.FormatInt(newVal, 10))}, store.SetOptions{KeepTTL: true}); err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteInteger(newVal)
	}

	if err := ctx.Store.Set(key, &store.StringValue{Data: []byte(strconv.FormatInt(incr, 10))}, store.SetOptions{}); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(incr)
}

func cmdAPPEND(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	suffix := ctx.Arg(1)

	entry, exists := ctx.Store.Get(key)
	var newData []byte
	if !exists {
		newData = suffix
	} else {
		cur, ok := stringValueOf(entry.Value)
		if !ok {
			return ctx.WriteError(store.ErrWrongType)
		}
		newData = make([]byte, 0, len(cur)+len(suffix))
		newData = append(newData, cur...)
		newData = append(newData, suffix...)
	}

	if int64(len(newData)) > store.MaxValueSize {
		return ctx.WriteError(errStringTooLong)
	}

	if err := ctx.Store.Set(key, &store.StringValue{Data: newData}, store.SetOptions{KeepTTL: true}); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(len(newData)))
}

func cmdSTRLEN(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteInteger(0)
	}

	data, ok := stringValueOf(entry.Value)
	if !ok {
		return ctx.WriteError(store.ErrWrongType)
	}

	return ctx.WriteInteger(int64(len(data)))
}

func cmdGETRANGE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	start64, ok := parseRedisInt(ctx.Arg(1))
	if !ok {
		return ctx.WriteError(ErrNotInteger)
	}
	end64, ok := parseRedisInt(ctx.Arg(2))
	if !ok {
		return ctx.WriteError(ErrNotInteger)
	}
	if start64 < 0 && end64 < 0 && start64 > end64 {
		start64, end64 = 1, 0
	}
	start64 = max(min(start64, math.MaxInt32), math.MinInt32)
	end64 = max(min(end64, math.MaxInt32), math.MinInt32)
	start, end := int(start64), int(end64)

	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteBulkString("")
	}

	data, ok := stringValueOf(entry.Value)
	if !ok {
		return ctx.WriteError(store.ErrWrongType)
	}

	length := len(data)
	if length == 0 {
		return ctx.WriteBulkString("")
	}

	if start < 0 {
		start = length + start
	}
	if end < 0 {
		end = length + end
		if end < 0 {
			end = 0
		}
	}
	if start < 0 {
		start = 0
	}
	if end >= length {
		end = length - 1
	}
	if start > end {
		return ctx.WriteBulkString("")
	}

	return ctx.WriteBulkBytes(data[start : end+1])
}

var errStringTooLong = errors.New("ERR string exceeds maximum allowed size (proto-max-bulk-len)")

func cmdSETRANGE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	offset, ok := parseRedisInt(ctx.Arg(1))
	if !ok {
		return ctx.WriteError(ErrNotInteger)
	}
	if offset < 0 {
		return ctx.WriteError(errors.New("ERR offset is out of range"))
	}
	value := ctx.Arg(2)

	entry, exists := ctx.Store.Get(key)
	var cur []byte
	if exists {
		data, ok := stringValueOf(entry.Value)
		if !ok {
			return ctx.WriteError(store.ErrWrongType)
		}
		cur = data
	}
	// Writing nothing changes nothing: Redis replies with the current length
	// and does not create a missing key.
	if len(value) == 0 {
		return ctx.WriteInteger(int64(len(cur)))
	}
	if offset > store.MaxValueSize-int64(len(value)) {
		return ctx.WriteError(errStringTooLong)
	}

	newLen := max(int(offset)+len(value), len(cur))
	data := make([]byte, newLen)
	copy(data, cur)
	copy(data[offset:], value)
	if err := ctx.Store.Set(key, &store.StringValue{Data: data}, store.SetOptions{KeepTTL: true}); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(len(data)))
}

func cmdSETNX(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	value := ctx.Arg(1)

	err := ctx.Store.Set(key, &store.StringValue{Data: value}, store.SetOptions{NX: true})
	if err != nil {
		if errors.Is(err, store.ErrKeyExists) {
			return ctx.WriteInteger(0)
		}
		return ctx.WriteError(err)
	}

	return ctx.WriteInteger(1)
}

func cmdSETEX(ctx *Context) error {
	return setExpireGeneric(ctx, true, "setex")
}

func cmdPSETEX(ctx *Context) error {
	return setExpireGeneric(ctx, false, "psetex")
}

func setExpireGeneric(ctx *Context, seconds bool, cmd string) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	whenMs, err := stringExpireMs(ctx.Arg(1), seconds, true, cmd)
	if err != nil {
		return ctx.WriteError(err)
	}
	return setGeneric(ctx, ctx.ArgString(0), ctx.Arg(2), 0, true, whenMs)
}

func cmdMSETNX(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount()%2 != 0 {
		return ctx.WriteError(arityError("msetnx"))
	}

	// MSETNX is atomic ("all given keys are set at once"): validate every
	// pair and pre-check capacity before applying anything — a mid-way
	// failure would leave a partially applied MSETNX. The padding keeps the
	// estimate conservative against the tracker's per-entry overhead.
	var estimated int64
	for i := 0; i < ctx.ArgCount(); i += 2 {
		k := ctx.ArgString(i)
		if len(k) == 0 || len(k) > store.MaxKeySize {
			return ctx.WriteError(store.ErrInvalidKey)
		}
		if len(ctx.Arg(i+1)) > store.MaxValueSize {
			return ctx.WriteError(store.ErrValueTooLarge)
		}
		estimated += int64(len(k) + len(ctx.Arg(i+1)) + 128)
	}

	for i := 0; i < ctx.ArgCount(); i += 2 {
		if ctx.Store.Exists(ctx.ArgString(i)) {
			return ctx.WriteInteger(0)
		}
	}

	if mt := ctx.Store.MemoryTracker(); mt != nil && !mt.CanAllocate(estimated) {
		return ctx.WriteError(store.ErrMemoryLimit)
	}

	for i := 0; i < ctx.ArgCount(); i += 2 {
		if err := ctx.Store.Set(ctx.ArgString(i), &store.StringValue{Data: ctx.Arg(i + 1)}, store.SetOptions{}); err != nil {
			// Reachable only under concurrent pressure after the pre-flight:
			// report it rather than hiding a partial application behind :1.
			return ctx.WriteError(err)
		}
	}
	return ctx.WriteInteger(1)
}

func cmdGETSET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	newValue := ctx.Arg(1)

	entry, exists := ctx.Store.Get(key)

	var data []byte
	if exists {
		var ok bool
		data, ok = stringValueOf(entry.Value)
		if !ok {
			return ctx.WriteError(store.ErrWrongType)
		}
	}

	if err := ctx.Store.Set(key, &store.StringValue{Data: newValue}, store.SetOptions{}); err != nil {
		return ctx.WriteError(err)
	}
	if !exists {
		return ctx.WriteNullBulkString()
	}

	return ctx.WriteBulkBytes(data)
}

func cmdGETDEL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteNullBulkString()
	}

	data, ok := stringValueOf(entry.Value)
	if !ok {
		return ctx.WriteError(store.ErrWrongType)
	}

	result := make([]byte, len(data))
	copy(result, data)

	ctx.Store.Delete(key)

	return ctx.WriteBulkBytes(result)
}

func cmdGETEX(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	flags, expireArg, err := parseStringOptions(ctx.Args[1:], false)
	if err != nil {
		return ctx.WriteError(err)
	}

	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteNullBulkString()
	}
	data, ok := stringValueOf(entry.Value)
	if !ok {
		return ctx.WriteError(store.ErrWrongType)
	}
	var whenMs int64
	if expireArg != nil {
		if whenMs, err = stringOptionsExpireMs(flags, expireArg, "getex"); err != nil {
			return ctx.WriteError(err)
		}
	}

	result := make([]byte, len(data))
	copy(result, data)

	switch {
	case expireArg != nil && whenMs <= time.Now().UnixMilli():
		ctx.Store.Delete(key)
	case expireArg != nil:
		ctx.Store.SetExpiresAt(key, expireMsToNs(whenMs))
	case flags&strOptPERSIST != 0:
		ctx.Store.Persist(key)
	}

	return ctx.WriteBulkBytes(result)
}

func cmdINCRBYFLOAT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)
	current := new(big.Float).SetPrec(longDoublePrec)
	if exists {
		data, ok := stringValueOf(entry.Value)
		if !ok {
			return ctx.WriteError(store.ErrWrongType)
		}
		if current, ok = parseLongDouble(data); !ok {
			return ctx.WriteError(ErrNotFloat)
		}
	}
	incr, ok := parseLongDouble(ctx.Arg(1))
	if !ok {
		return ctx.WriteError(ErrNotFloat)
	}

	formatted, ok := addLongDouble(current, incr)
	if !ok {
		return ctx.WriteError(ErrFloatOverflow)
	}
	opts := store.SetOptions{KeepTTL: true}
	if err := ctx.Store.Set(key, &store.StringValue{Data: []byte(formatted)}, opts); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(formatted)
}

// Redis computes INCRBYFLOAT in C long double, which on x86-64 is the x87
// 80-bit format: a 64-bit mantissa and a binary exponent of at most 16383.
// big.Float at 64 bits with round-to-nearest-even reproduces its parsing and
// addition exactly, so results such as 0.1+0.2 come out as Redis prints them.
const (
	longDoublePrec   = 64
	longDoubleMaxExp = 16384  // MantExp of LDBL_MAX
	longDoubleMinExp = -16444 // MantExp of the smallest subnormal
)

// parseLongDouble mirrors Redis' string2ld (strtold): no leading space, no
// trailing garbage, NaN rejected, and out-of-range values rejected.
func parseLongDouble(b []byte) (*big.Float, bool) {
	if len(b) == 0 || len(b) >= 5*1024 {
		return nil, false
	}
	switch b[0] {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return nil, false
	}
	s := string(b)
	body := strings.ToLower(strings.TrimLeft(s, "+-"))
	if len(s)-len(body) > 1 {
		return nil, false
	}
	if body == "inf" || body == "infinity" {
		return new(big.Float).SetInf(s[0] == '-'), true
	}
	if strings.ContainsRune(body, '_') || strings.HasPrefix(body, "0b") || strings.HasPrefix(body, "0o") || strings.HasPrefix(body, "nan") {
		return nil, false
	}
	f, _, err := big.ParseFloat(s, 0, longDoublePrec, big.ToNearestEven)
	if err != nil || f.IsInf() {
		return nil, false
	}
	if f.Sign() != 0 {
		if exp := f.MantExp(nil); exp > longDoubleMaxExp || exp < longDoubleMinExp {
			return nil, false
		}
	}
	return f, true
}

// addLongDouble adds two long doubles and formats the sum the way Redis'
// ld2string(LD_STR_HUMAN) does: "%.17Lf" with trailing zeros (and a bare
// trailing dot) removed, and "-0" printed as "0". It reports false when the
// sum is NaN or infinite.
func addLongDouble(x, y *big.Float) (string, bool) {
	if x.IsInf() || y.IsInf() {
		return "", false
	}
	sum := new(big.Float).SetPrec(longDoublePrec).SetMode(big.ToNearestEven).Add(x, y)
	if sum.Sign() != 0 && sum.MantExp(nil) > longDoubleMaxExp {
		return "", false
	}
	out := sum.Text('f', 17)
	if strings.IndexByte(out, '.') >= 0 {
		out = strings.TrimRight(out, "0")
		out = strings.TrimSuffix(out, ".")
	}
	if out == "-0" {
		out = "0"
	}
	return out, true
}

func cmdLCS(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key1 := ctx.ArgString(0)
	key2 := ctx.ArgString(1)

	idx := false
	lenOnly := false
	minMatchLen := 1
	withMatchLen := false

	for i := 2; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "IDX":
			idx = true
		case "LEN":
			lenOnly = true
		case "MINMATCHLEN":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			minMatchLen, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			if minMatchLen < 1 {
				minMatchLen = 1
			}
		case "WITHMATCHLEN":
			withMatchLen = true
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	entry1, exists1 := ctx.Store.Get(key1)
	entry2, exists2 := ctx.Store.Get(key2)

	if !exists1 || !exists2 {
		if lenOnly {
			return ctx.WriteInteger(0)
		}
		if idx {
			// The requested reply shape applies to empty results too.
			return ctx.WriteArray([]*resp.Value{
				resp.BulkString("matches"),
				resp.ArrayValue([]*resp.Value{}),
				resp.BulkString("len"),
				resp.IntegerValue(0),
			})
		}
		return ctx.WriteBulkString("")
	}

	strVal1, ok1 := entry1.Value.(*store.StringValue)
	strVal2, ok2 := entry2.Value.(*store.StringValue)

	if !ok1 || !ok2 {
		return ctx.WriteError(store.ErrWrongType)
	}

	s1 := string(strVal1.Data)
	s2 := string(strVal2.Data)

	if len(s1) == 0 || len(s2) == 0 {
		if lenOnly {
			return ctx.WriteInteger(0)
		}
		if idx {
			// The requested reply shape applies to empty results too.
			return ctx.WriteArray([]*resp.Value{
				resp.BulkString("matches"),
				resp.ArrayValue([]*resp.Value{}),
				resp.BulkString("len"),
				resp.IntegerValue(0),
			})
		}
		return ctx.WriteBulkString("")
	}

	m := len(s1)
	n := len(s2)

	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if s1[i-1] == s2[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	lcsLen := dp[m][n]

	if lenOnly {
		return ctx.WriteInteger(int64(lcsLen))
	}

	if !idx {
		result := make([]byte, 0, lcsLen)
		i, j := m, n
		for i > 0 && j > 0 {
			if s1[i-1] == s2[j-1] {
				result = append([]byte{s1[i-1]}, result...)
				i--
				j--
			} else if dp[i-1][j] > dp[i][j-1] {
				i--
			} else {
				j--
			}
		}
		return ctx.WriteBulkString(string(result))
	}

	type Match struct {
		A1, A2 int
		B1, B2 int
		Len    int
	}
	matches := make([]Match, 0)

	i, j := m, n
	for i > 0 && j > 0 {
		if s1[i-1] == s2[j-1] {
			endI, endJ := i, j
			for i > 0 && j > 0 && s1[i-1] == s2[j-1] {
				i--
				j--
			}
			matchLen := endI - i
			if matchLen >= minMatchLen {
				matches = append([]Match{{A1: i, A2: endI - 1, B1: j, B2: endJ - 1, Len: matchLen}}, matches...)
			}
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	result := make([]*resp.Value, 0, len(matches)*2+2)
	result = append(result, resp.BulkString("matches"))
	matchArray := make([]*resp.Value, 0, len(matches))
	for _, match := range matches {
		matchEntry := []*resp.Value{
			resp.ArrayValue([]*resp.Value{
				resp.IntegerValue(int64(match.A1)),
				resp.IntegerValue(int64(match.A2)),
			}),
			resp.ArrayValue([]*resp.Value{
				resp.IntegerValue(int64(match.B1)),
				resp.IntegerValue(int64(match.B2)),
			}),
		}
		if withMatchLen {
			matchEntry = append(matchEntry, resp.IntegerValue(int64(match.Len)))
		}
		matchArray = append(matchArray, resp.ArrayValue(matchEntry))
	}
	result = append(result, resp.ArrayValue(matchArray), resp.BulkString("len"), resp.IntegerValue(int64(lcsLen)))

	return ctx.WriteArray(result)
}
