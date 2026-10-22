package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// runIntegrationCmd drives a command through the router and returns the raw
// RESP reply, so a test can assert on the real wire result rather than on the
// handler's error return (which is nil even when the reply is an error).
func runIntegrationCmd(t *testing.T, router *Router, s *store.Store, cmd string, args ...string) string {
	t.Helper()
	argb := make([][]byte, len(args))
	for i, a := range args {
		argb[i] = []byte(a)
	}
	var b bytes.Buffer
	ctx := NewContext(cmd, argb, s, resp.NewWriter(&b))
	def, ok := router.Get(cmd)
	if !ok {
		t.Fatalf("%s is not registered", cmd)
	}
	if err := def.Handler(ctx); err != nil {
		t.Fatalf("%s returned a transport error: %v", cmd, err)
	}
	return b.String()
}

// ARRAY.MERGE and OBJECT.MERGE read only ArgString(0) and ArgString(1), but
// their handlers used to guard with ArgCount() < 3. That made the merge
// unreachable through its own two-key call: `ARRAY.MERGE dest src` returned
// "wrong number of arguments" even though every two-key sibling
// (ARRAY.INCLUDES, OBJECT.GET, ...) accepts two arguments.
func TestRegressionArrayMergeAcceptsTwoArguments(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	runIntegrationCmd(t, router, s, "ARRAY.PUSH", "reg_arr_dest", "a", "b")
	runIntegrationCmd(t, router, s, "ARRAY.PUSH", "reg_arr_src", "c")

	out := runIntegrationCmd(t, router, s, "ARRAY.MERGE", "reg_arr_dest", "reg_arr_src")
	if strings.HasPrefix(out, "-") {
		t.Fatalf("ARRAY.MERGE with the two arguments it consumes returned an error: %q", out)
	}

	// The destination must now actually contain the source elements.
	// ARRAY.SLICE treats the end index literally and clamps it to len(arr),
	// so a large sentinel reads the whole array.
	merged := runIntegrationCmd(t, router, s, "ARRAY.SLICE", "reg_arr_dest", "0", "100")
	for _, want := range []string{"a", "b", "c"} {
		if !strings.Contains(merged, want) {
			t.Errorf("merged array is missing %q; got %q", want, merged)
		}
	}
}

func TestRegressionObjectMergeAcceptsTwoArguments(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	runIntegrationCmd(t, router, s, "OBJECT.SET", "reg_obj_dest", "k1", "v1")
	runIntegrationCmd(t, router, s, "OBJECT.SET", "reg_obj_src", "k2", "v2")

	out := runIntegrationCmd(t, router, s, "OBJECT.MERGE", "reg_obj_dest", "reg_obj_src")
	if strings.HasPrefix(out, "-") {
		t.Fatalf("OBJECT.MERGE with the two arguments it consumes returned an error: %q", out)
	}

	// Both the destination's own pair and the merged-in pair must be readable.
	if got := runIntegrationCmd(t, router, s, "OBJECT.GET", "reg_obj_dest", "k1"); strings.Contains(got, "-") {
		t.Errorf("merged object lost its original field k1; got %q", got)
	}
	if got := runIntegrationCmd(t, router, s, "OBJECT.GET", "reg_obj_dest", "k2"); strings.Contains(got, "-") {
		t.Errorf("merged object is missing the source field k2; got %q", got)
	}
}

// The boundary the fix must not over-relax: one argument is still not enough,
// because the handler cannot merge a single key.
func TestRegressionMergeStillRequiresTwoArguments(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	if out := runIntegrationCmd(t, router, s, "ARRAY.MERGE", "reg_only_one"); !strings.HasPrefix(out, "-") {
		t.Errorf("ARRAY.MERGE with 1 argument should be rejected, got %q", out)
	}
	if out := runIntegrationCmd(t, router, s, "OBJECT.MERGE", "reg_only_one"); !strings.HasPrefix(out, "-") {
		t.Errorf("OBJECT.MERGE with 1 argument should be rejected, got %q", out)
	}
}
