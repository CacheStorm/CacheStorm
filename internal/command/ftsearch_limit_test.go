package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/search"
	"github.com/cachestorm/cachestorm/internal/store"
)

// runSearchCmd dispatches through the REAL registered handler, capturing a
// panic instead of letting it kill the test binary.
func runSearchCmd(s *store.Store, r *Router, cmd string, args ...string) (out string, panicked bool) {
	defer func() {
		if rec := recover(); rec != nil {
			panicked = true
			out = "PANIC: " + strings.TrimSpace(toStr(rec))
		}
	}()
	h, ok := r.Get(cmd)
	if !ok {
		return "ERR HARNESS: " + cmd + " not registered", false
	}
	buf := &bytes.Buffer{}
	ctx := NewContext(cmd, bytesArgs(args...), s, resp.NewWriter(buf))
	if err := h.Handler(ctx); err != nil {
		return "ERR HARNESS: " + err.Error(), false
	}
	return buf.String(), false
}

func toStr(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case error:
		return t.Error()
	default:
		return "unprintable"
	}
}

// TestProofFTSearchRejectsNegativeLimit is the round proof.
//
// CONTRACT-INDEPENDENT ANCHOR: a negative LIMIT must not make the server index
// a Go slice at a negative position. That is a crash regardless of what Redis
// itself does, so this proof needs no reference server to condemn the code —
// the same reasoning that anchored round 9's BITOP proof on commutativity.
//
// DEFECT: cmdFTSEARCH parses the LIMIT option with parseInt64
// (internal/command/timeseries_commands.go:454), which returns a negative value
// verbatim. It passes that offset straight into search.Index.Search
// (internal/search/search.go:177), which does
//
//     if offset >= len(scored) { return empty }
//     end := offset + limit
//     for i := offset; i < end; i++ { ... scored[i].id ... }
//
// A negative offset fails the `offset >= len(scored)` guard (negative is never
// >= len), reaches the loop, and indexes scored[-1] — panic: index out of
// range.
//
// SearchField (search.go:227) has the identical unguarded shape.
//
// IN-REPO BASIS: this is the only FT.* command that accepts a user-supplied
// LIMIT at all — cmdFTAGGREGATE hardcodes Search(query, 100, 0), and every
// other command in this codebase that takes an offset validates it first.
func TestProofFTSearchRejectsNegativeLimit(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterSearchCommands(r)

	if out, panicked := runSearchCmd(s, r, "FT.CREATE", "idx", "SCHEMA", "body", "TEXT"); panicked || out != "+OK\r\n" {
		t.Fatalf("harness: FT.CREATE returned %q (panicked=%v), want +OK", out, panicked)
	}
	// FT.ADD's real signature is: FT.ADD <index> <docID> <score> FIELDS <name> <value>...
// Passing "body" in the score position made the document index the field
// hello="world", so the query matched nothing and CONTROL 1 failed for a
// harness reason rather than exposing the defect.
if out, panicked := runSearchCmd(s, r, "FT.ADD", "idx", "doc1", "1.0", "FIELDS", "body", "hello world"); panicked || out != "+OK\r\n" {
		t.Fatalf("harness: FT.ADD returned %q (panicked=%v), want +OK", out, panicked)
	}

	// ---- CONTROL 1: a valid LIMIT still pages correctly.
	// FT.SEARCH replies with an ARRAY whose first element is the total count,
	// so the reply is "*N\r\n:<total>\r\n..." — not a bare integer.
	out, panicked := runSearchCmd(s, r, "FT.SEARCH", "idx", "hello", "LIMIT", "0", "1")
	if panicked {
		t.Fatalf("CONTROL 1 broken harness: a VALID limit panicked: %s", out)
	}
	if !strings.HasPrefix(out, "*") || !strings.Contains(out, ":1\r\n") || !strings.Contains(out, "doc1") {
		t.Fatalf("CONTROL 1 broken harness: FT.SEARCH with a valid LIMIT returned %q, want an "+
			"array reporting total 1 and the doc1 hit", out)
	}
	t.Log("CONTROL 1 ok: a valid LIMIT pages normally")

	// ---- CONTROL 2: a non-existent index still errors rather than panicking.
	// Uses a VALID limit so this isolates the index lookup. (With a negative
	// limit the new argument validation fires first and returns the LIMIT error,
	// which is also correct — argument validation before resource lookup — but it
	// would stop this control from testing what it is meant to test.)
	out, panicked = runSearchCmd(s, r, "FT.SEARCH", "no-such-index", "hello", "LIMIT", "0", "1")
	if panicked {
		t.Fatalf("CONTROL 2 broken harness: a missing index panicked: %s", out)
	}
	if !strings.Contains(out, "index not found") {
		t.Fatalf("CONTROL 2 broken harness: missing index returned %q, want an index-not-found error", out)
	}
	t.Log("CONTROL 2 ok: a missing index errors before any search runs")

	// ---- THE DEFECT: a negative LIMIT offset must not panic.
	for _, tc := range []struct{ name string; args []string }{
		{"negative offset", []string{"idx", "hello", "LIMIT", "-1", "1"}},
		{"large negative offset", []string{"idx", "hello", "LIMIT", "-100", "5"}},
		{"negative offset with negative limit", []string{"idx", "hello", "LIMIT", "-1", "-1"}},
		{"negative limit only", []string{"idx", "hello", "LIMIT", "0", "-1"}},
		{"mixed negative offset", []string{"idx", "hello", "NOCONTENT", "LIMIT", "-2", "3"}},
	} {
		out, panicked := runSearchCmd(s, r, "FT.SEARCH", tc.args...)
		if panicked {
			t.Fatalf("FAIL: FT.SEARCH %s panicked: %s\n"+
				"cmdFTSEARCH passes the LIMIT offset through parseInt64 into "+
				"search.Index.Search, whose `offset >= len(scored)` guard does not catch a "+
				"negative value, so the result loop indexes scored[%s] — index out of range. "+
				"A negative LIMIT must be rejected, never used as a slice index.",
				tc.name, out, "offset")
		}
		if !strings.HasPrefix(out, "-") {
			t.Fatalf("FAIL: FT.SEARCH %s returned %q, want a RESP error rejecting the negative LIMIT",
				tc.name, out)
		}
		t.Logf("PASS: %s is rejected with %q", tc.name, strings.SplitN(out, "\r\n", 2)[0])
	}

	// ---- LIBRARY LEVEL: the engine itself must not panic, so no future caller
	// of Search/SearchField can reintroduce this.
	idx, ok := search.GetIndexManager().GetIndex("idx")
	if !ok {
		t.Fatalf("harness: index not found after FT.CREATE")
	}
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				t.Fatalf("FAIL: search.Index.Search panicked on a negative offset (%v). The "+
					"guard belongs in the engine so every caller is protected, not just "+
					"cmdFTSEARCH.", rec)
			}
		}()
		res := idx.Search("hello", 5, -1)
		t.Logf("PASS: Search(offset=-1) returned %d documents without panicking", len(res.Documents))
	}()

	func() {
		defer func() {
			if rec := recover(); rec != nil {
				t.Fatalf("FAIL: search.Index.SearchField panicked on a negative offset (%v).", rec)
			}
		}()
		res := idx.SearchField("body", "hello", 5, -1)
		t.Logf("PASS: SearchField(offset=-1) returned %d documents without panicking", len(res.Documents))
	}()
}