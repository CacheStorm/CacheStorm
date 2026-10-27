package command

import (
	"fmt"
	"strings"
	"sync"

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
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	data := make(map[string]interface{})

	for i := 1; i+1 < ctx.ArgCount(); i += 2 {
		key := ctx.ArgString(i)
		val := ctx.ArgString(i + 1)
		data[key] = val
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

func cmdEVENTGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	limit := 10
	if ctx.ArgCount() >= 2 {
		limit = int(parseInt64(ctx.ArgString(1)))
	}

	events := store.GlobalEventManager.GetEvents(name, limit)

	results := make([]*resp.Value, 0)
	for _, e := range events {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString(e.ID),
			resp.BulkString(e.Name),
			resp.IntegerValue(e.Timestamp),
		}))
	}

	return ctx.WriteArray(results)
}

func cmdEVENTLIST(ctx *Context) error {
	events := store.GlobalEventManager.GetEvents("", 100)

	results := make([]*resp.Value, 0)
	for _, e := range events {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString(e.ID),
			resp.BulkString(e.Name),
			resp.IntegerValue(e.Timestamp),
		}))
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

var (
	queues   = make(map[string][]string)
	queuesMu sync.RWMutex
	stacks   = make(map[string][]string)
	stacksMu sync.RWMutex
)

func cmdQUEUECREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	queuesMu.Lock()
	queues[name] = make([]string, 0)
	queuesMu.Unlock()

	return ctx.WriteOK()
}

func cmdQUEUEPUSH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	value := ctx.ArgString(1)

	queuesMu.Lock()
	if _, exists := queues[name]; !exists {
		queues[name] = make([]string, 0)
	}
	queues[name] = append(queues[name], value)
	length := len(queues[name])
	queuesMu.Unlock()

	return ctx.WriteInteger(int64(length))
}

func cmdQUEUEPOP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	queuesMu.Lock()
	defer queuesMu.Unlock()

	q, exists := queues[name]
	if !exists || len(q) == 0 {
		return ctx.WriteNull()
	}

	value := q[0]
	queues[name] = q[1:]

	return ctx.WriteBulkString(value)
}

func cmdQUEUEPEEK(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	queuesMu.RLock()

	q, exists := queues[name]
	if !exists || len(q) == 0 {
		queuesMu.RUnlock()
		return ctx.WriteNull()
	}

	reply := q[0]
	queuesMu.RUnlock()
	return ctx.WriteBulkString(reply)
}

func cmdQUEUELEN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	queuesMu.RLock()

	reply := int64(int64(len(queues[name])))
	queuesMu.RUnlock()
	return ctx.WriteInteger(reply)
}

func cmdQUEUECLEAR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	queuesMu.Lock()
	queues[name] = make([]string, 0)
	queuesMu.Unlock()

	return ctx.WriteOK()
}

func cmdSTACKCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	stacksMu.Lock()
	stacks[name] = make([]string, 0)
	stacksMu.Unlock()

	return ctx.WriteOK()
}

func cmdSTACKPUSH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	value := ctx.ArgString(1)

	stacksMu.Lock()
	if _, exists := stacks[name]; !exists {
		stacks[name] = make([]string, 0)
	}
	stacks[name] = append(stacks[name], value)
	length := len(stacks[name])
	stacksMu.Unlock()

	return ctx.WriteInteger(int64(length))
}

func cmdSTACKPOP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	stacksMu.Lock()
	defer stacksMu.Unlock()

	s, exists := stacks[name]
	if !exists || len(s) == 0 {
		return ctx.WriteNull()
	}

	value := s[len(s)-1]
	stacks[name] = s[:len(s)-1]

	return ctx.WriteBulkString(value)
}

func cmdSTACKPEEK(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	stacksMu.RLock()

	s, exists := stacks[name]
	if !exists || len(s) == 0 {
		stacksMu.RUnlock()
		return ctx.WriteNull()
	}

	reply := s[len(s)-1]
	stacksMu.RUnlock()
	return ctx.WriteBulkString(reply)
}

func cmdSTACKLEN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	stacksMu.RLock()

	reply := int64(int64(len(stacks[name])))
	stacksMu.RUnlock()
	return ctx.WriteInteger(reply)
}

func cmdSTACKCLEAR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	stacksMu.Lock()
	stacks[name] = make([]string, 0)
	stacksMu.Unlock()

	return ctx.WriteOK()
}
