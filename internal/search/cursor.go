package search

import (
	"errors"
	"math/rand/v2"
	"sync"
	"time"
)

// Cursors follow RediSearch's cursor list: ids are random, a cursor is
// either idle or taken by the command reading it, idle cursors past their
// MAXIDLE are collected every 500 cursor operations (at most once a
// second) or by FT.CURSOR GC, and an index holds at most 128 of them.
const (
	CursorReadSize     = 1000
	CursorMaxIdle      = 300000 * time.Millisecond
	IndexCursorLimit   = 128
	cursorSweepEvery   = 500
	cursorSweepTimeout = time.Second
)

//nolint:staticcheck // RediSearch's reply text
var ErrCursorLimit = errors.New("INDEX_CURSOR_LIMIT of 128 has been reached for an index")

// Cursor is a paused aggregation; State is the command layer's.
type Cursor struct {
	ID       uint64
	Chunk    int
	State    any
	owner    *cacheSet
	maxIdle  time.Duration
	deadline time.Time
	idle     bool
	deleted  bool
}

type cursorList struct {
	mu          sync.Mutex
	cursors     map[uint64]*Cursor
	counter     int
	lastCollect time.Time
	nextIdle    time.Time
}

func (cl *cursorList) tick(now time.Time) {
	cl.counter++
	if cl.counter%cursorSweepEvery == 0 {
		cl.collect(now, false)
	}
}

func (cl *cursorList) collect(now time.Time, force bool) int {
	if !cl.nextIdle.IsZero() && cl.nextIdle.After(now) || !force && now.Sub(cl.lastCollect) < cursorSweepTimeout {
		return -1
	}
	cl.lastCollect = now
	n := 0
	for id, c := range cl.cursors {
		if c.idle && !c.deadline.After(now) {
			cl.unidle(c)
			delete(cl.cursors, id)
			n++
		}
	}
	return n
}

func (cl *cursorList) unidle(c *Cursor) {
	if c.deadline.Equal(cl.nextIdle) {
		cl.nextIdle = time.Time{}
	}
	c.idle = false
}

func (m *IndexManager) cursorList() *cursorList {
	m.cursorOnce.Do(func() { m.cursors = &cursorList{cursors: make(map[uint64]*Cursor)} })
	return m.cursors
}

// ReserveCursor opens a cursor on idx; it is taken until PauseCursor.
func (m *IndexManager) ReserveCursor(idx *Index, maxIdle time.Duration, state any) (*Cursor, error) {
	cl := m.cursorList()
	cl.mu.Lock()
	defer cl.mu.Unlock()
	now := time.Now()
	cl.tick(now)
	if cl.active(idx.cacheSet) >= IndexCursorLimit {
		cl.collect(now, false)
		if cl.active(idx.cacheSet) >= IndexCursorLimit {
			return nil, ErrCursorLimit
		}
	}
	id := uint64(rand.Uint32()&0x7fffffff|1) + 1
	for cl.cursors[id] != nil {
		id = uint64(rand.Uint32()&0x7fffffff|1) + 1
	}
	c := &Cursor{ID: id, State: state, owner: idx.cacheSet, maxIdle: maxIdle}
	cl.cursors[id] = c
	return c, nil
}

func (cl *cursorList) active(owner *cacheSet) int {
	n := 0
	for _, c := range cl.cursors {
		if c.owner == owner {
			n++
		}
	}
	return n
}

// TakeCursor hands an idle cursor to its reader, or returns nil.
func (m *IndexManager) TakeCursor(id uint64) *Cursor {
	cl := m.cursorList()
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.tick(time.Now())
	c := cl.cursors[id]
	if c == nil || !c.idle {
		return nil
	}
	cl.unidle(c)
	return c
}

// PauseCursor returns a taken cursor to the idle list.
func (m *IndexManager) PauseCursor(c *Cursor) {
	cl := m.cursorList()
	cl.mu.Lock()
	defer cl.mu.Unlock()
	now := time.Now()
	cl.tick(now)
	if c.deleted {
		delete(cl.cursors, c.ID)
		return
	}
	c.deadline = now.Add(c.maxIdle)
	if cl.nextIdle.IsZero() || c.deadline.Before(cl.nextIdle) {
		cl.nextIdle = c.deadline
	}
	c.idle = true
}

// FreeCursor closes a taken cursor.
func (m *IndexManager) FreeCursor(c *Cursor) {
	cl := m.cursorList()
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.tick(time.Now())
	delete(cl.cursors, c.ID)
}

// PurgeCursor closes a cursor, at once when idle or after its read.
func (m *IndexManager) PurgeCursor(id uint64) bool {
	cl := m.cursorList()
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.tick(time.Now())
	c := cl.cursors[id]
	if c == nil {
		return false
	}
	if c.idle {
		cl.unidle(c)
		delete(cl.cursors, id)
	} else {
		c.deleted = true
	}
	return true
}

// CollectCursors closes the idle cursors past their MAXIDLE, or returns
// -1 while the earliest deadline it knows of lies ahead, as RediSearch's
// FT.CURSOR GC does.
func (m *IndexManager) CollectCursors() int {
	cl := m.cursorList()
	cl.mu.Lock()
	defer cl.mu.Unlock()
	return cl.collect(time.Now(), true)
}

// CursorIndexAlive reports whether the cursor's index still exists.
func (m *IndexManager) CursorIndexAlive(c *Cursor) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, idx := range m.indexes {
		if idx.cacheSet == c.owner {
			return true
		}
	}
	return false
}

// CursorStats returns the idle and total cursors, and those of idx.
func (m *IndexManager) CursorStats(idx *Index) (idle, total, index int) {
	cl := m.cursorList()
	cl.mu.Lock()
	defer cl.mu.Unlock()
	for _, c := range cl.cursors {
		if c.idle {
			idle++
		}
		if c.owner == idx.cacheSet {
			index++
		}
	}
	return idle, len(cl.cursors), index
}
