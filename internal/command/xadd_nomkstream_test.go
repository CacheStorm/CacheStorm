package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdXADD parsed the NOMKSTREAM option and discarded it — the
// flag was consumed but never consulted, so XADD key NOMKSTREAM * f v on a
// missing key created the stream and returned an ID. Redis returns null and
// creates nothing; on an existing key NOMKSTREAM behaves like plain XADD.

func newNomkstreamRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterStreamCommands(router)
	return router
}

func TestXAddNomkstreamDoesNotCreateMissingStream(t *testing.T) {
	s := store.NewStore()
	router := newNomkstreamRouter(s)

	reply := runCmd(t, s, router, "XADD", "k", "NOMKSTREAM", "*", "f", "v")
	if !strings.Contains(reply, "_") {
		t.Fatalf("NOMKSTREAM on a missing key must return null, got %q", reply)
	}
	if got := runCmd(t, s, router, "XLEN", "k"); strings.Contains(got, ":1") {
		t.Fatalf("NOMKSTREAM must not create the stream key, XLEN got %q", got)
	}
}

func TestXAddNomkstreamAddsToExistingStream(t *testing.T) {
	s := store.NewStore()
	router := newNomkstreamRouter(s)

	if reply := runCmd(t, s, router, "XADD", "k", "*", "f", "v"); !strings.Contains(reply, "-") {
		t.Fatalf("XADD setup failed, got %q", reply)
	}

	reply := runCmd(t, s, router, "XADD", "k", "NOMKSTREAM", "*", "f2", "v2")
	if !strings.Contains(reply, "-") {
		t.Fatalf("NOMKSTREAM on an existing stream must add normally, got %q", reply)
	}
	if got := runCmd(t, s, router, "XLEN", "k"); !strings.Contains(got, ":2") {
		t.Fatalf("stream did not grow, XLEN got %q", got)
	}
}

// Sanity: plain XADD on a missing key (no NOMKSTREAM) still creates.
func TestXAddDefaultStillCreates(t *testing.T) {
	s := store.NewStore()
	router := newNomkstreamRouter(s)

	reply := runCmd(t, s, router, "XADD", "k", "*", "f", "v")
	if !strings.Contains(reply, "-") {
		t.Fatalf("default XADD must create the stream, got %q", reply)
	}
	if got := runCmd(t, s, router, "XLEN", "k"); !strings.Contains(got, ":1") {
		t.Fatalf("default XADD did not create the key, XLEN got %q", got)
	}
}
