package store

import (
	"reflect"
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

	ts := NewTimeSeriesValue(0)
	ts.Add(30, 3)
	ts.Add(10, 1)
	ts.Add(20, 2)
	checkAudit50R16X(t, "inclusive ordered range", []TimeSeriesSample{{Timestamp: 10, Value: 1}, {Timestamp: 20, Value: 2}}, ts.Range(10, 20))
	checkAudit50R16X(t, "count boundary", 1, len(ts.RangeWithCount(10, 30, 1)))

}
