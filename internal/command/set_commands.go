package command

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A count argument that must be non-negative: SPOP's pop count and XTRIM's
// MAXLEN both reject a negative value. (SRANDMEMBER instead treats a negative
// count as a repetition count, per Redis.)
var errCountOutOfRange = errors.New("ERR value is out of range, must be positive")

func RegisterSetCommands(router *Router) {
	router.Register(&CommandDef{Name: "SADD", Handler: cmdSADD})
	router.Register(&CommandDef{Name: "SREM", Handler: cmdSREM})
	router.Register(&CommandDef{Name: "SMEMBERS", Handler: cmdSMEMBERS})
	router.Register(&CommandDef{Name: "SISMEMBER", Handler: cmdSISMEMBER})
	router.Register(&CommandDef{Name: "SMISMEMBER", Handler: cmdSMISMEMBER})
	router.Register(&CommandDef{Name: "SCARD", Handler: cmdSCARD})
	router.Register(&CommandDef{Name: "SPOP", Handler: cmdSPOP})
	router.Register(&CommandDef{Name: "SRANDMEMBER", Handler: cmdSRANDMEMBER})
	router.Register(&CommandDef{Name: "SMOVE", Handler: cmdSMOVE})
	router.Register(&CommandDef{Name: "SUNION", Handler: cmdSUNION})
	router.Register(&CommandDef{Name: "SINTER", Handler: cmdSINTER})
	router.Register(&CommandDef{Name: "SINTERCARD", Handler: cmdSINTERCARD})
	router.Register(&CommandDef{Name: "SDIFF", Handler: cmdSDIFF})
	router.Register(&CommandDef{Name: "SUNIONSTORE", Handler: cmdSUNIONSTORE})
	router.Register(&CommandDef{Name: "SINTERSTORE", Handler: cmdSINTERSTORE})
	router.Register(&CommandDef{Name: "SDIFFSTORE", Handler: cmdSDIFFSTORE})
	router.Register(&CommandDef{Name: "SSCAN", Handler: cmdSSCAN})
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
		return ctx.WriteError(arityError(ctx.Command))
	}

	ctx.writeEffectsHandled = true
	members := make([]string, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		members[i-1] = ctx.ArgString(i)
	}
	added, err := ctx.Store.AddSetMembers(ctx.ArgString(0), members)
	if err != nil {
		return ctx.WriteError(err)
	}
	if added == 0 {
		ctx.PropagateAs()
	} else {
		ctx.notify(store.NotifySet, "sadd", ctx.ArgString(0))
	}
	return ctx.WriteInteger(int64(added))
}

func cmdSREM(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	set, err := lockSet(ctx, key)
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
			set.RemoveMember(member)
			removed++
		}
	}

	isEmpty := len(set.Members) == 0
	set.Unlock()

	if removed > 0 {
		ctx.notify(store.NotifySet, "srem", key)
	} else if !isEmpty {
		ctx.PropagateAs()
	}
	if isEmpty {
		deleteIfEmpty(ctx, key, set)
	}

	return ctx.WriteInteger(int64(removed))
}

func cmdSMEMBERS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	set, err := getSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if set == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	set.RLock()
	members := make([]*resp.Value, 0, len(set.Members))
	for member := range set.Members {
		members = append(members, resp.BulkString(member))
	}

	reply := members
	set.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdSISMEMBER(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
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

	set.RLock()
	if _, exists := set.Members[member]; exists {
		reply := int64(1)
		set.RUnlock()
		return ctx.WriteInteger(reply)
	}
	reply := int64(0)
	set.RUnlock()
	return ctx.WriteInteger(reply)
}

func cmdSCARD(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	set, err := getSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if set == nil {
		return ctx.WriteInteger(0)
	}

	set.RLock()
	reply := int64(int64(len(set.Members)))
	set.RUnlock()
	return ctx.WriteInteger(reply)
}

func cmdSPOP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	if ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrSyntaxError)
	}
	key := ctx.ArgString(0)
	count := 1
	withCount := ctx.ArgCount() == 2
	if withCount {
		var err error
		if count, err = parsePositiveCount(ctx.ArgString(1)); err != nil {
			return ctx.WriteError(err)
		}
	}

	set, err := lockSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if set == nil {
		ctx.PropagateAs()
		if !withCount {
			return ctx.WriteNullBulkString()
		}
		return ctx.WriteArray([]*resp.Value{})
	}

	if !withCount {
		for member := range set.Members {
			set.RemoveMember(member)
			isEmpty := len(set.Members) == 0
			set.Unlock()
			ctx.notify(store.NotifySet, "spop", key)
			if isEmpty {
				deleteIfEmpty(ctx, key, set)
			}
			propagateSREM(ctx, key, member)
			return ctx.WriteBulkString(member)
		}
		set.Unlock()
		ctx.PropagateAs()
		return ctx.WriteNullBulkString()
	}

	if count >= len(set.Members) {
		members := make([]*resp.Value, 0, len(set.Members))
		popped := make([]string, 0, len(set.Members))
		for member := range set.Members {
			members = append(members, resp.BulkString(member))
			popped = append(popped, member)
		}
		set.ClearMembers()
		set.Unlock()
		if len(popped) > 0 {
			ctx.notify(store.NotifySet, "spop", key)
		}
		deleteIfEmpty(ctx, key, set)
		propagateSREM(ctx, key, popped...)
		return ctx.WriteArray(members)
	}

	members := make([]*resp.Value, 0, count)
	popped := make([]string, 0, count)
	i := 0
	for member := range set.Members {
		if i >= count {
			break
		}
		members = append(members, resp.BulkString(member))
		popped = append(popped, member)
		set.RemoveMember(member)
		i++
	}
	set.Unlock()
	if len(popped) > 0 {
		ctx.notify(store.NotifySet, "spop", key)
	}

	propagateSREM(ctx, key, popped...)
	return ctx.WriteArray(members)
}

// propagateSREM records an SPOP as the SREM of the members it popped: SPOP
// picks members at random, so replaying it would remove different ones.
func propagateSREM(ctx *Context, key string, members ...string) {
	if len(members) == 0 {
		ctx.PropagateAs()
		return
	}
	args := make([][]byte, 0, len(members)+1)
	args = append(args, []byte(key))
	for _, m := range members {
		args = append(args, []byte(m))
	}
	ctx.PropagateAs(ExecutedCommand{Name: "SREM", Args: args, DB: ctx.DB})
}

func cmdSRANDMEMBER(ctx *Context) error {
	n := ctx.ArgCount()
	if n < 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	if n > 2 {
		return ctx.WriteError(ErrSyntaxError)
	}

	key := ctx.ArgString(0)
	withCount := n == 2
	var count int64 = 1
	if withCount {
		var err error
		if count, err = parseRandCount(ctx.ArgString(1)); err != nil {
			return ctx.WriteError(err)
		}
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

	set.RLock()
	if !withCount {
		member, ok := randomKey(set.Members)
		set.RUnlock()
		if !ok {
			return ctx.WriteNullBulkString()
		}
		return ctx.WriteBulkString(member)
	}
	picked := randomKeys(set.Members, count)
	set.RUnlock()
	result := make([]*resp.Value, len(picked))
	for i, m := range picked {
		result[i] = resp.BulkString(m)
	}
	return ctx.WriteArray(result)
}

func cmdSMOVE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	ctx.writeEffectsHandled = true
	moved, err := ctx.Store.MoveSetMember(ctx.ArgString(0), ctx.ArgString(1), ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}

	if !moved || ctx.ArgString(0) == ctx.ArgString(1) {
		ctx.PropagateAs()
	}
	if moved {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdSUNION(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	// The shared set-operations lock: a concurrent MoveSetMember takes it
	// exclusively, so this multi-set snapshot can never observe the move
	// half-applied.
	ctx.Store.SetOpsRLock()

	// Resolve every source set, then lock them all simultaneously in
	// deterministic key order. Holding every lock at once makes the union
	// an atomic multi-set snapshot: a concurrent SMOVE (which acquires the
	// same locks in the same order) can never be observed half-applied.
	type keyedSet struct {
		key string
		set *store.SetValue
	}
	sets := make([]keyedSet, 0, ctx.ArgCount())
	for i := 0; i < ctx.ArgCount(); i++ {
		key := ctx.ArgString(i)
		set, err := getSetOrEmpty(ctx, key)
		if err != nil {
			replyErr := err
			ctx.Store.SetOpsRUnlock()
			return ctx.WriteError(replyErr)
		}
		sets = append(sets, keyedSet{key: key, set: set})
	}
	sort.Slice(sets, func(i, j int) bool { return sets[i].key < sets[j].key })
	for _, ks := range sets {
		ks.set.RLock()
	}
	defer func() {
		for _, ks := range sets {
			ks.set.RUnlock()
		}
	}()

	result := make(map[string]struct{})
	for _, ks := range sets {
		for member := range ks.set.Members {
			result[member] = struct{}{}
		}
	}

	members := make([]*resp.Value, 0, len(result))
	for member := range result {
		members = append(members, resp.BulkString(member))
	}

	reply := members
	ctx.Store.SetOpsRUnlock()
	return ctx.WriteArray(reply)
}

func cmdSINTER(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	// The shared set-operations lock: a concurrent MoveSetMember takes it
	// exclusively, so this multi-set read can never observe the move
	// half-applied.
	ctx.Store.SetOpsRLock()
	firstSet, err := getSetOrEmpty(ctx, ctx.ArgString(0))
	if err != nil {
		replyErr := err
		ctx.Store.SetOpsRUnlock()
		return ctx.WriteError(replyErr)
	}

	firstSet.RLock()
	result := make(map[string]struct{})
	for member := range firstSet.Members {
		result[member] = struct{}{}
	}
	firstSet.RUnlock()

	for i := 1; i < ctx.ArgCount(); i++ {
		set, err := getSet(ctx, ctx.ArgString(i))
		if err != nil {
			replyErr := err
			ctx.Store.SetOpsRUnlock()
			return ctx.WriteError(replyErr)
		}
		if set == nil {
			clear(result)
			continue
		}

		set.RLock()
		for member := range result {
			if _, exists := set.Members[member]; !exists {
				delete(result, member)
			}
		}
		set.RUnlock()
	}

	members := make([]*resp.Value, 0, len(result))
	for member := range result {
		members = append(members, resp.BulkString(member))
	}

	reply := members
	ctx.Store.SetOpsRUnlock()
	return ctx.WriteArray(reply)
}

func cmdSDIFF(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	// The shared set-operations lock: a concurrent MoveSetMember takes it
	// exclusively, so this multi-set read can never observe the move
	// half-applied.
	ctx.Store.SetOpsRLock()
	firstSet, err := getSetOrEmpty(ctx, ctx.ArgString(0))
	if err != nil {
		replyErr := err
		ctx.Store.SetOpsRUnlock()
		return ctx.WriteError(replyErr)
	}

	result := make(map[string]struct{})
	firstSet.RLock()
	for member := range firstSet.Members {
		result[member] = struct{}{}
	}
	firstSet.RUnlock()

	for i := 1; i < ctx.ArgCount(); i++ {
		set, err := getSetOrEmpty(ctx, ctx.ArgString(i))
		if err != nil {
			replyErr := err
			ctx.Store.SetOpsRUnlock()
			return ctx.WriteError(replyErr)
		}
		set.RLock()
		for member := range set.Members {
			delete(result, member)
		}
		set.RUnlock()
	}

	members := make([]*resp.Value, 0, len(result))
	for member := range result {
		members = append(members, resp.BulkString(member))
	}

	reply := members
	ctx.Store.SetOpsRUnlock()
	return ctx.WriteArray(reply)
}

func cmdSUNIONSTORE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	// The shared set-operations lock: a concurrent MoveSetMember takes it
	// exclusively, so this multi-set read can never observe the move
	// half-applied.
	ctx.writeEffectsHandled = true
	ctx.Store.SetOpsRLock()
	dstKey := ctx.ArgString(0)
	result := make(map[string]struct{})

	for i := 1; i < ctx.ArgCount(); i++ {
		set, err := getSetOrEmpty(ctx, ctx.ArgString(i))
		if err != nil {
			replyErr := err
			ctx.Store.SetOpsRUnlock()
			return ctx.WriteError(replyErr)
		}
		set.RLock()
		for member := range set.Members {
			result[member] = struct{}{}
		}
		set.RUnlock()
	}

	if len(result) == 0 {
		if ctx.Store.Delete(dstKey) {
			ctx.notify(store.NotifyGeneric, "del", dstKey)
			ctx.Store.KeyNotifier().NotifyKey(dstKey)
		} else {
			ctx.PropagateAs()
		}
		reply := int64(0)
		ctx.Store.SetOpsRUnlock()
		return ctx.WriteInteger(reply)
	}

	dstSet := &store.SetValue{Members: result}
	if err := ctx.Store.Set(dstKey, dstSet, store.SetOptions{}); err != nil {
		replyErr := err
		ctx.Store.SetOpsRUnlock()
		return ctx.WriteError(replyErr)
	}
	ctx.notify(store.NotifySet, "sunionstore", dstKey)

	reply := int64(int64(len(result)))
	ctx.Store.SetOpsRUnlock()
	return ctx.WriteInteger(reply)
}

func cmdSINTERSTORE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	// The shared set-operations lock: a concurrent MoveSetMember takes it
	// exclusively, so this multi-set read can never observe the move
	// half-applied.
	ctx.writeEffectsHandled = true
	ctx.Store.SetOpsRLock()
	dstKey := ctx.ArgString(0)

	firstSet, err := getSetOrEmpty(ctx, ctx.ArgString(1))
	if err != nil {
		replyErr := err
		ctx.Store.SetOpsRUnlock()
		return ctx.WriteError(replyErr)
	}

	result := make(map[string]struct{})
	firstSet.RLock()
	for member := range firstSet.Members {
		result[member] = struct{}{}
	}
	firstSet.RUnlock()

	for i := 2; i < ctx.ArgCount(); i++ {
		set, err := getSet(ctx, ctx.ArgString(i))
		if err != nil {
			replyErr := err
			ctx.Store.SetOpsRUnlock()
			return ctx.WriteError(replyErr)
		}
		if set == nil {
			clear(result)
			continue
		}

		set.RLock()
		for member := range result {
			if _, exists := set.Members[member]; !exists {
				delete(result, member)
			}
		}
		set.RUnlock()
	}

	if len(result) == 0 {
		if ctx.Store.Delete(dstKey) {
			ctx.notify(store.NotifyGeneric, "del", dstKey)
			ctx.Store.KeyNotifier().NotifyKey(dstKey)
		} else {
			ctx.PropagateAs()
		}
		reply := int64(0)
		ctx.Store.SetOpsRUnlock()
		return ctx.WriteInteger(reply)
	}

	dstSet := &store.SetValue{Members: result}
	if err := ctx.Store.Set(dstKey, dstSet, store.SetOptions{}); err != nil {
		replyErr := err
		ctx.Store.SetOpsRUnlock()
		return ctx.WriteError(replyErr)
	}
	ctx.notify(store.NotifySet, "sinterstore", dstKey)

	reply := int64(int64(len(result)))
	ctx.Store.SetOpsRUnlock()
	return ctx.WriteInteger(reply)
}

func cmdSDIFFSTORE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	// The shared set-operations lock: a concurrent MoveSetMember takes it
	// exclusively, so this multi-set read can never observe the move
	// half-applied.
	ctx.writeEffectsHandled = true
	ctx.Store.SetOpsRLock()
	dstKey := ctx.ArgString(0)

	firstSet, err := getSetOrEmpty(ctx, ctx.ArgString(1))
	if err != nil {
		replyErr := err
		ctx.Store.SetOpsRUnlock()
		return ctx.WriteError(replyErr)
	}

	result := make(map[string]struct{})
	firstSet.RLock()
	for member := range firstSet.Members {
		result[member] = struct{}{}
	}
	firstSet.RUnlock()

	for i := 2; i < ctx.ArgCount(); i++ {
		set, err := getSetOrEmpty(ctx, ctx.ArgString(i))
		if err != nil {
			replyErr := err
			ctx.Store.SetOpsRUnlock()
			return ctx.WriteError(replyErr)
		}
		set.RLock()
		for member := range set.Members {
			delete(result, member)
		}
		set.RUnlock()
	}

	if len(result) == 0 {
		if ctx.Store.Delete(dstKey) {
			ctx.notify(store.NotifyGeneric, "del", dstKey)
			ctx.Store.KeyNotifier().NotifyKey(dstKey)
		} else {
			ctx.PropagateAs()
		}
		reply := int64(0)
		ctx.Store.SetOpsRUnlock()
		return ctx.WriteInteger(reply)
	}

	dstSet := &store.SetValue{Members: result}
	if err := ctx.Store.Set(dstKey, dstSet, store.SetOptions{}); err != nil {
		replyErr := err
		ctx.Store.SetOpsRUnlock()
		return ctx.WriteError(replyErr)
	}
	ctx.notify(store.NotifySet, "sdiffstore", dstKey)

	reply := int64(int64(len(result)))
	ctx.Store.SetOpsRUnlock()
	return ctx.WriteInteger(reply)
}

func cmdSSCAN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	cursor, err := parseCollectionScanCursor(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}

	set, err := getSet(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if set == nil {
		return writeEmptyCollectionScan(ctx)
	}

	opts, err := parseCollectionScanOptions(ctx, 2, false)
	if err != nil {
		return ctx.WriteError(err)
	}

	// SADD/SREM/SMOVE mutate Members under the set's write lock, so the page
	// is selected under the matching read lock.
	set.RLock()
	members, next := collectionScanPage(set, len(set.Members), cursor, opts.count, func(emit func(string)) {
		for m := range set.Members {
			emit(m)
		}
	})
	set.RUnlock()

	items := make([]*resp.Value, 0, len(members))
	for _, m := range members {
		if opts.match && !matchPattern(m, opts.pattern) {
			continue
		}
		items = append(items, resp.BulkString(m))
	}
	return writeCollectionScanReply(ctx, next, items)
}

var errSetTooManyKeys = errors.New("ERR Number of keys can't be greater than number of args")

func cmdSINTERCARD(ctx *Context) error {
	n := ctx.ArgCount()
	if n < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	// Redis grammar: SINTERCARD numkeys key [key ...] [LIMIT limit].
	numKeys, err := strconv.ParseInt(ctx.ArgString(0), 10, 64)
	if err != nil || numKeys < 1 {
		return ctx.WriteError(errNumKeysPositive)
	}
	if numKeys > int64(n-1) {
		return ctx.WriteError(errSetTooManyKeys)
	}

	var limit int64
	for j := 1 + int(numKeys); j < n; j++ {
		if strings.EqualFold(ctx.ArgString(j), "LIMIT") && j+1 < n {
			v, err := strconv.ParseInt(ctx.ArgString(j+1), 10, 64)
			if err != nil || v < 0 {
				return ctx.WriteError(errLimitNegative)
			}
			limit = v
			j++
			continue
		}
		return ctx.WriteError(ErrSyntaxError)
	}

	// The shared set-operations lock: a concurrent MoveSetMember takes it
	// exclusively, so this multi-set read can never observe the move
	// half-applied.
	ctx.Store.SetOpsRLock()
	sets := make([]*store.SetValue, 0, numKeys)
	missing := false
	for i := 1; i <= int(numKeys); i++ {
		set, err := getSet(ctx, ctx.ArgString(i))
		if err != nil {
			ctx.Store.SetOpsRUnlock()
			return ctx.WriteError(err)
		}
		if set == nil {
			missing = true
			continue
		}
		sets = append(sets, set)
	}

	count := int64(0)
	if !missing {
		// Start from the smallest set and filter it through the others, one
		// lock at a time.
		smallest, smallestLen := 0, -1
		for i, s := range sets {
			s.RLock()
			size := len(s.Members)
			s.RUnlock()
			if smallestLen < 0 || size < smallestLen {
				smallest, smallestLen = i, size
			}
		}
		sets[0], sets[smallest] = sets[smallest], sets[0]
		sets[0].RLock()
		result := make(map[string]struct{}, len(sets[0].Members))
		for m := range sets[0].Members {
			result[m] = struct{}{}
		}
		sets[0].RUnlock()
		for _, s := range sets[1:] {
			s.RLock()
			for m := range result {
				if _, ok := s.Members[m]; !ok {
					delete(result, m)
				}
			}
			s.RUnlock()
		}
		count = int64(len(result))
		if limit > 0 && count > limit {
			count = limit
		}
	}
	ctx.Store.SetOpsRUnlock()
	return ctx.WriteInteger(count)
}

func cmdSMISMEMBER(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)

	set, err := getSet(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	result := make([]*resp.Value, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		member := ctx.ArgString(i)
		if set == nil {
			result = append(result, resp.IntegerValue(0))
		} else {
			set.RLock()
			_, exists := set.Members[member]
			set.RUnlock()
			if exists {
				result = append(result, resp.IntegerValue(1))
			} else {
				result = append(result, resp.IntegerValue(0))
			}
		}
	}

	return ctx.WriteArray(result)
}
