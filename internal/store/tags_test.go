package store

import (
	"sync"
	"testing"
)

func TestUpdateTagsAccountsAndIndexes(t *testing.T) {
	s := NewStore()
	if err := s.Set("k", &StringValue{Data: []byte("v")}, SetOptions{}); err != nil {
		t.Fatal(err)
	}
	before := s.MemUsage()
	if !s.UpdateTags("k", func(tags []string) []string { return append(tags, "red", "blue") }) {
		t.Fatal("UpdateTags on an existing key reported false")
	}
	if got, want := s.MemUsage()-before, int64(len("red")+16+len("blue")+16); got != want {
		t.Fatalf("tags charged %d bytes, want %d", got, want)
	}
	if keys := s.GetTagIndex().GetKeys("red"); len(keys) != 1 || keys[0] != "k" {
		t.Fatalf("tag index for red = %v", keys)
	}
	s.UpdateTags("k", func([]string) []string { return []string{"blue"} })
	if keys := s.GetTagIndex().GetKeys("red"); len(keys) != 0 {
		t.Fatalf("red still indexed after removal: %v", keys)
	}
	if s.MemUsage()-before != int64(len("blue")+16) {
		t.Fatalf("memory after removing a tag = %d over the base", s.MemUsage()-before)
	}
	if s.UpdateTags("missing", func(tags []string) []string { return append(tags, "x") }) {
		t.Fatal("UpdateTags on a missing key reported true")
	}
}

func TestUpdateTagsConcurrentWithReads(t *testing.T) {
	s := NewStore()
	_ = s.Set("k", &StringValue{Data: []byte("v")}, SetOptions{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				s.UpdateTags("k", func(tags []string) []string { return append(tags[:0], "a", "b") })
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if e, ok := s.Get("k"); ok {
					_ = len(e.TagList())
				}
			}
		}()
	}
	wg.Wait()
}
