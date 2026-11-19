package store

import (
	"math"
	"math/rand/v2"
	"testing"
)

// The expected texts were taken from Redis 8.2's RedisTimeSeries.
func TestTSAvgOverflowMatchesRedis(t *testing.T) {
	var samples []TimeSeriesSample
	for i, v := range []float64{1.5e308, 1.7e308, -1e308, 1.6e308, math.MaxFloat64, 3, 1.1e308, 1.7e308, 1.7e308} {
		samples = append(samples, TimeSeriesSample{Timestamp: int64(i + 1), Value: v})
	}
	for _, c := range []struct {
		from, to int
		want     string
	}{{0, 9, "1.1219659038735906E308"}, {3, 7, "1.1244232837155789E308"}, {2, 5, "7.992310449541052E307"}, {5, 8, "9.333333333333332E307"}} {
		if got := FormatTSValue(TSAggregate("avg", samples[c.from:c.to])); got != c.want {
			t.Errorf("avg of %d..%d = %s, Redis %s", c.from, c.to, got, c.want)
		}
	}
}

func TestFormatTSValueDragonboxExponent(t *testing.T) {
	for in, want := range map[float64]string{
		1758894235889692000: "1.758894235889692E18", -1403191434225110500: "-1.4031914342251105E18",
		1 << 53: "9007199254740992", 4503599627370500: "4.5035996273705E15", math.Inf(1): "Infinity",
	} {
		if got := FormatTSValue(in); got != want {
			t.Errorf("FormatTSValue(%v) = %s, want %s", in, got, want)
		}
	}
}

// TestTSGorillaRoundTrip encodes random samples in chunks as RedisTimeSeries
// fills them and decodes them bit for bit.
func TestTSGorillaRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 7))
	for range 200 {
		size := uint64(8 * (6 + rng.IntN(60)))
		c := NewTSGorillaChunk(size, true)
		var want []TimeSeriesSample
		ts := uint64(rng.Int64N(1 << 40))
		for {
			switch rng.IntN(4) {
			case 0:
				ts += 1 + uint64(rng.Int64N(1<<34))
			default:
				ts += uint64(rng.IntN(20))
			}
			v := math.Float64frombits(rng.Uint64())
			if math.IsNaN(v) || rng.IntN(3) == 0 {
				v = float64(rng.IntN(100))
			}
			if !c.Append(ts, v) {
				break
			}
			want = append(want, TimeSeriesSample{Timestamp: int64(ts), Value: v})
		}
		got, err := c.Samples()
		if err != nil || len(got) != len(want) {
			t.Fatalf("decoded %d of %d: %v", len(got), len(want), err)
		}
		for i := range got {
			if got[i].Timestamp != want[i].Timestamp || math.Float64bits(got[i].Value) != math.Float64bits(want[i].Value) {
				t.Fatalf("sample %d = %v, want %v", i, got[i], want[i])
			}
		}
		if c.Idx > c.Size*8 {
			t.Fatalf("wrote %d bits into %d bytes", c.Idx, c.Size)
		}
	}
}

// TestTSMemoryUsageMatchesRedis checks TS.INFO's memoryUsage against
// figures Redis 8.2 reported.
func TestTSMemoryUsageMatchesRedis(t *testing.T) {
	series := func(chunk int64, uncompressed bool, n int, first, step int64) *TimeSeriesValue {
		v := NewTimeSeriesValue(0)
		v.ChunkSize, v.Uncompressed = chunk, uncompressed
		for i := range n {
			v.Upsert(first+int64(i)*step, float64(i%7), "") //nolint:errcheck
		}
		return v
	}
	for _, c := range []struct {
		v    *TimeSeriesValue
		want int64
	}{
		{series(4096, false, 1000, 1000, 10), 4856},
		{series(48, true, 10, 1, 1), 1976},
		{series(64, true, 10, 1, 1), 1700},
		{series(128, true, 10, 1, 1), 1488},
		{series(4104, false, 0, 0, 0), 5880},
		{series(48, true, 40, 1000000, 1000), 8876},
	} {
		if got := c.v.SizeOf(); got != c.want {
			t.Errorf("%d samples in chunks of %d: memoryUsage %d, Redis %d", len(c.v.Samples), c.v.ChunkSize, got, c.want)
		}
	}
	v := series(4096, false, 25, 100, 3)
	v.SetLabels([]string{"x", "y"}, []string{"1", "2"})
	v.Rules = []TSRule{{Dest: "d", Aggregation: "avg", Bucket: 10}}
	if got := v.SizeOf(); got != 5072 {
		t.Errorf("labels and a rule: %d, Redis 5072", got)
	}
}

// TestTSRuleFeedTwaMatchesRange closes twa buckets as a range query
// aggregates them.
func TestTSRuleFeedTwaMatchesRange(t *testing.T) {
	r := TSRule{Dest: "d", Aggregation: "twa", Bucket: 10, Align: 3}
	v := NewTimeSeriesValue(0)
	var closed []TimeSeriesSample
	for i, ts := range []int64{1, 4, 9, 15, 16, 40, 41, 55} {
		val := float64(i*i) - 3
		v.Upsert(ts, val, "") //nolint:errcheck
		if start, agg, ok := r.Feed(ts, val); ok {
			closed = append(closed, TimeSeriesSample{Timestamp: start, Value: agg})
		}
	}
	for _, c := range closed {
		want, _ := v.RuleValue("twa", uint64(c.Timestamp), uint64(TSBucketStart(c.Timestamp, 10, 3)+9))
		if c.Timestamp > 0 && math.Float64bits(c.Value) != math.Float64bits(want) {
			t.Fatalf("bucket %d: fed %v, recomputed %v", c.Timestamp, c.Value, want)
		}
	}
}
