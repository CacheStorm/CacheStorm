package store

import (
	"reflect"
	"testing"
)

func checkAudit50R25X(t *testing.T, label string, want, got interface{}) {
	t.Helper()
	t.Logf("%s EXPECTED: %v | ACTUAL: %v", label, want, got)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s mismatch", label)
	}
}

func TestAudit50R25X(t *testing.T) {
	defer func() {
		if t.Failed() {
			t.Log("PROBLEM CONFIRMED")
		} else {
			t.Log("FIX VERIFIED")
		}
	}()

	control := NewHistogram(0, 1, 0.25)
	control.Add(0.125)
	control.Add(0.375)
	checkAudit50R25X(t, "control exact bucket labels", map[string]int64{"[0,0.25)": 1, "[0.25,0.5)": 1}, control.Get())
	h := NewHistogram(0, 0.000001, 0.0000001)
	h.Add(0.00000001)
	h.Add(0.00000011)
	h.Add(0.00000021)
	got := h.Get()
	var total int64
	for _, count := range got {
		total += count
	}
	checkAudit50R25X(t, "small-width histogram retains all counts", int64(3), total)
	checkAudit50R25X(t, "small-width bins stay distinct", 3, len(got))

	for _, width := range []float64{0.00000001, 0.125, 0.5} {
		hist := NewHistogram(0, width*10, width)
		for i := 0; i < 4; i++ {
			hist.Add((float64(i) + 0.5) * width)
		}
		var sum int64
		for _, count := range hist.Get() {
			sum += count
		}
		checkAudit50R25X(t, "edge count conservation", int64(4), sum)
		checkAudit50R25X(t, "edge bins distinct", 4, len(hist.Get()))
		hist.Reset()
		checkAudit50R25X(t, "reset buckets", 0, len(hist.Get()))
	}
	td := NewTDigest(100)
	td.Add(2, 2)
	td.Add(8, 1)
	checkAudit50R25X(t, "weighted mean control", float64(4), td.Mean())
	td.Merge(td)
	checkAudit50R25X(t, "self merge count", float64(6), td.Count())
	rs := NewReservoirSampler(2)
	rs.AddBatch([]float64{1, 2, 3, 4})
	checkAudit50R25X(t, "reservoir cap", 2, rs.Size())
	checkAudit50R25X(t, "reservoir count", int64(4), rs.TotalCount())

}
