package store

import (
	"errors"
	"slices"
	"testing"
)

// TAGLINK used to accept any (parent, child) pair, so TAGLINK a b followed by
// TAGLINK b a made the hierarchy cyclic, and a recursive walk of it aborted
// the process with a stack overflow. Link now refuses a link that closes a
// cycle, and the walk keeps a visited set and its own queue regardless.
func TestLinkRefusesCyclesX(t *testing.T) {
	ti := NewTagIndex()
	if err := ti.Link("a", "a"); !errors.Is(err, ErrTagLinkCycle) {
		t.Fatalf("Link(a, a) = %v, want ErrTagLinkCycle", err)
	}
	for _, l := range [][2]string{{"a", "b"}, {"b", "c"}, {"c", "d"}} {
		if err := ti.Link(l[0], l[1]); err != nil {
			t.Fatalf("Link%v: %v", l, err)
		}
	}
	for _, l := range [][2]string{{"b", "a"}, {"d", "a"}, {"c", "b"}} {
		if err := ti.Link(l[0], l[1]); !errors.Is(err, ErrTagLinkCycle) {
			t.Fatalf("Link%v = %v, want ErrTagLinkCycle", l, err)
		}
	}
	// A second path to a descendant is no cycle.
	if err := ti.Link("a", "d"); err != nil {
		t.Fatalf("Link(a, d): %v", err)
	}
	if got := ti.Links(); len(got) != 4 {
		t.Fatalf("Links = %v", got)
	}
}

// A hierarchy loaded with a cycle (or built by hand here) is still walked
// once per tag, without the root.
func TestGetAllDescendantsTerminatesOnCycleX(t *testing.T) {
	ti := NewTagIndex()
	ti.children = map[string]map[string]struct{}{
		"a": {"a": {}, "b": {}},
		"b": {"c": {}},
		"c": {"a": {}, "d": {}},
	}
	got := ti.GetAllDescendants("a")
	slices.Sort(got)
	if !slices.Equal(got, []string{"b", "c", "d"}) {
		t.Fatalf("descendants of a = %v", got)
	}
}

// A deep chain is walked without recursion.
func TestGetAllDescendantsDeepChainX(t *testing.T) {
	ti := NewTagIndex()
	const depth = 100000
	ti.children = make(map[string]map[string]struct{}, depth)
	for i := range depth {
		ti.children[tagName(i)] = map[string]struct{}{tagName(i + 1): {}}
	}
	if got := len(ti.GetAllDescendants(tagName(0))); got != depth {
		t.Fatalf("descendants = %d, want %d", got, depth)
	}
}

func tagName(i int) string {
	b := []byte{'t'}
	for {
		b = append(b, byte('0'+i%10))
		i /= 10
		if i == 0 {
			return string(b)
		}
	}
}

// Diamond shapes share descendants: the visited set must not drop a tag
// reachable by more than one path.
func TestGetAllDescendantsAcyclicDiamondUnaffected(t *testing.T) {
	ti := NewTagIndex()

	for _, l := range [][2]string{{"root", "left"}, {"root", "right"}, {"left", "shared"}, {"right", "shared"}} {
		if err := ti.Link(l[0], l[1]); err != nil {
			t.Fatal(err)
		}
	}
	got := ti.GetAllDescendants("root")
	slices.Sort(got)
	if !slices.Equal(got, []string{"left", "right", "shared"}) {
		t.Fatalf("descendants = %v", got)
	}
}

// A tag may have several parents; unlinking one leaves the others.
func TestUnlinkKeepsOtherParentsX(t *testing.T) {
	ti := NewTagIndex()
	_ = ti.Link("a", "c")
	_ = ti.Link("b", "c")
	ti.Unlink("b", "c")
	if got := ti.GetAllDescendants("a"); !slices.Equal(got, []string{"c"}) {
		t.Fatalf("descendants of a = %v", got)
	}
	if got := ti.GetAllDescendants("b"); len(got) != 0 {
		t.Fatalf("descendants of b = %v", got)
	}
}

func TestInvalidateCascadeCollectsKeys(t *testing.T) {
	ti := NewTagIndex()

	ti.AddTags("key1", []string{"a"})
	ti.AddTags("key2", []string{"b"})
	_ = ti.Link("a", "b")

	result := ti.InvalidateCascade("a")

	if got := result["a"]; len(got) != 1 || got[0] != "key1" {
		t.Errorf("expected tag a to yield key1, got %v", got)
	}
	if got := result["b"]; len(got) != 1 || got[0] != "key2" {
		t.Errorf("expected cascade to reach tag b and yield key2, got %v", got)
	}
}
