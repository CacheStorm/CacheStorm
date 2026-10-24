package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/acl"
	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/plugin"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

const (
	defaultReadTimeout  = 5 * time.Minute
	defaultWriteTimeout = 5 * time.Minute
	keepAlivePeriod     = 3 * time.Minute
)

type Connection struct {
	ID           int64
	conn         net.Conn
	bufReader    *bufio.Reader
	reader       *resp.Reader
	writer       *resp.Writer
	store        *store.Store
	router       *command.Router
	plugins      *plugin.Manager
	namespace    string
	createdAt    time.Time
	lastCmd      string
	readTimeout  time.Duration
	writeTimeout time.Duration
	subscriber   *store.Subscriber // PubSub subscriber, persists across commands
	// authenticated belongs to the connection, not the per-command Context:
	// Handle() builds a fresh Context for every command, so auth state set
	// during AUTH would otherwise be discarded before the next command runs.
	authenticated bool
	// aclUser is the ACL user resolved by AUTH, carried the same way: it must
	// outlive the per-command Context for permissions to persist.
	aclUser *acl.User
	// clientName is set by CLIENT SETNAME and carried back onto the next
	// command's Context, the same way aclUser is.
	clientName  string
	transaction *command.Transaction

	// writeMu serialises use of writer (and pushBuf, which shares its
	// socket) between the command loop and the Pub/Sub delivery goroutine.
	writeMu      sync.Mutex
	pushBuf      *bufio.Writer
	deliveryStop chan struct{}
	deliveryDone chan struct{}
	stopOnce     sync.Once
}

// countingConn wraps a connection and feeds the global byte counters so the
// advertised bytes_in/bytes_out statistics reflect real traffic.
type countingConn struct {
	net.Conn
}

func (c *countingConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n > 0 {
		store.GlobalMetrics.RecordBytesIn(int64(n))
	}
	return n, err
}

func (c *countingConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	if n > 0 {
		store.GlobalMetrics.RecordBytesOut(int64(n))
	}
	return n, err
}

// deadlineWriterConn re-arms the socket write deadline immediately before each
// write, instead of latching one deadline for a whole command.
//
// Handle() arms a write deadline before router.Execute, but blocking commands
// (BLPOP, BRPOPLPUSH, XREAD BLOCK ...) wait inside Execute for as long as the
// client asked. A deadline armed before the block expires while the command is
// still running, so the reply of a completed, successful command was dropped and
// the client disconnected having received nothing. Refreshing per write still
// bounds each individual write against a stalled peer — it just no longer
// charges the command's own execution time against the reply's deadline.
type deadlineWriterConn struct {
	net.Conn
	// timeout points at Connection.writeTimeout rather than copying it: the
	// accept path (server.go) assigns the configured timeout after
	// NewConnection has returned, so a copy would pin the default forever.
	timeout *time.Duration
}

func (d *deadlineWriterConn) Write(b []byte) (int, error) {
	_ = d.SetWriteDeadline(time.Now().Add(*d.timeout))
	return d.Conn.Write(b)
}

func NewConnection(id int64, conn net.Conn, s *store.Store, r *command.Router, pm *plugin.Manager) *Connection {
	// Keep-alive needs the raw TCP connection: Handle()'s type assertion
	// would miss the counting wrapper installed below.
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(keepAlivePeriod)
	}

	cc := &countingConn{Conn: conn}
	br := bufio.NewReader(cc)
	c := &Connection{
		ID:           id,
		conn:         cc,
		bufReader:    br,
		reader:       resp.NewReader(br),
		writer:       resp.NewWriter(bufio.NewWriter(cc)),
		store:        s,
		router:       r,
		plugins:      pm,
		namespace:    "default",
		createdAt:    time.Now(),
		readTimeout:  defaultReadTimeout,
		writeTimeout: defaultWriteTimeout,
		transaction:  command.NewTransaction(),
	}
	// Keep-alive was set on the raw TCP connection above, before this wrapper,
	// so the deadline refresh sits between the byte counters and the socket.
	// The wrapper reads Connection.writeTimeout live, so the configured timeout
	// the accept path assigns later is honoured on every write.
	cc.Conn = &deadlineWriterConn{Conn: conn, timeout: &c.writeTimeout}

	// Track the connection so CONNECTION.INFO, CONNECTION.LIST and
	// CONNECTION.COUNT report the live connections.
	command.TrackConnection(id, conn.RemoteAddr().String())

	return c
}

func (c *Connection) Handle() {
	defer c.Close()
	defer c.recoverPanic()

	logger.Debug().
		Int64("conn_id", c.ID).
		Str("remote", c.conn.RemoteAddr().String()).
		Msg("client connected")

	for {
		if c.subscribed() {
			// Redis never times out a pub/sub client for idleness: it is
			// expected to sit silently waiting for messages.
			c.conn.SetReadDeadline(time.Time{})
		} else {
			c.conn.SetReadDeadline(time.Now().Add(c.readTimeout))
		}
		cmd, args, err := c.reader.ReadCommand()
		if err != nil {
			if err == io.EOF || isClosedError(err) {
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return
			}
			logger.Error().Err(err).Int64("conn_id", c.ID).Msg("read error")
			return
		}

		cmd = command.CanonicalName(cmd)
		c.lastCmd = cmd

		if c.runCommand(cmd, args) {
			return
		}
	}
}

// runCommand executes one command and reports whether the connection should
// close. It holds writeMu throughout, so Pub/Sub frames written by the
// delivery goroutine never interleave with a reply; the deferred unlock keeps
// a panicking command from wedging that goroutine.
func (c *Connection) runCommand(cmd string, args [][]byte) (quit bool) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	{
		// Frames queued before this command was read go out ahead of its
		// reply, as they would have in Redis's output buffer.
		if c.subscriber != nil {
			if err := c.writePendingLocked(c.subscriber, nil); err != nil {
				return true
			}
		}

		if c.subscribed() {
			if handled := c.handleSubscribedMode(cmd, args); handled {
				command.BumpConnectionCommand(c.ID)
				return false
			}
		}

		ctx := command.NewContextWithClient(cmd, args, c.store, c.writer, c.ID, c.conn.RemoteAddr().String())
		if c.transaction == nil {
			c.transaction = ctx.Transaction
		}
		ctx.Transaction = c.transaction
		// Share the subscriber across commands so PubSub state persists
		if c.subscriber != nil {
			ctx.Subscriber = c.subscriber
		}
		// Auth state lives on the connection; the Context is per-command.
		ctx.SetAuthenticated(c.authenticated)
		ctx.ACLUser = c.aclUser
		ctx.ClientName = c.clientName

		if cmd == "QUIT" {
			c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
			c.writer.WriteOK()
			return true
		}

		if c.plugins != nil {
			if err := c.plugins.RunBeforeHooks(ctx); err != nil {
				logger.Warn().Err(err).Int64("conn_id", c.ID).Msg("before-command hook error")
			}
		}

		c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
		var stopWatch func()
		if blockingCommands[strings.ToUpper(cmd)] {
			ctx.Done, stopWatch = c.watchDisconnect()
		}
		err := c.router.Execute(ctx)
		if stopWatch != nil {
			stopWatch()
		}
		if err != nil {
			store.GlobalMetrics.RecordError()
			if err == command.ErrUnknownCommand {
				c.writer.WriteError("ERR unknown command '" + cmd + "'")
			} else {
				c.writer.WriteError(err.Error())
			}
		}
		elapsed := time.Since(ctx.StartTime)
		store.GlobalMetrics.RecordCommand(cmd, elapsed.Nanoseconds())
		store.GlobalSlowLog.AddIfSlow(elapsed, cmd, ctx.Args, ctx.RemoteAddr, ctx.ClientName)
		if writeCommands[strings.ToUpper(cmd)] {
			store.GlobalMetrics.RecordWrite()
		} else {
			store.GlobalMetrics.RecordRead()
		}
		if c.plugins != nil {
			c.plugins.RunAfterHooks(ctx)
		}
		// Capture subscriber if created during this command (e.g. SUBSCRIBE)
		if ctx.Subscriber != nil && c.subscriber == nil {
			c.subscriber = ctx.Subscriber
			c.startDelivery(c.subscriber)
		}
		// Carry auth state back onto the connection: AUTH sets the flag on
		// the Context, which is rebuilt from scratch for the next command.
		c.authenticated = ctx.IsAuthenticated()
		c.aclUser = ctx.ACLUser
		c.clientName = ctx.ClientName

		// Count the command against the connection so CONNECTION.INFO can
		// report it.
		command.BumpConnectionCommand(c.ID)
	}
	return false
}

// blockingCommands may wait for data for as long as the client asked, or
// forever with a zero timeout.
var blockingCommands = map[string]bool{
	"BLPOP": true, "BRPOP": true, "BLMOVE": true, "BRPOPLPUSH": true, "BLMPOP": true,
	"BZPOPMIN": true, "BZPOPMAX": true, "BZMPOP": true,
	"XREAD": true, "XREADGROUP": true,
}

// watchDisconnect reports the client going away while a blocking command
// waits. The handler goroutine is parked inside the command, so nothing else
// reads the socket and a closed connection would otherwise leave the command
// waiting, possibly forever. The watcher peeks for one byte: EOF or an error
// closes the returned channel, while a pipelined command just ends the watch
// with its bytes still buffered for the next read. The returned stop function
// unblocks the peek and waits for the watcher to exit, so the reader is never
// used by two goroutines at once.
func (c *Connection) watchDisconnect() (<-chan struct{}, func()) {
	if c.bufReader == nil || c.bufReader.Buffered() > 0 {
		return nil, nil
	}
	gone := make(chan struct{})
	stopped := make(chan struct{})
	exited := make(chan struct{})
	_ = c.conn.SetReadDeadline(time.Time{})
	go func() {
		defer close(exited)
		if _, err := c.bufReader.Peek(1); err != nil {
			select {
			case <-stopped:
			default:
				close(gone)
			}
		}
	}()
	return gone, func() {
		close(stopped)
		_ = c.conn.SetReadDeadline(time.Now())
		<-exited
	}
}

func (c *Connection) recoverPanic() {
	if r := recover(); r != nil {
		logger.Error().
			Int64("conn_id", c.ID).
			Str("last_cmd", c.lastCmd).
			Str("remote", c.conn.RemoteAddr().String()).
			Str("panic", fmt.Sprintf("%v", r)).
			Str("stack", string(debug.Stack())).
			Msg("panic recovered in connection handler")
	}
}

func (c *Connection) Close() {
	// Stop reporting the connection: it is gone.
	command.UntrackConnection(c.ID)
	// Clean up PubSub subscriber to prevent resource leak
	if c.subscriber != nil {
		if ps := c.store.GetPubSub(); ps != nil {
			ps.RemoveSubscriber(c.subscriber)
		} else {
			c.subscriber.Close()
		}
		c.subscriber = nil
	}
	c.conn.Close()
	c.stopDelivery()
	logger.Debug().
		Int64("conn_id", c.ID).
		Msg("client disconnected")
}

func (c *Connection) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}

func isClosedError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) || err.Error() == "use of closed network connection"
}

// subscribedModeCommands are the only commands a RESP2 client may send while
// it holds any subscription.
var subscribedModeCommands = map[string]bool{
	"SUBSCRIBE": true, "PSUBSCRIBE": true, "SSUBSCRIBE": true,
	"UNSUBSCRIBE": true, "PUNSUBSCRIBE": true, "SUNSUBSCRIBE": true,
	"PING": true, "QUIT": true, "RESET": true,
}

// pushBatchLimit caps how many queued frames one delivery pass writes before
// flushing and releasing writeMu, so a busy channel cannot starve the
// command loop.
const pushBatchLimit = 256

func (c *Connection) subscribed() bool {
	return c.subscriber != nil && c.subscriber.TotalSubscriptions() > 0
}

// handleSubscribedMode applies Redis's RESP2 subscribed-state rules. It
// reports whether it answered the command itself.
func (c *Connection) handleSubscribedMode(cmd string, args [][]byte) bool {
	if cmd == "PING" {
		c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
		if len(args) > 1 {
			c.writer.WriteError("ERR wrong number of arguments for 'ping' command")
			return true
		}
		msg := []byte{}
		if len(args) == 1 {
			msg = args[0]
		}
		c.writer.WriteValue(resp.ArrayValue([]*resp.Value{
			resp.BulkString("pong"),
			resp.BulkBytes(msg),
		}))
		return true
	}
	if subscribedModeCommands[cmd] {
		return false
	}
	if _, known := c.router.Get(cmd); !known {
		return false
	}
	store.GlobalMetrics.RecordError()
	c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	c.writer.WriteError("ERR Can't execute '" + strings.ToLower(cmd) +
		"': only (P|S)SUBSCRIBE / (P|S)UNSUBSCRIBE / PING / QUIT / RESET are allowed in this context")
	return true
}

// startDelivery starts the goroutine that moves the subscriber's queued
// frames onto the socket. It runs until the connection closes or the
// subscriber is removed (which closes its queue).
func (c *Connection) startDelivery(sub *store.Subscriber) {
	if c.deliveryDone != nil {
		return
	}
	c.pushBuf = bufio.NewWriter(c.conn)
	stop := make(chan struct{})
	done := make(chan struct{})
	c.deliveryStop, c.deliveryDone = stop, done
	sub.SetOverflowHandler(func() {
		logger.Warn().
			Int64("conn_id", c.ID).
			Int64("dropped", sub.DropCount()).
			Msg("closing pub/sub client: output queue overflow (client-output-buffer-limit pubsub)")
		c.conn.Close()
	})
	go c.deliver(sub, stop, done)
}

func (c *Connection) deliver(sub *store.Subscriber, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	frames := sub.Channel()
	for {
		select {
		case <-stop:
			return
		case <-sub.Overflowed():
			return
		case frame, ok := <-frames:
			if !ok {
				return
			}
			c.writeMu.Lock()
			err := c.writePendingLocked(sub, frame)
			c.writeMu.Unlock()
			if err != nil {
				c.conn.Close()
				return
			}
		}
	}
}

// writePendingLocked writes first (when non-nil) and then whatever else is
// already queued, up to pushBatchLimit frames, with a single flush. Caller
// holds writeMu.
func (c *Connection) writePendingLocked(sub *store.Subscriber, first []byte) error {
	if c.pushBuf == nil || sub.IsOverflowed() {
		return nil
	}
	if first == nil {
		select {
		case f, ok := <-sub.Channel():
			if !ok {
				return nil
			}
			first = f
		default:
			return nil
		}
	}
	// Anything the command loop left buffered goes out first.
	if err := c.writer.Flush(); err != nil {
		return err
	}
	if _, err := c.pushBuf.Write(first); err != nil {
		return err
	}
	sub.Delivered(first)
	for i := 1; i < pushBatchLimit; i++ {
		select {
		case f, ok := <-sub.Channel():
			if !ok {
				return c.pushBuf.Flush()
			}
			if _, err := c.pushBuf.Write(f); err != nil {
				return err
			}
			sub.Delivered(f)
			continue
		default:
		}
		break
	}
	return c.pushBuf.Flush()
}

// stopDelivery ends the delivery goroutine, if one was started, and waits for
// it so none outlives the connection.
func (c *Connection) stopDelivery() {
	c.stopOnce.Do(func() {
		if c.deliveryStop == nil {
			return
		}
		close(c.deliveryStop)
		<-c.deliveryDone
	})
}
