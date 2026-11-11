package command

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// A CIRCUITX breaker is a hash key holding its settings, its state and its
// counters, times in unix milliseconds. Callers ask CIRCUITX.ALLOW before a
// call and report its outcome with CIRCUITX.RECORD. In the closed state
// failure_threshold consecutive failures open it; once it has been open for
// timeout milliseconds it turns half-open, where success_threshold
// successes close it and any failure opens it again. Being a key, a breaker
// is per database, persisted and replicated: an open breaker stays open
// across a restart. Changes are recorded as HSET of the fields that moved.
//
// CIRCUITBREAKER is the same breaker under the older, smaller command set.

const (
	circuitClosed   = "closed"
	circuitOpen     = "open"
	circuitHalfOpen = "half-open"
)

var (
	circuitXKind   = recordKind{name: "circuit", fields: []string{"state", "failures", "successes", "failure_threshold", "success_threshold", "timeout", "last_failure", "last_state_change", "total_failures", "total_successes", "rejected"}}
	errNoCircuitX  = errors.New("ERR circuit not found")
	errNoCircuitBr = errors.New("ERR circuit breaker not found")
)

type circuitX struct {
	state                                           string
	failures, successes, failThreshold, okThreshold int64
	timeout, lastFailure, lastChange                int64
	totalFailures, totalSuccesses, rejected         int64
}

func loadCircuitX(ctx *Context, key string, missing error) (*circuitX, error) {
	r, err := circuitXKind.read(ctx, key)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, missing
	}
	c := &circuitX{
		state: r.vals["state"], failures: r.int("failures"), successes: r.int("successes"),
		failThreshold: r.int("failure_threshold"), okThreshold: r.int("success_threshold"),
		timeout: r.int("timeout"), lastFailure: r.int("last_failure"), lastChange: r.int("last_state_change"),
		totalFailures: r.int("total_failures"), totalSuccesses: r.int("total_successes"), rejected: r.int("rejected"),
	}
	switch c.state {
	case circuitClosed, circuitOpen, circuitHalfOpen:
	default:
		r.bad = true
	}
	return c, errWrongTypeOr(r.bad, nil)
}

func (c *circuitX) pairs() []string {
	return []string{
		"state", c.state, "failures", i64(c.failures), "successes", i64(c.successes),
		"failure_threshold", i64(c.failThreshold), "success_threshold", i64(c.okThreshold),
		"timeout", i64(c.timeout), "last_failure", i64(c.lastFailure), "last_state_change", i64(c.lastChange),
		"total_failures", i64(c.totalFailures), "total_successes", i64(c.totalSuccesses), "rejected", i64(c.rejected),
	}
}

func (c *circuitX) setState(state string, now int64) {
	c.state, c.failures, c.successes, c.lastChange = state, 0, 0, now
}

// refresh turns an open breaker half-open once its timeout has passed.
func (c *circuitX) refresh(now int64) {
	if c.state == circuitOpen && now-c.lastChange >= c.timeout {
		c.setState(circuitHalfOpen, now)
	}
}

func (c *circuitX) retryAfter(now int64) int64 {
	if c.state != circuitOpen {
		return 0
	}
	return max(c.lastChange+c.timeout-now, 0)
}

func (c *circuitX) record(success bool, now int64) {
	c.refresh(now)
	if success {
		c.totalSuccesses++
		switch c.state {
		case circuitClosed:
			c.failures = 0
		case circuitHalfOpen:
			c.successes++
			if c.successes >= c.okThreshold {
				c.setState(circuitClosed, now)
			}
		}
		return
	}
	c.totalFailures++
	c.lastFailure = now
	switch c.state {
	case circuitClosed:
		c.failures++
		if c.failures >= c.failThreshold {
			c.setState(circuitOpen, now)
		}
	case circuitHalfOpen:
		c.setState(circuitOpen, now)
	}
}

func parseCircuitInt(s string, min int64) (int64, bool) {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v < min {
		return 0, false
	}
	return v, true
}

func createCircuitX(ctx *Context, key string, failThreshold, timeout, okThreshold int64) error {
	if _, err := circuitXKind.read(ctx, key); err != nil {
		return ctx.WriteError(err)
	}
	c := &circuitX{state: circuitClosed, failThreshold: failThreshold, okThreshold: okThreshold, timeout: timeout, lastChange: ctx.nowMS()}
	if err := replaceRecord(ctx, key, c.pairs()...); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// CIRCUITX.CREATE name failure_threshold timeout_ms [success_threshold]
// creates or replaces the breaker, closed.
func cmdCIRCUITXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 3 || ctx.ArgCount() > 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
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
	return createCircuitX(ctx, ctx.ArgString(0), failureThreshold, timeoutMs, successThreshold)
}

// changeCircuitX loads the breaker named by the first argument, applies
// change and stores it, replying with reply.
func changeCircuitX(ctx *Context, missing error, change func(c *circuitX, now int64), reply func(c *circuitX) error) error {
	key := ctx.ArgString(0)
	c, err := loadCircuitX(ctx, key, missing)
	if err != nil {
		return ctx.WriteError(err)
	}
	before := strings.Join(c.pairs(), "\x00")
	change(c, ctx.nowMS())
	if strings.Join(c.pairs(), "\x00") != before {
		if err := updateRecord(ctx, key, c.pairs()...); err != nil {
			return ctx.WriteError(err)
		}
	} else {
		ctx.PropagateAs()
	}
	return reply(c)
}

func okReply(ctx *Context) func(*circuitX) error {
	return func(*circuitX) error { return ctx.WriteOK() }
}

// setCircuitXState forces a breaker into a state, as the manual OPEN, CLOSE,
// HALFOPEN and RESET commands do.
func setCircuitXState(ctx *Context, state string, resetTotals bool) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return changeCircuitX(ctx, errNoCircuitX, func(c *circuitX, now int64) {
		c.setState(state, now)
		if state == circuitOpen {
			c.lastFailure = now
		}
		if resetTotals {
			c.totalFailures, c.totalSuccesses, c.rejected, c.lastFailure = 0, 0, 0, 0
		}
	}, okReply(ctx))
}

func cmdCIRCUITXOPEN(ctx *Context) error     { return setCircuitXState(ctx, circuitOpen, false) }
func cmdCIRCUITXCLOSE(ctx *Context) error    { return setCircuitXState(ctx, circuitClosed, false) }
func cmdCIRCUITXHALFOPEN(ctx *Context) error { return setCircuitXState(ctx, circuitHalfOpen, false) }
func cmdCIRCUITXRESET(ctx *Context) error    { return setCircuitXState(ctx, circuitClosed, true) }

func cmdCIRCUITXALLOW(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	allowed := false
	return changeCircuitX(ctx, errNoCircuitX, func(c *circuitX, now int64) {
		c.refresh(now)
		allowed = c.state != circuitOpen
		if !allowed {
			c.rejected++
		}
	}, func(*circuitX) error { return ctx.WriteInteger(utilityBoolToInt(allowed)) })
}

func cmdCIRCUITXRECORD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	var success bool
	switch strings.ToUpper(ctx.ArgString(1)) {
	case "SUCCESS":
		success = true
	case "FAILURE":
	default:
		return ctx.WriteError(errors.New("ERR outcome must be SUCCESS or FAILURE"))
	}
	return changeCircuitX(ctx, errNoCircuitX, func(c *circuitX, now int64) { c.record(success, now) },
		func(c *circuitX) error { return ctx.WriteBulkString(c.state) })
}

func cmdCIRCUITXSTATUS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	c, err := loadCircuitX(ctx, ctx.ArgString(0), errNoCircuitX)
	if err != nil {
		return ctx.WriteError(err)
	}
	now := ctx.nowMS()
	c.refresh(now)
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(ctx.ArgString(0)),
		resp.BulkString("state"), resp.BulkString(c.state),
		resp.BulkString("failures"), resp.IntegerValue(c.failures),
		resp.BulkString("successes"), resp.IntegerValue(c.successes),
		resp.BulkString("failure_threshold"), resp.IntegerValue(c.failThreshold),
		resp.BulkString("success_threshold"), resp.IntegerValue(c.okThreshold),
		resp.BulkString("timeout"), resp.IntegerValue(c.timeout),
		resp.BulkString("retry_after"), resp.IntegerValue(c.retryAfter(now)),
	})
}

func cmdCIRCUITXMETRICS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	c, err := loadCircuitX(ctx, ctx.ArgString(0), errNoCircuitX)
	if err != nil {
		return ctx.WriteError(err)
	}
	rate := 0.0
	if total := c.totalFailures + c.totalSuccesses; total > 0 {
		rate = float64(c.totalFailures) / float64(total)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("total_failures"), resp.IntegerValue(c.totalFailures),
		resp.BulkString("total_successes"), resp.IntegerValue(c.totalSuccesses),
		resp.BulkString("rejected"), resp.IntegerValue(c.rejected),
		resp.BulkString("failure_rate"), resp.BulkString(fmt.Sprintf("%.2f", rate)),
	})
}

func cmdCIRCUITXDELETE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return deleteRecord(ctx, circuitXKind)
}

// CIRCUITBREAKER.CREATE name threshold timeout_ms
func cmdCIRCUITBREAKERCREATE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	threshold, ok := parseCircuitInt(ctx.ArgString(1), 1)
	if !ok {
		return ctx.WriteError(errors.New("ERR threshold must be a positive integer"))
	}
	timeoutMs, ok := parseCircuitInt(ctx.ArgString(2), 0)
	if !ok {
		return ctx.WriteError(errors.New("ERR timeout must be a non-negative integer"))
	}
	return createCircuitX(ctx, ctx.ArgString(0), threshold, timeoutMs, 1)
}

// CIRCUITBREAKER.STATE name replies name, state, failures and successes,
// turning an open breaker half-open once its timeout has passed.
func cmdCIRCUITBREAKERSTATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return changeCircuitX(ctx, errNoCircuitBr, func(c *circuitX, now int64) { c.refresh(now) }, func(c *circuitX) error {
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(ctx.ArgString(0)),
			resp.BulkString("state"), resp.BulkString(c.state),
			resp.BulkString("failures"), resp.IntegerValue(c.failures),
			resp.BulkString("successes"), resp.IntegerValue(c.successes),
		})
	})
}

// CIRCUITBREAKER.TRIP name records a failure.
func cmdCIRCUITBREAKERTRIP(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return changeCircuitX(ctx, errNoCircuitBr, func(c *circuitX, now int64) { c.record(false, now) }, okReply(ctx))
}

// CIRCUITBREAKER.RESET name closes the breaker.
func cmdCIRCUITBREAKERRESET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return changeCircuitX(ctx, errNoCircuitBr, func(c *circuitX, now int64) { c.setState(circuitClosed, now) }, okReply(ctx))
}
