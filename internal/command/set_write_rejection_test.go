package command

import (
	"math"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestSetWritesReportStoreRejection(t *testing.T) {
	for _, cmd := range []string{"SADD", "SUNIONSTORE", "SINTERSTORE", "SDIFFSTORE"} {
		t.Run(cmd, func(t *testing.T) {
			s := store.NewStore()
			s.ConfigureMemory(1000, store.EvictionNoEviction, 70, 85, 5)
			for key, value := range map[string]store.Value{
				"src": &store.SetValue{Members: map[string]struct{}{"a": {}}},
				"dst": &store.StringValue{Data: []byte("keep")},
			} {
				if err := s.Set(key, value, store.SetOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.Set("filler", &store.StringValue{Data: []byte(strings.Repeat("x", 550))}, store.SetOptions{}); err != nil {
				t.Fatal(err)
			}
			if s.MemoryTracker().Usage() != 951 {
				t.Fatal("memory-pressure setup changed")
			}
			before, _ := s.Get("dst")
			before.ExpiresAt = math.MaxInt64
			args := []string{"dst", "src"}
			if cmd == "SADD" {
				args = []string{"new", "a"}
			}
			for repeat := 0; repeat < 2; repeat++ {
				got := runSetAuditX(t, s, cmd, args...)
				t.Logf("EXPECTED: OOM ACTUAL: %q", got)
				if !strings.HasPrefix(got, "-OOM") {
					t.Fatal("rejected write was acknowledged")
				}
				after, ok := s.Get("dst")
				if !ok || after != before || after.ExpiresAt != math.MaxInt64 || string(after.Value.(*store.StringValue).Data) != "keep" {
					t.Fatal("rejected write changed destination")
				}
				if _, ok := s.Get("new"); ok {
					t.Fatal("failed SADD created a detached set key")
				}
				if got := runSetAuditX(t, s, "SISMEMBER", "src", "a"); got != ":1\r\n" {
					t.Fatal("source changed")
				}
				if s.MemoryTracker().Usage() != 951 {
					t.Fatal("rejected write changed accounting")
				}
			}
			s.Delete("filler")
			if got := runSetAuditX(t, s, cmd, args...); got != ":1\r\n" {
				t.Fatalf("write after pressure removed: %q", got)
			}
			if got := runSetAuditX(t, s, "SISMEMBER", args[0], "a"); got != ":1\r\n" {
				t.Fatal("successful result not persisted")
			}
			if cmd != "SADD" {
				if got := runSetAuditX(t, s, cmd, "dst", "missing"); got != ":0\r\n" {
					t.Fatal("empty result control")
				}
				if _, ok := s.Get("dst"); ok {
					t.Fatal("empty result retained destination")
				}
			}
		})
	}
	s := store.NewStore()
	if got := runSetAuditX(t, s, "SADD", "", "a"); got != "-ERR invalid key name\r\n" {
		t.Fatalf("invalid key boundary: %q", got)
	}
	runSetAuditX(t, s, "SADD", "existing", "a")
	if got := runSetAuditX(t, s, "SADD", "existing", "a", "b"); got != ":1\r\n" {
		t.Fatalf("existing-key duplicate control: %q", got)
	}
	for _, cmd := range []string{"SUNIONSTORE", "SINTERSTORE", "SDIFFSTORE"} {
		if got := runSetAuditX(t, s, cmd, "existing", "existing"); got != ":2\r\n" {
			t.Fatalf("same-key store: %q", got)
		}
	}
}
