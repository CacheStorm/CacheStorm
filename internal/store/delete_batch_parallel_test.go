package store

import (
	"strconv"
	"testing"
)

func TestDeleteBatchParallelX(t *testing.T) {
	s := NewStore()
	if err := s.Set("keep", &StringValue{Data: []byte("v")}, SetOptions{Tags: []string{"g0"}}); err != nil {
		t.Fatal(err)
	}
	baseMem, baseKeys := s.MemUsage(), s.KeyCount()
	n := parallelDeleteMin * 3
	keys := make([]string, 0, n+10)
	for i := 0; i < n; i++ {
		k := "k:" + strconv.Itoa(i)
		tags := []string{"all", "g" + strconv.Itoa(i%7)}
		if i%3 == 0 {
			tags = nil
		}
		if err := s.Set(k, &StringValue{Data: []byte("value")}, SetOptions{Tags: tags}); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, k)
	}
	for i := 0; i < 10; i++ {
		keys = append(keys, "missing:"+strconv.Itoa(i))
	}
	watched := s.WatchKey("k:5")
	unwatched := s.WatchKey("keep")

	if got := s.DeleteBatch(keys); got != n {
		t.Fatalf("deleted %d, want %d", got, n)
	}
	for _, k := range keys {
		if s.Exists(k) {
			t.Fatalf("%s survived", k)
		}
	}
	if got := s.tagIndex.Count("all"); got != 0 {
		t.Fatalf("tag all still lists %d keys", got)
	}
	if got := s.tagIndex.GetKeys("g0"); len(got) != 1 || got[0] != "keep" {
		t.Fatalf("tag g0 lists %v, want [keep]", got)
	}
	if s.KeyCount() != baseKeys || s.MemUsage() != baseMem {
		t.Fatalf("keys=%d mem=%d, want %d %d", s.KeyCount(), s.MemUsage(), baseKeys, baseMem)
	}
	if s.GetVersion("k:5") == watched {
		t.Fatal("deleting a watched key did not bump its version")
	}
	if s.GetVersion("keep") != unwatched {
		t.Fatal("a key left alone had its version bumped")
	}
}
