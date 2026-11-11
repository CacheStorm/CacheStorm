package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// The resilience namespaces keep each object in a key, per database,
// persisted and replicated, with changes recorded as Redis commands. The
// commands that wait (BULKHEAD.ACQUIRE, PROMISE.AWAIT, FUTURE.GET) wait
// without the write gate and wake when their key is written or every
// lockPoll, as SEM.ACQUIRE does.

const (
	bulkheadMarker     = "__bulkhead"
	telemetryMarker    = "__telemetry"
	promiseMarker      = "__promise"
	futureMarker       = "__future"
	streamProcMarker   = "__streamproc"
	eventSourcedMarker = "__eventsourcing"
)

// waitForKey waits up to timeout for ready to hold on key, without the write
// gate, and reports whether it does. It does not wait inside MULTI.
func waitForKey(ctx *Context, key string, timeout time.Duration, ready func() bool) bool {
	deadline := time.Now().Add(timeout)
	for !ready() {
		remaining := time.Until(deadline)
		if remaining <= 0 || ctx.InExec {
			return false
		}
		if ctx.Done != nil {
			select {
			case <-ctx.Done:
				return false
			default:
			}
		}
		ctx.waitForStreamKeys([]string{key}, min(remaining, lockPoll), func(string) bool { return ready() })
	}
	return true
}

// --- BULKHEAD ---

// A bulkhead is a hash marked by __bulkhead (its maximum) with current, the
// permits held, and rejected. The waiting count is process-local.

var (
	bulkheadWaitersMu sync.Mutex
	bulkheadWaiters   = map[string]int64{}
)

func addBulkheadWaiter(ctx *Context, key string, d int64) {
	k := strconv.Itoa(ctx.DB) + "\x00" + key
	bulkheadWaitersMu.Lock()
	if bulkheadWaiters[k] += d; bulkheadWaiters[k] <= 0 {
		delete(bulkheadWaiters, k)
	}
	bulkheadWaitersMu.Unlock()
}

func loadBulkhead(ctx *Context, name string) ([]int64, error) {
	v, err := markedInts(ctx, name, bulkheadMarker, "bulkhead", bulkheadMarker, "current", "rejected")
	if err != nil {
		return nil, err
	}
	if v[0] < 1 || v[1] < 0 || v[1] > v[0] {
		return nil, store.ErrWrongType
	}
	return v, nil
}

func cmdBULKHEADCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	maxConcurrent, ok := parseCircuitInt(ctx.ArgString(1), 1)
	if !ok || maxConcurrent > 1<<31-1 {
		return ctx.WriteError(errors.New("ERR max concurrent must be a positive integer"))
	}
	name := ctx.ArgString(0)
	if err := createMarked(ctx, name, bulkheadMarker, bulkheadMarker, i64(maxConcurrent), "current", "0", "rejected", "0"); err != nil {
		return ctx.WriteError(err)
	}
	ctx.Store.KeyNotifier().NotifyKey(name)
	return ctx.WriteOK()
}

// BULKHEAD.ACQUIRE name [timeout_ms] takes a permit, waiting up to
// timeout_ms for one (0 waits up to five minutes); 1 when taken, 0 when
// rejected.
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
	v, err := loadBulkhead(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if v[1] >= v[0] && timeout > 0 && !ctx.InExec {
		addBulkheadWaiter(ctx, name, 1)
		waitForKey(ctx, name, timeout, func() bool {
			w, err := loadBulkhead(ctx, name)
			return err != nil || w[1] < w[0]
		})
		addBulkheadWaiter(ctx, name, -1)
		if v, err = loadBulkhead(ctx, name); err != nil {
			return ctx.WriteError(err)
		}
	}
	if v[1] < v[0] {
		if err := updateRecord(ctx, name, "current", i64(v[1]+1)); err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteInteger(1)
	}
	if err := updateRecord(ctx, name, "rejected", i64(v[2]+1)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(0)
}

func cmdBULKHEADRELEASE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	v, err := loadBulkhead(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if v[1] == 0 {
		return ctx.WriteError(errors.New("ERR bulkhead has no permit to release"))
	}
	if err := updateRecord(ctx, name, "current", i64(v[1]-1)); err != nil {
		return ctx.WriteError(err)
	}
	ctx.Store.KeyNotifier().NotifyKey(name)
	return ctx.WriteOK()
}

func cmdBULKHEADSTATUS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	v, err := loadBulkhead(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	bulkheadWaitersMu.Lock()
	waiting := bulkheadWaiters[strconv.Itoa(ctx.DB)+"\x00"+name]
	bulkheadWaitersMu.Unlock()
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(name),
		resp.BulkString("current"), resp.IntegerValue(v[1]),
		resp.BulkString("max"), resp.IntegerValue(v[0]),
		resp.BulkString("waiting"), resp.IntegerValue(waiting),
		resp.BulkString("rejected"), resp.IntegerValue(v[2]),
	})
}

func cmdBULKHEADDELETE(ctx *Context) error {
	err := deleteMarked(ctx, bulkheadMarker)
	if ctx.ArgCount() == 1 {
		ctx.Store.KeyNotifier().NotifyKey(ctx.ArgString(0))
	}
	return err
}

// --- TELEMETRY ---

// A telemetry series is a log (see readLog) marked by __telemetry whose
// entries are JSON points.

type telemetryPoint struct {
	Timestamp int64             `json:"timestamp"`
	Value     float64           `json:"value"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// TELEMETRY.RECORD name timestamp value [tag value ...] appends a point.
func cmdTELEMETRYRECORD(ctx *Context) error {
	if ctx.ArgCount() < 3 || (ctx.ArgCount()-3)%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ts, err := parseInt64Arg(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	v, err := parseFiniteFloat(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	p := telemetryPoint{Timestamp: ts, Value: v}
	for i := 3; i+1 < ctx.ArgCount(); i += 2 {
		if p.Tags == nil {
			p.Tags = map[string]string{}
		}
		p.Tags[ctx.ArgString(i)] = ctx.ArgString(i + 1)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return ctx.WriteError(err)
	}
	name := ctx.ArgString(0)
	n, _, _, err := logHead(ctx, name, telemetryMarker)
	if err == nil {
		err = appendLog(ctx, name, telemetryMarker, n, string(raw))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdTELEMETRYQUERY(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	raw, _, err := readLog(ctx, ctx.ArgString(0), telemetryMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	out := make([]*resp.Value, len(raw))
	for i, r := range raw {
		var p telemetryPoint
		if json.Unmarshal([]byte(r), &p) != nil {
			return ctx.WriteError(store.ErrWrongType)
		}
		out[i] = resp.ArrayValue([]*resp.Value{
			resp.BulkString("timestamp"), resp.IntegerValue(p.Timestamp),
			resp.BulkString("value"), resp.BulkString(fmt.Sprintf("%.6f", p.Value)),
		})
	}
	return ctx.WriteArray(out)
}

// PROFILEX.LIST replies no profiles: no command creates one.
func cmdPROFILEXLIST(ctx *Context) error {
	return ctx.WriteArray([]*resp.Value{})
}

// --- PROMISE and FUTURE ---

// A promise is a hash named by its generated id, marked by __promise (its
// status: pending, resolved or rejected) with value, error and created_at.
// A future is the same with __future (pending, completed or cancelled).

// PROMISE.CREATE replies the new promise's id; arguments are ignored.
func cmdPROMISECREATE(ctx *Context) error {
	id := newObjectID(ctx)
	if err := replaceRecord(ctx, id, promiseMarker, "pending", "created_at", i64(ctx.nowMS())); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(id)
}

func settlePromise(ctx *Context, status string) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	r, err := readMarked(ctx, id, promiseMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil || r[promiseMarker] != "pending" {
		return ctx.WriteError(errors.New("ERR promise not found or not pending"))
	}
	field := "value"
	if status == "rejected" {
		field = "error"
	}
	if err := updateRecord(ctx, id, promiseMarker, status, field, ctx.ArgString(1)); err != nil {
		return ctx.WriteError(err)
	}
	ctx.Store.KeyNotifier().NotifyKey(id)
	return ctx.WriteOK()
}

func cmdPROMISERESOLVE(ctx *Context) error { return settlePromise(ctx, "resolved") }
func cmdPROMISEREJECT(ctx *Context) error  { return settlePromise(ctx, "rejected") }

func cmdPROMISESTATUS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	r, err := loadMarked(ctx, id, promiseMarker, "promise")
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(id),
		resp.BulkString("status"), resp.BulkString(r[promiseMarker]),
	})
}

// awaitSettled waits up to the timeout argument at index 1 for the object
// at id to leave pending, and returns its fields; nil when it did not.
func awaitSettled(ctx *Context, id, marker, what string, timeout time.Duration) (map[string]string, error) {
	r, err := loadMarked(ctx, id, marker, what)
	if err != nil {
		return nil, err
	}
	if r[marker] == "pending" && timeout > 0 {
		waitForKey(ctx, id, timeout, func() bool {
			w, err := readMarked(ctx, id, marker)
			return err != nil || w == nil || w[marker] != "pending"
		})
		if r, err = loadMarked(ctx, id, marker, what); err != nil {
			return nil, err
		}
	}
	if r[marker] == "pending" {
		return nil, nil
	}
	return r, nil
}

// PROMISE.AWAIT id timeout_ms replies the value once resolved (an error
// once rejected), or null when it is still pending at the timeout.
func cmdPROMISEAWAIT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	timeout, err := parseResilienceTimeout(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	r, err := awaitSettled(ctx, ctx.ArgString(0), promiseMarker, "promise", timeout)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		return ctx.WriteNull()
	}
	if r[promiseMarker] == "resolved" {
		return ctx.WriteBulkString(r["value"])
	}
	return ctx.WriteError(errors.New("ERR promise rejected: " + r["error"]))
}

// FUTURE.CREATE replies the new future's id; arguments are ignored.
func cmdFUTURECREATE(ctx *Context) error {
	id := newObjectID(ctx)
	if err := replaceRecord(ctx, id, futureMarker, "pending"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(id)
}

func finishFuture(ctx *Context, id, status, value string) error {
	r, err := loadMarked(ctx, id, futureMarker, "future")
	if err != nil {
		return ctx.WriteError(err)
	}
	if r[futureMarker] != "pending" {
		return ctx.WriteError(fmt.Errorf("ERR future is already %s", r[futureMarker]))
	}
	if err := updateRecord(ctx, id, futureMarker, status, "value", value); err != nil {
		return ctx.WriteError(err)
	}
	ctx.Store.KeyNotifier().NotifyKey(id)
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

// FUTURE.GET id [timeout_ms] replies the value once completed (an error
// once cancelled), or null while pending; with a timeout it waits for it.
func cmdFUTUREGET(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	var timeout time.Duration
	if ctx.ArgCount() == 2 {
		var err error
		if timeout, err = parseResilienceTimeout(ctx.ArgString(1)); err != nil {
			return ctx.WriteError(err)
		}
	}
	r, err := awaitSettled(ctx, ctx.ArgString(0), futureMarker, "future", timeout)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		return ctx.WriteNull()
	}
	if r[futureMarker] == "completed" {
		return ctx.WriteBulkString(r["value"])
	}
	return ctx.WriteError(errors.New("ERR future was cancelled"))
}

// --- STREAMPROC ---

// A STREAMPROC stream is a queue in a hash marked by __streamproc: the
// values are the fields m:<seq> from head to tail.

func loadStreamProc(ctx *Context, name string) (*mailbox, error) {
	h, err := readMarkedHash(ctx, name, streamProcMarker)
	if err != nil || h == nil {
		return nil, err
	}
	h.RLock()
	head, _ := h.Get("head")
	tail, _ := h.Get("tail")
	h.RUnlock()
	m := &mailbox{}
	var e1, e2 error
	m.head, e1 = strconv.ParseInt(string(head), 10, 64)
	m.tail, e2 = strconv.ParseInt(string(tail), 10, 64)
	if e1 != nil || e2 != nil || m.head > m.tail {
		return nil, store.ErrWrongType
	}
	return m, nil
}

func streamProcValue(ctx *Context, name string, seq int64) (string, error) {
	h, err := readMarkedHash(ctx, name, streamProcMarker)
	if err != nil || h == nil {
		return "", errWrongTypeOr(true, err)
	}
	h.RLock()
	v, ok := h.Get("m:" + strconv.FormatInt(seq, 10))
	h.RUnlock()
	if !ok {
		return "", store.ErrWrongType
	}
	return string(v), nil
}

func cmdSTREAMPROCCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), streamProcMarker, streamProcMarker, "1", "head", "0", "tail", "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdSTREAMPROCPUSH(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	m, err := loadStreamProc(ctx, name)
	if err == nil && m == nil {
		err = errors.New("ERR stream not found")
	}
	if err == nil {
		err = updateRecord(ctx, name, "m:"+i64(m.tail), ctx.ArgString(1), "tail", i64(m.tail+1))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdSTREAMPROCPOP(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	m, err := loadStreamProc(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if m == nil || m.head == m.tail {
		ctx.PropagateAs()
		return ctx.WriteNull()
	}
	v, err := streamProcValue(ctx, name, m.head)
	if err != nil {
		return ctx.WriteError(err)
	}
	deleteMarkedFields(ctx, name, streamProcMarker, "m:"+i64(m.head))
	hdel, _ := ctx.Propagation()
	if err := updateRecord(ctx, name, "head", i64(m.head+1)); err != nil {
		return ctx.WriteError(err)
	}
	hset, _ := ctx.Propagation()
	ctx.PropagateAs(append(hdel, hset...)...)
	return ctx.WriteBulkString(v)
}

func cmdSTREAMPROCPEEK(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	m, err := loadStreamProc(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if m == nil || m.head == m.tail {
		return ctx.WriteNull()
	}
	v, err := streamProcValue(ctx, name, m.head)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(v)
}

func cmdSTREAMPROCDELETE(ctx *Context) error { return deleteMarked(ctx, streamProcMarker) }

// --- EVENTSOURCING ---

// An event stream is a log (see readLog) marked by __eventsourcing whose
// entry i is version i+1, a JSON object.

type sourcedEvent struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Data      string `json:"data"`
	Timestamp int64  `json:"timestamp"`
}

// EVENTSOURCING.APPEND stream type data appends an event and replies its
// version.
func cmdEVENTSOURCINGAPPEND(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	raw, err := json.Marshal(sourcedEvent{ID: generateUUID(), Type: ctx.ArgString(1), Data: ctx.ArgString(2), Timestamp: ctx.nowMS()})
	if err != nil {
		return ctx.WriteError(err)
	}
	n, _, _, err := logHead(ctx, name, eventSourcedMarker)
	if err == nil {
		err = appendLog(ctx, name, eventSourcedMarker, n, string(raw))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(n + 1))
}

func readSourcedEvents(ctx *Context, name string) ([]sourcedEvent, error) {
	raw, _, err := readLog(ctx, name, eventSourcedMarker)
	if err != nil {
		return nil, err
	}
	out := make([]sourcedEvent, len(raw))
	for i, r := range raw {
		if json.Unmarshal([]byte(r), &out[i]) != nil {
			return nil, store.ErrWrongType
		}
	}
	return out, nil
}

// EVENTSOURCING.REPLAY stream [from_version] replies the events from
// from_version on.
func cmdEVENTSOURCINGREPLAY(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	from := int64(0)
	if ctx.ArgCount() == 2 {
		var err error
		if from, err = parseInt64Arg(ctx.ArgString(1)); err != nil {
			return ctx.WriteError(err)
		}
	}
	events, err := readSourcedEvents(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	var out []*resp.Value
	for i, e := range events {
		if v := int64(i + 1); v >= from {
			out = append(out, resp.ArrayValue([]*resp.Value{
				resp.BulkString("version"), resp.IntegerValue(v),
				resp.BulkString("type"), resp.BulkString(e.Type),
				resp.BulkString("data"), resp.BulkString(e.Data),
			}))
		}
	}
	return ctx.WriteArray(out)
}

func cmdEVENTSOURCINGGET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, err := parseInt64Arg(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	events, err := readSourcedEvents(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if v < 1 || v > int64(len(events)) {
		return ctx.WriteNull()
	}
	e := events[v-1]
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("version"), resp.IntegerValue(v),
		resp.BulkString("type"), resp.BulkString(e.Type),
		resp.BulkString("data"), resp.BulkString(e.Data),
		resp.BulkString("timestamp"), resp.IntegerValue(e.Timestamp),
	})
}

// --- BACKPRESSURE, THROTTLEX, DEBOUNCEX ---

var (
	backpressureKind = recordKind{name: "backpressure", fields: []string{"high_water", "low_water", "current", "paused"}}
	throttleXKind    = recordKind{name: "throttle", fields: []string{"rate_ms", "last_sent"}}
	debounceXKind    = recordKind{name: "debounce", fields: []string{"delay", "last_call", "pending", "arg"}}
)

const coalesceMarker = "__coalesce"

// BACKPRESSURE.CREATE name high_water low_water
func cmdBACKPRESSURECREATE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	high, err1 := parseInt64Arg(ctx.ArgString(1))
	low, err2 := parseInt64Arg(ctx.ArgString(2))
	if err1 != nil || err2 != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	return createRecord(ctx, backpressureKind, ctx.ArgString(0), "high_water", i64(high), "low_water", i64(low), "current", "0", "paused", "0")
}

// BACKPRESSURE.CHECK name delta adds delta to the level: it pauses at the
// high water mark and resumes at the low one.
func cmdBACKPRESSURECHECK(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	delta, err := parseInt64Arg(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	name := ctx.ArgString(0)
	r, err := backpressureKind.load(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	high, low, cur, paused := r.int("high_water"), r.int("low_water"), r.int("current"), r.vals["paused"] == "1"
	if r.bad {
		return ctx.WriteError(store.ErrWrongType)
	}
	if (delta > 0 && cur > (1<<63-1)-delta) || (delta < 0 && cur < -(1<<63-1)-1-delta) {
		return ctx.WriteError(errors.New("ERR level would overflow"))
	}
	cur += delta
	if cur >= high {
		paused = true
	}
	if cur <= low {
		paused = false
	}
	if err := updateRecord(ctx, name, "current", i64(cur), "paused", i64(boolInt(paused))); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("current"), resp.IntegerValue(cur),
		resp.BulkString("paused"), resp.BulkString(strconv.FormatBool(paused)),
	})
}

func cmdBACKPRESSURESTATUS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	r, err := backpressureKind.load(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	cur, high, low := r.int("current"), r.int("high_water"), r.int("low_water")
	if r.bad {
		return ctx.WriteError(store.ErrWrongType)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(name),
		resp.BulkString("current"), resp.IntegerValue(cur),
		resp.BulkString("high_water"), resp.IntegerValue(high),
		resp.BulkString("low_water"), resp.IntegerValue(low),
		resp.BulkString("paused"), resp.BulkString(strconv.FormatBool(r.vals["paused"] == "1")),
	})
}

// THROTTLEX.CREATE name interval_ms
func cmdTHROTTLEXCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	rate, err := parseNonNegative(ctx.ArgString(1), "interval")
	if err != nil {
		return ctx.WriteError(err)
	}
	return createRecord(ctx, throttleXKind, ctx.ArgString(0), "rate_ms", i64(rate), "last_sent", "0")
}

// THROTTLEX.CHECK name is 1, recording the send, when the interval has
// passed since the last one.
func cmdTHROTTLEXCHECK(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	r, err := throttleXKind.load(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	rate, last := r.int("rate_ms"), r.int("last_sent")
	if r.bad {
		return ctx.WriteError(store.ErrWrongType)
	}
	now := ctx.nowMS()
	if now-last < rate {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	if err := updateRecord(ctx, name, "last_sent", i64(now)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(1)
}

func cmdTHROTTLEXSTATUS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	r, err := throttleXKind.load(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	rate, last := r.int("rate_ms"), r.int("last_sent")
	if r.bad {
		return ctx.WriteError(store.ErrWrongType)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(name),
		resp.BulkString("rate_ms"), resp.IntegerValue(rate),
		resp.BulkString("last_sent"), resp.IntegerValue(last),
	})
}

// DEBOUNCEX.CREATE name delay_ms
func cmdDEBOUNCEXCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	delay, ok := parseCircuitInt(ctx.ArgString(1), 0)
	if !ok {
		return ctx.WriteError(errors.New("ERR delay must be a non-negative integer"))
	}
	return createRecord(ctx, debounceXKind, ctx.ArgString(0), "delay", i64(delay), "last_call", "0", "pending", "0", "arg", "")
}

// DEBOUNCEX.CALL name arg records a call; the last one's arg fires.
func cmdDEBOUNCEXCALL(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	if _, err := debounceXKind.load(ctx, name); err != nil {
		return ctx.WriteError(err)
	}
	if err := updateRecord(ctx, name, "last_call", i64(ctx.nowMS()), "pending", "1", "arg", ctx.ArgString(1)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdDEBOUNCEXCANCEL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	if _, err := debounceXKind.load(ctx, name); err != nil {
		return ctx.WriteError(err)
	}
	if err := updateRecord(ctx, name, "pending", "0", "arg", ""); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// DEBOUNCEX.FLUSH name [FORCE] replies the pending arg once the delay has
// passed since the last call (at once with FORCE), clearing it; else null.
func cmdDEBOUNCEXFLUSH(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	force := false
	if ctx.ArgCount() == 2 {
		if !strings.EqualFold(ctx.ArgString(1), "FORCE") {
			return ctx.WriteError(ErrSyntaxError)
		}
		force = true
	}
	name := ctx.ArgString(0)
	r, err := debounceXKind.load(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	delay, last := r.int("delay"), r.int("last_call")
	if r.bad {
		return ctx.WriteError(store.ErrWrongType)
	}
	if r.vals["pending"] != "1" || (!force && ctx.nowMS()-last < delay) {
		ctx.PropagateAs()
		return ctx.WriteNull()
	}
	if err := updateRecord(ctx, name, "pending", "0", "arg", ""); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(r.vals["arg"])
}

// --- COALESCE ---

// A coalescer is a log (see readLog) marked by __coalesce; GET replies its
// first value.

func cmdCOALESCECREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), coalesceMarker, coalesceMarker, "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdCOALESCEADD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	n, _, ok, err := logHead(ctx, name, coalesceMarker)
	if err == nil && !ok {
		err = errors.New("ERR coalesce not found")
	}
	if err == nil {
		err = appendLog(ctx, name, coalesceMarker, n, ctx.ArgString(1))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(n + 1))
}

func cmdCOALESCEGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	n, _, ok, err := logHead(ctx, name, coalesceMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if !ok || n == 0 {
		return ctx.WriteNull()
	}
	v, err := logEntry(ctx, name, coalesceMarker, 0)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(v)
}

func cmdCOALESCECLEAR(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	if _, _, ok, err := logHead(ctx, name, coalesceMarker); err != nil || !ok {
		return ctx.WriteError(errWrongTypeOr(err != nil, errors.New("ERR coalesce not found")))
	}
	if err := replaceRecord(ctx, name, coalesceMarker, "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// --- AGGREGATOR ---

// An aggregator is a hash marked by __aggregator (its type: sum, avg, min,
// max or count) with the count, sum, min and max of the values added.

const (
	aggregatorMarker = "__aggregator"
	windowXMarker    = "__windowx"
	joinXMarker      = "__joinx"
	shuffleMarker    = "__shuffle"
	partitionXMarker = "__partitionx"
	maxJoinXEntries  = 100000
)

func validAggregatorType(t string) bool {
	switch t {
	case "sum", "avg", "min", "max", "count":
		return true
	}
	return false
}

func cmdAGGREGATORCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if !validAggregatorType(ctx.ArgString(1)) {
		return ctx.WriteError(errors.New("ERR unknown aggregator type"))
	}
	if err := createMarked(ctx, ctx.ArgString(0), aggregatorMarker, aggregatorMarker, ctx.ArgString(1), "count", "0", "sum", "0", "min", "0", "max", "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

type aggregatorState struct {
	kind             string
	count            int64
	sum, least, most float64
}

func loadAggregator(ctx *Context, name string) (*aggregatorState, error) {
	r, err := readMarked(ctx, name, aggregatorMarker)
	if err != nil || r == nil {
		return nil, err
	}
	a := &aggregatorState{kind: r[aggregatorMarker]}
	var e [4]error
	a.count, e[0] = strconv.ParseInt(r["count"], 10, 64)
	a.sum, e[1] = strconv.ParseFloat(r["sum"], 64)
	a.least, e[2] = strconv.ParseFloat(r["min"], 64)
	a.most, e[3] = strconv.ParseFloat(r["max"], 64)
	for _, x := range e {
		if x != nil {
			return nil, store.ErrWrongType
		}
	}
	return a, nil
}

func cmdAGGREGATORADD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, err := parseFiniteFloat(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	name := ctx.ArgString(0)
	a, err := loadAggregator(ctx, name)
	if err == nil && a == nil {
		err = errors.New("ERR aggregator not found")
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	if a.count == 0 {
		a.least, a.most = v, v
	}
	a.least, a.most = min(a.least, v), max(a.most, v)
	if err := updateRecord(ctx, name, "count", i64(a.count+1), "sum", f64(a.sum+v), "min", f64(a.least), "max", f64(a.most)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdAGGREGATORGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	a, err := loadAggregator(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if a == nil || a.count == 0 {
		return ctx.WriteBulkString("0")
	}
	var v float64
	switch a.kind {
	case "sum":
		v = a.sum
	case "avg":
		v = a.sum / float64(a.count)
	case "min":
		v = a.least
	case "max":
		v = a.most
	case "count":
		return ctx.WriteInteger(a.count)
	default:
		return ctx.WriteError(errors.New("ERR unknown aggregator type"))
	}
	return ctx.WriteBulkString(fmt.Sprintf("%.6f", v))
}

func cmdAGGREGATORRESET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	a, err := loadAggregator(ctx, name)
	if err == nil && a == nil {
		err = errors.New("ERR aggregator not found")
	}
	if err == nil {
		err = updateRecord(ctx, name, "count", "0", "sum", "0", "min", "0", "max", "0")
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// --- WINDOWX ---

// A WINDOWX window is a ring (see loadRing2) of the last size numbers.

func cmdWINDOWXCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	size, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	if err := checkSlots("size", size); err != nil {
		return ctx.WriteError(err)
	}
	if err := ringCreate(ctx, windowXMarker, size); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdWINDOWXADD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, err := parseFiniteFloat(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	name := ctx.ArgString(0)
	r, err := loadRing2(ctx, name, windowXMarker, "window")
	if err == nil {
		err = r.push(ctx, name, f64(v))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func windowXValues(ctx *Context) ([]float64, error) {
	r, err := loadRing2(ctx, ctx.ArgString(0), windowXMarker, "window")
	if err != nil {
		if err == store.ErrWrongType {
			return nil, err
		}
		return nil, nil
	}
	raw := r.values()
	out := make([]float64, len(raw))
	for i, s := range raw {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, store.ErrWrongType
		}
		out[i] = v
	}
	return out, nil
}

func cmdWINDOWXGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	data, err := windowXValues(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	out := make([]*resp.Value, len(data))
	for i, v := range data {
		out[i] = resp.BulkString(fmt.Sprintf("%.6f", v))
	}
	return ctx.WriteArray(out)
}

func cmdWINDOWXAGGREGATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	agg := ctx.ArgString(1)
	if agg != "sum" && agg != "avg" {
		return ctx.WriteError(errors.New("ERR unknown aggregation type"))
	}
	data, err := windowXValues(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	if len(data) == 0 {
		return ctx.WriteBulkString("0")
	}
	sum := aggSum(data)
	if agg == "avg" {
		sum /= float64(len(data))
	}
	return ctx.WriteBulkString(fmt.Sprintf("%.6f", sum))
}

// --- JOINX ---

// A join is a hash marked by __joinx (its entry count) with each key's left
// and right values as JSON arrays in l:<key> and r:<key>.

func cmdJOINXCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if ctx.ArgCount() > 1 {
		return ctx.WriteError(errors.New("ERR JOINX.CREATE does not support type or condition arguments"))
	}
	if err := createMarked(ctx, ctx.ArgString(0), joinXMarker, joinXMarker, "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func joinSide(h *store.HashValue, field string) ([]string, error) {
	raw, ok := h.Get(field)
	if !ok {
		return nil, nil
	}
	var out []string
	if json.Unmarshal(raw, &out) != nil {
		return nil, store.ErrWrongType
	}
	return out, nil
}

// JOINX.ADD name LEFT|RIGHT key value
func cmdJOINXADD(ctx *Context) error {
	if ctx.ArgCount() != 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	side := strings.ToUpper(ctx.ArgString(1))
	if side != "LEFT" && side != "RIGHT" {
		return ctx.WriteError(errors.New("ERR side must be LEFT or RIGHT"))
	}
	h, err := readMarkedHash(ctx, name, joinXMarker)
	if err == nil && h == nil {
		err = errors.New("ERR join not found")
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	field := "l:" + ctx.ArgString(2)
	if side == "RIGHT" {
		field = "r:" + ctx.ArgString(2)
	}
	h.RLock()
	rawN, _ := h.Get(joinXMarker)
	values, err := joinSide(h, field)
	h.RUnlock()
	n, nerr := strconv.Atoi(string(rawN))
	if err != nil || nerr != nil {
		return ctx.WriteError(store.ErrWrongType)
	}
	if n >= maxJoinXEntries {
		return ctx.WriteError(errors.New("ERR join is full"))
	}
	raw, err := json.Marshal(append(values, ctx.ArgString(3)))
	if err == nil {
		err = updateRecord(ctx, name, field, string(raw), joinXMarker, strconv.Itoa(n+1))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// JOINX.GET name [key] replies key, left, right for each matching pair, by
// key.
func cmdJOINXGET(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := readMarked(ctx, ctx.ArgString(0), joinXMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		return ctx.WriteArray([]*resp.Value{})
	}
	var keys []string
	if ctx.ArgCount() == 2 {
		keys = []string{ctx.ArgString(1)}
	} else {
		seen := map[string]bool{}
		for f := range r {
			if k, ok := strings.CutPrefix(f, "l:"); ok && !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			} else if k, ok := strings.CutPrefix(f, "r:"); ok && !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
		sortStrings(keys)
	}
	side := func(f string) ([]string, error) {
		raw, ok := r[f]
		if !ok {
			return nil, nil
		}
		var out []string
		if json.Unmarshal([]byte(raw), &out) != nil {
			return nil, store.ErrWrongType
		}
		return out, nil
	}
	var out []*resp.Value
	for _, k := range keys {
		left, err1 := side("l:" + k)
		right, err2 := side("r:" + k)
		if err1 != nil || err2 != nil {
			return ctx.WriteError(store.ErrWrongType)
		}
		if len(out)+len(left)*len(right) > maxJoinXEntries {
			return ctx.WriteError(errors.New("ERR join result too large"))
		}
		for _, l := range left {
			for _, rv := range right {
				out = append(out, resp.ArrayValue([]*resp.Value{
					resp.BulkString("key"), resp.BulkString(k),
					resp.BulkString("left"), resp.BulkString(l),
					resp.BulkString("right"), resp.BulkString(rv),
				}))
			}
		}
	}
	return ctx.WriteArray(out)
}

func cmdJOINXDELETE(ctx *Context) error { return deleteMarked(ctx, joinXMarker) }

// --- SHUFFLE ---

// A shuffle is a hash marked by __shuffle (the next sequence number) with
// each value as e:<seq>; GET removes a random one, recorded as that HDEL.

func cmdSHUFFLECREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), shuffleMarker, shuffleMarker, "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdSHUFFLEADD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	v, err := markedInts(ctx, name, shuffleMarker, "shuffle", shuffleMarker)
	if err == nil {
		err = updateRecord(ctx, name, "e:"+i64(v[0]), ctx.ArgString(1), shuffleMarker, i64(v[0]+1))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdSHUFFLEGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	r, err := readMarked(ctx, name, shuffleMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	var fields []string
	for f := range r {
		if strings.HasPrefix(f, "e:") {
			fields = append(fields, f)
		}
	}
	if len(fields) == 0 {
		ctx.PropagateAs()
		return ctx.WriteNull()
	}
	sortStrings(fields)
	f := fields[rand.Intn(len(fields))]
	deleteMarkedFields(ctx, name, shuffleMarker, f)
	return ctx.WriteBulkString(r[f])
}

// --- PARTITIONX ---

func cmdPARTITIONXCREATE(ctx *Context) error { return partitionCreate(ctx, partitionXMarker) }
func cmdPARTITIONXADD(ctx *Context) error    { return partitionAdd(ctx, partitionXMarker) }
func cmdPARTITIONXGET(ctx *Context) error    { return partitionGet(ctx, partitionXMarker) }

func sortStrings(s []string) { sort.Strings(s) }
