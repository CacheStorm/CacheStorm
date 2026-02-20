package command

import (
	"strconv"
	"strings"

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
	zset, err := getOrCreateSortedSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	added := 0
	i := 1

	for i < ctx.ArgCount() {
		arg := strings.ToUpper(ctx.ArgString(i))

		switch arg {
		case "NX", "XX", "CH", "INCR":
			i++
			continue
		case "GT", "LT":
			i++
			continue
		}

		if i+1 >= ctx.ArgCount() {
			break
		}

		score, err := strconv.ParseFloat(ctx.ArgString(i), 64)
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}

		member := ctx.ArgString(i + 1)

		if _, exists := zset.Members[member]; !exists {
			added++
		}
		zset.Members[member] = score

		i += 2
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
	start, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	stop, err := strconv.Atoi(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	withScores := false
	rev := false
	for i := 3; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		if arg == "WITHSCORES" {
			withScores = true
		} else if arg == "REV" {
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

	entries := zset.GetSortedRange(start, stop, withScores, rev)

	results := make([]*resp.Value, 0, len(entries)*2)
	for _, e := range entries {
		results = append(results, resp.BulkString(e.Member))
		if withScores {
			results = append(results, resp.BulkString(strconv.FormatFloat(e.Score, 'f', -1, 64)))
		}
	}

	return ctx.WriteArray(results)
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

	entries := zset.RangeByScore(minScore, maxScore, withScores, false)

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

	rank := zset.Rank(member, false)
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

	removed := 0
	for i := 1; i < ctx.ArgCount(); i++ {
		member := ctx.ArgString(i)
		if _, exists := zset.Members[member]; exists {
			delete(zset.Members, member)
			removed++
		}
	}

	if len(zset.Members) == 0 {
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

	removed := zset.RemoveRangeByRank(start, stop)

	if len(zset.Members) == 0 {
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

	removed := zset.RemoveRangeByScore(min, max)

	if len(zset.Members) == 0 {
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

	score, exists := zset.Members[member]
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

	entries := zset.GetSortedRange(start, stop, withScores, true)

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

	return ctx.WriteInteger(0)
}

func cmdZRANGEBYLEX(ctx *Context) error {
	return ctx.WriteArray([]*resp.Value{})
}

func cmdZREMRANGEBYLEX(ctx *Context) error {
	return ctx.WriteInteger(0)
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

	result := make(map[string]float64)
	for i := 0; i < numKeys; i++ {
		key := ctx.ArgString(2 + i)
		zset, err := getSortedSet(ctx, key)
		if err != nil {
			return ctx.WriteError(err)
		}
		if zset == nil {
			continue
		}
		for member, score := range zset.Members {
			if existing, exists := result[member]; exists {
				result[member] = existing + score
			} else {
				result[member] = score
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

func cmdZINTERSTORE(ctx *Context) error {
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
		if zset == nil {
			ctx.Store.Delete(destKey)
			return ctx.WriteInteger(0)
		}
		for member := range result {
			if score, exists := zset.Members[member]; exists {
				result[member] += score
			} else {
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
