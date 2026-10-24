package command

// Regression coverage for duplicate command registration.
//
// Router.Register is last-write-wins: a second Register for the same name
// replaced the first handler with no diagnostic. Six names were registered in two
// files at once, and for REPLICAOF/SLAVEOF the two handlers were NOT equivalent
// — the stub cmdSLAVEOF discards both arguments and replies +OK, so which one
// answered depended entirely on the order of the Register*Commands calls in
// internal/server/server.go. Reorder two lines there and both commands would
// have silently stopped configuring replication.
//
// Register now REFUSES a duplicate: the first handler is kept and the later one
// is discarded. That removes the order-dependence entirely — a second
// registration can no longer shadow the first, whatever order the
// Register*Commands calls run in. The refusal is still recorded (see
// DuplicateRegistrations) and logged, so a collision is reported rather than
// silently dropped.
//
// This file pins four things: a duplicate IS recorded, a distinct name is NOT,
// the FIRST handler survives a refusal, and the real production wiring registers
// every family with ZERO duplicates.

import (
	"fmt"
	"reflect"
	"testing"
)

// registerEveryFamily mirrors the production wiring in
// internal/server/server.go:151-213. If a new Register*Commands function is
// added to the server, add it here too — this is the list that must stay
// collision-free.
func registerEveryFamily(router *Router) {
	RegisterStringCommands(router)
	RegisterServerCommands(router)
	RegisterKeyCommands(router)
	RegisterHashCommands(router)
	RegisterListCommands(router)
	RegisterSetCommands(router)
	RegisterTagCommands(router)
	RegisterNamespaceCommands(router)
	RegisterClusterCommands(router)
	RegisterClientCommands(router)
	RegisterConfigCommands(router)
	RegisterTransactionCommands(router)
	RegisterSortedSetCommands(router)
	RegisterPubSubCommands(router)
	RegisterBitmapCommands(router)
	RegisterHyperLogLogCommands(router)
	RegisterStreamCommands(router)
	RegisterGeoCommands(router)
	RegisterScriptCommands(router)
	RegisterDebugCommands(router)
	RegisterCacheCommands(router)
	RegisterFunctionCommands(router)
	RegisterModuleCommands(router)
	RegisterSentinelCommands(router)
	RegisterJSONCommands(router)
	RegisterTSCommands(router)
	RegisterSearchCommands(router)
	RegisterProbabilisticCommands(router)
	RegisterGraphCommands(router)
	RegisterDigestCommands(router)
	RegisterUtilityCommands(router)
	RegisterMonitoringCommands(router)
	RegisterCacheWarmingCommands(router)
	RegisterStatsCommands(router)
	RegisterSchedulerCommands(router)
	RegisterEventCommands(router)
	RegisterUtilityExtCommands(router)
	RegisterTemplateCommands(router)
	RegisterWorkflowCommands(router)
	RegisterDataStructuresCommands(router)
	RegisterEncodingCommands(router)
	RegisterActorCommands(router)
	RegisterMVCCCommands(router)
	RegisterIntegrationCommands(router)
	RegisterExtendedCommands(router)
	RegisterMoreCommands(router)
	RegisterExtraCommands(router)
	RegisterResilienceCommands(router)
	RegisterMLCommands(router)
}

// The headline invariant: wiring every command family must register each name
// exactly once. This is what caught ROLE, PSYNC, SYNC and MODULE being
// registered in both server_commands.go and their owning file.
func TestProductionWiringHasNoDuplicateRegistrations(t *testing.T) {
	router := NewRouter()
	registerEveryFamily(router)

	if dups := router.DuplicateRegistrations(); len(dups) != 0 {
		t.Fatalf("command names registered more than once: %v — each name must have "+
			"exactly one registration site, or which handler answers depends on "+
			"Register*Commands call order", dups)
	}
}

// A duplicate registration is recorded, so the collision is reported rather than
// silent.
func TestRegisterRecordsDuplicateName(t *testing.T) {
	router := NewRouter()
	router.Register(&CommandDef{Name: "DUP", Handler: cmdPING})
	router.Register(&CommandDef{Name: "DUP", Handler: cmdECHO})

	dups := router.DuplicateRegistrations()
	if len(dups) != 1 || dups[0] != "DUP" {
		t.Fatalf("DuplicateRegistrations() = %v, want [\"DUP\"] after registering the "+
			"same name twice", dups)
	}
}

// The control: distinct names are never recorded. Without this, the detection
// above could pass by recording everything.
func TestRegisterDoesNotRecordDistinctNames(t *testing.T) {
	router := NewRouter()
	router.Register(&CommandDef{Name: "AAA", Handler: cmdPING})
	router.Register(&CommandDef{Name: "BBB", Handler: cmdECHO})

	if dups := router.DuplicateRegistrations(); len(dups) != 0 {
		t.Fatalf("DuplicateRegistrations() = %v, want empty for distinct names", dups)
	}
}

// Refusal changes dispatch in a specific way: the FIRST handler wins and the
// later one is discarded. This pins that, because it is exactly what removes the
// order-dependence that let the cmdSLAVEOF stub shadow the functional
// cmdREPLICAOF. An earlier version of this test only asserted the name still
// resolved, which passed under either policy.
func TestDuplicateRegistrationKeepsFirstHandler(t *testing.T) {
	router := NewRouter()
	router.Register(&CommandDef{Name: "WINNER", Handler: cmdPING})
	router.Register(&CommandDef{Name: "WINNER", Handler: cmdECHO})

	def, ok := router.Get("WINNER")
	if !ok {
		t.Fatal("WINNER is not registered")
	}

	if got, want := reflect.ValueOf(def.Handler).Pointer(), reflect.ValueOf(cmdPING).Pointer(); got != want {
		t.Errorf("WINNER resolved to the discarded second handler; a refused duplicate "+
			"must leave the first one in place (got %v, want cmdPING %v)", got, want)
	}

	if dups := router.DuplicateRegistrations(); len(dups) != 1 || dups[0] != "WINNER" {
		t.Errorf("DuplicateRegistrations() = %v, want [\"WINNER\"] — a refusal must still "+
			"be reported", dups)
	}
}

// A refused duplicate must not evict the command: the name keeps resolving, so
// refusal degrades to "keep the first" rather than "drop the command".
func TestRefusedDuplicateStillResolves(t *testing.T) {
	router := NewRouter()
	router.Register(&CommandDef{Name: "KEEP", Handler: cmdPING})
	router.Register(&CommandDef{Name: "KEEP", Handler: func(ctx *Context) error {
		return ctx.WriteError(fmt.Errorf("the discarded handler must never run"))
	}})

	def, ok := router.Get("KEEP")
	if !ok {
		t.Fatal("KEEP no longer resolves after a refused duplicate")
	}
	if got, want := reflect.ValueOf(def.Handler).Pointer(), reflect.ValueOf(cmdPING).Pointer(); got != want {
		t.Errorf("KEEP resolved to the discarded handler %v, want cmdPING %v", got, want)
	}
}

// A name registered many times is recorded once, not once per collision.
func TestDuplicateRecordedOncePerName(t *testing.T) {
	router := NewRouter()
	for i := 0; i < 5; i++ {
		router.Register(&CommandDef{Name: "REPEAT", Handler: cmdPING})
	}

	if dups := router.DuplicateRegistrations(); len(dups) != 1 || dups[0] != "REPEAT" {
		t.Fatalf("DuplicateRegistrations() = %v, want exactly [\"REPEAT\"]", dups)
	}
}
