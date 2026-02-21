package command

import (
	"fmt"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterSchedulerCommands(router *Router) {
	router.Register(&CommandDef{Name: "JOB.CREATE", Handler: cmdJOBCREATE})
	router.Register(&CommandDef{Name: "JOB.DELETE", Handler: cmdJOBDELETE})
	router.Register(&CommandDef{Name: "JOB.GET", Handler: cmdJOBGET})
	router.Register(&CommandDef{Name: "JOB.LIST", Handler: cmdJOBLIST})
	router.Register(&CommandDef{Name: "JOB.ENABLE", Handler: cmdJOBENABLE})
	router.Register(&CommandDef{Name: "JOB.DISABLE", Handler: cmdJOBDISABLE})
	router.Register(&CommandDef{Name: "JOB.RUN", Handler: cmdJOBRUN})
	router.Register(&CommandDef{Name: "JOB.STATS", Handler: cmdJOBSTATS})
	router.Register(&CommandDef{Name: "JOB.RESET", Handler: cmdJOBRESET})
	router.Register(&CommandDef{Name: "JOB.UPDATE", Handler: cmdJOBUPDATE})

	router.Register(&CommandDef{Name: "CIRCUIT.CREATE", Handler: cmdCIRCUITCREATE})
	router.Register(&CommandDef{Name: "CIRCUIT.DELETE", Handler: cmdCIRCUITDELETE})
	router.Register(&CommandDef{Name: "CIRCUIT.ALLOW", Handler: cmdCIRCUITALLOW})
	router.Register(&CommandDef{Name: "CIRCUIT.SUCCESS", Handler: cmdCIRCUITSUCCESS})
	router.Register(&CommandDef{Name: "CIRCUIT.FAILURE", Handler: cmdCIRCUITFAILURE})
	router.Register(&CommandDef{Name: "CIRCUIT.STATE", Handler: cmdCIRCUITSTATE})
	router.Register(&CommandDef{Name: "CIRCUIT.RESET", Handler: cmdCIRCUITRESET})
	router.Register(&CommandDef{Name: "CIRCUIT.STATS", Handler: cmdCIRCUITSTATS})
	router.Register(&CommandDef{Name: "CIRCUIT.LIST", Handler: cmdCIRCUITLIST})

	router.Register(&CommandDef{Name: "SESSION.CREATE", Handler: cmdSESSIONCREATE})
	router.Register(&CommandDef{Name: "SESSION.GET", Handler: cmdSESSIONGET})
	router.Register(&CommandDef{Name: "SESSION.SET", Handler: cmdSESSIONSET})
	router.Register(&CommandDef{Name: "SESSION.DEL", Handler: cmdSESSIONDEL})
	router.Register(&CommandDef{Name: "SESSION.DELETE", Handler: cmdSESSIONDELETE})
	router.Register(&CommandDef{Name: "SESSION.EXISTS", Handler: cmdSESSIONEXISTS})
	router.Register(&CommandDef{Name: "SESSION.TTL", Handler: cmdSESSIONTTL})
	router.Register(&CommandDef{Name: "SESSION.REFRESH", Handler: cmdSESSIONREFRESH})
	router.Register(&CommandDef{Name: "SESSION.CLEAR", Handler: cmdSESSIONCLEAR})
	router.Register(&CommandDef{Name: "SESSION.ALL", Handler: cmdSESSIONALL})
	router.Register(&CommandDef{Name: "SESSION.LIST", Handler: cmdSESSIONLIST})
	router.Register(&CommandDef{Name: "SESSION.COUNT", Handler: cmdSESSIONCOUNT})
	router.Register(&CommandDef{Name: "SESSION.CLEANUP", Handler: cmdSESSIONCLEANUP})
}

func cmdJOBCREATE(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)
	name := ctx.ArgString(1)
	command := ctx.ArgString(2)
	intervalMs := parseInt64(ctx.ArgString(3))

	job := store.GlobalJobScheduler.Create(id, name, command, time.Duration(intervalMs)*time.Millisecond)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"),
		resp.BulkString(job.ID),
		resp.BulkString("name"),
		resp.BulkString(job.Name),
		resp.BulkString("interval"),
		resp.IntegerValue(intervalMs),
	})
}

func cmdJOBDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	if store.GlobalJobScheduler.Delete(id) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdJOBGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)
	stats := store.GlobalJobScheduler.Stats(id)
	if stats == nil {
		return ctx.WriteNull()
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"),
		resp.BulkString(stats["id"].(string)),
		resp.BulkString("name"),
		resp.BulkString(stats["name"].(string)),
		resp.BulkString("command"),
		resp.BulkString(stats["command"].(string)),
		resp.BulkString("interval"),
		resp.IntegerValue(stats["interval"].(int64)),
		resp.BulkString("runs"),
		resp.IntegerValue(stats["runs"].(int64)),
		resp.BulkString("errors"),
		resp.IntegerValue(stats["errors"].(int64)),
		resp.BulkString("enabled"),
		resp.BulkString(fmt.Sprintf("%v", stats["enabled"])),
	})
}

func cmdJOBLIST(ctx *Context) error {
	jobs := store.GlobalJobScheduler.List()

	results := make([]*resp.Value, 0)
	for _, job := range jobs {
		results = append(results,
			resp.BulkString(job.ID),
			resp.BulkString(job.Name),
			resp.IntegerValue(job.Interval.Milliseconds()),
			resp.BulkString(fmt.Sprintf("%v", job.Enabled)),
		)
	}

	return ctx.WriteArray(results)
}

func cmdJOBENABLE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	if store.GlobalJobScheduler.Enable(id) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdJOBDISABLE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	if store.GlobalJobScheduler.Disable(id) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdJOBRUN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	result, err := store.GlobalJobScheduler.Run(id)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteBulkString(result)
}

func cmdJOBSTATS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)
	stats := store.GlobalJobScheduler.Stats(id)
	if stats == nil {
		return ctx.WriteNull()
	}

	results := make([]*resp.Value, 0)
	for k, v := range stats {
		results = append(results, resp.BulkString(k))
		switch val := v.(type) {
		case string:
			results = append(results, resp.BulkString(val))
		case int64:
			results = append(results, resp.IntegerValue(val))
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

func cmdJOBRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	if store.GlobalJobScheduler.Reset(id) {
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR job not found"))
}

func cmdJOBUPDATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)
	intervalMs := parseInt64(ctx.ArgString(1))

	if store.GlobalJobScheduler.UpdateInterval(id, time.Duration(intervalMs)*time.Millisecond) {
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR job not found"))
}

func cmdCIRCUITCREATE(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	failureThreshold := int(parseInt64(ctx.ArgString(1)))
	successThreshold := int(parseInt64(ctx.ArgString(2)))
	timeoutMs := parseInt64(ctx.ArgString(3))

	cb := store.GetOrCreateCircuitBreaker(name, failureThreshold, successThreshold, time.Duration(timeoutMs)*time.Millisecond)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"),
		resp.BulkString(cb.Name),
		resp.BulkString("state"),
		resp.BulkString(cb.GetState().String()),
	})
}

func cmdCIRCUITDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	if store.DeleteCircuitBreaker(name) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdCIRCUITALLOW(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	cb, ok := store.GetCircuitBreaker(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR circuit breaker not found"))
	}

	if cb.Allow() {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdCIRCUITSUCCESS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	cb, ok := store.GetCircuitBreaker(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR circuit breaker not found"))
	}

	cb.RecordSuccess()
	return ctx.WriteOK()
}

func cmdCIRCUITFAILURE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	cb, ok := store.GetCircuitBreaker(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR circuit breaker not found"))
	}

	cb.RecordFailure()
	return ctx.WriteOK()
}

func cmdCIRCUITSTATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	cb, ok := store.GetCircuitBreaker(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR circuit breaker not found"))
	}

	return ctx.WriteBulkString(cb.GetState().String())
}

func cmdCIRCUITRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	cb, ok := store.GetCircuitBreaker(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR circuit breaker not found"))
	}

	cb.Reset()
	return ctx.WriteOK()
}

func cmdCIRCUITSTATS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	cb, ok := store.GetCircuitBreaker(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR circuit breaker not found"))
	}

	stats := cb.Stats()

	results := make([]*resp.Value, 0)
	for k, v := range stats {
		results = append(results, resp.BulkString(k))
		switch val := v.(type) {
		case string:
			results = append(results, resp.BulkString(val))
		case int64:
			results = append(results, resp.IntegerValue(val))
		case int:
			results = append(results, resp.IntegerValue(int64(val)))
		default:
			results = append(results, resp.BulkString(fmt.Sprintf("%v", val)))
		}
	}

	return ctx.WriteArray(results)
}

func cmdCIRCUITLIST(ctx *Context) error {
	names := store.ListCircuitBreakers()

	results := make([]*resp.Value, 0)
	for _, name := range names {
		cb, _ := store.GetCircuitBreaker(name)
		if cb != nil {
			results = append(results,
				resp.BulkString(name),
				resp.BulkString(cb.GetState().String()),
			)
		}
	}

	return ctx.WriteArray(results)
}

func cmdSESSIONCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)
	ttlMs := parseInt64(ctx.ArgString(1))

	session := store.GlobalSessionManager.Create(id, time.Duration(ttlMs)*time.Millisecond)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"),
		resp.BulkString(session.ID),
		resp.BulkString("ttl"),
		resp.IntegerValue(ttlMs),
	})
}

func cmdSESSIONGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)
	key := ctx.ArgString(1)

	session, ok := store.GlobalSessionManager.Get(id)
	if !ok {
		return ctx.WriteNull()
	}

	val, ok := session.Get(key)
	if !ok {
		return ctx.WriteNull()
	}

	return ctx.WriteBulkString(val)
}

func cmdSESSIONSET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)

	session, ok := store.GlobalSessionManager.Get(id)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR session not found"))
	}

	session.Set(key, value)
	return ctx.WriteOK()
}

func cmdSESSIONDEL(ctx *Context) error {
	return cmdSESSIONDELETE(ctx)
}

func cmdSESSIONDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	if ctx.ArgCount() >= 2 {
		key := ctx.ArgString(1)
		session, ok := store.GlobalSessionManager.Get(id)
		if !ok {
			return ctx.WriteError(fmt.Errorf("ERR session not found"))
		}
		if session.Delete(key) {
			return ctx.WriteInteger(1)
		}
		return ctx.WriteInteger(0)
	}

	if store.GlobalSessionManager.Delete(id) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdSESSIONEXISTS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	if store.GlobalSessionManager.Exists(id) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdSESSIONTTL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	session, ok := store.GlobalSessionManager.Get(id)
	if !ok {
		return ctx.WriteInteger(-2)
	}

	ttl := session.TTL()
	if ttl < 0 {
		return ctx.WriteInteger(-1)
	}
	return ctx.WriteInteger(int64(ttl.Milliseconds()))
}

func cmdSESSIONREFRESH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)
	ttlMs := parseInt64(ctx.ArgString(1))

	session, ok := store.GlobalSessionManager.Get(id)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR session not found"))
	}

	session.Refresh(time.Duration(ttlMs) * time.Millisecond)
	return ctx.WriteOK()
}

func cmdSESSIONCLEAR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	session, ok := store.GlobalSessionManager.Get(id)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR session not found"))
	}

	session.Clear()
	return ctx.WriteOK()
}

func cmdSESSIONALL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)

	session, ok := store.GlobalSessionManager.Get(id)
	if !ok {
		return ctx.WriteNull()
	}

	data := session.GetAll()
	results := make([]*resp.Value, 0)
	for k, v := range data {
		results = append(results, resp.BulkString(k), resp.BulkString(v))
	}

	return ctx.WriteArray(results)
}

func cmdSESSIONLIST(ctx *Context) error {
	ids := store.GlobalSessionManager.List()

	results := make([]*resp.Value, len(ids))
	for i, id := range ids {
		results[i] = resp.BulkString(id)
	}

	return ctx.WriteArray(results)
}

func cmdSESSIONCOUNT(ctx *Context) error {
	count := store.GlobalSessionManager.Count()
	return ctx.WriteInteger(count)
}

func cmdSESSIONCLEANUP(ctx *Context) error {
	cleaned := store.GlobalSessionManager.Cleanup()
	return ctx.WriteInteger(cleaned)
}
