package command

import (
	"errors"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

var (
	errZScoreRange      = errors.New("ERR min or max is not a float")
	errZLexRange        = errors.New("ERR min or max not valid string range item")
	errZNaNScore        = errors.New("ERR resulting score is not a number (NaN)")
	errZWeightNotFloat  = errors.New("ERR weight value is not a float")
	errZLimitRank       = errors.New("ERR syntax error, LIMIT is only supported in combination with either BYSCORE or BYLEX")
	errZWithScoresLex   = errors.New("ERR syntax error, WITHSCORES not supported in combination with BYLEX")
	errZAddXXNX         = errors.New("ERR XX and NX options at the same time are not compatible")
	errZAddGTLTNX       = errors.New("ERR GT, LT, and/or NX options at the same time are not compatible")
	errZAddIncrPair     = errors.New("ERR INCR option supports a single increment-element pair")
	errNumKeysPositive  = errors.New("ERR numkeys should be greater than 0")
	errPopCountPositive = errors.New("ERR count should be greater than 0")
	errLimitNegative    = errors.New("ERR LIMIT can't be negative")
	errValueOutOfRange  = errors.New("ERR value is out of range")
)

func RegisterSortedSetCommands(router *Router) {
	router.Register(&CommandDef{Name: "ZADD", Handler: cmdZADD})
	router.Register(&CommandDef{Name: "ZCARD", Handler: cmdZCARD})
	router.Register(&CommandDef{Name: "ZCOUNT", Handler: cmdZCOUNT})
	router.Register(&CommandDef{Name: "ZINCRBY", Handler: cmdZINCRBY})
	router.Register(&CommandDef{Name: "ZRANGE", Handler: cmdZRANGE})
	router.Register(&CommandDef{Name: "ZRANGEBYSCORE", Handler: cmdZRANGEBYSCORE})
	router.Register(&CommandDef{Name: "ZRANGESTORE", Handler: cmdZRANGESTORE})
	router.Register(&CommandDef{Name: "ZRANK", Handler: cmdZRANK})
	router.Register(&CommandDef{Name: "ZREM", Handler: cmdZREM})
	router.Register(&CommandDef{Name: "ZREMRANGEBYRANK", Handler: cmdZREMRANGEBYRANK})
	router.Register(&CommandDef{Name: "ZREMRANGEBYSCORE", Handler: cmdZREMRANGEBYSCORE})
	router.Register(&CommandDef{Name: "ZSCORE", Handler: cmdZSCORE})
	router.Register(&CommandDef{Name: "ZREVRANGE", Handler: cmdZREVRANGE})
	router.Register(&CommandDef{Name: "ZREVRANK", Handler: cmdZREVRANK})
	router.Register(&CommandDef{Name: "ZREVRANGEBYSCORE", Handler: cmdZREVRANGEBYSCORE})
	router.Register(&CommandDef{Name: "ZLEXCOUNT", Handler: cmdZLEXCOUNT})
	router.Register(&CommandDef{Name: "ZRANGEBYLEX", Handler: cmdZRANGEBYLEX})
	router.Register(&CommandDef{Name: "ZREMRANGEBYLEX", Handler: cmdZREMRANGEBYLEX})
	router.Register(&CommandDef{Name: "ZSCAN", Handler: cmdZSCAN})
	router.Register(&CommandDef{Name: "ZPOPMIN", Handler: cmdZPOPMIN})
	router.Register(&CommandDef{Name: "ZPOPMAX", Handler: cmdZPOPMAX})
	router.Register(&CommandDef{Name: "ZRANDMEMBER", Handler: cmdZRANDMEMBER})
	router.Register(&CommandDef{Name: "ZMSCORE", Handler: cmdZMSCORE})
	router.Register(&CommandDef{Name: "ZUNIONSTORE", Handler: cmdZUNIONSTORE})
	router.Register(&CommandDef{Name: "ZINTERSTORE", Handler: cmdZINTERSTORE})
	router.Register(&CommandDef{Name: "ZDIFFSTORE", Handler: cmdZDIFFSTORE})
	router.Register(&CommandDef{Name: "ZUNION", Handler: cmdZUNION})
	router.Register(&CommandDef{Name: "ZINTER", Handler: cmdZINTER})
	router.Register(&CommandDef{Name: "ZDIFF", Handler: cmdZDIFF})
	router.Register(&CommandDef{Name: "ZINTERCARD", Handler: cmdZINTERCARD})
	router.Register(&CommandDef{Name: "ZMPOP", Handler: cmdZMPOP})
	router.Register(&CommandDef{Name: "BZPOPMIN", Handler: cmdBZPOPMIN})
	router.Register(&CommandDef{Name: "BZPOPMAX", Handler: cmdBZPOPMAX})
	router.Register(&CommandDef{Name: "BZMPOP", Handler: cmdBZMPOP})
	router.Register(&CommandDef{Name: "ZREVRANGEBYLEX", Handler: cmdZREVRANGEBYLEX})
}

func getSortedSet(ctx *Context, key string) (*store.SortedSetValue, error) {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return nil, nil
	}

	zset, ok := entry.Value.(*store.SortedSetValue)
	if !ok {
		return nil, store.ErrWrongType
	}

	return zset, nil
}

// formatZScore renders a score the way Redis 7.4 replies with it: integers
// that fit in half the int64 range print in full, other values use the
// shortest round-trip digits in fixed or exponent notation (fpconv_dtoa).
func formatZScore(f float64) string {
	switch {
	case math.IsNaN(f):
		return "nan"
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	case f == 0:
		return "0"
	}
	if f >= -4.611686018427387904e18 && f <= 4.611686018427387904e18 && f == math.Trunc(f) {
		return strconv.FormatInt(int64(f), 10)
	}
	return fpconvDtoa(f)
}

// fpconvDtoa mirrors the layout rules of Redis' fpconv_dtoa.
func fpconvDtoa(f float64) string {
	neg := f < 0
	if neg {
		f = -f
	}
	e := strconv.FormatFloat(f, 'e', -1, 64)
	mant, expPart, _ := strings.Cut(e, "e")
	digits := strings.Replace(mant, ".", "", 1)
	exp10, err := strconv.Atoi(expPart)
	if err != nil {
		return e
	}
	nd := len(digits)
	k := exp10 - (nd - 1)
	absExp := exp10
	if absExp < 0 {
		absExp = -absExp
	}

	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	switch {
	case k >= 0 && absExp < nd+7:
		b.WriteString(digits)
		b.WriteString(strings.Repeat("0", k))
	case k < 0 && (k > -7 || absExp < 4):
		offset := nd + k
		if offset <= 0 {
			b.WriteString("0.")
			b.WriteString(strings.Repeat("0", -offset))
			b.WriteString(digits)
		} else {
			b.WriteString(digits[:offset])
			b.WriteByte('.')
			b.WriteString(digits[offset:])
		}
	default:
		b.WriteByte(digits[0])
		if nd > 1 {
			b.WriteByte('.')
			b.WriteString(digits[1:])
		}
		b.WriteByte('e')
		if exp10 < 0 {
			b.WriteByte('-')
		} else {
			b.WriteByte('+')
		}
		b.WriteString(strconv.Itoa(absExp))
	}
	return b.String()
}

// cHexFloat adds the binary exponent Go requires to a hexadecimal float
// written without one ("0x10"), which C's strtod accepts.
func cHexFloat(s string) string {
	body := strings.TrimLeft(s, "+-")
	if len(body) > 2 && (body[:2] == "0x" || body[:2] == "0X") && !strings.ContainsAny(body, "pP") {
		return s + "p0"
	}
	return s
}

// parseZScore parses a score or increment as Redis' string2d does: NaN and
// out-of-range values are refused, infinities are accepted.
func parseZScore(s string) (float64, bool) {
	f, err := strconv.ParseFloat(cHexFloat(s), 64)
	if err != nil || math.IsNaN(f) {
		return 0, false
	}
	return f, true
}

// zScoreRange is a ZRANGEBYSCORE-style interval.
type zScoreRange struct {
	min, max     float64
	minex, maxex bool
}

func parseScoreBound(s string) (float64, bool, error) {
	ex := strings.HasPrefix(s, "(")
	if ex {
		s = s[1:]
	}
	if s == "" {
		return 0, ex, nil
	}
	f, err := strconv.ParseFloat(cHexFloat(s), 64)
	if (err != nil && !errors.Is(err, strconv.ErrRange)) || math.IsNaN(f) {
		return 0, ex, errZScoreRange
	}
	return f, ex, nil
}

// parseScoreRange parses a score interval such as "(1" "+inf".
func parseScoreRange(minStr, maxStr string) (min float64, minExclusive bool, max float64, maxExclusive bool, err error) {
	if min, minExclusive, err = parseScoreBound(minStr); err != nil {
		return
	}
	max, maxExclusive, err = parseScoreBound(maxStr)
	return
}

func parseZScoreRange(minStr, maxStr string) (zScoreRange, error) {
	min, minex, max, maxex, err := parseScoreRange(minStr, maxStr)
	return zScoreRange{min: min, max: max, minex: minex, maxex: maxex}, err
}

// zLexBound is one end of a lexicographic interval; inf is -1 for "-" and
// +1 for "+".
type zLexBound struct {
	val string
	ex  bool
	inf int
}

type zLexRange struct {
	min, max zLexBound
}

func parseZLexBound(s string) (zLexBound, bool) {
	switch {
	case s == "-":
		return zLexBound{inf: -1}, true
	case s == "+":
		return zLexBound{inf: 1}, true
	case strings.HasPrefix(s, "("):
		return zLexBound{val: s[1:], ex: true}, true
	case strings.HasPrefix(s, "["):
		return zLexBound{val: s[1:]}, true
	}
	return zLexBound{}, false
}

func parseZLexRange(minStr, maxStr string) (zLexRange, error) {
	min, ok1 := parseZLexBound(minStr)
	max, ok2 := parseZLexBound(maxStr)
	if !ok1 || !ok2 {
		return zLexRange{}, errZLexRange
	}
	return zLexRange{min: min, max: max}, nil
}

// zRankWindow maps a Redis start/stop rank pair onto [lo, hi] indexes of an
// n-element sorted set; ok is false when the window is empty.
func zRankWindow(n int, start, stop int64) (lo, hi int, ok bool) {
	ln := int64(n)
	if start < 0 {
		start += ln
	}
	if stop < 0 {
		stop += ln
	}
	if start < 0 {
		start = 0
	}
	if start > stop || start >= ln {
		return 0, 0, false
	}
	if stop >= ln {
		stop = ln - 1
	}
	return int(start), int(stop), true
}

// zRankSlice returns the members at ascending ranks first..last, in
// descending order when rev is set, after offset and at most limit of them
// (all when limit < 0). The caller holds the set's lock.
func zRankSlice(z *store.SortedSetValue, first, last int, rev bool, offset, limit int) []store.SortedEntry {
	if last < first || offset < 0 || limit == 0 {
		return nil
	}
	n := last - first + 1
	if offset >= n {
		return nil
	}
	cnt := n - offset
	if limit > 0 && limit < cnt {
		cnt = limit
	}
	if rev {
		lo := z.Card() - 1 - last + offset
		return z.RangeByRank(lo, lo+cnt-1, true)
	}
	return z.RangeByRank(first+offset, first+offset+cnt-1, false)
}

func (b zLexBound) store() store.LexBound {
	return store.LexBound{Val: b.val, Ex: b.ex, Inf: b.inf}
}

func sortEntries(entries []store.SortedEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Score != entries[j].Score {
			return entries[i].Score < entries[j].Score
		}
		return entries[i].Member < entries[j].Member
	})
}

// zScoreValue is a score reply: a RESP3 double, or its text in RESP2.
func zScoreValue(score float64) *resp.Value {
	return resp.DoubleValue(formatZScore(score))
}

// zEntriesValue renders members, optionally with scores: a flat
// member, score, ... array in RESP2 and [member, score] pairs in RESP3, as
// Redis does.
func zEntriesValue(ctx *Context, entries []store.SortedEntry, withScores bool) *resp.Value {
	if withScores && ctx.Writer.IsRESP3() {
		return resp.ArrayValue(zPairsReply(entries))
	}
	return resp.ArrayValue(zEntriesReply(entries, withScores))
}

// zEntriesReply renders members, optionally each followed by its score.
func zEntriesReply(entries []store.SortedEntry, withScores bool) []*resp.Value {
	n := len(entries)
	if withScores {
		n *= 2
	}
	out := make([]*resp.Value, 0, n)
	for _, e := range entries {
		out = append(out, resp.BulkString(e.Member))
		if withScores {
			out = append(out, zScoreValue(e.Score))
		}
	}
	return out
}

// zPairsReply renders entries as nested [member, score] pairs (ZMPOP shape).
func zPairsReply(entries []store.SortedEntry) []*resp.Value {
	out := make([]*resp.Value, 0, len(entries))
	for _, e := range entries {
		out = append(out, resp.ArrayValue([]*resp.Value{
			resp.BulkString(e.Member),
			zScoreValue(e.Score),
		}))
	}
	return out
}

// storeSortedSet replaces dst with entries, or deletes it when entries is
// empty, as the *STORE commands do whatever type dst held.
func storeSortedSet(ctx *Context, dst string, entries []store.SortedEntry, event string) error {
	if len(entries) == 0 {
		if ctx.Store.Delete(dst) {
			ctx.notify(store.NotifyGeneric, "del", dst)
		}
		return nil
	}
	z := &store.SortedSetValue{Members: make(map[string]float64, len(entries))}
	for _, e := range entries {
		z.Add(e.Member, e.Score)
	}
	if err := ctx.Store.Set(dst, z, store.SetOptions{}); err != nil {
		return err
	}
	ctx.notify(store.NotifyZSet, event, dst)
	return nil
}

func cmdZADD(ctx *Context) error {
	n := ctx.ArgCount()
	if n < 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	var nx, xx, ch, incr, gt, lt bool
	i := 1
flags:
	for ; i < n; i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "NX":
			nx = true
		case "XX":
			xx = true
		case "CH":
			ch = true
		case "INCR":
			incr = true
		case "GT":
			gt = true
		case "LT":
			lt = true
		default:
			break flags
		}
	}

	elements := n - i
	if elements == 0 || elements%2 != 0 {
		return ctx.WriteError(ErrSyntaxError)
	}
	if nx && xx {
		return ctx.WriteError(errZAddXXNX)
	}
	if (gt && nx) || (lt && nx) || (gt && lt) {
		return ctx.WriteError(errZAddGTLTNX)
	}
	pairs := elements / 2
	if incr && pairs > 1 {
		return ctx.WriteError(errZAddIncrPair)
	}

	scores := make([]float64, pairs)
	for j := 0; j < pairs; j++ {
		score, ok := parseZScore(ctx.ArgString(i + 2*j))
		if !ok {
			return ctx.WriteError(ErrNotFloat)
		}
		scores[j] = score
	}

	var zset *store.SortedSetValue
	var err error
	if xx {
		zset, err = lockSortedSet(ctx, key)
	} else {
		zset, err = lockOrCreateSortedSet(ctx, key)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		if incr {
			return ctx.WriteNullBulkString()
		}
		return ctx.WriteInteger(0)
	}

	added, updated, processed := 0, 0, 0
	var lastScore float64
	for j := 0; j < pairs; j++ {
		score := scores[j]
		member := ctx.ArgString(i + 2*j + 1)
		cur, exists := zset.Members[member]
		if exists {
			if nx {
				continue
			}
			if incr {
				score += cur
				if math.IsNaN(score) {
					zset.Unlock()
					deleteIfEmpty(ctx, key, zset)
					return ctx.WriteError(errZNaNScore)
				}
			}
			if (lt && score >= cur) || (gt && score <= cur) {
				continue
			}
			processed++
			lastScore = score
			if score != cur {
				zset.Add(member, score)
				updated++
			}
		} else if !xx {
			zset.Add(member, score)
			added++
			processed++
			lastScore = score
		}
	}
	empty := len(zset.Members) == 0
	zset.Unlock()
	if added+updated > 0 {
		if incr {
			ctx.notify(store.NotifyZSet, "zincr", key)
		} else {
			ctx.notify(store.NotifyZSet, "zadd", key)
		}
	} else if !empty {
		ctx.PropagateAs()
	}
	if empty {
		deleteIfEmpty(ctx, key, zset)
	}

	if added+updated > 0 {
		ctx.Store.KeyNotifier().NotifyKey(key)
	}

	if incr {
		if processed == 0 {
			return ctx.WriteNullBulkString()
		}
		return ctx.Writer.WriteValue(zScoreValue(lastScore))
	}
	if ch {
		return ctx.WriteInteger(int64(added + updated))
	}
	return ctx.WriteInteger(int64(added))
}

func cmdZCARD(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	zset, err := getSortedSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteInteger(0)
	}

	zset.RLock()
	reply := int64(len(zset.Members))
	zset.RUnlock()
	return ctx.WriteInteger(reply)
}

func cmdZCOUNT(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	r, err := parseZScoreRange(ctx.ArgString(1), ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}

	zset, err := getSortedSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteInteger(0)
	}

	zset.RLock()
	first, last := zset.ScoreRanks(r.min, r.minex, r.max, r.maxex)
	zset.RUnlock()
	if last < first {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(int64(last - first + 1))
}

func cmdZINCRBY(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	incr, ok := parseZScore(ctx.ArgString(1))
	if !ok {
		return ctx.WriteError(ErrNotFloat)
	}
	member := ctx.ArgString(2)

	zset, err := lockOrCreateSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	newScore := incr
	changed := true
	if cur, exists := zset.Members[member]; exists {
		newScore = cur + incr
		changed = newScore != cur
	}
	if math.IsNaN(newScore) {
		empty := len(zset.Members) == 0
		zset.Unlock()
		if empty {
			deleteIfEmpty(ctx, key, zset)
		}
		return ctx.WriteError(errZNaNScore)
	}
	zset.Add(member, newScore)
	zset.Unlock()

	if changed {
		ctx.notify(store.NotifyZSet, "zincr", key)
	}
	ctx.Store.KeyNotifier().NotifyKey(key)
	return ctx.Writer.WriteValue(zScoreValue(newScore))
}

const (
	zrangeAuto = iota
	zrangeRank
	zrangeScore
	zrangeLex
)

const (
	zdirAuto = iota
	zdirForward
	zdirReverse
)

// zrangeGeneric implements ZRANGE, ZRANGESTORE and the legacy Z[REV]RANGE*
// commands the way Redis' zrangeGenericCommand does. rangeType and dir are
// fixed by the legacy commands and left to the options otherwise.
func zrangeGeneric(ctx *Context, storeResult bool, rangeType, dir int) error {
	keyIdx := 0
	if storeResult {
		keyIdx = 1
	}
	n := ctx.ArgCount()
	if n < keyIdx+3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	minIdx, maxIdx := keyIdx+1, keyIdx+2

	withScores := false
	var offset, limit int64 = 0, -1
	for j := keyIdx + 3; j < n; j++ {
		left := n - j - 1
		opt := strings.ToUpper(ctx.ArgString(j))
		switch {
		case !storeResult && opt == "WITHSCORES":
			withScores = true
		case opt == "LIMIT" && left >= 2:
			var err1, err2 error
			offset, err1 = strconv.ParseInt(ctx.ArgString(j+1), 10, 64)
			limit, err2 = strconv.ParseInt(ctx.ArgString(j+2), 10, 64)
			if err1 != nil || err2 != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			j += 2
		case dir == zdirAuto && opt == "REV":
			dir = zdirReverse
		case rangeType == zrangeAuto && opt == "BYLEX":
			rangeType = zrangeLex
		case rangeType == zrangeAuto && opt == "BYSCORE":
			rangeType = zrangeScore
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}
	if dir == zdirAuto {
		dir = zdirForward
	}
	if rangeType == zrangeAuto {
		rangeType = zrangeRank
	}
	if limit != -1 && rangeType == zrangeRank {
		return ctx.WriteError(errZLimitRank)
	}
	if withScores && rangeType == zrangeLex {
		return ctx.WriteError(errZWithScoresLex)
	}
	rev := dir == zdirReverse
	if rev && rangeType != zrangeRank {
		minIdx, maxIdx = maxIdx, minIdx
	}

	var (
		start, stop int64
		scoreRange  zScoreRange
		lexRange    zLexRange
		err         error
	)
	switch rangeType {
	case zrangeRank:
		var err1, err2 error
		start, err1 = strconv.ParseInt(ctx.ArgString(minIdx), 10, 64)
		stop, err2 = strconv.ParseInt(ctx.ArgString(maxIdx), 10, 64)
		if err1 != nil || err2 != nil {
			return ctx.WriteError(ErrNotInteger)
		}
	case zrangeScore:
		if scoreRange, err = parseZScoreRange(ctx.ArgString(minIdx), ctx.ArgString(maxIdx)); err != nil {
			return ctx.WriteError(err)
		}
	case zrangeLex:
		if lexRange, err = parseZLexRange(ctx.ArgString(minIdx), ctx.ArgString(maxIdx)); err != nil {
			return ctx.WriteError(err)
		}
	}

	srcKey := ctx.ArgString(keyIdx)
	zset, err := getSortedSet(ctx, srcKey)
	if err != nil {
		return ctx.WriteError(err)
	}

	var entries []store.SortedEntry
	if zset != nil {
		zset.RLock()
		switch rangeType {
		case zrangeRank:
			if lo, hi, ok := zRankWindow(zset.Card(), start, stop); ok {
				entries = zset.RangeByRank(lo, hi, rev)
			}
		case zrangeScore:
			first, last := zset.ScoreRanks(scoreRange.min, scoreRange.minex, scoreRange.max, scoreRange.maxex)
			entries = zRankSlice(zset, first, last, rev, clampInt(offset), clampInt(limit))
		case zrangeLex:
			if offset >= 0 && limit != 0 {
				entries = zset.LexRange(lexRange.min.store(), lexRange.max.store(), rev, clampInt(offset), clampInt(limit))
			}
		}
		zset.RUnlock()
	}

	if storeResult {
		if err := storeSortedSet(ctx, ctx.ArgString(0), entries, "zrangestore"); err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteInteger(int64(len(entries)))
	}
	return ctx.Writer.WriteValue(zEntriesValue(ctx, entries, withScores))
}

func clampInt(v int64) int {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int(v)
}

func cmdZRANGE(ctx *Context) error {
	return zrangeGeneric(ctx, false, zrangeAuto, zdirAuto)
}

func cmdZRANGESTORE(ctx *Context) error {
	return zrangeGeneric(ctx, true, zrangeAuto, zdirAuto)
}

func cmdZREVRANGE(ctx *Context) error {
	return zrangeGeneric(ctx, false, zrangeRank, zdirReverse)
}

func cmdZRANGEBYSCORE(ctx *Context) error {
	return zrangeGeneric(ctx, false, zrangeScore, zdirForward)
}

func cmdZREVRANGEBYSCORE(ctx *Context) error {
	return zrangeGeneric(ctx, false, zrangeScore, zdirReverse)
}

func cmdZRANGEBYLEX(ctx *Context) error {
	return zrangeGeneric(ctx, false, zrangeLex, zdirForward)
}

func cmdZREVRANGEBYLEX(ctx *Context) error {
	return zrangeGeneric(ctx, false, zrangeLex, zdirReverse)
}

func zrankGeneric(ctx *Context, reverse bool) error {
	n := ctx.ArgCount()
	if n < 2 || n > 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	withScore := false
	if n == 3 {
		if !strings.EqualFold(ctx.ArgString(2), "WITHSCORE") {
			return ctx.WriteError(ErrSyntaxError)
		}
		withScore = true
	}
	writeMissing := func() error {
		if withScore {
			return ctx.WriteNullArray()
		}
		return ctx.WriteNull()
	}

	zset, err := getSortedSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return writeMissing()
	}

	member := ctx.ArgString(1)
	zset.RLock()
	score, exists := zset.Members[member]
	rank := 0
	if exists {
		rank = zset.Rank(member, reverse)
	}
	zset.RUnlock()
	if !exists {
		return writeMissing()
	}
	if withScore {
		return ctx.WriteArray([]*resp.Value{
			resp.IntegerValue(int64(rank)),
			zScoreValue(score),
		})
	}
	return ctx.WriteInteger(int64(rank))
}

func cmdZRANK(ctx *Context) error {
	return zrankGeneric(ctx, false)
}

func cmdZREVRANK(ctx *Context) error {
	return zrankGeneric(ctx, true)
}

func cmdZREM(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	zset, err := lockSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteInteger(0)
	}

	removed := 0
	for i := 1; i < ctx.ArgCount(); i++ {
		member := ctx.ArgString(i)
		if _, exists := zset.Members[member]; exists {
			zset.Remove(member)
			removed++
		}
	}
	isEmpty := len(zset.Members) == 0
	zset.Unlock()

	if removed > 0 {
		ctx.notify(store.NotifyZSet, "zrem", key)
	} else if !isEmpty {
		ctx.PropagateAs()
	}
	if isEmpty {
		deleteIfEmpty(ctx, key, zset)
	}
	return ctx.WriteInteger(int64(removed))
}

// zremRange removes the members selected by pick from the zset at key and
// deletes the key once it is empty.
func zremRange(ctx *Context, key, event string, pick func(z *store.SortedSetValue) []store.SortedEntry) error {
	zset, err := lockSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteInteger(0)
	}

	victims := pick(zset)
	for _, e := range victims {
		zset.Remove(e.Member)
	}
	isEmpty := len(zset.Members) == 0
	zset.Unlock()

	if len(victims) > 0 {
		ctx.notify(store.NotifyZSet, event, key)
	} else if !isEmpty {
		ctx.PropagateAs()
	}
	if isEmpty {
		deleteIfEmpty(ctx, key, zset)
	}
	return ctx.WriteInteger(int64(len(victims)))
}

func cmdZREMRANGEBYRANK(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	start, err1 := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	stop, err2 := strconv.ParseInt(ctx.ArgString(2), 10, 64)
	if err1 != nil || err2 != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	return zremRange(ctx, ctx.ArgString(0), "zremrangebyrank", func(z *store.SortedSetValue) []store.SortedEntry {
		lo, hi, ok := zRankWindow(z.Card(), start, stop)
		if !ok {
			return nil
		}
		return z.RangeByRank(lo, hi, false)
	})
}

func cmdZREMRANGEBYSCORE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	r, err := parseZScoreRange(ctx.ArgString(1), ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	return zremRange(ctx, ctx.ArgString(0), "zremrangebyscore", func(z *store.SortedSetValue) []store.SortedEntry {
		first, last := z.ScoreRanks(r.min, r.minex, r.max, r.maxex)
		return zRankSlice(z, first, last, false, 0, -1)
	})
}

func cmdZREMRANGEBYLEX(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	r, err := parseZLexRange(ctx.ArgString(1), ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	return zremRange(ctx, ctx.ArgString(0), "zremrangebylex", func(z *store.SortedSetValue) []store.SortedEntry {
		return z.LexRange(r.min.store(), r.max.store(), false, 0, -1)
	})
}

func cmdZSCORE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	zset, err := getSortedSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteNull()
	}

	zset.RLock()
	score, exists := zset.Members[ctx.ArgString(1)]
	zset.RUnlock()
	if !exists {
		return ctx.WriteNull()
	}
	return ctx.Writer.WriteValue(zScoreValue(score))
}

func cmdZMSCORE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	zset, err := getSortedSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}

	result := make([]*resp.Value, 0, ctx.ArgCount()-1)
	if zset != nil {
		zset.RLock()
	}
	for i := 1; i < ctx.ArgCount(); i++ {
		if zset == nil {
			result = append(result, resp.NullValue())
			continue
		}
		if score, exists := zset.Members[ctx.ArgString(i)]; exists {
			result = append(result, zScoreValue(score))
		} else {
			result = append(result, resp.NullValue())
		}
	}
	if zset != nil {
		zset.RUnlock()
	}
	return ctx.WriteArray(result)
}

func cmdZLEXCOUNT(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	r, err := parseZLexRange(ctx.ArgString(1), ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}

	zset, err := getSortedSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteInteger(0)
	}

	zset.RLock()
	count := zset.CountLex(r.min.store(), r.max.store())
	zset.RUnlock()
	return ctx.WriteInteger(int64(count))
}

func cmdZSCAN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	cursor, err := parseCollectionScanCursor(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}

	zset, err := getSortedSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return writeEmptyCollectionScan(ctx)
	}

	opts, err := parseCollectionScanOptions(ctx, 2, false)
	if err != nil {
		return ctx.WriteError(err)
	}

	zset.RLock()
	members, next := collectionScanPage(zset, len(zset.Members), cursor, opts.count, func(emit func(string)) {
		for m := range zset.Members {
			emit(m)
		}
	})
	items := make([]*resp.Value, 0, len(members)*2)
	for _, m := range members {
		if opts.match && !matchPattern(m, opts.pattern) {
			continue
		}
		items = append(items, resp.BulkString(m), resp.BulkString(formatZScore(zset.Members[m])))
	}
	zset.RUnlock()

	return writeCollectionScanReply(ctx, next, items)
}

// parsePositiveCount parses a pop count the way getPositiveLongFromObject
// does: a non-integer and a negative value both get the "must be positive"
// reply.
func parsePositiveCount(arg string) (int, error) {
	v, err := strconv.ParseInt(arg, 10, 64)
	if err != nil || v < 0 {
		return 0, errCountOutOfRange
	}
	if v > math.MaxInt32 {
		v = math.MaxInt32
	}
	return int(v), nil
}

// zpopFrom pops up to count lowest (or highest) members from the zset at key
// under its lock, deleting key when the set empties. It returns nil when the
// set is missing or empty.
func zpopFrom(ctx *Context, key string, count int, max bool) []store.SortedEntry {
	zset, err := lockSortedSet(ctx, key)
	if err != nil || zset == nil {
		return nil
	}
	if len(zset.Members) == 0 {
		zset.Unlock()
		return nil
	}
	all := zset.RangeByRank(0, count-1, max)
	for _, e := range all {
		zset.Remove(e.Member)
	}
	isEmpty := len(zset.Members) == 0
	zset.Unlock()

	if max {
		ctx.notify(store.NotifyZSet, "zpopmax", key)
	} else {
		ctx.notify(store.NotifyZSet, "zpopmin", key)
	}
	if isEmpty {
		deleteIfEmpty(ctx, key, zset)
	}
	return all
}

// zpopFirst pops from the first key that holds a sorted set. A key of
// another type met before it is a WRONGTYPE error, as in Redis.
func zpopFirst(ctx *Context, keys []string, count int, max bool) (string, []store.SortedEntry, error) {
	for _, key := range keys {
		zset, err := getSortedSet(ctx, key)
		if err != nil {
			return "", nil, err
		}
		if zset == nil {
			continue
		}
		if popped := zpopFrom(ctx, key, count, max); len(popped) > 0 {
			return key, popped, nil
		}
	}
	return "", nil, nil
}

func zpopMinMax(ctx *Context, max bool) error {
	n := ctx.ArgCount()
	if n < 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	if n > 2 {
		return ctx.WriteError(ErrSyntaxError)
	}
	count := 1
	if n == 2 {
		var err error
		if count, err = parsePositiveCount(ctx.ArgString(1)); err != nil {
			return ctx.WriteError(err)
		}
	}

	key := ctx.ArgString(0)
	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil || count == 0 {
		return ctx.WriteArray([]*resp.Value{})
	}
	popped := zpopFrom(ctx, key, count, max)
	// RESP3 pairs each member with its score, except when no count was
	// given: then the single member and score come flat, as in Redis.
	if n == 1 {
		return ctx.WriteArray(zEntriesReply(popped, true))
	}
	return ctx.Writer.WriteValue(zEntriesValue(ctx, popped, true))
}

func cmdZPOPMIN(ctx *Context) error {
	return zpopMinMax(ctx, false)
}

func cmdZPOPMAX(ctx *Context) error {
	return zpopMinMax(ctx, true)
}

func cmdZRANDMEMBER(ctx *Context) error {
	n := ctx.ArgCount()
	if n < 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	if n == 1 {
		zset, err := getSortedSet(ctx, key)
		if err != nil {
			return ctx.WriteError(err)
		}
		if zset == nil {
			return ctx.WriteNullBulkString()
		}
		zset.RLock()
		var picked []store.SortedEntry
		if n := zset.Card(); n > 0 {
			r := rand.Intn(n)
			picked = zset.RangeByRank(r, r, false)
		}
		zset.RUnlock()
		if len(picked) == 0 {
			return ctx.WriteNullBulkString()
		}
		return ctx.WriteBulkString(picked[0].Member)
	}

	count, err := parseRandCount(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	withScores := false
	if n > 3 || (n == 3 && !strings.EqualFold(ctx.ArgString(2), "WITHSCORES")) {
		return ctx.WriteError(ErrSyntaxError)
	}
	if n == 3 {
		withScores = true
		if count < -math.MaxInt64/2 || count > math.MaxInt64/2 {
			return ctx.WriteError(errValueOutOfRange)
		}
	}

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil || count == 0 {
		return ctx.WriteArray([]*resp.Value{})
	}

	zset.RLock()
	picked := randomKeys(zset.Members, count)
	out := make([]store.SortedEntry, len(picked))
	for i, m := range picked {
		out[i] = store.SortedEntry{Member: m, Score: zset.Members[m]}
	}
	zset.RUnlock()
	return ctx.Writer.WriteValue(zEntriesValue(ctx, out, withScores))
}

// maxRandRepeats bounds the reply of a negative random-member count.
const maxRandRepeats = 1000000

// parseRandCount parses the count of SRANDMEMBER/ZRANDMEMBER/HRANDFIELD,
// which must lie in [-LONG_MAX, LONG_MAX].
func parseRandCount(arg string) (int64, error) {
	v, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return 0, ErrNotInteger
	}
	if v == math.MinInt64 {
		return 0, errors.New("ERR value is out of range, value must between -9223372036854775807 and 9223372036854775807")
	}
	return v, nil
}

// zsetOpSource is a snapshot of one input of ZUNION/ZINTER/ZDIFF; a plain set
// contributes its members with score 1.
type zsetOpSource struct {
	members map[string]float64
	weight  float64
}

const (
	zsetOpUnion = iota
	zsetOpInter
	zsetOpDiff
)

// zsetOpGeneric implements ZUNION, ZINTER, ZDIFF, their STORE forms and
// ZINTERCARD as Redis' zunionInterDiffGenericCommand does.
func zsetOpGeneric(ctx *Context, op int, hasDst, cardOnly bool) error {
	numIdx := 0
	if hasDst {
		numIdx = 1
	}
	n := ctx.ArgCount()
	if n < numIdx+2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	numKeys, err := strconv.ParseInt(ctx.ArgString(numIdx), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	if numKeys < 1 {
		return ctx.WriteError(errors.New("ERR at least 1 input key is needed for '" + strings.ToLower(ctx.Command) + "' command"))
	}
	if numKeys > int64(n-numIdx-1) {
		return ctx.WriteError(ErrSyntaxError)
	}

	srcs := make([]zsetOpSource, numKeys)
	for i := range srcs {
		srcs[i].weight = 1
		entry, ok := ctx.Store.Get(ctx.ArgString(numIdx + 1 + i))
		if !ok {
			continue
		}
		switch v := entry.Value.(type) {
		case *store.SortedSetValue:
			v.RLock()
			srcs[i].members = make(map[string]float64, len(v.Members))
			for m, s := range v.Members {
				srcs[i].members[m] = s
			}
			v.RUnlock()
		case *store.SetValue:
			v.RLock()
			srcs[i].members = make(map[string]float64, len(v.Members))
			for m := range v.Members {
				srcs[i].members[m] = 1
			}
			v.RUnlock()
		default:
			return ctx.WriteError(store.ErrWrongType)
		}
	}

	aggregate := "SUM"
	withScores := false
	var limit int64
	j := numIdx + 1 + int(numKeys)
	for j < n {
		remaining := n - j
		opt := strings.ToUpper(ctx.ArgString(j))
		switch {
		case op != zsetOpDiff && !cardOnly && remaining >= int(numKeys)+1 && opt == "WEIGHTS":
			j++
			for i := range srcs {
				w, ok := parseZScore(ctx.ArgString(j))
				if !ok {
					return ctx.WriteError(errZWeightNotFloat)
				}
				srcs[i].weight = w
				j++
			}
		case op != zsetOpDiff && !cardOnly && remaining >= 2 && opt == "AGGREGATE":
			aggregate = strings.ToUpper(ctx.ArgString(j + 1))
			if aggregate != "SUM" && aggregate != "MIN" && aggregate != "MAX" {
				return ctx.WriteError(ErrSyntaxError)
			}
			j += 2
		case !hasDst && !cardOnly && opt == "WITHSCORES":
			withScores = true
			j++
		case cardOnly && remaining >= 2 && opt == "LIMIT":
			v, err := strconv.ParseInt(ctx.ArgString(j+1), 10, 64)
			if err != nil || v < 0 {
				return ctx.WriteError(errLimitNegative)
			}
			limit = v
			j += 2
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	weighted := func(src zsetOpSource, score float64) float64 {
		v := score * src.weight
		if math.IsNaN(v) {
			return 0
		}
		return v
	}
	combine := func(acc, v float64) float64 {
		switch aggregate {
		case "MIN":
			return math.Min(acc, v)
		case "MAX":
			return math.Max(acc, v)
		}
		s := acc + v
		if math.IsNaN(s) {
			return 0
		}
		return s
	}

	result := make(map[string]float64)
	switch op {
	case zsetOpUnion:
		for _, src := range srcs {
			for m, s := range src.members {
				v := weighted(src, s)
				if acc, ok := result[m]; ok {
					result[m] = combine(acc, v)
				} else {
					result[m] = v
				}
			}
		}
	case zsetOpInter:
		smallest := 0
		for i, src := range srcs {
			if len(src.members) < len(srcs[smallest].members) {
				smallest = i
			}
		}
		if len(srcs[smallest].members) > 0 {
		members:
			for m := range srcs[smallest].members {
				var acc float64
				for i, src := range srcs {
					s, ok := src.members[m]
					if !ok {
						continue members
					}
					if i == 0 {
						acc = weighted(src, s)
					} else {
						acc = combine(acc, weighted(src, s))
					}
				}
				result[m] = acc
				if cardOnly && limit > 0 && int64(len(result)) >= limit {
					break
				}
			}
		}
	case zsetOpDiff:
		for m, s := range srcs[0].members {
			in := false
			for _, src := range srcs[1:] {
				if _, ok := src.members[m]; ok {
					in = true
					break
				}
			}
			if !in {
				result[m] = s
			}
		}
	}

	if cardOnly {
		return ctx.WriteInteger(int64(len(result)))
	}

	entries := make([]store.SortedEntry, 0, len(result))
	for m, s := range result {
		entries = append(entries, store.SortedEntry{Member: m, Score: s})
	}
	if hasDst {
		event := "zunionstore"
		switch op {
		case zsetOpInter:
			event = "zinterstore"
		case zsetOpDiff:
			event = "zdiffstore"
		}
		if err := storeSortedSet(ctx, ctx.ArgString(0), entries, event); err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteInteger(int64(len(entries)))
	}
	sortEntries(entries)
	return ctx.Writer.WriteValue(zEntriesValue(ctx, entries, withScores))
}

func cmdZUNIONSTORE(ctx *Context) error {
	return zsetOpGeneric(ctx, zsetOpUnion, true, false)
}

func cmdZINTERSTORE(ctx *Context) error {
	return zsetOpGeneric(ctx, zsetOpInter, true, false)
}

func cmdZDIFFSTORE(ctx *Context) error {
	return zsetOpGeneric(ctx, zsetOpDiff, true, false)
}

func cmdZUNION(ctx *Context) error {
	return zsetOpGeneric(ctx, zsetOpUnion, false, false)
}

func cmdZINTER(ctx *Context) error {
	return zsetOpGeneric(ctx, zsetOpInter, false, false)
}

func cmdZDIFF(ctx *Context) error {
	return zsetOpGeneric(ctx, zsetOpDiff, false, false)
}

func cmdZINTERCARD(ctx *Context) error {
	return zsetOpGeneric(ctx, zsetOpInter, false, true)
}

// parseZMPop parses "numkeys key [key ...] MIN|MAX [COUNT count]" starting
// at argument from, with Redis' error replies.
func parseZMPop(ctx *Context, from int) ([]string, bool, int, error) {
	n := ctx.ArgCount()
	numKeys, err := strconv.ParseInt(ctx.ArgString(from), 10, 64)
	if err != nil || numKeys < 1 {
		return nil, false, 0, errNumKeysPositive
	}
	whereIdx := int64(from) + numKeys + 1
	if whereIdx >= int64(n) {
		return nil, false, 0, ErrSyntaxError
	}
	keys := make([]string, numKeys)
	for i := range keys {
		keys[i] = ctx.ArgString(from + 1 + i)
	}
	var max bool
	switch strings.ToUpper(ctx.ArgString(int(whereIdx))) {
	case "MIN":
	case "MAX":
		max = true
	default:
		return nil, false, 0, ErrSyntaxError
	}
	count := -1
	for j := int(whereIdx) + 1; j < n; j++ {
		if count == -1 && strings.EqualFold(ctx.ArgString(j), "COUNT") && j+1 < n {
			v, err := strconv.ParseInt(ctx.ArgString(j+1), 10, 64)
			if err != nil || v < 1 {
				return nil, false, 0, errPopCountPositive
			}
			if v > math.MaxInt32 {
				v = math.MaxInt32
			}
			count = int(v)
			j++
			continue
		}
		return nil, false, 0, ErrSyntaxError
	}
	if count == -1 {
		count = 1
	}
	return keys, max, count, nil
}

func cmdZMPOP(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	keys, max, count, err := parseZMPop(ctx, 0)
	if err != nil {
		return ctx.WriteError(err)
	}
	key, popped, err := zpopFirst(ctx, keys, count, max)
	if err != nil {
		return ctx.WriteError(err)
	}
	if popped == nil {
		return ctx.WriteNullArray()
	}
	return ctx.WriteArray([]*resp.Value{resp.BulkString(key), resp.ArrayValue(zPairsReply(popped))})
}

func cmdBZPOPMIN(ctx *Context) error {
	return cmdBZPOPGeneric(ctx, false)
}

func cmdBZPOPMAX(ctx *Context) error {
	return cmdBZPOPGeneric(ctx, true)
}

// bzpopWait blocks on keys until pop succeeds, the timeout expires or the
// client goes away, returning false on timeout. A key that changed to
// another type while waiting is skipped, as Redis keeps such a client
// blocked.
func bzpopWait(ctx *Context, keys []string, timeout time.Duration, pop func(keys []string) bool) bool {
	return blockOnKeys(ctx, keys, timeout, ctx.waitForZSetKeys, func(k string) bool {
		return pop([]string{k})
	})
}

func cmdBZPOPGeneric(ctx *Context, max bool) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	keys := make([]string, 0, ctx.ArgCount()-1)
	for i := 0; i < ctx.ArgCount()-1; i++ {
		keys = append(keys, ctx.ArgString(i))
	}
	timeout, err := parseBlockTimeout(ctx.ArgString(ctx.ArgCount() - 1))
	if err != nil {
		return ctx.WriteError(err)
	}

	var reply []*resp.Value
	pop := func(ks []string) bool {
		key, popped, err := zpopFirst(ctx, ks, 1, max)
		if err != nil || popped == nil {
			return false
		}
		reply = []*resp.Value{
			resp.BulkString(key),
			resp.BulkString(popped[0].Member),
			zScoreValue(popped[0].Score),
		}
		return true
	}

	key, popped, err := zpopFirst(ctx, keys, 1, max)
	if err != nil {
		return ctx.WriteError(err)
	}
	if popped != nil {
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString(key),
			resp.BulkString(popped[0].Member),
			zScoreValue(popped[0].Score),
		})
	}
	if !bzpopWait(ctx, keys, timeout, pop) {
		return ctx.WriteNullArray()
	}
	return ctx.WriteArray(reply)
}

func cmdBZMPOP(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	timeout, err := parseBlockTimeout(ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	keys, max, count, err := parseZMPop(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}

	var reply []*resp.Value
	pop := func(ks []string) bool {
		key, popped, err := zpopFirst(ctx, ks, count, max)
		if err != nil || popped == nil {
			return false
		}
		reply = []*resp.Value{resp.BulkString(key), resp.ArrayValue(zPairsReply(popped))}
		return true
	}

	key, popped, err := zpopFirst(ctx, keys, count, max)
	if err != nil {
		return ctx.WriteError(err)
	}
	if popped != nil {
		return ctx.WriteArray([]*resp.Value{resp.BulkString(key), resp.ArrayValue(zPairsReply(popped))})
	}
	if !bzpopWait(ctx, keys, timeout, pop) {
		return ctx.WriteNullArray()
	}
	return ctx.WriteArray(reply)
}
