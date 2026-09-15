package command

import (
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: cmdMSET applied pairs in a loop with Store.Set's error
// discarded and replied +OK unconditionally — under max_memory pressure an
// OOM mid-pair left earlier keys applied behind a success reply, and an
// invalid key was silently skipped. MSET is atomic ("all given keys are set
// at once"), so the handler pre-flights validation and capacity before
// applying anything.

func newMSETRouter(s *store.Store) *Router {
	router := NewRouter()
	RegisterStringCommands(router)
	return router
}

func TestMSETAtomicUnderOOM(t *testing.T) {
	s := store.NewStore()
	router := newMSETRouter(s)

	// ConfigureMemory installs a fresh tracker starting at zero, and
	// CanAllocate is a RATIO check (newUsage/max < emergencyPct, hardcoded
	// 0.95) — so the limit must be set before the fill and sized from a
	// calibrated probe such that the fill lands just under the 0.95 line
	// while the MSET's conservative estimate (268) pushes it over.
	s.ConfigureMemory(10<<20, store.EvictionNoEviction, 80, 90, 5)
	const probeVal = 1000
	runCmd(t, s, router, "SET", "probe", strings.Repeat("p", probeVal))
	before := s.MemUsage()
	runCmd(t, s, router, "DEL", "probe")
	after := s.MemUsage()
	e := before - after - int64(probeVal+len("probe")+16) // entry overhead
	if e < 0 {
		t.Fatalf("calibration produced a negative entry overhead: %d", e)
	}

	const fillerVal = 94000
	fillDelta := e + int64(fillerVal+len("filler")+16)
	// max = fillDelta/0.95 + 200: the fill ratio lands below 0.95 (accepted)
	// while the MSET's estimate pushes the ratio above 0.95 (rejected).
	max := int64(float64(fillDelta)/0.95) + 200
	s.ConfigureMemory(max, store.EvictionNoEviction, 80, 90, 5)

	runCmd(t, s, router, "SET", "filler", strings.Repeat("x", fillerVal))

	reply := runCmd(t, s, router, "MSET", "mk1", "mv1", "mk2", "mv2")
	if !strings.Contains(reply, "OOM") {
		t.Fatalf("MSET past the memory limit must be rejected up front, got %q", reply)
	}
	for _, k := range []string{"mk1", "mk2"} {
		if _, exists := s.Get(k); exists {
			t.Fatalf("rejected MSET must not apply %q", k)
		}
	}
}

func TestMSETRejectsInvalidKeyUpfront(t *testing.T) {
	s := store.NewStore()
	router := newMSETRouter(s)

	// The empty key is invalid: the whole MSET must be rejected before any
	// pair is applied (pre-fix: the empty key was silently skipped and
	// goodkey was applied behind +OK).
	reply := runCmd(t, s, router, "MSET", "", "v", "goodkey", "v2")
	if !strings.Contains(reply, "invalid key") {
		t.Fatalf("MSET with an invalid key must be rejected up front, got %q", reply)
	}
	if _, exists := s.Get("goodkey"); exists {
		t.Fatal("rejected MSET must not apply the remaining pairs")
	}
}

func TestMSETNormalOperation(t *testing.T) {
	s := store.NewStore()
	router := newMSETRouter(s)

	reply := runCmd(t, s, router, "MSET", "a", "1", "b", "2")
	if !strings.Contains(reply, "+OK") {
		t.Fatalf("normal MSET must succeed, got %q", reply)
	}
	for k, want := range map[string]string{"a": "1", "b": "2"} {
		if got := storedStr(t, s, k); got != want {
			t.Fatalf("key %q = %q, want %q", k, got, want)
		}
	}
}
