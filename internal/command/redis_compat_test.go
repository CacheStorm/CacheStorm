package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func newCompatRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterListCommands(router)
	RegisterBitmapCommands(router)
	return router
}

// Redis: LPOS key element COUNT 0 returns ALL matches (0 = unlimited),
// as an array. CacheStorm conflated COUNT 0 with the no-COUNT default and
// returned a single integer.
func TestLPosCountZeroReturnsAllMatches(t *testing.T) {
	s := store.NewStore()
	r := newCompatRouter(s)

	runCmd(t, s, r, "RPUSH", "k", "a", "x", "a", "x", "a")
	reply := runCmd(t, s, r, "LPOS", "k", "a", "COUNT", "0")
	for _, want := range []string{":0", ":2", ":4"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("LPOS COUNT 0 must return all matches [0,2,4], got %q", reply)
		}
	}
}

// Redis: LPOS ... RANK 0 is rejected with an error. CacheStorm accepted it
// silently and returned null (the scan can never match rank 0).
func TestLPosRankZeroRejected(t *testing.T) {
	s := store.NewStore()
	r := newCompatRouter(s)

	runCmd(t, s, r, "RPUSH", "k", "a", "b", "c")
	reply := runCmd(t, s, r, "LPOS", "k", "a", "RANK", "0")
	if !strings.Contains(reply, "RANK should not be 0") {
		t.Fatalf("LPOS RANK 0 must be rejected with an error, got %q", reply)
	}
}

// Redis: bitmaps are strings — GET on a bitmap key returns the string
// bytes. CacheStorm rejected it with WRONGTYPE.
func TestGetReturnsStringBytesForBitmapKey(t *testing.T) {
	s := store.NewStore()
	r := newCompatRouter(s)

	runCmd(t, s, r, "SET", "k", "hello")
	runCmd(t, s, r, "SETBIT", "k", "100", "1")
	reply := runCmd(t, s, r, "GET", "k")
	if !strings.Contains(reply, "hello") {
		t.Fatalf("GET on a bitmap key must return the string bytes, got %q", reply)
	}
	if !strings.Contains(reply, "13") {
		t.Fatalf("GET on a bitmap key must return the full bitmap allocation (13 bytes), got %q", reply)
	}
}
