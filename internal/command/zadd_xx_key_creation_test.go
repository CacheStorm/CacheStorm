package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// exec runs the real router handler for cmd with the given args against s and
// returns the raw RESP reply. This is the production command path: the same
// Router.Execute entry point the server uses, with only the network boundary
// replaced by a buffer.
func exec(s *store.Store, r *Router, cmd string, args ...string) string {
	handler, ok := r.Get(cmd)
	if !ok {
		return "ERR HARNESS: " + cmd + " not registered"
	}
	var buf bytes.Buffer
	ctx := NewContext(cmd, bytesArgs(args...), s, resp.NewWriter(&buf))
	if err := handler.Handler(ctx); err != nil {
		return "ERR HARNESS: handler returned " + err.Error()
	}
	return buf.String()
}

func zaddRouter() *Router {
	r := NewRouter()
	RegisterSortedSetCommands(r)
	RegisterStringCommands(r) // EXISTS / DEL, for observation only
	RegisterKeyCommands(r)    // TYPE, for observation only
	return r
}

// TestProofZADDXXMustNotCreateKey is the round proof.
//
// Contract (Redis ZADD): "XX — Only update existing keys. Never add elements."
// A ZADD XX against a key that does not exist must return 0 and must NOT
// create the key.
//
// Control: the same command shape without XX DOES create the key, and ZADD XX
// against an existing key still updates it. Both must pass before and after
// the fix, so a broken harness cannot masquerade as the defect.
func TestProofZADDXXMustNotCreateKey(t *testing.T) {
	s := store.NewStore()
	r := zaddRouter()

	// ---- CONTROL 1: ZADD without XX on a missing key DOES create it.
	if reply := exec(s, r, "ZADD", "ctl:created", "1", "a"); reply != ":1\r\n" {
		t.Fatalf("CONTROL 1 broken harness: ZADD (no XX) on missing key = %q, want \":1\\r\\n\"", reply)
	}
	if !s.Exists("ctl:created") {
		t.Fatalf("CONTROL 1 broken harness: ZADD (no XX) should create the key")
	}
	t.Logf("CONTROL 1 ok: plain ZADD created the key (reply %q)", exec(s, r, "ZADD", "ctl:created2", "1", "a"))

	// ---- CONTROL 2: ZADD XX against an EXISTING key updates it.
	// ZADD without CH replies with the number ADDED (0 when an existing member
	// is re-scored); CH makes it reply with the number CHANGED. Assert both the
	// observable score and the CH reply so this control pins real update
	// behaviour rather than a bare return value.
	if reply := exec(s, r, "ZADD", "ctl:existing", "5", "m"); reply != ":1\r\n" {
		t.Fatalf("CONTROL 2 broken harness: seeding ZADD = %q, want \":1\\r\\n\"", reply)
	}
	if reply := exec(s, r, "ZADD", "ctl:existing", "XX", "CH", "9", "m"); reply != ":1\r\n" {
		t.Fatalf("CONTROL 2 broken harness: ZADD XX CH on existing key = %q, want \":1\\r\\n\"", reply)
	}
	if got := exec(s, r, "ZSCORE", "ctl:existing", "m"); got != "$1\r\n9\r\n" {
		t.Fatalf("CONTROL 2 broken harness: ZSCORE after ZADD XX CH = %q, want score 9", got)
	}

	// ---- THE DEFECT: ZADD XX against a MISSING key.
	if reply := exec(s, r, "ZADD", "victim", "XX", "1", "m"); reply != ":0\r\n" {
		t.Fatalf("CONTROL: ZADD XX on missing key should reply 0, got %q", reply)
	}
	if s.Exists("victim") {
		t.Fatalf("FAIL: ZADD victim XX 1 m reported 0 added, but the key \"victim\" was CREATED "+
			"(EXISTS=%s, TYPE=%s) — XX must never create a key",
			exec(s, r, "EXISTS", "victim"), strings.TrimSpace(exec(s, r, "TYPE", "victim")))
	}
	t.Log("PASS: ZADD XX on a missing key left no key behind")

	// ---- SECONDARY BRANCH: the same defect via CH and GT, and INCR.
	// CH is parsed before the XX branch, so it reaches the identical
	// early-return that skips the mutation.
	if reply := exec(s, r, "ZADD", "victim:ch", "XX", "CH", "1", "m"); reply != ":0\r\n" {
		t.Fatalf("CONTROL: ZADD XX CH on missing key should reply 0, got %q", reply)
	}
	if s.Exists("victim:ch") {
		t.Fatalf("FAIL: ZADD victim:ch XX CH 1 m created the key (EXISTS=%s)",
			exec(s, r, "EXISTS", "victim:ch"))
	}
	if reply := exec(s, r, "ZADD", "victim:incr", "XX", "INCR", "1", "m"); reply != "$-1\r\n" {
		t.Fatalf("CONTROL: ZADD XX INCR on missing key should reply nil, got %q", reply)
	}
	if s.Exists("victim:incr") {
		t.Fatalf("FAIL: ZADD victim:incr XX INCR 1 m created the key (EXISTS=%s)",
			exec(s, r, "EXISTS", "victim:incr"))
	}
	t.Log("PASS: XX CH and XX INCR branches leave no key behind")
}