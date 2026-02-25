package cachestorm

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// Client is a CacheStorm client
type Client struct {
	addr     string
	pool     *pool
	opts     *Options
	mu       sync.RWMutex
	closed   bool
}

// Options contains client options
type Options struct {
	PoolSize        int
	MinIdleConns    int
	MaxRetries      int
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	PoolTimeout     time.Duration
	IdleTimeout     time.Duration
	MaxConnAge      time.Duration
}

// DefaultOptions returns default options
func DefaultOptions() *Options {
	return &Options{
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
		IdleTimeout:  5 * time.Minute,
		MaxConnAge:   30 * time.Minute,
	}
}

// Option is a client option function
type Option func(*Options)

// WithPoolSize sets pool size
func WithPoolSize(size int) Option {
	return func(o *Options) {
		o.PoolSize = size
	}
}

// WithMinIdleConns sets minimum idle connections
func WithMinIdleConns(n int) Option {
	return func(o *Options) {
		o.MinIdleConns = n
	}
}

// WithMaxRetries sets max retries
func WithMaxRetries(n int) Option {
	return func(o *Options) {
		o.MaxRetries = n
	}
}

// NewClient creates a new CacheStorm client
func NewClient(addr string, opts ...Option) (*Client, error) {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	c := &Client{
		addr: addr,
		opts: options,
	}

	c.pool = newPool(addr, options)
	return c, nil
}

// Close closes the client
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.pool.Close()
}

// Do executes a raw command
func (c *Client) Do(ctx context.Context, args ...interface{}) (interface{}, error) {
	conn, err := c.pool.Get(ctx)
	if err != nil {
		return nil, err
	}
	defer c.pool.Put(conn)

	return conn.Do(ctx, args...)
}

// Set sets a key-value pair
func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	args := []interface{}{"SET", key, value}
	if expiration > 0 {
		args = append(args, "EX", int64(expiration.Seconds()))
	}

	_, err := c.Do(ctx, args...)
	return err
}

// SetWithTags sets a key with tags for invalidation
func (c *Client) SetWithTags(ctx context.Context, key string, value interface{}, tags []string) error {
	args := []interface{}{"SET", key, value}
	if len(tags) > 0 {
		args = append(args, "TAGS")
		for _, tag := range tags {
			args = append(args, tag)
		}
	}

	_, err := c.Do(ctx, args...)
	return err
}

// Get gets a value by key
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	val, err := c.Do(ctx, "GET", key)
	if err != nil {
		return "", err
	}
	if val == nil {
		return "", fmt.Errorf("key not found")
	}
	return val.(string), nil
}

// Del deletes keys
func (c *Client) Del(ctx context.Context, keys ...string) (int64, error) {
	args := make([]interface{}, len(keys)+1)
	args[0] = "DEL"
	for i, key := range keys {
		args[i+1] = key
	}

	val, err := c.Do(ctx, args...)
	if err != nil {
		return 0, err
	}
	return val.(int64), nil
}

// Invalidate invalidates keys by tag
func (c *Client) Invalidate(ctx context.Context, tag string) error {
	_, err := c.Do(ctx, "INVALIDATE", tag)
	return err
}

// TagKeys gets all keys with a tag
func (c *Client) TagKeys(ctx context.Context, tag string) ([]string, error) {
	val, err := c.Do(ctx, "TAGKEYS", tag)
	if err != nil {
		return nil, err
	}

	// Parse response
	if arr, ok := val.([]interface{}); ok {
		keys := make([]string, len(arr))
		for i, v := range arr {
			keys[i] = v.(string)
		}
		return keys, nil
	}
	return nil, fmt.Errorf("unexpected response type")
}

// Pipeline creates a new pipeline
func (c *Client) Pipeline() *Pipeline {
	return &Pipeline{client: c}
}

// Subscribe subscribes to channels
func (c *Client) Subscribe(ctx context.Context, channels ...string) *PubSub {
	return newPubSub(c, channels...)
}

// pool is a connection pool
type pool struct {
	addr     string
	opts     *Options
	conns    chan *conn
	mu       sync.Mutex
	closed   bool
}

func newPool(addr string, opts *Options) *pool {
	return &pool{
		addr:  addr,
		opts:  opts,
		conns: make(chan *conn, opts.PoolSize),
	}
}

func (p *pool) Get(ctx context.Context) (*conn, error) {
	select {
	case conn := <-p.conns:
		return conn, nil
	default:
		return p.newConn()
	}
}

func (p *pool) Put(c *conn) {
	if c == nil || c.closed {
		return
	}

	select {
	case p.conns <- c:
	default:
		c.Close()
	}
}

func (p *pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	close(p.conns)

	for c := range p.conns {
		c.Close()
	}

	return nil
}

func (p *pool) newConn() (*conn, error) {
	netConn, err := net.DialTimeout("tcp", p.addr, p.opts.DialTimeout)
	if err != nil {
		return nil, err
	}

	return &conn{
		netConn: netConn,
		closed:  false,
	}, nil
}

// conn is a connection
type conn struct {
	netConn net.Conn
	mu      sync.Mutex
	closed  bool
}

func (c *conn) Do(ctx context.Context, args ...interface{}) (interface{}, error) {
	// Simplified RESP protocol implementation
	// In production, use proper RESP encoding
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, fmt.Errorf("connection closed")
	}

	// TODO: Implement proper RESP protocol
	return "OK", nil
}

func (c *conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.netConn.Close()
}

// Pipeline for batching commands
type Pipeline struct {
	client *Client
	cmds   []Cmd
}

// Cmd represents a command
type Cmd struct {
	Args []interface{}
	Err  error
	Val  interface{}
}

// Set adds SET command to pipeline
func (p *Pipeline) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) {
	args := []interface{}{"SET", key, value}
	if expiration > 0 {
		args = append(args, "EX", int64(expiration.Seconds()))
	}
	p.cmds = append(p.cmds, Cmd{Args: args})
}

// Get adds GET command to pipeline
func (p *Pipeline) Get(ctx context.Context, key string) {
	p.cmds = append(p.cmds, Cmd{Args: []interface{}{"GET", key}})
}

// Exec executes all commands in pipeline
func (p *Pipeline) Exec(ctx context.Context) ([]Cmd, error) {
	// TODO: Implement pipeline execution
	return p.cmds, nil
}

// PubSub for pub/sub functionality
type PubSub struct {
	client   *Client
	channels []string
	closed   bool
	mu       sync.Mutex
}

func newPubSub(client *Client, channels ...string) *PubSub {
	return &PubSub{
		client:   client,
		channels: channels,
	}
}

// ReceiveMessage receives a message
func (ps *PubSub) ReceiveMessage(ctx context.Context) (*Message, error) {
	// TODO: Implement pub/sub
	return nil, fmt.Errorf("not implemented")
}

// Close closes pubsub
func (ps *PubSub) Close() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.closed = true
	return nil
}

// Message represents a pub/sub message
type Message struct {
	Channel string
	Pattern string
	Payload string
}
