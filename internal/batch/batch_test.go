package batch

import (
	"sync"
	"testing"
	"time"
)

type mockProcessor struct {
	results []BatchResult
	mu      sync.Mutex
}

func (m *mockProcessor) Process(items []BatchItem) []BatchResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	results := make([]BatchResult, len(items))
	for i, item := range items {
		results[i] = BatchResult{Key: item.Key, Value: item.Value}
		m.results = append(m.results, results[i])
	}
	return results
}

func TestNewBatcherDefaults(t *testing.T) {
	processor := &mockProcessor{}
	b := NewBatcher(BatchConfig{}, processor)

	if b.config.MaxSize != 100 {
		t.Errorf("expected MaxSize 100, got %d", b.config.MaxSize)
	}

	if b.config.MaxWait != 10*time.Millisecond {
		t.Errorf("expected MaxWait 10ms, got %v", b.config.MaxWait)
	}

	if b.config.MaxWorkers != 4 {
		t.Errorf("expected MaxWorkers 4, got %d", b.config.MaxWorkers)
	}

	b.Close()
}

func TestBatcherAdd(t *testing.T) {
	processor := &mockProcessor{}
	b := NewBatcher(BatchConfig{
		MaxSize: 2,
		MaxWait: 100 * time.Millisecond,
	}, processor)

	resultCh := b.Add(BatchItem{Key: "key1", Value: []byte("value1")})

	select {
	case result := <-resultCh:
		if result.Key != "key1" {
			t.Errorf("expected key1, got %s", result.Key)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for result")
	}

	b.Close()
}

func TestBatcherFlush(t *testing.T) {
	processor := &mockProcessor{}
	b := NewBatcher(BatchConfig{
		MaxSize: 100,
		MaxWait: 1 * time.Second,
	}, processor)

	b.Add(BatchItem{Key: "key1", Value: []byte("value1")})
	b.Flush()

	time.Sleep(50 * time.Millisecond)
	b.Close()
}

func TestPipelineAdd(t *testing.T) {
	p := NewPipeline()
	p.Add("SET", [][]byte{[]byte("key"), []byte("value")})

	if p.Len() != 1 {
		t.Errorf("expected 1 command, got %d", p.Len())
	}
}

func TestPipelineCommands(t *testing.T) {
	p := NewPipeline()
	p.Add("SET", [][]byte{[]byte("key"), []byte("value")})
	p.Add("GET", [][]byte{[]byte("key")})

	cmds := p.Commands()
	if len(cmds) != 2 {
		t.Errorf("expected 2 commands, got %d", len(cmds))
	}
}

func TestPipelineClear(t *testing.T) {
	p := NewPipeline()
	p.Add("SET", [][]byte{[]byte("key")})
	p.Clear()

	if p.Len() != 0 {
		t.Errorf("expected 0 commands, got %d", p.Len())
	}
}

func TestPipelineLen(t *testing.T) {
	p := NewPipeline()

	if p.Len() != 0 {
		t.Errorf("expected 0, got %d", p.Len())
	}

	p.Add("SET", nil)
	p.Add("GET", nil)

	if p.Len() != 2 {
		t.Errorf("expected 2, got %d", p.Len())
	}
}

type mockMultiGetter struct {
	data map[string][]byte
}

func (m *mockMultiGetter) GetMulti(keys []string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for _, k := range keys {
		if v, ok := m.data[k]; ok {
			result[k] = v
		}
	}
	return result, nil
}

func TestMultiGetGet(t *testing.T) {
	mg := &mockMultiGetter{
		data: map[string][]byte{
			"key1": []byte("value1"),
			"key2": []byte("value2"),
		},
	}

	m := NewMultiGet(mg, 10)
	result, err := m.Get([]string{"key1", "key2"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}
}

func TestMultiGetEmpty(t *testing.T) {
	mg := &mockMultiGetter{data: make(map[string][]byte)}
	m := NewMultiGet(mg, 10)

	result, err := m.Get([]string{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

func TestMultiGetBatchSize(t *testing.T) {
	mg := &mockMultiGetter{
		data: map[string][]byte{
			"key1": []byte("v1"),
			"key2": []byte("v2"),
			"key3": []byte("v3"),
		},
	}

	m := NewMultiGet(mg, 2)
	result, err := m.Get([]string{"key1", "key2", "key3"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 results, got %d", len(result))
	}
}

func TestMultiGetDefaultBatchSize(t *testing.T) {
	mg := &mockMultiGetter{data: make(map[string][]byte)}
	m := NewMultiGet(mg, 0)

	if m.batch != 100 {
		t.Errorf("expected batch 100, got %d", m.batch)
	}
}

func TestBatcherAddAsync(t *testing.T) {
	processor := &mockProcessor{}
	b := NewBatcher(BatchConfig{
		MaxSize: 1,
		MaxWait: 100 * time.Millisecond,
	}, processor)

	var result BatchResult
	var wg sync.WaitGroup
	wg.Add(1)

	b.AddAsync(BatchItem{Key: "key1", Value: []byte("value1")}, func(r BatchResult) {
		result = r
		wg.Done()
	})

	wg.Wait()

	if result.Key != "key1" {
		t.Errorf("expected key1, got %s", result.Key)
	}

	b.Close()
}

func TestBatcherClose(t *testing.T) {
	processor := &mockProcessor{}
	b := NewBatcher(BatchConfig{}, processor)

	b.Close()
}

func TestBatchItem(t *testing.T) {
	item := BatchItem{
		Key:   "testkey",
		Value: []byte("testvalue"),
		TTL:   10 * time.Second,
	}

	if item.Key != "testkey" {
		t.Errorf("expected testkey, got %s", item.Key)
	}
}

func TestBatchResult(t *testing.T) {
	result := BatchResult{
		Key:   "testkey",
		Value: []byte("testvalue"),
		Error: nil,
	}

	if result.Key != "testkey" {
		t.Errorf("expected testkey, got %s", result.Key)
	}
}

func TestCommand(t *testing.T) {
	cmd := Command{
		Name: "SET",
		Args: [][]byte{[]byte("key"), []byte("value")},
	}

	if cmd.Name != "SET" {
		t.Errorf("expected SET, got %s", cmd.Name)
	}
}
