package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdLPOS applied MAXLEN by truncating the scan window from the
// head (searchLen = maxlen) even for negative-RANK tail searches — the tail
// elements were never compared, so LPOS key elem RANK -1 MAXLEN n returned
// null (or a wrong match) whenever n < length. Redis caps MAXLEN comparisons
// in the search direction: a tail search compares the LAST n elements.

func newListRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterListCommands(router)
	return router
}

func TestLPosNegativeRankWithMaxlenScansTail(t *testing.T) {
	s := store.NewStore()
	router := newListRouter(s)

	for _, elem := range []string{"a", "b", "c", "X", "X"} {
		if reply := runCmd(t, s, router, "RPUSH", "k", elem); reply == "" {
			t.Fatal("RPUSH setup failed")
		}
	}

	// X sits at indices 3 and 4; the last 3 elements are c,X,X — the first
	// match from the tail is index 4.
	reply := runCmd(t, s, router, "LPOS", "k", "X", "RANK", "-1", "MAXLEN", "3")
	if !strings.Contains(reply, ":4") {
		t.Fatalf("tail search with MAXLEN must scan the last n elements, got %q", reply)
	}
}

func TestLPosNegativeRankWithMaxlenFindsWithinTailWindow(t *testing.T) {
	s := store.NewStore()
	router := newListRouter(s)

	for _, elem := range []string{"a", "b", "c", "X", "d"} {
		if reply := runCmd(t, s, router, "RPUSH", "k", elem); reply == "" {
			t.Fatal("RPUSH setup failed")
		}
	}

	// X sits at index 3, inside the last-3 window (c,X,d); RANK -1 finds it.
	reply := runCmd(t, s, router, "LPOS", "k", "X", "RANK", "-1", "MAXLEN", "3")
	if !strings.Contains(reply, ":3") {
		t.Fatalf("tail search must find matches inside the tail window, got %q", reply)
	}
}

// Sanity: the head-direction MAXLEN and the no-MAXLEN tail search were
// already correct — they must stay that way.
func TestLPosCorrectPathsUnchanged(t *testing.T) {
	s := store.NewStore()
	router := newListRouter(s)

	for _, elem := range []string{"X", "b", "c"} {
		if reply := runCmd(t, s, router, "RPUSH", "k", elem); reply == "" {
			t.Fatal("RPUSH setup failed")
		}
	}
	if reply := runCmd(t, s, router, "LPOS", "k", "X", "MAXLEN", "1"); !strings.Contains(reply, ":0") {
		t.Fatalf("head search with MAXLEN regressed, got %q", reply)
	}
	if reply := runCmd(t, s, router, "LPOS", "k", "c", "RANK", "-1"); !strings.Contains(reply, ":2") {
		t.Fatalf("tail search without MAXLEN regressed, got %q", reply)
	}
}
