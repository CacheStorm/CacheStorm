package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestHRandFieldReplyDependsOnCountPresence(t *testing.T) {
	s := store.NewStore()
	runHashAuditX(t, s, "HSET", "h", "f", "v")
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"h"}, "$1\r\nf\r\n"},
		{[]string{"h", "1"}, "*1\r\n$1\r\nf\r\n"},
		{[]string{"h", "0"}, "*0\r\n"},
		{[]string{"h", "3"}, "*1\r\n$1\r\nf\r\n"},
		{[]string{"h", "-2"}, "*2\r\n$1\r\nf\r\n$1\r\nf\r\n"},
		{[]string{"h", "1", "WITHVALUES"}, "*2\r\n$1\r\nf\r\n$1\r\nv\r\n"},
		{[]string{"missing"}, "$-1\r\n"},
		{[]string{"missing", "1"}, "*0\r\n"},
		{[]string{"missing", "1", "WITHVALUES"}, "*0\r\n"},
	}
	for _, tc := range cases {
		for repeat := 0; repeat < 2; repeat++ {
			got := runHashAuditX(t, s, "HRANDFIELD", tc.args...)
			t.Logf("%v EXPECTED: %q ACTUAL: %q", tc.args, tc.want, got)
			if got != tc.want {
				t.Fatal("reply shape")
			}
		}
	}
	if got := runHashAuditX(t, s, "HGET", "h", "f"); got != "$1\r\nv\r\n" {
		t.Fatal("read changed source")
	}
	runHashAuditX(t, s, "HSET", "many", "a", "1", "b", "2")
	got := runHashAuditX(t, s, "HRANDFIELD", "many", "1")
	v, err := resp.NewReader(bytes.NewBufferString(got)).ReadValue()
	if err != nil || v.Type != resp.TypeArray || len(v.Array) != 1 {
		t.Fatalf("multiple-fields count one: %q, %v", got, err)
	}
	if field := string(v.Array[0].Bulk); field != "a" && field != "b" {
		t.Fatal("selected field is not a member")
	}
	if got := runHashAuditX(t, s, "HLEN", "many"); got != ":2\r\n" {
		t.Fatal("random read changed field count")
	}
}
