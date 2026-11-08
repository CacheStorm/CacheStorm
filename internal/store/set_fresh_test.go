package store

import (
	"fmt"
	"slices"
	"testing"
)

// SADD counts each new member once, whether the list is short (deduplicated
// by scanning) or long (by a map), and a single member costs no copy.
func TestAddSetMembersCountsDuplicatesOnce(t *testing.T) {
	s := NewStore()
	if n, err := s.AddSetMembers("k", []string{"a"}); err != nil || n != 1 {
		t.Fatalf("first add = %d, %v", n, err)
	}
	if n, _ := s.AddSetMembers("k", []string{"a"}); n != 0 {
		t.Fatalf("re-add = %d, want 0", n)
	}
	if n, _ := s.AddSetMembers("k", []string{"b", "a", "b", "c"}); n != 2 {
		t.Fatalf("short list = %d, want 2", n)
	}
	long := make([]string, 0, 40)
	for i := range 20 {
		long = append(long, fmt.Sprint("m", i), fmt.Sprint("m", i))
	}
	long = append(long, "a")
	if n, _ := s.AddSetMembers("k", long); n != 20 {
		t.Fatalf("long list = %d, want 20", n)
	}
	v, _ := s.Get("k")
	set := v.Value.(*SetValue)
	set.RLock()
	defer set.RUnlock()
	if set.Card() != 23 {
		t.Fatalf("card = %d, want 23", set.Card())
	}
	if got := freshMembers(set, []string{"zz"}); !slices.Equal(got, []string{"zz"}) {
		t.Fatalf("fresh single = %v", got)
	}
}
