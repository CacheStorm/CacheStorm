package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// execC runs the real router handler for cmd and returns the raw RESP reply.
func execC(s *store.Store, r *Router, cmd string, args ...string) string {
	handler, ok := r.Get(cmd)
	if !ok {
		return "ERR HARNESS: " + cmd + " not registered"
	}
	raw := make([][]byte, 0, len(args))
	for _, a := range args {
		raw = append(raw, []byte(a))
	}
	var buf bytes.Buffer
	ctx := NewContext(cmd, raw, s, resp.NewWriter(&buf))
	if err := handler.Handler(ctx); err != nil {
		return "ERR HARNESS: " + err.Error()
	}
	return buf.String()
}

// TestProofSInterCardHonoursNumKeys is the round proof.
//
// Contract (Redis SINTERCARD):
//
//	SINTERCARD numkeys key [key ...] [LIMIT limit]
//
// The FIRST argument is a COUNT of how many keys follow, NOT a key. The reply
// is the cardinality of the intersection of those numkeys keys.
//
// Defect: cmdSINTERCARD never parsed that count. It set
// `numKeys := ctx.ArgCount()` — the number of ARGUMENTS — and then read its
// first set from `ctx.ArgString(0)`, which is the numkeys token itself. So
// `SINTERCARD 2 s1 s2` intersected a phantom set named "2" (which never
// exists, so getSetOrEmpty yields an empty one) against the real keys. An empty
// first set intersects to empty, so EVERY well-formed call answered 0.
//
// This is not a tolerance question: the command can only ever return 0 today,
// for any input whatsoever.
//
// IN-REPO BASIS: the sibling handlers cmdSUNION / cmdSINTER / cmdSDIFF take
// their keys straight from index 0 — correct for THEIR grammar, which has no
// numkeys token. SINTERCARD is the only member of that family whose first
// argument is a count, and it is the only one that read index 0 as a key.
//
// Controls (must pass before AND after): the sibling SINTER still returns the
// right members, SCARD still works, and a missing key still yields 0 (which is
// the correct answer there, so the harness cannot confuse the two).
func TestProofSInterCardHonoursNumKeys(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterSetCommands(r)

	execC(s, r, "SADD", "s1", "a", "b", "c")
	execC(s, r, "SADD", "s2", "b", "c", "d")

	// ---- CONTROL 1: the sibling SINTER still computes the intersection.
	got := execC(s, r, "SINTER", "s1", "s2")
	if !strings.Contains(got, "b") || !strings.Contains(got, "c") ||
		strings.Contains(got, "a") || strings.Contains(got, "d") {
		t.Fatalf("CONTROL 1 broken harness: SINTER s1 s2 = %q, want exactly b,c", got)
	}
	t.Log("CONTROL 1 ok: sibling SINTER still intersects correctly")

	// ---- CONTROL 2: SCARD on the same key.
	if got := execC(s, r, "SCARD", "s1"); got != ":3\r\n" {
		t.Fatalf("CONTROL 2 broken harness: SCARD s1 = %q, want \":3\\r\\n\"", got)
	}
	t.Log("CONTROL 2 ok: SCARD reports 3 members for s1")

	// ---- CONTROL 3: a missing key yields 0 — coincidentally what the broken
	// handler always returned, so this pins that the fix is not just "return
	// something non-zero".
	if got := execC(s, r, "SINTERCARD", "1", "nosuchkey"); got != ":0\r\n" {
		t.Fatalf("CONTROL 3 broken harness: SINTERCARD on a missing key = %q, want \":0\\r\\n\"", got)
	}
	t.Log("CONTROL 3 ok: a missing key still yields 0")

	// ---- THE DEFECT: two real keys must intersect to 2.
	got = execC(s, r, "SINTERCARD", "2", "s1", "s2")
	if got != ":2\r\n" {
		t.Fatalf("FAIL: SINTERCARD 2 s1 s2 returned %q, want \":2\\r\\n\" — s1{a,b,c} and "+
			"s2{b,c,d} share b and c. The numkeys token was read as a key name, so the "+
			"intersection started from a phantom empty set and collapsed to 0.",
			strings.TrimSpace(got))
	}
	t.Log("PASS: SINTERCARD 2 s1 s2 returned 2")

	// ---- SINGLE-KEY FORM: with numkeys 1 the answer is that key's cardinality.
	got = execC(s, r, "SINTERCARD", "1", "s1")
	if got != ":3\r\n" {
		t.Fatalf("FAIL: SINTERCARD 1 s1 returned %q, want \":3\\r\\n\" (s1 holds three members)", got)
	}
	t.Log("PASS: the single-key form returns the key's own cardinality")

	// ---- THREE KEYS: numkeys must bound how many keys participate, so an
	// unlisted third key must NOT drag the answer down.
	execC(s, r, "SADD", "s3", "c")
	got = execC(s, r, "SINTERCARD", "2", "s1", "s2")
	if got != ":2\r\n" {
		t.Fatalf("SECONDARY FAIL: SINTERCARD 2 s1 s2 = %q, want \":2\\r\\n\" — numkeys is 2, "+
			"so the unlisted s3 must be ignored even though it would not change the result", got)
	}
	if got = execC(s, r, "SINTERCARD", "3", "s1", "s2", "s3"); got != ":1\r\n" {
		t.Fatalf("SECONDARY FAIL: SINTERCARD 3 s1 s2 s3 = %q, want \":1\\r\\n\" (only c is in all three)", got)
	}
	t.Log("PASS: numkeys bounds participation — 2 keys give 2, 3 keys give 1")

	// ---- BOUNDARY 1: an empty result is still 0.
	execC(s, r, "SADD", "s9", "zzz")
	if got = execC(s, r, "SINTERCARD", "2", "s1", "s9"); got != ":0\r\n" {
		t.Fatalf("BOUNDARY FAIL: SINTERCARD 2 s1 s9 = %q, want \":0\\r\\n\" (disjoint)", got)
	}
	t.Log("PASS: disjoint sets still yield 0")

	// ---- BOUNDARY 2: LIMIT caps the reply.
	if got = execC(s, r, "SINTERCARD", "2", "s1", "s2", "LIMIT", "1"); got != ":1\r\n" {
		t.Fatalf("BOUNDARY FAIL: SINTERCARD 2 s1 s2 LIMIT 1 = %q, want \":1\\r\\n\"", got)
	}
	if got = execC(s, r, "SINTERCARD", "2", "s1", "s2", "LIMIT", "5"); got != ":2\r\n" {
		t.Fatalf("BOUNDARY FAIL: LIMIT 5 above the cardinality = %q, want \":2\\r\\n\"", got)
	}
	t.Log("PASS: LIMIT caps the reply without changing an under-limit answer")

	// ---- A key literally named like a count must still work, proving the
	// count is consumed as a count and never looked up as a key.
	execC(s, r, "SADD", "2", "a", "b")
	if got = execC(s, r, "SINTERCARD", "1", "2"); got != ":2\r\n" {
		t.Fatalf("EDGE FAIL: SINTERCARD 1 2 = %q, want \":2\\r\\n\" — with numkeys 1 the key "+
			"\"2\" must be the only key read, giving its two members", got)
	}
	t.Log("PASS: a key named like a count is read as a key when numkeys says so")
}