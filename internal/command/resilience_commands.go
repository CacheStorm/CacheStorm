package command

import (
	"errors"
	"fmt"
	"math/rand"
	"runtime"
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

	router.Register(&CommandDef{Name: "BULKHEAD.CREATE", Handler: cmdBULKHEADCREATE})
	router.Register(&CommandDef{Name: "BULKHEAD.ACQUIRE", Handler: cmdBULKHEADACQUIRE})
	router.Register(&CommandDef{Name: "BULKHEAD.RELEASE", Handler: cmdBULKHEADRELEASE})
	router.Register(&CommandDef{Name: "BULKHEAD.STATUS", Handler: cmdBULKHEADSTATUS})
	router.Register(&CommandDef{Name: "BULKHEAD.DELETE", Handler: cmdBULKHEADDELETE})

	router.Register(&CommandDef{Name: "TELEMETRY.RECORD", Handler: cmdTELEMETRYRECORD})
	router.Register(&CommandDef{Name: "TELEMETRY.QUERY", Handler: cmdTELEMETRYQUERY})

	router.Register(&CommandDef{Name: "PROFILEX.LIST", Handler: cmdPROFILEXLIST})

	router.Register(&CommandDef{Name: "HEAP.STATS", Handler: cmdHEAPSTATS})
	router.Register(&CommandDef{Name: "HEAP.GC", Handler: cmdHEAPGC})

	router.Register(&CommandDef{Name: "LOCKX.ACQUIRE", Handler: cmdLOCKXACQUIRE})
	router.Register(&CommandDef{Name: "LOCKX.RELEASE", Handler: cmdLOCKXRELEASE})
	router.Register(&CommandDef{Name: "LOCKX.EXTEND", Handler: cmdLOCKXEXTEND})
	router.Register(&CommandDef{Name: "LOCKX.STATUS", Handler: cmdLOCKXSTATUS})

	router.Register(&CommandDef{Name: "SEMAPHOREX.CREATE", Handler: cmdSEMAPHOREXCREATE})
	router.Register(&CommandDef{Name: "SEMAPHOREX.ACQUIRE", Handler: cmdSEMAPHOREXACQUIRE})
	router.Register(&CommandDef{Name: "SEMAPHOREX.RELEASE", Handler: cmdSEMAPHOREXRELEASE})
	router.Register(&CommandDef{Name: "SEMAPHOREX.STATUS", Handler: cmdSEMAPHOREXSTATUS})

	router.Register(&CommandDef{Name: "PROMISE.CREATE", Handler: cmdPROMISECREATE})
	router.Register(&CommandDef{Name: "PROMISE.RESOLVE", Handler: cmdPROMISERESOLVE})
	router.Register(&CommandDef{Name: "PROMISE.REJECT", Handler: cmdPROMISEREJECT})
	router.Register(&CommandDef{Name: "PROMISE.STATUS", Handler: cmdPROMISESTATUS})
	router.Register(&CommandDef{Name: "PROMISE.AWAIT", Handler: cmdPROMISEAWAIT})

	router.Register(&CommandDef{Name: "FUTURE.CREATE", Handler: cmdFUTURECREATE})
	router.Register(&CommandDef{Name: "FUTURE.COMPLETE", Handler: cmdFUTURECOMPLETE})
	router.Register(&CommandDef{Name: "FUTURE.GET", Handler: cmdFUTUREGET})
	router.Register(&CommandDef{Name: "FUTURE.CANCEL", Handler: cmdFUTURECANCEL})

	router.Register(&CommandDef{Name: "STREAMPROC.CREATE", Handler: cmdSTREAMPROCCREATE})
	router.Register(&CommandDef{Name: "STREAMPROC.PUSH", Handler: cmdSTREAMPROCPUSH})
	router.Register(&CommandDef{Name: "STREAMPROC.POP", Handler: cmdSTREAMPROCPOP})
	router.Register(&CommandDef{Name: "STREAMPROC.PEEK", Handler: cmdSTREAMPROCPEEK})
	router.Register(&CommandDef{Name: "STREAMPROC.DELETE", Handler: cmdSTREAMPROCDELETE})

	router.Register(&CommandDef{Name: "EVENTSOURCING.APPEND", Handler: cmdEVENTSOURCINGAPPEND})
	router.Register(&CommandDef{Name: "EVENTSOURCING.REPLAY", Handler: cmdEVENTSOURCINGREPLAY})
	router.Register(&CommandDef{Name: "EVENTSOURCING.GET", Handler: cmdEVENTSOURCINGGET})

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
	if _, exists := circuits[name]; !exists && len(circuits) >= 10000 {
		circuitsMu.Unlock()
		return ctx.WriteError(errors.New("ERR maximum number of circuits reached"))
	}
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
	if !exists {
		reply := fmt.Errorf("ERR circuit not found")
		circuitsMu.RUnlock()
		return ctx.WriteError(reply)
	}
	reply := []*resp.Value{
		resp.BulkString("name"), resp.BulkString(c.Name),
		resp.BulkString("state"), resp.BulkString(c.State),
		resp.BulkString("failures"), resp.IntegerValue(c.Failures),
		resp.BulkString("successes"), resp.IntegerValue(c.Successes),
	}
	circuitsMu.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdCIRCUITXMETRICS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	circuitsMu.RLock()
	c, exists := circuits[name]
	if !exists {
		reply := fmt.Errorf("ERR circuit not found")
		circuitsMu.RUnlock()
		return ctx.WriteError(reply)
	}
	reply := []*resp.Value{
		resp.BulkString("total_failures"), resp.IntegerValue(c.Failures),
		resp.BulkString("total_successes"), resp.IntegerValue(c.Successes),
		resp.BulkString("failure_rate"), resp.BulkString(fmt.Sprintf("%.2f", float64(c.Failures)/float64(c.Failures+c.Successes+1))),
	}
	circuitsMu.RUnlock()
	return ctx.WriteArray(reply)
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
	if _, exists := rateLimitersX[name]; !exists && len(rateLimitersX) >= 10000 {
		rateLimitersXMux.Unlock()
		return ctx.WriteError(errors.New("ERR maximum number of rate limiters reached"))
	}
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
	if !exists {
		reply := fmt.Errorf("ERR rate limiter not found")
		rateLimitersXMux.RUnlock()
		return ctx.WriteError(reply)
	}
	reply := []*resp.Value{
		resp.BulkString("name"), resp.BulkString(rl.Name),
		resp.BulkString("limit"), resp.IntegerValue(rl.Limit),
		resp.BulkString("current"), resp.IntegerValue(int64(len(rl.Requests))),
		resp.BulkString("strategy"), resp.BulkString(rl.Strategy),
	}
	rateLimitersXMux.RUnlock()
	return ctx.WriteArray(reply)
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

type Timeout struct {
	Name    string
	Timeout int64
	Created int64
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
	if !exists {
		reply := fmt.Errorf("ERR bulkhead not found")
		bulkheadsMu.RUnlock()
		return ctx.WriteError(reply)
	}
	reply := []*resp.Value{
		resp.BulkString("name"), resp.BulkString(b.Name),
		resp.BulkString("current"), resp.IntegerValue(int64(b.Current)),
		resp.BulkString("max"), resp.IntegerValue(int64(b.MaxConcurrent)),
		resp.BulkString("waiting"), resp.IntegerValue(int64(b.WaitQueue)),
	}
	bulkheadsMu.RUnlock()
	return ctx.WriteArray(reply)
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

func cmdPROFILEXLIST(ctx *Context) error {
	profilesX2Mu.RLock()
	results := make([]*resp.Value, 0)
	for id := range profilesX2 {
		results = append(results, resp.BulkString(id))
	}
	reply := results
	profilesX2Mu.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdHEAPSTATS(ctx *Context) error {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("allocations"), resp.IntegerValue(int64(ms.Mallocs)),
		resp.BulkString("frees"), resp.IntegerValue(int64(ms.Frees)),
		resp.BulkString("in_use"), resp.IntegerValue(int64(ms.HeapInuse)),
		resp.BulkString("heap_alloc"), resp.IntegerValue(int64(ms.HeapAlloc)),
		resp.BulkString("heap_sys"), resp.IntegerValue(int64(ms.HeapSys)),
		resp.BulkString("gc_cycles"), resp.IntegerValue(int64(ms.NumGC)),
	})
}

func cmdHEAPGC(ctx *Context) error {
	runtime.GC()
	return ctx.WriteOK()
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
	if !exists || time.Now().UnixMilli() >= l.ExpiresAt {
		reply := []*resp.Value{
			resp.BulkString("locked"), resp.IntegerValue(0),
		}
		locksXMux.RUnlock()
		return ctx.WriteArray(reply)
	}
	reply := []*resp.Value{
		resp.BulkString("locked"), resp.IntegerValue(1),
		resp.BulkString("holder"), resp.BulkString(l.Holder),
		resp.BulkString("expires_at"), resp.IntegerValue(l.ExpiresAt),
	}
	locksXMux.RUnlock()
	return ctx.WriteArray(reply)
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
	if !exists {
		reply := fmt.Errorf("ERR semaphore not found")
		semaphoresXMux.RUnlock()
		return ctx.WriteError(reply)
	}
	reply := []*resp.Value{
		resp.BulkString("name"), resp.BulkString(s.Name),
		resp.BulkString("current"), resp.IntegerValue(s.Current),
		resp.BulkString("max"), resp.IntegerValue(s.Max),
		resp.BulkString("available"), resp.IntegerValue(s.Max - s.Current),
	}
	semaphoresXMux.RUnlock()
	return ctx.WriteArray(reply)
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
	if !exists {
		reply := fmt.Errorf("ERR promise not found")
		promisesMu.RUnlock()
		return ctx.WriteError(reply)
	}
	reply := []*resp.Value{
		resp.BulkString("id"), resp.BulkString(p.ID),
		resp.BulkString("status"), resp.BulkString(p.Status),
	}
	promisesMu.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdPROMISEAWAIT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	promisesMu.RLock()
	p, exists := promises[id]
	if !exists {
		reply := fmt.Errorf("ERR promise not found")
		promisesMu.RUnlock()
		return ctx.WriteError(reply)
	}
	if p.Status == "pending" {
		promisesMu.RUnlock()
		return ctx.WriteNull()
	}
	if p.Status == "resolved" {
		reply := p.Value
		promisesMu.RUnlock()
		return ctx.WriteBulkString(reply)
	}
	reply := errors.New(p.Error)
	promisesMu.RUnlock()
	return ctx.WriteError(reply)
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
	if !exists {
		reply := fmt.Errorf("ERR future not found")
		futuresMu.RUnlock()
		return ctx.WriteError(reply)
	}
	if f.Status == "pending" {
		futuresMu.RUnlock()
		return ctx.WriteNull()
	}
	if f.Status == "completed" {
		reply := f.Value
		futuresMu.RUnlock()
		return ctx.WriteBulkString(reply)
	}
	reply := errors.New(f.Error)
	futuresMu.RUnlock()
	return ctx.WriteError(reply)
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
	if !exists || len(s.Data) == 0 {
		streamProcsMu.RUnlock()
		return ctx.WriteNull()
	}
	reply := s.Data[0]
	streamProcsMu.RUnlock()
	return ctx.WriteBulkString(reply)
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
	if !exists {
		reply := fmt.Errorf("ERR backpressure not found")
		backpressuresMu.RUnlock()
		return ctx.WriteError(reply)
	}
	reply := []*resp.Value{
		resp.BulkString("name"), resp.BulkString(b.Name),
		resp.BulkString("current"), resp.IntegerValue(b.Current),
		resp.BulkString("high_water"), resp.IntegerValue(b.HighWater),
		resp.BulkString("low_water"), resp.IntegerValue(b.LowWater),
		resp.BulkString("paused"), resp.BulkString(fmt.Sprintf("%v", b.Paused)),
	}
	backpressuresMu.RUnlock()
	return ctx.WriteArray(reply)
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
	if !exists || len(c.Values) == 0 {
		coalescesMu.RUnlock()
		return ctx.WriteNull()
	}
	reply := c.Values[0]
	coalescesMu.RUnlock()
	return ctx.WriteBulkString(reply)
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
	if !exists || len(a.Values) == 0 {
		aggregatorsMu.RUnlock()
		return ctx.WriteBulkString("0")
	}
	// The reply is built while the lock is held and written after it is
	// released, so a slow client cannot block the registry's writers.
	var reply *resp.Value
	switch a.Type {
	case "sum":
		var sum float64
		for _, v := range a.Values {
			sum += v
		}
		reply = resp.BulkString(fmt.Sprintf("%.6f", sum))
	case "avg":
		var sum float64
		for _, v := range a.Values {
			sum += v
		}
		reply = resp.BulkString(fmt.Sprintf("%.6f", sum/float64(len(a.Values))))
	case "min":
		min := a.Values[0]
		for _, v := range a.Values {
			if v < min {
				min = v
			}
		}
		reply = resp.BulkString(fmt.Sprintf("%.6f", min))
	case "max":
		max := a.Values[0]
		for _, v := range a.Values {
			if v > max {
				max = v
			}
		}
		reply = resp.BulkString(fmt.Sprintf("%.6f", max))
	case "count":
		reply = resp.IntegerValue(int64(len(a.Values)))
	default:
		aggregatorsMu.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR unknown aggregator type"))
	}
	aggregatorsMu.RUnlock()
	return ctx.WriteValue(reply)
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
	if !exists {
		reply := []*resp.Value{}
		windowsXMux.RUnlock()
		return ctx.WriteArray(reply)
	}
	results := make([]*resp.Value, len(w.Data))
	for i, v := range w.Data {
		results[i] = resp.BulkString(fmt.Sprintf("%.6f", v))
	}
	reply := results
	windowsXMux.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdWINDOWXAGGREGATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	aggType := ctx.ArgString(1)
	if aggType != "sum" && aggType != "avg" {
		return ctx.WriteError(fmt.Errorf("ERR unknown aggregation type"))
	}
	windowsXMux.RLock()
	w, exists := windowsX[name]
	if !exists || len(w.Data) == 0 {
		windowsXMux.RUnlock()
		return ctx.WriteBulkString("0")
	}
	var sum float64
	for _, v := range w.Data {
		sum += v
	}
	n := len(w.Data)
	windowsXMux.RUnlock()
	if aggType == "avg" {
		sum /= float64(n)
	}
	return ctx.WriteBulkString(fmt.Sprintf("%.6f", sum))
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
	// JoinX stores only Name/Left/Right; there is nowhere to record a join
	// type or condition. Accepting those arguments and replying +OK would
	// report success for configuration that is silently dropped.
	if ctx.ArgCount() > 1 {
		return ctx.WriteError(fmt.Errorf("ERR JOINX.CREATE does not support type or condition arguments"))
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
	if !exists {
		reply := []*resp.Value{}
		joinsXMux.RUnlock()
		return ctx.WriteArray(reply)
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
	reply := results
	joinsXMux.RUnlock()
	return ctx.WriteArray(reply)
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
	last := len(s.Data) - 1
	i := rand.Intn(len(s.Data))
	s.Data[i], s.Data[last] = s.Data[last], s.Data[i]
	val := s.Data[last]
	s.Data = s.Data[:last]
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
	if count <= 0 {
		return ctx.WriteError(fmt.Errorf("ERR count must be positive"))
	}
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
