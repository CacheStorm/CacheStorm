package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: getOrCreateBitmap's StringValue case returned an unsaved
// &BitmapValue{Data: v.Data} wrapper — SETBIT on a string-created key took
// the reallocation branch (offset beyond the string length), grew the
// wrapper's NEW slice, and the store still held the old StringValue: the
// bit was silently lost while SETBIT reported success.

func newBitmapRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterBitmapCommands(router)
	return router
}

func TestSetBitPersistsOnStringValueKey(t *testing.T) {
	s := store.NewStore()
	router := newBitmapRouter(s)

	if reply := runCmd(t, s, router, "SET", "k", "hello"); reply == "" {
		t.Fatal("SET setup failed")
	}

	// Offset 100 is beyond the string length: the old bit is 0, and the bit
	// must survive the SETBIT->GETBIT round trip.
	if reply := runCmd(t, s, router, "SETBIT", "k", "100", "1"); !strings.Contains(reply, ":0") {
		t.Fatalf("SETBIT must report the old bit 0, got %q", reply)
	}
	if got := runCmd(t, s, router, "GETBIT", "k", "100"); !strings.Contains(got, ":1") {
		t.Fatalf("SETBIT reported success but the bit was lost, GETBIT got %q", got)
	}
}

func TestSetBitFreshKeyRoundTrip(t *testing.T) {
	s := store.NewStore()
	router := newBitmapRouter(s)

	if reply := runCmd(t, s, router, "SETBIT", "fresh", "9", "1"); !strings.Contains(reply, ":0") {
		t.Fatalf("SETBIT on a fresh key must report the old bit 0, got %q", reply)
	}
	if got := runCmd(t, s, router, "GETBIT", "fresh", "9"); !strings.Contains(got, ":1") {
		t.Fatalf("bit lost on a fresh key, GETBIT got %q", got)
	}
}

// Sanity: an in-place SETBIT (offset within the existing bytes) must read
// back as 1 both before and after the conversion fix.
func TestSetBitInPlaceRoundTrip(t *testing.T) {
	s := store.NewStore()
	router := newBitmapRouter(s)

	if reply := runCmd(t, s, router, "SET", "k", "hello"); reply == "" {
		t.Fatal("SET setup failed")
	}
	if reply := runCmd(t, s, router, "SETBIT", "k", "4", "1"); !strings.Contains(reply, ":0") {
		t.Fatalf("SETBIT must report the old bit 0, got %q", reply)
	}
	if got := runCmd(t, s, router, "GETBIT", "k", "4"); !strings.Contains(got, ":1") {
		t.Fatalf("in-place bit lost, GETBIT got %q", got)
	}
}
