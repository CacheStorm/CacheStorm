package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func runEntryTransfer(t *testing.T, s *store.Store, dbs *store.Databases, r *Router, name string, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	ctx := NewContext(name, bytesArgs(args...), s, resp.NewWriter(&out))
	ctx.DBs = dbs
	if err := r.Execute(ctx); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestEntryTransferRejectsGrowthWithoutMutation(t *testing.T) {
	for _, name := range []string{"COPY", "RENAME", "RENAMENX"} {
		t.Run(name, func(t *testing.T) {
			s := store.NewStore()
			s.ConfigureMemory(1024, store.EvictionNoEviction, 70, 90, 5)
			if err := s.Set("src", &store.StringValue{Data: make([]byte, 500)}, store.SetOptions{Tags: []string{"source"}}); err != nil {
				t.Fatalf("control: %v", err)
			}
			r := NewRouter()
			RegisterKeyCommands(r)
			before := s.MemUsage()
			dst := strings.Repeat("d", 1024)
			version := s.WatchKey("src")
			defer s.UnwatchKey("src")
			records := 0
			r.SetPostExecute(func(string, [][]byte) { records++ })
			reply := runEntryTransfer(t, s, nil, r, name, "src", dst)
			if !strings.HasPrefix(reply, "-OOM") || !s.Exists("src") || s.Exists(dst) || s.MemUsage() != before || s.MemoryTracker().Usage() != before || s.GetVersion("src") != version || records != 0 {
				t.Fatalf("EXPECTED: rejection preserves source/budget/watch/journal | ACTUAL: reply=%q src=%v dst=%v usage=%d before=%d records=%d", reply, s.Exists("src"), s.Exists(dst), s.MemUsage(), before, records)
			}
		})
	}
}

func TestCopyRejectsInvalidDestination(t *testing.T) {
	s := store.NewStore()
	if err := s.Set("src", &store.StringValue{Data: []byte("kept")}, store.SetOptions{}); err != nil {
		t.Fatal(err)
	}
	r := NewRouter()
	RegisterKeyCommands(r)
	got := runEntryTransfer(t, s, nil, r, "COPY", "src", strings.Repeat("x", store.MaxKeySize+1))
	if !strings.HasPrefix(got, "-ERR") || !s.Exists("src") {
		t.Fatalf("EXPECTED: invalid destination error | ACTUAL: %q", got)
	}
}

func TestTransferNoOpsAndCopyDoNotTouchSourceWatch(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{{"RENAME", []string{"src", "src"}, "+OK\r\n"}, {"RENAMENX", []string{"src", "dst"}, ":0\r\n"}, {"COPY", []string{"src", "new"}, ":1\r\n"}, {"COPY", []string{"src", "dst"}, ":0\r\n"}, {"MOVE", []string{"src", "1"}, ":0\r\n"}} {
		t.Run(tc.name+strings.Join(tc.args, "/"), func(t *testing.T) {
			s := store.NewStore()
			dbs := store.NewDatabases(s, 2)
			for _, st := range []*store.Store{s, dbs.DB(1)} {
				for _, key := range []string{"src", "dst"} {
					if err := st.Set(key, &store.StringValue{Data: []byte(key)}, store.SetOptions{}); err != nil {
						t.Fatal(err)
					}
				}
			}
			r := NewRouter()
			RegisterKeyCommands(r)
			version := s.WatchKey("src")
			defer s.UnwatchKey("src")
			if got := runEntryTransfer(t, s, dbs, r, tc.name, tc.args...); got != tc.want {
				t.Fatalf("reply=%q", got)
			}
			if s.GetVersion("src") != version {
				t.Fatal("EXPECTED: unchanged source WATCH | ACTUAL: version changed")
			}
		})
	}
}
