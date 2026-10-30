package server

import (
	"runtime/debug"
	"testing"
)

func TestApplyDefaultGCPercent(t *testing.T) {
	prev := debug.SetGCPercent(100)
	defer debug.SetGCPercent(prev)

	t.Setenv("GOGC", "200")
	ApplyDefaultGCPercent()
	if got := debug.SetGCPercent(100); got != 100 {
		t.Fatalf("GOGC in the environment: percent = %d, want it left at 100", got)
	}

	t.Setenv("GOGC", "")
	ApplyDefaultGCPercent()
	if got := debug.SetGCPercent(100); got != DefaultGCPercent {
		t.Fatalf("percent = %d, want %d", got, DefaultGCPercent)
	}
}
