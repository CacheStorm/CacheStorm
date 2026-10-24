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

// TestProofXPendingSummaryEmptyPEL pins the Redis 7.4 summary form of
// XPENDING: [count, smallest-ID, greatest-ID, [[consumer, count]...]] when
// entries are pending, and [0, nil, nil, nil-array] when none are (verified
// against redis:7.4: "*4\r\n:0\r\n$-1\r\n$-1\r\n*-1\r\n"). An earlier
// version of this test expected an empty array, which is not what Redis sends.
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

	const emptySummary = "*4\r\n:0\r\n$-1\r\n$-1\r\n*-1\r\n"
	got = execPend(s, r, "XPENDING", "S", "g")
	if got != emptySummary {
		t.Fatalf("FAIL: XPENDING S g on a group whose PEL is empty returned %q, want %q", got, emptySummary)
	}
	t.Log("PASS: an empty PEL answers with the zero summary")

	// ---- SECONDARY: the same must hold for a group that was created but
	// NEVER read from, which is the most common way to reach an empty PEL.
	execPend(s, r, "XADD", "T", "3000-1", "z", "9")
	if got := execPend(s, r, "XGROUP", "CREATE", "T", "fresh", "0"); got != "+OK\r\n" {
		t.Fatalf("SECONDARY setup: XGROUP CREATE = %q", got)
	}
	if got := execPend(s, r, "XPENDING", "T", "fresh"); got != emptySummary {
		t.Fatalf("SECONDARY FAIL: XPENDING on a never-read group = %q, want %q", got, emptySummary)
	}
	t.Log("PASS: a never-read group reports the zero summary")
}
