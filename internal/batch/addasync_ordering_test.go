package batch

import (
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// instantProcessor returns one result per item, immediately.
type instantProcessor struct{}

func (instantProcessor) Process(items []BatchItem) []BatchResult {
	out := make([]BatchResult, 0, len(items))
	for _, it := range items {
		out = append(out, BatchResult{Key: it.Key, Value: it.Value})
	}
	return out
}

// TestProofAddAsyncRegistersPendingBeforePublishing is the round proof.
//
// Contract: AddAsync(item, callback) must invoke callback exactly once for
// every item it accepts.
//
// Defect: AddAsync pushes the item onto b.items (line 129) but does not
// register the callback's result channel in b.pending until line 141 — twelve
// lines later. processLoop can dequeue the item, run the processor and push
// the result inside that window. resultDispatcher then evaluates
// `if ch, ok := b.pending.Load(result.Key); ok`, which is FALSE because the
// entry does not exist yet — and there is NO else branch, so the result is
// silently DISCARDED. The callback goroutine then blocks on
// `result := <-resultCh` (line 153) forever, holding its workerPool slot, so
// enough drops deadlock AddAsync entirely.
//
// This is publish-before-subscribe: the result is announced to the dispatcher
// before anyone has subscribed to receive it.
//
// IN-REPO BASIS: Batcher.Add performs the SAME two operations in the opposite
// order — `b.pending.Store(item.Key, resultCh)` (line 81) precedes
// `b.items <- item` (line 82). AddAsync is the only path that publishes first
// and subscribes afterwards, and the only path that can drop a result. The
// intended order is therefore unambiguous from the sibling function.
//
// DESIGN NOTE: every item uses a DISTINCT key. b.pending is single-slot per
// key and delete-after-delivery, so reusing one key would conflate this defect
// with that separate design limitation. Distinct keys isolate the ordering bug.
//
// MaxSize:1 makes processLoop dispatch the instant it dequeues, widening the
// window deterministically. The Adds run on a separate goroutine so that a
// deadlock (AddAsync blocked on workerPool) surfaces as the watchdog firing
// rather than as a test-runner timeout.
func TestProofAddAsyncRegistersPendingBeforePublishing(t *testing.T) {
	const iterations = 4000

	b := NewBatcher(BatchConfig{MaxSize: 1, MaxWait: time.Nanosecond, MaxWorkers: 4}, instantProcessor{})
	defer b.Close()

	var fired atomic.Int64
	done := make(chan struct{}, iterations)

	producerDone := make(chan struct{})
	go func() {
		defer close(producerDone)
		for i := 0; i < iterations; i++ {
			// Distinct key per item: isolates ordering from the one-slot-per-key design.
			key := "k" + strconv.Itoa(i)
			b.AddAsync(BatchItem{Key: key, Value: []byte("v")}, func(BatchResult) {
				fired.Add(1)
				done <- struct{}{}
			})
		}
	}()

	received := 0
	deadline := time.After(10 * time.Second)
	for received < iterations {
		select {
		case <-done:
			received++
		case <-deadline:
			producerStuck := false
			select {
			case <-producerDone:
			default:
				producerStuck = true
			}
			t.Fatalf("FAIL: AddAsync dropped %d of %d results%s. "+
				"AddAsync publishes the item onto b.items (line 129) BEFORE registering its "+
				"channel in b.pending (line 141), so resultDispatcher finds no pending entry, "+
				"discards the result, and the callback blocks forever on <-resultCh. "+
				"Batcher.Add registers pending first (line 81) then publishes (line 82); "+
				"AddAsync must use that same order.",
				iterations-received, iterations, producerNote(producerStuck))
		}
	}

	t.Logf("PASS: all %d AddAsync callbacks fired", iterations)
}

// CONTROL: the shared machinery — processor, processLoop and resultDispatcher —
// still routes a single in-flight item correctly. Deliberately SEQUENTIAL: one
// item at a time, so no result can be in flight while the next is published.
//
// It cannot and does not exercise Batcher.Add's concurrency. A previous version
// of this control drove Add from 3000 concurrent goroutines and DEADLOCKED,
// because Add (batch.go:102) drains the shared b.results channel rather than the
// per-key resultCh it registered — a separate, pre-existing defect that this
// round deliberately does NOT fix (one root cause per round). Driving Add
// concurrently would confound the ordering proof under test with that unrelated
// bug, so this control stays sequential.
func TestProofSingleItemRoutingControl(t *testing.T) {
	const iterations = 500

	b := NewBatcher(BatchConfig{MaxSize: 1, MaxWait: time.Nanosecond, MaxWorkers: 4}, instantProcessor{})
	defer b.Close()

	for i := 0; i < iterations; i++ {
		key := "single" + strconv.Itoa(i)
		fired := make(chan BatchResult, 1)
		b.AddAsync(BatchItem{Key: key, Value: []byte("v")}, func(r BatchResult) { fired <- r })

		select {
		case r := <-fired:
			if r.Key != key {
				t.Fatalf("CONTROL FAILED: routed result for %q to the callback for %q — the "+
					"dispatcher is misrouting results", r.Key, key)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("CONTROL FAILED: a single sequential item never produced a callback. " +
				"The shared processor/processLoop/resultDispatcher machinery is broken, which " +
				"would mean the AddAsync ordering defect is not the whole story.")
		}
	}
	t.Logf("PASS: all %d sequential items routed to the correct callback", iterations)
}

// producerNote explains a deadlock signature so the failure names its cause.
func producerNote(stuck bool) string {
	if stuck {
		return " (the producer also deadlocked: dropped callbacks hold their workerPool slots " +
			"forever, so AddAsync eventually blocks on b.workerPool)"
	}
	return ""
}
