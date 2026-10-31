package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestSetStoreAccessKeysDoNotBecomeWriteEffectsX(t *testing.T) {
	for _, name := range []string{"SUNIONSTORE", "SINTERSTORE", "SDIFFSTORE"} {
		t.Run(name, func(t *testing.T) {
			s := store.NewStore()
			s.ConfigureMemory(3100, store.EvictionNoEviction, 70, 90, 5)
			if _, err := s.AddSetMembers("a", []string{strings.Repeat("a", 600)}); err != nil {
				t.Fatal(err)
			}
			if _, err := s.AddSetMembers("b", []string{strings.Repeat("b", 400)}); err != nil {
				t.Fatal(err)
			}
			if err := s.Set("d", &store.StringValue{Data: make([]byte, 50)}, store.SetOptions{}); err != nil {
				t.Fatal(err)
			}
			r := NewRouter()
			RegisterSetCommands(r)
			vA, vD := s.WatchKey("a"), s.WatchKey("d")
			defer s.UnwatchKey("a")
			defer s.UnwatchKey("d")
			records := 0
			r.SetPostExecute(func(string, [][]byte) { records++ })
			if got := runEntryTransfer(t, s, nil, r, name, "d", "a"); !strings.HasPrefix(got, "-OOM") || s.GetVersion("a") != vA || s.GetVersion("d") != vD || records != 0 {
				t.Fatalf("refusal effects: %q records=%d", got, records)
			}
			if got := runEntryTransfer(t, s, nil, r, name, "d", "missing"); got != ":0\r\n" || s.Exists("d") || s.GetVersion("d") == vD || records != 1 {
				t.Fatalf("deletion effects: %q records=%d", got, records)
			}
			vD = s.GetVersion("d")
			if got := runEntryTransfer(t, s, nil, r, name, "d", "missing"); got != ":0\r\n" || s.GetVersion("d") != vD || records != 1 {
				t.Fatalf("empty no-op effects: %q records=%d", got, records)
			}
			if got := runEntryTransfer(t, s, nil, r, name, "d", "b"); got != ":1\r\n" || s.GetVersion("a") != vA || s.GetVersion("d") == vD || records != 2 {
				t.Fatalf("write effects: %q records=%d", got, records)
			}
		})
	}
}
