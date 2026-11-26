package server

import (
	"sync"
	"sync/atomic"
)

// Writes to one hot key all queue on one stripe of the write gate. Handing
// the stripe from goroutine to goroutine costs more than the writes it
// covers: the woken goroutine has to be scheduled while the stripe sits
// idle, and the key's data then moves to another core's cache. So a write
// that finds its stripe busy does not wait for it: it queues the part of the
// command that needs the stripe, and whoever holds the stripe runs it before
// letting go (flat combining). The hot key stays on one core, and the
// stripe stays busy while there is work for it.

// combineSignal is what a queued write is told when it is let go.
type combineSignal uint8

const (
	// combineServed: the holder ran the write.
	combineServed combineSignal = iota + 1
	// combineTurn: the stripe was released with the write still queued; it
	// is the write's turn to take it.
	combineTurn
)

// combineReq is a write queued on a busy stripe.
type combineReq struct {
	run  func()
	done chan combineSignal
	// panicked is what run panicked with, re-raised by the write's own
	// goroutine so its connection is the one that ends.
	panicked any
}

func newCombineReq(run func()) *combineReq {
	return &combineReq{run: run, done: make(chan combineSignal, 1)}
}

func (r *combineReq) serve() {
	defer func() {
		if p := recover(); p != nil {
			r.panicked = p
		}
	}()
	r.run()
}

// combineQueue holds the writes waiting on one stripe.
type combineQueue struct {
	waiting atomic.Int32
	mu      sync.Mutex
	reqs    []*combineReq
	head    int
}

func (q *combineQueue) push(r *combineReq) {
	q.mu.Lock()
	q.reqs = append(q.reqs, r)
	q.waiting.Add(1)
	q.mu.Unlock()
}

func (q *combineQueue) pop() *combineReq {
	if q.waiting.Load() == 0 {
		return nil
	}
	q.mu.Lock()
	if q.head == len(q.reqs) {
		q.mu.Unlock()
		return nil
	}
	r := q.reqs[q.head]
	q.reqs[q.head] = nil
	q.head++
	if q.head == len(q.reqs) {
		q.reqs, q.head = q.reqs[:0], 0
	}
	q.waiting.Add(-1)
	q.mu.Unlock()
	return r
}

// maxCombine bounds how many queued writes a holder runs at a time before
// its own client gets a turn.
const maxCombine = 128

// drain runs writes queued on stripe s, which the caller holds for writing.
func (g *writeGate) drain(s, limit int) {
	q := &g.queues[s]
	for i := 0; i < limit; i++ {
		r := q.pop()
		if r == nil {
			return
		}
		r.serve()
		r.done <- combineServed
	}
}

// passTurn wakes the first write queued on stripe s, which the caller has
// just released, to take it. Every release checks, so a write that queued
// while the holder was letting go is not left waiting.
func (g *writeGate) passTurn(s int) {
	if r := g.queues[s].pop(); r != nil {
		r.done <- combineTurn
	}
}

// combine takes stripe s of h, a single-stripe write hold whose global part
// the caller already holds, or has r run by the holder. It reports whether
// the caller now holds the stripe, and whether r has run.
func (g *writeGate) combine(s int, r *combineReq) (held, ran bool) {
	mu := &g.stripes[s]
	if mu.TryLock() {
		return true, false
	}
	q := &g.queues[s]
	for {
		q.push(r)
		// The holder may have let go between the TryLock above and the
		// push, seeing nothing queued; taking the stripe now covers that.
		if mu.TryLock() {
			// r is queued, or was just popped and told it is its turn.
			g.drain(s, int(q.waiting.Load()))
			for {
				switch <-r.done {
				case combineServed:
					return true, true
				case combineTurn:
					return true, false
				}
			}
		}
		if <-r.done == combineServed {
			return false, true
		}
		// It is r's turn: the stripe was released. Take it if it is still
		// free, or queue again behind whoever took it.
		if mu.TryLock() {
			return true, false
		}
	}
}
