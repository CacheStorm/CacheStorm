package command

import (
	"errors"
	"math"
	"math/big"
	"math/rand"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterHashCommands(router *Router) {
	router.Register(&CommandDef{Name: "HSET", Handler: cmdHSET})
	router.Register(&CommandDef{Name: "HGET", Handler: cmdHGET})
	router.Register(&CommandDef{Name: "HMSET", Handler: cmdHMSET})
	router.Register(&CommandDef{Name: "HMGET", Handler: cmdHMGET})
	router.Register(&CommandDef{Name: "HGETALL", Handler: cmdHGETALL})
	router.Register(&CommandDef{Name: "HDEL", Handler: cmdHDEL})
	router.Register(&CommandDef{Name: "HEXISTS", Handler: cmdHEXISTS})
	router.Register(&CommandDef{Name: "HLEN", Handler: cmdHLEN})
	router.Register(&CommandDef{Name: "HKEYS", Handler: cmdHKEYS})
	router.Register(&CommandDef{Name: "HVALS", Handler: cmdHVALS})
	router.Register(&CommandDef{Name: "HINCRBY", Handler: cmdHINCRBY})
	router.Register(&CommandDef{Name: "HINCRBYFLOAT", Handler: cmdHINCRBYFLOAT})
	router.Register(&CommandDef{Name: "HSETNX", Handler: cmdHSETNX})
	router.Register(&CommandDef{Name: "HSTRLEN", Handler: cmdHSTRLEN})
	router.Register(&CommandDef{Name: "HSCAN", Handler: cmdHSCAN})
	router.Register(&CommandDef{Name: "HRANDFIELD", Handler: cmdHRANDFIELD})
	router.Register(&CommandDef{Name: "HGETDEL", Handler: cmdHGETDEL})
	RegisterHashTTLCommands(router)
}

func getHash(ctx *Context, key string) (*store.HashValue, error) {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return nil, nil
	}

	hash, ok := entry.Value.(*store.HashValue)
	if !ok {
		return nil, store.ErrWrongType
	}

	return hash, nil
}

func applyHashSetFields(ctx *Context, nx bool) (int, error) {
	return ctx.Store.ApplyHashPairs(ctx.ArgString(0), nx, ctx.Args[1:])
}

func cmdHSET(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount()%2 == 0 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	ctx.writeEffectsHandled = true
	added, err := applyHashSetFields(ctx, false)
	if err != nil {
		return ctx.WriteError(err)
	}
	ctx.notify(store.NotifyHash, "hset", ctx.ArgString(0))
	return ctx.WriteInteger(int64(added))
}

func cmdHGET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	field := ctx.ArgString(1)

	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.WriteNullBulkString()
	}

	hash.RLock()
	value, exists := hash.Fields[field]
	if !exists {
		hash.RUnlock()
		return ctx.WriteNullBulkString()
	}

	reply := value
	hash.RUnlock()
	return ctx.WriteBulkBytes(reply)
}

func cmdHMSET(ctx *Context) error {
	if ctx.ArgCount() < 3 || ctx.ArgCount()%2 == 0 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	ctx.writeEffectsHandled = true
	if _, err := applyHashSetFields(ctx, false); err != nil {
		return ctx.WriteError(err)
	}
	ctx.notify(store.NotifyHash, "hset", ctx.ArgString(0))
	return ctx.WriteOK()
}

func cmdHMGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	results := make([]*resp.Value, ctx.ArgCount()-1)
	if hash == nil {
		for i := 1; i < ctx.ArgCount(); i++ {
			results[i-1] = resp.NullBulkString()
		}
		return ctx.WriteArray(results)
	}

	hash.RLock()
	for i := 1; i < ctx.ArgCount(); i++ {
		field := ctx.ArgString(i)
		value, exists := hash.Fields[field]
		if !exists {
			results[i-1] = resp.NullBulkString()
		} else {
			results[i-1] = resp.BulkBytes(value)
		}
	}

	reply := results
	hash.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdHGETALL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.Writer.WriteValue(resp.PairsValue([]*resp.Value{}))
	}

	hash.RLock()
	results := make([]*resp.Value, 0, len(hash.Fields)*2)
	for field, value := range hash.Fields {
		results = append(results, resp.BulkString(field), resp.BulkBytes(value))
	}

	reply := results
	hash.RUnlock()
	return ctx.Writer.WriteValue(resp.PairsValue(reply))
}

func cmdHDEL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	hash, err := lockHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.WriteInteger(0)
	}

	deleted := 0
	now := ctx.Store.HashFieldClock()
	for i := 1; i < ctx.ArgCount(); i++ {
		field := ctx.ArgString(i)
		if hash.FieldLive(field, now) {
			hash.DeleteField(field)
			deleted++
		}
	}

	isEmpty := len(hash.Fields) == 0
	hash.Unlock()

	if deleted > 0 {
		ctx.notify(store.NotifyHash, "hdel", key)
	}
	if isEmpty {
		deleteIfEmpty(ctx, key, hash)
	}

	return ctx.WriteInteger(int64(deleted))
}

func cmdHEXISTS(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	field := ctx.ArgString(1)

	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.WriteInteger(0)
	}

	hash.RLock()
	if _, exists := hash.Fields[field]; exists {
		reply := int64(1)
		hash.RUnlock()
		return ctx.WriteInteger(reply)
	}
	reply := int64(0)
	hash.RUnlock()
	return ctx.WriteInteger(reply)
}

func cmdHLEN(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.WriteInteger(0)
	}

	hash.RLock()
	reply := int64(int64(len(hash.Fields)))
	hash.RUnlock()
	return ctx.WriteInteger(reply)
}

func cmdHKEYS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	hash.RLock()
	keys := make([]*resp.Value, 0, len(hash.Fields))
	for field := range hash.Fields {
		keys = append(keys, resp.BulkString(field))
	}

	reply := keys
	hash.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdHVALS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	hash.RLock()
	vals := make([]*resp.Value, 0, len(hash.Fields))
	for _, value := range hash.Fields {
		vals = append(vals, resp.BulkBytes(value))
	}

	reply := vals
	hash.RUnlock()
	return ctx.WriteArray(reply)
}

var (
	errHashNotInteger   = errors.New("ERR hash value is not an integer")
	errHashNotFloat     = errors.New("ERR hash value is not a float")
	errHashIncrNaNOrInf = errors.New("ERR value is NaN or Infinity")
)

func cmdHINCRBY(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	ctx.writeEffectsHandled = true
	field := ctx.ArgString(1)
	incr, err := strconv.ParseInt(ctx.ArgString(2), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	var newVal int64
	err = ctx.Store.ApplyHashFields(ctx.ArgString(0), func(hash *store.HashValue) (map[string][]byte, error) {
		newVal = incr
		if current, exists := hash.LiveField(field, ctx.Store.HashFieldClock()); exists {
			var err error
			newVal, err = computeIntIncr(current, incr)
			if err == ErrNotInteger {
				err = errHashNotInteger
			}
			if err != nil {
				return nil, err
			}
		}
		return map[string][]byte{field: []byte(strconv.FormatInt(newVal, 10))}, nil
	})
	if err != nil {
		return ctx.WriteError(err)
	}
	ctx.notify(store.NotifyHash, "hincrby", ctx.ArgString(0))
	return ctx.WriteInteger(newVal)
}

// longDoublePrec is the mantissa width of the x87 long double Redis uses
// for HINCRBYFLOAT and INCRBYFLOAT.
const longDoublePrec = 64

// parseLongDouble parses a number the way Redis' string2ld does, at long
// double precision. Infinities are returned as such; NaN and malformed input
// are refused.
func parseLongDouble(s string) (*big.Float, bool) {
	s = cHexFloat(s)
	f, err := strconv.ParseFloat(s, 64)
	rangeErr := errors.Is(err, strconv.ErrRange)
	if (err != nil && !rangeErr) || math.IsNaN(f) {
		return nil, false
	}
	if math.IsInf(f, 0) && !rangeErr {
		return new(big.Float).SetInf(f < 0), true
	}
	v, _, err := big.ParseFloat(s, 0, longDoublePrec, big.ToNearestEven)
	if err != nil {
		if rangeErr {
			return nil, false
		}
		return new(big.Float).SetPrec(longDoublePrec).SetFloat64(f), true
	}
	if rangeErr {
		// Beyond double range: accept what a long double still holds.
		exp := v.MantExp(nil)
		if v.Sign() != 0 && (exp > longDoubleMaxExp || exp < longDoubleMinExp) {
			return nil, false
		}
	}
	return v, true
}

// The binary exponent range of an x87 long double (as reported by
// big.Float.MantExp, mantissa in [0.5, 1)).
const (
	longDoubleMaxExp = 16384
	longDoubleMinExp = -16444
)

// formatLongDoubleHuman renders v like Redis' ld2string in LD_STR_HUMAN
// mode: "%.17Lf" with trailing zeroes (and a bare point) removed.
func formatLongDoubleHuman(v *big.Float) string {
	s := v.Text('f', 17)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimSuffix(s, ".")
	}
	if s == "-0" {
		s = "0"
	}
	return s
}

func cmdHINCRBYFLOAT(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	ctx.writeEffectsHandled = true
	field := ctx.ArgString(1)
	incr, ok := parseLongDouble(ctx.ArgString(2))
	if !ok {
		return ctx.WriteError(ErrNotFloat)
	}
	if incr.IsInf() {
		return ctx.WriteError(errHashIncrNaNOrInf)
	}
	var result string
	err := ctx.Store.ApplyHashFields(ctx.ArgString(0), func(hash *store.HashValue) (map[string][]byte, error) {
		sum := new(big.Float).SetPrec(longDoublePrec).SetMode(big.ToNearestEven)
		if current, exists := hash.LiveField(field, ctx.Store.HashFieldClock()); exists {
			cur, ok := parseLongDouble(string(current))
			if !ok {
				return nil, errHashNotFloat
			}
			if cur.IsInf() {
				return nil, ErrFloatOverflow
			}
			sum.Add(cur, incr)
		} else {
			sum.Set(incr)
		}
		if sum.Sign() != 0 && sum.MantExp(nil) > longDoubleMaxExp {
			return nil, ErrFloatOverflow
		}
		result = formatLongDoubleHuman(sum)
		return map[string][]byte{field: []byte(result)}, nil
	})
	if err != nil {
		return ctx.WriteError(err)
	}
	ctx.notify(store.NotifyHash, "hincrbyfloat", ctx.ArgString(0))
	return ctx.WriteBulkString(result)
}

func cmdHSETNX(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	ctx.writeEffectsHandled = true
	added, err := applyHashSetFields(ctx, true)
	if err != nil {
		return ctx.WriteError(err)
	}
	if added == 0 {
		ctx.PropagateAs()
	} else {
		ctx.notify(store.NotifyHash, "hset", ctx.ArgString(0))
	}
	return ctx.WriteInteger(int64(added))
}

func cmdHSTRLEN(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	field := ctx.ArgString(1)

	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return ctx.WriteInteger(0)
	}

	hash.RLock()
	value, exists := hash.Fields[field]
	if !exists {
		reply := int64(0)
		hash.RUnlock()
		return ctx.WriteInteger(reply)
	}

	reply := int64(int64(len(value)))
	hash.RUnlock()
	return ctx.WriteInteger(reply)
}

func cmdHSCAN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	cursor, err := parseCollectionScanCursor(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}

	hash, err := getHash(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil {
		return writeEmptyCollectionScan(ctx)
	}

	opts, err := parseCollectionScanOptions(ctx, 2, true)
	if err != nil {
		return ctx.WriteError(err)
	}

	hash.RLock()
	fields, next := collectionScanPage(len(hash.Fields), cursor, opts.count, func(emit func(string)) {
		for f := range hash.Fields {
			emit(f)
		}
	})
	items := make([]*resp.Value, 0, len(fields)*2)
	for _, f := range fields {
		if opts.match && !matchPattern(f, opts.pattern) {
			continue
		}
		items = append(items, resp.BulkString(f))
		if !opts.noValues {
			items = append(items, resp.BulkBytes(hash.Fields[f]))
		}
	}
	hash.RUnlock()

	return writeCollectionScanReply(ctx, next, items)
}

func cmdHRANDFIELD(ctx *Context) error {
	n := ctx.ArgCount()
	if n < 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	if n == 1 {
		hash, err := getHash(ctx, key)
		if err != nil {
			return ctx.WriteError(err)
		}
		if hash == nil {
			return ctx.WriteNullBulkString()
		}
		hash.RLock()
		var picked string
		found := false
		if len(hash.Fields) > 0 {
			idx := rand.Intn(len(hash.Fields))
			for f := range hash.Fields {
				if idx == 0 {
					picked, found = f, true
					break
				}
				idx--
			}
		}
		hash.RUnlock()
		if !found {
			return ctx.WriteNullBulkString()
		}
		return ctx.WriteBulkString(picked)
	}

	count, err := parseRandCount(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	if n > 3 || (n == 3 && !strings.EqualFold(ctx.ArgString(2), "WITHVALUES")) {
		return ctx.WriteError(ErrSyntaxError)
	}
	withValues := n == 3
	if withValues && (count < -math.MaxInt64/2 || count > math.MaxInt64/2) {
		return ctx.WriteError(errValueOutOfRange)
	}

	hash, err := getHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if hash == nil || count == 0 {
		return ctx.WriteArray([]*resp.Value{})
	}

	hash.RLock()
	fields := make([]string, 0, len(hash.Fields))
	values := make([][]byte, 0, len(hash.Fields))
	for f, v := range hash.Fields {
		fields = append(fields, f)
		values = append(values, v)
	}
	hash.RUnlock()

	picked := pickRandom(len(fields), count)
	size := len(picked)
	if withValues {
		size *= 2
	}
	result := make([]*resp.Value, 0, size)
	for _, idx := range picked {
		result = append(result, resp.BulkString(fields[idx]))
		if withValues {
			result = append(result, resp.BulkBytes(values[idx]))
		}
	}
	return ctx.WriteArray(result)
}

// cmdHGETDEL is Redis 8's HGETDEL key FIELDS numfields field ...: it
// returns the values of the fields, nil for a missing one, and deletes them,
// and the key once the hash is empty. It is recorded as an HDEL of the
// fields it deleted, or not at all.
func cmdHGETDEL(ctx *Context) error {
	ctx.writeEffectsHandled = true
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	if _, err := getHash(ctx, key); err != nil {
		return ctx.WriteError(err)
	}
	fields, err := hfeFields(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	hash, err := lockHash(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	results := make([]*resp.Value, len(fields))
	if hash == nil {
		for i := range results {
			results[i] = resp.NullBulkString()
		}
		ctx.PropagateAs()
		return ctx.WriteArray(results)
	}
	var deleted []string
	now := ctx.Store.HashFieldClock()
	for i, f := range fields {
		value, ok := hash.Fields[f]
		if !ok || !hash.FieldLive(f, now) {
			results[i] = resp.NullBulkString()
			continue
		}
		results[i] = resp.BulkBytes(value)
		hash.DeleteField(f)
		deleted = append(deleted, f)
	}
	empty := len(hash.Fields) == 0
	hash.Unlock()
	if len(deleted) == 0 {
		ctx.PropagateAs()
		return ctx.WriteArray(results)
	}
	ctx.notify(store.NotifyHash, "hdel", key)
	if empty {
		deleteIfEmpty(ctx, key, hash)
	}
	ctx.Store.TouchKey(key)
	ctx.PropagateAs(ExecutedCommand{Name: "HDEL", Args: hfeArgs(key, deleted...), DB: ctx.DB})
	return ctx.WriteArray(results)
}
