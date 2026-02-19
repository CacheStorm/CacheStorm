package store

import (
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
	maxMemory    int64
	currentUsage atomic.Int64
	warningPct   float64
	criticalPct  float64
	emergencyPct float64
}

func NewMemoryTracker(maxMemory int64, warningPct, criticalPct int) *MemoryTracker {
	return &MemoryTracker{
		maxMemory:    maxMemory,
		warningPct:   float64(warningPct) / 100,
		criticalPct:  float64(criticalPct) / 100,
		emergencyPct: 0.95,
	}
}

func (mt *MemoryTracker) Add(bytes int64) {
	mt.currentUsage.Add(bytes)
}

func (mt *MemoryTracker) Sub(bytes int64) {
	mt.currentUsage.Add(-bytes)
}

func (mt *MemoryTracker) Usage() int64 {
	return mt.currentUsage.Load()
}

func (mt *MemoryTracker) Max() int64 {
	return mt.maxMemory
}

func (mt *MemoryTracker) Pressure() PressureLevel {
	if mt.maxMemory == 0 {
		return PressureNormal
	}

	usage := mt.currentUsage.Load()
	pct := float64(usage) / float64(mt.maxMemory)

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
	if mt.maxMemory == 0 {
		return true
	}

	current := mt.currentUsage.Load()
	newUsage := current + bytes

	if float64(newUsage)/float64(mt.maxMemory) >= mt.emergencyPct {
		return false
	}

	return true
}

func (mt *MemoryTracker) PressurePercent() float64 {
	if mt.maxMemory == 0 {
		return 0
	}
	return float64(mt.currentUsage.Load()) / float64(mt.maxMemory) * 100
}
