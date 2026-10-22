package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// OBJECT.FROMENTRIES takes a name followed by field/value PAIRS. Its handler
// guarded only `ArgCount() < 2`, so a call carrying only the name was accepted:
// the pair loop (`for i := 1; i+1 < ctx.ArgCount(); i += 2`) never ran, an empty
// object was still stored under that name, and the command replied +OK.
//
// Redis rejects that arity, and the sibling writer cmdOBJECTSET guards `< 3`.
// The same gap silently dropped a trailing odd field name, so
// `OBJECT.FROMENTRIES obj f1 v1 f2` reported success while discarding f2.
func TestRegressionObjectFromEntriesRejectsBadArity(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	// A name plus a single dangling field is not a valid call: there is no pair.
	// ArgCount() is 2 here, which the old `< 2` guard let through — the pair loop
	// never ran, an empty object was still stored, and +OK was returned. Red
	// before the fix.
	out := runIntegrationCmd(t, router, s, "OBJECT.FROMENTRIES", "reg_fe_noargs", "f2")
	if !strings.HasPrefix(out, "-") {
		t.Fatalf("FAIL: OBJECT.FROMENTRIES with a name and a dangling field replied %q, want an "+
			"arity error — the pair loop never ran, so an empty object was still stored under "+
			"the name and the client was told it had succeeded", strings.TrimSpace(out))
	}

	// A trailing field with no value is equally invalid and must not be dropped.
	out = runIntegrationCmd(t, router, s, "OBJECT.FROMENTRIES", "reg_fe_odd", "f1", "v1", "f2")
	if !strings.HasPrefix(out, "-") {
		t.Fatalf("FAIL: OBJECT.FROMENTRIES with an odd trailing field replied %q, want an arity "+
			"error — f2 was silently discarded and the command reported success",
			strings.TrimSpace(out))
	}

	// Control: the well-formed call must still work and store the pair. This
	// passes before and after the fix, so a broken harness cannot masquerade as
	// the defect.
	out = runIntegrationCmd(t, router, s, "OBJECT.FROMENTRIES", "reg_fe_ok", "f1", "v1")
	if out != "+OK\r\n" {
		t.Fatalf("control failed: well-formed OBJECT.FROMENTRIES replied %q, want +OK",
			strings.TrimSpace(out))
	}
	if got := runIntegrationCmd(t, router, s, "OBJECT.GET", "reg_fe_ok", "f1"); !strings.Contains(got, "v1") {
		t.Fatalf("control failed: OBJECT.GET returned %q, want the stored value v1 — "+
			"the pair was not stored", strings.TrimSpace(got))
	}
}
