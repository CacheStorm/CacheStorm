package store

import (
	"errors"
	"maps"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TimeSeriesSample is one sample of a time series. Labels is unused; it is
// kept so older saved series still decode.
type TimeSeriesSample struct {
	Timestamp int64
	Value     float64
	Labels    map[string]string
}

// TimeSeriesValue is a time series, RedisTimeSeries' TSDB-TYPE: samples in
// timestamp order, the series' labels (in the order they were given) and
// its settings and compaction rules.
type TimeSeriesValue struct {
	Samples           []TimeSeriesSample
	Labels            map[string]string
	LabelKeys         []string
	Retention         time.Duration
	DuplicatePolicy   string
	ChunkSize         int64
	Uncompressed      bool
	IgnoreMaxTimeDiff int64
	IgnoreMaxValDiff  float64
	Rules             []TSRule
	SourceKey         string
	layout            tsLayout
	loadedChunks      bool
	last              TimeSeriesSample
	lastSet           bool
	labelMem          int64
	labelMemSet       bool
	countSkew         int64
	mu                sync.RWMutex
}

// TSRule is a compaction rule: samples are aggregated per bucket into the
// series Dest. Ctx aggregates the bucket starting at Start, once Started
// (RedisTimeSeries' startCurrentTimeBucket, -1 before the first sample).
type TSRule struct {
	Dest        string
	Aggregation string
	Bucket      int64
	Align       int64
	Started     bool
	Start       uint64
	Ctx         TSAggContext
}

// TSDefaultChunkSize is a series' default CHUNK_SIZE.
const TSDefaultChunkSize = 4096

//nolint:staticcheck // RedisTimeSeries' reply texts
var (
	ErrTSDuplicateBlock = errors.New("ERR TSDB: Error at upsert, update is not supported when DUPLICATE_POLICY is set to BLOCK mode")
	ErrTSRetention      = errors.New("ERR TSDB: Timestamp is older than retention")
)

func (v *TimeSeriesValue) Lock()    { v.mu.Lock() }
func (v *TimeSeriesValue) Unlock()  { v.mu.Unlock() }
func (v *TimeSeriesValue) RLock()   { v.mu.RLock() }
func (v *TimeSeriesValue) RUnlock() { v.mu.RUnlock() }

func NewTimeSeriesValue(retention time.Duration) *TimeSeriesValue {
	return &TimeSeriesValue{
		Samples:   make([]TimeSeriesSample, 0),
		Labels:    make(map[string]string),
		Retention: retention,
		ChunkSize: TSDefaultChunkSize,
	}
}

func (v *TimeSeriesValue) Type() DataType { return DataTypeTimeSeries }
func (v *TimeSeriesValue) String() string { return "timeseries" }

func (v *TimeSeriesValue) SizeOf() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.memoryUsageLocked()
}

func (v *TimeSeriesValue) chunkSizeLocked() int64 {
	if v.ChunkSize <= 0 {
		return TSDefaultChunkSize
	}
	return v.ChunkSize
}

func (v *TimeSeriesValue) chunkCountLocked() int64 {
	return int64(len(v.layoutLocked()))
}

func (v *TimeSeriesValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	c := &TimeSeriesValue{
		Samples:           append([]TimeSeriesSample(nil), v.Samples...),
		Labels:            maps.Clone(v.Labels),
		LabelKeys:         append([]string(nil), v.LabelKeys...),
		Retention:         v.Retention,
		DuplicatePolicy:   v.DuplicatePolicy,
		ChunkSize:         v.ChunkSize,
		Uncompressed:      v.Uncompressed,
		IgnoreMaxTimeDiff: v.IgnoreMaxTimeDiff,
		IgnoreMaxValDiff:  v.IgnoreMaxValDiff,
		SourceKey:         v.SourceKey,
		loadedChunks:      v.loadedChunks,
		last:              v.last,
		lastSet:           v.lastSet,
		labelMem:          v.labelMem,
		labelMemSet:       v.labelMemSet,
		countSkew:         v.countSkew,
	}
	c.Rules = append(c.Rules, v.Rules...)
	return c
}

// TSDuplicatePolicies are the DUPLICATE_POLICY values.
var TSDuplicatePolicies = []string{"BLOCK", "FIRST", "LAST", "MIN", "MAX", "SUM"}

// ParseTSDuplicatePolicy returns a policy name in upper case.
func ParseTSDuplicatePolicy(s string) (string, bool) {
	p := strings.ToUpper(s)
	for _, known := range TSDuplicatePolicies {
		if p == known {
			return p, true
		}
	}
	return "", false
}

// Policy returns the series' duplicate policy, BLOCK by default.
func (v *TimeSeriesValue) Policy() string {
	if v.DuplicatePolicy == "" || v.DuplicatePolicy == TSNoPolicy {
		return "BLOCK"
	}
	return v.DuplicatePolicy
}

// TSNoPolicy marks a series that has no duplicate policy of its own, as
// the compactions RedisTimeSeries' compaction policy creates.
const TSNoPolicy = "NONE"

// SetLabels replaces the labels, keeping the order they are given in.
func (v *TimeSeriesValue) SetLabels(keys []string, values []string) {
	v.labelMemSet = false
	v.Labels = make(map[string]string, len(keys))
	v.LabelKeys = v.LabelKeys[:0]
	for i, k := range keys {
		if _, dup := v.Labels[k]; !dup {
			v.LabelKeys = append(v.LabelKeys, k)
		}
		v.Labels[k] = values[i]
	}
}

// LabelList returns the labels as key, value pairs in their order.
func (v *TimeSeriesValue) LabelList() [][2]string {
	keys := v.LabelKeys
	if len(keys) != len(v.Labels) {
		keys = make([]string, 0, len(v.Labels))
		for k := range v.Labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}
	out := make([][2]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, [2]string{k, v.Labels[k]})
	}
	return out
}

func (v *TimeSeriesValue) search(ts int64) int {
	return sort.Search(len(v.Samples), func(i int) bool { return v.Samples[i].Timestamp >= ts })
}

// Last returns RedisTimeSeries' last sample: the one appended last, which
// an upsert into a later bucket of a compaction's destination leaves
// behind the newest sample.
func (v *TimeSeriesValue) Last() (TimeSeriesSample, bool) {
	if len(v.Samples) == 0 {
		return TimeSeriesSample{}, false
	}
	if v.lastSet {
		return v.last, true
	}
	return v.Samples[len(v.Samples)-1], true
}

// TotalSamples is TS.INFO's totalSamples: the samples, and the difference
// a loaded series' own count had from them. RedisTimeSeries keeps a count
// that some sequences of upserts and deletions leave off by a few, and
// carries it through a save.
func (v *TimeSeriesValue) TotalSamples() int64 {
	return max(int64(len(v.Samples))+v.countSkew, 0)
}

// SetTotalSamples records the count a loaded series came with.
func (v *TimeSeriesValue) SetTotalSamples(n int64) {
	v.countSkew = n - int64(len(v.Samples))
}

// SetLast sets the last sample, as a loaded series carries it.
func (v *TimeSeriesValue) SetLast(s TimeSeriesSample) {
	v.last, v.lastSet = s, true
}

// Ignores reports whether IGNORE drops a sample as too close to the newest
// one. The caller holds the lock.
func (v *TimeSeriesValue) Ignores(ts int64, value float64, policy string) bool {
	if policy == "" {
		policy = v.Policy()
	}
	last, ok := v.Last()
	return ok && policy == "LAST" && ts >= last.Timestamp && ts-last.Timestamp <= v.IgnoreMaxTimeDiff &&
		math.Abs(value-last.Value) <= v.IgnoreMaxValDiff && (v.IgnoreMaxTimeDiff > 0 || v.IgnoreMaxValDiff > 0)
}

// Upsert adds a sample under policy (the series' own when empty) and
// returns the timestamp TS.ADD replies with. The caller holds the lock.
func (v *TimeSeriesValue) Upsert(ts int64, value float64, policy string) (int64, error) {
	if policy == "" {
		policy = v.Policy()
	}
	last, had := v.Last()
	if had {
		if v.Retention > 0 && ts < last.Timestamp-v.Retention.Milliseconds() {
			return 0, ErrTSRetention
		}
		if v.Ignores(ts, value, policy) {
			return last.Timestamp, nil
		}
	}
	appending := !had || ts > last.Timestamp
	if appending {
		v.last, v.lastSet = TimeSeriesSample{Timestamp: ts, Value: value}, true
	}
	i := v.search(ts)
	if i < len(v.Samples) && v.Samples[i].Timestamp == ts {
		if appending {
			v.Samples[i].Value = value
			v.layout.valid = false
			return ts, nil
		}
		defer func() {
			if ts == last.Timestamp {
				v.last.Value = v.Samples[i].Value
			}
		}()
		s := &v.Samples[i]
		v.layout.valid = false
		merged, ok := tsDuplicate(policy, s.Value, value)
		if !ok {
			return 0, ErrTSDuplicateBlock
		}
		s.Value = merged
		return ts, nil
	}
	v.Samples = append(v.Samples, TimeSeriesSample{})
	copy(v.Samples[i+1:], v.Samples[i:])
	v.Samples[i] = TimeSeriesSample{Timestamp: ts, Value: value}
	if i == len(v.Samples)-1 {
		v.appended()
	} else {
		v.layout.valid = false
	}
	v.trimLocked()
	return ts, nil
}

// tsDuplicate is handleDuplicateSample: the value a sample at an occupied
// timestamp leaves under policy; ok is false when BLOCK refuses it. A NaN
// gives way to the other value whatever the policy.
func tsDuplicate(policy string, old, value float64) (float64, bool) {
	if policy == "BLOCK" {
		return 0, false
	}
	if math.IsNaN(old) || math.IsNaN(value) {
		if math.IsNaN(value) {
			return old, true
		}
		return value, true
	}
	switch policy {
	case "FIRST":
		return old, true
	case "MIN":
		if old < value {
			return old, true
		}
	case "MAX":
		if old > value {
			return old, true
		}
	case "SUM":
		return value + old, true
	}
	return value, true
}

// UpsertSample is SeriesUpsertSample, which writes a compaction's
// recomputed bucket: the sample is inserted or overwrites the one at ts, and
// the last sample changes only when it is the one at ts. An empty series
// takes it as its last sample. The caller holds the lock.
func (v *TimeSeriesValue) UpsertSample(ts int64, value float64) {
	last, had := v.Last()
	if !had {
		v.Upsert(ts, value, "LAST") //nolint:errcheck // LAST never fails
		return
	}
	v.layout.valid = false
	i := v.search(ts)
	if i < len(v.Samples) && v.Samples[i].Timestamp == ts {
		value, _ = tsDuplicate("LAST", v.Samples[i].Value, value)
		v.Samples[i].Value = value
	} else {
		v.Samples = append(v.Samples, TimeSeriesSample{})
		copy(v.Samples[i+1:], v.Samples[i:])
		v.Samples[i] = TimeSeriesSample{Timestamp: ts, Value: value}
	}
	if !v.lastSet {
		v.last, v.lastSet = last, true
	}
	if ts == last.Timestamp {
		v.last.Value = value
	}
}

// trimLocked drops the samples the retention period no longer covers.
func (v *TimeSeriesValue) trimLocked() {
	last, ok := v.Last()
	if !ok || v.Retention <= 0 {
		return
	}
	if cut := v.search(last.Timestamp - v.Retention.Milliseconds()); cut > 0 {
		v.layout.valid = false
		v.loadedChunks = true
		v.Samples = append(v.Samples[:0], v.Samples[cut:]...)
	}
}

// SetRetention changes the retention period and trims to it.
func (v *TimeSeriesValue) SetRetention(retention time.Duration) {
	v.Retention = retention
	v.trimLocked()
}

// Range returns the samples with from <= timestamp <= to.
func (v *TimeSeriesValue) Range(from, to int64) []TimeSeriesSample {
	if from > to {
		return nil
	}
	i, j := v.search(from), len(v.Samples)
	if to < math.MaxInt64 {
		j = v.search(to + 1)
	}
	return append([]TimeSeriesSample(nil), v.Samples[i:j]...)
}

// DeleteRange removes the samples with from <= timestamp <= to.
func (v *TimeSeriesValue) DeleteRange(from, to int64) int {
	if from > to {
		return 0
	}
	i, j := v.search(from), len(v.Samples)
	if to < math.MaxInt64 {
		j = v.search(to + 1)
	}
	if j > i {
		v.layout.valid = false
	}
	v.Samples = append(v.Samples[:i], v.Samples[j:]...)
	if v.lastSet && from <= v.last.Timestamp && v.last.Timestamp <= to {
		v.lastSet = false
	}
	return j - i
}

// TSInfo is what TS.INFO reports besides the labels and rules.
type TSInfo struct {
	Samples, Memory, First, Last, ChunkCount, ChunkSize int64
}

func (v *TimeSeriesValue) Info() TSInfo {
	info := TSInfo{Samples: v.TotalSamples(), Memory: v.memoryUsageLocked(), ChunkCount: v.chunkCountLocked(), ChunkSize: v.chunkSizeLocked()}
	if last, ok := v.Last(); ok {
		info.First, info.Last = v.Samples[0].Timestamp, last.Timestamp
	}
	return info
}

// TSAggregations are the aggregation types.
var TSAggregations = []string{"avg", "sum", "min", "max", "range", "count", "first", "last", "std.p", "std.s", "var.p", "var.s", "twa"}

// ParseTSAggregation returns an aggregation name in lower case.
func ParseTSAggregation(s string) (string, bool) {
	a := strings.ToLower(s)
	for _, known := range TSAggregations {
		if a == known {
			return a, true
		}
	}
	return "", false
}

// TSAggregate reduces the samples of one bucket.
func TSAggregate(agg string, samples []TimeSeriesSample) float64 {
	n := float64(len(samples))
	if n == 0 {
		if agg == "count" || agg == "sum" {
			return 0
		}
		return math.NaN()
	}
	if agg != "twa" {
		var c TSAggContext
		for _, s := range samples {
			c.Add(agg, s.Value, uint64(s.Timestamp))
		}
		v, _ := c.Finalize(agg)
		return v
	}
	if len(samples) == 1 {
		return samples[0].Value
	}
	sum, area := 0.0, 0.0
	for i, s := range samples {
		sum += s.Value
		if i > 0 {
			dt := float64(s.Timestamp - samples[i-1].Timestamp)
			area += (s.Value + samples[i-1].Value) / 2 * dt
		}
	}
	span := float64(samples[len(samples)-1].Timestamp - samples[0].Timestamp)
	if span == 0 {
		return sum / n
	}
	return area / span
}

// TSBucketStart is the start of the bucket ts falls in.
func TSBucketStart(ts, bucket, align int64) int64 {
	d := ts - align
	start := d - d%bucket
	if d < 0 && d%bucket != 0 {
		start -= bucket
	}
	return start + align
}

// FormatTSValue prints a sample value as RedisTimeSeries does: the shortest
// digits, in exponent notation (1E-1, 1E16) below 1 and when Dragonbox's
// decimal exponent is positive.
func FormatTSValue(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		if math.Signbit(f) {
			return "-0"
		}
		return "0"
	}
	s := strconv.FormatFloat(f, 'e', -1, 64)
	sign := ""
	if s[0] == '-' {
		sign, s = "-", s[1:]
	}
	mant, expText, _ := strings.Cut(s, "e")
	exp, err := strconv.Atoi(expText)
	if err != nil {
		return s
	}
	digits := strings.Replace(mant, ".", "", 1)
	k := exp - (len(digits) - 1)
	if exp < 0 || (math.Abs(f) >= 1<<52 && dragonboxExponent(math.Abs(f)) > 0) {
		out := digits[:1]
		if len(digits) > 1 {
			out += "." + digits[1:]
		}
		return sign + out + "E" + strconv.Itoa(exp)
	}
	if k >= 0 {
		return sign + digits + strings.Repeat("0", k)
	}
	return sign + digits[:exp+1] + "." + digits[exp+1:]
}

// dragonboxExponent is the decimal exponent Dragonbox gives f (at least
// 2^52) before removing trailing zeros: one more than its kappa exponent
// when a multiple of that larger power of ten lies in the interval of
// numbers rounding to f.
func dragonboxExponent(f float64) int {
	b := math.Float64bits(f)
	frac, biased := b&(1<<52-1), int(b>>52)
	fc := new(big.Int).SetUint64(frac | 1<<52)
	e2 := biased - 1075
	shorter := frac == 0 && biased > 1
	var e int
	if shorter {
		e = (e2*631305-261663)>>21 + 1
	} else {
		e = (e2*315653)>>20 + 1
	}
	// The interval, scaled by 4 so it has integer ends for e2 >= 0.
	v4 := new(big.Int).Lsh(fc, uint(e2+2))
	lowGap := new(big.Int).Lsh(big.NewInt(1), uint(e2+1))
	if shorter {
		lowGap.Rsh(lowGap, 1)
	}
	lo := new(big.Int).Sub(v4, lowGap)
	hi := new(big.Int).Add(v4, new(big.Int).Lsh(big.NewInt(1), uint(e2+1)))
	m := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(e)), nil)
	m.Lsh(m, 2)
	if frac&1 == 1 {
		lo.Add(lo, big.NewInt(1))
		hi.Sub(hi, big.NewInt(1))
	}
	q := new(big.Int).Div(hi, m)
	if q.Mul(q, m).Cmp(lo) >= 0 {
		return e
	}
	return e - 1
}
