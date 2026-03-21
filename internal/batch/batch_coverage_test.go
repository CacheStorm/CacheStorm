package batch

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// mockErrorMultiGetter returns an error from GetMulti
type mockErrorMultiGetter struct{}

func (m *mockErrorMultiGetter) GetMulti(keys []string) (map[string][]byte, error) {
	return nil, errors.New("store error")
}

// Cover MultiGet.Get error path
func TestMultiGetGetError(t *testing.T) {
	mg := &mockErrorMultiGetter{}
	m := NewMultiGet(mg, 2)

	_, err := m.Get([]string{"key1", "key2", "key3"})
	if err == nil {
		t.Error("expected error from GetMulti")
	}
}

// Cover NewMultiGet with negative batch size (defaults to 100)
func TestNewMultiGetNegativeBatchSize(t *testing.T) {
	mg := &mockErrorMultiGetter{}
	m := NewMultiGet(mg, -5)

	if m.batch != 100 {
		t.Errorf("expected batch 100 for negative input, got %d", m.batch)
	}
}

// Cover AddAsync with nil callback (fire-and-forget path)
func TestBatcherAddAsyncNilCallback(t *testing.T) {
	processor := &mockProcessor{}
	b := NewBatcher(BatchConfig{
		MaxSize: 10,
		MaxWait: 50 * time.Millisecond,
	}, processor)

	b.AddAsync(BatchItem{Key: "async-nil", Value: []byte("value")}, nil)

	time.Sleep(100 * time.Millisecond)
	b.Close()
}

// Cover AddAsync nil callback with flush trigger (count >= MaxSize)
func TestBatcherAddAsyncNilCallbackFlush(t *testing.T) {
	processor := &mockProcessor{}
	b := NewBatcher(BatchConfig{
		MaxSize: 1,
		MaxWait: 1 * time.Second,
	}, processor)

	b.AddAsync(BatchItem{Key: "flush-nil", Value: []byte("val")}, nil)

	time.Sleep(100 * time.Millisecond)
	b.Close()
}

// Cover AddAsync with callback and flush trigger
func TestBatcherAddAsyncWithCallbackFlush(t *testing.T) {
	processor := &mockProcessor{}
	b := NewBatcher(BatchConfig{
		MaxSize:    1,
		MaxWait:    1 * time.Second,
		MaxWorkers: 4,
	}, processor)

	var wg sync.WaitGroup
	wg.Add(1)

	b.AddAsync(BatchItem{Key: "callback-flush", Value: []byte("val")}, func(r BatchResult) {
		wg.Done()
	})

	wg.Wait()
	b.Close()
}

// Cover Batcher.Add with batch size triggering flush
func TestBatcherAddTriggersFlush(t *testing.T) {
	processor := &mockProcessor{}
	b := NewBatcher(BatchConfig{
		MaxSize:    2,
		MaxWait:    1 * time.Second,
		MaxWorkers: 8,
	}, processor)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		ch := b.Add(BatchItem{Key: "k1", Value: []byte("v1")})
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
		}
	}()

	go func() {
		defer wg.Done()
		ch := b.Add(BatchItem{Key: "k2", Value: []byte("v2")})
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
		}
	}()

	wg.Wait()
	b.Close()
}

// Cover processBatch with empty items (short-circuit)
func TestBatcherProcessBatchEmpty(t *testing.T) {
	processor := &mockProcessor{}
	b := NewBatcher(BatchConfig{
		MaxSize: 10,
		MaxWait: 50 * time.Millisecond,
	}, processor)

	b.processBatch(nil)
	b.processBatch([]BatchItem{})

	b.Close()
}

// Cover NewBatcher with custom config values (no defaults applied)
func TestNewBatcherCustomConfig(t *testing.T) {
	processor := &mockProcessor{}
	b := NewBatcher(BatchConfig{
		MaxSize:    50,
		MaxWait:    200 * time.Millisecond,
		MaxWorkers: 8,
	}, processor)

	if b.config.MaxSize != 50 {
		t.Errorf("expected MaxSize 50, got %d", b.config.MaxSize)
	}
	if b.config.MaxWait != 200*time.Millisecond {
		t.Errorf("expected MaxWait 200ms, got %v", b.config.MaxWait)
	}
	if b.config.MaxWorkers != 8 {
		t.Errorf("expected MaxWorkers 8, got %d", b.config.MaxWorkers)
	}

	b.Close()
}

// Cover MultiGet.Get with batch boundary (exact multiple)
func TestMultiGetExactBatchBoundary(t *testing.T) {
	mg := &mockMultiGetter{
		data: map[string][]byte{
			"k1": []byte("v1"),
			"k2": []byte("v2"),
			"k3": []byte("v3"),
			"k4": []byte("v4"),
		},
	}

	m := NewMultiGet(mg, 2)
	result, err := m.Get([]string{"k1", "k2", "k3", "k4"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 4 {
		t.Errorf("expected 4 results, got %d", len(result))
	}
}

// Cover MultiGet.Get with single key
func TestMultiGetSingleKey(t *testing.T) {
	mg := &mockMultiGetter{
		data: map[string][]byte{
			"only": []byte("one"),
		},
	}

	m := NewMultiGet(mg, 10)
	result, err := m.Get([]string{"only"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 result, got %d", len(result))
	}
}

// Cover Flush when flushCh is already full (default branch)
func TestBatcherFlushWhenAlreadyPending(t *testing.T) {
	processor := &mockProcessor{}
	b := NewBatcher(BatchConfig{
		MaxSize: 100,
		MaxWait: 1 * time.Second,
	}, processor)

	b.Flush()
	b.Flush()

	b.Close()
}

// Cover MultiGet error on second batch (first succeeds, second fails)
type mockPartialErrorMultiGetter struct {
	callCount int
	mu        sync.Mutex
}

func (m *mockPartialErrorMultiGetter) GetMulti(keys []string) (map[string][]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if m.callCount > 1 {
		return nil, errors.New("second batch error")
	}
	result := make(map[string][]byte)
	for _, k := range keys {
		result[k] = []byte("val")
	}
	return result, nil
}

func TestMultiGetPartialError(t *testing.T) {
	mg := &mockPartialErrorMultiGetter{}
	m := NewMultiGet(mg, 2)

	_, err := m.Get([]string{"k1", "k2", "k3", "k4"})
	if err == nil {
		t.Error("expected error from second batch")
	}
}

// Cover resultDispatcher with invalid channel type in pending map (lines 222-226).
// We manually store a non-channel value and then push a result that triggers
// the type assertion failure path.
func TestResultDispatcherInvalidChannelType(t *testing.T) {
	processor := &mockProcessor{}
	b := NewBatcher(BatchConfig{
		MaxSize:    100,
		MaxWait:    50 * time.Millisecond,
		MaxWorkers: 4,
	}, processor)

	// Manually inject an invalid (non-channel) value into the pending map
	b.pending.Store("badkey", "not-a-channel")

	// Push a result for "badkey" into the results channel.
	// The resultDispatcher should hit the !validCh branch and delete the key.
	b.results <- BatchResult{Key: "badkey", Value: []byte("val")}

	// Give the dispatcher time to process it
	time.Sleep(100 * time.Millisecond)

	// Verify the invalid entry was cleaned up
	if _, ok := b.pending.Load("badkey"); ok {
		t.Error("expected pending entry for badkey to be deleted")
	}

	b.Close()
}

// Cover processLoop flushCh case with items in the batch (lines 188-192).
// We add items slowly enough that they accumulate but then trigger a flush
// before the ticker fires.
func TestProcessLoopFlushWithPendingItems(t *testing.T) {
	processor := &mockProcessor{}
	b := NewBatcher(BatchConfig{
		MaxSize:    100, // large so items don't auto-batch
		MaxWait:    5 * time.Second,
		MaxWorkers: 8,
	}, processor)

	// Add an item - it will sit in the internal batch waiting for ticker or flush
	resultCh := b.Add(BatchItem{Key: "pending1", Value: []byte("v1")})

	// Small sleep to ensure processLoop has dequeued the item from the items channel
	time.Sleep(50 * time.Millisecond)

	// Now trigger a flush - this should hit the flushCh case while batch has items
	b.Flush()

	select {
	case <-resultCh:
		// Good, got a result
	case <-time.After(2 * time.Second):
		// The item may have been processed already, that's fine
	}

	b.Close()
}

// Cover the default branch in Add's flush select (line 88).
// When flushCh is already full, the select hits default.
func TestBatcherAddFlushChFull(t *testing.T) {
	processor := &mockProcessor{}
	b := NewBatcher(BatchConfig{
		MaxSize:    1,
		MaxWait:    5 * time.Second,
		MaxWorkers: 8,
	}, processor)

	// Pre-fill the flush channel
	select {
	case b.flushCh <- struct{}{}:
	default:
	}

	// Now Add should hit default branch in the flush select since flushCh is full
	ch := b.Add(BatchItem{Key: "overflow", Value: []byte("val")})

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
	}

	b.Close()
}

// Cover AddAsync nil callback when flushCh is already full (line 123 default branch)
func TestBatcherAddAsyncNilCallbackFlushChFull(t *testing.T) {
	processor := &mockProcessor{}
	b := NewBatcher(BatchConfig{
		MaxSize: 1,
		MaxWait: 50 * time.Millisecond,
	}, processor)

	// Pre-fill the flush channel
	select {
	case b.flushCh <- struct{}{}:
	default:
	}

	// Now AddAsync with nil callback should hit default in flush select
	b.AddAsync(BatchItem{Key: "nil-overflow", Value: []byte("val")}, nil)

	time.Sleep(100 * time.Millisecond)
	b.Close()
}

// Cover AddAsync with callback when flushCh is already full (line 135 default branch)
func TestBatcherAddAsyncCallbackFlushChFull(t *testing.T) {
	processor := &mockProcessor{}
	b := NewBatcher(BatchConfig{
		MaxSize:    1,
		MaxWait:    5 * time.Second,
		MaxWorkers: 8,
	}, processor)

	// Pre-fill the flush channel
	select {
	case b.flushCh <- struct{}{}:
	default:
	}

	var wg sync.WaitGroup
	wg.Add(1)

	b.AddAsync(BatchItem{Key: "cb-overflow", Value: []byte("val")}, func(r BatchResult) {
		wg.Done()
	})

	wg.Wait()
	b.Close()
}

// Cover resultDispatcher stopCh-during-send path (lines 231-232).
// We store a pending entry with a full (blocked) channel, then push a result and close.
func TestResultDispatcherStopDuringSend(t *testing.T) {
	processor := &mockProcessor{}
	b := NewBatcher(BatchConfig{
		MaxSize:    100,
		MaxWait:    5 * time.Second,
		MaxWorkers: 4,
	}, processor)

	// Create a channel that's already full (buffer 0, nobody reading)
	blockedCh := make(chan BatchResult)
	b.pending.Store("blocked-key", blockedCh)

	// Push a result for that key; the dispatcher will try to send on blockedCh
	// but it's blocked because nobody is reading. Then closing will trigger stopCh.
	b.results <- BatchResult{Key: "blocked-key", Value: []byte("val")}

	// Give the dispatcher a moment to pick up the result and block on send
	time.Sleep(50 * time.Millisecond)

	// Now close - the dispatcher should exit via the stopCh case in the inner select
	b.Close()
}

