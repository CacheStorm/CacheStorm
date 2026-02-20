package command

import (
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

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
	router.Register(&CommandDef{Name: "BLPOP", Handler: cmdBLPOP})
	router.Register(&CommandDef{Name: "BRPOP", Handler: cmdBRPOP})
	router.Register(&CommandDef{Name: "BRPOPLPUSH", Handler: cmdBRPOPLPUSH})
	router.Register(&CommandDef{Name: "LPOS", Handler: cmdLPOS})
	router.Register(&CommandDef{Name: "LMPOP", Handler: cmdLMPOP})
	router.Register(&CommandDef{Name: "LMPUSH", Handler: cmdLMPUSH})
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

	for i := 1; i < ctx.ArgCount(); i++ {
		value := ctx.Arg(i)
		list.Elements = append([][]byte{value}, list.Elements...)
	}

	return ctx.WriteInteger(int64(len(list.Elements)))
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

	for i := 1; i < ctx.ArgCount(); i++ {
		value := ctx.Arg(i)
		list.Elements = append(list.Elements, value)
	}

	return ctx.WriteInteger(int64(len(list.Elements)))
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

	for i := 1; i < ctx.ArgCount(); i++ {
		value := ctx.Arg(i)
		list.Elements = append([][]byte{value}, list.Elements...)
	}

	return ctx.WriteInteger(int64(len(list.Elements)))
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

	for i := 1; i < ctx.ArgCount(); i++ {
		value := ctx.Arg(i)
		list.Elements = append(list.Elements, value)
	}

	return ctx.WriteInteger(int64(len(list.Elements)))
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
	if list == nil || len(list.Elements) == 0 {
		return ctx.WriteNullBulkString()
	}

	value := list.Elements[0]
	list.Elements = list.Elements[1:]

	if len(list.Elements) == 0 {
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
	if list == nil || len(list.Elements) == 0 {
		return ctx.WriteNullBulkString()
	}

	idx := len(list.Elements) - 1
	value := list.Elements[idx]
	list.Elements = list.Elements[:idx]

	if len(list.Elements) == 0 {
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

	return ctx.WriteInteger(int64(len(list.Elements)))
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

	length := len(list.Elements)
	if length == 0 {
		return ctx.WriteArray([]*resp.Value{})
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
		return ctx.WriteArray([]*resp.Value{})
	}

	results := make([]*resp.Value, 0, stop-start+1)
	for i := start; i <= stop; i++ {
		results = append(results, resp.BulkBytes(list.Elements[i]))
	}

	return ctx.WriteArray(results)
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

	length := len(list.Elements)
	if index < 0 {
		index = length + index
	}
	if index < 0 || index >= length {
		return ctx.WriteNullBulkString()
	}

	return ctx.WriteBulkBytes(list.Elements[index])
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

	length := len(list.Elements)
	if index < 0 {
		index = length + index
	}
	if index < 0 || index >= length {
		return ctx.WriteError(ErrIndexOutOfRange)
	}

	list.Elements[index] = value
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

	removed := 0
	newElements := make([][]byte, 0, len(list.Elements))

	if count == 0 {
		for _, elem := range list.Elements {
			if string(elem) == string(value) {
				removed++
			} else {
				newElements = append(newElements, elem)
			}
		}
	} else if count > 0 {
		for _, elem := range list.Elements {
			if removed < count && string(elem) == string(value) {
				removed++
			} else {
				newElements = append(newElements, elem)
			}
		}
	} else {
		for i := len(list.Elements) - 1; i >= 0; i-- {
			elem := list.Elements[i]
			if removed < -count && string(elem) == string(value) {
				removed++
			} else {
				newElements = append([][]byte{elem}, newElements...)
			}
		}
	}

	list.Elements = newElements

	if len(list.Elements) == 0 {
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

	pivotIdx := -1
	for i, elem := range list.Elements {
		if string(elem) == string(pivot) {
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

	length := len(list.Elements)
	if length == 0 {
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
		ctx.Store.Delete(key)
		return ctx.WriteOK()
	}

	list.Elements = list.Elements[start : stop+1]
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

	idx := len(srcList.Elements) - 1
	value := srcList.Elements[idx]
	srcList.Elements = srcList.Elements[:idx]

	if len(srcList.Elements) == 0 {
		ctx.Store.Delete(srcKey)
	}

	dstList, err := getOrCreateList(ctx, dstKey)
	if err != nil {
		return ctx.WriteError(err)
	}

	dstList.Elements = append([][]byte{value}, dstList.Elements...)

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

	srcList, err := getList(ctx, srcKey)
	if err != nil {
		return ctx.WriteError(err)
	}
	if srcList == nil || len(srcList.Elements) == 0 {
		return ctx.WriteNullBulkString()
	}

	var value []byte
	switch whereFrom {
	case "LEFT":
		value = srcList.Elements[0]
		srcList.Elements = srcList.Elements[1:]
	case "RIGHT":
		value = srcList.Elements[len(srcList.Elements)-1]
		srcList.Elements = srcList.Elements[:len(srcList.Elements)-1]
	default:
		return ctx.WriteError(ErrSyntaxError)
	}

	if len(srcList.Elements) == 0 {
		ctx.Store.Delete(srcKey)
	}

	dstList, err := getOrCreateList(ctx, dstKey)
	if err != nil {
		return ctx.WriteError(err)
	}

	switch whereTo {
	case "LEFT":
		dstList.Elements = append([][]byte{value}, dstList.Elements...)
	case "RIGHT":
		dstList.Elements = append(dstList.Elements, value)
	default:
		return ctx.WriteError(ErrSyntaxError)
	}

	return ctx.WriteBulkBytes(value)
}

func cmdBLPOP(ctx *Context) error {
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
			list, err := getList(ctx, key)
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
						ctx.Store.Delete(key)
					}
					return ctx.WriteArray([]*resp.Value{
						resp.BulkString(key),
						resp.BulkBytes(value),
					})
				}
				list.Unlock()
			}
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

func cmdBRPOP(ctx *Context) error {
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
			list, err := getList(ctx, key)
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
						ctx.Store.Delete(key)
					}
					return ctx.WriteArray([]*resp.Value{
						resp.BulkString(key),
						resp.BulkBytes(value),
					})
				}
				list.Unlock()
			}
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

func cmdBRPOPLPUSH(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	srcKey := ctx.ArgString(0)
	dstKey := ctx.ArgString(1)
	timeout, err := strconv.Atoi(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	_ = timeout

	srcList, err := getList(ctx, srcKey)
	if err != nil {
		return ctx.WriteError(err)
	}
	if srcList == nil || len(srcList.Elements) == 0 {
		return ctx.WriteNullBulkString()
	}

	idx := len(srcList.Elements) - 1
	value := srcList.Elements[idx]
	srcList.Elements = srcList.Elements[:idx]

	if len(srcList.Elements) == 0 {
		ctx.Store.Delete(srcKey)
	}

	dstList, err := getOrCreateList(ctx, dstKey)
	if err != nil {
		return ctx.WriteError(err)
	}

	dstList.Elements = append([][]byte{value}, dstList.Elements...)

	return ctx.WriteBulkBytes(value)
}

func cmdLPOS(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	element := ctx.Arg(1)

	rank := 1
	count := 0
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
			if string(list.Elements[i]) == string(element) {
				found++
				if found == absRank {
					if count > 0 {
						result := make([]*resp.Value, 0)
						result = append(result, resp.IntegerValue(int64(i)))
						for j := i + 1; j < searchLen && len(result) < count; j++ {
							if string(list.Elements[j]) == string(element) {
								result = append(result, resp.IntegerValue(int64(j)))
							}
						}
						return ctx.WriteArray(result)
					}
					return ctx.WriteInteger(int64(i))
				}
			}
		}
	} else {
		for i := searchLen - 1; i >= 0 && found < absRank; i-- {
			if string(list.Elements[i]) == string(element) {
				found++
				if found == absRank {
					if count > 0 {
						result := make([]*resp.Value, 0)
						result = append(result, resp.IntegerValue(int64(i)))
						for j := i - 1; j >= 0 && len(result) < count; j-- {
							if string(list.Elements[j]) == string(element) {
								result = append(result, resp.IntegerValue(int64(j)))
							}
						}
						return ctx.WriteArray(result)
					}
					return ctx.WriteInteger(int64(i))
				}
			}
		}
	}

	if count > 0 {
		return ctx.WriteArray([]*resp.Value{})
	}
	return ctx.WriteNull()
}

func cmdLMPOP(ctx *Context) error {
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

	keys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = ctx.ArgString(1 + i)
	}

	dir := "LEFT"
	count := 1

	for i := 1 + numKeys; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "LEFT":
			dir = "LEFT"
		case "RIGHT":
			dir = "RIGHT"
		case "COUNT":
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

	for _, key := range keys {
		list, err := getList(ctx, key)
		if err != nil {
			return ctx.WriteError(err)
		}
		if list != nil && len(list.Elements) > 0 {
			elements := make([]*resp.Value, 0, count)
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
			if len(list.Elements) == 0 {
				ctx.Store.Delete(key)
			}
			return ctx.WriteArray([]*resp.Value{
				resp.BulkString(key),
				resp.ArrayValue(elements),
			})
		}
	}

	return ctx.WriteNull()
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

	list.Elements = append(elements, list.Elements...)
	return ctx.WriteInteger(int64(len(list.Elements)))
}
