package command

import (
	"errors"
	"strconv"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A CIRCUIT is a hash key holding a circuit breaker's settings and state:
// state, failures, successes, failure_threshold, success_threshold,
// timeout_ms, and the times in milliseconds of the last failure, success and
// state change. Being a key, it is per database, persisted, replicated and
// removed by DEL and FLUSHALL. Every change is recorded as an HSET of the
// fields with their new values, so replay does not depend on the clock and
// Redis can load the AOF.

var (
	errNoCircuit  = errors.New("ERR circuit breaker not found")
	errNotCircuit = errors.New("WRONGTYPE Operation against a key holding a value that is not a circuit breaker")
)

var circuitFields = []string{"state", "failures", "successes", "failure_threshold", "success_threshold", "timeout_ms", "last_failure", "last_success", "last_state_change"}

type circuit struct {
	state                                            string
	failures, successes, failThreshold, okThreshold  int64
	timeoutMs, lastFailure, lastSuccess, lastChanged int64
}

func readCircuit(ctx *Context, key string) (*circuit, error) {
	entry, ok := ctx.Store.Get(key)
	if !ok {
		return nil, errNoCircuit
	}
	return circuitOf(entry)
}

func circuitOf(entry *store.Entry) (*circuit, error) {
	h, ok := entry.Value.(*store.HashValue)
	if !ok {
		return nil, errNotCircuit
	}
	h.RLock()
	defer h.RUnlock()
	if h.Len() != len(circuitFields) {
		return nil, errNotCircuit
	}
	c := &circuit{state: string(h.Field("state"))}
	nums := []*int64{nil, &c.failures, &c.successes, &c.failThreshold, &c.okThreshold, &c.timeoutMs, &c.lastFailure, &c.lastSuccess, &c.lastChanged}
	for i, f := range circuitFields[1:] {
		n, err := strconv.ParseInt(string(h.Field(f)), 10, 64)
		if err != nil {
			return nil, errNotCircuit
		}
		*nums[i+1] = n
	}
	switch c.state {
	case "closed", "open", "half-open":
	default:
		return nil, errNotCircuit
	}
	return c, nil
}

// writeCircuit stores c at key, recorded as one HSET of every field.
func writeCircuit(ctx *Context, key string, c *circuit) error {
	vals := []string{c.state}
	for _, n := range []int64{c.failures, c.successes, c.failThreshold, c.okThreshold, c.timeoutMs, c.lastFailure, c.lastSuccess, c.lastChanged} {
		vals = append(vals, strconv.FormatInt(n, 10))
	}
	args := [][]byte{[]byte(key)}
	for i, f := range circuitFields {
		args = append(args, []byte(f), []byte(vals[i]))
	}
	if _, err := ctx.Store.ApplyHashPairs(key, false, args[1:]); err != nil {
		return err
	}
	ctx.notify(store.NotifyHash, "hset", key)
	ctx.PropagateAs(ExecutedCommand{Name: "HSET", Args: args, DB: ctx.DB})
	return nil
}

func (c *circuit) setState(state string, now int64) {
	if c.state != state {
		c.state, c.lastChanged = state, now
	}
}

// CIRCUIT.CREATE name failure_threshold success_threshold timeout_ms creates
// the breaker, closed; an existing one is kept as it is.
func cmdCIRCUITCREATE(ctx *Context) error {
	if ctx.ArgCount() != 4 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	failT, err1 := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	okT, err2 := strconv.ParseInt(ctx.ArgString(2), 10, 64)
	timeout, err3 := strconv.ParseInt(ctx.ArgString(3), 10, 64)
	if err1 != nil || err2 != nil || err3 != nil || failT < 1 || okT < 1 || timeout < 0 {
		return ctx.WriteError(errors.New("ERR thresholds must be positive integers and the timeout a non-negative integer"))
	}
	c, err := readCircuit(ctx, key)
	switch {
	case errors.Is(err, errNoCircuit):
		c = &circuit{state: "closed", failThreshold: failT, okThreshold: okT, timeoutMs: timeout, lastChanged: time.Now().UnixMilli()}
		if err := writeCircuit(ctx, key, c); err != nil {
			return ctx.WriteError(err)
		}
	case err != nil:
		return ctx.WriteError(err)
	default:
		ctx.PropagateAs()
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(key),
		resp.BulkString("state"), resp.BulkString(c.state),
	})
}

// circuitUpdate reads the breaker at the first argument, lets change update
// it, and writes it back when change reports a change.
func circuitUpdate(ctx *Context, change func(c *circuit, now int64) bool, reply func(c *circuit) error) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	c, err := readCircuit(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if change(c, time.Now().UnixMilli()) {
		if err := writeCircuit(ctx, key, c); err != nil {
			return ctx.WriteError(err)
		}
	} else {
		ctx.PropagateAs()
	}
	return reply(c)
}

// CIRCUIT.ALLOW name replies 1 when a call may go through: always while
// closed or half-open, and once the timeout has passed since the last
// failure while open, which moves the breaker to half-open.
func cmdCIRCUITALLOW(ctx *Context) error {
	allowed := false
	return circuitUpdate(ctx, func(c *circuit, now int64) bool {
		switch c.state {
		case "closed", "half-open":
			allowed = true
		case "open":
			if now-c.lastFailure > c.timeoutMs {
				c.setState("half-open", now)
				c.successes = 0
				allowed = true
				return true
			}
		}
		return false
	}, func(*circuit) error {
		if allowed {
			return ctx.WriteInteger(1)
		}
		return ctx.WriteInteger(0)
	})
}

// CIRCUIT.SUCCESS name records a successful call: success_threshold of them
// while half-open close the breaker.
func cmdCIRCUITSUCCESS(ctx *Context) error {
	return circuitUpdate(ctx, func(c *circuit, now int64) bool {
		c.successes++
		c.lastSuccess = now
		if c.state == "half-open" && c.successes >= c.okThreshold {
			c.setState("closed", now)
			c.failures = 0
		}
		return true
	}, func(*circuit) error { return ctx.WriteOK() })
}

// CIRCUIT.FAILURE name records a failed call: failure_threshold of them
// while closed, or one while half-open, open the breaker.
func cmdCIRCUITFAILURE(ctx *Context) error {
	return circuitUpdate(ctx, func(c *circuit, now int64) bool {
		c.failures++
		c.lastFailure = now
		if c.state == "half-open" || (c.state == "closed" && c.failures >= c.failThreshold) {
			c.setState("open", now)
		}
		return true
	}, func(*circuit) error { return ctx.WriteOK() })
}

// CIRCUIT.RESET name closes the breaker and clears its counts.
func cmdCIRCUITRESET(ctx *Context) error {
	return circuitUpdate(ctx, func(c *circuit, now int64) bool {
		c.state, c.lastChanged = "closed", now
		c.failures, c.successes = 0, 0
		return true
	}, func(*circuit) error { return ctx.WriteOK() })
}

// CIRCUIT.STATE name
func cmdCIRCUITSTATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	c, err := readCircuit(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(c.state)
}

// CIRCUIT.STATS name; the last_* times are Unix seconds.
func cmdCIRCUITSTATS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	c, err := readCircuit(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(key),
		resp.BulkString("state"), resp.BulkString(c.state),
		resp.BulkString("failures"), resp.IntegerValue(c.failures),
		resp.BulkString("successes"), resp.IntegerValue(c.successes),
		resp.BulkString("failure_threshold"), resp.IntegerValue(c.failThreshold),
		resp.BulkString("success_threshold"), resp.IntegerValue(c.okThreshold),
		resp.BulkString("timeout_ms"), resp.IntegerValue(c.timeoutMs),
		resp.BulkString("last_failure"), resp.IntegerValue(c.lastFailure / 1000),
		resp.BulkString("last_success"), resp.IntegerValue(c.lastSuccess / 1000),
		resp.BulkString("last_state_change"), resp.IntegerValue(c.lastChanged / 1000),
	})
}

// CIRCUIT.DELETE name replies 1 when it removed a breaker, 0 otherwise.
func cmdCIRCUITDELETE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	if _, err := readCircuit(ctx, key); err != nil {
		if errors.Is(err, errNoCircuit) {
			ctx.PropagateAs()
			return ctx.WriteInteger(0)
		}
		return ctx.WriteError(err)
	}
	ctx.Store.Delete(key)
	ctx.notify(store.NotifyGeneric, "del", key)
	ctx.propagateDelete(key)
	return ctx.WriteInteger(1)
}

// CIRCUIT.LIST replies name, state for each breaker of the selected
// database, a walk over all its keys like KEYS.
func cmdCIRCUITLIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	var out []*resp.Value
	_ = ctx.Store.ForEachLive(func(key string, e *store.Entry) error { //nolint:errcheck // the callback never fails
		if c, err := circuitOf(e); err == nil {
			out = append(out, resp.BulkString(key), resp.BulkString(c.state))
		}
		return nil
	})
	return ctx.WriteArray(out)
}
