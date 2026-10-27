package command

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterUtilityExtCommands(router *Router) {
	router.Register(&CommandDef{Name: "AUDIT.LOG", Handler: cmdAUDITLOG})
	router.Register(&CommandDef{Name: "AUDIT.GET", Handler: cmdAUDITGET})
	router.Register(&CommandDef{Name: "AUDIT.GETRANGE", Handler: cmdAUDITGETRANGE})
	router.Register(&CommandDef{Name: "AUDIT.GETBYCMD", Handler: cmdAUDITGETBYCMD})
	router.Register(&CommandDef{Name: "AUDIT.GETBYKEY", Handler: cmdAUDITGETBYKEY})
	router.Register(&CommandDef{Name: "AUDIT.CLEAR", Handler: cmdAUDITCLEAR})
	router.Register(&CommandDef{Name: "AUDIT.COUNT", Handler: cmdAUDITCOUNT})
	router.Register(&CommandDef{Name: "AUDIT.STATS", Handler: cmdAUDITSTATS})
	router.Register(&CommandDef{Name: "AUDIT.ENABLE", Handler: cmdAUDITENABLE})
	router.Register(&CommandDef{Name: "AUDIT.DISABLE", Handler: cmdAUDITDISABLE})

	router.Register(&CommandDef{Name: "FLAG.CREATE", Handler: cmdFLAGCREATE})
	router.Register(&CommandDef{Name: "FLAG.DELETE", Handler: cmdFLAGDELETE})
	router.Register(&CommandDef{Name: "FLAG.GET", Handler: cmdFLAGGET})
	router.Register(&CommandDef{Name: "FLAG.ENABLE", Handler: cmdFLAGENABLE})
	router.Register(&CommandDef{Name: "FLAG.DISABLE", Handler: cmdFLAGDISABLE})
	router.Register(&CommandDef{Name: "FLAG.TOGGLE", Handler: cmdFLAGTOGGLE})
	router.Register(&CommandDef{Name: "FLAG.ISENABLED", Handler: cmdFLAGISENABLED})
	router.Register(&CommandDef{Name: "FLAG.LIST", Handler: cmdFLAGLIST})
	router.Register(&CommandDef{Name: "FLAG.LISTENABLED", Handler: cmdFLAGLISTENABLED})
	router.Register(&CommandDef{Name: "FLAG.ADDVARIANT", Handler: cmdFLAGADDVARIANT})
	router.Register(&CommandDef{Name: "FLAG.GETVARIANT", Handler: cmdFLAGGETVARIANT})

	router.Register(&CommandDef{Name: "COUNTER.GET", Handler: cmdCOUNTERGET})
	router.Register(&CommandDef{Name: "COUNTER.SET", Handler: cmdCOUNTERSET})
	router.Register(&CommandDef{Name: "COUNTER.INCR", Handler: cmdCOUNTERINCR})
	router.Register(&CommandDef{Name: "COUNTER.DECR", Handler: cmdCOUNTERDECR})
	router.Register(&CommandDef{Name: "COUNTER.INCRBY", Handler: cmdCOUNTERINCRBY})
	router.Register(&CommandDef{Name: "COUNTER.DECRBY", Handler: cmdCOUNTERDECRBY})
	router.Register(&CommandDef{Name: "COUNTER.DELETE", Handler: cmdCOUNTERDELETE})
	router.Register(&CommandDef{Name: "COUNTER.LIST", Handler: cmdCOUNTERLIST})
	router.Register(&CommandDef{Name: "COUNTER.GETALL", Handler: cmdCOUNTERGETALL})
	router.Register(&CommandDef{Name: "COUNTER.RESET", Handler: cmdCOUNTERRESET})
	router.Register(&CommandDef{Name: "COUNTER.RESETALL", Handler: cmdCOUNTERRESETALL})

	router.Register(&CommandDef{Name: "MEMORY.TRIM", Handler: cmdMEMORYTRIM})
	router.Register(&CommandDef{Name: "MEMORY.FRAG", Handler: cmdMEMORYFRAG})
	router.Register(&CommandDef{Name: "MEMORY.PURGE", Handler: cmdMEMORYPURGE})
}

func cmdAUDITLOG(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	command := ctx.ArgString(0)
	key := ""
	if ctx.ArgCount() >= 2 {
		key = ctx.ArgString(1)
	}

	args := make([]string, 0)
	for i := 2; i < ctx.ArgCount(); i++ {
		args = append(args, ctx.ArgString(i))
	}

	id := store.GlobalAuditLog.Log(command, key, args, ctx.RemoteAddr, ctx.Username, true, 0)

	return ctx.WriteInteger(id)
}

func cmdAUDITGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := parseInt64(ctx.ArgString(0))
	entry := store.GlobalAuditLog.Get(id)

	if entry == nil {
		return ctx.WriteNull()
	}

	args := make([]*resp.Value, len(entry.Args))
	for i, a := range entry.Args {
		args[i] = resp.BulkString(a)
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"),
		resp.IntegerValue(entry.ID),
		resp.BulkString("timestamp"),
		resp.IntegerValue(entry.Timestamp),
		resp.BulkString("command"),
		resp.BulkString(entry.Command),
		resp.BulkString("key"),
		resp.BulkString(entry.Key),
		resp.BulkString("args"),
		resp.ArrayValue(args),
		resp.BulkString("client_ip"),
		resp.BulkString(entry.ClientIP),
		resp.BulkString("user"),
		resp.BulkString(entry.User),
		resp.BulkString("success"),
		resp.BulkString(fmt.Sprintf("%v", entry.Success)),
		resp.BulkString("duration"),
		resp.IntegerValue(entry.Duration),
	})
}

func cmdAUDITGETRANGE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	start := parseInt64(ctx.ArgString(0))
	end := parseInt64(ctx.ArgString(1))

	entries := store.GlobalAuditLog.GetRange(start, end)

	results := make([]*resp.Value, 0)
	for _, e := range entries {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.IntegerValue(e.ID),
			resp.IntegerValue(e.Timestamp),
			resp.BulkString(e.Command),
			resp.BulkString(e.Key),
			resp.BulkString(e.ClientIP),
		}))
	}

	return ctx.WriteArray(results)
}

func cmdAUDITGETBYCMD(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	cmd := ctx.ArgString(0)
	limit := 100
	if ctx.ArgCount() >= 2 {
		limit = int(parseInt64(ctx.ArgString(1)))
	}

	entries := store.GlobalAuditLog.GetByCommand(cmd, limit)

	results := make([]*resp.Value, 0)
	for _, e := range entries {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.IntegerValue(e.ID),
			resp.IntegerValue(e.Timestamp),
			resp.BulkString(e.Command),
			resp.BulkString(e.Key),
		}))
	}

	return ctx.WriteArray(results)
}

func cmdAUDITGETBYKEY(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	limit := 100
	if ctx.ArgCount() >= 2 {
		limit = int(parseInt64(ctx.ArgString(1)))
	}

	entries := store.GlobalAuditLog.GetByKey(key, limit)

	results := make([]*resp.Value, 0)
	for _, e := range entries {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.IntegerValue(e.ID),
			resp.IntegerValue(e.Timestamp),
			resp.BulkString(e.Command),
			resp.BulkString(e.Key),
		}))
	}

	return ctx.WriteArray(results)
}

func cmdAUDITCLEAR(ctx *Context) error {
	store.GlobalAuditLog.Clear()
	return ctx.WriteOK()
}

func cmdAUDITCOUNT(ctx *Context) error {
	count := store.GlobalAuditLog.Count()
	return ctx.WriteInteger(count)
}

func cmdAUDITSTATS(ctx *Context) error {
	stats := store.GlobalAuditLog.Stats()

	results := make([]*resp.Value, 0)
	for k, v := range stats {
		results = append(results, resp.BulkString(k))
		switch val := v.(type) {
		case int64:
			results = append(results, resp.IntegerValue(val))
		case bool:
			results = append(results, resp.BulkString(fmt.Sprintf("%v", val)))
		case map[string]int64:
			arr := make([]*resp.Value, 0)
			for ck, cv := range val {
				arr = append(arr, resp.BulkString(ck), resp.IntegerValue(cv))
			}
			results = append(results, resp.ArrayValue(arr))
		default:
			results = append(results, resp.BulkString(fmt.Sprintf("%v", val)))
		}
	}

	return ctx.WriteArray(results)
}

func cmdAUDITENABLE(ctx *Context) error {
	store.GlobalAuditLog.Enabled = true
	return ctx.WriteOK()
}

func cmdAUDITDISABLE(ctx *Context) error {
	store.GlobalAuditLog.Enabled = false
	return ctx.WriteOK()
}

func cmdFLAGCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	description := ""
	if ctx.ArgCount() >= 2 {
		description = ctx.ArgString(1)
	}

	flag := store.GlobalFeatureFlags.Create(name, description)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"),
		resp.BulkString(flag.Name),
		resp.BulkString("enabled"),
		resp.BulkString(fmt.Sprintf("%v", flag.Enabled)),
	})
}

func cmdFLAGDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	if store.GlobalFeatureFlags.Delete(name) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdFLAGGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	flag, ok := store.GlobalFeatureFlags.Get(name)
	if !ok {
		return ctx.WriteNull()
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"),
		resp.BulkString(flag.Name),
		resp.BulkString("enabled"),
		resp.BulkString(fmt.Sprintf("%v", flag.Enabled)),
		resp.BulkString("description"),
		resp.BulkString(flag.Description),
		resp.BulkString("created_at"),
		resp.IntegerValue(flag.CreatedAt),
		resp.BulkString("updated_at"),
		resp.IntegerValue(flag.UpdatedAt),
	})
}

func cmdFLAGENABLE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	if store.GlobalFeatureFlags.Enable(name) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdFLAGDISABLE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	if store.GlobalFeatureFlags.Disable(name) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdFLAGTOGGLE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	if store.GlobalFeatureFlags.Toggle(name) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdFLAGISENABLED(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	if store.GlobalFeatureFlags.IsEnabled(name) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdFLAGLIST(ctx *Context) error {
	names := store.GlobalFeatureFlags.List()

	results := make([]*resp.Value, len(names))
	for i, name := range names {
		results[i] = resp.BulkString(name)
	}

	return ctx.WriteArray(results)
}

func cmdFLAGLISTENABLED(ctx *Context) error {
	names := store.GlobalFeatureFlags.ListEnabled()

	results := make([]*resp.Value, len(names))
	for i, name := range names {
		results[i] = resp.BulkString(name)
	}

	return ctx.WriteArray(results)
}

func cmdFLAGADDVARIANT(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)

	if store.GlobalFeatureFlags.AddVariant(name, key, value) {
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR flag not found"))
}

func cmdFLAGGETVARIANT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	key := ctx.ArgString(1)

	value, ok := store.GlobalFeatureFlags.GetVariant(name, key)
	if !ok {
		return ctx.WriteNull()
	}

	return ctx.WriteBulkString(value)
}

func cmdCOUNTERGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	value := store.GlobalAtomicCounter.Get(name)

	return ctx.WriteInteger(value)
}

func cmdCOUNTERSET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	value := parseInt64(ctx.ArgString(1))

	store.GlobalAtomicCounter.Set(name, value)

	return ctx.WriteOK()
}

func cmdCOUNTERINCR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	value := store.GlobalAtomicCounter.Increment(name, 1)

	return ctx.WriteInteger(value)
}

func cmdCOUNTERDECR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	value := store.GlobalAtomicCounter.Decrement(name, 1)

	return ctx.WriteInteger(value)
}

func cmdCOUNTERINCRBY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	delta := parseInt64(ctx.ArgString(1))
	value := store.GlobalAtomicCounter.Increment(name, delta)

	return ctx.WriteInteger(value)
}

func cmdCOUNTERDECRBY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	delta := parseInt64(ctx.ArgString(1))
	value := store.GlobalAtomicCounter.Decrement(name, delta)

	return ctx.WriteInteger(value)
}

func cmdCOUNTERDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	if store.GlobalAtomicCounter.Delete(name) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdCOUNTERLIST(ctx *Context) error {
	names := store.GlobalAtomicCounter.List()

	results := make([]*resp.Value, len(names))
	for i, name := range names {
		results[i] = resp.BulkString(name)
	}

	return ctx.WriteArray(results)
}

func cmdCOUNTERGETALL(ctx *Context) error {
	counters := store.GlobalAtomicCounter.GetAll()

	results := make([]*resp.Value, 0)
	for k, v := range counters {
		results = append(results, resp.BulkString(k), resp.IntegerValue(v))
	}

	return ctx.WriteArray(results)
}

func cmdCOUNTERRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	if store.GlobalAtomicCounter.Reset(name) {
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR counter not found"))
}

func cmdCOUNTERRESETALL(ctx *Context) error {
	store.GlobalAtomicCounter.ResetAll()
	return ctx.WriteOK()
}

func cmdMEMORYTRIM(ctx *Context) error {
	debug.FreeOSMemory()
	return ctx.WriteOK()
}

func cmdMEMORYFRAG(ctx *Context) error {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	ratio := 1.0
	if ms.HeapInuse > 0 {
		ratio = float64(ms.HeapSys-ms.HeapReleased) / float64(ms.HeapInuse)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("fragmentation_ratio"),
		resp.BulkString(strconv.FormatFloat(ratio, 'f', 2, 64)),
		resp.BulkString("fragmented_bytes"),
		resp.IntegerValue(int64(ms.HeapSys - ms.HeapReleased - ms.HeapInuse)),
	})
}

func cmdMEMORYPURGE(ctx *Context) error {
	debug.FreeOSMemory()
	return ctx.WriteOK()
}
