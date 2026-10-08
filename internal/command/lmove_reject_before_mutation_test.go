package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// execL runs the real router handler for cmd and returns the RESP reply.
func execL(s *store.Store, r *Router, cmd string, args ...string) string {
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

// listState renders a list key straight from the store so the assertion never
// trusts a command's own reply. Returns "<missing>" for an absent key.
func listState(s *store.Store, key string) string {
	entry, exists := s.Get(key)
	if !exists {
		return "<missing>"
	}
	list, ok := entry.Value.(*store.ListValue)
	if !ok {
		return "<wrongtype>"
	}
	var sb strings.Builder
	sb.WriteString("[")
	for _, e := range list.Elements {
		sb.WriteString(string(e))
		sb.WriteString(" ")
	}
	sb.WriteString("]")
	return sb.String()
}

// TestProofLMoveRejectsBeforeMutating is the round proof.
//
// Contract (Redis LMOVE): "LMOVE source destination LEFT|RIGHT LEFT|RIGHT".
// The two direction arguments are SYNTAX. A syntax error must be rejected
// before any state changes, so a rejected call must leave both lists exactly
// as they were.
//
// Defect: cmdLMOVE popped the element off the source (and deleted the source
// key when it emptied, and created the destination key via getOrCreateList)
// BEFORE validating whereTo. Its switch on whereTo hit `default: return
// ErrSyntaxError` only at the very END — after the element had already been
// removed from the source and the destination key had already been created.
// A client typo therefore DESTROYS one element of a list and silently loses
// the value, while the reply says only "syntax error".
//
// IN-REPO BASIS: the sibling cmdBLMOVE (list_commands.go:617-622) validates
// BOTH whereFrom and whereTo up front, before calling tryListMove — so the
// correct ordering already exists in this file.
//
// Controls (must pass before AND after): a valid LMOVE still moves correctly,
// and an invalid whereFrom is still rejected without touching the source.
func TestProofLMoveRejectsBeforeMutating(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterListCommands(r)

	// ---- CONTROL 1: a valid LMOVE still moves element and mutates both lists.
	execL(s, r, "RPUSH", "src", "a", "b", "c")
	if got := listState(s, "src"); got != "[a b c ]" {
		t.Fatalf("CONTROL 1 broken harness: seed src = %q, want \"[a b c ]\"", got)
	}
	if got := execL(s, r, "LMOVE", "src", "dst", "RIGHT", "RIGHT"); got != "$1\r\nc\r\n" {
		t.Fatalf("CONTROL 1 broken harness: LMOVE = %q, want \"$1\\r\\nc\\r\\n\"", got)
	}
	if got := listState(s, "src"); got != "[a b ]" {
		t.Fatalf("CONTROL 1 broken harness: src after LMOVE = %q, want \"[a b ]\"", got)
	}
	if got := listState(s, "dst"); got != "[c ]" {
		t.Fatalf("CONTROL 1 broken harness: dst after LMOVE = %q, want \"[c ]\"", got)
	}
	t.Log("CONTROL 1 ok: valid LMOVE moves correctly")

	// ---- CONTROL 2: an invalid whereFrom is rejected WITHOUT mutating.
	// (whereFrom is validated before the pop, so this path was already safe.)
	execL(s, r, "RPUSH", "cf", "p", "q")
	if got := execL(s, r, "LMOVE", "cf", "cd", "MIDDLE", "LEFT"); got != "-ERR syntax error\r\n" {
		t.Fatalf("CONTROL 2 broken harness: bad whereFrom = %q, want a syntax error", got)
	}
	if got := listState(s, "cf"); got != "[p q ]" {
		t.Fatalf("CONTROL 2 broken harness: bad whereFrom mutated the source: %q", got)
	}
	t.Log("CONTROL 2 ok: invalid whereFrom rejected without mutation")

	// ---- THE DEFECT: invalid whereTo on a SINGLE-element source.
	// The pop empties the list, the key is deleted, getOrCreateList creates the
	// destination — all before the whereTo switch rejects "BOGUS".
	execL(s, r, "RPUSH", "victim", "only")
	before := listState(s, "victim")

	got := execL(s, r, "LMOVE", "victim", "victim:dst", "RIGHT", "BOGUS")
	afterSrc := listState(s, "victim")
	afterDst := listState(s, "victim:dst")

	if !strings.HasPrefix(got, "-ERR syntax error") {
		t.Fatalf("DEFECT case harness: LMOVE with bad whereTo = %q, want a syntax error", got)
	}
	if afterSrc != before {
		t.Fatalf("FAIL: LMOVE victim victim:dst RIGHT BOGUS replied %q but MUTATED the "+
			"source: it was %q before and is %q after — a rejected command must not "+
			"move or destroy an element", strings.TrimSpace(got), before, afterSrc)
	}
	if afterDst != "<missing>" {
		t.Fatalf("FAIL: the rejected LMOVE created destination key \"victim:dst\" = %q; "+
			"a syntax error must not create any key", afterDst)
	}
	t.Log("PASS: rejected LMOVE left both keys untouched")

	// ---- SECONDARY: the same data loss with a multi-element source. Here the
	// element is silently REMOVED rather than the key disappearing, which is
	// the more insidious form — the key still exists, just shorter.
	execL(s, r, "RPUSH", "multi", "x", "y", "z")
	if got := execL(s, r, "LMOVE", "multi", "multi:dst", "LEFT", "BOGUS"); got != "-ERR syntax error\r\n" {
		t.Fatalf("SECONDARY setup: expected syntax error, got %q", got)
	}
	if got := listState(s, "multi"); got != "[x y z ]" {
		t.Fatalf("FAIL: rejected LMOVE silently removed an element: source is now %q, "+
			"want \"[x y z ]\" unchanged — an element was destroyed by a command that "+
			"replied with a syntax error", got)
	}
	if got := listState(s, "multi:dst"); got != "<missing>" {
		t.Fatalf("FAIL: rejected LMOVE created destination \"multi:dst\" = %q", got)
	}
	t.Log("PASS: rejected LMOVE left the multi-element source intact")
}
