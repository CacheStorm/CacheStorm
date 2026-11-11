package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// The namespaces of more_commands.go keep each object in a key, per
// database, persisted and replicated, with changes recorded as Redis
// commands: SLIDING is a RATELIMITER ring, the others hashes marked by a
// field naming their kind.

const (
	idempotencyMarker = "__idempotency"
	experimentMarker  = "__experiment"
	rolloutMarker     = "__rollout"
	alertMarker       = "__alert"
	counterX1Marker   = "__counterx"
	maxAlertHistory   = 1000
)

// --- SLIDING ---

// SLIDING.CREATE name limit window_ms makes a sliding window limiter: a
// RATELIMITER, which RATELIMITER.STATUS also reads.
func cmdSLIDINGCREATE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	limit, err1 := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	window, err2 := strconv.ParseInt(ctx.ArgString(2), 10, 64)
	if err1 != nil || limit < 1 || limit > maxRateLimit {
		return ctx.WriteError(fmt.Errorf("ERR limit must be an integer from 1 to %d", maxRateLimit))
	}
	if err2 != nil || window < 1 {
		return ctx.WriteError(errors.New("ERR window must be a positive integer"))
	}
	key := ctx.ArgString(0)
	if _, err := readRateLimiter(ctx, key); err != nil && !errors.Is(err, errNoRateLimiter) {
		return ctx.WriteError(err)
	}
	if err := resetRateLimiter(ctx, key, limit, window, "sliding"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func loadSlidingLimiter(ctx *Context) (*rateLimiter, error) {
	rl, err := readRateLimiter(ctx, ctx.ArgString(0))
	if errors.Is(err, errNoRateLimiter) {
		err = errors.New("ERR sliding window not found")
	}
	return rl, err
}

// SLIDING.CHECK name replies [1, remaining, window_ms] when the request is
// allowed and [0, 0, ms until one is] when it is not.
func cmdSLIDINGCHECK(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	rl, err := loadSlidingLimiter(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	now := ctx.nowMS()
	ok, wait, err := rl.take(ctx, ctx.ArgString(0), now)
	if err != nil {
		return ctx.WriteError(err)
	}
	if !ok {
		ctx.PropagateAs()
		return ctx.WriteArray([]*resp.Value{resp.IntegerValue(0), resp.IntegerValue(0), resp.IntegerValue(wait)})
	}
	return ctx.WriteArray([]*resp.Value{resp.IntegerValue(1), resp.IntegerValue(max(rl.limit-rl.inWindow(now), 0)), resp.IntegerValue(rl.window)})
}

func cmdSLIDINGRESET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	rl, err := loadSlidingLimiter(ctx)
	if err == nil {
		err = resetRateLimiter(ctx, ctx.ArgString(0), rl.limit, rl.window, rl.strategy)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdSLIDINGDELETE(ctx *Context) error { return cmdRATELIMITERDELETE(ctx) }

func cmdSLIDINGSTATS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	rl, err := loadSlidingLimiter(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(ctx.ArgString(0)),
		resp.BulkString("limit"), resp.IntegerValue(rl.limit),
		resp.BulkString("window_ms"), resp.IntegerValue(rl.window),
		resp.BulkString("current"), resp.IntegerValue(rl.inWindow(ctx.nowMS())),
	})
}

// --- BUCKETX ---

// A BUCKETX bucket is a hash of capacity, tokens, refill (tokens added per
// interval), interval (ms) and last_check.

var bucketXKind = recordKind{name: "bucket", fields: []string{"capacity", "tokens", "refill", "interval", "last_check"}}

type bucketX struct{ capacity, tokens, refill, interval, last int64 }

func loadBucketX(ctx *Context) (*bucketX, error) {
	r, err := bucketXKind.load(ctx, ctx.ArgString(0))
	if err != nil {
		return nil, err
	}
	b := &bucketX{r.int("capacity"), r.int("tokens"), r.int("refill"), r.int("interval"), r.int("last_check")}
	if r.bad || b.interval < 1 {
		return nil, store.ErrWrongType
	}
	return b, nil
}

// refillAt adds the whole intervals' refills due by now.
func (b *bucketX) refillAt(now int64) {
	if elapsed := now - b.last; elapsed >= b.interval {
		periods := elapsed / b.interval
		if b.refill > 0 && periods > (b.capacity-b.tokens)/b.refill {
			b.tokens = b.capacity
		} else {
			b.tokens = min(b.tokens+periods*b.refill, b.capacity)
		}
		b.last = now
	}
}

func (b *bucketX) save(ctx *Context) error {
	return updateRecord(ctx, ctx.ArgString(0), "tokens", i64(b.tokens), "last_check", i64(b.last))
}

// BUCKETX.CREATE name capacity refill interval_ms makes a full bucket.
func cmdBUCKETXCREATE(ctx *Context) error {
	if ctx.ArgCount() != 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	capacity, err := parseNonNegative(ctx.ArgString(1), "capacity")
	if err != nil {
		return ctx.WriteError(err)
	}
	refill, err := parseNonNegative(ctx.ArgString(2), "refill")
	if err != nil {
		return ctx.WriteError(err)
	}
	interval, err := strconv.ParseInt(ctx.ArgString(3), 10, 64)
	if err != nil || interval <= 0 {
		return ctx.WriteError(errors.New("ERR interval must be positive"))
	}
	return createRecord(ctx, bucketXKind, ctx.ArgString(0), "capacity", i64(capacity), "tokens", i64(capacity),
		"refill", i64(refill), "interval", i64(interval), "last_check", i64(ctx.nowMS()))
}

// BUCKETX.TAKE name n replies [1, tokens left] when it took n tokens and
// [0, tokens] when there were too few.
func cmdBUCKETXTAKE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	n, err := parseNonNegative(ctx.ArgString(1), "tokens")
	if err != nil {
		return ctx.WriteError(err)
	}
	b, err := loadBucketX(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	b.refillAt(ctx.nowMS())
	took := b.tokens >= n
	if took {
		b.tokens -= n
	}
	if err := b.save(ctx); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{resp.IntegerValue(boolInt(took)), resp.IntegerValue(b.tokens)})
}

// BUCKETX.RETURN name n gives back n tokens, up to the capacity, and
// replies the tokens.
func cmdBUCKETXRETURN(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	n, err := parseNonNegative(ctx.ArgString(1), "tokens")
	if err != nil {
		return ctx.WriteError(err)
	}
	b, err := loadBucketX(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	b.tokens = min(b.tokens+min(n, b.capacity), b.capacity)
	if err := b.save(ctx); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(b.tokens)
}

func cmdBUCKETXREFILL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	b, err := loadBucketX(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	b.tokens, b.last = b.capacity, ctx.nowMS()
	if err := b.save(ctx); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdBUCKETXDELETE(ctx *Context) error { return deleteRecord(ctx, bucketXKind) }

// --- IDEMPOTENCY ---

// An idempotency key is a hash marked by __idempotency (the response) with
// status and created_at, expiring with its TTL.

func cmdIDEMPOTENCYSET(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	ttl, err := parseTokenTTL(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	r, err := readMarked(ctx, key, idempotencyMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r != nil {
		return ctx.WriteError(errors.New("ERR key already exists"))
	}
	now := ctx.nowMS()
	pairs := []string{idempotencyMarker, ctx.ArgString(1), "status", "pending", "created_at", i64(now)}
	h, _ := store.NewLoadedHash(pairs)
	if err := ctx.Store.Set(key, h, store.SetOptions{ExpireAtMS: now + ttl}); err != nil {
		return ctx.WriteError(err)
	}
	ctx.notify(store.NotifyHash, "hset", key)
	ctx.PropagateAs(
		ExecutedCommand{Name: "HSET", Args: pairArgs(key, pairs), DB: ctx.DB},
		ExecutedCommand{Name: "PEXPIREAT", Args: [][]byte{[]byte(key), strconv.AppendInt(nil, now+ttl, 10)}, DB: ctx.DB},
	)
	return ctx.WriteOK()
}

func cmdIDEMPOTENCYGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := readMarked(ctx, ctx.ArgString(0), idempotencyMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(r[idempotencyMarker])
}

func cmdIDEMPOTENCYCHECK(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := readMarked(ctx, ctx.ArgString(0), idempotencyMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(boolInt(r != nil))
}

func cmdIDEMPOTENCYDELETE(ctx *Context) error { return deleteMarked(ctx, idempotencyMarker) }
func cmdIDEMPOTENCYLIST(ctx *Context) error   { return listMarked(ctx, idempotencyMarker) }

// --- EXPERIMENT ---

// An experiment is a hash marked by __experiment (its variants as a JSON
// array) with a:<user> holding each user's variant and, per event and user,
// a field made by orsetField('e', event, user) holding the count.

type experiment struct {
	variants    []string
	assignments map[string]string
	events      map[string]map[string]int64
}

func loadExperiment(ctx *Context, name string) (*experiment, error) {
	r, err := loadMarked(ctx, name, experimentMarker, "experiment")
	if err != nil {
		return nil, err
	}
	e := &experiment{assignments: map[string]string{}, events: map[string]map[string]int64{}}
	if json.Unmarshal([]byte(r[experimentMarker]), &e.variants) != nil || len(e.variants) == 0 {
		return nil, store.ErrWrongType
	}
	for f, v := range r {
		switch {
		case f == experimentMarker:
		case strings.HasPrefix(f, "a:"):
			e.assignments[f[2:]] = v
		default:
			kind, event, user, ok := unpackPair(f)
			n, err := strconv.ParseInt(v, 10, 64)
			if !ok || kind != 'e' || err != nil {
				return nil, store.ErrWrongType
			}
			if e.events[event] == nil {
				e.events[event] = map[string]int64{}
			}
			e.events[event][user] = n
		}
	}
	return e, nil
}

// EXPERIMENT.CREATE name variant [variant ...] makes an experiment,
// replacing one at name.
func cmdEXPERIMENTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	variants := make([]string, 0, ctx.ArgCount()-1)
	seen := map[string]bool{}
	for i := 1; i < ctx.ArgCount(); i++ {
		v := ctx.ArgString(i)
		if seen[v] {
			return ctx.WriteError(fmt.Errorf("ERR duplicate variant '%s'", v))
		}
		seen[v] = true
		variants = append(variants, v)
	}
	raw, err := json.Marshal(variants)
	if err == nil {
		err = createMarked(ctx, ctx.ArgString(0), experimentMarker, experimentMarker, string(raw))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdEXPERIMENTDELETE(ctx *Context) error { return deleteMarked(ctx, experimentMarker) }

func hashString(s string) int {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	if h < 0 {
		h = 0
	}
	return h
}

// EXPERIMENT.ASSIGN name user replies the user's variant, assigning one by
// a hash of the user the first time.
func cmdEXPERIMENTASSIGN(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, user := ctx.ArgString(0), ctx.ArgString(1)
	e, err := loadExperiment(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if v, ok := e.assignments[user]; ok {
		ctx.PropagateAs()
		return ctx.WriteBulkString(v)
	}
	v := e.variants[hashString(user)%len(e.variants)]
	if err := updateRecord(ctx, name, "a:"+user, v); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(v)
}

func cmdEXPERIMENTTRACK(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, user, event := ctx.ArgString(0), ctx.ArgString(1), ctx.ArgString(2)
	e, err := loadExperiment(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if _, ok := e.assignments[user]; !ok {
		return ctx.WriteError(errors.New("ERR user is not assigned to this experiment"))
	}
	if err := updateRecord(ctx, name, orsetField('e', event, user), i64(e.events[event][user]+1)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdEXPERIMENTRESULTS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	e, err := loadExperiment(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	assigned := map[string]int64{}
	for _, v := range e.assignments {
		assigned[v]++
	}
	eventNames := make([]string, 0, len(e.events))
	for ev := range e.events {
		eventNames = append(eventNames, ev)
	}
	sort.Strings(eventNames)
	out := []*resp.Value{
		resp.BulkString("name"), resp.BulkString(name),
		resp.BulkString("assignments"), resp.IntegerValue(int64(len(e.assignments))),
	}
	for _, v := range e.variants {
		events := make([]*resp.Value, 0, len(eventNames))
		for _, ev := range eventNames {
			var count, users int64
			for user, n := range e.events[ev] {
				if e.assignments[user] == v {
					count += n
					users++
				}
			}
			rate := 0.0
			if assigned[v] > 0 {
				rate = float64(users) / float64(assigned[v])
			}
			events = append(events, resp.ArrayValue([]*resp.Value{
				resp.BulkString("event"), resp.BulkString(ev),
				resp.BulkString("count"), resp.IntegerValue(count),
				resp.BulkString("users"), resp.IntegerValue(users),
				resp.BulkString("conversion_rate"), resp.BulkString(strconv.FormatFloat(rate, 'f', 4, 64)),
			}))
		}
		out = append(out,
			resp.BulkString("variant_"+v), resp.IntegerValue(assigned[v]),
			resp.BulkString("events_"+v), resp.ArrayValue(events),
		)
	}
	return ctx.WriteArray(out)
}

func cmdEXPERIMENTLIST(ctx *Context) error { return listMarked(ctx, experimentMarker) }

// --- ROLLOUT ---

func parsePercentage(arg string) (int64, error) {
	p, err := strconv.ParseInt(arg, 10, 64)
	if err != nil || p < 0 || p > 100 {
		return 0, errors.New("ERR percentage must be an integer from 0 to 100")
	}
	return p, nil
}

// ROLLOUT.CREATE name [percentage] makes a rollout, replacing one at name.
func cmdROLLOUTCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	p := int64(0)
	if ctx.ArgCount() == 2 {
		var err error
		if p, err = parsePercentage(ctx.ArgString(1)); err != nil {
			return ctx.WriteError(err)
		}
	}
	if err := createMarked(ctx, ctx.ArgString(0), rolloutMarker, rolloutMarker, i64(p)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdROLLOUTDELETE(ctx *Context) error { return deleteMarked(ctx, rolloutMarker) }

// ROLLOUT.CHECK name user is 1 when a hash of the user falls within the
// percentage.
func cmdROLLOUTCHECK(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := readMarked(ctx, ctx.ArgString(0), rolloutMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		return ctx.WriteInteger(0)
	}
	p, err := strconv.ParseInt(r[rolloutMarker], 10, 64)
	if err != nil {
		return ctx.WriteError(store.ErrWrongType)
	}
	return ctx.WriteInteger(boolInt(p >= 100 || int64(hashString(ctx.ArgString(1))%100) < p))
}

func cmdROLLOUTPERCENTAGE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	p, err := parsePercentage(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	name := ctx.ArgString(0)
	if _, err := loadMarked(ctx, name, rolloutMarker, "rollout"); err != nil {
		return ctx.WriteError(err)
	}
	if err := updateRecord(ctx, name, rolloutMarker, i64(p)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdROLLOUTLIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	var out []*resp.Value
	for _, name := range markedKeys(ctx, rolloutMarker) {
		v, err := markedInts(ctx, name, rolloutMarker, "rollout", rolloutMarker)
		if err != nil {
			continue
		}
		out = append(out, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("percentage"), resp.IntegerValue(v[0]),
		}))
	}
	return ctx.WriteArray(out)
}

// --- ALERT ---

// An alert is a hash marked by __alert (its status) with id, message,
// created_at and its last maxAlertHistory status changes as the fields
// h:<seq> ("status timestamp") from hfirst to hnext.

func alertTransition(ctx *Context, name, status string, r map[string]string) error {
	now := ctx.nowMS()
	first, next := int64(0), int64(0)
	if r != nil {
		var e1, e2 error
		first, e1 = strconv.ParseInt(r["hfirst"], 10, 64)
		next, e2 = strconv.ParseInt(r["hnext"], 10, 64)
		if e1 != nil || e2 != nil {
			return store.ErrWrongType
		}
	}
	pairs := []string{alertMarker, status, "h:" + i64(next), status + " " + i64(now), "hnext", i64(next + 1)}
	var drop string
	if next+1-first > maxAlertHistory {
		drop = "h:" + i64(first)
		pairs = append(pairs, "hfirst", i64(first+1))
	}
	if err := updateRecord(ctx, name, pairs...); err != nil {
		return err
	}
	if drop != "" {
		set, _ := ctx.Propagation()
		deleteMarkedFields(ctx, name, alertMarker, drop)
		del, _ := ctx.Propagation()
		ctx.PropagateAs(append(set, del...)...)
	}
	return nil
}

// ALERT.CREATE name message makes an active alert, replacing one at name.
func cmdALERTCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	now := ctx.nowMS()
	err := createMarked(ctx, name, alertMarker, alertMarker, "active", "id", generateUUID(), "message", ctx.ArgString(1),
		"created_at", i64(now), "hfirst", "0", "hnext", "1", "h:0", "active "+i64(now))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func alertTo(ctx *Context, status string) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	r, err := loadMarked(ctx, name, alertMarker, "alert")
	if err == nil {
		err = alertTransition(ctx, name, status, r)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdALERTTRIGGER(ctx *Context) error     { return alertTo(ctx, "firing") }
func cmdALERTACKNOWLEDGE(ctx *Context) error { return alertTo(ctx, "acknowledged") }
func cmdALERTRESOLVE(ctx *Context) error     { return alertTo(ctx, "resolved") }

func cmdALERTLIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	var out []*resp.Value
	for _, name := range markedKeys(ctx, alertMarker) {
		h, err := readMarkedHash(ctx, name, alertMarker)
		if err != nil || h == nil {
			continue
		}
		h.RLock()
		status, _ := h.Get(alertMarker)
		h.RUnlock()
		out = append(out, resp.ArrayValue([]*resp.Value{
			resp.BulkString("name"), resp.BulkString(name),
			resp.BulkString("status"), resp.BulkString(string(status)),
		}))
	}
	return ctx.WriteArray(out)
}

func cmdALERTHISTORY(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	r, err := loadMarked(ctx, name, alertMarker, "alert")
	if err != nil {
		return ctx.WriteError(err)
	}
	first, e1 := strconv.ParseInt(r["hfirst"], 10, 64)
	next, e2 := strconv.ParseInt(r["hnext"], 10, 64)
	created, e3 := strconv.ParseInt(r["created_at"], 10, 64)
	if e1 != nil || e2 != nil || e3 != nil {
		return ctx.WriteError(store.ErrWrongType)
	}
	history := make([]*resp.Value, 0, next-first)
	for i := first; i < next; i++ {
		status, ts, _ := strings.Cut(r["h:"+i64(i)], " ")
		n, err := strconv.ParseInt(ts, 10, 64)
		if err != nil {
			return ctx.WriteError(store.ErrWrongType)
		}
		history = append(history, resp.ArrayValue([]*resp.Value{
			resp.BulkString("status"), resp.BulkString(status),
			resp.BulkString("timestamp"), resp.IntegerValue(n),
		}))
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(r["id"]),
		resp.BulkString("name"), resp.BulkString(name),
		resp.BulkString("message"), resp.BulkString(r["message"]),
		resp.BulkString("status"), resp.BulkString(r[alertMarker]),
		resp.BulkString("created_at"), resp.IntegerValue(created),
		resp.BulkString("history"), resp.ArrayValue(history),
	})
}

// --- COUNTERX ---

func cmdCOUNTERXCREATE(ctx *Context) error { return counterXCreate(ctx, counterX1Marker) }
func cmdCOUNTERXINCR(ctx *Context) error   { return counterXAdd(ctx, counterX1Marker, 1) }
func cmdCOUNTERXDECR(ctx *Context) error   { return counterXAdd(ctx, counterX1Marker, -1) }
func cmdCOUNTERXGET(ctx *Context) error    { return counterXGet(ctx, counterX1Marker) }

// COUNTERX.RESET name sets the counter to 0, creating it.
func cmdCOUNTERXRESET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), counterX1Marker, counterX1Marker, "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdCOUNTERXDELETE(ctx *Context) error { return deleteMarked(ctx, counterX1Marker) }

// --- GAUGE ---

const (
	gaugeMarker  = "__gauge"
	traceMarker  = "__trace"
	logXMarker   = "__logx"
	apiKeyMarker = "__apikey"
)

func parseFiniteFloat(arg string) (float64, error) {
	v, err := strconv.ParseFloat(arg, 64)
	if err != nil || v != v || v > 1e308 || v < -1e308 {
		return 0, errAggValue
	}
	return v, nil
}

func gaugeValue(ctx *Context, name string) (float64, bool, error) {
	r, err := readMarked(ctx, name, gaugeMarker)
	if err != nil || r == nil {
		return 0, false, err
	}
	v, err := strconv.ParseFloat(r[gaugeMarker], 64)
	if err != nil {
		return 0, false, store.ErrWrongType
	}
	return v, true, nil
}

func setGauge(ctx *Context, name string, v float64) error {
	if _, _, err := gaugeValue(ctx, name); err != nil {
		return err
	}
	return updateRecord(ctx, name, gaugeMarker, f64(v))
}

// GAUGE.CREATE name [value] sets the gauge, 0 by default.
func cmdGAUGECREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v := 0.0
	if ctx.ArgCount() == 2 {
		var err error
		if v, err = parseFiniteFloat(ctx.ArgString(1)); err != nil {
			return ctx.WriteError(err)
		}
	}
	if err := setGauge(ctx, ctx.ArgString(0), v); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdGAUGESET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, err := parseFiniteFloat(ctx.ArgString(1))
	if err == nil {
		err = setGauge(ctx, ctx.ArgString(0), v)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdGAUGEGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, ok, err := gaugeValue(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if !ok {
		return ctx.WriteBulkString("0")
	}
	return ctx.WriteBulkString(fmt.Sprintf("%.6f", v))
}

// gaugeAdd adds sign times the optional amount (1 by default), creating the
// gauge at 0, and replies the value.
func gaugeAdd(ctx *Context, sign float64) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	by := 1.0
	if ctx.ArgCount() == 2 {
		var err error
		if by, err = parseFiniteFloat(ctx.ArgString(1)); err != nil {
			return ctx.WriteError(err)
		}
	}
	name := ctx.ArgString(0)
	v, _, err := gaugeValue(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	v += sign * by
	if v > 1e308 || v < -1e308 {
		return ctx.WriteError(errors.New("ERR increment would produce NaN or Infinity"))
	}
	if err := updateRecord(ctx, name, gaugeMarker, f64(v)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(fmt.Sprintf("%.6f", v))
}

func cmdGAUGEINCR(ctx *Context) error   { return gaugeAdd(ctx, 1) }
func cmdGAUGEDECR(ctx *Context) error   { return gaugeAdd(ctx, -1) }
func cmdGAUGEDELETE(ctx *Context) error { return deleteMarked(ctx, gaugeMarker) }

// --- TRACE ---

// A trace is a hash marked by __trace (its status) with spans, the span
// count, and each span as the field s:<i> holding a JSON object.

type traceSpan struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Start int64  `json:"start"`
	End   int64  `json:"end"`
}

func loadTrace(ctx *Context, id string) (string, []traceSpan, error) {
	r, err := loadMarked(ctx, id, traceMarker, "trace")
	if err != nil {
		return "", nil, err
	}
	n, err := strconv.Atoi(r["spans"])
	if err != nil || n < 0 {
		return "", nil, store.ErrWrongType
	}
	spans := make([]traceSpan, n)
	for i := range spans {
		if json.Unmarshal([]byte(r["s:"+strconv.Itoa(i)]), &spans[i]) != nil {
			return "", nil, store.ErrWrongType
		}
	}
	return r[traceMarker], spans, nil
}

// TRACE.START id starts a trace, replacing one at id.
func cmdTRACESTART(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), traceMarker, traceMarker, "running", "spans", "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// TRACE.SPAN trace name span_id opens a span.
func cmdTRACESPAN(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	_, spans, err := loadTrace(ctx, id)
	if err != nil {
		return ctx.WriteError(err)
	}
	raw, err := json.Marshal(traceSpan{ID: ctx.ArgString(2), Name: ctx.ArgString(1), Start: ctx.nowMS()})
	if err == nil {
		err = updateRecord(ctx, id, "s:"+strconv.Itoa(len(spans)), string(raw), "spans", strconv.Itoa(len(spans)+1))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// TRACE.END id completes the trace and closes its open spans.
func cmdTRACEEND(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	_, spans, err := loadTrace(ctx, id)
	if err != nil {
		return ctx.WriteError(err)
	}
	now := ctx.nowMS()
	pairs := []string{traceMarker, "completed"}
	for i, s := range spans {
		if s.End == 0 {
			s.End = now
			raw, err := json.Marshal(s)
			if err != nil {
				return ctx.WriteError(err)
			}
			pairs = append(pairs, "s:"+strconv.Itoa(i), string(raw))
		}
	}
	if err := updateRecord(ctx, id, pairs...); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdTRACEGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	status, spans, err := loadTrace(ctx, id)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(id),
		resp.BulkString("status"), resp.BulkString(status),
		resp.BulkString("spans"), resp.IntegerValue(int64(len(spans))),
	})
}

func cmdTRACELIST(ctx *Context) error { return listMarked(ctx, traceMarker) }

// --- LOGX ---

// A LOGX log is a log (see readLog) marked by __logx whose entries are JSON
// objects.

type logXEntry struct {
	Timestamp int64  `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

func cmdLOGXWRITE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	raw, err := json.Marshal(logXEntry{Timestamp: ctx.nowMS(), Level: ctx.ArgString(1), Message: ctx.ArgString(2)})
	if err != nil {
		return ctx.WriteError(err)
	}
	n, _, _, err := logHead(ctx, name, logXMarker)
	if err == nil {
		err = appendLog(ctx, name, logXMarker, n, string(raw))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func readLogX(ctx *Context, name string) ([]logXEntry, error) {
	raw, _, err := readLog(ctx, name, logXMarker)
	if err != nil {
		return nil, err
	}
	out := make([]logXEntry, len(raw))
	for i, r := range raw {
		if json.Unmarshal([]byte(r), &out[i]) != nil {
			return nil, store.ErrWrongType
		}
	}
	return out, nil
}

func logXReply(e logXEntry) *resp.Value {
	return resp.ArrayValue([]*resp.Value{
		resp.BulkString("timestamp"), resp.IntegerValue(e.Timestamp),
		resp.BulkString("level"), resp.BulkString(e.Level),
		resp.BulkString("message"), resp.BulkString(e.Message),
	})
}

// LOGX.READ name [limit] replies the last limit entries (100 by default).
func cmdLOGXREAD(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	limit := 100
	if ctx.ArgCount() == 2 {
		n, err := strconv.Atoi(ctx.ArgString(1))
		if err != nil || n < 0 {
			return ctx.WriteError(errors.New("ERR limit must be a non-negative integer"))
		}
		limit = n
	}
	entries, err := readLogX(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	entries = entries[max(len(entries)-limit, 0):]
	out := make([]*resp.Value, len(entries))
	for i, e := range entries {
		out[i] = logXReply(e)
	}
	return ctx.WriteArray(out)
}

// LOGX.SEARCH name text replies the entries whose message contains text.
func cmdLOGXSEARCH(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	entries, err := readLogX(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	var out []*resp.Value
	for _, e := range entries {
		if strings.Contains(e.Message, ctx.ArgString(1)) {
			out = append(out, logXReply(e))
		}
	}
	return ctx.WriteArray(out)
}

// LOGX.CLEAR name deletes the log and replies how many entries it held.
func cmdLOGXCLEAR(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	n, _, ok, err := logHead(ctx, name, logXMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if !ok {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	ctx.Store.Delete(name)
	ctx.notify(store.NotifyGeneric, "del", name)
	ctx.propagateDelete(name)
	return ctx.WriteInteger(int64(n))
}

func cmdLOGXSTATS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	n, _, _, err := logHead(ctx, ctx.ArgString(0), logXMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(ctx.ArgString(0)),
		resp.BulkString("entries"), resp.IntegerValue(int64(n)),
	})
}

// --- APIKEY ---

// An API key is a hash named by the generated key, marked by __apikey (its
// name), with created_at, usage and active.

func cmdAPIKEYCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := newObjectID(ctx)
	if err := replaceRecord(ctx, key, apiKeyMarker, ctx.ArgString(0), "created_at", i64(ctx.nowMS()), "usage", "0", "active", "1"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(key)
}

// APIKEY.VALIDATE key is 1 for an active key, counting the use.
func cmdAPIKEYVALIDATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	r, err := readMarked(ctx, key, apiKeyMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil || r["active"] != "1" {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	usage, err := strconv.ParseInt(r["usage"], 10, 64)
	if err != nil {
		return ctx.WriteError(store.ErrWrongType)
	}
	if err := updateRecord(ctx, key, "usage", i64(usage+1)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(1)
}

// APIKEY.REVOKE key deactivates the key: 1 when it existed.
func cmdAPIKEYREVOKE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	r, err := readMarked(ctx, key, apiKeyMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	if err := updateRecord(ctx, key, "active", "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(1)
}

func cmdAPIKEYLIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	var out []*resp.Value
	for _, key := range markedKeys(ctx, apiKeyMarker) {
		r, err := readMarked(ctx, key, apiKeyMarker)
		if err != nil || r == nil {
			continue
		}
		out = append(out, resp.ArrayValue([]*resp.Value{
			resp.BulkString("key"), resp.BulkString(key),
			resp.BulkString("name"), resp.BulkString(r[apiKeyMarker]),
			resp.BulkString("active"), resp.BulkString(strconv.FormatBool(r["active"] == "1")),
		}))
	}
	return ctx.WriteArray(out)
}

func cmdAPIKEYUSAGE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, err := markedInts(ctx, ctx.ArgString(0), apiKeyMarker, "api key", "usage")
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(v[0])
}

// --- QUOTAX ---

// A QUOTAX quota is a hash of limit, used, period (ms; 0 never resets) and
// reset_at.

var quotaXKind = recordKind{name: "quota", fields: []string{"limit", "used", "period", "reset_at"}}

type quotaX struct{ limit, used, period, resetAt int64 }

func loadQuotaX(ctx *Context) (*quotaX, error) {
	r, err := quotaXKind.load(ctx, ctx.ArgString(0))
	if err != nil {
		return nil, err
	}
	q := &quotaX{r.int("limit"), r.int("used"), r.int("period"), r.int("reset_at")}
	return q, errWrongTypeOr(r.bad, nil)
}

// roll starts a new window once the current one is over.
func (q *quotaX) roll(now int64) {
	if q.period > 0 && now > q.resetAt {
		q.used, q.resetAt = 0, now+q.period
	}
}

func (q *quotaX) save(ctx *Context) error {
	return updateRecord(ctx, ctx.ArgString(0), "used", i64(q.used), "reset_at", i64(q.resetAt))
}

// QUOTAX.CREATE name limit period_ms makes a quota, replacing one at name.
func cmdQUOTAXCREATE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	limit, err := parseNonNegative(ctx.ArgString(1), "limit")
	if err != nil {
		return ctx.WriteError(err)
	}
	period, err := parseNonNegative(ctx.ArgString(2), "period")
	if err != nil {
		return ctx.WriteError(err)
	}
	return createRecord(ctx, quotaXKind, ctx.ArgString(0), "limit", i64(limit), "used", "0", "period", i64(period), "reset_at", i64(ctx.nowMS()+period))
}

// QUOTAX.CHECK name amount uses amount when it fits: [1, remaining], else
// [0, remaining].
func cmdQUOTAXCHECK(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	amount, err := parseNonNegative(ctx.ArgString(1), "amount")
	if err != nil {
		return ctx.WriteError(err)
	}
	q, err := loadQuotaX(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	q.roll(ctx.nowMS())
	ok := amount <= q.limit-q.used
	if ok {
		q.used += amount
	}
	if err := q.save(ctx); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{resp.IntegerValue(boolInt(ok)), resp.IntegerValue(q.limit - q.used)})
}

func cmdQUOTAXUSAGE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	q, err := loadQuotaX(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	q.roll(ctx.nowMS())
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("used"), resp.IntegerValue(q.used),
		resp.BulkString("limit"), resp.IntegerValue(q.limit),
		resp.BulkString("remaining"), resp.IntegerValue(q.limit - q.used),
	})
}

func cmdQUOTAXRESET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	q, err := loadQuotaX(ctx)
	if err == nil {
		q.used, q.resetAt = 0, ctx.nowMS()+q.period
		err = q.save(ctx)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdQUOTAXDELETE(ctx *Context) error { return deleteRecord(ctx, quotaXKind) }

// --- METER ---

// A meter is a hash marked by __meter (its unit) with the total and count
// of the values recorded.

const (
	meterMarker  = "__meter"
	tenantMarker = "__tenant"
	leaseMarker  = "__lease"
)

func cmdMETERCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), meterMarker, meterMarker, ctx.ArgString(1), "total", "0", "count", "0"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdMETERRECORD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, err := parseInt64Arg(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	name := ctx.ArgString(0)
	m, err := markedInts(ctx, name, meterMarker, "meter", "total", "count")
	if err != nil {
		return ctx.WriteError(err)
	}
	if (v > 0 && m[0] > (1<<63-1)-v) || (v < 0 && m[0] < -(1<<63-1)-1-v) {
		return ctx.WriteError(errors.New("ERR meter total would overflow"))
	}
	if err := updateRecord(ctx, name, "total", i64(m[0]+v), "count", i64(m[1]+1)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdMETERGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	r, err := loadMarked(ctx, name, meterMarker, "meter")
	if err != nil {
		return ctx.WriteError(err)
	}
	m, err := markedInts(ctx, name, meterMarker, "meter", "total", "count")
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(name),
		resp.BulkString("unit"), resp.BulkString(r[meterMarker]),
		resp.BulkString("total"), resp.IntegerValue(m[0]),
		resp.BulkString("count"), resp.IntegerValue(m[1]),
	})
}

func cmdMETERDELETE(ctx *Context) error { return deleteMarked(ctx, meterMarker) }

// --- TENANT ---

// A tenant is a hash marked by __tenant (its name) with created_at and its
// settings as c:<key>.

func cmdTENANTCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), tenantMarker, tenantMarker, ctx.ArgString(1), "created_at", i64(ctx.nowMS())); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdTENANTDELETE(ctx *Context) error { return deleteMarked(ctx, tenantMarker) }

func cmdTENANTGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	r, err := loadMarked(ctx, id, tenantMarker, "tenant")
	if err != nil {
		return ctx.WriteError(err)
	}
	created, err := strconv.ParseInt(r["created_at"], 10, 64)
	if err != nil {
		return ctx.WriteError(store.ErrWrongType)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(id),
		resp.BulkString("name"), resp.BulkString(r[tenantMarker]),
		resp.BulkString("created_at"), resp.IntegerValue(created),
	})
}

func cmdTENANTLIST(ctx *Context) error { return listMarked(ctx, tenantMarker) }

func cmdTENANTCONFIG(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	if _, err := loadMarked(ctx, id, tenantMarker, "tenant"); err != nil {
		return ctx.WriteError(err)
	}
	if err := updateRecord(ctx, id, "c:"+ctx.ArgString(1), ctx.ArgString(2)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// --- LEASE ---

// A lease is a hash marked by __lease (its holder) that expires with the
// lease, so an expired lease can be taken again.

func cmdLEASECREATE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ttl, err := parseTokenTTL(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	id := ctx.ArgString(0)
	r, err := readMarked(ctx, id, leaseMarker)
	if err == nil && r != nil {
		err = errors.New("ERR lease already exists")
	}
	if err == nil {
		err = setExpiring(ctx, id, ttl, leaseMarker, ctx.ArgString(1))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdLEASERENEW(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ttl, err := parseTokenTTL(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	id := ctx.ArgString(0)
	r, err := loadMarked(ctx, id, leaseMarker, "lease")
	if err == nil {
		err = setExpiring(ctx, id, ttl, leaseMarker, r[leaseMarker])
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdLEASEREVOKE(ctx *Context) error { return deleteMarked(ctx, leaseMarker) }

func cmdLEASEGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	r, err := readMarked(ctx, id, leaseMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	entry, ok := ctx.Store.Get(id)
	if r == nil || !ok {
		return ctx.WriteNull()
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.BulkString(id),
		resp.BulkString("holder"), resp.BulkString(r[leaseMarker]),
		resp.BulkString("expires_at"), resp.IntegerValue(entry.ExpiresAt.Load()),
	})
}

func cmdLEASELIST(ctx *Context) error { return listMarked(ctx, leaseMarker) }

// --- HEAP ---

// A heap is a min-heap of integers kept as a sorted set scored by value,
// with a sequence number as each member so a value may be pushed twice.

const maxHeapValue = 1 << 53

func cmdHEAPPUSH(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil || v < -maxHeapValue || v > maxHeapValue {
		return ctx.WriteError(fmt.Errorf("ERR value must be an integer between %d and %d", int64(-maxHeapValue), int64(maxHeapValue)))
	}
	name := ctx.ArgString(0)
	if _, err := readPQ(ctx, name); err != nil {
		return ctx.WriteError(err)
	}
	saved := ctx.Writer
	var sink strings.Builder
	ctx.Writer = resp.NewWriter(&sink)
	err = runAs(ctx, "ZADD", cmdZADD, name, strconv.FormatInt(v, 10), nextPQSeq(ctx)+":")
	ctx.Writer = saved
	if err != nil {
		return err
	}
	if reply := sink.String(); strings.HasPrefix(reply, "-") {
		ctx.PropagateAs()
		return ctx.WriteError(errors.New(strings.TrimRight(reply[1:], "\r\n")))
	}
	z, err := readPQ(ctx, name)
	if err != nil || z == nil {
		return ctx.WriteError(errWrongTypeOr(true, err))
	}
	z.RLock()
	n := z.Card()
	z.RUnlock()
	return ctx.WriteInteger(int64(n))
}

func cmdHEAPPOP(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	e, err := pqFirst(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if e == nil {
		ctx.PropagateAs()
		return ctx.WriteNull()
	}
	saved := ctx.Writer
	var sink strings.Builder
	ctx.Writer = resp.NewWriter(&sink)
	err = runAs(ctx, "ZREM", cmdZREM, name, e.Member)
	ctx.Writer = saved
	if err != nil {
		return err
	}
	return ctx.WriteInteger(int64(e.Score))
}

func cmdHEAPPEEK(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	e, err := pqFirst(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if e == nil {
		return ctx.WriteNull()
	}
	return ctx.WriteInteger(int64(e.Score))
}

func cmdHEAPSIZE(ctx *Context) error { return cmdPQLEN(ctx) }

func cmdHEAPDELETE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	z, err := readPQ(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if z == nil {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	ctx.Store.Delete(name)
	ctx.notify(store.NotifyGeneric, "del", name)
	ctx.propagateDelete(name)
	return ctx.WriteInteger(1)
}

// --- BLOOMX ---

// A BLOOMX filter is a hash marked by __bloomx (its size in bits) with
// hashes and one field b<i> per set bit, so it holds as much as was added.

const (
	maxBloomXSize     = 1 << 26
	maxBloomXHashes   = 64
	maxExtensionSlots = 1 << 20
	maxSketchCells    = 1 << 22
	bloomXMarker      = "__bloomx"
	sketchMarker      = "__sketch"
	ringBufferMarker  = "__ringbuffer"
	windowMarker      = "__window"
	freqMarker        = "__freq"
	partitionMarker   = "__partition"
)

func checkSlots(what string, n int) error {
	if n <= 0 {
		return fmt.Errorf("ERR %s must be positive", what)
	}
	if n > maxExtensionSlots {
		return fmt.Errorf("ERR %s must be at most %d", what, maxExtensionSlots)
	}
	return nil
}

func bloomHash(item string, seed, size int) int {
	h := seed
	for _, c := range item {
		h = h*31 + int(c)
	}
	return (h%size + size) % size
}

func cmdBLOOMXCREATE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	size, err1 := strconv.Atoi(ctx.ArgString(1))
	hashes, err2 := strconv.Atoi(ctx.ArgString(2))
	if err1 != nil || size <= 0 || size > maxBloomXSize {
		return ctx.WriteError(fmt.Errorf("ERR size must be between 1 and %d", maxBloomXSize))
	}
	if err2 != nil || hashes <= 0 || hashes > maxBloomXHashes {
		return ctx.WriteError(fmt.Errorf("ERR hashes must be between 1 and %d", maxBloomXHashes))
	}
	if err := createMarked(ctx, ctx.ArgString(0), bloomXMarker, bloomXMarker, strconv.Itoa(size), "hashes", strconv.Itoa(hashes)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func loadBloomX(ctx *Context, name string) (*store.HashValue, int, int, error) {
	h, err := readMarkedHash(ctx, name, bloomXMarker)
	if err == nil && h == nil {
		err = errors.New("ERR bloom filter not found")
	}
	if err != nil {
		return nil, 0, 0, err
	}
	h.RLock()
	rs, _ := h.Get(bloomXMarker)
	rh, _ := h.Get("hashes")
	h.RUnlock()
	size, e1 := strconv.Atoi(string(rs))
	hashes, e2 := strconv.Atoi(string(rh))
	if e1 != nil || e2 != nil || size <= 0 || hashes <= 0 || hashes > maxBloomXHashes {
		return nil, 0, 0, store.ErrWrongType
	}
	return h, size, hashes, nil
}

func cmdBLOOMXADD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, item := ctx.ArgString(0), ctx.ArgString(1)
	_, size, hashes, err := loadBloomX(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	pairs := make([]string, 0, 2*hashes)
	seen := map[int]bool{}
	for i := 0; i < hashes; i++ {
		if b := bloomHash(item, i, size); !seen[b] {
			seen[b] = true
			pairs = append(pairs, "b"+strconv.Itoa(b), "")
		}
	}
	if err := updateRecord(ctx, name, pairs...); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(1)
}

func cmdBLOOMXCHECK(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	h, size, hashes, err := loadBloomX(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	item := ctx.ArgString(1)
	h.RLock()
	defer h.RUnlock()
	for i := 0; i < hashes; i++ {
		if _, ok := h.Get("b" + strconv.Itoa(bloomHash(item, i, size))); !ok {
			return ctx.WriteInteger(0)
		}
	}
	return ctx.WriteInteger(1)
}

func cmdBLOOMXINFO(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_, size, hashes, err := loadBloomX(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(ctx.ArgString(0)),
		resp.BulkString("size"), resp.IntegerValue(int64(size)),
		resp.BulkString("hashes"), resp.IntegerValue(int64(hashes)),
	})
}

func cmdBLOOMXDELETE(ctx *Context) error { return deleteMarked(ctx, bloomXMarker) }

// --- SKETCH ---

// A count-min sketch is a hash marked by __sketch (its width) with depth
// and one field c<row>:<col> per nonzero cell.

func loadSketch(ctx *Context, name string) (*store.HashValue, int, int, error) {
	h, err := readMarkedHash(ctx, name, sketchMarker)
	if err == nil && h == nil {
		err = errors.New("ERR sketch not found")
	}
	if err != nil {
		return nil, 0, 0, err
	}
	h.RLock()
	rw, _ := h.Get(sketchMarker)
	rd, _ := h.Get("depth")
	h.RUnlock()
	width, e1 := strconv.Atoi(string(rw))
	depth, e2 := strconv.Atoi(string(rd))
	if e1 != nil || e2 != nil || width <= 0 || depth <= 0 {
		return nil, 0, 0, store.ErrWrongType
	}
	return h, width, depth, nil
}

func sketchCell(h *store.HashValue, row, col int) (string, int64) {
	f := "c" + strconv.Itoa(row) + ":" + strconv.Itoa(col)
	v, ok := h.Get(f)
	if !ok {
		return f, 0
	}
	n, _ := strconv.ParseInt(string(v), 10, 64) //nolint:errcheck // a damaged cell counts as 0
	return f, n
}

func cmdSKETCHCREATE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	width, err1 := strconv.Atoi(ctx.ArgString(1))
	depth, err2 := strconv.Atoi(ctx.ArgString(2))
	if err1 != nil || err2 != nil || width <= 0 || depth <= 0 {
		return ctx.WriteError(errors.New("ERR width and depth must be positive"))
	}
	if width > maxSketchCells || depth > maxSketchCells/width {
		return ctx.WriteError(fmt.Errorf("ERR width times depth must be at most %d", maxSketchCells))
	}
	if err := createMarked(ctx, ctx.ArgString(0), sketchMarker, sketchMarker, strconv.Itoa(width), "depth", strconv.Itoa(depth)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdSKETCHUPDATE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	count, err := parseInt64Arg(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	name, item := ctx.ArgString(0), ctx.ArgString(1)
	h, width, depth, err := loadSketch(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	pairs := make([]string, 0, 2*depth)
	h.RLock()
	for row := 0; row < depth; row++ {
		f, n := sketchCell(h, row, bloomHash(item, row, width))
		pairs = append(pairs, f, i64(n+count))
	}
	h.RUnlock()
	if err := updateRecord(ctx, name, pairs...); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdSKETCHQUERY(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	h, width, depth, err := loadSketch(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	item := ctx.ArgString(1)
	h.RLock()
	defer h.RUnlock()
	_, least := sketchCell(h, 0, bloomHash(item, 0, width))
	for row := 1; row < depth; row++ {
		_, n := sketchCell(h, row, bloomHash(item, row, width))
		least = min(least, n)
	}
	return ctx.WriteInteger(least)
}

func cmdSKETCHDELETE(ctx *Context) error { return deleteMarked(ctx, sketchMarker) }

// --- RINGBUFFER and WINDOW ---

// A ring is a hash marked by its kind (its size) with head, count and the
// slots s<i>, so a write sets one slot whatever the size.

type ring struct {
	size, head, count int
	h                 *store.HashValue
}

func loadRing2(ctx *Context, name, marker, what string) (*ring, error) {
	h, err := readMarkedHash(ctx, name, marker)
	if err == nil && h == nil {
		err = fmt.Errorf("ERR %s not found", what)
	}
	if err != nil {
		return nil, err
	}
	h.RLock()
	rs, _ := h.Get(marker)
	rh, _ := h.Get("head")
	rc, _ := h.Get("count")
	h.RUnlock()
	size, e1 := strconv.Atoi(string(rs))
	head, e2 := strconv.Atoi(string(rh))
	count, e3 := strconv.Atoi(string(rc))
	if e1 != nil || e2 != nil || e3 != nil || size < 1 || head < 0 || head >= size || count < 0 || count > size {
		return nil, store.ErrWrongType
	}
	return &ring{size: size, head: head, count: count, h: h}, nil
}

func (r *ring) values() []string {
	out := make([]string, r.count)
	tail := (r.head - r.count + r.size) % r.size
	r.h.RLock()
	for i := range out {
		v, _ := r.h.Get("s" + strconv.Itoa((tail+i)%r.size))
		out[i] = string(v)
	}
	r.h.RUnlock()
	return out
}

func ringCreate(ctx *Context, marker string, size int) error {
	return createMarked(ctx, ctx.ArgString(0), marker, marker, strconv.Itoa(size), "head", "0", "count", "0")
}

func (r *ring) push(ctx *Context, name, value string) error {
	return updateRecord(ctx, name, "s"+strconv.Itoa(r.head), value, "head", strconv.Itoa((r.head+1)%r.size), "count", strconv.Itoa(min(r.count+1, r.size)))
}

func cmdRINGBUFFERCREATE(ctx *Context) error {
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
	if err := ringCreate(ctx, ringBufferMarker, size); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdRINGBUFFERWRITE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	r, err := loadRing2(ctx, name, ringBufferMarker, "ring buffer")
	if err == nil {
		err = r.push(ctx, name, ctx.ArgString(1))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// RINGBUFFER.READ name replies the values, oldest first.
func cmdRINGBUFFERREAD(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := loadRing2(ctx, ctx.ArgString(0), ringBufferMarker, "ring buffer")
	if err != nil {
		if err == store.ErrWrongType {
			return ctx.WriteError(err)
		}
		return ctx.WriteArray([]*resp.Value{})
	}
	return ctx.WriteArray(bulkStrings(r.values()))
}

func cmdRINGBUFFERSIZE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r, err := loadRing2(ctx, ctx.ArgString(0), ringBufferMarker, "ring buffer")
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("size"), resp.IntegerValue(int64(r.size)),
		resp.BulkString("count"), resp.IntegerValue(int64(r.count)),
	})
}

func cmdRINGBUFFERDELETE(ctx *Context) error { return deleteMarked(ctx, ringBufferMarker) }

// A WINDOW is a ring of the last size numbers added.

func cmdWINDOWCREATE(ctx *Context) error {
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
	if err := ringCreate(ctx, windowMarker, size); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdWINDOWADD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	v, err := parseFiniteFloat(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	name := ctx.ArgString(0)
	r, err := loadRing2(ctx, name, windowMarker, "window")
	if err == nil {
		err = r.push(ctx, name, f64(v))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func windowValues(ctx *Context) ([]float64, error) {
	r, err := loadRing2(ctx, ctx.ArgString(0), windowMarker, "window")
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

func cmdWINDOWGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	data, err := windowValues(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	out := make([]*resp.Value, len(data))
	for i, v := range data {
		out[i] = resp.BulkString(fmt.Sprintf("%.6f", v))
	}
	return ctx.WriteArray(out)
}

// WINDOW.AGGREGATE name sum|avg|min|max
func cmdWINDOWAGGREGATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	agg := ctx.ArgString(1)
	if agg != "sum" && agg != "avg" && agg != "min" && agg != "max" {
		return ctx.WriteError(errors.New("ERR unknown aggregation"))
	}
	data, err := windowValues(ctx)
	if err != nil {
		return ctx.WriteError(err)
	}
	if len(data) == 0 {
		return ctx.WriteBulkString("0")
	}
	var out float64
	switch agg {
	case "sum", "avg":
		out = aggSum(data)
		if agg == "avg" {
			out /= float64(len(data))
		}
	case "min":
		out = data[0]
		for _, v := range data {
			out = min(out, v)
		}
	case "max":
		out = data[0]
		for _, v := range data {
			out = max(out, v)
		}
	}
	return ctx.WriteBulkString(fmt.Sprintf("%.6f", out))
}

func cmdWINDOWDELETE(ctx *Context) error { return deleteMarked(ctx, windowMarker) }

// --- FREQ ---

// A frequency table is a hash marked by __freq with i:<item> holding each
// item's count.

func cmdFREQCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), freqMarker, freqMarker, "1"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdFREQADD(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount() > 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	by := int64(1)
	if ctx.ArgCount() == 3 {
		var err error
		if by, err = parseInt64Arg(ctx.ArgString(2)); err != nil {
			return ctx.WriteError(err)
		}
	}
	name, item := ctx.ArgString(0), ctx.ArgString(1)
	h, err := readMarkedHash(ctx, name, freqMarker)
	if err == nil && h == nil {
		err = errors.New("ERR frequency not found")
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	h.RLock()
	raw, ok := h.Get("i:" + item)
	h.RUnlock()
	cur := int64(0)
	if ok {
		if cur, err = strconv.ParseInt(string(raw), 10, 64); err != nil {
			return ctx.WriteError(store.ErrWrongType)
		}
	}
	if (by > 0 && cur > (1<<63-1)-by) || (by < 0 && cur < -(1<<63-1)-1-by) {
		return ctx.WriteError(errors.New("ERR count would overflow"))
	}
	if err := updateRecord(ctx, name, "i:"+item, i64(cur+by)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(cur + by)
}

func cmdFREQCOUNT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	h, err := readMarkedHash(ctx, ctx.ArgString(0), freqMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if h == nil {
		return ctx.WriteInteger(0)
	}
	h.RLock()
	raw, _ := h.Get("i:" + ctx.ArgString(1))
	h.RUnlock()
	n, _ := strconv.ParseInt(string(raw), 10, 64) //nolint:errcheck // an absent item counts 0
	return ctx.WriteInteger(n)
}

// FREQ.TOP name [k] replies the k most frequent items (10 by default).
func cmdFREQTOP(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	k := 10
	if ctx.ArgCount() == 2 {
		n, err := strconv.Atoi(ctx.ArgString(1))
		if err != nil || n < 0 {
			return ctx.WriteError(errors.New("ERR k must be a non-negative integer"))
		}
		k = n
	}
	r, err := readMarked(ctx, ctx.ArgString(0), freqMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	type kv struct {
		item  string
		count int64
	}
	var items []kv
	for f, v := range r {
		if item, ok := strings.CutPrefix(f, "i:"); ok {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return ctx.WriteError(store.ErrWrongType)
			}
			items = append(items, kv{item, n})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		return items[i].item < items[j].item
	})
	items = items[:min(k, len(items))]
	out := make([]*resp.Value, len(items))
	for i, it := range items {
		out[i] = resp.ArrayValue([]*resp.Value{
			resp.BulkString("item"), resp.BulkString(it.item),
			resp.BulkString("count"), resp.IntegerValue(it.count),
		})
	}
	return ctx.WriteArray(out)
}

func cmdFREQDELETE(ctx *Context) error { return deleteMarked(ctx, freqMarker) }

// --- PARTITION ---

// A partitioned log is a hash marked by __partition (the partition count)
// with n:<p> holding partition p's length and v:<p>:<i> its values.

func loadPartition(ctx *Context, name string) (*store.HashValue, int, error) {
	h, err := readMarkedHash(ctx, name, partitionMarker)
	if err != nil || h == nil {
		return nil, 0, err
	}
	h.RLock()
	raw, _ := h.Get(partitionMarker)
	h.RUnlock()
	count, err := strconv.Atoi(string(raw))
	if err != nil || count < 1 || count > maxExtensionSlots {
		return nil, 0, store.ErrWrongType
	}
	return h, count, nil
}

func partitionLen(h *store.HashValue, p int) int {
	raw, _ := h.Get("n:" + strconv.Itoa(p))
	n, _ := strconv.Atoi(string(raw)) //nolint:errcheck // an empty partition has no length field
	return n
}

func cmdPARTITIONCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	count, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}
	if err := checkSlots("count", count); err != nil {
		return ctx.WriteError(err)
	}
	if err := createMarked(ctx, ctx.ArgString(0), partitionMarker, partitionMarker, strconv.Itoa(count)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// PARTITION.ADD name key value appends value to the partition key hashes to
// and replies that partition.
func cmdPARTITIONADD(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	h, count, err := loadPartition(ctx, name)
	if err == nil && h == nil {
		err = errors.New("ERR partition not found")
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	p := hashString(ctx.ArgString(1)) % count
	h.RLock()
	n := partitionLen(h, p)
	h.RUnlock()
	ps := strconv.Itoa(p)
	if err := updateRecord(ctx, name, "v:"+ps+":"+strconv.Itoa(n), ctx.ArgString(2), "n:"+ps, strconv.Itoa(n+1)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(p))
}

func cmdPARTITIONGET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	h, count, err := loadPartition(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	p, perr := strconv.Atoi(ctx.ArgString(1))
	if h == nil || perr != nil || p < 0 || p >= count {
		return ctx.WriteArray([]*resp.Value{})
	}
	ps := strconv.Itoa(p)
	h.RLock()
	n := partitionLen(h, p)
	values := make([]string, n)
	for i := range values {
		v, _ := h.Get("v:" + ps + ":" + strconv.Itoa(i))
		values[i] = string(v)
	}
	h.RUnlock()
	return ctx.WriteArray(bulkStrings(values))
}

func cmdPARTITIONLIST(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	h, count, err := loadPartition(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if h == nil {
		return ctx.WriteArray([]*resp.Value{})
	}
	out := make([]*resp.Value, count)
	h.RLock()
	for p := 0; p < count; p++ {
		out[p] = resp.ArrayValue([]*resp.Value{
			resp.BulkString("partition"), resp.IntegerValue(int64(p)),
			resp.BulkString("count"), resp.IntegerValue(int64(partitionLen(h, p))),
		})
	}
	h.RUnlock()
	return ctx.WriteArray(out)
}

func cmdPARTITIONDELETE(ctx *Context) error { return deleteMarked(ctx, partitionMarker) }
