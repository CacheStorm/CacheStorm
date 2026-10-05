package store

import (
	"fmt"
	"reflect"
	"testing"
)

func checkAudit50R17X(t *testing.T, label string, want, got interface{}) {
	t.Helper()
	t.Logf("%s EXPECTED: %v | ACTUAL: %v", label, want, got)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s mismatch", label)
	}
}

func TestAudit50R17X(t *testing.T) {
	defer func() {
		if t.Failed() {
			t.Log("PROBLEM CONFIRMED")
		} else {
			t.Log("FIX VERIFIED")
		}
	}()
	control := NewCuckooFilter(4, 2)
	checkAudit50R17X(t, "control insertion", true, control.Add([]byte("control")))
	checkAudit50R17X(t, "control lookup", true, control.Exists([]byte("control")))
	cf := NewCuckooFilter(4, 2)
	failed := false
	for i := 0; i < 30; i++ {
		before := make([][]byte, len(cf.buckets))
		for j := range before {
			before[j] = append([]byte(nil), cf.buckets[j]...)
		}
		oldCount := cf.Count()
		if !cf.Add([]byte(fmt.Sprintf("item-%d", i))) {
			failed = true
			checkAudit50R17X(t, "failed insertion keeps count", oldCount, cf.Count())
			checkAudit50R17X(t, "failed insertion preserves accepted fingerprints", before, cf.buckets)
			break
		}
	}
	checkAudit50R17X(t, "bounded full table reached", true, failed)

	for _, dims := range [][2]uint{{1, 1}, {2, 1}, {8, 2}} {
		f := NewCuckooFilter(dims[0], dims[1])
		f.kicks = 1
		for i := 0; i < 30; i++ {
			before := make([][]byte, len(f.buckets))
			for j := range before {
				before[j] = append([]byte(nil), f.buckets[j]...)
			}
			if !f.Add([]byte(fmt.Sprintf("edge-%d", i))) {
				checkAudit50R17X(t, "single-kick rollback", before, f.buckets)
				checkAudit50R17X(t, "repeated failed insertion", false, f.Add([]byte(fmt.Sprintf("edge-%d", i))))
				checkAudit50R17X(t, "repeated rollback", before, f.buckets)
				break
			}
		}
	}
	f := NewCuckooFilter(4, 2)
	f.Add([]byte("one"))
	checkAudit50R17X(t, "successful delete", true, f.Delete([]byte("one")))
	checkAudit50R17X(t, "delete count", uint(0), f.Count())
	bf := NewBloomFilter(256, 0.01)
	bf.Add([]byte("one"))
	checkAudit50R17X(t, "bloom membership control", true, bf.Exists([]byte("one")))
	bf.Clear()
	checkAudit50R17X(t, "bloom clear", uint(0), bf.Count())

}
