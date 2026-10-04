package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// negCountRun executes a command through the real router and converts a panic
// into a failure, so the reason names the defect instead of crashing the run.
func negCountRun(t *testing.T, s *store.Store, args ...string) (reply *resp.Value, panicked interface{}) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			panicked = r
		}
	}()

	router := NewRouter()
	RegisterStringCommands(router)
	RegisterSetCommands(router)
	RegisterHashCommands(router)
	RegisterKeyCommands(router)

	var buf bytes.Buffer
	ctx := NewContext(args[0], negCountBytes(args[1:]), s, resp.NewWriter(&buf))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%s: Execute returned error: %v", args[0], err)
	}
	v, err := resp.NewReader(bytes.NewReader(buf.Bytes())).ReadValue()
	if err != nil {
		t.Fatalf("%s: could not parse RESP reply %q: %v", args[0], buf.String(), err)
	}
	return v, nil
}

func negCountBytes(ss []string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

func negCountMustNotPanic(t *testing.T, s *store.Store, args ...string) *resp.Value {
	t.Helper()
	v, p := negCountRun(t, s, args...)
	if p != nil {
		t.Fatalf("%v panicked: %v — the count is used directly as a make() capacity, and a\n"+
			"negative count becomes a negative capacity", args, p)
	}
	return v
}

func negCountSeedSet(t *testing.T, members ...string) *store.Store {
	t.Helper()
	s := store.NewStore()
	for _, m := range members {
		negCountMustNotPanic(t, s, "SADD", "s", m)
	}
	return s
}

func negCountSeedHash(t *testing.T, fields ...string) *store.Store {
	t.Helper()
	s := store.NewStore()
	for i := 0; i+1 < len(fields); i += 2 {
		negCountMustNotPanic(t, s, "HSET", "h", fields[i], fields[i+1])
	}
	return s
}

// SPOP's count is a pop count: Redis rejects a negative one. It was passed
// straight to make() as a capacity, so a negative value panicked.
func TestSPOPRejectsNegativeCount(t *testing.T) {
	s := negCountSeedSet(t, "a", "b", "c")

	reply := negCountMustNotPanic(t, s, "SPOP", "s", "-1")

	if reply.Type != resp.TypeError {
		t.Fatalf("SPOP s -1 returned type %v, want an error", reply.Type)
	}
	if n := negCountMustNotPanic(t, s, "SCARD", "s").Int; n != 3 {
		t.Fatalf("SCARD after rejected SPOP = %d, want 3 — the rejected SPOP must not pop", n)
	}
}

// SRANDMEMBER treats a negative count as a repetition count: exactly |count|
// elements, repetition allowed.
func TestSRANDMEMBERHandlesNegativeCount(t *testing.T) {
	s := negCountSeedSet(t, "a", "b", "c")

	reply := negCountMustNotPanic(t, s, "SRANDMEMBER", "s", "-2")

	if reply.Type != resp.TypeArray {
		t.Fatalf("SRANDMEMBER s -2 returned type %v, want an array", reply.Type)
	}
	if len(reply.Array) != 2 {
		t.Fatalf("SRANDMEMBER s -2 returned %d elements, want 2", len(reply.Array))
	}
	for _, e := range reply.Array {
		switch string(e.Bulk) {
		case "a", "b", "c":
		default:
			t.Fatalf("SRANDMEMBER s -2 returned %q, which is not a member", e.Bulk)
		}
	}
}

// HRANDFIELD already had a negative-count branch, but make(..., count*2)
// panicked before it could run, so the support was unreachable.
func TestHRANDFIELDHandlesNegativeCount(t *testing.T) {
	s := negCountSeedHash(t, "f1", "v1", "f2", "v2")

	reply := negCountMustNotPanic(t, s, "HRANDFIELD", "h", "-3")

	if reply.Type != resp.TypeArray {
		t.Fatalf("HRANDFIELD h -3 returned type %v, want an array", reply.Type)
	}
	if len(reply.Array) != 3 {
		t.Fatalf("HRANDFIELD h -3 returned %d elements, want 3", len(reply.Array))
	}
	for _, e := range reply.Array {
		switch string(e.Bulk) {
		case "f1", "f2":
		default:
			t.Fatalf("HRANDFIELD h -3 returned %q, which is not a field", e.Bulk)
		}
	}
}

// Control: the no-count and positive-count forms are unchanged.
func TestRandomMemberCountControls(t *testing.T) {
	s := negCountSeedSet(t, "a", "b", "c")

	if r := negCountMustNotPanic(t, s, "SPOP", "s"); r.Type != resp.TypeBulkString {
		t.Fatalf("SPOP s returned type %v, want a bulk string", r.Type)
	}
	if n := negCountMustNotPanic(t, s, "SCARD", "s").Int; n != 2 {
		t.Fatalf("SCARD after SPOP = %d, want 2", n)
	}

	// Each command gets its own store: SPOP mutates the set, so a shared store
	// would leave SRANDMEMBER with fewer members than the assertion expects.
	s2 := negCountSeedSet(t, "a", "b", "c")
	if r := negCountMustNotPanic(t, s2, "SPOP", "s", "2"); r.Type != resp.TypeArray || len(r.Array) != 2 {
		t.Fatalf("SPOP s 2 returned %v with %d elements, want 2", r.Type, len(r.Array))
	}

	sRand := negCountSeedSet(t, "a", "b", "c")
	if r := negCountMustNotPanic(t, sRand, "SRANDMEMBER", "s", "2"); r.Type != resp.TypeArray || len(r.Array) != 2 {
		t.Fatalf("SRANDMEMBER s 2 returned %v with %d elements, want 2", r.Type, len(r.Array))
	}
	if r := negCountMustNotPanic(t, sRand, "SRANDMEMBER", "s"); r.Type != resp.TypeBulkString {
		t.Fatalf("SRANDMEMBER s returned type %v, want a bulk string", r.Type)
	}

	s3 := negCountSeedHash(t, "f1", "v1", "f2", "v2")
	if r := negCountMustNotPanic(t, s3, "HRANDFIELD", "h"); r.Type != resp.TypeBulkString {
		t.Fatalf("HRANDFIELD h returned type %v, want a bulk string", r.Type)
	}
	if r := negCountMustNotPanic(t, s3, "HRANDFIELD", "h", "2"); len(r.Array) != 2 {
		t.Fatalf("HRANDFIELD h 2 returned %d elements, want 2", len(r.Array))
	}
	if r := negCountMustNotPanic(t, s3, "HRANDFIELD", "h", "2", "WITHVALUES"); len(r.Array) != 4 {
		t.Fatalf("HRANDFIELD h 2 WITHVALUES returned %d elements, want 4", len(r.Array))
	}
}
