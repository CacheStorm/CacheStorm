package command

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Regression: SORT read ListValue.Elements, SetValue.Members and (via BY
// patterns) HashValue.Fields with NO lock, while the command handlers mutate
// those same maps/slices under their write lock:
//
//	doSort          ranged v.Elements and v.Members unlocked
//	sortLookupValue read  hv.Fields[field] unlocked
//
// cmdRPUSH/cmdLPOP mutate ListValue.Elements, cmdSADD/cmdSREM mutate
// SetValue.Members and cmdHSET mutates HashValue.Fields, all under the value
// write lock. A concurrent map iteration and map write is a PROCESS-FATAL
// runtime error, and a slice read racing a slice append can return torn data.
//
// doSort obtains the LIVE value pointer from ctx.Store.Get, so it observes the
// very map a concurrent writer holds locked.

// sortRaceRouter registers every command these tests need. RPUSH/LPOP come
// from RegisterListCommands, NOT RegisterKeyCommands — omitting it leaves the
// list unseeded and every race assertion below vacuously passes.
func sortRaceRouter() *Router {
	router := NewRouter()
	RegisterServerCommands(router)
	RegisterKeyCommands(router)
	RegisterStringCommands(router)
	RegisterHashCommands(router)
	RegisterListCommands(router)
	RegisterSetCommands(router)
	return router
}

func sortRaceRun(t *testing.T, s *store.Store, router *Router, cmd string, args ...string) string {
	t.Helper()
	parsed := make([][]byte, len(args))
	for i, a := range args {
		parsed[i] = []byte(a)
	}
	buf := &bytes.Buffer{}
	if err := router.Execute(NewContext(cmd, parsed, s, resp.NewWriter(buf))); err != nil {
		t.Fatalf("%s: Execute returned error: %v", cmd, err)
	}
	return buf.String()
}

func sortRaceSeedList(t *testing.T, s *store.Store, router *Router, key string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if out := sortRaceRun(t, s, router, "RPUSH", key, fmt.Sprintf("e%04d", i)); out == "" {
			t.Fatalf("RPUSH seed failed for %s", key)
		}
	}
}

// TestSortListIsRaceFreeWithListWriters is the arm that reproduced: SORT's copy
// of ListValue.Elements against cmdRPUSH/cmdLPOP.
func TestSortListIsRaceFreeWithListWriters(t *testing.T) {
	s := store.NewStore()
	router := sortRaceRouter()
	sortRaceSeedList(t, s, router, "l", 256)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			sortRaceRun(t, s, router, "RPUSH", "l", fmt.Sprintf("w%04d", i%64))
			sortRaceRun(t, s, router, "LPOP", "l")
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			sortRaceRun(t, s, router, "SORT", "l", "ALPHA")
		}
	}()

	wg.Wait()
}

// TestSortSetIsRaceFreeWithSetWriters covers the SetValue.Members arm.
func TestSortSetIsRaceFreeWithSetWriters(t *testing.T) {
	s := store.NewStore()
	router := sortRaceRouter()
	for i := 0; i < 256; i++ {
		sortRaceRun(t, s, router, "SADD", "st", fmt.Sprintf("m%04d", i))
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			sortRaceRun(t, s, router, "SADD", "st", fmt.Sprintf("w%04d", i%64))
			sortRaceRun(t, s, router, "SREM", "st", fmt.Sprintf("m%04d", i%128))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			sortRaceRun(t, s, router, "SORT", "st", "ALPHA")
		}
	}()

	wg.Wait()
}

// TestSortByPatternIsRaceFreeWithHashWriters covers sortLookupValue's
// HashValue.Fields read.
func TestSortByPatternIsRaceFreeWithHashWriters(t *testing.T) {
	s := store.NewStore()
	router := sortRaceRouter()
	sortRaceSeedList(t, s, router, "l", 64)
	for i := 0; i < 256; i++ {
		sortRaceRun(t, s, router, "HSET", "h", fmt.Sprintf("w%04d", i), "1")
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			sortRaceRun(t, s, router, "HSET", "h", fmt.Sprintf("w%04d", i%256), "2")
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			sortRaceRun(t, s, router, "SORT", "l", "BY", "weight_*->w")
		}
	}()

	wg.Wait()
}

// TestSortControlsUncontended is the control: against collections nobody
// mutates, SORT must still order numerically, honour DESC/LIMIT, and project
// BY/GET results. This proves the harness reaches real SORT behaviour, so a
// race failure above is the defect and not a broken setup.
func TestSortControlsUncontended(t *testing.T) {
	s := store.NewStore()
	router := sortRaceRouter()

	sortRaceRun(t, s, router, "RPUSH", "l", "3", "1", "2")
	if got := sortRaceRun(t, s, router, "SORT", "l"); got != "*3\r\n$1\r\n1\r\n$1\r\n2\r\n$1\r\n3\r\n" {
		t.Fatalf("CONTROL FAIL: numeric SORT = %q, want 1 2 3", got)
	}
	if got := sortRaceRun(t, s, router, "SORT", "l", "DESC"); got != "*3\r\n$1\r\n3\r\n$1\r\n2\r\n$1\r\n1\r\n" {
		t.Fatalf("CONTROL FAIL: SORT DESC = %q, want 3 2 1", got)
	}
	if got := sortRaceRun(t, s, router, "SORT", "l", "LIMIT", "1", "1"); got != "*1\r\n$1\r\n2\r\n" {
		t.Fatalf("CONTROL FAIL: SORT LIMIT 1 1 = %q, want just 2", got)
	}

	// BY w* orders by the external weight and GET projects two values per
	// element, so 3 elements yield 6 projected values.
	sortRaceRun(t, s, router, "HSET", "wa", "3", "b", "1")
	sortRaceRun(t, s, router, "HSET", "wb", "1", "a", "2")
	proj := sortRaceRun(t, s, router, "SORT", "l", "BY", "w*", "GET", "#", "GET", "w*")
	pv, err := resp.NewReader(bytes.NewReader([]byte(proj))).ReadValue()
	if err != nil {
		t.Fatalf("CONTROL FAIL: unparsable SORT BY/GET reply %q: %v", proj, err)
	}
	if pv.Type != resp.TypeArray || len(pv.Array) != 6 {
		t.Fatalf("CONTROL FAIL: SORT BY/GET returned %d projected values, want 6 (raw %q)", len(pv.Array), proj)
	}
	// "w*" has no "->", so each element expands to the plain STRING key
	// w3/w1/w2 — which do not exist. Every lookup misses, every weight is 0,
	// and Redis breaks the ties by the elements themselves: 1, 2, 3.
	if got := string(pv.Array[0].Bulk); got != "1" {
		t.Fatalf("CONTROL FAIL: first BY-sorted element = %q, want 1 (all weights 0, ties by element)", got)
	}
}
