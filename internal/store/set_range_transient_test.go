package store

import (
	"slices"
	"strconv"
	"testing"
)

// RangeTransient yields what Range yields, in the same order for a compact
// set, and walks an intset without a string per member.
func TestSetRangeTransientAgreesWithRange(t *testing.T) {
	sets := map[string]*SetValue{"intset": {}, "listpack": {}, "hashtable": {}}
	for i := range 100 {
		sets["intset"].AddMember(strconv.Itoa(1_000_000 - i*37))
		sets["listpack"].AddMember("m" + strconv.Itoa(i))
	}
	for i := range 600 {
		sets["hashtable"].AddMember(strconv.Itoa(i))
	}
	sets["intset"].AddMember("-9223372036854775808")
	for enc, set := range sets {
		if got := set.Encoding(); got != enc {
			t.Fatalf("%s set has encoding %s", enc, got)
		}
		var want, got []string
		set.Range(func(m string) bool { want = append(want, m); return true })
		set.RangeTransient(func(m string) bool { got = append(got, string([]byte(m))); return true })
		if enc == "hashtable" {
			// A map walks in no fixed order.
			slices.Sort(got)
			slices.Sort(want)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("%s: RangeTransient %v, Range %v", enc, got[:3], want[:3])
		}
		n := 0
		set.RangeTransient(func(string) bool { n++; return n < 5 })
		if n != 5 {
			t.Fatalf("%s: stopping at 5 yielded %d", enc, n)
		}
	}
	set := sets["intset"]
	if allocs := testing.AllocsPerRun(20, func() { set.RangeTransient(func(string) bool { return true }) }); allocs > 1 {
		t.Fatalf("walking 101 intset members allocated %v times", allocs)
	}
}
