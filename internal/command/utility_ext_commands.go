package command

import (
	"fmt"
	"sync"

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
	router.Register(&CommandDef{Name: "FLAG.ADDRULE", Handler: cmdFLAGADDRULE})

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

	router.Register(&CommandDef{Name: "BACKUP.CREATE", Handler: cmdBACKUPCREATE})
	router.Register(&CommandDef{Name: "BACKUP.RESTORE", Handler: cmdBACKUPRESTORE})
	router.Register(&CommandDef{Name: "BACKUP.LIST", Handler: cmdBACKUPLIST})
	router.Register(&CommandDef{Name: "BACKUP.DELETE", Handler: cmdBACKUPDELETE})

	router.Register(&CommandDef{Name: "MEMORY.TRIM", Handler: cmdMEMORYTRIM})
	router.Register(&CommandDef{Name: "MEMORY.FRAG", Handler: cmdMEMORYFRAG})
	router.Register(&CommandDef{Name: "MEMORY.PURGE", Handler: cmdMEMORYPURGE})
	router.Register(&CommandDef{Name: "MEMORY.ALLOC", Handler: cmdMEMORYALLOC})
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

func cmdFLAGADDRULE(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	attribute := ctx.ArgString(1)
	operator := ctx.ArgString(2)
	value := ctx.ArgString(3)

	rule := store.FeatureRule{
		Attribute: attribute,
		Operator:  operator,
		Value:     value,
	}

	if store.GlobalFeatureFlags.AddRule(name, rule) {
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR flag not found"))
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

var (
	backups   = make(map[string]string)
	backupsMu sync.RWMutex
)

func cmdBACKUPCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	keys := ctx.Store.Keys()
	backup := ""
	for _, k := range keys {
		entry, ok := ctx.Store.Get(k)
		if ok && entry != nil {
			backup += k + ":" + entry.Value.String() + "\n"
		}
	}

	backupsMu.Lock()
	backups[name] = backup
	backupsMu.Unlock()

	return ctx.WriteInteger(int64(len(keys)))
}

func cmdBACKUPRESTORE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	backupsMu.RLock()
	backup, ok := backups[name]
	backupsMu.RUnlock()

	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR backup not found"))
	}

	lines := splitLines(backup)
	count := 0

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := splitFirst(line, ":")
		if len(parts) == 2 {
			ctx.Store.Set(parts[0], &store.StringValue{Data: []byte(parts[1])}, store.SetOptions{})
			count++
		}
	}

	return ctx.WriteInteger(int64(count))
}

func cmdBACKUPLIST(ctx *Context) error {
	backupsMu.RLock()
	defer backupsMu.RUnlock()

	results := make([]*resp.Value, 0)
	for name := range backups {
		results = append(results, resp.BulkString(name))
	}

	return ctx.WriteArray(results)
}

func cmdBACKUPDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	backupsMu.Lock()
	defer backupsMu.Unlock()

	if _, exists := backups[name]; !exists {
		return ctx.WriteInteger(0)
	}

	delete(backups, name)
	return ctx.WriteInteger(1)
}

func splitLines(s string) []string {
	result := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

func splitFirst(s, sep string) []string {
	for i := 0; i < len(s)-len(sep)+1; i++ {
		if s[i:i+len(sep)] == sep {
			return []string{s[:i], s[i+len(sep):]}
		}
	}
	return []string{s}
}

func cmdMEMORYTRIM(ctx *Context) error {
	return ctx.WriteOK()
}

func cmdMEMORYFRAG(ctx *Context) error {
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("fragmentation_ratio"),
		resp.BulkString("1.0"),
		resp.BulkString("fragmented_bytes"),
		resp.IntegerValue(0),
	})
}

func cmdMEMORYPURGE(ctx *Context) error {
	return ctx.WriteOK()
}

func cmdMEMORYALLOC(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	size := parseInt64(ctx.ArgString(0))

	data := make([]byte, size)
	for i := range data {
		data[i] = 0
	}

	return ctx.WriteInteger(size)
}
