package store

import (
	"sync"
	"sync/atomic"
	"time"
)

type Metrics struct {
	TotalConnections  atomic.Int64
	ActiveConnections atomic.Int64
	TotalCommands     atomic.Int64
	TotalReads        atomic.Int64
	TotalWrites       atomic.Int64
	TotalHits         atomic.Int64
	TotalMisses       atomic.Int64
	TotalErrors       atomic.Int64
	TotalBytesIn      atomic.Int64
	TotalBytesOut     atomic.Int64
	StartTime         time.Time
	CommandCounts     map[string]*atomic.Int64
	CommandLatencies  map[string]*LatencyTracker
	mu                sync.RWMutex
}

type LatencyTracker struct {
	Count   atomic.Int64
	Total   atomic.Int64
	Min     atomic.Int64
	Max     atomic.Int64
	Buckets []atomic.Int64
}

func NewLatencyTracker() *LatencyTracker {
	lt := &LatencyTracker{
		Buckets: make([]atomic.Int64, 16),
	}
	lt.Min.Store(int64(^uint64(0) >> 1))
	return lt
}

func (lt *LatencyTracker) Record(latencyNs int64) {
	lt.Count.Add(1)
	lt.Total.Add(latencyNs)

	for {
		current := lt.Min.Load()
		if latencyNs >= current || lt.Min.CompareAndSwap(current, latencyNs) {
			break
		}
	}

	for {
		current := lt.Max.Load()
		if latencyNs <= current || lt.Max.CompareAndSwap(current, latencyNs) {
			break
		}
	}

	bucket := latencyNs / 1_000_000
	if bucket > 15 {
		bucket = 15
	}
	lt.Buckets[bucket].Add(1)
}

func (lt *LatencyTracker) Stats() map[string]interface{} {
	count := lt.Count.Load()
	if count == 0 {
		return map[string]interface{}{
			"count": int64(0),
			"avg":   int64(0),
			"min":   int64(0),
			"max":   int64(0),
		}
	}

	buckets := make(map[string]int64)
	bucketLabels := []string{"0-1ms", "1-2ms", "2-3ms", "3-4ms", "4-5ms", "5-6ms", "6-7ms", "7-8ms",
		"8-9ms", "9-10ms", "10-11ms", "11-12ms", "12-13ms", "13-14ms", "14-15ms", "15ms+"}
	for i, label := range bucketLabels {
		buckets[label] = lt.Buckets[i].Load()
	}

	return map[string]interface{}{
		"count":   count,
		"avg":     lt.Total.Load() / count,
		"min":     lt.Min.Load(),
		"max":     lt.Max.Load(),
		"buckets": buckets,
	}
}

func NewMetrics() *Metrics {
	return &Metrics{
		StartTime:        time.Now(),
		CommandCounts:    make(map[string]*atomic.Int64),
		CommandLatencies: make(map[string]*LatencyTracker),
	}
}

func (m *Metrics) RecordConnection() {
	m.TotalConnections.Add(1)
	m.ActiveConnections.Add(1)
}

func (m *Metrics) RecordDisconnection() {
	m.ActiveConnections.Add(-1)
}

func (m *Metrics) RecordCommand(cmd string, latencyNs int64) {
	m.TotalCommands.Add(1)

	m.mu.Lock()
	if _, exists := m.CommandCounts[cmd]; !exists {
		m.CommandCounts[cmd] = &atomic.Int64{}
		m.CommandLatencies[cmd] = NewLatencyTracker()
	}
	m.mu.Unlock()

	m.CommandCounts[cmd].Add(1)
	m.CommandLatencies[cmd].Record(latencyNs)
}

func (m *Metrics) RecordRead() {
	m.TotalReads.Add(1)
}

func (m *Metrics) RecordWrite() {
	m.TotalWrites.Add(1)
}

func (m *Metrics) RecordHit() {
	m.TotalHits.Add(1)
}

func (m *Metrics) RecordMiss() {
	m.TotalMisses.Add(1)
}

func (m *Metrics) RecordError() {
	m.TotalErrors.Add(1)
}

func (m *Metrics) RecordBytesIn(n int64) {
	m.TotalBytesIn.Add(n)
}

func (m *Metrics) RecordBytesOut(n int64) {
	m.TotalBytesOut.Add(n)
}

func (m *Metrics) Snapshot() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cmdStats := make(map[string]interface{})
	for cmd, counter := range m.CommandCounts {
		cmdStats[cmd] = map[string]interface{}{
			"count":   counter.Load(),
			"latency": m.CommandLatencies[cmd].Stats(),
		}
	}

	hitRate := float64(0)
	total := m.TotalHits.Load() + m.TotalMisses.Load()
	if total > 0 {
		hitRate = float64(m.TotalHits.Load()) / float64(total) * 100
	}

	uptime := time.Since(m.StartTime)

	return map[string]interface{}{
		"uptime_seconds":     int64(uptime.Seconds()),
		"total_connections":  m.TotalConnections.Load(),
		"active_connections": m.ActiveConnections.Load(),
		"total_commands":     m.TotalCommands.Load(),
		"total_reads":        m.TotalReads.Load(),
		"total_writes":       m.TotalWrites.Load(),
		"total_hits":         m.TotalHits.Load(),
		"total_misses":       m.TotalMisses.Load(),
		"hit_rate":           hitRate,
		"total_errors":       m.TotalErrors.Load(),
		"bytes_in":           m.TotalBytesIn.Load(),
		"bytes_out":          m.TotalBytesOut.Load(),
		"commands_per_sec":   float64(m.TotalCommands.Load()) / uptime.Seconds(),
		"command_stats":      cmdStats,
	}
}

func (m *Metrics) Reset() {
	m.TotalConnections.Store(0)
	m.ActiveConnections.Store(0)
	m.TotalCommands.Store(0)
	m.TotalReads.Store(0)
	m.TotalWrites.Store(0)
	m.TotalHits.Store(0)
	m.TotalMisses.Store(0)
	m.TotalErrors.Store(0)
	m.TotalBytesIn.Store(0)
	m.TotalBytesOut.Store(0)
	m.StartTime = time.Now()

	m.mu.Lock()
	m.CommandCounts = make(map[string]*atomic.Int64)
	m.CommandLatencies = make(map[string]*LatencyTracker)
	m.mu.Unlock()
}

func (m *Metrics) GetCommandStats(cmd string) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counter, exists := m.CommandCounts[cmd]
	if !exists {
		return nil
	}

	return map[string]interface{}{
		"count":   counter.Load(),
		"latency": m.CommandLatencies[cmd].Stats(),
	}
}

type SlowLog struct {
	Entries  []SlowLogEntry
	MaxSize  int
	mu       sync.RWMutex
	sequence atomic.Int64
}

type SlowLogEntry struct {
	ID        int64
	Timestamp time.Time
	Duration  time.Duration
	Command   string
	Args      []string
	ClientIP  string
}

func NewSlowLog(maxSize int) *SlowLog {
	if maxSize <= 0 {
		maxSize = 128
	}
	return &SlowLog{
		Entries: make([]SlowLogEntry, 0, maxSize),
		MaxSize: maxSize,
	}
}

func (sl *SlowLog) Add(duration time.Duration, cmd string, args []string, clientIP string) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	entry := SlowLogEntry{
		ID:        sl.sequence.Add(1),
		Timestamp: time.Now(),
		Duration:  duration,
		Command:   cmd,
		Args:      args,
		ClientIP:  clientIP,
	}

	sl.Entries = append(sl.Entries, entry)

	if len(sl.Entries) > sl.MaxSize {
		sl.Entries = sl.Entries[len(sl.Entries)-sl.MaxSize:]
	}
}

func (sl *SlowLog) Get(n int) []SlowLogEntry {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	if n <= 0 || n > len(sl.Entries) {
		n = len(sl.Entries)
	}

	result := make([]SlowLogEntry, n)
	copy(result, sl.Entries[len(sl.Entries)-n:])
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

func (sl *SlowLog) Clear() {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	sl.Entries = sl.Entries[:0]
}

func (sl *SlowLog) Len() int {
	sl.mu.RLock()
	defer sl.mu.RUnlock()
	return len(sl.Entries)
}

var GlobalMetrics = NewMetrics()
var GlobalSlowLog = NewSlowLog(128)
