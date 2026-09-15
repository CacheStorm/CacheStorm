package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: StreamValue.Add performed no ID validation — an explicit ID
// equal to or smaller than the stream's LastID was appended (regressing
// LastID and corrupting the ordering invariant XRANGE depends on), the
// invalid 0-0 ID was accepted, and malformed IDs ("notanid", bare "123")
// were stored verbatim. Redis rejects each with a distinct error and
// leaves the stream untouched.

func newStreamRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterStreamCommands(router)
	return router
}

func TestXAddRejectsEqualOrSmallerID(t *testing.T) {
	s := store.NewStore()
	router := newStreamRouter(s)

	if reply := runCmd(t, s, router, "XADD", "k", "100-1", "f", "v"); !strings.Contains(reply, "100-1") {
		t.Fatalf("XADD setup failed, got %q", reply)
	}

	reply := runCmd(t, s, router, "XADD", "k", "50-1", "f", "v")
	if !strings.Contains(reply, "equal or smaller") {
		t.Fatalf("XADD with a smaller ID must be rejected, got %q", reply)
	}
	if got := runCmd(t, s, router, "XLEN", "k"); !strings.Contains(got, ":1") {
		t.Fatalf("rejected XADD must not grow the stream, XLEN got %q", got)
	}
}

func TestXAddRejectsZeroID(t *testing.T) {
	s := store.NewStore()
	router := newStreamRouter(s)

	reply := runCmd(t, s, router, "XADD", "k", "0-0", "f", "v")
	if !strings.Contains(reply, "greater than 0-0") {
		t.Fatalf("XADD with the 0-0 ID must be rejected, got %q", reply)
	}
	if got := runCmd(t, s, router, "XLEN", "k"); !strings.Contains(got, ":0") {
		t.Fatalf("rejected XADD must not add entries, XLEN got %q", got)
	}
}

func TestXAddRejectsMalformedID(t *testing.T) {
	s := store.NewStore()
	router := newStreamRouter(s)

	for _, bad := range []string{"notanid", "123", "1-2-3"} {
		reply := runCmd(t, s, router, "XADD", "k", bad, "f", "v")
		if !strings.Contains(reply, "Invalid stream ID") {
			t.Fatalf("XADD with malformed ID %q must be rejected, got %q", bad, reply)
		}
	}
}

// Sanity: the auto-ID flow is unaffected by the validation.
func TestXAddAutoIDStillWorks(t *testing.T) {
	s := store.NewStore()
	router := newStreamRouter(s)

	reply := runCmd(t, s, router, "XADD", "k", "*", "f", "v")
	if !strings.Contains(reply, "-0") {
		t.Fatalf("auto-ID XADD on a fresh stream must yield ms-0, got %q", reply)
	}
	if got := runCmd(t, s, router, "XLEN", "k"); !strings.Contains(got, ":1") {
		t.Fatalf("stream did not grow, XLEN got %q", got)
	}
}
