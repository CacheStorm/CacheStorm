package command

import (
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterStreamCommands(router *Router) {
	router.Register(&CommandDef{Name: "XADD", Handler: cmdXADD})
	router.Register(&CommandDef{Name: "XLEN", Handler: cmdXLEN})
	router.Register(&CommandDef{Name: "XRANGE", Handler: cmdXRANGE})
	router.Register(&CommandDef{Name: "XREVRANGE", Handler: cmdXREVRANGE})
	router.Register(&CommandDef{Name: "XREAD", Handler: cmdXREAD})
	router.Register(&CommandDef{Name: "XDEL", Handler: cmdXDEL})
	router.Register(&CommandDef{Name: "XTRIM", Handler: cmdXTRIM})
	router.Register(&CommandDef{Name: "XINFO", Handler: cmdXINFO})
	router.Register(&CommandDef{Name: "XGROUP", Handler: cmdXGROUP})
	router.Register(&CommandDef{Name: "XREADGROUP", Handler: cmdXREADGROUP})
	router.Register(&CommandDef{Name: "XACK", Handler: cmdXACK})
	router.Register(&CommandDef{Name: "XPENDING", Handler: cmdXPENDING})
	router.Register(&CommandDef{Name: "XCLAIM", Handler: cmdXCLAIM})
}

func getOrCreateStream(ctx *Context, key string, maxLen int64) *store.StreamValue {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		stream := store.NewStreamValue(maxLen)
		ctx.Store.Set(key, stream, store.SetOptions{})
		return stream
	}

	if stream, ok := entry.Value.(*store.StreamValue); ok {
		return stream
	}
	return nil
}

func getStream(ctx *Context, key string) *store.StreamValue {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return nil
	}

	if stream, ok := entry.Value.(*store.StreamValue); ok {
		return stream
	}
	return nil
}

func generateStreamID(lastID string) string {
	now := time.Now().UnixMilli()

	if lastID == "" || lastID == "0-0" {
		return strconv.FormatInt(now, 10) + "-0"
	}

	parts := strings.Split(lastID, "-")
	if len(parts) != 2 {
		return strconv.FormatInt(now, 10) + "-0"
	}

	ms, err1 := strconv.ParseInt(parts[0], 10, 64)
	seq, err2 := strconv.ParseInt(parts[1], 10, 64)

	if err1 != nil || err2 != nil {
		return strconv.FormatInt(now, 10) + "-0"
	}

	if ms == now {
		return strconv.FormatInt(now, 10) + "-" + strconv.FormatInt(seq+1, 10)
	}

	if now > ms {
		return strconv.FormatInt(now, 10) + "-0"
	}

	return strconv.FormatInt(ms, 10) + "-" + strconv.FormatInt(seq+1, 10)
}

func cmdXADD(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	maxLen := int64(0)
	approximate := false
	argIdx := 1

	for argIdx < ctx.ArgCount() {
		arg := strings.ToUpper(ctx.ArgString(argIdx))
		if arg == "MAXLEN" {
			argIdx++
			if argIdx < ctx.ArgCount() && strings.ToUpper(ctx.ArgString(argIdx)) == "~" {
				approximate = true
				argIdx++
			}
			if argIdx < ctx.ArgCount() {
				var err error
				maxLen, err = strconv.ParseInt(ctx.ArgString(argIdx), 10, 64)
				if err != nil {
					return ctx.WriteError(ErrNotInteger)
				}
				argIdx++
			}
		} else {
			break
		}
	}

	if argIdx >= ctx.ArgCount() {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(argIdx)
	argIdx++

	if (argIdx-ctx.ArgCount())%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	fields := make(map[string][]byte)
	for i := argIdx; i < ctx.ArgCount(); i += 2 {
		fields[ctx.ArgString(i)] = ctx.Arg(i + 1)
	}

	stream := getOrCreateStream(ctx, key, maxLen)
	if stream == nil {
		return ctx.WriteError(store.ErrWrongType)
	}

	_ = approximate

	if id == "*" {
		id = generateStreamID(stream.LastID)
	}

	entry, err := stream.Add(id, fields)
	if err != nil {
		return ctx.WriteError(err)
	}

	_ = entry
	return ctx.WriteBulkString(id)
}

func cmdXLEN(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	stream := getStream(ctx, key)
	if stream == nil {
		return ctx.WriteInteger(0)
	}

	return ctx.WriteInteger(stream.Len())
}

func cmdXRANGE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	start := ctx.ArgString(1)
	end := ctx.ArgString(2)
	count := int64(0)

	for i := 3; i < ctx.ArgCount(); i++ {
		if strings.ToUpper(ctx.ArgString(i)) == "COUNT" && i+1 < ctx.ArgCount() {
			var err error
			count, err = strconv.ParseInt(ctx.ArgString(i+1), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			i++
		}
	}

	stream := getStream(ctx, key)
	if stream == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	if start == "-" {
		start = "0-0"
	}
	if end == "+" {
		end = "9999999999999-9999999999999"
	}

	entries := stream.GetRange(start, end, count)

	results := make([]*resp.Value, 0, len(entries))
	for _, entry := range entries {
		fields := make([]*resp.Value, 0, len(entry.Fields)*2)
		for k, v := range entry.Fields {
			fields = append(fields, resp.BulkString(k), resp.BulkBytes(v))
		}
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString(entry.ID),
			resp.ArrayValue(fields),
		}))
	}

	return ctx.WriteArray(results)
}

func cmdXREVRANGE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	end := ctx.ArgString(1)
	start := ctx.ArgString(2)

	_ = end
	_ = start

	stream := getStream(ctx, key)
	if stream == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	entries := stream.GetRange("0-0", "+", 0)

	results := make([]*resp.Value, 0, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		fields := make([]*resp.Value, 0, len(entry.Fields)*2)
		for k, v := range entry.Fields {
			fields = append(fields, resp.BulkString(k), resp.BulkBytes(v))
		}
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString(entry.ID),
			resp.ArrayValue(fields),
		}))
	}

	return ctx.WriteArray(results)
}

func cmdXREAD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	count := int64(0)
	block := int64(0)
	streamsIdx := 1

	for i := 1; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "COUNT":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			count, err = strconv.ParseInt(ctx.ArgString(i), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
		case "BLOCK":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			block, err = strconv.ParseInt(ctx.ArgString(i), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
		case "STREAMS":
			streamsIdx = i + 1
			i = ctx.ArgCount()
		}
	}

	_ = block

	remaining := ctx.ArgCount() - streamsIdx
	if remaining < 2 || remaining%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	numStreams := remaining / 2
	results := make([]*resp.Value, 0, numStreams)

	for i := 0; i < numStreams; i++ {
		key := ctx.ArgString(streamsIdx + i)
		id := ctx.ArgString(streamsIdx + numStreams + i)

		stream := getStream(ctx, key)
		if stream == nil {
			continue
		}

		if id == "$" {
			id = stream.LastID
		}

		entries := stream.GetRange(id, "+", count)
		if len(entries) == 0 {
			continue
		}

		entryResults := make([]*resp.Value, 0, len(entries))
		for _, entry := range entries {
			if entry.ID == id {
				continue
			}
			fields := make([]*resp.Value, 0, len(entry.Fields)*2)
			for k, v := range entry.Fields {
				fields = append(fields, resp.BulkString(k), resp.BulkBytes(v))
			}
			entryResults = append(entryResults, resp.ArrayValue([]*resp.Value{
				resp.BulkString(entry.ID),
				resp.ArrayValue(fields),
			}))
		}

		if len(entryResults) > 0 {
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.BulkString(key),
				resp.ArrayValue(entryResults),
			}))
		}
	}

	if len(results) == 0 {
		return ctx.WriteNull()
	}

	return ctx.WriteArray(results)
}

func cmdXDEL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	stream := getStream(ctx, key)
	if stream == nil {
		return ctx.WriteInteger(0)
	}

	ids := make([]string, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		ids = append(ids, ctx.ArgString(i))
	}

	deleted := stream.Delete(ids...)
	return ctx.WriteInteger(deleted)
}

func cmdXTRIM(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	if strings.ToUpper(ctx.ArgString(1)) != "MAXLEN" {
		return ctx.WriteError(ErrSyntaxError)
	}

	approximate := false
	idx := 2

	if ctx.ArgCount() > idx && strings.ToUpper(ctx.ArgString(idx)) == "~" {
		approximate = true
		idx++
	}

	if idx >= ctx.ArgCount() {
		return ctx.WriteError(ErrWrongArgCount)
	}

	maxLen, err := strconv.ParseInt(ctx.ArgString(idx), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	stream := getStream(ctx, key)
	if stream == nil {
		return ctx.WriteInteger(0)
	}

	_ = approximate
	removed := stream.Trim(maxLen, approximate)
	return ctx.WriteInteger(removed)
}

func cmdXINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "STREAM":
		if ctx.ArgCount() < 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		key := ctx.ArgString(1)
		stream := getStream(ctx, key)
		if stream == nil {
			return ctx.WriteError(store.ErrKeyNotFound)
		}

		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("length"), resp.IntegerValue(stream.Len()),
			resp.BulkString("radix-tree-keys"), resp.IntegerValue(stream.Len()),
			resp.BulkString("radix-tree-nodes"), resp.IntegerValue(stream.Len() + 1),
			resp.BulkString("last-generated-id"), resp.BulkString(stream.LastID),
		})

	case "GROUPS":
		return ctx.WriteArray([]*resp.Value{})

	case "CONSUMERS":
		return ctx.WriteArray([]*resp.Value{})

	case "HELP":
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("XINFO STREAM <key>"),
			resp.BulkString("XINFO GROUPS <key>"),
			resp.BulkString("XINFO CONSUMERS <key> <group>"),
		})

	default:
		return ctx.WriteError(ErrUnknownCommand)
	}
}

func cmdXGROUP(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "CREATE":
		return ctx.WriteOK()
	case "DESTROY":
		return ctx.WriteInteger(0)
	case "SETID":
		return ctx.WriteOK()
	case "DELCONSUMER":
		return ctx.WriteInteger(0)
	default:
		return ctx.WriteError(ErrUnknownCommand)
	}
}

func cmdXREADGROUP(ctx *Context) error {
	return ctx.WriteNull()
}

func cmdXACK(ctx *Context) error {
	return ctx.WriteInteger(0)
}

func cmdXPENDING(ctx *Context) error {
	return ctx.WriteArray([]*resp.Value{})
}

func cmdXCLAIM(ctx *Context) error {
	return ctx.WriteArray([]*resp.Value{})
}
