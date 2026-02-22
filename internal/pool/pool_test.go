package pool

import (
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

type mockConn struct {
	closed bool
}

func (m *mockConn) Read(b []byte) (n int, err error)  { return 0, nil }
func (m *mockConn) Write(b []byte) (n int, err error) { return len(b), nil }
func (m *mockConn) Close() error {
	m.closed = true
	return nil
}
func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestPoolConfigDefaults(t *testing.T) {
	tests := []struct {
		name   string
		config PoolConfig
		check  func(PoolConfig)
	}{
		{
			name:   "negative initial size",
			config: PoolConfig{InitialSize: -1},
			check: func(c PoolConfig) {
				if c.InitialSize != 0 {
					t.Errorf("expected 0, got %d", c.InitialSize)
				}
			},
		},
		{
			name:   "zero max size",
			config: PoolConfig{MaxSize: 0},
			check: func(c PoolConfig) {
				if c.MaxSize != 10 {
					t.Errorf("expected 10, got %d", c.MaxSize)
				}
			},
		},
		{
			name:   "zero max idle",
			config: PoolConfig{MaxSize: 20, MaxIdle: 0},
			check: func(c PoolConfig) {
				if c.MaxIdle != 20 {
					t.Errorf("expected 20, got %d", c.MaxIdle)
				}
			},
		},
		{
			name:   "zero idle timeout",
			config: PoolConfig{IdleTimeout: 0},
			check: func(c PoolConfig) {
				if c.IdleTimeout != 5*time.Minute {
					t.Errorf("expected 5m, got %v", c.IdleTimeout)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := func() (net.Conn, error) { return &mockConn{}, nil }
			p := NewPool(tt.config, factory)
			defer p.Close()
			tt.check(p.config)
		})
	}
}

func TestPoolGet(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 5}, factory)
	defer p.Close()

	conn, err := p.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn == nil {
		t.Fatal("expected connection")
	}
	conn.Close()
}

func TestPoolGetClosed(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 5}, factory)
	p.Close()

	_, err := p.Get()
	if err != ErrPoolClosed {
		t.Errorf("expected ErrPoolClosed, got %v", err)
	}
}

func TestPoolGetFactoryError(t *testing.T) {
	factoryErr := errors.New("factory error")
	factory := func() (net.Conn, error) { return nil, factoryErr }
	p := NewPool(PoolConfig{MaxSize: 5, InitialSize: 0}, factory)
	defer p.Close()

	_, err := p.Get()
	if err != factoryErr {
		t.Errorf("expected factory error, got %v", err)
	}
}

func TestPoolRelease(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 5}, factory)
	defer p.Close()

	conn, _ := p.Get()
	if err := conn.Close(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPoolReleaseClosed(t *testing.T) {
	mc := &mockConn{}
	factory := func() (net.Conn, error) { return mc, nil }
	p := NewPool(PoolConfig{MaxSize: 5}, factory)

	conn, _ := p.Get()
	p.Close()

	conn.Close()
	if !mc.closed {
		t.Error("connection should be closed")
	}
}

func TestPoolCloseTwice(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 5}, factory)

	p.Close()
	p.Close()
}

func TestPoolStats(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 5, InitialSize: 2}, factory)
	defer p.Close()

	stats := p.Stats()
	if stats.Total < 0 {
		t.Errorf("unexpected total: %d", stats.Total)
	}
}

func TestPoolConcurrentAccess(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 10}, factory)
	defer p.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := p.Get()
			if err == nil {
				conn.Close()
			}
		}()
	}
	wg.Wait()
}

func TestConnRaw(t *testing.T) {
	mc := &mockConn{}
	factory := func() (net.Conn, error) { return mc, nil }
	p := NewPool(PoolConfig{MaxSize: 5}, factory)
	defer p.Close()

	conn, _ := p.Get()
	raw := conn.Raw()
	if raw != mc {
		t.Error("expected raw connection")
	}
	conn.Close()
}

func TestPoolErrorMessages(t *testing.T) {
	if ErrPoolClosed.Error() != "pool is closed" {
		t.Errorf("unexpected error message: %s", ErrPoolClosed.Error())
	}
	if ErrPoolTimeout.Error() != "pool timeout" {
		t.Errorf("unexpected error message: %s", ErrPoolTimeout.Error())
	}
}

func TestConnWithoutPool(t *testing.T) {
	mc := &mockConn{}
	c := &Conn{conn: mc, pool: nil}

	if err := c.Close(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !mc.closed {
		t.Error("connection should be closed")
	}
}

func TestPoolInitialSize(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 10, InitialSize: 3}, factory)
	defer p.Close()

	stats := p.Stats()
	if stats.Total < 3 {
		t.Errorf("expected at least 3 connections, got %d", stats.Total)
	}
}

func TestPoolInitialSizeWithErrors(t *testing.T) {
	callCount := 0
	factory := func() (net.Conn, error) {
		callCount++
		if callCount <= 2 {
			return nil, errors.New("error")
		}
		return &mockConn{}, nil
	}
	p := NewPool(PoolConfig{MaxSize: 10, InitialSize: 5}, factory)
	defer p.Close()
}

func TestPoolGetReusesIdleConnection(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 5, InitialSize: 0}, factory)
	defer p.Close()

	conn1, _ := p.Get()
	conn1.Close()

	conn2, _ := p.Get()
	conn2.Close()
}

func TestPoolMaxIdleLimit(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 10, MaxIdle: 2}, factory)
	defer p.Close()

	conns := make([]*Conn, 5)
	for i := 0; i < 5; i++ {
		conns[i], _ = p.Get()
	}

	for _, c := range conns {
		c.Close()
	}
}

func TestConnRead(t *testing.T) {
	mc := &mockConn{}
	c := &Conn{conn: mc}

	buf := make([]byte, 10)
	n, err := c.Read(buf)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 bytes, got %d", n)
	}
}

func TestConnWrite(t *testing.T) {
	mc := &mockConn{}
	c := &Conn{conn: mc}

	n, err := c.Write([]byte("test"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 4 {
		t.Errorf("expected 4 bytes, got %d", n)
	}
}

func TestPoolNotifyChannel(t *testing.T) {
	factory := func() (net.Conn, error) { return &mockConn{}, nil }
	p := NewPool(PoolConfig{MaxSize: 1}, factory)
	defer p.Close()

	select {
	case p.notifyCh <- struct{}{}:
	default:
	}
}
