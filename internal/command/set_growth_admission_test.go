package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestSetGrowthX(t *testing.T) {
	for _, fresh := range []bool{false, true} {
		t.Run(map[bool]string{true: "new", false: "existing"}[fresh], func(t *testing.T) {
			s := store.NewStore()
			s.ConfigureMemory(1000, store.EvictionNoEviction, 70, 90, 5)
			r := NewRouter()
			RegisterSetCommands(r)
			if got := runEntryTransfer(t, s, nil, r, "SADD", "control", "small"); got != ":1\r\n" {
				t.Fatal("CONTROL", got)
			}
			s.Delete("control")
			if !fresh {
				if err := s.Set("s", &store.SetValue{Members: map[string]struct{}{"kept": {}}}, store.SetOptions{Tags: []string{"tag"}, ExpireAtMS: 4102444800000}); err != nil {
					t.Fatal(err)
				}
			}
			before := s.MemUsage()
			version := s.WatchKey("s")
			defer s.UnwatchKey("s")
			records := 0
			r.SetPostExecute(func(string, [][]byte) { records++ })
			got := runEntryTransfer(t, s, nil, r, "SADD", "s", "prefix", strings.Repeat("x", 1000))
			entry, exists := s.Get("s")
			same := !exists
			if !fresh && exists {
				set := entry.Value.(*store.SetValue)
				same = len(set.Members) == 1
				_, kept := set.Members["kept"]
				same = same && kept && entry.ExpiresAt.Load() == 4102444800000 && len(entry.TagList()) == 1
			}
			t.Logf("EXPECTED: OOM unchanged data/budget/watch/journal | ACTUAL: %q same=%v usage=%d->%d records=%d", got, same, before, s.MemUsage(), records)
			if !strings.HasPrefix(got, "-OOM") || !same || s.MemUsage() != before || s.GetVersion("s") != version || records != 0 {
				t.Error("refused growth must preserve state")
			}
		})
	}
}
