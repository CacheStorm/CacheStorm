package store

import (
	"testing"
	"time"
)

// Regression: nothing ever recorded into GlobalSlowLog — SLOWLOG.GET was
// always empty and SLOWLOG.LEN always 0. The connection loop now calls
// AddIfSlow for every dispatched command; these tests pin the recording
// semantics (threshold, enabled flag).

func TestSlowLogAddIfSlowRecordsSlowCommands(t *testing.T) {
	sl := NewSlowLog(10)
	sl.SetThreshold(10 * time.Millisecond)

	sl.AddIfSlow(50*time.Millisecond, "GET", [][]byte{[]byte("key")}, "127.0.0.1")

	if sl.Len() != 1 {
		t.Fatalf("expected slow command recorded, got %d entries", sl.Len())
	}
	entries := sl.Get(10)
	if len(entries) != 1 || entries[0].Command != "GET" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestSlowLogAddIfSlowSkipsFastCommands(t *testing.T) {
	sl := NewSlowLog(10)
	sl.SetThreshold(10 * time.Millisecond)

	sl.AddIfSlow(1*time.Microsecond, "GET", [][]byte{[]byte("key")}, "127.0.0.1")

	if sl.Len() != 0 {
		t.Fatalf("expected fast command skipped, got %d entries", sl.Len())
	}
}

func TestSlowLogSetThreshold(t *testing.T) {
	sl := NewSlowLog(10)
	sl.SetThreshold(10 * time.Millisecond)
	sl.AddIfSlow(1*time.Microsecond, "GET", nil, "127.0.0.1")
	if sl.Len() != 0 {
		t.Fatalf("expected below-threshold command skipped, got %d entries", sl.Len())
	}

	// Threshold 0 records everything (SLOWLOG.CONFIG THRESHOLD 0 semantics).
	sl.SetThreshold(0)
	sl.AddIfSlow(1*time.Microsecond, "SET", [][]byte{[]byte("k"), []byte("v")}, "127.0.0.1")
	if sl.Len() != 1 {
		t.Fatalf("expected threshold 0 to record everything, got %d entries", sl.Len())
	}
	entries := sl.Get(10)
	if len(entries) == 1 && (entries[0].Command != "SET" || string(entries[0].Args[0]) != "k") {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
}

func TestSlowLogSetEnabled(t *testing.T) {
	sl := NewSlowLog(10)
	sl.SetEnabled(false)
	sl.AddIfSlow(time.Hour, "GET", nil, "127.0.0.1")
	if sl.Len() != 0 {
		t.Fatalf("expected disabled slow log to skip recording, got %d entries", sl.Len())
	}

	sl.SetEnabled(true)
	sl.AddIfSlow(time.Hour, "GET", nil, "127.0.0.1")
	if sl.Len() != 1 {
		t.Fatalf("expected re-enabled slow log to record, got %d entries", sl.Len())
	}
}

// The shipped default config enables the slow log with a 10ms threshold.
func TestNewSlowLogDefaults(t *testing.T) {
	sl := NewSlowLog(100)
	if !sl.enabled {
		t.Fatal("expected slow log enabled by default")
	}
	if sl.threshold != 10*time.Millisecond {
		t.Fatalf("expected default threshold 10ms, got %v", sl.threshold)
	}
	if sl.MaxSize != 100 {
		t.Fatalf("expected MaxSize 100, got %d", sl.MaxSize)
	}
}
