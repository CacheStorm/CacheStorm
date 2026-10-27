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
	"sync/atomic"
	"time"
)

// ErrClosed is returned by every operation on a closed Client or PubSub.
var ErrClosed = errors.New("cachestorm: client is closed")

// ErrPoolTimeout is returned when no pooled connection became available in time.
var ErrPoolTimeout = errors.New("cachestorm: connection pool timeout")

// ErrProtocol is wrapped by errors caused by malformed server replies.
var ErrProtocol = errors.New("cachestorm: protocol error")

const maxBulkLen = 512 * 1024 * 1024

// RedisError is an error reply sent by the server.
type RedisError string

func (e RedisError) Error() string { return string(e) }

// Client is a CacheStorm client
type Client struct {
	addr string
	pool *pool
	opts *Options
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
	if options.PoolSize <= 0 {
		options.PoolSize = 1
	}

	c := &Client{
		addr: addr,
		opts: options,
	}

	c.pool = newPool(addr, options)
	return c, nil
}

// Close closes the client. It is safe to call more than once and
// concurrently with commands; later commands return ErrClosed.
func (c *Client) Close() error {
	return c.pool.Close()
}

// Do executes a raw command
func (c *Client) Do(ctx context.Context, args ...interface{}) (interface{}, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cn, err := c.pool.Get(ctx)
	if err != nil {
		return nil, err
	}
	cmds := []Cmd{{Args: args}}
	err = cn.exec(ctx, cmds)
	c.pool.Put(cn)
	if err != nil {
		return nil, c.pool.mapErr(ctx, err)
	}
	return cmds[0].Val, cmds[0].Err
}

// Ping sends a PING command
func (c *Client) Ping(ctx context.Context) (string, error) {
	val, err := c.Do(ctx, "PING")
	if err != nil {
		return "", err
	}
	return toString(val), nil
}

func appendExpiration(args []interface{}, expiration time.Duration) []interface{} {
	if expiration <= 0 {
		return args
	}
	if expiration%time.Second == 0 {
		return append(args, "EX", int64(expiration/time.Second))
	}
	ms := int64(expiration / time.Millisecond)
	if ms < 1 {
		ms = 1
	}
	return append(args, "PX", ms)
}

// Set sets a key-value pair
func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	args := appendExpiration([]interface{}{"SET", key, value}, expiration)
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
	var val interface{}
	var err error
	if expiration%time.Second == 0 {
		val, err = c.Do(ctx, "EXPIRE", key, int64(expiration/time.Second))
	} else {
		val, err = c.Do(ctx, "PEXPIRE", key, int64(expiration/time.Millisecond))
	}
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

// Subscribe opens a dedicated connection subscribed to channels. A failure
// to connect or subscribe is reported by ReceiveMessage.
func (c *Client) Subscribe(ctx context.Context, channels ...string) *PubSub {
	return newPubSub(ctx, c, "SUBSCRIBE", channels)
}

// PSubscribe opens a dedicated connection subscribed to patterns.
func (c *Client) PSubscribe(ctx context.Context, patterns ...string) *PubSub {
	return newPubSub(ctx, c, "PSUBSCRIBE", patterns)
}

// --- Connection Pool ---

type pool struct {
	addr   string
	opts   *Options
	turns  chan struct{}
	done   chan struct{}
	mu     sync.Mutex
	closed bool
	idle   []*conn
	conns  map[*conn]struct{}
}

func newPool(addr string, opts *Options) *pool {
	return &pool{
		addr:  addr,
		opts:  opts,
		turns: make(chan struct{}, opts.PoolSize),
		done:  make(chan struct{}),
		conns: make(map[*conn]struct{}),
	}
}

func (p *pool) isClosed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}

func (p *pool) mapErr(ctx context.Context, err error) error {
	var re RedisError
	if err == nil || errors.As(err, &re) {
		return err
	}
	if p.isClosed() {
		return ErrClosed
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
			return context.DeadlineExceeded
		}
	}
	return err
}

func (p *pool) waitTurn(ctx context.Context) error {
	select {
	case <-p.done:
		return ErrClosed
	default:
	}
	select {
	case p.turns <- struct{}{}:
		return nil
	default:
	}
	var timeout <-chan time.Time
	if p.opts.PoolTimeout > 0 {
		t := time.NewTimer(p.opts.PoolTimeout)
		defer t.Stop()
		timeout = t.C
	}
	select {
	case p.turns <- struct{}{}:
		return nil
	case <-p.done:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	case <-timeout:
		return ErrPoolTimeout
	}
}

func (p *pool) releaseTurn() {
	<-p.turns
}

func (p *pool) Get(ctx context.Context) (*conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := p.waitTurn(ctx); err != nil {
		return nil, err
	}
	cn, stale, err := p.popIdle()
	for _, s := range stale {
		s.Close()
	}
	if err != nil {
		p.releaseTurn()
		return nil, err
	}
	if cn != nil {
		return cn, nil
	}
	cn, err = p.dial(ctx)
	if err != nil {
		p.releaseTurn()
		return nil, p.mapErr(ctx, err)
	}
	return cn, nil
}

func (p *pool) popIdle() (*conn, []*conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, nil, ErrClosed
	}
	now := time.Now()
	var stale []*conn
	for len(p.idle) > 0 {
		cn := p.idle[len(p.idle)-1]
		p.idle[len(p.idle)-1] = nil
		p.idle = p.idle[:len(p.idle)-1]
		if cn.broken.Load() ||
			(p.opts.IdleTimeout > 0 && now.Sub(cn.usedAt) > p.opts.IdleTimeout) ||
			(p.opts.MaxConnAge > 0 && now.Sub(cn.createdAt) > p.opts.MaxConnAge) {
			delete(p.conns, cn)
			stale = append(stale, cn)
			continue
		}
		return cn, stale, nil
	}
	return nil, stale, nil
}

func (p *pool) dial(ctx context.Context) (*conn, error) {
	d := net.Dialer{Timeout: p.opts.DialTimeout}
	netConn, err := d.DialContext(ctx, "tcp", p.addr)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	cn := &conn{
		netConn:   netConn,
		reader:    bufio.NewReaderSize(netConn, 4096),
		opts:      p.opts,
		createdAt: now,
		usedAt:    now,
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		cn.Close()
		return nil, ErrClosed
	}
	p.conns[cn] = struct{}{}
	p.mu.Unlock()

	if p.opts.Password != "" {
		cmds := []Cmd{{Args: []interface{}{"AUTH", p.opts.Password}}}
		err := cn.exec(ctx, cmds)
		if err == nil {
			err = cmds[0].Err
		}
		if err != nil {
			p.remove(cn)
			return nil, fmt.Errorf("auth failed: %w", p.mapErr(ctx, err))
		}
	}

	return cn, nil
}

func (p *pool) remove(cn *conn) {
	p.mu.Lock()
	delete(p.conns, cn)
	p.mu.Unlock()
	cn.Close()
}

func (p *pool) Put(cn *conn) {
	if cn == nil {
		return
	}
	defer p.releaseTurn()
	p.mu.Lock()
	if p.closed || cn.broken.Load() {
		delete(p.conns, cn)
		p.mu.Unlock()
		cn.Close()
		return
	}
	cn.usedAt = time.Now()
	p.idle = append(p.idle, cn)
	p.mu.Unlock()
}

func (p *pool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	close(p.done)
	all := make([]*conn, 0, len(p.conns))
	for cn := range p.conns {
		all = append(all, cn)
	}
	p.conns = make(map[*conn]struct{})
	p.idle = nil
	p.mu.Unlock()

	for _, cn := range all {
		cn.Close()
	}
	return nil
}

// --- Connection with RESP Protocol ---

type conn struct {
	netConn   net.Conn
	reader    *bufio.Reader
	opts      *Options
	mu        sync.Mutex
	closeOnce sync.Once
	broken    atomic.Bool
	createdAt time.Time
	usedAt    time.Time
}

var errConnBroken = errors.New("cachestorm: connection is broken")

func (c *conn) deadline(ctx context.Context, timeout time.Duration) time.Time {
	var t time.Time
	if timeout > 0 {
		t = time.Now().Add(timeout)
	}
	if d, ok := ctx.Deadline(); ok && (t.IsZero() || d.Before(t)) {
		t = d
	}
	return t
}

func (c *conn) exec(ctx context.Context, cmds []Cmd) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	fail := func(from int, err error) error {
		c.Close()
		for i := from; i < len(cmds); i++ {
			cmds[i].Err = err
		}
		return err
	}

	if c.broken.Load() {
		return fail(0, errConnBroken)
	}
	if err := ctx.Err(); err != nil {
		return fail(0, err)
	}
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()

	var buf []byte
	for i := range cmds {
		buf = appendCommand(buf, cmds[i].Args)
	}
	if err := c.netConn.SetWriteDeadline(c.deadline(ctx, c.opts.WriteTimeout)); err != nil {
		return fail(0, err)
	}
	if _, err := c.netConn.Write(buf); err != nil {
		return fail(0, err)
	}

	for i := range cmds {
		if err := c.netConn.SetReadDeadline(c.deadline(ctx, c.opts.ReadTimeout)); err != nil {
			return fail(i, err)
		}
		val, err := readReply(c.reader)
		if err != nil {
			var re RedisError
			if errors.As(err, &re) {
				cmds[i].Err = re
				continue
			}
			return fail(i, err)
		}
		cmds[i].Val = val
	}
	return nil
}

func appendCommand(buf []byte, args []interface{}) []byte {
	buf = append(buf, '*')
	buf = strconv.AppendInt(buf, int64(len(args)), 10)
	buf = append(buf, '\r', '\n')
	for _, arg := range args {
		s := argString(arg)
		buf = append(buf, '$')
		buf = strconv.AppendInt(buf, int64(len(s)), 10)
		buf = append(buf, '\r', '\n')
		buf = append(buf, s...)
		buf = append(buf, '\r', '\n')
	}
	return buf
}

func argString(arg interface{}) string {
	switch v := arg.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		return string(v)
	case int:
		return strconv.FormatInt(int64(v), 10)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "1"
		}
		return "0"
	case time.Duration:
		return strconv.FormatInt(int64(v), 10)
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func readReply(r *bufio.Reader) (interface{}, error) {
	v, err := readValue(r)
	if err != nil {
		return nil, err
	}
	if re, ok := v.(RedisError); ok {
		return nil, re
	}
	return v, nil
}

func readValue(r *bufio.Reader) (interface{}, error) {
	b, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	line, err := readLine(r)
	if err != nil {
		return nil, err
	}

	switch b {
	case '+':
		return string(line), nil
	case '-':
		return RedisError(line), nil
	case ':':
		n, err := strconv.ParseInt(string(line), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: bad integer %q", ErrProtocol, line)
		}
		return n, nil
	case '$':
		size, err := strconv.ParseInt(string(line), 10, 64)
		if err != nil || size < -1 || size > maxBulkLen {
			return nil, fmt.Errorf("%w: bad bulk length %q", ErrProtocol, line)
		}
		if size == -1 {
			return nil, nil
		}
		buf := make([]byte, size+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		if buf[size] != '\r' || buf[size+1] != '\n' {
			return nil, fmt.Errorf("%w: bulk string not terminated by CRLF", ErrProtocol)
		}
		return string(buf[:size]), nil
	case '*':
		count, err := strconv.ParseInt(string(line), 10, 64)
		if err != nil || count < -1 || count > maxBulkLen {
			return nil, fmt.Errorf("%w: bad array length %q", ErrProtocol, line)
		}
		if count == -1 {
			return nil, nil
		}
		arr := make([]interface{}, 0, min(count, 1024))
		for i := int64(0); i < count; i++ {
			v, err := readValue(r)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		return arr, nil
	default:
		return nil, fmt.Errorf("%w: unknown reply type %q", ErrProtocol, b)
	}
}

func readLine(r *bufio.Reader) ([]byte, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return nil, fmt.Errorf("%w: line not terminated by CRLF", ErrProtocol)
	}
	return line[:len(line)-2], nil
}

func (c *conn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		c.broken.Store(true)
		err = c.netConn.Close()
	})
	return err
}

// --- Pipeline ---

// Pipeline buffers commands and sends them to the server in one write on a
// single connection, then reads all replies.
type Pipeline struct {
	client *Client
	mu     sync.Mutex
	cmds   []Cmd
}

type Cmd struct {
	Args []interface{}
	Err  error
	Val  interface{}
}

// Do queues an arbitrary command.
func (p *Pipeline) Do(ctx context.Context, args ...interface{}) {
	p.mu.Lock()
	p.cmds = append(p.cmds, Cmd{Args: args})
	p.mu.Unlock()
}

func (p *Pipeline) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) {
	p.Do(ctx, appendExpiration([]interface{}{"SET", key, value}, expiration)...)
}

func (p *Pipeline) Get(ctx context.Context, key string) {
	p.Do(ctx, "GET", key)
}

// Exec sends the queued commands and clears the queue. Each Cmd holds its
// own reply or error; the returned error is the first one encountered.
func (p *Pipeline) Exec(ctx context.Context) ([]Cmd, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	p.mu.Lock()
	cmds := p.cmds
	p.cmds = nil
	p.mu.Unlock()
	if len(cmds) == 0 {
		return cmds, nil
	}

	cn, err := p.client.pool.Get(ctx)
	if err != nil {
		for i := range cmds {
			cmds[i].Err = err
		}
		return cmds, err
	}
	err = cn.exec(ctx, cmds)
	p.client.pool.Put(cn)
	if err != nil {
		mapped := p.client.pool.mapErr(ctx, err)
		for i := range cmds {
			var re RedisError
			if cmds[i].Err != nil && !errors.As(cmds[i].Err, &re) {
				cmds[i].Err = mapped
			}
		}
	}
	for i := range cmds {
		if cmds[i].Err != nil {
			return cmds, cmds[i].Err
		}
	}
	return cmds, nil
}

// --- PubSub ---

// PubSub is a subscription on a dedicated connection.
type PubSub struct {
	client *Client
	cn     *conn
	err    error
	mu     sync.Mutex
	closed bool
	rmu    sync.Mutex
	dmu    sync.Mutex
	gen    uint64
}

func newPubSub(ctx context.Context, client *Client, command string, names []string) *PubSub {
	ps := &PubSub{client: client}
	if ctx == nil {
		ctx = context.Background()
	}
	if len(names) == 0 {
		ps.err = fmt.Errorf("cachestorm: %s requires at least one name", command)
		return ps
	}
	cn, err := client.pool.dial(ctx)
	if err != nil {
		ps.err = client.pool.mapErr(ctx, err)
		return ps
	}
	ps.cn = cn

	args := make([]interface{}, 0, 1+len(names))
	args = append(args, command)
	for _, n := range names {
		args = append(args, n)
	}
	if err := cn.netConn.SetWriteDeadline(cn.deadline(ctx, client.opts.WriteTimeout)); err == nil {
		_, err = cn.netConn.Write(appendCommand(nil, args))
		if err == nil {
			err = cn.netConn.SetWriteDeadline(time.Time{})
		}
		if err != nil {
			ps.fail(client.pool.mapErr(ctx, err))
		}
	} else {
		ps.fail(err)
	}
	return ps
}

func (ps *PubSub) fail(err error) {
	ps.mu.Lock()
	if ps.err == nil {
		ps.err = err
	}
	cn := ps.cn
	ps.mu.Unlock()
	if cn != nil {
		ps.client.pool.remove(cn)
	}
}

func (ps *PubSub) state() (*conn, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.closed {
		return nil, ErrClosed
	}
	if ps.err != nil {
		return nil, ps.err
	}
	return ps.cn, nil
}

type Message struct {
	Channel string
	Pattern string
	Payload string
}

// ReceiveMessage blocks until a message arrives, ctx is done, or the
// subscription fails. Subscription confirmations are skipped. A context that
// ends before any byte of a reply was read leaves the subscription usable.
func (ps *PubSub) ReceiveMessage(ctx context.Context) (*Message, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ps.rmu.Lock()
	defer ps.rmu.Unlock()

	for {
		cn, err := ps.state()
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ps.dmu.Lock()
		ps.gen++
		gen := ps.gen
		d, _ := ctx.Deadline()
		err = cn.netConn.SetReadDeadline(d)
		ps.dmu.Unlock()
		if err != nil {
			ps.fail(err)
			return nil, ps.errAfter(ctx, err)
		}
		stop := context.AfterFunc(ctx, func() {
			ps.dmu.Lock()
			if ps.gen == gen {
				cn.netConn.SetReadDeadline(time.Unix(1, 0))
			}
			ps.dmu.Unlock()
		})

		_, perr := cn.reader.Peek(1)
		var v interface{}
		if perr == nil {
			v, err = readValue(cn.reader)
		}
		stop()
		ps.dmu.Lock()
		ps.gen++
		ps.dmu.Unlock()

		if perr != nil {
			var ne net.Error
			if errors.As(perr, &ne) && ne.Timeout() && !ps.isClosed() {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return nil, ctxErr
				}
				return nil, context.DeadlineExceeded
			}
			ps.fail(perr)
			return nil, ps.errAfter(ctx, perr)
		}
		if err != nil {
			ps.fail(err)
			return nil, ps.errAfter(ctx, err)
		}

		if re, ok := v.(RedisError); ok {
			return nil, re
		}
		msg, ok := parseMessage(v)
		if ok {
			return msg, nil
		}
	}
}

func (ps *PubSub) isClosed() bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.closed
}

func (ps *PubSub) errAfter(ctx context.Context, err error) error {
	if ps.isClosed() {
		return ErrClosed
	}
	return ps.client.pool.mapErr(ctx, err)
}

func parseMessage(v interface{}) (*Message, bool) {
	arr, ok := v.([]interface{})
	if !ok || len(arr) == 0 {
		return nil, false
	}
	kind, _ := arr[0].(string)
	switch kind {
	case "message", "smessage":
		if len(arr) == 3 {
			return &Message{Channel: toString(arr[1]), Payload: toString(arr[2])}, true
		}
	case "pmessage":
		if len(arr) == 4 {
			return &Message{Pattern: toString(arr[1]), Channel: toString(arr[2]), Payload: toString(arr[3])}, true
		}
	}
	return nil, false
}

// Close closes the subscription connection. It is safe to call more than once.
func (ps *PubSub) Close() error {
	ps.mu.Lock()
	if ps.closed {
		ps.mu.Unlock()
		return nil
	}
	ps.closed = true
	cn := ps.cn
	ps.mu.Unlock()
	if cn != nil {
		ps.client.pool.remove(cn)
	}
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
