package server

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/acl"
	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/plugin"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

const (
	defaultWriteTimeout    = 5 * time.Minute
	defaultKeepAlivePeriod = 300 * time.Second
	defaultReadBufferSize  = 4096
	defaultWriteBufferSize = 16 << 10
)

// connectionOptions are the per-connection socket settings from the server
// configuration.
type connectionOptions struct {
	keepAlive       time.Duration
	readBufferSize  int
	writeBufferSize int
}

type Connection struct {
	ID int64
	// sink holds replies while a command runs; see replySink.
	sink *replySink
	// draining is set when the server shuts down: the connection finishes the
	// command it is running and then closes instead of reading another.
	draining    atomic.Bool
	conn        net.Conn
	bufReader   *bufio.Reader
	reader      *resp.Reader
	writer      *resp.Writer
	store       *store.Store
	router      *command.Router
	plugins     *plugin.Manager
	namespace   string
	createdAt   time.Time
	lastCmd     string
	rawCmd      string
	readTimeout time.Duration
	// blocked is set while a blocking command (BLPOP, XREAD BLOCK, ...)
	// waits, for INFO's blocked_clients.
	blocked      atomic.Bool
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
	// clusterConn is the READONLY/ASKING state of the connection.
	clusterConn command.ClusterConn

	// writeMu serialises use of writer (and pushBuf, which shares its
	// socket) between the command loop and the Pub/Sub delivery goroutine.
	writeMu      sync.Mutex
	pushBuf      *bufio.Writer
	deliveryStop chan struct{}
	deliveryDone chan struct{}
	stopOnce     sync.Once

	// dbs and db are the databases and the one this connection selected;
	// with dbs nil the connection only ever uses store.
	dbs *store.Databases
	db  int

	// srv is the server the connection belongs to; nil for a connection
	// built on its own (tests).
	srv *Server
	// replConf is what REPLCONF announced on this connection.
	replConf command.ReplicaConf
	// handler serves the connection once a command (PSYNC) took it over.
	handler command.ConnHandler
	// remoteAddr caches conn.RemoteAddr().String() for the command contexts.
	remoteAddr string

	// info is what CLIENT LIST reports, readable from other goroutines.
	info clientInfo
	// handedOff, subscribing and the subscription counts mirror connection
	// state for CLIENT LIST and CLIENT KILL TYPE.
	handedOff   atomic.Bool
	subscribing atomic.Bool
	subCounts   atomic.Int64
	psubCounts  atomic.Int64
	ssubCounts  atomic.Int64
	// killed is set by CLIENT KILL of the calling client: it is closed once
	// its reply is out.
	killed atomic.Bool
	// replyOff and replySkip implement CLIENT REPLY OFF and SKIP.
	replyOff  bool
	replySkip bool
	// blockMu guards unblockCh and unblockErr, which CLIENT UNBLOCK uses to
	// end the blocking command the connection is running.
	blockMu    sync.Mutex
	unblockCh  chan struct{}
	unblockErr bool
	// heldGate is the write-gate hold kept from the previous command of a
	// pipeline, so the next command on the same stripes runs without
	// handing the gate to another connection in between; heldCount bounds
	// how many commands share one hold.
	heldGate  *gateHold
	heldCount int
	// holds are the two buffers commands plan their hold into: one may be
	// heldGate while the next command plans into the other.
	holds [2]gateHold
	// gateWaiting is set while a blocking command has handed heldGate back
	// to wait, so releaseGate does not unlock it a second time.
	gateWaiting bool
}

// maxGateBatch bounds how many pipelined commands run under one gate hold
// before other connections get a turn.
const maxGateBatch = 64

// releaseGate drops the gate hold kept from the previous command.
func (c *Connection) releaseGate() {
	if c.heldGate != nil {
		if !c.gateWaiting {
			c.heldGate.unlock()
		}
		c.heldGate = nil
		c.heldCount = 0
		c.gateWaiting = false
	}
}

// keepGate reports whether the hold of the command that just ran may be kept
// for the next one: only for a shared (non-exclusive) hold, a command that
// did not block, and a next command already complete in the read buffer, so
// the hold is never kept while waiting on the network.
func (c *Connection) keepGate(h *gateHold, ctx *command.Context, blocking bool) bool {
	if h.exclusive || blocking || c.subscriber != nil || c.heldCount+1 >= maxGateBatch || ctx.CloseConnection || ctx.Handoff != nil || c.killed.Load() {
		return false
	}
	n := c.bufReader.Buffered()
	if n == 0 {
		return false
	}
	buf, err := c.bufReader.Peek(n)
	return err == nil && resp.HasCompleteCommand(buf)
}

// unblock ends the blocking command the connection is waiting in, as a
// timeout or, withError, with the UNBLOCKED error.
func (c *Connection) unblock(withError bool) bool {
	c.blockMu.Lock()
	defer c.blockMu.Unlock()
	if c.unblockCh == nil {
		return false
	}
	c.unblockErr = withError
	close(c.unblockCh)
	c.unblockCh = nil
	return true
}

// beginBlocking arms CLIENT UNBLOCK for a blocking command and returns the
// channel that closes when the command is unblocked.
func (c *Connection) beginBlocking() <-chan struct{} {
	c.blockMu.Lock()
	defer c.blockMu.Unlock()
	c.unblockErr = false
	c.unblockCh = make(chan struct{})
	return c.unblockCh
}

// endBlocking disarms CLIENT UNBLOCK and reports whether the command was
// unblocked with the error.
func (c *Connection) endBlocking() bool {
	c.blockMu.Lock()
	defer c.blockMu.Unlock()
	c.unblockCh = nil
	return c.unblockErr
}

// mergeDone closes the returned channel once either input closes; stop
// releases the goroutine when the command finishes first.
func mergeDone(a, b <-chan struct{}) (<-chan struct{}, func()) {
	if a == nil {
		return b, func() {}
	}
	out := make(chan struct{})
	stop := make(chan struct{})
	go func() {
		select {
		case <-a:
			close(out)
		case <-b:
			close(out)
		case <-stop:
		}
	}()
	return out, func() { close(stop) }
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

// replySink sits between the connection's buffered writers and the socket.
// While held it keeps output in memory, so a command never writes to the
// socket while it holds the server's write gate: a client that reads its
// replies slowly would otherwise keep the gate, and with it every other
// client, waiting on the socket. Releasing it writes what it kept, in order;
// a write made while it is not held first drains what it kept.
type replySink struct {
	conn net.Conn
	held bool
	buf  bytes.Buffer
}

// maxHeldReplies bounds how much pipelined output is kept before it is
// written, so a long pipeline does not grow the buffer without limit.
const maxHeldReplies = 64 << 10

func (s *replySink) Write(p []byte) (int, error) {
	if s.held {
		return s.buf.Write(p)
	}
	if err := s.release(); err != nil {
		return 0, err
	}
	return s.conn.Write(p)
}

func (s *replySink) hold() { s.held = true }

// release writes the kept output to the socket and stops holding.
func (s *replySink) release() error {
	s.held = false
	if s.buf.Len() == 0 {
		return nil
	}
	_, err := s.conn.Write(s.buf.Bytes())
	s.buf.Reset()
	return err
}

// idleTimeout is how long the connection may sit idle between commands; the
// server's live CONFIG timeout wins over the connection's own setting.
func (c *Connection) idleTimeout() time.Duration {
	if c.srv != nil {
		return time.Duration(c.srv.idleTimeout.Load())
	}
	return c.readTimeout
}

func NewConnection(id int64, conn net.Conn, s *store.Store, r *command.Router, pm *plugin.Manager) *Connection {
	return newConnection(id, conn, s, r, pm, connectionOptions{keepAlive: defaultKeepAlivePeriod})
}

func newConnection(id int64, conn net.Conn, s *store.Store, r *command.Router, pm *plugin.Manager, opts connectionOptions) *Connection {
	// Keep-alive needs the raw TCP connection: Handle()'s type assertion
	// would miss the counting wrapper installed below. A zero period turns
	// keep-alive off, as Redis' tcp-keepalive 0 does.
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if opts.keepAlive > 0 {
			tcpConn.SetKeepAlive(true)
			tcpConn.SetKeepAlivePeriod(opts.keepAlive)
		} else {
			tcpConn.SetKeepAlive(false)
		}
	}
	if opts.readBufferSize <= 0 {
		opts.readBufferSize = defaultReadBufferSize
	}
	if opts.writeBufferSize <= 0 {
		opts.writeBufferSize = defaultWriteBufferSize
	}

	cc := &countingConn{Conn: conn}
	br := bufio.NewReaderSize(cc, opts.readBufferSize)
	sink := &replySink{conn: cc}
	c := &Connection{
		ID:           id,
		conn:         cc,
		bufReader:    br,
		reader:       resp.NewReader(br),
		sink:         sink,
		writer:       resp.NewWriter(bufio.NewWriterSize(sink, opts.writeBufferSize)),
		store:        s,
		router:       r,
		plugins:      pm,
		namespace:    "default",
		createdAt:    time.Now(),
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
	// The gate hold belongs to this goroutine: it is dropped here, not in
	// Close, which shutdown may call from another goroutine.
	defer c.releaseGate()
	defer c.recoverPanic()

	logger.Debug().
		Int64("conn_id", c.ID).
		Str("remote", c.conn.RemoteAddr().String()).
		Msg("client connected")

	for {
		idle := c.idleTimeout()
		if c.subscribed() || idle <= 0 {
			// Redis never times out a pub/sub client for idleness: it is
			// expected to sit silently waiting for messages. Without a
			// configured read_timeout no client is, as with Redis' timeout 0.
			c.conn.SetReadDeadline(time.Time{})
		} else {
			c.conn.SetReadDeadline(time.Now().Add(idle))
		}
		// Checked after the deadline is armed: Drain sets the flag and then
		// moves the deadline to now, so either this check sees the flag or
		// the read below is interrupted by Drain's deadline.
		if c.draining.Load() && c.bufReader.Buffered() == 0 {
			return
		}
		if c.heldGate != nil {
			if n := c.bufReader.Buffered(); n == 0 {
				c.releaseGate()
			} else if buf, perr := c.bufReader.Peek(n); perr != nil || !resp.HasCompleteCommand(buf) {
				c.releaseGate()
			}
		}
		cmd, args, err := c.reader.ReadClientCommand()
		if err != nil {
			// Replies kept for a pipeline go out before the connection
			// reports the error or closes.
			c.releaseGate()
			c.writeMu.Lock()
			if ferr := c.writer.Flush(); ferr == nil {
				if rerr := c.sink.release(); rerr != nil {
					logger.Debug().Err(rerr).Int64("conn_id", c.ID).Msg("pending replies not delivered")
				}
			}
			c.writeMu.Unlock()
			if err == io.EOF || isClosedError(err) {
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return
			}
			if isProtocolError(err) {
				// Redis tells the client why before it drops the connection.
				c.writeMu.Lock()
				c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
				c.writer.WriteError("ERR Protocol error: " + err.Error())
				c.writeMu.Unlock()
				logger.Debug().Err(err).Int64("conn_id", c.ID).Msg("protocol error")
				return
			}
			logger.Error().Err(err).Int64("conn_id", c.ID).Msg("read error")
			return
		}

		c.rawCmd = cmd
		cmd = command.CanonicalName(cmd)
		c.lastCmd = cmd

		if c.handler != nil {
			c.handler.Command(cmd, args)
			continue
		}

		if c.srv != nil && c.srv.pauseActive() {
			c.releaseGate()
			c.srv.waitWhilePaused(cmd)
		}
		if c.runCommand(cmd, args) {
			// QUIT, a handoff or a write error ends the connection; replies
			// kept for the pipeline before it still go out.
			c.releaseGate()
			c.writeMu.Lock()
			if err := c.writer.Flush(); err == nil {
				if err := c.sink.release(); err != nil {
					logger.Debug().Err(err).Int64("conn_id", c.ID).Msg("pending replies not delivered")
				}
			}
			c.writeMu.Unlock()
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

		// RESP3 carries pub/sub messages out of band, so a subscribed RESP3
		// client may run any command, as in Redis.
		if c.subscribed() && !c.writer.IsRESP3() {
			if handled := c.handleSubscribedMode(cmd, args); handled {
				command.BumpConnectionCommand(c.ID)
				return false
			}
		}

		st := c.store
		if c.dbs != nil {
			if db := c.dbs.DB(c.db); db != nil {
				st = db
			}
		}
		c.noteCommand(cmd, args)
		c.srv.feedMonitors(c, c.db, c.rawCmd, cmd, args)
		out := c.writer
		replyCmd := cmd == "CLIENT" && len(args) > 0 && strings.EqualFold(string(args[0]), "REPLY")
		suppress := (c.replyOff || c.replySkip) && !replyCmd
		if suppress {
			out = resp.NewWriter(io.Discard)
			out.SetRESP3(c.writer.IsRESP3())
		}
		if !replyCmd {
			c.replySkip = false
		}
		var captured *bytes.Buffer
		if blockingCommands[strings.ToUpper(cmd)] && !suppress {
			captured = &bytes.Buffer{}
			out = resp.NewWriter(captured)
			out.SetRESP3(c.writer.IsRESP3())
		}
		if c.transaction == nil {
			c.transaction = command.NewTransaction()
		}
		if c.remoteAddr == "" {
			c.remoteAddr = c.conn.RemoteAddr().String()
		}
		ctx := command.NewConnectionContext(cmd, args, st, out, c.ID, c.remoteAddr, c.transaction)
		ctx.DB = c.db
		ctx.DBs = c.dbs
		// Share the subscriber across commands so PubSub state persists
		if c.subscriber != nil {
			ctx.Subscriber = c.subscriber
		}
		// Auth state lives on the connection; the Context is per-command.
		ctx.SetAuthenticated(c.authenticated)
		ctx.ACLUser = c.aclUser
		ctx.ClientName = c.clientName
		ctx.ReplConf = &c.replConf
		ctx.ClusterConn = &c.clusterConn

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

		var stopWatch func()
		var stopMerge func()
		if blockingCommands[strings.ToUpper(cmd)] {
			var gone <-chan struct{}
			gone, stopWatch = c.watchDisconnect()
			ctx.Done, stopMerge = mergeDone(gone, c.beginBlocking())
			c.blocked.Store(true)
			defer c.blocked.Store(false)
		}
		var err error
		c.sink.hold()
		if c.srv != nil && c.srv.refuseOnReplica(ctx) {
			c.releaseGate()
			err = nil
		} else {
			// The hold is recorded in heldGate as soon as it is taken, so a
			// panicking command still has it released by Handle.
			buf := &c.holds[0]
			if c.heldGate == buf {
				buf = &c.holds[1]
			}
			want := c.srv.planGate(ctx, buf)
			if want != nil {
				if !c.heldGate.covers(want) {
					c.releaseGate()
					want.lock()
					c.heldGate = want
				}
				if c.srv.pauseActive() && c.srv.pausedNow(cmd) {
					// A pause that began after the check before this
					// command waits for the writes holding the gate, so a
					// write that took the gate after it must wait too.
					c.releaseGate()
					c.srv.waitWhilePaused(cmd)
					want.lock()
					c.heldGate = want
				}
				h := c.heldGate
				ctx.BeforeWait = func() { h.unlock(); c.gateWaiting = true }
				ctx.AfterWait = func() { h.lock(); c.gateWaiting = false }
			} else {
				c.releaseGate()
			}
			errsBefore := c.writer.Errors()
			err = c.router.Execute(ctx)
			c.srv.trackCommand(ctx, err != nil || c.writer.Errors() != errsBefore)
			if c.heldGate != nil {
				if c.keepGate(c.heldGate, ctx, stopMerge != nil) {
					c.heldCount++
				} else {
					c.releaseGate()
				}
			}
		}
		if stopWatch != nil {
			stopWatch()
		}
		if stopMerge != nil {
			stopMerge()
			unblockedWithError := c.endBlocking()
			if captured != nil {
				if unblockedWithError {
					c.writer.WriteError("UNBLOCKED client unblocked via CLIENT UNBLOCK")
				} else if werr := c.writer.WriteCaptured(captured.Bytes(), ctx.Writer.Errors()); werr != nil {
					return true
				}
				ctx.Writer = c.writer
			}
		}
		if err != nil {
			store.GlobalMetrics.RecordError()
			if err == command.ErrUnknownCommand {
				c.writer.WriteError(unknownCommandError(c.rawCmd, cmd, args))
			} else {
				c.writer.WriteError(err.Error())
			}
		}
		elapsed := time.Since(ctx.StartTime)
		if stopMerge == nil {
			command.RecordLatency("command", elapsed)
		}
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
		if ctx.ResetConnection {
			if c.srv != nil {
				c.srv.monitor.remove(c.ID)
			}
			c.clusterConn = command.ClusterConn{}
			c.replySkip = false
			c.setClientFlags(-1, -1)
		}
		if ctx.ClientNoEvict != 0 || ctx.ClientNoTouch != 0 {
			c.setClientFlags(ctx.ClientNoEvict, ctx.ClientNoTouch)
		}
		// Capture subscriber if created during this command (e.g. SUBSCRIBE)
		if ctx.Subscriber != nil && c.subscriber == nil {
			c.subscriber = ctx.Subscriber
			c.startDelivery(c.subscriber)
			if c.srv != nil && c.srv.tracker != nil {
				c.srv.tracker.register(c)
			}
		}
		// Carry auth state back onto the connection: AUTH sets the flag on
		// the Context, which is rebuilt from scratch for the next command.
		c.authenticated = ctx.IsAuthenticated()
		if c.dbs == nil && ctx.DBs != nil {
			c.dbs = ctx.DBs
		}
		c.db = ctx.DB
		c.aclUser = ctx.ACLUser
		c.clientName = ctx.ClientName
		if ctx.ClientLibName != "" || ctx.ClientLibVer != "" {
			c.setLibInfo(ctx.ClientLibName, ctx.ClientLibVer)
		}
		switch ctx.ReplyMode {
		case "on":
			c.replyOff = false
		case "off":
			c.replyOff = true
		case "skip":
			c.replySkip = true
		}
		if c.subscriber != nil {
			total := int64(c.subscriber.TotalSubscriptions())
			chans, shards := int64(c.subscriber.SubscriptionCount()), int64(c.subscriber.ShardSubscriptionCount())
			c.subCounts.Store(chans)
			c.ssubCounts.Store(shards)
			c.psubCounts.Store(total - chans - shards)
			c.subscribing.Store(total > 0)
		}
		if ctx.Handoff != nil {
			c.handedOff.Store(true)
		}
		if ctx.CloseConnection {
			c.killed.Store(true)
		}
		if ctx.Monitor && c.srv != nil && c.subscriber != nil {
			c.srv.monitor.add(c)
		}
		c.publishInfo()

		if ctx.Handoff != nil {
			c.releaseGate()
			_ = c.writer.Flush()
			if err := c.sink.release(); err != nil {
				return true
			}
			c.handler = ctx.Handoff(c.conn)
		}

		// Count the command against the connection so CONNECTION.INFO can
		// report it.
		command.BumpConnectionCommand(c.ID)

		// Replies go out once the pipeline the client sent has been served,
		// so a pipeline of N commands costs one socket write instead of N.
		if err := c.writer.Flush(); err != nil {
			return true
		}
		if c.bufReader.Buffered() == 0 || c.sink.buf.Len() >= maxHeldReplies || c.subscriber != nil || c.killed.Load() {
			// The gate is never held across a socket write: a client that
			// does not read its replies would keep it, and every other
			// client, waiting.
			c.releaseGate()
			if err := c.sink.release(); err != nil {
				return true
			}
		}
		if c.killed.Load() {
			return true
		}
	}
	return false
}

// blockingCommands may wait for data for as long as the client asked, or
// forever with a zero timeout.
var blockingCommands = map[string]bool{
	"BLPOP": true, "BRPOP": true, "BLMOVE": true, "BRPOPLPUSH": true, "BLMPOP": true,
	"BZPOPMIN": true, "BZPOPMAX": true, "BZMPOP": true,
	"XREAD": true, "XREADGROUP": true, "WAIT": true,
	"PROMISE.AWAIT": true, "FUTURE.GET": true, "RATELIMITER.WAIT": true, "BULKHEAD.ACQUIRE": true,
	"SEM.ACQUIRE": true,
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
	// Stop reporting the connection: it is gone. The tracker forgets it
	// first, under its lock, so no invalidation reaches the subscriber
	// released below.
	if c.srv != nil && c.srv.tracker != nil {
		c.srv.tracker.forget(c.ID)
	}
	if c.srv != nil {
		c.srv.monitor.remove(c.ID)
	}
	command.UntrackConnection(c.ID)
	// Release the connection's WATCH registrations.
	if c.transaction != nil {
		c.transaction.ClearWatch()
	}
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
	if c.handler != nil {
		c.handler.Closed()
	}
	c.stopDelivery()
	logger.Debug().
		Int64("conn_id", c.ID).
		Msg("client disconnected")
}

// Drain asks the connection to close once its current command, and any
// command already buffered, has run. An idle connection blocked in a read is
// woken at once.
func (c *Connection) Drain() {
	c.draining.Store(true)
	_ = c.conn.SetReadDeadline(time.Now())
}

func (c *Connection) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}

// isProtocolError reports whether a read failed because the client sent
// bytes that are not a valid command, as opposed to an I/O failure.
func isProtocolError(err error) bool {
	switch err {
	case resp.ErrInvalidFormat, resp.ErrInvalidType, resp.ErrLineTooLong,
		resp.ErrUnbalancedQuote, resp.ErrBulkStringTooBig, resp.ErrArrayTooLarge:
		return true
	}
	return false
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
	// Redis checks the arguments before the subscribed context.
	if err := command.ArgumentError(cmd, args); err != nil {
		c.writer.WriteError(err.Error())
		return true
	}
	c.writer.WriteError(command.RejectedReply(cmd, "ERR Can't execute '"+command.FullName(cmd, args)+
		"': only (P|S)SUBSCRIBE / (P|S)UNSUBSCRIBE / PING / QUIT / RESET are allowed in this context"))
	return true
}

// startDelivery starts the goroutine that moves the subscriber's queued
// frames onto the socket. It runs until the connection closes or the
// subscriber is removed (which closes its queue).
func (c *Connection) startDelivery(sub *store.Subscriber) {
	if c.deliveryDone != nil {
		return
	}
	c.pushBuf = bufio.NewWriter(c.sink)
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
	if err := c.writePushFrame(first); err != nil {
		return err
	}
	sub.Delivered(first)
	for i := 1; i < pushBatchLimit; i++ {
		select {
		case f, ok := <-sub.Channel():
			if !ok {
				return c.pushBuf.Flush()
			}
			if err := c.writePushFrame(f); err != nil {
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

// writePushFrame writes one queued out-of-band frame. Frames are encoded once
// as RESP2 arrays and shared between subscribers; a RESP3 client receives
// them as push messages, which differ only in the type byte.
func (c *Connection) writePushFrame(frame []byte) error {
	if len(frame) > 0 && frame[0] == '*' && c.writer.IsRESP3() {
		if err := c.pushBuf.WriteByte('>'); err != nil {
			return err
		}
		frame = frame[1:]
	}
	_, err := c.pushBuf.Write(frame)
	return err
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

// unknownCommandError is the Redis reply for a command the server does not
// know: the name as the client sent it and the start of its arguments.
func unknownCommandError(raw, canonical string, args [][]byte) string {
	name := raw
	if name == "" {
		name = canonical
	}
	if len(name) > 128 {
		name = name[:128]
	}
	var sb strings.Builder
	for _, a := range args {
		if sb.Len() >= 128 {
			break
		}
		room := 128 - sb.Len()
		if len(a) > room {
			a = a[:room]
		}
		sb.WriteByte('\'')
		sb.Write(a)
		sb.WriteString("' ")
	}
	msg := "ERR unknown command '" + name + "', with args beginning with: " + sb.String()
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' {
			return ' '
		}
		return r
	}, msg)
}
