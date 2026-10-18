package store

import (
	"testing"
)

// TAGLINK accepts any (parent, child) pair and never checks whether the edge
// closes a cycle, so a client can make the tag hierarchy cyclic with two
// ordinary commands: TAGLINK a b followed by TAGLINK b a. INVALIDATE <tag>
// CASCADE then walks that cycle in TagIndex.collectDescendants, which recursed
// once per revisit with no visited set. Go cannot recover a stack overflow, so
// the recursion did not return an error or hang -- it aborted the whole server
// process. collectDescendants must bound its walk to one visit per tag.
func TestGetAllDescendantsTerminatesOnSelfLinkCycle(t *testing.T) {
	ti := NewTagIndex()

	// The shortest cycle: a tag linked to itself.
	ti.Link("a", "a")

	// A tag is not a descendant of itself, so the walk finds nothing below it.
	descendants := ti.GetAllDescendants("a")

	if len(descendants) != 0 {
		t.Fatalf("self-linked tag: expected no descendants, got %v", descendants)
	}
}

func TestGetAllDescendantsTerminatesOnTwoNodeCycle(t *testing.T) {
	ti := NewTagIndex()

	ti.Link("a", "b")
	ti.Link("b", "a")

	descendants := ti.GetAllDescendants("a")

	seen := make(map[string]int, len(descendants))
	for _, d := range descendants {
		seen[d]++
	}
	if len(descendants) != 1 || seen["b"] != 1 {
		t.Fatalf("two-node cycle from a: expected only b as a descendant, got %v", descendants)
	}
	if seen["a"] != 0 {
		t.Fatalf("two-node cycle: the root must not be its own descendant, got %v", descendants)
	}
}

func TestGetAllDescendantsTerminatesOnLongCycle(t *testing.T) {
	ti := NewTagIndex()

	// A longer cycle plus a tail hanging off it: the walk must visit every
	// reachable tag once and must not be cut short by the cycle.
	ti.Link("a", "b")
	ti.Link("b", "c")
	ti.Link("c", "a")
	ti.Link("c", "d")

	descendants := ti.GetAllDescendants("a")

	got := make(map[string]bool, len(descendants))
	for _, d := range descendants {
		if got[d] {
			t.Fatalf("tag %q reported twice: %v", d, descendants)
		}
		got[d] = true
	}
	// The root is seeded as visited, so the cycle back to it terminates the
	// walk; the tail below the cycle must still be reached.
	for _, want := range []string{"b", "c", "d"} {
		if !got[want] {
			t.Errorf("expected descendant %q in %v", want, descendants)
		}
	}
	if got["a"] {
		t.Errorf("the root must not be its own descendant: %v", descendants)
	}
}

// Control: the acyclic hierarchy this fix must leave untouched. Diamond shapes
// share descendants, so the visited set must not drop a tag that is reachable
// by more than one path.
func TestGetAllDescendantsAcyclicDiamondUnaffected(t *testing.T) {
	ti := NewTagIndex()

	ti.Link("root", "left")
	ti.Link("root", "right")
	ti.Link("left", "shared")
	ti.Link("right", "shared")

	descendants := ti.GetAllDescendants("root")

	got := make(map[string]bool, len(descendants))
	for _, d := range descendants {
		got[d] = true
	}
	if len(descendants) != 3 {
		t.Fatalf("expected 3 distinct descendants, got %d (%v)", len(descendants), descendants)
	}
	for _, want := range []string{"left", "right", "shared"} {
		if !got[want] {
			t.Errorf("expected descendant %q in %v", want, descendants)
		}
	}
	if got["root"] {
		t.Errorf("root must not be its own descendant: %v", descendants)
	}
}

// InvalidateCascade is the production entry point behind the INVALIDATE ... CASCADE
// command. On a cyclic hierarchy it must return the invalidated key set rather
// than aborting the process, and it must still invalidate every linked tag.
func TestInvalidateCascadeTerminatesOnCycleAndCollectsKeys(t *testing.T) {
	ti := NewTagIndex()

	ti.AddTags("key1", []string{"a"})
	ti.AddTags("key2", []string{"b"})

	ti.Link("a", "b")
	ti.Link("b", "a")

	result := ti.InvalidateCascade("a")

	if got := result["a"]; len(got) != 1 || got[0] != "key1" {
		t.Errorf("expected tag a to yield key1, got %v", got)
	}
	if got := result["b"]; len(got) != 1 || got[0] != "key2" {
		t.Errorf("expected cascade to reach tag b and yield key2, got %v", got)
	}
}
