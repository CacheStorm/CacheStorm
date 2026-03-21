package cachestorm

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

// Client is a CacheStorm client
type Client struct {
	addr   string
	pool   *pool
	opts   *Options
	mu     sync.RWMutex
	closed bool
}

// Options contains client options
type Options struct {
	PoolSize     int
	MinIdleConns int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolTimeout  time.Duration
	IdleTimeout  time.Duration
	MaxConnAge   time.Duration
	Password     string
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
	return func(o *Options) { o.PoolSize = size }
}

// WithMinIdleConns sets minimum idle connections
func WithMinIdleConns(n int) Option {
	return func(o *Options) { o.MinIdleConns = n }
}

// WithMaxRetries sets max retries
func WithMaxRetries(n int) Option {
	return func(o *Options) { o.MaxRetries = n }
}

// WithPassword sets the authentication password
func WithPassword(pw string) Option {
	return func(o *Options) { o.Password = pw }
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
	cn, err := c.pool.Get(ctx)
	if err != nil {
		return nil, err
	}
	defer c.pool.Put(cn)

	return cn.Do(ctx, args...)
}

// Ping sends a PING command
func (c *Client) Ping(ctx context.Context) (string, error) {
	val, err := c.Do(ctx, "PING")
	if err != nil {
		return "", err
	}
	return toString(val), nil
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
		return "", nil
	}
	return toString(val), nil
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
	return toInt64(val), nil
}

// Incr increments a key by 1
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	val, err := c.Do(ctx, "INCR", key)
	if err != nil {
		return 0, err
	}
	return toInt64(val), nil
}

// HSet sets hash fields
func (c *Client) HSet(ctx context.Context, key string, values ...interface{}) (int64, error) {
	args := make([]interface{}, 0, 2+len(values))
	args = append(args, "HSET", key)
	args = append(args, values...)
	val, err := c.Do(ctx, args...)
	if err != nil {
		return 0, err
	}
	return toInt64(val), nil
}

// HGet gets a hash field
func (c *Client) HGet(ctx context.Context, key, field string) (string, error) {
	val, err := c.Do(ctx, "HGET", key, field)
	if err != nil {
		return "", err
	}
	if val == nil {
		return "", nil
	}
	return toString(val), nil
}

// HGetAll gets all hash fields
func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	val, err := c.Do(ctx, "HGETALL", key)
	if err != nil {
		return nil, err
	}
	arr, ok := val.([]interface{})
	if !ok || len(arr) == 0 {
		return map[string]string{}, nil
	}
	result := make(map[string]string, len(arr)/2)
	for i := 0; i+1 < len(arr); i += 2 {
		result[toString(arr[i])] = toString(arr[i+1])
	}
	return result, nil
}

// LPush prepends elements to a list
func (c *Client) LPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	args := make([]interface{}, 0, 2+len(values))
	args = append(args, "LPUSH", key)
	args = append(args, values...)
	val, err := c.Do(ctx, args...)
	if err != nil {
		return 0, err
	}
	return toInt64(val), nil
}

// LRange returns a range of elements from a list
func (c *Client) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	val, err := c.Do(ctx, "LRANGE", key, start, stop)
	if err != nil {
		return nil, err
	}
	return toStringSlice(val), nil
}

// LPop removes and returns the first element of a list
func (c *Client) LPop(ctx context.Context, key string) (string, error) {
	val, err := c.Do(ctx, "LPOP", key)
	if err != nil {
		return "", err
	}
	if val == nil {
		return "", nil
	}
	return toString(val), nil
}

// SAdd adds members to a set
func (c *Client) SAdd(ctx context.Context, key string, members ...interface{}) (int64, error) {
	args := make([]interface{}, 0, 2+len(members))
	args = append(args, "SADD", key)
	args = append(args, members...)
	val, err := c.Do(ctx, args...)
	if err != nil {
		return 0, err
	}
	return toInt64(val), nil
}

// SMembers returns all members of a set
func (c *Client) SMembers(ctx context.Context, key string) ([]string, error) {
	val, err := c.Do(ctx, "SMEMBERS", key)
	if err != nil {
		return nil, err
	}
	return toStringSlice(val), nil
}

// SRem removes members from a set
func (c *Client) SRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	args := make([]interface{}, 0, 2+len(members))
	args = append(args, "SREM", key)
	args = append(args, members...)
	val, err := c.Do(ctx, args...)
	if err != nil {
		return 0, err
	}
	return toInt64(val), nil
}

// ZAdd adds members to a sorted set
func (c *Client) ZAdd(ctx context.Context, key string, members ...interface{}) (int64, error) {
	args := make([]interface{}, 0, 2+len(members))
	args = append(args, "ZADD", key)
	args = append(args, members...)
	val, err := c.Do(ctx, args...)
	if err != nil {
		return 0, err
	}
	return toInt64(val), nil
}

// ZIncrBy increments the score of a member in a sorted set
func (c *Client) ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error) {
	val, err := c.Do(ctx, "ZINCRBY", key, increment, member)
	if err != nil {
		return 0, err
	}
	return toFloat64(val), nil
}

// ZRevRange returns members in reverse order by score
func (c *Client) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	val, err := c.Do(ctx, "ZREVRANGE", key, start, stop)
	if err != nil {
		return nil, err
	}
	return toStringSlice(val), nil
}

// Expire sets a timeout on a key
func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	val, err := c.Do(ctx, "EXPIRE", key, int64(expiration.Seconds()))
	if err != nil {
		return false, err
	}
	return toInt64(val) == 1, nil
}

// XAdd adds to a stream
func (c *Client) XAdd(ctx context.Context, args ...interface{}) (string, error) {
	cmdArgs := make([]interface{}, 0, 1+len(args))
	cmdArgs = append(cmdArgs, "XADD")
	cmdArgs = append(cmdArgs, args...)
	val, err := c.Do(ctx, cmdArgs...)
	if err != nil {
		return "", err
	}
	return toString(val), nil
}

// XRange returns entries from a stream
func (c *Client) XRange(ctx context.Context, key, start, end string) ([]interface{}, error) {
	val, err := c.Do(ctx, "XRANGE", key, start, end)
	if err != nil {
		return nil, err
	}
	if arr, ok := val.([]interface{}); ok {
		return arr, nil
	}
	return nil, nil
}

// XRevRange returns entries from a stream in reverse order
func (c *Client) XRevRange(ctx context.Context, key, end, start string) ([]interface{}, error) {
	val, err := c.Do(ctx, "XREVRANGE", key, end, start)
	if err != nil {
		return nil, err
	}
	if arr, ok := val.([]interface{}); ok {
		return arr, nil
	}
	return nil, nil
}

// XTrim trims a stream
func (c *Client) XTrim(ctx context.Context, key string, maxLen int64) (int64, error) {
	val, err := c.Do(ctx, "XTRIM", key, "MAXLEN", maxLen)
	if err != nil {
		return 0, err
	}
	return toInt64(val), nil
}

// LTrim trims a list
func (c *Client) LTrim(ctx context.Context, key string, start, stop int64) error {
	_, err := c.Do(ctx, "LTRIM", key, start, stop)
	return err
}

// HIncrBy increments a hash field by an integer
func (c *Client) HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error) {
	val, err := c.Do(ctx, "HINCRBY", key, field, incr)
	if err != nil {
		return 0, err
	}
	return toInt64(val), nil
}

// PFAdd adds elements to a HyperLogLog
func (c *Client) PFAdd(ctx context.Context, key string, elements ...interface{}) (int64, error) {
	args := make([]interface{}, 0, 2+len(elements))
	args = append(args, "PFADD", key)
	args = append(args, elements...)
	val, err := c.Do(ctx, args...)
	if err != nil {
		return 0, err
	}
	return toInt64(val), nil
}

// PFCount returns the approximate cardinality of a HyperLogLog
func (c *Client) PFCount(ctx context.Context, keys ...string) (int64, error) {
	args := make([]interface{}, 1+len(keys))
	args[0] = "PFCOUNT"
	for i, key := range keys {
		args[i+1] = key
	}
	val, err := c.Do(ctx, args...)
	if err != nil {
		return 0, err
	}
	return toInt64(val), nil
}

// SCard returns the number of members in a set
func (c *Client) SCard(ctx context.Context, key string) (int64, error) {
	val, err := c.Do(ctx, "SCARD", key)
	if err != nil {
		return 0, err
	}
	return toInt64(val), nil
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
	return toStringSlice(val), nil
}

// Pipeline creates a new pipeline
func (c *Client) Pipeline() *Pipeline {
	return &Pipeline{client: c}
}

// Subscribe subscribes to channels
func (c *Client) Subscribe(ctx context.Context, channels ...string) *PubSub {
	return newPubSub(c, channels...)
}

// --- Connection Pool ---

type pool struct {
	addr   string
	opts   *Options
	conns  chan *conn
	mu     sync.Mutex
	closed bool
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
	case cn := <-p.conns:
		if cn.closed {
			return p.newConn()
		}
		return cn, nil
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

	cn := &conn{
		netConn: netConn,
		reader:  bufio.NewReaderSize(netConn, 4096),
		opts:    p.opts,
	}

	// Authenticate if password set
	if p.opts.Password != "" {
		_, err := cn.Do(context.Background(), "AUTH", p.opts.Password)
		if err != nil {
			netConn.Close()
			return nil, fmt.Errorf("auth failed: %w", err)
		}
	}

	return cn, nil
}

// --- Connection with RESP Protocol ---

type conn struct {
	netConn net.Conn
	reader  *bufio.Reader
	mu      sync.Mutex
	closed  bool
	opts    *Options
}

func (c *conn) Do(ctx context.Context, args ...interface{}) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, errors.New("connection closed")
	}

	// Set deadlines
	if c.opts != nil && c.opts.WriteTimeout > 0 {
		c.netConn.SetWriteDeadline(time.Now().Add(c.opts.WriteTimeout))
	}

	// Write RESP array
	if err := c.writeCommand(args); err != nil {
		c.closed = true
		return nil, err
	}

	// Set read deadline
	if c.opts != nil && c.opts.ReadTimeout > 0 {
		c.netConn.SetReadDeadline(time.Now().Add(c.opts.ReadTimeout))
	}

	// Read RESP response
	return c.readResponse()
}

func (c *conn) writeCommand(args []interface{}) error {
	// Write RESP array header: *<count>\r\n
	buf := make([]byte, 0, 256)
	buf = append(buf, '*')
	buf = strconv.AppendInt(buf, int64(len(args)), 10)
	buf = append(buf, '\r', '\n')

	for _, arg := range args {
		s := fmt.Sprintf("%v", arg)
		buf = append(buf, '$')
		buf = strconv.AppendInt(buf, int64(len(s)), 10)
		buf = append(buf, '\r', '\n')
		buf = append(buf, s...)
		buf = append(buf, '\r', '\n')
	}

	_, err := c.netConn.Write(buf)
	return err
}

func (c *conn) readResponse() (interface{}, error) {
	b, err := c.reader.ReadByte()
	if err != nil {
		return nil, err
	}

	switch b {
	case '+': // Simple string
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		return string(line), nil

	case '-': // Error
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		return nil, errors.New(string(line))

	case ':': // Integer
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		n, err := strconv.ParseInt(string(line), 10, 64)
		if err != nil {
			return nil, err
		}
		return n, nil

	case '$': // Bulk string
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		size, err := strconv.ParseInt(string(line), 10, 64)
		if err != nil {
			return nil, err
		}
		if size == -1 {
			return nil, nil
		}
		buf := make([]byte, size)
		if _, err := io.ReadFull(c.reader, buf); err != nil {
			return nil, err
		}
		// Read trailing \r\n
		c.reader.ReadByte()
		c.reader.ReadByte()
		return string(buf), nil

	case '*': // Array
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		count, err := strconv.ParseInt(string(line), 10, 64)
		if err != nil {
			return nil, err
		}
		if count == -1 {
			return nil, nil
		}
		arr := make([]interface{}, count)
		for i := int64(0); i < count; i++ {
			arr[i], err = c.readResponse()
			if err != nil {
				return nil, err
			}
		}
		return arr, nil

	default:
		return nil, fmt.Errorf("unknown RESP type: %c", b)
	}
}

func (c *conn) readLine() ([]byte, error) {
	line, err := c.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	if len(line) >= 2 && line[len(line)-2] == '\r' {
		return line[:len(line)-2], nil
	}
	return line, nil
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

// --- Pipeline ---

type Pipeline struct {
	client *Client
	cmds   []Cmd
}

type Cmd struct {
	Args []interface{}
	Err  error
	Val  interface{}
}

func (p *Pipeline) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) {
	args := []interface{}{"SET", key, value}
	if expiration > 0 {
		args = append(args, "EX", int64(expiration.Seconds()))
	}
	p.cmds = append(p.cmds, Cmd{Args: args})
}

func (p *Pipeline) Get(ctx context.Context, key string) {
	p.cmds = append(p.cmds, Cmd{Args: []interface{}{"GET", key}})
}

func (p *Pipeline) Exec(ctx context.Context) ([]Cmd, error) {
	for i := range p.cmds {
		p.cmds[i].Val, p.cmds[i].Err = p.client.Do(ctx, p.cmds[i].Args...)
	}
	return p.cmds, nil
}

// --- PubSub ---

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

type Message struct {
	Channel string
	Pattern string
	Payload string
}

func (ps *PubSub) ReceiveMessage(ctx context.Context) (*Message, error) {
	return nil, errors.New("not implemented")
}

func (ps *PubSub) Close() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.closed = true
	return nil
}

// --- Helpers ---

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case int64:
		return strconv.FormatInt(val, 10)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func toInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int64:
		return val
	case string:
		n, _ := strconv.ParseInt(val, 10, 64)
		return n
	default:
		return 0
	}
}

func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case int64:
		return float64(val)
	default:
		return 0
	}
}

func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, len(arr))
	for i, item := range arr {
		result[i] = toString(item)
	}
	return result
}
