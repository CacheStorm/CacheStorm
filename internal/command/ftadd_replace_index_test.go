package command

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// addExec dispatches through the real registered handler and returns raw RESP.
func addExec(s *store.Store, r *Router, cmd string, args ...string) string {
	h, ok := r.Get(cmd)
	if !ok {
		return "ERR HARNESS: " + cmd + " not registered"
	}
	buf := &bytes.Buffer{}
	ctx := NewContext(cmd, bytesArgs(args...), s, resp.NewWriter(buf))
	if err := h.Handler(ctx); err != nil {
		return "ERR HARNESS: " + err.Error()
	}
	return buf.String()
}

// searchTotal extracts the total count from an FT.SEARCH reply. The reply is an
// array whose FIRST ELEMENT is the total: "*3\r\n:1\r\n$4\r\ndoc1\r\n...".
func searchTotal(reply string) int {
	parts := strings.Split(reply, "\r\n")
	if len(parts) < 2 || !strings.HasPrefix(parts[0], "*") {
		return -1
	}
	n, err := strconv.Atoi(strings.TrimPrefix(parts[1], ":"))
	if err != nil {
		return -1
	}
	return n
}

// TestProofFTAddReplaceReconcilesIndexEntries is the round proof.
//
// CONTRACT: FT.ADD <index> <docID> <score> REPLACE FIELDS <name> <value>...
// replaces an existing document. After a replace, the document's OLD index
// entries must be gone: searching for a term the document no longer contains
// must not return it. A search index that returns a document for a term the
// document does not contain is returning a false match.
//
// DEFECT, two parts in series:
//   - cmdFTADD parses a REPLACE flag into a local bool and then discards it
//     (`_ = replace`), so REPLACE and plain ADD take the identical path.
//   - search.Index.AddDocument never reconciles a document that is already
//     present: it does `idx.Documents[doc.ID] = doc` and then APPENDS new
//     postings, without removing the postings the previous revision created.
//     idx.Inverted[token] is a map keyed by docID, so re-adding a document
//     under a new field value leaves the OLD token still mapped to that docID.
//
// Net effect: replace doc1's body from "alpha" to "beta" and FT.SEARCH idx
// alpha still reports doc1 as a hit.
//
// IN-REPO BASIS: the sibling cmdFTDEL already routes through
// Index.DeleteDocument, which carefully removes every posting belonging to a
// document before it is removed. The teardown half of the lifecycle exists in
// this codebase; only the replace path fails to use it.
func TestProofFTAddReplaceReconcilesIndexEntries(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterSearchCommands(r)

	if out := addExec(s, r, "FT.CREATE", "idx", "SCHEMA", "body", "TEXT"); out != "+OK\r\n" {
		t.Fatalf("CONTROL broken harness: FT.CREATE returned %q, want +OK", out)
	}

	// ---- CONTROL 1: indexing then searching works at all today.
	if out := addExec(s, r, "FT.ADD", "idx", "doc1", "1.0", "FIELDS", "body", "alpha"); out != "+OK\r\n" {
		t.Fatalf("CONTROL broken harness: FT.ADD returned %q, want +OK", out)
	}
	if got := searchTotal(addExec(s, r, "FT.SEARCH", "idx", "alpha")); got != 1 {
		t.Fatalf("CONTROL broken harness: searching the indexed term returned total %d, want 1", got)
	}
	t.Log("CONTROL 1 ok: a freshly indexed document is findable")

	// ---- CONTROL 2: two DISTINCT documents are both findable.
	if out := addExec(s, r, "FT.ADD", "idx", "doc2", "1.0", "FIELDS", "body", "gamma"); out != "+OK\r\n" {
		t.Fatalf("CONTROL broken harness: FT.ADD doc2 returned %q, want +OK", out)
	}
	if got := searchTotal(addExec(s, r, "FT.SEARCH", "idx", "gamma")); got != 1 {
		t.Fatalf("CONTROL broken harness: searching doc2's term returned total %d, want 1", got)
	}
	t.Log("CONTROL 2 ok: distinct documents are independently findable")

	// ---- THE DEFECT: replace doc1's content, then search the OLD term.
	if out := addExec(s, r, "FT.ADD", "idx", "doc1", "1.0", "REPLACE", "FIELDS", "body", "beta"); out != "+OK\r\n" {
		t.Fatalf("harness: FT.ADD ... REPLACE returned %q, want +OK", out)
	}

	// The new term must be findable.
	if got := searchTotal(addExec(s, r, "FT.SEARCH", "idx", "beta")); got != 1 {
		t.Fatalf("harness: the replaced term is not findable (total %d) — the harness cannot "+
			"evaluate the defect", got)
	}

	// THE DEFECT: the old term must now match NOTHING.
	reply := addExec(s, r, "FT.SEARCH", "idx", "alpha")
	if got := searchTotal(reply); got != 0 {
		t.Fatalf("FAIL: after FT.ADD ... REPLACE rewrote doc1's body from \"alpha\" to \"beta\", "+
			"FT.SEARCH idx alpha still reports %d hit(s). The document no longer contains the "+
			"term, so it must not be a match.\n"+
			"cmdFTADD parses REPLACE into a local bool and discards it (`_ = replace`), and "+
			"Index.AddDocument appends new postings without removing the previous revision's, so "+
			"idx.Inverted[\"alpha\"][doc1] survives. The sibling cmdFTDEL already routes through "+
			"Index.DeleteDocument for exactly this teardown.\nreply=%q",
			got, reply)
	}
	t.Log("PASS: the replaced-away term no longer matches")

	// ---- SECONDARY: replacing with a DIFFERENT FIELD SET must also clear the
	// old field's postings entirely.
	if out := addExec(s, r, "FT.CREATE", "idx2", "SCHEMA", "body", "TEXT", "note", "TEXT"); out != "+OK\r\n" {
		t.Fatalf("harness: FT.CREATE idx2 returned %q, want +OK", out)
	}
	addExec(s, r, "FT.ADD", "idx2", "d1", "1.0", "FIELDS", "body", "zulu", "note", "yankee")
	addExec(s, r, "FT.ADD", "idx2", "d1", "1.0", "REPLACE", "FIELDS", "body", "zulu")
	reply = addExec(s, r, "FT.SEARCH", "idx2", "yankee")
	if got := searchTotal(reply); got != 0 {
		t.Fatalf("BOUNDARY FAIL: after replacing d1 without its \"note\" field, "+
			"FT.SEARCH idx2 yankee still reports %d hit(s) — postings from the dropped field "+
			"must be removed too.\nreply=%q", got, reply)
	}
	t.Log("PASS: postings for a dropped field are cleared on replace")
}
