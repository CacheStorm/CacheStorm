package command

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestSetIntersectionValidatesEverySource(t *testing.T) {
	for _, cmd := range []string{"SINTER", "SINTERSTORE", "SINTERCARD"} {
		t.Run(cmd, func(t *testing.T) {
			s := store.NewStore()
			runSetAuditX(t, s, "SADD", "src", "a", "b")
			runSetAuditX(t, s, "SADD", "other", "a")
			if err := s.Set("bad", &store.StringValue{Data: []byte("unchanged")}, store.SetOptions{}); err != nil {
				t.Fatal(err)
			}
			call := func(keys ...string) string {
				args := keys
				if cmd == "SINTERSTORE" {
					args = append([]string{"dest"}, args...)
				}
				if cmd == "SINTERCARD" {
					args = append([]string{fmt.Sprint(len(keys))}, args...)
				}
				return runSetAuditX(t, s, cmd, args...)
			}
			for _, keys := range [][]string{{"src", "missing", "bad"}, {"missing", "src", "bad"}, {"src", "bad", "missing"}, {"missing", "bad", "src"}} {
				runSetAuditX(t, s, "SADD", "dest", "keep")
				before, _ := s.Get("dest")
				before.ExpiresAt = math.MaxInt64
				for repeat := 0; repeat < 2; repeat++ {
					got := call(keys...)
					t.Logf("%v EXPECTED: WRONGTYPE ACTUAL: %q", keys, got)
					if !strings.HasPrefix(got, "-WRONGTYPE") {
						t.Fatal("later input type not checked")
					}
					after, exists := s.Get("dest")
					if !exists || after != before || after.ExpiresAt != math.MaxInt64 {
						t.Fatal("rejected intersection changed destination metadata")
					}
					if got := runSetAuditX(t, s, "SISMEMBER", "dest", "keep"); got != ":1\r\n" {
						t.Fatal("destination member changed")
					}
				}
			}
			expected := ":1\r\n"
			if cmd == "SINTER" {
				expected = "*1\r\n$1\r\na\r\n"
			}
			if got := call("src", "other"); got != expected {
				t.Fatalf("normal intersection: %q", got)
			}
			expected = ":0\r\n"
			if cmd == "SINTER" {
				expected = "*0\r\n"
			}
			for _, keys := range [][]string{{"src", "missing", "other"}, {"missing", "src"}, {"src", "other", "missing"}} {
				runSetAuditX(t, s, "SADD", "dest", "keep")
				if got := call(keys...); got != expected {
					t.Fatalf("valid empty intersection: %q", got)
				}
				if cmd == "SINTERSTORE" {
					if _, ok := s.Get("dest"); ok {
						t.Fatal("empty intersection did not remove destination")
					}
				}
			}
			if got := runSetAuditX(t, s, "SCARD", "src"); got != ":2\r\n" {
				t.Fatal("source changed")
			}
			if got := runSetAuditX(t, s, "SCARD", "other"); got != ":1\r\n" {
				t.Fatal("second source changed")
			}
			if got, _ := s.Get("bad"); string(got.Value.(*store.StringValue).Data) != "unchanged" {
				t.Fatal("wrong-type input changed")
			}
		})
	}
	s := store.NewStore()
	runSetAuditX(t, s, "SADD", "same", "a")
	if got := runSetAuditX(t, s, "SINTERSTORE", "same", "same", "missing"); got != ":0\r\n" {
		t.Fatal("destination-as-source empty result")
	}
	if _, ok := s.Get("same"); ok {
		t.Fatal("same-key destination not removed")
	}
}
