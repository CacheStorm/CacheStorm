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
	reply, err := ts.Upsert(at, value, policy)
	var closed []tsClosedBucket
	if err == nil {
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
	return reply, nil
}

type tsClosedBucket struct {
	dest  string
	start int64
	value float64
}

// feedRules adds a sample to each compaction rule's open bucket; a sample
// in a later bucket closes the open one, whose aggregate goes to the rule's
// destination. Samples for buckets already closed are not compacted again.
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
				closed = append(closed, tsClosedBucket{r.Dest, open, store.TSAggregate(r.Aggregation, r.Open)})
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
	ts.Lock()
	deleted := ts.DeleteRange(from, to)
	ts.Unlock()
	if deleted > 0 {
		valueWritten(ctx, key)
	}
	return ctx.WriteInteger(int64(deleted))
}

func tsSampleReply(s store.TimeSeriesSample) *resp.Value {
	return resp.ArrayValue([]*resp.Value{resp.IntegerValue(s.Timestamp), resp.SimpleString(store.FormatTSValue(s.Value))})
}

func tsSamplesReply(samples []store.TimeSeriesSample) *resp.Value {
	out := make([]*resp.Value, len(samples))
	for i, s := range samples {
		out[i] = tsSampleReply(s)
	}
	return resp.ArrayValue(out)
}

func tsLastReply(ts *store.TimeSeriesValue) *resp.Value {
	ts.RLock()
	defer ts.RUnlock()
	if last, ok := ts.Last(); ok {
		return tsSampleReply(last)
	}
	return resp.ArrayValue([]*resp.Value{})
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
	return ctx.WriteValue(tsLastReply(ts))
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
	samples := ts.Range(q.from, q.to)
	ts.RUnlock()
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
	if q.agg != "" {
		samples = q.aggregate(samples)
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

func (q *tsRangeQuery) aggregate(samples []store.TimeSeriesSample) []store.TimeSeriesSample {
	shift := int64(0)
	switch q.bucketTS {
	case "end":
		shift = q.bucket
	case "mid":
		shift = q.bucket / 2
	}
	var out []store.TimeSeriesSample
	next := int64(math.MinInt64)
	for i := 0; i < len(samples); {
		start := store.TSBucketStart(samples[i].Timestamp, q.bucket, q.align)
		j := i
		for j < len(samples) && samples[j].Timestamp < start+q.bucket {
			j++
		}
		if q.empty && next != math.MinInt64 {
			for gap := next; gap < start; gap += q.bucket {
				out = append(out, store.TimeSeriesSample{Timestamp: gap + shift, Value: store.TSAggregate(q.agg, nil)})
			}
		}
		out = append(out, store.TimeSeriesSample{Timestamp: start + shift, Value: store.TSAggregate(q.agg, samples[i:j])})
		next = start + q.bucket
		i = j
	}
	return out
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
	return ctx.WriteValue(tsSamplesReply(q.run(ts, rev)))
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
func labelsReply(ts *store.TimeSeriesValue, all bool, selected []string) *resp.Value {
	ts.RLock()
	defer ts.RUnlock()
	out := []*resp.Value{}
	switch {
	case all:
		for _, kv := range ts.LabelList() {
			out = append(out, resp.ArrayValue([]*resp.Value{resp.BulkString(kv[0]), resp.BulkString(kv[1])}))
		}
	case len(selected) > 0:
		for _, k := range selected {
			value := resp.NullBulkString()
			if v, ok := ts.Labels[k]; ok {
				value = resp.BulkString(v)
			}
			out = append(out, resp.ArrayValue([]*resp.Value{resp.BulkString(k), value}))
		}
	}
	return resp.ArrayValue(out)
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
	out := make([]*resp.Value, 0, len(keys))
	for _, k := range keys {
		ts := series[k]
		out = append(out, resp.ArrayValue([]*resp.Value{resp.BulkString(k), labelsReply(ts, withLabels, selected), tsLastReply(ts)}))
	}
	return ctx.WriteArray(out)
}

func reverseSamples(s []store.TimeSeriesSample) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func tsMRange(ctx *Context, rev bool) error {
	if ctx.ArgCount() < 4 {
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
	if q.groupBy == "" {
		out := make([]*resp.Value, 0, len(keys))
		for _, k := range keys {
			ts := series[k]
			out = append(out, resp.ArrayValue([]*resp.Value{resp.BulkString(k), labelsReply(ts, q.withLabels, q.selectedLabels), tsSamplesReply(q.run(ts, rev))}))
		}
		return ctx.WriteArray(out)
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
	out := make([]*resp.Value, 0, len(values))
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
		labels := []*resp.Value{}
		if q.withLabels {
			labels = []*resp.Value{
				resp.ArrayValue([]*resp.Value{resp.BulkString(q.groupBy), resp.BulkString(v)}),
				resp.ArrayValue([]*resp.Value{resp.BulkString("__reducer__"), resp.BulkString(q.reducer)}),
				resp.ArrayValue([]*resp.Value{resp.BulkString("__source__"), resp.BulkString(strings.Join(groups[v], ","))}),
			}
		}
		out = append(out, resp.ArrayValue([]*resp.Value{resp.BulkString(q.groupBy + "=" + v), resp.ArrayValue(labels), tsSamplesReply(reduced)}))
	}
	return ctx.WriteArray(out)
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
	labels := []*resp.Value{}
	for _, kv := range ts.LabelList() {
		labels = append(labels, resp.ArrayValue([]*resp.Value{resp.BulkString(kv[0]), resp.BulkString(kv[1])}))
	}
	source := resp.NullBulkString()
	if ts.SourceKey != "" {
		source = resp.BulkString(ts.SourceKey)
	}
	rules := []*resp.Value{}
	for _, r := range ts.Rules {
		rules = append(rules, resp.ArrayValue([]*resp.Value{resp.BulkString(r.Dest), resp.IntegerValue(r.Bucket),
			resp.SimpleString(strings.ToUpper(r.Aggregation)), resp.IntegerValue(r.Align)}))
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
		resp.SimpleString("labels"), resp.ArrayValue(labels),
		resp.SimpleString("sourceKey"), source,
		resp.SimpleString("rules"), resp.ArrayValue(rules),
		resp.SimpleString("ignoreMaxTimeDiff"), resp.IntegerValue(ts.IgnoreMaxTimeDiff),
		resp.SimpleString("ignoreMaxValDiff"), resp.BulkString(strconv.FormatFloat(ts.IgnoreMaxValDiff, 'f', -1, 64)),
	}
	if ctx.ArgCount() == 2 && strings.EqualFold(ctx.ArgString(1), "DEBUG") {
		start, end := int64(-1), int64(-1)
		if info.Samples > 0 {
			start, end = info.First, info.Last
		}
		size := info.ChunkSize * info.ChunkCount
		reply = append(reply, resp.SimpleString("keySelfName"), resp.BulkString(key),
			resp.SimpleString("Chunks"), resp.ArrayValue([]*resp.Value{resp.ArrayValue([]*resp.Value{
				resp.SimpleString("startTimestamp"), resp.IntegerValue(start),
				resp.SimpleString("endTimestamp"), resp.IntegerValue(end),
				resp.SimpleString("samples"), resp.IntegerValue(info.Samples),
				resp.SimpleString("size"), resp.IntegerValue(size),
				resp.SimpleString("bytesPerSample"), resp.BulkString(strconv.FormatFloat(float64(size)/math.Max(float64(info.Samples), 1), 'f', -1, 64)),
			})}))
	}
	return ctx.WriteArray(reply)
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
	dest.Lock()
	if dest.SourceKey != "" {
		dest.Unlock()
		return ctx.WriteError(errTSDestHasSource)
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
	return ctx.WriteArray(results)
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
