package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func sortLimitRun(t *testing.T, s *store.Store, args ...string) *resp.Value {
	t.Helper()
	router := NewRouter()
	RegisterServerCommands(router)
	RegisterListCommands(router)
	RegisterKeyCommands(router)

	var buf bytes.Buffer
	ctx := NewContext(args[0], sortLimitBytes(args[1:]), s, resp.NewWriter(&buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: Execute returned error: %v", args[0], err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", args[0], buf.String(), err)
	}
	return v
}

func sortLimitBytes(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

func sortLimitStrings(v *resp.Value) []string {
	out := make([]string, 0, len(v.Array))
	for _, e := range v.Array {
		out = append(out, string(e.Bulk))
	}
	return out
}

func sortLimitEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// seedSortList builds list "l" holding a..e.
func seedSortList(t *testing.T) *store.Store {
	t.Helper()
	s := store.NewStore()
	for _, e := range []string{"a", "b", "c", "d", "e"} {
		sortLimitRun(t, s, "RPUSH", "l", e)
	}
	return s
}

// Redis defines a negative LIMIT offset as a count back from the end. The
// handler applied the offset only `if offset > 0`, so a negative offset was
// dropped and the page was taken from the head instead.
func TestSortNegativeOffsetCountsFromEnd(t *testing.T) {
	for _, tc := range []struct {
		offset string
		count  string
		want   []string
	}{
		{"-1", "1", []string{"e"}},
		{"-2", "2", []string{"d", "e"}},
		{"-3", "1", []string{"c"}},
		{"-5", "2", []string{}},
		{"-9", "2", []string{}}, // magnitude exceeds the list
	} {
		s := seedSortList(t)
		got := sortLimitStrings(sortLimitRun(t, s, "SORT", "l", "ALPHA", "LIMIT", tc.offset, tc.count))
		if !sortLimitEqual(got, tc.want) {
			t.Fatalf("SORT l ALPHA LIMIT %s %s = %v, want %v", tc.offset, tc.count, got, tc.want)
		}
	}
}

// Control: non-negative offsets, including the beyond-length clamp that must
// still yield an empty page rather than restarting from the head.
func TestSortNonNegativeOffsetControls(t *testing.T) {
	s := seedSortList(t)

	if got := sortLimitStrings(sortLimitRun(t, s, "SORT", "l", "ALPHA", "LIMIT", "0", "2")); !sortLimitEqual(got, []string{"a", "b"}) {
		t.Fatalf("LIMIT 0 2 = %v, want [a b]", got)
	}
	if got := sortLimitStrings(sortLimitRun(t, s, "SORT", "l", "ALPHA", "LIMIT", "2", "2")); !sortLimitEqual(got, []string{"c", "d"}) {
		t.Fatalf("LIMIT 2 2 = %v, want [c d]", got)
	}
	if got := sortLimitStrings(sortLimitRun(t, s, "SORT", "l", "ALPHA", "LIMIT", "3", "100")); !sortLimitEqual(got, []string{"d", "e"}) {
		t.Fatalf("LIMIT 3 100 = %v, want [d e]", got)
	}
	if got := sortLimitStrings(sortLimitRun(t, s, "SORT", "l", "ALPHA", "LIMIT", "9", "2")); len(got) != 0 {
		t.Fatalf("LIMIT 9 2 = %v, want an empty page — an offset past the end must clamp", got)
	}
}

// Control: a NEGATIVE count already means "all elements from the offset".
func TestSortNegativeCountControls(t *testing.T) {
	s := seedSortList(t)

	if got := sortLimitStrings(sortLimitRun(t, s, "SORT", "l", "ALPHA", "LIMIT", "0", "-1")); !sortLimitEqual(got, []string{"a", "b", "c", "d", "e"}) {
		t.Fatalf("LIMIT 0 -1 = %v, want all five", got)
	}
	if got := sortLimitStrings(sortLimitRun(t, s, "SORT", "l", "ALPHA", "LIMIT", "3", "-1")); !sortLimitEqual(got, []string{"d", "e"}) {
		t.Fatalf("LIMIT 3 -1 = %v, want [d e]", got)
	}
}

// Control: combining a negative offset with a negative count, and plain SORT
// with no LIMIT, are unaffected.
func TestSortCombinedAndNoLimitControls(t *testing.T) {
	s := seedSortList(t)

	if got := sortLimitStrings(sortLimitRun(t, s, "SORT", "l", "ALPHA", "LIMIT", "-2", "-1")); !sortLimitEqual(got, []string{"d", "e"}) {
		t.Fatalf("LIMIT -2 -1 = %v, want [d e]", got)
	}
	if got := sortLimitStrings(sortLimitRun(t, s, "SORT", "l", "ALPHA")); !sortLimitEqual(got, []string{"a", "b", "c", "d", "e"}) {
		t.Fatalf("SORT l ALPHA = %v, want all five", got)
	}
}
