package store

import (
	"testing"
	"time"
)

func TestMemoryTracker(t *testing.T) {
	mt := NewMemoryTracker(1024*1024, 70, 85)

	mt.Add(512 * 1024)
	if mt.Usage() != 512*1024 {
		t.Errorf("expected 512KB usage, got %d", mt.Usage())
	}

	mt.Sub(256 * 1024)
	if mt.Usage() != 256*1024 {
		t.Errorf("expected 256KB usage, got %d", mt.Usage())
	}
}

func TestMemoryTrackerPressure(t *testing.T) {
	mt := NewMemoryTracker(1000, 70, 85)

	if mt.Pressure() != PressureNormal {
		t.Errorf("expected normal pressure, got %v", mt.Pressure())
	}

	mt.Add(750)
	if mt.Pressure() != PressureWarning {
		t.Errorf("expected warning pressure, got %v", mt.Pressure())
	}

	mt.Add(150)
	if mt.Pressure() != PressureCritical {
		t.Errorf("expected critical pressure, got %v", mt.Pressure())
	}
}

func TestMemoryTrackerCanAllocate(t *testing.T) {
	mt := NewMemoryTracker(1000, 70, 85)

	if !mt.CanAllocate(500) {
		t.Error("should be able to allocate 500")
	}

	mt.Add(900)
	if mt.CanAllocate(500) {
		t.Error("should not be able to allocate 500 when near limit")
	}
}

func TestTimingWheelAddRemove(t *testing.T) {
	s := NewStore()
	tw := NewTimingWheel(s)

	tw.Add("key1", time.Now().Add(10*time.Second).UnixNano())
	tw.Remove("key1")
}

func TestTimingWheelStartStop(t *testing.T) {
	s := NewStore()
	tw := NewTimingWheel(s)

	tw.Start()
	time.Sleep(200 * time.Millisecond)
	tw.Stop()
}
