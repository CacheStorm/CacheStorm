package command

import (
	"strconv"

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
