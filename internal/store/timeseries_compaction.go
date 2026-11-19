package store

import (
	"math"
	"math/big"
	"sort"
)

// TSAggContext is the state RedisTimeSeries keeps of a bucket being
// aggregated, in its own representation, so a compaction rule's open bucket
// moves to and from Redis whole and further samples aggregate to the same
// bits. Which fields an aggregation uses: Value (sum, count, last, first,
// avg's running sum or, once it overflowed, mean), Reset (first), Min and Max
// (min, max, range), Count and Overflow (avg), Sum, Sum2 and N (std.*,
// var.*), and the Twa fields (twa).
type TSAggContext struct {
	Ready     bool
	Value     float64
	Reset     bool
	Min, Max  float64
	Count     float64
	Overflow  bool
	Sum, Sum2 float64
	N         uint64

	Res, PrevValue                                  float64
	PrevTS, BucketStart, BucketEnd, FirstTS, LastTS uint64
	FirstBucket, Reverse                            bool
	Iteration                                       int64
}

// Init puts the context in the state RedisTimeSeries creates it in.
func (c *TSAggContext) Init() {
	*c = TSAggContext{Ready: true, Reset: true, Min: math.MaxFloat64, Max: -math.MaxFloat64, FirstBucket: true, Reverse: c.Reverse}
}

func (c *TSAggContext) ready() {
	if !c.Ready {
		c.Init()
	}
}

// ResetFor starts a new bucket: "last" keeps its value, as RedisTimeSeries'
// LastValueReset does.
func (c *TSAggContext) ResetFor(agg string) {
	c.ready()
	if agg != "last" {
		c.Init()
	}
}

// Add appends a sample as the aggregation's appendValue does.
func (c *TSAggContext) Add(agg string, value float64, ts uint64) {
	c.ready()
	switch agg {
	case "sum":
		c.Value += value
	case "count":
		c.Value++
	case "last":
		c.Value = value
	case "first":
		if c.Reset {
			c.Reset, c.Value = false, value
		}
	case "min":
		if value < c.Min {
			c.Min = value
		}
	case "max":
		if value > c.Max {
			c.Max = value
		}
	case "range":
		if value > c.Max {
			c.Max = value
		}
		if value < c.Min {
			c.Min = value
		}
	case "avg":
		c.avgAdd(value)
	case "std.p", "std.s", "var.p", "var.s":
		c.N++
		c.Sum += value
		c.Sum2 += float64(value * value)
	case "twa":
		c.twaAdd(value, ts)
	}
}

// avgAdd is AvgAddValue. Once the running sum would overflow it keeps the
// mean instead, computed in x87 extended precision (a 64-bit mantissa) as
// RedisTimeSeries does on x86-64.
func (c *TSAggContext) avgAdd(value float64) {
	c.Count++
	if !c.Overflow && !((c.Value < 0) == (value < 0) && math.Abs(c.Value) > math.MaxFloat64-math.Abs(value)) {
		c.Value += value
		return
	}
	if math.IsNaN(value) || math.IsInf(value, 0) || math.IsNaN(c.Value) || math.IsInf(c.Value, 0) {
		mean := c.Value / c.Count
		if c.Overflow {
			mean *= c.Count - 1
		}
		c.Value, c.Overflow = mean+value/c.Count, true
		return
	}
	ext := func(f float64) *big.Float { return new(big.Float).SetPrec(64).SetFloat64(f) }
	cnt := ext(c.Count)
	mean := ext(c.Value)
	mean.Quo(mean, cnt)
	if c.Overflow {
		mean.Mul(mean, ext(c.Count-1))
	}
	mean.Add(mean, ext(value).Quo(ext(value), cnt))
	c.Value, _ = mean.Float64()
	c.Overflow = true
}

func tsVariance(sum, sum2, count float64) float64 {
	if count == 0 {
		return 0
	}
	mean := sum / count
	return (sum2 - float64(2*sum*sum)/count + float64(mean*mean)*count) / count
}

// Finalize is the aggregation's finalize: the bucket's value, ok false when
// RedisTimeSeries reports an error (an empty avg, std or var).
func (c *TSAggContext) Finalize(agg string) (float64, bool) {
	c.ready()
	switch agg {
	case "sum", "count", "last", "first":
		return c.Value, true
	case "min":
		return c.Min, true
	case "max":
		return c.Max, true
	case "range":
		return c.Max - c.Min, true
	case "avg":
		if c.Count == 0 {
			return 0, false
		}
		if c.Overflow {
			return c.Value, true
		}
		return c.Value / c.Count, true
	case "var.p", "std.p", "var.s", "std.s":
		if c.N == 0 {
			return 0, false
		}
		n := float64(c.N)
		v := tsVariance(c.Sum, c.Sum2, n)
		if agg[4] == 's' {
			if c.N == 1 {
				v = 0
			} else {
				v = v * n / float64(c.N-1)
			}
		}
		if agg[0] == 's' {
			return math.Sqrt(v), true
		}
		return v, true
	case "twa":
		if c.LastTS == c.FirstTS {
			return c.PrevValue, true
		}
		span := int64(c.LastTS - c.FirstTS)
		if span < 0 {
			span = -span
		}
		return c.Res / float64(span), true
	}
	return math.NaN(), false
}

// TwaBucket is TwaAddBucketParams.
func (c *TSAggContext) TwaBucket(start, end uint64) {
	c.ready()
	if c.Reverse {
		start, end = end, start
	}
	c.BucketStart, c.BucketEnd = start, end
}

// TwaPrev is TwaAddPrevBucketLastSample.
func (c *TSAggContext) TwaPrev(value float64, ts uint64) {
	c.ready()
	c.PrevValue, c.PrevTS = value, ts
	c.FirstBucket = false
}

// twaAdd is TwaAddValue. Timestamp differences are unsigned and wrap as
// RedisTimeSeries' do; each product is rounded before it is added.
func (c *TSAggContext) twaAdd(value float64, ts uint64) {
	t1, t2 := c.PrevTS, ts
	v1, v2 := c.PrevValue, value
	if c.Reverse {
		t1, t2 = t2, t1
		v1, v2 = v2, v1
	}
	deltaTime := float64(t2 - t1)
	deltaVal := v2 - v1
	ta := c.BucketStart
	if c.Iteration == 0 {
		if !c.FirstBucket {
			c.FirstTS = ta
			vab := v1 + float64(float64(ta-t1)*deltaVal)/deltaTime
			if !c.Reverse {
				c.Res += float64((vab+v2)*float64(t2-ta)) / 2.0
			} else {
				c.Res += float64((vab+v1)*float64(ta-t1)) / 2.0
			}
		} else {
			c.FirstTS = ts
		}
	} else {
		c.Res += float64((v1+v2)*float64(t2-t1)) / 2.0
	}
	c.LastTS, c.PrevTS = ts, ts
	c.PrevValue = value
	c.Iteration++
}

// TwaNext is TwaAddNextBucketFirstSample.
func (c *TSAggContext) TwaNext(value float64, ts uint64) {
	c.ready()
	t1, t2 := c.PrevTS, ts
	v1, v2 := c.PrevValue, value
	if c.Reverse {
		t1, t2 = t2, t1
		v1, v2 = v2, v1
	}
	deltaTime := float64(t2 - t1)
	deltaVal := v2 - v1
	tb := c.BucketEnd
	vab := v1 + float64(float64(tb-t1)*deltaVal)/deltaTime
	if !c.Reverse {
		c.Res += float64((vab+v1)*float64(tb-t1)) / 2.0
	} else {
		c.Res += float64((vab+v2)*float64(t2-tb)) / 2.0
	}
	c.LastTS = tb
}

// TSCalcBucketStart is RedisTimeSeries' CalcBucketStart, in its unsigned
// arithmetic: the start of ts's bucket, which wraps below zero when the
// alignment is after ts.
func TSCalcBucketStart(ts, bucket, align uint64) uint64 {
	diff, n := int64(ts-align), int64(bucket)
	return ts - uint64(((diff%n)+n)%n)
}

// TSNormalizeBucket is BucketStartNormalize: a bucket start below zero is 0.
func TSNormalizeBucket(b uint64) uint64 {
	if int64(b) < 0 {
		return 0
	}
	return b
}

// Feed adds a sample appended to the source series to the rule, as
// RedisTimeSeries' handleCompaction does. A sample in a later bucket than
// the open one closes it: closed is then true and agg its value at bucket,
// unless the aggregation reported an error.
func (r *TSRule) Feed(at int64, value float64) (bucket int64, agg float64, closed bool) {
	dur, align := uint64(r.Bucket), uint64(r.Align)
	cur := TSCalcBucketStart(uint64(at), dur, align)
	curNorm := TSNormalizeBucket(cur)
	twa := r.Aggregation == "twa"
	c := &r.Ctx
	c.ready()
	if !r.Started {
		r.Started, r.Start = true, curNorm
		if twa {
			c.TwaBucket(curNorm, cur+dur)
		}
	}
	if curNorm > r.Start {
		if twa {
			c.TwaNext(value, uint64(at))
		}
		agg, closed = c.Finalize(r.Aggregation)
		bucket = int64(r.Start)
		lastTS, lastValue := c.PrevTS, c.PrevValue
		c.ResetFor(r.Aggregation)
		if twa {
			c.TwaBucket(curNorm, cur+dur)
			c.TwaPrev(lastValue, lastTS)
		}
		r.Start = curNorm
	}
	c.Add(r.Aggregation, value, uint64(at))
	return bucket, agg, closed
}

// searchU returns the index of the first sample at or after ts, compared
// unsigned as RedisTimeSeries compares timestamps.
func (v *TimeSeriesValue) searchU(ts uint64) int {
	return sort.Search(len(v.Samples), func(i int) bool { return uint64(v.Samples[i].Timestamp) >= ts })
}

// RuleRange is SeriesCalcRange: a fresh context of agg over the samples from
// start to end (unsigned, so end may be the largest timestamp), with a twa
// context's neighbours, and whether the range held no sample. The caller
// holds the lock.
func (v *TimeSeriesValue) RuleRange(agg string, start, end uint64) (TSAggContext, bool) {
	var c TSAggContext
	c.Init()
	twa := agg == "twa"
	if twa {
		c.TwaBucket(start, end+1)
		if start > 0 {
			if i := v.searchU(start); i > 0 {
				p := v.Samples[i-1]
				c.TwaPrev(p.Value, uint64(p.Timestamp))
			}
		}
	}
	empty := true
	for i := v.searchU(start); i < len(v.Samples) && uint64(v.Samples[i].Timestamp) <= end; i++ {
		c.Add(agg, v.Samples[i].Value, uint64(v.Samples[i].Timestamp))
		empty = false
	}
	if twa {
		if i := v.searchU(end + 1); i < len(v.Samples) {
			c.TwaNext(v.Samples[i].Value, uint64(v.Samples[i].Timestamp))
		}
	}
	return c, empty
}

// RuleValue is SeriesCalcRange's value: the aggregate of the range, 0 when
// it is empty.
func (v *TimeSeriesValue) RuleValue(agg string, start, end uint64) (float64, bool) {
	c, empty := v.RuleRange(agg, start, end)
	if empty {
		return 0, false
	}
	val, ok := c.Finalize(agg)
	if !ok {
		val = 0
	}
	return val, true
}
