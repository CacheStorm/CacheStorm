package command

import (
	"errors"
	"math"
	"sort"
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

// normalizeStreamBound expands a partial stream ID for range bounds: a bare
// millisecond becomes ms-0 (start) or ms-maxseq (end); "-" and "+" map to
// the extremes. Malformed IDs are rejected.
func normalizeStreamBound(bound string, isEnd bool) (string, error) {
	switch bound {
	case "-":
		return "0-0", nil
	case "+":
		return "9223372036854775807-9223372036854775807", nil
	}
	parts := strings.Split(bound, "-")
	switch len(parts) {
	case 1:
		ms, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return "", errors.New("ERR Invalid stream ID specified as stream command argument")
		}
		if isEnd {
			return strconv.FormatInt(ms, 10) + "-9223372036854775807", nil
		}
		return strconv.FormatInt(ms, 10) + "-0", nil
	case 2:
		if _, err1 := strconv.ParseInt(parts[0], 10, 64); err1 != nil {
			return "", errors.New("ERR Invalid stream ID specified as stream command argument")
		}
		if _, err2 := strconv.ParseInt(parts[1], 10, 64); err2 != nil {
			return "", errors.New("ERR Invalid stream ID specified as stream command argument")
		}
		return bound, nil
	default:
		return "", errors.New("ERR Invalid stream ID specified as stream command argument")
	}
}

func streamIDParts(id string) (int64, int64, bool) {
	parts := strings.SplitN(id, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	ms, err1 := strconv.ParseInt(parts[0], 10, 64)
	seq, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return ms, seq, true
}

func cmdXADD(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	maxLen := int64(0)
	trimLimit := int64(0)
	minID := ""
	approximate := false
	trimStrategy := ""
	nomkstream := false
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
			nomkstream = true
			argIdx++
		case "LIMIT":
			argIdx++
			if argIdx >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			trimLimit, err = strconv.ParseInt(ctx.ArgString(argIdx), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			if trimLimit < 0 {
				return ctx.WriteError(errCountOutOfRange)
			}
			argIdx++
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

	if nomkstream {
		if _, exists := ctx.Store.Get(key); !exists {
			return ctx.WriteNull()
		}
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
		if approximate {
			stream.TrimByMinIDLimited(minID, trimLimit)
		} else {
			stream.TrimByMinID(minID, false)
		}
	} else if trimStrategy == "MAXLEN" && maxLen > 0 {
		if approximate {
			stream.TrimLimited(maxLen, trimLimit)
		} else {
			stream.Trim(maxLen, approximate)
		}
	}

	_ = entry
	_ = approximate
	ctx.Store.KeyNotifier().NotifyKey(key)
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
		end = "9223372036854775807-9223372036854775807"
	}

	start, err := normalizeStreamBound(start, false)
	if err != nil {
		return ctx.WriteError(err)
	}
	end, err = normalizeStreamBound(end, true)
	if err != nil {
		return ctx.WriteError(err)
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

	start, err := normalizeStreamBound(start, false)
	if err != nil {
		return ctx.WriteError(err)
	}
	end, err = normalizeStreamBound(end, true)
	if err != nil {
		return ctx.WriteError(err)
	}

	entries := stream.GetRange(start, end, 0)
	if count > 0 && int64(len(entries)) > count {
		entries = entries[int64(len(entries))-count:]
	}

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
	blockSpecified := false
	streamsIdx := -1

	// Args exclude the command name, so options start at index 0; parsing
	// from 1 silently ignored every option the client sent first (COUNT,
	// BLOCK, unknown tokens alike).
	for i := 0; i < ctx.ArgCount(); i++ {
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
			// A negative timeout is a client error; silently treating it as
			// non-blocking would hide the mistake.
			if block < 0 {
				return ctx.WriteError(errCountOutOfRange)
			}
			blockSpecified = true
		case "STREAMS":
			streamsIdx = i + 1
			i = ctx.ArgCount()
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	if streamsIdx < 0 {
		return ctx.WriteError(ErrSyntaxError)
	}

	remaining := ctx.ArgCount() - streamsIdx
	if remaining < 2 || remaining%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	numStreams := remaining / 2
	keys := make([]string, numStreams)
	ids := make([]string, numStreams)

	for i := 0; i < numStreams; i++ {
		keys[i] = ctx.ArgString(streamsIdx + i)
		ids[i] = ctx.ArgString(streamsIdx + numStreams + i)
	}

	// Resolve "$" IDs to current last ID before blocking
	for i, id := range ids {
		if id == "$" {
			stream := getStream(ctx, keys[i])
			if stream != nil {
				ids[i] = stream.LastID
			} else {
				ids[i] = "0-0"
			}
		}
	}

	// Normalize and validate IDs: GetRange cannot parse partial ("2") or
	// malformed IDs and would silently return an empty read instead of
	// delivering everything after the boundary or reporting the error.
	for i, id := range ids {
		norm, err := normalizeStreamBound(id, false)
		if err != nil {
			return ctx.WriteError(err)
		}
		ids[i] = norm
	}

	// Try immediate read
	results := xreadStreams(ctx, keys, ids, count)
	if len(results) > 0 {
		return ctx.WriteArray(results)
	}

	// If no BLOCK, return null
	if !blockSpecified {
		return ctx.WriteNull()
	}

	// Block until notified or timeout. BLOCK 0 blocks indefinitely, so it
	// gets a deadline far enough out that only arriving data can return
	// before it.
	notifier := ctx.Store.KeyNotifier()
	dur := time.Duration(block) * time.Millisecond
	if block == 0 {
		dur = time.Duration(1) << 62
	}
	deadline := time.Now().Add(dur)
	const maxRetries = 100

	for attempt := 0; attempt < maxRetries; attempt++ {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return ctx.WriteNull()
		}
		_, notified := notifier.WaitForKeys(keys, remaining)
		if !notified {
			return ctx.WriteNull()
		}

		results = xreadStreams(ctx, keys, ids, count)
		if len(results) > 0 {
			return ctx.WriteArray(results)
		}
	}
	return ctx.WriteNull()
}

func xreadStreams(ctx *Context, keys, ids []string, count int64) []*resp.Value {
	results := make([]*resp.Value, 0, len(keys))

	for i, key := range keys {
		stream := getStream(ctx, key)
		if stream == nil {
			continue
		}

		readCount := count
		if count > 0 && count < math.MaxInt64 && stream.GetEntryByID(ids[i]) != nil {
			readCount++
		}
		entries := stream.GetRange(ids[i], "+", readCount)
		if len(entries) == 0 {
			continue
		}

		entryResults := make([]*resp.Value, 0, len(entries))
		for _, entry := range entries {
			if entry.ID == ids[i] {
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
			if count > 0 && int64(len(entryResults)) >= count {
				break
			}
		}

		if len(entryResults) > 0 {
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.BulkString(key),
				resp.ArrayValue(entryResults),
			}))
		}
	}
	return results
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

	trimStrategy := strings.ToUpper(ctx.ArgString(1))
	if trimStrategy != "MAXLEN" && trimStrategy != "MINID" {
		return ctx.WriteError(ErrSyntaxError)
	}

	approximate := false
	idx := 2

	if ctx.ArgCount() > idx && strings.ToUpper(ctx.ArgString(idx)) == "~" {
		approximate = true
		idx++
	}

	if ctx.ArgCount() > idx && strings.ToUpper(ctx.ArgString(idx)) == "=" {
		idx++
	}

	if idx >= ctx.ArgCount() {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if trimStrategy == "MINID" {
		minID, err := normalizeStreamBound(ctx.ArgString(idx), false)
		if err != nil {
			return ctx.WriteError(err)
		}
		idx++

		limit, err := parseTrimLimit(ctx, idx)
		if err != nil {
			return ctx.WriteError(err)
		}

		stream := getStream(ctx, key)
		if stream == nil {
			return ctx.WriteInteger(0)
		}

		if approximate {
			return ctx.WriteInteger(stream.TrimByMinIDLimited(minID, limit))
		}
		return ctx.WriteInteger(stream.TrimByMinID(minID, false))
	}

	maxLen, err := strconv.ParseInt(ctx.ArgString(idx), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	// MAXLEN is a count of entries to keep. A negative count is a client
	// error: StreamValue.Trim would otherwise compute remove = Length-maxLen,
	// overshoot the entry count, and slice v.Entries[remove:] out of range.
	if maxLen < 0 {
		return ctx.WriteError(errCountOutOfRange)
	}

	limit, err := parseTrimLimit(ctx, idx+1)
	if err != nil {
		return ctx.WriteError(err)
	}

	stream := getStream(ctx, key)
	if stream == nil {
		return ctx.WriteInteger(0)
	}

	if approximate {
		return ctx.WriteInteger(stream.TrimLimited(maxLen, limit))
	}
	return ctx.WriteInteger(stream.Trim(maxLen, approximate))
}

// parseTrimLimit reads an optional trailing "LIMIT count" clause at idx.
// LIMIT 0 disables the eviction cap, matching Redis; a negative count is a
// client error.
func parseTrimLimit(ctx *Context, idx int) (int64, error) {
	if idx >= ctx.ArgCount() || strings.ToUpper(ctx.ArgString(idx)) != "LIMIT" {
		return 0, nil
	}
	idx++
	if idx >= ctx.ArgCount() {
		return 0, ErrSyntaxError
	}
	limit, err := strconv.ParseInt(ctx.ArgString(idx), 10, 64)
	if err != nil {
		return 0, ErrNotInteger
	}
	if limit < 0 {
		return 0, errCountOutOfRange
	}
	return limit, nil
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
			// Redis raises NOGROUP rather than an empty listing here.
			return ctx.WriteError(ErrNoGroup)
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
			// Redis raises NOGROUP rather than an empty listing here.
			return ctx.WriteError(ErrNoGroup)
		}

		group := stream.GetGroup(groupName)
		if group == nil {
			// Redis raises NOGROUP rather than an empty listing here.
			return ctx.WriteError(ErrNoGroup)
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

		// A malformed or partial id would be stored raw and break every
		// later read of this group; "$" is resolved inside CreateGroup.
		groupStart := lastID
		if lastID != "$" {
			var nerr error
			groupStart, nerr = normalizeStreamBound(lastID, false)
			if nerr != nil {
				return ctx.WriteError(nerr)
			}
		}

		err := stream.CreateGroup(groupName, groupStart)
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

		// Same raw-storage hazard as CREATE: an unparseable id would leave
		// the group permanently silent. "$" resolves to the stream's last
		// generated id.
		var groupStart string
		if lastID == "$" {
			groupStart = stream.LastID
		} else {
			var nerr error
			groupStart, nerr = normalizeStreamBound(lastID, false)
			if nerr != nil {
				return ctx.WriteError(nerr)
			}
		}

		if !stream.SetGroupLastID(groupName, groupStart) {
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
			// Redis raises NOGROUP for the key-requiring XGROUP subcommands
			// instead of a zero count (family-consistent with SETID and
			// CREATECONSUMER).
			return ctx.WriteError(ErrNoGroup)
		}

		group := stream.GetGroup(groupName)
		if group == nil {
			return ctx.WriteError(ErrNoGroup)
		}

		var pending int64
		if c, exists := group.Consumers[consumerName]; exists {
			pending = c.Pending
			// Redis purges the deleted consumer's pending entries from the
			// group PEL: they become unclaimable, not orphaned under a ghost.
			for id, p := range group.Pending {
				if p.Consumer == consumerName {
					delete(group.Pending, id)
				}
			}
			delete(group.Consumers, consumerName)
		}

		return ctx.WriteInteger(pending)

	case "CREATECONSUMER":
		if ctx.ArgCount() < 4 {
			return ctx.WriteError(ErrWrongArgCount)
		}

		key := ctx.ArgString(1)
		groupName := ctx.ArgString(2)
		consumerName := ctx.ArgString(3)

		stream := getStream(ctx, key)
		if stream == nil {
			return ctx.WriteError(ErrNoGroup)
		}

		group := stream.GetGroup(groupName)
		if group == nil {
			return ctx.WriteError(ErrNoGroup)
		}

		created := int64(0)
		if _, exists := group.Consumers[consumerName]; !exists {
			group.GetOrCreateConsumer(consumerName)
			created = 1
		}

		return ctx.WriteInteger(created)

	default:
		return ctx.WriteError(ErrUnknownCommand)
	}
}

func cmdXREADGROUP(ctx *Context) error {
	if ctx.ArgCount() < 6 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	var groupName, consumerName string
	var count int64 = 0
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
		// A stream that does not exist has no consumer group either, so this
		// is the same condition as the missing-group arm below and must be
		// answered the same way. It used to `continue`, handing the caller an
		// empty reply that reads as "no new messages" — three lines below, the
		// missing GROUP returned NOGROUP, so one condition had two opposite
		// answers in the same loop.
		if stream == nil {
			return ctx.WriteError(ErrNoGroup)
		}

		group := stream.GetGroup(groupName)
		if group == nil {
			return ctx.WriteError(ErrNoGroup)
		}

		consumer := group.GetOrCreateConsumer(consumerName)
		consumer.SeenTime = time.Now().UnixMilli()

		if ids[i] != ">" {
			pending := group.GetPending("-", "+", 0)
			type pelItem struct {
				id  string
				ms  int64
				seq int64
			}
			items := make([]pelItem, 0)
			for _, p := range pending {
				if p.Consumer != consumerName {
					continue
				}
				ms, seq, ok := streamIDParts(p.ID)
				if !ok {
					continue
				}
				if startMS, startSeq, valid := streamIDParts(ids[i]); valid {
					if ms < startMS || (ms == startMS && seq <= startSeq) {
						continue
					}
				}
				items = append(items, pelItem{p.ID, ms, seq})
			}
			sort.Slice(items, func(a, b int) bool {
				return items[a].ms < items[b].ms || (items[a].ms == items[b].ms && items[a].seq < items[b].seq)
			})
			if count > 0 && int64(len(items)) > count {
				items = items[:count]
			}
			for _, it := range items {
				entryResult := []*resp.Value{resp.BulkString(it.id)}
				if entry := stream.GetEntryByID(it.id); entry != nil {
					fieldValues := make([]*resp.Value, 0, len(entry.Fields)*2)
					for k, v := range entry.Fields {
						fieldValues = append(fieldValues, resp.BulkString(k), resp.BulkBytes(v))
					}
					entryResult = append(entryResult, resp.ArrayValue(fieldValues))
				} else {
					entryResult = append(entryResult, resp.NullArray())
				}
				results[i] = append(results[i], entryResult...)
				totalEntries++
			}
			continue
		}

		var startID string
		if ids[i] == ">" {
			startID = group.LastID
			if startID == "0-0" || startID == "0" {
				startID = "-"
			}
		} else {
			startID = ids[i]
		}

		readCount := count
		if ids[i] == ">" && startID != "-" && readCount > 0 {
			readCount++
		}

		entries := stream.GetRange(startID, "+", readCount)
		var lastDelivered string
		for _, entry := range entries {
			if ids[i] == ">" {
				if startID != "-" && entry.ID == startID {
					continue
				}
				if !noack {
					group.AddPending(entry.ID, consumerName)
				}
				lastDelivered = entry.ID
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

		if ids[i] == ">" && lastDelivered != "" {
			stream.SetGroupLastID(groupName, lastDelivered)
		}
	}

	if totalEntries == 0 && block > 0 && len(ids) > 0 && ids[0] == ">" {
		notifier := ctx.Store.KeyNotifier()
		dur := time.Duration(block) * time.Millisecond

		for {
			_, notified := notifier.WaitForKeys(keys, dur)
			if !notified {
				break
			}

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
				if startID == "0-0" || startID == "0" {
					startID = "-"
				}

				readCount := count
				if startID != "-" && readCount > 0 {
					readCount++
				}

				entries := stream.GetRange(startID, "+", readCount)
				var lastDelivered string
				for _, entry := range entries {
					if startID != "-" && entry.ID == startID {
						continue
					}
					if !noack {
						group.AddPending(entry.ID, consumerName)
					}
					lastDelivered = entry.ID

					entryResult := []*resp.Value{resp.BulkString(entry.ID)}
					fieldValues := make([]*resp.Value, 0)
					for k, v := range entry.Fields {
						fieldValues = append(fieldValues, resp.BulkString(k), resp.BulkBytes(v))
					}
					entryResult = append(entryResult, resp.ArrayValue(fieldValues))
					results[i] = append(results[i], entryResult...)
					totalEntries++
				}

				if lastDelivered != "" {
					stream.SetGroupLastID(groupName, lastDelivered)
				}
			}

			if totalEntries > 0 {
				break
			}
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
		// Redis counts a missing group as "nothing acknowledged", not an error.
		return ctx.WriteInteger(0)
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

	if ctx.ArgCount() > 2 {
		// Redis requires the full <start> <end> <count> triple for the
		// detail form.
		if ctx.ArgCount() < 5 {
			return ctx.WriteError(ErrWrongArgCount)
		}

		start, err := normalizeStreamBound(ctx.ArgString(2), false)
		if err != nil {
			return ctx.WriteError(err)
		}
		end, err := normalizeStreamBound(ctx.ArgString(3), true)
		if err != nil {
			return ctx.WriteError(err)
		}
		count, err := strconv.ParseInt(ctx.ArgString(4), 10, 64)
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
		if count <= 0 {
			return ctx.WriteError(ErrSyntaxError)
		}
		consumer := ""
		if ctx.ArgCount() > 5 {
			consumer = ctx.ArgString(5)
		}

		// GetPending's own bounds compare lexicographically, so fetch the
		// whole PEL and range/filter numerically: variable-width IDs sort
		// wrong as strings ("10-0" < "9-0").
		pending := group.GetPending("-", "+", 0)
		now := time.Now().UnixMilli()
		startMS, startSeq, hasStart := streamIDParts(start)
		endMS, endSeq, hasEnd := streamIDParts(end)
		type pendingRow struct {
			id         string
			consumer   string
			idle       int64
			deliveries int64
			ms, seq    int64
		}
		rows := make([]pendingRow, 0, len(pending))
		for _, p := range pending {
			ms, seq, ok := streamIDParts(p.ID)
			if !ok {
				continue
			}
			if hasStart && (ms < startMS || (ms == startMS && seq < startSeq)) {
				continue
			}
			if hasEnd && (ms > endMS || (ms == endMS && seq > endSeq)) {
				continue
			}
			if consumer != "" && p.Consumer != consumer {
				continue
			}
			rows = append(rows, pendingRow{
				id:         p.ID,
				consumer:   p.Consumer,
				idle:       now - p.DeliveryTS,
				deliveries: p.Deliveries,
				ms:         ms,
				seq:        seq,
			})
		}
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].ms != rows[j].ms {
				return rows[i].ms < rows[j].ms
			}
			return rows[i].seq < rows[j].seq
		})
		if int64(len(rows)) > count {
			rows = rows[:count]
		}

		results := make([]*resp.Value, 0, len(rows))
		for _, r := range rows {
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.BulkString(r.id),
				resp.BulkString(r.consumer),
				resp.IntegerValue(r.idle),
				resp.IntegerValue(r.deliveries),
			}))
		}
		return ctx.WriteArray(results)
	}

	pendingCount := group.GetPendingCount()
	firstID, lastID := group.GetFirstLastID()
	consumers := group.GetAllConsumers()

	if pendingCount == 0 {
		// Redis short-circuits here and answers with an EMPTY array: an empty
		// PEL has no smallest/greatest ID to report, so the 4-element summary
		// shape does not exist. Building it with null placeholders told a
		// client reading element [1] of a 4-element reply that its ID was null.
		return ctx.WriteArray([]*resp.Value{})
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
	var setRetry bool
	var force bool
	var justid bool
	var idleMS int64
	var setIdle bool
	var timeMS int64
	var setTime bool

	i := 4
	for i < ctx.ArgCount() {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "IDLE", "TIME", "RETRYCOUNT":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			value, err := strconv.ParseInt(ctx.ArgString(i+1), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrSyntaxError)
			}
			switch arg {
			case "RETRYCOUNT":
				retryCount = value
				setRetry = true
			case "IDLE":
				idleMS = value
				setIdle = true
			case "TIME":
				timeMS = value
				setTime = true
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

	if force {
		filtered := make([]string, 0, len(entryIDs))
		for _, id := range entryIDs {
			if stream.GetEntryByID(id) != nil {
				filtered = append(filtered, id)
			}
		}
		entryIDs = filtered
	}

	claimed := group.ClaimWithOptions(entryIDs, consumerName, store.ClaimOptions{
		MinIdleTime: minIdleTime,
		JustID:      justid,
		Force:       force,
		RetryCount:  retryCount,
		SetRetry:    setRetry,
		IdleMS:      idleMS,
		SetIdle:     setIdle,
		TimeMS:      timeMS,
		SetTime:     setTime,
	})

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
			resp.ArrayValue([]*resp.Value{}),
		})
	}

	group := stream.GetGroup(groupName)
	if group == nil {
		return ctx.WriteError(ErrNoGroup)
	}

	now := time.Now().UnixMilli()

	// GetPending's own bounds compare lexicographically, so normalize the
	// start and scan the whole PEL numerically: variable-width IDs sort wrong
	// as strings ("10-0" < "9-0").
	startNorm, err := normalizeStreamBound(start, false)
	if err != nil {
		return ctx.WriteError(err)
	}
	startMS, startSeq, hasStart := streamIDParts(startNorm)
	pending := group.GetPending("-", "+", 0)
	type pendingRef struct {
		id         string
		ms, seq    int64
		deliveryTS int64
	}
	refs := make([]pendingRef, 0, len(pending))
	for _, p := range pending {
		ms, seq, ok := streamIDParts(p.ID)
		if !ok {
			continue
		}
		if hasStart && (ms < startMS || (ms == startMS && seq < startSeq)) {
			continue
		}
		refs = append(refs, pendingRef{id: p.ID, ms: ms, seq: seq, deliveryTS: p.DeliveryTS})
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].ms != refs[j].ms {
			return refs[i].ms < refs[j].ms
		}
		return refs[i].seq < refs[j].seq
	})

	var candidates []string
	var deletedIDs []string
	for _, ref := range refs {
		// A PEL entry whose stream entry was removed by XDEL is stale: it
		// must leave the PEL and be reported, or it leaks to every future
		// consumer.
		if stream.GetEntryByID(ref.id) == nil {
			group.Ack(ref.id)
			deletedIDs = append(deletedIDs, ref.id)
			continue
		}
		if now-ref.deliveryTS < minIdleTime {
			continue
		}
		candidates = append(candidates, ref.id)
		if len(candidates) >= int(count) {
			break
		}
	}

	claimed := group.ClaimWithOptions(candidates, consumerName, store.ClaimOptions{JustID: justid})

	nextCursor := "0-0"
	if len(candidates) >= int(count) {
		nextCursor = candidates[len(candidates)-1]
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

	deletedVals := make([]*resp.Value, 0, len(deletedIDs))
	for _, id := range deletedIDs {
		deletedVals = append(deletedVals, resp.BulkString(id))
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(nextCursor),
		resp.ArrayValue(results),
		resp.ArrayValue(deletedVals),
	})
}

func cmdXSETID(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	stream := getStream(ctx, key)
	if stream == nil {
		return ctx.WriteError(store.ErrKeyNotFound)
	}

	// A malformed or partial ID here would corrupt every later XADD: the
	// monotonicity guard compares parsed IDs and silently passes anything it
	// cannot parse.
	lastID, err := normalizeStreamBound(ctx.ArgString(1), false)
	if err != nil {
		return ctx.WriteError(err)
	}

	entriesAdded := int64(0)
	maxDeletedID := ""
	i := 2
	for i < ctx.ArgCount() {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "ENTRIESADDED":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			entriesAdded, err = strconv.ParseInt(ctx.ArgString(i+1), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			i += 2
		case "MAXDELETEDID":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			maxDeletedID, err = normalizeStreamBound(ctx.ArgString(i+1), true)
			if err != nil {
				return ctx.WriteError(err)
			}
			i += 2
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	// The new last ID must not go backwards past the stream's top item.
	if top := stream.LastID; top != "" {
		topMS, topSeq, topOK := streamIDParts(top)
		newMS, newSeq, newOK := streamIDParts(lastID)
		if topOK && newOK && (newMS < topMS || (newMS == topMS && newSeq < topSeq)) {
			return ctx.WriteError(errors.New("ERR The ID specified in XSETID is smaller than the target stream top item"))
		}
	}

	// ENTRIESADDED remains untracked (no counter exists). MAXDELETEDID is
	// consumed: it sets the deleted-ID floor that XADD enforces.
	_ = entriesAdded
	stream.MaxDeletedID = maxDeletedID

	stream.SetLastID(lastID)
	return ctx.WriteOK()
}
