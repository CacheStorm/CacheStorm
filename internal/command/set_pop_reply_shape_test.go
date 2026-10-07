package command

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func runSetAuditX(t *testing.T, s *store.Store, cmd string, args ...string) string {
	t.Helper()
	r := NewRouter()
	RegisterSetCommands(r)
	a := make([][]byte, len(args))
	for i, v := range args {
		a[i] = []byte(v)
	}
	var b bytes.Buffer
	w := resp.NewWriter(&b)
	if err := r.Execute(NewContext(cmd, a, s, w)); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestSetPopReplyDependsOnCountPresence(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		seed      bool
		want      string
		remaining int64
	}{
		{"scalar", []string{"set"}, true, "$1\r\na\r\n", 0},
		{"count one", []string{"set", "1"}, true, "*1\r\n$1\r\na\r\n", 0},
		{"count zero", []string{"set", "0"}, true, "*0\r\n", 1},
		{"oversized count", []string{"set", "10"}, true, "*1\r\n$1\r\na\r\n", 0},
		{"missing scalar", []string{"missing"}, false, "$-1\r\n", 0},
		{"missing count one", []string{"missing", "1"}, false, "*0\r\n", 0},
		{"missing count zero", []string{"missing", "0"}, false, "*0\r\n", 0},
		{"missing count two", []string{"missing", "2"}, false, "*0\r\n", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := store.NewStore()
			if tc.seed {
				runSetAuditX(t, s, "SADD", "set", "a")
			}
			got := runSetAuditX(t, s, "SPOP", tc.args...)
			t.Logf("EXPECTED: %q ACTUAL: %q", tc.want, got)
			if got != tc.want {
				t.Fatal("reply shape changed")
			}
			wantCard := fmt.Sprintf(":%d\r\n", tc.remaining)
			if got := runSetAuditX(t, s, "SCARD", tc.args[0]); got != wantCard {
				t.Fatalf("cardinality: %q", got)
			}
			if tc.remaining == 0 {
				if _, ok := s.Get(tc.args[0]); ok {
					t.Fatal("exhausted set retained")
				}
			}
			wantRepeat := tc.want
			if tc.remaining == 0 {
				wantRepeat = "*0\r\n"
				if len(tc.args) == 1 {
					wantRepeat = "$-1\r\n"
				}
			}
			if got := runSetAuditX(t, s, "SPOP", tc.args...); got != wantRepeat {
				t.Fatalf("repeated pop: %q", got)
			}
		})
	}

	s := store.NewStore()
	runSetAuditX(t, s, "SADD", "many", "a", "b")
	got := runSetAuditX(t, s, "SPOP", "many", "1")
	value, err := resp.NewReader(bytes.NewBufferString(got)).ReadValue()
	if err != nil || value.Type != resp.TypeArray || len(value.Array) != 1 {
		t.Fatalf("count-one partial pop: %q, error=%v", got, err)
	}
	member := string(value.Array[0].Bulk)
	if member != "a" && member != "b" {
		t.Fatal("popped an unrelated member")
	}
	if got := runSetAuditX(t, s, "SCARD", "many"); got != ":1\r\n" {
		t.Fatal("count-one did not pop exactly one member")
	}
	got = runSetAuditX(t, s, "SPOP", "many", "2")
	value, err = resp.NewReader(bytes.NewBufferString(got)).ReadValue()
	if err != nil || value.Type != resp.TypeArray || len(value.Array) != 1 {
		t.Fatalf("pop remaining member: %q, error=%v", got, err)
	}
	if string(value.Array[0].Bulk) == member {
		t.Fatal("returned a previously removed member")
	}
	if _, ok := s.Get("many"); ok {
		t.Fatal("exhausted multi-member set retained")
	}
}
