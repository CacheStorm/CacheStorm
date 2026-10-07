package command

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestZAddScoreValidationIsAtomic(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterSortedSetCommands(r)
	call := func(args ...string) string {
		a := make([][]byte, len(args))
		for i, v := range args {
			a[i] = []byte(v)
		}
		var b bytes.Buffer
		w := resp.NewWriter(&b)
		if err := r.Execute(NewContext("ZADD", a, s, w)); err != nil {
			t.Fatal(err)
		}
		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	if got := call("good", "1", "a", "2", "b"); got != ":2\r\n" {
		t.Fatalf("control: %q", got)
	}
	t.Log("CONTROL EXPECTED: 2 inserted ACTUAL: 2 inserted")
	for _, args := range [][]string{
		{"missing", "1", "a", "invalid", "b"},
		{"missing", "NaN", "a"},
		{"missing", "XX", "bad", "a"},
		{"missing", "1", "a", "", "b"},
	} {
		got := call(args...)
		_, exists := s.Get("missing")
		t.Logf("EXPECTED: error, missing key ACTUAL: %q, exists=%v", got, exists)
		if !strings.HasPrefix(got, "-") || exists {
			t.Fatal("invalid write created key")
		}
	}
	entry, _ := s.Get("good")
	for i := 0; i < 2; i++ {
		got := call("good", "9", "a", "NaN", "c")
		if !strings.HasPrefix(got, "-") {
			t.Fatalf("NaN accepted: %q", got)
		}
		z := entry.Value.(*store.SortedSetValue)
		t.Logf("EXPECTED: a=1 and cardinality=2 ACTUAL: a=%v cardinality=%d", z.Members["a"], len(z.Members))
		if z.Members["a"] != 1 || len(z.Members) != 2 {
			t.Fatal("existing set partially changed")
		}
	}
	fmt.Println("FIX VERIFIED")
}
