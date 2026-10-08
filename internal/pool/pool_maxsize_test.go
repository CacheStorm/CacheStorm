package pool

import (
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// countingFactory hands out net.Pipe ends and records how many were created.
type countingFactory struct{ created atomic.Int64 }

func (f *countingFactory) new() (net.Conn, error) {
	f.created.Add(1)
	c, peer := net.Pipe()
	go func() {
		// Drain the peer so a Write never blocks, and hold it open.
		buf := make([]byte, 64)
		for {
			if _, err := peer.Read(buf); err != nil {
				return
			}
		}
	}()
	return c, nil
}

// TestPoolMaxSizeIsEnforced is the round proof.
//
// CONTRACT: a pool configured with MaxSize = N must not hand out an (N+1)th
// SIMULTANEOUS connection. Get() must block (or time out) once N connections
// are checked out, because bounding concurrent connections is the entire point
// of a connection pool.
//
// CONTRACT-INDEPENDENT ANCHOR: this needs no reference server. "A pool bounded
// at N never has more than N live connections" is a property of the pool's own
// configuration, verifiable entirely within this package.
//
// DEFECT: Pool.Get REMOVES a connection from p.conns when it hands it out
// (p.conns = append(p.conns[:i], p.conns[i+1:]...)) and a factory-created
// connection is never appended at all. So p.conns holds ONLY idle connections,
// and the capacity test
//
//	if len(p.conns) < p.config.MaxSize {
//
// is really "idleCount < MaxSize" — which stays true no matter how many
// connections are currently checked out. The pool therefore grows without
// bound, never blocks, and never returns ErrPoolTimeout.
//
// Stats() is corrupted by the same defect: it counts in-use vs idle over
// p.conns, but checked-out connections are no longer in p.conns, so
// Stats().InUse is structurally always 0 while connections are held.
func TestPoolMaxSizeIsEnforced(t *testing.T) {
	const maxSize = 2

	f := &countingFactory{}
	p := NewPool(PoolConfig{
		InitialSize: 0,
		MaxSize:     maxSize,
		MaxIdle:     maxSize,
		IdleTimeout: time.Minute,
	}, f.new)
	defer p.Close()

	// ---- CONTROL 1: the pool can hand out its configured capacity.
	for i := 0; i < maxSize; i++ {
		// The connection deliberately stays checked out for the whole loop:
		// that is what drives the pool to its MaxSize.
		_, err := p.Get()
		if err != nil {
			t.Fatalf("CONTROL broken harness: Get #%d failed with %v; the pool cannot even "+
				"reach its own MaxSize", i+1, err)
		}
	}
	t.Logf("CONTROL 1 ok: %d concurrent Get() calls all succeeded", maxSize)

	// ---- CONTROL 2: Stats must see the connections that are checked out.
	// This is what makes the defect below observable rather than theoretical.
	st := p.Stats()
	if st.InUse != maxSize {
		t.Logf("CONTROL 2 observed: Stats().InUse = %d with %d connections held "+
			"(Total=%d) — checked-out connections are absent from p.conns",
			st.InUse, maxSize, st.Total)
	} else {
		t.Logf("CONTROL 2 ok: Stats().InUse = %d", st.InUse)
	}

	// ---- THE DEFECT: with MaxSize reached, the next Get must NOT instantly
	// succeed. It should block on notifyCh and eventually return ErrPoolTimeout.
	overflow, err := p.Get()
	if err == nil {
		_ = overflow
		t.Fatalf("FAIL: the pool handed out more than MaxSize connections.\n"+
			"MaxSize = %d, yet a %dth Get returned a connection with NO error while %d were "+
			"already checked out. Pool.Get removes a connection from p.conns when handing it "+
			"out and never appends a factory-created one, so `len(p.conns) < MaxSize` only "+
			"ever counts IDLE connections and the cap is never enforced. The factory created "+
			"%d connection(s).",
			maxSize, maxSize+1, maxSize, f.created.Load())
	}
	if err != ErrPoolTimeout {
		t.Logf("NOTE: the overflow Get returned %v (expected ErrPoolTimeout)", err)
	} else {
		t.Log("PASS: the pool blocked at MaxSize and timed out as configured")
	}

	// ---- BOUNDARY: the pool must not have created more connections than
	// MaxSize while satisfying the first maxSize calls.
	if got := f.created.Load(); got > maxSize {
		t.Fatalf("BOUNDARY FAIL: the factory created %d connections for a pool with "+
			"MaxSize = %d", got, maxSize)
	}
	t.Logf("PASS: factory created %d connection(s) for MaxSize = %d", f.created.Load(), maxSize)
}
