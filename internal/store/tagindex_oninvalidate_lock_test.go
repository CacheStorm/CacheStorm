package store

import (
	"sync"
	"testing"
)

// Prove or dismiss: TagIndex.onInvalidate is a multi-word func value written and
// read with no synchronisation.
//
//	tag_index.go:133  func (ti *TagIndex) SetOnInvalidate(fn ...) { ti.onInvalidate = fn }
//	tag_index.go:139  func (ti *TagIndex) Invalidate(tag string) {
//	                     keys := ti.invalidate(tag)
//	                     if ti.onInvalidate != nil { ti.onInvalidate(tag, keys) }
//
// A func value is a two-word pair (code pointer + closure pointer), so an
// unsynchronised read can observe a torn pair and then CALL it -- a garbage code
// pointer, i.e. a crash rather than a stale value.
//
// REACHABILITY (verified by reading, not inferred):
//	store/hooks.go:20     Store.SetHooks calls SetOnInvalidate
//	server/server.go:90   the ONLY non-test caller of SetHooks, inside func New()
//	                      (starts line 66), which completes before Start() spawns
//	                      goroutines.
//
// So today the write is ordered before any reader exists -- a happens-before
// edge, and no race is live. This test drives them concurrently anyway, because
// the field is reachable through two EXPORTED methods with no documented
// precondition, and SetHooks already locks its sibling:
//
//	hooks.go:16-21  SetHooks takes hooksMu for s.hooks ... then calls
//	                SetOnInvalidate, whose write is unguarded
//
// The sibling is locked and this one is not. That asymmetry -- not the
// hypothetical caller -- is the defect.

// TestOnInvalidateRacesSetHooks is the primary case.
// Only meaningful under -race.
func TestOnInvalidateRacesSetHooks(t *testing.T) {
	s := NewStore()
	ti := s.GetTagIndex()
	if ti == nil {
		t.Fatal("Store.GetTagIndex returned nil")
	}
	// Give the index something so Invalidate has a shard to lock.
	ti.AddTags("key-a", []string{"tag"})

	var wg sync.WaitGroup
	// Writer: re-installing the callback concurrently.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 3000; i++ {
			s.SetHooks(StoreHooks{
				OnTagInvalidate: func(string, []string) {},
			})
		}
	}()
	// Reader: firing the callback.
	for i := 0; i < 3000; i++ {
		ti.Invalidate("tag")
	}
	wg.Wait()
}

// TestOnInvalidateControls is the control: after a quiet SetHooks, Invalidate
// must still invoke the installed callback. Passes before and after the fix, and
// proves the race test above actually reached the read with a callback set.
func TestOnInvalidateControls(t *testing.T) {
	s := NewStore()
	ti := s.GetTagIndex()
	if ti == nil {
		t.Fatal("Store.GetTagIndex returned nil")
	}

	var mu sync.Mutex
	var calls int
	hit := make(chan struct{}, 8)

	s.SetHooks(StoreHooks{
		OnTagInvalidate: func(tag string, keys []string) {
			mu.Lock()
			calls++
			mu.Unlock()
			select {
			case hit <- struct{}{}:
			default:
			}
		},
	})

	ti.AddTags("ctl-key", []string{"ctl-tag"})

	mu.Lock()
	before := calls
	mu.Unlock()

	ti.Invalidate("ctl-tag")

	mu.Lock()
	after := calls
	mu.Unlock()

	if after != before+1 {
		t.Fatalf("CONTROL FAIL: Invalidate fired the hook %d time(s), want exactly 1", after-before)
	}
	select {
	case <-hit:
	default:
	}
	t.Log("CONTROL PASS: SetHooks-installed callback fired exactly once on Invalidate")
}
