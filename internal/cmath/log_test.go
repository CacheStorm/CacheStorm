package cmath

import (
	"math"
	"testing"
)

// The expected values are glibc's log, which is correctly rounded.
func TestLogMatchesGlibc(t *testing.T) {
	for _, c := range []struct{ x, want float64 }{
		{float64(float32(3.4285715)), 1.2321436912267398},
		{1, 0},
		{2, math.Ln2},
		{10, 2.302585092994046},
		{0.5, -math.Ln2},
		{1e-300, -690.7755278982137},
	} {
		if got := Log(c.x); got != c.want {
			t.Errorf("Log(%v) = %v, want %v", c.x, got, c.want)
		}
	}
	if !math.IsNaN(Log(-1)) || !math.IsInf(Log(0), -1) {
		t.Error("Log of a non-positive number")
	}
}
