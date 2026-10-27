package command

import (
	"bytes"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// runArrayCmd executes one command through a real Router, as a client would.
// It returns the handler error rather than failing the test directly, because
// t.Fatalf must not be called from a goroutine other than the test's own.
func runArrayCmd(router *Router, args ...string) error {
	raw := make([][]byte, 0, len(args))
	for _, a := range args {
		raw = append(raw, []byte(a))
	}
	var out bytes.Buffer
	ctx := NewContext(args[0], raw[1:], store.NewStore(), resp.NewWriter(&out))
	return router.Execute(ctx)
}

// The ARRAY readers copied the slice header under arraysMu.RLock and then
// RELEASED the lock before touching the elements, while ARRAY.REVERSE and
// ARRAY.SORT permute those same elements in place under the write lock. A Go
// slice header is a value, so the copy captured (ptr, len, cap) safely -- but
// the elements themselves live in a backing array the lock does not protect
// once it is released. Concurrent clients therefore raced on string elements.
//
// This test is only meaningful under -race, which is the project's CI gate
// (`go test -race ./internal/... -count=1`). Without the race detector the
// interleaving is invisible and the test passes either way.
func TestArrayReadersDoNotRaceWithInPlaceWriters(t *testing.T) {
	const arrayName = "race-regression-array"
	const otherName = "race-regression-other"

	router := NewRouter()
	RegisterIntegrationCommands(router)

	// Clean up the package-global ARRAY state this test touches, so it cannot
	// leak into or depend on other tests in this package.
	t.Cleanup(func() {
		arraysMu.Lock()
		delete(arrays, arrayName)
		delete(arrays, otherName)
		arraysMu.Unlock()
	})

	for _, args := range [][]string{
		{"ARRAY.PUSH", arrayName, "a0", "a1", "a2", "a3", "a4", "a5", "a6", "a7"},
		{"ARRAY.PUSH", otherName, "a1", "a3", "a5"},
	} {
		if err := runArrayCmd(router, args...); err != nil {
			t.Fatalf("%v errored: %v", args, err)
		}
	}

	readers := [][]string{
		{"ARRAY.INDEXOF", arrayName, "a4"},
		{"ARRAY.LASTINDEXOF", arrayName, "a4"},
		{"ARRAY.INCLUDES", arrayName, "a4"},
		{"ARRAY.SLICE", arrayName, "1", "5"},
		{"ARRAY.INTERSECT", arrayName, otherName},
		{"ARRAY.DIFF", arrayName, otherName},
	}
	writers := [][]string{
		{"ARRAY.REVERSE", arrayName},
		{"ARRAY.SORT", arrayName},
	}

	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 3000; i++ {
				for _, cmd := range readers {
					// Errors are ignored here on purpose: t.Fatalf is not legal
					// off the test goroutine. The seeding above already asserts
					// the commands are valid, and this loop only races them.
					_ = runArrayCmd(router, cmd...)
				}
			}
		}()
	}
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 3000; i++ {
				for _, cmd := range writers {
					_ = runArrayCmd(router, cmd...)
				}
			}
		}()
	}
	wg.Wait()

	// The array must still hold every element: an in-place writer racing an
	// unlocked reader could otherwise drop or duplicate entries.
	arraysMu.RLock()
	got := len(arrays[arrayName])
	arraysMu.RUnlock()
	if got != 8 {
		t.Errorf("array length = %d after concurrent access, want 8", got)
	}
}
