package command

import (
	"fmt"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func RegisterMoreCommands(router *Router) {
	router.Register(&CommandDef{Name: "SLIDING.CREATE", Handler: cmdSLIDINGCREATE})
	router.Register(&CommandDef{Name: "SLIDING.CHECK", Handler: cmdSLIDINGCHECK})
	router.Register(&CommandDef{Name: "SLIDING.RESET", Handler: cmdSLIDINGRESET})
	router.Register(&CommandDef{Name: "SLIDING.DELETE", Handler: cmdSLIDINGDELETE})
	router.Register(&CommandDef{Name: "SLIDING.STATS", Handler: cmdSLIDINGSTATS})

	router.Register(&CommandDef{Name: "BUCKETX.CREATE", Handler: cmdBUCKETXCREATE})
	router.Register(&CommandDef{Name: "BUCKETX.TAKE", Handler: cmdBUCKETXTAKE})
	router.Register(&CommandDef{Name: "BUCKETX.RETURN", Handler: cmdBUCKETXRETURN})
	router.Register(&CommandDef{Name: "BUCKETX.REFILL", Handler: cmdBUCKETXREFILL})
	router.Register(&CommandDef{Name: "BUCKETX.DELETE", Handler: cmdBUCKETXDELETE})

	router.Register(&CommandDef{Name: "IDEMPOTENCY.SET", Handler: cmdIDEMPOTENCYSET})
	router.Register(&CommandDef{Name: "IDEMPOTENCY.GET", Handler: cmdIDEMPOTENCYGET})
	router.Register(&CommandDef{Name: "IDEMPOTENCY.CHECK", Handler: cmdIDEMPOTENCYCHECK})
	router.Register(&CommandDef{Name: "IDEMPOTENCY.DELETE", Handler: cmdIDEMPOTENCYDELETE})
	router.Register(&CommandDef{Name: "IDEMPOTENCY.LIST", Handler: cmdIDEMPOTENCYLIST})

	router.Register(&CommandDef{Name: "EXPERIMENT.CREATE", Handler: cmdEXPERIMENTCREATE})
	router.Register(&CommandDef{Name: "EXPERIMENT.DELETE", Handler: cmdEXPERIMENTDELETE})
	router.Register(&CommandDef{Name: "EXPERIMENT.ASSIGN", Handler: cmdEXPERIMENTASSIGN})
	router.Register(&CommandDef{Name: "EXPERIMENT.TRACK", Handler: cmdEXPERIMENTTRACK})
	router.Register(&CommandDef{Name: "EXPERIMENT.RESULTS", Handler: cmdEXPERIMENTRESULTS})
	router.Register(&CommandDef{Name: "EXPERIMENT.LIST", Handler: cmdEXPERIMENTLIST})

	router.Register(&CommandDef{Name: "ROLLOUT.CREATE", Handler: cmdROLLOUTCREATE})
	router.Register(&CommandDef{Name: "ROLLOUT.DELETE", Handler: cmdROLLOUTDELETE})
	router.Register(&CommandDef{Name: "ROLLOUT.CHECK", Handler: cmdROLLOUTCHECK})
	router.Register(&CommandDef{Name: "ROLLOUT.PERCENTAGE", Handler: cmdROLLOUTPERCENTAGE})
	router.Register(&CommandDef{Name: "ROLLOUT.LIST", Handler: cmdROLLOUTLIST})

	router.Register(&CommandDef{Name: "SCHEMA.REGISTER", Handler: cmdSCHEMAREGISTER})
	router.Register(&CommandDef{Name: "SCHEMA.VALIDATE", Handler: cmdSCHEMAVALIDATE})
	router.Register(&CommandDef{Name: "SCHEMA.DELETE", Handler: cmdSCHEMADELETE})
	router.Register(&CommandDef{Name: "SCHEMA.LIST", Handler: cmdSCHEMALIST})

	router.Register(&CommandDef{Name: "PIPELINE.CREATE", Handler: cmdPIPELINECREATE})
	router.Register(&CommandDef{Name: "PIPELINE.ADDSTAGE", Handler: cmdPIPELINEADDSTAGE})
	router.Register(&CommandDef{Name: "PIPELINE.EXECUTE", Handler: cmdPIPELINEEXECUTE})
	router.Register(&CommandDef{Name: "PIPELINE.STATUS", Handler: cmdPIPELINESTATUS})
	router.Register(&CommandDef{Name: "PIPELINE.DELETE", Handler: cmdPIPELINEDELETE})
	router.Register(&CommandDef{Name: "PIPELINE.LIST", Handler: cmdPIPELINELIST})

	router.Register(&CommandDef{Name: "NOTIFY.CREATE", Handler: cmdNOTIFYCREATE})
	router.Register(&CommandDef{Name: "NOTIFY.SEND", Handler: cmdNOTIFYSEND})
	router.Register(&CommandDef{Name: "NOTIFY.LIST", Handler: cmdNOTIFYLIST})
	router.Register(&CommandDef{Name: "NOTIFY.DELETE", Handler: cmdNOTIFYDELETE})
	router.Register(&CommandDef{Name: "NOTIFY.TEMPLATE", Handler: cmdNOTIFYTEMPLATE})

	router.Register(&CommandDef{Name: "ALERT.CREATE", Handler: cmdALERTCREATE})
	router.Register(&CommandDef{Name: "ALERT.TRIGGER", Handler: cmdALERTTRIGGER})
	router.Register(&CommandDef{Name: "ALERT.ACKNOWLEDGE", Handler: cmdALERTACKNOWLEDGE})
	router.Register(&CommandDef{Name: "ALERT.RESOLVE", Handler: cmdALERTRESOLVE})
	router.Register(&CommandDef{Name: "ALERT.LIST", Handler: cmdALERTLIST})
	router.Register(&CommandDef{Name: "ALERT.HISTORY", Handler: cmdALERTHISTORY})

	router.Register(&CommandDef{Name: "COUNTERX.CREATE", Handler: cmdCOUNTERXCREATE})
	router.Register(&CommandDef{Name: "COUNTERX.INCR", Handler: cmdCOUNTERXINCR})
	router.Register(&CommandDef{Name: "COUNTERX.DECR", Handler: cmdCOUNTERXDECR})
	router.Register(&CommandDef{Name: "COUNTERX.GET", Handler: cmdCOUNTERXGET})
	router.Register(&CommandDef{Name: "COUNTERX.RESET", Handler: cmdCOUNTERXRESET})
	router.Register(&CommandDef{Name: "COUNTERX.DELETE", Handler: cmdCOUNTERXDELETE})

	router.Register(&CommandDef{Name: "GAUGE.CREATE", Handler: cmdGAUGECREATE})
	router.Register(&CommandDef{Name: "GAUGE.SET", Handler: cmdGAUGESET})
	router.Register(&CommandDef{Name: "GAUGE.GET", Handler: cmdGAUGEGET})
	router.Register(&CommandDef{Name: "GAUGE.INCR", Handler: cmdGAUGEINCR})
	router.Register(&CommandDef{Name: "GAUGE.DECR", Handler: cmdGAUGEDECR})
	router.Register(&CommandDef{Name: "GAUGE.DELETE", Handler: cmdGAUGEDELETE})

	router.Register(&CommandDef{Name: "TRACE.START", Handler: cmdTRACESTART})
	router.Register(&CommandDef{Name: "TRACE.SPAN", Handler: cmdTRACESPAN})
	router.Register(&CommandDef{Name: "TRACE.END", Handler: cmdTRACEEND})
	router.Register(&CommandDef{Name: "TRACE.GET", Handler: cmdTRACEGET})
	router.Register(&CommandDef{Name: "TRACE.LIST", Handler: cmdTRACELIST})

	router.Register(&CommandDef{Name: "LOGX.WRITE", Handler: cmdLOGXWRITE})
	router.Register(&CommandDef{Name: "LOGX.READ", Handler: cmdLOGXREAD})
	router.Register(&CommandDef{Name: "LOGX.SEARCH", Handler: cmdLOGXSEARCH})
	router.Register(&CommandDef{Name: "LOGX.CLEAR", Handler: cmdLOGXCLEAR})
	router.Register(&CommandDef{Name: "LOGX.STATS", Handler: cmdLOGXSTATS})

	router.Register(&CommandDef{Name: "APIKEY.CREATE", Handler: cmdAPIKEYCREATE})
	router.Register(&CommandDef{Name: "APIKEY.VALIDATE", Handler: cmdAPIKEYVALIDATE})
	router.Register(&CommandDef{Name: "APIKEY.REVOKE", Handler: cmdAPIKEYREVOKE})
	router.Register(&CommandDef{Name: "APIKEY.LIST", Handler: cmdAPIKEYLIST})
	router.Register(&CommandDef{Name: "APIKEY.USAGE", Handler: cmdAPIKEYUSAGE})

	router.Register(&CommandDef{Name: "QUOTAX.CREATE", Handler: cmdQUOTAXCREATE})
	router.Register(&CommandDef{Name: "QUOTAX.CHECK", Handler: cmdQUOTAXCHECK})
	router.Register(&CommandDef{Name: "QUOTAX.USAGE", Handler: cmdQUOTAXUSAGE})
	router.Register(&CommandDef{Name: "QUOTAX.RESET", Handler: cmdQUOTAXRESET})
	router.Register(&CommandDef{Name: "QUOTAX.DELETE", Handler: cmdQUOTAXDELETE})

	router.Register(&CommandDef{Name: "METER.CREATE", Handler: cmdMETERCREATE})
	router.Register(&CommandDef{Name: "METER.RECORD", Handler: cmdMETERRECORD})
	router.Register(&CommandDef{Name: "METER.GET", Handler: cmdMETERGET})
	router.Register(&CommandDef{Name: "METER.BILLING", Handler: cmdMETERBILLING})
	router.Register(&CommandDef{Name: "METER.DELETE", Handler: cmdMETERDELETE})

	router.Register(&CommandDef{Name: "TENANT.CREATE", Handler: cmdTENANTCREATE})
	router.Register(&CommandDef{Name: "TENANT.DELETE", Handler: cmdTENANTDELETE})
	router.Register(&CommandDef{Name: "TENANT.GET", Handler: cmdTENANTGET})
	router.Register(&CommandDef{Name: "TENANT.LIST", Handler: cmdTENANTLIST})
	router.Register(&CommandDef{Name: "TENANT.CONFIG", Handler: cmdTENANTCONFIG})

	router.Register(&CommandDef{Name: "LEASE.CREATE", Handler: cmdLEASECREATE})
	router.Register(&CommandDef{Name: "LEASE.RENEW", Handler: cmdLEASERENEW})
	router.Register(&CommandDef{Name: "LEASE.REVOKE", Handler: cmdLEASEREVOKE})
	router.Register(&CommandDef{Name: "LEASE.GET", Handler: cmdLEASEGET})
	router.Register(&CommandDef{Name: "LEASE.LIST", Handler: cmdLEASELIST})

	router.Register(&CommandDef{Name: "HEAP.PUSH", Handler: cmdHEAPPUSH})
	router.Register(&CommandDef{Name: "HEAP.POP", Handler: cmdHEAPPOP})
	router.Register(&CommandDef{Name: "HEAP.PEEK", Handler: cmdHEAPPEEK})
	router.Register(&CommandDef{Name: "HEAP.SIZE", Handler: cmdHEAPSIZE})
	router.Register(&CommandDef{Name: "HEAP.DELETE", Handler: cmdHEAPDELETE})

	router.Register(&CommandDef{Name: "BLOOMX.CREATE", Handler: cmdBLOOMXCREATE})
	router.Register(&CommandDef{Name: "BLOOMX.ADD", Handler: cmdBLOOMXADD})
	router.Register(&CommandDef{Name: "BLOOMX.CHECK", Handler: cmdBLOOMXCHECK})
	router.Register(&CommandDef{Name: "BLOOMX.INFO", Handler: cmdBLOOMXINFO})
	router.Register(&CommandDef{Name: "BLOOMX.DELETE", Handler: cmdBLOOMXDELETE})

	router.Register(&CommandDef{Name: "SKETCH.CREATE", Handler: cmdSKETCHCREATE})
	router.Register(&CommandDef{Name: "SKETCH.UPDATE", Handler: cmdSKETCHUPDATE})
	router.Register(&CommandDef{Name: "SKETCH.QUERY", Handler: cmdSKETCHQUERY})
	router.Register(&CommandDef{Name: "SKETCH.MERGE", Handler: cmdSKETCHMERGE})
	router.Register(&CommandDef{Name: "SKETCH.DELETE", Handler: cmdSKETCHDELETE})

	router.Register(&CommandDef{Name: "RINGBUFFER.CREATE", Handler: cmdRINGBUFFERCREATE})
	router.Register(&CommandDef{Name: "RINGBUFFER.WRITE", Handler: cmdRINGBUFFERWRITE})
	router.Register(&CommandDef{Name: "RINGBUFFER.READ", Handler: cmdRINGBUFFERREAD})
	router.Register(&CommandDef{Name: "RINGBUFFER.SIZE", Handler: cmdRINGBUFFERSIZE})
	router.Register(&CommandDef{Name: "RINGBUFFER.DELETE", Handler: cmdRINGBUFFERDELETE})

	router.Register(&CommandDef{Name: "WINDOW.CREATE", Handler: cmdWINDOWCREATE})
	router.Register(&CommandDef{Name: "WINDOW.ADD", Handler: cmdWINDOWADD})
	router.Register(&CommandDef{Name: "WINDOW.GET", Handler: cmdWINDOWGET})
	router.Register(&CommandDef{Name: "WINDOW.AGGREGATE", Handler: cmdWINDOWAGGREGATE})
	router.Register(&CommandDef{Name: "WINDOW.DELETE", Handler: cmdWINDOWDELETE})

	router.Register(&CommandDef{Name: "FREQ.CREATE", Handler: cmdFREQCREATE})
	router.Register(&CommandDef{Name: "FREQ.ADD", Handler: cmdFREQADD})
	router.Register(&CommandDef{Name: "FREQ.COUNT", Handler: cmdFREQCOUNT})
	router.Register(&CommandDef{Name: "FREQ.TOP", Handler: cmdFREQTOP})
	router.Register(&CommandDef{Name: "FREQ.DELETE", Handler: cmdFREQDELETE})

	router.Register(&CommandDef{Name: "PARTITION.CREATE", Handler: cmdPARTITIONCREATE})
	router.Register(&CommandDef{Name: "PARTITION.ADD", Handler: cmdPARTITIONADD})
	router.Register(&CommandDef{Name: "PARTITION.GET", Handler: cmdPARTITIONGET})
	router.Register(&CommandDef{Name: "PARTITION.LIST", Handler: cmdPARTITIONLIST})
	router.Register(&CommandDef{Name: "PARTITION.DELETE", Handler: cmdPARTITIONDELETE})
}

var (
	slidingWindows   = make(map[string]*SlidingWindow)
	slidingWindowsMu sync.RWMutex
)

type SlidingWindow struct {
	Name    string
	Limit   int64
	Window  int64
	Entries []int64
}

func cmdSLIDINGCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	limit := parseInt64(ctx.ArgString(1))
	windowMs := parseInt64(ctx.ArgString(2))
	slidingWindowsMu.Lock()
	slidingWindows[name] = &SlidingWindow{Name: name, Limit: limit, Window: windowMs, Entries: make([]int64, 0)}
	slidingWindowsMu.Unlock()
	return ctx.WriteOK()
}

func cmdSLIDINGCHECK(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	slidingWindowsMu.Lock()
	defer slidingWindowsMu.Unlock()
	sw, exists := slidingWindows[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR sliding window not found"))
	}
	now := time.Now().UnixMilli()
	cutoff := now - sw.Window
	newEntries := make([]int64, 0)
	for _, ts := range sw.Entries {
		if ts > cutoff {
			newEntries = append(newEntries, ts)
		}
	}
	sw.Entries = newEntries
	if int64(len(sw.Entries)) >= sw.Limit {
		return ctx.WriteArray([]*resp.Value{
			resp.IntegerValue(0),
			resp.IntegerValue(0),
			resp.IntegerValue(sw.Window - (now - newEntries[0])),
		})
	}
	sw.Entries = append(sw.Entries, now)
	return ctx.WriteArray([]*resp.Value{
		resp.IntegerValue(1),
		resp.IntegerValue(sw.Limit - int64(len(sw.Entries))),
		resp.IntegerValue(sw.Window),
	})
}

func cmdSLIDINGRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	slidingWindowsMu.Lock()
	defer slidingWindowsMu.Unlock()
	sw, exists := slidingWindows[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR sliding window not found"))
	}
	sw.Entries = make([]int64, 0)
	return ctx.WriteOK()
}

func cmdSLIDINGDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	slidingWindowsMu.Lock()
	defer slidingWindowsMu.Unlock()
	if _, exists := slidingWindows[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(slidingWindows, name)
	return ctx.WriteInteger(1)
}

func cmdSLIDINGSTATS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	// Read the window under the read lock. The lock used to be released right
	// after the lookup, so the Entries slice was read with no lock at all
	// while SLIDING.CHECK replaced it under the write lock.
	slidingWindowsMu.RLock()
	sw, exists := slidingWindows[name]
	if !exists {
		slidingWindowsMu.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR sliding window not found"))
	}
	swName := sw.Name
	swLimit := sw.Limit
	swWindow := sw.Window
	swCurrent := int64(len(sw.Entries))
	slidingWindowsMu.RUnlock()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(swName),
		resp.BulkString("limit"), resp.IntegerValue(swLimit),
		resp.BulkString("window_ms"), resp.IntegerValue(swWindow),
		resp.BulkString("current"), resp.IntegerValue(swCurrent),
	})
}

var (
	bucketsX    = make(map[string]*BucketX)
	bucketsXMux sync.RWMutex
)

type BucketX struct {
	Name      string
	Capacity  int64
	Tokens    int64
	Refill    int64
	Interval  int64
	LastCheck int64
}

func cmdBUCKETXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	capacity := parseInt64(ctx.ArgString(1))
	refill := parseInt64(ctx.ArgString(2))
	intervalMs := parseInt64(ctx.ArgString(3))
	bucketsXMux.Lock()
	bucketsX[name] = &BucketX{Name: name, Capacity: capacity, Tokens: capacity, Refill: refill, Interval: intervalMs, LastCheck: time.Now().UnixMilli()}
	bucketsXMux.Unlock()
	return ctx.WriteOK()
}

func cmdBUCKETXTAKE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	tokens := parseInt64(ctx.ArgString(1))
	bucketsXMux.Lock()
	defer bucketsXMux.Unlock()
	tb, exists := bucketsX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR bucket not found"))
	}
	now := time.Now().UnixMilli()
	elapsed := now - tb.LastCheck
	if elapsed >= tb.Interval {
		refills := (elapsed / tb.Interval) * tb.Refill
		tb.Tokens += refills
		if tb.Tokens > tb.Capacity {
			tb.Tokens = tb.Capacity
		}
		tb.LastCheck = now
	}
	if tb.Tokens >= tokens {
		tb.Tokens -= tokens
		return ctx.WriteArray([]*resp.Value{resp.IntegerValue(1), resp.IntegerValue(tb.Tokens)})
	}
	return ctx.WriteArray([]*resp.Value{resp.IntegerValue(0), resp.IntegerValue(tb.Tokens)})
}

func cmdBUCKETXRETURN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	tokens := parseInt64(ctx.ArgString(1))
	bucketsXMux.Lock()
	defer bucketsXMux.Unlock()
	tb, exists := bucketsX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR bucket not found"))
	}
	tb.Tokens += tokens
	if tb.Tokens > tb.Capacity {
		tb.Tokens = tb.Capacity
	}
	return ctx.WriteInteger(tb.Tokens)
}

func cmdBUCKETXREFILL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	bucketsXMux.Lock()
	defer bucketsXMux.Unlock()
	tb, exists := bucketsX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR bucket not found"))
	}
	tb.Tokens = tb.Capacity
	tb.LastCheck = time.Now().UnixMilli()
	return ctx.WriteOK()
}

func cmdBUCKETXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	bucketsXMux.Lock()
	defer bucketsXMux.Unlock()
	if _, exists := bucketsX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(bucketsX, name)
	return ctx.WriteInteger(1)
}

var (
	idempotencyKeys   = make(map[string]*IdempotencyEntry)
	idempotencyKeysMu sync.RWMutex
)

type IdempotencyEntry struct {
	Key       string
	Response  string
	Status    string
	CreatedAt int64
	ExpiresAt int64
}

func cmdIDEMPOTENCYSET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	response := ctx.ArgString(1)
	ttlMs := parseInt64(ctx.ArgString(2))
	idempotencyKeysMu.Lock()
	defer idempotencyKeysMu.Unlock()
	if _, exists := idempotencyKeys[key]; exists {
		return ctx.WriteError(fmt.Errorf("ERR key already exists"))
	}
	idempotencyKeys[key] = &IdempotencyEntry{Key: key, Response: response, Status: "pending", CreatedAt: time.Now().UnixMilli(), ExpiresAt: time.Now().UnixMilli() + ttlMs}
	return ctx.WriteOK()
}

func cmdIDEMPOTENCYGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	idempotencyKeysMu.RLock()
	entry, exists := idempotencyKeys[key]
	idempotencyKeysMu.RUnlock()
	if !exists {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(entry.Response)
}

func cmdIDEMPOTENCYCHECK(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	idempotencyKeysMu.RLock()
	entry, exists := idempotencyKeys[key]
	idempotencyKeysMu.RUnlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	if time.Now().UnixMilli() > entry.ExpiresAt {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(1)
}

func cmdIDEMPOTENCYDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	idempotencyKeysMu.Lock()
	defer idempotencyKeysMu.Unlock()
	if _, exists := idempotencyKeys[key]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(idempotencyKeys, key)
	return ctx.WriteInteger(1)
}

func cmdIDEMPOTENCYLIST(ctx *Context) error {
	idempotencyKeysMu.RLock()
	defer idempotencyKeysMu.RUnlock()
	results := make([]*resp.Value, 0)
	for key := range idempotencyKeys {
		results = append(results, resp.BulkString(key))
	}
	return ctx.WriteArray(results)
}

var (
	experiments   = make(map[string]*Experiment)
	experimentsMu sync.RWMutex
)

type Experiment struct {
	Name        string
	Variants    []string
	Weights     []int64
	Assignments map[string]string
	Events      map[string][]string
}

func cmdEXPERIMENTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	variants := make([]string, 0)
	weights := make([]int64, 0)
	for i := 1; i < ctx.ArgCount(); i++ {
		variants = append(variants, ctx.ArgString(i))
		weights = append(weights, 1)
	}
	experimentsMu.Lock()
	experiments[name] = &Experiment{Name: name, Variants: variants, Weights: weights, Assignments: make(map[string]string), Events: make(map[string][]string)}
	experimentsMu.Unlock()
	return ctx.WriteOK()
}

func cmdEXPERIMENTDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	experimentsMu.Lock()
	defer experimentsMu.Unlock()
	if _, exists := experiments[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(experiments, name)
	return ctx.WriteInteger(1)
}

func cmdEXPERIMENTASSIGN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	userID := ctx.ArgString(1)
	experimentsMu.Lock()
	defer experimentsMu.Unlock()
	exp, exists := experiments[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR experiment not found"))
	}
	if variant, ok := exp.Assignments[userID]; ok {
		return ctx.WriteBulkString(variant)
	}
	idx := hashString(userID) % len(exp.Variants)
	variant := exp.Variants[idx]
	exp.Assignments[userID] = variant
	return ctx.WriteBulkString(variant)
}

func hashString(s string) int {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}

func cmdEXPERIMENTTRACK(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	userID := ctx.ArgString(1)
	event := ctx.ArgString(2)
	experimentsMu.Lock()
	defer experimentsMu.Unlock()
	exp, exists := experiments[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR experiment not found"))
	}
	key := userID + ":" + event
	exp.Events[key] = append(exp.Events[key], fmt.Sprintf("%d", time.Now().UnixMilli()))
	return ctx.WriteOK()
}

func cmdEXPERIMENTRESULTS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	// Iterate the assignments map while holding the read lock. The lock used
	// to be released right after the lookup, so the map was iterated with no
	// lock at all while EXPERIMENT.ASSIGN wrote it under the write lock.
	// Concurrent map iteration and map write is a FATAL Go runtime error
	// ("fatal error: concurrent map iteration and map write"): it cannot be
	// recovered and it terminates the whole server process, not just this
	// connection.
	experimentsMu.RLock()
	exp, exists := experiments[name]
	if !exists {
		experimentsMu.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR experiment not found"))
	}
	results := make([]*resp.Value, 0)
	results = append(results, resp.BulkString("name"), resp.BulkString(exp.Name), resp.BulkString("assignments"), resp.IntegerValue(int64(len(exp.Assignments))))
	for _, v := range exp.Variants {
		count := 0
		for _, a := range exp.Assignments {
			if a == v {
				count++
			}
		}
		results = append(results, resp.BulkString("variant_"+v), resp.IntegerValue(int64(count)))
	}
	experimentsMu.RUnlock()
	return ctx.WriteArray(results)
}

func cmdEXPERIMENTLIST(ctx *Context) error {
	experimentsMu.RLock()
	defer experimentsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range experiments {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

var (
	rollouts   = make(map[string]*Rollout)
	rolloutsMu sync.RWMutex
)

type Rollout struct {
	Name       string
	Percentage int64
	Users      map[string]bool
}

func cmdROLLOUTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	percentage := int64(0)
	if ctx.ArgCount() >= 2 {
		percentage = parseInt64(ctx.ArgString(1))
	}
	rolloutsMu.Lock()
	rollouts[name] = &Rollout{Name: name, Percentage: percentage, Users: make(map[string]bool)}
	rolloutsMu.Unlock()
	return ctx.WriteOK()
}

func cmdROLLOUTDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	rolloutsMu.Lock()
	defer rolloutsMu.Unlock()
	if _, exists := rollouts[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(rollouts, name)
	return ctx.WriteInteger(1)
}

func cmdROLLOUTCHECK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	userID := ctx.ArgString(1)
	rolloutsMu.RLock()
	ro, exists := rollouts[name]
	rolloutsMu.RUnlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	if ro.Users[userID] {
		return ctx.WriteInteger(1)
	}
	if ro.Percentage >= 100 {
		return ctx.WriteInteger(1)
	}
	hash := hashString(userID) % 100
	if int64(hash) < ro.Percentage {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdROLLOUTPERCENTAGE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	percentage := parseInt64(ctx.ArgString(1))
	rolloutsMu.Lock()
	defer rolloutsMu.Unlock()
	ro, exists := rollouts[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR rollout not found"))
	}
	ro.Percentage = percentage
	return ctx.WriteOK()
}

func cmdROLLOUTLIST(ctx *Context) error {
	rolloutsMu.RLock()
	defer rolloutsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name, ro := range rollouts {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("percentage"), resp.IntegerValue(ro.Percentage),
		}))
	}
	return ctx.WriteArray(results)
}

var (
	schemas   = make(map[string]string)
	schemasMu sync.RWMutex
)

func cmdSCHEMAREGISTER(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	schema := ctx.ArgString(1)
	schemasMu.Lock()
	schemas[name] = schema
	schemasMu.Unlock()
	return ctx.WriteOK()
}

func cmdSCHEMAVALIDATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteInteger(1)
}

func cmdSCHEMADELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	schemasMu.Lock()
	defer schemasMu.Unlock()
	if _, exists := schemas[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(schemas, name)
	return ctx.WriteInteger(1)
}

func cmdSCHEMALIST(ctx *Context) error {
	schemasMu.RLock()
	defer schemasMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range schemas {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

var (
	pipelines   = make(map[string]*Pipeline)
	pipelinesMu sync.RWMutex
)

type Pipeline struct {
	Name   string
	Stages []string
	Status string
}

func cmdPIPELINECREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	pipelinesMu.Lock()
	pipelines[name] = &Pipeline{Name: name, Stages: make([]string, 0), Status: "idle"}
	pipelinesMu.Unlock()
	return ctx.WriteOK()
}

func cmdPIPELINEADDSTAGE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	stage := ctx.ArgString(1)
	pipelinesMu.Lock()
	defer pipelinesMu.Unlock()
	pipe, exists := pipelines[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR pipeline not found"))
	}
	pipe.Stages = append(pipe.Stages, stage)
	return ctx.WriteOK()
}

func cmdPIPELINEEXECUTE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	pipelinesMu.Lock()
	defer pipelinesMu.Unlock()
	pipe, exists := pipelines[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR pipeline not found"))
	}
	pipe.Status = "completed"
	return ctx.WriteOK()
}

func cmdPIPELINESTATUS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	// Read the pipeline under the read lock. The lock used to be released
	// right after the lookup, so the Status string and the Stages slice were
	// read with no lock at all while PIPELINE.ADDSTAGE and PIPELINE.EXECUTE
	// wrote them under the write lock.
	pipelinesMu.RLock()
	pipe, exists := pipelines[name]
	if !exists {
		pipelinesMu.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR pipeline not found"))
	}
	pipeName := pipe.Name
	pipeStatus := pipe.Status
	pipeStages := int64(len(pipe.Stages))
	pipelinesMu.RUnlock()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(pipeName),
		resp.BulkString("status"), resp.BulkString(pipeStatus),
		resp.BulkString("stages"), resp.IntegerValue(pipeStages),
	})
}

func cmdPIPELINEDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	pipelinesMu.Lock()
	defer pipelinesMu.Unlock()
	if _, exists := pipelines[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(pipelines, name)
	return ctx.WriteInteger(1)
}

func cmdPIPELINELIST(ctx *Context) error {
	pipelinesMu.RLock()
	defer pipelinesMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name := range pipelines {
		results = append(results, resp.BulkString(name))
	}
	return ctx.WriteArray(results)
}

var (
	notifications   = make(map[string]*Notification)
	notificationsMu sync.RWMutex
)

type Notification struct {
	ID        string
	Name      string
	Type      string
	Template  string
	CreatedAt int64
}

func cmdNOTIFYCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	notifyType := ctx.ArgString(1)
	template := ctx.ArgString(2)
	notificationsMu.Lock()
	notifications[name] = &Notification{ID: generateUUID(), Name: name, Type: notifyType, Template: template, CreatedAt: time.Now().UnixMilli()}
	notificationsMu.Unlock()
	return ctx.WriteOK()
}

func cmdNOTIFYSEND(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	return ctx.WriteOK()
}

func cmdNOTIFYLIST(ctx *Context) error {
	notificationsMu.RLock()
	defer notificationsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name, n := range notifications {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("type"), resp.BulkString(n.Type),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdNOTIFYDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	notificationsMu.Lock()
	defer notificationsMu.Unlock()
	if _, exists := notifications[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(notifications, name)
	return ctx.WriteInteger(1)
}

func cmdNOTIFYTEMPLATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	notificationsMu.RLock()
	n, exists := notifications[name]
	notificationsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR notification not found"))
	}
	return ctx.WriteBulkString(n.Template)
}

var (
	alerts   = make(map[string]*Alert)
	alertsMu sync.RWMutex
)

type Alert struct {
	ID        string
	Name      string
	Message   string
	Status    string
	CreatedAt int64
}

func cmdALERTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	message := ctx.ArgString(1)
	alertsMu.Lock()
	alerts[name] = &Alert{ID: generateUUID(), Name: name, Message: message, Status: "active", CreatedAt: time.Now().UnixMilli()}
	alertsMu.Unlock()
	return ctx.WriteOK()
}

func cmdALERTTRIGGER(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	alertsMu.Lock()
	defer alertsMu.Unlock()
	alert, exists := alerts[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR alert not found"))
	}
	alert.Status = "firing"
	return ctx.WriteOK()
}

func cmdALERTACKNOWLEDGE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	alertsMu.Lock()
	defer alertsMu.Unlock()
	alert, exists := alerts[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR alert not found"))
	}
	alert.Status = "acknowledged"
	return ctx.WriteOK()
}

func cmdALERTRESOLVE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	alertsMu.Lock()
	defer alertsMu.Unlock()
	alert, exists := alerts[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR alert not found"))
	}
	alert.Status = "resolved"
	return ctx.WriteOK()
}

func cmdALERTLIST(ctx *Context) error {
	alertsMu.RLock()
	defer alertsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for name, a := range alerts {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("status"), resp.BulkString(a.Status),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdALERTHISTORY(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	// Read the alert under the read lock. The lock used to be released right
	// after the lookup, so the alert's Status string was read with no lock at
	// all while ALERT.TRIGGER/ACKNOWLEDGE/RESOLVE rewrote it under the write
	// lock.
	alertsMu.RLock()
	alert, exists := alerts[name]
	if !exists {
		alertsMu.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR alert not found"))
	}
	alertID := alert.ID
	alertName := alert.Name
	alertMessage := alert.Message
	alertStatus := alert.Status
	alertCreatedAt := alert.CreatedAt
	alertsMu.RUnlock()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(alertID),
		resp.BulkString("name"), resp.BulkString(alertName),
		resp.BulkString("message"), resp.BulkString(alertMessage),
		resp.BulkString("status"), resp.BulkString(alertStatus),
		resp.BulkString("created_at"), resp.IntegerValue(alertCreatedAt),
	})
}

var (
	countersX    = make(map[string]int64)
	countersXMux sync.RWMutex
)

func cmdCOUNTERXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	initVal := int64(0)
	if ctx.ArgCount() >= 2 {
		initVal = parseInt64(ctx.ArgString(1))
	}
	countersXMux.Lock()
	countersX[name] = initVal
	countersXMux.Unlock()
	return ctx.WriteOK()
}

func cmdCOUNTERXINCR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	incr := int64(1)
	if ctx.ArgCount() >= 2 {
		incr = parseInt64(ctx.ArgString(1))
	}
	countersXMux.Lock()
	defer countersXMux.Unlock()
	countersX[name] += incr
	return ctx.WriteInteger(countersX[name])
}

func cmdCOUNTERXDECR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	decr := int64(1)
	if ctx.ArgCount() >= 2 {
		decr = parseInt64(ctx.ArgString(1))
	}
	countersXMux.Lock()
	defer countersXMux.Unlock()
	countersX[name] -= decr
	return ctx.WriteInteger(countersX[name])
}

func cmdCOUNTERXGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	countersXMux.RLock()
	val, exists := countersX[name]
	countersXMux.RUnlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(val)
}

func cmdCOUNTERXRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	countersXMux.Lock()
	countersX[name] = 0
	countersXMux.Unlock()
	return ctx.WriteOK()
}

func cmdCOUNTERXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	countersXMux.Lock()
	defer countersXMux.Unlock()
	if _, exists := countersX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(countersX, name)
	return ctx.WriteInteger(1)
}

var (
	gauges   = make(map[string]*Gauge)
	gaugesMu sync.RWMutex
)

type Gauge struct {
	Name  string
	Value float64
}

func cmdGAUGECREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	initVal := float64(0)
	if ctx.ArgCount() >= 2 {
		initVal = parseFloatExt([]byte(ctx.ArgString(1)))
	}
	gaugesMu.Lock()
	gauges[name] = &Gauge{Name: name, Value: initVal}
	gaugesMu.Unlock()
	return ctx.WriteOK()
}

func cmdGAUGESET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := parseFloatExt([]byte(ctx.ArgString(1)))
	gaugesMu.Lock()
	defer gaugesMu.Unlock()
	if g, exists := gauges[name]; exists {
		g.Value = value
		return ctx.WriteOK()
	}
	gauges[name] = &Gauge{Name: name, Value: value}
	return ctx.WriteOK()
}

func cmdGAUGEGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	// Read the value under the read lock. The lock used to be released right
	// after the lookup, so g.Value was read with no lock at all while
	// GAUGE.SET/GAUGE.INCR/GAUGE.DECR updated it under the write lock.
	gaugesMu.RLock()
	g, exists := gauges[name]
	if !exists {
		gaugesMu.RUnlock()
		return ctx.WriteBulkString("0")
	}
	value := g.Value
	gaugesMu.RUnlock()
	return ctx.WriteBulkString(fmt.Sprintf("%.6f", value))
}

func cmdGAUGEINCR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	incr := float64(1)
	if ctx.ArgCount() >= 2 {
		incr = parseFloatExt([]byte(ctx.ArgString(1)))
	}
	gaugesMu.Lock()
	defer gaugesMu.Unlock()
	if g, exists := gauges[name]; exists {
		g.Value += incr
		return ctx.WriteBulkString(fmt.Sprintf("%.6f", g.Value))
	}
	gauges[name] = &Gauge{Name: name, Value: incr}
	return ctx.WriteBulkString(fmt.Sprintf("%.6f", incr))
}

func cmdGAUGEDECR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	decr := float64(1)
	if ctx.ArgCount() >= 2 {
		decr = parseFloatExt([]byte(ctx.ArgString(1)))
	}
	gaugesMu.Lock()
	defer gaugesMu.Unlock()
	if g, exists := gauges[name]; exists {
		g.Value -= decr
		return ctx.WriteBulkString(fmt.Sprintf("%.6f", g.Value))
	}
	gauges[name] = &Gauge{Name: name, Value: -decr}
	return ctx.WriteBulkString(fmt.Sprintf("%.6f", -decr))
}

func cmdGAUGEDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	gaugesMu.Lock()
	defer gaugesMu.Unlock()
	if _, exists := gauges[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(gauges, name)
	return ctx.WriteInteger(1)
}

var (
	traces   = make(map[string]*Trace)
	tracesMu sync.RWMutex
)

type Trace struct {
	ID     string
	Spans  []*Span
	Status string
}

type Span struct {
	ID    string
	Name  string
	Start int64
	End   int64
	Tags  map[string]string
}

func cmdTRACESTART(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	tracesMu.Lock()
	traces[id] = &Trace{ID: id, Spans: make([]*Span, 0), Status: "running"}
	tracesMu.Unlock()
	return ctx.WriteOK()
}

func cmdTRACESPAN(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	traceID := ctx.ArgString(0)
	spanName := ctx.ArgString(1)
	spanID := ctx.ArgString(2)
	tracesMu.Lock()
	defer tracesMu.Unlock()
	trace, exists := traces[traceID]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR trace not found"))
	}
	trace.Spans = append(trace.Spans, &Span{ID: spanID, Name: spanName, Start: time.Now().UnixMilli(), Tags: make(map[string]string)})
	return ctx.WriteOK()
}

func cmdTRACEEND(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	tracesMu.Lock()
	defer tracesMu.Unlock()
	trace, exists := traces[id]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR trace not found"))
	}
	trace.Status = "completed"
	for _, span := range trace.Spans {
		if span.End == 0 {
			span.End = time.Now().UnixMilli()
		}
	}
	return ctx.WriteOK()
}

func cmdTRACEGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	// Read the trace under the read lock. The lock used to be released right
	// after the lookup, so the Status string and the Spans slice were read
	// with no lock at all while TRACE.SPAN and TRACE.END wrote them under
	// the write lock.
	tracesMu.RLock()
	trace, exists := traces[id]
	if !exists {
		tracesMu.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR trace not found"))
	}
	traceID := trace.ID
	traceStatus := trace.Status
	traceSpans := int64(len(trace.Spans))
	tracesMu.RUnlock()

	results := make([]*resp.Value, 0)
	results = append(results, resp.BulkString("id"), resp.BulkString(traceID), resp.BulkString("status"), resp.BulkString(traceStatus), resp.BulkString("spans"), resp.IntegerValue(traceSpans))
	return ctx.WriteArray(results)
}

func cmdTRACELIST(ctx *Context) error {
	tracesMu.RLock()
	defer tracesMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range traces {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	logsX    = make(map[string][]*LogEntry)
	logsXMux sync.RWMutex
)

type LogEntry struct {
	Timestamp int64
	Level     string
	Message   string
}

func cmdLOGXWRITE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	logName := ctx.ArgString(0)
	level := ctx.ArgString(1)
	message := ctx.ArgString(2)
	logsXMux.Lock()
	defer logsXMux.Unlock()
	if _, exists := logsX[logName]; !exists {
		logsX[logName] = make([]*LogEntry, 0)
	}
	logsX[logName] = append(logsX[logName], &LogEntry{Timestamp: time.Now().UnixMilli(), Level: level, Message: message})
	return ctx.WriteOK()
}

func cmdLOGXREAD(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	logName := ctx.ArgString(0)
	limit := 100
	if ctx.ArgCount() >= 2 {
		limit = int(parseInt64(ctx.ArgString(1)))
	}
	logsXMux.RLock()
	entries, exists := logsX[logName]
	logsXMux.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	start := len(entries) - limit
	if start < 0 {
		start = 0
	}
	results := make([]*resp.Value, 0)
	for i := start; i < len(entries); i++ {
		e := entries[i]
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("timestamp"), resp.IntegerValue(e.Timestamp),
			resp.BulkString("level"), resp.BulkString(e.Level),
			resp.BulkString("message"), resp.BulkString(e.Message),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdLOGXSEARCH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	logName := ctx.ArgString(0)
	query := ctx.ArgString(1)
	logsXMux.RLock()
	entries, exists := logsX[logName]
	logsXMux.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, 0)
	for _, e := range entries {
		if containsStr(e.Message, query) {
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.BulkString("timestamp"), resp.IntegerValue(e.Timestamp),
				resp.BulkString("level"), resp.BulkString(e.Level),
				resp.BulkString("message"), resp.BulkString(e.Message),
			}))
		}
	}
	return ctx.WriteArray(results)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func cmdLOGXCLEAR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	logName := ctx.ArgString(0)
	logsXMux.Lock()
	defer logsXMux.Unlock()
	count := len(logsX[logName])
	logsX[logName] = make([]*LogEntry, 0)
	return ctx.WriteInteger(int64(count))
}

func cmdLOGXSTATS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	logName := ctx.ArgString(0)
	logsXMux.RLock()
	entries, exists := logsX[logName]
	logsXMux.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(logName),
			resp.BulkString("entries"), resp.IntegerValue(0),
		})
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(logName),
		resp.BulkString("entries"), resp.IntegerValue(int64(len(entries))),
	})
}

var (
	apiKeys   = make(map[string]*APIKey)
	apiKeysMu sync.RWMutex
)

type APIKey struct {
	Key       string
	Name      string
	CreatedAt int64
	Usage     int64
	Active    bool
}

func cmdAPIKEYCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := generateUUID()
	apiKeysMu.Lock()
	apiKeys[key] = &APIKey{Key: key, Name: name, CreatedAt: time.Now().UnixMilli(), Usage: 0, Active: true}
	apiKeysMu.Unlock()
	return ctx.WriteBulkString(key)
}

func cmdAPIKEYVALIDATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	apiKeysMu.Lock()
	defer apiKeysMu.Unlock()
	if ak, exists := apiKeys[key]; exists && ak.Active {
		ak.Usage++
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdAPIKEYREVOKE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	apiKeysMu.Lock()
	defer apiKeysMu.Unlock()
	if ak, exists := apiKeys[key]; exists {
		ak.Active = false
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdAPIKEYLIST(ctx *Context) error {
	apiKeysMu.RLock()
	defer apiKeysMu.RUnlock()
	results := make([]*resp.Value, 0)
	for key, ak := range apiKeys {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("key"), resp.BulkString(key),
			resp.BulkString("name"), resp.BulkString(ak.Name),
			resp.BulkString("active"), resp.BulkString(fmt.Sprintf("%v", ak.Active)),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdAPIKEYUSAGE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	// Read the counter under the read lock. The lock used to be released right
	// after the lookup, so ak.Usage was read with no lock at all while
	// APIKEY.VALIDATE incremented it under the write lock.
	apiKeysMu.RLock()
	ak, exists := apiKeys[key]
	if !exists {
		apiKeysMu.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR api key not found"))
	}
	usage := ak.Usage
	apiKeysMu.RUnlock()
	return ctx.WriteInteger(usage)
}

var (
	quotasX    = make(map[string]*QuotaX)
	quotasXMux sync.RWMutex
)

type QuotaX struct {
	Name    string
	Limit   int64
	Used    int64
	Period  int64
	ResetAt int64
}

// rollQuotaXWindow starts a new counting window when the current one has
// expired, the same way the QUOTA family in mvcc_commands.go rolls its window
// over.
//
// A quota without a usable period never rolls over. A zero-length window is
// over the moment the clock moves on, so rolling it over reset the counter on
// every check and the limit was never enforced: "0" and a non-numeric period
// both reach the handler as a zero-length window. Without a window the limit
// applies for the lifetime of the quota instead.
func rollQuotaXWindow(q *QuotaX, now int64) {
	if now <= q.ResetAt {
		return
	}
	if q.Period <= 0 {
		return
	}
	q.Used = 0
	q.ResetAt = now + q.Period
}

func cmdQUOTAXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	limit := parseInt64(ctx.ArgString(1))
	periodMs := parseInt64(ctx.ArgString(2))
	quotasXMux.Lock()
	quotasX[name] = &QuotaX{Name: name, Limit: limit, Used: 0, Period: periodMs, ResetAt: time.Now().UnixMilli() + periodMs}
	quotasXMux.Unlock()
	return ctx.WriteOK()
}

func cmdQUOTAXCHECK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	amount := parseInt64(ctx.ArgString(1))
	quotasXMux.Lock()
	defer quotasXMux.Unlock()
	q, exists := quotasX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR quota not found"))
	}
	now := time.Now().UnixMilli()
	rollQuotaXWindow(q, now)
	if q.Used+amount > q.Limit {
		return ctx.WriteArray([]*resp.Value{
			resp.IntegerValue(0),
			resp.IntegerValue(q.Limit - q.Used),
		})
	}
	q.Used += amount
	return ctx.WriteArray([]*resp.Value{
		resp.IntegerValue(1),
		resp.IntegerValue(q.Limit - q.Used),
	})
}

func cmdQUOTAXUSAGE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	quotasXMux.RLock()
	q, exists := quotasX[name]
	if !exists {
		quotasXMux.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR quota not found"))
	}
	// Snapshot the values while the lock is held: cmdQUOTAXCHECK writes q.Used
	// under the write lock, so reading the fields after the lock was released
	// raced with that write.
	used := q.Used
	limit := q.Limit
	quotasXMux.RUnlock()
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("used"), resp.IntegerValue(used),
		resp.BulkString("limit"), resp.IntegerValue(limit),
		resp.BulkString("remaining"), resp.IntegerValue(limit - used),
	})
}

func cmdQUOTAXRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	quotasXMux.Lock()
	defer quotasXMux.Unlock()
	q, exists := quotasX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR quota not found"))
	}
	q.Used = 0
	q.ResetAt = time.Now().UnixMilli() + q.Period
	return ctx.WriteOK()
}

func cmdQUOTAXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	quotasXMux.Lock()
	defer quotasXMux.Unlock()
	if _, exists := quotasX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(quotasX, name)
	return ctx.WriteInteger(1)
}

var (
	meters   = make(map[string]*Meter)
	metersMu sync.RWMutex
)

type Meter struct {
	Name    string
	Unit    string
	Records []int64
	Rate    float64
}

func cmdMETERCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	unit := ctx.ArgString(1)
	metersMu.Lock()
	meters[name] = &Meter{Name: name, Unit: unit, Records: make([]int64, 0), Rate: 0}
	metersMu.Unlock()
	return ctx.WriteOK()
}

func cmdMETERRECORD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := parseInt64(ctx.ArgString(1))
	metersMu.Lock()
	defer metersMu.Unlock()
	m, exists := meters[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR meter not found"))
	}
	m.Records = append(m.Records, value)
	return ctx.WriteOK()
}

func cmdMETERGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	// Sum the records under the read lock. The lock used to be released right
	// after the lookup, so the Records slice was walked with no lock at all
	// while METER.RECORD appended to it under the write lock.
	metersMu.RLock()
	m, exists := meters[name]
	if !exists {
		metersMu.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR meter not found"))
	}
	meterName := m.Name
	meterUnit := m.Unit
	var total int64
	for _, v := range m.Records {
		total += v
	}
	count := int64(len(m.Records))
	metersMu.RUnlock()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(meterName),
		resp.BulkString("unit"), resp.BulkString(meterUnit),
		resp.BulkString("total"), resp.IntegerValue(total),
		resp.BulkString("count"), resp.IntegerValue(count),
	})
}

func cmdMETERBILLING(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	// Sum the records under the read lock. The lock used to be released right
	// after the lookup, so the Records slice was walked with no lock at all
	// while METER.RECORD appended to it under the write lock.
	metersMu.RLock()
	m, exists := meters[name]
	if !exists {
		metersMu.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR meter not found"))
	}
	meterName := m.Name
	meterUnit := m.Unit
	var total int64
	for _, v := range m.Records {
		total += v
	}
	metersMu.RUnlock()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("meter"), resp.BulkString(meterName),
		resp.BulkString("usage"), resp.IntegerValue(total),
		resp.BulkString("unit"), resp.BulkString(meterUnit),
	})
}

func cmdMETERDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	metersMu.Lock()
	defer metersMu.Unlock()
	if _, exists := meters[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(meters, name)
	return ctx.WriteInteger(1)
}

var (
	tenants   = make(map[string]*Tenant)
	tenantsMu sync.RWMutex
)

type Tenant struct {
	ID        string
	Name      string
	Config    map[string]string
	CreatedAt int64
}

func cmdTENANTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	name := ctx.ArgString(1)
	tenantsMu.Lock()
	tenants[id] = &Tenant{ID: id, Name: name, Config: make(map[string]string), CreatedAt: time.Now().UnixMilli()}
	tenantsMu.Unlock()
	return ctx.WriteOK()
}

func cmdTENANTDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	tenantsMu.Lock()
	defer tenantsMu.Unlock()
	if _, exists := tenants[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(tenants, id)
	return ctx.WriteInteger(1)
}

func cmdTENANTGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	tenantsMu.RLock()
	t, exists := tenants[id]
	tenantsMu.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR tenant not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(t.ID),
		resp.BulkString("name"), resp.BulkString(t.Name),
		resp.BulkString("created_at"), resp.IntegerValue(t.CreatedAt),
	})
}

func cmdTENANTLIST(ctx *Context) error {
	tenantsMu.RLock()
	defer tenantsMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range tenants {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

func cmdTENANTCONFIG(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)
	tenantsMu.Lock()
	defer tenantsMu.Unlock()
	t, exists := tenants[id]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR tenant not found"))
	}
	t.Config[key] = value
	return ctx.WriteOK()
}

var (
	leases   = make(map[string]*Lease)
	leasesMu sync.RWMutex
)

type Lease struct {
	ID        string
	Holder    string
	ExpiresAt int64
}

func cmdLEASECREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	holder := ctx.ArgString(1)
	ttlMs := parseInt64(ctx.ArgString(2))
	leasesMu.Lock()
	defer leasesMu.Unlock()
	if _, exists := leases[id]; exists {
		return ctx.WriteError(fmt.Errorf("ERR lease already exists"))
	}
	leases[id] = &Lease{ID: id, Holder: holder, ExpiresAt: time.Now().UnixMilli() + ttlMs}
	return ctx.WriteOK()
}

func cmdLEASERENEW(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	ttlMs := parseInt64(ctx.ArgString(1))
	leasesMu.Lock()
	defer leasesMu.Unlock()
	lease, exists := leases[id]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR lease not found"))
	}
	lease.ExpiresAt = time.Now().UnixMilli() + ttlMs
	return ctx.WriteOK()
}

func cmdLEASEREVOKE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	leasesMu.Lock()
	defer leasesMu.Unlock()
	if _, exists := leases[id]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(leases, id)
	return ctx.WriteInteger(1)
}

func cmdLEASEGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	// Read the lease under the read lock. The lock used to be released right
	// after the lookup, so the lease's expiry was read with no lock at all
	// while LEASE.RENEW wrote it under the write lock.
	leasesMu.RLock()
	lease, exists := leases[id]
	if !exists {
		leasesMu.RUnlock()
		return ctx.WriteNull()
	}
	leaseID := lease.ID
	leaseHolder := lease.Holder
	leaseExpiresAt := lease.ExpiresAt
	leasesMu.RUnlock()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(leaseID),
		resp.BulkString("holder"), resp.BulkString(leaseHolder),
		resp.BulkString("expires_at"), resp.IntegerValue(leaseExpiresAt),
	})
}

func cmdLEASELIST(ctx *Context) error {
	leasesMu.RLock()
	defer leasesMu.RUnlock()
	results := make([]*resp.Value, 0)
	for id := range leases {
		results = append(results, resp.BulkString(id))
	}
	return ctx.WriteArray(results)
}

var (
	heaps   = make(map[string]*Heap)
	heapsMu sync.RWMutex
)

type Heap struct {
	Name    string
	MinHeap bool
	Data    []int64
}

func cmdHEAPPUSH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := parseInt64(ctx.ArgString(1))
	heapsMu.Lock()
	defer heapsMu.Unlock()
	if _, exists := heaps[name]; !exists {
		heaps[name] = &Heap{Name: name, MinHeap: true, Data: make([]int64, 0)}
	}
	heaps[name].Data = append(heaps[name].Data, value)
	return ctx.WriteInteger(int64(len(heaps[name].Data)))
}

func cmdHEAPPOP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	heapsMu.Lock()
	defer heapsMu.Unlock()
	h, exists := heaps[name]
	if !exists || len(h.Data) == 0 {
		return ctx.WriteNull()
	}
	minIdx := 0
	for i, v := range h.Data {
		if v < h.Data[minIdx] {
			minIdx = i
		}
	}
	val := h.Data[minIdx]
	h.Data = append(h.Data[:minIdx], h.Data[minIdx+1:]...)
	return ctx.WriteInteger(val)
}

func cmdHEAPPEEK(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	// Read the heap under the read lock. The lock used to be released right
	// after the lookup, so the Data slice was read with no lock at all while
	// HEAP.PUSH grew it and HEAP.POP shifted its elements in place under the
	// write lock: a data race, and a reader that sized the slice from a
	// header the writer had already replaced could index past the end of the
	// backing array.
	heapsMu.RLock()
	h, exists := heaps[name]
	if !exists {
		heapsMu.RUnlock()
		return ctx.WriteNull()
	}
	if len(h.Data) == 0 {
		heapsMu.RUnlock()
		return ctx.WriteNull()
	}
	minIdx := 0
	for i, v := range h.Data {
		if v < h.Data[minIdx] {
			minIdx = i
		}
	}
	min := h.Data[minIdx]
	heapsMu.RUnlock()
	return ctx.WriteInteger(min)
}

func cmdHEAPSIZE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	// Read the size under the read lock. The lock used to be released right
	// after the lookup, so len(h.Data) was read with no lock at all while
	// HEAP.PUSH replaced that slice header under the write lock.
	heapsMu.RLock()
	h, exists := heaps[name]
	if !exists {
		heapsMu.RUnlock()
		return ctx.WriteInteger(0)
	}
	size := len(h.Data)
	heapsMu.RUnlock()
	return ctx.WriteInteger(int64(size))
}

func cmdHEAPDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	heapsMu.Lock()
	defer heapsMu.Unlock()
	if _, exists := heaps[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(heaps, name)
	return ctx.WriteInteger(1)
}

var (
	bloomX    = make(map[string]*BloomXFilter)
	bloomXMux sync.RWMutex
)

type BloomXFilter struct {
	Name   string
	Size   int
	Hashes int
	Bits   []bool
}

func cmdBLOOMXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	size := int(parseInt64(ctx.ArgString(1)))
	hashes := int(parseInt64(ctx.ArgString(2)))
	bloomXMux.Lock()
	bloomX[name] = &BloomXFilter{Name: name, Size: size, Hashes: hashes, Bits: make([]bool, size)}
	bloomXMux.Unlock()
	return ctx.WriteOK()
}

func cmdBLOOMXADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	item := ctx.ArgString(1)
	bloomXMux.Lock()
	defer bloomXMux.Unlock()
	f, exists := bloomX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR bloom filter not found"))
	}
	for i := 0; i < f.Hashes; i++ {
		idx := bloomHash(item, i, f.Size)
		f.Bits[idx] = true
	}
	return ctx.WriteInteger(1)
}

func cmdBLOOMXCHECK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	item := ctx.ArgString(1)
	bloomXMux.RLock()
	defer bloomXMux.RUnlock()
	f, exists := bloomX[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR bloom filter not found"))
	}
	for i := 0; i < f.Hashes; i++ {
		idx := bloomHash(item, i, f.Size)
		if !f.Bits[idx] {
			return ctx.WriteInteger(0)
		}
	}
	return ctx.WriteInteger(1)
}

func bloomHash(item string, seed, size int) int {
	h := seed
	for _, c := range item {
		h = h*31 + int(c)
	}
	return (h%size + size) % size
}

func cmdBLOOMXINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	bloomXMux.RLock()
	f, exists := bloomX[name]
	bloomXMux.RUnlock()
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR bloom filter not found"))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(f.Name),
		resp.BulkString("size"), resp.IntegerValue(int64(f.Size)),
		resp.BulkString("hashes"), resp.IntegerValue(int64(f.Hashes)),
	})
}

func cmdBLOOMXDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	bloomXMux.Lock()
	defer bloomXMux.Unlock()
	if _, exists := bloomX[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(bloomX, name)
	return ctx.WriteInteger(1)
}

var (
	sketches   = make(map[string]*Sketch)
	sketchesMu sync.RWMutex
)

type Sketch struct {
	Name   string
	Width  int
	Depth  int
	Counts [][]int64
}

func cmdSKETCHCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	width := int(parseInt64(ctx.ArgString(1)))
	depth := int(parseInt64(ctx.ArgString(2)))
	counts := make([][]int64, depth)
	for i := range counts {
		counts[i] = make([]int64, width)
	}
	sketchesMu.Lock()
	sketches[name] = &Sketch{Name: name, Width: width, Depth: depth, Counts: counts}
	sketchesMu.Unlock()
	return ctx.WriteOK()
}

func cmdSKETCHUPDATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	item := ctx.ArgString(1)
	count := parseInt64(ctx.ArgString(2))
	sketchesMu.Lock()
	defer sketchesMu.Unlock()
	s, exists := sketches[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR sketch not found"))
	}
	for i := 0; i < s.Depth; i++ {
		idx := bloomHash(item, i, s.Width)
		s.Counts[i][idx] += count
	}
	return ctx.WriteOK()
}

func cmdSKETCHQUERY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	item := ctx.ArgString(1)
	// Read the counter cells under the read lock. The lock used to be
	// released right after the lookup, so the cells were read with no lock at
	// all while SKETCH.UPDATE incremented them in place under the write lock.
	sketchesMu.RLock()
	s, exists := sketches[name]
	if !exists {
		sketchesMu.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR sketch not found"))
	}
	min := int64(-1)
	for i := 0; i < s.Depth; i++ {
		idx := bloomHash(item, i, s.Width)
		if min == -1 || s.Counts[i][idx] < min {
			min = s.Counts[i][idx]
		}
	}
	sketchesMu.RUnlock()
	return ctx.WriteInteger(min)
}

func cmdSKETCHMERGE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_ = ctx.ArgString(0)
	_ = ctx.ArgString(1)
	_ = ctx.ArgString(2)
	return ctx.WriteOK()
}

func cmdSKETCHDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	sketchesMu.Lock()
	defer sketchesMu.Unlock()
	if _, exists := sketches[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(sketches, name)
	return ctx.WriteInteger(1)
}

var (
	ringBuffers   = make(map[string]*RingBuffer)
	ringBuffersMu sync.RWMutex
)

type RingBuffer struct {
	Name  string
	Size  int
	Data  []string
	Head  int
	Count int
}

func cmdRINGBUFFERCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	size := int(parseInt64(ctx.ArgString(1)))
	ringBuffersMu.Lock()
	ringBuffers[name] = &RingBuffer{Name: name, Size: size, Data: make([]string, size), Head: 0, Count: 0}
	ringBuffersMu.Unlock()
	return ctx.WriteOK()
}

func cmdRINGBUFFERWRITE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := ctx.ArgString(1)
	ringBuffersMu.Lock()
	defer ringBuffersMu.Unlock()
	rb, exists := ringBuffers[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR ring buffer not found"))
	}
	rb.Data[rb.Head] = value
	rb.Head = (rb.Head + 1) % rb.Size
	if rb.Count < rb.Size {
		rb.Count++
	}
	return ctx.WriteOK()
}

func cmdRINGBUFFERREAD(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	ringBuffersMu.RLock()
	rb, exists := ringBuffers[name]
	ringBuffersMu.RUnlock()
	if !exists || rb.Count == 0 {
		return ctx.WriteArray([]*resp.Value{})
	}
	results := make([]*resp.Value, rb.Count)
	tail := (rb.Head - rb.Count + rb.Size) % rb.Size
	for i := 0; i < rb.Count; i++ {
		results[i] = resp.BulkString(rb.Data[(tail+i)%rb.Size])
	}
	return ctx.WriteArray(results)
}

func cmdRINGBUFFERSIZE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	// Read the buffer under the read lock. The lock used to be released right
	// after the lookup, so the Size and Count fields were read with no lock at
	// all while RINGBUFFER.WRITE mutated them under the write lock.
	ringBuffersMu.RLock()
	rb, exists := ringBuffers[name]
	if !exists {
		ringBuffersMu.RUnlock()
		return ctx.WriteError(fmt.Errorf("ERR ring buffer not found"))
	}
	rbSize := rb.Size
	rbCount := rb.Count
	ringBuffersMu.RUnlock()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("size"), resp.IntegerValue(int64(rbSize)),
		resp.BulkString("count"), resp.IntegerValue(int64(rbCount)),
	})
}

func cmdRINGBUFFERDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	ringBuffersMu.Lock()
	defer ringBuffersMu.Unlock()
	if _, exists := ringBuffers[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(ringBuffers, name)
	return ctx.WriteInteger(1)
}

var (
	windows   = make(map[string]*Window)
	windowsMu sync.RWMutex
)

type Window struct {
	Name string
	Size int
	Data []float64
}

func cmdWINDOWCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	size := int(parseInt64(ctx.ArgString(1)))
	windowsMu.Lock()
	windows[name] = &Window{Name: name, Size: size, Data: make([]float64, 0)}
	windowsMu.Unlock()
	return ctx.WriteOK()
}

func cmdWINDOWADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	value := parseFloatExt([]byte(ctx.ArgString(1)))
	windowsMu.Lock()
	defer windowsMu.Unlock()
	w, exists := windows[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR window not found"))
	}
	w.Data = append(w.Data, value)
	if len(w.Data) > w.Size {
		w.Data = w.Data[1:]
	}
	return ctx.WriteOK()
}

func cmdWINDOWGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	// Copy the values while the read lock is held. The lock used to be
	// released right after the lookup, so the window's Data slice was read
	// with no lock at all while WINDOW.ADD grew and trimmed it under the
	// write lock: a data race, and a reply sized from a header the writer
	// had already replaced did not match the window it came from.
	windowsMu.RLock()
	w, exists := windows[name]
	if !exists {
		windowsMu.RUnlock()
		return ctx.WriteArray([]*resp.Value{})
	}
	data := make([]float64, len(w.Data))
	copy(data, w.Data)
	windowsMu.RUnlock()

	results := make([]*resp.Value, len(data))
	for i, v := range data {
		results[i] = resp.BulkString(fmt.Sprintf("%.6f", v))
	}
	return ctx.WriteArray(results)
}

func cmdWINDOWAGGREGATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	aggType := ctx.ArgString(1)
	// Copy the values while the read lock is held. The lock used to be
	// released right after the lookup, so the window's Data slice was read
	// with no lock at all — including indexing element 0 for the min and max
	// aggregations — while WINDOW.ADD grew and trimmed that slice under the
	// write lock.
	windowsMu.RLock()
	w, exists := windows[name]
	if !exists {
		windowsMu.RUnlock()
		return ctx.WriteBulkString("0")
	}
	data := make([]float64, len(w.Data))
	copy(data, w.Data)
	windowsMu.RUnlock()
	if len(data) == 0 {
		return ctx.WriteBulkString("0")
	}
	switch aggType {
	case "sum":
		var sum float64
		for _, v := range data {
			sum += v
		}
		return ctx.WriteBulkString(fmt.Sprintf("%.6f", sum))
	case "avg":
		var sum float64
		for _, v := range data {
			sum += v
		}
		return ctx.WriteBulkString(fmt.Sprintf("%.6f", sum/float64(len(data))))
	case "min":
		min := data[0]
		for _, v := range data {
			if v < min {
				min = v
			}
		}
		return ctx.WriteBulkString(fmt.Sprintf("%.6f", min))
	case "max":
		max := data[0]
		for _, v := range data {
			if v > max {
				max = v
			}
		}
		return ctx.WriteBulkString(fmt.Sprintf("%.6f", max))
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown aggregation"))
	}
}

func cmdWINDOWDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	windowsMu.Lock()
	defer windowsMu.Unlock()
	if _, exists := windows[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(windows, name)
	return ctx.WriteInteger(1)
}

var (
	freqs   = make(map[string]*Frequency)
	freqsMu sync.RWMutex
)

type Frequency struct {
	Name  string
	Items map[string]int64
}

func cmdFREQCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	freqsMu.Lock()
	freqs[name] = &Frequency{Name: name, Items: make(map[string]int64)}
	freqsMu.Unlock()
	return ctx.WriteOK()
}

func cmdFREQADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	item := ctx.ArgString(1)
	count := int64(1)
	if ctx.ArgCount() >= 3 {
		count = parseInt64(ctx.ArgString(2))
	}
	freqsMu.Lock()
	defer freqsMu.Unlock()
	f, exists := freqs[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR frequency not found"))
	}
	f.Items[item] += count
	return ctx.WriteInteger(f.Items[item])
}

func cmdFREQCOUNT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	item := ctx.ArgString(1)
	// Read the counter while holding the read lock. The lock used to be
	// released right after the lookup, so the map was read with no lock at all
	// while FREQ.ADD wrote it under the write lock. Concurrent map read and
	// map write is a FATAL Go runtime error ("fatal error: concurrent map
	// read and map write"): it cannot be recovered and it terminates the
	// whole server process, not just this connection.
	freqsMu.RLock()
	f, exists := freqs[name]
	if !exists {
		freqsMu.RUnlock()
		return ctx.WriteInteger(0)
	}
	count := f.Items[item]
	freqsMu.RUnlock()
	return ctx.WriteInteger(count)
}

func cmdFREQTOP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	k := 10
	if ctx.ArgCount() >= 2 {
		k = int(parseInt64(ctx.ArgString(1)))
	}
	freqsMu.RLock()
	f, exists := freqs[name]
	freqsMu.RUnlock()
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}
	type kv struct {
		Key   string
		Value int64
	}
	var items []kv
	for k, v := range f.Items {
		items = append(items, kv{k, v})
	}
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Value > items[i].Value {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	if k > len(items) {
		k = len(items)
	}
	results := make([]*resp.Value, 0)
	for i := 0; i < k; i++ {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("item"), resp.BulkString(items[i].Key),
			resp.BulkString("count"), resp.IntegerValue(items[i].Value),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdFREQDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	freqsMu.Lock()
	defer freqsMu.Unlock()
	if _, exists := freqs[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(freqs, name)
	return ctx.WriteInteger(1)
}

var (
	partitions   = make(map[string]*Partition)
	partitionsMu sync.RWMutex
)

type Partition struct {
	Name  string
	Count int
	Data  [][]string
}

func cmdPARTITIONCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	count := int(parseInt64(ctx.ArgString(1)))
	partitionsMu.Lock()
	partitions[name] = &Partition{Name: name, Count: count, Data: make([][]string, count)}
	for i := range partitions[name].Data {
		partitions[name].Data[i] = make([]string, 0)
	}
	partitionsMu.Unlock()
	return ctx.WriteOK()
}

func cmdPARTITIONADD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)
	partitionsMu.Lock()
	defer partitionsMu.Unlock()
	p, exists := partitions[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR partition not found"))
	}
	idx := hashString(key) % p.Count
	p.Data[idx] = append(p.Data[idx], value)
	return ctx.WriteInteger(int64(idx))
}

func cmdPARTITIONGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	idx := int(parseInt64(ctx.ArgString(1)))
	// Copy the partition's values under the read lock. The lock used to be
	// released right after the lookup, so the slice was walked with no lock at
	// all while PARTITION.ADD appended to it under the write lock.
	partitionsMu.RLock()
	p, exists := partitions[name]
	if !exists {
		partitionsMu.RUnlock()
		return ctx.WriteArray([]*resp.Value{})
	}
	if idx < 0 || idx >= p.Count {
		partitionsMu.RUnlock()
		return ctx.WriteArray([]*resp.Value{})
	}
	values := make([]string, len(p.Data[idx]))
	copy(values, p.Data[idx])
	partitionsMu.RUnlock()

	results := make([]*resp.Value, len(values))
	for i, v := range values {
		results[i] = resp.BulkString(v)
	}
	return ctx.WriteArray(results)
}

func cmdPARTITIONLIST(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	// Read the counts under the read lock. The lock used to be released right
	// after the lookup, so every partition's slice length was read with no
	// lock at all while PARTITION.ADD appended to those slices under the
	// write lock.
	partitionsMu.RLock()
	p, exists := partitions[name]
	if !exists {
		partitionsMu.RUnlock()
		return ctx.WriteArray([]*resp.Value{})
	}
	counts := make([]int, p.Count)
	for i := 0; i < p.Count; i++ {
		counts[i] = len(p.Data[i])
	}
	partitionsMu.RUnlock()

	results := make([]*resp.Value, 0)
	for i := 0; i < p.Count; i++ {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("partition"), resp.IntegerValue(int64(i)),
			resp.BulkString("count"), resp.IntegerValue(int64(counts[i])),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdPARTITIONDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	partitionsMu.Lock()
	defer partitionsMu.Unlock()
	if _, exists := partitions[name]; !exists {
		return ctx.WriteInteger(0)
	}
	delete(partitions, name)
	return ctx.WriteInteger(1)
}
