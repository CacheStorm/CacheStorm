package pool

import (
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type PoolConfig struct {
	InitialSize int
	MaxSize     int
	MaxIdle     int
	IdleTimeout time.Duration
}

type Conn struct {
	conn      net.Conn
	pool      *Pool
	createdAt time.Time
	lastUsed  time.Time
	inUse     atomic.Bool
}

func (c *Conn) Read(b []byte) (n int, err error) {
	return c.conn.Read(b)
}

func (c *Conn) Write(b []byte) (n int, err error) {
	return c.conn.Write(b)
}

func (c *Conn) Close() error {
	if c.pool != nil {
		return c.pool.Release(c)
	}
	return c.conn.Close()
}

func (c *Conn) Raw() net.Conn {
	return c.conn
}

type Pool struct {
	mu       sync.Mutex
	conns    []*Conn
	config   PoolConfig
	factory  func() (net.Conn, error)
	closed   atomic.Bool
	waiting  int32
	notifyCh chan struct{}
}

func NewPool(config PoolConfig, factory func() (net.Conn, error)) *Pool {
	if config.InitialSize < 0 {
		config.InitialSize = 0
	}
	if config.MaxSize <= 0 {
		config.MaxSize = 10
	}
	if config.MaxIdle <= 0 {
		config.MaxIdle = config.MaxSize
	}
	if config.IdleTimeout <= 0 {
		config.IdleTimeout = 5 * time.Minute
	}

	p := &Pool{
		config:   config,
		factory:  factory,
		conns:    make([]*Conn, 0, config.MaxSize),
		notifyCh: make(chan struct{}, 1),
	}

	for i := 0; i < config.InitialSize; i++ {
		conn, err := factory()
		if err != nil {
			continue
		}
		p.conns = append(p.conns, &Conn{
			conn:      conn,
			pool:      p,
			createdAt: time.Now(),
		})
	}

	go p.cleanup()

	return p
}

func (p *Pool) Get() (*Conn, error) {
	if p.closed.Load() {
		return nil, ErrPoolClosed
	}

	p.mu.Lock()

	for i := len(p.conns) - 1; i >= 0; i-- {
		c := p.conns[i]
		if !c.inUse.Load() {
			p.conns = append(p.conns[:i], p.conns[i+1:]...)
			p.mu.Unlock()

			if time.Since(c.lastUsed) > p.config.IdleTimeout {
				c.conn.Close()
				continue
			}

			c.inUse.Store(true)
			c.lastUsed = time.Now()
			return c, nil
		}
	}

	if len(p.conns) < p.config.MaxSize {
		conn, err := p.factory()
		if err != nil {
			p.mu.Unlock()
			return nil, err
		}
		p.mu.Unlock()

		c := &Conn{
			conn:      conn,
			pool:      p,
			createdAt: time.Now(),
			lastUsed:  time.Now(),
		}
		c.inUse.Store(true)
		return c, nil
	}

	p.mu.Unlock()

	atomic.AddInt32(&p.waiting, 1)
	defer atomic.AddInt32(&p.waiting, -1)

	select {
	case <-p.notifyCh:
		return p.Get()
	case <-time.After(5 * time.Second):
		return nil, ErrPoolTimeout
	}
}

func (p *Pool) Release(c *Conn) error {
	if p.closed.Load() {
		return c.conn.Close()
	}

	c.inUse.Store(false)
	c.lastUsed = time.Now()

	p.mu.Lock()
	if len(p.conns) < p.config.MaxIdle {
		p.conns = append(p.conns, c)
	} else {
		p.mu.Unlock()
		return c.conn.Close()
	}
	p.mu.Unlock()

	select {
	case p.notifyCh <- struct{}{}:
	default:
	}

	return nil
}

func (p *Pool) Close() {
	if !p.closed.CompareAndSwap(false, true) {
		return
	}

	p.mu.Lock()
	for _, c := range p.conns {
		c.conn.Close()
	}
	p.conns = nil
	p.mu.Unlock()
}

func (p *Pool) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if p.closed.Load() {
			return
		}

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
	}
}

func (p *Pool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	inUse := 0
	idle := 0
	for _, c := range p.conns {
		if c.inUse.Load() {
			inUse++
		} else {
			idle++
		}
	}

	return PoolStats{
		Total:   len(p.conns),
		InUse:   inUse,
		Idle:    idle,
		Waiting: int(atomic.LoadInt32(&p.waiting)),
	}
}

type PoolStats struct {
	Total   int
	InUse   int
	Idle    int
	Waiting int
}

var (
	ErrPoolClosed  = &PoolError{msg: "pool is closed"}
	ErrPoolTimeout = &PoolError{msg: "pool timeout"}
)

type PoolError struct {
	msg string
}

func (e *PoolError) Error() string {
	return e.msg
}
