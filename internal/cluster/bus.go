package cluster

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/logger"
)

const (
	cronInterval     = 100 * time.Millisecond
	minDialBackoff   = 100 * time.Millisecond
	maxDialBackoff   = 2 * time.Second
	sendBufferLimit  = 64 << 20
	blacklistTTL     = 60 * time.Second
	failReportMult   = 2
	failUndoMult     = 2
	pongClockSkewMs  = 500
	minHandshakeTime = time.Second
)

func nowMs() int64 { return time.Now().UnixMilli() }

// BusOptions configure the cluster bus of a node.
type BusOptions struct {
	// Listener accepts the bus connections of the other nodes.
	Listener    net.Listener
	NodeTimeout time.Duration
	// Publish delivers a PUBLISH that another node broadcast.
	Publish func(channel string, message []byte)
	// KeysInSlot counts this node's keys in a slot; DelKeysInSlot deletes
	// them, when the slot is found to belong to another node.
	KeysInSlot    func(slot int) int
	DelKeysInSlot func(slot int)

	// SetMaster points replication at the master at ip:port, or, with port
	// 0, stops replicating and keeps the data, as REPLICAOF NO ONE does.
	SetMaster func(ip string, port int)
	// ReplOffset is this node's replication offset; ReplDataAge how long it
	// has gone without data from its master.
	ReplOffset  func() int64
	ReplDataAge func() time.Duration
	// PauseWrites pauses client writes until the given time and returns
	// once the writes in progress are done; UnpauseWrites resumes them.
	PauseWrites   func(until time.Time)
	UnpauseWrites func()
	// ReplicaValidityFactor is cluster-replica-validity-factor and
	// ReplPingPeriod repl-ping-replica-period, for the data age check.
	ReplicaValidityFactor int
	ReplPingPeriod        time.Duration
}

type bus struct {
	opts      BusOptions
	timeout   int64
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	stopped   bool
	inbound   map[*link]struct{}
	iteration int
	delSlots  []int
	published []*message
	roleWake  chan struct{}
}

type msgStats struct {
	sent, recv    [msgTypeCount]uint64
	limitExceeded uint64
}

func (s *msgStats) write(b *strings.Builder) {
	var total uint64
	for i, n := range s.sent {
		if n > 0 {
			fmt.Fprintf(b, "cluster_stats_messages_%s_sent:%d\r\n", msgTypeNames[i], n)
			total += n
		}
	}
	fmt.Fprintf(b, "cluster_stats_messages_sent:%d\r\n", total)
	total = 0
	for i, n := range s.recv {
		if n > 0 {
			fmt.Fprintf(b, "cluster_stats_messages_%s_received:%d\r\n", msgTypeNames[i], n)
			total += n
		}
	}
	fmt.Fprintf(b, "cluster_stats_messages_received:%d\r\n", total)
	fmt.Fprintf(b, "total_cluster_links_buffer_limit_exceeded:%d\r\n", s.limitExceeded)
}

// link is one TCP connection of the bus. An outbound link is the one this
// node opened to node; an inbound link was accepted, and node is set once
// the peer is known. Its fields other than the send queue are guarded by the
// cluster mutex.
type link struct {
	node      *Node
	inbound   bool
	conn      net.Conn
	connected bool
	closed    bool
	ctime     int64
	done      chan struct{}

	qmu    sync.Mutex
	q      [][]byte
	qbytes int64
	wake   chan struct{}
}

func newLink(n *Node, inbound bool) *link {
	return &link{node: n, inbound: inbound, ctime: nowMs(), done: make(chan struct{}), wake: make(chan struct{}, 1)}
}

func (l *link) up() bool { return l != nil && l.connected && !l.closed }

func (l *link) queued() int64 {
	l.qmu.Lock()
	defer l.qmu.Unlock()
	return l.qbytes
}

// enqueue queues a frame for the writer; false means the send buffer limit
// was reached.
func (l *link) enqueue(frame []byte) bool {
	l.qmu.Lock()
	if l.qbytes+int64(len(frame)) > sendBufferLimit {
		l.qmu.Unlock()
		return false
	}
	l.q = append(l.q, frame)
	l.qbytes += int64(len(frame))
	l.qmu.Unlock()
	select {
	case l.wake <- struct{}{}:
	default:
	}
	return true
}

func (l *link) take() [][]byte {
	l.qmu.Lock()
	defer l.qmu.Unlock()
	q := l.q
	l.q, l.qbytes = nil, 0
	return q
}

// closeLocked closes the link and detaches it from its node.
func (c *Cluster) closeLinkLocked(l *link) {
	if l == nil || l.closed {
		return
	}
	l.closed = true
	close(l.done)
	if l.conn != nil {
		_ = l.conn.Close()
	}
	if n := l.node; n != nil {
		if n.link == l {
			n.link = nil
		}
		if n.inbound == l {
			n.inbound = nil
		}
	}
	if c.bus != nil {
		delete(c.bus.inbound, l)
	}
}

func (c *Cluster) closeLink(l *link) {
	c.mu.Lock()
	c.closeLinkLocked(l)
	c.mu.Unlock()
}

// sendLocked queues frame on l, closing a link whose buffer is full.
func (c *Cluster) sendLocked(l *link, typ msgType, frame []byte) {
	if !l.up() {
		return
	}
	if !l.enqueue(frame) {
		c.stats.limitExceeded++
		logger.Warn().Msg("cluster bus: a link exceeded its send buffer limit, closing it")
		c.closeLinkLocked(l)
		return
	}
	c.stats.sent[typ]++
}

// StartBus starts the cluster bus on opts.Listener: the gossip cron, the
// accept loop and a link to every known node.
func (c *Cluster) StartBus(opts BusOptions) error {
	if opts.Listener == nil || opts.NodeTimeout <= 0 {
		return errors.New("cluster bus: a listener and a node timeout are required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.bus != nil {
		return errors.New("cluster bus: already started")
	}
	b := &bus{opts: opts, timeout: opts.NodeTimeout.Milliseconds(), inbound: make(map[*link]struct{}), roleWake: make(chan struct{}, 1)}
	if b.timeout < 1 {
		b.timeout = 1
	}
	b.ctx, b.cancel = context.WithCancel(context.Background())
	now := nowMs()
	for _, n := range c.nodes {
		if n.PingSent != 0 {
			n.PingSent = now
		}
		if n.PongRecv != 0 {
			n.PongRecv = now
		}
		n.ctime = now
	}
	c.bus = b
	c.roleSet = false
	b.roleWake <- struct{}{}
	b.wg.Add(3)
	go c.acceptLoop(b)
	go c.cronLoop(b)
	go c.roleLoop(b)
	return nil
}

// StopBus stops the bus: the listener, the cron and every link, and waits
// for their goroutines.
func (c *Cluster) StopBus() {
	c.mu.Lock()
	b := c.bus
	if b == nil || b.stopped {
		c.mu.Unlock()
		return
	}
	b.stopped = true
	b.cancel()
	_ = b.opts.Listener.Close()
	for _, n := range c.nodes {
		c.closeLinkLocked(n.link)
		c.closeLinkLocked(n.inbound)
	}
	for l := range b.inbound {
		c.closeLinkLocked(l)
	}
	c.mu.Unlock()
	b.wg.Wait()
	c.mu.Lock()
	c.bus = nil
	c.mu.Unlock()
}

// BusAddr returns the address the bus listens on, empty when it is off.
func (c *Cluster) BusAddr() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.bus == nil {
		return ""
	}
	return c.bus.opts.Listener.Addr().String()
}

func (c *Cluster) acceptLoop(b *bus) {
	defer b.wg.Done()
	for {
		conn, err := b.opts.Listener.Accept()
		if err != nil {
			c.mu.RLock()
			stopped := b.stopped
			c.mu.RUnlock()
			if stopped || errors.Is(err, net.ErrClosed) {
				return
			}
			logger.Warn().Err(err).Msg("cluster bus: accept failed")
			select {
			case <-b.ctx.Done():
				return
			case <-time.After(50 * time.Millisecond):
			}
			continue
		}
		c.mu.Lock()
		if b.stopped {
			c.mu.Unlock()
			_ = conn.Close()
			return
		}
		l := newLink(nil, true)
		l.conn, l.connected = conn, true
		b.inbound[l] = struct{}{}
		b.wg.Add(2)
		c.mu.Unlock()
		go c.readLoop(b, l)
		go func() {
			defer b.wg.Done()
			c.writeLoop(b, l)
		}()
	}
}

func (b *bus) readTimeout() time.Duration {
	return time.Duration(2*b.timeout)*time.Millisecond + time.Second
}

func (b *bus) writeTimeout() time.Duration {
	d := time.Duration(b.timeout) * time.Millisecond
	if d < time.Second {
		d = time.Second
	}
	return d
}

func (b *bus) maxBackoff() time.Duration {
	d := time.Duration(b.timeout/2) * time.Millisecond
	if d < minDialBackoff {
		d = minDialBackoff
	}
	if d > maxDialBackoff {
		d = maxDialBackoff
	}
	return d
}

func (c *Cluster) readLoop(b *bus, l *link) {
	defer b.wg.Done()
	defer c.closeLink(l)
	r := bufio.NewReaderSize(l.conn, 16<<10)
	for {
		_ = l.conn.SetReadDeadline(time.Now().Add(b.readTimeout()))
		frame, err := readFrame(r)
		if err != nil {
			return
		}
		m, err := decodeMessage(frame)
		if err != nil {
			logger.Warn().Err(err).Str("peer", l.conn.RemoteAddr().String()).Msg("cluster bus: closing a link after a bad frame")
			return
		}
		c.processPacket(b, l, m)
	}
}

func (c *Cluster) writeLoop(b *bus, l *link) {
	for {
		select {
		case <-l.done:
			return
		case <-l.wake:
		}
		batch := l.take()
		if len(batch) == 0 {
			continue
		}
		_ = l.conn.SetWriteDeadline(time.Now().Add(b.writeTimeout()))
		bufs := net.Buffers(batch)
		if _, err := bufs.WriteTo(l.conn); err != nil {
			c.closeLink(l)
			return
		}
	}
}

// connectLocked opens the outbound link to n.
func (c *Cluster) connectLocked(b *bus, n *Node) {
	l := newLink(n, false)
	n.link = l
	addr := net.JoinHostPort(n.IP, strconv.Itoa(n.CPort))
	b.wg.Add(1)
	go c.dial(b, l, addr)
}

func (c *Cluster) dial(b *bus, l *link, addr string) {
	defer b.wg.Done()
	d := net.Dialer{Timeout: b.writeTimeout()}
	conn, err := d.DialContext(b.ctx, "tcp", addr)
	c.mu.Lock()
	n := l.node
	if err != nil {
		if !l.closed {
			c.closeLinkLocked(l)
			now := nowMs()
			if n.PingSent == 0 {
				n.PingSent = now
			}
			if n.dialBackoff < minDialBackoff {
				n.dialBackoff = minDialBackoff
			}
			n.nextDial = now + n.dialBackoff.Milliseconds()
			if n.dialBackoff *= 2; n.dialBackoff > b.maxBackoff() {
				n.dialBackoff = b.maxBackoff()
			}
		}
		c.mu.Unlock()
		return
	}
	if l.closed || b.stopped {
		c.mu.Unlock()
		_ = conn.Close()
		return
	}
	l.conn, l.connected = conn, true
	n.dialBackoff = 0
	oldPing := n.PingSent
	typ := msgPing
	if n.meet {
		typ = msgMeet
	}
	c.sendPingLocked(l, typ)
	if oldPing != 0 {
		n.PingSent = oldPing
	}
	n.meet = false
	b.wg.Add(1)
	c.mu.Unlock()
	go c.readLoop(b, l)
	c.writeLoop(b, l)
}

func (c *Cluster) cronLoop(b *bus) {
	defer b.wg.Done()
	t := time.NewTicker(cronInterval)
	defer t.Stop()
	for {
		select {
		case <-b.ctx.Done():
			return
		case <-t.C:
			c.cron(b)
		}
	}
}

// LinkView is one entry of CLUSTER LINKS.
type LinkView struct {
	Direction  string
	Node       string
	CreateTime int64
	Events     string
	SendBuffer int64
}

// Links returns the bus links with a known peer, as CLUSTER LINKS reports.
func (c *Cluster) Links() []LinkView {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []LinkView
	for _, n := range c.sortedNodesLocked() {
		for _, l := range []*link{n.link, n.inbound} {
			if !l.up() {
				continue
			}
			v := LinkView{Direction: "to", Node: n.ID, CreateTime: l.ctime, Events: "r", SendBuffer: l.queued()}
			if l.inbound {
				v.Direction = "from"
			}
			if v.SendBuffer > 0 {
				v.Events = "rw"
			}
			out = append(out, v)
		}
	}
	return out
}
