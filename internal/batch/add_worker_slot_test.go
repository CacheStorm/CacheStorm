package batch

import (
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// probeFactory returns one result per item, immediately.
type probeFactory struct{}

func (probeFactory) Process(items []BatchItem) []BatchResult {
	out := make([]BatchResult, 0, len(items))
	for _, it := range items {
		out = append(out, BatchResult{Key: it.Key, Value: it.Value})
	}
	return out
}

// TestProofAddReleasesItsWorkerSlot is the round proof.
//
// CONTRACT: Batcher.Add(item) returns a channel that yields that item's result,
// and Add must release its worker-pool slot once it is done. A batcher that
// leaks one slot per call deadlocks permanently after MaxWorkers calls.
//
// DEFECT: Add registers resultCh in b.pending (line 81) and then spawns a
// goroutine that waits on `result := <-b.results` — the SHARED results
// channel. But resultDispatcher is a separate goroutine that ALSO drains
// b.results and routes each result into the per-key channel in b.pending.
// So the dispatcher normally consumes the result and delivers it to resultCh;
// the Add goroutine's receive on b.results then never fires, and that goroutine
// blocks FOREVER while still holding its workerPool slot (acquired at line 93,
// released only in its deferred <-b.workerPool).
//
// After MaxWorkers such leaks every slot is held by a permanently blocked
// goroutine, and the next b.Add blocks forever at `b.workerPool <- struct{}{}`.
//
// Note the caller still receives a correct result — the dispatcher delivers it.
// The failure is not a lost value; it is an unreleased resource that wedges the
// batcher.
//
// IN-REPO BASIS: the sibling AddAsync waits on its OWN registered channel
// (`result := <-resultCh`) — the correct form, fixed in round 16. Add is the
// only path still waiting on the shared channel.
//
// MaxWorkers=1 makes the single leaked slot visible immediately, and the loop
// is long enough that the dispatcher winning the race is overwhelmingly likely
// rather than merely possible. The producer runs on its own goroutine behind a
// watchdog so the deadlock surfaces as a named failure instead of a test-runner
// timeout.
func TestProofAddReleasesItsWorkerSlot(t *testing.T) {
	const iterations = 64

	b := NewBatcher(BatchConfig{MaxSize: 1, MaxWait: time.Nanosecond, MaxWorkers: 1}, probeFactory{})
	defer b.Close()

	// ---- CONTROL: a single Add delivers its result to the caller.
	ch := b.Add(BatchItem{Key: "control", Value: []byte("v")})
	select {
	case r := <-ch:
		if r.Key != "control" {
			t.Fatalf("CONTROL broken harness: routed %q to the control's channel", r.Key)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("CONTROL broken harness: a single Add never delivered a result")
	}
	t.Log("CONTROL 1 ok: a single Add delivers the right result")

	// ---- CONTROL: the batcher is still usable immediately afterwards.
	ch2 := b.Add(BatchItem{Key: "control2", Value: []byte("v")})
	select {
	case <-ch2:
	case <-time.After(5 * time.Second):
		t.Fatalf("CONTROL broken harness: the batcher wedged after a single Add — the harness " +
			"cannot evaluate the defect")
	}
	t.Log("CONTROL 2 ok: a second Add still succeeds")

	// ---- THE DEFECT: repeated Adds must not exhaust the worker pool.
	var delivered atomic.Int64
	producerDone := make(chan struct{})

	go func() {
		defer close(producerDone)
		for i := 0; i < iterations; i++ {
			// Distinct keys: b.pending is single-slot per key, so reusing one
			// would conflate this defect with that separate design limit.
			c := b.Add(BatchItem{Key: "k" + strconv.Itoa(i), Value: []byte("v")})
			go func(res <-chan BatchResult) {
				<-res
				delivered.Add(1)
			}(c)
		}
	}()

	deadline := time.After(15 * time.Second)
	for delivered.Load() < int64(iterations) {
		select {
		case <-time.After(50 * time.Millisecond):
			if delivered.Load() >= int64(iterations) {
				goto done
			}
		case <-deadline:
			producerStuck := false
			select {
			case <-producerDone:
			default:
				producerStuck = true
			}
			stalled := "a caller goroutine is stuck"
			if producerStuck {
				stalled = "Batcher.Add itself is blocked on b.workerPool"
			}
			t.Fatalf("FAIL: only %d of %d Add calls delivered; %s.\n"+
				"MaxWorkers is 1 and the batcher wedged permanently after roughly one call.\n"+
				"Add's goroutine waits on the SHARED `result := <-b.results`, but "+
				"resultDispatcher is a separate goroutine that also drains b.results and routes "+
				"each result into the per-key channel in b.pending. The dispatcher consumes the "+
				"result, so Add's goroutine blocks forever while still holding its workerPool "+
				"slot acquired at `b.workerPool <- struct{}{}`; after MaxWorkers leaks the "+
				"batcher deadlocks. The sibling AddAsync correctly waits on its own registered "+
				"channel (`<-resultCh`) and does not leak.",
				delivered.Load(), iterations, stalled)
		}
	}
done:
	if delivered.Load() < int64(iterations) {
		t.Fatalf("FAIL: only %d of %d Add calls delivered", delivered.Load(), iterations)
	}
	t.Logf("PASS: all %d Add calls delivered without exhausting the worker pool", iterations)
}

// TestProofAddAndAddAsyncInterleave is a control that both public entry points
// still work together after the fix.
func TestProofAddAndAddAsyncInterleave(t *testing.T) {
	b := NewBatcher(BatchConfig{MaxSize: 2, MaxWait: time.Nanosecond, MaxWorkers: 2}, probeFactory{})
	defer b.Close()

	for i := 0; i < 40; i++ {
		key := "mix" + strconv.Itoa(i)
		c := b.Add(BatchItem{Key: key, Value: []byte("v")})
		select {
		case r := <-c:
			if r.Key != key {
				t.Fatalf("CONTROL FAILED: result for %q delivered to %q's channel", r.Key, key)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("CONTROL FAILED: Add wedged at iteration %d", i)
		}
	}
	t.Log("PASS: 40 interleaved Add calls all delivered")
}
