package store

import "testing"

// newTaggedStore returns a store holding a single key "k" whose value is
// tagged with the given tags.
func newTaggedStore(t *testing.T, tags ...string) *Store {
	t.Helper()
	s := NewStore()
	if err := s.Set("k", &StringValue{Data: []byte("v")}, SetOptions{Tags: tags}); err != nil {
		t.Fatalf("setup Set: %v", err)
	}
	return s
}

// SetEntry stores an entry verbatim, so it must reconcile the tag index: drop
// any stale mapping for the key, then register the entry's own tags. It
// previously did neither, so a key moved in via RENAME or COPY was invisible
// to TAGKEYS/TAGCOUNT even though its entry still carried the tags.
func TestSetEntryRegistersTags(t *testing.T) {
	s := newTaggedStore(t, "env")

	entry, ok := s.Get("k")
	if !ok {
		t.Fatal("setup: key k missing")
	}

	s.SetEntry("moved", entry)

	got := s.GetTagIndex().GetKeys("env")
	found := false
	for _, k := range got {
		if k == "moved" {
			found = true
		}
	}
	if !found {
		t.Fatalf("GetKeys(env) = %v, want it to contain \"moved\"", got)
	}
}

// A replaced key must not leave its previous occupant's tag mapping behind:
// the stale tag is swept and only the new entry's tags remain.
func TestSetEntryDropsStaleTagsOnOverwrite(t *testing.T) {
	s := newTaggedStore(t, "oldtag")

	old, ok := s.Get("k")
	if !ok {
		t.Fatal("setup: key k missing")
	}

	fresh := NewEntry(&StringValue{Data: []byte("v2")})
	fresh.Tags = []string{"newtag"}
	s.SetEntry("k", fresh)

	if got := s.GetTagIndex().GetKeys("oldtag"); len(got) != 0 {
		t.Fatalf("GetKeys(oldtag) = %v, want empty — the replaced entry's mapping must be dropped", got)
	}
	if got := s.GetTagIndex().GetKeys("newtag"); len(got) != 1 || got[0] != "k" {
		t.Fatalf("GetKeys(newtag) = %v, want [k]", got)
	}

	// Sanity: the old entry is no longer the one in the store.
	cur, _ := s.Get("k")
	if cur == old {
		t.Fatal("store still holds the previous entry pointer")
	}
}

// Control: an untagged SetEntry is a no-op for the tag index.
func TestSetEntryWithoutTagsLeavesIndexAlone(t *testing.T) {
	s := newTaggedStore(t, "env")
	if got := s.GetTagIndex().GetKeys("env"); len(got) != 1 {
		t.Fatalf("setup: GetKeys(env) = %v, want one key", got)
	}

	s.SetEntry("plain", NewEntry(&StringValue{Data: []byte("v")}))

	if got := s.GetTagIndex().GetKeys("env"); len(got) != 1 || got[0] != "k" {
		t.Fatalf("GetKeys(env) = %v, want [k] — an untagged SetEntry must not disturb the index", got)
	}
}
