package server

import "testing"

// Free pages go back to the system only when there are enough of them and
// allocation was quiet.
func TestShouldTrim(t *testing.T) {
	const mb = 1 << 20
	for _, c := range []struct {
		allocated, free, live uint64
		want                  bool
	}{
		{1 * mb, 40 * mb, 100 * mb, true},     // quiet, a lot free
		{1 * mb, 10 * mb, 100 * mb, false},    // little free
		{50 * mb, 40 * mb, 100 * mb, false},   // busy
		{1 * mb, 100 * mb, 4000 * mb, false},  // free under an eighth of a big heap
		{100 * mb, 900 * mb, 4000 * mb, true}, // a big heap's quiet is relative
	} {
		if got := shouldTrim(c.allocated, c.free, c.live); got != c.want {
			t.Errorf("shouldTrim(%d, %d, %d) = %v", c.allocated/mb, c.free/mb, c.live/mb, got)
		}
	}
}
