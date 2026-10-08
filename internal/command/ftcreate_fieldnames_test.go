package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// ftExec dispatches through the real registered handler and returns raw RESP.
func ftExec(s *store.Store, r *Router, cmd string, args ...string) string {
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

// countFields reports how many field definitions FT.INFO reports for an index.
// FT.INFO replies "num_docs" N "fields" then a flat name/type list, so the
// field names are the elements between "fields" and the end of the map.
func ftFieldNames(reply string) []string {
	parts := strings.Split(reply, "\r\n")
	var names []string
	inFields := false
	// RESP flat map: key, value, key, value... Values alternate; a field entry
	// is the token following a token that looks like a bare field name and
	// preceding a type. Collect every odd-position token after "fields".
	for i := 0; i < len(parts); i++ {
		if parts[i] == "fields" {
			inFields = true
			continue
		}
		if !inFields {
			continue
		}
		if parts[i] == "" {
			continue
		}
		names = append(names, parts[i])
	}
	return names
}

// TestProofFTCreateAcceptsFieldNamesWithKeywordPrefixes is the round proof.
//
// CONTRACT: FT.CREATE <index> SCHEMA <field> <type> [<field> <type> ...] takes
// arbitrary field names. A client may legitimately index a field called "Onyx",
// "Prefix" or "Stopwatch" — those are ordinary identifiers, not modifiers.
//
// DEFECT: cmdFTCREATE's schema loop decides "this token is a modifier, not a
// field name" with strings.HasPrefix:
//
//	if strings.HasPrefix(strings.ToUpper(fieldName), "ON") ||
//	   strings.HasPrefix(strings.ToUpper(fieldName), "PREFIX") ||
//	   strings.HasPrefix(strings.ToUpper(fieldName), "STOPWORDS") { break }
//
// HasPrefix("ONYX", "ON") is true, so a field named "Onyx" is treated as a
// modifier and the loop `break`s. Every field from that point on is SILENTLY
// DROPPED: the index is created and replies +OK, but it holds fewer (or zero)
// fields than asked for. The client's subsequent FT.ADD then stores documents
// whose fields are not in the schema.
//
// This is silent truncation behind a success reply — the worst failure shape
// for an index, because the data looks ingested.
//
// IN-REPO BASIS: the sibling options loop in the same handler already treats an
// unrecognized token as "the next field's NAME" (it does i-- + break options
// precisely so the outer loop re-reads it), so the schema loop is intended to
// accept any identifier in the name position. Detecting a keyword by PREFIX
// rather than by whole-token equality contradicts that design in the same
// function.
//
// Note "STOPWORDS" has the same substring shape as the common English word
// "Stopwatch"/"StopWords", and "ON" prefixes "Onyx", "Only", "Online" — all
// realistic field names.
func TestProofFTCreateAcceptsFieldNamesWithKeywordPrefixes(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterSearchCommands(r)

	// ---- CONTROL: ordinary field names all work today. This pins that the
	// harness can see a multi-field index at all.
	if out := ftExec(s, r, "FT.CREATE", "plain", "SCHEMA", "title", "TEXT", "body", "TEXT"); out != "+OK\r\n" {
		t.Fatalf("CONTROL broken harness: FT.CREATE plain returned %q, want +OK", out)
	}
	info := ftExec(s, r, "FT.INFO", "plain")
	for _, want := range []string{"title", "body"} {
		if !strings.Contains(info, want) {
			t.Fatalf("CONTROL broken harness: a two-field index did not report %q; FT.INFO=%q", want, info)
		}
	}
	t.Log("CONTROL ok: ordinary field names create both fields")

	// ---- THE DEFECT: field names that merely START WITH a modifier keyword.
	cases := []struct {
		name   string
		fields []string
		want   []string
	}{
		{"field named Onyx", []string{"Onyx", "TEXT", "title", "TEXT"}, []string{"Onyx", "title"}},
		{"field named Only", []string{"Only", "TEXT", "title", "TEXT"}, []string{"Only", "title"}},
		{"field named Online", []string{"Online", "TEXT", "title", "TEXT"}, []string{"Online", "title"}},
		{"field named PrefixSum", []string{"PrefixSum", "TEXT", "title", "TEXT"}, []string{"PrefixSum", "title"}},
		{"field named Stopwatch", []string{"Stopwatch", "TEXT", "title", "TEXT"}, []string{"Stopwatch", "title"}},
		{"lowercase onyx", []string{"onyx", "TEXT", "title", "TEXT"}, []string{"onyx", "title"}},
		{"trailing field after keyword-named one", []string{"Onyx", "TEXT", "tag", "TAG", "title", "TEXT"}, []string{"Onyx", "tag", "title"}},
	}

	for _, tc := range cases {
		idx := "i_" + strings.ToLower(tc.name)
		args := append([]string{idx, "SCHEMA"}, tc.fields...)
		if out := ftExec(s, r, "FT.CREATE", args...); out != "+OK\r\n" {
			t.Fatalf("FAIL: FT.CREATE for %s returned %q, want +OK — the whole point is that it "+
				"claims success while dropping fields", tc.name, out)
		}
		info := ftExec(s, r, "FT.INFO", idx)
		missing := []string{}
		for _, w := range tc.want {
			if !strings.Contains(info, w) {
				missing = append(missing, w)
			}
		}
		if len(missing) > 0 {
			t.Fatalf("FAIL: FT.CREATE %s silently dropped field(s) %v.\n"+
				"The schema loop detects a modifier with strings.HasPrefix, so \"Onyx\" matches "+
				"\"ON\" and the loop `break`s, abandoning every remaining field while still "+
				"replying +OK. Field names that merely START WITH a keyword must still be "+
				"fields.\nFT.INFO=%q", tc.name, missing, info)
		}
		t.Logf("PASS: %s kept every field (%v)", tc.name, tc.want)
	}

	// ---- BOUNDARY: the exact modifier tokens must STILL terminate the schema,
	// so this fix does not silently swallow the keyword's own syntax.
	for _, kw := range []string{"ON", "PREFIX", "STOPWORDS", "prefix", "StopWords"} {
		idx := "k_" + kw
		out := ftExec(s, r, "FT.CREATE", idx, "SCHEMA", "title", "TEXT", kw, "1", "title")
		if out != "+OK\r\n" {
			t.Fatalf("BOUNDARY FAIL: FT.CREATE with the exact keyword %q returned %q, want +OK", kw, out)
		}
		info := ftExec(s, r, "FT.INFO", idx)
		if !strings.Contains(info, "title") {
			t.Fatalf("BOUNDARY FAIL: the exact keyword %q swallowed the preceding field; FT.INFO=%q", kw, info)
		}
	}
	t.Log("PASS: the exact modifier tokens still terminate the schema as intended")
}
