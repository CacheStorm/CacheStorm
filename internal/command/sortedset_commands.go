package command

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
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
	router.Register(&CommandDef{Name: "ZMPOP", Handler: cmdZMPOP})
	router.Register(&CommandDef{Name: "BZPOPMIN", Handler: cmdBZPOPMIN})
	router.Register(&CommandDef{Name: "BZPOPMAX", Handler: cmdBZPOPMAX})
	router.Register(&CommandDef{Name: "BZMPOP", Handler: cmdBZMPOP})
	router.Register(&CommandDef{Name: "ZREVRANGEBYLEX", Handler: cmdZREVRANGEBYLEX})
}

func getOrCreateSortedSet(ctx *Context, key string) (*store.SortedSetValue, error) {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		zset := &store.SortedSetValue{Members: make(map[string]float64)}
		ctx.Store.Set(key, zset, store.SetOptions{})
		return zset, nil
	}

	zset, ok := entry.Value.(*store.SortedSetValue)
	if !ok {
		return nil, store.ErrWrongType
	}

	return zset, nil
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

func cmdZADD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	nx := false
	xx := false
	ch := false
	incr := false
	gt := false
	lt := false
	i := 1

	for i < ctx.ArgCount() {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "NX":
			nx = true
			i++
		case "XX":
			xx = true
			i++
		case "CH":
			ch = true
			i++
		case "INCR":
			incr = true
			i++
		case "GT":
			gt = true
			i++
		case "LT":
			lt = true
			i++
		default:
			break
		}
		if arg == "NX" || arg == "XX" || arg == "CH" || arg == "INCR" || arg == "GT" || arg == "LT" {
			continue
		}
		break
	}

	if nx && xx {
		return ctx.WriteError(ErrSyntaxError)
	}
	if (gt && lt) || (incr && (gt || lt)) {
		return ctx.WriteError(ErrSyntaxError)
	}

	scoreMemberCount := ctx.ArgCount() - i
	if scoreMemberCount < 2 || scoreMemberCount%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if incr && scoreMemberCount > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	zset, err := getOrCreateSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	zset.Lock()
	defer zset.Unlock()

	if xx && len(zset.Members) == 0 {
		if incr {
			return ctx.WriteNullBulkString()
		}
		return ctx.WriteInteger(0)
	}

	added := 0
	changed := 0

	for i < ctx.ArgCount() {
		score, err := strconv.ParseFloat(ctx.ArgString(i), 64)
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
		member := ctx.ArgString(i + 1)

		currentScore, exists := zset.Members[member]

		if nx && exists {
			i += 2
			continue
		}

		if xx && !exists {
			i += 2
			continue
		}

		if incr {
			if !exists {
				if xx {
					return ctx.WriteNullBulkString()
				}
				zset.Members[member] = score
				return ctx.WriteBulkString(strconv.FormatFloat(score, 'f', -1, 64))
			}
			newScore := currentScore + score
			zset.Members[member] = newScore
			return ctx.WriteBulkString(strconv.FormatFloat(newScore, 'f', -1, 64))
		}

		shouldUpdate := false
		if !exists {
			shouldUpdate = true
			added++
		} else {
			if gt && score > currentScore {
				shouldUpdate = true
			} else if lt && score < currentScore {
				shouldUpdate = true
			} else if !gt && !lt && score != currentScore {
				shouldUpdate = true
			}
		}

		if shouldUpdate {
			zset.Members[member] = score
			changed++
		}

		i += 2
	}

	if ch {
		return ctx.WriteInteger(int64(changed))
	}
	return ctx.WriteInteger(int64(added))
}

func cmdZCARD(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteInteger(0)
	}

	zset.RLock()
	defer zset.RUnlock()
	return ctx.WriteInteger(int64(len(zset.Members)))
}

func cmdZCOUNT(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	minScore, err := strconv.ParseFloat(ctx.ArgString(1), 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	maxScore, err := strconv.ParseFloat(ctx.ArgString(2), 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteInteger(0)
	}

	zset.RLock()
	defer zset.RUnlock()
	return ctx.WriteInteger(int64(zset.Count(minScore, maxScore)))
}

func cmdZINCRBY(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	incr, err := strconv.ParseFloat(ctx.ArgString(1), 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	member := ctx.ArgString(2)

	zset, err := getOrCreateSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	zset.Lock()
	defer zset.Unlock()
	newScore := incr
	if current, exists := zset.Members[member]; exists {
		newScore = current + incr
	}
	zset.Members[member] = newScore

	return ctx.WriteBulkString(strconv.FormatFloat(newScore, 'f', -1, 64))
}

func cmdZRANGE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	startStr := ctx.ArgString(1)
	stopStr := ctx.ArgString(2)

	withScores := false
	rev := false
	byScore := false
	byLex := false
	offset := 0
	count := -1

	for i := 3; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "WITHSCORES":
			withScores = true
		case "REV":
			rev = true
		case "BYSCORE":
			byScore = true
		case "BYLEX":
			byLex = true
		case "LIMIT":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			offset, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			count, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
		}
	}

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	zset.RLock()
	defer zset.RUnlock()

	var entries []store.SortedEntry

	if byScore {
		minScore, minExclusive, maxScore, maxExclusive := parseScoreRange(startStr, stopStr)
		entries = zset.GetByScoreRange(minScore, minExclusive, maxScore, maxExclusive, rev)
		if offset > 0 || count >= 0 {
			start := offset
			if start > len(entries) {
				start = len(entries)
			}
			end := len(entries)
			if count >= 0 && start+count < end {
				end = start + count
			}
			entries = entries[start:end]
		}
	} else if byLex {
		minLex, minExclusive, maxLex, maxExclusive := parseLexRange(startStr, stopStr)
		entries = zset.GetByLexRange(minLex, minExclusive, maxLex, maxExclusive, rev)
		if offset > 0 || count >= 0 {
			start := offset
			if start > len(entries) {
				start = len(entries)
			}
			end := len(entries)
			if count >= 0 && start+count < end {
				end = start + count
			}
			entries = entries[start:end]
		}
	} else {
		start, err := strconv.Atoi(startStr)
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
		stop, err := strconv.Atoi(stopStr)
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
		entries = zset.GetSortedRange(start, stop, false, rev)
	}

	results := make([]*resp.Value, 0, len(entries)*2)
	for _, e := range entries {
		results = append(results, resp.BulkString(e.Member))
		if withScores {
			results = append(results, resp.BulkString(strconv.FormatFloat(e.Score, 'f', -1, 64)))
		}
	}

	return ctx.WriteArray(results)
}

func parseScoreRange(minStr, maxStr string) (min float64, minExclusive bool, max float64, maxExclusive bool) {
	minExclusive = strings.HasPrefix(minStr, "(")
	if minExclusive {
		minStr = minStr[1:]
	}
	if minStr == "-inf" {
		min = math.Inf(-1)
	} else if minStr == "+inf" || minStr == "inf" {
		min = math.Inf(1)
	} else {
		min, _ = strconv.ParseFloat(minStr, 64)
	}

	maxExclusive = strings.HasPrefix(maxStr, "(")
	if maxExclusive {
		maxStr = maxStr[1:]
	}
	if maxStr == "-inf" {
		max = math.Inf(-1)
	} else if maxStr == "+inf" || maxStr == "inf" {
		max = math.Inf(1)
	} else {
		max, _ = strconv.ParseFloat(maxStr, 64)
	}

	return
}

func parseLexRange(minStr, maxStr string) (min string, minExclusive bool, max string, maxExclusive bool) {
	if strings.HasPrefix(minStr, "[") {
		min = minStr[1:]
		minExclusive = false
	} else if strings.HasPrefix(minStr, "(") {
		min = minStr[1:]
		minExclusive = true
	} else if minStr == "-" {
		min = ""
		minExclusive = false
	} else if minStr == "+" {
		min = ""
		minExclusive = true
	}

	if strings.HasPrefix(maxStr, "[") {
		max = maxStr[1:]
		maxExclusive = false
	} else if strings.HasPrefix(maxStr, "(") {
		max = maxStr[1:]
		maxExclusive = true
	} else if maxStr == "+" {
		max = ""
		maxExclusive = false
	} else if maxStr == "-" {
		max = ""
		maxExclusive = true
	}

	return
}

func cmdZRANGEBYSCORE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	minScore, err := strconv.ParseFloat(ctx.ArgString(1), 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	maxScore, err := strconv.ParseFloat(ctx.ArgString(2), 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	withScores := false
	for i := 3; i < ctx.ArgCount(); i++ {
		if strings.ToUpper(ctx.ArgString(i)) == "WITHSCORES" {
			withScores = true
		}
	}

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	zset.RLock()
	entries := zset.RangeByScore(minScore, maxScore, withScores, false)
	zset.RUnlock()

	results := make([]*resp.Value, 0, len(entries)*2)
	for _, e := range entries {
		results = append(results, resp.BulkString(e.Member))
		if withScores {
			results = append(results, resp.BulkString(strconv.FormatFloat(e.Score, 'f', -1, 64)))
		}
	}

	return ctx.WriteArray(results)
}

func cmdZRANK(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	member := ctx.ArgString(1)

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteNull()
	}

	zset.RLock()
	rank := zset.Rank(member, false)
	zset.RUnlock()
	if rank == -1 {
		return ctx.WriteNull()
	}

	return ctx.WriteInteger(int64(rank))
}

func cmdZREM(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteInteger(0)
	}

	zset.Lock()
	removed := 0
	for i := 1; i < ctx.ArgCount(); i++ {
		member := ctx.ArgString(i)
		if _, exists := zset.Members[member]; exists {
			delete(zset.Members, member)
			removed++
		}
	}

	isEmpty := len(zset.Members) == 0
	zset.Unlock()

	if isEmpty {
		ctx.Store.Delete(key)
	}

	return ctx.WriteInteger(int64(removed))
}

func cmdZREMRANGEBYRANK(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	start, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	stop, err := strconv.Atoi(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteInteger(0)
	}

	zset.Lock()
	removed := zset.RemoveRangeByRank(start, stop)
	isEmpty := len(zset.Members) == 0
	zset.Unlock()

	if isEmpty {
		ctx.Store.Delete(key)
	}

	return ctx.WriteInteger(int64(removed))
}

func cmdZREMRANGEBYSCORE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	min, err := strconv.ParseFloat(ctx.ArgString(1), 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	max, err := strconv.ParseFloat(ctx.ArgString(2), 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteInteger(0)
	}

	zset.Lock()
	removed := zset.RemoveRangeByScore(min, max)
	isEmpty := len(zset.Members) == 0
	zset.Unlock()

	if isEmpty {
		ctx.Store.Delete(key)
	}

	return ctx.WriteInteger(int64(removed))
}

func cmdZSCORE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	member := ctx.ArgString(1)

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteNull()
	}

	zset.RLock()
	score, exists := zset.Members[member]
	zset.RUnlock()
	if !exists {
		return ctx.WriteNull()
	}

	return ctx.WriteBulkString(strconv.FormatFloat(score, 'f', -1, 64))
}

func cmdZREVRANGE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	start, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	stop, err := strconv.Atoi(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	withScores := false
	for i := 3; i < ctx.ArgCount(); i++ {
		if strings.ToUpper(ctx.ArgString(i)) == "WITHSCORES" {
			withScores = true
		}
	}

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	zset.RLock()
	entries := zset.GetSortedRange(start, stop, withScores, true)
	zset.RUnlock()

	results := make([]*resp.Value, 0, len(entries)*2)
	for _, e := range entries {
		results = append(results, resp.BulkString(e.Member))
		if withScores {
			results = append(results, resp.BulkString(strconv.FormatFloat(e.Score, 'f', -1, 64)))
		}
	}

	return ctx.WriteArray(results)
}

func cmdZREVRANK(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	member := ctx.ArgString(1)

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteNull()
	}

	rank := zset.Rank(member, true)
	if rank == -1 {
		return ctx.WriteNull()
	}

	return ctx.WriteInteger(int64(rank))
}

func cmdZREVRANGEBYSCORE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	max, err := strconv.ParseFloat(ctx.ArgString(1), 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	min, err := strconv.ParseFloat(ctx.ArgString(2), 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	withScores := false
	for i := 3; i < ctx.ArgCount(); i++ {
		if strings.ToUpper(ctx.ArgString(i)) == "WITHSCORES" {
			withScores = true
		}
	}

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	entries := zset.RangeByScore(min, max, withScores, true)

	results := make([]*resp.Value, 0, len(entries)*2)
	for _, e := range entries {
		results = append(results, resp.BulkString(e.Member))
		if withScores {
			results = append(results, resp.BulkString(strconv.FormatFloat(e.Score, 'f', -1, 64)))
		}
	}

	return ctx.WriteArray(results)
}

func cmdZLEXCOUNT(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	min := ctx.ArgString(1)
	max := ctx.ArgString(2)

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteInteger(0)
	}

	zset.RLock()
	count := zset.LexCount(min, max)
	zset.RUnlock()

	return ctx.WriteInteger(int64(count))
}

func cmdZRANGEBYLEX(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	min := ctx.ArgString(1)
	max := ctx.ArgString(2)

	offset := 0
	count := -1
	rev := false

	for i := 3; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "LIMIT":
			if i+2 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			offset, err = strconv.Atoi(ctx.ArgString(i + 1))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			count, err = strconv.Atoi(ctx.ArgString(i + 2))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			i += 2
		case "REV":
			rev = true
		}
	}

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	zset.RLock()
	members := zset.RangeByLex(min, max, offset, count, rev)
	zset.RUnlock()

	results := make([]*resp.Value, 0, len(members))
	for _, m := range members {
		results = append(results, resp.BulkString(m))
	}

	return ctx.WriteArray(results)
}

func cmdZREMRANGEBYLEX(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	min := ctx.ArgString(1)
	max := ctx.ArgString(2)

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteInteger(0)
	}

	zset.Lock()
	removed := zset.RemoveRangeByLex(min, max)
	isEmpty := len(zset.Members) == 0
	zset.Unlock()

	if isEmpty {
		ctx.Store.Delete(key)
	}

	return ctx.WriteInteger(int64(removed))
}

func cmdZSCAN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	cursor, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	count := 10
	pattern := "*"

	for i := 2; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "COUNT":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			count, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
		case "MATCH":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			pattern = ctx.ArgString(i)
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("0"),
			resp.ArrayValue([]*resp.Value{}),
		})
	}

	entries := zset.GetSortedRange(0, -1, true, false)

	members := make([]store.SortedEntry, 0, len(entries))
	for _, entry := range entries {
		if matchPattern(entry.Member, pattern) {
			members = append(members, entry)
		}
	}

	start := cursor
	if start >= len(members) {
		start = 0
	}

	end := start + count
	if end > len(members) {
		end = len(members)
	}

	nextCursor := 0
	if end < len(members) {
		nextCursor = end
	}

	result := make([]*resp.Value, 0, (end-start)*2)
	for i := start; i < end; i++ {
		entry := members[i]
		result = append(result, resp.BulkString(entry.Member))
		result = append(result, resp.BulkString(strconv.FormatFloat(entry.Score, 'f', -1, 64)))
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(strconv.Itoa(nextCursor)),
		resp.ArrayValue(result),
	})
}

func cmdZPOPMIN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	count := 1
	if ctx.ArgCount() > 1 {
		var err error
		count, err = strconv.Atoi(ctx.ArgString(1))
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
	}

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	entries := zset.GetSortedRange(0, count-1, true, false)
	if len(entries) == 0 {
		return ctx.WriteArray([]*resp.Value{})
	}

	result := make([]*resp.Value, 0, len(entries)*2)
	for _, entry := range entries {
		result = append(result, resp.BulkString(entry.Member))
		result = append(result, resp.BulkString(strconv.FormatFloat(entry.Score, 'f', -1, 64)))
		zset.Remove(entry.Member)
	}

	return ctx.WriteArray(result)
}

func cmdZPOPMAX(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	count := 1
	if ctx.ArgCount() > 1 {
		var err error
		count, err = strconv.Atoi(ctx.ArgString(1))
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
	}

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	entries := zset.GetSortedRange(0, count-1, true, true)
	if len(entries) == 0 {
		return ctx.WriteArray([]*resp.Value{})
	}

	result := make([]*resp.Value, 0, len(entries)*2)
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		result = append(result, resp.BulkString(entry.Member))
		result = append(result, resp.BulkString(strconv.FormatFloat(entry.Score, 'f', -1, 64)))
		zset.Remove(entry.Member)
	}

	return ctx.WriteArray(result)
}

func cmdZRANDMEMBER(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	withScores := false
	count := 1

	for i := 1; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "COUNT":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			count, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
		case "WITHSCORES":
			withScores = true
		}
	}

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		if count == 0 {
			return ctx.WriteArray([]*resp.Value{})
		}
		return ctx.WriteNullBulkString()
	}

	entries := zset.GetSortedRange(0, -1, true, false)
	if len(entries) == 0 {
		return ctx.WriteNullBulkString()
	}

	if count > 0 {
		if count > len(entries) {
			count = len(entries)
		}
		result := make([]*resp.Value, 0, count*2)
		for i := 0; i < count; i++ {
			result = append(result, resp.BulkString(entries[i].Member))
			if withScores {
				result = append(result, resp.BulkString(strconv.FormatFloat(entries[i].Score, 'f', -1, 64)))
			}
		}
		return ctx.WriteArray(result)
	}

	result := make([]*resp.Value, 0, (-count)*2)
	for i := 0; i < -count; i++ {
		idx := i % len(entries)
		result = append(result, resp.BulkString(entries[idx].Member))
		if withScores {
			result = append(result, resp.BulkString(strconv.FormatFloat(entries[idx].Score, 'f', -1, 64)))
		}
	}
	return ctx.WriteArray(result)
}

func cmdZMSCORE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	result := make([]*resp.Value, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		member := ctx.ArgString(i)
		if zset == nil {
			result = append(result, resp.NullValue())
			continue
		}
		score, exists := zset.GetScore(member)
		if !exists {
			result = append(result, resp.NullValue())
		} else {
			result = append(result, resp.BulkString(strconv.FormatFloat(score, 'f', -1, 64)))
		}
	}

	return ctx.WriteArray(result)
}

func cmdZUNIONSTORE(ctx *Context) error {
	return zUnionInterStore(ctx, true, true)
}

func cmdZINTERSTORE(ctx *Context) error {
	return zUnionInterStore(ctx, false, true)
}

func zUnionInterStore(ctx *Context, isUnion bool, hasDestKey bool) error {
	offset := 0
	if hasDestKey {
		offset = 1
	}

	if ctx.ArgCount() < 2+offset {
		return ctx.WriteError(ErrWrongArgCount)
	}

	var destKey string
	if hasDestKey {
		destKey = ctx.ArgString(0)
	}

	numKeys, err := strconv.Atoi(ctx.ArgString(offset))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	if numKeys < 1 {
		return ctx.WriteError(ErrSyntaxError)
	}

	if ctx.ArgCount() < 1+offset+numKeys {
		return ctx.WriteError(ErrWrongArgCount)
	}

	keys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = ctx.ArgString(1 + offset + i)
	}

	weights := make([]float64, numKeys)
	for i := range weights {
		weights[i] = 1.0
	}

	aggregate := "SUM"

	argIdx := 1 + offset + numKeys
	for argIdx < ctx.ArgCount() {
		arg := strings.ToUpper(ctx.ArgString(argIdx))
		switch arg {
		case "WEIGHTS":
			argIdx++
			if argIdx+numKeys > ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			for i := 0; i < numKeys; i++ {
				w, err := strconv.ParseFloat(ctx.ArgString(argIdx), 64)
				if err != nil {
					return ctx.WriteError(ErrNotInteger)
				}
				weights[i] = w
				argIdx++
			}
		case "AGGREGATE":
			argIdx++
			if argIdx >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			aggregate = strings.ToUpper(ctx.ArgString(argIdx))
			if aggregate != "SUM" && aggregate != "MIN" && aggregate != "MAX" {
				return ctx.WriteError(ErrSyntaxError)
			}
			argIdx++
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	result := make(map[string]float64)

	if isUnion {
		for i, key := range keys {
			zset, err := getSortedSet(ctx, key)
			if err != nil {
				return ctx.WriteError(err)
			}
			if zset == nil {
				continue
			}
			zset.RLock()
			for member, score := range zset.Members {
				weightedScore := score * weights[i]
				if existing, exists := result[member]; exists {
					switch aggregate {
					case "SUM":
						result[member] = existing + weightedScore
					case "MIN":
						if weightedScore < existing {
							result[member] = weightedScore
						}
					case "MAX":
						if weightedScore > existing {
							result[member] = weightedScore
						}
					}
				} else {
					result[member] = weightedScore
				}
			}
			zset.RUnlock()
		}
	} else {
		firstZset, err := getSortedSet(ctx, keys[0])
		if err != nil {
			return ctx.WriteError(err)
		}
		if firstZset == nil {
			if hasDestKey {
				ctx.Store.Delete(destKey)
			}
			return ctx.WriteInteger(0)
		}

		firstZset.RLock()
		for member, score := range firstZset.Members {
			result[member] = score * weights[0]
		}
		firstZset.RUnlock()

		for i := 1; i < numKeys; i++ {
			zset, err := getSortedSet(ctx, keys[i])
			if err != nil {
				return ctx.WriteError(err)
			}
			if zset == nil {
				if hasDestKey {
					ctx.Store.Delete(destKey)
				}
				return ctx.WriteInteger(0)
			}
			zset.RLock()
			for member := range result {
				if score, exists := zset.Members[member]; exists {
					weightedScore := score * weights[i]
					switch aggregate {
					case "SUM":
						result[member] += weightedScore
					case "MIN":
						if weightedScore < result[member] {
							result[member] = weightedScore
						}
					case "MAX":
						if weightedScore > result[member] {
							result[member] = weightedScore
						}
					}
				} else {
					delete(result, member)
				}
			}
			zset.RUnlock()
		}
	}

	if hasDestKey {
		if len(result) == 0 {
			ctx.Store.Delete(destKey)
			return ctx.WriteInteger(0)
		}

		destZset, err := getOrCreateSortedSet(ctx, destKey)
		if err != nil {
			return ctx.WriteError(err)
		}

		destZset.Lock()
		destZset.Members = result
		destZset.Unlock()
		return ctx.WriteInteger(int64(len(result)))
	}

	return nil
}

func cmdZDIFFSTORE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	destKey := ctx.ArgString(0)
	numKeys, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	if ctx.ArgCount() < 2+numKeys {
		return ctx.WriteError(ErrWrongArgCount)
	}

	firstZset, err := getSortedSet(ctx, ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	if firstZset == nil {
		ctx.Store.Delete(destKey)
		return ctx.WriteInteger(0)
	}

	result := make(map[string]float64)
	for member, score := range firstZset.Members {
		result[member] = score
	}

	for i := 1; i < numKeys; i++ {
		key := ctx.ArgString(2 + i)
		zset, err := getSortedSet(ctx, key)
		if err != nil {
			return ctx.WriteError(err)
		}
		if zset != nil {
			for member := range zset.Members {
				delete(result, member)
			}
		}
	}

	if len(result) == 0 {
		ctx.Store.Delete(destKey)
		return ctx.WriteInteger(0)
	}

	destZset, err := getOrCreateSortedSet(ctx, destKey)
	if err != nil {
		return ctx.WriteError(err)
	}

	destZset.Members = result
	return ctx.WriteInteger(int64(len(result)))
}

func cmdZRANGESTORE(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	destKey := ctx.ArgString(0)
	srcKey := ctx.ArgString(1)
	start, err := strconv.Atoi(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	stop, err := strconv.Atoi(ctx.ArgString(3))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	rev := false
	for i := 4; i < ctx.ArgCount(); i++ {
		if strings.ToUpper(ctx.ArgString(i)) == "REV" {
			rev = true
		}
	}

	zset, err := getSortedSet(ctx, srcKey)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		ctx.Store.Delete(destKey)
		return ctx.WriteInteger(0)
	}

	zset.RLock()
	entries := zset.GetSortedRange(start, stop, false, rev)
	zset.RUnlock()

	if len(entries) == 0 {
		ctx.Store.Delete(destKey)
		return ctx.WriteInteger(0)
	}

	destZset, err := getOrCreateSortedSet(ctx, destKey)
	if err != nil {
		return ctx.WriteError(err)
	}

	destZset.Lock()
	destZset.Members = make(map[string]float64)
	for _, e := range entries {
		destZset.Members[e.Member] = e.Score
	}
	destZset.Unlock()

	return ctx.WriteInteger(int64(len(entries)))
}

func cmdZDIFF(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	numKeys, err := strconv.Atoi(ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	if ctx.ArgCount() < 1+numKeys {
		return ctx.WriteError(ErrWrongArgCount)
	}

	withScores := false
	if ctx.ArgCount() > 1+numKeys && strings.ToUpper(ctx.ArgString(1+numKeys)) == "WITHSCORES" {
		withScores = true
	}

	firstZset, err := getSortedSet(ctx, ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	if firstZset == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	firstZset.RLock()
	result := make(map[string]float64)
	for member, score := range firstZset.Members {
		result[member] = score
	}
	firstZset.RUnlock()

	for i := 1; i < numKeys; i++ {
		key := ctx.ArgString(1 + i)
		zset, err := getSortedSet(ctx, key)
		if err != nil {
			return ctx.WriteError(err)
		}
		if zset != nil {
			zset.RLock()
			for member := range zset.Members {
				delete(result, member)
			}
			zset.RUnlock()
		}
	}

	if len(result) == 0 {
		return ctx.WriteArray([]*resp.Value{})
	}

	entries := make([]store.SortedEntry, 0, len(result))
	for member, score := range result {
		entries = append(entries, store.SortedEntry{Member: member, Score: score})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Score < entries[j].Score || (entries[i].Score == entries[j].Score && entries[i].Member < entries[j].Member)
	})

	respEntries := make([]*resp.Value, 0, len(entries)*2)
	for _, e := range entries {
		respEntries = append(respEntries, resp.BulkString(e.Member))
		if withScores {
			respEntries = append(respEntries, resp.BulkString(strconv.FormatFloat(e.Score, 'f', -1, 64)))
		}
	}

	return ctx.WriteArray(respEntries)
}

func cmdZUNION(ctx *Context) error {
	return zUnionInter(ctx, true)
}

func cmdZINTER(ctx *Context) error {
	return zUnionInter(ctx, false)
}

func zUnionInter(ctx *Context, isUnion bool) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	numKeys, err := strconv.Atoi(ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	if numKeys < 1 {
		return ctx.WriteError(ErrSyntaxError)
	}

	if ctx.ArgCount() < 1+numKeys {
		return ctx.WriteError(ErrWrongArgCount)
	}

	keys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = ctx.ArgString(1 + i)
	}

	weights := make([]float64, numKeys)
	for i := range weights {
		weights[i] = 1.0
	}

	aggregate := "SUM"
	withScores := false

	argIdx := 1 + numKeys
	for argIdx < ctx.ArgCount() {
		arg := strings.ToUpper(ctx.ArgString(argIdx))
		switch arg {
		case "WEIGHTS":
			argIdx++
			if argIdx+numKeys > ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			for i := 0; i < numKeys; i++ {
				w, err := strconv.ParseFloat(ctx.ArgString(argIdx), 64)
				if err != nil {
					return ctx.WriteError(ErrNotInteger)
				}
				weights[i] = w
				argIdx++
			}
		case "AGGREGATE":
			argIdx++
			if argIdx >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			aggregate = strings.ToUpper(ctx.ArgString(argIdx))
			if aggregate != "SUM" && aggregate != "MIN" && aggregate != "MAX" {
				return ctx.WriteError(ErrSyntaxError)
			}
			argIdx++
		case "WITHSCORES":
			withScores = true
			argIdx++
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	result := make(map[string]float64)

	if isUnion {
		for i, key := range keys {
			zset, err := getSortedSet(ctx, key)
			if err != nil {
				return ctx.WriteError(err)
			}
			if zset == nil {
				continue
			}
			zset.RLock()
			for member, score := range zset.Members {
				weightedScore := score * weights[i]
				if existing, exists := result[member]; exists {
					switch aggregate {
					case "SUM":
						result[member] = existing + weightedScore
					case "MIN":
						if weightedScore < existing {
							result[member] = weightedScore
						}
					case "MAX":
						if weightedScore > existing {
							result[member] = weightedScore
						}
					}
				} else {
					result[member] = weightedScore
				}
			}
			zset.RUnlock()
		}
	} else {
		firstZset, err := getSortedSet(ctx, keys[0])
		if err != nil {
			return ctx.WriteError(err)
		}
		if firstZset == nil {
			return ctx.WriteArray([]*resp.Value{})
		}

		firstZset.RLock()
		for member, score := range firstZset.Members {
			result[member] = score * weights[0]
		}
		firstZset.RUnlock()

		for i := 1; i < numKeys; i++ {
			zset, err := getSortedSet(ctx, keys[i])
			if err != nil {
				return ctx.WriteError(err)
			}
			if zset == nil {
				return ctx.WriteArray([]*resp.Value{})
			}
			zset.RLock()
			for member := range result {
				if score, exists := zset.Members[member]; exists {
					weightedScore := score * weights[i]
					switch aggregate {
					case "SUM":
						result[member] += weightedScore
					case "MIN":
						if weightedScore < result[member] {
							result[member] = weightedScore
						}
					case "MAX":
						if weightedScore > result[member] {
							result[member] = weightedScore
						}
					}
				} else {
					delete(result, member)
				}
			}
			zset.RUnlock()
		}
	}

	if len(result) == 0 {
		return ctx.WriteArray([]*resp.Value{})
	}

	entries := make([]store.SortedEntry, 0, len(result))
	for member, score := range result {
		entries = append(entries, store.SortedEntry{Member: member, Score: score})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Score < entries[j].Score || (entries[i].Score == entries[j].Score && entries[i].Member < entries[j].Member)
	})

	respEntries := make([]*resp.Value, 0, len(entries)*2)
	for _, e := range entries {
		respEntries = append(respEntries, resp.BulkString(e.Member))
		if withScores {
			respEntries = append(respEntries, resp.BulkString(strconv.FormatFloat(e.Score, 'f', -1, 64)))
		}
	}

	return ctx.WriteArray(respEntries)
}

func cmdZMPOP(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	numKeys, err := strconv.Atoi(ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	if ctx.ArgCount() < 1+numKeys {
		return ctx.WriteError(ErrWrongArgCount)
	}

	dir := "MIN"
	count := 1
	argIdx := 1 + numKeys

	for argIdx < ctx.ArgCount() {
		arg := strings.ToUpper(ctx.ArgString(argIdx))
		switch arg {
		case "MIN", "MAX":
			dir = arg
			argIdx++
		case "COUNT":
			argIdx++
			if argIdx >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			count, err = strconv.Atoi(ctx.ArgString(argIdx))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			argIdx++
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	for i := 0; i < numKeys; i++ {
		key := ctx.ArgString(1 + i)
		zset, err := getSortedSet(ctx, key)
		if err != nil {
			return ctx.WriteError(err)
		}
		if zset == nil || len(zset.Members) == 0 {
			continue
		}

		zset.Lock()
		entries := zset.GetSortedRange(0, count-1, false, dir == "MAX")
		if len(entries) == 0 {
			zset.Unlock()
			continue
		}

		popped := make([]*resp.Value, 0, len(entries)*2)
		for _, e := range entries {
			popped = append(popped, resp.BulkString(e.Member))
			popped = append(popped, resp.BulkString(strconv.FormatFloat(e.Score, 'f', -1, 64)))
			delete(zset.Members, e.Member)
		}

		isEmpty := len(zset.Members) == 0
		zset.Unlock()

		if isEmpty {
			ctx.Store.Delete(key)
		}

		return ctx.WriteArray([]*resp.Value{
			resp.BulkString(key),
			resp.ArrayValue(popped),
		})
	}

	return ctx.WriteNull()
}

func cmdBZPOPMIN(ctx *Context) error {
	return cmdBZPOPGeneric(ctx, false)
}

func cmdBZPOPMAX(ctx *Context) error {
	return cmdBZPOPGeneric(ctx, true)
}

func cmdBZPOPGeneric(ctx *Context, max bool) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	keys := make([]string, 0, ctx.ArgCount()-1)
	var timeout int
	var err error

	for i := 0; i < ctx.ArgCount()-1; i++ {
		keys = append(keys, ctx.ArgString(i))
	}
	timeout, err = strconv.Atoi(ctx.ArgString(ctx.ArgCount() - 1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	deadline := time.Now().Add(time.Duration(timeout) * time.Second)

	for {
		for _, key := range keys {
			zset, err := getSortedSet(ctx, key)
			if err != nil {
				return ctx.WriteError(err)
			}
			if zset == nil || len(zset.Members) == 0 {
				continue
			}

			zset.Lock()
			entries := zset.GetSortedRange(0, 0, false, max)
			if len(entries) > 0 {
				entry := entries[0]
				delete(zset.Members, entry.Member)
				isEmpty := len(zset.Members) == 0
				zset.Unlock()

				if isEmpty {
					ctx.Store.Delete(key)
				}

				return ctx.WriteArray([]*resp.Value{
					resp.BulkString(key),
					resp.BulkString(entry.Member),
					resp.BulkString(strconv.FormatFloat(entry.Score, 'f', -1, 64)),
				})
			}
			zset.Unlock()
		}

		if timeout == 0 {
			return ctx.WriteNull()
		}

		if time.Now().After(deadline) {
			return ctx.WriteNull()
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func cmdBZMPOP(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	timeout, err := strconv.ParseInt(ctx.ArgString(0), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	numKeys, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	if numKeys < 1 || 2+numKeys > ctx.ArgCount() {
		return ctx.WriteError(ErrWrongArgCount)
	}

	keys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = ctx.ArgString(2 + i)
	}

	dir := "MIN"
	count := 1
	argIdx := 2 + numKeys

	for argIdx < ctx.ArgCount() {
		arg := strings.ToUpper(ctx.ArgString(argIdx))
		switch arg {
		case "MIN", "MAX":
			dir = arg
			argIdx++
		case "COUNT":
			argIdx++
			if argIdx >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			count, err = strconv.Atoi(ctx.ArgString(argIdx))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			argIdx++
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	max := dir == "MAX"

	for {
		for _, key := range keys {
			zset, err := getSortedSet(ctx, key)
			if err != nil {
				return ctx.WriteError(err)
			}
			if zset == nil || len(zset.Members) == 0 {
				continue
			}

			zset.Lock()
			if len(zset.Members) == 0 {
				zset.Unlock()
				continue
			}

			entries := zset.GetSortedRange(0, count-1, false, max)
			if len(entries) == 0 {
				zset.Unlock()
				continue
			}

			popped := make([]*resp.Value, 0, len(entries))
			for _, entry := range entries {
				delete(zset.Members, entry.Member)
				popped = append(popped, resp.ArrayValue([]*resp.Value{
					resp.BulkString(entry.Member),
					resp.BulkString(strconv.FormatFloat(entry.Score, 'f', -1, 64)),
				}))
			}

			isEmpty := len(zset.Members) == 0
			zset.Unlock()

			if isEmpty {
				ctx.Store.Delete(key)
			}

			return ctx.WriteArray([]*resp.Value{
				resp.BulkString(key),
				resp.ArrayValue(popped),
			})
		}

		if timeout == 0 {
			return ctx.WriteNull()
		}

		if time.Now().After(deadline) {
			return ctx.WriteNull()
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func cmdZREVRANGEBYLEX(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	max := ctx.ArgString(1)
	min := ctx.ArgString(2)

	offset := 0
	count := -1

	for i := 3; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "LIMIT":
			if i+2 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			offset, err = strconv.Atoi(ctx.ArgString(i + 1))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			count, err = strconv.Atoi(ctx.ArgString(i + 2))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			i += 2
		}
	}

	zset, err := getSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if zset == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	zset.RLock()
	members := zset.RangeByLex(min, max, offset, count, true)
	zset.RUnlock()

	results := make([]*resp.Value, 0, len(members))
	for i := len(members) - 1; i >= 0; i-- {
		results = append(results, resp.BulkString(members[i]))
	}

	return ctx.WriteArray(results)
}
