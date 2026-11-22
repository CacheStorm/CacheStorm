package command

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/sparsearray"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Redis 8.8's array type (t_array.c): a sparse array of strings indexed
// from 0 to 2^64-2.

func RegisterArrayCommands(router *Router) {
	for name, h := range map[string]func(*Context) error{
		"ARGET": cmdARGET, "ARMGET": cmdARMGET, "ARSET": cmdARSET, "ARMSET": cmdARMSET,
		"ARDEL": cmdARDEL, "ARDELRANGE": cmdARDELRANGE, "ARLEN": cmdARLEN, "ARCOUNT": cmdARCOUNT,
		"ARGETRANGE": cmdARGETRANGE, "ARSCAN": cmdARSCAN, "ARGREP": cmdARGREP, "AROP": cmdAROP,
		"ARINSERT": cmdARINSERT, "ARRING": cmdARRING, "ARNEXT": cmdARNEXT, "ARSEEK": cmdARSEEK,
		"ARLASTITEMS": cmdARLASTITEMS, "ARINFO": cmdARINFO,
	} {
		router.Register(&CommandDef{Name: name, Handler: h})
	}
}

const argetrangeMaxItems = 1000000

var (
	errArrayIndex         = errors.New("ERR invalid array index")
	errArrayIndexOverflow = errors.New("ERR array index overflow")
	errArrayInsertOverlow = errors.New("ERR insert index overflow")
	errArrayLimit         = errors.New("ERR LIMIT must be positive")
)

// parseArrayIndex is getArrayIndexFromObject: string2ull, which falls back
// to strtoull (leading blanks, '+', leading zeros, and a negative wrapping
// around), refusing 2^64-1 unless allowMax.
func parseArrayIndex(s string, allowMax bool) (uint64, bool) {
	if v, ok := parseRedisInt([]byte(s)); ok {
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	}
	if i := strings.IndexByte(s, 0); i >= 0 {
		s = s[:i]
	}
	v, ok := cStrtoull(s)
	if !ok || (v == math.MaxUint64 && !allowMax) {
		return 0, false
	}
	return v, true
}

// cStrtoull is strtoull(s, &end, 10) required to consume all of s.
func cStrtoull(s string) (uint64, bool) {
	i := 0
	for i < len(s) && (s[i] == ' ' || (s[i] >= '\t' && s[i] <= '\r')) {
		i++
	}
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	if i == len(s) {
		return 0, false
	}
	var v uint64
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		d := uint64(c - '0')
		if v > (math.MaxUint64-d)/10 {
			return 0, false
		}
		v = v*10 + d
	}
	if neg {
		v = -v
	}
	return v, true
}

func arrayIndexArg(ctx *Context, i int) (uint64, error) {
	v, ok := parseArrayIndex(ctx.ArgString(i), false)
	if !ok {
		return 0, errArrayIndex
	}
	return v, nil
}

// readArray returns the array at key read-locked, nil when missing.
func readArray(ctx *Context, key string) (*store.ArrayValue, error) {
	entry, ok := ctx.Store.Get(key)
	if !ok {
		return nil, nil
	}
	a, ok := entry.Value.(*store.ArrayValue)
	if !ok {
		return nil, store.ErrWrongType
	}
	a.RLock()
	return a, nil
}

func arrayElem(v string, ok bool) *resp.Value {
	if !ok {
		return resp.NullBulkString()
	}
	return resp.BulkString(v)
}

func uintReply(n uint64) *resp.Value { return resp.UintValue(n) }

func writeUint(ctx *Context, n uint64) error { return ctx.WriteValue(resp.UintValue(n)) }

func cmdARGET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError("arget"))
	}
	a, err := readArray(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	idx, err := arrayIndexArg(ctx, 1)
	if err != nil {
		if a != nil {
			a.RUnlock()
		}
		return ctx.WriteError(err)
	}
	if a == nil {
		return ctx.WriteNull()
	}
	v, ok := a.Array().Get(idx)
	a.RUnlock()
	if !ok {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(v)
}

func cmdARMGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError("armget"))
	}
	a, err := readArray(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if a != nil {
		defer a.RUnlock()
	}
	idxs := make([]uint64, ctx.ArgCount()-1)
	for i := range idxs {
		if idxs[i], err = arrayIndexArg(ctx, i+1); err != nil {
			return ctx.WriteError(err)
		}
	}
	out := make([]*resp.Value, len(idxs))
	for i, idx := range idxs {
		if a == nil {
			out[i] = resp.NullBulkString()
			continue
		}
		out[i] = arrayElem(a.Array().Get(idx))
	}
	return ctx.WriteArray(out)
}

func lockOrCreateArray(ctx *Context, key string) (*store.ArrayValue, error) {
	a, _, err := lockOrCreate(ctx, key, store.NewArrayValue)
	return a, err
}

func cmdARSET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError("arset"))
	}
	start, err := arrayIndexArg(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	n := uint64(ctx.ArgCount() - 2)
	last := start + n - 1
	if last < start || last == math.MaxUint64 {
		return ctx.WriteError(errArrayIndexOverflow)
	}
	key := ctx.ArgString(0)
	a, err := lockOrCreateArray(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	ar := a.Array()
	before := ar.Count()
	if n > 1 {
		ar.MayPromoteToDenseForRangeSet(start, last)
	}
	for i := uint64(0); i < n; i++ {
		ar.Set(start+i, ctx.ArgString(2+int(i)))
	}
	added := ar.Count() - before
	a.Unlock()
	ctx.notify(store.NotifyArray, "arset", key)
	return ctx.WriteInteger(int64(added))
}

func cmdARMSET(ctx *Context) error {
	if ctx.ArgCount() < 3 || (ctx.ArgCount()-1)%2 != 0 {
		return ctx.WriteError(arityError("armset"))
	}
	idxs := make([]uint64, (ctx.ArgCount()-1)/2)
	for i := range idxs {
		var err error
		if idxs[i], err = arrayIndexArg(ctx, 1+2*i); err != nil {
			return ctx.WriteError(err)
		}
	}
	key := ctx.ArgString(0)
	a, err := lockOrCreateArray(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	ar := a.Array()
	before := ar.Count()
	for i, idx := range idxs {
		ar.Set(idx, ctx.ArgString(2+2*i))
	}
	added := ar.Count() - before
	a.Unlock()
	ctx.notify(store.NotifyArray, "armset", key)
	return ctx.WriteInteger(int64(added))
}

// finishArrayDelete reports what ARDEL and ARDELRANGE did and deletes the
// key they emptied; the array is unlocked.
func finishArrayDelete(ctx *Context, key, event string, a *store.ArrayValue, deleted uint64, empty bool) error {
	if deleted == 0 {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	ctx.notify(store.NotifyArray, event, key)
	if empty {
		deleteIfEmpty(ctx, key, a)
	}
	return writeUint(ctx, deleted)
}

func cmdARDEL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError("ardel"))
	}
	idxs := make([]uint64, ctx.ArgCount()-1)
	for i := range idxs {
		var err error
		if idxs[i], err = arrayIndexArg(ctx, i+1); err != nil {
			return ctx.WriteError(err)
		}
	}
	key := ctx.ArgString(0)
	a, err := lockExisting[*store.ArrayValue](ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if a == nil {
		return ctx.WriteInteger(0)
	}
	deleted := uint64(0)
	for _, idx := range idxs {
		if a.Array().Del(idx) {
			deleted++
		}
	}
	empty := a.Array().Count() == 0
	a.Unlock()
	return finishArrayDelete(ctx, key, "ardel", a, deleted, empty)
}

func cmdARDELRANGE(ctx *Context) error {
	if ctx.ArgCount() < 3 || (ctx.ArgCount()-1)%2 != 0 {
		return ctx.WriteError(arityError("ardelrange"))
	}
	bounds := make([]uint64, ctx.ArgCount()-1)
	for i := range bounds {
		var err error
		if bounds[i], err = arrayIndexArg(ctx, i+1); err != nil {
			return ctx.WriteError(err)
		}
	}
	key := ctx.ArgString(0)
	a, err := lockExisting[*store.ArrayValue](ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if a == nil {
		return ctx.WriteInteger(0)
	}
	deleted := uint64(0)
	for i := 0; i < len(bounds); i += 2 {
		lo, hi := bounds[i], bounds[i+1]
		if lo > hi {
			lo, hi = hi, lo
		}
		deleted += a.Array().DeleteRange(lo, hi)
	}
	empty := a.Array().Count() == 0
	a.Unlock()
	return finishArrayDelete(ctx, key, "ardelrange", a, deleted, empty)
}

func arrayCounter(ctx *Context, name string, f func(*sparsearray.Array) uint64) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(name))
	}
	a, err := readArray(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if a == nil {
		return ctx.WriteInteger(0)
	}
	n := f(a.Array())
	a.RUnlock()
	return writeUint(ctx, n)
}

func cmdARLEN(ctx *Context) error {
	return arrayCounter(ctx, "arlen", (*sparsearray.Array).Len)
}

func cmdARCOUNT(ctx *Context) error {
	return arrayCounter(ctx, "arcount", (*sparsearray.Array).Count)
}

func cmdARGETRANGE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError("argetrange"))
	}
	start, err := arrayIndexArg(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	end, err := arrayIndexArg(ctx, 2)
	if err != nil {
		return ctx.WriteError(err)
	}
	a, err := readArray(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if a != nil {
		defer a.RUnlock()
	}
	lo, hi := start, end
	if lo > hi {
		lo, hi = hi, lo
	}
	if hi-lo >= argetrangeMaxItems {
		return ctx.WriteError(fmt.Errorf("ERR range exceeds maximum of %d items", argetrangeMaxItems))
	}
	out := make([]*resp.Value, 0, hi-lo+1)
	for i := uint64(0); i <= hi-lo; i++ {
		idx := lo + i
		if start > end {
			idx = hi - i
		}
		if a == nil {
			out = append(out, resp.NullBulkString())
			continue
		}
		out = append(out, arrayElem(a.Array().Get(idx)))
	}
	return ctx.WriteArray(out)
}

func cmdARSCAN(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError("arscan"))
	}
	start, err := arrayIndexArg(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	end, err := arrayIndexArg(ctx, 2)
	if err != nil {
		return ctx.WriteError(err)
	}
	remaining := uint64(math.MaxUint64)
	switch ctx.ArgCount() {
	case 5:
		if !strings.EqualFold(ctx.ArgString(3), "LIMIT") {
			return ctx.WriteError(ErrSyntaxError)
		}
		n, ok := parseStreamLL(ctx.ArgString(4))
		if !ok {
			return ctx.WriteError(ErrNotInteger)
		}
		if n <= 0 {
			return ctx.WriteError(errArrayLimit)
		}
		remaining = uint64(n)
	case 3:
	default:
		return ctx.WriteError(arityError("arscan"))
	}
	a, err := readArray(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if a == nil {
		return ctx.WriteArray([]*resp.Value{})
	}
	var out []*resp.Value
	a.Array().Scan(start, end, func(idx uint64, v string) bool {
		out = append(out, resp.ArrayValue([]*resp.Value{uintReply(idx), resp.BulkString(v)}))
		remaining--
		return remaining > 0
	})
	a.RUnlock()
	if out == nil {
		out = []*resp.Value{}
	}
	return ctx.WriteArray(out)
}

const (
	argrepMaxPredicates = 250
	argrepMaxRELen      = 2048
)

type argrepPredicate struct {
	kind    string
	pattern string
	re      *regexp.Regexp
}

// argrepBound is "-" (the start), "+" (the end) or an index.
type argrepBound struct {
	sign byte
	idx  uint64
}

func parseArgrepBound(s string) (argrepBound, bool) {
	if s == "-" || s == "+" {
		return argrepBound{sign: s[0]}, true
	}
	v, ok := parseArrayIndex(s, false)
	return argrepBound{idx: v}, ok
}

func (b argrepBound) resolve(maxIndex uint64) uint64 {
	switch b.sign {
	case '-':
		return 0
	case '+':
		return maxIndex
	}
	return b.idx
}

func cmdARGREP(ctx *Context) error {
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(arityError("argrep"))
	}
	startB, ok := parseArgrepBound(ctx.ArgString(1))
	if !ok {
		return ctx.WriteError(errArrayIndex)
	}
	endB, ok := parseArgrepBound(ctx.ArgString(2))
	if !ok {
		return ctx.WriteError(errArrayIndex)
	}
	var preds []argrepPredicate
	and, withValues, nocase := false, false, false
	remaining := uint64(math.MaxUint64)
	argc := ctx.ArgCount()
	for i := 3; i < argc; {
		tok := ctx.ArgString(i)
		up := strings.ToUpper(tok)
		switch up {
		case "EXACT", "MATCH", "GLOB", "RE":
			if i+1 >= argc {
				return ctx.WriteError(ErrSyntaxError)
			}
			if len(preds) >= argrepMaxPredicates {
				return ctx.WriteError(fmt.Errorf("ERR too many predicates, maximum is %d", argrepMaxPredicates))
			}
			p := argrepPredicate{kind: up, pattern: ctx.ArgString(i + 1)}
			if up == "RE" && len(p.pattern) > argrepMaxRELen {
				return ctx.WriteError(fmt.Errorf("ERR regular expression is too long, maximum is %d bytes", argrepMaxRELen))
			}
			preds = append(preds, p)
			i += 2
		case "LIMIT":
			if i+1 >= argc {
				return ctx.WriteError(ErrSyntaxError)
			}
			n, ok := parseStreamLL(ctx.ArgString(i + 1))
			if !ok {
				return ctx.WriteError(ErrNotInteger)
			}
			if n <= 0 {
				return ctx.WriteError(errArrayLimit)
			}
			remaining = uint64(n)
			i += 2
		case "WITHVALUES":
			withValues = true
			i++
		case "NOCASE":
			nocase = true
			i++
		case "AND", "OR":
			and = up == "AND"
			i++
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}
	if len(preds) == 0 {
		return ctx.WriteError(ErrSyntaxError)
	}
	for i := range preds {
		p := &preds[i]
		switch p.kind {
		case "RE":
			if p.pattern == "" {
				return ctx.WriteError(errors.New("ERR regular expression is empty"))
			}
			re, err := compileArgrepRegex(p.pattern, nocase)
			if err != nil {
				return ctx.WriteError(err)
			}
			p.re = re
		case "GLOB":
			if nocase {
				p.pattern = asciiLower(p.pattern)
			}
		}
	}
	matchOne := func(p *argrepPredicate, v string) bool {
		switch p.kind {
		case "EXACT":
			return asciiEqual(v, p.pattern, nocase)
		case "MATCH":
			return asciiContains(v, p.pattern, nocase)
		case "GLOB":
			if nocase {
				v = asciiLower(v)
			}
			return matchPattern(v, p.pattern)
		}
		return p.re.MatchString(v)
	}
	matches := func(v string) bool {
		for i := range preds {
			hit := matchOne(&preds[i], v)
			if and && !hit {
				return false
			}
			if !and && hit {
				return true
			}
		}
		return and
	}

	a, err := readArray(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if a == nil {
		return ctx.WriteArray([]*resp.Value{})
	}
	defer a.RUnlock()
	ar := a.Array()
	out := []*resp.Value{}
	if ar.Len() == 0 || ar.Count() == 0 {
		return ctx.WriteArray(out)
	}
	maxIndex := ar.Len() - 1
	ar.Scan(startB.resolve(maxIndex), endB.resolve(maxIndex), func(idx uint64, v string) bool {
		if !matches(v) {
			return true
		}
		if withValues {
			out = append(out, resp.ArrayValue([]*resp.Value{uintReply(idx), resp.BulkString(v)}))
		} else {
			out = append(out, uintReply(idx))
		}
		remaining--
		return remaining > 0
	})
	return ctx.WriteArray(out)
}

func cmdAROP(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(arityError("arop"))
	}
	start, err := arrayIndexArg(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	end, err := arrayIndexArg(ctx, 2)
	if err != nil {
		return ctx.WriteError(err)
	}
	op := strings.ToUpper(ctx.ArgString(3))
	switch op {
	case "SUM", "MIN", "MAX", "AND", "OR", "XOR", "MATCH", "USED":
	default:
		return ctx.WriteError(errors.New("ERR unknown operation"))
	}
	var target string
	if op == "MATCH" {
		if ctx.ArgCount() != 5 {
			return ctx.WriteError(errors.New("ERR MATCH requires a value argument"))
		}
		target = ctx.ArgString(4)
	} else if ctx.ArgCount() != 4 {
		return ctx.WriteError(arityError("arop"))
	}
	entry, exists := ctx.Store.Get(ctx.ArgString(0))
	if !exists {
		if op == "MATCH" || op == "USED" {
			return ctx.WriteInteger(0)
		}
		return ctx.WriteNull()
	}
	a, ok := entry.Value.(*store.ArrayValue)
	if !ok {
		return ctx.WriteError(store.ErrWrongType)
	}
	a.RLock()
	var (
		count     int64
		acc       ldNum
		haveNum   bool
		bits      int64
		haveInt   bool
		bitwiseOp = op == "AND" || op == "OR" || op == "XOR"
	)
	a.Array().Scan(start, end, func(_ uint64, v string) bool {
		switch op {
		case "USED":
			count++
			return true
		case "MATCH":
			if v == target {
				count++
			}
			return true
		}
		n, iv, isInt, ok := arrayNumber(v)
		if !ok {
			return true
		}
		if bitwiseOp {
			if !isInt {
				var fits bool
				if iv, fits = n.int64Part(); !fits {
					return true
				}
			}
			switch {
			case !haveInt:
				bits, haveInt = iv, true
			case op == "AND":
				bits &= iv
			case op == "OR":
				bits |= iv
			default:
				bits ^= iv
			}
			return true
		}
		switch {
		case !haveNum:
			acc, haveNum = n, true
		case op == "SUM":
			acc = acc.add(n)
		case op == "MIN" && n.cmp(acc) < 0, op == "MAX" && n.cmp(acc) > 0:
			acc = n
		}
		return true
	})
	a.RUnlock()
	switch {
	case op == "MATCH" || op == "USED":
		return ctx.WriteInteger(count)
	case bitwiseOp:
		if !haveInt {
			return ctx.WriteNull()
		}
		return ctx.WriteInteger(bits)
	case !haveNum:
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(acc.format())
}

func cmdARINSERT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError("arinsert"))
	}
	key := ctx.ArgString(0)
	a, err := lockOrCreateArray(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	ar := a.Array()
	n := uint64(ctx.ArgCount() - 1)
	start := uint64(0)
	if ar.InsertIdx != sparsearray.InsertIdxNone {
		if ar.InsertIdx >= math.MaxUint64-1 {
			a.Unlock()
			return ctx.WriteError(errArrayInsertOverlow)
		}
		start = ar.InsertIdx + 1
	}
	last := start + n - 1
	if last < start || last == math.MaxUint64 {
		a.Unlock()
		return ctx.WriteError(errArrayInsertOverlow)
	}
	if n > 1 {
		ar.MayPromoteToDenseForRangeSet(start, last)
	}
	for i := uint64(0); i < n; i++ {
		ar.Set(start+i, ctx.ArgString(1+int(i)))
	}
	ar.InsertIdx = last
	a.Unlock()
	ctx.notify(store.NotifyArray, "arinsert", key)
	return writeUint(ctx, last)
}

func ringNextCursor(ar *sparsearray.Array) uint64 {
	if ar.InsertIdx == sparsearray.InsertIdxNone {
		return 0
	}
	return ar.InsertIdx + 1
}

// ringNeedsRework is arRingNeedsRework.
func ringNeedsRework(ar *sparsearray.Array, size uint64) (oldSpan, keepSpan uint64, rework bool) {
	oldSpan = ar.Len()
	switch {
	case oldSpan == 0, size == oldSpan, size > oldSpan && ar.InsertIdx == sparsearray.InsertIdxNone:
		return oldSpan, 0, false
	case size < oldSpan:
		return oldSpan, size, true
	case ringNextCursor(ar) < oldSpan:
		return oldSpan, oldSpan, true
	}
	return oldSpan, 0, false
}

// ringRework is arRingRework: the latest contiguous items, at most
// keepSpan, replayed oldest first from index 0 into a new array.
func ringRework(ar *sparsearray.Array, oldSpan, keepSpan uint64) *sparsearray.Array {
	out := store.NewArrayValue().Array()
	anchor := oldSpan - 1
	if ar.InsertIdx != sparsearray.InsertIdxNone {
		anchor = ar.InsertIdx % oldSpan
	}
	retained := uint64(0)
	src := anchor
	for retained < keepSpan {
		if _, ok := ar.Get(src); !ok {
			break
		}
		retained++
		if src == 0 {
			src = oldSpan - 1
		} else {
			src--
		}
	}
	src++
	if src == oldSpan {
		src = 0
	}
	for dst := uint64(0); dst < retained; dst++ {
		v, _ := ar.Get(src)
		out.Set(dst, v)
		src++
		if src == oldSpan {
			src = 0
		}
	}
	if retained != 0 {
		out.InsertIdx = retained - 1
	}
	return out
}

func cmdARRING(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError("arring"))
	}
	size, ok := parseStreamLL(ctx.ArgString(1))
	if !ok {
		return ctx.WriteError(errors.New("ERR invalid size"))
	}
	if size <= 0 {
		return ctx.WriteError(errors.New("ERR size must be positive"))
	}
	ringSize := uint64(size)
	key := ctx.ArgString(0)
	a, err := lockOrCreateArray(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if oldSpan, keepSpan, rework := ringNeedsRework(a.Array(), ringSize); rework {
		a.Replace(ringRework(a.Array(), oldSpan, keepSpan))
	}
	ar := a.Array()
	cursor := uint64(0)
	for i := 2; i < ctx.ArgCount(); i++ {
		cursor = ringNextCursor(ar)
		if cursor >= ringSize {
			cursor %= ringSize
		}
		ar.Set(cursor, ctx.ArgString(i))
		ar.InsertIdx = cursor
	}
	a.Unlock()
	ctx.notify(store.NotifyArray, "arring", key)
	return writeUint(ctx, cursor)
}

func cmdARNEXT(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError("arnext"))
	}
	a, err := readArray(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if a == nil {
		return ctx.WriteInteger(0)
	}
	idx := a.Array().InsertIdx
	a.RUnlock()
	switch idx {
	case sparsearray.InsertIdxNone:
		return ctx.WriteInteger(0)
	case math.MaxUint64 - 1:
		return ctx.WriteNull()
	}
	return writeUint(ctx, idx+1)
}

func cmdARSEEK(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError("arseek"))
	}
	idx, ok := parseArrayIndex(ctx.ArgString(1), true)
	if !ok {
		return ctx.WriteError(errArrayIndex)
	}
	key := ctx.ArgString(0)
	a, err := lockExisting[*store.ArrayValue](ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if a == nil {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	if idx == 0 {
		a.Array().InsertIdx = sparsearray.InsertIdxNone
	} else {
		a.Array().InsertIdx = idx - 1
	}
	a.Unlock()
	ctx.notify(store.NotifyArray, "arseek", key)
	return ctx.WriteInteger(1)
}

func cmdARLASTITEMS(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError("arlastitems"))
	}
	count, ok := parseStreamLL(ctx.ArgString(1))
	if !ok {
		return ctx.WriteError(errors.New("ERR invalid COUNT"))
	}
	if count <= 0 {
		return ctx.WriteArray([]*resp.Value{})
	}
	rev := false
	switch ctx.ArgCount() {
	case 3:
		if !strings.EqualFold(ctx.ArgString(2), "REV") {
			return ctx.WriteError(ErrSyntaxError)
		}
		rev = true
	case 2:
	default:
		return ctx.WriteError(arityError("arlastitems"))
	}
	a, err := readArray(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if a == nil {
		return ctx.WriteArray([]*resp.Value{})
	}
	defer a.RUnlock()
	ar := a.Array()
	n := min(uint64(count), ar.Count())
	if n == 0 {
		return ctx.WriteArray([]*resp.Value{})
	}
	length := ar.Len()
	cur := ar.InsertIdx
	if cur == sparsearray.InsertIdxNone {
		cur = length - 1
	}
	got := make([]*resp.Value, n)
	for i := uint64(0); i < n; i++ {
		got[i] = arrayElem(ar.Get(cur))
		if cur == 0 {
			cur = length - 1
		} else {
			cur--
		}
	}
	if !rev {
		for i, j := 0, len(got)-1; i < j; i, j = i+1, j-1 {
			got[i], got[j] = got[j], got[i]
		}
	}
	return ctx.WriteArray(got)
}

func cmdARINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(arityError("arinfo"))
	}
	full := false
	if ctx.ArgCount() > 1 {
		if ctx.ArgCount() != 2 || !strings.EqualFold(ctx.ArgString(1), "FULL") {
			return ctx.WriteError(ErrSyntaxError)
		}
		full = true
	}
	a, err := readArray(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if a == nil {
		return ctx.WriteError(errNoSuchKey)
	}
	ar := a.Array()
	info := ar.Info(full)
	insert := ar.InsertIdx
	a.RUnlock()
	next := uint64(0)
	if insert != sparsearray.InsertIdxNone && insert != math.MaxUint64-1 {
		next = insert + 1
	}
	out := []*resp.Value{
		resp.BulkString("count"), uintReply(info.Count),
		resp.BulkString("len"), uintReply(info.Len),
		resp.BulkString("next-insert-index"), uintReply(next),
		resp.BulkString("slices"), uintReply(info.NumSlices),
		resp.BulkString("directory-size"), uintReply(info.DirectorySize),
		resp.BulkString("super-dir-entries"), uintReply(info.SuperDirEntries),
		resp.BulkString("slice-size"), resp.IntegerValue(int64(info.SliceSize)),
	}
	if full {
		ratio := func(a, b uint64) *resp.Value {
			if b == 0 {
				return resp.DoubleValue("0")
			}
			return resp.DoubleValue(fpconvDtoa(float64(a) / float64(b)))
		}
		out = append(out,
			resp.BulkString("dense-slices"), uintReply(info.DenseSlices),
			resp.BulkString("sparse-slices"), uintReply(info.SparseSlices),
			resp.BulkString("avg-dense-size"), ratio(info.DenseTotalWinsize, info.DenseSlices),
			resp.BulkString("avg-dense-fill"), ratio(info.DenseTotalCount, info.DenseTotalWinsize),
			resp.BulkString("avg-sparse-size"), ratio(info.SparseTotalCap, info.SparseSlices),
		)
	}
	return ctx.WriteValue(resp.PairsValue(out))
}
