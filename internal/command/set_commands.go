package command

import (
	"math/rand"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterSetCommands(router *Router) {
	router.Register(&CommandDef{Name: "SADD", Handler: cmdSADD})
	router.Register(&CommandDef{Name: "SREM", Handler: cmdSREM})
	router.Register(&CommandDef{Name: "SMEMBERS", Handler: cmdSMEMBERS})
	router.Register(&CommandDef{Name: "SISMEMBER", Handler: cmdSISMEMBER})
	router.Register(&CommandDef{Name: "SCARD", Handler: cmdSCARD})
	router.Register(&CommandDef{Name: "SPOP", Handler: cmdSPOP})
	router.Register(&CommandDef{Name: "SRANDMEMBER", Handler: cmdSRANDMEMBER})
	router.Register(&CommandDef{Name: "SMOVE", Handler: cmdSMOVE})
	router.Register(&CommandDef{Name: "SUNION", Handler: cmdSUNION})
	router.Register(&CommandDef{Name: "SINTER", Handler: cmdSINTER})
	router.Register(&CommandDef{Name: "SDIFF", Handler: cmdSDIFF})
	router.Register(&CommandDef{Name: "SUNIONSTORE", Handler: cmdSUNIONSTORE})
	router.Register(&CommandDef{Name: "SINTERSTORE", Handler: cmdSINTERSTORE})
	router.Register(&CommandDef{Name: "SDIFFSTORE", Handler: cmdSDIFFSTORE})
	router.Register(&CommandDef{Name: "SSCAN", Handler: cmdSSCAN})
}

func getOrCreateSet(ctx *Context, key string) (*store.SetValue, error) {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		set := &store.SetValue{Members: make(map[string]struct{})}
		ctx.Store.Set(key, set, store.SetOptions{})
		return set, nil
	}

	set, ok := entry.Value.(*store.SetValue)
	if !ok {
		return nil, store.ErrWrongType
	}

	return set, nil
}

func getSet(ctx *Context, key string) (*store.SetValue, error) {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return nil, nil
	}

	set, ok := entry.Value.(*store.SetValue)
	if !ok {
		return nil, store.ErrWrongType
	}

	return set, nil
}

func getSetOrEmpty(ctx *Context, key string) (*store.SetValue, error) {
	set, err := getSet(ctx, key)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return &store.SetValue{Members: make(map[string]struct{})}, nil
	}
	return set, nil
}

func cmdSADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	set, err := getOrCreateSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	added := 0
	for i := 1; i < ctx.ArgCount(); i++ {
		member := ctx.ArgString(i)
		if _, exists := set.Members[member]; !exists {
			set.Members[member] = struct{}{}
			added++
		}
	}

	return ctx.WriteInteger(int64(added))
}

func cmdSREM(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	set, err := getSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if set == nil {
		return ctx.WriteInteger(0)
	}

	removed := 0
	for i := 1; i < ctx.ArgCount(); i++ {
		member := ctx.ArgString(i)
		if _, exists := set.Members[member]; exists {
			delete(set.Members, member)
			removed++
		}
	}

	if len(set.Members) == 0 {
		ctx.Store.Delete(key)
	}

	return ctx.WriteInteger(int64(removed))
}

func cmdSMEMBERS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	set, err := getSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if set == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	members := make([]*resp.Value, 0, len(set.Members))
	for member := range set.Members {
		members = append(members, resp.BulkString(member))
	}

	return ctx.WriteArray(members)
}

func cmdSISMEMBER(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	member := ctx.ArgString(1)

	set, err := getSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if set == nil {
		return ctx.WriteInteger(0)
	}

	if _, exists := set.Members[member]; exists {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdSCARD(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	set, err := getSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if set == nil {
		return ctx.WriteInteger(0)
	}

	return ctx.WriteInteger(int64(len(set.Members)))
}

func cmdSPOP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	count := 1
	if ctx.ArgCount() >= 2 {
		var err error
		count, err = strconv.Atoi(ctx.ArgString(1))
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
	}

	set, err := getSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if set == nil {
		if count == 1 {
			return ctx.WriteNullBulkString()
		}
		return ctx.WriteArray([]*resp.Value{})
	}

	if count == 1 {
		for member := range set.Members {
			delete(set.Members, member)
			if len(set.Members) == 0 {
				ctx.Store.Delete(key)
			}
			return ctx.WriteBulkString(member)
		}
		return ctx.WriteNullBulkString()
	}

	if count >= len(set.Members) {
		members := make([]*resp.Value, 0, len(set.Members))
		for member := range set.Members {
			members = append(members, resp.BulkString(member))
		}
		ctx.Store.Delete(key)
		return ctx.WriteArray(members)
	}

	members := make([]*resp.Value, 0, count)
	i := 0
	for member := range set.Members {
		if i >= count {
			break
		}
		members = append(members, resp.BulkString(member))
		delete(set.Members, member)
		i++
	}

	return ctx.WriteArray(members)
}

func cmdSRANDMEMBER(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	count := 1
	withCount := false
	if ctx.ArgCount() >= 2 {
		var err error
		count, err = strconv.Atoi(ctx.ArgString(1))
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
		withCount = true
	}

	set, err := getSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if set == nil {
		if withCount {
			return ctx.WriteArray([]*resp.Value{})
		}
		return ctx.WriteNullBulkString()
	}

	members := make([]string, 0, len(set.Members))
	for member := range set.Members {
		members = append(members, member)
	}

	if !withCount {
		idx := 0
		if len(members) > 1 {
			idx = rand.Intn(len(members))
		}
		return ctx.WriteBulkString(members[idx])
	}

	if count > 0 && count > len(members) {
		count = len(members)
	}

	result := make([]*resp.Value, 0, count)
	for i := 0; i < count && i < len(members); i++ {
		result = append(result, resp.BulkString(members[i]))
	}

	return ctx.WriteArray(result)
}

func cmdSMOVE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	srcKey := ctx.ArgString(0)
	dstKey := ctx.ArgString(1)
	member := ctx.ArgString(2)

	srcSet, err := getSet(ctx, srcKey)
	if err != nil {
		return ctx.WriteError(err)
	}
	if srcSet == nil {
		return ctx.WriteInteger(0)
	}

	if _, exists := srcSet.Members[member]; !exists {
		return ctx.WriteInteger(0)
	}

	delete(srcSet.Members, member)
	if len(srcSet.Members) == 0 {
		ctx.Store.Delete(srcKey)
	}

	dstSet, err := getOrCreateSet(ctx, dstKey)
	if err != nil {
		return ctx.WriteError(err)
	}

	dstSet.Members[member] = struct{}{}
	return ctx.WriteInteger(1)
}

func cmdSUNION(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	result := make(map[string]struct{})
	for i := 0; i < ctx.ArgCount(); i++ {
		set, err := getSetOrEmpty(ctx, ctx.ArgString(i))
		if err != nil {
			return ctx.WriteError(err)
		}
		for member := range set.Members {
			result[member] = struct{}{}
		}
	}

	members := make([]*resp.Value, 0, len(result))
	for member := range result {
		members = append(members, resp.BulkString(member))
	}

	return ctx.WriteArray(members)
}

func cmdSINTER(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	firstSet, err := getSetOrEmpty(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}

	result := make(map[string]struct{})
	for member := range firstSet.Members {
		result[member] = struct{}{}
	}

	for i := 1; i < ctx.ArgCount(); i++ {
		set, err := getSet(ctx, ctx.ArgString(i))
		if err != nil {
			return ctx.WriteError(err)
		}
		if set == nil {
			return ctx.WriteArray([]*resp.Value{})
		}

		for member := range result {
			if _, exists := set.Members[member]; !exists {
				delete(result, member)
			}
		}
	}

	members := make([]*resp.Value, 0, len(result))
	for member := range result {
		members = append(members, resp.BulkString(member))
	}

	return ctx.WriteArray(members)
}

func cmdSDIFF(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	firstSet, err := getSetOrEmpty(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}

	result := make(map[string]struct{})
	for member := range firstSet.Members {
		result[member] = struct{}{}
	}

	for i := 1; i < ctx.ArgCount(); i++ {
		set, err := getSetOrEmpty(ctx, ctx.ArgString(i))
		if err != nil {
			return ctx.WriteError(err)
		}
		for member := range set.Members {
			delete(result, member)
		}
	}

	members := make([]*resp.Value, 0, len(result))
	for member := range result {
		members = append(members, resp.BulkString(member))
	}

	return ctx.WriteArray(members)
}

func cmdSUNIONSTORE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	dstKey := ctx.ArgString(0)
	result := make(map[string]struct{})

	for i := 1; i < ctx.ArgCount(); i++ {
		set, err := getSetOrEmpty(ctx, ctx.ArgString(i))
		if err != nil {
			return ctx.WriteError(err)
		}
		for member := range set.Members {
			result[member] = struct{}{}
		}
	}

	if len(result) == 0 {
		ctx.Store.Delete(dstKey)
		return ctx.WriteInteger(0)
	}

	dstSet := &store.SetValue{Members: result}
	ctx.Store.Set(dstKey, dstSet, store.SetOptions{})

	return ctx.WriteInteger(int64(len(result)))
}

func cmdSINTERSTORE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	dstKey := ctx.ArgString(0)

	firstSet, err := getSetOrEmpty(ctx, ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}

	result := make(map[string]struct{})
	for member := range firstSet.Members {
		result[member] = struct{}{}
	}

	for i := 2; i < ctx.ArgCount(); i++ {
		set, err := getSet(ctx, ctx.ArgString(i))
		if err != nil {
			return ctx.WriteError(err)
		}
		if set == nil {
			ctx.Store.Delete(dstKey)
			return ctx.WriteInteger(0)
		}

		for member := range result {
			if _, exists := set.Members[member]; !exists {
				delete(result, member)
			}
		}
	}

	if len(result) == 0 {
		ctx.Store.Delete(dstKey)
		return ctx.WriteInteger(0)
	}

	dstSet := &store.SetValue{Members: result}
	ctx.Store.Set(dstKey, dstSet, store.SetOptions{})

	return ctx.WriteInteger(int64(len(result)))
}

func cmdSDIFFSTORE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	dstKey := ctx.ArgString(0)

	firstSet, err := getSetOrEmpty(ctx, ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}

	result := make(map[string]struct{})
	for member := range firstSet.Members {
		result[member] = struct{}{}
	}

	for i := 2; i < ctx.ArgCount(); i++ {
		set, err := getSetOrEmpty(ctx, ctx.ArgString(i))
		if err != nil {
			return ctx.WriteError(err)
		}
		for member := range set.Members {
			delete(result, member)
		}
	}

	if len(result) == 0 {
		ctx.Store.Delete(dstKey)
		return ctx.WriteInteger(0)
	}

	dstSet := &store.SetValue{Members: result}
	ctx.Store.Set(dstKey, dstSet, store.SetOptions{})

	return ctx.WriteInteger(int64(len(result)))
}

func cmdSSCAN(ctx *Context) error {
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

	set, err := getSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if set == nil {
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("0"),
			resp.ArrayValue([]*resp.Value{}),
		})
	}

	members := make([]string, 0, len(set.Members))
	for member := range set.Members {
		if matchPattern(member, pattern) {
			members = append(members, member)
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

	result := make([]*resp.Value, 0, end-start)
	for i := start; i < end; i++ {
		result = append(result, resp.BulkString(members[i]))
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(strconv.Itoa(nextCursor)),
		resp.ArrayValue(result),
	})
}
