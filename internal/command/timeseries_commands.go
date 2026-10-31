package command

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

var (
	errTSKeyExists  = errors.New("ERR TSDB: key already exists")
	errTSKeyMissing = errors.New("ERR TSDB: the key does not exist")
	errTSMatcher    = errors.New("ERR TSDB: please provide at least one matcher")
	errTSFilter     = errors.New("ERR TSDB: failed parsing labels")
)

// getTimeSeries returns the time series at key, nil when the key is absent.
func getTimeSeries(ctx *Context, key string) (*store.TimeSeriesValue, error) {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return nil, nil
	}
	ts, ok := entry.Value.(*store.TimeSeriesValue)
	if !ok {
		return nil, store.ErrWrongType
	}
	return ts, nil
}

// timeSeriesFor returns the time series at key, creating it with retention
// and labels when the key is absent.
func timeSeriesFor(ctx *Context, key string, retention time.Duration, labels map[string]string) (*store.TimeSeriesValue, error) {
	ts, err := getTimeSeries(ctx, key)
	if err != nil || ts != nil {
		return ts, err
	}
	ts = store.NewTimeSeriesValue(retention)
	ts.SetLabels(labels)
	if err := ctx.Store.Set(key, ts, store.SetOptions{}); err != nil {
		return nil, err
	}
	return ts, nil
}

// valueWritten accounts for a value changed in place: the key's memory and
// its watchers.
func valueWritten(ctx *Context, key string) {
	ctx.Store.Reaccount(key)
	ctx.Store.TouchKeys([]string{key})
}

func RegisterTSCommands(router *Router) {
	router.Register(&CommandDef{Name: "TS.CREATE", Handler: cmdTSCREATE})
	router.Register(&CommandDef{Name: "TS.DEL", Handler: cmdTSDEL})
	router.Register(&CommandDef{Name: "TS.ADD", Handler: cmdTSADD})
	router.Register(&CommandDef{Name: "TS.MADD", Handler: cmdTSMADD})
	router.Register(&CommandDef{Name: "TS.RANGE", Handler: cmdTSRANGE})
	router.Register(&CommandDef{Name: "TS.REVRANGE", Handler: cmdTSREVRANGE})
	router.Register(&CommandDef{Name: "TS.GET", Handler: cmdTSGET})
	router.Register(&CommandDef{Name: "TS.INFO", Handler: cmdTSINFO})
	router.Register(&CommandDef{Name: "TS.QUERYINDEX", Handler: cmdTSQUERYINDEX})
	router.Register(&CommandDef{Name: "TS.ALTER", Handler: cmdTSALTER})
	router.Register(&CommandDef{Name: "TS.INCRBY", Handler: cmdTSINCRBY})
	router.Register(&CommandDef{Name: "TS.DECRBY", Handler: cmdTSDECRBY})
}

func cmdTSCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	retention := time.Duration(0)
	labels := make(map[string]string)

	for i := 1; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "RETENTION":
			i++
			if i < ctx.ArgCount() {
				ms := parseInt64(ctx.ArgString(i))
				// Bound RETENTION before multiplying: ms * 1e6 overflows int64
				// nanoseconds and wraps NEGATIVE, and every trim site in
				// internal/store/timeseries.go gates on `if v.Retention > 0`, so a
				// wrapped-negative retention silently disables trimming forever
				// (unbounded growth). maxExpireAtMillis is MaxInt64/time.Millisecond,
				// the largest value whose product still fits in int64. Redis also
				// rejects a negative retention.
				if ms < 0 || ms > maxExpireAtMillis {
					return ctx.WriteError(errInvalidExpireTime)
				}
				retention = time.Duration(ms) * time.Millisecond
			}
		case "LABELS":
			for i+2 < ctx.ArgCount() {
				i++
				labelKey := ctx.ArgString(i)
				i++
				labelVal := ctx.ArgString(i)
				labels[labelKey] = labelVal
			}
		}
	}

	if existing, err := getTimeSeries(ctx, key); err != nil || existing != nil {
		return ctx.WriteError(errTSKeyExists)
	}
	if _, err := timeSeriesFor(ctx, key, retention, labels); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdTSDEL(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	from := parseTSBound(ctx.ArgString(1))
	to := parseTSBound(ctx.ArgString(2))

	ts, err := getTimeSeries(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if ts == nil {
		return ctx.WriteError(errTSKeyMissing)
	}

	deleted := 0
	for _, sample := range ts.Range(from, to) {
		deleted += ts.Delete(sample.Timestamp)
	}
	if deleted > 0 {
		valueWritten(ctx, key)
	}

	return ctx.WriteInteger(int64(deleted))
}

func cmdTSADD(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	timestamp := parseInt64(ctx.ArgString(1))
	// Parse the value strictly, and before any store access. parseJSONFloat
	// returns 0 for anything it cannot read, so a typo'd value silently wrote a
	// bogus zero sample and replied with the resolved timestamp as if it had
	// been stored. The timestamp is deliberately left to parseInt64: "*" is the
	// documented auto-timestamp sentinel here.
	value, err := strconv.ParseFloat(ctx.ArgString(2), 64)
	if err != nil {
		return ctx.WriteError(ErrNotFloat)
	}

	retention := time.Duration(0)
	labels := make(map[string]string)
	onDuplicate := ""

	for i := 3; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "RETENTION":
			i++
			if i < ctx.ArgCount() {
				ms := parseInt64(ctx.ArgString(i))
				// Bound RETENTION before multiplying: ms * 1e6 overflows int64
				// nanoseconds and wraps NEGATIVE, and every trim site in
				// internal/store/timeseries.go gates on `if v.Retention > 0`, so a
				// wrapped-negative retention silently disables trimming forever
				// (unbounded growth). maxExpireAtMillis is MaxInt64/time.Millisecond,
				// the largest value whose product still fits in int64. Redis also
				// rejects a negative retention.
				if ms < 0 || ms > maxExpireAtMillis {
					return ctx.WriteError(errInvalidExpireTime)
				}
				retention = time.Duration(ms) * time.Millisecond
			}
		case "LABELS":
			for i+2 < ctx.ArgCount() {
				i++
				labelKey := ctx.ArgString(i)
				i++
				labelVal := ctx.ArgString(i)
				labels[labelKey] = labelVal
			}
		case "ON_DUPLICATE":
			i++
			if i < ctx.ArgCount() {
				onDuplicate = ctx.ArgString(i)
			}
		}
	}

	policy := store.DuplicateReject
	if onDuplicate != "" {
		var ok bool
		if policy, ok = store.ParseDuplicatePolicy(onDuplicate); !ok {
			return ctx.WriteError(ErrInvalidArg)
		}
	}

	ts, err := timeSeriesFor(ctx, key, retention, labels)
	if err != nil {
		return ctx.WriteError(err)
	}
	written, resultTs := ts.AddWithPolicy(timestamp, value, policy)
	if !written {
		return ctx.WriteError(fmt.Errorf("TSDB: duplicate timestamp"))
	}
	valueWritten(ctx, key)
	return ctx.WriteInteger(resultTs)
}

func cmdTSMADD(ctx *Context) error {
	if ctx.ArgCount() < 3 || ctx.ArgCount()%3 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	results := make([]*resp.Value, 0)

	for i := 0; i < ctx.ArgCount(); i += 3 {
		key := ctx.ArgString(i)
		timestamp := parseInt64(ctx.ArgString(i + 1))
		// Strict parse, as in cmdTSADD: parseJSONFloat turned a typo'd value
		// into 0 and wrote a bogus sample. "*" remains valid for the timestamp.
		value, err := strconv.ParseFloat(ctx.ArgString(i+2), 64)
		if err != nil {
			return ctx.WriteError(ErrNotFloat)
		}

		ts, err := timeSeriesFor(ctx, key, 0, nil)
		if err != nil {
			return ctx.WriteError(err)
		}
		written, resultTs := ts.AddWithPolicy(timestamp, value, store.DuplicateReject)
		if !written {
			return ctx.WriteError(fmt.Errorf("TSDB: duplicate timestamp"))
		}
		valueWritten(ctx, key)
		results = append(results, resp.IntegerValue(resultTs))
	}

	return ctx.WriteArray(results)
}

func cmdTSRANGE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	from := parseTSBound(ctx.ArgString(1))
	to := parseTSBound(ctx.ArgString(2))

	count := 0
	aggType := ""
	bucketSize := int64(0)

	for i := 3; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "COUNT":
			i++
			if i < ctx.ArgCount() {
				count = int(parseInt64(ctx.ArgString(i)))
			}
		case "AGGREGATION":
			i++
			if i < ctx.ArgCount() {
				aggType = strings.ToLower(ctx.ArgString(i))
			}
			i++
			if i < ctx.ArgCount() {
				bucketSize = parseInt64(ctx.ArgString(i))
			}
		}
	}

	ts, err := getTimeSeries(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if ts == nil {
		return ctx.WriteError(errTSKeyMissing)
	}

	var samples []store.TimeSeriesSample
	if aggType != "" && bucketSize > 0 {
		samples = ts.Aggregation(from, to, aggType, bucketSize)
		if count > 0 && len(samples) > count {
			samples = samples[:count]
		}
	} else if count > 0 {
		samples = ts.RangeWithCount(from, to, count)
	} else {
		samples = ts.Range(from, to)
	}

	results := make([]*resp.Value, 0, len(samples))
	for _, s := range samples {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.IntegerValue(s.Timestamp),
			resp.BulkString(fmt.Sprintf("%v", s.Value)),
		}))
	}

	return ctx.WriteArray(results)
}

func cmdTSREVRANGE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	from := parseTSBound(ctx.ArgString(1))
	to := parseTSBound(ctx.ArgString(2))

	count := 0
	aggType := ""
	bucketSize := int64(0)

	for i := 3; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "COUNT":
			i++
			if i < ctx.ArgCount() {
				count = int(parseInt64(ctx.ArgString(i)))
			}
		case "AGGREGATION":
			i++
			if i < ctx.ArgCount() {
				aggType = strings.ToLower(ctx.ArgString(i))
			}
			i++
			if i < ctx.ArgCount() {
				bucketSize = parseInt64(ctx.ArgString(i))
			}
		}
	}

	ts, err := getTimeSeries(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if ts == nil {
		return ctx.WriteError(errTSKeyMissing)
	}

	var samples []store.TimeSeriesSample
	if aggType != "" && bucketSize > 0 {
		samples = ts.Aggregation(from, to, aggType, bucketSize)
	} else {
		samples = ts.Range(from, to)
	}
	if count > 0 && len(samples) > count {
		samples = samples[len(samples)-count:]
	}

	results := make([]*resp.Value, 0, len(samples))
	for i := len(samples) - 1; i >= 0; i-- {
		s := samples[i]
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.IntegerValue(s.Timestamp),
			resp.BulkString(fmt.Sprintf("%v", s.Value)),
		}))
	}

	return ctx.WriteArray(results)
}

func cmdTSGET(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	ts, err := getTimeSeries(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if ts == nil {
		return ctx.WriteError(errTSKeyMissing)
	}

	latest := ts.Latest()
	if latest == nil {
		return ctx.WriteArray([]*resp.Value{})
	}

	return ctx.WriteArray([]*resp.Value{
		resp.IntegerValue(latest.Timestamp),
		resp.BulkString(fmt.Sprintf("%v", latest.Value)),
	})
}

func cmdTSINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	ts, err := getTimeSeries(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if ts == nil {
		return ctx.WriteError(errTSKeyMissing)
	}

	first := ts.First()
	latest := ts.Latest()

	firstTs := int64(0)
	lastTs := int64(0)
	if first != nil {
		firstTs = first.Timestamp
	}
	if latest != nil {
		lastTs = latest.Timestamp
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("totalSamples"),
		resp.IntegerValue(int64(ts.Len())),
		resp.BulkString("memoryUsage"),
		resp.IntegerValue(ts.SizeOf()),
		resp.BulkString("firstTimestamp"),
		resp.IntegerValue(firstTs),
		resp.BulkString("lastTimestamp"),
		resp.IntegerValue(lastTs),
		resp.BulkString("retentionTime"),
		resp.IntegerValue(ts.Retention.Milliseconds()),
		resp.BulkString("labels"),
		formatLabels(ts.GetLabels()),
	})
}

func formatLabels(labels map[string]string) *resp.Value {
	result := make([]*resp.Value, 0, len(labels))
	for k, v := range labels {
		result = append(result, resp.ArrayValue([]*resp.Value{
			resp.BulkString(k),
			resp.BulkString(v),
		}))
	}
	return resp.ArrayValue(result)
}

func cmdTSQUERYINDEX(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	filters := make([]tsFilter, 0, ctx.ArgCount())
	positive := false
	for i := 0; i < ctx.ArgCount(); i++ {
		f, ok := parseTSFilter(ctx.ArgString(i))
		if !ok {
			return ctx.WriteError(errTSFilter)
		}
		positive = positive || (!f.negate && len(f.values) > 0)
		filters = append(filters, f)
	}
	if !positive {
		return ctx.WriteError(errTSMatcher)
	}

	keys := make([]string, 0)
	err := ctx.Store.ForEachLive(func(key string, entry *store.Entry) error {
		ts, ok := entry.Value.(*store.TimeSeriesValue)
		if !ok {
			return nil
		}
		labels := ts.GetLabels()
		for _, f := range filters {
			if !f.matches(labels) {
				return nil
			}
		}
		keys = append(keys, key)
		return nil
	})
	if err != nil {
		return ctx.WriteError(err)
	}
	sort.Strings(keys)
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

func cmdTSALTER(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	retention := time.Duration(0)
	clearRetention := false
	replaceLabels := false
	labels := make(map[string]string)

	for i := 1; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "RETENTION":
			i++
			if i < ctx.ArgCount() {
				ms, err := strconv.ParseInt(ctx.ArgString(i), 10, 64)
				if err != nil {
					ms = 0
				}
				// Bound RETENTION before multiplying: ms * 1e6 overflows int64
				// nanoseconds and wraps NEGATIVE, and every trim site in
				// internal/store/timeseries.go gates on `if v.Retention > 0`, so a
				// wrapped-negative retention silently disables trimming forever
				// (unbounded growth). maxExpireAtMillis is MaxInt64/time.Millisecond,
				// the largest value whose product still fits in int64. Redis also
				// rejects a negative retention.
				if ms < 0 || ms > maxExpireAtMillis {
					return ctx.WriteError(errInvalidExpireTime)
				}
				retention = time.Duration(ms) * time.Millisecond
				clearRetention = err == nil && ms == 0
			}
		case "LABELS":
			if (ctx.ArgCount()-i-1)%2 != 0 {
				return ctx.WriteError(ErrSyntaxError)
			}
			replaceLabels = true
			for i+2 < ctx.ArgCount() {
				i++
				labelKey := ctx.ArgString(i)
				i++
				labelVal := ctx.ArgString(i)
				labels[labelKey] = labelVal
			}
		}
	}

	ts, err := getTimeSeries(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if ts == nil {
		return ctx.WriteError(errTSKeyMissing)
	}

	if retention > 0 || clearRetention {
		ts.SetRetention(retention)
	}

	if replaceLabels {
		ts.ReplaceLabels(labels)
	}
	valueWritten(ctx, key)

	return ctx.WriteOK()
}

func cmdTSINCRBY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	// Strict parse, as in cmdTSADD/cmdTSMADD: parseJSONFloat turned a typo'd
	// increment into 0, so the counter silently froze while still replying with
	// a success-shaped timestamp. The TIMESTAMP below stays lenient because 0
	// is this handler's "use now" sentinel.
	increment, err := strconv.ParseFloat(ctx.ArgString(1), 64)
	if err != nil {
		return ctx.WriteError(ErrNotFloat)
	}

	timestamp := int64(0)
	for i := 2; i < ctx.ArgCount(); i++ {
		if strings.ToUpper(ctx.ArgString(i)) == "TIMESTAMP" {
			i++
			if i < ctx.ArgCount() {
				timestamp = parseInt64(ctx.ArgString(i))
			}
		}
	}

	ts, err := timeSeriesFor(ctx, key, 0, nil)
	if err != nil {
		return ctx.WriteError(err)
	}

	latest := ts.Latest()
	if timestamp == 0 {
		timestamp = time.Now().UnixMilli()
	}
	if latest != nil && timestamp < latest.Timestamp {
		return ctx.WriteError(fmt.Errorf("TSDB: timestamp must be equal to or higher than the maximum existing timestamp"))
	}

	var newValue float64
	if latest != nil {
		newValue = latest.Value + increment
	} else {
		newValue = increment
	}

	_, result := ts.AddWithPolicy(timestamp, newValue, store.DuplicateLast)
	valueWritten(ctx, key)
	return ctx.WriteInteger(result)
}

func cmdTSDECRBY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	// Strict parse, mirroring cmdTSINCRBY: parseJSONFloat turned a typo'd
	// decrement into 0, so the counter silently stayed put and still replied
	// with a success-shaped timestamp.
	decrement, err := strconv.ParseFloat(ctx.ArgString(1), 64)
	if err != nil {
		return ctx.WriteError(ErrNotFloat)
	}

	timestamp := int64(0)
	for i := 2; i < ctx.ArgCount(); i++ {
		if strings.ToUpper(ctx.ArgString(i)) == "TIMESTAMP" {
			i++
			if i < ctx.ArgCount() {
				timestamp = parseInt64(ctx.ArgString(i))
			}
		}
	}

	ts, err := timeSeriesFor(ctx, key, 0, nil)
	if err != nil {
		return ctx.WriteError(err)
	}

	latest := ts.Latest()
	if timestamp == 0 {
		timestamp = time.Now().UnixMilli()
	}
	if latest != nil && timestamp < latest.Timestamp {
		return ctx.WriteError(fmt.Errorf("TSDB: timestamp must be equal to or higher than the maximum existing timestamp"))
	}

	var newValue float64
	if latest != nil {
		newValue = latest.Value - decrement
	} else {
		newValue = -decrement
	}

	_, result := ts.AddWithPolicy(timestamp, newValue, store.DuplicateLast)
	valueWritten(ctx, key)
	return ctx.WriteInteger(result)
}

// parseTSBound parses a TS range endpoint. Redis defines "-" as the minimum
// possible timestamp and "+" as the maximum. parseInt64 cannot express those:
// it returns 0 for any non-numeric input, so "-" and "+" were silently
// reinterpreted as the literal instant 0, collapsing `TS.RANGE key - +` to
// Range(0, 0) and matching nothing.
func parseTSBound(s string) int64 {
	switch s {
	case "-":
		return math.MinInt64
	case "+":
		return math.MaxInt64
	}
	return parseInt64(s)
}

func parseInt64(s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
