package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// execPend runs the real stream handler for cmd and returns the raw reply.
func execPend(s *store.Store, r *Router, cmd string, args ...string) string {
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

// TestProofXPendingSummaryEmptyPEL is the round proof.
//
// Contract (Redis XPENDING summary form): XPENDING key group returns either the
// 4-element summary [count, smallest-ID, greatest-ID, [[consumer, count]...]]
// when there ARE pending entries, or an EMPTY ARRAY when there are none. Redis
// short-circuits on an empty PEL before building the summary — an empty PEL has
// no smallest/greatest ID to report, and a client that reads element [1] would
// otherwise be handed a null it must special-case.
//
// Defect: cmdXPENDING's summary branch handled the empty case by building the
// 4-element shape anyway, substituting two resp.NullValue() placeholders:
//
//	if pendingCount == 0 {
//	    return ctx.WriteArray([]*resp.Value{
//	        resp.IntegerValue(0), resp.NullValue(), resp.NullValue(),
//	        resp.ArrayValue([]*resp.Value{}),
//	    })
//	}
//
// so `XPENDING S g` on a group with nothing pending answered
// "*4 :0 _ _ *0" instead of "*0".
//
// IN-REPO BASIS: the sibling detail form in the same handler already returns a
// bare empty array for "nothing in range" (ctx.WriteArray(results) where
// results is empty), and the handler's own missing-stream path
// (ctx.WriteArray([]*resp.Value{})) does the same — so the empty-array reply
// is the established shape in this very function. Only the summary branch
// deviated.
//
// Controls (must pass before AND after): the summary form with actual pending
// entries still returns the full 4-element shape with real IDs; the DETAIL form
// still returns its rows; a missing group still errors.
func TestProofXPendingSummaryEmptyPEL(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterStreamCommands(r)

	execPend(s, r, "XADD", "S", "1000-1", "a", "1")
	execPend(s, r, "XADD", "S", "1000-2", "b", "2")
	execPend(s, r, "XADD", "S", "1000-3", "c", "3")
	if got := execPend(s, r, "XGROUP", "CREATE", "S", "g", "0"); got != "+OK\r\n" {
		t.Fatalf("setup broken: XGROUP CREATE = %q, want \"+OK\\r\\n\"", got)
	}

	// A group that has been read from but never re-read: entries delivered once
	// are in the PEL, so the summary is non-empty here.
	execPend(s, r, "XREADGROUP", "GROUP", "g", "c1", "STREAMS", "S", ">")

	// ---- CONTROL 1: a POPULATED PEL still returns the 4-element summary with
	// real smallest/greatest IDs.
	got := execPend(s, r, "XPENDING", "S", "g")
	if !bytes.HasPrefix([]byte(got), []byte("*4\r\n")) {
		t.Fatalf("CONTROL 1 broken harness: populated XPENDING = %q, want a 4-element summary", got)
	}
	if !bytes.Contains([]byte(got), []byte("1000-1")) || !bytes.Contains([]byte(got), []byte("1000-3")) {
		t.Fatalf("CONTROL 1 broken harness: populated XPENDING = %q, want the smallest and "+
			"greatest pending IDs", got)
	}
	t.Log("CONTROL 1 ok: a populated PEL returns the full 4-element summary")

	// ---- CONTROL 2: the DETAIL form still returns its rows.
	detail := execPend(s, r, "XPENDING", "S", "g", "-", "+", "10")
	if !bytes.Contains([]byte(detail), []byte("1000-1")) {
		t.Fatalf("CONTROL 2 broken harness: detail-form XPENDING = %q, want pending rows", detail)
	}
	t.Log("CONTROL 2 ok: the detail form still returns pending rows")

	// ---- CONTROL 3: a missing group still errors.
	if got := execPend(s, r, "XPENDING", "S", "nosuch"); !bytes.Contains([]byte(got), []byte("NOGROUP")) {
		t.Fatalf("CONTROL 3 broken harness: XPENDING on a missing group = %q, want NOGROUP", got)
	}
	t.Log("CONTROL 3 ok: a missing group still answers NOGROUP")

	// ---- Now empty the PEL: ack every pending entry. Confirm emptiness with the
	// DETAIL form, which is unaffected by the summary-shape contract under
	// test (it returns a bare array of rows either way).
	execPend(s, r, "XACK", "S", "g", "1000-1", "1000-2", "1000-3")
	if got := execPend(s, r, "XPENDING", "S", "g", "-", "+", "10"); got != "*0\r\n" {
		t.Fatalf("setup broken: after XACK the detail form should report no rows, got %q", got)
	}

	// ---- THE DEFECT: with an empty PEL the summary must be an EMPTY ARRAY.
	got = execPend(s, r, "XPENDING", "S", "g")
	if got != "*0\r\n" {
		t.Fatalf("FAIL: XPENDING S g on a group whose PEL is empty returned %q, want \"*0\\r\\n\". "+
			"The summary branch builds the 4-element shape unconditionally and substitutes "+
			"nulls for the smallest/greatest IDs; Redis returns an empty array because an empty "+
			"PEL has no IDs to report, and a client reading element [1] of a 4-element reply "+
			"would be handed a null it never expects.",
			bytes.TrimSpace([]byte(got)))
	}
	t.Log("PASS: an empty PEL now answers with an empty array")

	// ---- SECONDARY: the same must hold for a group that was created but
	// NEVER read from, which is the most common way to reach an empty PEL.
	execPend(s, r, "XADD", "T", "3000-1", "z", "9")
	if got := execPend(s, r, "XGROUP", "CREATE", "T", "fresh", "0"); got != "+OK\r\n" {
		t.Fatalf("SECONDARY setup: XGROUP CREATE = %q", got)
	}
	if got := execPend(s, r, "XPENDING", "T", "fresh"); got != "*0\r\n" {
		t.Fatalf("SECONDARY FAIL: XPENDING on a never-read group = %q, want \"*0\\r\\n\" — "+
			"a freshly created group is the commonest empty-PEL case", bytes.TrimSpace([]byte(got)))
	}
	t.Log("PASS: a never-read group reports an empty array")
}