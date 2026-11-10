package command

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterEventCommands(router *Router) {
	router.Register(&CommandDef{Name: "EVENT.EMIT", Handler: cmdEVENTEMIT})
	router.Register(&CommandDef{Name: "EVENT.GET", Handler: cmdEVENTGET})
	router.Register(&CommandDef{Name: "EVENT.LIST", Handler: cmdEVENTLIST})
	router.Register(&CommandDef{Name: "EVENT.CLEAR", Handler: cmdEVENTCLEAR})

	router.Register(&CommandDef{Name: "COMPRESS.RLE", Handler: cmdCOMPRESSRLE})
	router.Register(&CommandDef{Name: "DECOMPRESS.RLE", Handler: cmdDECOMPRESSRLE})
	router.Register(&CommandDef{Name: "COMPRESS.LZ4", Handler: cmdCOMPRESSLZ4})
	router.Register(&CommandDef{Name: "DECOMPRESS.LZ4", Handler: cmdDECOMPRESSLZ4})
	router.Register(&CommandDef{Name: "COMPRESS.CUSTOM", Handler: cmdCOMPRESSCUSTOM})

	router.Register(&CommandDef{Name: "QUEUE.CREATE", Handler: cmdQUEUECREATE})
	router.Register(&CommandDef{Name: "QUEUE.PUSH", Handler: cmdQUEUEPUSH})
	router.Register(&CommandDef{Name: "QUEUE.POP", Handler: cmdQUEUEPOP})
	router.Register(&CommandDef{Name: "QUEUE.PEEK", Handler: cmdQUEUEPEEK})
	router.Register(&CommandDef{Name: "QUEUE.LEN", Handler: cmdQUEUELEN})
	router.Register(&CommandDef{Name: "QUEUE.CLEAR", Handler: cmdQUEUECLEAR})

	router.Register(&CommandDef{Name: "STACK.CREATE", Handler: cmdSTACKCREATE})
	router.Register(&CommandDef{Name: "STACK.PUSH", Handler: cmdSTACKPUSH})
	router.Register(&CommandDef{Name: "STACK.POP", Handler: cmdSTACKPOP})
	router.Register(&CommandDef{Name: "STACK.PEEK", Handler: cmdSTACKPEEK})
	router.Register(&CommandDef{Name: "STACK.LEN", Handler: cmdSTACKLEN})
	router.Register(&CommandDef{Name: "STACK.CLEAR", Handler: cmdSTACKCLEAR})
}

func cmdEVENTEMIT(ctx *Context) error {
	if ctx.ArgCount() < 1 || (ctx.ArgCount()-1)%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	data := make(map[string]interface{})

	for i := 1; i+1 < ctx.ArgCount(); i += 2 {
		data[ctx.ArgString(i)] = ctx.ArgString(i + 1)
	}

	event := store.GlobalEventManager.Emit(name, data)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"),
		resp.BulkString(event.ID),
		resp.BulkString("name"),
		resp.BulkString(event.Name),
		resp.BulkString("timestamp"),
		resp.IntegerValue(event.Timestamp),
	})
}

func eventReply(e *store.Event) *resp.Value {
	keys := make([]string, 0, len(e.Data))
	for k := range e.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	data := make([]*resp.Value, 0, 2*len(keys))
	for _, k := range keys {
		data = append(data, resp.BulkString(k), resp.BulkString(fmt.Sprint(e.Data[k])))
	}
	return resp.ArrayValue([]*resp.Value{
		resp.BulkString("id"),
		resp.BulkString(e.ID),
		resp.BulkString("name"),
		resp.BulkString(e.Name),
		resp.BulkString("timestamp"),
		resp.IntegerValue(e.Timestamp),
		resp.BulkString("data"),
		resp.ArrayValue(data),
	})
}

func cmdEVENTGET(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	limit := 10
	if ctx.ArgCount() == 2 {
		v, err := strconv.Atoi(ctx.ArgString(1))
		if err != nil || v < 0 {
			return ctx.WriteError(ErrNotInteger)
		}
		limit = v
	}

	events := store.GlobalEventManager.GetEvents(name, limit)

	results := make([]*resp.Value, 0, len(events))
	for _, e := range events {
		results = append(results, eventReply(e))
	}

	return ctx.WriteArray(results)
}

func cmdEVENTLIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	events := store.GlobalEventManager.GetEvents("", 100)

	results := make([]*resp.Value, 0, len(events))
	for _, e := range events {
		results = append(results, eventReply(e))
	}

	return ctx.WriteArray(results)
}

func cmdEVENTCLEAR(ctx *Context) error {
	store.GlobalEventManager.Clear()
	return ctx.WriteOK()
}

func cmdCOMPRESSRLE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.Arg(0)
	compressor := &store.RLECompressor{}

	compressed, err := compressor.Compress(data)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteBulkBytes(compressed)
}

func cmdDECOMPRESSRLE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.Arg(0)
	compressor := &store.RLECompressor{}

	decompressed, err := compressor.Decompress(data)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteBulkBytes(decompressed)
}

func cmdCOMPRESSLZ4(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.Arg(0)
	compressor := &store.LZ4Compressor{}

	compressed, err := compressor.Compress(data)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteBulkBytes(compressed)
}

func cmdDECOMPRESSLZ4(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.Arg(0)
	compressor := &store.LZ4Compressor{}

	decompressed, err := compressor.Decompress(data)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteBulkBytes(decompressed)
}

func cmdCOMPRESSCUSTOM(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	algo := strings.ToUpper(ctx.ArgString(0))
	data := ctx.Arg(1)

	var compressor store.Compressor

	switch algo {
	case "RLE":
		compressor = &store.RLECompressor{}
	case "LZ4":
		compressor = &store.LZ4Compressor{}
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown compression algorithm: %s", algo))
	}

	compressed, err := compressor.Compress(data)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteBulkBytes(compressed)
}

// QUEUE and STACK are lists in the keyspace: a queue pushes at the tail and
// pops at the head, a stack pushes and pops at the tail. Being keys, they are
// per database, saved in the RDB and the AOF, replicated, cleared by
// FLUSHDB and FLUSHALL, and watched by WATCH; LRANGE and the other list
// commands read them. QUEUE.CREATE and STACK.CREATE only check that the name
// holds no other type, since a list exists while it has elements.

func cmdQUEUECREATE(ctx *Context) error { return listCollectionCreate(ctx) }
func cmdQUEUEPUSH(ctx *Context) error   { return listPush(ctx, false) }
func cmdQUEUEPOP(ctx *Context) error    { return listCollectionPop(ctx, true) }
func cmdQUEUEPEEK(ctx *Context) error   { return listCollectionPeek(ctx, "0") }
func cmdQUEUELEN(ctx *Context) error    { return cmdLLEN(ctx) }
func cmdQUEUECLEAR(ctx *Context) error  { return listCollectionClear(ctx) }
func cmdSTACKCREATE(ctx *Context) error { return listCollectionCreate(ctx) }
func cmdSTACKPUSH(ctx *Context) error   { return listPush(ctx, false) }
func cmdSTACKPOP(ctx *Context) error    { return listCollectionPop(ctx, false) }
func cmdSTACKPEEK(ctx *Context) error   { return listCollectionPeek(ctx, "-1") }
func cmdSTACKLEN(ctx *Context) error    { return cmdLLEN(ctx) }
func cmdSTACKCLEAR(ctx *Context) error  { return listCollectionClear(ctx) }

func listCollectionCreate(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	if _, err := getList(ctx, ctx.ArgString(0)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func listCollectionPop(ctx *Context, head bool) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	return listPopGeneric(ctx, head)
}

func listCollectionPeek(ctx *Context, index string) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	args := ctx.Args
	ctx.Args = [][]byte{args[0], []byte(index)}
	defer func() { ctx.Args = args }()
	return cmdLINDEX(ctx)
}

func listCollectionClear(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	if _, err := getList(ctx, key); err != nil {
		return ctx.WriteError(err)
	}
	if ctx.Store.Delete(key) {
		ctx.notify(store.NotifyGeneric, "del", key)
	}
	return ctx.WriteOK()
}
