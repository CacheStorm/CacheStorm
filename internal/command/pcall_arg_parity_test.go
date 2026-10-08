package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Round r38 proof: redis.call and redis.pcall convert Lua arguments
// differently. call type-switches each argument (booleans become "1"/"0",
// numbers use %v), while pcall passes everything through bare L.ToString(i)
// (booleans become "true"/"false"). The same Lua value therefore produces
// different command arguments — and different stored bytes — depending only
// on which binding the author picked.

func parityEval(t *testing.T, s *store.Store, script string) string {
	t.Helper()
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterScriptCommands(router)

	savedEngine := scriptEngine
	scriptEngine = nil
	t.Cleanup(func() { scriptEngine = savedEngine })

	ctx, buf := bufCtx("EVAL", bytesArgs(script, "0"), s)
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("EVAL failed: %v", err)
	}
	return buf.String()
}

// The same Lua values passed through call and pcall must produce identical
// command arguments, so identical stored bytes.
func TestProofPcallArgConversionMatchesCall(t *testing.T) {
	s := store.NewStore()

	reply := parityEval(t, s, `
		redis.call('set',  'via-call-str',   'txt')
		redis.pcall('set', 'via-pcall-str',  'txt')
		redis.call('set',  'via-call-num',   3)
		redis.pcall('set', 'via-pcall-num',  3)
		redis.call('set',  'via-call-bool',  true)
		redis.pcall('set', 'via-pcall-bool', true)
		return 'ok'
	`)
	if !strings.Contains(reply, "ok") {
		t.Fatalf("seeding scripts failed: %q", reply)
	}

	for _, pair := range [][2]string{
		{"via-call-str", "via-pcall-str"},
		{"via-call-num", "via-pcall-num"},
		{"via-call-bool", "via-pcall-bool"},
	} {
		viaCall := parityEval(t, s, "return redis.call('get','"+pair[0]+"')")
		viaPcall := parityEval(t, s, "return redis.call('get','"+pair[1]+"')")
		if viaCall != viaPcall {
			t.Fatalf("FAIL: argument conversion diverges for %s: call stored %q, pcall stored %q",
				pair[0], viaCall, viaPcall)
		}
	}
}

// Control: pcall keeps its error-catching contract while call surfaces the
// error value directly — the fix must not blur the two bindings together.
func TestProofPcallErrorContractPreserved(t *testing.T) {
	s := store.NewStore()

	viaCall := parityEval(t, s, "return redis.call('nosuchcommand')")
	viaPcall := parityEval(t, s, "return redis.pcall('nosuchcommand')")

	if !strings.Contains(viaCall, "unknown command") {
		t.Fatalf("control failed: call must surface the unknown-command error, got %q", viaCall)
	}
	if !strings.Contains(viaPcall, "unknown command") {
		t.Fatalf("control failed: pcall must surface the unknown-command error, got %q", viaPcall)
	}
}
