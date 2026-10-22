package command

import (
	"fmt"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func RegisterIntegrationCommands(router *Router) {
	router.Register(&CommandDef{Name: "CIRCUITBREAKER.CREATE", Handler: cmdCIRCUITBREAKERCREATE})
	router.Register(&CommandDef{Name: "CIRCUITBREAKER.STATE", Handler: cmdCIRCUITBREAKERSTATE})
	router.Register(&CommandDef{Name: "CIRCUITBREAKER.TRIP", Handler: cmdCIRCUITBREAKERTRIP})
	router.Register(&CommandDef{Name: "CIRCUITBREAKER.RESET", Handler: cmdCIRCUITBREAKERRESET})

	router.Register(&CommandDef{Name: "RATELIMIT.CREATE", Handler: cmdRATELIMITCREATE})
	router.Register(&CommandDef{Name: "RATELIMIT.CHECK", Handler: cmdRATELIMITCHECK})
	router.Register(&CommandDef{Name: "RATELIMIT.RESET", Handler: cmdRATELIMITRESET})
	router.Register(&CommandDef{Name: "RATELIMIT.DELETE", Handler: cmdRATELIMITDELETE})

	router.Register(&CommandDef{Name: "CACHE.LOCK", Handler: cmdCACHELOCK})
	router.Register(&CommandDef{Name: "CACHE.UNLOCK", Handler: cmdCACHEUNLOCK})
	router.Register(&CommandDef{Name: "CACHE.LOCKED", Handler: cmdCACHELOCKED})
	router.Register(&CommandDef{Name: "CACHE.REFRESH", Handler: cmdCACHEREFRESH})

	router.Register(&CommandDef{Name: "NET.WHOIS", Handler: cmdNETWHOIS})
	router.Register(&CommandDef{Name: "NET.DNS", Handler: cmdNETDNS})
	router.Register(&CommandDef{Name: "NET.PING", Handler: cmdNETPING})
	router.Register(&CommandDef{Name: "NET.PORT", Handler: cmdNETPORT})

	router.Register(&CommandDef{Name: "ARRAY.PUSH", Handler: cmdARRAYPUSH})
	router.Register(&CommandDef{Name: "ARRAY.POP", Handler: cmdARRAYPOP})
	router.Register(&CommandDef{Name: "ARRAY.SHIFT", Handler: cmdARRAYSHIFT})
	router.Register(&CommandDef{Name: "ARRAY.UNSHIFT", Handler: cmdARRAYUNSHIFT})
	router.Register(&CommandDef{Name: "ARRAY.SLICE", Handler: cmdARRAYSLICE})
	router.Register(&CommandDef{Name: "ARRAY.SPLICE", Handler: cmdARRAYSPLICE})
	router.Register(&CommandDef{Name: "ARRAY.REVERSE", Handler: cmdARRAYREVERSE})
	router.Register(&CommandDef{Name: "ARRAY.SORT", Handler: cmdARRAYSORT})
	router.Register(&CommandDef{Name: "ARRAY.UNIQUE", Handler: cmdARRAYUNIQUE})
	router.Register(&CommandDef{Name: "ARRAY.FLATTEN", Handler: cmdARRAYFLATTEN})
	router.Register(&CommandDef{Name: "ARRAY.MERGE", Handler: cmdARRAYMERGE})
	router.Register(&CommandDef{Name: "ARRAY.INTERSECT", Handler: cmdARRAYINTERSECT})
	router.Register(&CommandDef{Name: "ARRAY.DIFF", Handler: cmdARRAYDIFF})
	router.Register(&CommandDef{Name: "ARRAY.INDEXOF", Handler: cmdARRAYINDEXOF})
	router.Register(&CommandDef{Name: "ARRAY.LASTINDEXOF", Handler: cmdARRAYLASTINDEXOF})
	router.Register(&CommandDef{Name: "ARRAY.INCLUDES", Handler: cmdARRAYINCLUDES})

	router.Register(&CommandDef{Name: "OBJECT.KEYS", Handler: cmdOBJECTKEYS})
	router.Register(&CommandDef{Name: "OBJECT.VALUES", Handler: cmdOBJECTVALUES})
	router.Register(&CommandDef{Name: "OBJECT.ENTRIES", Handler: cmdOBJECTENTRIES})
	router.Register(&CommandDef{Name: "OBJECT.FROMENTRIES", Handler: cmdOBJECTFROMENTRIES})
	router.Register(&CommandDef{Name: "OBJECT.MERGE", Handler: cmdOBJECTMERGE})
	router.Register(&CommandDef{Name: "OBJECT.PICK", Handler: cmdOBJECTPICK})
	router.Register(&CommandDef{Name: "OBJECT.OMIT", Handler: cmdOBJECTOMIT})
	router.Register(&CommandDef{Name: "OBJECT.HAS", Handler: cmdOBJECTHAS})
	router.Register(&CommandDef{Name: "OBJECT.GET", Handler: cmdOBJECTGET})
	router.Register(&CommandDef{Name: "OBJECT.SET", Handler: cmdOBJECTSET})
	router.Register(&CommandDef{Name: "OBJECT.DELETE", Handler: cmdOBJECTDELETE})

	router.Register(&CommandDef{Name: "MATH.ADD", Handler: cmdMATHADD})
	router.Register(&CommandDef{Name: "MATH.SUB", Handler: cmdMATHSUB})
	router.Register(&CommandDef{Name: "MATH.MUL", Handler: cmdMATHMUL})
	router.Register(&CommandDef{Name: "MATH.DIV", Handler: cmdMATHDIV})
	router.Register(&CommandDef{Name: "MATH.MOD", Handler: cmdMATHMOD})
	router.Register(&CommandDef{Name: "MATH.POW", Handler: cmdMATHPOW})
	router.Register(&CommandDef{Name: "MATH.SQRT", Handler: cmdMATHSQRT})
	router.Register(&CommandDef{Name: "MATH.ABS", Handler: cmdMATHABS})
	router.Register(&CommandDef{Name: "MATH.MIN", Handler: cmdMATHMIN})
	router.Register(&CommandDef{Name: "MATH.MAX", Handler: cmdMATHMAX})
	router.Register(&CommandDef{Name: "MATH.FLOOR", Handler: cmdMATHFLOOR})
	router.Register(&CommandDef{Name: "MATH.CEIL", Handler: cmdMATHCEIL})
	router.Register(&CommandDef{Name: "MATH.ROUND", Handler: cmdMATHROUND})
	router.Register(&CommandDef{Name: "MATH.RANDOM", Handler: cmdMATHRANDOM})
	router.Register(&CommandDef{Name: "MATH.SUM", Handler: cmdMATHSUM})
	router.Register(&CommandDef{Name: "MATH.AVG", Handler: cmdMATHAVG})
	router.Register(&CommandDef{Name: "MATH.MEDIAN", Handler: cmdMATHMEDIAN})
	router.Register(&CommandDef{Name: "MATH.STDDEV", Handler: cmdMATHSTDDEV})

	router.Register(&CommandDef{Name: "GEO.ENCODE", Handler: cmdGEOENCODE})
	router.Register(&CommandDef{Name: "GEO.DECODE", Handler: cmdGEODECODE})
	router.Register(&CommandDef{Name: "GEO.DISTANCE", Handler: cmdGEODISTANCE})
	router.Register(&CommandDef{Name: "GEO.BOUNDINGBOX", Handler: cmdGEOBOUNDINGBOX})

	router.Register(&CommandDef{Name: "CAPTCHA.GENERATE", Handler: cmdCAPTCHAGENERATE})
	router.Register(&CommandDef{Name: "CAPTCHA.VERIFY", Handler: cmdCAPTCHAVERIFY})

	router.Register(&CommandDef{Name: "SEQUENCE.NEXT", Handler: cmdSEQUENCENEXT})
	router.Register(&CommandDef{Name: "SEQUENCE.CURRENT", Handler: cmdSEQUENCECURRENT})
	router.Register(&CommandDef{Name: "SEQUENCE.RESET", Handler: cmdSEQUENCERESET})
	router.Register(&CommandDef{Name: "SEQUENCE.SET", Handler: cmdSEQUENCESET})
}

var (
	circuitBreakersExt   = make(map[string]*CircuitBreakerExt)
	circuitBreakersExtMu sync.RWMutex
)

type CircuitBreakerExt struct {
	Name            string
	State           string
	Failures        int64
	Successes       int64
	LastFailure     int64
	Threshold       int64
	Timeout         int64
	LastStateChange int64
}

func cmdCIRCUITBREAKERCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	threshold := parseInt64(ctx.ArgString(1))
	timeoutMs := parseInt64(ctx.ArgString(2))

	circuitBreakersExtMu.Lock()
	circuitBreakersExt[name] = &CircuitBreakerExt{
		Name:            name,
		State:           "closed",
		Failures:        0,
		Successes:       0,
		Threshold:       threshold,
		Timeout:         timeoutMs,
		LastStateChange: time.Now().UnixMilli(),
	}
	circuitBreakersExtMu.Unlock()

	return ctx.WriteOK()
}

func cmdCIRCUITBREAKERSTATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	// This handler MUTATES the shared breaker, so it needs the WRITE lock.
	// An RLock protects only the map lookup: circuitBreakersExt maps names to
	// *CircuitBreakerExt, so the pointer escapes the lock and every field
	// touched below was being mutated with no lock held at all.
	circuitBreakersExtMu.Lock()
	defer circuitBreakersExtMu.Unlock()

	cb, exists := circuitBreakersExt[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR circuit breaker not found"))
	}

	if cb.State == "open" {
		now := time.Now().UnixMilli()
		if now-cb.LastStateChange >= cb.Timeout {
			cb.State = "half-open"
			cb.LastStateChange = now
		}
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"),
		resp.BulkString(cb.Name),
		resp.BulkString("state"),
		resp.BulkString(cb.State),
		resp.BulkString("failures"),
		resp.IntegerValue(cb.Failures),
		resp.BulkString("successes"),
		resp.IntegerValue(cb.Successes),
	})
}

func cmdCIRCUITBREAKERTRIP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	// This handler MUTATES the shared breaker, so it needs the WRITE lock.
	// An RLock protects only the map lookup: circuitBreakersExt maps names to
	// *CircuitBreakerExt, so the pointer escapes the lock and every field
	// touched below was being mutated with no lock held at all.
	circuitBreakersExtMu.Lock()
	defer circuitBreakersExtMu.Unlock()

	cb, exists := circuitBreakersExt[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR circuit breaker not found"))
	}

	cb.Failures++
	if cb.Failures >= cb.Threshold {
		cb.State = "open"
		cb.LastStateChange = time.Now().UnixMilli()
	}

	return ctx.WriteOK()
}

func cmdCIRCUITBREAKERRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	// This handler MUTATES the shared breaker, so it needs the WRITE lock.
	// An RLock protects only the map lookup: circuitBreakersExt maps names to
	// *CircuitBreakerExt, so the pointer escapes the lock and every field
	// touched below was being mutated with no lock held at all.
	circuitBreakersExtMu.Lock()
	defer circuitBreakersExtMu.Unlock()

	cb, exists := circuitBreakersExt[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR circuit breaker not found"))
	}

	cb.State = "closed"
	cb.Failures = 0
	cb.Successes = 0
	cb.LastStateChange = time.Now().UnixMilli()

	return ctx.WriteOK()
}

var (
	rateLimiters   = make(map[string]*RateLimiterExt)
	rateLimitersMu sync.RWMutex
)

type RateLimiterExt struct {
	Name     string
	Requests map[int64]int64
	Limit    int64
	Window   int64
}

func cmdRATELIMITCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	limit := parseInt64(ctx.ArgString(1))
	windowMs := parseInt64(ctx.ArgString(2))

	rateLimitersMu.Lock()
	rateLimiters[name] = &RateLimiterExt{
		Name:     name,
		Requests: make(map[int64]int64),
		Limit:    limit,
		Window:   windowMs,
	}
	rateLimitersMu.Unlock()

	return ctx.WriteOK()
}

func cmdRATELIMITCHECK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	_ = ctx.ArgString(1)

	// CHECK counts this request: it READS rl.Requests[window] and then WRITES
	// rl.Requests[window]++. rateLimiters maps names to *RateLimiterExt, so an
	// RLock protected only the map lookup and both the counter read and the
	// counter increment ran unlocked -- a data race on a Go map (an
	// unrecoverable process-fatal throw) plus lost increments that could let
	// the limiter never reach its limit.
	rateLimitersMu.Lock()
	defer rateLimitersMu.Unlock()

	rl, exists := rateLimiters[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR rate limiter not found"))
	}

	now := time.Now().UnixMilli()
	window := (now / rl.Window) * rl.Window

	count := rl.Requests[window]
	remaining := rl.Limit - count

	if remaining <= 0 {
		return ctx.WriteArray([]*resp.Value{
			resp.IntegerValue(0),
			resp.IntegerValue(0),
			resp.IntegerValue(rl.Window - (now - window)),
		})
	}

	rl.Requests[window]++

	return ctx.WriteArray([]*resp.Value{
		resp.IntegerValue(1),
		resp.IntegerValue(remaining - 1),
		resp.IntegerValue(rl.Window - (now - window)),
	})
}

func cmdRATELIMITRESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	// CHECK counts this request: it READS rl.Requests[window] and then WRITES
	// rl.Requests[window]++. rateLimiters maps names to *RateLimiterExt, so an
	// RLock protected only the map lookup and both the counter read and the
	// counter increment ran unlocked -- a data race on a Go map (an
	// unrecoverable process-fatal throw) plus lost increments that could let
	// the limiter never reach its limit.
	rateLimitersMu.Lock()
	defer rateLimitersMu.Unlock()

	rl, exists := rateLimiters[name]
	if !exists {
		return ctx.WriteError(fmt.Errorf("ERR rate limiter not found"))
	}

	rl.Requests = make(map[int64]int64)

	return ctx.WriteOK()
}

func cmdRATELIMITDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	rateLimitersMu.Lock()
	defer rateLimitersMu.Unlock()

	if _, exists := rateLimiters[name]; !exists {
		return ctx.WriteInteger(0)
	}

	delete(rateLimiters, name)
	return ctx.WriteInteger(1)
}

var (
	cacheLocks   = make(map[string]*CacheLock)
	cacheLocksMu sync.RWMutex
)

type CacheLock struct {
	Key       string
	Holder    string
	ExpiresAt int64
}

func cmdCACHELOCK(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	holder := ctx.ArgString(1)
	ttlMs := parseInt64(ctx.ArgString(2))

	cacheLocksMu.Lock()
	defer cacheLocksMu.Unlock()

	if lock, exists := cacheLocks[key]; exists {
		if time.Now().UnixMilli() < lock.ExpiresAt && lock.Holder != holder {
			return ctx.WriteInteger(0)
		}
	}

	cacheLocks[key] = &CacheLock{
		Key:       key,
		Holder:    holder,
		ExpiresAt: time.Now().UnixMilli() + ttlMs,
	}

	return ctx.WriteInteger(1)
}

func cmdCACHEUNLOCK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	holder := ctx.ArgString(1)

	cacheLocksMu.Lock()
	defer cacheLocksMu.Unlock()

	if lock, exists := cacheLocks[key]; exists {
		if lock.Holder == holder {
			delete(cacheLocks, key)
			return ctx.WriteInteger(1)
		}
	}

	return ctx.WriteInteger(0)
}

func cmdCACHELOCKED(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	// Read the expiry under the lock. cacheLocks is map[string]*CacheLock, so
	// the RLock protected only the map lookup: the pointer escaped and
	// cmdCACHEREFRESH could rewrite lock.ExpiresAt under the write lock while
	// this handler was still reading it unlocked.
	cacheLocksMu.RLock()
	lock, exists := cacheLocks[key]
	var expiresAt int64
	if exists {
		expiresAt = lock.ExpiresAt
	}
	cacheLocksMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(0)
	}

	if time.Now().UnixMilli() >= expiresAt {
		return ctx.WriteInteger(0)
	}

	return ctx.WriteInteger(1)
}

func cmdCACHEREFRESH(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	holder := ctx.ArgString(1)
	ttlMs := parseInt64(ctx.ArgString(2))

	cacheLocksMu.Lock()
	defer cacheLocksMu.Unlock()

	if lock, exists := cacheLocks[key]; exists {
		if lock.Holder == holder {
			lock.ExpiresAt = time.Now().UnixMilli() + ttlMs
			return ctx.WriteInteger(1)
		}
	}

	return ctx.WriteInteger(0)
}

func cmdNETWHOIS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	domain := ctx.ArgString(0)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("domain"),
		resp.BulkString(domain),
		resp.BulkString("registrar"),
		resp.BulkString("unknown"),
		resp.BulkString("status"),
		resp.BulkString("active"),
	})
}

func cmdNETDNS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	domain := ctx.ArgString(0)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("domain"),
		resp.BulkString(domain),
		resp.BulkString("type"),
		resp.BulkString("A"),
		resp.BulkString("records"),
		resp.ArrayValue([]*resp.Value{
			resp.BulkString("127.0.0.1"),
		}),
	})
}

func cmdNETPING(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	host := ctx.ArgString(0)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("host"),
		resp.BulkString(host),
		resp.BulkString("status"),
		resp.BulkString("reachable"),
		resp.BulkString("latency_ms"),
		resp.IntegerValue(1),
	})
}

func cmdNETPORT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	host := ctx.ArgString(0)
	port := parseInt64(ctx.ArgString(1))

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("host"),
		resp.BulkString(host),
		resp.BulkString("port"),
		resp.IntegerValue(port),
		resp.BulkString("status"),
		resp.BulkString("open"),
	})
}

var (
	arrays   = make(map[string][]string)
	arraysMu sync.RWMutex
)

func cmdARRAYPUSH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	arraysMu.Lock()
	defer arraysMu.Unlock()

	if _, exists := arrays[name]; !exists {
		arrays[name] = make([]string, 0)
	}

	for i := 1; i < ctx.ArgCount(); i++ {
		arrays[name] = append(arrays[name], ctx.ArgString(i))
	}

	return ctx.WriteInteger(int64(len(arrays[name])))
}

func cmdARRAYPOP(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	arraysMu.Lock()
	defer arraysMu.Unlock()

	arr, exists := arrays[name]
	if !exists || len(arr) == 0 {
		return ctx.WriteNull()
	}

	lastIdx := len(arr) - 1
	val := arr[lastIdx]
	arrays[name] = arr[:lastIdx]

	return ctx.WriteBulkString(val)
}

func cmdARRAYSHIFT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	arraysMu.Lock()
	defer arraysMu.Unlock()

	arr, exists := arrays[name]
	if !exists || len(arr) == 0 {
		return ctx.WriteNull()
	}

	val := arr[0]
	arrays[name] = arr[1:]

	return ctx.WriteBulkString(val)
}

func cmdARRAYUNSHIFT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	arraysMu.Lock()
	defer arraysMu.Unlock()

	if _, exists := arrays[name]; !exists {
		arrays[name] = make([]string, 0)
	}

	newArr := make([]string, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		newArr[i-1] = ctx.ArgString(i)
	}
	arrays[name] = append(newArr, arrays[name]...)

	return ctx.WriteInteger(int64(len(arrays[name])))
}

func cmdARRAYSLICE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	start := int(parseInt64(ctx.ArgString(1)))
	end := int(parseInt64(ctx.ArgString(2)))

	arraysMu.RLock()
	src, exists := arrays[name]
	var arr []string
	if exists {
		arr = make([]string, len(src))
		copy(arr, src)
	}
	arraysMu.RUnlock()

	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}

	if start < 0 {
		start = 0
	}
	if end > len(arr) {
		end = len(arr)
	}
	if start >= end {
		return ctx.WriteArray([]*resp.Value{})
	}

	slice := arr[start:end]
	results := make([]*resp.Value, len(slice))
	for i, v := range slice {
		results[i] = resp.BulkString(v)
	}

	return ctx.WriteArray(results)
}

func cmdARRAYSPLICE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	start := int(parseInt64(ctx.ArgString(1)))
	deleteCount := int(parseInt64(ctx.ArgString(2)))

	arraysMu.Lock()
	defer arraysMu.Unlock()

	arr, exists := arrays[name]
	if !exists {
		arrays[name] = make([]string, 0)
		return ctx.WriteArray([]*resp.Value{})
	}

	if start < 0 {
		start = 0
	}
	if start > len(arr) {
		start = len(arr)
	}
	if deleteCount < 0 {
		deleteCount = 0
	}
	if start+deleteCount > len(arr) {
		deleteCount = len(arr) - start
	}

	deleted := arr[start : start+deleteCount]
	var newArr []string
	newArr = append(newArr, arr[:start]...)

	for i := 3; i < ctx.ArgCount(); i++ {
		newArr = append(newArr, ctx.ArgString(i))
	}

	newArr = append(newArr, arr[start+deleteCount:]...)
	arrays[name] = newArr

	results := make([]*resp.Value, len(deleted))
	for i, v := range deleted {
		results[i] = resp.BulkString(v)
	}

	return ctx.WriteArray(results)
}

func cmdARRAYREVERSE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	arraysMu.Lock()
	defer arraysMu.Unlock()

	arr, exists := arrays[name]
	if !exists {
		return ctx.WriteOK()
	}

	for i, j := 0, len(arr)-1; i < j; i, j = i+1, j-1 {
		arr[i], arr[j] = arr[j], arr[i]
	}

	return ctx.WriteOK()
}

func cmdARRAYSORT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	desc := ctx.ArgCount() >= 2 && ctx.ArgString(1) == "DESC"

	arraysMu.Lock()
	defer arraysMu.Unlock()

	arr, exists := arrays[name]
	if !exists {
		return ctx.WriteOK()
	}

	for i := 0; i < len(arr)-1; i++ {
		for j := i + 1; j < len(arr); j++ {
			if (!desc && arr[i] > arr[j]) || (desc && arr[i] < arr[j]) {
				arr[i], arr[j] = arr[j], arr[i]
			}
		}
	}

	return ctx.WriteOK()
}

func cmdARRAYUNIQUE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	arraysMu.Lock()
	defer arraysMu.Unlock()

	arr, exists := arrays[name]
	if !exists {
		return ctx.WriteInteger(0)
	}

	seen := make(map[string]bool)
	unique := make([]string, 0)
	for _, v := range arr {
		if !seen[v] {
			seen[v] = true
			unique = append(unique, v)
		}
	}

	removed := len(arr) - len(unique)
	arrays[name] = unique

	return ctx.WriteInteger(int64(removed))
}

func cmdARRAYFLATTEN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	arraysMu.RLock()
	src, exists := arrays[name]
	var arr []string
	if exists {
		arr = make([]string, len(src))
		copy(arr, src)
	}
	arraysMu.RUnlock()

	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}

	results := make([]*resp.Value, len(arr))
	for i, v := range arr {
		results[i] = resp.BulkString(v)
	}

	return ctx.WriteArray(results)
}

func cmdARRAYMERGE(ctx *Context) error {
	// Only ArgString(0) and ArgString(1) are read; a third argument was never
	// consumed, so requiring one made the merge unreachable on its own
	// two-argument contract (ARRAY.INCLUDES and friends guard with < 2).
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	dest := ctx.ArgString(0)
	src := ctx.ArgString(1)

	arraysMu.Lock()
	defer arraysMu.Unlock()

	destArr, _ := arrays[dest]
	srcArr, exists := arrays[src]
	if !exists {
		return ctx.WriteInteger(int64(len(destArr)))
	}

	arrays[dest] = append(destArr, srcArr...)

	return ctx.WriteInteger(int64(len(arrays[dest])))
}

func cmdARRAYINTERSECT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name1 := ctx.ArgString(0)
	name2 := ctx.ArgString(1)

	arraysMu.RLock()
	src1, _ := arrays[name1]
	src2, _ := arrays[name2]
	arr1 := make([]string, len(src1))
	copy(arr1, src1)
	arr2 := make([]string, len(src2))
	copy(arr2, src2)
	arraysMu.RUnlock()

	set2 := make(map[string]bool)
	for _, v := range arr2 {
		set2[v] = true
	}

	seen := make(map[string]bool)
	results := make([]*resp.Value, 0)
	for _, v := range arr1 {
		if set2[v] && !seen[v] {
			seen[v] = true
			results = append(results, resp.BulkString(v))
		}
	}

	return ctx.WriteArray(results)
}

func cmdARRAYDIFF(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name1 := ctx.ArgString(0)
	name2 := ctx.ArgString(1)

	arraysMu.RLock()
	src1, _ := arrays[name1]
	src2, _ := arrays[name2]
	arr1 := make([]string, len(src1))
	copy(arr1, src1)
	arr2 := make([]string, len(src2))
	copy(arr2, src2)
	arraysMu.RUnlock()

	set2 := make(map[string]bool)
	for _, v := range arr2 {
		set2[v] = true
	}

	results := make([]*resp.Value, 0)
	for _, v := range arr1 {
		if !set2[v] {
			results = append(results, resp.BulkString(v))
		}
	}

	return ctx.WriteArray(results)
}

func cmdARRAYINDEXOF(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	value := ctx.ArgString(1)

	// Snapshot under the read lock. Copying only the slice header is not
	// enough: a slice header is a value, but the elements live in a backing
	// array that ARRAY.REVERSE and ARRAY.SORT mutate in place under the write
	// lock. Iterating after RUnlock therefore raced with those writers.
	arraysMu.RLock()
	src, exists := arrays[name]
	var arr []string
	if exists {
		arr = make([]string, len(src))
		copy(arr, src)
	}
	arraysMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(-1)
	}

	for i, v := range arr {
		if v == value {
			return ctx.WriteInteger(int64(i))
		}
	}

	return ctx.WriteInteger(-1)
}

func cmdARRAYLASTINDEXOF(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	value := ctx.ArgString(1)

	arraysMu.RLock()
	src, exists := arrays[name]
	var arr []string
	if exists {
		arr = make([]string, len(src))
		copy(arr, src)
	}
	arraysMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(-1)
	}

	for i := len(arr) - 1; i >= 0; i-- {
		if arr[i] == value {
			return ctx.WriteInteger(int64(i))
		}
	}

	return ctx.WriteInteger(-1)
}

func cmdARRAYINCLUDES(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	value := ctx.ArgString(1)

	arraysMu.RLock()
	src, exists := arrays[name]
	var arr []string
	if exists {
		arr = make([]string, len(src))
		copy(arr, src)
	}
	arraysMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(0)
	}

	for _, v := range arr {
		if v == value {
			return ctx.WriteInteger(1)
		}
	}

	return ctx.WriteInteger(0)
}

var (
	objects   = make(map[string]map[string]string)
	objectsMu sync.RWMutex
)

func cmdOBJECTKEYS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	// Snapshot the inner map under the read lock. Copying only the map
	// header captures a reference, not its contents: cmdOBJECTSET writes
	// objects[name][key] and cmdOBJECTOMIT deletes keys in place under the
	// write lock, so reading this map after RUnlock was a data race -- and on
	// a Go map, concurrent read and write is an unrecoverable runtime throw.
	objectsMu.RLock()
	src, exists := objects[name]
	var obj map[string]string
	if exists {
		obj = make(map[string]string, len(src))
		for k, v := range src {
			obj[k] = v
		}
	}
	objectsMu.RUnlock()

	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}

	results := make([]*resp.Value, 0, len(obj))
	for k := range obj {
		results = append(results, resp.BulkString(k))
	}

	return ctx.WriteArray(results)
}

func cmdOBJECTVALUES(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	// Snapshot under the read lock; see cmdOBJECTKEYS.
	objectsMu.RLock()
	src, exists := objects[name]
	var obj map[string]string
	if exists {
		obj = make(map[string]string, len(src))
		for k, v := range src {
			obj[k] = v
		}
	}
	objectsMu.RUnlock()

	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}

	results := make([]*resp.Value, 0, len(obj))
	for _, v := range obj {
		results = append(results, resp.BulkString(v))
	}

	return ctx.WriteArray(results)
}

func cmdOBJECTENTRIES(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	// Snapshot under the read lock; see cmdOBJECTKEYS.
	objectsMu.RLock()
	src, exists := objects[name]
	var obj map[string]string
	if exists {
		obj = make(map[string]string, len(src))
		for k, v := range src {
			obj[k] = v
		}
	}
	objectsMu.RUnlock()

	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}

	results := make([]*resp.Value, 0, len(obj)*2)
	for k, v := range obj {
		results = append(results, resp.BulkString(k), resp.BulkString(v))
	}

	return ctx.WriteArray(results)
}

func cmdOBJECTFROMENTRIES(ctx *Context) error {
	// OBJECT.FROMENTRIES takes a name followed by field/value PAIRS. The old
	// guard was `< 2`, which let a name plus a single dangling field through:
	// the pair loop below then never ran, an empty object was still stored under
	// that name, and the command replied +OK. The same gap silently dropped a
	// trailing odd field name (`obj f1 v1 f2` reported success and discarded
	// f2). Require at least one complete pair and an even number of arguments
	// after the name, matching the sibling writer cmdOBJECTSET's `< 3`.
	if ctx.ArgCount() < 3 || (ctx.ArgCount()-1)%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	objectsMu.Lock()
	defer objectsMu.Unlock()

	obj := make(map[string]string)
	for i := 1; i+1 < ctx.ArgCount(); i += 2 {
		obj[ctx.ArgString(i)] = ctx.ArgString(i + 1)
	}

	objects[name] = obj

	return ctx.WriteOK()
}

func cmdOBJECTMERGE(ctx *Context) error {
	// Only ArgString(0) and ArgString(1) are read; see cmdARRAYMERGE.
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	dest := ctx.ArgString(0)
	src := ctx.ArgString(1)

	objectsMu.Lock()
	defer objectsMu.Unlock()

	if _, exists := objects[dest]; !exists {
		objects[dest] = make(map[string]string)
	}

	srcObj, exists := objects[src]
	if !exists {
		return ctx.WriteInteger(0)
	}

	count := 0
	for k, v := range srcObj {
		objects[dest][k] = v
		count++
	}

	return ctx.WriteInteger(int64(count))
}

func cmdOBJECTPICK(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	// PICK is a read-modify-write and must be atomic. Snapshotting under
	// RLock and storing the result back under a later Lock left a window in
	// which a concurrent OBJECT.SET could insert a field that the overwrite
	// then discarded.
	objectsMu.Lock()
	defer objectsMu.Unlock()

	obj, exists := objects[name]
	if !exists {
		return ctx.WriteArray([]*resp.Value{})
	}

	picked := make(map[string]string)
	for i := 1; i < ctx.ArgCount(); i++ {
		key := ctx.ArgString(i)
		if val, ok := obj[key]; ok {
			picked[key] = val
		}
	}

	objects[name] = picked

	return ctx.WriteInteger(int64(len(picked)))
}

func cmdOBJECTOMIT(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	objectsMu.Lock()
	defer objectsMu.Unlock()

	obj, exists := objects[name]
	if !exists {
		return ctx.WriteInteger(0)
	}

	omitSet := make(map[string]bool)
	for i := 1; i < ctx.ArgCount(); i++ {
		omitSet[ctx.ArgString(i)] = true
	}

	count := 0
	for k := range omitSet {
		if _, ok := obj[k]; ok {
			delete(obj, k)
			count++
		}
	}

	return ctx.WriteInteger(int64(count))
}

func cmdOBJECTHAS(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	key := ctx.ArgString(1)

	// Snapshot under the read lock; see cmdOBJECTKEYS.
	objectsMu.RLock()
	src, exists := objects[name]
	obj := make(map[string]string, len(src))
	for k, v := range src {
		obj[k] = v
	}
	objectsMu.RUnlock()

	if !exists {
		return ctx.WriteInteger(0)
	}

	if _, ok := obj[key]; ok {
		return ctx.WriteInteger(1)
	}

	return ctx.WriteInteger(0)
}

func cmdOBJECTGET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	key := ctx.ArgString(1)

	// Snapshot under the read lock; see cmdOBJECTKEYS.
	objectsMu.RLock()
	src, exists := objects[name]
	obj := make(map[string]string, len(src))
	for k, v := range src {
		obj[k] = v
	}
	objectsMu.RUnlock()

	if !exists {
		return ctx.WriteNull()
	}

	if val, ok := obj[key]; ok {
		return ctx.WriteBulkString(val)
	}

	return ctx.WriteNull()
}

func cmdOBJECTSET(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	key := ctx.ArgString(1)
	value := ctx.ArgString(2)

	objectsMu.Lock()
	defer objectsMu.Unlock()

	if _, exists := objects[name]; !exists {
		objects[name] = make(map[string]string)
	}

	objects[name][key] = value

	return ctx.WriteOK()
}

func cmdOBJECTDELETE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	key := ctx.ArgString(1)

	objectsMu.Lock()
	defer objectsMu.Unlock()

	obj, exists := objects[name]
	if !exists {
		return ctx.WriteInteger(0)
	}

	if _, ok := obj[key]; ok {
		delete(obj, key)
		return ctx.WriteInteger(1)
	}

	return ctx.WriteInteger(0)
}

func cmdMATHADD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	result := parseInt64(ctx.ArgString(0))
	for i := 1; i < ctx.ArgCount(); i++ {
		result += parseInt64(ctx.ArgString(i))
	}

	return ctx.WriteInteger(result)
}

func cmdMATHSUB(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	result := parseInt64(ctx.ArgString(0))
	for i := 1; i < ctx.ArgCount(); i++ {
		result -= parseInt64(ctx.ArgString(i))
	}

	return ctx.WriteInteger(result)
}

func cmdMATHMUL(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	result := parseInt64(ctx.ArgString(0))
	for i := 1; i < ctx.ArgCount(); i++ {
		result *= parseInt64(ctx.ArgString(i))
	}

	return ctx.WriteInteger(result)
}

func cmdMATHDIV(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	result := parseInt64(ctx.ArgString(0))
	for i := 1; i < ctx.ArgCount(); i++ {
		divisor := parseInt64(ctx.ArgString(i))
		if divisor == 0 {
			return ctx.WriteError(fmt.Errorf("ERR division by zero"))
		}
		result /= divisor
	}

	return ctx.WriteInteger(result)
}

func cmdMATHMOD(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	a := parseInt64(ctx.ArgString(0))
	b := parseInt64(ctx.ArgString(1))

	if b == 0 {
		return ctx.WriteError(fmt.Errorf("ERR division by zero"))
	}

	return ctx.WriteInteger(a % b)
}

func cmdMATHPOW(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	base := parseInt64(ctx.ArgString(0))
	exp := parseInt64(ctx.ArgString(1))

	result := int64(1)
	for i := int64(0); i < exp; i++ {
		result *= base
	}

	return ctx.WriteInteger(result)
}

func cmdMATHSQRT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	n := parseFloatExt([]byte(ctx.ArgString(0)))
	if n < 0 {
		return ctx.WriteError(fmt.Errorf("ERR negative number"))
	}

	result := n / 2
	for i := 0; i < 20; i++ {
		result = (result + n/result) / 2
	}

	return ctx.WriteBulkString(fmt.Sprintf("%.6f", result))
}

func cmdMATHABS(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	n := parseInt64(ctx.ArgString(0))
	if n < 0 {
		n = -n
	}

	return ctx.WriteInteger(n)
}

func cmdMATHMIN(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	min := parseInt64(ctx.ArgString(0))
	for i := 1; i < ctx.ArgCount(); i++ {
		val := parseInt64(ctx.ArgString(i))
		if val < min {
			min = val
		}
	}

	return ctx.WriteInteger(min)
}

func cmdMATHMAX(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	max := parseInt64(ctx.ArgString(0))
	for i := 1; i < ctx.ArgCount(); i++ {
		val := parseInt64(ctx.ArgString(i))
		if val > max {
			max = val
		}
	}

	return ctx.WriteInteger(max)
}

func cmdMATHFLOOR(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	n := parseFloatExt([]byte(ctx.ArgString(0)))
	result := int64(n)

	return ctx.WriteInteger(result)
}

func cmdMATHCEIL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	n := parseFloatExt([]byte(ctx.ArgString(0)))
	result := int64(n)
	if float64(result) < n {
		result++
	}

	return ctx.WriteInteger(result)
}

func cmdMATHROUND(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	n := parseFloatExt([]byte(ctx.ArgString(0)))
	result := int64(n + 0.5)

	return ctx.WriteInteger(result)
}

func cmdMATHRANDOM(ctx *Context) error {
	min := int64(0)
	max := int64(100)

	if ctx.ArgCount() >= 2 {
		min = parseInt64(ctx.ArgString(0))
		max = parseInt64(ctx.ArgString(1))
	}

	seed := time.Now().UnixNano()
	range_ := max - min + 1
	result := (seed % range_) + min

	return ctx.WriteInteger(result)
}

func cmdMATHSUM(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	var sum int64
	for i := 0; i < ctx.ArgCount(); i++ {
		sum += parseInt64(ctx.ArgString(i))
	}

	return ctx.WriteInteger(sum)
}

func cmdMATHAVG(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	var sum int64
	count := ctx.ArgCount()
	for i := 0; i < count; i++ {
		sum += parseInt64(ctx.ArgString(i))
	}

	return ctx.WriteBulkString(fmt.Sprintf("%.2f", float64(sum)/float64(count)))
}

func cmdMATHMEDIAN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	values := make([]int64, ctx.ArgCount())
	for i := 0; i < ctx.ArgCount(); i++ {
		values[i] = parseInt64(ctx.ArgString(i))
	}

	for i := 0; i < len(values)-1; i++ {
		for j := i + 1; j < len(values); j++ {
			if values[i] > values[j] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}

	n := len(values)
	if n%2 == 1 {
		return ctx.WriteInteger(values[n/2])
	}

	median := float64(values[n/2-1]+values[n/2]) / 2
	return ctx.WriteBulkString(fmt.Sprintf("%.2f", median))
}

func cmdMATHSTDDEV(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	var sum int64
	n := ctx.ArgCount()
	values := make([]int64, n)

	for i := 0; i < n; i++ {
		values[i] = parseInt64(ctx.ArgString(i))
		sum += values[i]
	}

	mean := float64(sum) / float64(n)
	var variance float64

	for _, v := range values {
		diff := float64(v) - mean
		variance += diff * diff
	}

	variance /= float64(n)
	stddev := sqrtFloat(variance)

	return ctx.WriteBulkString(fmt.Sprintf("%.6f", stddev))
}

func sqrtFloat(x float64) float64 {
	if x < 0 {
		return 0
	}
	z := x
	for i := 0; i < 20; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func cmdGEOENCODE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	lat := parseFloatExt([]byte(ctx.ArgString(0)))
	lon := parseFloatExt([]byte(ctx.ArgString(1)))

	geohash := encodeGeohash(lat, lon)

	return ctx.WriteBulkString(geohash)
}

func encodeGeohash(lat, lon float64) string {
	const base32 = "0123456789bcdefghjkmnpqrstuvwxyz"

	latMin, latMax := -90.0, 90.0
	lonMin, lonMax := -180.0, 180.0

	var bits uint64
	for i := 0; i < 30; i++ {
		if i%2 == 0 {
			mid := (lonMin + lonMax) / 2
			if lon >= mid {
				bits |= 1 << (59 - i)
				lonMin = mid
			} else {
				lonMax = mid
			}
		} else {
			mid := (latMin + latMax) / 2
			if lat >= mid {
				bits |= 1 << (59 - i)
				latMin = mid
			} else {
				latMax = mid
			}
		}
	}

	result := ""
	for i := 0; i < 12; i++ {
		idx := (bits >> uint(55-i*5)) & 0x1F
		result += string(base32[idx])
	}

	return result
}

func cmdGEODECODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	geohash := ctx.ArgString(0)

	lat, lon := decodeGeohash(geohash)

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("lat"),
		resp.BulkString(fmt.Sprintf("%.6f", lat)),
		resp.BulkString("lon"),
		resp.BulkString(fmt.Sprintf("%.6f", lon)),
	})
}

func decodeGeohash(geohash string) (float64, float64) {
	const base32 = "0123456789bcdefghjkmnpqrstuvwxyz"

	decodeMap := make(map[rune]int)
	for i, c := range base32 {
		decodeMap[c] = i
	}

	var bits uint64
	for _, c := range geohash {
		if val, ok := decodeMap[c]; ok {
			bits = (bits << 5) | uint64(val)
		}
	}

	latMin, latMax := -90.0, 90.0
	lonMin, lonMax := -180.0, 180.0

	for i := 0; i < 30; i++ {
		if i%2 == 0 {
			mid := (lonMin + lonMax) / 2
			if bits&(1<<uint(59-i)) != 0 {
				lonMin = mid
			} else {
				lonMax = mid
			}
		} else {
			mid := (latMin + latMax) / 2
			if bits&(1<<uint(59-i)) != 0 {
				latMin = mid
			} else {
				latMax = mid
			}
		}
	}

	return (latMin + latMax) / 2, (lonMin + lonMax) / 2
}

func cmdGEODISTANCE(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	lat1 := parseFloatExt([]byte(ctx.ArgString(0)))
	lon1 := parseFloatExt([]byte(ctx.ArgString(1)))
	lat2 := parseFloatExt([]byte(ctx.ArgString(2)))
	lon2 := parseFloatExt([]byte(ctx.ArgString(3)))

	dist := haversine(lat1, lon1, lat2, lon2)

	unit := "km"
	if ctx.ArgCount() >= 5 {
		unit = ctx.ArgString(4)
	}

	switch unit {
	case "m", "meters":
		dist *= 1000
	case "mi", "miles":
		dist *= 0.621371
	case "ft", "feet":
		dist *= 3280.84
	}

	return ctx.WriteBulkString(fmt.Sprintf("%.2f", dist))
}

func cmdGEOBOUNDINGBOX(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	lat := parseFloatExt([]byte(ctx.ArgString(0)))
	lon := parseFloatExt([]byte(ctx.ArgString(1)))
	radiusKm := parseFloatExt([]byte(ctx.ArgString(2)))

	latOffset := radiusKm / 111.0
	lonOffset := radiusKm / (111.0 * cosFloat(lat*0.01745329252))

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("min_lat"),
		resp.BulkString(fmt.Sprintf("%.6f", lat-latOffset)),
		resp.BulkString("min_lon"),
		resp.BulkString(fmt.Sprintf("%.6f", lon-lonOffset)),
		resp.BulkString("max_lat"),
		resp.BulkString(fmt.Sprintf("%.6f", lat+latOffset)),
		resp.BulkString("max_lon"),
		resp.BulkString(fmt.Sprintf("%.6f", lon+lonOffset)),
	})
}

func cosFloat(x float64) float64 {
	return sinFloat(x + 1.57079632679)
}

func sinFloat(x float64) float64 {
	result := x
	term := x
	for i := 1; i < 10; i++ {
		term *= -x * x / float64(2*i*(2*i+1))
		result += term
	}
	return result
}

var (
	captchas   = make(map[string]*Captcha)
	captchasMu sync.RWMutex
)

type Captcha struct {
	ID        string
	Answer    string
	ExpiresAt int64
}

func cmdCAPTCHAGENERATE(ctx *Context) error {
	length := 6
	if ctx.ArgCount() >= 1 {
		length = int(parseInt64(ctx.ArgString(0)))
	}

	id := generateUUID()
	answer := ""
	for i := 0; i < length; i++ {
		answer += string(rune('0' + randomInt(10)))
	}

	captchasMu.Lock()
	captchas[id] = &Captcha{
		ID:        id,
		Answer:    answer,
		ExpiresAt: time.Now().UnixMilli() + 300000,
	}
	captchasMu.Unlock()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"),
		resp.BulkString(id),
		resp.BulkString("answer"),
		resp.BulkString(answer),
	})
}

func cmdCAPTCHAVERIFY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	id := ctx.ArgString(0)
	answer := ctx.ArgString(1)

	captchasMu.Lock()
	defer captchasMu.Unlock()

	captcha, exists := captchas[id]
	if !exists {
		return ctx.WriteInteger(0)
	}

	if time.Now().UnixMilli() > captcha.ExpiresAt {
		delete(captchas, id)
		return ctx.WriteInteger(0)
	}

	if captcha.Answer == answer {
		delete(captchas, id)
		return ctx.WriteInteger(1)
	}

	return ctx.WriteInteger(0)
}

var (
	sequences   = make(map[string]int64)
	sequencesMu sync.RWMutex
)

func cmdSEQUENCENEXT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	increment := int64(1)
	if ctx.ArgCount() >= 2 {
		increment = parseInt64(ctx.ArgString(1))
	}

	sequencesMu.Lock()
	defer sequencesMu.Unlock()

	sequences[name] += increment
	return ctx.WriteInteger(sequences[name])
}

func cmdSEQUENCECURRENT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	sequencesMu.RLock()
	val := sequences[name]
	sequencesMu.RUnlock()

	return ctx.WriteInteger(val)
}

func cmdSEQUENCERESET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)

	sequencesMu.Lock()
	sequences[name] = 0
	sequencesMu.Unlock()

	return ctx.WriteOK()
}

func cmdSEQUENCESET(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	value := parseInt64(ctx.ArgString(1))

	sequencesMu.Lock()
	sequences[name] = value
	sequencesMu.Unlock()

	return ctx.WriteOK()
}
