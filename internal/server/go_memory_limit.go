package server

import (
	"math"
	"os"
	"runtime/debug"
	"runtime/metrics"
	"sync/atomic"
	"time"
)

// ManageGoMemoryLimit lets the server derive the Go runtime's soft memory
// limit from maxmemory. The binary turns it on; embedders and tests keep the
// runtime's own settings. A GOMEMLIMIT in the environment always wins.
var ManageGoMemoryLimit atomic.Bool

// DefaultGCPercent is the binary's GOGC unless the environment sets one.
// Go's 100 lets the heap reach twice the live data between collections; a
// dataset is long-lived, so collecting at one and a half times costs little
// CPU and keeps RSS closer to used_memory.
const DefaultGCPercent = 50

// ApplyDefaultGCPercent sets DefaultGCPercent when GOGC is not in the
// environment.
func ApplyDefaultGCPercent() {
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(DefaultGCPercent)
	}
}

// applyGoMemoryLimit makes the garbage collector work harder as the heap
// nears maxmemory instead of letting it grow to twice the live data, which
// is GOGC's default and what made RSS several times used_memory. The limit
// leaves room for buffers and the runtime above the dataset.
func applyGoMemoryLimit(maxMemory int64) {
	if !ManageGoMemoryLimit.Load() || os.Getenv("GOMEMLIMIT") != "" {
		return
	}
	if maxMemory <= 0 {
		debug.SetMemoryLimit(math.MaxInt64)
		return
	}
	debug.SetMemoryLimit(maxMemory + maxMemory/4 + 64<<20)
}

// heapFeedbackInterval is how often the server compares the live heap with
// used_memory; the heap's live size changes only once a collection ends.
const heapFeedbackInterval = 250 * time.Millisecond

// startHeapFeedback holds the live heap, not only used_memory, to
// maxmemory. used_memory counts each structure at fixed rates and the heap
// runs above it by what those miss; under a limit the gap left the runtime
// so little room below the Go memory limit that it collected without pause
// and throughput fell several times over.
func (s *Server) startHeapFeedback() {
	if !ManageGoMemoryLimit.Load() {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(heapFeedbackInterval)
		defer ticker.Stop()
		samples := []metrics.Sample{{Name: "/gc/cycles/total:gc-cycles"}, {Name: "/gc/heap/live:bytes"}}
		var lastCycle uint64
		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
			}
			mt := s.store.MemoryTracker()
			if mt == nil || mt.Max() <= 0 {
				continue
			}
			metrics.Read(samples)
			if samples[0].Value.Kind() != metrics.KindUint64 || samples[1].Value.Kind() != metrics.KindUint64 {
				return
			}
			cycle := samples[0].Value.Uint64()
			if cycle == lastCycle {
				continue
			}
			lastCycle = cycle
			if usage := mt.Usage(); usage >= 1<<20 {
				mt.SetHeapRatio(float64(samples[1].Value.Uint64()) / float64(usage))
			}
		}
	}()
}
