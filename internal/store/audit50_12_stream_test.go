package store

import (
	"reflect"
	"testing"
)

func checkAudit50R12X(t *testing.T, label string, want, got interface{}) {
	t.Helper()
	t.Logf("%s EXPECTED: %v | ACTUAL: %v", label, want, got)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s mismatch", label)
	}
}

func TestAudit50R12X(t *testing.T) {
	defer func() {
		if t.Failed() {
			t.Log("PROBLEM CONFIRMED")
		} else {
			t.Log("FIX VERIFIED")
		}
	}()
	v := NewStreamValue(0)
	for _, id := range []string{"9-0", "10-0", "10-9", "10-10"} {
		if _, err := v.Add(id, nil); err != nil {
			t.Fatal(err)
		}
	}
	checkAudit50R12X(t, "control numeric inclusive range", 4, len(v.GetRange("9-0", "10-10", 0)))
	ids := func(entries []*StreamEntry) []string {
		result := make([]string, 0, len(entries))
		for _, e := range entries {
			result = append(result, e.ID)
		}
		return result
	}
	checkAudit50R12X(t, "later millisecond IDs", []string{"10-0", "10-9", "10-10"}, ids(v.GetEntriesAfter("9-0", 0)))
	checkAudit50R12X(t, "later sequence ID", []string{"10-10"}, ids(v.GetEntriesAfter("10-9", 0)))
	g := NewConsumerGroup("g")
	g.GetOrCreateConsumer("c")
	for _, id := range []string{"9-0", "10-0", "10-9", "10-10"} {
		g.AddPending(id, "c")
	}
	checkAudit50R12X(t, "control unbounded pending", 4, len(g.GetPending("-", "+", 0)))
	checkAudit50R12X(t, "pending numeric millisecond range", 4, len(g.GetPending("9-0", "10-10", 0)))
	checkAudit50R12X(t, "pending numeric sequence range", 2, len(g.GetPending("10-9", "10-10", 0)))
	checkAudit50R12X(t, "exclusive boundary", []string{"10-0"}, ids(v.GetEntriesAfter("9-0", 1)))
	checkAudit50R12X(t, "last entry has no successor", 0, len(v.GetEntriesAfter("10-10", 1)))
	checkAudit50R12X(t, "pending exact inclusive boundary", 1, len(g.GetPending("10-10", "10-10", 0)))
	checkAudit50R12X(t, "pending reversed range empty", 0, len(g.GetPending("10-10", "10-9", 0)))
	checkAudit50R12X(t, "pending limited count retained", 1, len(g.GetPending("9-0", "10-10", 1)))
	checkAudit50R12X(t, "empty stream selection", 0, len(NewStreamValue(0).GetEntriesAfter("0-0", 0)))

}
