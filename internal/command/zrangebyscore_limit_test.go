package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func execZ(s *store.Store, r *Router, cmd string, args ...string) string {
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

// membersOf renders a bare-member array reply as a comma-joined string so the
// assertions read clearly, e.g. "a,b,c".
func membersOf(reply string) string {
	if strings.HasPrefix(reply, "-") {
		return "ERR:" + strings.TrimPrefix(strings.TrimSpace(reply), "-")
	}
	var out []string
	for _, line := range strings.Split(reply, "\r\n") {
		if strings.HasPrefix(line, "$") {
			out = append(out, "<bulk>")
		}
	}
	return strings.Join(out, ",")
}

// TestProofZRangeByScoreHonoursLimit is the round proof.
//
// Contract (Redis ZRANGEBYSCORE / ZREVRANGEBYSCORE):
//
//	ZRANGEBYSCORE key min max [WITHSCORES] [LIMIT offset count]
//
// LIMIT pages the result set: `offset` members are skipped and at most
// `count` are returned. A negative count means "all remaining".
//
// Defect: both handlers parsed ONLY "WITHSCORES" and ignored every other token.
// LIMIT was therefore consumed by the loop and silently discarded, so a client
// paging a large sorted set received the ENTIRE result set while believing it
// had received one page — unbounded memory use client-side and a broken
// pagination contract, with no error to signal it.
//
// IN-REPO BASIS: the sibling handlers in the SAME file already implement LIMIT
// correctly — cmdZRANGE (sortedset_commands.go:376) and cmdZRANGEBYLEX (:883)
// parse offset/count and slice the result. Only the two *SCORE variants were
// missed.
//
// Controls (must pass before AND after): the unlimited form still returns every
// match; WITHSCORES still works; the sibling ZRANGEBYLEX LIMIT still pages.
func TestProofZRangeByScoreHonoursLimit(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterSortedSetCommands(r)

	execZ(s, r, "ZADD", "Z", "1", "a", "2", "b", "3", "c", "4", "d", "5", "e")

	// ---- CONTROL 1: the no-LIMIT form is unchanged (all matches).
	if got := membersOf(execZ(s, r, "ZRANGEBYSCORE", "Z", "-inf", "+inf")); got != "<bulk>,<bulk>,<bulk>,<bulk>,<bulk>" {
		t.Fatalf("CONTROL 1 broken harness: ZRANGEBYSCORE -inf +inf = %q, want all 5", got)
	}
	t.Log("CONTROL 1 ok: unlimited ZRANGEBYSCORE returns every match")

	// ---- CONTROL 2: WITHSCORES still works (the option this handler did parse).
	gotWS := execZ(s, r, "ZRANGEBYSCORE", "Z", "1", "2", "WITHSCORES")
	if !strings.Contains(gotWS, "a") || !strings.Contains(gotWS, "1") {
		t.Fatalf("CONTROL 2 broken harness: WITHSCORES form = %q, want member a with score 1", gotWS)
	}
	t.Log("CONTROL 2 ok: WITHSCORES still honoured")

	// ---- CONTROL 3: the sibling ZRANGEBYLEX pages correctly (in-repo basis).
	execZ(s, r, "ZADD", "L", "0", "aa", "0", "bb", "0", "cc", "0", "dd")
	lexLimited := execZ(s, r, "ZRANGEBYLEX", "L", "-", "+", "LIMIT", "1", "2")
	if lexLimited != "*2\r\n$2\r\nbb\r\n$2\r\ncc\r\n" {
		t.Fatalf("CONTROL 3 broken harness: ZRANGEBYLEX LIMIT 1 2 = %q, want exactly bb,cc", lexLimited)
	}
	t.Log("CONTROL 3 ok: sibling ZRANGEBYLEX already pages with LIMIT")

	// ---- THE DEFECT: ZRANGEBYSCORE ... LIMIT must page.
	got := execZ(s, r, "ZRANGEBYSCORE", "Z", "-inf", "+inf", "LIMIT", "0", "2")
	if got != "*2\r\n$1\r\na\r\n$1\r\nb\r\n" {
		t.Fatalf("FAIL: ZRANGEBYSCORE -inf +inf LIMIT 0 2 returned %q — want exactly a,b "+
			"(LIMIT was parsed and silently discarded, so the whole set came back)", got)
	}
	t.Log("PASS: LIMIT 0 2 returned exactly two members")

	// ---- SECONDARY: a non-zero offset must SKIP that many members.
	got = execZ(s, r, "ZRANGEBYSCORE", "Z", "-inf", "+inf", "LIMIT", "3", "2")
	if got != "*2\r\n$1\r\nd\r\n$1\r\ne\r\n" {
		t.Fatalf("FAIL: ZRANGEBYSCORE ... LIMIT 3 2 returned %q, want exactly d,e "+
			"(offset must skip the first three members)", got)
	}
	t.Log("PASS: LIMIT 3 2 skipped three and returned d,e")

	// ---- BOUNDARY 1: a negative count means "all remaining".
	got = execZ(s, r, "ZRANGEBYSCORE", "Z", "-inf", "+inf", "LIMIT", "3", "-1")
	if got != "*2\r\n$1\r\nd\r\n$1\r\ne\r\n" {
		t.Fatalf("BOUNDARY FAIL: LIMIT 3 -1 (count negative = all remaining) returned %q, want d,e", got)
	}
	t.Log("PASS: negative count returns all remaining members")

	// ---- BOUNDARY 2: an offset beyond the end returns an EMPTY array.
	got = execZ(s, r, "ZRANGEBYSCORE", "Z", "-inf", "+inf", "LIMIT", "99", "2")
	if got != "*0\r\n" {
		t.Fatalf("BOUNDARY FAIL: LIMIT 99 2 (offset past the end) returned %q, want an empty array", got)
	}
	t.Log("PASS: an offset past the end yields an empty array")

	// ---- BOUNDARY 3: LIMIT 0 0 must return nothing.
	got = execZ(s, r, "ZRANGEBYSCORE", "Z", "-inf", "+inf", "LIMIT", "0", "0")
	if got != "*0\r\n" {
		t.Fatalf("BOUNDARY FAIL: LIMIT 0 0 returned %q, want an empty array", got)
	}
	t.Log("PASS: LIMIT 0 0 yields an empty array")

	// ---- COMBINED WITH WITHSCORES: paging must not break the score pairing.
	got = execZ(s, r, "ZRANGEBYSCORE", "Z", "-inf", "+inf", "WITHSCORES", "LIMIT", "1", "2")
	if got != "*4\r\n$1\r\nb\r\n$1\r\n2\r\n$1\r\nc\r\n$1\r\n3\r\n" {
		t.Fatalf("FAIL: WITHSCORES + LIMIT 1 2 returned %q, want b,2,c,3 "+
			"(each member must stay paired with its score)", got)
	}
	t.Log("PASS: WITHSCORES stays paired with members under LIMIT")

	// ---- SECOND COMMAND: ZREVRANGEBYSCORE shares the identical defect, and its
	// arguments are (key max min) — the REVERSE order.
	got = execZ(s, r, "ZREVRANGEBYSCORE", "Z", "+inf", "-inf", "LIMIT", "0", "2")
	if got != "*2\r\n$1\r\ne\r\n$1\r\nd\r\n" {
		t.Fatalf("FAIL: ZREVRANGEBYSCORE +inf -inf LIMIT 0 2 returned %q, want e,d "+
			"(the reversed handler discards LIMIT too)", got)
	}
	t.Log("PASS: ZREVRANGEBYSCORE honours LIMIT as well")

	// ---- Its control: the reversed unlimited form is unchanged.
	got = execZ(s, r, "ZREVRANGEBYSCORE", "Z", "+inf", "-inf")
	if got != "*5\r\n$1\r\ne\r\n$1\r\nd\r\n$1\r\nc\r\n$1\r\nb\r\n$1\r\na\r\n" {
		t.Fatalf("CONTROL: ZREVRANGEBYSCORE unlimited = %q, want the 5 members descending", got)
	}
	t.Log("PASS: reversed unlimited form still returns every member")
}
