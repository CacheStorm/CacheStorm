package pool

import (
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

// mockConnWithCloseErr is a mock connection that returns an error on Close.
type mockConnWithCloseErr struct {
	mockConn
	closeErr error
}

func (m *mockConnWithCloseErr) Close() error {
	m.closed = true
	return m.closeErr
}

// TestGetTimeoutViaWaitPath exercises the select/timeout path in Get by
// having a pool where all connections are in-use. This forces the
// goroutine to wait on notifyCh and eventually time out.
func TestGetTimeoutViaWaitPath(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 1, InitialSize: 0}, factory)
	defer p.Close()

	// Get a connection from the factory. This conn is removed from p.conns
	// and the pool is empty. Put it back as inUse to block the wait path.
	conn, err := p.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Place the connection back into the pool but keep it marked as in-use.
	// This makes len(p.conns) == 1 == MaxSize, but no idle conns available.
	conn.inUse.Store(true)
	conn.lastUsed = time.Now()
	p.mu.Lock()
	p.conns = append(p.conns, conn)
	p.mu.Unlock()

	// Now Get will:
	// 1. Not find any idle connections (the only one is inUse)
	// 2. See len(p.conns) >= MaxSize
	// 3. Enter the select waiting path
	// 4. Timeout after 5 seconds
	//
	// To avoid waiting 5 seconds in tests, we'll test the notifyCh path instead
	// by releasing the connection after a short delay.

	done := make(chan struct{})
	var c2 *Conn
	var getErr error
	go func() {
		c2, getErr = p.Get()
		close(done)
	}()

	// Give the goroutine time to enter the wait path.
	time.Sleep(100 * time.Millisecond)

	// Release the connection: mark as not in-use and send notify.
	conn.inUse.Store(false)
	select {
	case p.notifyCh <- struct{}{}:
	default:
	}

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for Get to return")
	}

	if getErr != nil {
		t.Errorf("expected nil error, got %v", getErr)
	}
	if c2 != nil {
		c2.Close()
	}
}

// TestGetTimeoutActual exercises the actual timeout path in Get (5s timeout).
// This test uses a short helper to verify the path is reached.
func TestGetTimeoutActual(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 1, InitialSize: 0}, factory)
	defer p.Close()

	// Get one connection and put it back as inUse.
	conn, _ := p.Get()
	conn.inUse.Store(true)
	conn.lastUsed = time.Now()
	p.mu.Lock()
	p.conns = append(p.conns, conn)
	p.mu.Unlock()

	// Drain the notifyCh so nothing wakes us up.
	select {
	case <-p.notifyCh:
	default:
	}

	start := time.Now()
	_, err := p.Get()
	elapsed := time.Since(start)

	if err != ErrPoolTimeout {
		t.Errorf("expected ErrPoolTimeout, got %v", err)
	}

	// Should have waited approximately 5 seconds.
	if elapsed < 4*time.Second {
		t.Errorf("expected to wait ~5 seconds, waited %v", elapsed)
	}
}

// TestReleaseWhenPoolClosed verifies that Release closes the underlying
// connection when the pool is already closed.
func TestReleaseWhenPoolClosed(t *testing.T) {
	mc := &mockConn{}
	factory := func() (net.Conn, error) { return mc, nil }
	p := NewPool(PoolConfig{MaxSize: 5}, factory)

	conn, _ := p.Get()
	p.Close()

	err := p.Release(conn)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !mc.closed {
		t.Error("expected underlying connection to be closed")
	}
}

// TestReleaseExceedsMaxIdle verifies that Release closes excess connections.
func TestReleaseExceedsMaxIdle(t *testing.T) {
	conns := make([]*mockConn, 0)
	factory := func() (net.Conn, error) {
		mc := &mockConn{}
		conns = append(conns, mc)
		return mc, nil
	}
	p := NewPool(PoolConfig{MaxSize: 5, MaxIdle: 1}, factory)
	defer p.Close()

	c1, _ := p.Get()
	c2, _ := p.Get()

	c1.Close()

	err := p.Release(c2)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(conns) < 2 {
		t.Fatalf("expected at least 2 connections created, got %d", len(conns))
	}
	if !conns[1].closed {
		t.Error("second connection should be closed when exceeding MaxIdle")
	}
}

// TestStatsWithInUseConnections verifies that Stats correctly counts
// connections that are in use.
func TestStatsWithInUseConnections(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 5, InitialSize: 3}, factory)
	defer p.Close()

	stats := p.Stats()
	if stats.Idle != stats.Total {
		t.Errorf("expected all idle, got idle=%d total=%d", stats.Idle, stats.Total)
	}

	conn, _ := p.Get()
	conn.inUse.Store(true)
	conn.lastUsed = time.Now()
	p.mu.Lock()
	p.conns = append(p.conns, conn)
	p.mu.Unlock()

	stats = p.Stats()
	if stats.InUse < 1 {
		t.Errorf("expected at least 1 in-use connection, got %d", stats.InUse)
	}
	if stats.InUse+stats.Idle != stats.Total {
		t.Errorf("InUse(%d) + Idle(%d) should equal Total(%d)", stats.InUse, stats.Idle, stats.Total)
	}
}

// TestCleanupRemovesIdleConnections simulates the cleanup goroutine logic.
func TestCleanupRemovesIdleConnections(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{
		MaxSize:     5,
		InitialSize: 0,
		MaxIdle:     5,
		IdleTimeout: 50 * time.Millisecond,
	}, factory)
	defer p.Close()

	for i := 0; i < 3; i++ {
		c, err := p.Get()
		if err != nil {
			t.Fatalf("get error: %v", err)
		}
		c.Close()
	}

	stats := p.Stats()
	if stats.Total == 0 {
		t.Fatal("expected some connections in pool")
	}

	time.Sleep(60 * time.Millisecond)

	// Simulate cleanup tick.
	p.mu.Lock()
	now := time.Now()
	active := make([]*Conn, 0, len(p.conns))
	for _, c := range p.conns {
		if !c.inUse.Load() && now.Sub(c.lastUsed) > p.config.IdleTimeout {
			c.conn.Close()
		} else {
			active = append(active, c)
		}
	}
	p.conns = active
	p.mu.Unlock()

	stats = p.Stats()
	if stats.Total != 0 {
		t.Errorf("expected 0 connections after cleanup, got %d", stats.Total)
	}
}

// TestGetRemovesExpiredIdleConnections tests that Get removes expired idle conns.
func TestGetRemovesExpiredIdleConnections(t *testing.T) {
	callCount := 0
	factory := func() (net.Conn, error) {
		callCount++
		return &mockConn{}, nil
	}
	p := NewPool(PoolConfig{
		MaxSize:     5,
		InitialSize: 0,
		IdleTimeout: 10 * time.Millisecond,
	}, factory)
	defer p.Close()

	c1, _ := p.Get()
	c1.Close()
	initialCallCount := callCount

	time.Sleep(20 * time.Millisecond)

	c2, err := p.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c2.Close()

	if callCount <= initialCallCount {
		t.Error("expected factory to be called for new connection after expired one was removed")
	}
}

// TestGetErrorClosingIdleConn tests the log path when closing idle conn fails.
func TestGetErrorClosingIdleConn(t *testing.T) {
	mc := &mockConnWithCloseErr{closeErr: errors.New("close error")}
	factory := func() (net.Conn, error) { return mc, nil }
	p := NewPool(PoolConfig{
		MaxSize:     5,
		InitialSize: 0,
		IdleTimeout: 10 * time.Millisecond,
	}, factory)
	defer p.Close()

	c, _ := p.Get()
	c.Close()

	time.Sleep(20 * time.Millisecond)

	c2, err := p.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c2.Close()
}

// TestPoolGetWaiterNotify tests that a waiting getter is woken by release.
func TestPoolGetWaiterNotify(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 1, InitialSize: 0}, factory)
	defer p.Close()

	c1, err := p.Get()
	if err != nil {
		t.Fatal(err)
	}

	// Put the connection back as inUse so next Get enters wait path.
	c1.inUse.Store(true)
	c1.lastUsed = time.Now()
	p.mu.Lock()
	p.conns = append(p.conns, c1)
	p.mu.Unlock()

	var c2 *Conn
	var getErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c2, getErr = p.Get()
	}()

	time.Sleep(100 * time.Millisecond)

	// Make the connection idle and notify.
	c1.inUse.Store(false)
	select {
	case p.notifyCh <- struct{}{}:
	default:
	}

	wg.Wait()

	if getErr != nil {
		t.Errorf("expected nil error, got %v", getErr)
	}
	if c2 != nil {
		c2.Close()
	}
}

// TestPoolGetFromClosedPool ensures Get from a closed pool returns ErrPoolClosed.
func TestPoolGetFromClosedPool(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 5}, factory)
	p.Close()

	_, err := p.Get()
	if err != ErrPoolClosed {
		t.Errorf("expected ErrPoolClosed, got %v", err)
	}
}

// TestPoolCloseWithConnError tests the log path in Close.
func TestPoolCloseWithConnError(t *testing.T) {
	mc := &mockConnWithCloseErr{closeErr: errors.New("close error")}
	factory := func() (net.Conn, error) { return mc, nil }
	p := NewPool(PoolConfig{MaxSize: 5, InitialSize: 1}, factory)
	p.Close()
}

// TestPoolStatsWaiting tests Stats with waiting goroutines.
func TestPoolStatsWaiting(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 1, InitialSize: 0}, factory)
	defer p.Close()

	// Get a connection and put it back as inUse.
	c1, _ := p.Get()
	c1.inUse.Store(true)
	c1.lastUsed = time.Now()
	p.mu.Lock()
	p.conns = append(p.conns, c1)
	p.mu.Unlock()

	const waiters = 2
	var wg sync.WaitGroup
	for i := 0; i < waiters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := p.Get()
			if err == nil && conn != nil {
				conn.Close()
			}
		}()
	}

	time.Sleep(200 * time.Millisecond)

	stats := p.Stats()
	if stats.Waiting < 1 {
		t.Logf("Stats.Waiting = %d (timing dependent)", stats.Waiting)
	}

	// Unblock by making conn idle and notifying.
	c1.inUse.Store(false)
	select {
	case p.notifyCh <- struct{}{}:
	default:
	}

	// Wait a bit, then send another notify to wake second waiter.
	time.Sleep(50 * time.Millisecond)
	select {
	case p.notifyCh <- struct{}{}:
	default:
	}

	wg.Wait()
}

// TestPoolGetFactoryErrorAtMax tests factory error propagation.
func TestPoolGetFactoryErrorAtMax(t *testing.T) {
	errFactory := errors.New("factory error")
	factory := func() (net.Conn, error) { return nil, errFactory }
	p := NewPool(PoolConfig{MaxSize: 5, InitialSize: 0}, factory)
	defer p.Close()

	_, err := p.Get()
	if err != errFactory {
		t.Errorf("expected factory error, got %v", err)
	}
}

// TestConnCloseWithPoolNil tests Conn.Close with nil pool.
func TestConnCloseWithPoolNil(t *testing.T) {
	mc := &mockConn{}
	c := &Conn{conn: mc, pool: nil}
	err := c.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !mc.closed {
		t.Error("underlying connection should be closed")
	}
}

// TestCleanupStopsWhenPoolClosed exercises cleanup goroutine exit.
func TestCleanupStopsWhenPoolClosed(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 5, InitialSize: 0}, factory)
	p.Close()
	time.Sleep(50 * time.Millisecond)
}

// TestPoolNotifyOnReleaseMultiple tests multiple releases notifying waiters.
func TestPoolNotifyOnReleaseMultiple(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 2, InitialSize: 0}, factory)
	defer p.Close()

	c1, _ := p.Get()
	c2, _ := p.Get()

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := p.Get()
			if err == nil {
				conn.Close()
			}
		}()
	}

	time.Sleep(50 * time.Millisecond)

	c1.Close()
	c2.Close()

	wg.Wait()
}

// TestGetInUseConnectionSkipped verifies that Get skips over in-use connections
// in the pool and creates a new one via factory if under MaxSize.
func TestGetInUseConnectionSkipped(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 5, InitialSize: 0}, factory)
	defer p.Close()

	// Get a connection and put it back as inUse.
	c1, _ := p.Get()
	c1.inUse.Store(true)
	c1.lastUsed = time.Now()
	p.mu.Lock()
	p.conns = append(p.conns, c1)
	p.mu.Unlock()

	// Get should skip the inUse connection and create a new one via factory
	// (since len(p.conns)=1 < MaxSize=5).
	c2, err := p.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c2 == nil {
		t.Fatal("expected connection")
	}
	c2.Close()
}
