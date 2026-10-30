package store

import (
	"testing"
	"time"
)

func TestNowUnixFollowsTheClock(t *testing.T) {
	if d := time.Now().Unix() - nowUnix(); d < 0 || d > 1 {
		t.Fatalf("coarse clock is %ds behind", d)
	}
}
