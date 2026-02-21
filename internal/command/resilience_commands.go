package command

import (
	"fmt"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func RegisterResilienceCommands(router *Router) {
	router.Register(&CommandDef{Name: "CIRCUITX.CREATE", Handler: cmdCIRCUITXCREATE})
	router.Register(&CommandDef{Name: "CIRCUITX.OPEN", Handler: cmdCIRCUITXOPEN})
	router.Register(&CommandDef{Name: "CIRCUITX.CLOSE", Handler: cmdCIRCUITXCLOSE})
	router.Register(&CommandDef{Name: "CIRCUITX.HALFOPEN", Handler: cmdCIRCUITXHALFOPEN})
	router.Register(&CommandDef{Name: "CIRCUITX.STATUS", Handler: cmdCIRCUITXSTATUS})
	router.Register(&CommandDef{Name: "CIRCUITX.METRICS", Handler: cmdCIRCUITXMETRICS})
	router.Register(&CommandDef{Name: "CIRCUITX.RESET", Handler: cmdCIRCUITXRESET})
	router.Register(&CommandDef{Name: "CIRCUITX.DELETE", Handler: cmdCIRCUITXDELETE})

	router.Register(&CommandDef{Name: "RATELIMITER.CREATE", Handler: cmdRATELIMITERCREATE})
	router.Register(&CommandDef{Name: "RATELIMITER.TRY", Handler: cmdRATELIMITERTRY})
	router.Register(&CommandDef{Name: "RATELIMITER.WAIT", Handler: cmdRATELIMITERWAIT})
	router.Register(&CommandDef{Name: "RATELIMITER.RESET", Handler: cmdRATELIMITERRESET})
	router.Register(&CommandDef{Name: "RATELIMITER.STATUS", Handler: cmdRATELIMITERSTATUS})
	router.Register(&CommandDef{Name: "RATELIMITER.DELETE", Handler: cmdRATELIMITERDELETE})

	router.Register(&CommandDef{Name: "RETRY.CREATE", Handler: cmdRETRYCREATE})
	router.Register(&CommandDef{Name: "RETRY.EXECUTE", Handler: cmdRETRYEXECUTE})
	router.Register(&CommandDef{Name: "RETRY.STATUS", Handler: cmdRETRYSTATUS})
	router.Register(&CommandDef{Name: "RETRY.DELETE", Handler: cmdRETRYDELETE})

	router.Register(&CommandDef{Name: "TIMEOUT.CREATE", Handler: cmdTIMEOUTCREATE})
	router.Register(&CommandDef{Name: "TIMEOUT.EXECUTE", Handler: cmdTIMEOUTEXECUTE})
	router.Register(&CommandDef{Name: "TIMEOUT.DELETE", Handler: cmdTIMEOUTDELETE})

	router.Register(&CommandDef{Name: "BULKHEAD.CREATE", Handler: cmdBULKHEADCREATE})
	router.Register(&CommandDef{Name: "BULKHEAD.ACQUIRE", Handler: cmdBULKHEADACQUIRE})
	router.Register(&CommandDef{Name: "BULKHEAD.RELEASE", Handler: cmdBULKHEADRELEASE})
	router.Register(&CommandDef{Name: "BULKHEAD.STATUS", Handler: cmdBULKHEADSTATUS})
	router.Register(&CommandDef{Name: "BULKHEAD.DELETE", Handler: cmdBULKHEADDELETE})

	router.Register(&CommandDef{Name: "FALLBACK.CREATE", Handler: cmdFALLBACKCREATE})
	router.Register(&CommandDef{Name: "FALLBACK.EXECUTE", Handler: cmdFALLBACKEXECUTE})
	router.Register(&CommandDef{Name: "FALLBACK.DELETE", Handler: cmdFALLBACKDELETE})

	router.Register(&CommandDef{Name: "OBSERVABILITY.TRACE", Handler: cmdOBSERVABILITYTRACE})
	router.Register(&CommandDef{Name: "OBSERVABILITY.METRIC", Handler: cmdOBSERVABILITYMETRIC})
	router.Register(&CommandDef{Name: "OBSERVABILITY.LOG", Handler: cmdOBSERVABILITYLOG})
	router.Register(&CommandDef{Name: "OBSERVABILITY.SPAN", Handler: cmdOBSERVABILITYSPAN})

	router.Register(&CommandDef{Name: "TELEMETRY.RECORD", Handler: cmdTELEMETRYRECORD})
	router.Register(&CommandDef{Name: "TELEMETRY.QUERY", Handler: cmdTELEMETRYQUERY})
	router.Register(&CommandDef{Name: "TELEMETRY.EXPORT", Handler: cmdTELEMETRYEXPORT})

	router.Register(&CommandDef{Name: "DIAGNOSTIC.RUN", Handler: cmdDIAGNOSTICRUN})
	router.Register(&CommandDef{Name: "DIAGNOSTIC.RESULT", Handler: cmdDIAGNOSTICRESULT})
	router.Register(&CommandDef{Name: "DIAGNOSTIC.LIST", Handler: cmdDIAGNOSTICLIST})

	router.Register(&CommandDef{Name: "PROFILE.START", Handler: cmdPROFILESTART})
	router.Register(&CommandDef{Name: "PROFILE.STOP", Handler: cmdPROFILESTOP})
	router.Register(&CommandDef{Name: "PROFILE.RESULT", Handler: cmdPROFILERESULT})
	router.Register(&CommandDef{Name: "PROFILEX.LIST", Handler: cmdPROFILEXLIST})

	router.Register(&CommandDef{Name: "HEAP.STATS", Handler: cmdHEAPSTATS})
	router.Register(&CommandDef{Name: "HEAP.DUMP", Handler: cmdHEAPDUMP})
	router.Register(&CommandDef{Name: "HEAP.GC", Handler: cmdHEAPGC})

	router.Register(&CommandDef{Name: "MEMORYX.ALLOC", Handler: cmdMEMORYXALLOC})
	router.Register(&CommandDef{Name: "MEMORYX.FREE", Handler: cmdMEMORYXFREE})
	router.Register(&CommandDef{Name: "MEMORYX.STATS", Handler: cmdMEMORYXSTATS})
	router.Register(&CommandDef{Name: "MEMORYX.TRACK", Handler: cmdMEMORYXTRACK})

	router.Register(&CommandDef{Name: "CONPOOL.CREATE", Handler: cmdCONPOOLCREATE})
	router.Register(&CommandDef{Name: "CONPOOL.GET", Handler: cmdCONPOOLGET})
	router.Register(&CommandDef{Name: "CONPOOL.RETURN", Handler: cmdCONPOOLRETURN})
	router.Register(&CommandDef{Name: "CONPOOL.STATUS", Handler: cmdCONPOOLSTATUS})
	router.Register(&CommandDef{Name: "CONPOOL.DELETE", Handler: cmdCONPOOLDELETE})

	router.Register(&CommandDef{Name: "BATCHX.CREATE", Handler: cmdBATCHXCREATE})
	router.Register(&CommandDef{Name: "BATCHX.ADD", Handler: cmdBATCHXADD})
	router.Register(&CommandDef{Name: "BATCHX.EXECUTE", Handler: cmdBATCHXEXECUTE})
	router.Register(&CommandDef{Name: "BATCHX.STATUS", Handler: cmdBATCHXSTATUS})
	router.Register(&CommandDef{Name: "BATCHX.DELETE", Handler: cmdBATCHXDELETE})

	router.Register(&CommandDef{Name: "PIPELINEX.START", Handler: cmdPIPELINEXSTART})
	router.Register(&CommandDef{Name: "PIPELINEX.ADD", Handler: cmdPIPELINEXADD})
	router.Register(&CommandDef{Name: "PIPELINEX.EXECUTE", Handler: cmdPIPELINEXEXECUTE})
	router.Register(&CommandDef{Name: "PIPELINEX.CANCEL", Handler: cmdPIPELINEXCANCEL})

	router.Register(&CommandDef{Name: "TRANSX.BEGIN", Handler: cmdTRANSXBEGIN})
	router.Register(&CommandDef{Name: "TRANSX.COMMIT", Handler: cmdTRANSXCOMMIT})
	router.Register(&CommandDef{Name: "TRANSX.ROLLBACK", Handler: cmdTRANSXROLLBACK})
	router.Register(&CommandDef{Name: "TRANSX.STATUS", Handler: cmdTRANSXSTATUS})

	router.Register(&CommandDef{Name: "LOCKX.ACQUIRE", Handler: cmdLOCKXACQUIRE})
	router.Register(&CommandDef{Name: "LOCKX.RELEASE", Handler: cmdLOCKXRELEASE})
	router.Register(&CommandDef{Name: "LOCKX.EXTEND", Handler: cmdLOCKXEXTEND})
	router.Register(&CommandDef{Name: "LOCKX.STATUS", Handler: cmdLOCKXSTATUS})

	router.Register(&CommandDef{Name: "SEMAPHOREX.CREATE", Handler: cmdSEMAPHOREXCREATE})
	router.Register(&CommandDef{Name: "SEMAPHOREX.ACQUIRE", Handler: cmdSEMAPHOREXACQUIRE})
	router.Register(&CommandDef{Name: "SEMAPHOREX.RELEASE", Handler: cmdSEMAPHOREXRELEASE})
	router.Register(&CommandDef{Name: "SEMAPHOREX.STATUS", Handler: cmdSEMAPHOREXSTATUS})

	router.Register(&CommandDef{Name: "ASYNC.SUBMIT", Handler: cmdASYNCSUBMIT})
	router.Register(&CommandDef{Name: "ASYNC.STATUS", Handler: cmdASYNCSTATUS})
	router.Register(&CommandDef{Name: "ASYNC.RESULT", Handler: cmdASYNCRESULT})
	router.Register(&CommandDef{Name: "ASYNC.CANCEL", Handler: cmdASYNCCANCEL})

	router.Register(&CommandDef{Name: "PROMISE.CREATE", Handler: cmdPROMISECREATE})
	router.Register(&CommandDef{Name: "PROMISE.RESOLVE", Handler: cmdPROMISERESOLVE})
	router.Register(&CommandDef{Name: "PROMISE.REJECT", Handler: cmdPROMISEREJECT})
	router.Register(&CommandDef{Name: "PROMISE.STATUS", Handler: cmdPROMISESTATUS})
	router.Register(&CommandDef{Name: "PROMISE.AWAIT", Handler: cmdPROMISEAWAIT})

	router.Register(&CommandDef{Name: "FUTURE.CREATE", Handler: cmdFUTURECREATE})
	router.Register(&CommandDef{Name: "FUTURE.COMPLETE", Handler: cmdFUTURECOMPLETE})
	router.Register(&CommandDef{Name: "FUTURE.GET", Handler: cmdFUTUREGET})
	router.Register(&CommandDef{Name: "FUTURE.CANCEL", Handler: cmdFUTURECANCEL})

	router.Register(&CommandDef{Name: "OBSERVABLE.CREATE", Handler: cmdOBSERVABLECREATE})
	router.Register(&CommandDef{Name: "OBSERVABLE.NEXT", Handler: cmdOBSERVABLENEXT})
	router.Register(&CommandDef{Name: "OBSERVABLE.COMPLETE", Handler: cmdOBSERVABLECOMPLETE})
	router.Register(&CommandDef{Name: "OBSERVABLE.ERROR", Handler: cmdOBSERVABLEERROR})
	router.Register(&CommandDef{Name: "OBSERVABLE.SUBSCRIBE", Handler: cmdOBSERVABLESUBSCRIBE})

	router.Register(&CommandDef{Name: "STREAMPROC.CREATE", Handler: cmdSTREAMPROCCREATE})
	router.Register(&CommandDef{Name: "STREAMPROC.PUSH", Handler: cmdSTREAMPROCPUSH})
	router.Register(&CommandDef{Name: "STREAMPROC.POP", Handler: cmdSTREAMPROCPOP})
	router.Register(&CommandDef{Name: "STREAMPROC.PEEK", Handler: cmdSTREAMPROCPEEK})
	router.Register(&CommandDef{Name: "STREAMPROC.DELETE", Handler: cmdSTREAMPROCDELETE})

	router.Register(&CommandDef{Name: "EVENTSOURCING.APPEND", Handler: cmdEVENTSOURCINGAPPEND})
	router.Register(&CommandDef{Name: "EVENTSOURCING.REPLAY", Handler: cmdEVENTSOURCINGREPLAY})
	router.Register(&CommandDef{Name: "EVENTSOURCING.SNAPSHOT", Handler: cmdEVENTSOURCINGSNAPSHOT})
	router.Register(&CommandDef{Name: "EVENTSOURCING.GET", Handler: cmdEVENTSOURCINGGET})

	router.Register(&CommandDef{Name: "COMPACT.MERGE", Handler: cmdCOMPACTMERGE})
	router.Register(&CommandDef{Name: "COMPACT.STATUS", Handler: cmdCOMPACTSTATUS})

	router.Register(&CommandDef{Name: "BACKPRESSURE.CREATE", Handler: cmdBACKPRESSURECREATE})
	router.Register(&CommandDef{Name: "BACKPRESSURE.CHECK", Handler: cmdBACKPRESSURECHECK})
	router.Register(&CommandDef{Name: "BACKPRESSURE.STATUS", Handler: cmdBACKPRESSURESTATUS})

	router.Register(&CommandDef{Name: "THROTTLEX.CREATE", Handler: cmdTHROTTLEXCREATE})
	router.Register(&CommandDef{Name: "THROTTLEX.CHECK", Handler: cmdTHROTTLEXCHECK})
	router.Register(&CommandDef{Name: "THROTTLEX.STATUS", Handler: cmdTHROTTLEXSTATUS})

	router.Register(&CommandDef{Name: "DEBOUNCEX.CREATE", Handler: cmdDEBOUNCEXCREATE})
	router.Register(&CommandDef{Name: "DEBOUNCEX.CALL", Handler: cmdDEBOUNCEXCALL})
	router.Register(&CommandDef{Name: "DEBOUNCEX.CANCEL", Handler: cmdDEBOUNCEXCANCEL})
	router.Register(&CommandDef{Name: "DEBOUNCEX.FLUSH", Handler: cmdDEBOUNCEXFLUSH})

	router.Register(&CommandDef{Name: "COALESCE.CREATE", Handler: cmdCOALESCECREATE})
	router.Register(&CommandDef{Name: "COALESCE.ADD", Handler: cmdCOALESCEADD})
	router.Register(&CommandDef{Name: "COALESCE.GET", Handler: cmdCOALESCEGET})
	router.Register(&CommandDef{Name: "COALESCE.CLEAR", Handler: cmdCOALESCECLEAR})

	router.Register(&CommandDef{Name: "AGGREGATOR.CREATE", Handler: cmdAGGREGATORCREATE})
	router.Register(&CommandDef{Name: "AGGREGATOR.ADD", Handler: cmdAGGREGATORADD})
	router.Register(&CommandDef{Name: "AGGREGATOR.GET", Handler: cmdAGGREGATORGET})
	router.Register(&CommandDef{Name: "AGGREGATOR.RESET", Handler: cmdAGGREGATORRESET})

	router.Register(&CommandDef{Name: "WINDOWX.CREATE", Handler: cmdWINDOWXCREATE})
	router.Register(&CommandDef{Name: "WINDOWX.ADD", Handler: cmdWINDOWXADD})
	router.Register(&CommandDef{Name: "WINDOWX.GET", Handler: cmdWINDOWXGET})
	router.Register(&CommandDef{Name: "WINDOWX.AGGREGATE", Handler: cmdWINDOWXAGGREGATE})

	router.Register(&CommandDef{Name: "JOINX.CREATE", Handler: cmdJOINXCREATE})
	router.Register(&CommandDef{Name: "JOINX.ADD", Handler: cmdJOINXADD})
	router.Register(&CommandDef{Name: "JOINX.GET", Handler: cmdJOINXGET})
	router.Register(&CommandDef{Name: "JOINX.DELETE", Handler: cmdJOINXDELETE})

	router.Register(&CommandDef{Name: "SHUFFLE.CREATE", Handler: cmdSHUFFLECREATE})
	router.Register(&CommandDef{Name: "SHUFFLE.ADD", Handler: cmdSHUFFLEADD})
	router.Register(&CommandDef{Name: "SHUFFLE.GET", Handler: cmdSHUFFLEGET})

	router.Register(&CommandDef{Name: "PARTITIONX.CREATE", Handler: cmdPARTITIONXCREATE})
	router.Register(&CommandDef{Name: "PARTITIONX.ADD", Handler: cmdPARTITIONXADD})
	router.Register(&CommandDef{Name: "PARTITIONX.GET", Handler: cmdPARTITIONXGET})
	router.Register(&CommandDef{Name: "PARTITIONX.REBALANCE", Handler: cmdPARTITIONXREBALANCE})
}

var (
	circuits   = make(map[string]*Circuit)
	circuitsMu sync.RWMutex
)

type Circuit struct {
	Name            string
	State           string
	Failures        int64
	Successes       int64
	Threshold       int64
	Timeout         int64
	LastFailure     int64
	LastStateChange int64
	HalfOpenMax     int64
	HalfOpenCount   int64
}

func cmdCIRCUITXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	threshold := parseInt64(ctx.ArgString(1))
	timeoutMs := parseInt64(ctx.ArgString(2))
	halfOpenMax := int64(5)
	if ctx.ArgCount() >= 4 {
		halfOpenMax = parseInt64(ctx.ArgString(3))
	}
	circuitsMu.Lock()
	circuits[name] = &Circuit{
		Name:            name,
		State:           "closed",
		Failures:        0,
		Successes:       0,
		Threshold:       threshold,
		Timeout:         timeoutMs,
		LastFailure:     0,
		LastStateChange: time.Now().UnixMilli(),
		HalfOpenMax:     halfOpenMax,
		HalfOpenCount:   0,
	}
	circuitsMu.Unlock()
	return ctx.WriteOK()
}

func cmdCIRCUITXOPEN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	circuitsMu.Lock()
	defer circuitsMu.Unlock()
	if c, exists := circuits[name]; exists {
		c.State = "open"
		c.LastStateChange = time.Now().UnixMilli()
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR circuit not found"))
}

func cmdCIRCUITXCLOSE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	circuitsMu.Lock()
	defer circuitsMu.Unlock()
	if c, exists := circuits[name]; exists {
		c.State = "closed"
		c.Failures = 0
		c.Successes = 0
		c.HalfOpenCount = 0
		c.LastStateChange = time.Now().UnixMilli()
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR circuit not found"))
}

func cmdCIRCUITXHALFOPEN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	circuitsMu.Lock()
	defer circuitsMu.Unlock()
	if c, exists := circuits[name]; exists {
		c.State = "half-open"
		c.HalfOpenCount = 0
		c.LastStateChange = time.Now().UnixMilli()
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR circuit not found"))
}

func cmdCIRCUITXSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	circuitsMu.RLock()
	c, exists := circuits[name]
	circuitsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR circuit not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(c.Name),
		resp.BulkString("state"), resp.BulkString(c.State),
		resp.BulkString("failures"), resp.IntegerValue(c.Failures),
		resp.BulkString("successes"), resp.IntegerValue(c.Successes),
	})
}

func cmdCIRCUITXMETRICS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	circuitsMu.RLock()
	c, exists := circuits[name]
	circuitsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR circuit not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("total_failures"), resp.IntegerValue(c.Failures),
		resp.BulkString("total_successes"), resp.IntegerValue(c.Successes),
		resp.BulkString("failure_rate"), resp.BulkString(fmt.Sprintf("%.2f", float64(c.Failures)/float64(c.Failures+c.Successes+1))),
	})
}

func cmdCIRCUITXRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	circuitsMu.Lock()
	defer circuitsMu.Unlock()
	if c, exists := circuits[name]; exists {
		c.State = "closed"
		c.Failures = 0
		c.Successes = 0
		c.HalfOpenCount = 0
		c.LastStateChange = time.Now().UnixMilli()
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR circuit not found"))
}

func cmdCIRCUITXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	circuitsMu.Lock()
	defer circuitsMu.Unlock()
	if _, exists := circuits[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(circuits, name)
	return ctx.WriteInteger(1)
}

var (
	rateLimitersX    = make(map[string]*RateLimiterX)
	rateLimitersXMux sync.RWMutex
)

type RateLimiterX struct {
	Name     string
	Limit    int64
	Window   int64
	Requests []int64
	Strategy string
}

func cmdRATELIMITERCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	limit := parseInt64(ctx.ArgString(1))
	windowMs := parseInt64(ctx.ArgString(2))
	strategy := "sliding"
	if ctx.ArgCount() >= 4 {
		strategy = ctx.ArgString(3)
	}
	rateLimitersXMux.Lock()
	rateLimitersX[name] = &RateLimiterX{
		Name:     name,
		Limit:    limit,
		Window:   windowMs,
		Requests: make([]int64, 0),
		Strategy: strategy,
	}
	rateLimitersXMux.Unlock()
	return ctx.WriteOK()
}

func cmdRATELIMITERTRY(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	rateLimitersXMux.Lock()
	defer rateLimitersXMux.Unlock()
	rl, exists := rateLimitersX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR rate limiter not found"))
	}
	now := time.Now().UnixMilli()
	cutoff := now - rl.Window
	newRequests := make([]int64, 0)
	for _, ts := range rl.Requests {
		if ts > cutoff {
			newRequests = append(newRequests, ts)
		}
	}
	rl.Requests = newRequests
	if int64(len(rl.Requests)) >= rl.Limit {
		return ctx.WriteArray([]*resp.Value{
			resp.IntegerValue(0),
			resp.IntegerValue(0),
			resp.IntegerValue(rl.Window),
		})
	}
	rl.Requests = append(rl.Requests, now)
	return ctx.WriteArray([]*resp.Value{
		resp.IntegerValue(1),
		resp.IntegerValue(rl.Limit - int64(len(rl.Requests))),
		resp.IntegerValue(rl.Window),
	})
}

func cmdRATELIMITERWAIT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	rateLimitersXMux.RLock()
	rl, exists := rateLimitersX[name]
	rateLimitersXMux.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR rate limiter not found"))
	}
	return ctx.WriteInteger(rl.Window)
}

func cmdRATELIMITERRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	rateLimitersXMux.Lock()
	defer rateLimitersXMux.Unlock()
	rl, exists := rateLimitersX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR rate limiter not found"))
	}
	rl.Requests = make([]int64, 0)
	return ctx.WriteOK()
}

func cmdRATELIMITERSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	rateLimitersXMux.RLock()
	rl, exists := rateLimitersX[name]
	rateLimitersXMux.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR rate limiter not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(rl.Name),
		resp.BulkString("limit"), resp.IntegerValue(rl.Limit),
		resp.BulkString("current"), resp.IntegerValue(int64(len(rl.Requests))),
		resp.BulkString("strategy"), resp.BulkString(rl.Strategy),
	})
}

func cmdRATELIMITERDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	rateLimitersXMux.Lock()
	defer rateLimitersXMux.Unlock()
	if _, exists := rateLimitersX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(rateLimitersX, name)
	return ctx.WriteInteger(1)
}

var (
	retries   = make(map[string]*Retry)
	retriesMu sync.RWMutex
)

type Retry struct {
	Name      string
	MaxRetry  int
	Backoff   int64
	Attempts  int
	LastError string
}

func cmdRETRYCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	maxRetry := int(parseInt64(ctx.ArgString(1)))
	backoffMs := parseInt64(ctx.ArgString(2))
	retriesMu.Lock()
	retries[name] = &Retry{Name: name, MaxRetry: maxRetry, Backoff: backoffMs, Attempts: 0, LastError: ""}
	retriesMu.Unlock()
	return ctx.WriteOK()
}

func cmdRETRYEXECUTE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	_ = ctx.ArgString(1)
	retriesMu.Lock()
	defer retriesMu.Unlock()
	r, exists := retries[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR retry not found"))
	}
	if r.Attempts >= r.MaxRetry {
		return ctx.WriteError(fmt.Errorf("ERR max retries exceeded"))
	}
	r.Attempts++
	return ctx.WriteInteger(int64(r.Attempts))
}

func cmdRETRYSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	retriesMu.RLock()
	r, exists := retries[name]
	retriesMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR retry not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(r.Name),
		resp.BulkString("attempts"), resp.IntegerValue(int64(r.Attempts)),
		resp.BulkString("max_retry"), resp.IntegerValue(int64(r.MaxRetry)),
	})
}

func cmdRETRYDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	retriesMu.Lock()
	defer retriesMu.Unlock()
	if _, exists := retries[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(retries, name)
	return ctx.WriteInteger(1)
}

var (
	timeouts   = make(map[string]*Timeout)
	timeoutsMu sync.RWMutex
)

type Timeout struct {
	Name    string
	Timeout int64
	Created int64
}

func cmdTIMEOUTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	timeoutMs := parseInt64(ctx.ArgString(1))
	timeoutsMu.Lock()
	timeouts[name] = &Timeout{Name: name, Timeout: timeoutMs, Created: time.Now().UnixMilli()}
	timeoutsMu.Unlock()
	return ctx.WriteOK()
}

func cmdTIMEOUTEXECUTE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	_ = ctx.ArgString(1)
	timeoutsMu.RLock()
	t, exists := timeouts[name]
	timeoutsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR timeout not found"))
	}
	elapsed := time.Now().UnixMilli() - t.Created
	if elapsed > t.Timeout {
		return ctx.WriteError(fmt.Errorf("ERR timeout exceeded"))
	}
	return ctx.WriteOK()
}

func cmdTIMEOUTDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	timeoutsMu.Lock()
	defer timeoutsMu.Unlock()
	if _, exists := timeouts[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(timeouts, name)
	return ctx.WriteInteger(1)
}

var (
	bulkheads   = make(map[string]*Bulkhead)
	bulkheadsMu sync.RWMutex
)

type Bulkhead struct {
	Name          string
	MaxConcurrent int
	Current       int
	WaitQueue     int
}

func cmdBULKHEADCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	maxConcurrent := int(parseInt64(ctx.ArgString(1)))
	bulkheadsMu.Lock()
	bulkheads[name] = &Bulkhead{Name: name, MaxConcurrent: maxConcurrent, Current: 0, WaitQueue: 0}
	bulkheadsMu.Unlock()
	return ctx.WriteOK()
}

func cmdBULKHEADACQUIRE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	bulkheadsMu.Lock()
	defer bulkheadsMu.Unlock()
	b, exists := bulkheads[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR bulkhead not found"))
	}
	if b.Current >= b.MaxConcurrent {
		b.WaitQueue++
		return ctx.WriteInteger(0)
	}
	b.Current++
	return ctx.WriteInteger(1)
}

func cmdBULKHEADRELEASE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	bulkheadsMu.Lock()
	defer bulkheadsMu.Unlock()
	b, exists := bulkheads[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR bulkhead not found"))
	}
	if b.Current > 0 {
		b.Current--
	}
	return ctx.WriteOK()
}

func cmdBULKHEADSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	bulkheadsMu.RLock()
	b, exists := bulkheads[name]
	bulkheadsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR bulkhead not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(b.Name),
		resp.BulkString("current"), resp.IntegerValue(int64(b.Current)),
		resp.BulkString("max"), resp.IntegerValue(int64(b.MaxConcurrent)),
		resp.BulkString("waiting"), resp.IntegerValue(int64(b.WaitQueue)),
	})
}

func cmdBULKHEADDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	bulkheadsMu.Lock()
	defer bulkheadsMu.Unlock()
	if _, exists := bulkheads[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(bulkheads, name)
	return ctx.WriteInteger(1)
}

var (
	fallbacks   = make(map[string]string)
	fallbacksMu sync.RWMutex
)

func cmdFALLBACKCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	action := ctx.ArgString(1)
	fallbacksMu.Lock()
	fallbacks[name] = action
	fallbacksMu.Unlock()
	return ctx.WriteOK()
}

func cmdFALLBACKEXECUTE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	fallbacksMu.RLock()
	action, exists := fallbacks[name]
	fallbacksMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR fallback not found"))
	}
	return ctx.WriteBulkString(action)
}

func cmdFALLBACKDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	fallbacksMu.Lock()
	defer fallbacksMu.Unlock()
	if _, exists := fallbacks[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(fallbacks, name)
	return ctx.WriteInteger(1)
}

func cmdOBSERVABILITYTRACE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteOK()
}

func cmdOBSERVABILITYMETRIC(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	_ = ctx.ArgString(2)
	return ctx.WriteOK()
}

func cmdOBSERVABILITYLOG(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteOK()
}

func cmdOBSERVABILITYSPAN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteOK()
}

var (
	telemetry   = make(map[string][]*TelemetryPoint)
	telemetryMu sync.RWMutex
)

type TelemetryPoint struct {
	Timestamp int64
	Value     float64
	Tags      map[string]string
}

func cmdTELEMETRYRECORD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	ts := parseInt64(ctx.ArgString(1))
	value := parseFloatExt([]byte(ctx.ArgString(2)))
	tags := make(map[string]string)
	for i := 3; i+1 < ctx.ArgCount(); i += 2 {
		tags[ctx.ArgString(i)] = ctx.ArgString(i + 1)
	}
	telemetryMu.Lock()
	if _, exists := telemetry[name]; !exists {
		telemetry[name] = make([]*TelemetryPoint, 0)
	}
	telemetry[name] = append(telemetry[name], &TelemetryPoint{Timestamp: ts, Value: value, Tags: tags})
	telemetryMu.Unlock()
	return ctx.WriteOK()
}

func cmdTELEMETRYQUERY(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	telemetryMu.RLock()
	points, exists := telemetry[name]
	telemetryMu.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	for _, p := range points {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("timestamp"), resp.IntegerValue(p.Timestamp),
			resp.BulkString("value"), resp.BulkString(fmt.Sprintf("%.6f", p.Value)),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdTELEMETRYEXPORT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	return ctx.WriteOK()
}

var (
	diagnostics   = make(map[string]*Diagnostic)
	diagnosticsMu sync.RWMutex
)

type Diagnostic struct {
	ID        string
	Name      string
	Status    string
	Result    string
	Timestamp int64
}

func cmdDIAGNOSTICRUN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	checkType := ctx.ArgString(1)
	id := generateUUID()
	diagnosticsMu.Lock()
	diagnostics[id] = &Diagnostic{ID: id, Name: name, Status: "passed", Result: checkType, Timestamp: time.Now().UnixMilli()}
	diagnosticsMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdDIAGNOSTICRESULT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	diagnosticsMu.RLock()
	d, exists := diagnostics[id]
	diagnosticsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR diagnostic not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(d.ID),
		resp.BulkString("name"), resp.BulkString(d.Name),
		resp.BulkString("status"), resp.BulkString(d.Status),
		resp.BulkString("result"), resp.BulkString(d.Result),
	})
}

func cmdDIAGNOSTICLIST(ctx *Context) error {
	diagnosticsMu.RLock()
	defer diagnosticsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range diagnostics {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	profilesX2   = make(map[string]*ProfileX2)
	profilesX2Mu sync.RWMutex
)

type ProfileX2 struct {
	ID        string
	Name      string
	Status    string
	StartTime int64
	EndTime   int64
	Data      string
}

func cmdPROFILESTART(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := generateUUID()
	profilesX2Mu.Lock()
	profilesX2[id] = &ProfileX2{ID: id, Name: name, Status: "running", StartTime: time.Now().UnixMilli(), EndTime: 0, Data: ""}
	profilesX2Mu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdPROFILESTOP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	profilesX2Mu.Lock()
	defer profilesX2Mu.Unlock()
	if p, exists := profilesX2[id]; exists {
		p.Status = "stopped"
		p.EndTime = time.Now().UnixMilli()
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR profile not found"))
}

func cmdPROFILERESULT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	profilesX2Mu.RLock()
	p, exists := profilesX2[id]
	profilesX2Mu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR profile not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(p.ID),
		resp.BulkString("status"), resp.BulkString(p.Status),
		resp.BulkString("duration_ms"), resp.IntegerValue(p.EndTime - p.StartTime),
	})
}

func cmdPROFILEXLIST(ctx *Context) error {
	profilesX2Mu.RLock()
	defer profilesX2Mu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range profilesX2 {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

func cmdHEAPSTATS(ctx *Context) error {
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("allocations"), resp.IntegerValue(0),
		resp.BulkString("frees"), resp.IntegerValue(0),
		resp.BulkString("in_use"), resp.IntegerValue(0),
	})
}

func cmdHEAPDUMP(ctx *Context) error {
	return ctx.WriteOK()
}

func cmdHEAPGC(ctx *Context) error {
	return ctx.WriteOK()
}

var (
	memoryAllocs = make(map[string]int64)
	memoryStats  = make(map[string]*MemoryStat)
	memoryMu     sync.RWMutex
)

type MemoryStat struct {
	Name   string
	Alloc  int64
	Peak   int64
	Tracks int64
}

func cmdMEMORYXALLOC(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	size := parseInt64(ctx.ArgString(1))
	memoryMu.Lock()
	defer memoryMu.Unlock()
	if _, exists := memoryStats[name]; !exists {
		memoryStats[name] = &MemoryStat{Name: name, Alloc: 0, Peak: 0, Tracks: 0}
	}
	memoryStats[name].Alloc += size
	if memoryStats[name].Alloc > memoryStats[name].Peak {
		memoryStats[name].Peak = memoryStats[name].Alloc
	}
	memoryStats[name].Tracks++
	return ctx.WriteInteger(memoryStats[name].Alloc)
}

func cmdMEMORYXFREE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	size := parseInt64(ctx.ArgString(1))
	memoryMu.Lock()
	defer memoryMu.Unlock()
	if s, exists := memoryStats[name]; exists {
		s.Alloc -= size
		if s.Alloc < 0 {
			s.Alloc = 0
		}
		return ctx.WriteInteger(s.Alloc)
	}
	return ctx.WriteInteger(0)
}

func cmdMEMORYXSTATS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	memoryMu.RLock()
	s, exists := memoryStats[name]
	memoryMu.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("alloc"), resp.IntegerValue(0),
			resp.BulkString("peak"), resp.IntegerValue(0),
		})
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(s.Name),
		resp.BulkString("alloc"), resp.IntegerValue(s.Alloc),
		resp.BulkString("peak"), resp.IntegerValue(s.Peak),
		resp.BulkString("tracks"), resp.IntegerValue(s.Tracks),
	})
}

func cmdMEMORYXTRACK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteOK()
}

var (
	conPools   = make(map[string]*ConPool)
	conPoolsMu sync.RWMutex
)

type ConPool struct {
	Name     string
	MaxConns int
	Conns    []string
	InUse    map[string]bool
}

func cmdCONPOOLCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	maxConns := int(parseInt64(ctx.ArgString(1)))
	conns := make([]string, maxConns)
	for i := 0; i < maxConns; i++ {
		conns[i] = fmt.Sprintf("conn-%d", i)
	}
	conPoolsMu.Lock()
	conPools[name] = &ConPool{Name: name, MaxConns: maxConns, Conns: conns, InUse: make(map[string]bool)}
	conPoolsMu.Unlock()
	return ctx.WriteOK()
}

func cmdCONPOOLGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	conPoolsMu.Lock()
	defer conPoolsMu.Unlock()
	p, exists := conPools[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR pool not found"))
	}
	for _, conn := range p.Conns {
		if !p.InUse[conn] {
			p.InUse[conn] = true
			return ctx.WriteBulkString(conn)
		}
	}
	return ctx.WriteNull()
}

func cmdCONPOOLRETURN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	conn := ctx.ArgString(1)
	conPoolsMu.Lock()
	defer conPoolsMu.Unlock()
	p, exists := conPools[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR pool not found"))
	}
	delete(p.InUse, conn)
	return ctx.WriteOK()
}

func cmdCONPOOLSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	conPoolsMu.RLock()
	p, exists := conPools[name]
	conPoolsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR pool not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(p.Name),
		resp.BulkString("total"), resp.IntegerValue(int64(p.MaxConns)),
		resp.BulkString("in_use"), resp.IntegerValue(int64(len(p.InUse))),
		resp.BulkString("available"), resp.IntegerValue(int64(p.MaxConns - len(p.InUse))),
	})
}

func cmdCONPOOLDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	conPoolsMu.Lock()
	defer conPoolsMu.Unlock()
	if _, exists := conPools[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(conPools, name)
	return ctx.WriteInteger(1)
}

var (
	batchesX    = make(map[string]*BatchX)
	batchesXMux sync.RWMutex
)

type BatchX struct {
	ID      string
	Name    string
	Items   []string
	Status  string
	Created int64
}

func cmdBATCHXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	id := generateUUID()
	batchesXMux.Lock()
	batchesX[id] = &BatchX{ID: id, Name: name, Items: make([]string, 0), Status: "pending", Created: time.Now().UnixMilli()}
	batchesXMux.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdBATCHXADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	item := ctx.ArgString(1)
	batchesXMux.Lock()
	defer batchesXMux.Unlock()
	if b, exists := batchesX[id]; exists {
		b.Items = append(b.Items, item)
		return ctx.WriteInteger(int64(len(b.Items)))
	}
	return ctx.WriteError(fmt.Errorf("ERR batch not found"))
}

func cmdBATCHXEXECUTE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	batchesXMux.Lock()
	defer batchesXMux.Unlock()
	if b, exists := batchesX[id]; exists {
		b.Status = "completed"
		return ctx.WriteInteger(int64(len(b.Items)))
	}
	return ctx.WriteError(fmt.Errorf("ERR batch not found"))
}

func cmdBATCHXSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	batchesXMux.RLock()
	b, exists := batchesX[id]
	batchesXMux.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR batch not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(b.ID),
		resp.BulkString("name"), resp.BulkString(b.Name),
		resp.BulkString("status"), resp.BulkString(b.Status),
		resp.BulkString("items"), resp.IntegerValue(int64(len(b.Items))),
	})
}

func cmdBATCHXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	batchesXMux.Lock()
	defer batchesXMux.Unlock()
	if _, exists := batchesX[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(batchesX, id)
	return ctx.WriteInteger(1)
}

var (
	pipelinesX    = make(map[string]*PipelineX)
	pipelinesXMux sync.RWMutex
)

type PipelineX struct {
	ID      string
	Items   []string
	Status  string
	Created int64
}

func cmdPIPELINEXSTART(ctx *Context) error {
	id := generateUUID()
	pipelinesXMux.Lock()
	pipelinesX[id] = &PipelineX{ID: id, Items: make([]string, 0), Status: "open", Created: time.Now().UnixMilli()}
	pipelinesXMux.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdPIPELINEXADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	item := ctx.ArgString(1)
	pipelinesXMux.Lock()
	defer pipelinesXMux.Unlock()
	if p, exists := pipelinesX[id]; exists && p.Status == "open" {
		p.Items = append(p.Items, item)
		return ctx.WriteInteger(int64(len(p.Items)))
	}
	return ctx.WriteError(fmt.Errorf("ERR pipeline not found or closed"))
}

func cmdPIPELINEXEXECUTE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	pipelinesXMux.Lock()
	defer pipelinesXMux.Unlock()
	if p, exists := pipelinesX[id]; exists {
		p.Status = "executed"
		return ctx.WriteInteger(int64(len(p.Items)))
	}
	return ctx.WriteError(fmt.Errorf("ERR pipeline not found"))
}

func cmdPIPELINEXCANCEL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	pipelinesXMux.Lock()
	defer pipelinesXMux.Unlock()
	if p, exists := pipelinesX[id]; exists {
		p.Status = "cancelled"
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR pipeline not found"))
}

var (
	transactionsX    = make(map[string]*TransactionX)
	transactionsXMux sync.RWMutex
)

type TransactionX struct {
	ID       string
	Status   string
	Commands []string
	Created  int64
}

func cmdTRANSXBEGIN(ctx *Context) error {
	id := generateUUID()
	transactionsXMux.Lock()
	transactionsX[id] = &TransactionX{ID: id, Status: "active", Commands: make([]string, 0), Created: time.Now().UnixMilli()}
	transactionsXMux.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdTRANSXCOMMIT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	transactionsXMux.Lock()
	defer transactionsXMux.Unlock()
	if t, exists := transactionsX[id]; exists {
		t.Status = "committed"
		return ctx.WriteInteger(int64(len(t.Commands)))
	}
	return ctx.WriteError(fmt.Errorf("ERR transaction not found"))
}

func cmdTRANSXROLLBACK(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	transactionsXMux.Lock()
	defer transactionsXMux.Unlock()
	if t, exists := transactionsX[id]; exists {
		t.Status = "rolledback"
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR transaction not found"))
}

func cmdTRANSXSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	transactionsXMux.RLock()
	t, exists := transactionsX[id]
	transactionsXMux.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR transaction not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(t.ID),
		resp.BulkString("status"), resp.BulkString(t.Status),
		resp.BulkString("commands"), resp.IntegerValue(int64(len(t.Commands))),
	})
}

var (
	locksX    = make(map[string]*LockX)
	locksXMux sync.RWMutex
)

type LockX struct {
	Key       string
	Holder    string
	ExpiresAt int64
}

func cmdLOCKXACQUIRE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	holder := ctx.ArgString(1)
	ttlMs := parseInt64(ctx.ArgString(2))
	locksXMux.Lock()
	defer locksXMux.Unlock()
	if l, exists := locksX[key]; exists {
		if time.Now().UnixMilli() < l.ExpiresAt && l.Holder != holder {
			return ctx.WriteInteger(0)
		}
	}
	locksX[key] = &LockX{Key: key, Holder: holder, ExpiresAt: time.Now().UnixMilli() + ttlMs}
	return ctx.WriteInteger(1)
}

func cmdLOCKXRELEASE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	holder := ctx.ArgString(1)
	locksXMux.Lock()
	defer locksXMux.Unlock()
	if l, exists := locksX[key]; exists && l.Holder == holder {
		delete(locksX, key)
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdLOCKXEXTEND(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	holder := ctx.ArgString(1)
	ttlMs := parseInt64(ctx.ArgString(2))
	locksXMux.Lock()
	defer locksXMux.Unlock()
	if l, exists := locksX[key]; exists && l.Holder == holder {
		l.ExpiresAt = time.Now().UnixMilli() + ttlMs
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdLOCKXSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	locksXMux.RLock()
	l, exists := locksX[key]
	locksXMux.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("locked"), resp.IntegerValue(0),
		})
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("locked"), resp.IntegerValue(1),
		resp.BulkString("holder"), resp.BulkString(l.Holder),
		resp.BulkString("expires_at"), resp.IntegerValue(l.ExpiresAt),
	})
}

var (
	semaphoresX    = make(map[string]*SemaphoreX)
	semaphoresXMux sync.RWMutex
)

type SemaphoreX struct {
	Name    string
	Max     int64
	Current int64
}

func cmdSEMAPHOREXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	max := parseInt64(ctx.ArgString(1))
	semaphoresXMux.Lock()
	semaphoresX[name] = &SemaphoreX{Name: name, Max: max, Current: 0}
	semaphoresXMux.Unlock()
	return ctx.WriteOK()
}

func cmdSEMAPHOREXACQUIRE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	count := int64(1)
	if ctx.ArgCount() >= 2 {
		count = parseInt64(ctx.ArgString(1))
	}
	semaphoresXMux.Lock()
	defer semaphoresXMux.Unlock()
	s, exists := semaphoresX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR semaphore not found"))
	}
	if s.Current+count > s.Max {
		return ctx.WriteInteger(0)
	}
	s.Current += count
	return ctx.WriteInteger(1)
}

func cmdSEMAPHOREXRELEASE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	count := int64(1)
	if ctx.ArgCount() >= 2 {
		count = parseInt64(ctx.ArgString(1))
	}
	semaphoresXMux.Lock()
	defer semaphoresXMux.Unlock()
	s, exists := semaphoresX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR semaphore not found"))
	}
	s.Current -= count
	if s.Current < 0 {
		s.Current = 0
	}
	return ctx.WriteOK()
}

func cmdSEMAPHOREXSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	semaphoresXMux.RLock()
	s, exists := semaphoresX[name]
	semaphoresXMux.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR semaphore not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(s.Name),
		resp.BulkString("current"), resp.IntegerValue(s.Current),
		resp.BulkString("max"), resp.IntegerValue(s.Max),
		resp.BulkString("available"), resp.IntegerValue(s.Max - s.Current),
	})
}

var (
	asyncJobs   = make(map[string]*AsyncJob)
	asyncJobsMu sync.RWMutex
)

type AsyncJob struct {
	ID        string
	Status    string
	Result    string
	Created   int64
	Completed int64
}

func cmdASYNCSUBMIT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	id := generateUUID()
	asyncJobsMu.Lock()
	asyncJobs[id] = &AsyncJob{ID: id, Status: "pending", Result: "", Created: time.Now().UnixMilli(), Completed: 0}
	asyncJobsMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdASYNCSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	asyncJobsMu.RLock()
	j, exists := asyncJobs[id]
	asyncJobsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR job not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(j.ID),
		resp.BulkString("status"), resp.BulkString(j.Status),
	})
}

func cmdASYNCRESULT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	asyncJobsMu.Lock()
	defer asyncJobsMu.Unlock()
	j, exists := asyncJobs[id]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR job not found"))
	}
	if j.Status != "completed" {
		return ctx.WriteError(fmt.Errorf("ERR job not completed"))
	}
	return ctx.WriteBulkString(j.Result)
}

func cmdASYNCCANCEL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	asyncJobsMu.Lock()
	defer asyncJobsMu.Unlock()
	if j, exists := asyncJobs[id]; exists {
		j.Status = "cancelled"
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR job not found"))
}

var (
	promises   = make(map[string]*Promise)
	promisesMu sync.RWMutex
)

type Promise struct {
	ID      string
	Status  string
	Value   string
	Error   string
	Created int64
}

func cmdPROMISECREATE(ctx *Context) error {
	id := generateUUID()
	promisesMu.Lock()
	promises[id] = &Promise{ID: id, Status: "pending", Value: "", Error: "", Created: time.Now().UnixMilli()}
	promisesMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdPROMISERESOLVE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	value := ctx.ArgString(1)
	promisesMu.Lock()
	defer promisesMu.Unlock()
	if p, exists := promises[id]; exists && p.Status == "pending" {
		p.Status = "resolved"
		p.Value = value
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR promise not found or not pending"))
}

func cmdPROMISEREJECT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	err := ctx.ArgString(1)
	promisesMu.Lock()
	defer promisesMu.Unlock()
	if p, exists := promises[id]; exists && p.Status == "pending" {
		p.Status = "rejected"
		p.Error = err
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR promise not found or not pending"))
}

func cmdPROMISESTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	promisesMu.RLock()
	p, exists := promises[id]
	promisesMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR promise not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(p.ID),
		resp.BulkString("status"), resp.BulkString(p.Status),
	})
}

func cmdPROMISEAWAIT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	promisesMu.RLock()
	p, exists := promises[id]
	promisesMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR promise not found"))
	}
	if p.Status == "pending" {
		return ctx.WriteNull()
	}
	if p.Status == "resolved" {
		return ctx.WriteBulkString(p.Value)
	}
	return ctx.WriteError(fmt.Errorf(p.Error))
}

var (
	futures   = make(map[string]*Future)
	futuresMu sync.RWMutex
)

type Future struct {
	ID     string
	Status string
	Value  string
	Error  string
}

func cmdFUTURECREATE(ctx *Context) error {
	id := generateUUID()
	futuresMu.Lock()
	futures[id] = &Future{ID: id, Status: "pending", Value: "", Error: ""}
	futuresMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdFUTURECOMPLETE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	value := ctx.ArgString(1)
	futuresMu.Lock()
	defer futuresMu.Unlock()
	if f, exists := futures[id]; exists {
		f.Status = "completed"
		f.Value = value
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR future not found"))
}

func cmdFUTUREGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	futuresMu.RLock()
	f, exists := futures[id]
	futuresMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR future not found"))
	}
	if f.Status == "pending" {
		return ctx.WriteNull()
	}
	if f.Status == "completed" {
		return ctx.WriteBulkString(f.Value)
	}
	return ctx.WriteError(fmt.Errorf(f.Error))
}

func cmdFUTURECANCEL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	futuresMu.Lock()
	defer futuresMu.Unlock()
	if f, exists := futures[id]; exists {
		f.Status = "cancelled"
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR future not found"))
}

var (
	observables   = make(map[string]*Observable)
	observablesMu sync.RWMutex
)

type Observable struct {
	ID          string
	Values      []string
	Status      string
	Subscribers []string
}

func cmdOBSERVABLECREATE(ctx *Context) error {
	id := generateUUID()
	observablesMu.Lock()
	observables[id] = &Observable{ID: id, Values: make([]string, 0), Status: "active", Subscribers: make([]string, 0)}
	observablesMu.Unlock()
	return ctx.WriteBulkString(id)
}

func cmdOBSERVABLENEXT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	value := ctx.ArgString(1)
	observablesMu.Lock()
	defer observablesMu.Unlock()
	if o, exists := observables[id]; exists && o.Status == "active" {
		o.Values = append(o.Values, value)
		return ctx.WriteInteger(int64(len(o.Subscribers)))
	}
	return ctx.WriteError(fmt.Errorf("ERR observable not found or not active"))
}

func cmdOBSERVABLECOMPLETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	observablesMu.Lock()
	defer observablesMu.Unlock()
	if o, exists := observables[id]; exists {
		o.Status = "completed"
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR observable not found"))
}

func cmdOBSERVABLEERROR(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	err := ctx.ArgString(1)
	observablesMu.Lock()
	defer observablesMu.Unlock()
	if o, exists := observables[id]; exists {
		o.Status = "error"
		_ = err
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR observable not found"))
}

func cmdOBSERVABLESUBSCRIBE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	subscriber := ctx.ArgString(1)
	observablesMu.Lock()
	defer observablesMu.Unlock()
	if o, exists := observables[id]; exists {
		o.Subscribers = append(o.Subscribers, subscriber)
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR observable not found"))
}

var (
	streamProcs   = make(map[string]*StreamProc)
	streamProcsMu sync.RWMutex
)

type StreamProc struct {
	Name string
	Data []string
	Pos  int
}

func cmdSTREAMPROCCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	streamProcsMu.Lock()
	streamProcs[name] = &StreamProc{Name: name, Data: make([]string, 0), Pos: 0}
	streamProcsMu.Unlock()
	return ctx.WriteOK()
}

func cmdSTREAMPROCPUSH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := ctx.ArgString(1)
	streamProcsMu.Lock()
	defer streamProcsMu.Unlock()
	if s, exists := streamProcs[name]; exists {
		s.Data = append(s.Data, value)
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR stream not found"))
}

func cmdSTREAMPROCPOP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	streamProcsMu.Lock()
	defer streamProcsMu.Unlock()
	if s, exists := streamProcs[name]; exists && len(s.Data) > 0 {
		val := s.Data[0]
		s.Data = s.Data[1:]
		return ctx.WriteBulkString(val)
	}
	return ctx.WriteNull()
}

func cmdSTREAMPROCPEEK(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	streamProcsMu.RLock()
	s, exists := streamProcs[name]
	streamProcsMu.RUnlock()
	if !exists || len(s.Data) == 0 {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(s.Data[0])
}

func cmdSTREAMPROCDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	streamProcsMu.Lock()
	defer streamProcsMu.Unlock()
	if _, exists := streamProcs[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(streamProcs, name)
	return ctx.WriteInteger(1)
}

var (
	eventSourcing   = make(map[string][]*EventX2)
	eventSourcingMu sync.RWMutex
)

type EventX2 struct {
	ID        string
	Type      string
	Data      string
	Timestamp int64
	Version   int64
}

func cmdEVENTSOURCINGAPPEND(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	stream := ctx.ArgString(0)
	eventType := ctx.ArgString(1)
	data := ctx.ArgString(2)
	eventSourcingMu.Lock()
	defer eventSourcingMu.Unlock()
	version := int64(len(eventSourcing[stream]) + 1)
	event := &EventX2{
		ID:        generateUUID(),
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
		Version:   version,
	}
	eventSourcing[stream] = append(eventSourcing[stream], event)
	return ctx.WriteInteger(version)
}

func cmdEVENTSOURCINGREPLAY(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	stream := ctx.ArgString(0)
	fromVersion := int64(0)
	if ctx.ArgCount() >= 2 {
		fromVersion = parseInt64(ctx.ArgString(1))
	}
	eventSourcingMu.RLock()
	events, exists := eventSourcing[stream]
	eventSourcingMu.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	for _, e := range events {
		if e.Version >= fromVersion {
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.BulkString("version"), resp.IntegerValue(e.Version),
				resp.BulkString("type"), resp.BulkString(e.Type),
				resp.BulkString("data"), resp.BulkString(e.Data),
			}))
		}
	}
	return ctx.WriteArray(results)
}

func cmdEVENTSOURCINGSNAPSHOT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteOK()
}

func cmdEVENTSOURCINGGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	stream := ctx.ArgString(0)
	version := parseInt64(ctx.ArgString(1))
	eventSourcingMu.RLock()
	events, exists := eventSourcing[stream]
	eventSourcingMu.RUnlock()
	if !exists {
		return ctx.WriteNull()
	}
	for _, e := range events {
		if e.Version == version {
			return ctx.WriteArray([]*resp.Value{
				resp.BulkString("version"), resp.IntegerValue(e.Version),
				resp.BulkString("type"), resp.BulkString(e.Type),
				resp.BulkString("data"), resp.BulkString(e.Data),
				resp.BulkString("timestamp"), resp.IntegerValue(e.Timestamp),
			})
		}
	}
	return ctx.WriteNull()
}

func cmdCOMPACTMERGE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	return ctx.WriteOK()
}

func cmdCOMPACTSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("status"), resp.BulkString("idle"),
		resp.BulkString("last_compact"), resp.IntegerValue(0),
	})
}

var (
	backpressures   = make(map[string]*Backpressure)
	backpressuresMu sync.RWMutex
)

type Backpressure struct {
	Name      string
	HighWater int64
	LowWater  int64
	Current   int64
	Paused    bool
}

func cmdBACKPRESSURECREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	highWater := parseInt64(ctx.ArgString(1))
	lowWater := parseInt64(ctx.ArgString(2))
	backpressuresMu.Lock()
	backpressures[name] = &Backpressure{Name: name, HighWater: highWater, LowWater: lowWater, Current: 0, Paused: false}
	backpressuresMu.Unlock()
	return ctx.WriteOK()
}

func cmdBACKPRESSURECHECK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	delta := parseInt64(ctx.ArgString(1))
	backpressuresMu.Lock()
	defer backpressuresMu.Unlock()
	b, exists := backpressures[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR backpressure not found"))
	}
	b.Current += delta
	if b.Current >= b.HighWater {
		b.Paused = true
	}
	if b.Current <= b.LowWater {
		b.Paused = false
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("current"), resp.IntegerValue(b.Current),
		resp.BulkString("paused"), resp.BulkString(fmt.Sprintf("%v", b.Paused)),
	})
}

func cmdBACKPRESSURESTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	backpressuresMu.RLock()
	b, exists := backpressures[name]
	backpressuresMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR backpressure not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(b.Name),
		resp.BulkString("current"), resp.IntegerValue(b.Current),
		resp.BulkString("high_water"), resp.IntegerValue(b.HighWater),
		resp.BulkString("low_water"), resp.IntegerValue(b.LowWater),
		resp.BulkString("paused"), resp.BulkString(fmt.Sprintf("%v", b.Paused)),
	})
}

var (
	throttlesX    = make(map[string]*ThrottleX)
	throttlesXMux sync.RWMutex
)

type ThrottleX struct {
	Name     string
	Rate     int64
	LastSent int64
}

func cmdTHROTTLEXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	ratePerMs := parseInt64(ctx.ArgString(1))
	throttlesXMux.Lock()
	throttlesX[name] = &ThrottleX{Name: name, Rate: ratePerMs, LastSent: 0}
	throttlesXMux.Unlock()
	return ctx.WriteOK()
}

func cmdTHROTTLEXCHECK(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	throttlesXMux.Lock()
	defer throttlesXMux.Unlock()
	t, exists := throttlesX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR throttle not found"))
	}
	now := time.Now().UnixMilli()
	if now-t.LastSent < t.Rate {
		return ctx.WriteInteger(0)
	}
	t.LastSent = now
	return ctx.WriteInteger(1)
}

func cmdTHROTTLEXSTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	throttlesXMux.RLock()
	t, exists := throttlesX[name]
	throttlesXMux.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR throttle not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(t.Name),
		resp.BulkString("rate_ms"), resp.IntegerValue(t.Rate),
		resp.BulkString("last_sent"), resp.IntegerValue(t.LastSent),
	})
}

var (
	debouncesX    = make(map[string]*DebounceX)
	debouncesXMux sync.RWMutex
)

type DebounceX struct {
	Name       string
	Delay      int64
	LastCall   int64
	Pending    bool
	PendingArg string
}

func cmdDEBOUNCEXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	delayMs := parseInt64(ctx.ArgString(1))
	debouncesXMux.Lock()
	debouncesX[name] = &DebounceX{Name: name, Delay: delayMs, LastCall: 0, Pending: false, PendingArg: ""}
	debouncesXMux.Unlock()
	return ctx.WriteOK()
}

func cmdDEBOUNCEXCALL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	arg := ctx.ArgString(1)
	debouncesXMux.Lock()
	defer debouncesXMux.Unlock()
	d, exists := debouncesX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR debounce not found"))
	}
	d.LastCall = time.Now().UnixMilli()
	d.Pending = true
	d.PendingArg = arg
	return ctx.WriteOK()
}

func cmdDEBOUNCEXCANCEL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	debouncesXMux.Lock()
	defer debouncesXMux.Unlock()
	if d, exists := debouncesX[name]; exists {
		d.Pending = false
		d.PendingArg = ""
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR debounce not found"))
}

func cmdDEBOUNCEXFLUSH(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	debouncesXMux.Lock()
	defer debouncesXMux.Unlock()
	d, exists := debouncesX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR debounce not found"))
	}
	if d.Pending {
		arg := d.PendingArg
		d.Pending = false
		d.PendingArg = ""
		return ctx.WriteBulkString(arg)
	}
	return ctx.WriteNull()
}

var (
	coalesces   = make(map[string]*Coalesce)
	coalescesMu sync.RWMutex
)

type Coalesce struct {
	Name   string
	Values []string
}

func cmdCOALESCECREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	coalescesMu.Lock()
	coalesces[name] = &Coalesce{Name: name, Values: make([]string, 0)}
	coalescesMu.Unlock()
	return ctx.WriteOK()
}

func cmdCOALESCEADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := ctx.ArgString(1)
	coalescesMu.Lock()
	defer coalescesMu.Unlock()
	if c, exists := coalesces[name]; exists {
		c.Values = append(c.Values, value)
		return ctx.WriteInteger(int64(len(c.Values)))
	}
	return ctx.WriteError(fmt.Errorf("ERR coalesce not found"))
}

func cmdCOALESCEGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	coalescesMu.RLock()
	c, exists := coalesces[name]
	coalescesMu.RUnlock()
	if !exists || len(c.Values) == 0 {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(c.Values[0])
}

func cmdCOALESCECLEAR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	coalescesMu.Lock()
	defer coalescesMu.Unlock()
	if c, exists := coalesces[name]; exists {
		c.Values = make([]string, 0)
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR coalesce not found"))
}

var (
	aggregators   = make(map[string]*Aggregator)
	aggregatorsMu sync.RWMutex
)

type Aggregator struct {
	Name   string
	Type   string
	Values []float64
}

func cmdAGGREGATORCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	aggType := ctx.ArgString(1)
	aggregatorsMu.Lock()
	aggregators[name] = &Aggregator{Name: name, Type: aggType, Values: make([]float64, 0)}
	aggregatorsMu.Unlock()
	return ctx.WriteOK()
}

func cmdAGGREGATORADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := parseFloatExt([]byte(ctx.ArgString(1)))
	aggregatorsMu.Lock()
	defer aggregatorsMu.Unlock()
	if a, exists := aggregators[name]; exists {
		a.Values = append(a.Values, value)
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR aggregator not found"))
}

func cmdAGGREGATORGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	aggregatorsMu.RLock()
	a, exists := aggregators[name]
	aggregatorsMu.RUnlock()
	if !exists || len(a.Values) == 0 {
		return ctx.WriteBulkString("0")
	}
	switch a.Type {
	case "sum":
		var sum float64
		for _, v := range a.Values {
			sum += v
		}
		return ctx.WriteBulkString(fmt.Sprintf("%.6f", sum))
	case "avg":
		var sum float64
		for _, v := range a.Values {
			sum += v
		}
		return ctx.WriteBulkString(fmt.Sprintf("%.6f", sum/float64(len(a.Values))))
	case "min":
		min := a.Values[0]
		for _, v := range a.Values {
			if v < min {
				min = v
			}
		}
		return ctx.WriteBulkString(fmt.Sprintf("%.6f", min))
	case "max":
		max := a.Values[0]
		for _, v := range a.Values {
			if v > max {
				max = v
			}
		}
		return ctx.WriteBulkString(fmt.Sprintf("%.6f", max))
	case "count":
		return ctx.WriteInteger(int64(len(a.Values)))
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown aggregator type"))
	}
}

func cmdAGGREGATORRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	aggregatorsMu.Lock()
	defer aggregatorsMu.Unlock()
	if a, exists := aggregators[name]; exists {
		a.Values = make([]float64, 0)
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR aggregator not found"))
}

var (
	windowsX    = make(map[string]*WindowX)
	windowsXMux sync.RWMutex
)

type WindowX struct {
	Name string
	Size int
	Data []float64
}

func cmdWINDOWXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	size := int(parseInt64(ctx.ArgString(1)))
	windowsXMux.Lock()
	windowsX[name] = &WindowX{Name: name, Size: size, Data: make([]float64, 0)}
	windowsXMux.Unlock()
	return ctx.WriteOK()
}

func cmdWINDOWXADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := parseFloatExt([]byte(ctx.ArgString(1)))
	windowsXMux.Lock()
	defer windowsXMux.Unlock()
	w, exists := windowsX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR window not found"))
	}
	w.Data = append(w.Data, value)
	if len(w.Data) > w.Size {
		w.Data = w.Data[1:]
	}
	return ctx.WriteOK()
}

func cmdWINDOWXGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	windowsXMux.RLock()
	w, exists := windowsX[name]
	windowsXMux.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, len(w.Data))
	for i, v := range w.Data {
		results[i] = resp.BulkString(fmt.Sprintf("%.6f", v))
	}
	return ctx.WriteArray(results)
}

func cmdWINDOWXAGGREGATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	aggType := ctx.ArgString(1)
	windowsXMux.RLock()
	w, exists := windowsX[name]
	windowsXMux.RUnlock()
	if !exists || len(w.Data) == 0 {
		return ctx.WriteBulkString("0")
	}
	switch aggType {
	case "sum":
		var sum float64
		for _, v := range w.Data {
			sum += v
		}
		return ctx.WriteBulkString(fmt.Sprintf("%.6f", sum))
	case "avg":
		var sum float64
		for _, v := range w.Data {
			sum += v
		}
		return ctx.WriteBulkString(fmt.Sprintf("%.6f", sum/float64(len(w.Data))))
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown aggregation type"))
	}
}

var (
	joinsX    = make(map[string]*JoinX)
	joinsXMux sync.RWMutex
)

type JoinX struct {
	Name  string
	Left  []string
	Right []string
}

func cmdJOINXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	joinsXMux.Lock()
	joinsX[name] = &JoinX{Name: name, Left: make([]string, 0), Right: make([]string, 0)}
	joinsXMux.Unlock()
	return ctx.WriteOK()
}

func cmdJOINXADD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	side := ctx.ArgString(1)
	value := ctx.ArgString(2)
	joinsXMux.Lock()
	defer joinsXMux.Unlock()
	j, exists := joinsX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR join not found"))
	}
	if side == "left" {
		j.Left = append(j.Left, value)
	} else {
		j.Right = append(j.Right, value)
	}
	return ctx.WriteOK()
}

func cmdJOINXGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	joinsXMux.RLock()
	j, exists := joinsX[name]
	joinsXMux.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	for _, l := range j.Left {
		for _, r := range j.Right {
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.BulkString("left"), resp.BulkString(l),
				resp.BulkString("right"), resp.BulkString(r),
			}))
		}
	}
	return ctx.WriteArray(results)
}

func cmdJOINXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	joinsXMux.Lock()
	defer joinsXMux.Unlock()
	if _, exists := joinsX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(joinsX, name)
	return ctx.WriteInteger(1)
}

var (
	shuffles   = make(map[string]*Shuffle)
	shufflesMu sync.RWMutex
)

type Shuffle struct {
	Name string
	Data []string
}

func cmdSHUFFLECREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	shufflesMu.Lock()
	shuffles[name] = &Shuffle{Name: name, Data: make([]string, 0)}
	shufflesMu.Unlock()
	return ctx.WriteOK()
}

func cmdSHUFFLEADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := ctx.ArgString(1)
	shufflesMu.Lock()
	defer shufflesMu.Unlock()
	if s, exists := shuffles[name]; exists {
		s.Data = append(s.Data, value)
		return ctx.WriteOK()
	}
	return ctx.WriteError(fmt.Errorf("ERR shuffle not found"))
}

func cmdSHUFFLEGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	shufflesMu.Lock()
	defer shufflesMu.Unlock()
	s, exists := shuffles[name]
	if !exists || len(s.Data) == 0 {
		return ctx.WriteNull()
	}
	val := s.Data[0]
	s.Data = s.Data[1:]
	return ctx.WriteBulkString(val)
}

var (
	partitionsX    = make(map[string]*PartitionX)
	partitionsXMux sync.RWMutex
)

type PartitionX struct {
	Name  string
	Count int
	Data  [][]string
}

func cmdPARTITIONXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	count := int(parseInt64(ctx.ArgString(1)))
	partitionsXMux.Lock()
	data := make([][]string, count)
	for i := range data {
		data[i] = make([]string, 0)
	}
	partitionsX[name] = &PartitionX{Name: name, Count: count, Data: data}
	partitionsXMux.Unlock()
	return ctx.WriteOK()
}

func cmdPARTITIONXADD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)
	partitionsXMux.Lock()
	defer partitionsXMux.Unlock()
	p, exists := partitionsX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR partition not found"))
	}
	idx := hashString(key) % p.Count
	p.Data[idx] = append(p.Data[idx], value)
	return ctx.WriteInteger(int64(idx))
}

func cmdPARTITIONXGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	idx := int(parseInt64(ctx.ArgString(1)))
	partitionsXMux.RLock()
	p, exists := partitionsX[name]
	partitionsXMux.RUnlock()
	if !exists || idx < 0 || idx >= p.Count {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, len(p.Data[idx]))
	for i, v := range p.Data[idx] {
		results[i] = resp.BulkString(v)
	}
	return ctx.WriteArray(results)
}

func cmdPARTITIONXREBALANCE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteOK()
}
