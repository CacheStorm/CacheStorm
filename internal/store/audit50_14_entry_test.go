package store

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func checkAudit50R14X(t *testing.T, label string, want, got interface{}) {
	t.Helper()
	t.Logf("%s EXPECTED: %v | ACTUAL: %v", label, want, got)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s mismatch", label)
	}
}

func TestAudit50R14X(t *testing.T) {
	defer func() {
		if t.Failed() {
			t.Log("PROBLEM CONFIRMED")
		} else {
			t.Log("FIX VERIFIED")
		}
	}()
	control := &ListValue{Elements: [][]byte{[]byte("a"), []byte("b")}}
	checkAudit50R14X(t, "control list string", "a, b", control.String())
	value := &ListValue{Elements: [][]byte{[]byte(""), []byte("b")}}
	checkAudit50R14X(t, "leading empty element preserved", ", b", value.String())
	empty := &ListValue{Elements: [][]byte{[]byte(""), []byte("")}}
	checkAudit50R14X(t, "two empty elements preserved", ", ", empty.String())
	for _, parts := range [][]string{nil, {""}, {"a", "", "b"}, {"a", ""}, {"", "", "tail"}} {
		elements := make([][]byte, len(parts))
		for i, p := range parts {
			elements[i] = []byte(p)
		}
		checkAudit50R14X(t, "empty/middle/trailing boundary", strings.Join(parts, ", "), (&ListValue{Elements: elements}).String())
	}
	source := &HashValue{Fields: map[string][]byte{"k": []byte("original")}}
	clone := source.Clone().(*HashValue)
	ready, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() { close(ready); <-release; clone.Fields["k"][0] = 'X'; close(done) }()
	<-ready
	close(release)
	<-done
	checkAudit50R14X(t, "control nested hash clone", "original", string(source.Fields["k"]))
	entry := NewEntry(&StringValue{Data: []byte("value")})
	checkAudit50R14X(t, "no expiry TTL", time.Duration(-1), entry.TTL())
	entry.Touch()
	entry.Touch()
	checkAudit50R14X(t, "access counter", uint64(2), entry.AccessCount.Load())

}
