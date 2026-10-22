package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// MEMORY USAGE accepts a trailing `SAMPLES n` option
// (`MEMORY USAGE key [SAMPLES count]`, docs/commands.md). The handler's option
// loop was
//
//	if arg == "SAMPLES" && i+1 < ctx.ArgCount() {
//
// so a `SAMPLES` with no value failed the whole condition and fell through
// SILENTLY -- no error, no else. This is exactly the shape fixed in round 12
// for cmdXRANGE's COUNT, and a sweep of every command handler found this as the
// last remaining instance of it.
//
// It is worth fixing even though `samples` is currently discarded downstream
// (`_ = samples`): the option is part of the documented signature, so a client
// that sends a valueless SAMPLES gets a success and a byte count while
// believing the option was understood. Accepting an option and then ignoring it
// silently is the same read-and-discard defect the ACL category work fixed in
// round 8.
//
// Note the wire form: MEMORY is a dispatcher whose subcommand is ArgString(0),
// so the key lands at ArgString(1) -- there is no "MEMORY.USAGE" command.
func TestRegressionMemoryUsageRejectsDanglingSamples(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDebugCommands(router)
	RegisterStringCommands(router)

	if got := runCmd(t, s, router, "SET", "reg_mu", "value"); strings.HasPrefix(got, "-") {
		t.Fatalf("setup failed: SET replied %q", strings.TrimSpace(got))
	}

	// Control: MEMORY USAGE without any option must still answer with the byte
	// count. Asserted as a non-empty INTEGER reply so it cannot pass vacuously
	// on an empty buffer, which is what a wrong command name would produce.
	out := runCmd(t, s, router, "MEMORY", "USAGE", "reg_mu")
	if out == "" || strings.HasPrefix(out, "-") || !strings.HasPrefix(out, ":") {
		t.Fatalf("control failed: MEMORY USAGE returned %q, want an integer byte count",
			strings.TrimSpace(out))
	}

	// A SAMPLES with no value is a syntax error, not a silent drop.
	out = runCmd(t, s, router, "MEMORY", "USAGE", "reg_mu", "SAMPLES")
	if !strings.HasPrefix(out, "-") {
		t.Fatalf("FAIL: MEMORY USAGE with a dangling SAMPLES returned %q, want a syntax error — "+
			"the option was silently discarded and the command reported success",
			strings.TrimSpace(out))
	}
}
