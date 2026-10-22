package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// CONFIG.IMPORT consumes a namespace followed by key/value PAIRS
// (`for i := 1; i+1 < ctx.ArgCount(); i += 2`). A prior round raised the
// minimum to `< 3` so a namespace could no longer be created empty, but the
// PARITY of the pair list was never checked.
//
// So `CONFIG.IMPORT ns k1 v1 k2` stored k1=v1, silently DISCARDED k2, and
// answered :1 as if the whole call had been applied. The caller is told N
// settings were imported while one of its settings never existed.
//
// This is the same class as round r87 (cmdOBJECTFROMENTRIES) and r88
// (cmdDOCINSERT/cmdDOCUPDATE): a command must not report success for arguments
// it could not apply.
func TestRegressionConfigImportRejectsOddPairList(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterActorCommands(router)

	// Control: a well-formed import must still succeed and be readable back.
	if out := runCmd(t, s, router, "CONFIG.IMPORT", "reg_cfg_ok", "a", "1"); out != ":1\r\n" {
		t.Fatalf("control failed: CONFIG.IMPORT of one pair returned %q, want :1",
			strings.TrimSpace(out))
	}
	if got := runCmd(t, s, router, "CONFIG.EXPORT", "reg_cfg_ok"); !strings.Contains(got, "a") {
		t.Fatalf("control failed: CONFIG.EXPORT returned %q, want the imported key a",
			strings.TrimSpace(got))
	}

	// A trailing key with no value is invalid. Asserting the REPLY matters: the
	// unfixed handler simply dropped k2, so inspecting the stored namespace
	// could never observe the loss and the test would pass vacuously.
	out := runCmd(t, s, router, "CONFIG.IMPORT", "reg_cfg_odd", "a", "1", "b")
	if !strings.HasPrefix(out, "-") {
		t.Fatalf("FAIL: CONFIG.IMPORT with an odd trailing key replied %q, want an arity error — "+
			"k2 had no value, was silently discarded, and the command reported success",
			strings.TrimSpace(out))
	}
}
