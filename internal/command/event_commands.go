package command

import (
	"fmt"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterEventCommands(router *Router) {
	router.Register(&CommandDef{Name: "EVENT.EMIT", Handler: cmdEVENTEMIT})
	router.Register(&CommandDef{Name: "EVENT.GET", Handler: cmdEVENTGET})
	router.Register(&CommandDef{Name: "EVENT.LIST", Handler: cmdEVENTLIST})
	router.Register(&CommandDef{Name: "EVENT.CLEAR", Handler: cmdEVENTCLEAR})

	router.Register(&CommandDef{Name: "WEBHOOK.CREATE", Handler: cmdWEBHOOKCREATE})
	router.Register(&CommandDef{Name: "WEBHOOK.DELETE", Handler: cmdWEBHOOKDELETE})
	router.Register(&CommandDef{Name: "WEBHOOK.GET", Handler: cmdWEBHOOKGET})
	router.Register(&CommandDef{Name: "WEBHOOK.LIST", Handler: cmdWEBHOOKLIST})
	router.Register(&CommandDef{Name: "WEBHOOK.ENABLE", Handler: cmdWEBHOOKENABLE})
	router.Register(&CommandDef{Name: "WEBHOOK.DISABLE", Handler: cmdWEBHOOKDISABLE})
	router.Register(&CommandDef{Name: "WEBHOOK.STATS", Handler: cmdWEBHOOKSTATS})

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
	store.GlobalEventManager.Events = make([]*store.Event, 0)
	return ctx.WriteOK()
}

func cmdWEBHOOKCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)
	url := ctx.ArgString(1)
	method := strings.ToUpper(ctx.ArgString(2))
	if method == "" {
		method = "POST"
	}

	events := make([]string, 0)
	for i := 3; i < ctx.ArgCount(); i++ {
		events = append(events, ctx.ArgString(i))
	}

	wh := store.GlobalEventManager.CreateWebhook(id, url, method, events)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"),
		resp.BulkString(wh.ID),
		resp.BulkString("url"),
		resp.BulkString(wh.URL),
		resp.BulkString("method"),
		resp.BulkString(wh.Method),
	})
}

func cmdWEBHOOKDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	if store.GlobalEventManager.DeleteWebhook(id) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdWEBHOOKGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	wh, ok := store.GlobalEventManager.GetWebhook(id)
	if !ok {
		return ctx.WriteNull()
	}

	events := make([]*resp.Value, len(wh.Events))
	for i, e := range wh.Events {
		events[i] = resp.BulkString(e)
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"),
		resp.BulkString(wh.ID),
		resp.BulkString("url"),
		resp.BulkString(wh.URL),
		resp.BulkString("method"),
		resp.BulkString(wh.Method),
		resp.BulkString("events"),
		resp.ArrayValue(events),
		resp.BulkString("enabled"),
		resp.BulkString(fmt.Sprintf("%v", wh.Enabled)),
	})
}

func cmdWEBHOOKLIST(ctx *Context) error {
	webhooks := store.GlobalEventManager.ListWebhooks()

	results := make([]*resp.Value, 0)
	for _, wh := range webhooks {
		results = append(results,
			resp.BulkString(wh.ID),
			resp.BulkString(wh.URL),
			resp.BulkString(fmt.Sprintf("%v", wh.Enabled)),
		)
	}

	return ctx.WriteArray(results)
}

func cmdWEBHOOKENABLE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	if store.GlobalEventManager.EnableWebhook(id) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdWEBHOOKDISABLE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	if store.GlobalEventManager.DisableWebhook(id) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdWEBHOOKSTATS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	wh, ok := store.GlobalEventManager.GetWebhook(id)
	if !ok {
		return ctx.WriteNull()
	}

	stats := wh.Stats()

	results := make([]*resp.Value, 0)
	for k, v := range stats {
		results = append(results, resp.BulkString(k))
		switch val := v.(type) {
		case string:
			results = append(results, resp.BulkString(val))
		case int64:
			results = append(results, resp.IntegerValue(val))
		case []string:
			arr := make([]*resp.Value, len(val))
			for i, s := range val {
				arr[i] = resp.BulkString(s)
			}
			results = append(results, resp.ArrayValue(arr))
		case bool:
			if val {
				results = append(results, resp.BulkString("true"))
			} else {
				results = append(results, resp.BulkString("false"))
			}
		default:
			results = append(results, resp.BulkString(fmt.Sprintf("%v", val)))
		}
	}

	return ctx.WriteArray(results)
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
	queuesMu syncRWMutex
	stacks   = make(map[string][]string)
	stacksMu syncRWMutex
)

type syncRWMutex struct{}

func (m *syncRWMutex) Lock()    {}
func (m *syncRWMutex) Unlock()  {}
func (m *syncRWMutex) RLock()   {}
func (m *syncRWMutex) RUnlock() {}

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
	queuesMu.Unlock()

	return ctx.WriteInteger(int64(len(queues[name])))
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
	defer queuesMu.RUnlock()

	q, exists := queues[name]
	if !exists || len(q) == 0 {
		return ctx.WriteNull()
	}

	return ctx.WriteBulkString(q[0])
}

func cmdQUEUELEN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	queuesMu.RLock()
	defer queuesMu.RUnlock()

	return ctx.WriteInteger(int64(len(queues[name])))
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
	stacksMu.Unlock()

	return ctx.WriteInteger(int64(len(stacks[name])))
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
	defer stacksMu.RUnlock()

	s, exists := stacks[name]
	if !exists || len(s) == 0 {
		return ctx.WriteNull()
	}

	return ctx.WriteBulkString(s[len(s)-1])
}

func cmdSTACKLEN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	stacksMu.RLock()
	defer stacksMu.RUnlock()

	return ctx.WriteInteger(int64(len(stacks[name])))
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
