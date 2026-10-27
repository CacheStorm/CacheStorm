package store

import (
	"math"
	"sync/atomic"
)

type PressureLevel int

const (
	PressureNormal PressureLevel = iota
	PressureWarning
	PressureCritical
	PressureEmergency
)

type MemoryTracker struct {
	maxMemory    atomic.Int64
	currentUsage atomic.Int64
	// peak is the highest usage seen, INFO's used_memory_peak.
	peak         atomic.Int64
	warningPct   float64
	criticalPct  float64
	emergencyPct float64
}

func NewMemoryTracker(maxMemory int64, warningPct, criticalPct int) *MemoryTracker {
	mt := &MemoryTracker{
		warningPct:   float64(warningPct) / 100,
		criticalPct:  float64(criticalPct) / 100,
		emergencyPct: 0.95,
	}
	mt.maxMemory.Store(maxMemory)
	return mt
}

// SetMax changes the memory limit; 0 removes it. Usage is unaffected, so a
// lower limit is enforced by the next write that needs memory.
func (mt *MemoryTracker) SetMax(maxMemory int64) {
	mt.maxMemory.Store(maxMemory)
}

func (mt *MemoryTracker) Add(bytes int64) {
	mt.notePeak(mt.currentUsage.Add(bytes))
}

func (mt *MemoryTracker) notePeak(usage int64) {
	for {
		peak := mt.peak.Load()
		if usage <= peak || mt.peak.CompareAndSwap(peak, usage) {
			return
		}
	}
}

// Peak returns the highest usage seen since the tracker was created or the
// peak was reset.
func (mt *MemoryTracker) Peak() int64 {
	return mt.peak.Load()
}

// ResetPeak sets the peak to the current usage (CONFIG RESETSTAT).
func (mt *MemoryTracker) ResetPeak() {
	mt.peak.Store(mt.currentUsage.Load())
}

func (mt *MemoryTracker) Sub(bytes int64) {
	mt.currentUsage.Add(-bytes)
}

func (mt *MemoryTracker) Usage() int64 {
	return mt.currentUsage.Load()
}

func (mt *MemoryTracker) Max() int64 {
	return mt.maxMemory.Load()
}

func (mt *MemoryTracker) Pressure() PressureLevel {
	maxMemory := mt.maxMemory.Load()
	if maxMemory <= 0 {
		return PressureNormal
	}

	usage := mt.currentUsage.Load()
	pct := float64(usage) / float64(maxMemory)

	switch {
	case pct >= mt.emergencyPct:
		return PressureEmergency
	case pct >= mt.criticalPct:
		return PressureCritical
	case pct >= mt.warningPct:
		return PressureWarning
	default:
		return PressureNormal
	}
}

func (mt *MemoryTracker) CanAllocate(bytes int64) bool {
	maxMemory := mt.maxMemory.Load()
	if maxMemory <= 0 {
		return true
	}

	current := mt.currentUsage.Load()
	newUsage := current + bytes
	if bytes > 0 && newUsage < current {
		return false
	}

	return float64(newUsage)/float64(maxMemory) < mt.emergencyPct
}

func (mt *MemoryTracker) PressurePercent() float64 {
	maxMemory := mt.maxMemory.Load()
	if maxMemory <= 0 {
		return 0
	}
	return float64(mt.currentUsage.Load()) / float64(maxMemory) * 100
}

func (mt *MemoryTracker) withinLimit(usage int64) bool {
	maxMemory := mt.maxMemory.Load()
	return maxMemory <= 0 || float64(usage)/float64(maxMemory) < mt.emergencyPct
}

func (mt *MemoryTracker) TryReserve(delta int64) bool {
	if delta == 0 {
		return true
	}
	for {
		current := mt.currentUsage.Load()
		if delta > 0 && current > math.MaxInt64-delta {
			return false
		}
		if delta < 0 && current < math.MinInt64-delta {
			return false
		}
		next := current + delta
		if delta > 0 && !mt.withinLimit(next) {
			return false
		}
		if mt.currentUsage.CompareAndSwap(current, next) {
			mt.notePeak(next)
			return true
		}
	}
}
