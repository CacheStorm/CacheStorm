package command

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

var (
	ErrBusyGroup = errors.New("BUSYGROUP Consumer Group name already exists")
	ErrNoGroup   = errors.New("NOGROUP No such key")
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
	router.Register(&CommandDef{Name: "XAUTOCLAIM", Handler: cmdXAUTOCLAIM})
	router.Register(&CommandDef{Name: "XSETID", Handler: cmdXSETID})
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
	minID := ""
	approximate := false
	trimStrategy := ""
	argIdx := 1

loop:
	for argIdx < ctx.ArgCount() {
		arg := strings.ToUpper(ctx.ArgString(argIdx))
		switch arg {
		case "MAXLEN":
			trimStrategy = "MAXLEN"
			argIdx++
			if argIdx < ctx.ArgCount() && strings.ToUpper(ctx.ArgString(argIdx)) == "~" {
				approximate = true
				argIdx++
			}
			if argIdx < ctx.ArgCount() && strings.ToUpper(ctx.ArgString(argIdx)) == "=" {
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
		case "MINID":
			trimStrategy = "MINID"
			argIdx++
			if argIdx < ctx.ArgCount() && strings.ToUpper(ctx.ArgString(argIdx)) == "~" {
				approximate = true
				argIdx++
			}
			if argIdx < ctx.ArgCount() && strings.ToUpper(ctx.ArgString(argIdx)) == "=" {
				argIdx++
			}
			if argIdx < ctx.ArgCount() {
				minID = ctx.ArgString(argIdx)
				argIdx++
			}
		case "NOMKSTREAM":
			argIdx++
		case "LIMIT":
			argIdx += 2
		default:
			break loop
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

	if id == "*" {
		id = generateStreamID(stream.LastID)
	}

	entry, err := stream.Add(id, fields)
	if err != nil {
		return ctx.WriteError(err)
	}

	if trimStrategy == "MINID" && minID != "" {
		stream.TrimByMinID(minID, approximate)
	} else if trimStrategy == "MAXLEN" && maxLen > 0 {
		stream.Trim(maxLen, approximate)
	}

	_ = entry
	_ = approximate
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

	count := int64(0)
	for i := 3; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		if arg == "COUNT" {
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			count, err = strconv.ParseInt(ctx.ArgString(i), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
		}
	}

	stream := getStream(ctx, key)
	if stream == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	entries := stream.GetRange(start, end, count)

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

		full := false
		for i := 2; i < ctx.ArgCount(); i++ {
			if strings.ToUpper(ctx.ArgString(i)) == "FULL" {
				full = true
			}
		}

		if full {
			entries := stream.GetRange("-", "+", 0)
			entryResults := make([]*resp.Value, 0, len(entries))
			for _, e := range entries {
				fieldValues := make([]*resp.Value, 0)
				for k, v := range e.Fields {
					fieldValues = append(fieldValues, resp.BulkString(k), resp.BulkBytes(v))
				}
				entryResults = append(entryResults, resp.ArrayValue([]*resp.Value{
					resp.BulkString(e.ID),
					resp.ArrayValue(fieldValues),
				}))
			}

			groupResults := make([]*resp.Value, 0)
			for name, group := range stream.Groups {
				consumerResults := make([]*resp.Value, 0)
				for cname, c := range group.Consumers {
					consumerResults = append(consumerResults, resp.ArrayValue([]*resp.Value{
						resp.BulkString("name"), resp.BulkString(cname),
						resp.BulkString("seen-time"), resp.IntegerValue(c.SeenTime),
						resp.BulkString("pel-count"), resp.IntegerValue(c.Pending),
					}))
				}
				groupResults = append(groupResults, resp.ArrayValue([]*resp.Value{
					resp.BulkString("name"), resp.BulkString(name),
					resp.BulkString("last-delivered-id"), resp.BulkString(group.LastID),
					resp.BulkString("pel-count"), resp.IntegerValue(int64(len(group.Pending))),
					resp.BulkString("consumers"), resp.ArrayValue(consumerResults),
				}))
			}

			return ctx.WriteArray([]*resp.Value{
				resp.BulkString("length"), resp.IntegerValue(stream.Len()),
				resp.BulkString("entries"), resp.ArrayValue(entryResults),
				resp.BulkString("groups"), resp.ArrayValue(groupResults),
			})
		}

		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("length"), resp.IntegerValue(stream.Len()),
			resp.BulkString("radix-tree-keys"), resp.IntegerValue(stream.Len()),
			resp.BulkString("radix-tree-nodes"), resp.IntegerValue(stream.Len() + 1),
			resp.BulkString("last-generated-id"), resp.BulkString(stream.LastID),
			resp.BulkString("groups"), resp.IntegerValue(int64(len(stream.Groups))),
		})

	case "GROUPS":
		if ctx.ArgCount() < 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		key := ctx.ArgString(1)
		stream := getStream(ctx, key)
		if stream == nil {
			return ctx.WriteArray([]*resp.Value{})
		}

		results := make([]*resp.Value, 0)
		for name, group := range stream.Groups {
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.BulkString("name"), resp.BulkString(name),
				resp.BulkString("consumers"), resp.IntegerValue(int64(len(group.Consumers))),
				resp.BulkString("pending"), resp.IntegerValue(int64(len(group.Pending))),
				resp.BulkString("last-delivered-id"), resp.BulkString(group.LastID),
			}))
		}
		return ctx.WriteArray(results)

	case "CONSUMERS":
		if ctx.ArgCount() < 3 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		key := ctx.ArgString(1)
		groupName := ctx.ArgString(2)

		stream := getStream(ctx, key)
		if stream == nil {
			return ctx.WriteArray([]*resp.Value{})
		}

		group := stream.GetGroup(groupName)
		if group == nil {
			return ctx.WriteArray([]*resp.Value{})
		}

		results := make([]*resp.Value, 0)
		for name, c := range group.Consumers {
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.BulkString("name"), resp.BulkString(name),
				resp.BulkString("pending"), resp.IntegerValue(c.Pending),
				resp.BulkString("idle"), resp.IntegerValue(time.Now().UnixMilli() - c.SeenTime),
			}))
		}
		return ctx.WriteArray(results)

	case "HELP":
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("XINFO STREAM <key> [FULL]"),
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
		if ctx.ArgCount() < 4 {
			return ctx.WriteError(ErrWrongArgCount)
		}

		key := ctx.ArgString(1)
		groupName := ctx.ArgString(2)
		lastID := ctx.ArgString(3)

		stream := getStream(ctx, key)
		if stream == nil {
			if ctx.ArgCount() > 4 && strings.ToUpper(ctx.ArgString(4)) == "MKSTREAM" {
				stream = getOrCreateStream(ctx, key, 0)
				if stream == nil {
					return ctx.WriteError(store.ErrWrongType)
				}
			} else {
				return ctx.WriteError(store.ErrKeyNotFound)
			}
		}

		err := stream.CreateGroup(groupName, lastID)
		if err != nil {
			return ctx.WriteError(ErrBusyGroup)
		}

		return ctx.WriteOK()

	case "DESTROY":
		if ctx.ArgCount() < 3 {
			return ctx.WriteError(ErrWrongArgCount)
		}

		key := ctx.ArgString(1)
		groupName := ctx.ArgString(2)

		stream := getStream(ctx, key)
		if stream == nil {
			return ctx.WriteInteger(0)
		}

		if stream.DestroyGroup(groupName) {
			return ctx.WriteInteger(1)
		}
		return ctx.WriteInteger(0)

	case "SETID":
		if ctx.ArgCount() < 4 {
			return ctx.WriteError(ErrWrongArgCount)
		}

		key := ctx.ArgString(1)
		groupName := ctx.ArgString(2)
		lastID := ctx.ArgString(3)

		stream := getStream(ctx, key)
		if stream == nil {
			return ctx.WriteError(store.ErrKeyNotFound)
		}

		if !stream.SetGroupLastID(groupName, lastID) {
			return ctx.WriteError(ErrNoGroup)
		}

		return ctx.WriteOK()

	case "DELCONSUMER":
		if ctx.ArgCount() < 4 {
			return ctx.WriteError(ErrWrongArgCount)
		}

		key := ctx.ArgString(1)
		groupName := ctx.ArgString(2)
		consumerName := ctx.ArgString(3)

		stream := getStream(ctx, key)
		if stream == nil {
			return ctx.WriteInteger(0)
		}

		group := stream.GetGroup(groupName)
		if group == nil {
			return ctx.WriteError(ErrNoGroup)
		}

		var pending int64
		if c, exists := group.Consumers[consumerName]; exists {
			pending = c.Pending
			delete(group.Consumers, consumerName)
		}

		return ctx.WriteInteger(pending)

	default:
		return ctx.WriteError(ErrUnknownCommand)
	}
}

func cmdXREADGROUP(ctx *Context) error {
	if ctx.ArgCount() < 6 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	var groupName, consumerName string
	var count int64 = 1
	var block int64 = 0
	var streamsIdx int
	var noack bool

	i := 0
	for i < ctx.ArgCount() {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "GROUP":
			if i+2 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			groupName = ctx.ArgString(i + 1)
			consumerName = ctx.ArgString(i + 2)
			i += 3
		case "COUNT":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			count, err = strconv.ParseInt(ctx.ArgString(i+1), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			i += 2
		case "BLOCK":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			block, err = strconv.ParseInt(ctx.ArgString(i+1), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			i += 2
		case "STREAMS":
			streamsIdx = i + 1
			i = ctx.ArgCount()
		case "NOACK":
			noack = true
			i++
		default:
			i++
		}
	}

	if groupName == "" || consumerName == "" || streamsIdx == 0 {
		return ctx.WriteError(ErrSyntaxError)
	}

	remaining := ctx.ArgCount() - streamsIdx
	if remaining < 2 || remaining%2 != 0 {
		return ctx.WriteError(ErrSyntaxError)
	}

	numStreams := remaining / 2
	keys := make([]string, numStreams)
	ids := make([]string, numStreams)

	for i := 0; i < numStreams; i++ {
		keys[i] = ctx.ArgString(streamsIdx + i)
		ids[i] = ctx.ArgString(streamsIdx + numStreams + i)
	}

	results := make([][]*resp.Value, numStreams)
	totalEntries := int64(0)

	for i, key := range keys {
		stream := getStream(ctx, key)
		if stream == nil {
			continue
		}

		group := stream.GetGroup(groupName)
		if group == nil {
			return ctx.WriteError(ErrNoGroup)
		}

		consumer := group.GetOrCreateConsumer(consumerName)
		consumer.SeenTime = time.Now().UnixMilli()

		var startID string
		if ids[i] == ">" {
			startID = group.LastID
			if startID == "0-0" {
				startID = "-"
			}
		} else {
			startID = ids[i]
		}

		entries := stream.GetRange(startID, "+", count)
		for _, entry := range entries {
			if ids[i] == ">" && entry.ID > group.LastID {
				group.AddPending(entry.ID, consumerName)
			}

			entryResult := []*resp.Value{
				resp.BulkString(entry.ID),
			}
			fieldValues := make([]*resp.Value, 0, len(entry.Fields)*2)
			for k, v := range entry.Fields {
				fieldValues = append(fieldValues, resp.BulkString(k), resp.BulkBytes(v))
			}
			entryResult = append(entryResult, resp.ArrayValue(fieldValues))
			results[i] = append(results[i], entryResult...)
			totalEntries++
		}
	}

	if totalEntries == 0 && block > 0 {
		deadline := time.Now().Add(time.Duration(block) * time.Second)
		for time.Now().Before(deadline) {
			for i, key := range keys {
				stream := getStream(ctx, key)
				if stream == nil {
					continue
				}

				group := stream.GetGroup(groupName)
				if group == nil {
					continue
				}

				startID := group.LastID
				if startID == "0-0" {
					startID = "-"
				}

				entries := stream.GetRange(startID, "+", count)
				for _, entry := range entries {
					if entry.ID > group.LastID {
						group.AddPending(entry.ID, consumerName)
						entryResult := []*resp.Value{resp.BulkString(entry.ID)}
						fieldValues := make([]*resp.Value, 0)
						for k, v := range entry.Fields {
							fieldValues = append(fieldValues, resp.BulkString(k), resp.BulkBytes(v))
						}
						entryResult = append(entryResult, resp.ArrayValue(fieldValues))
						results[i] = append(results[i], entryResult...)
						totalEntries++
					}
				}
			}

			if totalEntries > 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	if totalEntries == 0 {
		return ctx.WriteNull()
	}

	finalResults := make([]*resp.Value, numStreams)
	for i := range keys {
		if len(results[i]) > 0 {
			finalResults[i] = resp.ArrayValue([]*resp.Value{
				resp.BulkString(keys[i]),
				resp.ArrayValue(results[i]),
			})
		}
	}

	_ = noack

	return ctx.WriteArray(finalResults)
}

func cmdXACK(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	groupName := ctx.ArgString(1)
	entryIDs := make([]string, 0, ctx.ArgCount()-2)
	for i := 2; i < ctx.ArgCount(); i++ {
		entryIDs = append(entryIDs, ctx.ArgString(i))
	}

	stream := getStream(ctx, key)
	if stream == nil {
		return ctx.WriteInteger(0)
	}

	group := stream.GetGroup(groupName)
	if group == nil {
		return ctx.WriteError(ErrNoGroup)
	}

	acked := int64(0)
	for _, id := range entryIDs {
		if group.Ack(id) {
			acked++
		}
	}

	return ctx.WriteInteger(acked)
}

func cmdXPENDING(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	groupName := ctx.ArgString(1)

	stream := getStream(ctx, key)
	if stream == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	group := stream.GetGroup(groupName)
	if group == nil {
		return ctx.WriteError(ErrNoGroup)
	}

	start := "-"
	end := "+"
	var count int64 = 10
	var consumer string

	if ctx.ArgCount() > 2 {
		var err error
		start = ctx.ArgString(2)
		if ctx.ArgCount() > 3 {
			end = ctx.ArgString(3)
		}
		if ctx.ArgCount() > 4 {
			count, err = strconv.ParseInt(ctx.ArgString(4), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
		}
		if ctx.ArgCount() > 5 {
			consumer = ctx.ArgString(5)
		}

		pending := group.GetPending(start, end, count)
		results := make([]*resp.Value, 0, len(pending))
		for _, p := range pending {
			if consumer == "" || p.Consumer == consumer {
				results = append(results, resp.ArrayValue([]*resp.Value{
					resp.BulkString(p.ID),
					resp.BulkString(p.Consumer),
					resp.IntegerValue(p.DeliveryTS),
					resp.IntegerValue(p.Deliveries),
				}))
			}
		}
		return ctx.WriteArray(results)
	}

	pendingCount := group.GetPendingCount()
	firstID, lastID := group.GetFirstLastID()
	consumers := group.GetAllConsumers()

	if pendingCount == 0 {
		return ctx.WriteArray([]*resp.Value{
			resp.IntegerValue(0),
			resp.NullValue(),
			resp.NullValue(),
			resp.ArrayValue([]*resp.Value{}),
		})
	}

	consumerResults := make([]*resp.Value, 0, len(consumers))
	for _, c := range consumers {
		cPending := group.GetConsumerPending(c)
		consumerResults = append(consumerResults, resp.ArrayValue([]*resp.Value{
			resp.BulkString(c),
			resp.IntegerValue(cPending),
		}))
	}

	return ctx.WriteArray([]*resp.Value{
		resp.IntegerValue(pendingCount),
		resp.BulkString(firstID),
		resp.BulkString(lastID),
		resp.ArrayValue(consumerResults),
	})
}

func cmdXCLAIM(ctx *Context) error {
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	groupName := ctx.ArgString(1)
	consumerName := ctx.ArgString(2)
	minIdleTime, err := strconv.ParseInt(ctx.ArgString(3), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	var entryIDs []string
	var retryCount int64
	var force bool
	var justid bool

	i := 4
	for i < ctx.ArgCount() {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "IDLE", "TIME", "RETRYCOUNT":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			if arg == "RETRYCOUNT" {
				retryCount, _ = strconv.ParseInt(ctx.ArgString(i+1), 10, 64)
			}
			i += 2
		case "FORCE":
			force = true
			i++
		case "JUSTID":
			justid = true
			i++
		default:
			entryIDs = append(entryIDs, ctx.ArgString(i))
			i++
		}
	}

	if len(entryIDs) == 0 {
		return ctx.WriteError(ErrSyntaxError)
	}

	stream := getStream(ctx, key)
	if stream == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	group := stream.GetGroup(groupName)
	if group == nil {
		return ctx.WriteError(ErrNoGroup)
	}

	_ = minIdleTime
	_ = force
	_ = retryCount

	now := time.Now().UnixMilli()
	claimed := group.Claim(entryIDs, consumerName)

	results := make([]*resp.Value, 0, len(claimed))
	for _, id := range claimed {
		if justid {
			results = append(results, resp.BulkString(id))
		} else {
			entry := stream.GetEntryByID(id)
			if entry != nil {
				entryResult := []*resp.Value{resp.BulkString(entry.ID)}
				fieldValues := make([]*resp.Value, 0)
				for k, v := range entry.Fields {
					fieldValues = append(fieldValues, resp.BulkString(k), resp.BulkBytes(v))
				}
				entryResult = append(entryResult, resp.ArrayValue(fieldValues))
				results = append(results, resp.ArrayValue(entryResult))
			}
		}
	}

	_ = now

	return ctx.WriteArray(results)
}

func cmdXAUTOCLAIM(ctx *Context) error {
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	groupName := ctx.ArgString(1)
	consumerName := ctx.ArgString(2)
	minIdleTime, err := strconv.ParseInt(ctx.ArgString(3), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	start := ctx.ArgString(4)

	var count int64 = 100
	var justid bool

	i := 5
	for i < ctx.ArgCount() {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "COUNT":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			count, err = strconv.ParseInt(ctx.ArgString(i+1), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			i += 2
		case "JUSTID":
			justid = true
			i++
		default:
			i++
		}
	}

	stream := getStream(ctx, key)
	if stream == nil {
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("0-0"),
			resp.ArrayValue([]*resp.Value{}),
		})
	}

	group := stream.GetGroup(groupName)
	if group == nil {
		return ctx.WriteError(ErrNoGroup)
	}

	_ = minIdleTime

	now := time.Now().UnixMilli()
	pending := group.GetPending(start, "+", count)

	var entryIDs []string
	for _, p := range pending {
		if now-p.DeliveryTS >= minIdleTime {
			entryIDs = append(entryIDs, p.ID)
		}
	}

	claimed := group.Claim(entryIDs, consumerName)

	nextCursor := "0-0"
	if len(pending) >= int(count) {
		nextCursor = pending[count-1].ID
	}

	var results []*resp.Value
	if justid {
		results = make([]*resp.Value, 0, len(claimed))
		for _, id := range claimed {
			results = append(results, resp.BulkString(id))
		}
	} else {
		results = make([]*resp.Value, 0, len(claimed))
		for _, id := range claimed {
			entry := stream.GetEntryByID(id)
			if entry != nil {
				entryResult := []*resp.Value{resp.BulkString(entry.ID)}
				fieldValues := make([]*resp.Value, 0)
				for k, v := range entry.Fields {
					fieldValues = append(fieldValues, resp.BulkString(k), resp.BulkBytes(v))
				}
				entryResult = append(entryResult, resp.ArrayValue(fieldValues))
				results = append(results, resp.ArrayValue(entryResult))
			}
		}
	}

	_ = now

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(nextCursor),
		resp.ArrayValue(results),
	})
}

func cmdXSETID(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	lastID := ctx.ArgString(1)

	stream := getStream(ctx, key)
	if stream == nil {
		return ctx.WriteError(store.ErrKeyNotFound)
	}

	entriesAdded := int64(0)
	maxDeletedId := int64(0)
	for i := 2; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "ENTRIESADDED":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			entriesAdded, err = strconv.ParseInt(ctx.ArgString(i+1), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			i++
		case "MAXDELETEDID":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			i++
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	_ = entriesAdded
	_ = maxDeletedId

	stream.SetLastID(lastID)
	return ctx.WriteOK()
}
