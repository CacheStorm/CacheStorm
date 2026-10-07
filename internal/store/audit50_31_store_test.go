package store

import (
	"reflect"
	"testing"
)

func checkAudit50R31X(t *testing.T, label string, want, got interface{}) {
	t.Helper()
	t.Logf("%s EXPECTED: %v | ACTUAL: %v", label, want, got)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s mismatch", label)
	}
}

func TestAudit50R31X(t *testing.T) {
	defer func() {
		if t.Failed() {
			t.Log("PROBLEM CONFIRMED")
		} else {
			t.Log("FIX VERIFIED")
		}
	}()

	s := NewStore()
	s.Set("control", &StringValue{Data: []byte("v")}, SetOptions{Tags: []string{"control-tag"}})
	s.Delete("control")
	checkAudit50R31X(t, "control delete removes tags", 0, s.GetTagIndex().Count("control-tag"))
	s.Set("tagged", &StringValue{Data: []byte("v")}, SetOptions{Tags: []string{"one", "two"}})
	s.Flush()
	checkAudit50R31X(t, "flush removed keys", int64(0), s.KeyCount())
	checkAudit50R31X(t, "flush removes first tag membership", 0, s.GetTagIndex().Count("one"))
	checkAudit50R31X(t, "flush removes second tag membership", 0, s.GetTagIndex().Count("two"))

	s.GetTagIndex().Link("parent", "child")
	calls := 0
	s.SetHooks(StoreHooks{OnTagInvalidate: func(tag string, keys []string) { calls++ }})
	for i := 0; i < 3; i++ {
		s.Set("cycle", &StringValue{Data: []byte("v")}, SetOptions{Tags: []string{"child"}})
		s.Flush()
		checkAudit50R31X(t, "repeated flush tags", 0, len(s.GetTagIndex().Tags()))
	}
	checkAudit50R31X(t, "hierarchy preserved", []string{"child"}, s.GetTagIndex().GetChildren("parent"))
	checkAudit50R31X(t, "no synthetic invalidation callbacks", 0, calls)
	s.GetTagIndex().AddTags("fresh", []string{"child"})
	s.GetTagIndex().Invalidate("child")
	checkAudit50R31X(t, "hook still installed", 1, calls)
	s.Flush()
	checkAudit50R31X(t, "repeat empty flush memory", int64(0), s.MemUsage())
	checkAudit50R31X(t, "flush versions", 0, len(s.versions))

}
