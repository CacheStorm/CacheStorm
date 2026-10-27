package command

import (
	"bytes"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

var errListNoSuchKey = errors.New("ERR no such key")

func RegisterListCommands(router *Router) {
	router.Register(&CommandDef{Name: "LPUSH", Handler: cmdLPUSH})
	router.Register(&CommandDef{Name: "RPUSH", Handler: cmdRPUSH})
	router.Register(&CommandDef{Name: "LPUSHX", Handler: cmdLPUSHX})
	router.Register(&CommandDef{Name: "RPUSHX", Handler: cmdRPUSHX})
	router.Register(&CommandDef{Name: "LPOP", Handler: cmdLPOP})
	router.Register(&CommandDef{Name: "RPOP", Handler: cmdRPOP})
	router.Register(&CommandDef{Name: "LLEN", Handler: cmdLLEN})
	router.Register(&CommandDef{Name: "LRANGE", Handler: cmdLRANGE})
	router.Register(&CommandDef{Name: "LINDEX", Handler: cmdLINDEX})
	router.Register(&CommandDef{Name: "LSET", Handler: cmdLSET})
	router.Register(&CommandDef{Name: "LREM", Handler: cmdLREM})
	router.Register(&CommandDef{Name: "LTRIM", Handler: cmdLTRIM})
	router.Register(&CommandDef{Name: "RPOPLPUSH", Handler: cmdRPOPLPUSH})
	router.Register(&CommandDef{Name: "LINSERT", Handler: cmdLINSERT})
	router.Register(&CommandDef{Name: "LMOVE", Handler: cmdLMOVE})
	router.Register(&CommandDef{Name: "BLMOVE", Handler: cmdBLMOVE})
	router.Register(&CommandDef{Name: "BLPOP", Handler: cmdBLPOP})
	router.Register(&CommandDef{Name: "BRPOP", Handler: cmdBRPOP})
	router.Register(&CommandDef{Name: "BRPOPLPUSH", Handler: cmdBRPOPLPUSH})
	router.Register(&CommandDef{Name: "LPOS", Handler: cmdLPOS})
	router.Register(&CommandDef{Name: "LMPOP", Handler: cmdLMPOP})
	router.Register(&CommandDef{Name: "LMPUSH", Handler: cmdLMPUSH})
	router.Register(&CommandDef{Name: "BLMPOP", Handler: cmdBLMPOP})
}

func getList(ctx *Context, key string) (*store.ListValue, error) {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return nil, nil
	}

	list, ok := entry.Value.(*store.ListValue)
	if !ok {
		return nil, store.ErrWrongType
	}

	return list, nil
}

// listPush implements LPUSH and RPUSH. A missing key is created already
// holding the elements, so a blocked client is never woken by an empty list.
func listPush(ctx *Context, left bool) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	argCount := ctx.ArgCount() - 1
	list, created, err := lockOrInstall(ctx, key, func() *store.ListValue {
		elements := make([][]byte, argCount)
		for i := 0; i < argCount; i++ {
			if left {
				// LPUSH inserts in reverse order: the last argument ends up at the head.
				elements[i] = ctx.Arg(argCount - i)
			} else {
				elements[i] = ctx.Arg(i + 1)
			}
		}
		return &store.ListValue{Elements: elements}
	})
	if err != nil {
		return ctx.WriteError(err)
	}
	event := "rpush"
	if left {
		event = "lpush"
	}
	if created {
		ctx.notify(store.NotifyList, event, key)
		return ctx.WriteInteger(int64(argCount))
	}

	if left {
		items := make([][]byte, argCount)
		for i := 0; i < argCount; i++ {
			items[i] = ctx.Arg(argCount - i)
		}
		list.PushFront(items...)
	} else {
		items := make([][]byte, argCount)
		for i := 0; i < argCount; i++ {
			items[i] = ctx.Arg(i + 1)
		}
		list.PushBack(items...)
	}
	length := int64(len(list.Elements))
	list.Unlock()

	ctx.notify(store.NotifyList, event, key)
	ctx.Store.KeyNotifier().NotifyKey(key)
	return ctx.WriteInteger(length)
}

func cmdLPUSH(ctx *Context) error {
	return listPush(ctx, true)
}

func cmdRPUSH(ctx *Context) error {
	return listPush(ctx, false)
}

func cmdLPUSHX(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	list, err := lockList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		return ctx.WriteInteger(0)
	}

	argCount := ctx.ArgCount() - 1
	items := make([][]byte, argCount)
	for i := 0; i < argCount; i++ {
		items[i] = ctx.Arg(argCount - i)
	}
	list.PushFront(items...)
	length := int64(len(list.Elements))
	list.Unlock()

	ctx.notify(store.NotifyList, "lpush", key)
	ctx.Store.KeyNotifier().NotifyKey(key)
	return ctx.WriteInteger(length)
}

func cmdRPUSHX(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	list, err := lockList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		return ctx.WriteInteger(0)
	}

	items := make([][]byte, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		items = append(items, ctx.Arg(i))
	}
	list.PushBack(items...)
	length := int64(len(list.Elements))
	list.Unlock()

	ctx.notify(store.NotifyList, "rpush", key)
	ctx.Store.KeyNotifier().NotifyKey(key)
	return ctx.WriteInteger(length)
}

// listPopGeneric implements LPOP and RPOP with the optional count.
func listPopGeneric(ctx *Context, left bool) error {
	n := ctx.ArgCount()
	if n < 1 || n > 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	hasCount := n == 2
	count := 1
	if hasCount {
		var err error
		if count, err = parsePositiveCount(ctx.ArgString(1)); err != nil {
			return ctx.WriteError(err)
		}
	}

	key := ctx.ArgString(0)
	list, err := lockList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		if hasCount {
			return ctx.WriteNullArray()
		}
		return ctx.WriteNullBulkString()
	}
	if hasCount && count == 0 {
		list.Unlock()
		return ctx.WriteArray([]*resp.Value{})
	}

	if len(list.Elements) == 0 {
		list.Unlock()
		if hasCount {
			return ctx.WriteNullArray()
		}
		return ctx.WriteNullBulkString()
	}
	if count > len(list.Elements) {
		count = len(list.Elements)
	}
	var popped [][]byte
	if left {
		popped = list.PopFront(count)
	} else {
		popped = list.PopBack(count)
	}
	empty := len(list.Elements) == 0
	list.Unlock()

	ctx.notify(store.NotifyList, popEvent(left), key)
	if empty {
		deleteIfEmpty(ctx, key, list)
	}

	if !hasCount {
		return ctx.WriteBulkBytes(popped[0])
	}
	items := make([]*resp.Value, len(popped))
	for i, v := range popped {
		items[i] = resp.BulkBytes(v)
	}
	return ctx.WriteArray(items)
}

func cmdLPOP(ctx *Context) error {
	return listPopGeneric(ctx, true)
}

func cmdRPOP(ctx *Context) error {
	return listPopGeneric(ctx, false)
}

func cmdLLEN(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	list, err := getList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		return ctx.WriteInteger(0)
	}

	list.RLock()
	length := int64(len(list.Elements))
	list.RUnlock()

	return ctx.WriteInteger(length)
}

func cmdLRANGE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
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

	list, err := getList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	list.RLock()

	length := len(list.Elements)
	if length == 0 {
		reply := []*resp.Value{}
		list.RUnlock()
		return ctx.WriteArray(reply)
	}

	if start < 0 {
		start = length + start
	}
	if stop < 0 {
		stop = length + stop
	}

	if start < 0 {
		start = 0
	}
	if stop >= length {
		stop = length - 1
	}
	if start > stop {
		reply := []*resp.Value{}
		list.RUnlock()
		return ctx.WriteArray(reply)
	}

	results := make([]*resp.Value, 0, stop-start+1)
	for i := start; i <= stop; i++ {
		results = append(results, resp.BulkBytes(list.Elements[i]))
	}

	reply := results
	list.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdLINDEX(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	index, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	list, err := getList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		return ctx.WriteNullBulkString()
	}

	list.RLock()

	length := len(list.Elements)
	if index < 0 {
		index = length + index
	}
	if index < 0 || index >= length {
		list.RUnlock()
		return ctx.WriteNullBulkString()
	}

	reply := list.Elements[index]
	list.RUnlock()
	return ctx.WriteBulkBytes(reply)
}

func cmdLSET(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	index, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	value := ctx.Arg(2)

	list, err := lockList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		return ctx.WriteError(errListNoSuchKey)
	}

	length := len(list.Elements)
	if index < 0 {
		index = length + index
	}
	if index < 0 || index >= length {
		list.Unlock()
		return ctx.WriteError(ErrIndexOutOfRange)
	}

	list.SetAt(index, value)
	list.Unlock()
	ctx.notify(store.NotifyList, "lset", key)
	return ctx.WriteOK()
}

func cmdLREM(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	count, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	value := ctx.Arg(2)

	list, err := lockList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		return ctx.WriteInteger(0)
	}

	removed := 0
	newElements := make([][]byte, 0, len(list.Elements))

	if count == 0 {
		for _, elem := range list.Elements {
			if bytes.Equal(elem, value) {
				removed++
			} else {
				newElements = append(newElements, elem)
			}
		}
	} else if count > 0 {
		for _, elem := range list.Elements {
			if removed < count && bytes.Equal(elem, value) {
				removed++
			} else {
				newElements = append(newElements, elem)
			}
		}
	} else {
		// Build in forward order then reverse to avoid O(n^2) prepend
		kept := make([][]byte, 0, len(list.Elements))
		for i := len(list.Elements) - 1; i >= 0; i-- {
			elem := list.Elements[i]
			if removed < -count && bytes.Equal(elem, value) {
				removed++
			} else {
				kept = append(kept, elem)
			}
		}
		// Reverse kept to restore original order
		for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
			kept[i], kept[j] = kept[j], kept[i]
		}
		newElements = kept
	}

	list.ReplaceElements(newElements)
	empty := len(list.Elements) == 0
	list.Unlock()

	if removed > 0 {
		ctx.notify(store.NotifyList, "lrem", key)
	}
	if empty {
		deleteIfEmpty(ctx, key, list)
	}

	return ctx.WriteInteger(int64(removed))
}

func cmdLINSERT(ctx *Context) error {
	if ctx.ArgCount() != 4 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	position := strings.ToUpper(ctx.ArgString(1))
	pivot := ctx.Arg(2)
	value := ctx.Arg(3)

	if position != "BEFORE" && position != "AFTER" {
		return ctx.WriteError(ErrSyntaxError)
	}

	list, err := lockList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		return ctx.WriteInteger(0)
	}

	defer list.Unlock()

	pivotIdx := -1
	for i, elem := range list.Elements {
		if bytes.Equal(elem, pivot) {
			pivotIdx = i
			break
		}
	}

	if pivotIdx == -1 {
		return ctx.WriteInteger(-1)
	}

	newElements := make([][]byte, 0, len(list.Elements)+1)

	if position == "BEFORE" {
		newElements = append(newElements, list.Elements[:pivotIdx]...)
		newElements = append(newElements, value)
		newElements = append(newElements, list.Elements[pivotIdx:]...)
	} else {
		newElements = append(newElements, list.Elements[:pivotIdx+1]...)
		newElements = append(newElements, value)
		newElements = append(newElements, list.Elements[pivotIdx+1:]...)
	}

	list.ReplaceElements(newElements)
	ctx.notify(store.NotifyList, "linsert", key)
	return ctx.WriteInteger(int64(len(list.Elements)))
}

func cmdLTRIM(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
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

	list, err := lockList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		return ctx.WriteOK()
	}

	length := len(list.Elements)
	if length == 0 {
		list.Unlock()
		ctx.notify(store.NotifyList, "ltrim", key)
		return ctx.WriteOK()
	}

	if start < 0 {
		start = length + start
	}
	if stop < 0 {
		stop = length + stop
	}

	if start < 0 {
		start = 0
	}
	if stop >= length {
		stop = length - 1
	}
	if start > stop || start >= length {
		list.ReplaceElements(make([][]byte, 0))
		list.Unlock()
		ctx.notify(store.NotifyList, "ltrim", key)
		deleteIfEmpty(ctx, key, list)
		return ctx.WriteOK()
	}

	list.Trim(start, stop)
	list.Unlock()
	ctx.notify(store.NotifyList, "ltrim", key)
	return ctx.WriteOK()
}

func cmdRPOPLPUSH(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	value, ok, err := tryListMove(ctx, ctx.ArgString(0), ctx.ArgString(1), "RIGHT", "LEFT")
	if err != nil {
		return ctx.WriteError(err)
	}
	if !ok {
		return ctx.WriteNullBulkString()
	}
	return ctx.WriteBulkBytes(value)
}

func cmdLMOVE(ctx *Context) error {
	if ctx.ArgCount() != 4 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	whereFrom := strings.ToUpper(ctx.ArgString(2))
	whereTo := strings.ToUpper(ctx.ArgString(3))

	// Both directions are syntax, so validate them before touching any state.
	if whereFrom != "LEFT" && whereFrom != "RIGHT" {
		return ctx.WriteError(ErrSyntaxError)
	}
	if whereTo != "LEFT" && whereTo != "RIGHT" {
		return ctx.WriteError(ErrSyntaxError)
	}

	value, ok, err := tryListMove(ctx, ctx.ArgString(0), ctx.ArgString(1), whereFrom, whereTo)
	if err != nil {
		return ctx.WriteError(err)
	}
	if !ok {
		return ctx.WriteNullBulkString()
	}
	return ctx.WriteBulkBytes(value)
}

func cmdBLMOVE(ctx *Context) error {
	if ctx.ArgCount() != 5 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	srcKey := ctx.ArgString(0)
	dstKey := ctx.ArgString(1)
	whereFrom := strings.ToUpper(ctx.ArgString(2))
	whereTo := strings.ToUpper(ctx.ArgString(3))
	if whereFrom != "LEFT" && whereFrom != "RIGHT" {
		return ctx.WriteError(ErrSyntaxError)
	}
	if whereTo != "LEFT" && whereTo != "RIGHT" {
		return ctx.WriteError(ErrSyntaxError)
	}
	timeout, err := parseBlockTimeout(ctx.ArgString(4))
	if err != nil {
		return ctx.WriteError(err)
	}
	return blockingListMove(ctx, srcKey, dstKey, whereFrom, whereTo, timeout)
}

// blockingListMove is the shared body of BLMOVE and BRPOPLPUSH. A timeout
// replies with a null array, as Redis does.
func blockingListMove(ctx *Context, srcKey, dstKey, whereFrom, whereTo string, timeout time.Duration) error {
	value, ok, err := tryListMove(ctx, srcKey, dstKey, whereFrom, whereTo)
	if err != nil {
		return ctx.WriteError(err)
	}
	if ok {
		return ctx.WriteBulkBytes(value)
	}
	var moveErr error
	waitSrc := func(keys []string, d time.Duration) (string, bool) { return srcKey, ctx.waitForKey(srcKey, d) }
	moved := blockOnKeys(ctx, []string{srcKey}, timeout, waitSrc, func(string) bool {
		value, ok, moveErr = tryListMove(ctx, srcKey, dstKey, whereFrom, whereTo)
		return ok || moveErr != nil
	})
	if moveErr != nil {
		return ctx.WriteError(moveErr)
	}
	if !moved {
		return ctx.WriteNullArray()
	}
	return ctx.WriteBulkBytes(value)
}

var (
	errTimeoutNotFloat   = errors.New("ERR timeout is not a float or out of range")
	errTimeoutNegative   = errors.New("ERR timeout is negative")
	errTimeoutOutOfRange = errors.New("ERR timeout is out of range")
)

// blockForever is the wait used for a zero timeout, which Redis treats as
// "block until data arrives".
const blockForever = time.Duration(1) << 62

// parseBlockTimeout parses a blocking command's timeout in seconds the way
// Redis' getTimeoutFromObjectOrReply does: fractional seconds are rounded up
// to whole milliseconds and 0 waits without a deadline.
func parseBlockTimeout(arg string) (time.Duration, error) {
	sec, err := strconv.ParseFloat(cHexFloat(arg), 64)
	if (err != nil && !errors.Is(err, strconv.ErrRange)) || math.IsNaN(sec) {
		return 0, errTimeoutNotFloat
	}
	ms := sec * 1000
	if ms > math.MaxInt64 {
		return 0, errTimeoutOutOfRange
	}
	tval := math.Ceil(ms)
	if tval < 0 {
		return 0, errTimeoutNegative
	}
	if tval == 0 {
		return blockForever, nil
	}
	if tval > float64(math.MaxInt64-time.Now().UnixMilli()) {
		return 0, errTimeoutOutOfRange
	}
	if tval > float64(blockForever/time.Millisecond) {
		return blockForever, nil
	}
	return time.Duration(tval) * time.Millisecond, nil
}

// pushToList adds value at the head (left) or tail of the list at key,
// creating the list when the key is missing.
func pushToList(ctx *Context, key string, value []byte, left bool) error {
	list, created, err := lockOrInstall(ctx, key, func() *store.ListValue {
		return &store.ListValue{Elements: [][]byte{value}}
	})
	if err != nil || created {
		return err
	}
	if left {
		list.PushFront(value)
	} else {
		list.PushBack(value)
	}
	list.Unlock()
	ctx.Store.KeyNotifier().NotifyKey(key)
	return nil
}

// tryListMove moves one element from srcKey to dstKey. It reports false
// when the source is missing or empty, and WRONGTYPE for either key before
// anything is popped.
func tryListMove(ctx *Context, srcKey, dstKey, whereFrom, whereTo string) ([]byte, bool, error) {
	srcList, err := getList(ctx, srcKey)
	if err != nil {
		return nil, false, err
	}
	if srcList == nil {
		return nil, false, nil
	}
	if _, err := getList(ctx, dstKey); err != nil {
		return nil, false, err
	}

	if srcList, err = lockList(ctx, srcKey); err != nil || srcList == nil {
		return nil, false, err
	}
	if len(srcList.Elements) == 0 {
		srcList.Unlock()
		return nil, false, nil
	}

	var value []byte
	if whereFrom == "LEFT" {
		value = srcList.PopFront(1)[0]
	} else {
		value = srcList.PopBack(1)[0]
	}

	if srcKey == dstKey {
		// Rotating a list in place: push back under the same lock.
		if whereTo == "LEFT" {
			srcList.PushFront(value)
		} else {
			srcList.PushBack(value)
		}
		srcList.Unlock()
		ctx.notify(store.NotifyList, pushEvent(whereTo == "LEFT"), dstKey)
		ctx.notify(store.NotifyList, popEvent(whereFrom == "LEFT"), srcKey)
		return value, true, nil
	}

	srcEmpty := len(srcList.Elements) == 0
	srcList.Unlock()
	srcDeleted := srcEmpty && ctx.Store.DeleteIfEmpty(srcKey, srcList)

	if err := pushToList(ctx, dstKey, value, whereTo == "LEFT"); err != nil {
		return nil, false, err
	}
	// Redis pushes onto the destination before it announces the pop.
	ctx.notify(store.NotifyList, pushEvent(whereTo == "LEFT"), dstKey)
	ctx.notify(store.NotifyList, popEvent(whereFrom == "LEFT"), srcKey)
	if srcDeleted {
		ctx.notify(store.NotifyGeneric, "del", srcKey)
	}
	return value, true, nil
}

// listPopFirst pops one element from the first non-empty list among keys.
// A key of another type met before it is a WRONGTYPE error, as Redis checks
// the keys of BLPOP/BRPOP in order before blocking.
func listPopFirst(ctx *Context, keys []string, left bool) (string, []byte, error) {
	for _, key := range keys {
		list, err := lockList(ctx, key)
		if err != nil {
			return "", nil, err
		}
		if list == nil {
			continue
		}
		if len(list.Elements) == 0 {
			list.Unlock()
			continue
		}
		var value []byte
		if left {
			value = list.PopFront(1)[0]
		} else {
			value = list.PopBack(1)[0]
		}
		isEmpty := len(list.Elements) == 0
		list.Unlock()
		ctx.notify(store.NotifyList, popEvent(left), key)
		if isEmpty {
			deleteIfEmpty(ctx, key, list)
		}
		return key, value, nil
	}
	return "", nil, nil
}

// blockOnKeys waits on keys and calls try with each notified key until try
// succeeds, the timeout expires or the client goes away; it reports whether
// try succeeded. try takes the value's lock itself, so nothing is read
// before the lock. The deadline is kept across retries.
func blockOnKeys(ctx *Context, keys []string, timeout time.Duration, wait func([]string, time.Duration) (string, bool), try func(key string) bool) bool {
	var deadline time.Time
	if timeout < blockForever {
		deadline = time.Now().Add(timeout)
	}
	for {
		remaining := blockForever
		if !deadline.IsZero() {
			remaining = time.Until(deadline)
			if remaining <= 0 {
				return false
			}
		}
		key, ok := wait(keys, remaining)
		if !ok {
			return false
		}
		if try(key) {
			return true
		}
	}
}

// blockingListPop is the shared body of BLPOP and BRPOP.
func blockingListPop(ctx *Context, left bool) error {
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

	key, value, err := listPopFirst(ctx, keys, left)
	if err != nil {
		return ctx.WriteError(err)
	}
	if value == nil {
		popped := blockOnKeys(ctx, keys, timeout, ctx.waitForKeys, func(k string) bool {
			var popErr error
			key, value, popErr = listPopFirst(ctx, []string{k}, left)
			return popErr == nil && value != nil
		})
		if !popped {
			return ctx.WriteNullArray()
		}
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(key),
		resp.BulkBytes(value),
	})
}

func cmdBLPOP(ctx *Context) error {
	return blockingListPop(ctx, true)
}

func cmdBRPOP(ctx *Context) error {
	return blockingListPop(ctx, false)
}

func cmdBRPOPLPUSH(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	timeout, err := parseBlockTimeout(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	return blockingListMove(ctx, ctx.ArgString(0), ctx.ArgString(1), "RIGHT", "LEFT", timeout)
}

var (
	errLPosRankZero = errors.New("ERR RANK can't be zero: use 1 to start from the first match, 2 from the second ... or use negative to start from the end of the list")
	errLPosCount    = errors.New("ERR COUNT can't be negative")
	errLPosMaxlen   = errors.New("ERR MAXLEN can't be negative")
	errLPosRank     = errors.New("ERR value is out of range, value must between -9223372036854775807 and 9223372036854775807")
)

func cmdLPOS(ctx *Context) error {
	n := ctx.ArgCount()
	if n < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	element := ctx.Arg(1)

	var rank int64 = 1
	var count int64 = -1
	var maxlen int64
	for i := 2; i < n; i++ {
		opt := strings.ToUpper(ctx.ArgString(i))
		more := i+1 < n
		switch {
		case opt == "RANK" && more:
			i++
			v, err := strconv.ParseInt(ctx.ArgString(i), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			if v == math.MinInt64 {
				return ctx.WriteError(errLPosRank)
			}
			if v == 0 {
				return ctx.WriteError(errLPosRankZero)
			}
			rank = v
		case opt == "COUNT" && more:
			i++
			v, err := strconv.ParseInt(ctx.ArgString(i), 10, 64)
			if err != nil || v < 0 {
				return ctx.WriteError(errLPosCount)
			}
			count = v
		case opt == "MAXLEN" && more:
			i++
			v, err := strconv.ParseInt(ctx.ArgString(i), 10, 64)
			if err != nil || v < 0 {
				return ctx.WriteError(errLPosMaxlen)
			}
			maxlen = v
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	list, err := getList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		if count != -1 {
			return ctx.WriteArray([]*resp.Value{})
		}
		return ctx.WriteNull()
	}

	fromTail := rank < 0
	if fromTail {
		rank = -rank
	}

	list.RLock()
	length := int64(len(list.Elements))
	var matches []int64
	var seen int64
	for idx := int64(0); idx < length && (maxlen == 0 || idx < maxlen); idx++ {
		pos := idx
		if fromTail {
			pos = length - 1 - idx
		}
		if !bytes.Equal(list.Elements[pos], element) {
			continue
		}
		seen++
		if seen < rank {
			continue
		}
		matches = append(matches, pos)
		if count == -1 || (count > 0 && int64(len(matches)) >= count) {
			break
		}
	}
	list.RUnlock()

	if count == -1 {
		if len(matches) == 0 {
			return ctx.WriteNull()
		}
		return ctx.WriteInteger(matches[0])
	}
	items := make([]*resp.Value, len(matches))
	for i, m := range matches {
		items[i] = resp.IntegerValue(m)
	}
	return ctx.WriteArray(items)
}

func cmdLMPOP(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	keys, dir, count, err := parseListMPop(ctx, 0)
	if err != nil {
		return ctx.WriteError(err)
	}
	reply, err := tryListMPop(ctx, keys, dir, count)
	if err != nil {
		return ctx.WriteError(err)
	}
	if reply == nil {
		return ctx.WriteNullArray()
	}
	return ctx.WriteValue(reply)
}

// parseListMPop parses the "numkeys key [key ...] LEFT|RIGHT [COUNT count]"
// tail shared by LMPOP and BLMPOP, starting at argument index from, with
// Redis' error replies.
func parseListMPop(ctx *Context, from int) ([]string, string, int, error) {
	n := ctx.ArgCount()
	if n < from+1 {
		return nil, "", 0, arityError(ctx.Command)
	}
	numKeys, err := strconv.ParseInt(ctx.ArgString(from), 10, 64)
	if err != nil || numKeys < 1 {
		return nil, "", 0, errNumKeysPositive
	}
	whereIdx := int64(from) + numKeys + 1
	if whereIdx >= int64(n) {
		return nil, "", 0, ErrSyntaxError
	}

	keys := make([]string, numKeys)
	for i := range keys {
		keys[i] = ctx.ArgString(from + 1 + i)
	}

	dir := strings.ToUpper(ctx.ArgString(int(whereIdx)))
	if dir != "LEFT" && dir != "RIGHT" {
		return nil, "", 0, ErrSyntaxError
	}

	count := -1
	for j := int(whereIdx) + 1; j < n; j++ {
		if count == -1 && strings.EqualFold(ctx.ArgString(j), "COUNT") && j+1 < n {
			v, err := strconv.ParseInt(ctx.ArgString(j+1), 10, 64)
			if err != nil || v < 1 {
				return nil, "", 0, errPopCountPositive
			}
			if v > math.MaxInt32 {
				v = math.MaxInt32
			}
			count = int(v)
			j++
			continue
		}
		return nil, "", 0, ErrSyntaxError
	}
	if count == -1 {
		count = 1
	}
	return keys, dir, count, nil
}

// tryListMPop pops up to count elements from the first non-empty list among
// keys. It returns nil when every list is empty.
func tryListMPop(ctx *Context, keys []string, dir string, count int) (*resp.Value, error) {
	for _, key := range keys {
		list, err := lockList(ctx, key)
		if err != nil {
			return nil, err
		}
		if list == nil {
			continue
		}
		if len(list.Elements) == 0 {
			list.Unlock()
			continue
		}
		elements := make([]*resp.Value, 0, min(count, len(list.Elements)))
		for i := 0; i < count && len(list.Elements) > 0; i++ {
			var value []byte
			if dir == "LEFT" {
				value = list.PopFront(1)[0]
			} else {
				value = list.PopBack(1)[0]
			}
			elements = append(elements, resp.BulkBytes(value))
		}
		isEmpty := len(list.Elements) == 0
		list.Unlock()
		ctx.notify(store.NotifyList, popEvent(dir == "LEFT"), key)
		if isEmpty {
			deleteIfEmpty(ctx, key, list)
		}
		return resp.ArrayValue([]*resp.Value{
			resp.BulkString(key),
			resp.ArrayValue(elements),
		}), nil
	}
	return nil, nil
}

func cmdLMPUSH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}

	key := ctx.ArgString(0)
	elements := make([][]byte, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		elements = append(elements, ctx.Arg(i))
	}

	list, err := lockOrCreateList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	list.PushFront(elements...)
	length := int64(len(list.Elements))
	list.Unlock()

	ctx.notify(store.NotifyList, "lpush", key)
	ctx.Store.KeyNotifier().NotifyKey(key)
	return ctx.WriteInteger(length)
}

func cmdBLMPOP(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	timeout, err := parseBlockTimeout(ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	keys, dir, count, err := parseListMPop(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}

	reply, err := tryListMPop(ctx, keys, dir, count)
	if err != nil {
		return ctx.WriteError(err)
	}
	if reply == nil {
		popped := blockOnKeys(ctx, keys, timeout, ctx.waitForKeys, func(k string) bool {
			var popErr error
			reply, popErr = tryListMPop(ctx, []string{k}, dir, count)
			return popErr == nil && reply != nil
		})
		if !popped {
			return ctx.WriteNullArray()
		}
	}
	return ctx.WriteValue(reply)
}

func pushEvent(left bool) string {
	if left {
		return "lpush"
	}
	return "rpush"
}

func popEvent(left bool) string {
	if left {
		return "lpop"
	}
	return "rpop"
}
