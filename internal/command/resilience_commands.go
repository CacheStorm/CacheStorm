package command

import (
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"strconv"
	"strings"
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
	router.Register(&CommandDef{Name: "CIRCUITX.ALLOW", Handler: cmdCIRCUITXALLOW})
	router.Register(&CommandDef{Name: "CIRCUITX.RECORD", Handler: cmdCIRCUITXRECORD})

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
	circuitsMu sync.Mutex
)

const (
	circuitClosed   = "closed"
	circuitOpen     = "open"
	circuitHalfOpen = "half-open"
)

// Circuit is a circuit breaker. Callers ask CIRCUITX.ALLOW before a call and
// report its outcome with CIRCUITX.RECORD. In the closed state
// FailureThreshold consecutive failures open the circuit; once it has been
// open for Timeout milliseconds it turns half-open, where SuccessThreshold
// successes close it and any failure opens it again.
type Circuit struct {
	Name             string
	State            string
	Failures         int64
	Successes        int64
	FailureThreshold int64
	SuccessThreshold int64
	Timeout          int64
	LastFailure      int64
	LastStateChange  int64
	TotalFailures    int64
	TotalSuccesses   int64
	Rejected         int64
}

func (c *Circuit) setState(state string, now int64) {
	c.State = state
	c.Failures = 0
	c.Successes = 0
	c.LastStateChange = now
}

func (c *Circuit) refresh(now int64) {
	if c.State == circuitOpen && now-c.LastStateChange >= c.Timeout {
		c.setState(circuitHalfOpen, now)
	}
}

func (c *Circuit) retryAfter(now int64) int64 {
	if c.State != circuitOpen {
		return 0
	}
	if left := c.LastStateChange + c.Timeout - now; left > 0 {
		return left
	}
	return 0
}

func parseCircuitInt(s string, min int64) (int64, bool) {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v < min {
		return 0, false
	}
	return v, true
}

func cmdCIRCUITXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 || ctx.ArgCount() > 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	failureThreshold, ok := parseCircuitInt(ctx.ArgString(1), 1)
	if !ok {
		return ctx.WriteError(errors.New("ERR failure threshold must be a positive integer"))
	}
	timeoutMs, ok := parseCircuitInt(ctx.ArgString(2), 0)
	if !ok {
		return ctx.WriteError(errors.New("ERR timeout must be a non-negative integer"))
	}
	successThreshold := int64(1)
	if ctx.ArgCount() == 4 {
		if successThreshold, ok = parseCircuitInt(ctx.ArgString(3), 1); !ok {
			return ctx.WriteError(errors.New("ERR success threshold must be a positive integer"))
		}
	}
	circuitsMu.Lock()
	if _, exists := circuits[name]; !exists && len(circuits) >= 10000 {
		circuitsMu.Unlock()
		return ctx.WriteError(errors.New("ERR maximum number of circuits reached"))
	}
	circuits[name] = &Circuit{
		Name:             name,
		State:            circuitClosed,
		FailureThreshold: failureThreshold,
		SuccessThreshold: successThreshold,
		Timeout:          timeoutMs,
		LastStateChange:  time.Now().UnixMilli(),
	}
	circuitsMu.Unlock()
	return ctx.WriteOK()
}

// setCircuitState forces a circuit into a state, as the manual OPEN, CLOSE,
// HALFOPEN and RESET commands do.
func setCircuitState(ctx *Context, state string, resetTotals bool) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	circuitsMu.Lock()
	c, exists := circuits[name]
	if exists {
		now := time.Now().UnixMilli()
		c.setState(state, now)
		if state == circuitOpen {
			c.LastFailure = now
		}
		if resetTotals {
			c.TotalFailures, c.TotalSuccesses, c.Rejected, c.LastFailure = 0, 0, 0, 0
		}
	}
	circuitsMu.Unlock()
	if !exists {
		return ctx.WriteError(errors.New("ERR circuit not found"))
	}
	return ctx.WriteOK()
}

func cmdCIRCUITXOPEN(ctx *Context) error {
	return setCircuitState(ctx, circuitOpen, false)
}

func cmdCIRCUITXCLOSE(ctx *Context) error {
	return setCircuitState(ctx, circuitClosed, false)
}

func cmdCIRCUITXHALFOPEN(ctx *Context) error {
	return setCircuitState(ctx, circuitHalfOpen, false)
}

func cmdCIRCUITXRESET(ctx *Context) error {
	return setCircuitState(ctx, circuitClosed, true)
}

func cmdCIRCUITXALLOW(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	circuitsMu.Lock()
	c, exists := circuits[name]
	allowed := false
	if exists {
		c.refresh(time.Now().UnixMilli())
		allowed = c.State != circuitOpen
		if !allowed {
			c.Rejected++
		}
	}
	circuitsMu.Unlock()
	if !exists {
		return ctx.WriteError(errors.New("ERR circuit not found"))
	}
	if allowed {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdCIRCUITXRECORD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	var success bool
	switch strings.ToUpper(ctx.ArgString(1)) {
	case "SUCCESS":
		success = true
	case "FAILURE":
	default:
		return ctx.WriteError(errors.New("ERR outcome must be SUCCESS or FAILURE"))
	}
	circuitsMu.Lock()
	c, exists := circuits[name]
	state := ""
	if exists {
		now := time.Now().UnixMilli()
		c.refresh(now)
		if success {
			c.TotalSuccesses++
			switch c.State {
			case circuitClosed:
				c.Failures = 0
			case circuitHalfOpen:
				c.Successes++
				if c.Successes >= c.SuccessThreshold {
					c.setState(circuitClosed, now)
				}
			}
		} else {
			c.TotalFailures++
			c.LastFailure = now
			switch c.State {
			case circuitClosed:
				c.Failures++
				if c.Failures >= c.FailureThreshold {
					c.setState(circuitOpen, now)
				}
			case circuitHalfOpen:
				c.setState(circuitOpen, now)
			}
		}
		state = c.State
	}
	circuitsMu.Unlock()
	if !exists {
		return ctx.WriteError(errors.New("ERR circuit not found"))
	}
	return ctx.WriteBulkString(state)
}

func cmdCIRCUITXSTATUS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	circuitsMu.Lock()
	c, exists := circuits[name]
	if !exists {
		circuitsMu.Unlock()
		return ctx.WriteError(errors.New("ERR circuit not found"))
	}
	now := time.Now().UnixMilli()
	c.refresh(now)
	reply := []*resp.Value{
		resp.BulkString("name"), resp.BulkString(c.Name),
		resp.BulkString("state"), resp.BulkString(c.State),
		resp.BulkString("failures"), resp.IntegerValue(c.Failures),
		resp.BulkString("successes"), resp.IntegerValue(c.Successes),
		resp.BulkString("failure_threshold"), resp.IntegerValue(c.FailureThreshold),
		resp.BulkString("success_threshold"), resp.IntegerValue(c.SuccessThreshold),
		resp.BulkString("timeout"), resp.IntegerValue(c.Timeout),
		resp.BulkString("retry_after"), resp.IntegerValue(c.retryAfter(now)),
	}
	circuitsMu.Unlock()
	return ctx.WriteArray(reply)
}

func cmdCIRCUITXMETRICS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	circuitsMu.Lock()
	c, exists := circuits[name]
	if !exists {
		circuitsMu.Unlock()
		return ctx.WriteError(errors.New("ERR circuit not found"))
	}
	rate := 0.0
	if total := c.TotalFailures + c.TotalSuccesses; total > 0 {
		rate = float64(c.TotalFailures) / float64(total)
	}
	reply := []*resp.Value{
		resp.BulkString("total_failures"), resp.IntegerValue(c.TotalFailures),
		resp.BulkString("total_successes"), resp.IntegerValue(c.TotalSuccesses),
		resp.BulkString("rejected"), resp.IntegerValue(c.Rejected),
		resp.BulkString("failure_rate"), resp.BulkString(fmt.Sprintf("%.2f", rate)),
	}
	circuitsMu.Unlock()
	return ctx.WriteArray(reply)
}

func cmdCIRCUITXDELETE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	circuitsMu.Lock()
	_, exists := circuits[name]
	delete(circuits, name)
	circuitsMu.Unlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(1)
}

// resilienceMaxWait caps how long PROMISE.AWAIT, FUTURE.GET,
// RATELIMITER.WAIT and BULKHEAD.ACQUIRE block; a zero timeout means the cap.
const resilienceMaxWait = 5 * time.Minute

func parseResilienceTimeout(s string) (time.Duration, error) {
	ms, err := strconv.ParseInt(s, 10, 64)
	if err != nil || ms < 0 {
		return 0, errors.New("ERR timeout is not an integer or out of range")
	}
	if ms == 0 || ms > resilienceMaxWait.Milliseconds() {
		return resilienceMaxWait, nil
	}
	return time.Duration(ms) * time.Millisecond, nil
}

// beginResilienceWait releases the connection's gate for the time a command
// blocks; the returned function takes it back.
func beginResilienceWait(ctx *Context) func() {
	if ctx.BeforeWait == nil {
		return func() {}
	}
	ctx.BeforeWait()
	if ctx.AfterWait == nil {
		return func() {}
	}
	return ctx.AfterWait
}

// resilienceWait blocks until ch is closed, d passes or the client goes away,
// and reports whether ch was closed.
func resilienceWait(ctx *Context, ch <-chan struct{}, d time.Duration) bool {
	if d <= 0 {
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ch:
		return true
	case <-t.C:
		return false
	case <-ctx.Done:
		return false
	}
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
	// changed is closed, and replaced, when RESET, DELETE or a new CREATE
	// frees slots early, so a blocked RATELIMITER.WAIT looks again.
	changed chan struct{}
}

func (rl *RateLimiterX) notify() {
	close(rl.changed)
	rl.changed = make(chan struct{})
}

func (rl *RateLimiterX) prune(now int64) {
	cutoff := now - rl.Window
	i := 0
	for i < len(rl.Requests) && rl.Requests[i] <= cutoff {
		i++
	}
	if i > 0 {
		rl.Requests = append(rl.Requests[:0], rl.Requests[i:]...)
	}
}

// take records a request if a slot is free; otherwise it returns how many
// milliseconds remain until the oldest request that blocks one leaves the
// window.
func (rl *RateLimiterX) take(now int64) (bool, int64) {
	rl.prune(now)
	n := int64(len(rl.Requests))
	if n < rl.Limit {
		rl.Requests = append(rl.Requests, now)
		return true, 0
	}
	wait := rl.Requests[n-rl.Limit] + rl.Window - now
	if wait < 1 {
		wait = 1
	}
	return false, wait
}

func cmdRATELIMITERCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 || ctx.ArgCount() > 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	limit, ok := parseCircuitInt(ctx.ArgString(1), 1)
	if !ok {
		return ctx.WriteError(errors.New("ERR limit must be a positive integer"))
	}
	windowMs, ok := parseCircuitInt(ctx.ArgString(2), 1)
	if !ok {
		return ctx.WriteError(errors.New("ERR window must be a positive integer"))
	}
	strategy := "sliding"
	if ctx.ArgCount() == 4 {
		strategy = ctx.ArgString(3)
	}
	rateLimitersXMux.Lock()
	old, exists := rateLimitersX[name]
	if !exists && len(rateLimitersX) >= 10000 {
		rateLimitersXMux.Unlock()
		return ctx.WriteError(errors.New("ERR maximum number of rate limiters reached"))
	}
	if exists {
		old.notify()
	}
	rateLimitersX[name] = &RateLimiterX{
		Name:     name,
		Limit:    limit,
		Window:   windowMs,
		Requests: make([]int64, 0),
		Strategy: strategy,
		changed:  make(chan struct{}),
	}
	rateLimitersXMux.Unlock()
	return ctx.WriteOK()
}

func cmdRATELIMITERTRY(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	rateLimitersXMux.Lock()
	rl, exists := rateLimitersX[name]
	if !exists {
		rateLimitersXMux.Unlock()
		return ctx.WriteError(errors.New("ERR rate limiter not found"))
	}
	allowed, _ := rl.take(time.Now().UnixMilli())
	remaining := rl.Limit - int64(len(rl.Requests))
	window := rl.Window
	rateLimitersXMux.Unlock()
	if !allowed {
		return ctx.WriteArray([]*resp.Value{
			resp.IntegerValue(0),
			resp.IntegerValue(0),
			resp.IntegerValue(window),
		})
	}
	return ctx.WriteArray([]*resp.Value{
		resp.IntegerValue(1),
		resp.IntegerValue(remaining),
		resp.IntegerValue(window),
	})
}

// cmdRATELIMITERWAIT blocks until the limiter has a free slot, takes it and
// replies with the milliseconds it waited, or a null reply on timeout.
func cmdRATELIMITERWAIT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	timeout, err := parseResilienceTimeout(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	start := time.Now()
	deadline := start.Add(timeout)
	var endWait func()
	defer func() {
		if endWait != nil {
			endWait()
		}
	}()
	for {
		rateLimitersXMux.Lock()
		rl, exists := rateLimitersX[name]
		if !exists {
			rateLimitersXMux.Unlock()
			return ctx.WriteError(errors.New("ERR rate limiter not found"))
		}
		now := time.Now()
		allowed, waitMs := rl.take(now.UnixMilli())
		changed := rl.changed
		rateLimitersXMux.Unlock()
		if allowed {
			return ctx.WriteInteger(now.Sub(start).Milliseconds())
		}
		left := deadline.Sub(now)
		if ctx.InExec || left <= 0 {
			return ctx.WriteNull()
		}
		if endWait == nil {
			endWait = beginResilienceWait(ctx)
		}
		d := time.Duration(waitMs) * time.Millisecond
		if d > left {
			d = left
		}
		if !resilienceWait(ctx, changed, d) {
			select {
			case <-ctx.Done:
				return ctx.WriteNull()
			default:
			}
		}
	}
}

func cmdRATELIMITERRESET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	rateLimitersXMux.Lock()
	rl, exists := rateLimitersX[name]
	if exists {
		rl.Requests = make([]int64, 0)
		rl.notify()
	}
	rateLimitersXMux.Unlock()
	if !exists {
		return ctx.WriteError(errors.New("ERR rate limiter not found"))
	}
	return ctx.WriteOK()
}

func cmdRATELIMITERSTATUS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	rateLimitersXMux.RLock()
	rl, exists := rateLimitersX[name]
	if !exists {
		rateLimitersXMux.RUnlock()
		return ctx.WriteError(errors.New("ERR rate limiter not found"))
	}
	cutoff := time.Now().UnixMilli() - rl.Window
	current := int64(0)
	for _, ts := range rl.Requests {
		if ts > cutoff {
			current++
		}
	}
	reply := []*resp.Value{
		resp.BulkString("name"), resp.BulkString(rl.Name),
		resp.BulkString("limit"), resp.IntegerValue(rl.Limit),
		resp.BulkString("current"), resp.IntegerValue(current),
		resp.BulkString("strategy"), resp.BulkString(rl.Strategy),
	}
	rateLimitersXMux.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdRATELIMITERDELETE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	rateLimitersXMux.Lock()
	rl, exists := rateLimitersX[name]
	if exists {
		rl.notify()
		delete(rateLimitersX, name)
	}
	rateLimitersXMux.Unlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
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

// Bulkhead limits concurrent work to MaxConcurrent permits. Waiting counts
// the BULKHEAD.ACQUIRE calls blocked for a permit right now; Rejected counts
// the acquires that gave up without one.
type Bulkhead struct {
	Name          string
	MaxConcurrent int
	Current       int
	Waiting       int
	Rejected      int64
	// released is closed, and replaced, whenever a permit may have become
	// free, waking the blocked acquires.
	released chan struct{}
}

func (b *Bulkhead) notify() {
	close(b.released)
	b.released = make(chan struct{})
}

func cmdBULKHEADCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	maxConcurrent, ok := parseCircuitInt(ctx.ArgString(1), 1)
	if !ok || maxConcurrent > 1<<31-1 {
		return ctx.WriteError(errors.New("ERR max concurrent must be a positive integer"))
	}
	bulkheadsMu.Lock()
	old, exists := bulkheads[name]
	if !exists && len(bulkheads) >= 10000 {
		bulkheadsMu.Unlock()
		return ctx.WriteError(errors.New("ERR maximum number of bulkheads reached"))
	}
	if exists {
		old.notify()
	}
	bulkheads[name] = &Bulkhead{Name: name, MaxConcurrent: int(maxConcurrent), released: make(chan struct{})}
	bulkheadsMu.Unlock()
	return ctx.WriteOK()
}

// cmdBULKHEADACQUIRE takes a permit and replies 1, or 0 when none is free.
// With a timeout it blocks for up to that long waiting for one.
func cmdBULKHEADACQUIRE(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	var timeout time.Duration
	if ctx.ArgCount() == 2 {
		var err error
		if timeout, err = parseResilienceTimeout(ctx.ArgString(1)); err != nil {
			return ctx.WriteError(err)
		}
	}
	if ctx.InExec {
		timeout = 0
	}
	deadline := time.Now().Add(timeout)
	var endWait func()
	defer func() {
		if endWait != nil {
			endWait()
		}
	}()
	var waitingOn *Bulkhead
	for {
		bulkheadsMu.Lock()
		if waitingOn != nil {
			waitingOn.Waiting--
		}
		b, exists := bulkheads[name]
		if !exists || (waitingOn != nil && b != waitingOn) {
			bulkheadsMu.Unlock()
			return ctx.WriteError(errors.New("ERR bulkhead not found"))
		}
		if b.Current < b.MaxConcurrent {
			b.Current++
			bulkheadsMu.Unlock()
			return ctx.WriteInteger(1)
		}
		left := time.Until(deadline)
		cancelled := false
		select {
		case <-ctx.Done:
			cancelled = true
		default:
		}
		if left <= 0 || cancelled {
			b.Rejected++
			bulkheadsMu.Unlock()
			return ctx.WriteInteger(0)
		}
		b.Waiting++
		waitingOn = b
		released := b.released
		bulkheadsMu.Unlock()
		if endWait == nil {
			endWait = beginResilienceWait(ctx)
		}
		resilienceWait(ctx, released, left)
	}
}

func cmdBULKHEADRELEASE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	bulkheadsMu.Lock()
	b, exists := bulkheads[name]
	held := exists && b.Current > 0
	if held {
		b.Current--
		b.notify()
	}
	bulkheadsMu.Unlock()
	if !exists {
		return ctx.WriteError(errors.New("ERR bulkhead not found"))
	}
	if !held {
		return ctx.WriteError(errors.New("ERR bulkhead has no permit to release"))
	}
	return ctx.WriteOK()
}

func cmdBULKHEADSTATUS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	bulkheadsMu.RLock()
	b, exists := bulkheads[name]
	if !exists {
		bulkheadsMu.RUnlock()
		return ctx.WriteError(errors.New("ERR bulkhead not found"))
	}
	reply := []*resp.Value{
		resp.BulkString("name"), resp.BulkString(b.Name),
		resp.BulkString("current"), resp.IntegerValue(int64(b.Current)),
		resp.BulkString("max"), resp.IntegerValue(int64(b.MaxConcurrent)),
		resp.BulkString("waiting"), resp.IntegerValue(int64(b.Waiting)),
		resp.BulkString("rejected"), resp.IntegerValue(b.Rejected),
	}
	bulkheadsMu.RUnlock()
	return ctx.WriteArray(reply)
}

func cmdBULKHEADDELETE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	bulkheadsMu.Lock()
	b, exists := bulkheads[name]
	if exists {
		b.notify()
		delete(bulkheads, name)
	}
	bulkheadsMu.Unlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
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
	// settled is closed once the promise is resolved or rejected.
	settled chan struct{}
}

func cmdPROMISECREATE(ctx *Context) error {
	id := generateUUID()
	promisesMu.Lock()
	promises[id] = &Promise{ID: id, Status: "pending", Created: time.Now().UnixMilli(), settled: make(chan struct{})}
	promisesMu.Unlock()
	return ctx.WriteBulkString(id)
}

func settlePromise(ctx *Context, status string) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	value := ctx.ArgString(1)
	promisesMu.Lock()
	p, exists := promises[id]
	ok := exists && p.Status == "pending"
	if ok {
		p.Status = status
		if status == "resolved" {
			p.Value = value
		} else {
			p.Error = value
		}
		close(p.settled)
	}
	promisesMu.Unlock()
	if !ok {
		return ctx.WriteError(errors.New("ERR promise not found or not pending"))
	}
	return ctx.WriteOK()
}

func cmdPROMISERESOLVE(ctx *Context) error {
	return settlePromise(ctx, "resolved")
}

func cmdPROMISEREJECT(ctx *Context) error {
	return settlePromise(ctx, "rejected")
}

func cmdPROMISESTATUS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	promisesMu.RLock()
	p, exists := promises[id]
	if !exists {
		promisesMu.RUnlock()
		return ctx.WriteError(errors.New("ERR promise not found"))
	}
	reply := []*resp.Value{
		resp.BulkString("id"), resp.BulkString(p.ID),
		resp.BulkString("status"), resp.BulkString(p.Status),
	}
	promisesMu.RUnlock()
	return ctx.WriteArray(reply)
}

// cmdPROMISEAWAIT blocks until the promise settles, for up to timeout_ms.
// It replies with the resolved value, the rejection as an error, or a null
// reply when the promise is still pending at the timeout.
func cmdPROMISEAWAIT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	timeout, err := parseResilienceTimeout(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	promisesMu.RLock()
	p, exists := promises[id]
	var settled chan struct{}
	if exists {
		settled = p.settled
	}
	promisesMu.RUnlock()
	if !exists {
		return ctx.WriteError(errors.New("ERR promise not found"))
	}
	if ctx.InExec {
		timeout = 0
	}
	select {
	case <-settled:
	default:
		if timeout <= 0 {
			return ctx.WriteNull()
		}
		endWait := beginResilienceWait(ctx)
		ok := resilienceWait(ctx, settled, timeout)
		endWait()
		if !ok {
			return ctx.WriteNull()
		}
	}
	promisesMu.RLock()
	status, value, reason := p.Status, p.Value, p.Error
	promisesMu.RUnlock()
	if status == "resolved" {
		return ctx.WriteBulkString(value)
	}
	return ctx.WriteError(errors.New("ERR promise rejected: " + reason))
}

var (
	futures   = make(map[string]*Future)
	futuresMu sync.RWMutex
)

// Future moves once from pending to completed or cancelled.
type Future struct {
	ID     string
	Status string
	Value  string
	// done is closed once the future is completed or cancelled.
	done chan struct{}
}

func cmdFUTURECREATE(ctx *Context) error {
	id := generateUUID()
	futuresMu.Lock()
	futures[id] = &Future{ID: id, Status: "pending", done: make(chan struct{})}
	futuresMu.Unlock()
	return ctx.WriteBulkString(id)
}

func finishFuture(ctx *Context, id, status, value string) error {
	futuresMu.Lock()
	f, exists := futures[id]
	if !exists {
		futuresMu.Unlock()
		return ctx.WriteError(errors.New("ERR future not found"))
	}
	if f.Status != "pending" {
		current := f.Status
		futuresMu.Unlock()
		return ctx.WriteError(fmt.Errorf("ERR future is already %s", current))
	}
	f.Status = status
	f.Value = value
	close(f.done)
	futuresMu.Unlock()
	return ctx.WriteOK()
}

func cmdFUTURECOMPLETE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return finishFuture(ctx, ctx.ArgString(0), "completed", ctx.ArgString(1))
}

func cmdFUTURECANCEL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return finishFuture(ctx, ctx.ArgString(0), "cancelled", "")
}

// cmdFUTUREGET replies with the completed value, an error for a cancelled
// future, or a null reply while it is pending. With a timeout it first
// blocks for up to that long for the future to finish.
func cmdFUTUREGET(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	var timeout time.Duration
	if ctx.ArgCount() == 2 {
		var err error
		if timeout, err = parseResilienceTimeout(ctx.ArgString(1)); err != nil {
			return ctx.WriteError(err)
		}
	}
	futuresMu.RLock()
	f, exists := futures[id]
	var done chan struct{}
	if exists {
		done = f.done
	}
	futuresMu.RUnlock()
	if !exists {
		return ctx.WriteError(errors.New("ERR future not found"))
	}
	if ctx.InExec {
		timeout = 0
	}
	select {
	case <-done:
	default:
		if timeout <= 0 {
			return ctx.WriteNull()
		}
		endWait := beginResilienceWait(ctx)
		ok := resilienceWait(ctx, done, timeout)
		endWait()
		if !ok {
			return ctx.WriteNull()
		}
	}
	futuresMu.RLock()
	status, value := f.Status, f.Value
	futuresMu.RUnlock()
	if status == "completed" {
		return ctx.WriteBulkString(value)
	}
	return ctx.WriteError(errors.New("ERR future was cancelled"))
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

// DebounceX keeps the argument of the latest CALL. The call fires once no
// other CALL has arrived for Delay milliseconds: only then does FLUSH hand
// the argument out, unless FLUSH is given FORCE.
type DebounceX struct {
	Name       string
	Delay      int64
	LastCall   int64
	Pending    bool
	PendingArg string
}

func cmdDEBOUNCEXCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	delayMs, ok := parseCircuitInt(ctx.ArgString(1), 0)
	if !ok {
		return ctx.WriteError(errors.New("ERR delay must be a non-negative integer"))
	}
	debouncesXMux.Lock()
	if _, exists := debouncesX[name]; !exists && len(debouncesX) >= 10000 {
		debouncesXMux.Unlock()
		return ctx.WriteError(errors.New("ERR maximum number of debouncers reached"))
	}
	debouncesX[name] = &DebounceX{Name: name, Delay: delayMs}
	debouncesXMux.Unlock()
	return ctx.WriteOK()
}

func cmdDEBOUNCEXCALL(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	arg := ctx.ArgString(1)
	debouncesXMux.Lock()
	d, exists := debouncesX[name]
	if exists {
		d.LastCall = time.Now().UnixMilli()
		d.Pending = true
		d.PendingArg = arg
	}
	debouncesXMux.Unlock()
	if !exists {
		return ctx.WriteError(errors.New("ERR debounce not found"))
	}
	return ctx.WriteOK()
}

func cmdDEBOUNCEXCANCEL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	debouncesXMux.Lock()
	d, exists := debouncesX[name]
	if exists {
		d.Pending = false
		d.PendingArg = ""
	}
	debouncesXMux.Unlock()
	if !exists {
		return ctx.WriteError(errors.New("ERR debounce not found"))
	}
	return ctx.WriteOK()
}

// cmdDEBOUNCEXFLUSH replies with the pending argument, and clears it, once
// the delay has passed since the last CALL; FORCE flushes it at once. It
// replies null when nothing is pending or the delay has not passed yet.
func cmdDEBOUNCEXFLUSH(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	force := false
	if ctx.ArgCount() == 2 {
		if !strings.EqualFold(ctx.ArgString(1), "FORCE") {
			return ctx.WriteError(ErrSyntaxError)
		}
		force = true
	}
	debouncesXMux.Lock()
	d, exists := debouncesX[name]
	if !exists {
		debouncesXMux.Unlock()
		return ctx.WriteError(errors.New("ERR debounce not found"))
	}
	fire := d.Pending && (force || time.Now().UnixMilli()-d.LastCall >= d.Delay)
	arg := d.PendingArg
	if fire {
		d.Pending = false
		d.PendingArg = ""
	}
	debouncesXMux.Unlock()
	if !fire {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(arg)
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

const maxJoinXEntries = 100000

// JoinX is an inner join on a key: GET pairs every left value with every
// right value added under the same key.
type JoinX struct {
	Name    string
	Left    map[string][]string
	Right   map[string][]string
	Keys    []string
	Entries int
}

func cmdJOINXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	// JoinX is always an inner join on the key given to ADD; there is nowhere
	// to record another join type or condition, so they are rejected rather
	// than silently dropped.
	if ctx.ArgCount() > 1 {
		return ctx.WriteError(fmt.Errorf("ERR JOINX.CREATE does not support type or condition arguments"))
	}
	name := ctx.ArgString(0)
	joinsXMux.Lock()
	if _, exists := joinsX[name]; !exists && len(joinsX) >= 10000 {
		joinsXMux.Unlock()
		return ctx.WriteError(errors.New("ERR maximum number of joins reached"))
	}
	joinsX[name] = &JoinX{Name: name, Left: make(map[string][]string), Right: make(map[string][]string)}
	joinsXMux.Unlock()
	return ctx.WriteOK()
}

func cmdJOINXADD(ctx *Context) error {
	if ctx.ArgCount() != 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	side := strings.ToUpper(ctx.ArgString(1))
	if side != "LEFT" && side != "RIGHT" {
		return ctx.WriteError(errors.New("ERR side must be LEFT or RIGHT"))
	}
	key := ctx.ArgString(2)
	value := ctx.ArgString(3)
	joinsXMux.Lock()
	j, exists := joinsX[name]
	if !exists {
		joinsXMux.Unlock()
		return ctx.WriteError(errors.New("ERR join not found"))
	}
	if j.Entries >= maxJoinXEntries {
		joinsXMux.Unlock()
		return ctx.WriteError(errors.New("ERR join is full"))
	}
	_, inLeft := j.Left[key]
	_, inRight := j.Right[key]
	if !inLeft && !inRight {
		j.Keys = append(j.Keys, key)
	}
	if side == "LEFT" {
		j.Left[key] = append(j.Left[key], value)
	} else {
		j.Right[key] = append(j.Right[key], value)
	}
	j.Entries++
	joinsXMux.Unlock()
	return ctx.WriteOK()
}

// cmdJOINXGET replies with one [key k left l right r] row for every left and
// right value that share a key, keys in the order they were first added.
// An optional key limits the rows to that key.
func cmdJOINXGET(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	joinsXMux.RLock()
	j, exists := joinsX[name]
	if !exists {
		joinsXMux.RUnlock()
		return ctx.WriteArray([]*resp.Value{})
	}
	keys := j.Keys
	if ctx.ArgCount() == 2 {
		keys = []string{ctx.ArgString(1)}
	}
	rows := 0
	for _, k := range keys {
		rows += len(j.Left[k]) * len(j.Right[k])
		if rows > maxJoinXEntries {
			joinsXMux.RUnlock()
			return ctx.WriteError(errors.New("ERR join result too large"))
		}
	}
	results := make([]*resp.Value, 0, rows)
	for _, k := range keys {
		for _, l := range j.Left[k] {
			for _, r := range j.Right[k] {
				results = append(results, resp.ArrayValue([]*resp.Value{
					resp.BulkString("key"), resp.BulkString(k),
					resp.BulkString("left"), resp.BulkString(l),
					resp.BulkString("right"), resp.BulkString(r),
				}))
			}
		}
	}
	joinsXMux.RUnlock()
	return ctx.WriteArray(results)
}

func cmdJOINXDELETE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	joinsXMux.Lock()
	_, exists := joinsX[name]
	delete(joinsX, name)
	joinsXMux.Unlock()
	if !exists {
		return ctx.WriteInteger(0)
	}
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
