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
	router.Register(&CommandDef{Name: "XDELEX", Handler: cmdXDELEX})
	router.Register(&CommandDef{Name: "XACKDEL", Handler: cmdXACKDEL})
	router.Register(&CommandDef{Name: "XTRIM", Handler: cmdXTRIM})
	router.Register(&CommandDef{Name: "XINFO", Handler: cmdXINFO})
	router.Register(&CommandDef{Name: "XGROUP", Handler: cmdXGROUP})
	router.Register(&CommandDef{Name: "XREADGROUP", Handler: cmdXREADGROUP})
	router.Register(&CommandDef{Name: "XACK", Handler: cmdXACK})
	router.Register(&CommandDef{Name: "XNACK", Handler: cmdXNACK})
	router.Register(&CommandDef{Name: "XCFGSET", Handler: cmdXCFGSET})
	router.Register(&CommandDef{Name: "XIDMPRECORD", Handler: cmdXIDMPRECORD})
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
	return parseRedisInt([]byte(s))
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
	// idmpPID names the producer of XADD IDMP or IDMPAUTO; idmpIID is
	// IDMP's iid, computed from the fields for IDMPAUTO.
	idmpPID, idmpIID string
	idmpAuto         bool
}

// parseAddTrimArgs parses the options shared by XADD and XTRIM, starting
// after the key. For XADD it stops at the ID and returns its index.
func parseAddTrimArgs(ctx *Context, xadd bool) (addTrimArgs, int, error) {
	var a addTrimArgs
	argc := ctx.ArgCount()
	limitGiven, refsGiven := false, false
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
		case !refsGiven && streamRefsOption(opt) >= 0:
			a.trim.Refs, refsGiven = streamRefsOption(opt), true
		case xadd && strings.EqualFold(opt, "NOMKSTREAM"):
			a.noMkStream = true
		case xadd && strings.EqualFold(opt, "IDMPAUTO") && moreargs > 0:
			if a.idmpPID != "" {
				return a, 0, streamReplyError("ERR syntax error, IDMP/IDMPAUTO specified multiple times")
			}
			if ctx.ArgString(i+1) == "" {
				return a, 0, streamReplyError("ERR syntax error, IDMPAUTO requires a non-empty producer ID")
			}
			a.idmpPID, a.idmpAuto = ctx.ArgString(i+1), true
			i++
		case xadd && strings.EqualFold(opt, "IDMP") && moreargs >= 2:
			if a.idmpPID != "" {
				return a, 0, streamReplyError("ERR syntax error, IDMP/IDMPAUTO specified multiple times")
			}
			if ctx.ArgString(i+1) == "" {
				return a, 0, streamReplyError("ERR syntax error, IDMP requires a non-empty producer ID")
			}
			if ctx.ArgString(i+2) == "" {
				return a, 0, streamReplyError("ERR syntax error, IDMP requires a non-empty idempotent ID")
			}
			a.idmpPID, a.idmpIID = ctx.ArgString(i+1), ctx.ArgString(i+2)
			i += 2
		case xadd:
			id, err := parseXAddID(opt)
			if err != nil {
				return a, 0, err
			}
			// Replicas and the AOF get the ID XADD assigned.
			if a.idmpPID != "" && !ctx.FromMaster && !ctx.Replaying {
				return a, 0, streamReplyError("ERR syntax error, IDMP/IDMPAUTO can be used only with auto-generated IDs")
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
	if a.idmpPID != "" {
		return xaddIDMP(ctx, key, idIdx, stream, created, &a, pairs)
	}

	entry, trimmed, err := stream.XAddTrim(a.id, pairs, a.trim, uint64(ctx.nowMS()))
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
			if entry, trimmed, err = existing.XAddTrim(a.id, pairs, a.trim, uint64(ctx.nowMS())); err != nil {
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
	// The entries are written from the stream's nodes as they are read.
	w := ctx.Writer
	var id [41]byte
	stream.VisitRange(start, end, count, rev, w.WriteArrayHeader, func(e *store.StreamVisit) {
		writeStreamVisit(w, e, &id)
	})
	return w.Flush()
}

// writeStreamVisit writes an entry as [id, [field, value, ...]], formatting
// the ID in buf.
func writeStreamVisit(w *resp.Writer, e *store.StreamVisit, buf *[41]byte) {
	w.WriteArrayHeader(2)
	b := strconv.AppendUint(buf[:0], e.ID.Ms, 10)
	b = append(b, '-')
	w.WriteBulkItem(strconv.AppendUint(b, e.ID.Seq, 10))
	w.WriteArrayHeader(2 * e.FieldCount())
	e.EachField(func(name, value []byte) {
		w.WriteBulkItem(name)
		w.WriteBulkItem(value)
	})
}

// xreadResult is one stream's part of an XREAD reply: the entries after gt,
// or for XREADGROUP the items it served.
type xreadResult struct {
	key    string
	items  *resp.Value
	stream *store.StreamValue
	gt     store.StreamID
}

type xreadSpec struct {
	key     string
	gt      store.StreamID
	newOnly bool
}

// groupItemsValue renders XREADGROUP's entries: [id, fields], or with
// CLAIM [id, fields, idle, delivery count], 0 and 0 for a new entry.
func groupItemsValue(items []store.StreamGroupItem, claim bool) *resp.Value {
	if !claim {
		return streamItemsValue(items, false)
	}
	out := make([]*resp.Value, 0, len(items))
	for _, it := range items {
		e := streamEntryValue(it.Entry)
		e.Array = append(e.Array, resp.IntegerValue(it.Idle), resp.IntegerValue(it.Deliveries))
		out = append(out, e)
	}
	return resp.ArrayValue(out)
}

// readGroupEffects is what an XREADGROUP of one stream records, as Redis'
// streamPropagateXCLAIM, streamPropagateGroupID and
// streamPropagateConsumerCreation do: replicas cannot repeat the read, whose
// result depends on the time and on the order of consumers.
func readGroupEffects(ctx *Context, key, group, consumer string, now int64, noack bool, r *store.ReadGroupResult) []ExecutedCommand {
	var cmds []ExecutedCommand
	b := func(s string) []byte { return []byte(s) }
	nowArg := strconv.AppendInt(nil, now, 10)
	for _, it := range r.Items {
		if !it.Pending {
			continue
		}
		cmds = append(cmds, ExecutedCommand{Name: "XCLAIM", DB: ctx.DB, Args: [][]byte{
			b(key), b(group), b(consumer), b("0"), b(it.ID.String()), b("TIME"), nowArg,
			b("RETRYCOUNT"), strconv.AppendInt(nil, it.DeliveredCount, 10), b("FORCE"), b("JUSTID"),
			b("LASTID"), b(r.LastIDBefore),
		}})
	}
	if r.Advanced {
		cmds = append(cmds, ExecutedCommand{Name: "XGROUP", DB: ctx.DB, Args: [][]byte{
			b("SETID"), b(key), b(group), b(r.LastID), b("ENTRIESREAD"), strconv.AppendInt(nil, r.EntriesRead, 10),
		}})
	}
	if r.ConsumerCreated && (noack || len(cmds) == 0) {
		cmds = append(cmds, ExecutedCommand{Name: "XGROUP", DB: ctx.DB, Args: [][]byte{
			b("CREATECONSUMER"), b(key), b(group), b(consumer),
		}})
	}
	return cmds
}

// claimEffectCommands is how XCLAIM and XAUTOCLAIM reach replicas and the
// AOF, as Redis sends them: one XCLAIM per entry with its delivery time and
// count spelled out, so a replica or a later AOF load does not judge idle
// times again, and an XACK for a released entry deleted from the stream.
func claimEffectCommands(ctx *Context, key, group, lastID string, effects []store.StreamClaimEffect) []ExecutedCommand {
	cmds := make([]ExecutedCommand, 0, len(effects))
	for _, e := range effects {
		if e.Released {
			if e.Deleted {
				cmds = append(cmds, ExecutedCommand{Name: "XACK", DB: ctx.DB, Args: cmdArgs(key, group, e.ID.String())})
			}
			continue
		}
		cmds = append(cmds, ExecutedCommand{Name: "XCLAIM", DB: ctx.DB, Args: cmdArgs(
			key, group, e.Consumer, "0", e.ID.String(),
			"TIME", strconv.FormatInt(e.DeliveryTS, 10),
			"RETRYCOUNT", strconv.FormatInt(e.Deliveries, 10),
			"FORCE", "JUSTID", "LASTID", lastID)})
	}
	return cmds
}

func createConsumerCommand(ctx *Context, key, group, consumer string) ExecutedCommand {
	return ExecutedCommand{Name: "XGROUP", DB: ctx.DB, Args: cmdArgs("CREATECONSUMER", key, group, consumer)}
}

// budgetedRange is the entries XREAD returns from one stream under
// MAXCOUNT or MAXSIZE.
func budgetedRange(b *xreadBudget, stream *store.StreamValue, sp xreadSpec, count int64) *resp.Value {
	b.claim = false
	b.openStream(sp.key)
	start, _ := sp.gt.Next()
	entries := stream.Range(start, store.MaxStreamID, b.streamCount(count), false)
	n := 0
	for _, e := range entries {
		it := store.StreamGroupItem{ID: e.StreamID(), Entry: e}
		if !b.admit(&it) {
			break
		}
		n++
	}
	b.closeStream(n)
	return streamEntriesValue(entries[:n])
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
	minIdle := int64(-1)
	maxCount, maxSize := int64(0), int64(0)

parse:
	for i := 0; i < argc; i++ {
		moreargs := argc - i - 1
		o := ctx.ArgString(i)
		switch {
		case strings.EqualFold(o, "CLAIM") && moreargs > 0:
			if !xreadgroup {
				return ctx.WriteError(streamReplyError("ERR The CLAIM option is only supported by XREADGROUP. You called XREAD instead."))
			}
			i++
			n, ok := parseStreamLL(ctx.ArgString(i))
			if !ok {
				return ctx.WriteError(streamReplyError("ERR min-idle-time is not an integer or out of range"))
			}
			if n < 0 {
				return ctx.WriteError(streamReplyError("ERR min-idle-time must be a positive integer"))
			}
			minIdle = n
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
		case (strings.EqualFold(o, "MAXCOUNT") || strings.EqualFold(o, "MAXSIZE")) && moreargs > 0:
			i++
			n, ok := parseStreamLL(ctx.ArgString(i))
			if !ok {
				return ctx.WriteError(ErrNotInteger)
			}
			if n <= 0 {
				return ctx.WriteError(fmt.Errorf("ERR %s must be a positive integer", strings.ToUpper(o)))
			}
			if strings.EqualFold(o, "MAXCOUNT") {
				maxCount = n
			} else {
				maxSize = n
			}
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
	if maxCount > 0 && count > 0 && maxCount < count {
		return ctx.WriteError(streamReplyError("ERR MAXCOUNT must be greater than or equal to COUNT"))
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
	// XREAD's entries are written from the stream as they are read; the
	// gate keeps the streams as serve found them.
	writeResults := func(results []xreadResult) error {
		w := ctx.Writer
		resp3 := w.IsRESP3()
		if resp3 {
			w.WriteMapHeader(len(results))
		} else {
			w.WriteArrayHeader(len(results))
		}
		var id [41]byte
		for _, r := range results {
			if !resp3 {
				w.WriteArrayHeader(2)
			}
			w.WriteBulkStringItem(r.key)
			if r.items != nil {
				_ = w.WriteValueNoFlush(r.items)
				continue
			}
			start, _ := r.gt.Next()
			r.stream.VisitRange(start, store.MaxStreamID, count, false, w.WriteArrayHeader, func(e *store.StreamVisit) {
				writeStreamVisit(w, e, &id)
			})
		}
		return w.Flush()
	}

	// propagated is what XREADGROUP records for replicas and the AOF: an
	// XCLAIM per entry it delivered, XGROUP SETID when it moved the group's
	// last ID, XGROUP CREATECONSUMER for a consumer nothing else creates.
	var propagated []ExecutedCommand
	if xreadgroup {
		defer func() { ctx.PropagateAs(propagated...) }()
	}

	serve := func() ([]xreadResult, error) {
		results := make([]xreadResult, 0, len(specs))
		now := time.Now().UnixMilli()
		var budget *xreadBudget
		if maxCount > 0 || maxSize > 0 {
			budget = &xreadBudget{maxCount: maxCount, maxSize: maxSize, resp3: ctx.Writer.IsRESP3()}
		}
		for _, sp := range specs {
			if budget.limited() && budget.exhausted() {
				break
			}
			stream, err := lookupStream(ctx, sp.key)
			if err != nil {
				return nil, err
			}
			if !xreadgroup {
				if stream == nil {
					continue
				}
				if last, ok := stream.LastEntryID(); ok && sp.gt.Less(last) {
					if budget.limited() {
						results = append(results, xreadResult{key: sp.key, items: budgetedRange(budget, stream, sp, count)})
						continue
					}
					results = append(results, xreadResult{key: sp.key, stream: stream, gt: sp.gt})
				}
				continue
			}
			if stream == nil {
				return nil, errNoReadGroup(sp.key, groupName)
			}
			claim := minIdle
			if !sp.newOnly {
				// CLAIM is ignored when the consumer reads its history.
				claim = -1
			}
			opts := store.ReadGroupOptions{
				History: !sp.newOnly, After: sp.gt, Count: count, NoAck: noack, MinIdle: claim, Now: now,
			}
			opened := false
			if budget.limited() {
				budget.claim = claim >= 0
				opts.Count = budget.streamCount(count)
				if !sp.newOnly {
					budget.openStream(sp.key)
					opened = true
				}
				opts.Admit = func(it *store.StreamGroupItem) bool {
					if !opened {
						budget.openStream(sp.key)
						opened = true
					}
					return budget.admit(it)
				}
			}
			r, ok := stream.ReadGroupWith(groupName, consumerName, opts)
			if !ok {
				return nil, errNoReadGroup(sp.key, groupName)
			}
			if r.ConsumerCreated {
				ctx.notify(store.NotifyStream, "xgroup-createconsumer", sp.key)
			}
			propagated = append(propagated, readGroupEffects(ctx, sp.key, groupName, consumerName, now, noack, &r)...)
			if opened {
				budget.closeStream(len(r.Items))
			}
			if r.Served || opened {
				results = append(results, xreadResult{key: sp.key, items: groupItemsValue(r.Items, claim >= 0)})
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
				if minIdle >= 0 && sp.newOnly {
					if at, ok := stream.ClaimableAt(groupName, minIdle); ok && at <= time.Now().UnixMilli() {
						return true
					}
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
		claimDue := false
		if xreadgroup && minIdle >= 0 {
			// Wake when a pending entry becomes claimable, or after
			// min-idle-time when nothing is pending, as Redis does.
			at := time.Now().UnixMilli() + minIdle
			for _, sp := range specs {
				if stream, err := lookupStream(ctx, sp.key); err == nil && stream != nil && sp.newOnly {
					if t, ok := stream.ClaimableAt(groupName, minIdle); ok && t < at {
						at = t
					}
				}
			}
			if d := time.Until(time.UnixMilli(at)); d < wait {
				wait, claimDue = max(d, 0), true
			}
		}
		if len(propagated) > 0 {
			// The consumer this read created, as Redis propagates it
			// when the command blocks.
			if err := ctx.recordNow(propagated); err != nil {
				return ctx.WriteError(err)
			}
			propagated = nil
		}
		if _, notified := ctx.waitForStreamKeys(keys, wait, ready); !notified && claimDue {
			results, err := serve()
			if err != nil {
				return ctx.WriteError(err)
			}
			if len(results) > 0 {
				return writeResults(results)
			}
			continue
		} else if !notified {
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
		ctx.PropagateAs()
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
	} else {
		ctx.PropagateAs()
	}
	return ctx.WriteInteger(deleted)
}

// streamRefsOption returns the reference strategy opt names, KEEPREF,
// DELREF or ACKED, or -1.
func streamRefsOption(opt string) int {
	switch {
	case strings.EqualFold(opt, "KEEPREF"):
		return store.StreamKeepRef
	case strings.EqualFold(opt, "DELREF"):
		return store.StreamDelRef
	case strings.EqualFold(opt, "ACKED"):
		return store.StreamAcked
	}
	return -1
}

// parseAckDelArgs parses the options of XDELEX and XACKDEL from index start:
// a reference strategy and IDS numids id..., in any order. It returns the
// strategy and the IDs' arguments, unparsed.
func parseAckDelArgs(ctx *Context, start int) (refs int, ids [][]byte, err error) {
	argc := ctx.ArgCount()
	refs = -1
	first := -1
	n := 0
	for j := start; j < argc; {
		opt := ctx.ArgString(j)
		switch {
		case refs < 0 && streamRefsOption(opt) >= 0:
			refs = streamRefsOption(opt)
			j++
		case strings.EqualFold(opt, "IDS") && j+1 < argc:
			v, ok := parseStreamLL(ctx.ArgString(j + 1))
			if !ok || v < 1 {
				return 0, nil, streamReplyError("ERR Number of IDs must be a positive integer")
			}
			if v > int64(argc-j-2) {
				return 0, nil, streamReplyError("ERR The `numids` parameter must match the number of arguments")
			}
			first, n = j+2, int(v)
			j = first + n
		default:
			return 0, nil, ErrSyntaxError
		}
	}
	if first < 0 {
		return 0, nil, streamReplyError("ERR IDS option is required")
	}
	if refs < 0 {
		refs = store.StreamKeepRef
	}
	return refs, ctx.Args[first : first+n], nil
}

// streamDelReplies writes the replies of XDELEX or XACKDEL.
func streamDelReplies(ctx *Context, replies []int64) error {
	out := make([]*resp.Value, len(replies))
	for i, r := range replies {
		out[i] = resp.IntegerValue(r)
	}
	return ctx.WriteArray(out)
}

func parseStreamIDArgs(args [][]byte) ([]store.StreamID, error) {
	ids := make([]store.StreamID, len(args))
	for i, a := range args {
		id, err := parseStrictStreamID(string(a))
		if err != nil {
			return nil, err
		}
		ids[i] = id
	}
	return ids, nil
}

// cmdXDELEX is XDEL with Redis 8.2's reference strategies, replying for
// each ID: -1 missing, 1 deleted, 2 kept since a group still needs it.
func cmdXDELEX(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(arityError("xdelex"))
	}
	key := ctx.ArgString(0)
	stream, err := lookupStream(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	refs, idArgs, err := parseAckDelArgs(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	if stream == nil {
		ctx.PropagateAs()
		return streamDelReplies(ctx, missingStreamReplies(len(idArgs)))
	}
	ids, err := parseStreamIDArgs(idArgs)
	if err != nil {
		return ctx.WriteError(err)
	}
	replies, deleted, changed := stream.DeleteWithRefs(ids, refs)
	if deleted > 0 {
		ctx.notify(store.NotifyStream, "xdel", key)
	}
	if !changed {
		ctx.PropagateAs()
	}
	return streamDelReplies(ctx, replies)
}

// cmdXACKDEL acknowledges IDs in a group and deletes them under Redis 8.2's
// reference strategies, replying for each ID: -1 not pending in the group,
// 1 acknowledged and deleted, 2 acknowledged but kept for another group.
func cmdXACKDEL(ctx *Context) error {
	if ctx.ArgCount() < 5 {
		return ctx.WriteError(arityError("xackdel"))
	}
	key := ctx.ArgString(0)
	stream, err := lookupStream(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	refs, idArgs, err := parseAckDelArgs(ctx, 2)
	if err != nil {
		return ctx.WriteError(err)
	}
	group := ctx.ArgString(1)
	if stream == nil || !stream.HasGroup(group) {
		ctx.PropagateAs()
		return streamDelReplies(ctx, missingStreamReplies(len(idArgs)))
	}
	ids, err := parseStreamIDArgs(idArgs)
	if err != nil {
		return ctx.WriteError(err)
	}
	replies, deleted, changed, ok := stream.AckDelete(group, ids, refs)
	if !ok {
		replies = missingStreamReplies(len(ids))
	}
	if deleted > 0 {
		ctx.notify(store.NotifyStream, "xdel", key)
	}
	if !changed {
		ctx.PropagateAs()
	}
	return streamDelReplies(ctx, replies)
}

func missingStreamReplies(n int) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = store.StreamDelNoID
	}
	return out
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
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	trimmed := stream.TrimStream(a.trim)
	if trimmed > 0 {
		ctx.notify(store.NotifyStream, "xtrim", ctx.ArgString(0))
	} else {
		ctx.PropagateAs()
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
			out = append(out, resp.PairsValue([]*resp.Value{
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
		for i := range groups {
			g := &groups[i]
			out = append(out, resp.PairsValue([]*resp.Value{
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
	radixNodes := int64(info.RadixNodes)
	out := []*resp.Value{
		resp.BulkString("length"), resp.IntegerValue(info.Length),
		resp.BulkString("radix-tree-keys"), resp.IntegerValue(int64(nodes)),
		resp.BulkString("radix-tree-nodes"), resp.IntegerValue(radixNodes),
		resp.BulkString("last-generated-id"), resp.BulkString(info.LastID),
		resp.BulkString("max-deleted-entry-id"), resp.BulkString(info.MaxDeletedID),
		resp.BulkString("entries-added"), resp.IntegerValue(info.EntriesAdded),
		resp.BulkString("recorded-first-entry-id"), resp.BulkString(info.FirstID),
	}
	out = append(out, idmpInfoFields(stream)...)
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
		return ctx.WriteValue(resp.PairsValue(out))
	}
	groups := make([]*resp.Value, 0, len(info.GroupInfos))
	for i := range info.GroupInfos {
		g := &info.GroupInfos[i]
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
			consumers = append(consumers, resp.PairsValue([]*resp.Value{
				resp.BulkString("name"), resp.BulkString(c.Name),
				resp.BulkString("seen-time"), resp.IntegerValue(c.SeenTime),
				resp.BulkString("active-time"), resp.IntegerValue(c.ActiveTime),
				resp.BulkString("pel-count"), resp.IntegerValue(c.Pending),
				resp.BulkString("pending"), resp.ArrayValue(cpel),
			}))
		}
		group := []*resp.Value{
			resp.BulkString("name"), resp.BulkString(g.Name),
			resp.BulkString("last-delivered-id"), resp.BulkString(g.LastID),
			resp.BulkString("entries-read"), entriesReadValue(g.EntriesRead),
			resp.BulkString("lag"), lagValue(g),
			resp.BulkString("pel-count"), resp.IntegerValue(g.Pending),
		}
		if g.Nacked > 0 {
			// Redis 8.8's field, shown only while XNACK has released
			// entries, for the same reason as the IDMP fields.
			group = append(group, resp.BulkString("nacked-count"), resp.IntegerValue(g.Nacked))
		}
		groups = append(groups, resp.PairsValue(append(group,
			resp.BulkString("pending"), resp.ArrayValue(pel),
			resp.BulkString("consumers"), resp.ArrayValue(consumers),
		)))
	}
	out = append(out,
		resp.BulkString("entries"), streamEntriesValue(info.Entries),
		resp.BulkString("groups"), resp.ArrayValue(groups),
	)
	return ctx.WriteValue(resp.PairsValue(out))
}

func entriesReadValue(n int64) *resp.Value {
	if n == store.EntriesReadInvalid {
		return resp.NullBulkString()
	}
	return resp.IntegerValue(n)
}

func lagValue(g *store.StreamGroupInfo) *resp.Value {
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
		ctx.PropagateAs()
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
		ctx.PropagateAs()
		return ctx.WriteInteger(0)

	default:
		existed := stream.HasConsumer(groupName, ctx.ArgString(3))
		pending, ok := stream.DeleteConsumer(groupName, ctx.ArgString(3))
		if !ok {
			return ctx.WriteError(errNoConsumerGroup(groupName, key))
		}
		if existed {
			ctx.notify(store.NotifyStream, "xgroup-delconsumer", key)
		} else {
			ctx.PropagateAs()
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
		ctx.PropagateAs()
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
	if acked == 0 {
		ctx.PropagateAs()
	}
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
		if r.IsNacked() {
			idle = -1
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
	args.Restore = ctx.Replaying || ctx.LegacyReplay || ctx.FromMaster

	knownConsumer := !ctx.Store.Notifying(store.NotifyStream) || stream.HasConsumer(groupName, consumerName)
	r, ok := stream.ClaimIDs(groupName, consumerName, ids, args, now)
	if !ok {
		return ctx.WriteError(errNoGroupFor(key, groupName))
	}
	if !knownConsumer && stream.HasConsumer(groupName, consumerName) {
		ctx.notify(store.NotifyStream, "xgroup-createconsumer", key)
	}
	cmds := claimEffectCommands(ctx, key, groupName, r.LastID, r.Effects)
	if r.LastIDMoved && len(cmds) == 0 {
		cmds = append(cmds, ExecutedCommand{Name: "XGROUP", DB: ctx.DB, Args: cmdArgs(
			"SETID", key, groupName, r.LastID, "ENTRIESREAD", strconv.FormatInt(r.EntriesRead, 10))})
	}
	if r.ConsumerCreated && len(r.Items) == 0 {
		cmds = append([]ExecutedCommand{createConsumerCommand(ctx, key, groupName, consumerName)}, cmds...)
	}
	ctx.PropagateAs(cmds...)
	return ctx.WriteValue(streamItemsValue(r.Items, args.JustID))
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
	r, ok := stream.AutoClaim(groupName, consumerName, start, minIdle, count, justID, time.Now().UnixMilli())
	if !ok {
		return ctx.WriteError(errNoGroupFor(key, groupName))
	}
	if !knownConsumer && stream.HasConsumer(groupName, consumerName) {
		ctx.notify(store.NotifyStream, "xgroup-createconsumer", key)
	}
	cmds := claimEffectCommands(ctx, key, groupName, r.LastID, r.Effects)
	if r.ConsumerCreated && len(r.Items) == 0 {
		cmds = append([]ExecutedCommand{createConsumerCommand(ctx, key, groupName, consumerName)}, cmds...)
	}
	ctx.PropagateAs(cmds...)
	deletedVals := make([]*resp.Value, 0, len(r.Deleted))
	for _, id := range r.Deleted {
		deletedVals = append(deletedVals, resp.BulkString(id.String()))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(r.Next.String()),
		streamItemsValue(r.Items, justID),
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
