package store

import (
	"testing"
)

// Regression: Store.Set added new tag mappings (tagIndex.AddTags) but never
// removed the replaced entry's mappings, so re-tagging a key left it mapped
// under the old tag forever: TAGKEYS old-tag returned the key after its tags
// had changed, and every re-tag/overwrite cycle grew the index.

func TestSetReTagsReplacesOldMapping(t *testing.T) {
	s := NewStore()

	if err := s.Set("rk", &StringValue{Data: []byte("v")}, SetOptions{Tags: []string{"old"}}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s.Set("rk", &StringValue{Data: []byte("v2")}, SetOptions{Tags: []string{"new"}}); err != nil {
		t.Fatalf("re-Set: %v", err)
	}

	if contains(s.tagIndex.GetKeys("old"), "rk") {
		t.Fatal("re-tagged key is still mapped under its old tag (stale mapping)")
	}
	if !contains(s.tagIndex.GetKeys("new"), "rk") {
		t.Fatal("re-tagged key missing from its new tag mapping")
	}
}

// Overwriting a tagged key with a tag-less SET must drop the old mapping: the
// replacement entry carries no tags, so the index must not keep mapping it.
func TestSetWithoutTagsClearsOldMapping(t *testing.T) {
	s := NewStore()

	if err := s.Set("uk", &StringValue{Data: []byte("v")}, SetOptions{Tags: []string{"old"}}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s.Set("uk", &StringValue{Data: []byte("v2")}, SetOptions{}); err != nil {
		t.Fatalf("overwrite: %v", err)
	}

	if contains(s.tagIndex.GetKeys("old"), "uk") {
		t.Fatal("key overwritten without tags is still mapped under its old tag (stale mapping)")
	}
}

// Tags shared between the old and new sets must survive the reconciliation.
func TestSetSharedTagsSurviveRetag(t *testing.T) {
	s := NewStore()

	if err := s.Set("sk", &StringValue{Data: []byte("v")}, SetOptions{Tags: []string{"a", "b"}}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s.Set("sk", &StringValue{Data: []byte("v2")}, SetOptions{Tags: []string{"b", "c"}}); err != nil {
		t.Fatalf("re-Set: %v", err)
	}

	if contains(s.tagIndex.GetKeys("a"), "sk") {
		t.Fatal("dropped tag a still maps the key")
	}
	if !contains(s.tagIndex.GetKeys("b"), "sk") {
		t.Fatal("shared tag b lost the key mapping")
	}
	if !contains(s.tagIndex.GetKeys("c"), "sk") {
		t.Fatal("new tag c missing the key mapping")
	}
}
