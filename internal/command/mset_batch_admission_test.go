package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestMSetBatchReplacementCreditX(t *testing.T) {
	s := store.NewStore()
	s.ConfigureMemory(2048, store.EvictionNoEviction, 70, 90, 5)
	r := NewRouter()
	RegisterStringCommands(r)
	for _, key := range []string{"a", "b"} {
		if err := s.Set(key, &store.StringValue{Data: []byte(strings.Repeat("x", 600))}, store.SetOptions{}); err != nil {
			t.Fatal("CONTROL seed", err)
		}
	}
	before := s.MemUsage()
	if got := runEntryTransfer(t, s, nil, r, "SET", "a", strings.Repeat("y", 600)); got != "+OK\r\n" {
		t.Fatal("CONTROL equal-size SET", got)
	}
	got := runEntryTransfer(t, s, nil, r, "MSET", "a", strings.Repeat("z", 600), "b", strings.Repeat("z", 600))
	t.Logf("EXPECTED: +OK, usage=%d | ACTUAL: %q usage=%d", before, got, s.MemUsage())
	if got != "+OK\r\n" || s.MemUsage() != before {
		t.Fatal("replacement batch must credit existing entries")
	}
}

func TestMSetBatchFailureAndNoOpEffectsX(t *testing.T) {
	for _, name := range []string{"MSET", "MSETNX"} {
		s := store.NewStore()
		s.ConfigureMemory(1024, store.EvictionNoEviction, 70, 90, 5)
		r := NewRouter()
		RegisterStringCommands(r)
		if err := s.Set("a", &store.StringValue{Data: []byte("old")}, store.SetOptions{Tags: []string{"old"}, ExpireAtMS: 4102444800000}); err != nil {
			t.Fatal(err)
		}
		before := s.WatchKey("a")
		defer s.UnwatchKey("a")
		records := 0
		r.SetPostExecute(func(string, [][]byte) { records++ })
		var got string
		if name == "MSETNX" {
			got = runEntryTransfer(t, s, nil, r, name, "b", "new", "a", "replace")
			if got != ":0\r\n" {
				t.Fatalf("NX conflict=%q", got)
			}
		} else {
			got = runEntryTransfer(t, s, nil, r, name, "b", "new", "a", strings.Repeat("x", 2048))
			if !strings.HasPrefix(got, "-OOM") {
				t.Fatalf("budget rejection=%q", got)
			}
		}
		entry, _ := s.Get("a")
		if s.Exists("b") || entry.Value.String() != "old" || entry.ExpiresAt.Load() != 4102444800000 || len(entry.Tags) != 1 || s.GetVersion("a") != before || records != 0 {
			t.Fatalf("failed/no-op %s changed state: records=%d", name, records)
		}
		if got := runEntryTransfer(t, s, nil, r, name, "b", "new", "", "invalid"); !strings.HasPrefix(got, "-ERR") || s.Exists("b") || records != 0 {
			t.Fatalf("invalid late key=%q records=%d", got, records)
		}
	}
}
