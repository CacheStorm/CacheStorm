package store

import (
	"errors"
	"maps"
	"math"
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
	mu                sync.RWMutex
}

// TSRule is a compaction rule: samples are aggregated per bucket into the
// series Dest. Open holds the samples of the bucket not yet closed.
type TSRule struct {
	Dest        string
	Aggregation string
	Bucket      int64
	Align       int64
	Open        []TimeSeriesSample
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

// MemoryUsage is TS.INFO's memoryUsage: the chunks the samples fill and the
// series' own overhead.
func (v *TimeSeriesValue) memoryUsageLocked() int64 {
	size := 760 + v.chunkCountLocked()*v.chunkSizeLocked()
	for _, k := range v.LabelKeys {
		size += int64(len(k)+len(v.Labels[k])) + 30
	}
	return size + int64(len(v.Rules))*72
}

func (v *TimeSeriesValue) chunkSizeLocked() int64 {
	if v.ChunkSize <= 0 {
		return TSDefaultChunkSize
	}
	return v.ChunkSize
}

func (v *TimeSeriesValue) chunkCountLocked() int64 {
	perChunk := max(v.chunkSizeLocked()/16, 1)
	return max((int64(len(v.Samples))+perChunk-1)/perChunk, 1)
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
	}
	for _, r := range v.Rules {
		r.Open = append([]TimeSeriesSample(nil), r.Open...)
		c.Rules = append(c.Rules, r)
	}
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
	if v.DuplicatePolicy == "" {
		return "BLOCK"
	}
	return v.DuplicatePolicy
}

// SetLabels replaces the labels, keeping the order they are given in.
func (v *TimeSeriesValue) SetLabels(keys []string, values []string) {
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

// Last returns the newest sample.
func (v *TimeSeriesValue) Last() (TimeSeriesSample, bool) {
	if len(v.Samples) == 0 {
		return TimeSeriesSample{}, false
	}
	return v.Samples[len(v.Samples)-1], true
}

// Upsert adds a sample under policy (the series' own when empty) and
// returns the timestamp TS.ADD replies with. The caller holds the lock.
func (v *TimeSeriesValue) Upsert(ts int64, value float64, policy string) (int64, error) {
	if policy == "" {
		policy = v.Policy()
	}
	if last, ok := v.Last(); ok {
		if v.Retention > 0 && ts < last.Timestamp-v.Retention.Milliseconds() {
			return 0, ErrTSRetention
		}
		if policy == "LAST" && ts >= last.Timestamp && ts-last.Timestamp <= v.IgnoreMaxTimeDiff &&
			math.Abs(value-last.Value) <= v.IgnoreMaxValDiff && (v.IgnoreMaxTimeDiff > 0 || v.IgnoreMaxValDiff > 0) {
			return last.Timestamp, nil
		}
	}
	i := v.search(ts)
	if i < len(v.Samples) && v.Samples[i].Timestamp == ts {
		s := &v.Samples[i]
		switch policy {
		case "BLOCK":
			return 0, ErrTSDuplicateBlock
		case "LAST":
			s.Value = value
		case "MIN":
			s.Value = math.Min(s.Value, value)
		case "MAX":
			s.Value = math.Max(s.Value, value)
		case "SUM":
			s.Value += value
		}
		return ts, nil
	}
	v.Samples = append(v.Samples, TimeSeriesSample{})
	copy(v.Samples[i+1:], v.Samples[i:])
	v.Samples[i] = TimeSeriesSample{Timestamp: ts, Value: value}
	v.trimLocked()
	return ts, nil
}

// trimLocked drops the samples the retention period no longer covers.
func (v *TimeSeriesValue) trimLocked() {
	last, ok := v.Last()
	if !ok || v.Retention <= 0 {
		return
	}
	if cut := v.search(last.Timestamp - v.Retention.Milliseconds()); cut > 0 {
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
	v.Samples = append(v.Samples[:i], v.Samples[j:]...)
	return j - i
}

// TSInfo is what TS.INFO reports besides the labels and rules.
type TSInfo struct {
	Samples, Memory, First, Last, ChunkCount, ChunkSize int64
}

func (v *TimeSeriesValue) Info() TSInfo {
	info := TSInfo{Samples: int64(len(v.Samples)), Memory: v.memoryUsageLocked(), ChunkCount: v.chunkCountLocked(), ChunkSize: v.chunkSizeLocked()}
	if len(v.Samples) > 0 {
		info.First, info.Last = v.Samples[0].Timestamp, v.Samples[len(v.Samples)-1].Timestamp
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
	sum, lo, hi := 0.0, math.Inf(1), math.Inf(-1)
	for _, s := range samples {
		sum += s.Value
		lo, hi = math.Min(lo, s.Value), math.Max(hi, s.Value)
	}
	variance := func(sample bool) float64 {
		if sample && n < 2 {
			return 0
		}
		mean, sq := sum/n, 0.0
		for _, s := range samples {
			sq += (s.Value - mean) * (s.Value - mean)
		}
		if sample {
			return sq / (n - 1)
		}
		return sq / n
	}
	switch agg {
	case "avg":
		return sum / n
	case "sum":
		return sum
	case "min":
		return lo
	case "max":
		return hi
	case "range":
		return hi - lo
	case "count":
		return n
	case "first":
		return samples[0].Value
	case "last":
		return samples[len(samples)-1].Value
	case "var.p":
		return variance(false)
	case "var.s":
		return variance(true)
	case "std.p":
		return math.Sqrt(variance(false))
	case "std.s":
		return math.Sqrt(variance(true))
	case "twa":
		if len(samples) == 1 {
			return samples[0].Value
		}
		area := 0.0
		for i := 1; i < len(samples); i++ {
			dt := float64(samples[i].Timestamp - samples[i-1].Timestamp)
			area += (samples[i].Value + samples[i-1].Value) / 2 * dt
		}
		span := float64(samples[len(samples)-1].Timestamp - samples[0].Timestamp)
		if span == 0 {
			return sum / n
		}
		return area / span
	}
	return math.NaN()
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
// digits, in exponent notation (1E-1, 1E16) below 1 and from 1e16 up.
func FormatTSValue(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
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
	if exp < 0 || k >= 16 {
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
