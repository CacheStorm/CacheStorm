package command

import (
	"fmt"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func RegisterAdvancedCommands2(router *Router) {
	router.Register(&CommandDef{Name: "FILTER.CREATE", Handler: cmdFILTERCREATE})
	router.Register(&CommandDef{Name: "FILTER.DELETE", Handler: cmdFILTERDELETE})
	router.Register(&CommandDef{Name: "FILTER.APPLY", Handler: cmdFILTERAPPLY})
	router.Register(&CommandDef{Name: "FILTER.LIST", Handler: cmdFILTERLIST})

	router.Register(&CommandDef{Name: "TRANSFORM.CREATE", Handler: cmdTRANSFORMCREATE})
	router.Register(&CommandDef{Name: "TRANSFORM.DELETE", Handler: cmdTRANSFORMDELETE})
	router.Register(&CommandDef{Name: "TRANSFORM.APPLY", Handler: cmdTRANSFORMAPPLY})
	router.Register(&CommandDef{Name: "TRANSFORM.LIST", Handler: cmdTRANSFORMLIST})

	router.Register(&CommandDef{Name: "ENRICH.CREATE", Handler: cmdENRICHCREATE})
	router.Register(&CommandDef{Name: "ENRICH.DELETE", Handler: cmdENRICHDELETE})
	router.Register(&CommandDef{Name: "ENRICH.APPLY", Handler: cmdENRICHAPPLY})
	router.Register(&CommandDef{Name: "ENRICH.LIST", Handler: cmdENRICHLIST})

	router.Register(&CommandDef{Name: "VALIDATE.CREATE", Handler: cmdVALIDATECREATE})
	router.Register(&CommandDef{Name: "VALIDATE.DELETE", Handler: cmdVALIDATEDELETE})
	router.Register(&CommandDef{Name: "VALIDATE.CHECK", Handler: cmdVALIDATECHECK})
	router.Register(&CommandDef{Name: "VALIDATE.LIST", Handler: cmdVALIDATELIST})

	router.Register(&CommandDef{Name: "JOBX.CREATE", Handler: cmdJOBXCREATE})
	router.Register(&CommandDef{Name: "JOBX.DELETE", Handler: cmdJOBXDELETE})
	router.Register(&CommandDef{Name: "JOBX.RUN", Handler: cmdJOBXRUN})
	router.Register(&CommandDef{Name: "JOBX.STATUS", Handler: cmdJOBXSTATUS})
	router.Register(&CommandDef{Name: "JOBX.LIST", Handler: cmdJOBXLIST})

	router.Register(&CommandDef{Name: "STAGE.CREATE", Handler: cmdSTAGECREATE})
	router.Register(&CommandDef{Name: "STAGE.DELETE", Handler: cmdSTAGEDELETE})
	router.Register(&CommandDef{Name: "STAGE.NEXT", Handler: cmdSTAGENEXT})
	router.Register(&CommandDef{Name: "STAGE.PREV", Handler: cmdSTAGEPREV})
	router.Register(&CommandDef{Name: "STAGE.LIST", Handler: cmdSTAGELIST})

	router.Register(&CommandDef{Name: "CONTEXT.CREATE", Handler: cmdCONTEXTCREATE})
	router.Register(&CommandDef{Name: "CONTEXT.DELETE", Handler: cmdCONTEXTDELETE})
	router.Register(&CommandDef{Name: "CONTEXT.SET", Handler: cmdCONTEXTSET})
	router.Register(&CommandDef{Name: "CONTEXT.GET", Handler: cmdCONTEXTGET})
	router.Register(&CommandDef{Name: "CONTEXT.LIST", Handler: cmdCONTEXTLIST})

	router.Register(&CommandDef{Name: "RULE.CREATE", Handler: cmdRULECREATE})
	router.Register(&CommandDef{Name: "RULE.DELETE", Handler: cmdRULEDELETE})
	router.Register(&CommandDef{Name: "RULE.EVAL", Handler: cmdRULEEVAL})
	router.Register(&CommandDef{Name: "RULE.LIST", Handler: cmdRULELIST})

	router.Register(&CommandDef{Name: "POLICY.CREATE", Handler: cmdPOLICYCREATE})
	router.Register(&CommandDef{Name: "POLICY.DELETE", Handler: cmdPOLICYDELETE})
	router.Register(&CommandDef{Name: "POLICY.CHECK", Handler: cmdPOLICYCHECK})
	router.Register(&CommandDef{Name: "POLICY.LIST", Handler: cmdPOLICYLIST})

	router.Register(&CommandDef{Name: "PERMIT.GRANT", Handler: cmdPERMITGRANT})
	router.Register(&CommandDef{Name: "PERMIT.REVOKE", Handler: cmdPERMITREVOKE})
	router.Register(&CommandDef{Name: "PERMIT.CHECK", Handler: cmdPERMITCHECK})
	router.Register(&CommandDef{Name: "PERMIT.LIST", Handler: cmdPERMITLIST})

	router.Register(&CommandDef{Name: "GRANT.CREATE", Handler: cmdGRANTCREATE})
	router.Register(&CommandDef{Name: "GRANT.DELETE", Handler: cmdGRANTDELETE})
	router.Register(&CommandDef{Name: "GRANT.CHECK", Handler: cmdGRANTCHECK})
	router.Register(&CommandDef{Name: "GRANT.LIST", Handler: cmdGRANTLIST})

	router.Register(&CommandDef{Name: "CHAINX.CREATE", Handler: cmdCHAINXCREATE})
	router.Register(&CommandDef{Name: "CHAINX.DELETE", Handler: cmdCHAINXDELETE})
	router.Register(&CommandDef{Name: "CHAINX.EXECUTE", Handler: cmdCHAINXEXECUTE})
	router.Register(&CommandDef{Name: "CHAINX.LIST", Handler: cmdCHAINXLIST})

	router.Register(&CommandDef{Name: "TASKX.CREATE", Handler: cmdTASKXCREATE})
	router.Register(&CommandDef{Name: "TASKX.DELETE", Handler: cmdTASKXDELETE})
	router.Register(&CommandDef{Name: "TASKX.RUN", Handler: cmdTASKXRUN})
	router.Register(&CommandDef{Name: "TASKX.LIST", Handler: cmdTASKXLIST})

	router.Register(&CommandDef{Name: "TIMER.CREATE", Handler: cmdTIMERCREATE})
	router.Register(&CommandDef{Name: "TIMER.DELETE", Handler: cmdTIMERDELETE})
	router.Register(&CommandDef{Name: "TIMER.STATUS", Handler: cmdTIMERSTATUS})
	router.Register(&CommandDef{Name: "TIMER.LIST", Handler: cmdTIMERLIST})

	router.Register(&CommandDef{Name: "COUNTERX2.CREATE", Handler: cmdCOUNTERX2CREATE})
	router.Register(&CommandDef{Name: "COUNTERX2.INCR", Handler: cmdCOUNTERX2INCR})
	router.Register(&CommandDef{Name: "COUNTERX2.DECR", Handler: cmdCOUNTERX2DECR})
	router.Register(&CommandDef{Name: "COUNTERX2.GET", Handler: cmdCOUNTERX2GET})
	router.Register(&CommandDef{Name: "COUNTERX2.LIST", Handler: cmdCOUNTERX2LIST})

	router.Register(&CommandDef{Name: "LEVEL.CREATE", Handler: cmdLEVELCREATE})
	router.Register(&CommandDef{Name: "LEVEL.DELETE", Handler: cmdLEVELDELETE})
	router.Register(&CommandDef{Name: "LEVEL.SET", Handler: cmdLEVELSET})
	router.Register(&CommandDef{Name: "LEVEL.GET", Handler: cmdLEVELGET})
	router.Register(&CommandDef{Name: "LEVEL.LIST", Handler: cmdLEVELLIST})

	router.Register(&CommandDef{Name: "RECORD.CREATE", Handler: cmdRECORDCREATE})
	router.Register(&CommandDef{Name: "RECORD.ADD", Handler: cmdRECORDADD})
	router.Register(&CommandDef{Name: "RECORD.GET", Handler: cmdRECORDGET})
	router.Register(&CommandDef{Name: "RECORD.DELETE", Handler: cmdRECORDDELETE})

	router.Register(&CommandDef{Name: "ENTITY.CREATE", Handler: cmdENTITYCREATE})
	router.Register(&CommandDef{Name: "ENTITY.DELETE", Handler: cmdENTITYDELETE})
	router.Register(&CommandDef{Name: "ENTITY.GET", Handler: cmdENTITYGET})
	router.Register(&CommandDef{Name: "ENTITY.SET", Handler: cmdENTITYSET})
	router.Register(&CommandDef{Name: "ENTITY.LIST", Handler: cmdENTITYLIST})

	router.Register(&CommandDef{Name: "RELATION.CREATE", Handler: cmdRELATIONCREATE})
	router.Register(&CommandDef{Name: "RELATION.DELETE", Handler: cmdRELATIONDELETE})
	router.Register(&CommandDef{Name: "RELATION.GET", Handler: cmdRELATIONGET})
	router.Register(&CommandDef{Name: "RELATION.LIST", Handler: cmdRELATIONLIST})

	router.Register(&CommandDef{Name: "CONNECTIONX.CREATE", Handler: cmdCONNECTIONXCREATE})
	router.Register(&CommandDef{Name: "CONNECTIONX.DELETE", Handler: cmdCONNECTIONXDELETE})
	router.Register(&CommandDef{Name: "CONNECTIONX.STATUS", Handler: cmdCONNECTIONXSTATUS})
	router.Register(&CommandDef{Name: "CONNECTIONX.LIST", Handler: cmdCONNECTIONXLIST})

	router.Register(&CommandDef{Name: "POOLX.CREATE", Handler: cmdPOOLXCREATE})
	router.Register(&CommandDef{Name: "POOLX.DELETE", Handler: cmdPOOLXDELETE})
	router.Register(&CommandDef{Name: "POOLX.ACQUIRE", Handler: cmdPOOLXACQUIRE})
	router.Register(&CommandDef{Name: "POOLX.RELEASE", Handler: cmdPOOLXRELEASE})
	router.Register(&CommandDef{Name: "POOLX.STATUS", Handler: cmdPOOLXSTATUS})

	router.Register(&CommandDef{Name: "BUFFERX.CREATE", Handler: cmdBUFFERXCREATE})
	router.Register(&CommandDef{Name: "BUFFERX.WRITE", Handler: cmdBUFFERXWRITE})
	router.Register(&CommandDef{Name: "BUFFERX.READ", Handler: cmdBUFFERXREAD})
	router.Register(&CommandDef{Name: "BUFFERX.DELETE", Handler: cmdBUFFERXDELETE})

	router.Register(&CommandDef{Name: "STREAMX.CREATE", Handler: cmdSTREAMXCREATE})
	router.Register(&CommandDef{Name: "STREAMX.WRITE", Handler: cmdSTREAMXWRITE})
	router.Register(&CommandDef{Name: "STREAMX.READ", Handler: cmdSTREAMXREAD})
	router.Register(&CommandDef{Name: "STREAMX.DELETE", Handler: cmdSTREAMXDELETE})

	router.Register(&CommandDef{Name: "EVENTX.CREATE", Handler: cmdEVENTXCREATE})
	router.Register(&CommandDef{Name: "EVENTX.DELETE", Handler: cmdEVENTXDELETE})
	router.Register(&CommandDef{Name: "EVENTX.EMIT", Handler: cmdEVENTXEMIT})
	router.Register(&CommandDef{Name: "EVENTX.SUBSCRIBE", Handler: cmdEVENTXSUBSCRIBE})
	router.Register(&CommandDef{Name: "EVENTX.LIST", Handler: cmdEVENTXLIST})

	router.Register(&CommandDef{Name: "HOOK.CREATE", Handler: cmdHOOKCREATE})
	router.Register(&CommandDef{Name: "HOOK.DELETE", Handler: cmdHOOKDELETE})
	router.Register(&CommandDef{Name: "HOOK.TRIGGER", Handler: cmdHOOKTRIGGER})
	router.Register(&CommandDef{Name: "HOOK.LIST", Handler: cmdHOOKLIST})

	router.Register(&CommandDef{Name: "MIDDLEWARE.CREATE", Handler: cmdMIDDLEWARECREATE})
	router.Register(&CommandDef{Name: "MIDDLEWARE.DELETE", Handler: cmdMIDDLEWAREDELETE})
	router.Register(&CommandDef{Name: "MIDDLEWARE.EXECUTE", Handler: cmdMIDDLEWAREEXECUTE})
	router.Register(&CommandDef{Name: "MIDDLEWARE.LIST", Handler: cmdMIDDLEWARELIST})

	router.Register(&CommandDef{Name: "INTERCEPTOR.CREATE", Handler: cmdINTERCEPTORCREATE})
	router.Register(&CommandDef{Name: "INTERCEPTOR.DELETE", Handler: cmdINTERCEPTORDELETE})
	router.Register(&CommandDef{Name: "INTERCEPTOR.CHECK", Handler: cmdINTERCEPTORCHECK})
	router.Register(&CommandDef{Name: "INTERCEPTOR.LIST", Handler: cmdINTERCEPTORLIST})

	router.Register(&CommandDef{Name: "GUARD.CREATE", Handler: cmdGUARDCREATE})
	router.Register(&CommandDef{Name: "GUARD.DELETE", Handler: cmdGUARDDELETE})
	router.Register(&CommandDef{Name: "GUARD.CHECK", Handler: cmdGUARDCHECK})
	router.Register(&CommandDef{Name: "GUARD.LIST", Handler: cmdGUARDLIST})

	router.Register(&CommandDef{Name: "PROXY.CREATE", Handler: cmdPROXYCREATE})
	router.Register(&CommandDef{Name: "PROXY.DELETE", Handler: cmdPROXYDELETE})
	router.Register(&CommandDef{Name: "PROXY.ROUTE", Handler: cmdPROXYROUTE})
	router.Register(&CommandDef{Name: "PROXY.LIST", Handler: cmdPROXYLIST})

	router.Register(&CommandDef{Name: "CACHEX.CREATE", Handler: cmdCACHEXCREATE})
	router.Register(&CommandDef{Name: "CACHEX.DELETE", Handler: cmdCACHEXDELETE})
	router.Register(&CommandDef{Name: "CACHEX.GET", Handler: cmdCACHEXGET})
	router.Register(&CommandDef{Name: "CACHEX.SET", Handler: cmdCACHEXSET})
	router.Register(&CommandDef{Name: "CACHEX.LIST", Handler: cmdCACHEXLIST})

	router.Register(&CommandDef{Name: "STOREX.CREATE", Handler: cmdSTOREXCREATE})
	router.Register(&CommandDef{Name: "STOREX.DELETE", Handler: cmdSTOREXDELETE})
	router.Register(&CommandDef{Name: "STOREX.PUT", Handler: cmdSTOREXPUT})
	router.Register(&CommandDef{Name: "STOREX.GET", Handler: cmdSTOREXGET})
	router.Register(&CommandDef{Name: "STOREX.LIST", Handler: cmdSTOREXLIST})

	router.Register(&CommandDef{Name: "INDEX.CREATE", Handler: cmdINDEXCREATE})
	router.Register(&CommandDef{Name: "INDEX.DELETE", Handler: cmdINDEXDELETE})
	router.Register(&CommandDef{Name: "INDEX.ADD", Handler: cmdINDEXADD})
	router.Register(&CommandDef{Name: "INDEX.SEARCH", Handler: cmdINDEXSEARCH})
	router.Register(&CommandDef{Name: "INDEX.LIST", Handler: cmdINDEXLIST})

	router.Register(&CommandDef{Name: "QUERY.CREATE", Handler: cmdQUERYCREATE})
	router.Register(&CommandDef{Name: "QUERY.DELETE", Handler: cmdQUERYDELETE})
	router.Register(&CommandDef{Name: "QUERY.EXECUTE", Handler: cmdQUERYEXECUTE})
	router.Register(&CommandDef{Name: "QUERY.LIST", Handler: cmdQUERYLIST})

	router.Register(&CommandDef{Name: "VIEW.CREATE", Handler: cmdVIEWCREATE})
	router.Register(&CommandDef{Name: "VIEW.DELETE", Handler: cmdVIEWDELETE})
	router.Register(&CommandDef{Name: "VIEW.GET", Handler: cmdVIEWGET})
	router.Register(&CommandDef{Name: "VIEW.LIST", Handler: cmdVIEWLIST})

	router.Register(&CommandDef{Name: "REPORT.CREATE", Handler: cmdREPORTCREATE})
	router.Register(&CommandDef{Name: "REPORT.DELETE", Handler: cmdREPORTDELETE})
	router.Register(&CommandDef{Name: "REPORT.GENERATE", Handler: cmdREPORTGENERATE})
	router.Register(&CommandDef{Name: "REPORT.LIST", Handler: cmdREPORTLIST})

	router.Register(&CommandDef{Name: "AUDITX.LOG", Handler: cmdAUDITXLOG})
	router.Register(&CommandDef{Name: "AUDITX.GET", Handler: cmdAUDITXGET})
	router.Register(&CommandDef{Name: "AUDITX.SEARCH", Handler: cmdAUDITXSEARCH})
	router.Register(&CommandDef{Name: "AUDITX.LIST", Handler: cmdAUDITXLIST})

	router.Register(&CommandDef{Name: "TOKEN.CREATE", Handler: cmdTOKENCREATE})
	router.Register(&CommandDef{Name: "TOKEN.DELETE", Handler: cmdTOKENDELETE})
	router.Register(&CommandDef{Name: "TOKEN.VALIDATE", Handler: cmdTOKENVALIDATE})
	router.Register(&CommandDef{Name: "TOKEN.REFRESH", Handler: cmdTOKENREFRESH})
	router.Register(&CommandDef{Name: "TOKEN.LIST", Handler: cmdTOKENLIST})

	router.Register(&CommandDef{Name: "SESSIONX.CREATE", Handler: cmdSESSIONXCREATE})
	router.Register(&CommandDef{Name: "SESSIONX.DELETE", Handler: cmdSESSIONXDELETE})
	router.Register(&CommandDef{Name: "SESSIONX.GET", Handler: cmdSESSIONXGET})
	router.Register(&CommandDef{Name: "SESSIONX.SET", Handler: cmdSESSIONXSET})
	router.Register(&CommandDef{Name: "SESSIONX.LIST", Handler: cmdSESSIONXLIST})

	router.Register(&CommandDef{Name: "PROFILE.CREATE", Handler: cmdPROFILECREATE})
	router.Register(&CommandDef{Name: "PROFILE.DELETE", Handler: cmdPROFILEDELETE})
	router.Register(&CommandDef{Name: "PROFILE.GET", Handler: cmdPROFILEGET})
	router.Register(&CommandDef{Name: "PROFILE.SET", Handler: cmdPROFILESET})
	router.Register(&CommandDef{Name: "PROFILE.LIST", Handler: cmdPROFILELIST})

	router.Register(&CommandDef{Name: "ROLEX.CREATE", Handler: cmdROLEXCREATE})
	router.Register(&CommandDef{Name: "ROLEX.DELETE", Handler: cmdROLEXDELETE})
	router.Register(&CommandDef{Name: "ROLEX.ASSIGN", Handler: cmdROLEXASSIGN})
	router.Register(&CommandDef{Name: "ROLEX.CHECK", Handler: cmdROLEXCHECK})
	router.Register(&CommandDef{Name: "ROLEX.LIST", Handler: cmdROLEXLIST})
}

var (
	filters   = make(map[string]string)
	filtersMu sync.RWMutex
)

func cmdFILTERCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	expr := ctx.ArgString(1)
	filtersMu.Lock()
	filters[name] = expr
	filtersMu.Unlock()
	return ctx.WriteOK()
}

func cmdFILTERDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	filtersMu.Lock()
	defer filtersMu.Unlock()
	if _, exists := filters[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(filters, name)
	return ctx.WriteInteger(1)
}

func cmdFILTERAPPLY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteInteger(1)
}

func cmdFILTERLIST(ctx *Context) error {
	filtersMu.RLock()
	defer filtersMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range filters {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

var (
	transforms   = make(map[string]string)
	transformsMu sync.RWMutex
)

func cmdTRANSFORMCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	expr := ctx.ArgString(1)
	transformsMu.Lock()
	transforms[name] = expr
	transformsMu.Unlock()
	return ctx.WriteOK()
}

func cmdTRANSFORMDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	transformsMu.Lock()
	defer transformsMu.Unlock()
	if _, exists := transforms[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(transforms, name)
	return ctx.WriteInteger(1)
}

func cmdTRANSFORMAPPLY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	data := ctx.ArgString(1)
	transformsMu.RLock()
	expr, exists := transforms[name]
	transformsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR transform not found"))
	}
	return ctx.WriteBulkString(fmt.Sprintf("transformed[%s]:%s", expr, data))
}

func cmdTRANSFORMLIST(ctx *Context) error {
	transformsMu.RLock()
	defer transformsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range transforms {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

var (
	enrichers   = make(map[string]string)
	enrichersMu sync.RWMutex
)

func cmdENRICHCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	source := ctx.ArgString(1)
	enrichersMu.Lock()
	enrichers[name] = source
	enrichersMu.Unlock()
	return ctx.WriteOK()
}

func cmdENRICHDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	enrichersMu.Lock()
	defer enrichersMu.Unlock()
	if _, exists := enrichers[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(enrichers, name)
	return ctx.WriteInteger(1)
}

func cmdENRICHAPPLY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	data := ctx.ArgString(1)
	enrichersMu.RLock()
	source, exists := enrichers[name]
	enrichersMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR enricher not found"))
	}
	return ctx.WriteBulkString(fmt.Sprintf("enriched[%s]:%s", source, data))
}

func cmdENRICHLIST(ctx *Context) error {
	enrichersMu.RLock()
	defer enrichersMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range enrichers {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

var (
	validators   = make(map[string]string)
	validatorsMu sync.RWMutex
)

func cmdVALIDATECREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	rule := ctx.ArgString(1)
	validatorsMu.Lock()
	validators[name] = rule
	validatorsMu.Unlock()
	return ctx.WriteOK()
}

func cmdVALIDATEDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	validatorsMu.Lock()
	defer validatorsMu.Unlock()
	if _, exists := validators[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(validators, name)
	return ctx.WriteInteger(1)
}

func cmdVALIDATECHECK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	_ = ctx.ArgString(1)
	validatorsMu.RLock()
	_, exists := validators[name]
	validatorsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR validator not found"))
	}
	return ctx.WriteInteger(1)
}

func cmdVALIDATELIST(ctx *Context) error {
	validatorsMu.RLock()
	defer validatorsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range validators {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

var (
	jobsX    = make(map[string]*JobX)
	jobsXMux sync.RWMutex
)

type JobX struct {
	ID        string
	Name      string
	Status    string
	CreatedAt int64
}

func cmdJOBXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := generateUUID()
	jobsXMux.Lock()
	jobsX[id] = &JobX{ID: id, Name: name, Status: "pending", CreatedAt: time.Now().UnixMilli()}
	jobsXMux.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdJOBXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	jobsXMux.Lock()
	defer jobsXMux.Unlock()
	if _, exists := jobsX[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(jobsX, id)
	return ctx.WriteInteger(1)
}

func cmdJOBXRUN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	jobsXMux.Lock()
	defer jobsXMux.Unlock()
	if job, exists := jobsX[id]; exists {
		job.Status = "completed"
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR job not found"))
}

func cmdJOBXSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	jobsXMux.RLock()
	job, exists := jobsX[id]
	jobsXMux.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR job not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(job.ID),
		resp.BulkString("name"), resp.BulkString(job.Name),
		resp.BulkString("status"), resp.BulkString(job.Status),
	})
}

func cmdJOBXLIST(ctx *Context) error {
	jobsXMux.RLock()
	defer jobsXMux.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range jobsX {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	stages   = make(map[string]*Stage)
	stagesMu sync.RWMutex
)

type Stage struct {
	Name    string
	Current int
	Total   int
}

func cmdSTAGECREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	total := int(parseInt64(ctx.ArgString(1)))
	stagesMu.Lock()
	stages[name] = &Stage{Name: name, Current: 0, Total: total}
	stagesMu.Unlock()
	return ctx.WriteOK()
}

func cmdSTAGEDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	stagesMu.Lock()
	defer stagesMu.Unlock()
	if _, exists := stages[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(stages, name)
	return ctx.WriteInteger(1)
}

func cmdSTAGENEXT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	stagesMu.Lock()
	defer stagesMu.Unlock()
	if stage, exists := stages[name]; exists {
		if stage.Current < stage.Total {
			stage.Current++
		}
		return ctx.WriteInteger(int64(stage.Current))
	}
	return ctx.WriteError(fmt.Errorf("ERR stage not found"))
}

func cmdSTAGEPREV(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	stagesMu.Lock()
	defer stagesMu.Unlock()
	if stage, exists := stages[name]; exists {
		if stage.Current > 0 {
			stage.Current--
		}
		return ctx.WriteInteger(int64(stage.Current))
	}
	return ctx.WriteError(fmt.Errorf("ERR stage not found"))
}

func cmdSTAGELIST(ctx *Context) error {
	stagesMu.RLock()
	defer stagesMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name, stage := range stages {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("current"), resp.IntegerValue(int64(stage.Current)),
			resp.BulkString("total"), resp.IntegerValue(int64(stage.Total)),
		}))
	}
	return ctx.WriteArray(results)
}

var (
	contexts   = make(map[string]map[string]string)
	contextsMu sync.RWMutex
)

func cmdCONTEXTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	contextsMu.Lock()
	contexts[name] = make(map[string]string)
	contextsMu.Unlock()
	return ctx.WriteOK()
}

func cmdCONTEXTDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	contextsMu.Lock()
	defer contextsMu.Unlock()
	if _, exists := contexts[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(contexts, name)
	return ctx.WriteInteger(1)
}

func cmdCONTEXTSET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)
	contextsMu.Lock()
	defer contextsMu.Unlock()
	if c, exists := contexts[name]; exists {
		c[key] = value
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR context not found"))
}

func cmdCONTEXTGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	contextsMu.RLock()
	defer contextsMu.RUnlock()
	if c, exists := contexts[name]; exists {
		if val, ok := c[key]; ok {
			return ctx.WriteBulkString(val)
		}
	}
	return ctx.WriteNull()
}

func cmdCONTEXTLIST(ctx *Context) error {
	contextsMu.RLock()
	defer contextsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range contexts {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

var (
	rules   = make(map[string]string)
	rulesMu sync.RWMutex
)

func cmdRULECREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	expr := ctx.ArgString(1)
	rulesMu.Lock()
	rules[name] = expr
	rulesMu.Unlock()
	return ctx.WriteOK()
}

func cmdRULEDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	rulesMu.Lock()
	defer rulesMu.Unlock()
	if _, exists := rules[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(rules, name)
	return ctx.WriteInteger(1)
}

func cmdRULEEVAL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	_ = ctx.ArgString(1)
	rulesMu.RLock()
	_, exists := rules[name]
	rulesMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR rule not found"))
	}
	return ctx.WriteInteger(1)
}

func cmdRULELIST(ctx *Context) error {
	rulesMu.RLock()
	defer rulesMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range rules {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

var (
	policies   = make(map[string]string)
	policiesMu sync.RWMutex
)

func cmdPOLICYCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	def := ctx.ArgString(1)
	policiesMu.Lock()
	policies[name] = def
	policiesMu.Unlock()
	return ctx.WriteOK()
}

func cmdPOLICYDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	policiesMu.Lock()
	defer policiesMu.Unlock()
	if _, exists := policies[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(policies, name)
	return ctx.WriteInteger(1)
}

func cmdPOLICYCHECK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteInteger(1)
}

func cmdPOLICYLIST(ctx *Context) error {
	policiesMu.RLock()
	defer policiesMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range policies {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

var (
	permits   = make(map[string]map[string]bool)
	permitsMu sync.RWMutex
)

func cmdPERMITGRANT(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	resource := ctx.ArgString(1)
	action := ctx.ArgString(2)
	permitsMu.Lock()
	defer permitsMu.Unlock()
	key := user + ":" + resource + ":" + action
	if _, exists := permits[user]; !exists {
		permits[user] = make(map[string]bool)
	}
	permits[user][key] = true
	return ctx.WriteOK()
}

func cmdPERMITREVOKE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	resource := ctx.ArgString(1)
	action := ctx.ArgString(2)
	permitsMu.Lock()
	defer permitsMu.Unlock()
	key := user + ":" + resource + ":" + action
	if p, exists := permits[user]; exists {
		delete(p, key)
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdPERMITCHECK(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	resource := ctx.ArgString(1)
	action := ctx.ArgString(2)
	permitsMu.RLock()
	defer permitsMu.RUnlock()
	key := user + ":" + resource + ":" + action
	if p, exists := permits[user]; exists {
		if p[key] {
			return ctx.WriteInteger(1)
		}
	}
	return ctx.WriteInteger(0)
}

func cmdPERMITLIST(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	permitsMu.RLock()
	defer permitsMu.RUnlock()
	results := make([]*resp.Value, 0)
	if p, exists := permits[user]; exists {
		for k := range p {
			results = append(results, resp.BulkString(k))
		}
	}
	return ctx.WriteArray(results)
}

var (
	grants   = make(map[string]*Grant)
	grantsMu sync.RWMutex
)

type Grant struct {
	ID       string
	User     string
	Resource string
	Actions  []string
}

func cmdGRANTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	resource := ctx.ArgString(1)
	actions := make([]string, 0)
	for i := 2; i < ctx.ArgCount(); i++ {
		actions = append(actions, ctx.ArgString(i))
	}
	id := generateUUID()
	grantsMu.Lock()
	grants[id] = &Grant{ID: id, User: user, Resource: resource, Actions: actions}
	grantsMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdGRANTDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	grantsMu.Lock()
	defer grantsMu.Unlock()
	if _, exists := grants[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(grants, id)
	return ctx.WriteInteger(1)
}

func cmdGRANTCHECK(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	resource := ctx.ArgString(1)
	action := ctx.ArgString(2)
	grantsMu.RLock()
	defer grantsMu.RUnlock()
	for _, g := range grants {
		if g.User == user && g.Resource == resource {
			for _, a := range g.Actions {
				if a == action || a == "*" {
					return ctx.WriteInteger(1)
				}
			}
		}
	}
	return ctx.WriteInteger(0)
}

func cmdGRANTLIST(ctx *Context) error {
	grantsMu.RLock()
	defer grantsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id, g := range grants {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("id"), resp.BulkString(id),
			resp.BulkString("user"), resp.BulkString(g.User),
			resp.BulkString("resource"), resp.BulkString(g.Resource),
		}))
	}
	return ctx.WriteArray(results)
}

var (
	chainsX    = make(map[string]*ChainX)
	chainsXMux sync.RWMutex
)

type ChainX struct {
	Name  string
	Steps []string
}

func cmdCHAINXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	chainsXMux.Lock()
	chainsX[name] = &ChainX{Name: name, Steps: make([]string, 0)}
	chainsXMux.Unlock()
	return ctx.WriteOK()
}

func cmdCHAINXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	chainsXMux.Lock()
	defer chainsXMux.Unlock()
	if _, exists := chainsX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(chainsX, name)
	return ctx.WriteInteger(1)
}

func cmdCHAINXEXECUTE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	chainsXMux.RLock()
	chain, exists := chainsX[name]
	chainsXMux.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR chain not found"))
	}
	return ctx.WriteInteger(int64(len(chain.Steps)))
}

func cmdCHAINXLIST(ctx *Context) error {
	chainsXMux.RLock()
	defer chainsXMux.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range chainsX {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

var (
	tasksX    = make(map[string]*TaskX)
	tasksXMux sync.RWMutex
)

type TaskX struct {
	ID     string
	Name   string
	Status string
}

func cmdTASKXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := generateUUID()
	tasksXMux.Lock()
	tasksX[id] = &TaskX{ID: id, Name: name, Status: "pending"}
	tasksXMux.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdTASKXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	tasksXMux.Lock()
	defer tasksXMux.Unlock()
	if _, exists := tasksX[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(tasksX, id)
	return ctx.WriteInteger(1)
}

func cmdTASKXRUN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	tasksXMux.Lock()
	defer tasksXMux.Unlock()
	if task, exists := tasksX[id]; exists {
		task.Status = "completed"
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR task not found"))
}

func cmdTASKXLIST(ctx *Context) error {
	tasksXMux.RLock()
	defer tasksXMux.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range tasksX {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	timers   = make(map[string]*Timer)
	timersMu sync.RWMutex
)

type Timer struct {
	ID        string
	Name      string
	Duration  int64
	StartTime int64
	Running   bool
}

func cmdTIMERCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	duration := parseInt64(ctx.ArgString(1))
	id := generateUUID()
	timersMu.Lock()
	timers[id] = &Timer{ID: id, Name: name, Duration: duration, StartTime: time.Now().UnixMilli(), Running: true}
	timersMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdTIMERDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	timersMu.Lock()
	defer timersMu.Unlock()
	if _, exists := timers[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(timers, id)
	return ctx.WriteInteger(1)
}

func cmdTIMERSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	timersMu.RLock()
	timer, exists := timers[id]
	timersMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR timer not found"))
	}
	elapsed := time.Now().UnixMilli() - timer.StartTime
	remaining := timer.Duration - elapsed
	if remaining < 0 {
		remaining = 0
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(timer.ID),
		resp.BulkString("running"), resp.BulkString(fmt.Sprintf("%v", timer.Running)),
		resp.BulkString("remaining_ms"), resp.IntegerValue(remaining),
	})
}

func cmdTIMERLIST(ctx *Context) error {
	timersMu.RLock()
	defer timersMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range timers {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	countersX3   = make(map[string]int64)
	countersX3Mu sync.RWMutex
)

func cmdCOUNTERX2CREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	initVal := int64(0)
	if ctx.ArgCount() >= 2 {
		initVal = parseInt64(ctx.ArgString(1))
	}
	countersX3Mu.Lock()
	countersX3[name] = initVal
	countersX3Mu.Unlock()
	return ctx.WriteOK()
}

func cmdCOUNTERX2INCR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	incr := int64(1)
	if ctx.ArgCount() >= 2 {
		incr = parseInt64(ctx.ArgString(1))
	}
	countersX3Mu.Lock()
	defer countersX3Mu.Unlock()
	countersX3[name] += incr
	return ctx.WriteInteger(countersX3[name])
}

func cmdCOUNTERX2DECR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	decr := int64(1)
	if ctx.ArgCount() >= 2 {
		decr = parseInt64(ctx.ArgString(1))
	}
	countersX3Mu.Lock()
	defer countersX3Mu.Unlock()
	countersX3[name] -= decr
	return ctx.WriteInteger(countersX3[name])
}

func cmdCOUNTERX2GET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	countersX3Mu.RLock()
	val, exists := countersX3[name]
	countersX3Mu.RUnlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(val)
}

func cmdCOUNTERX2LIST(ctx *Context) error {
	countersX3Mu.RLock()
	defer countersX3Mu.RUnlock()
	results := make([]*resp.Value, 0)
	for name, val := range countersX3 {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("value"), resp.IntegerValue(val),
		}))
	}
	return ctx.WriteArray(results)
}

var (
	levels   = make(map[string]int64)
	levelsMu sync.RWMutex
)

func cmdLEVELCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	maxLevel := parseInt64(ctx.ArgString(1))
	levelsMu.Lock()
	levels[name] = maxLevel
	levelsMu.Unlock()
	return ctx.WriteOK()
}

func cmdLEVELDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	levelsMu.Lock()
	defer levelsMu.Unlock()
	if _, exists := levels[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(levels, name)
	return ctx.WriteInteger(1)
}

func cmdLEVELSET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	level := parseInt64(ctx.ArgString(1))
	levelsMu.Lock()
	defer levelsMu.Unlock()
	levels[name+"_current"] = level
	return ctx.WriteOK()
}

func cmdLEVELGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	levelsMu.RLock()
	current := levels[name+"_current"]
	max := levels[name]
	levelsMu.RUnlock()
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("current"), resp.IntegerValue(current),
		resp.BulkString("max"), resp.IntegerValue(max),
	})
}

func cmdLEVELLIST(ctx *Context) error {
	levelsMu.RLock()
	defer levelsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range levels {
		if len(name) > 8 && name[len(name)-8:] != "_current" {
			results = append(results, resp.BulkString(name))
		}
	}
	return ctx.WriteArray(results)
}

var (
	records   = make(map[string]*Record)
	recordsMu sync.RWMutex
)

type Record struct {
	ID     string
	Name   string
	Fields map[string]string
}

func cmdRECORDCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := generateUUID()
	recordsMu.Lock()
	records[id] = &Record{ID: id, Name: name, Fields: make(map[string]string)}
	recordsMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdRECORDADD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)
	recordsMu.Lock()
	defer recordsMu.Unlock()
	if r, exists := records[id]; exists {
		r.Fields[key] = value
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR record not found"))
}

func cmdRECORDGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	recordsMu.RLock()
	r, exists := records[id]
	recordsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR record not found"))
	}
	results := make([]*resp.Value, 0)
	results = append(results, resp.BulkString("id"), resp.BulkString(r.ID))
	results = append(results, resp.BulkString("name"), resp.BulkString(r.Name))
	for k, v := range r.Fields {
		results = append(results, resp.BulkString(k), resp.BulkString(v))
	}
	return ctx.WriteArray(results)
}

func cmdRECORDDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	recordsMu.Lock()
	defer recordsMu.Unlock()
	if _, exists := records[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(records, id)
	return ctx.WriteInteger(1)
}

var (
	entities   = make(map[string]*Entity)
	entitiesMu sync.RWMutex
)

type Entity struct {
	ID         string
	Type       string
	Attributes map[string]string
}

func cmdENTITYCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	entityType := ctx.ArgString(1)
	entitiesMu.Lock()
	entities[id] = &Entity{ID: id, Type: entityType, Attributes: make(map[string]string)}
	entitiesMu.Unlock()
	return ctx.WriteOK()
}

func cmdENTITYDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	entitiesMu.Lock()
	defer entitiesMu.Unlock()
	if _, exists := entities[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(entities, id)
	return ctx.WriteInteger(1)
}

func cmdENTITYGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	entitiesMu.RLock()
	e, exists := entities[id]
	entitiesMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR entity not found"))
	}
	results := make([]*resp.Value, 0)
	results = append(results, resp.BulkString("id"), resp.BulkString(e.ID))
	results = append(results, resp.BulkString("type"), resp.BulkString(e.Type))
	for k, v := range e.Attributes {
		results = append(results, resp.BulkString(k), resp.BulkString(v))
	}
	return ctx.WriteArray(results)
}

func cmdENTITYSET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)
	entitiesMu.Lock()
	defer entitiesMu.Unlock()
	if e, exists := entities[id]; exists {
		e.Attributes[key] = value
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR entity not found"))
}

func cmdENTITYLIST(ctx *Context) error {
	entitiesMu.RLock()
	defer entitiesMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range entities {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	relations   = make(map[string]*Relation)
	relationsMu sync.RWMutex
)

type Relation struct {
	ID       string
	From     string
	To       string
	Type     string
	Metadata map[string]string
}

func cmdRELATIONCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	from := ctx.ArgString(0)
	to := ctx.ArgString(1)
	relType := ctx.ArgString(2)
	id := generateUUID()
	relationsMu.Lock()
	relations[id] = &Relation{ID: id, From: from, To: to, Type: relType, Metadata: make(map[string]string)}
	relationsMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdRELATIONDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	relationsMu.Lock()
	defer relationsMu.Unlock()
	if _, exists := relations[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(relations, id)
	return ctx.WriteInteger(1)
}

func cmdRELATIONGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	relationsMu.RLock()
	r, exists := relations[id]
	relationsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR relation not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(r.ID),
		resp.BulkString("from"), resp.BulkString(r.From),
		resp.BulkString("to"), resp.BulkString(r.To),
		resp.BulkString("type"), resp.BulkString(r.Type),
	})
}

func cmdRELATIONLIST(ctx *Context) error {
	relationsMu.RLock()
	defer relationsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range relations {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	connectionsX    = make(map[string]*ConnectionX)
	connectionsXMux sync.RWMutex
)

type ConnectionX struct {
	ID        string
	Source    string
	Target    string
	Status    string
	CreatedAt int64
}

func cmdCONNECTIONXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	source := ctx.ArgString(0)
	target := ctx.ArgString(1)
	id := generateUUID()
	connectionsXMux.Lock()
	connectionsX[id] = &ConnectionX{ID: id, Source: source, Target: target, Status: "active", CreatedAt: time.Now().UnixMilli()}
	connectionsXMux.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdCONNECTIONXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	connectionsXMux.Lock()
	defer connectionsXMux.Unlock()
	if _, exists := connectionsX[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(connectionsX, id)
	return ctx.WriteInteger(1)
}

func cmdCONNECTIONXSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	connectionsXMux.RLock()
	c, exists := connectionsX[id]
	connectionsXMux.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR connection not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(c.ID),
		resp.BulkString("status"), resp.BulkString(c.Status),
		resp.BulkString("source"), resp.BulkString(c.Source),
		resp.BulkString("target"), resp.BulkString(c.Target),
	})
}

func cmdCONNECTIONXLIST(ctx *Context) error {
	connectionsXMux.RLock()
	defer connectionsXMux.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range connectionsX {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	poolsX    = make(map[string]*PoolX)
	poolsXMux sync.RWMutex
)

type PoolX struct {
	Name      string
	Size      int
	Available []string
	InUse     map[string]bool
}

func cmdPOOLXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	size := int(parseInt64(ctx.ArgString(1)))
	available := make([]string, size)
	for i := 0; i < size; i++ {
		available[i] = fmt.Sprintf("resource-%d", i)
	}
	poolsXMux.Lock()
	poolsX[name] = &PoolX{Name: name, Size: size, Available: available, InUse: make(map[string]bool)}
	poolsXMux.Unlock()
	return ctx.WriteOK()
}

func cmdPOOLXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	poolsXMux.Lock()
	defer poolsXMux.Unlock()
	if _, exists := poolsX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(poolsX, name)
	return ctx.WriteInteger(1)
}

func cmdPOOLXACQUIRE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	poolsXMux.Lock()
	defer poolsXMux.Unlock()
	pool, exists := poolsX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR pool not found"))
	}
	if len(pool.Available) == 0 {
		return ctx.WriteNull()
	}
	resource := pool.Available[0]
	pool.Available = pool.Available[1:]
	pool.InUse[resource] = true
	return ctx.WriteBulkString(resource)
}

func cmdPOOLXRELEASE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	resource := ctx.ArgString(1)
	poolsXMux.Lock()
	defer poolsXMux.Unlock()
	pool, exists := poolsX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR pool not found"))
	}
	if pool.InUse[resource] {
		delete(pool.InUse, resource)
		pool.Available = append(pool.Available, resource)
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdPOOLXSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	poolsXMux.RLock()
	pool, exists := poolsX[name]
	poolsXMux.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR pool not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(pool.Name),
		resp.BulkString("size"), resp.IntegerValue(int64(pool.Size)),
		resp.BulkString("available"), resp.IntegerValue(int64(len(pool.Available))),
		resp.BulkString("in_use"), resp.IntegerValue(int64(len(pool.InUse))),
	})
}

var (
	buffersX    = make(map[string]*BufferX)
	buffersXMux sync.RWMutex
)

type BufferX struct {
	Name string
	Data []byte
}

func cmdBUFFERXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	size := int(parseInt64(ctx.ArgString(1)))
	buffersXMux.Lock()
	buffersX[name] = &BufferX{Name: name, Data: make([]byte, size)}
	buffersXMux.Unlock()
	return ctx.WriteOK()
}

func cmdBUFFERXWRITE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	data := ctx.ArgString(1)
	buffersXMux.Lock()
	defer buffersXMux.Unlock()
	if b, exists := buffersX[name]; exists {
		b.Data = append(b.Data, []byte(data)...)
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR buffer not found"))
}

func cmdBUFFERXREAD(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	buffersXMux.RLock()
	b, exists := buffersX[name]
	buffersXMux.RUnlock()
	if !exists {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(string(b.Data))
}

func cmdBUFFERXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	buffersXMux.Lock()
	defer buffersXMux.Unlock()
	if _, exists := buffersX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(buffersX, name)
	return ctx.WriteInteger(1)
}

var (
	streamsX    = make(map[string]*StreamX)
	streamsXMux sync.RWMutex
)

type StreamX struct {
	Name string
	Data []string
}

func cmdSTREAMXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	streamsXMux.Lock()
	streamsX[name] = &StreamX{Name: name, Data: make([]string, 0)}
	streamsXMux.Unlock()
	return ctx.WriteOK()
}

func cmdSTREAMXWRITE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	data := ctx.ArgString(1)
	streamsXMux.Lock()
	defer streamsXMux.Unlock()
	if s, exists := streamsX[name]; exists {
		s.Data = append(s.Data, data)
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR stream not found"))
}

func cmdSTREAMXREAD(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	streamsXMux.RLock()
	s, exists := streamsX[name]
	streamsXMux.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, len(s.Data))
	for i, d := range s.Data {
		results[i] = resp.BulkString(d)
	}
	return ctx.WriteArray(results)
}

func cmdSTREAMXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	streamsXMux.Lock()
	defer streamsXMux.Unlock()
	if _, exists := streamsX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(streamsX, name)
	return ctx.WriteInteger(1)
}

var (
	eventsX    = make(map[string]*EventX)
	eventsXMux sync.RWMutex
)

type EventX struct {
	Name        string
	Subscribers map[string]bool
	History     []string
}

func cmdEVENTXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	eventsXMux.Lock()
	eventsX[name] = &EventX{Name: name, Subscribers: make(map[string]bool), History: make([]string, 0)}
	eventsXMux.Unlock()
	return ctx.WriteOK()
}

func cmdEVENTXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	eventsXMux.Lock()
	defer eventsXMux.Unlock()
	if _, exists := eventsX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(eventsX, name)
	return ctx.WriteInteger(1)
}

func cmdEVENTXEMIT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	data := ctx.ArgString(1)
	eventsXMux.Lock()
	defer eventsXMux.Unlock()
	if e, exists := eventsX[name]; exists {
		e.History = append(e.History, data)
		return ctx.WriteInteger(int64(len(e.Subscribers)))
	}
	return ctx.WriteError(fmt.Errorf("ERR event not found"))
}

func cmdEVENTXSUBSCRIBE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	subscriber := ctx.ArgString(1)
	eventsXMux.Lock()
	defer eventsXMux.Unlock()
	if e, exists := eventsX[name]; exists {
		e.Subscribers[subscriber] = true
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR event not found"))
}

func cmdEVENTXLIST(ctx *Context) error {
	eventsXMux.RLock()
	defer eventsXMux.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range eventsX {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

var (
	hooks   = make(map[string]*Hook)
	hooksMu sync.RWMutex
)

type Hook struct {
	ID      string
	Name    string
	Trigger string
	Action  string
}

func cmdHOOKCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	trigger := ctx.ArgString(1)
	action := ctx.ArgString(2)
	id := generateUUID()
	hooksMu.Lock()
	hooks[id] = &Hook{ID: id, Name: name, Trigger: trigger, Action: action}
	hooksMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdHOOKDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	hooksMu.Lock()
	defer hooksMu.Unlock()
	if _, exists := hooks[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(hooks, id)
	return ctx.WriteInteger(1)
}

func cmdHOOKTRIGGER(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	hooksMu.RLock()
	hook, exists := hooks[id]
	hooksMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR hook not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(hook.ID),
		resp.BulkString("action"), resp.BulkString(hook.Action),
	})
}

func cmdHOOKLIST(ctx *Context) error {
	hooksMu.RLock()
	defer hooksMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range hooks {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	middlewares   = make(map[string]*Middleware)
	middlewaresMu sync.RWMutex
)

type Middleware struct {
	ID     string
	Name   string
	Before string
	After  string
}

func cmdMIDDLEWARECREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	before := ctx.ArgString(1)
	after := ""
	if ctx.ArgCount() >= 3 {
		after = ctx.ArgString(2)
	}
	id := generateUUID()
	middlewaresMu.Lock()
	middlewares[id] = &Middleware{ID: id, Name: name, Before: before, After: after}
	middlewaresMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdMIDDLEWAREDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	middlewaresMu.Lock()
	defer middlewaresMu.Unlock()
	if _, exists := middlewares[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(middlewares, id)
	return ctx.WriteInteger(1)
}

func cmdMIDDLEWAREEXECUTE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteOK()
}

func cmdMIDDLEWARELIST(ctx *Context) error {
	middlewaresMu.RLock()
	defer middlewaresMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range middlewares {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	interceptors   = make(map[string]*Interceptor)
	interceptorsMu sync.RWMutex
)

type Interceptor struct {
	ID      string
	Name    string
	Pattern string
}

func cmdINTERCEPTORCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	pattern := ctx.ArgString(1)
	id := generateUUID()
	interceptorsMu.Lock()
	interceptors[id] = &Interceptor{ID: id, Name: name, Pattern: pattern}
	interceptorsMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdINTERCEPTORDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	interceptorsMu.Lock()
	defer interceptorsMu.Unlock()
	if _, exists := interceptors[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(interceptors, id)
	return ctx.WriteInteger(1)
}

func cmdINTERCEPTORCHECK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteInteger(1)
}

func cmdINTERCEPTORLIST(ctx *Context) error {
	interceptorsMu.RLock()
	defer interceptorsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range interceptors {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	guards   = make(map[string]*Guard)
	guardsMu sync.RWMutex
)

type Guard struct {
	ID        string
	Name      string
	Condition string
}

func cmdGUARDCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	condition := ctx.ArgString(1)
	id := generateUUID()
	guardsMu.Lock()
	guards[id] = &Guard{ID: id, Name: name, Condition: condition}
	guardsMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdGUARDDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	guardsMu.Lock()
	defer guardsMu.Unlock()
	if _, exists := guards[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(guards, id)
	return ctx.WriteInteger(1)
}

func cmdGUARDCHECK(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	return ctx.WriteInteger(1)
}

func cmdGUARDLIST(ctx *Context) error {
	guardsMu.RLock()
	defer guardsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range guards {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	proxies   = make(map[string]*Proxy)
	proxiesMu sync.RWMutex
)

type Proxy struct {
	ID     string
	Name   string
	Target string
}

func cmdPROXYCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	target := ctx.ArgString(1)
	id := generateUUID()
	proxiesMu.Lock()
	proxies[id] = &Proxy{ID: id, Name: name, Target: target}
	proxiesMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdPROXYDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	proxiesMu.Lock()
	defer proxiesMu.Unlock()
	if _, exists := proxies[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(proxies, id)
	return ctx.WriteInteger(1)
}

func cmdPROXYROUTE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteOK()
}

func cmdPROXYLIST(ctx *Context) error {
	proxiesMu.RLock()
	defer proxiesMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range proxies {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	cachesX    = make(map[string]*CacheX)
	cachesXMux sync.RWMutex
)

type CacheX struct {
	Name string
	Data map[string]string
}

func cmdCACHEXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	cachesXMux.Lock()
	cachesX[name] = &CacheX{Name: name, Data: make(map[string]string)}
	cachesXMux.Unlock()
	return ctx.WriteOK()
}

func cmdCACHEXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	cachesXMux.Lock()
	defer cachesXMux.Unlock()
	if _, exists := cachesX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(cachesX, name)
	return ctx.WriteInteger(1)
}

func cmdCACHEXGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	cachesXMux.RLock()
	defer cachesXMux.RUnlock()
	if c, exists := cachesX[name]; exists {
		if val, ok := c.Data[key]; ok {
			return ctx.WriteBulkString(val)
		}
	}
	return ctx.WriteNull()
}

func cmdCACHEXSET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)
	cachesXMux.Lock()
	defer cachesXMux.Unlock()
	if c, exists := cachesX[name]; exists {
		c.Data[key] = value
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR cache not found"))
}

func cmdCACHEXLIST(ctx *Context) error {
	cachesXMux.RLock()
	defer cachesXMux.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range cachesX {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

var (
	storesX    = make(map[string]*StoreX)
	storesXMux sync.RWMutex
)

type StoreX struct {
	Name string
	Data map[string]string
}

func cmdSTOREXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	storesXMux.Lock()
	storesX[name] = &StoreX{Name: name, Data: make(map[string]string)}
	storesXMux.Unlock()
	return ctx.WriteOK()
}

func cmdSTOREXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	storesXMux.Lock()
	defer storesXMux.Unlock()
	if _, exists := storesX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(storesX, name)
	return ctx.WriteInteger(1)
}

func cmdSTOREXPUT(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)
	storesXMux.Lock()
	defer storesXMux.Unlock()
	if s, exists := storesX[name]; exists {
		s.Data[key] = value
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR store not found"))
}

func cmdSTOREXGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	storesXMux.RLock()
	defer storesXMux.RUnlock()
	if s, exists := storesX[name]; exists {
		if val, ok := s.Data[key]; ok {
			return ctx.WriteBulkString(val)
		}
	}
	return ctx.WriteNull()
}

func cmdSTOREXLIST(ctx *Context) error {
	storesXMux.RLock()
	defer storesXMux.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range storesX {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

var (
	indexes   = make(map[string]*Index)
	indexesMu sync.RWMutex
)

type Index struct {
	Name    string
	Entries map[string][]string
}

func cmdINDEXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	indexesMu.Lock()
	indexes[name] = &Index{Name: name, Entries: make(map[string][]string)}
	indexesMu.Unlock()
	return ctx.WriteOK()
}

func cmdINDEXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	indexesMu.Lock()
	defer indexesMu.Unlock()
	if _, exists := indexes[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(indexes, name)
	return ctx.WriteInteger(1)
}

func cmdINDEXADD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	id := ctx.ArgString(2)
	indexesMu.Lock()
	defer indexesMu.Unlock()
	if idx, exists := indexes[name]; exists {
		idx.Entries[key] = append(idx.Entries[key], id)
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR index not found"))
}

func cmdINDEXSEARCH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	indexesMu.RLock()
	idx, exists := indexes[name]
	indexesMu.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	if ids, ok := idx.Entries[key]; ok {
		for _, id := range ids {
			results = append(results, resp.BulkString(id))
		}
	}
	return ctx.WriteArray(results)
}

func cmdINDEXLIST(ctx *Context) error {
	indexesMu.RLock()
	defer indexesMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range indexes {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

var (
	queries   = make(map[string]string)
	queriesMu sync.RWMutex
)

func cmdQUERYCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	query := ctx.ArgString(1)
	queriesMu.Lock()
	queries[name] = query
	queriesMu.Unlock()
	return ctx.WriteOK()
}

func cmdQUERYDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	queriesMu.Lock()
	defer queriesMu.Unlock()
	if _, exists := queries[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(queries, name)
	return ctx.WriteInteger(1)
}

func cmdQUERYEXECUTE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	return ctx.WriteArray([]*resp.Value{})
}

func cmdQUERYLIST(ctx *Context) error {
	queriesMu.RLock()
	defer queriesMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range queries {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

var (
	views   = make(map[string]string)
	viewsMu sync.RWMutex
)

func cmdVIEWCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	def := ctx.ArgString(1)
	viewsMu.Lock()
	views[name] = def
	viewsMu.Unlock()
	return ctx.WriteOK()
}

func cmdVIEWDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	viewsMu.Lock()
	defer viewsMu.Unlock()
	if _, exists := views[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(views, name)
	return ctx.WriteInteger(1)
}

func cmdVIEWGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	viewsMu.RLock()
	def, exists := views[name]
	viewsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR view not found"))
	}
	return ctx.WriteBulkString(def)
}

func cmdVIEWLIST(ctx *Context) error {
	viewsMu.RLock()
	defer viewsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range views {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

var (
	reports   = make(map[string]*Report)
	reportsMu sync.RWMutex
)

type Report struct {
	ID       string
	Name     string
	Template string
}

func cmdREPORTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	template := ctx.ArgString(1)
	id := generateUUID()
	reportsMu.Lock()
	reports[id] = &Report{ID: id, Name: name, Template: template}
	reportsMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdREPORTDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	reportsMu.Lock()
	defer reportsMu.Unlock()
	if _, exists := reports[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(reports, id)
	return ctx.WriteInteger(1)
}

func cmdREPORTGENERATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	return ctx.WriteOK()
}

func cmdREPORTLIST(ctx *Context) error {
	reportsMu.RLock()
	defer reportsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range reports {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	auditsX    = make(map[string][]*AuditEntryX)
	auditsXMux sync.RWMutex
)

type AuditEntryX struct {
	Timestamp int64
	Action    string
	User      string
	Resource  string
}

func cmdAUDITXLOG(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	logName := ctx.ArgString(0)
	action := ctx.ArgString(1)
	user := ctx.ArgString(2)
	resource := ""
	if ctx.ArgCount() >= 4 {
		resource = ctx.ArgString(3)
	}
	auditsXMux.Lock()
	defer auditsXMux.Unlock()
	if _, exists := auditsX[logName]; !exists {
		auditsX[logName] = make([]*AuditEntryX, 0)
	}
	auditsX[logName] = append(auditsX[logName], &AuditEntryX{
		Timestamp: time.Now().UnixMilli(),
		Action:    action,
		User:      user,
		Resource:  resource,
	})
	return ctx.WriteOK()
}

func cmdAUDITXGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	logName := ctx.ArgString(0)
	auditsXMux.RLock()
	entries, exists := auditsX[logName]
	auditsXMux.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	for _, e := range entries {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("timestamp"), resp.IntegerValue(e.Timestamp),
			resp.BulkString("action"), resp.BulkString(e.Action),
			resp.BulkString("user"), resp.BulkString(e.User),
			resp.BulkString("resource"), resp.BulkString(e.Resource),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdAUDITXSEARCH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	logName := ctx.ArgString(0)
	query := ctx.ArgString(1)
	auditsXMux.RLock()
	entries, exists := auditsX[logName]
	auditsXMux.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	for _, e := range entries {
		if e.Action == query || e.User == query || e.Resource == query {
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.BulkString("timestamp"), resp.IntegerValue(e.Timestamp),
				resp.BulkString("action"), resp.BulkString(e.Action),
				resp.BulkString("user"), resp.BulkString(e.User),
			}))
		}
	}
	return ctx.WriteArray(results)
}

func cmdAUDITXLIST(ctx *Context) error {
	auditsXMux.RLock()
	defer auditsXMux.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range auditsX {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

var (
	tokens   = make(map[string]*Token)
	tokensMu sync.RWMutex
)

type Token struct {
	ID        string
	User      string
	ExpiresAt int64
}

func cmdTOKENCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	ttlMs := parseInt64(ctx.ArgString(1))
	id := generateUUID()
	tokensMu.Lock()
	tokens[id] = &Token{ID: id, User: user, ExpiresAt: time.Now().UnixMilli() + ttlMs}
	tokensMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdTOKENDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	tokensMu.Lock()
	defer tokensMu.Unlock()
	if _, exists := tokens[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(tokens, id)
	return ctx.WriteInteger(1)
}

func cmdTOKENVALIDATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	tokensMu.RLock()
	token, exists := tokens[id]
	tokensMu.RUnlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	if time.Now().UnixMilli() > token.ExpiresAt {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(1)
}

func cmdTOKENREFRESH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	ttlMs := parseInt64(ctx.ArgString(1))
	tokensMu.Lock()
	defer tokensMu.Unlock()
	if token, exists := tokens[id]; exists {
		token.ExpiresAt = time.Now().UnixMilli() + ttlMs
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR token not found"))
}

func cmdTOKENLIST(ctx *Context) error {
	tokensMu.RLock()
	defer tokensMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range tokens {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	sessionsX    = make(map[string]*SessionX)
	sessionsXMux sync.RWMutex
)

type SessionX struct {
	ID        string
	User      string
	Data      map[string]string
	CreatedAt int64
	ExpiresAt int64
}

func cmdSESSIONXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	ttlMs := parseInt64(ctx.ArgString(1))
	id := generateUUID()
	sessionsXMux.Lock()
	sessionsX[id] = &SessionX{ID: id, User: user, Data: make(map[string]string), CreatedAt: time.Now().UnixMilli(), ExpiresAt: time.Now().UnixMilli() + ttlMs}
	sessionsXMux.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdSESSIONXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	sessionsXMux.Lock()
	defer sessionsXMux.Unlock()
	if _, exists := sessionsX[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(sessionsX, id)
	return ctx.WriteInteger(1)
}

func cmdSESSIONXGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	sessionsXMux.RLock()
	session, exists := sessionsX[id]
	sessionsXMux.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR session not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(session.ID),
		resp.BulkString("user"), resp.BulkString(session.User),
		resp.BulkString("created_at"), resp.IntegerValue(session.CreatedAt),
	})
}

func cmdSESSIONXSET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)
	sessionsXMux.Lock()
	defer sessionsXMux.Unlock()
	if s, exists := sessionsX[id]; exists {
		s.Data[key] = value
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR session not found"))
}

func cmdSESSIONXLIST(ctx *Context) error {
	sessionsXMux.RLock()
	defer sessionsXMux.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range sessionsX {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	profiles   = make(map[string]*Profile)
	profilesMu sync.RWMutex
)

type Profile struct {
	ID         string
	User       string
	Attributes map[string]string
}

func cmdPROFILECREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	user := ctx.ArgString(0)
	id := generateUUID()
	profilesMu.Lock()
	profiles[id] = &Profile{ID: id, User: user, Attributes: make(map[string]string)}
	profilesMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdPROFILEDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	profilesMu.Lock()
	defer profilesMu.Unlock()
	if _, exists := profiles[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(profiles, id)
	return ctx.WriteInteger(1)
}

func cmdPROFILEGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	profilesMu.RLock()
	profile, exists := profiles[id]
	profilesMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR profile not found"))
	}
	results := make([]*resp.Value, 0)
	results = append(results, resp.BulkString("id"), resp.BulkString(profile.ID))
	results = append(results, resp.BulkString("user"), resp.BulkString(profile.User))
	for k, v := range profile.Attributes {
		results = append(results, resp.BulkString(k), resp.BulkString(v))
	}
	return ctx.WriteArray(results)
}

func cmdPROFILESET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)
	profilesMu.Lock()
	defer profilesMu.Unlock()
	if p, exists := profiles[id]; exists {
		p.Attributes[key] = value
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR profile not found"))
}

func cmdPROFILELIST(ctx *Context) error {
	profilesMu.RLock()
	defer profilesMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range profiles {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	rolesX    = make(map[string]*RoleX)
	rolesXMux sync.RWMutex
)

type RoleX struct {
	ID          string
	Name        string
	Permissions []string
}

func cmdROLEXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := generateUUID()
	rolesXMux.Lock()
	rolesX[id] = &RoleX{ID: id, Name: name, Permissions: make([]string, 0)}
	rolesXMux.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdROLEXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	rolesXMux.Lock()
	defer rolesXMux.Unlock()
	if _, exists := rolesX[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(rolesX, id)
	return ctx.WriteInteger(1)
}

func cmdROLEXASSIGN(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	_ = ctx.ArgString(2)
	return ctx.WriteOK()
}

func cmdROLEXCHECK(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	_ = ctx.ArgString(2)
	return ctx.WriteInteger(1)
}

func cmdROLEXLIST(ctx *Context) error {
	rolesXMux.RLock()
	defer rolesXMux.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range rolesX {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}
