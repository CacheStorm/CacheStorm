package batch

import (
	"sync"
	"sync/atomic"
	"time"
)

type BatchItem struct {
	Key   string
	Value []byte
	TTL   time.Duration
}

type BatchResult struct {
	Key   string
	Error error
	Value []byte
}

type Processor interface {
	Process(items []BatchItem) []BatchResult
}

type BatchConfig struct {
	MaxSize    int
	MaxWait    time.Duration
	MaxWorkers int
}

type Batcher struct {
	config      BatchConfig
	processor   Processor
	items       chan BatchItem
	results     chan BatchResult
	pending     sync.Map
	stopCh      chan struct{}
	wg          sync.WaitGroup
	flushCh     chan struct{}
	count       atomic.Int32
	workerPool  chan struct{} // Semaphore for limiting concurrent goroutines
}

func NewBatcher(config BatchConfig, processor Processor) *Batcher {
	if config.MaxSize <= 0 {
		config.MaxSize = 100
	}
	if config.MaxWait <= 0 {
		config.MaxWait = 10 * time.Millisecond
	}
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 4
	}

	b := &Batcher{
		config:     config,
		processor:  processor,
		items:      make(chan BatchItem, config.MaxSize*2),
		results:    make(chan BatchResult, config.MaxSize*2),
		stopCh:     make(chan struct{}),
		flushCh:    make(chan struct{}, 1),
		workerPool: make(chan struct{}, config.MaxWorkers),
	}

	// Start result dispatcher workers
	for i := 0; i < config.MaxWorkers; i++ {
		b.wg.Add(1)
		go b.resultDispatcher()
	}

	b.wg.Add(1)
	go b.processLoop()

	return b
}

func (b *Batcher) Add(item BatchItem) <-chan BatchResult {
	resultCh := make(chan BatchResult, 1)

	b.pending.Store(item.Key, resultCh)
	b.items <- item
	b.count.Add(1)

	if b.count.Load() >= int32(b.config.MaxSize) {
		select {
		case b.flushCh <- struct{}{}:
		default:
		}
	}

	// Acquire worker slot (blocks if MaxWorkers reached)
	b.workerPool <- struct{}{}

	go func() {
		defer func() { <-b.workerPool }() // Release worker slot
		result := <-b.results
		if ch, ok := b.pending.Load(result.Key); ok {
			ch.(chan BatchResult) <- result
			b.pending.Delete(result.Key)
		}
	}()

	return resultCh
}

func (b *Batcher) AddAsync(item BatchItem, callback func(BatchResult)) {
	// If no callback, just queue the item without starting a goroutine
	if callback == nil {
		b.items <- item
		b.count.Add(1)

		if b.count.Load() >= int32(b.config.MaxSize) {
			select {
			case b.flushCh <- struct{}{}:
			default:
			}
		}
		return
	}

	b.items <- item
	b.count.Add(1)

	if b.count.Load() >= int32(b.config.MaxSize) {
		select {
		case b.flushCh <- struct{}{}:
		default:
		}
	}

	// Acquire worker slot (blocks if MaxWorkers reached)
	b.workerPool <- struct{}{}

	go func() {
		defer func() { <-b.workerPool }() // Release worker slot
		result := <-b.results
		callback(result)
	}()
}

func (b *Batcher) processLoop() {
	defer b.wg.Done()

	ticker := time.NewTicker(b.config.MaxWait)
	defer ticker.Stop()

	batch := make([]BatchItem, 0, b.config.MaxSize)

	for {
		select {
		case <-b.stopCh:
			if len(batch) > 0 {
				b.processBatch(batch)
			}
			return

		case item := <-b.items:
			batch = append(batch, item)
			b.count.Add(-1)
			if len(batch) >= b.config.MaxSize {
				b.processBatch(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				b.processBatch(batch)
				batch = batch[:0]
			}

		case <-b.flushCh:
			if len(batch) > 0 {
				b.processBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

func (b *Batcher) processBatch(items []BatchItem) {
	if len(items) == 0 {
		return
	}

	results := b.processor.Process(items)
	for _, r := range results {
		b.results <- r
	}
}

func (b *Batcher) Flush() {
	select {
	case b.flushCh <- struct{}{}:
	default:
	}
}

func (b *Batcher) resultDispatcher() {
	defer b.wg.Done()
	for {
		select {
		case <-b.stopCh:
			return
		case result := <-b.results:
			if ch, ok := b.pending.Load(result.Key); ok {
				select {
				case ch.(chan BatchResult) <- result:
					b.pending.Delete(result.Key)
				case <-b.stopCh:
					return
				}
			}
		}
	}
}

func (b *Batcher) Close() {
	close(b.stopCh)
	b.wg.Wait()
	close(b.items)
	close(b.results)
	close(b.workerPool)
}

type Pipeline struct {
	commands []Command
	mu       sync.Mutex
}

type Command struct {
	Name string
	Args [][]byte
}

func NewPipeline() *Pipeline {
	return &Pipeline{
		commands: make([]Command, 0),
	}
}

func (p *Pipeline) Add(name string, args [][]byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.commands = append(p.commands, Command{Name: name, Args: args})
}

func (p *Pipeline) Commands() []Command {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]Command, len(p.commands))
	copy(result, p.commands)
	return result
}

func (p *Pipeline) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.commands = p.commands[:0]
}

func (p *Pipeline) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.commands)
}

type MultiGet struct {
	store MultiGetter
	batch int
}

type MultiGetter interface {
	GetMulti(keys []string) (map[string][]byte, error)
}

func NewMultiGet(store MultiGetter, batchSize int) *MultiGet {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &MultiGet{
		store: store,
		batch: batchSize,
	}
}

func (m *MultiGet) Get(keys []string) (map[string][]byte, error) {
	result := make(map[string][]byte)

	for i := 0; i < len(keys); i += m.batch {
		end := i + m.batch
		if end > len(keys) {
			end = len(keys)
		}

		batch := keys[i:end]
		batchResult, err := m.store.GetMulti(batch)
		if err != nil {
			return nil, err
		}

		for k, v := range batchResult {
			result[k] = v
		}
	}

	return result, nil
}
