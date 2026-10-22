package batch

import (
	"testing"
	"time"
)

// oneResultPerItemProcessor is a real Processor that emits exactly one result
// per submitted item, keyed the same way.
type oneResultPerItemProcessor struct{}

func (oneResultPerItemProcessor) Process(items []BatchItem) []BatchResult {
	out := make([]BatchResult, 0, len(items))
	for _, it := range items {
		out = append(out, BatchResult{Key: it.Key, Value: it.Value})
	}
	return out
}

func recvBounded(t *testing.T, ch <-chan BatchResult, d time.Duration) (BatchResult, bool) {
	t.Helper()
	select {
	case r := <-ch:
		return r, true
	case <-time.After(d):
		return BatchResult{}, false
	}
}

// pendingRegistrations reports how many waiters are registered for key.
func pendingRegistrations(b *Batcher, key string) int {
	b.pendingMu.Lock()
	defer b.pendingMu.Unlock()
	return len(b.pending[key])
}

// b.pending used to be a single-slot map keyed only by Key, so two Add calls
// carrying the same key overwrote each other's registration. resultDispatcher
// loaded the key once, routed a single result into the surviving channel, and
// deleted the entry -- the displaced caller never received its BatchResult and
// its forwarding goroutine stayed blocked forever holding a workerPool slot.
// Each Add caller is entitled to its own result regardless of key reuse.
func TestBatcherAddSameKeyDeliversToEveryCaller(t *testing.T) {
	b := NewBatcher(BatchConfig{MaxSize: 2, MaxWait: 5 * time.Millisecond, MaxWorkers: 4}, oneResultPerItemProcessor{})

	first := b.Add(BatchItem{Key: "same-key", Value: []byte("first")})
	second := b.Add(BatchItem{Key: "same-key", Value: []byte("second")})

	if _, ok := recvBounded(t, first, 5*time.Second); !ok {
		t.Fatal("first Add for a duplicated key never received its result")
	}
	if _, ok := recvBounded(t, second, 5*time.Second); !ok {
		t.Fatal("second Add for a duplicated key never received its result: a same-key " +
			"registration displaced the first caller")
	}

	b.Close()
}

// Control: distinct keys must keep working exactly as before. The same-key case
// above passes trivially if results are dropped for everyone, so this pins the
// unaffected path.
func TestBatcherAddDistinctKeysStillDelivers(t *testing.T) {
	b := NewBatcher(BatchConfig{MaxSize: 2, MaxWait: 5 * time.Millisecond, MaxWorkers: 4}, oneResultPerItemProcessor{})

	first := b.Add(BatchItem{Key: "key-1", Value: []byte("a")})
	second := b.Add(BatchItem{Key: "key-2", Value: []byte("b")})

	if _, ok := recvBounded(t, first, 5*time.Second); !ok {
		t.Error("distinct key 1 never received its result")
	}
	if _, ok := recvBounded(t, second, 5*time.Second); !ok {
		t.Error("distinct key 2 never received its result")
	}

	b.Close()
}

// Boundary: three same-key Adds must all be served, and every registration
// must be drained afterwards -- a leftover waiter means a workerPool slot that
// is never released, which is what wedged the batcher in the first place.
func TestBatcherAddSameKeyDrainsEveryRegistration(t *testing.T) {
	b := NewBatcher(BatchConfig{MaxSize: 3, MaxWait: 5 * time.Millisecond, MaxWorkers: 4}, oneResultPerItemProcessor{})

	chans := make([]<-chan BatchResult, 0, 3)
	for i := 0; i < 3; i++ {
		chans = append(chans, b.Add(BatchItem{Key: "triple", Value: []byte{byte(i)}}))
	}

	for i, ch := range chans {
		if _, ok := recvBounded(t, ch, 5*time.Second); !ok {
			t.Fatalf("Add #%d for a triplicated key never received its result", i+1)
		}
	}

	// Every registration must have been consumed, or its goroutine still holds
	// a workerPool slot forever.
	if n := pendingRegistrations(b, "triple"); n != 0 {
		t.Errorf("expected all registrations for the key to be drained, %d left", n)
	}

	b.Close()
}

// The worker-pool leak is the severe half of the same defect: a displaced
// waiter never completes, so its slot is never returned. With MaxWorkers=1 a
// single lost result wedges every later Add. Each Add here is given a bounded
// window; if any slot leaks the later Adds can never complete.
func TestBatcherAddSameKeyDoesNotLeakWorkerSlots(t *testing.T) {
	const maxWorkers = 1
	b := NewBatcher(BatchConfig{MaxSize: 2, MaxWait: 5 * time.Millisecond, MaxWorkers: maxWorkers}, oneResultPerItemProcessor{})

	// Two Add calls sharing one key: enough to displace a registration.
	ch1 := b.Add(BatchItem{Key: "shared", Value: []byte("1")})
	ch2 := b.Add(BatchItem{Key: "shared", Value: []byte("2")})

	if _, ok := recvBounded(t, ch1, 5*time.Second); !ok {
		t.Fatal("first shared-key Add never completed")
	}
	if _, ok := recvBounded(t, ch2, 5*time.Second); !ok {
		t.Fatal("second shared-key Add never completed")
	}

	// The only worker slot must be free again; otherwise this Add blocks.
	after := b.Add(BatchItem{Key: "after", Value: []byte("3")})
	if _, ok := recvBounded(t, after, 5*time.Second); !ok {
		t.Fatal("a later Add never completed: a displaced same-key waiter leaked its " +
			"workerPool slot and permanently reduced capacity")
	}

	b.Close()
}
