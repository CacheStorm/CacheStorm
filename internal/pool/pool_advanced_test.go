package pool

import (
	"net"
	"testing"
	"time"
)

func TestPoolAdvanced(t *testing.T) {
	t.Run("Pool Creation", func(t *testing.T) {
		factory := func() (net.Conn, error) {
			return &net.TCPConn{}, nil
		}
		config := PoolConfig{
			MaxSize:     10,
			InitialSize: 2,
			MaxIdle:     5,
			IdleTimeout: 2 * time.Minute,
		}
		p := NewPool(config, factory)
		if p == nil {
			t.Fatal("NewPool returned nil")
		}
		defer p.Close()
	})

	t.Run("Pool Stats", func(t *testing.T) {
		factory := func() (net.Conn, error) {
			return &net.TCPConn{}, nil
		}
		config := PoolConfig{MaxSize: 10, InitialSize: 0}
		p := NewPool(config, factory)
		defer p.Close()

		stats := p.Stats()
		// Stats should be valid
		_ = stats.Total
		_ = stats.InUse
		_ = stats.Idle
		_ = stats.Waiting
	})
}
