package store

import (
	"testing"
)

func TestTagIndexAddAndGetKeys(t *testing.T) {
	ti := NewTagIndex()

	ti.AddTags("key1", []string{"tag1", "tag2"})
	ti.AddTags("key2", []string{"tag1"})
	ti.AddTags("key3", []string{"tag2"})

	keys := ti.GetKeys("tag1")
	if len(keys) != 2 {
		t.Errorf("expected 2 keys for tag1, got %d", len(keys))
	}

	keys = ti.GetKeys("tag2")
	if len(keys) != 2 {
		t.Errorf("expected 2 keys for tag2, got %d", len(keys))
	}
}

func TestTagIndexRemoveKey(t *testing.T) {
	ti := NewTagIndex()

	ti.AddTags("key1", []string{"tag1"})
	ti.AddTags("key2", []string{"tag1"})

	ti.RemoveKey("key1", []string{"tag1"})

	keys := ti.GetKeys("tag1")
	if len(keys) != 1 {
		t.Errorf("expected 1 key after removal, got %d", len(keys))
	}
}

func TestTagIndexInvalidate(t *testing.T) {
	ti := NewTagIndex()

	ti.AddTags("key1", []string{"tag1"})
	ti.AddTags("key2", []string{"tag1"})
	ti.AddTags("key3", []string{"tag1"})

	keys := ti.Invalidate("tag1")
	if len(keys) != 3 {
		t.Errorf("expected 3 invalidated keys, got %d", len(keys))
	}

	count := ti.Count("tag1")
	if count != 0 {
		t.Errorf("expected 0 count after invalidation, got %d", count)
	}
}

func TestTagHierarchy(t *testing.T) {
	ti := NewTagIndex()

	ti.Link("parent", "child1")
	ti.Link("parent", "child2")

	children := ti.GetChildren("parent")
	if len(children) != 2 {
		t.Errorf("expected 2 children, got %d", len(children))
	}

	descendants := ti.GetAllDescendants("parent")
	if len(descendants) != 2 {
		t.Errorf("expected 2 descendants, got %d", len(descendants))
	}

	ti.Unlink("parent", "child1")
	children = ti.GetChildren("parent")
	if len(children) != 1 {
		t.Errorf("expected 1 child after unlink, got %d", len(children))
	}
}

func TestTagIndexCount(t *testing.T) {
	ti := NewTagIndex()

	ti.AddTags("key1", []string{"tag1"})
	ti.AddTags("key2", []string{"tag1"})
	ti.AddTags("key3", []string{"tag2"})

	count := ti.Count("tag1")
	if count != 2 {
		t.Errorf("expected count 2 for tag1, got %d", count)
	}

	count = ti.Count("tag2")
	if count != 1 {
		t.Errorf("expected count 1 for tag2, got %d", count)
	}
}
