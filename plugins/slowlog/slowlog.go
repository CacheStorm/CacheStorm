package slowlog

import (
	"container/ring"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/plugin"
)

type SlowLogEntry struct {
	ID         int64
	StartTime  time.Time
	Duration   time.Duration
	Command    string
	Args       [][]byte
	ClientAddr string
}

type SlowLogPlugin struct {
	mu         sync.RWMutex
	threshold  time.Duration
	maxEntries int
	entries    *ring.Ring
	nextID     int64
}

func New(threshold time.Duration, maxEntries int) *SlowLogPlugin {
	return &SlowLogPlugin{
		threshold:  threshold,
		maxEntries: maxEntries,
		entries:    ring.New(maxEntries),
	}
}

func (s *SlowLogPlugin) Name() string    { return "slowlog" }
func (s *SlowLogPlugin) Version() string { return "1.0.0" }

func (s *SlowLogPlugin) Init(config interface{}) error {
	return nil
}

func (s *SlowLogPlugin) Close() error {
	return nil
}

func (s *SlowLogPlugin) AfterCommand(ctx *command.Context) {
	duration := time.Since(ctx.StartTime)

	if duration >= s.threshold {
		s.addEntry(SlowLogEntry{
			ID:        s.nextID,
			StartTime: ctx.StartTime,
			Duration:  duration,
			Command:   ctx.Command,
			Args:      ctx.Args,
		})
		s.nextID++
	}
}

func (s *SlowLogPlugin) addEntry(entry SlowLogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries.Value = entry
	s.entries = s.entries.Next()
}

func (s *SlowLogPlugin) GetEntries(count int) []SlowLogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if count <= 0 || count > s.maxEntries {
		count = s.maxEntries
	}

	entries := make([]SlowLogEntry, 0, count)
	current := s.entries

	for i := 0; i < s.maxEntries && len(entries) < count; i++ {
		if current.Value != nil {
			entries = append(entries, current.Value.(SlowLogEntry))
		}
		current = current.Prev()
	}

	return entries
}

func (s *SlowLogPlugin) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	current := s.entries

	for i := 0; i < s.maxEntries; i++ {
		if current.Value != nil {
			count++
		}
		current = current.Next()
	}

	return count
}

func (s *SlowLogPlugin) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries = ring.New(s.maxEntries)
}

func (s *SlowLogPlugin) SetThreshold(threshold time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.threshold = threshold
}

var _ plugin.AfterCommandHook = (*SlowLogPlugin)(nil)
