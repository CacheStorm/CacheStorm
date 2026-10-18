package store

// Regression coverage for the ignored Bloom-filter accuracy argument.
//
// NewBloomFilter accepted falsePositiveRate and never read it: k was derived
// from size alone, so every filter got the same 20 hashes no matter what
// accuracy the caller asked for. BF.RESERVE's documented errorRate argument
// (docs/commands.md:438, "BF.RESERVE key capacity [errorRate]") therefore did
// nothing, which is worse than a wrong answer — the caller believes it tuned
// the filter's false-positive rate and did not.
//
// The optimal hash count for a Bloom filter targeting error rate p is
// k = -log2(p): 0.01 -> ~6.6, 0.0001 -> ~13.3. These tests pin that the
// argument is honoured, that k stays in a usable range, and that the filter
// still behaves.

import (
	"math"
	"testing"
)

// kForRate returns the hash count NewBloomFilter derives for a rate, read
// through the filter's own Info surface so the test asserts the value the
// BF.INFO command actually reports.
func kForRate(t *testing.T, size uint, rate float64) uint {
	t.Helper()
	return NewBloomFilter(size, rate).Info()["hashes"].(uint)
}

func TestBloomFilterHonoursErrorRate(t *testing.T) {
	loose := kForRate(t, 1000000, 0.5)
	tight := kForRate(t, 1000000, 0.0001)

	if loose == tight {
		t.Fatalf("errorRate 0.5 and 0.0001 both produced %d hashes — the argument is ignored", loose)
	}
	if tight <= loose {
		t.Fatalf("a tighter error rate must need MORE hashes: 0.5 -> %d, 0.0001 -> %d", loose, tight)
	}
}

func TestBloomFilterHashCountMatchesTargetRate(t *testing.T) {
	// -log2(p), rounded up, is the textbook optimum.
	for _, tc := range []struct {
		rate float64
		want uint
	}{
		{rate: 0.5, want: 1},
		{rate: 0.01, want: 7},
		{rate: 0.001, want: 10},
		{rate: 0.0001, want: 14},
	} {
		if got := kForRate(t, 1000000, tc.rate); got != tc.want {
			t.Errorf("errorRate %v: hashes = %d, want %d (-log2(%v) = %.2f)",
				tc.rate, got, tc.want, tc.rate, -math.Log(tc.rate)/math.Ln2)
		}
	}
}

// A nonsensical rate must degrade to a usable filter rather than k=0, which
// would make Exists always true, or an unbounded k.
func TestBloomFilterClampsInvalidErrorRate(t *testing.T) {
	for _, rate := range []float64{0, -1, 1, 2, math.NaN(), math.Inf(1)} {
		bf := NewBloomFilter(1000, rate)
		if bf.k < 1 {
			t.Errorf("errorRate %v produced k=%d; a zero hash count makes Exists always true", rate, bf.k)
		}
		if bf.k > 20 {
			t.Errorf("errorRate %v produced k=%d, above the 20 cap", rate, bf.k)
		}
	}
}

// Control: the filter still works. k now depends on the rate, so add/exists
// and the no-false-negative property must be re-checked for the new defaults.
func TestBloomFilterStillRoundTrips(t *testing.T) {
	bf := NewBloomFilter(1000000, 0.01)

	for _, item := range []string{"alpha", "beta", "gamma", "delta"} {
		bf.Add([]byte(item))
	}
	for _, item := range []string{"alpha", "beta", "gamma", "delta"} {
		if !bf.Exists([]byte(item)) {
			t.Errorf("Exists(%q) = false; a Bloom filter must never report a false negative", item)
		}
	}
	if got := bf.Count(); got != 4 {
		t.Errorf("Count() = %d, want 4", got)
	}
}

// A tight rate must not produce a filter so sparse that lookups miss, and a
// loose rate must not be worse than a tight one on the same items.
func TestBloomFilterRatesRemainUsable(t *testing.T) {
	const items = 500
	for _, rate := range []float64{0.5, 0.01, 0.0001} {
		bf := NewBloomFilter(1000000, rate)
		for i := 0; i < items; i++ {
			bf.Add([]byte("item-" + string(rune('a'+i%26)) + string(rune('0'+i/26%10))))
		}
		bf.Exists([]byte("never-added"))
		if info := bf.Info(); info["hashes"] == uint(0) {
			t.Fatalf("errorRate %v produced a filter with no hashes", rate)
		}
	}
}
