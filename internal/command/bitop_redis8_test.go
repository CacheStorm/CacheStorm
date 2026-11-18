package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// The replies and results Redis 8.2 gives for its DIFF, DIFF1, ANDOR and ONE.
func TestBITOPRedis82Operations(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterBitmapCommands(r)
	run := func(args ...string) *resp.Value {
		t.Helper()
		raw := make([][]byte, len(args))
		for i, a := range args {
			raw[i] = []byte(a)
		}
		var b bytes.Buffer
		if err := r.Execute(NewContext("BITOP", raw, s, resp.NewWriter(&b))); err != nil {
			t.Fatal(err)
		}
		v, err := resp.NewReader(&b).ReadValue()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for k, v := range map[string]string{"a": "\xf0\x0f", "b": "\x3c", "c": "\xff\x01\x80"} {
		if err := s.Set(k, &store.StringValue{Data: []byte(v)}, store.SetOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		args []string
		n    int64
		want string
	}{
		{[]string{"DIFF", "d", "a", "b", "c"}, 3, "\x00\x0e\x00"},
		{[]string{"diff1", "d", "a", "b", "c"}, 3, "\x0f\x00\x80"},
		{[]string{"AndOr", "d", "a", "b", "c"}, 3, "\xf0\x01\x00"},
		{[]string{"ONE", "d", "a", "b", "c"}, 3, "\x03\x0e\x80"},
		{[]string{"ONE", "d", "a"}, 2, "\xf0\x0f"},
		{[]string{"ONE", "d", "a", "a"}, 2, "\x00\x00"},
		{[]string{"ONE", "d", "a", "b", "c", "a"}, 3, "\x03\x00\x80"},
		{[]string{"DIFF", "d", "a", "nokey"}, 2, "\xf0\x0f"},
		{[]string{"DIFF1", "d", "a", "nokey"}, 2, "\x00\x00"},
		{[]string{"DIFF1", "d", "nokey", "a"}, 2, "\xf0\x0f"},
		{[]string{"ANDOR", "d", "nokey", "a"}, 2, "\x00\x00"},
		{[]string{"ANDOR", "d", "a", "b"}, 2, "0\x00"},
	} {
		if got := run(tc.args...); got.Int != tc.n {
			t.Errorf("%v: reply %v, want %d", tc.args, got, tc.n)
		}
		e, ok := s.Get("d")
		if !ok || e.Value.String() != tc.want {
			t.Errorf("%v: d = %q, want %q", tc.args, e.Value.String(), tc.want)
		}
	}
	if got := run("DIFF", "d", "nokey", "nokey2"); got.Int != 0 || s.Exists("d") {
		t.Errorf("all sources missing: %v, d exists %v", got, s.Exists("d"))
	}
	for _, op := range []string{"diff", "Diff1", "andor"} {
		if got := run(op, "d", "a"); got.Err != "ERR BITOP "+string(bytes.ToUpper([]byte(op)))+" must be called with at least two source keys." {
			t.Errorf("%s with one source: %v", op, got)
		}
	}
}
