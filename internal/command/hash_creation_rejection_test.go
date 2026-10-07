package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func runHashAuditX(t *testing.T, s *store.Store, cmd string, args ...string) string {
	t.Helper()
	r := NewRouter()
	RegisterHashCommands(r)
	argv := make([][]byte, len(args))
	for i, v := range args {
		argv[i] = []byte(v)
	}
	var b bytes.Buffer
	w := resp.NewWriter(&b)
	if err := r.Execute(NewContext(cmd, argv, s, w)); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestHashCreationReportsStoreRejection(t *testing.T) {
	for _, cmd := range []string{"HSET", "HMSET", "HSETNX", "HINCRBY", "HINCRBYFLOAT"} {
		t.Run(cmd, func(t *testing.T) {
			s := store.NewStore()
			s.ConfigureMemory(1, store.EvictionNoEviction, 70, 85, 5)
			for repeat := 0; repeat < 2; repeat++ {
				got := runHashAuditX(t, s, cmd, "missing", "f", "1")
				t.Logf("EXPECTED: OOM ACTUAL: %q", got)
				if !strings.HasPrefix(got, "-OOM") {
					t.Fatal("rejected creation acknowledged as success")
				}
				if _, ok := s.Get("missing"); ok {
					t.Fatal("failed creation retained key")
				}
				if s.MemoryTracker().Usage() != 0 {
					t.Fatal("failed creation changed accounting")
				}
			}
			s = store.NewStore()
			if got := runHashAuditX(t, s, cmd, "", "f", "1"); got != "-ERR invalid key name\r\n" {
				t.Fatalf("invalid-key rejection: %q", got)
			}
			if got := runHashAuditX(t, s, cmd, "ok", "f", "1"); strings.HasPrefix(got, "-") {
				t.Fatalf("unlimited creation: %q", got)
			}
			if got := runHashAuditX(t, s, "HGET", "ok", "f"); got != "$1\r\n1\r\n" {
				t.Fatal("successful value not persisted")
			}
			if err := s.Set("wrong", &store.StringValue{Data: []byte("keep")}, store.SetOptions{}); err != nil {
				t.Fatal(err)
			}
			if got := runHashAuditX(t, s, cmd, "wrong", "f", "1"); !strings.HasPrefix(got, "-WRONGTYPE") {
				t.Fatalf("wrong type: %q", got)
			}
			before, _ := s.Get("wrong")
			if string(before.Value.(*store.StringValue).Data) != "keep" {
				t.Fatal("wrong-type destination changed")
			}
		})
	}
	s := store.NewStore()
	runHashAuditX(t, s, "HSET", "multi", "a", "1", "b", "2")
	if got := runHashAuditX(t, s, "HSET", "multi", "a", "3", "c", "4"); got != ":1\r\n" {
		t.Fatalf("multi-field update count: %q", got)
	}
	if got := runHashAuditX(t, s, "HMGET", "multi", "a", "b", "c"); got != "*3\r\n$1\r\n3\r\n$1\r\n2\r\n$1\r\n4\r\n" {
		t.Fatal("multi-field data changed")
	}
	if got := runHashAuditX(t, s, "HSETNX", "multi", "a", "9"); got != ":0\r\n" {
		t.Fatal("NX existing field changed")
	}
	if got := runHashAuditX(t, s, "HDEL", "multi", "a", "a", "b", "c"); got != ":3\r\n" {
		t.Fatal("duplicate field deletion count")
	}
	if _, ok := s.Get("multi"); ok {
		t.Fatal("last field did not remove hash")
	}
	s = store.NewStore()
	s.ConfigureMemory(1, store.EvictionNoEviction, 70, 85, 5)
	ctx := NewContext("HINCRBY", nil, s, resp.NewWriter(&bytes.Buffer{}))
	value := respHIncrBy(ctx, queuedCommand{args: [][]byte{[]byte("missing"), []byte("f"), []byte("1")}}, 1)
	if value.Type != resp.TypeError || !strings.HasPrefix(value.Err, "OOM") {
		t.Fatalf("replay rejection: %#v", value)
	}
	if _, ok := s.Get("missing"); ok {
		t.Fatal("replay failed creation retained key")
	}
}
