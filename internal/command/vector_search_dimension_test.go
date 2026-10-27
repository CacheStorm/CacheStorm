package command

// Regression coverage for the client-controlled allocation in cmdVECTORSEARCH.
//
// The handler took the vector dimension straight from the client and used it as
// a make([]float64, dim) length with nothing bounding it. A negative dimension
// panicked the handler with "makeslice: len out of range"; the per-connection
// recover in Connection.Handle turned that into a dropped connection with no
// reply, so any client that could run VECTOR.SEARCH could crash the handler.
//
// dim == 0 is deliberately still allowed: make([]float64, 0) is legal and
// cosineSimilarity returns 0 on a length mismatch, so zero is a weak query
// rather than a fault. Only a negative dimension is rejected.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func newVectorRouter() *Router {
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterExtendedCommands(router)
	return router
}

// VECTOR handlers share package-level maps, so isolate by unique names.
func runVector(t *testing.T, router *Router, args ...string) (string, bool) {
	t.Helper()
	raw := make([][]byte, len(args))
	for i, a := range args {
		raw[i] = []byte(a)
	}
	var out bytes.Buffer
	ctx := NewContext(args[0], raw[1:], nil, resp.NewWriter(&out))
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("%v: unexpected execute error: %v", args, err)
	}
	return out.String(), false
}

// A negative dimension must be an error reply, never a panic.
func TestVectorSearchRejectsNegativeDimension(t *testing.T) {
	router := newVectorRouter()

	for _, dim := range []string{"-1", "-2", "-768"} {
		reply, _ := runVector(t, router, "VECTOR.SEARCH", "vec:neg", "1", dim)
		if strings.HasPrefix(reply, "-") {
			continue
		}
		t.Fatalf("VECTOR.SEARCH with dim %s replied %q, want an error — a negative "+
			"dimension must be rejected before it reaches make([]float64, dim)", dim, reply)
	}
}

// A large-but-positive dimension must not panic either.
func TestVectorSearchHandlesLargeDimension(t *testing.T) {
	router := newVectorRouter()

	reply, _ := runVector(t, router, "VECTOR.SEARCH", "vec:big", "1", "1000000")
	if strings.HasPrefix(reply, "-") && strings.Contains(reply, "out of range") {
		t.Fatalf("a positive dimension was rejected as out of range: %q", reply)
	}
	// Either an error (no such store) or an empty result is fine; a panic is not.
	if !strings.HasPrefix(reply, "*") && !strings.HasPrefix(reply, "-") && !strings.HasPrefix(reply, "$") {
		t.Fatalf("unexpected reply shape: %q", reply)
	}
}

// Control: the documented happy path still works end to end.
func TestVectorSearchHappyPathUnchanged(t *testing.T) {
	router := newVectorRouter()

	if reply, _ := runVector(t, router, "VECTOR.CREATE", "vec:ok", "2"); strings.HasPrefix(reply, "-") {
		t.Fatalf("VECTOR.CREATE was rejected: %q", reply)
	}
	if reply, _ := runVector(t, router, "VECTOR.ADD", "vec:ok", "a", "2", "1", "0"); strings.HasPrefix(reply, "-") {
		t.Fatalf("VECTOR.ADD was rejected: %q", reply)
	}

	reply, _ := runVector(t, router, "VECTOR.SEARCH", "vec:ok", "1", "2", "1", "0")
	if strings.HasPrefix(reply, "-") {
		t.Fatalf("a well-formed VECTOR.SEARCH was rejected: %q", reply)
	}
	if !strings.Contains(reply, "a") {
		t.Fatalf("search did not return the added vector: %q", reply)
	}
}

// cmdVECTORADD carries the same make([]float64, dim) as cmdVECTORSEARCH and
// was left unguarded when only the latter was fixed. Both handlers must reject
// a negative dimension, so a guard on one is not evidence about the other.
func TestVectorAddRejectsNegativeDimension(t *testing.T) {
	router := newVectorRouter()

	if reply, _ := runVector(t, router, "VECTOR.CREATE", "vec:add", "2"); strings.HasPrefix(reply, "-") {
		t.Fatalf("VECTOR.CREATE was rejected: %q", reply)
	}

	for _, dim := range []string{"-1", "-2", "-768"} {
		reply, _ := runVector(t, router, "VECTOR.ADD", "vec:add", "v"+dim, dim)
		if strings.HasPrefix(reply, "-") {
			continue
		}
		t.Fatalf("VECTOR.ADD with dim %s replied %q, want an error — a negative "+
			"dimension must be rejected before it reaches make([]float64, dim)", dim, reply)
	}
}
