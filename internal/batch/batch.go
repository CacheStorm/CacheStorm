package batch

import (
	"bytes"
	"log"
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
	config     BatchConfig
	processor  Processor
	items      chan BatchItem
	results    chan BatchResult
	pending    sync.Map
	stopCh     chan struct{}
	wg         sync.WaitGroup
	flushCh    chan struct{}
	count      atomic.Int32
	workerPool chan struct{} // Semaphore for limiting concurrent goroutines
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
	// Two channels: pendingCh is what resultDispatcher routes into, callerCh
	// is what the caller reads. They must not be the same channel — the
	// internal goroutine below consumes pendingCh and forwards, so sharing one
	// channel would let the goroutine and the caller race for the single
	// result, leaving the loser blocked forever holding its workerPool slot.
	callerCh := make(chan BatchResult, 1)
	pendingCh := make(chan BatchResult, 1)

	b.pending.Store(item.Key, pendingCh)
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
		defer func() {
			<-b.workerPool // Release worker slot
			if r := recover(); r != nil {
				log.Printf("batch: panic recovered in worker: %v", r)
			}
		}()
		// Wait on this item's own registered channel, not the shared
		// b.results: resultDispatcher already drains b.results and routes each
		// result into b.pending. Receiving from b.results meant the dispatcher
		// normally consumed the result first, so this goroutine blocked forever
		// while still holding its workerPool slot and the batcher deadlocked
		// once MaxWorkers slots leaked. Then forward to the caller's channel.
		if result, ok := <-pendingCh; ok {
			callerCh <- result
		}
	}()

	return callerCh
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

	// Register in pending BEFORE publishing the item. Publishing first let
	// processLoop run the processor and hand the result to resultDispatcher
	// before this key was in b.pending; the dispatcher's
	// `if ch, ok := b.pending.Load(key); ok` then found nothing, discarded the
	// result, and this callback blocked on <-resultCh forever holding its
	// workerPool slot — so drops accumulated until AddAsync deadlocked.
	// Batcher.Add already orders it this way (Store, then publish).
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
		defer func() {
			<-b.workerPool
			if r := recover(); r != nil {
				log.Printf("batch: panic recovered in async worker: %v", r)
			}
		}()
		result := <-resultCh
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
				resCh, validCh := ch.(chan BatchResult)
				if !validCh {
					b.pending.Delete(result.Key)
					continue
				}
				select {
				case resCh <- result:
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
	for i, command := range p.commands {
		if command.Args == nil {
			continue
		}
		result[i].Args = make([][]byte, len(command.Args))
		for j, arg := range command.Args {
			result[i].Args[j] = bytes.Clone(arg)
		}
	}
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
