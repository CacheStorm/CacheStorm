package store

import (
	"testing"
	"time"
)

// The expected texts were taken from Redis 8.2's RedisTimeSeries.
func TestFormatTSValueMatchesRedisTimeSeries(t *testing.T) {
	for in, want := range map[float64]string{
		0.5: "5E-1", 0.25: "2.5E-1", 1.25: "1.25", 100: "100", 123456.789: "123456.789",
		1e15: "1000000000000000", 1e16: "1E16", 1e17: "1E17", 12345678901234567: "12345678901234568",
		0.001: "1E-3", -0.5: "-5E-1", -2: "-2", 0: "0", 1e-300: "1E-300", 2.5e10: "25000000000",
		0.30000000000000004: "3.0000000000000004E-1", 3.14159265358979: "3.14159265358979",
	} {
		if got := FormatTSValue(in); got != want {
			t.Errorf("FormatTSValue(%v) = %s, want %s", in, got, want)
		}
	}
}

func TestTimeSeriesUpsertPoliciesRetentionAndIgnore(t *testing.T) {
	ts := NewTimeSeriesValue(0)
	ts.DuplicatePolicy = "SUM"
	for _, v := range []float64{1, 2} {
		if _, err := ts.Upsert(1, v, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ts.Upsert(1, 5, "MIN"); err != nil || ts.Samples[0].Value != 3 {
		t.Fatalf("SUM then MIN = %v, %v", ts.Samples, err)
	}
	if _, err := ts.Upsert(0, 7, ""); err != nil || ts.Samples[0].Timestamp != 0 {
		t.Fatal("an older sample must be inserted in order")
	}
	if _, err := ts.Upsert(1, 1, "BLOCK"); err != ErrTSDuplicateBlock {
		t.Fatalf("BLOCK = %v", err)
	}

	r := NewTimeSeriesValue(time.Second)
	r.Upsert(5000, 1, "") //nolint:errcheck
	if _, err := r.Upsert(3000, 1, ""); err != ErrTSRetention {
		t.Fatalf("a sample older than the retention = %v", err)
	}
	r.Upsert(6500, 1, "") //nolint:errcheck
	if len(r.Samples) != 1 || r.Samples[0].Timestamp != 6500 {
		t.Fatalf("retention trim left %v", r.Samples)
	}

	ig := NewTimeSeriesValue(0)
	ig.DuplicatePolicy, ig.IgnoreMaxTimeDiff, ig.IgnoreMaxValDiff = "LAST", 10, 0.5
	ig.Upsert(100, 1, "") //nolint:errcheck
	if at, _ := ig.Upsert(105, 1.2, ""); at != 100 || len(ig.Samples) != 1 {
		t.Fatal("a small change within IGNORE must be dropped")
	}
	if at, _ := ig.Upsert(120, 1.2, ""); at != 120 {
		t.Fatal("a change past IGNORE must be kept")
	}
}

func TestTSAggregate(t *testing.T) {
	s := []TimeSeriesSample{{1000, 1, nil}, {2000, 3, nil}}
	for agg, want := range map[string]float64{"avg": 2, "sum": 4, "min": 1, "max": 3, "range": 2, "count": 2,
		"first": 1, "last": 3, "var.s": 2, "var.p": 1, "std.p": 1, "twa": 2} {
		if got := TSAggregate(agg, s); got != want {
			t.Errorf("%s = %v, want %v", agg, got, want)
		}
	}
	if TSBucketStart(-1, 10, 0) != -10 || TSBucketStart(1000, 700, 0) != 700 || TSBucketStart(25, 10, 3) != 23 {
		t.Fatal("bucket starts are wrong")
	}
}
