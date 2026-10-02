package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// execS runs the real router handler for cmd and returns the raw RESP reply.
func execS(s *store.Store, r *Router, cmd string, args ...string) string {
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

func isWrongType(reply string) bool {
	return strings.HasPrefix(reply, "-") && strings.Contains(reply, "WRONGTYPE")
}

// TestProofStringCommandsAcceptBitmapRepresentation is the round proof.
//
// Contract (Redis): a bit is NOT a distinct type. SETBIT/BITCOUNT/BITFIELD all
// operate on an ordinary string value, so a key touched by SETBIT still has
// TYPE "string" and EVERY string command must work on it — STRLEN returns the
// length, APPEND extends it, GETRANGE reads it, and so on.
//
// CacheStorm stores bitmap bits in a *command.BitmapValue (a second Go
// representation of the same Redis string type). Only cmdGET learned to accept
// it; every other string command type-asserts *store.StringValue and answers
// WRONGTYPE. That is a SELF-CONTRADICTION, provable without appealing to Redis:
// the server reports `TYPE B` == "+string" and serves `GET B` happily, then
// refuses `STRLEN B` on that very key.
//
// Controls (must pass before AND after):
//   - a real string key still behaves (TYPE/GET/STRLEN/APPEND);
//   - GET on a bitmap key already works — the in-repo precedent for the fix;
//   - the bitmap commands themselves still work;
//   - a genuinely non-string key (a list) STILL gets WRONGTYPE, so the fix
//     must not turn the type check into a blanket allow.
func TestProofStringCommandsAcceptBitmapRepresentation(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterBitmapCommands(r)
	RegisterStringCommands(r)
	RegisterKeyCommands(r)
	RegisterListCommands(r)

	// ---- CONTROL 1: an ordinary string key.
	if got := execS(s, r, "SET", "plain", "hello"); got != "+OK\r\n" {
		t.Fatalf("CONTROL 1 broken harness: SET = %q", got)
	}
	if got := execS(s, r, "STRLEN", "plain"); got != ":5\r\n" {
		t.Fatalf("CONTROL 1 broken harness: STRLEN plain = %q, want \":5\\r\\n\"", got)
	}
	if got := execS(s, r, "GETRANGE", "plain", "1", "3"); got != "$3\r\nell\r\n" {
		t.Fatalf("CONTROL 1 broken harness: GETRANGE plain 1 3 = %q", got)
	}
	t.Log("CONTROL 1 ok: a real string key serves every string command")

	// ---- CONTROL 2: GET on a bitmap key already works (the in-repo precedent).
	if got := execS(s, r, "SETBIT", "precedent", "0", "1"); got != ":0\r\n" {
		t.Fatalf("CONTROL 2 broken harness: SETBIT = %q", got)
	}
	if got := execS(s, r, "GET", "precedent"); got != "$1\r\n\x01\r\n" {
		t.Fatalf("CONTROL 2 broken harness: GET on a bitmap key = %q, want the byte", got)
	}
	t.Log("CONTROL 2 ok: GET already accepts the bitmap representation")

	// ---- CONTROL 3: the bitmap commands themselves still work.
	if got := execS(s, r, "GETBIT", "precedent", "0"); got != ":1\r\n" {
		t.Fatalf("CONTROL 3 broken harness: GETBIT = %q, want \":1\\r\\n\"", got)
	}
	t.Log("CONTROL 3 ok: bitmap commands unaffected")

	// ---- CONTROL 4: a genuinely non-string key still gets WRONGTYPE.
	if got := execS(s, r, "RPUSH", "alist", "x"); got != ":1\r\n" {
		t.Fatalf("CONTROL 4 broken harness: RPUSH = %q", got)
	}
	if got := execS(s, r, "STRLEN", "alist"); !isWrongType(got) {
		t.Fatalf("CONTROL 4 broken harness: STRLEN on a list = %q, want WRONGTYPE", got)
	}
	if got := execS(s, r, "GET", "alist"); !isWrongType(got) {
		t.Fatalf("CONTROL 4 broken harness: GET on a list = %q, want WRONGTYPE", got)
	}
	t.Log("CONTROL 4 ok: a list is still correctly rejected with WRONGTYPE")

	// ---- THE DEFECT: the server calls the key a string, then refuses it.
	if got := execS(s, r, "TYPE", "precedent"); got != "+string\r\n" {
		t.Fatalf("DEFECT setup: TYPE precedent = %q, want \"+string\\r\\n\"", got)
	}

	for _, tc := range []struct {
		cmd  string
		args []string
		want string
	}{
		{"STRLEN", []string{"B"}, ":1\r\n"},
		{"GETRANGE", []string{"B", "0", "-1"}, "$1\r\n\x01\r\n"},
	} {
		execS(s, r, "SETBIT", "B", "0", "1")
		got := execS(s, r, tc.cmd, tc.args...)
		if isWrongType(got) {
			t.Fatalf("FAIL: TYPE B reports \"string\" and GET B works, but %s B answered %q — "+
				"a key whose type the server itself reports as string must not be refused "+
				"by the string family", tc.cmd, strings.TrimSpace(got))
		}
		if got != tc.want {
			t.Fatalf("FAIL: %s B = %q, want %q", tc.cmd, got, tc.want)
		}
	}
	t.Log("PASS: STRLEN and GETRANGE now serve a key TYPE calls a string")

	// ---- The mutating commands must accept it too, not just the readers.
	execS(s, r, "SETBIT", "C", "0", "1")
	if got := execS(s, r, "APPEND", "C", "x"); isWrongType(got) {
		t.Fatalf("FAIL: APPEND C x answered %q — APPEND must extend a key TYPE calls a string",
			strings.TrimSpace(got))
	}
	if got := execS(s, r, "GET", "C"); got != "$2\r\n\x01x\r\n" {
		t.Fatalf("FAIL: after APPEND, GET C = %q, want the two bytes 0x01 'x'", got)
	}
	t.Log("PASS: APPEND extends a bitmap-typed key and the bytes survive")

	// ---- SECONDARY: SETRANGE writes into it and STRLEN must agree afterwards.
	execS(s, r, "SETBIT", "D", "0", "1")
	if got := execS(s, r, "SETRANGE", "D", "0", "z"); isWrongType(got) {
		t.Fatalf("SECONDARY FAIL: SETRANGE D 0 z answered %q", strings.TrimSpace(got))
	}
	if got := execS(s, r, "GET", "D"); got != "$1\r\nz\r\n" {
		t.Fatalf("SECONDARY FAIL: after SETRANGE, GET D = %q, want \"z\"", got)
	}
	if got := execS(s, r, "STRLEN", "D"); got != ":1\r\n" {
		t.Fatalf("SECONDARY FAIL: after SETRANGE, STRLEN D = %q, want \":1\\r\\n\"", got)
	}
	t.Log("PASS: SETRANGE overwrites and STRLEN agrees afterwards")

	// ---- TERTIARY: the bitmap view must survive a string write, i.e. the two
	// representations are genuinely interchangeable in both directions.
	execS(s, r, "SETBIT", "E", "2", "1")
	if got := execS(s, r, "GETBIT", "E", "2"); got != ":1\r\n" {
		t.Fatalf("TERTIARY setup: GETBIT E 2 = %q, want \":1\\r\\n\"", got)
	}
	if got := execS(s, r, "APPEND", "E", "zz"); isWrongType(got) {
		t.Fatalf("TERTIARY FAIL: APPEND E zz answered %q", strings.TrimSpace(got))
	}
	if got := execS(s, r, "GETBIT", "E", "2"); got != ":1\r\n" {
		t.Fatalf("FAIL: the bit set before APPEND was lost — GETBIT E 2 = %q, want \":1\\r\\n\"", got)
	}
	t.Log("PASS: bitmap bits survive a string APPEND — representations are interchangeable")

	// ---- BOUNDARY: a bitmap whose length grew past one byte.
	execS(s, r, "SETBIT", "F", "9", "1")
	if got := execS(s, r, "STRLEN", "F"); got != ":2\r\n" {
		t.Fatalf("BOUNDARY FAIL: STRLEN F = %q, want \":2\\r\\n\" (bit 9 needs two bytes)", got)
	}
	t.Log("PASS: STRLEN reflects the real byte length of a multi-byte bitmap")
}