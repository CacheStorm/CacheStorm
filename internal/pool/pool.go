package pool

import (
	"log"
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
	if config.InitialSize > config.MaxSize {
		config.InitialSize = config.MaxSize
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
			lastUsed:  time.Now(),
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
			if time.Since(c.lastUsed) > p.config.IdleTimeout {
				if err := c.conn.Close(); err != nil {
					log.Printf("pool: error closing idle connection: %v", err)
				}
				p.conns = append(p.conns[:i], p.conns[i+1:]...)
				continue
			}

			// Claimed under the lock and kept in p.conns: removing it would
			// make len(p.conns) count only idle conns, so MaxSize is never enforced.
			c.inUse.Store(true)
			c.lastUsed = time.Now()
			p.mu.Unlock()
			return c, nil
		}
	}

	if len(p.conns) < p.config.MaxSize {
		conn, err := p.factory()
		if err != nil {
			p.mu.Unlock()
			return nil, err
		}

		c := &Conn{
			conn:      conn,
			pool:      p,
			createdAt: time.Now(),
			lastUsed:  time.Now(),
		}
		c.inUse.Store(true)
		p.conns = append(p.conns, c)
		p.mu.Unlock()
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

	p.mu.Lock()
	// The connection stays in p.conns while it is checked out, so Release
	// marks it idle rather than re-appending it — re-appending would duplicate
	// the entry and inflate len(p.conns) past the real connection count.
	found := false
	for _, existing := range p.conns {
		if existing == c {
			found = true
			break
		}
	}
	if !found {
		p.mu.Unlock()
		return c.conn.Close()
	}
	if !c.inUse.Load() {
		p.mu.Unlock()
		return nil
	}

	// c is still in-use here, so releasing it makes the idle count idle+1.
	idle := 0
	for _, existing := range p.conns {
		if !existing.inUse.Load() {
			idle++
		}
	}
	if idle >= p.config.MaxIdle {
		for i, existing := range p.conns {
			if existing == c {
				p.conns = append(p.conns[:i], p.conns[i+1:]...)
				break
			}
		}
		c.inUse.Store(false)
		p.mu.Unlock()
		return c.conn.Close()
	}

	c.inUse.Store(false)
	c.lastUsed = time.Now()
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
		if err := c.conn.Close(); err != nil {
			log.Printf("pool: error closing connection: %v", err)
		}
	}
	p.conns = nil
	p.mu.Unlock()
}

func (p *Pool) cleanup() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("pool: panic recovered in cleanup: %v", r)
		}
	}()
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
				if err := c.conn.Close(); err != nil {
					log.Printf("pool: error closing idle connection during cleanup: %v", err)
				}
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
