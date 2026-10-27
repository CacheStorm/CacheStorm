package command

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

var (
	ErrBusyGroup = streamReplyError("BUSYGROUP Consumer Group name already exists")
	ErrNoGroup   = streamReplyError("NOGROUP No such key")

	errStreamNoSuchKey   = streamReplyError("ERR no such key")
	errXGroupKeyRequired = streamReplyError("ERR The XGROUP subcommand requires the key to exist. Note that for CREATE you may want to use the MKSTREAM option to create an empty stream automatically.")
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

// streamReplyError carries a Redis error reply verbatim, trailing
// punctuation included, since clients match these texts.
type streamReplyError string

func (e streamReplyError) Error() string { return string(e) }

func streamReplyErrorf(format string, args ...interface{}) error {
	return streamReplyError(fmt.Sprintf(format, args...))
}

func errNoGroupFor(key, group string) error {
	return streamReplyErrorf("NOGROUP No such key '%s' or consumer group '%s'", key, group)
}

func errNoReadGroup(key, group string) error {
	return streamReplyErrorf("NOGROUP No such key '%s' or consumer group '%s' in XREADGROUP with GROUP option", key, group)
}

func errNoConsumerGroup(group, key string) error {
	return streamReplyErrorf("NOGROUP No such consumer group '%s' for key name '%s'", group, key)
}

func errSubcommandSyntax(sub, cmd string) error {
	return streamReplyErrorf("ERR unknown subcommand or wrong number of arguments for '%s'. Try %s HELP.", sub, cmd)
}

func errUnknownSubcommand(sub, cmd string) error {
	return streamReplyErrorf("ERR unknown subcommand '%s'. Try %s HELP.", sub, cmd)
}

// lookupStream returns the stream at key, nil when the key is missing, and
// WRONGTYPE when it holds another type.
func lookupStream(ctx *Context, key string) (*store.StreamValue, error) {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return nil, nil
	}
	stream, ok := entry.Value.(*store.StreamValue)
	if !ok {
		return nil, store.ErrWrongType
	}
	return stream, nil
}

func getStream(ctx *Context, key string) *store.StreamValue {
	stream, err := lookupStream(ctx, key)
	if err != nil {
		return nil
	}
	return stream
}

// parseStreamLL parses an integer argument the way Redis does: no sign
// other than a leading minus, no surrounding spaces.
func parseStreamLL(s string) (int64, bool) {
	if s == "" || s[0] == '+' {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	return v, err == nil
}

func parseStrictStreamID(s string) (store.StreamID, error) {
	return store.ParseStreamID(s, 0)
}

func streamEntryValue(e *store.StreamEntry) *resp.Value {
	pairs := e.FieldPairs()
	fields := make([]*resp.Value, 0, len(pairs)*2)
	for _, p := range pairs {
		fields = append(fields, resp.BulkString(p.Name), resp.BulkBytes(nonNilBytes(p.Value)))
	}
	return resp.ArrayValue([]*resp.Value{resp.BulkString(e.ID), resp.ArrayValue(fields)})
}

func nonNilBytes(b []byte) []byte {
	if b == nil {
		return []byte{}
	}
	return b
}

func streamEntriesValue(entries []*store.StreamEntry) *resp.Value {
	out := make([]*resp.Value, 0, len(entries))
	for _, e := range entries {
		out = append(out, streamEntryValue(e))
	}
	return resp.ArrayValue(out)
}

func streamItemsValue(items []store.StreamGroupItem, justID bool) *resp.Value {
	out := make([]*resp.Value, 0, len(items))
	for _, it := range items {
		switch {
		case justID:
			out = append(out, resp.BulkString(it.ID.String()))
		case it.Entry == nil:
			out = append(out, resp.ArrayValue([]*resp.Value{resp.BulkString(it.ID.String()), resp.NullArray()}))
		default:
			out = append(out, streamEntryValue(it.Entry))
		}
	}
	return resp.ArrayValue(out)
}

// generateStreamID returns the ID XADD * would assign after lastID.
func generateStreamID(lastID string) string {
	last, err := store.ParseStreamID(lastID, 0)
	if err != nil {
		last = store.StreamID{}
	}
	now := uint64(time.Now().UnixMilli())
	if now > last.Ms {
		return store.StreamID{Ms: now}.String()
	}
	next, _ := last.Next()
	return next.String()
}

type addTrimArgs struct {
	trim       store.StreamTrimArgs
	noMkStream bool
	id         store.StreamAddArgs
}

// parseAddTrimArgs parses the options shared by XADD and XTRIM, starting
// after the key. For XADD it stops at the ID and returns its index.
func parseAddTrimArgs(ctx *Context, xadd bool) (addTrimArgs, int, error) {
	var a addTrimArgs
	argc := ctx.ArgCount()
	limitGiven := false
	i := 1
loop:
	for ; i < argc; i++ {
		moreargs := argc - 1 - i
		opt := ctx.ArgString(i)
		switch {
		case xadd && opt == "*":
			break loop
		case strings.EqualFold(opt, "MAXLEN") && moreargs > 0, strings.EqualFold(opt, "MINID") && moreargs > 0:
			if a.trim.Strategy != store.StreamTrimNone {
				return a, 0, streamReplyError("ERR syntax error, MAXLEN and MINID options at the same time are not compatible")
			}
			a.trim.Approx = false
			next := ctx.ArgString(i + 1)
			if moreargs >= 2 && next == "~" {
				a.trim.Approx = true
				i++
			} else if moreargs >= 2 && next == "=" {
				i++
			}
			if strings.EqualFold(opt, "MAXLEN") {
				n, ok := parseStreamLL(ctx.ArgString(i + 1))
				if !ok {
					return a, 0, ErrNotInteger
				}
				if n < 0 {
					return a, 0, streamReplyError("ERR The MAXLEN argument must be >= 0.")
				}
				a.trim.Strategy = store.StreamTrimMaxLen
				a.trim.MaxLen = n
			} else {
				id, err := parseStrictStreamID(ctx.ArgString(i + 1))
				if err != nil {
					return a, 0, err
				}
				a.trim.Strategy = store.StreamTrimMinID
				a.trim.MinID = id
			}
			i++
		case strings.EqualFold(opt, "LIMIT") && moreargs > 0:
			n, ok := parseStreamLL(ctx.ArgString(i + 1))
			if !ok {
				return a, 0, ErrNotInteger
			}
			if n < 0 {
				return a, 0, streamReplyError("ERR The LIMIT argument must be >= 0.")
			}
			a.trim.Limit = n
			limitGiven = true
			i++
		case xadd && strings.EqualFold(opt, "NOMKSTREAM"):
			a.noMkStream = true
		case xadd:
			id, err := parseXAddID(opt)
			if err != nil {
				return a, 0, err
			}
			a.id = id
			break loop
		default:
			return a, 0, ErrSyntaxError
		}
	}
	if a.trim.Limit != 0 && a.trim.Strategy == store.StreamTrimNone {
		return a, 0, streamReplyError("ERR syntax error, LIMIT cannot be used without specifying a trimming strategy")
	}
	if !xadd && a.trim.Strategy == store.StreamTrimNone {
		return a, 0, streamReplyError("ERR syntax error, XTRIM must be called with a trimming strategy")
	}
	if limitGiven {
		if !a.trim.Approx {
			return a, 0, streamReplyError("ERR syntax error, LIMIT cannot be used without the special ~ option")
		}
	} else if a.trim.Approx {
		a.trim.Limit = 10000
	}
	return a, i, nil
}

// parseXAddID parses "*", "<ms>-*", "<ms>" or "<ms>-<seq>".
func parseXAddID(s string) (store.StreamAddArgs, error) {
	if s == "*" {
		return store.StreamAddArgs{}, nil
	}
	if strings.HasSuffix(s, "-*") {
		id, err := store.ParseStreamID(s[:len(s)-2], 0)
		if err != nil || strings.Contains(s[:len(s)-2], "-") {
			return store.StreamAddArgs{}, store.ErrStreamInvalidID
		}
		return store.StreamAddArgs{ID: id, IDGiven: true}, nil
	}
	id, err := store.ParseStreamID(s, 0)
	if err != nil {
		return store.StreamAddArgs{}, err
	}
	return store.StreamAddArgs{ID: id, IDGiven: true, SeqGiven: true}, nil
}

func cmdXADD(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(arityError("xadd"))
	}
	key := ctx.ArgString(0)
	a, idIdx, err := parseAddTrimArgs(ctx, true)
	if err != nil {
		return ctx.WriteError(err)
	}
	fieldPos := idIdx + 1
	n := ctx.ArgCount() - fieldPos
	if n < 2 || n%2 == 1 {
		return ctx.WriteError(arityError("xadd"))
	}
	if a.id.IDGiven && a.id.SeqGiven && a.id.ID.IsZero() {
		return ctx.WriteError(store.ErrStreamIDZero)
	}

	pairs := make([]store.StreamField, 0, n/2)
	for i := fieldPos; i < ctx.ArgCount(); i += 2 {
		pairs = append(pairs, store.StreamField{Name: ctx.ArgString(i), Value: ctx.Arg(i + 1)})
	}

	stream, err := lookupStream(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	created := false
	if stream == nil {
		if a.noMkStream {
			return ctx.WriteNull()
		}
		stream = store.NewStreamValue(0)
		created = true
	}

	entry, trimmed, err := stream.XAddTrim(a.id, pairs, a.trim, uint64(time.Now().UnixMilli()))
	if err != nil {
		return ctx.WriteError(err)
	}
	if created {
		// Install the new stream only if the key is still missing: a plain
		// Set let two concurrent creators both store one, the second
		// replacing the first and its entry.
		v, installed, err := ctx.Store.GetOrCreate(key, func() store.Value { return stream })
		if err != nil {
			return ctx.WriteError(err)
		}
		if !installed {
			existing, ok := v.(*store.StreamValue)
			if !ok {
				return ctx.WriteError(store.ErrWrongType)
			}
			if entry, trimmed, err = existing.XAddTrim(a.id, pairs, a.trim, uint64(time.Now().UnixMilli())); err != nil {
				return ctx.WriteError(err)
			}
		}
	}
	if !a.id.IDGiven || !a.id.SeqGiven {
		ctx.Args[idIdx] = []byte(entry.ID)
	}
	ctx.notify(store.NotifyStream, "xadd", key)
	if trimmed > 0 {
		ctx.notify(store.NotifyStream, "xtrim", key)
	}
	ctx.Store.KeyNotifier().NotifyKey(key)
	return ctx.WriteBulkString(entry.ID)
}

func cmdXLEN(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError("xlen"))
	}
	stream, err := lookupStream(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if stream == nil {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(stream.Len())
}

func cmdXRANGE(ctx *Context) error    { return xrangeGeneric(ctx, false) }
func cmdXREVRANGE(ctx *Context) error { return xrangeGeneric(ctx, true) }

func xrangeGeneric(ctx *Context, rev bool) error {
	name := "xrange"
	if rev {
		name = "xrevrange"
	}
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError(name))
	}
	startArg, endArg := ctx.ArgString(1), ctx.ArgString(2)
	if rev {
		startArg, endArg = endArg, startArg
	}
	start, err := store.ParseStreamInterval(startArg, false)
	if err != nil {
		return ctx.WriteError(err)
	}
	end, err := store.ParseStreamInterval(endArg, true)
	if err != nil {
		return ctx.WriteError(err)
	}
	count := int64(-1)
	for j := 3; j < ctx.ArgCount(); j++ {
		if strings.EqualFold(ctx.ArgString(j), "COUNT") && j+1 < ctx.ArgCount() {
			n, ok := parseStreamLL(ctx.ArgString(j + 1))
			if !ok {
				return ctx.WriteError(ErrNotInteger)
			}
			if n < 0 {
				n = 0
			}
			count = n
			j++
			continue
		}
		return ctx.WriteError(ErrSyntaxError)
	}

	stream, err := lookupStream(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if stream == nil {
		return ctx.WriteArray([]*resp.Value{})
	}
	if count == 0 {
		return ctx.WriteNullArray()
	}
	if count < 0 {
		count = 0
	}
	return ctx.WriteValue(streamEntriesValue(stream.Range(start, end, count, rev)))
}

type xreadSpec struct {
	key     string
	gt      store.StreamID
	newOnly bool
}

func cmdXREAD(ctx *Context) error      { return xreadGeneric(ctx, false) }
func cmdXREADGROUP(ctx *Context) error { return xreadGeneric(ctx, true) }

func xreadGeneric(ctx *Context, xreadgroup bool) error {
	name := "xread"
	symbol := "$"
	if xreadgroup {
		name = "xreadgroup"
		symbol = ">"
	}
	argc := ctx.ArgCount()
	timeout := int64(-1)
	count := int64(0)
	streamsArg := -1
	streamsCount := 0
	noack := false
	groupName, consumerName := "", ""
	haveGroup := false

parse:
	for i := 0; i < argc; i++ {
		moreargs := argc - i - 1
		o := ctx.ArgString(i)
		switch {
		case strings.EqualFold(o, "BLOCK") && moreargs > 0:
			i++
			t, ok := parseStreamLL(ctx.ArgString(i))
			if !ok {
				return ctx.WriteError(streamReplyError("ERR timeout is not an integer or out of range"))
			}
			if t < 0 {
				return ctx.WriteError(streamReplyError("ERR timeout is negative"))
			}
			timeout = t
		case strings.EqualFold(o, "COUNT") && moreargs > 0:
			i++
			n, ok := parseStreamLL(ctx.ArgString(i))
			if !ok {
				return ctx.WriteError(ErrNotInteger)
			}
			if n < 0 {
				n = 0
			}
			count = n
		case strings.EqualFold(o, "STREAMS") && moreargs > 0:
			streamsArg = i + 1
			n := argc - streamsArg
			if n%2 != 0 {
				return ctx.WriteError(streamReplyErrorf("ERR Unbalanced '%s' list of streams: for each stream key an ID or '%s' must be specified.", name, symbol))
			}
			streamsCount = n / 2
			break parse
		case strings.EqualFold(o, "GROUP") && moreargs >= 2:
			if !xreadgroup {
				return ctx.WriteError(streamReplyError("ERR The GROUP option is only supported by XREADGROUP. You called XREAD instead."))
			}
			groupName, consumerName = ctx.ArgString(i+1), ctx.ArgString(i+2)
			haveGroup = true
			i += 2
		case strings.EqualFold(o, "NOACK"):
			if !xreadgroup {
				return ctx.WriteError(streamReplyError("ERR The NOACK option is only supported by XREADGROUP. You called XREAD instead."))
			}
			noack = true
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}
	if streamsArg < 0 {
		return ctx.WriteError(ErrSyntaxError)
	}
	if xreadgroup && !haveGroup {
		return ctx.WriteError(streamReplyError("ERR Missing GROUP option for XREADGROUP"))
	}

	specs := make([]xreadSpec, streamsCount)
	keys := make([]string, streamsCount)
	for i := 0; i < streamsCount; i++ {
		key := ctx.ArgString(streamsArg + i)
		idArg := ctx.ArgString(streamsArg + streamsCount + i)
		keys[i] = key
		specs[i].key = key
		stream, err := lookupStream(ctx, key)
		if err != nil {
			return ctx.WriteError(err)
		}
		if xreadgroup && (stream == nil || !stream.HasGroup(groupName)) {
			return ctx.WriteError(errNoReadGroup(key, groupName))
		}
		switch idArg {
		case "$":
			if xreadgroup {
				return ctx.WriteError(streamReplyError("ERR The $ ID is meaningless in the context of XREADGROUP: you want to read the history of this consumer by specifying a proper ID, or use the > ID to get new messages. The $ ID would just return an empty result set."))
			}
			if stream != nil {
				specs[i].gt = stream.LastGeneratedID()
			}
		case ">":
			if !xreadgroup {
				return ctx.WriteError(streamReplyError("ERR The > ID can be specified only when calling XREADGROUP using the GROUP <group> <consumer> option."))
			}
			specs[i].newOnly = true
		case "+":
			if xreadgroup {
				return ctx.WriteError(streamReplyError("ERR The + ID is meaningless in the context of XREADGROUP: you want to read the history of this consumer by specifying a proper ID, or use the > ID to get new messages. The + ID would just return an empty result set."))
			}
			if stream != nil {
				if last, ok := stream.LastEntryID(); ok {
					specs[i].gt, _ = last.Prev()
				} else {
					specs[i].gt = stream.LastGeneratedID()
				}
			}
		default:
			id, err := parseStrictStreamID(idArg)
			if err != nil {
				return ctx.WriteError(err)
			}
			specs[i].gt = id
		}
	}

	// writeResults answers with one [key, entries] element per stream: an
	// array of pairs in RESP2 and a map keyed by stream in RESP3, as Redis.
	writeResults := func(results []*resp.Value) error {
		if !ctx.Writer.IsRESP3() {
			return ctx.WriteArray(results)
		}
		pairs := make([]*resp.Value, 0, 2*len(results))
		for _, r := range results {
			pairs = append(pairs, r.Array[0], r.Array[1])
		}
		return ctx.Writer.WriteValue(resp.PairsValue(pairs))
	}

	serve := func() ([]*resp.Value, error) {
		results := make([]*resp.Value, 0, len(specs))
		now := time.Now().UnixMilli()
		for _, sp := range specs {
			stream, err := lookupStream(ctx, sp.key)
			if err != nil {
				return nil, err
			}
			if !xreadgroup {
				if stream == nil {
					continue
				}
				entries := stream.After(sp.gt, count)
				if len(entries) == 0 {
					continue
				}
				results = append(results, resp.ArrayValue([]*resp.Value{resp.BulkString(sp.key), streamEntriesValue(entries)}))
				continue
			}
			if stream == nil {
				return nil, errNoReadGroup(sp.key, groupName)
			}
			knownConsumer := !ctx.Store.Notifying(store.NotifyStream) || stream.HasConsumer(groupName, consumerName)
			items, served, ok := stream.ReadGroup(groupName, consumerName, !sp.newOnly, sp.gt, count, noack, now)
			if !ok {
				return nil, errNoReadGroup(sp.key, groupName)
			}
			if !knownConsumer && stream.HasConsumer(groupName, consumerName) {
				ctx.notify(store.NotifyStream, "xgroup-createconsumer", sp.key)
			}
			if served {
				results = append(results, resp.ArrayValue([]*resp.Value{resp.BulkString(sp.key), streamItemsValue(items, false)}))
			}
		}
		return results, nil
	}

	results, err := serve()
	if err != nil {
		return ctx.WriteError(err)
	}
	if len(results) > 0 {
		return writeResults(results)
	}
	if timeout < 0 {
		return ctx.WriteNullArray()
	}

	ready := func(key string) bool {
		stream, err := lookupStream(ctx, key)
		if err != nil {
			return false
		}
		for _, sp := range specs {
			if sp.key != key {
				continue
			}
			if xreadgroup {
				if stream == nil {
					return true
				}
				hasNew, exists := stream.GroupHasNew(groupName)
				if !exists || hasNew {
					return true
				}
				continue
			}
			if stream == nil {
				continue
			}
			if last, ok := stream.LastEntryID(); ok && sp.gt.Less(last) {
				return true
			}
		}
		return false
	}

	if ctx.InExec {
		return ctx.WriteNullArray()
	}
	// A blocked XREADGROUP must fail once its key or group disappears, and
	// a deletion sends no write notification, so it rechecks periodically.
	const groupRecheck = 100 * time.Millisecond
	deadline := time.Now().Add(time.Duration(timeout) * time.Millisecond)
	for {
		wait := time.Duration(1) << 62
		if xreadgroup {
			wait = groupRecheck
		}
		if timeout > 0 {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return ctx.WriteNullArray()
			}
			if remaining < wait {
				wait = remaining
			}
		}
		if _, notified := ctx.waitForStreamKeys(keys, wait, ready); !notified {
			if streamClientGone(ctx) {
				return ctx.WriteNullArray()
			}
			if xreadgroup {
				for _, sp := range specs {
					stream, err := lookupStream(ctx, sp.key)
					if err != nil {
						return ctx.WriteError(err)
					}
					if stream == nil || !stream.HasGroup(groupName) {
						return ctx.WriteError(errNoReadGroup(sp.key, groupName))
					}
				}
			}
			continue
		}
		results, err := serve()
		if err != nil {
			return ctx.WriteError(err)
		}
		if len(results) > 0 {
			return writeResults(results)
		}
	}
}

func streamClientGone(ctx *Context) bool {
	if ctx.Done == nil {
		return false
	}
	select {
	case <-ctx.Done:
		return true
	default:
		return false
	}
}

func cmdXDEL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError("xdel"))
	}
	stream, err := lookupStream(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if stream == nil {
		return ctx.WriteInteger(0)
	}
	ids := make([]store.StreamID, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		id, err := parseStrictStreamID(ctx.ArgString(i))
		if err != nil {
			return ctx.WriteError(err)
		}
		ids = append(ids, id)
	}
	deleted := stream.DeleteIDs(ids)
	if deleted > 0 {
		ctx.notify(store.NotifyStream, "xdel", ctx.ArgString(0))
	}
	return ctx.WriteInteger(deleted)
}

func cmdXTRIM(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError("xtrim"))
	}
	a, _, err := parseAddTrimArgs(ctx, false)
	if err != nil {
		return ctx.WriteError(err)
	}
	stream, err := lookupStream(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if stream == nil {
		return ctx.WriteInteger(0)
	}
	trimmed := stream.TrimStream(a.trim)
	if trimmed > 0 {
		ctx.notify(store.NotifyStream, "xtrim", ctx.ArgString(0))
	}
	return ctx.WriteInteger(trimmed)
}

const xinfoHelp = `XINFO <subcommand> [<arg> [value] [opt] ...]. Subcommands are:
CONSUMERS <key> <groupname>
    Show consumers of <groupname>.
GROUPS <key>
    Show the stream consumer groups.
STREAM <key> [FULL [COUNT <count>]
    Show information about the stream.
HELP
    Print this help.`

const xgroupHelp = `XGROUP <subcommand> [<arg> [value] [opt] ...]. Subcommands are:
CREATE <key> <groupname> <id|$> [option]
    Create a new consumer group. Options are:
    * MKSTREAM
      Create the empty stream if it does not exist.
    * ENTRIESREAD entries_read
      Set the group's entries_read counter (internal use).
CREATECONSUMER <key> <groupname> <consumer>
    Create a new consumer in the specified group.
DELCONSUMER <key> <groupname> <consumer>
    Remove the specified consumer.
DESTROY <key> <groupname>
    Remove the specified group.
SETID <key> <groupname> <id|$> [ENTRIESREAD entries_read]
    Set the current group ID and entries_read counter.
HELP
    Print this help.`

func helpReply(text string) []*resp.Value {
	lines := strings.Split(text, "\n")
	out := make([]*resp.Value, 0, len(lines))
	for _, l := range lines {
		out = append(out, resp.SimpleString(l))
	}
	return out
}

// checkSubcommandArity enforces the Redis arity of a container subcommand;
// arity counts the command and subcommand names, negative is a minimum.
func checkSubcommandArity(ctx *Context, cmd, sub string, arity int) bool {
	n := ctx.ArgCount() + 1
	if (arity >= 0 && n != arity) || (arity < 0 && n < -arity) {
		_ = ctx.WriteError(errors.New("ERR wrong number of arguments for '" + cmd + "|" + strings.ToLower(sub) + "' command"))
		return false
	}
	return true
}

func cmdXINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(arityError("xinfo"))
	}
	sub := ctx.ArgString(0)
	arities := map[string]int{"STREAM": -3, "GROUPS": 3, "CONSUMERS": 4, "HELP": 2}
	arity, known := arities[strings.ToUpper(sub)]
	if !known {
		return ctx.WriteError(errUnknownSubcommand(sub, "XINFO"))
	}
	if !checkSubcommandArity(ctx, "xinfo", sub, arity) {
		return nil
	}
	if strings.EqualFold(sub, "HELP") {
		return ctx.WriteArray(helpReply(xinfoHelp))
	}
	key := ctx.ArgString(1)
	stream, err := lookupStream(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if stream == nil {
		return ctx.WriteError(errStreamNoSuchKey)
	}
	now := time.Now().UnixMilli()

	switch strings.ToUpper(sub) {
	case "CONSUMERS":
		group := ctx.ArgString(2)
		consumers, ok := stream.ConsumersInfo(group)
		if !ok {
			return ctx.WriteError(errNoConsumerGroup(group, key))
		}
		out := make([]*resp.Value, 0, len(consumers))
		for _, c := range consumers {
			idle := now - c.SeenTime
			if idle < 0 {
				idle = 0
			}
			inactive := int64(-1)
			if c.ActiveTime >= 0 {
				inactive = now - c.ActiveTime
				if inactive < 0 {
					inactive = 0
				}
			}
			out = append(out, resp.ArrayValue([]*resp.Value{
				resp.BulkString("name"), resp.BulkString(c.Name),
				resp.BulkString("pending"), resp.IntegerValue(c.Pending),
				resp.BulkString("idle"), resp.IntegerValue(idle),
				resp.BulkString("inactive"), resp.IntegerValue(inactive),
			}))
		}
		return ctx.WriteArray(out)

	case "GROUPS":
		groups := stream.GroupsInfo()
		out := make([]*resp.Value, 0, len(groups))
		for _, g := range groups {
			out = append(out, resp.ArrayValue([]*resp.Value{
				resp.BulkString("name"), resp.BulkString(g.Name),
				resp.BulkString("consumers"), resp.IntegerValue(g.Consumers),
				resp.BulkString("pending"), resp.IntegerValue(g.Pending),
				resp.BulkString("last-delivered-id"), resp.BulkString(g.LastID),
				resp.BulkString("entries-read"), entriesReadValue(g.EntriesRead),
				resp.BulkString("lag"), lagValue(g),
			}))
		}
		return ctx.WriteArray(out)
	}

	full := false
	count := int64(10)
	if ctx.ArgCount() > 2 {
		if !strings.EqualFold(ctx.ArgString(2), "FULL") {
			return ctx.WriteError(errSubcommandSyntax(sub, "XINFO"))
		}
		full = true
		if ctx.ArgCount() > 3 {
			if ctx.ArgCount() != 5 || !strings.EqualFold(ctx.ArgString(3), "COUNT") {
				return ctx.WriteError(errSubcommandSyntax(sub, "XINFO"))
			}
			n, ok := parseStreamLL(ctx.ArgString(4))
			if !ok {
				return ctx.WriteError(ErrNotInteger)
			}
			if n < 0 {
				n = 0
			}
			count = n
		}
	}
	info := stream.Info(full, count)
	nodes := info.Nodes
	radixNodes := int64(1)
	switch {
	case nodes == 1:
		radixNodes = 2
	case nodes > 1:
		radixNodes = int64(2*nodes + 2)
	}
	out := []*resp.Value{
		resp.BulkString("length"), resp.IntegerValue(info.Length),
		resp.BulkString("radix-tree-keys"), resp.IntegerValue(int64(nodes)),
		resp.BulkString("radix-tree-nodes"), resp.IntegerValue(radixNodes),
		resp.BulkString("last-generated-id"), resp.BulkString(info.LastID),
		resp.BulkString("max-deleted-entry-id"), resp.BulkString(info.MaxDeletedID),
		resp.BulkString("entries-added"), resp.IntegerValue(info.EntriesAdded),
		resp.BulkString("recorded-first-entry-id"), resp.BulkString(info.FirstID),
	}
	if !full {
		edge := func(e *store.StreamEntry) *resp.Value {
			if e == nil {
				return resp.NullBulkString()
			}
			return streamEntryValue(e)
		}
		out = append(out,
			resp.BulkString("groups"), resp.IntegerValue(int64(info.Groups)),
			resp.BulkString("first-entry"), edge(info.First),
			resp.BulkString("last-entry"), edge(info.Last),
		)
		return ctx.WriteArray(out)
	}
	groups := make([]*resp.Value, 0, len(info.GroupInfos))
	for _, g := range info.GroupInfos {
		pel := make([]*resp.Value, 0, len(g.PEL))
		for _, p := range g.PEL {
			pel = append(pel, resp.ArrayValue([]*resp.Value{
				resp.BulkString(p.ID), resp.BulkString(p.Consumer),
				resp.IntegerValue(p.DeliveryTS), resp.IntegerValue(p.Deliveries),
			}))
		}
		consumers := make([]*resp.Value, 0, len(g.ConsumerSet))
		for _, c := range g.ConsumerSet {
			cpel := make([]*resp.Value, 0, len(c.PEL))
			for _, p := range c.PEL {
				cpel = append(cpel, resp.ArrayValue([]*resp.Value{
					resp.BulkString(p.ID), resp.IntegerValue(p.DeliveryTS), resp.IntegerValue(p.Deliveries),
				}))
			}
			consumers = append(consumers, resp.ArrayValue([]*resp.Value{
				resp.BulkString("name"), resp.BulkString(c.Name),
				resp.BulkString("seen-time"), resp.IntegerValue(c.SeenTime),
				resp.BulkString("active-time"), resp.IntegerValue(c.ActiveTime),
				resp.BulkString("pel-count"), resp.IntegerValue(c.Pending),
				resp.BulkString("pending"), resp.ArrayValue(cpel),
			}))
		}
		groups = append(groups, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(g.Name),
			resp.BulkString("last-delivered-id"), resp.BulkString(g.LastID),
			resp.BulkString("entries-read"), entriesReadValue(g.EntriesRead),
			resp.BulkString("lag"), lagValue(g),
			resp.BulkString("pel-count"), resp.IntegerValue(g.Pending),
			resp.BulkString("pending"), resp.ArrayValue(pel),
			resp.BulkString("consumers"), resp.ArrayValue(consumers),
		}))
	}
	out = append(out,
		resp.BulkString("entries"), streamEntriesValue(info.Entries),
		resp.BulkString("groups"), resp.ArrayValue(groups),
	)
	return ctx.WriteArray(out)
}

func entriesReadValue(n int64) *resp.Value {
	if n == store.EntriesReadInvalid {
		return resp.NullBulkString()
	}
	return resp.IntegerValue(n)
}

func lagValue(g store.StreamGroupInfo) *resp.Value {
	if !g.LagValid {
		return resp.NullBulkString()
	}
	return resp.IntegerValue(g.Lag)
}

func cmdXGROUP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(arityError("xgroup"))
	}
	sub := ctx.ArgString(0)
	arities := map[string]int{"CREATE": -5, "SETID": -5, "DESTROY": 4, "CREATECONSUMER": 5, "DELCONSUMER": 5, "HELP": 2}
	upper := strings.ToUpper(sub)
	arity, known := arities[upper]
	if !known {
		return ctx.WriteError(errUnknownSubcommand(sub, "XGROUP"))
	}
	if !checkSubcommandArity(ctx, "xgroup", sub, arity) {
		return nil
	}
	if upper == "HELP" {
		return ctx.WriteArray(helpReply(xgroupHelp))
	}

	argc := ctx.ArgCount() + 1
	mkstream := false
	entriesRead := store.EntriesReadInvalid
	for i := 4; i < ctx.ArgCount(); {
		opt := ctx.ArgString(i)
		switch {
		case upper == "CREATE" && strings.EqualFold(opt, "MKSTREAM"):
			mkstream = true
			i++
		case (upper == "CREATE" || upper == "SETID") && strings.EqualFold(opt, "ENTRIESREAD") && i+1 < ctx.ArgCount():
			n, ok := parseStreamLL(ctx.ArgString(i + 1))
			if !ok {
				return ctx.WriteError(ErrNotInteger)
			}
			if n < 0 && n != store.EntriesReadInvalid {
				return ctx.WriteError(streamReplyError("ERR value for ENTRIESREAD must be positive or -1"))
			}
			entriesRead = n
			i += 2
		default:
			return ctx.WriteError(errSubcommandSyntax(sub, "XGROUP"))
		}
	}

	key := ctx.ArgString(1)
	groupName := ctx.ArgString(2)
	stream, err := lookupStream(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if stream == nil && !mkstream {
		return ctx.WriteError(errXGroupKeyRequired)
	}
	if stream != nil && (upper == "SETID" || upper == "CREATECONSUMER" || upper == "DELCONSUMER") && !stream.HasGroup(groupName) {
		return ctx.WriteError(errNoConsumerGroup(groupName, key))
	}

	switch upper {
	case "CREATE":
		if argc > 8 {
			return ctx.WriteError(errSubcommandSyntax(sub, "XGROUP"))
		}
		idArg := ctx.ArgString(3)
		var id store.StreamID
		useLast := idArg == "$"
		if !useLast {
			id, err = parseStrictStreamID(idArg)
			if err != nil {
				return ctx.WriteError(err)
			}
		}
		if stream == nil {
			v, _, err := ctx.Store.GetOrCreate(key, func() store.Value { return store.NewStreamValue(0) })
			if err != nil {
				return ctx.WriteError(err)
			}
			var ok bool
			if stream, ok = v.(*store.StreamValue); !ok {
				return ctx.WriteError(store.ErrWrongType)
			}
		}
		if !stream.CreateGroupAt(groupName, id, useLast, entriesRead) {
			return ctx.WriteError(ErrBusyGroup)
		}
		ctx.notify(store.NotifyStream, "xgroup-create", key)
		return ctx.WriteOK()

	case "SETID":
		if argc != 5 && argc != 7 {
			return ctx.WriteError(errSubcommandSyntax(sub, "XGROUP"))
		}
		idArg := ctx.ArgString(3)
		var id store.StreamID
		useLast := idArg == "$"
		if !useLast {
			id, err = store.ParseStreamRangeID(idArg, 0)
			if err != nil {
				return ctx.WriteError(err)
			}
		}
		if !stream.SetGroupID(groupName, id, useLast, entriesRead) {
			return ctx.WriteError(errNoConsumerGroup(groupName, key))
		}
		ctx.notify(store.NotifyStream, "xgroup-setid", key)
		return ctx.WriteOK()

	case "DESTROY":
		if stream.DestroyGroup(groupName) {
			ctx.notify(store.NotifyStream, "xgroup-destroy", key)
			ctx.Store.KeyNotifier().NotifyKey(key)
			return ctx.WriteInteger(1)
		}
		return ctx.WriteInteger(0)

	case "CREATECONSUMER":
		created, ok := stream.CreateConsumer(groupName, ctx.ArgString(3), time.Now().UnixMilli())
		if !ok {
			return ctx.WriteError(errNoConsumerGroup(groupName, key))
		}
		if created {
			ctx.notify(store.NotifyStream, "xgroup-createconsumer", key)
			return ctx.WriteInteger(1)
		}
		return ctx.WriteInteger(0)

	default:
		existed := stream.HasConsumer(groupName, ctx.ArgString(3))
		pending, ok := stream.DeleteConsumer(groupName, ctx.ArgString(3))
		if !ok {
			return ctx.WriteError(errNoConsumerGroup(groupName, key))
		}
		if existed {
			ctx.notify(store.NotifyStream, "xgroup-delconsumer", key)
		}
		return ctx.WriteInteger(pending)
	}
}

func cmdXACK(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError("xack"))
	}
	stream, err := lookupStream(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	groupName := ctx.ArgString(1)
	if stream == nil || !stream.HasGroup(groupName) {
		return ctx.WriteInteger(0)
	}
	ids := make([]store.StreamID, 0, ctx.ArgCount()-2)
	for i := 2; i < ctx.ArgCount(); i++ {
		id, err := parseStrictStreamID(ctx.ArgString(i))
		if err != nil {
			return ctx.WriteError(err)
		}
		ids = append(ids, id)
	}
	acked, _ := stream.AckIDs(groupName, ids)
	return ctx.WriteInteger(acked)
}

func cmdXPENDING(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError("xpending"))
	}
	argc := ctx.ArgCount() + 1
	justInfo := argc == 3
	if argc != 3 && (argc < 6 || argc > 9) {
		return ctx.WriteError(ErrSyntaxError)
	}
	key, groupName := ctx.ArgString(0), ctx.ArgString(1)

	var start, end store.StreamID
	var count, minIdle int64
	consumer := ""
	byConsumer := false
	if argc >= 6 {
		startIdx := 2
		if strings.EqualFold(ctx.ArgString(2), "IDLE") {
			n, ok := parseStreamLL(ctx.ArgString(3))
			if !ok {
				return ctx.WriteError(ErrNotInteger)
			}
			minIdle = n
			if argc < 8 {
				return ctx.WriteError(ErrSyntaxError)
			}
			startIdx += 2
		}
		n, ok := parseStreamLL(ctx.ArgString(startIdx + 2))
		if !ok {
			return ctx.WriteError(ErrNotInteger)
		}
		if n < 0 {
			n = 0
		}
		count = n
		var err error
		if start, err = store.ParseStreamInterval(ctx.ArgString(startIdx), false); err != nil {
			return ctx.WriteError(err)
		}
		if end, err = store.ParseStreamInterval(ctx.ArgString(startIdx+1), true); err != nil {
			return ctx.WriteError(err)
		}
		if startIdx+3 < ctx.ArgCount() {
			consumer = ctx.ArgString(startIdx + 3)
			byConsumer = true
		}
	}

	stream, err := lookupStream(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if stream == nil || !stream.HasGroup(groupName) {
		return ctx.WriteError(errNoGroupFor(key, groupName))
	}

	if justInfo {
		summary, ok := stream.PendingSummary(groupName)
		if !ok {
			return ctx.WriteError(errNoGroupFor(key, groupName))
		}
		if summary.Count == 0 {
			return ctx.WriteArray([]*resp.Value{
				resp.IntegerValue(0), resp.NullBulkString(), resp.NullBulkString(), resp.NullArray(),
			})
		}
		consumers := make([]*resp.Value, 0, len(summary.Consumers))
		for _, c := range summary.Consumers {
			consumers = append(consumers, resp.ArrayValue([]*resp.Value{
				resp.BulkString(c.Name), resp.BulkString(strconv.FormatInt(c.Count, 10)),
			}))
		}
		return ctx.WriteArray([]*resp.Value{
			resp.IntegerValue(summary.Count),
			resp.BulkString(summary.Min),
			resp.BulkString(summary.Max),
			resp.ArrayValue(consumers),
		})
	}

	now := time.Now().UnixMilli()
	rows, ok := stream.PendingRange(groupName, start, end, count, minIdle, consumer, byConsumer, now)
	if !ok {
		return ctx.WriteError(errNoGroupFor(key, groupName))
	}
	out := make([]*resp.Value, 0, len(rows))
	for _, r := range rows {
		idle := now - r.DeliveryTS
		if idle < 0 {
			idle = 0
		}
		out = append(out, resp.ArrayValue([]*resp.Value{
			resp.BulkString(r.ID), resp.BulkString(r.Consumer),
			resp.IntegerValue(idle), resp.IntegerValue(r.Deliveries),
		}))
	}
	return ctx.WriteArray(out)
}

func cmdXCLAIM(ctx *Context) error {
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(arityError("xclaim"))
	}
	key, groupName, consumerName := ctx.ArgString(0), ctx.ArgString(1), ctx.ArgString(2)
	stream, err := lookupStream(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if stream == nil || !stream.HasGroup(groupName) {
		return ctx.WriteError(errNoGroupFor(key, groupName))
	}
	minIdle, ok := parseStreamLL(ctx.ArgString(3))
	if !ok {
		return ctx.WriteError(streamReplyError("ERR Invalid min-idle-time argument for XCLAIM"))
	}
	if minIdle < 0 {
		minIdle = 0
	}

	j := 4
	ids := make([]store.StreamID, 0, ctx.ArgCount()-4)
	for ; j < ctx.ArgCount(); j++ {
		id, err := parseStrictStreamID(ctx.ArgString(j))
		if err != nil {
			break
		}
		ids = append(ids, id)
	}

	now := time.Now().UnixMilli()
	args := store.StreamClaimArgs{MinIdle: minIdle, DeliveryTS: -1, RetryCount: -1}
	for ; j < ctx.ArgCount(); j++ {
		moreargs := ctx.ArgCount() - 1 - j
		opt := ctx.ArgString(j)
		switch {
		case strings.EqualFold(opt, "FORCE"):
			args.Force = true
		case strings.EqualFold(opt, "JUSTID"):
			args.JustID = true
		case strings.EqualFold(opt, "IDLE") && moreargs > 0:
			j++
			n, ok := parseStreamLL(ctx.ArgString(j))
			if !ok {
				return ctx.WriteError(streamReplyError("ERR Invalid IDLE option argument for XCLAIM"))
			}
			args.DeliveryTS = now - n
		case strings.EqualFold(opt, "TIME") && moreargs > 0:
			j++
			n, ok := parseStreamLL(ctx.ArgString(j))
			if !ok {
				return ctx.WriteError(streamReplyError("ERR Invalid TIME option argument for XCLAIM"))
			}
			args.DeliveryTS = n
		case strings.EqualFold(opt, "RETRYCOUNT") && moreargs > 0:
			j++
			n, ok := parseStreamLL(ctx.ArgString(j))
			if !ok {
				return ctx.WriteError(streamReplyError("ERR Invalid RETRYCOUNT option argument for XCLAIM"))
			}
			args.RetryCount = n
		case strings.EqualFold(opt, "LASTID") && moreargs > 0:
			j++
			id, err := parseStrictStreamID(ctx.ArgString(j))
			if err != nil {
				return ctx.WriteError(err)
			}
			args.LastID = id
		default:
			return ctx.WriteError(streamReplyErrorf("ERR Unrecognized XCLAIM option '%s'", opt))
		}
	}
	if args.DeliveryTS == -1 {
		args.DeliveryTS = now
	}

	knownConsumer := !ctx.Store.Notifying(store.NotifyStream) || stream.HasConsumer(groupName, consumerName)
	items, ok := stream.ClaimIDs(groupName, consumerName, ids, args, now)
	if !ok {
		return ctx.WriteError(errNoGroupFor(key, groupName))
	}
	if !knownConsumer && stream.HasConsumer(groupName, consumerName) {
		ctx.notify(store.NotifyStream, "xgroup-createconsumer", key)
	}
	return ctx.WriteValue(streamItemsValue(items, args.JustID))
}

func cmdXAUTOCLAIM(ctx *Context) error {
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(arityError("xautoclaim"))
	}
	key, groupName, consumerName := ctx.ArgString(0), ctx.ArgString(1), ctx.ArgString(2)
	minIdle, ok := parseStreamLL(ctx.ArgString(3))
	if !ok {
		return ctx.WriteError(streamReplyError("ERR Invalid min-idle-time argument for XAUTOCLAIM"))
	}
	if minIdle < 0 {
		minIdle = 0
	}
	start, err := store.ParseStreamInterval(ctx.ArgString(4), false)
	if err != nil {
		return ctx.WriteError(err)
	}
	count := int64(100)
	justID := false
	for j := 5; j < ctx.ArgCount(); j++ {
		moreargs := ctx.ArgCount() - 1 - j
		opt := ctx.ArgString(j)
		switch {
		case strings.EqualFold(opt, "COUNT") && moreargs > 0:
			n, ok := parseStreamLL(ctx.ArgString(j + 1))
			if !ok || n < 1 || n > math.MaxInt64/16 {
				return ctx.WriteError(streamReplyError("ERR COUNT must be > 0"))
			}
			count = n
			j++
		case strings.EqualFold(opt, "JUSTID"):
			justID = true
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	stream, err := lookupStream(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if stream == nil || !stream.HasGroup(groupName) {
		return ctx.WriteError(errNoGroupFor(key, groupName))
	}
	knownConsumer := !ctx.Store.Notifying(store.NotifyStream) || stream.HasConsumer(groupName, consumerName)
	next, items, deleted, ok := stream.AutoClaim(groupName, consumerName, start, minIdle, count, justID, time.Now().UnixMilli())
	if !ok {
		return ctx.WriteError(errNoGroupFor(key, groupName))
	}
	if !knownConsumer && stream.HasConsumer(groupName, consumerName) {
		ctx.notify(store.NotifyStream, "xgroup-createconsumer", key)
	}
	deletedVals := make([]*resp.Value, 0, len(deleted))
	for _, id := range deleted {
		deletedVals = append(deletedVals, resp.BulkString(id.String()))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(next.String()),
		streamItemsValue(items, justID),
		resp.ArrayValue(deletedVals),
	})
}

func cmdXSETID(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError("xsetid"))
	}
	id, err := parseStrictStreamID(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	entriesAdded := int64(-1)
	var maxDeleted store.StreamID
	for i := 2; i < ctx.ArgCount(); {
		moreargs := ctx.ArgCount() - 1 - i
		opt := ctx.ArgString(i)
		switch {
		case strings.EqualFold(opt, "ENTRIESADDED") && moreargs > 0:
			n, ok := parseStreamLL(ctx.ArgString(i + 1))
			if !ok {
				return ctx.WriteError(ErrNotInteger)
			}
			if n < 0 {
				return ctx.WriteError(streamReplyError("ERR entries_added must be positive"))
			}
			entriesAdded = n
			i += 2
		case strings.EqualFold(opt, "MAXDELETEDID") && moreargs > 0:
			md, err := parseStrictStreamID(ctx.ArgString(i + 1))
			if err != nil {
				return ctx.WriteError(err)
			}
			if id.Less(md) {
				return ctx.WriteError(streamReplyError("ERR The ID specified in XSETID is smaller than the provided max_deleted_entry_id"))
			}
			maxDeleted = md
			i += 2
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}
	stream, err := lookupStream(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if stream == nil {
		return ctx.WriteError(errStreamNoSuchKey)
	}
	if err := stream.SetID(id, entriesAdded, maxDeleted); err != nil {
		return ctx.WriteError(err)
	}
	ctx.notify(store.NotifyStream, "xsetid", ctx.ArgString(0))
	return ctx.WriteOK()
}
