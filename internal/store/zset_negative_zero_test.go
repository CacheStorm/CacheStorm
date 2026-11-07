package store

import (
	"math"
	"testing"
)

// A listpack drops the sign of a zero score, as Redis stores it as the
// integer 0; a set already in the map form keeps it.
func TestZeroScoreSignByEncoding(t *testing.T) {
	negZero := math.Copysign(0, -1)
	small := NewSortedSet(0)
	small.Add("a", negZero)
	if s, _ := small.GetScore("a"); math.Signbit(s) {
		t.Fatal("listpack kept -0")
	}
	big := NewSortedSet(0)
	for i := range 200 {
		big.Add(string(rune('A'+i%26))+string(rune('a'+i/26)), float64(i))
	}
	big.Add("z", negZero)
	if s, _ := big.GetScore("z"); !math.Signbit(s) {
		t.Fatal("skiplist lost -0")
	}
}
