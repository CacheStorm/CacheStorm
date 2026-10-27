package command

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestHashGrowthAdmissionX(t *testing.T) {
	for _, name := range []string{"HSET", "HMSET", "HSETNX", "HINCRBY", "HINCRBYFLOAT"} {
		for _, fresh := range []bool{false, true} {
			t.Run(name+map[bool]string{true: "/new", false: "/existing"}[fresh], func(t *testing.T) {
				s := store.NewStore()
				s.ConfigureMemory(1000, store.EvictionNoEviction, 70, 90, 5)
				r := NewRouter()
				RegisterHashCommands(r)
				numeric := name == "HINCRBY" || name == "HINCRBYFLOAT"
				if !fresh {
					initial := "kept"
					if numeric {
						initial = "99"
					}
					if err := s.Set("h", &store.HashValue{Fields: map[string][]byte{"f": []byte(initial)}}, store.SetOptions{Tags: []string{"tag"}, ExpireAtMS: 4102444800000}); err != nil {
						t.Fatal("CONTROL", err)
					}
					if numeric {
						size := 949 - s.MemUsage() - (store.EntryOverhead + 24 + int64(len("p")) + 16)
						if err := s.Set("p", &store.StringValue{Data: make([]byte, size)}, store.SetOptions{}); err != nil {
							t.Fatal("CONTROL filler", err)
						}
					}
				}
				controlKey := "h"
				if fresh {
					controlKey = "control"
				}
				if got := runEntryTransfer(t, s, nil, r, "HSETNX", controlKey, "f", map[bool]string{true: "99", false: "kept"}[fresh]); strings.HasPrefix(got, "-") {
					t.Fatal("CONTROL", got)
				}
				if fresh {
					s.Delete(controlKey)
				}
				before, beforeOK := s.Get("h")
				var snapshot map[string][]byte
				if beforeOK {
					snapshot = before.Value.Clone().(*store.HashValue).Fields
				}
				usage := s.MemUsage()
				version := s.WatchKey("h")
				defer s.UnwatchKey("h")
				records := 0
				r.SetPostExecute(func(string, [][]byte) { records++ })
				field, value := "g", strings.Repeat("x", 1000)
				if numeric {
					field = "f"
					value = "1"
					if fresh {
						field = strings.Repeat("f", 1000)
					}
				}
				got := runEntryTransfer(t, s, nil, r, name, "h", field, value)
				after, afterOK := s.Get("h")
				var fields map[string][]byte
				if afterOK {
					h := after.Value.(*store.HashValue)
					fields = h.Clone().(*store.HashValue).Fields
				}
				same := beforeOK == afterOK && reflect.DeepEqual(fields, snapshot)
				t.Logf("EXPECTED: OOM, unchanged value/usage/watch/journal | ACTUAL: reply=%q same=%v usage=%d->%d version=%d->%d records=%d", got, same, usage, s.MemUsage(), version, s.GetVersion("h"), records)
				if !strings.HasPrefix(got, "-OOM") || !same || s.MemUsage() != usage || s.GetVersion("h") != version || records != 0 {
					t.Error("growth must be rejected before mutation")
				}
			})
		}
	}
}
