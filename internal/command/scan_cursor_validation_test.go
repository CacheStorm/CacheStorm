package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func scanCursorRun(t *testing.T, s *store.Store, args ...string) *resp.Value {
	t.Helper()
	router := NewRouter()
	RegisterServerCommands(router)
	RegisterHashCommands(router)
	RegisterSortedSetCommands(router)
	RegisterSetCommands(router)
	RegisterStringCommands(router)
	RegisterKeyCommands(router)

	var buf bytes.Buffer
	ctx := NewContext(args[0], scanCursorBytes(args[1:]), s, resp.NewWriter(&buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: Execute returned error: %v", args[0], err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", args[0], buf.String(), err)
	}
	return v
}

func scanCursorBytes(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

// A scan cursor is a non-negative offset. The handlers used it as a slice
// start with no lower-bound check, so a negative cursor indexed before the
// start of the collection and panicked. SCAN already rejected negatives; the
// HSCAN/ZSCAN/SSCAN variants did not.
func TestScanVariantsRejectNegativeCursor(t *testing.T) {
	hash := store.NewStore()
	scanCursorRun(t, hash, "HSET", "h", "f1", "v")
	scanCursorRun(t, hash, "HSET", "h", "f2", "v")

	zset := store.NewStore()
	scanCursorRun(t, zset, "ZADD", "z", "1", "m1")
	scanCursorRun(t, zset, "ZADD", "z", "1", "m2")

	set := store.NewStore()
	scanCursorRun(t, set, "SADD", "s", "m1")
	scanCursorRun(t, set, "SADD", "s", "m2")

	for _, tc := range []struct {
		name string
		run  func() *resp.Value
	}{
		{"HSCAN", func() *resp.Value { return scanCursorRun(t, hash, "HSCAN", "h", "-1") }},
		{"ZSCAN", func() *resp.Value { return scanCursorRun(t, zset, "ZSCAN", "z", "-1") }},
		{"SSCAN", func() *resp.Value { return scanCursorRun(t, set, "SSCAN", "s", "-1") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if v := tc.run(); v.Type != resp.TypeError {
				t.Fatalf("%s key -1 returned type %v, want an error", tc.name, v.Type)
			}
		})
	}
}

// Control: SCAN already rejected negative cursors — the in-repo basis for the
// expected behaviour, and it must stay that way.
func TestSCANRejectsNegativeCursorControl(t *testing.T) {
	s := store.NewStore()
	scanCursorRun(t, s, "SET", "k1", "v")

	if v := scanCursorRun(t, s, "SCAN", "-1"); v.Type != resp.TypeError {
		t.Fatalf("SCAN -1 returned type %v, want an error", v.Type)
	}
}

// Control: cursor 0 still works. HSCAN and ZSCAN emit field/member AND
// value/score, so N items come back as 2N elements; SSCAN emits one per member.
func TestScanVariantsValidCursorControls(t *testing.T) {
	hash := store.NewStore()
	for _, f := range []string{"f1", "f2", "f3"} {
		scanCursorRun(t, hash, "HSET", "h", f, "v")
	}
	if v := scanCursorRun(t, hash, "HSCAN", "h", "0"); len(v.Array[1].Array) != 6 {
		t.Fatalf("HSCAN h 0 returned %d elements, want 6 (3 field/value pairs)", len(v.Array[1].Array))
	}

	zset := store.NewStore()
	for _, m := range []string{"m1", "m2", "m3"} {
		scanCursorRun(t, zset, "ZADD", "z", "1", m)
	}
	if v := scanCursorRun(t, zset, "ZSCAN", "z", "0"); len(v.Array[1].Array) != 6 {
		t.Fatalf("ZSCAN z 0 returned %d elements, want 6 (3 member/score pairs)", len(v.Array[1].Array))
	}

	set := store.NewStore()
	for _, m := range []string{"m1", "m2", "m3"} {
		scanCursorRun(t, set, "SADD", "s", m)
	}
	if v := scanCursorRun(t, set, "SSCAN", "s", "0"); len(v.Array[1].Array) != 3 {
		t.Fatalf("SSCAN s 0 returned %d elements, want 3 members", len(v.Array[1].Array))
	}
}

// Control: COUNT paging still terminates, a past-the-end cursor still restarts
// from the head, and a non-numeric cursor is still an error.
func TestScanPagingAndErrorControls(t *testing.T) {
	hash := store.NewStore()
	for _, f := range []string{"f1", "f2", "f3"} {
		scanCursorRun(t, hash, "HSET", "h", f, "v")
	}

	first := scanCursorRun(t, hash, "HSCAN", "h", "0", "COUNT", "2")
	if len(first.Array[1].Array) != 4 {
		t.Fatalf("first page returned %d elements, want 4 (2 field/value pairs)", len(first.Array[1].Array))
	}
	cursor := string(first.Array[0].Bulk)
	if cursor == "0" {
		t.Fatal("expected a non-zero next cursor after a partial page")
	}

	second := scanCursorRun(t, hash, "HSCAN", "h", cursor, "COUNT", "2")
	if len(second.Array[1].Array) != 2 {
		t.Fatalf("second page returned %d elements, want 2 (the remaining 1 pair)", len(second.Array[1].Array))
	}
	if string(second.Array[0].Bulk) != "0" {
		t.Fatalf("final cursor = %q, want \"0\"", second.Array[0].Bulk)
	}

	if v := scanCursorRun(t, hash, "HSCAN", "h", "99", "COUNT", "2"); len(v.Array[1].Array) != 4 {
		t.Fatalf("past-the-end cursor returned %d elements, want 4", len(v.Array[1].Array))
	}
	if v := scanCursorRun(t, hash, "HSCAN", "h", "abc"); v.Type != resp.TypeError {
		t.Fatalf("non-numeric cursor returned type %v, want an error", v.Type)
	}
}
