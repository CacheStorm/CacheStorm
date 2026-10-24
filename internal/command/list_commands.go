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

var errLPosRankZero = errors.New("ERR RANK should not be 0")

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

func getOrCreateList(ctx *Context, key string) (*store.ListValue, error) {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		list := &store.ListValue{Elements: make([][]byte, 0)}
		ctx.Store.Set(key, list, store.SetOptions{})
		return list, nil
	}

	list, ok := entry.Value.(*store.ListValue)
	if !ok {
		return nil, store.ErrWrongType
	}

	return list, nil
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

func cmdLPUSH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	list, err := getOrCreateList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	list.Lock()
	argCount := ctx.ArgCount() - 1
	newElements := make([][]byte, argCount+len(list.Elements))
	// LPUSH inserts in reverse order (last arg ends up at head), matching Redis behavior
	for i := 0; i < argCount; i++ {
		newElements[i] = ctx.Arg(argCount - i)
	}
	copy(newElements[argCount:], list.Elements)
	list.Elements = newElements
	length := int64(len(list.Elements))
	list.Unlock()

	ctx.Store.KeyNotifier().NotifyKey(key)
	return ctx.WriteInteger(length)
}

func cmdRPUSH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	list, err := getOrCreateList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	list.Lock()
	for i := 1; i < ctx.ArgCount(); i++ {
		value := ctx.Arg(i)
		list.Elements = append(list.Elements, value)
	}
	length := int64(len(list.Elements))
	list.Unlock()

	ctx.Store.KeyNotifier().NotifyKey(key)
	return ctx.WriteInteger(length)
}

func cmdLPUSHX(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	list, err := getList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		return ctx.WriteInteger(0)
	}

	list.Lock()
	argCount := ctx.ArgCount() - 1
	newElements := make([][]byte, argCount+len(list.Elements))
	for i := 0; i < argCount; i++ {
		newElements[i] = ctx.Arg(argCount - i)
	}
	copy(newElements[argCount:], list.Elements)
	list.Elements = newElements
	length := int64(len(list.Elements))
	list.Unlock()

	return ctx.WriteInteger(length)
}

func cmdRPUSHX(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	list, err := getList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		return ctx.WriteInteger(0)
	}

	list.Lock()
	for i := 1; i < ctx.ArgCount(); i++ {
		value := ctx.Arg(i)
		list.Elements = append(list.Elements, value)
	}
	length := int64(len(list.Elements))
	list.Unlock()

	return ctx.WriteInteger(length)
}

func cmdLPOP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	list, err := getList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		return ctx.WriteNullBulkString()
	}

	list.Lock()
	if len(list.Elements) == 0 {
		list.Unlock()
		return ctx.WriteNullBulkString()
	}
	value := list.Elements[0]
	list.Elements = list.Elements[1:]
	empty := len(list.Elements) == 0
	list.Unlock()

	if empty {
		ctx.Store.Delete(key)
	}

	return ctx.WriteBulkBytes(value)
}

func cmdRPOP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	list, err := getList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		return ctx.WriteNullBulkString()
	}

	list.Lock()
	if len(list.Elements) == 0 {
		list.Unlock()
		return ctx.WriteNullBulkString()
	}
	idx := len(list.Elements) - 1
	value := list.Elements[idx]
	list.Elements = list.Elements[:idx]
	empty := len(list.Elements) == 0
	list.Unlock()

	if empty {
		ctx.Store.Delete(key)
	}

	return ctx.WriteBulkBytes(value)
}

func cmdLLEN(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
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
		return ctx.WriteError(ErrWrongArgCount)
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
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	index, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	value := ctx.Arg(2)

	list, err := getList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		return ctx.WriteError(store.ErrKeyNotFound)
	}

	list.Lock()
	length := len(list.Elements)
	if index < 0 {
		index = length + index
	}
	if index < 0 || index >= length {
		list.Unlock()
		return ctx.WriteError(ErrIndexOutOfRange)
	}

	list.Elements[index] = value
	list.Unlock()
	return ctx.WriteOK()
}

func cmdLREM(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	count, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	value := ctx.Arg(2)

	list, err := getList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		return ctx.WriteInteger(0)
	}

	list.Lock()

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

	list.Elements = newElements
	empty := len(list.Elements) == 0
	list.Unlock()

	if empty {
		ctx.Store.Delete(key)
	}

	return ctx.WriteInteger(int64(removed))
}

func cmdLINSERT(ctx *Context) error {
	if ctx.ArgCount() != 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	position := strings.ToUpper(ctx.ArgString(1))
	pivot := ctx.Arg(2)
	value := ctx.Arg(3)

	if position != "BEFORE" && position != "AFTER" {
		return ctx.WriteError(ErrSyntaxError)
	}

	list, err := getList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		return ctx.WriteInteger(0)
	}

	list.Lock()
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

	list.Elements = newElements
	return ctx.WriteInteger(int64(len(list.Elements)))
}

func cmdLTRIM(ctx *Context) error {
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

	list, err := getList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		return ctx.WriteOK()
	}

	list.Lock()
	length := len(list.Elements)
	if length == 0 {
		list.Unlock()
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
		list.Elements = make([][]byte, 0)
		list.Unlock()
		ctx.Store.Delete(key)
		return ctx.WriteOK()
	}

	list.Elements = list.Elements[start : stop+1]
	list.Unlock()
	return ctx.WriteOK()
}

func cmdRPOPLPUSH(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	srcKey := ctx.ArgString(0)
	dstKey := ctx.ArgString(1)

	srcList, err := getList(ctx, srcKey)
	if err != nil {
		return ctx.WriteError(err)
	}
	if srcList == nil || len(srcList.Elements) == 0 {
		return ctx.WriteNullBulkString()
	}
	dstList, err := getOrCreateList(ctx, dstKey)
	if err != nil {
		return ctx.WriteError(err)
	}

	idx := len(srcList.Elements) - 1
	value := srcList.Elements[idx]
	srcList.Elements = srcList.Elements[:idx]

	if len(srcList.Elements) == 0 && srcKey != dstKey {
		ctx.Store.Delete(srcKey)
	}

	newElements := make([][]byte, 1+len(dstList.Elements))
	newElements[0] = value
	copy(newElements[1:], dstList.Elements)
	dstList.Elements = newElements

	return ctx.WriteBulkBytes(value)
}

func cmdLMOVE(ctx *Context) error {
	if ctx.ArgCount() != 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	srcKey := ctx.ArgString(0)
	dstKey := ctx.ArgString(1)
	whereFrom := strings.ToUpper(ctx.ArgString(2))
	whereTo := strings.ToUpper(ctx.ArgString(3))

	// Both directions are syntax, so validate them BEFORE touching any state.
	// The whereTo check used to live in a switch at the very end of the
	// handler, by which time the element had already been popped off the
	// source (deleting the key when it emptied) and the destination key had
	// already been created — so a client typo destroyed a list element behind
	// an "ERR syntax error" reply. The sibling cmdBLMOVE already validated
	// both up front.
	if whereFrom != "LEFT" && whereFrom != "RIGHT" {
		return ctx.WriteError(ErrSyntaxError)
	}
	if whereTo != "LEFT" && whereTo != "RIGHT" {
		return ctx.WriteError(ErrSyntaxError)
	}

	srcList, err := getList(ctx, srcKey)
	if err != nil {
		return ctx.WriteError(err)
	}
	if srcList == nil || len(srcList.Elements) == 0 {
		return ctx.WriteNullBulkString()
	}
	dstList, err := getOrCreateList(ctx, dstKey)
	if err != nil {
		return ctx.WriteError(err)
	}

	var value []byte
	if whereFrom == "LEFT" {
		value = srcList.Elements[0]
		srcList.Elements = srcList.Elements[1:]
	} else {
		value = srcList.Elements[len(srcList.Elements)-1]
		srcList.Elements = srcList.Elements[:len(srcList.Elements)-1]
	}

	if len(srcList.Elements) == 0 && srcKey != dstKey {
		ctx.Store.Delete(srcKey)
	}

	if whereTo == "LEFT" {
		newElements := make([][]byte, 1+len(dstList.Elements))
		newElements[0] = value
		copy(newElements[1:], dstList.Elements)
		dstList.Elements = newElements
	} else {
		dstList.Elements = append(dstList.Elements, value)
	}

	return ctx.WriteBulkBytes(value)
}

func cmdBLMOVE(ctx *Context) error {
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	srcKey := ctx.ArgString(0)
	dstKey := ctx.ArgString(1)
	whereFrom := strings.ToUpper(ctx.ArgString(2))
	whereTo := strings.ToUpper(ctx.ArgString(3))
	timeout, err := parseBlockTimeout(ctx.ArgString(4))
	if err != nil {
		return ctx.WriteError(err)
	}

	if whereFrom != "LEFT" && whereFrom != "RIGHT" {
		return ctx.WriteError(ErrSyntaxError)
	}
	if whereTo != "LEFT" && whereTo != "RIGHT" {
		return ctx.WriteError(ErrSyntaxError)
	}

	// Try immediate move
	if value, ok, err := tryListMove(ctx, srcKey, dstKey, whereFrom, whereTo); err != nil {
		return ctx.WriteError(err)
	} else if ok {
		return ctx.WriteBulkBytes(value)
	}

	dur := timeout

	for {
		if !ctx.waitForKey(srcKey, dur) {
			return ctx.WriteNull()
		}
		if value, ok, err := tryListMove(ctx, srcKey, dstKey, whereFrom, whereTo); err != nil {
			return ctx.WriteError(err)
		} else if ok {
			return ctx.WriteBulkBytes(value)
		}
	}
}

var (
	errTimeoutNotFloat = errors.New("ERR timeout is not a float or out of range")
	errTimeoutNegative = errors.New("ERR timeout is negative")
)

// blockForever is the wait used for a zero timeout, which Redis treats as
// "block until data arrives".
const blockForever = time.Duration(1) << 62

// parseBlockTimeout parses a blocking command's timeout in seconds. Redis
// accepts fractional seconds and treats 0 as an unbounded wait.
func parseBlockTimeout(arg string) (time.Duration, error) {
	sec, err := strconv.ParseFloat(arg, 64)
	if err != nil || math.IsNaN(sec) || math.IsInf(sec, 0) {
		return 0, errTimeoutNotFloat
	}
	if sec < 0 {
		return 0, errTimeoutNegative
	}
	if sec == 0 || sec > float64(blockForever/time.Second) {
		return blockForever, nil
	}
	return time.Duration(sec * float64(time.Second)), nil
}

func tryListMove(ctx *Context, srcKey, dstKey, whereFrom, whereTo string) ([]byte, bool, error) {
	srcList, err := getList(ctx, srcKey)
	if err != nil {
		return nil, false, err
	}
	if srcList == nil {
		return nil, false, nil
	}

	srcList.RLock()
	sourceEmpty := len(srcList.Elements) == 0
	srcList.RUnlock()
	if sourceEmpty {
		return nil, false, nil
	}

	dstList, err := getOrCreateList(ctx, dstKey)
	if err != nil {
		return nil, false, err
	}

	srcList.Lock()
	if len(srcList.Elements) == 0 {
		srcList.Unlock()
		return nil, false, nil
	}

	var value []byte
	switch whereFrom {
	case "LEFT":
		value = srcList.Elements[0]
		srcList.Elements = srcList.Elements[1:]
	case "RIGHT":
		value = srcList.Elements[len(srcList.Elements)-1]
		srcList.Elements = srcList.Elements[:len(srcList.Elements)-1]
	}
	srcEmpty := len(srcList.Elements) == 0
	srcList.Unlock()

	if srcEmpty && srcKey != dstKey {
		ctx.Store.Delete(srcKey)
	}

	dstList.Lock()
	switch whereTo {
	case "LEFT":
		newEl := make([][]byte, 1+len(dstList.Elements))
		newEl[0] = value
		copy(newEl[1:], dstList.Elements)
		dstList.Elements = newEl
	case "RIGHT":
		dstList.Elements = append(dstList.Elements, value)
	}
	dstList.Unlock()

	ctx.Store.KeyNotifier().NotifyKey(dstKey)
	return value, true, nil
}

func cmdBLPOP(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	keys := make([]string, 0, ctx.ArgCount()-1)
	for i := 0; i < ctx.ArgCount()-1; i++ {
		keys = append(keys, ctx.ArgString(i))
	}
	timeout, err := parseBlockTimeout(ctx.ArgString(ctx.ArgCount() - 1))
	if err != nil {
		return ctx.WriteError(err)
	}

	// Try immediate pop first
	if key, value, ok := tryLPop(ctx, keys); ok {
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString(key),
			resp.BulkBytes(value),
		})
	}

	// Block until notified or timeout
	deadline := timeout

	for {
		notifiedKey, notified := ctx.waitForKeys(keys, deadline)
		if !notified {
			return ctx.WriteNullArray()
		}

		// A key was notified — try to pop from it
		list, err := getList(ctx, notifiedKey)
		if err != nil {
			return ctx.WriteError(err)
		}
		if list != nil && len(list.Elements) > 0 {
			list.Lock()
			if len(list.Elements) > 0 {
				value := list.Elements[0]
				list.Elements = list.Elements[1:]
				isEmpty := len(list.Elements) == 0
				list.Unlock()
				if isEmpty {
					ctx.Store.Delete(notifiedKey)
				}
				return ctx.WriteArray([]*resp.Value{
					resp.BulkString(notifiedKey),
					resp.BulkBytes(value),
				})
			}
			list.Unlock()
		}
		// Someone else grabbed it — retry
	}
}

func tryLPop(ctx *Context, keys []string) (string, []byte, bool) {
	for _, key := range keys {
		list, err := getList(ctx, key)
		if err != nil || list == nil || len(list.Elements) == 0 {
			continue
		}
		list.Lock()
		if len(list.Elements) > 0 {
			value := list.Elements[0]
			list.Elements = list.Elements[1:]
			isEmpty := len(list.Elements) == 0
			list.Unlock()
			if isEmpty {
				ctx.Store.Delete(key)
			}
			return key, value, true
		}
		list.Unlock()
	}
	return "", nil, false
}

func cmdBRPOP(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	keys := make([]string, 0, ctx.ArgCount()-1)
	for i := 0; i < ctx.ArgCount()-1; i++ {
		keys = append(keys, ctx.ArgString(i))
	}
	timeout, err := parseBlockTimeout(ctx.ArgString(ctx.ArgCount() - 1))
	if err != nil {
		return ctx.WriteError(err)
	}

	// Try immediate pop first
	if key, value, ok := tryRPop(ctx, keys); ok {
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString(key),
			resp.BulkBytes(value),
		})
	}

	deadline := timeout

	for {
		notifiedKey, notified := ctx.waitForKeys(keys, deadline)
		if !notified {
			return ctx.WriteNullArray()
		}

		list, err := getList(ctx, notifiedKey)
		if err != nil {
			return ctx.WriteError(err)
		}
		if list != nil && len(list.Elements) > 0 {
			list.Lock()
			if len(list.Elements) > 0 {
				idx := len(list.Elements) - 1
				value := list.Elements[idx]
				list.Elements = list.Elements[:idx]
				isEmpty := len(list.Elements) == 0
				list.Unlock()
				if isEmpty {
					ctx.Store.Delete(notifiedKey)
				}
				return ctx.WriteArray([]*resp.Value{
					resp.BulkString(notifiedKey),
					resp.BulkBytes(value),
				})
			}
			list.Unlock()
		}
	}
}

func tryRPop(ctx *Context, keys []string) (string, []byte, bool) {
	for _, key := range keys {
		list, err := getList(ctx, key)
		if err != nil || list == nil || len(list.Elements) == 0 {
			continue
		}
		list.Lock()
		if len(list.Elements) > 0 {
			idx := len(list.Elements) - 1
			value := list.Elements[idx]
			list.Elements = list.Elements[:idx]
			isEmpty := len(list.Elements) == 0
			list.Unlock()
			if isEmpty {
				ctx.Store.Delete(key)
			}
			return key, value, true
		}
		list.Unlock()
	}
	return "", nil, false
}

func cmdBRPOPLPUSH(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	srcKey := ctx.ArgString(0)
	dstKey := ctx.ArgString(1)
	timeout, err := parseBlockTimeout(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}

	// BRPOPLPUSH is RPOP src + LPUSH dst
	if value, ok, err := tryListMove(ctx, srcKey, dstKey, "RIGHT", "LEFT"); err != nil {
		return ctx.WriteError(err)
	} else if ok {
		return ctx.WriteBulkBytes(value)
	}

	dur := timeout

	for {
		if !ctx.waitForKey(srcKey, dur) {
			return ctx.WriteNull()
		}
		if value, ok, err := tryListMove(ctx, srcKey, dstKey, "RIGHT", "LEFT"); err != nil {
			return ctx.WriteError(err)
		} else if ok {
			return ctx.WriteBulkBytes(value)
		}
	}
}

func cmdLPOS(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	element := ctx.Arg(1)

	rank := 1
	count := 0
	countSpecified := false
	maxlen := 0

	for i := 2; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "RANK":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			rank, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			if rank == 0 {
				return ctx.WriteError(errLPosRankZero)
			}
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
			countSpecified = true
		case "MAXLEN":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			maxlen, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
		}
	}

	list, err := getList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if list == nil {
		return ctx.WriteNull()
	}

	list.RLock()

	absRank := rank
	if rank < 0 {
		absRank = -rank
	}

	found := 0
	searchLen := len(list.Elements)
	if maxlen > 0 && maxlen < searchLen {
		searchLen = maxlen
	}

	if rank > 0 {
		for i := 0; i < searchLen && found < absRank; i++ {
			if bytes.Equal(list.Elements[i], element) {
				found++
				if found == absRank {
					if countSpecified {
						result := make([]*resp.Value, 0)
						result = append(result, resp.IntegerValue(int64(i)))
						for j := i + 1; j < searchLen && (count == 0 || len(result) < count); j++ {
							if bytes.Equal(list.Elements[j], element) {
								result = append(result, resp.IntegerValue(int64(j)))
							}
						}
						reply := result
						list.RUnlock()
						return ctx.WriteArray(reply)
					}
					reply := int64(int64(i))
					list.RUnlock()
					return ctx.WriteInteger(reply)
				}
			}
		}
	} else {
		// MAXLEN caps comparisons in the search direction: a tail search
		// compares the LAST maxlen elements, not the first.
		tailStart := 0
		if maxlen > 0 && maxlen < len(list.Elements) {
			tailStart = len(list.Elements) - maxlen
		}
		for i := len(list.Elements) - 1; i >= tailStart && found < absRank; i-- {
			if bytes.Equal(list.Elements[i], element) {
				found++
				if found == absRank {
					if countSpecified {
						result := make([]*resp.Value, 0)
						result = append(result, resp.IntegerValue(int64(i)))
						for j := i - 1; j >= tailStart && (count == 0 || len(result) < count); j-- {
							if bytes.Equal(list.Elements[j], element) {
								result = append(result, resp.IntegerValue(int64(j)))
							}
						}
						reply := result
						list.RUnlock()
						return ctx.WriteArray(reply)
					}
					reply := int64(int64(i))
					list.RUnlock()
					return ctx.WriteInteger(reply)
				}
			}
		}
	}

	if count > 0 {
		reply := []*resp.Value{}
		list.RUnlock()
		return ctx.WriteArray(reply)
	}
	list.RUnlock()
	return ctx.WriteNull()
}

func cmdLMPOP(ctx *Context) error {
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
// tail shared by LMPOP and BLMPOP, starting at argument index from.
func parseListMPop(ctx *Context, from int) ([]string, string, int, error) {
	if ctx.ArgCount() < from+3 {
		return nil, "", 0, ErrWrongArgCount
	}
	numKeys, err := strconv.Atoi(ctx.ArgString(from))
	if err != nil {
		return nil, "", 0, ErrNotInteger
	}
	// numkeys is a key COUNT and must be at least 1; a negative value would
	// otherwise reach make() as a negative length.
	if numKeys < 1 {
		return nil, "", 0, ErrInvalidArg
	}
	if ctx.ArgCount() < from+1+numKeys+1 {
		return nil, "", 0, ErrWrongArgCount
	}

	keys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = ctx.ArgString(from + 1 + i)
	}

	i := from + 1 + numKeys
	dir := strings.ToUpper(ctx.ArgString(i))
	if dir != "LEFT" && dir != "RIGHT" {
		return nil, "", 0, ErrSyntaxError
	}
	i++

	count := 1
	if i < ctx.ArgCount() {
		if strings.ToUpper(ctx.ArgString(i)) != "COUNT" || i+2 != ctx.ArgCount() {
			return nil, "", 0, ErrSyntaxError
		}
		count, err = strconv.Atoi(ctx.ArgString(i + 1))
		if err != nil {
			return nil, "", 0, ErrNotInteger
		}
	}

	// A pop COUNT is a hard element count and must be at least 1. Without
	// this guard it reaches make() as a slice capacity: a negative count
	// panics, and 0 silently pops nothing. ZMPOP/BZMPOP already enforce it.
	if count < 1 {
		return nil, "", 0, ErrInvalidArg
	}
	return keys, dir, count, nil
}

// tryListMPop pops up to count elements from the first non-empty list among
// keys. It returns nil when every list is empty.
func tryListMPop(ctx *Context, keys []string, dir string, count int) (*resp.Value, error) {
	for _, key := range keys {
		list, err := getList(ctx, key)
		if err != nil {
			return nil, err
		}
		if list == nil {
			continue
		}
		list.Lock()
		if len(list.Elements) == 0 {
			list.Unlock()
			continue
		}
		elements := make([]*resp.Value, 0, min(count, len(list.Elements)))
		for i := 0; i < count && len(list.Elements) > 0; i++ {
			var value []byte
			if dir == "LEFT" {
				value = list.Elements[0]
				list.Elements = list.Elements[1:]
			} else {
				value = list.Elements[len(list.Elements)-1]
				list.Elements = list.Elements[:len(list.Elements)-1]
			}
			elements = append(elements, resp.BulkBytes(value))
		}
		isEmpty := len(list.Elements) == 0
		list.Unlock()
		if isEmpty {
			ctx.Store.Delete(key)
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
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	elements := make([][]byte, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		elements = append(elements, ctx.Arg(i))
	}

	list, err := getOrCreateList(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}

	newElements := make([][]byte, len(elements)+len(list.Elements))
	copy(newElements, elements)
	copy(newElements[len(elements):], list.Elements)
	list.Elements = newElements
	return ctx.WriteInteger(int64(len(list.Elements)))
}

func cmdBLMPOP(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	timeout, err := parseBlockTimeout(ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	keys, dir, count, err := parseListMPop(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}

	for {
		reply, err := tryListMPop(ctx, keys, dir, count)
		if err != nil {
			return ctx.WriteError(err)
		}
		if reply != nil {
			return ctx.WriteValue(reply)
		}
		if _, notified := ctx.waitForKeys(keys, timeout); !notified {
			return ctx.WriteNullArray()
		}
	}
}
