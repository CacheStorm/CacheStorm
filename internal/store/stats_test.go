package store

import (
	"testing"
	"time"
)

func TestNewTDigest(t *testing.T) {
	td := NewTDigest(100)
	if td == nil {
		t.Fatal("expected TDigest")
	}
	if td.Compression != 100 {
		t.Errorf("expected compression 100, got %f", td.Compression)
	}
}

func TestNewTDigestZeroCompression(t *testing.T) {
	td := NewTDigest(0)
	if td.Compression != 100 {
		t.Errorf("expected default compression 100, got %f", td.Compression)
	}
}

func TestTDigestAdd(t *testing.T) {
	td := NewTDigest(100)
	td.Add(10.0, 1.0)
	td.Add(20.0, 1.0)
	td.Add(30.0, 1.0)

	if td.Size() != 3 {
		t.Errorf("expected size 3, got %d", td.Size())
	}
}

func TestTDigestAddBatch(t *testing.T) {
	td := NewTDigest(100)
	td.AddBatch([]float64{1, 2, 3, 4, 5})

	if td.Count() != 5 {
		t.Errorf("expected count 5, got %f", td.Count())
	}
}

func TestTDigestQuantileEmpty(t *testing.T) {
	td := NewTDigest(100)
	result := td.Quantile(0.5)
	if result != 0 {
		t.Errorf("expected 0 for empty digest, got %f", result)
	}
}

func TestTDigestQuantileBounds(t *testing.T) {
	td := NewTDigest(100)
	td.AddBatch([]float64{1, 2, 3, 4, 5})

	if td.Quantile(0) != 1 {
		t.Errorf("expected min 1, got %f", td.Quantile(0))
	}
	if td.Quantile(1) != 5 {
		t.Errorf("expected max 5, got %f", td.Quantile(1))
	}
}

func TestTDigestQuantile(t *testing.T) {
	td := NewTDigest(100)
	td.AddBatch([]float64{1, 2, 3, 4, 5})

	median := td.Quantile(0.5)
	if median < 2 || median > 4 {
		t.Errorf("median should be around 3, got %f", median)
	}
}

func TestTDigestCDF(t *testing.T) {
	td := NewTDigest(100)
	td.AddBatch([]float64{1, 2, 3, 4, 5})

	cdf := td.CDF(3)
	if cdf < 0.3 || cdf > 0.7 {
		t.Errorf("CDF(3) should be around 0.5, got %f", cdf)
	}
}

func TestTDigestCDFEmpty(t *testing.T) {
	td := NewTDigest(100)
	if td.CDF(5) != 0 {
		t.Errorf("expected 0 for empty digest, got %f", td.CDF(5))
	}
}

func TestTDigestCDFBounds(t *testing.T) {
	td := NewTDigest(100)
	td.AddBatch([]float64{1, 2, 3, 4, 5})

	if td.CDF(0) != 0 {
		t.Errorf("expected 0 for value below min, got %f", td.CDF(0))
	}
	if td.CDF(10) != 1 {
		t.Errorf("expected 1 for value above max, got %f", td.CDF(10))
	}
}

func TestTDigestMean(t *testing.T) {
	td := NewTDigest(100)
	td.AddBatch([]float64{1, 2, 3, 4, 5})

	mean := td.Mean()
	if mean != 3 {
		t.Errorf("expected mean 3, got %f", mean)
	}
}

func TestTDigestMeanEmpty(t *testing.T) {
	td := NewTDigest(100)
	if td.Mean() != 0 {
		t.Errorf("expected 0 for empty digest, got %f", td.Mean())
	}
}

func TestTDigestMinMax(t *testing.T) {
	td := NewTDigest(100)
	td.AddBatch([]float64{1, 2, 3, 4, 5})

	if td.Min() != 1 {
		t.Errorf("expected min 1, got %f", td.Min())
	}
	if td.Max() != 5 {
		t.Errorf("expected max 5, got %f", td.Max())
	}
}

func TestTDigestMinMaxEmpty(t *testing.T) {
	td := NewTDigest(100)
	if td.Min() != 0 {
		t.Errorf("expected 0 for empty, got %f", td.Min())
	}
	if td.Max() != 0 {
		t.Errorf("expected 0 for empty, got %f", td.Max())
	}
}

func TestTDigestReset(t *testing.T) {
	td := NewTDigest(100)
	td.AddBatch([]float64{1, 2, 3})
	td.Reset()

	if td.Size() != 0 {
		t.Errorf("expected size 0 after reset, got %d", td.Size())
	}
}

func TestTDigestMerge(t *testing.T) {
	td1 := NewTDigest(100)
	td1.AddBatch([]float64{1, 2, 3})

	td2 := NewTDigest(100)
	td2.AddBatch([]float64{4, 5, 6})

	td1.Merge(td2)

	if td1.Count() != 6 {
		t.Errorf("expected count 6 after merge, got %f", td1.Count())
	}
}

func TestTDigestCompression(t *testing.T) {
	td := NewTDigest(10)
	for i := 0; i < 1000; i++ {
		td.Add(float64(i), 1)
	}

	if td.Size() > 50 {
		t.Errorf("expected compression, got size %d", td.Size())
	}
}

func TestNewReservoirSampler(t *testing.T) {
	rs := NewReservoirSampler(10)
	if rs == nil {
		t.Fatal("expected ReservoirSampler")
	}
	if rs.MaxSize != 10 {
		t.Errorf("expected max size 10, got %d", rs.MaxSize)
	}
}

func TestReservoirSamplerAdd(t *testing.T) {
	rs := NewReservoirSampler(5)

	for i := 0; i < 10; i++ {
		rs.Add(float64(i))
	}

	data := rs.Get()
	if len(data) != 5 {
		t.Errorf("expected 5 samples, got %d", len(data))
	}
}

func TestReservoirSamplerAddBatch(t *testing.T) {
	rs := NewReservoirSampler(5)
	rs.AddBatch([]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

	if rs.Size() != 5 {
		t.Errorf("expected size 5, got %d", rs.Size())
	}
}

func TestReservoirSamplerReset(t *testing.T) {
	rs := NewReservoirSampler(5)
	rs.AddBatch([]float64{1, 2, 3, 4, 5})
	rs.Reset()

	if rs.Size() != 0 {
		t.Errorf("expected size 0 after reset, got %d", rs.Size())
	}
	if rs.TotalCount() != 0 {
		t.Errorf("expected total count 0 after reset, got %d", rs.TotalCount())
	}
}

func TestReservoirSamplerTotalCount(t *testing.T) {
	rs := NewReservoirSampler(5)
	for i := 0; i < 100; i++ {
		rs.Add(float64(i))
	}

	if rs.TotalCount() != 100 {
		t.Errorf("expected total count 100, got %d", rs.TotalCount())
	}
}

func TestNewHistogram(t *testing.T) {
	h := NewHistogram(0, 100, 10)
	if h == nil {
		t.Fatal("expected Histogram")
	}
	if h.BucketWidth != 10 {
		t.Errorf("expected bucket width 10, got %f", h.BucketWidth)
	}
}

func TestNewHistogramZeroWidth(t *testing.T) {
	h := NewHistogram(0, 100, 0)
	if h.BucketWidth != 1 {
		t.Errorf("expected default bucket width 1, got %f", h.BucketWidth)
	}
}

func TestHistogramAdd(t *testing.T) {
	h := NewHistogram(0, 100, 10)
	h.Add(5)
	h.Add(15)
	h.Add(25)

	if h.Count != 3 {
		t.Errorf("expected count 3, got %d", h.Count)
	}
}

func TestHistogramAddBelowMin(t *testing.T) {
	h := NewHistogram(0, 100, 10)
	h.Add(-5)

	if h.Buckets[-1] != 1 {
		t.Errorf("expected bucket -1 to have count 1")
	}
}

func TestHistogramAddAboveMax(t *testing.T) {
	h := NewHistogram(0, 100, 10)
	h.Add(150)

	if h.Count != 1 {
		t.Errorf("expected count 1, got %d", h.Count)
	}
}

func TestHistogramGet(t *testing.T) {
	h := NewHistogram(0, 100, 10)
	h.Add(5)
	h.Add(15)

	result := h.Get()
	if len(result) != 2 {
		t.Errorf("expected 2 buckets, got %d", len(result))
	}
}

func TestHistogramMean(t *testing.T) {
	h := NewHistogram(0, 100, 10)
	h.Add(10)
	h.Add(20)
	h.Add(30)

	mean := h.Mean()
	if mean != 20 {
		t.Errorf("expected mean 20, got %f", mean)
	}
}

func TestHistogramMeanEmpty(t *testing.T) {
	h := NewHistogram(0, 100, 10)
	if h.Mean() != 0 {
		t.Errorf("expected 0 for empty histogram, got %f", h.Mean())
	}
}

func TestHistogramReset(t *testing.T) {
	h := NewHistogram(0, 100, 10)
	h.Add(5)
	h.Add(15)
	h.Reset()

	if h.Count != 0 {
		t.Errorf("expected count 0 after reset, got %d", h.Count)
	}
}

func TestFormatInt(t *testing.T) {
	tests := []struct {
		n        int64
		expected string
	}{
		{0, "0"},
		{123, "123"},
		{-456, "-456"},
	}

	for _, tt := range tests {
		result := formatInt(tt.n)
		if result != tt.expected {
			t.Errorf("formatInt(%d) = %s, expected %s", tt.n, result, tt.expected)
		}
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		f        float64
		expected string
	}{
		{0.0, "0"},
		{1.5, "1.5"},
		{-2.25, "-2.25"},
	}

	for _, tt := range tests {
		result := formatFloat(tt.f)
		if result != tt.expected {
			t.Errorf("formatFloat(%f) = %s, expected %s", tt.f, result, tt.expected)
		}
	}
}

func TestNewLatencyTracker(t *testing.T) {
	lt := NewLatencyTracker()
	if lt == nil {
		t.Fatal("expected LatencyTracker")
	}
}

func TestLatencyTrackerRecord(t *testing.T) {
	lt := NewLatencyTracker()
	lt.Record(1_000_000)
	lt.Record(2_000_000)
	lt.Record(3_000_000)

	stats := lt.Stats()
	if stats["count"] != int64(3) {
		t.Errorf("expected count 3, got %v", stats["count"])
	}
}

func TestLatencyTrackerStatsEmpty(t *testing.T) {
	lt := NewLatencyTracker()
	stats := lt.Stats()

	if stats["count"] != int64(0) {
		t.Errorf("expected count 0 for empty, got %v", stats["count"])
	}
}

func TestLatencyTrackerHighLatency(t *testing.T) {
	lt := NewLatencyTracker()
	lt.Record(20_000_000)

	stats := lt.Stats()
	buckets := stats["buckets"].(map[string]int64)
	if buckets["15ms+"] != 1 {
		t.Errorf("expected 15ms+ bucket to have count 1")
	}
}

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	if m == nil {
		t.Fatal("expected Metrics")
	}
}

func TestMetricsRecordConnection(t *testing.T) {
	m := NewMetrics()
	m.RecordConnection()
	m.RecordConnection()

	if m.TotalConnections.Load() != 2 {
		t.Errorf("expected 2 total connections, got %d", m.TotalConnections.Load())
	}
	if m.ActiveConnections.Load() != 2 {
		t.Errorf("expected 2 active connections, got %d", m.ActiveConnections.Load())
	}
}

func TestMetricsRecordDisconnection(t *testing.T) {
	m := NewMetrics()
	m.RecordConnection()
	m.RecordConnection()
	m.RecordDisconnection()

	if m.ActiveConnections.Load() != 1 {
		t.Errorf("expected 1 active connection, got %d", m.ActiveConnections.Load())
	}
}

func TestMetricsRecordCommand(t *testing.T) {
	m := NewMetrics()
	m.RecordCommand("GET", 1_000_000)
	m.RecordCommand("GET", 2_000_000)
	m.RecordCommand("SET", 1_000_000)

	if m.TotalCommands.Load() != 3 {
		t.Errorf("expected 3 total commands, got %d", m.TotalCommands.Load())
	}
}

func TestMetricsRecordReadWrite(t *testing.T) {
	m := NewMetrics()
	m.RecordRead()
	m.RecordWrite()

	if m.TotalReads.Load() != 1 {
		t.Errorf("expected 1 read, got %d", m.TotalReads.Load())
	}
	if m.TotalWrites.Load() != 1 {
		t.Errorf("expected 1 write, got %d", m.TotalWrites.Load())
	}
}

func TestMetricsRecordHitMiss(t *testing.T) {
	m := NewMetrics()
	m.RecordHit()
	m.RecordHit()
	m.RecordMiss()

	if m.TotalHits.Load() != 2 {
		t.Errorf("expected 2 hits, got %d", m.TotalHits.Load())
	}
	if m.TotalMisses.Load() != 1 {
		t.Errorf("expected 1 miss, got %d", m.TotalMisses.Load())
	}
}

func TestMetricsRecordError(t *testing.T) {
	m := NewMetrics()
	m.RecordError()
	m.RecordError()

	if m.TotalErrors.Load() != 2 {
		t.Errorf("expected 2 errors, got %d", m.TotalErrors.Load())
	}
}

func TestMetricsRecordBytes(t *testing.T) {
	m := NewMetrics()
	m.RecordBytesIn(100)
	m.RecordBytesOut(50)

	if m.TotalBytesIn.Load() != 100 {
		t.Errorf("expected 100 bytes in, got %d", m.TotalBytesIn.Load())
	}
	if m.TotalBytesOut.Load() != 50 {
		t.Errorf("expected 50 bytes out, got %d", m.TotalBytesOut.Load())
	}
}

func TestMetricsSnapshot(t *testing.T) {
	m := NewMetrics()
	m.RecordConnection()
	m.RecordCommand("GET", 1_000_000)
	m.RecordHit()

	snapshot := m.Snapshot()

	if snapshot["total_connections"] != int64(1) {
		t.Errorf("expected 1 connection in snapshot")
	}
	if snapshot["total_hits"] != int64(1) {
		t.Errorf("expected 1 hit in snapshot")
	}
}

func TestMetricsReset(t *testing.T) {
	m := NewMetrics()
	m.RecordConnection()
	m.RecordCommand("GET", 1_000_000)
	m.RecordHit()
	m.Reset()

	if m.TotalConnections.Load() != 0 {
		t.Errorf("expected 0 connections after reset")
	}
	if m.TotalCommands.Load() != 0 {
		t.Errorf("expected 0 commands after reset")
	}
}

func TestMetricsGetCommandStats(t *testing.T) {
	m := NewMetrics()
	m.RecordCommand("GET", 1_000_000)

	stats := m.GetCommandStats("GET")
	if stats == nil {
		t.Fatal("expected command stats")
	}
	if stats["count"] != int64(1) {
		t.Errorf("expected count 1, got %v", stats["count"])
	}
}

func TestMetricsGetCommandStatsNotFound(t *testing.T) {
	m := NewMetrics()
	stats := m.GetCommandStats("NONEXISTENT")
	if stats != nil {
		t.Errorf("expected nil for nonexistent command")
	}
}

func TestNewSlowLog(t *testing.T) {
	sl := NewSlowLog(128)
	if sl == nil {
		t.Fatal("expected SlowLog")
	}
	if sl.MaxSize != 128 {
		t.Errorf("expected max size 128, got %d", sl.MaxSize)
	}
}

func TestNewSlowLogZeroSize(t *testing.T) {
	sl := NewSlowLog(0)
	if sl.MaxSize != 128 {
		t.Errorf("expected default max size 128, got %d", sl.MaxSize)
	}
}

func TestSlowLogAdd(t *testing.T) {
	sl := NewSlowLog(10)
	sl.Add(100*time.Millisecond, "GET", []string{"key"}, "127.0.0.1")

	if sl.Len() != 1 {
		t.Errorf("expected 1 entry, got %d", sl.Len())
	}
}

func TestSlowLogGet(t *testing.T) {
	sl := NewSlowLog(10)
	sl.Add(100*time.Millisecond, "GET", []string{"key1"}, "127.0.0.1")
	sl.Add(200*time.Millisecond, "SET", []string{"key2", "value"}, "127.0.0.1")

	entries := sl.Get(10)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestSlowLogGetNegative(t *testing.T) {
	sl := NewSlowLog(10)
	sl.Add(100*time.Millisecond, "GET", []string{"key"}, "127.0.0.1")

	entries := sl.Get(-1)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

func TestSlowLogClear(t *testing.T) {
	sl := NewSlowLog(10)
	sl.Add(100*time.Millisecond, "GET", []string{"key"}, "127.0.0.1")
	sl.Clear()

	if sl.Len() != 0 {
		t.Errorf("expected 0 entries after clear, got %d", sl.Len())
	}
}

func TestSlowLogOverflow(t *testing.T) {
	sl := NewSlowLog(3)
	for i := 0; i < 10; i++ {
		sl.Add(time.Duration(i)*time.Millisecond, "GET", []string{"key"}, "127.0.0.1")
	}

	if sl.Len() != 3 {
		t.Errorf("expected 3 entries after overflow, got %d", sl.Len())
	}
}

func TestGlobalMetrics(t *testing.T) {
	if GlobalMetrics == nil {
		t.Error("expected GlobalMetrics to be initialized")
	}
}

func TestGlobalSlowLog(t *testing.T) {
	if GlobalSlowLog == nil {
		t.Error("expected GlobalSlowLog to be initialized")
	}
}

func TestNewTimingWheel(t *testing.T) {
	s := NewStore()
	tw := NewTimingWheel(s)
	if tw == nil {
		t.Fatal("expected TimingWheel")
	}
}

func TestTimingWheelAdd(t *testing.T) {
	s := NewStore()
	tw := NewTimingWheel(s)

	expiresAt := time.Now().Add(30 * time.Minute).UnixNano()
	tw.Add("key1", expiresAt)
}

func TestTimingWheelAddExpired(t *testing.T) {
	s := NewStore()
	tw := NewTimingWheel(s)

	tw.Add("key1", time.Now().Add(-1*time.Second).UnixNano())
}

func TestTimingWheelAddFarFuture(t *testing.T) {
	s := NewStore()
	tw := NewTimingWheel(s)

	expiresAt := time.Now().Add(400 * 24 * time.Hour).UnixNano()
	tw.Add("key1", expiresAt)
}

func TestTimingWheelRemove(t *testing.T) {
	s := NewStore()
	tw := NewTimingWheel(s)

	expiresAt := time.Now().Add(30 * time.Minute).UnixNano()
	tw.Add("key1", expiresAt)
	tw.Remove("key1")
}

func TestTimingWheelStartStop2(t *testing.T) {
	s := NewStore()
	tw := NewTimingWheel(s)

	tw.Start()
	time.Sleep(50 * time.Millisecond)
	tw.Stop()
}

func TestTimingWheelTick2(t *testing.T) {
	s := NewStore()
	tw := NewTimingWheel(s)

	tw.tick()
}
