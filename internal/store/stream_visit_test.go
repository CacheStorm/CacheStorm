package store

import (
	"fmt"
	"slices"
	"testing"
)

// VisitRange yields the entries Range returns, in the same order and with
// the same fields, across nodes, deletions, entries sharing their node's
// fields or not, and entries with no field list.
func TestStreamVisitRangeAgreesWithRange(t *testing.T) {
	v := NewStreamValue(0)
	for i := 1; i <= 450; i++ {
		var pairs []StreamField
		switch {
		case i%9 == 0:
			pairs = nil
		case i%5 == 0:
			pairs = []StreamField{{"z", []byte(fmt.Sprint(i))}, {"z", nil}, {"q", []byte("x")}}
		default:
			pairs = []StreamField{{"a", []byte(fmt.Sprint(i))}, {"b", []byte{}}}
		}
		if _, err := v.AddEntry(StreamAddArgs{ID: StreamID{Ms: uint64(i / 3), Seq: uint64(i)}, IDGiven: true, SeqGiven: true}, pairs, 0); err != nil {
			t.Fatal(err)
		}
	}
	for i := 7; i <= 450; i += 11 {
		v.DeleteIDs([]StreamID{{Ms: uint64(i / 3), Seq: uint64(i)}})
	}
	type flat struct {
		id     StreamID
		fields []string
	}
	for _, c := range []struct {
		start, end StreamID
		count      int64
	}{
		{StreamID{}, MaxStreamID, 0}, {StreamID{}, MaxStreamID, 25}, {StreamID{Ms: 40}, StreamID{Ms: 90, Seq: 260}, 0},
		{StreamID{Ms: 100}, StreamID{Ms: 120}, 7}, {StreamID{Ms: 300}, MaxStreamID, 0}, {StreamID{Ms: 9}, StreamID{Ms: 8}, 0},
	} {
		for _, rev := range []bool{false, true} {
			var want, got []flat
			for _, e := range v.Range(c.start, c.end, c.count, rev) {
				f := flat{id: e.streamID(), fields: []string{}}
				for _, p := range e.FieldPairs() {
					f.fields = append(f.fields, p.Name, string(p.Value))
				}
				want = append(want, f)
			}
			announced := -1
			v.VisitRange(c.start, c.end, c.count, rev, func(n int) { announced = n }, func(e *StreamVisit) {
				f := flat{id: e.ID, fields: []string{}}
				e.EachField(func(name, value []byte) { f.fields = append(f.fields, string(name), string(value)) })
				if len(f.fields) != 2*e.FieldCount() {
					t.Fatalf("%v: FieldCount %d, %d fields", e.ID, e.FieldCount(), len(f.fields)/2)
				}
				got = append(got, f)
			})
			if announced != len(got) || !slices.EqualFunc(got, want, func(a, b flat) bool { return a.id == b.id && slices.Equal(a.fields, b.fields) }) {
				t.Fatalf("%+v rev %v: announced %d, visited %d, Range %d entries", c, rev, announced, len(got), len(want))
			}
			if c.count == 0 && c.start == (StreamID{}) && len(got) < 400 {
				t.Fatalf("whole stream visited only %d entries", len(got))
			}
		}
	}
}
