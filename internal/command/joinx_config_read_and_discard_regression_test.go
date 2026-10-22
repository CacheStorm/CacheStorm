package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// JOINX.CREATE and CONFIG.IMPORT both accepted arguments they could not act
// on while replying +OK: JOINX.CREATE silently dropped its type/condition
// arguments (the JoinX struct stores only Name/Left/Right, so they were never
// storable), and CONFIG.IMPORT's guard let `CONFIG.IMPORT ns key` through even
// though its loop consumes key/value pairs from index 1 and therefore imported
// nothing.
//
// These tests assert on the RESP buffer, not the returned error: ctx.WriteError
// writes the error to the writer and returns nil, so an error return assertion
// cannot tell an error reply from +OK.

// CONTROL: the supported single-argument form must still succeed, so a broken
// harness cannot masquerade as the defect.
func TestRegressionJoinXCreateAcceptsNameOnly(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	out := runIntegrationCmd(t, router, s, "JOINX.CREATE", "reg_join_ok")
	if strings.HasPrefix(out, "-") {
		t.Errorf("CONTROL: JOINX.CREATE with a name only should succeed, got %q", out)
	}
}

// JOINX.CREATE must reject type/condition arguments rather than accept and
// silently discard them.
func TestRegressionJoinXCreateRejectsUnsupportedArguments(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	cases := []struct {
		name string
		args []string
	}{
		{"type argument", []string{"reg_join_t", "inner"}},
		{"type and condition", []string{"reg_join_tc", "inner", "CONDITION", "a=b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := runIntegrationCmd(t, router, s, "JOINX.CREATE", tc.args...)
			if !strings.HasPrefix(out, "-") {
				t.Errorf("JOINX.CREATE with a %s should be rejected, got %q", tc.name, out)
			}
		})
	}

	// The rejected calls must not have created a join either. JOINX.GET
	// answers an unknown join with an empty array (*0).
	if out := runIntegrationCmd(t, router, s, "JOINX.GET", "reg_join_t"); !strings.HasPrefix(out, "*0") {
		t.Errorf("rejected JOINX.CREATE must not create a join, GET returned %q", out)
	}
}

// CONTROL: a complete key/value pair must still import, and the value must be
// readable afterwards — the guard must not be over-tightened.
func TestRegressionConfigImportAcceptsCompletePair(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterActorCommands(router)

	out := runIntegrationCmd(t, router, s, "CONFIG.IMPORT", "reg_ns_ok", "k1", "v1")
	if strings.HasPrefix(out, "-") {
		t.Fatalf("CONTROL: CONFIG.IMPORT with a complete pair should succeed, got %q", out)
	}
	if got := runIntegrationCmd(t, router, s, "CONFIG.GET", "reg_ns_ok", "k1"); strings.Contains(got, "v1") == false {
		t.Errorf("CONTROL: imported value not readable via CONFIG.GET, got %q", got)
	}
}

// CONFIG.IMPORT must require a complete key/value pair: an incomplete pair
// previously passed the guard, created an empty namespace, and reported 0.
func TestRegressionConfigImportRequiresCompletePair(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterActorCommands(router)

	cases := []struct {
		name string
		args []string
	}{
		{"namespace only", []string{"reg_ns_only"}},
		{"namespace and key without value", []string{"reg_ns_key", "k1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := runIntegrationCmd(t, router, s, "CONFIG.IMPORT", tc.args...)
			if !strings.HasPrefix(out, "-") {
				t.Errorf("CONFIG.IMPORT with a %s should be rejected, got %q", tc.name, out)
			}
		})
	}

	// A rejected import must not have left an empty namespace behind. CONFIG.GET
	// answers an unknown namespace with a RESP null (_).
	if out := runIntegrationCmd(t, router, s, "CONFIG.GET", "reg_ns_only", "k1"); !strings.HasPrefix(out, "_") {
		t.Errorf("rejected CONFIG.IMPORT must not create a namespace, GET returned %q", out)
	}
}
