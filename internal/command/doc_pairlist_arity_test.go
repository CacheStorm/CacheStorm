package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// DOC.INSERT and DOC.UPDATE take a store name, a document id, and then a list
// of field/value PAIRS consumed by `for i := 2; i+1 < ctx.ArgCount(); i += 2`.
// Neither handler validated that list.
//
// Consequences, all reporting success:
//   - `DOC.INSERT s d` stores an EMPTY phantom document and returns its id.
//   - `DOC.INSERT s d f1 v1 f2` stores f1=v1 and SILENTLY DROPS f2.
//   - `DOC.UPDATE s d f2` applies nothing at all and still replies +OK.
//
// This is the mirror of round r77: there a guard's minimum EXCEEDED what the
// handler read (an outage); here it FALLS SHORT of what the pair loop consumes
// (a fabricated success). A command must not report success for arguments it
// could not apply.
func TestRegressionDocPairListRejectsBadArity(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterExtendedCommands(router)

	// Control: the well-formed call must still succeed and be findable. This
	// passes before and after the fix, so a broken harness cannot masquerade as
	// the defect.
	out := runCmd(t, s, router, "DOC.INSERT", "reg_docs_ok", "d1", "f1", "v1")
	if strings.HasPrefix(out, "-") {
		t.Fatalf("control failed: well-formed DOC.INSERT replied %q, want an id",
			strings.TrimSpace(out))
	}
	if found := runCmd(t, s, router, "DOC.FIND", "reg_docs_ok", "f1", "v1"); !strings.Contains(found, "d1") {
		t.Fatalf("control failed: DOC.FIND did not return d1, got %q", strings.TrimSpace(found))
	}

	// A document with no field/value pair is not a document: refuse it rather
	// than storing an empty phantom and reporting success. Red before the fix,
	// which replied with the new id.
	if out := runCmd(t, s, router, "DOC.INSERT", "reg_docs_empty", "d1"); !strings.HasPrefix(out, "-") {
		t.Fatalf("FAIL: DOC.INSERT with no field/value pair replied %q, want an arity error — "+
			"it stored an empty phantom document that DOC.FIND then returned",
			strings.TrimSpace(out))
	}

	// A trailing field with no value is invalid. Asserting the reply directly
	// matters: the unfixed handler stored f1=v1 and DROPPED f2, so a check of
	// the stored document could never observe the loss and would pass vacuously.
	if out := runCmd(t, s, router, "DOC.INSERT", "reg_docs_odd", "d1", "f1", "v1", "f2"); !strings.HasPrefix(out, "-") {
		t.Fatalf("FAIL: DOC.INSERT with an odd trailing field replied %q, want an arity error — "+
			"f2 had no value, was silently discarded, and the command reported success",
			strings.TrimSpace(out))
	}

	// DOC.UPDATE must not claim to have updated anything it could not parse.
	out = runCmd(t, s, router, "DOC.UPDATE", "reg_docs_ok", "d1", "dangling")
	if !strings.HasPrefix(out, "-") {
		t.Fatalf("FAIL: DOC.UPDATE with a dangling field replied %q, want an arity error — it "+
			"applied nothing while reporting success", strings.TrimSpace(out))
	}
}
