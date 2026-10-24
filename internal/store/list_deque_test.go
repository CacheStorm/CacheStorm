package store

import (
	"fmt"
	"testing"
	"time"
)

func listStrings(v *ListValue) []string {
	out := make([]string, len(v.Elements))
	for i, e := range v.Elements {
		out[i] = string(e)
	}
	return out
}

func TestListPushFrontPopFrontOrder(t *testing.T) {
	v := &ListValue{}
	v.PushFront([]byte("c"))
	v.PushFront([]byte("a"), []byte("b"))
	v.Elements = append(v.Elements, []byte("d"))
	if got := fmt.Sprint(listStrings(v)); got != "[a b c d]" {
		t.Fatalf("after pushes: %s", got)
	}
	if got := v.PopFront(1); string(got[0]) != "a" {
		t.Fatalf("PopFront: %q", got)
	}
	v.PushFront([]byte("z"))
	if got := fmt.Sprint(listStrings(v)); got != "[z b c d]" {
		t.Fatalf("after pop and push: %s", got)
	}
	if got := v.PopBack(2); string(got[0]) != "d" || string(got[1]) != "c" {
		t.Fatalf("PopBack: %q", got)
	}
	// Direct reassignment by other code must not corrupt later prepends.
	v.Elements = [][]byte{[]byte("x"), []byte("y")}
	v.PushFront([]byte("w"))
	if got := fmt.Sprint(listStrings(v)); got != "[w x y]" {
		t.Fatalf("after reassignment: %s", got)
	}
}

func TestListPopClearsVacatedSlots(t *testing.T) {
	v := &ListValue{}
	v.PushFront([]byte("a"), []byte("b"), []byte("c"))
	backing := v.backing
	head := v.head
	v.PopFront(1)
	if backing[head] != nil {
		t.Fatal("the popped slot still references the value")
	}
}

// LPUSH used to copy the whole list on every call.
func TestListPushFrontIsAmortisedConstant(t *testing.T) {
	v := &ListValue{}
	start := time.Now()
	for i := 0; i < 200_000; i++ {
		v.PushFront([]byte("x"))
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("200k PushFront took %v", took)
	}
	if len(v.Elements) != 200_000 {
		t.Fatalf("len %d", len(v.Elements))
	}
}
