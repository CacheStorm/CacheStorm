package command

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// The time series commands follow RedisTimeSeries, the module built into
// Redis 8: replies and error texts were checked against Redis 8.2. A series
// is a key of type TSDB-TYPE; sample values reply as simple strings in
// RedisTimeSeries' number format.

//nolint:staticcheck // RedisTimeSeries' reply texts
var (
	errTSKeyExists        = errors.New("ERR TSDB: key already exists")
	errTSKeyMissing       = errors.New("ERR TSDB: the key does not exist")
	errTSNotTSDB          = errors.New("ERR TSDB: the key is not a TSDB key")
	errTSWrongType        = errors.New("ERR WRONGTYPE Operation against a key holding the wrong kind of value")
	errTSMatcher          = errors.New("ERR TSDB: please provide at least one matcher")
	errTSFilter           = errors.New("ERR TSDB: failed parsing labels")
	errTSRetention        = errors.New("ERR TSDB: Couldn't parse RETENTION")
	errTSRetentionValue   = errors.New("TSDB: Couldn't parse RETENTION")
	errTSChunkParse       = errors.New("ERR TSDB: Couldn't parse CHUNK_SIZE")
	errTSChunkSize        = errors.New("ERR TSDB: CHUNK_SIZE value must be a multiple of 8 in the range [48 .. 1048576]")
	errTSEncoding         = errors.New("ERR TSDB: unknown ENCODING parameter")
	errTSPolicy           = errors.New("ERR TSDB: Unknown DUPLICATE_POLICY")
	errTSIgnore           = errors.New("ERR TSDB: Couldn't parse IGNORE")
	errTSTimestamp        = errors.New("ERR TSDB: invalid timestamp")
	errTSNegativeTS       = errors.New("ERR TSDB: invalid timestamp, must be a nonnegative integer")
	errTSValue            = errors.New("ERR TSDB: invalid value")
	errTSIncrValue        = errors.New("ERR TSDB: invalid increase/decrease value")
	errTSIncrOlder        = errors.New("TSDB: timestamp must be equal to or higher than the maximum existing timestamp")
	errTSFrom             = errors.New("ERR TSDB: wrong fromTimestamp")
	errTSTo               = errors.New("ERR TSDB: wrong toTimestamp")
	errTSCount            = errors.New("ERR TSDB: Invalid COUNT value")
	errTSFilterByTS       = errors.New("ERR TSDB: FILTER_BY_TS one or more arguments are missing")
	errTSFilterMin        = errors.New("ERR TSDB: Couldn't parse MIN")
	errTSFilterMax        = errors.New("ERR TSDB: Couldn't parse MAX")
	errTSAggregation      = errors.New("ERR TSDB: Unknown aggregation type")
	errTSBucket           = errors.New("ERR TSDB: bucketDuration must be greater than zero")
	errTSBucketTimestamp  = errors.New("ERR TSDB: unknown BUCKETTIMESTAMP parameter")
	errTSAlignStart       = errors.New("ERR TSDB: start alignment can only be used with explicit start timestamp")
	errTSAlignEnd         = errors.New("ERR TSDB: end alignment can only be used with explicit end timestamp")
	errTSAlign            = errors.New("ERR TSDB: unknown ALIGN parameter")
	errTSSameKey          = errors.New("ERR TSDB: the source key and destination key should be different")
	errTSDestHasSource    = errors.New("ERR TSDB: the destination key already has a src rule")
	errTSRuleMissing      = errors.New("ERR TSDB: compaction rule does not exist")
	errTSGroupByReducer   = errors.New("ERR TSDB: failed parsing reducer")
	errTSSelectedWithBoth = errors.New("ERR TSDB: cannot accept WITHLABELS and SELECT_LABELS together")
	errTSSrcHasSource     = errors.New("ERR TSDB: the source key already has a source rule")
	errTSDestHasDest      = errors.New("ERR TSDB: the destination key already has a dst rule")
	errTSMissingFilter    = errors.New("ERR TSDB: missing FILTER argument")
	errTSFilterNoLabels   = errors.New("ERR TSDB: missing labels for filter argument")
	errTSGroupByOrder     = errors.New("ERR TSDB: GROUPBY should always come after filter")
)

func RegisterTSCommands(router *Router) {
	for name, handler := range map[string]func(*Context) error{
		"TS.CREATE": cmdTSCREATE, "TS.ALTER": cmdTSALTER, "TS.ADD": cmdTSADD, "TS.MADD": cmdTSMADD,
		"TS.INCRBY": cmdTSINCRBY, "TS.DECRBY": cmdTSDECRBY, "TS.DEL": cmdTSDEL,
		"TS.GET": cmdTSGET, "TS.MGET": cmdTSMGET, "TS.RANGE": cmdTSRANGE, "TS.REVRANGE": cmdTSREVRANGE,
		"TS.MRANGE": cmdTSMRANGE, "TS.MREVRANGE": cmdTSMREVRANGE, "TS.INFO": cmdTSINFO,
		"TS.QUERYINDEX": cmdTSQUERYINDEX, "TS.CREATERULE": cmdTSCREATERULE, "TS.DELETERULE": cmdTSDELETERULE,
	} {
		router.Register(&CommandDef{Name: name, Handler: handler})
	}
}

// getTimeSeries returns the series at key, nil when the key is absent; a
// key of another type is wrongType.
func getTimeSeries(ctx *Context, key string, wrongType error) (*store.TimeSeriesValue, error) {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return nil, nil
	}
	ts, ok := entry.Value.(*store.TimeSeriesValue)
	if !ok {
		return nil, wrongType
	}
	return ts, nil
}

// valueWritten accounts for a value changed in place: the key's memory and
// its watchers.
func valueWritten(ctx *Context, key string) {
	ctx.Store.Reaccount(key)
	ctx.Store.TouchKeys([]string{key})
}

// tsOptions are the series settings TS.CREATE, TS.ADD, TS.INCRBY and
// TS.ALTER take.
type tsOptions struct {
	retention              *int64
	chunkSize              *int64
	uncompressed           *bool
	policy                 string
	onDuplicate            string
	ignoreTime             *int64
	ignoreValue            float64
	labelKeys, labelValues []string
	labels                 bool
}

// parseTSOptions reads the settings from args[from:]; unknown words are
// skipped, as RedisTimeSeries does.
func parseTSOptions(ctx *Context, from int) (*tsOptions, error) {
	o := &tsOptions{}
	n := ctx.ArgCount()
	for i := from; i < n; i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "RETENTION":
			if i+1 >= n {
				return nil, errTSRetention
			}
			i++
			v, err := strconv.ParseInt(ctx.ArgString(i), 10, 64)
			if err != nil || v < 0 {
				return nil, errTSRetentionValue
			}
			// A retention beyond what a duration holds (292 years) keeps
			// every sample anyway.
			v = min(v, maxExpireAtMillis)
			o.retention = &v
		case "CHUNK_SIZE":
			if i+1 >= n {
				return nil, errTSChunkParse
			}
			i++
			v, err := strconv.ParseInt(ctx.ArgString(i), 10, 64)
			if err != nil {
				return nil, errTSChunkParse
			}
			if v < 48 || v > 1048576 || v%8 != 0 {
				return nil, errTSChunkSize
			}
			o.chunkSize = &v
		case "ENCODING":
			if i+1 >= n {
				return nil, errTSEncoding
			}
			i++
			var u bool
			switch strings.ToUpper(ctx.ArgString(i)) {
			case "COMPRESSED":
			case "UNCOMPRESSED":
				u = true
			default:
				return nil, errTSEncoding
			}
			o.uncompressed = &u
		case "UNCOMPRESSED":
			u := true
			o.uncompressed = &u
		case "DUPLICATE_POLICY", "ON_DUPLICATE":
			word := strings.ToUpper(ctx.ArgString(i))
			if i+1 >= n {
				return nil, errTSPolicy
			}
			i++
			p, ok := store.ParseTSDuplicatePolicy(ctx.ArgString(i))
			if !ok {
				return nil, errTSPolicy
			}
			if word == "ON_DUPLICATE" {
				o.onDuplicate = p
			} else {
				o.policy = p
			}
		case "IGNORE":
			if i+2 >= n {
				return nil, errTSIgnore
			}
			t, err1 := strconv.ParseInt(ctx.ArgString(i+1), 10, 64)
			v, err2 := strconv.ParseFloat(ctx.ArgString(i+2), 64)
			if err1 != nil || err2 != nil || t < 0 || v < 0 {
				return nil, errTSIgnore
			}
			o.ignoreTime, o.ignoreValue = &t, v
			i += 2
		case "LABELS":
			o.labels = true
			for i+2 < n {
				o.labelKeys = append(o.labelKeys, ctx.ArgString(i+1))
				o.labelValues = append(o.labelValues, ctx.ArgString(i+2))
				i += 2
			}
			i = n
		}
	}
	return o, nil
}

func (o *tsOptions) apply(ts *store.TimeSeriesValue, creating bool) {
	if o.retention != nil {
		ts.SetRetention(time.Duration(*o.retention) * time.Millisecond)
	}
	if o.chunkSize != nil {
		ts.ChunkSize = *o.chunkSize
	}
	if o.uncompressed != nil {
		ts.Uncompressed = *o.uncompressed
	}
	if o.policy != "" {
		ts.DuplicatePolicy = o.policy
	}
	if o.ignoreTime != nil {
		ts.IgnoreMaxTimeDiff, ts.IgnoreMaxValDiff = *o.ignoreTime, o.ignoreValue
	}
	if o.labels || creating {
		ts.SetLabels(o.labelKeys, o.labelValues)
	}
}

// createSeries stores a new series at key with o's settings.
func createSeries(ctx *Context, key string, o *tsOptions) (*store.TimeSeriesValue, error) {
	ts := store.NewTimeSeriesValue(0)
	o.apply(ts, true)
	if err := ctx.Store.Set(key, ts, store.SetOptions{}); err != nil {
		return nil, err
	}
	return ts, nil
}

func cmdTSCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	o, err := parseTSOptions(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	key := ctx.ArgString(0)
	if ctx.Store.Exists(key) {
		return ctx.WriteError(errTSKeyExists)
	}
	if _, err := createSeries(ctx, key, o); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdTSALTER(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	o, err := parseTSOptions(ctx, 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	key := ctx.ArgString(0)
	ts, err := getTimeSeries(ctx, key, errTSNotTSDB)
	if err != nil {
		return ctx.WriteError(err)
	}
	if ts == nil {
		return ctx.WriteError(errTSKeyMissing)
	}
	ts.Lock()
	o.apply(ts, false)
	ts.Unlock()
	valueWritten(ctx, key)
	return ctx.WriteOK()
}

// parseTSTimestamp reads a sample timestamp; * is the current time.
func parseTSTimestamp(s string) (int64, error) {
	if s == "*" {
		return time.Now().UnixMilli(), nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, errTSTimestamp
	}
	if v < 0 {
		return 0, errTSNegativeTS
	}
	return v, nil
}

func parseTSValue(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
}

// addSample adds a sample to the series at key and feeds its compaction
// rules; policy overrides the series' duplicate policy when set.
func addSample(ctx *Context, key string, ts *store.TimeSeriesValue, at int64, value float64, policy string) (int64, error) {
	ts.Lock()
	last, had := ts.Last()
	ignored := ts.Ignores(at, value, policy)
	reply, err := ts.Upsert(at, value, policy)
	var closed, recomputed []tsClosedBucket
	switch {
	case err != nil || ignored:
	case had && at <= last.Timestamp:
		recomputed = upsertRules(ts, at, last.Timestamp)
	default:
		closed = feedRules(ts, at, value)
	}
	ts.Unlock()
	if err != nil {
		return 0, err
	}
	valueWritten(ctx, key)
	for _, c := range closed {
		dest, derr := getTimeSeries(ctx, c.dest, errTSNotTSDB)
		if derr != nil || dest == nil {
			continue
		}
		addSample(ctx, c.dest, dest, c.start, c.value, "LAST") //nolint:errcheck // a compaction is best effort, as in RedisTimeSeries
	}
	for _, c := range recomputed {
		upsertCompacted(ctx, c)
	}
	return reply, nil
}

type tsClosedBucket struct {
	dest  string
	start int64
	value float64
}

// upsertCompacted overwrites a recomputed bucket in a rule's destination,
// without feeding the destination's own rules, as RedisTimeSeries does.
func upsertCompacted(ctx *Context, c tsClosedBucket) {
	dest, err := getTimeSeries(ctx, c.dest, errTSNotTSDB)
	if err != nil || dest == nil {
		return
	}
	dest.Lock()
	_, err = dest.Upsert(c.start, c.value, "LAST")
	dest.Unlock()
	if err == nil {
		valueWritten(ctx, c.dest)
	}
}

// ruleBucketValue aggregates the series' samples in the rule's bucket at
// start; ok is false when the bucket has none.
func ruleBucketValue(ts *store.TimeSeriesValue, r *store.TSRule, start int64) (float64, bool) {
	norm := max(start, 0)
	in := ts.Range(norm, start+r.Bucket-1)
	if len(in) == 0 {
		return 0, false
	}
	if r.Aggregation != "twa" {
		return store.TSAggregate(r.Aggregation, in), true
	}
	var prev, next *store.TimeSeriesSample
	if p, ok := tsBefore(ts.Samples, norm); ok && norm > 0 {
		prev = &p
	}
	if n, ok := tsFrom(ts.Samples, start+r.Bucket); ok {
		next = &n
	}
	return twaBucket(in, prev, next, norm, start+r.Bucket, false), true
}

// openRuleBucket makes the series' samples from start to end the rule's
// open bucket and reports whether there were none.
func openRuleBucket(ts *store.TimeSeriesValue, r *store.TSRule, start, end int64) bool {
	norm := max(start, 0)
	r.Open = ts.Range(norm, end)
	r.Prev = nil
	if p, ok := tsBefore(ts.Samples, norm); ok && norm > 0 {
		r.Prev = &p
	}
	return len(r.Open) == 0
}

// upsertRules recomputes the compaction a sample at or before the newest one
// changed: the open bucket, or an already closed bucket in the destination.
func upsertRules(ts *store.TimeSeriesValue, at, last int64) []tsClosedBucket {
	var out []tsClosedBucket
	for i := range ts.Rules {
		r := &ts.Rules[i]
		cur := store.TSBucketStart(last, r.Bucket, r.Align)
		if at >= max(cur, 0) {
			openRuleBucket(ts, r, cur, cur+r.Bucket-1)
			continue
		}
		start := store.TSBucketStart(at, r.Bucket, r.Align)
		if v, ok := ruleBucketValue(ts, r, start); ok {
			out = append(out, tsClosedBucket{r.Dest, max(start, 0), v})
		}
	}
	return out
}

// tsRuleDeletion is what TS.DEL changes in a rule's destination: buckets to
// recompute, a range of buckets to delete, and whether the open bucket lost
// all its samples, so the destination's last bucket reopens.
type tsRuleDeletion struct {
	rule         int
	upserts      []tsClosedBucket
	from, to     int64
	reopenLatest bool
}

// deleteRules recomputes the compaction after the samples from..to were
// deleted from a series whose newest sample was at last.
func deleteRules(ts *store.TimeSeriesValue, from, to, last int64) []tsRuleDeletion {
	var out []tsRuleDeletion
	for i := range ts.Rules {
		r := &ts.Rules[i]
		d := tsRuleDeletion{rule: i, from: 1, to: 0}
		cur := store.TSBucketStart(last, r.Bucket, r.Align)
		curNorm := max(cur, 0)
		if from >= curNorm {
			d.reopenLatest = openRuleBucket(ts, r, cur, cur+r.Bucket-1)
			out = append(out, d)
			continue
		}
		start := store.TSBucketStart(from, r.Bucket, r.Align)
		end := store.TSBucketStart(to, r.Bucket, r.Align)
		if v, ok := ruleBucketValue(ts, r, start); ok {
			d.from = start + r.Bucket
			d.upserts = append(d.upserts, tsClosedBucket{r.Dest, max(start, 0), v})
		} else {
			d.from = max(start, 0)
		}
		switch v, ok := ruleBucketValue(ts, r, end); {
		case to >= curNorm:
			d.reopenLatest = openRuleBucket(ts, r, cur, math.MaxInt64)
			d.to = max(end-r.Bucket, 0)
		case ok:
			d.to = max(end-r.Bucket, 0)
			d.upserts = append(d.upserts, tsClosedBucket{r.Dest, max(end, 0), v})
		default:
			d.to = max(end, 0)
		}
		out = append(out, d)
	}
	return out
}

// deleteSamples is TS.DEL on a series, recomputing its compaction rules;
// deleting buckets from a destination cascades to that series' own rules.
func deleteSamples(ctx *Context, key string, ts *store.TimeSeriesValue, from, to int64, depth int) int {
	ts.Lock()
	last, had := ts.Last()
	deleted := ts.DeleteRange(from, to)
	var changes []tsRuleDeletion
	if had {
		changes = deleteRules(ts, from, to, last.Timestamp)
	}
	rules := append([]store.TSRule(nil), ts.Rules...)
	ts.Unlock()
	if deleted > 0 {
		valueWritten(ctx, key)
	}
	for _, d := range changes {
		r := rules[d.rule]
		for _, c := range d.upserts {
			upsertCompacted(ctx, c)
		}
		dest, err := getTimeSeries(ctx, r.Dest, errTSNotTSDB)
		if err != nil || dest == nil {
			continue
		}
		if d.to >= d.from && depth < 8 {
			deleteSamples(ctx, r.Dest, dest, d.from, d.to, depth+1)
		}
		if !d.reopenLatest {
			continue
		}
		reopened := int64(0)
		dest.Lock()
		if n := len(dest.Samples); n > 0 && dest.Samples[0].Timestamp < last.Timestamp {
			reopened = dest.Samples[n-1].Timestamp
			dest.DeleteRange(reopened, reopened)
		}
		dest.Unlock()
		valueWritten(ctx, r.Dest)
		ts.Lock()
		if d.rule < len(ts.Rules) && ts.Rules[d.rule].Dest == r.Dest {
			start := max(store.TSBucketStart(reopened, r.Bucket, r.Align), 0)
			openRuleBucket(ts, &ts.Rules[d.rule], start, start+r.Bucket-1)
		}
		ts.Unlock()
	}
	return deleted
}

// feedRules adds a sample to each compaction rule's open bucket; a sample
// in a later bucket closes the open one, whose aggregate goes to the rule's
// destination.
func feedRules(ts *store.TimeSeriesValue, at int64, value float64) []tsClosedBucket {
	var closed []tsClosedBucket
	for i := range ts.Rules {
		r := &ts.Rules[i]
		start := store.TSBucketStart(at, r.Bucket, r.Align)
		if len(r.Open) > 0 {
			open := store.TSBucketStart(r.Open[0].Timestamp, r.Bucket, r.Align)
			switch {
			case start < open:
				continue
			case start > open:
				var v float64
				if r.Aggregation == "twa" {
					next := store.TimeSeriesSample{Timestamp: at, Value: value}
					v = twaBucket(r.Open, r.Prev, &next, max(open, 0), open+r.Bucket, false)
				} else {
					v = store.TSAggregate(r.Aggregation, r.Open)
				}
				closed = append(closed, tsClosedBucket{r.Dest, max(open, 0), v})
				last := r.Open[len(r.Open)-1]
				r.Prev = &last
				r.Open = r.Open[:0]
			}
		}
		r.Open = append(r.Open, store.TimeSeriesSample{Timestamp: at, Value: value})
	}
	return closed
}

func cmdTSADD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	at, err := parseTSTimestamp(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	value, ok := parseTSValue(ctx.ArgString(2))
	if !ok {
		return ctx.WriteError(errTSValue)
	}
	o, err := parseTSOptions(ctx, 3)
	if err != nil {
		return ctx.WriteError(err)
	}
	ts, err := getTimeSeries(ctx, key, errTSNotTSDB)
	if err != nil {
		return ctx.WriteError(err)
	}
	if ts == nil {
		if ts, err = createSeries(ctx, key, o); err != nil {
			return ctx.WriteError(err)
		}
	}
	reply, err := addSample(ctx, key, ts, at, value, o.onDuplicate)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(reply)
}

func cmdTSMADD(ctx *Context) error {
	if ctx.ArgCount() < 3 || ctx.ArgCount()%3 != 0 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	replies := make([]*resp.Value, 0, ctx.ArgCount()/3)
	for i := 0; i < ctx.ArgCount(); i += 3 {
		key := ctx.ArgString(i)
		at, err := parseTSTimestamp(ctx.ArgString(i + 1))
		if err != nil {
			replies = append(replies, resp.ErrorValue(err.Error()))
			continue
		}
		value, ok := parseTSValue(ctx.ArgString(i + 2))
		if !ok {
			replies = append(replies, resp.ErrorValue(errTSValue.Error()))
			continue
		}
		ts, err := getTimeSeries(ctx, key, errTSNotTSDB)
		if err == nil && ts == nil {
			err = errTSNotTSDB
		}
		if err == nil {
			var reply int64
			if reply, err = addSample(ctx, key, ts, at, value, ""); err == nil {
				replies = append(replies, resp.IntegerValue(reply))
				continue
			}
		}
		replies = append(replies, resp.ErrorValue(err.Error()))
	}
	return ctx.WriteArray(replies)
}

func tsIncrBy(ctx *Context, sign float64) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	delta, ok := parseTSValue(ctx.ArgString(1))
	if !ok {
		return ctx.WriteError(errTSIncrValue)
	}
	at := time.Now().UnixMilli()
	for i := 2; i+1 < ctx.ArgCount(); i++ {
		if strings.EqualFold(ctx.ArgString(i), "TIMESTAMP") {
			var err error
			if at, err = parseTSTimestamp(ctx.ArgString(i + 1)); err != nil {
				return ctx.WriteError(err)
			}
			break
		}
	}
	o, err := parseTSOptions(ctx, 2)
	if err != nil {
		return ctx.WriteError(err)
	}
	ts, err := getTimeSeries(ctx, key, errTSNotTSDB)
	if err != nil {
		return ctx.WriteError(err)
	}
	if ts == nil {
		if ts, err = createSeries(ctx, key, o); err != nil {
			return ctx.WriteError(err)
		}
	}
	ts.RLock()
	last, hasLast := ts.Last()
	ts.RUnlock()
	value := sign * delta
	if hasLast {
		if at < last.Timestamp {
			return ctx.WriteError(errTSIncrOlder)
		}
		value += last.Value
	}
	reply, err := addSample(ctx, key, ts, at, value, "LAST")
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(reply)
}

func cmdTSINCRBY(ctx *Context) error { return tsIncrBy(ctx, 1) }
func cmdTSDECRBY(ctx *Context) error { return tsIncrBy(ctx, -1) }

// parseTSBound reads a range end: - and + are the extremes.
func parseTSBound(s string, err error) (int64, error) {
	switch s {
	case "-":
		return 0, nil
	case "+":
		return math.MaxInt64, nil
	}
	v, perr := strconv.ParseInt(s, 10, 64)
	if perr != nil || v < 0 {
		return 0, err
	}
	return v, nil
}

func cmdTSDEL(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	from, err := parseTSBound(ctx.ArgString(1), errTSFrom)
	if err != nil {
		return ctx.WriteError(err)
	}
	to, err := parseTSBound(ctx.ArgString(2), errTSTo)
	if err != nil {
		return ctx.WriteError(err)
	}
	key := ctx.ArgString(0)
	ts, err := getTimeSeries(ctx, key, errTSWrongType)
	if err != nil {
		return ctx.WriteError(err)
	}
	if ts == nil {
		return ctx.WriteError(errTSKeyMissing)
	}
	return ctx.WriteInteger(int64(deleteSamples(ctx, key, ts, from, to, 0)))
}

// tsDouble is a sample value as RedisTimeSeries replies with it: a simple
// string for RESP2 and a double for RESP3.
func tsDouble(resp3 bool, f float64) *resp.Value {
	if resp3 {
		return resp.DoubleValue(fpconvDtoa(f))
	}
	return resp.SimpleString(store.FormatTSValue(f))
}

func tsSampleReply(resp3 bool, s store.TimeSeriesSample) *resp.Value {
	return resp.ArrayValue([]*resp.Value{resp.IntegerValue(s.Timestamp), tsDouble(resp3, s.Value)})
}

func tsSamplesReply(resp3 bool, samples []store.TimeSeriesSample) *resp.Value {
	out := make([]*resp.Value, len(samples))
	for i, s := range samples {
		out[i] = tsSampleReply(resp3, s)
	}
	return resp.ArrayValue(out)
}

func tsLastReply(resp3 bool, ts *store.TimeSeriesValue) *resp.Value {
	ts.RLock()
	defer ts.RUnlock()
	if last, ok := ts.Last(); ok {
		return tsSampleReply(resp3, last)
	}
	return resp.ArrayValue([]*resp.Value{})
}

// tsPairs is a list of label pairs: a map for RESP3 and an array of
// two-element arrays for RESP2.
func tsPairs(resp3 bool, pairs [][2]*resp.Value) *resp.Value {
	if resp3 {
		flat := make([]*resp.Value, 0, 2*len(pairs))
		for _, p := range pairs {
			flat = append(flat, p[0], p[1])
		}
		return resp.PairsValue(flat)
	}
	out := make([]*resp.Value, len(pairs))
	for i, p := range pairs {
		out[i] = resp.ArrayValue(p[:])
	}
	return resp.ArrayValue(out)
}

// tsMultiReply is the reply of TS.MGET and TS.MRANGE: a map from key to its
// entry for RESP3 and an array of [key, entry...] for RESP2.
func tsMultiReply(ctx *Context, keys []string, entries [][]*resp.Value) error {
	if ctx.Writer.IsRESP3() {
		flat := make([]*resp.Value, 0, 2*len(keys))
		for i, k := range keys {
			flat = append(flat, resp.BulkString(k), resp.ArrayValue(entries[i]))
		}
		return ctx.WriteValue(resp.PairsValue(flat))
	}
	out := make([]*resp.Value, len(keys))
	for i, k := range keys {
		out[i] = resp.ArrayValue(append([]*resp.Value{resp.BulkString(k)}, entries[i]...))
	}
	return ctx.WriteArray(out)
}

func cmdTSGET(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	ts, err := getTimeSeries(ctx, ctx.ArgString(0), errTSWrongType)
	if err != nil {
		return ctx.WriteError(err)
	}
	if ts == nil {
		return ctx.WriteError(errTSKeyMissing)
	}
	return ctx.WriteValue(tsLastReply(ctx.Writer.IsRESP3(), ts))
}

// tsRangeQuery is a TS.RANGE or TS.MRANGE query.
type tsRangeQuery struct {
	from, to       int64
	filterTS       []int64
	filterMin      *float64
	filterMax      float64
	count          int
	agg            string
	bucket         int64
	align          int64
	bucketTS       string
	empty          bool
	withLabels     bool
	selectedLabels []string
	filters        []tsFilter
	groupBy        string
	reducer        string
}

// parseTSRange reads a range query whose bounds are args[at] and
// args[at+1]; multi allows the TS.MRANGE options.
func parseTSRange(ctx *Context, at int, multi bool) (*tsRangeQuery, error) {
	q := &tsRangeQuery{to: math.MaxInt64, bucketTS: "start"}
	var err error
	fromText, toText := ctx.ArgString(at), ctx.ArgString(at+1)
	if q.from, err = parseTSBound(fromText, errTSFrom); err != nil {
		return nil, err
	}
	if q.to, err = parseTSBound(toText, errTSTo); err != nil {
		return nil, err
	}
	n := ctx.ArgCount()
	if multi {
		if n, err = parseTSMultiTail(ctx, at+2, q); err != nil {
			return nil, err
		}
	}
	alignText := ""
	for i := at + 2; i < n; i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "FILTER_BY_TS":
			for i+1 < n {
				v, err := strconv.ParseInt(ctx.ArgString(i+1), 10, 64)
				if err != nil {
					break
				}
				q.filterTS = append(q.filterTS, v)
				i++
			}
			if len(q.filterTS) == 0 {
				return nil, errTSFilterByTS
			}
		case "FILTER_BY_VALUE":
			if i+2 >= n {
				return nil, errTSFilterMin
			}
			lo, err := strconv.ParseFloat(ctx.ArgString(i+1), 64)
			if err != nil {
				return nil, errTSFilterMin
			}
			hi, err := strconv.ParseFloat(ctx.ArgString(i+2), 64)
			if err != nil {
				return nil, errTSFilterMax
			}
			q.filterMin, q.filterMax = &lo, hi
			i += 2
		case "COUNT":
			if i+1 >= n {
				return nil, errTSCount
			}
			c, err := strconv.Atoi(ctx.ArgString(i + 1))
			if err != nil || c <= 0 {
				return nil, errTSCount
			}
			q.count = c
			i++
		case "ALIGN":
			if i+1 >= n {
				return nil, errTSAlign
			}
			alignText = ctx.ArgString(i + 1)
			i++
		case "AGGREGATION":
			if i+2 >= n {
				return nil, arityError(ctx.Command)
			}
			agg, ok := store.ParseTSAggregation(ctx.ArgString(i + 1))
			if !ok {
				return nil, errTSAggregation
			}
			b, err := strconv.ParseInt(ctx.ArgString(i+2), 10, 64)
			if err != nil || b <= 0 {
				return nil, errTSBucket
			}
			q.agg, q.bucket = agg, b
			i += 2
		case "BUCKETTIMESTAMP":
			if i+1 >= n {
				return nil, errTSBucketTimestamp
			}
			switch strings.ToLower(ctx.ArgString(i + 1)) {
			case "-", "start":
				q.bucketTS = "start"
			case "+", "end":
				q.bucketTS = "end"
			case "~", "mid":
				q.bucketTS = "mid"
			default:
				return nil, errTSBucketTimestamp
			}
			i++
		case "EMPTY":
			q.empty = true
		case "WITHLABELS":
			q.withLabels = multi
		case "SELECTED_LABELS":
			for i+1 < n && !tsMultiKeyword(ctx.ArgString(i+1)) {
				if multi {
					q.selectedLabels = append(q.selectedLabels, ctx.ArgString(i+1))
				}
				i++
			}
		case "FILTER":
			for i+1 < n && !tsMultiKeyword(ctx.ArgString(i+1)) {
				f, ok := parseTSFilter(ctx.ArgString(i + 1))
				if !ok {
					return nil, errTSFilter
				}
				if multi {
					q.filters = append(q.filters, f)
				}
				i++
			}
		case "GROUPBY":
			if !multi || i+3 >= n || !strings.EqualFold(ctx.ArgString(i+2), "REDUCE") {
				return nil, errTSGroupByReducer
			}
			q.groupBy = ctx.ArgString(i + 1)
			q.reducer = strings.ToLower(ctx.ArgString(i + 3))
			if _, ok := store.ParseTSAggregation(q.reducer); !ok || q.reducer == "first" || q.reducer == "last" || q.reducer == "twa" {
				return nil, errTSGroupByReducer
			}
			i += 3
		}
	}
	if q.withLabels && len(q.selectedLabels) > 0 {
		return nil, errTSSelectedWithBoth
	}
	if alignText != "" {
		switch strings.ToLower(alignText) {
		case "-", "start":
			if fromText == "-" {
				return nil, errTSAlignStart
			}
			q.align = q.from
		case "+", "end":
			if toText == "+" {
				return nil, errTSAlignEnd
			}
			q.align = q.to
		default:
			v, err := strconv.ParseInt(alignText, 10, 64)
			if err != nil {
				return nil, errTSAlign
			}
			q.align = v
		}
	}
	return q, nil
}

// parseTSMultiTail reads the FILTER and GROUPBY tail of TS.MRANGE, which
// RedisTimeSeries requires last, and returns where the other options end.
func parseTSMultiTail(ctx *Context, from int, q *tsRangeQuery) (int, error) {
	n := ctx.ArgCount()
	filterAt := -1
	for i := from; i < n && filterAt < 0; i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "FILTER":
			filterAt = i
		case "GROUPBY":
			for j := i + 1; j < n; j++ {
				if strings.EqualFold(ctx.ArgString(j), "FILTER") {
					return 0, errTSGroupByOrder
				}
			}
		}
	}
	if filterAt < 0 {
		return 0, errTSMissingFilter
	}
	end := n
	for i := filterAt + 1; i < n; i++ {
		if strings.EqualFold(ctx.ArgString(i), "GROUPBY") {
			end = i
			break
		}
	}
	if end == filterAt+1 {
		return 0, errTSFilterNoLabels
	}
	for i := filterAt + 1; i < end; i++ {
		f, ok := parseTSFilter(ctx.ArgString(i))
		if !ok {
			return 0, errTSFilter
		}
		q.filters = append(q.filters, f)
	}
	if end < n {
		if end+4 != n {
			return 0, arityError(ctx.Command)
		}
		if !strings.EqualFold(ctx.ArgString(end+2), "REDUCE") {
			return 0, errTSGroupByReducer
		}
		q.groupBy = ctx.ArgString(end + 1)
		q.reducer = strings.ToLower(ctx.ArgString(end + 3))
		if _, ok := store.ParseTSAggregation(q.reducer); !ok || q.reducer == "first" || q.reducer == "last" || q.reducer == "twa" {
			return 0, errTSGroupByReducer
		}
	}
	return filterAt, nil
}

func tsMultiKeyword(s string) bool {
	switch strings.ToUpper(s) {
	case "FILTER", "GROUPBY", "WITHLABELS", "SELECTED_LABELS", "COUNT", "AGGREGATION", "ALIGN", "FILTER_BY_TS", "FILTER_BY_VALUE", "LATEST", "EMPTY", "BUCKETTIMESTAMP":
		return true
	}
	return false
}

// run answers the query for one series, newest first when rev.
func (q *tsRangeQuery) run(ts *store.TimeSeriesValue, rev bool) []store.TimeSeriesSample {
	ts.RLock()
	defer ts.RUnlock()
	samples := ts.Range(q.from, q.to)
	if len(q.filterTS) > 0 || q.filterMin != nil {
		kept := samples[:0]
		for _, s := range samples {
			if len(q.filterTS) > 0 && !containsInt64(q.filterTS, s.Timestamp) {
				continue
			}
			if q.filterMin != nil && (s.Value < *q.filterMin || s.Value > q.filterMax) {
				continue
			}
			kept = append(kept, s)
		}
		samples = kept
	}
	switch q.agg {
	case "":
	case "twa":
		samples = q.aggregateTWA(ts.Samples, samples, rev)
	default:
		samples = q.aggregate(samples, rev)
	}
	if rev {
		for i, j := 0, len(samples)-1; i < j; i, j = i+1, j-1 {
			samples[i], samples[j] = samples[j], samples[i]
		}
	}
	if q.count > 0 && len(samples) > q.count {
		samples = samples[:q.count]
	}
	return samples
}

func containsInt64(list []int64, v int64) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (q *tsRangeQuery) bucketShift() int64 {
	switch q.bucketTS {
	case "end":
		return q.bucket
	case "mid":
		return q.bucket / 2
	}
	return 0
}

// aggregate buckets the samples. A reverse query feeds each bucket newest
// first, as RedisTimeSeries does: sums accumulate in that order and "last"
// is the oldest sample. With EMPTY, a gap gets the aggregation's empty
// value, which for "last" is the previous bucket's value. A bucket that
// ALIGN starts before 0 is reported at 0.
func (q *tsRangeQuery) aggregate(samples []store.TimeSeriesSample, rev bool) []store.TimeSeriesSample {
	shift := q.bucketShift()
	var out []store.TimeSeriesSample
	var bucket []store.TimeSeriesSample
	next := int64(math.MinInt64)
	for i := 0; i < len(samples); {
		start := store.TSBucketStart(samples[i].Timestamp, q.bucket, q.align)
		j := i
		for j < len(samples) && samples[j].Timestamp < start+q.bucket {
			j++
		}
		if q.empty && next != math.MinInt64 {
			empty := store.TSAggregate(q.agg, nil)
			if q.agg == "last" && len(out) > 0 {
				empty = out[len(out)-1].Value
			}
			for gap := next; gap < start; gap += q.bucket {
				out = append(out, store.TimeSeriesSample{Timestamp: max(gap, 0) + shift, Value: empty})
			}
		}
		bucket = samples[i:j]
		if rev {
			bucket = append(bucket[:0:0], bucket...)
			reverseSamples(bucket)
		}
		out = append(out, store.TimeSeriesSample{Timestamp: max(start, 0) + shift, Value: store.TSAggregate(q.agg, bucket)})
		next = start + q.bucket
		i = j
	}
	return out
}

// tsBefore returns the last of the sorted samples older than t.
func tsBefore(all []store.TimeSeriesSample, t int64) (store.TimeSeriesSample, bool) {
	i := sort.Search(len(all), func(i int) bool { return all[i].Timestamp >= t })
	if i == 0 {
		return store.TimeSeriesSample{}, false
	}
	return all[i-1], true
}

// tsFrom returns the first of the sorted samples at t or later.
func tsFrom(all []store.TimeSeriesSample, t int64) (store.TimeSeriesSample, bool) {
	i := sort.Search(len(all), func(i int) bool { return all[i].Timestamp >= t })
	if i == len(all) {
		return store.TimeSeriesSample{}, false
	}
	return all[i], true
}

// aggregateTWA is RedisTimeSeries' time-weighted average: each bucket's
// samples are extended to the bucket edges by interpolating with the
// neighbouring samples, which may lie outside the query range. With EMPTY,
// a bucket with no samples gets the mean of the line between the samples
// around it, when there are samples on both sides. Unlike RedisTimeSeries,
// which fills buckets up to the end of an open range until it runs out of
// memory, filling stops at the series' last sample.
func (q *tsRangeQuery) aggregateTWA(all, samples []store.TimeSeriesSample, rev bool) []store.TimeSeriesSample {
	shift := q.bucketShift()
	edges := func(start int64) (int64, int64) {
		return max(start, 0, q.from), min(start+q.bucket, q.to)
	}
	var out []store.TimeSeriesSample
	limit := q.to
	if len(all) > 0 && limit == math.MaxInt64 {
		limit = all[len(all)-1].Timestamp
	}
	fill := func(first, last int64) {
		last = min(last, store.TSBucketStart(limit, q.bucket, q.align))
		ta, _ := edges(first)
		before, ok := tsBefore(all, ta)
		after, ok2 := tsFrom(all, ta)
		if !ok || !ok2 {
			return
		}
		dv, dt := after.Value-before.Value, float64(after.Timestamp-before.Timestamp)
		for b := first; b <= last; b += q.bucket {
			ta, tb := edges(b)
			va := before.Value + float64(ta-before.Timestamp)*dv/dt
			vb := before.Value + float64(tb-before.Timestamp)*dv/dt
			out = append(out, store.TimeSeriesSample{Timestamp: max(b, 0) + shift, Value: (va + vb) / 2.0})
		}
	}
	if len(samples) == 0 {
		if q.empty {
			fill(store.TSBucketStart(q.from, q.bucket, q.align), store.TSBucketStart(q.to, q.bucket, q.align))
		}
		return out
	}
	var prev *store.TimeSeriesSample
	if p, ok := tsBefore(all, samples[0].Timestamp); ok {
		prev = &p
	}
	next := store.TSBucketStart(q.from, q.bucket, q.align)
	for i := 0; i < len(samples); {
		start := store.TSBucketStart(samples[i].Timestamp, q.bucket, q.align)
		j := i
		for j < len(samples) && samples[j].Timestamp < start+q.bucket {
			j++
		}
		if q.empty && next < start {
			fill(next, start-q.bucket)
		}
		var after *store.TimeSeriesSample
		if j < len(samples) {
			after = &samples[j]
		} else if n, ok := tsFrom(all, samples[j-1].Timestamp+1); ok {
			after = &n
		}
		ta, tb := edges(start)
		out = append(out, store.TimeSeriesSample{Timestamp: max(start, 0) + shift, Value: twaBucket(samples[i:j], prev, after, ta, tb, rev)})
		prev = &samples[j-1]
		next = start + q.bucket
		i = j
	}
	if q.empty {
		// RedisTimeSeries resumes after the last bucket's reported
		// timestamp, so a BUCKETTIMESTAMP other than start skips buckets.
		next = store.TSBucketStart(out[len(out)-1].Timestamp, q.bucket, q.align) + q.bucket
		if last := store.TSBucketStart(q.to, q.bucket, q.align); next <= last {
			fill(next, last)
		}
	}
	return out
}

// twaBucket is the time-weighted average of in over [ta, tb]. Its terms are
// summed in the order RedisTimeSeries iterates, newest first when rev, so
// the result matches it to the last bit.
func twaBucket(in []store.TimeSeriesSample, prev, next *store.TimeSeriesSample, ta, tb int64, rev bool) float64 {
	terms := make([]float64, 0, len(in)+1)
	first, last := in[0].Timestamp, in[len(in)-1].Timestamp
	if prev != nil {
		v1, v2, t1, t2 := prev.Value, in[0].Value, prev.Timestamp, in[0].Timestamp
		vab := v1 + float64(ta-t1)*(v2-v1)/float64(t2-t1)
		terms = append(terms, (vab+v2)*float64(t2-ta)/2.0)
		first = ta
	}
	for i := 1; i < len(in); i++ {
		terms = append(terms, (in[i-1].Value+in[i].Value)*float64(in[i].Timestamp-in[i-1].Timestamp)/2.0)
	}
	if next != nil {
		v1, v2, t1, t2 := in[len(in)-1].Value, next.Value, in[len(in)-1].Timestamp, next.Timestamp
		vab := v1 + float64(tb-t1)*(v2-v1)/float64(t2-t1)
		terms = append(terms, (vab+v1)*float64(tb-t1)/2.0)
		last = tb
	}
	if first == last {
		if rev {
			return in[0].Value
		}
		return in[len(in)-1].Value
	}
	res := 0.0
	for i := range terms {
		if rev {
			res += terms[len(terms)-1-i]
		} else {
			res += terms[i]
		}
	}
	span := last - first
	if span < 0 {
		span = -span
	}
	return res / float64(span)
}

func tsRange(ctx *Context, rev bool) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	q, err := parseTSRange(ctx, 1, false)
	if err != nil {
		return ctx.WriteError(err)
	}
	ts, err := getTimeSeries(ctx, ctx.ArgString(0), errTSWrongType)
	if err != nil {
		return ctx.WriteError(err)
	}
	if ts == nil {
		return ctx.WriteError(errTSKeyMissing)
	}
	return ctx.WriteValue(tsSamplesReply(ctx.Writer.IsRESP3(), q.run(ts, rev)))
}

func cmdTSRANGE(ctx *Context) error    { return tsRange(ctx, false) }
func cmdTSREVRANGE(ctx *Context) error { return tsRange(ctx, true) }

// matchingSeries returns the series of the database whose labels pass the
// filters, by key name.
func matchingSeries(ctx *Context, filters []tsFilter) ([]string, map[string]*store.TimeSeriesValue, error) {
	positive := false
	for _, f := range filters {
		positive = positive || (!f.negate && len(f.values) > 0)
	}
	if !positive {
		return nil, nil, errTSMatcher
	}
	series := make(map[string]*store.TimeSeriesValue)
	var keys []string
	err := ctx.Store.ForEachLive(func(key string, entry *store.Entry) error {
		ts, ok := entry.Value.(*store.TimeSeriesValue)
		if !ok {
			return nil
		}
		ts.RLock()
		matched := true
		for _, f := range filters {
			matched = matched && f.matches(ts.Labels)
		}
		ts.RUnlock()
		if matched {
			keys = append(keys, key)
			series[key] = ts
		}
		return nil
	})
	sort.Strings(keys)
	return keys, series, err
}

// labelsReply is the labels part of TS.MGET and TS.MRANGE.
func labelsReply(resp3 bool, ts *store.TimeSeriesValue, all bool, selected []string) *resp.Value {
	ts.RLock()
	defer ts.RUnlock()
	return selectLabels(resp3, ts.LabelList(), all, selected)
}

func selectLabels(resp3 bool, labels [][2]string, all bool, selected []string) *resp.Value {
	var out [][2]*resp.Value
	switch {
	case all:
		for _, kv := range labels {
			out = append(out, [2]*resp.Value{resp.BulkString(kv[0]), resp.BulkString(kv[1])})
		}
	case len(selected) > 0:
		for _, k := range selected {
			value := resp.NullBulkString()
			if resp3 {
				value = resp.NullValue()
			}
			for _, kv := range labels {
				if kv[0] == k {
					value = resp.BulkString(kv[1])
					break
				}
			}
			out = append(out, [2]*resp.Value{resp.BulkString(k), value})
		}
	}
	return tsPairs(resp3, out)
}

func cmdTSMGET(ctx *Context) error {
	var withLabels bool
	var selected []string
	filterAt := -1
	for i := 0; i < ctx.ArgCount() && filterAt < 0; i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "WITHLABELS":
			withLabels = true
		case "SELECTED_LABELS":
			for i+1 < ctx.ArgCount() && !tsMultiKeyword(ctx.ArgString(i+1)) {
				selected = append(selected, ctx.ArgString(i+1))
				i++
			}
		case "FILTER":
			filterAt = i
		}
	}
	if filterAt < 0 || filterAt+1 >= ctx.ArgCount() {
		return ctx.WriteError(arityError(ctx.Command))
	}
	if withLabels && len(selected) > 0 {
		return ctx.WriteError(errTSSelectedWithBoth)
	}
	var filters []tsFilter
	for i := filterAt + 1; i < ctx.ArgCount(); i++ {
		f, ok := parseTSFilter(ctx.ArgString(i))
		if !ok {
			return ctx.WriteError(errTSFilter)
		}
		filters = append(filters, f)
	}
	keys, series, err := matchingSeries(ctx, filters)
	if err != nil {
		return ctx.WriteError(err)
	}
	resp3 := ctx.Writer.IsRESP3()
	entries := make([][]*resp.Value, len(keys))
	for i, k := range keys {
		ts := series[k]
		entries[i] = []*resp.Value{labelsReply(resp3, ts, withLabels, selected), tsLastReply(resp3, ts)}
	}
	return tsMultiReply(ctx, keys, entries)
}

func reverseSamples(s []store.TimeSeriesSample) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func tsMRange(ctx *Context, rev bool) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	q, err := parseTSRange(ctx, 0, true)
	if err != nil {
		return ctx.WriteError(err)
	}
	if len(q.filters) == 0 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	keys, series, err := matchingSeries(ctx, q.filters)
	if err != nil {
		return ctx.WriteError(err)
	}
	resp3 := ctx.Writer.IsRESP3()
	if q.groupBy == "" {
		var aggregators []*resp.Value
		if q.agg != "" {
			aggregators = []*resp.Value{resp.BulkString(q.agg)}
		}
		entries := make([][]*resp.Value, len(keys))
		for i, k := range keys {
			ts := series[k]
			entries[i] = []*resp.Value{labelsReply(resp3, ts, q.withLabels, q.selectedLabels)}
			if resp3 {
				entries[i] = append(entries[i], resp.PairsValue([]*resp.Value{resp.BulkString("aggregators"), resp.ArrayValue(aggregators)}))
			}
			entries[i] = append(entries[i], tsSamplesReply(resp3, q.run(ts, rev)))
		}
		return tsMultiReply(ctx, keys, entries)
	}
	groups := make(map[string][]string)
	var values []string
	for _, k := range keys {
		ts := series[k]
		ts.RLock()
		v, ok := ts.Labels[q.groupBy]
		ts.RUnlock()
		if !ok {
			continue
		}
		if _, seen := groups[v]; !seen {
			values = append(values, v)
		}
		groups[v] = append(groups[v], k)
	}
	sort.Strings(values)
	names := make([]string, 0, len(values))
	entries := make([][]*resp.Value, 0, len(values))
	for _, v := range values {
		byTS := make(map[int64][]store.TimeSeriesSample)
		var stamps []int64
		for _, k := range groups[v] {
			for _, s := range q.run(series[k], false) {
				if _, seen := byTS[s.Timestamp]; !seen {
					stamps = append(stamps, s.Timestamp)
				}
				byTS[s.Timestamp] = append(byTS[s.Timestamp], s)
			}
		}
		sort.Slice(stamps, func(i, j int) bool { return stamps[i] < stamps[j] })
		reduced := make([]store.TimeSeriesSample, 0, len(stamps))
		for _, at := range stamps {
			reduced = append(reduced, store.TimeSeriesSample{Timestamp: at, Value: store.TSAggregate(q.reducer, byTS[at])})
		}
		if rev {
			reverseSamples(reduced)
		}
		if q.count > 0 && len(reduced) > q.count {
			reduced = reduced[:q.count]
		}
		groupLabels := [][2]string{{q.groupBy, v}, {"__reducer__", q.reducer}, {"__source__", strings.Join(groups[v], ",")}}
		names = append(names, q.groupBy+"="+v)
		if !resp3 {
			entries = append(entries, []*resp.Value{selectLabels(false, groupLabels, q.withLabels, q.selectedLabels), tsSamplesReply(false, reduced)})
			continue
		}
		labels := selectLabels(true, groupLabels[:1], q.withLabels, nil)
		if len(q.selectedLabels) > 0 {
			labels = selectLabels(true, groupLabels, false, q.selectedLabels)
		}
		sources := make([]*resp.Value, len(groups[v]))
		for i, k := range groups[v] {
			sources[i] = resp.BulkString(k)
		}
		entries = append(entries, []*resp.Value{labels,
			resp.PairsValue([]*resp.Value{resp.BulkString("reducers"), resp.ArrayValue([]*resp.Value{resp.BulkString(q.reducer)})}),
			resp.PairsValue([]*resp.Value{resp.BulkString("sources"), resp.ArrayValue(sources)}),
			tsSamplesReply(true, reduced)})
	}
	return tsMultiReply(ctx, names, entries)
}

func cmdTSMRANGE(ctx *Context) error    { return tsMRange(ctx, false) }
func cmdTSMREVRANGE(ctx *Context) error { return tsMRange(ctx, true) }

func cmdTSINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 || ctx.ArgCount() > 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	key := ctx.ArgString(0)
	ts, err := getTimeSeries(ctx, key, errTSWrongType)
	if err != nil {
		return ctx.WriteError(err)
	}
	if ts == nil {
		return ctx.WriteError(errTSKeyMissing)
	}
	ts.RLock()
	defer ts.RUnlock()
	info := ts.Info()
	chunkType := "compressed"
	if ts.Uncompressed {
		chunkType = "uncompressed"
	}
	resp3 := ctx.Writer.IsRESP3()
	source := resp.NullBulkString()
	if ts.SourceKey != "" {
		source = resp.BulkString(ts.SourceKey)
	}
	rules := []*resp.Value{}
	for _, r := range ts.Rules {
		rule := []*resp.Value{resp.IntegerValue(r.Bucket), resp.SimpleString(strings.ToUpper(r.Aggregation)), resp.IntegerValue(r.Align)}
		if resp3 {
			rules = append(rules, resp.BulkString(r.Dest), resp.ArrayValue(rule))
		} else {
			rules = append(rules, resp.ArrayValue(append([]*resp.Value{resp.BulkString(r.Dest)}, rule...)))
		}
	}
	rulesReply := resp.ArrayValue(rules)
	if resp3 {
		rulesReply = resp.PairsValue(rules)
	}
	ignoreValDiff := resp.BulkString(strconv.FormatFloat(ts.IgnoreMaxValDiff, 'f', -1, 64))
	if resp3 {
		ignoreValDiff = resp.DoubleValue(fpconvDtoa(ts.IgnoreMaxValDiff))
	}
	reply := []*resp.Value{
		resp.SimpleString("totalSamples"), resp.IntegerValue(info.Samples),
		resp.SimpleString("memoryUsage"), resp.IntegerValue(info.Memory),
		resp.SimpleString("firstTimestamp"), resp.IntegerValue(info.First),
		resp.SimpleString("lastTimestamp"), resp.IntegerValue(info.Last),
		resp.SimpleString("retentionTime"), resp.IntegerValue(ts.Retention.Milliseconds()),
		resp.SimpleString("chunkCount"), resp.IntegerValue(info.ChunkCount),
		resp.SimpleString("chunkSize"), resp.IntegerValue(info.ChunkSize),
		resp.SimpleString("chunkType"), resp.SimpleString(chunkType),
		resp.SimpleString("duplicatePolicy"), resp.SimpleString(strings.ToLower(ts.Policy())),
		resp.SimpleString("labels"), selectLabels(resp3, ts.LabelList(), true, nil),
		resp.SimpleString("sourceKey"), source,
		resp.SimpleString("rules"), rulesReply,
		resp.SimpleString("ignoreMaxTimeDiff"), resp.IntegerValue(ts.IgnoreMaxTimeDiff),
		resp.SimpleString("ignoreMaxValDiff"), ignoreValDiff,
	}
	if ctx.ArgCount() == 2 && strings.EqualFold(ctx.ArgString(1), "DEBUG") {
		start, end := int64(-1), int64(-1)
		if info.Samples > 0 {
			start, end = info.First, info.Last
		}
		size := info.ChunkSize * info.ChunkCount
		perSample := float64(size) / math.Max(float64(info.Samples), 1)
		perSampleReply := resp.BulkString(strconv.FormatFloat(perSample, 'f', -1, 64))
		if resp3 {
			perSampleReply = resp.DoubleValue(fpconvDtoa(perSample))
		}
		chunk := []*resp.Value{
			resp.SimpleString("startTimestamp"), resp.IntegerValue(start),
			resp.SimpleString("endTimestamp"), resp.IntegerValue(end),
			resp.SimpleString("samples"), resp.IntegerValue(info.Samples),
			resp.SimpleString("size"), resp.IntegerValue(size),
			resp.SimpleString("bytesPerSample"), perSampleReply,
		}
		reply = append(reply, resp.SimpleString("keySelfName"), resp.BulkString(key),
			resp.SimpleString("Chunks"), resp.ArrayValue([]*resp.Value{resp.PairsValue(chunk)}))
	}
	return ctx.WriteValue(resp.PairsValue(reply))
}

func cmdTSCREATERULE(ctx *Context) error {
	if ctx.ArgCount() != 5 && ctx.ArgCount() != 6 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	if !strings.EqualFold(ctx.ArgString(2), "AGGREGATION") {
		return ctx.WriteError(ErrSyntaxError)
	}
	agg, ok := store.ParseTSAggregation(ctx.ArgString(3))
	if !ok {
		return ctx.WriteError(errTSAggregation)
	}
	bucket, err := strconv.ParseInt(ctx.ArgString(4), 10, 64)
	if err != nil || bucket <= 0 {
		return ctx.WriteError(errTSBucket)
	}
	var align int64
	if ctx.ArgCount() == 6 {
		if align, err = strconv.ParseInt(ctx.ArgString(5), 10, 64); err != nil {
			return ctx.WriteError(errTSAlign)
		}
	}
	srcKey, destKey := ctx.ArgString(0), ctx.ArgString(1)
	if srcKey == destKey {
		return ctx.WriteError(errTSSameKey)
	}
	src, err := getTimeSeries(ctx, srcKey, errTSWrongType)
	if err == nil && src == nil {
		err = errTSKeyMissing
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	dest, err := getTimeSeries(ctx, destKey, errTSWrongType)
	if err == nil && dest == nil {
		err = errTSKeyMissing
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	src.RLock()
	srcHasSource := src.SourceKey != ""
	src.RUnlock()
	if srcHasSource {
		return ctx.WriteError(errTSSrcHasSource)
	}
	dest.Lock()
	if dest.SourceKey != "" || len(dest.Rules) > 0 {
		err := errTSDestHasSource
		if dest.SourceKey == "" {
			err = errTSDestHasDest
		}
		dest.Unlock()
		return ctx.WriteError(err)
	}
	dest.SourceKey = srcKey
	dest.Unlock()
	src.Lock()
	src.Rules = append(src.Rules, store.TSRule{Dest: destKey, Aggregation: agg, Bucket: bucket, Align: align})
	src.Unlock()
	valueWritten(ctx, srcKey)
	valueWritten(ctx, destKey)
	return ctx.WriteOK()
}

func cmdTSDELETERULE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	srcKey, destKey := ctx.ArgString(0), ctx.ArgString(1)
	src, err := getTimeSeries(ctx, srcKey, errTSWrongType)
	if err != nil {
		return ctx.WriteError(err)
	}
	if src == nil {
		return ctx.WriteError(errTSKeyMissing)
	}
	src.Lock()
	found := false
	for i, r := range src.Rules {
		if r.Dest == destKey {
			src.Rules = append(src.Rules[:i], src.Rules[i+1:]...)
			found = true
			break
		}
	}
	src.Unlock()
	if !found {
		return ctx.WriteError(errTSRuleMissing)
	}
	if dest, err := getTimeSeries(ctx, destKey, errTSWrongType); err == nil && dest != nil {
		dest.Lock()
		dest.SourceKey = ""
		dest.Unlock()
		valueWritten(ctx, destKey)
	}
	valueWritten(ctx, srcKey)
	return ctx.WriteOK()
}

func cmdTSQUERYINDEX(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(arityError(ctx.Command))
	}
	filters := make([]tsFilter, 0, ctx.ArgCount())
	for i := 0; i < ctx.ArgCount(); i++ {
		f, ok := parseTSFilter(ctx.ArgString(i))
		if !ok {
			return ctx.WriteError(errTSFilter)
		}
		filters = append(filters, f)
	}
	keys, _, err := matchingSeries(ctx, filters)
	if err != nil {
		return ctx.WriteError(err)
	}
	results := make([]*resp.Value, 0, len(keys))
	for _, k := range keys {
		results = append(results, resp.BulkString(k))
	}
	return ctx.WriteValue(resp.SetValue(results))
}

// tsFilter is one RedisTimeSeries label filter: label=value, label!=value,
// label= (no such label), label!= (has the label), or a value list in
// parentheses.
type tsFilter struct {
	label  string
	values []string
	negate bool
}

func parseTSFilter(s string) (tsFilter, bool) {
	i := strings.IndexByte(s, '=')
	if i <= 0 {
		return tsFilter{}, false
	}
	f := tsFilter{label: s[:i]}
	if s[i-1] == '!' {
		f.label, f.negate = s[:i-1], true
		if f.label == "" {
			return tsFilter{}, false
		}
	}
	value := s[i+1:]
	switch {
	case value == "":
	case strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")"):
		for _, v := range strings.Split(value[1:len(value)-1], ",") {
			if v = strings.TrimSpace(v); v != "" {
				f.values = append(f.values, v)
			}
		}
	default:
		f.values = []string{value}
	}
	return f, true
}

func (f tsFilter) matches(labels map[string]string) bool {
	value, has := labels[f.label]
	if len(f.values) == 0 {
		return has == f.negate
	}
	in := false
	for _, v := range f.values {
		in = in || (has && v == value)
	}
	return in != f.negate
}
