package store

import (
	"reflect"
	"sort"
	"testing"
)

func checkAudit50R16X(t *testing.T, label string, want, got interface{}) {
	t.Helper()
	t.Logf("%s EXPECTED: %v | ACTUAL: %v", label, want, got)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s mismatch", label)
	}
}

func TestAudit50R16X(t *testing.T) {
	defer func() {
		if t.Failed() {
			t.Log("PROBLEM CONFIRMED")
		} else {
			t.Log("FIX VERIFIED")
		}
	}()

	m := NewTimeSeriesManager()
	m.Create("absent", 0, nil)
	m.Create("empty", 0, map[string]string{"sensor": ""})
	m.Create("named", 0, map[string]string{"sensor": "cpu"})
	checkAudit50R16X(t, "control named label", []string{"named"}, m.QueryByLabels(map[string]string{"sensor": "cpu"}, ""))
	got := m.QueryByLabels(map[string]string{"sensor": ""}, "")
	sort.Strings(got)
	checkAudit50R16X(t, "empty label requires key presence", []string{"empty"}, got)

	checkAudit50R16X(t, "unknown empty label", 0, len(m.QueryByLabels(map[string]string{"unknown": ""}, "")))
	checkAudit50R16X(t, "no filters returns all", 3, len(m.QueryByLabels(nil, "")))
	checkAudit50R16X(t, "AND filter", 0, len(m.QueryByLabels(map[string]string{"sensor": "", "host": "x"}, "")))
	checkAudit50R16X(t, "delete empty series", true, m.Delete("empty"))
	checkAudit50R16X(t, "deleted label no match", 0, len(m.QueryByLabels(map[string]string{"sensor": ""}, "")))
	ts := NewTimeSeriesValue(0)
	ts.Add(30, 3)
	ts.Add(10, 1)
	ts.Add(20, 2)
	checkAudit50R16X(t, "inclusive ordered range", []TimeSeriesSample{{Timestamp: 10, Value: 1}, {Timestamp: 20, Value: 2}}, ts.Range(10, 20))
	checkAudit50R16X(t, "count boundary", 1, len(ts.RangeWithCount(10, 30, 1)))

}
