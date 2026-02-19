package stats

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/plugin"
)

type Stats struct {
	mu              sync.RWMutex
	TotalCommands   int64
	CommandCounts   map[string]int64
	HitCount        int64
	MissCount       int64
	KeysCreated     int64
	KeysDeleted     int64
	KeysExpired     int64
	KeysEvicted     int64
	Latencies       map[string][]time.Duration
	MaxLatencyStore int
}

type StatsPlugin struct {
	stats *Stats
}

func New() *StatsPlugin {
	return &StatsPlugin{
		stats: &Stats{
			CommandCounts:   make(map[string]int64),
			Latencies:       make(map[string][]time.Duration),
			MaxLatencyStore: 1000,
		},
	}
}

func (p *StatsPlugin) Name() string    { return "stats" }
func (p *StatsPlugin) Version() string { return "1.0.0" }

func (p *StatsPlugin) Init(config interface{}) error {
	return nil
}

func (p *StatsPlugin) Close() error {
	return nil
}

func (p *StatsPlugin) BeforeCommand(ctx *command.Context) error {
	atomic.AddInt64(&p.stats.TotalCommands, 1)

	p.stats.mu.Lock()
	p.stats.CommandCounts[ctx.Command]++
	p.stats.mu.Unlock()

	return nil
}

func (p *StatsPlugin) AfterCommand(ctx *command.Context) {
	latency := time.Since(ctx.StartTime)

	p.stats.mu.Lock()
	latencies := p.stats.Latencies[ctx.Command]
	latencies = append(latencies, latency)
	if len(latencies) > p.stats.MaxLatencyStore {
		latencies = latencies[1:]
	}
	p.stats.Latencies[ctx.Command] = latencies
	p.stats.mu.Unlock()
}

func (p *StatsPlugin) RecordHit() {
	atomic.AddInt64(&p.stats.HitCount, 1)
}

func (p *StatsPlugin) RecordMiss() {
	atomic.AddInt64(&p.stats.MissCount, 1)
}

func (p *StatsPlugin) RecordKeyCreated() {
	atomic.AddInt64(&p.stats.KeysCreated, 1)
}

func (p *StatsPlugin) RecordKeyDeleted() {
	atomic.AddInt64(&p.stats.KeysDeleted, 1)
}

func (p *StatsPlugin) RecordKeyExpired() {
	atomic.AddInt64(&p.stats.KeysExpired, 1)
}

func (p *StatsPlugin) RecordKeyEvicted() {
	atomic.AddInt64(&p.stats.KeysEvicted, 1)
}

func (p *StatsPlugin) GetStats() map[string]interface{} {
	p.stats.mu.RLock()
	defer p.stats.mu.RUnlock()

	hitCount := atomic.LoadInt64(&p.stats.HitCount)
	missCount := atomic.LoadInt64(&p.stats.MissCount)
	total := hitCount + missCount
	hitRatio := 0.0
	if total > 0 {
		hitRatio = float64(hitCount) / float64(total)
	}

	commandCounts := make(map[string]int64)
	for k, v := range p.stats.CommandCounts {
		commandCounts[k] = v
	}

	return map[string]interface{}{
		"total_commands": atomic.LoadInt64(&p.stats.TotalCommands),
		"command_counts": commandCounts,
		"hit_count":      hitCount,
		"miss_count":     missCount,
		"hit_ratio":      hitRatio,
		"keys_created":   atomic.LoadInt64(&p.stats.KeysCreated),
		"keys_deleted":   atomic.LoadInt64(&p.stats.KeysDeleted),
		"keys_expired":   atomic.LoadInt64(&p.stats.KeysExpired),
		"keys_evicted":   atomic.LoadInt64(&p.stats.KeysEvicted),
	}
}

var _ plugin.BeforeCommandHook = (*StatsPlugin)(nil)
var _ plugin.AfterCommandHook = (*StatsPlugin)(nil)
