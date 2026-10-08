package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// execStream runs the real stream handler for cmd and returns the raw reply.
func execStream(s *store.Store, r *Router, cmd string, args ...string) string {
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

// TestProofXReadGroupReportsMissingStream is the round proof.
//
// Contract (Redis XREADGROUP): the command can only read through a consumer
// GROUP. A stream key that does not exist has no group either, so Redis answers
// "NOGROUP No such key 'X' or consumer group 'Y' in XREADGROUP with GROUP
// option" — the caller learns the stream/group is not there.
//
// Defect: cmdXREADGROUP's per-stream loop did
//
//	stream := getStream(ctx, key)
//	if stream == nil { continue }                 // missing STREAM: silently skipped
//	group := stream.GetGroup(groupName)
//	if group == nil { return ctx.WriteError(ErrNoGroup) }   // missing GROUP: errors
//
// One condition — "there is no group to read through" — got two opposite
// answers in adjacent lines of the same loop. A missing stream produced an
// empty/null reply, so a consumer silently believed the stream simply had no
// new messages and would never retry or report the missing group.
//
// IN-REPO BASIS: the NOGROUP arm is three lines below the skip, and the same
// handler already returns ErrNoGroup for a missing group in the other read
// paths, so the correct behaviour already exists in this file.
//
// Controls (must pass before AND after): a real stream+group still delivers;
// a missing GROUP on an EXISTING stream still returns NOGROUP; XACK/XPENDING
// keep their missing-group behaviour.
func TestProofXReadGroupReportsMissingStream(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterStreamCommands(r)

	execStream(s, r, "XADD", "S", "1000-1", "a", "1")
	execStream(s, r, "XADD", "S", "1000-2", "b", "2")
	if got := execStream(s, r, "XGROUP", "CREATE", "S", "g", "0"); got != "+OK\r\n" {
		t.Fatalf("setup broken: XGROUP CREATE = %q, want \"+OK\\r\\n\"", got)
	}

	// ---- CONTROL 1: an existing stream+group still delivers.
	got := execStream(s, r, "XREADGROUP", "GROUP", "g", "c1", "STREAMS", "S", ">")
	if !bytes.Contains([]byte(got), []byte("1000-1")) || !bytes.Contains([]byte(got), []byte("1000-2")) {
		t.Fatalf("CONTROL 1 broken harness: XREADGROUP on a real group = %q, want both entries", got)
	}
	t.Log("CONTROL 1 ok: an existing stream and group still deliver")

	// ---- CONTROL 2: a missing GROUP on an EXISTING stream is NOGROUP. This is
	// the in-repo basis proving the missing-STREAM path should behave the same.
	got = execStream(s, r, "XREADGROUP", "GROUP", "nope", "c1", "STREAMS", "S", ">")
	if !bytes.Contains([]byte(got), []byte("NOGROUP")) {
		t.Fatalf("CONTROL 2 broken harness: missing group = %q, want NOGROUP", got)
	}
	t.Log("CONTROL 2 ok: a missing group on an existing stream answers NOGROUP")

	// ---- CONTROL 3: XACK/XPENDING keep reporting a missing group.
	if got := execStream(s, r, "XPENDING", "S", "nope"); !bytes.Contains([]byte(got), []byte("NOGROUP")) {
		t.Fatalf("CONTROL 3 broken harness: XPENDING on a missing group = %q, want NOGROUP", got)
	}
	t.Log("CONTROL 3 ok: XPENDING on a missing group answers NOGROUP")

	// ---- THE DEFECT: a missing STREAM must be NOGROUP, not an empty reply.
	got = execStream(s, r, "XREADGROUP", "GROUP", "g", "c1", "STREAMS", "NOSUCHSTREAM", ">")
	if !bytes.Contains([]byte(got), []byte("NOGROUP")) {
		t.Fatalf("FAIL: XREADGROUP on the non-existent stream NOSUCHSTREAM returned %q, want a "+
			"NOGROUP error. The loop `continue`s when the stream is missing, so the caller is "+
			"told \"no new messages\" instead of \"this stream/group does not exist\" — one "+
			"condition with two opposite answers three lines apart.",
			bytes.TrimSpace([]byte(got)))
	}
	t.Log("PASS: a missing stream now reports NOGROUP")

	// ---- SECONDARY: the same must hold on the pending-history read (id 0),
	// which takes an entirely separate branch of the same loop.
	got = execStream(s, r, "XREADGROUP", "GROUP", "g", "c1", "STREAMS", "NOSUCHSTREAM", "0")
	if !bytes.Contains([]byte(got), []byte("NOGROUP")) {
		t.Fatalf("SECONDARY FAIL: the pending-history read (id 0) on a missing stream = %q, want "+
			"NOGROUP — the guard must sit before both branches", bytes.TrimSpace([]byte(got)))
	}
	t.Log("PASS: the pending-history read reports NOGROUP too")

	// ---- TERTIARY: with several streams, one missing must not silently
	// produce a partial reply that hides the missing stream. Redis syntax is
	// "STREAMS key [key ...] id [id ...]", so two streams need TWO ids.
	got = execStream(s, r, "XREADGROUP", "GROUP", "g", "c2", "STREAMS", "S", "NOSUCHSTREAM", ">", ">")
	if !bytes.Contains([]byte(got), []byte("NOGROUP")) {
		t.Fatalf("TERTIARY FAIL: reading S plus a missing stream = %q, want NOGROUP — skipping "+
			"the missing key would hand back a partial result that hides it",
			bytes.TrimSpace([]byte(got)))
	}
	t.Log("PASS: a missing stream among several is reported, not hidden")

	// ---- The stream and group created at the top must be untouched by the
	// rejected reads above.
	if got := execStream(s, r, "XLEN", "S"); got != ":2\r\n" {
		t.Fatalf("CONTROL 4 broken harness: XLEN S = %q, want \":2\\r\\n\"", got)
	}
	t.Log("CONTROL 4 ok: the existing stream is untouched")
}
