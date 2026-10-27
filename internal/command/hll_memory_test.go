package command

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestHLLRejectedGrowthPreservesValueAndMetadata(t *testing.T) {
	for _, name := range []string{"PFADD", "PFMERGE"} {
		t.Run(name, func(t *testing.T) {
			s := store.NewStore()
			s.ConfigureMemory(20000, store.EvictionNoEviction, 70, 90, 5)
			state := &store.HLLState{Dense: true}
			args := []string{"h"}
			for i := 0; i < 20000; i++ {
				element := strconv.Itoa(i)
				state.Add([]byte(element))
				args = append(args, element)
			}
			source, err := state.Encode(true)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Set("source", &store.StringValue{Data: source}, store.SetOptions{}); err != nil {
				t.Fatal(err)
			}
			empty, err := (&store.HLLState{}).Encode(true)
			if err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(time.Minute).UnixMilli()
			if err := s.Set("h", &store.StringValue{Data: empty}, store.SetOptions{ExpireAtMS: deadline, Tags: []string{"tag"}}); err != nil {
				t.Fatal(err)
			}
			before := s.MemoryTracker().Usage()
			version := s.WatchKey("h")
			defer s.UnwatchKey("h")
			r := NewRouter()
			RegisterHyperLogLogCommands(r)
			if name == "PFMERGE" {
				args = []string{"h", "source"}
			}
			if got := execBf(s, r, name, args...); !strings.HasPrefix(got, "-OOM") {
				t.Fatalf("growth accepted: %q", got)
			}
			entry, ok := s.Get("h")
			if !ok || !bytes.Equal([]byte(entry.Value.String()), empty) || entry.ExpiresAt.Load() != deadline || len(entry.TagList()) != 1 || entry.TagList()[0] != "tag" {
				t.Fatal("rejected growth changed value or metadata")
			}
			if s.GetVersion("h") != version || s.MemoryTracker().Usage() != before || s.MemUsage() != before {
				t.Fatal("rejected growth changed WATCH or memory accounting")
			}
		})
	}
}
