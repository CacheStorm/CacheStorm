package command

import (
	"math"
	"testing"
)

// ZSCAN writes a skiplist's scores as Redis' ld2string(LD_STR_AUTO): %.17Lg,
// with the infinities spelled out. The expected texts are
// Redis 7.4's.
func TestFormatScanScoreMatchesRedis(t *testing.T) {
	for _, c := range []struct {
		f    float64
		want string
	}{
		{7.4618086549232663, "7.4618086549232663"}, {0.1, "0.10000000000000001"}, {2.5, "2.5"},
		{1e20, "1e+20"}, {1e16, "10000000000000000"}, {123456789012345678, "1.2345678901234568e+17"},
		{1e-5, "1.0000000000000001e-05"}, {4, "4"}, {math.Copysign(0, -1), "-0"}, {0, "0"},
		{math.Inf(1), "inf"}, {math.Inf(-1), "-inf"},
	} {
		if got := formatScanScore(c.f); got != c.want {
			t.Errorf("%v: %q, want %q", c.f, got, c.want)
		}
	}
}
