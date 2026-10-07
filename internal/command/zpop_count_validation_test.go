package command

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestZPopCountPreservesMembers(t *testing.T) {
	for _, cmd := range []string{"ZPOPMIN", "ZPOPMAX", "ZMPOP", "BZMPOP"} {
		t.Run(cmd, func(t *testing.T) {
			s := store.NewStore()
			r := NewRouter()
			RegisterSortedSetCommands(r)
			call := func(name string, args ...string) string {
				a := make([][]byte, len(args))
				for i, v := range args {
					a[i] = []byte(v)
				}
				var b bytes.Buffer
				w := resp.NewWriter(&b)
				if err := r.Execute(NewContext(name, a, s, w)); err != nil {
					t.Fatal(err)
				}
				if err := w.Flush(); err != nil {
					t.Fatal(err)
				}
				return b.String()
			}
			pop := func(key, count string) string {
				if cmd == "ZMPOP" {
					return call(cmd, "1", key, "MIN", "COUNT", count)
				}
				if cmd == "BZMPOP" {
					return call(cmd, "0", "1", key, "MIN", "COUNT", count)
				}
				return call(cmd, key, count)
			}
			call("ZADD", "target", "1", "a", "2", "b")
			for _, count := range []string{"0", "0", "-1", "-9223372036854775808"} {
				got := pop("target", count)
				wantEmpty := count == "0" && (cmd == "ZPOPMIN" || cmd == "ZPOPMAX")
				if wantEmpty && got != "*0\r\n" {
					t.Fatalf("expected empty reply, got %q", got)
				}
				if !wantEmpty && !strings.HasPrefix(got, "-") {
					t.Fatalf("expected rejection, got %q", got)
				}
				card := call("ZCARD", "target")
				t.Logf("count=%s EXPECTED: cardinality=2 ACTUAL: %q", count, card)
				if card != ":2\r\n" {
					t.Fatal("zero/invalid count consumed members")
				}
			}
			pop("missing", "0")
			if _, exists := s.Get("missing"); exists {
				t.Fatal("created missing key")
			}
			pop("target", "1")
			if got := call("ZCARD", "target"); got != ":1\r\n" {
				t.Fatalf("control expected one member left: %q", got)
			}
		})
	}
	fmt.Println("FIX VERIFIED")
}
