// Package replication implements master-replica replication between
// CacheStorm servers with Redis semantics: a replica connects to its master's
// RESP port, authenticates, announces itself with REPLCONF and asks for the
// stream with PSYNC. The master answers +FULLRESYNC with a snapshot (the
// command stream an AOF rewrite produces, sent as one bulk payload) followed
// by every write it executes, or +CONTINUE with what the replica missed when
// the replica's position is still in the backlog.
//
// The Manager owns both sides. As a master it encodes the writes the router
// reports into the stream, keeps the backlog and feeds each attached replica
// through a bounded queue drained by the replica's own goroutine. As a
// replica it runs the master link: handshake, full or partial resync,
// applying the stream through the router, acknowledging the offset every
// second and reconnecting with backoff when the link drops.
package replication

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Role is the replication role of a server.
type Role int

const (
	RoleMaster Role = iota
	RoleReplica
)

func (r Role) String() string {
	if r == RoleReplica {
		return "slave"
	}
	return "master"
}

// Host is what the Manager needs from the server it runs in.
type Host interface {
	// LockWrites blocks every write, from clients and from the master link,
	// until UnlockWrites. A write that is running finishes first, together
	// with its propagation.
	LockWrites()
	UnlockWrites()
	CommitFullSync(*store.Databases, *command.Router) error
	FunctionPayload() ([]byte, error)
	// ListeningPort is the port a replica announces to its master.
	ListeningPort() int
}

// Config tunes the Manager. Zero values take the defaults.
type Config struct {
	// MasterAuth is the password sent with AUTH to the master, and
	// MasterUser the ACL user it authenticates as (empty: default user).
	MasterAuth string
	MasterUser string
	// ReadOnly makes a replica refuse writes from its clients.
	ReadOnly bool
	// Timeout is how long a link may stay silent before it is dropped.
	Timeout time.Duration
	// BacklogSize is the size of the backlog partial resyncs are served from.
	BacklogSize int
	// ReplicaBufferLimit is the most bytes queued for one replica; a replica
	// that falls further behind is disconnected.
	ReplicaBufferLimit int
	// PingPeriod is how often a master pings its replicas through the stream.
	PingPeriod time.Duration
	// AckPeriod is how often a replica acknowledges its offset.
	AckPeriod time.Duration
	// MaxBackoff caps the delay between two connection attempts.
	MaxBackoff time.Duration
	// AnnounceIP and AnnouncePort override what a replica tells its master.
	AnnounceIP   string
	AnnouncePort int
}

func (c *Config) applyDefaults() {
	if c.Timeout <= 0 {
		c.Timeout = 60 * time.Second
	}
	if c.BacklogSize <= 0 {
		c.BacklogSize = 1 << 20
	}
	if c.ReplicaBufferLimit <= 0 {
		c.ReplicaBufferLimit = 64 << 20
	}
	if c.PingPeriod <= 0 {
		c.PingPeriod = 10 * time.Second
	}
	if c.AckPeriod <= 0 {
		c.AckPeriod = time.Second
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = time.Second
	}
}

// Manager is the replication state of one server.
type Manager struct {
	cfg    Config
	host   Host
	router *command.Router
	dbs    *store.Databases

	mu sync.Mutex
	// role is RoleMaster or RoleReplica.
	role Role
	// replid is the id of the stream this server holds; replid2 the id it
	// held before its last promotion, valid up to secondOffset.
	replid       string
	replid2      string
	secondOffset int64
	// offset counts the bytes of the stream so far.
	offset int64
	// backlog is a ring of the last len(backlog) stream bytes; byte p of the
	// stream sits at backlog[p%len]. histLen is how many are valid. It is
	// nil until a replica first attaches.
	backlog []byte
	histLen int64
	// streamDB is the database the stream is in, -1 when unknown, so the
	// next command is preceded by a SELECT.
	streamDB int
	replicas map[*replica]struct{}
	// feeding is true while the server is a master with a backlog, the only
	// state in which writes enter the stream. It lets the write path skip
	// the Manager's lock on a server that never had a replica.
	feeding atomic.Bool

	// Replica side: the master to follow and the state of the link. linkDB
	// and linkTx are the master client's selected database and transaction,
	// kept across reconnections so a partial resync continues exactly.
	masterHost     string
	masterPort     int
	link           *link
	linkState      string
	linkDB         int
	linkTx         *command.Transaction
	lastIO         time.Time
	linkDownSince  time.Time
	syncInProgress bool
	// cached reports whether replid/offset describe the data this server
	// holds, so a PSYNC can ask to continue from them.
	cached bool

	// Resynchronization counters, as INFO reports them.
	syncFull       int64
	syncPartialOK  int64
	syncPartialErr int64

	ackMu sync.Mutex
	ackCh chan struct{}

	stopCh  chan struct{}
	stopped bool
	wg      sync.WaitGroup
}

// New builds a Manager for a server that starts as a master.
func New(cfg Config, host Host, router *command.Router, dbs *store.Databases) *Manager {
	cfg.applyDefaults()
	return &Manager{
		cfg:          cfg,
		host:         host,
		router:       router,
		dbs:          dbs,
		role:         RoleMaster,
		replid:       NewReplID(),
		replid2:      zeroReplID,
		secondOffset: -1,
		streamDB:     -1,
		replicas:     make(map[*replica]struct{}),
		linkState:    "connect",
		linkTx:       command.NewTransaction(),
		cached:       true,
		ackCh:        make(chan struct{}),
		stopCh:       make(chan struct{}),
	}
}

const zeroReplID = "0000000000000000000000000000000000000000"

// NewReplID returns a random 40-character replication id.
func NewReplID() string {
	var b [20]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b[:])
}

// Start runs the master's replica pinger.
func (m *Manager) Start() {
	m.wg.Add(1)
	go m.pingLoop()
}

// Stop disconnects every replica, stops the master link and waits for the
// Manager's goroutines.
func (m *Manager) Stop() {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = true
	close(m.stopCh)
	l := m.link
	m.link = nil
	replicas := m.detachAllLocked()
	m.mu.Unlock()
	for _, r := range replicas {
		r.close()
	}
	if l != nil {
		l.stopAndWait()
	}
	m.wg.Wait()
}

// DisconnectReplicas closes the connection of every attached replica; each
// reconnects and resynchronizes.
func (m *Manager) DisconnectReplicas() {
	m.mu.Lock()
	replicas := m.detachAllLocked()
	m.mu.Unlock()
	for _, r := range replicas {
		r.close()
	}
}

// SyncStats returns how many full resynchronizations this server served, and
// how many partial resynchronization requests it accepted and refused.
func (m *Manager) SyncStats() (full, partialOK, partialErr int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.syncFull, m.syncPartialOK, m.syncPartialErr
}

// Role returns the current role.
func (m *Manager) Role() Role {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.role
}

// ReadOnly reports whether client writes must be refused: the server is a
// read-only replica.
func (m *Manager) ReadOnly() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.role == RoleReplica && m.cfg.ReadOnly
}

// ReplID returns the current replication id and offset.
func (m *Manager) ReplID() (string, int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.replid, m.offset
}

func (m *Manager) MarkLocalWrite() {
	m.mu.Lock()
	if m.role != RoleReplica {
		m.mu.Unlock()
		return
	}
	m.cached = false
	replicas := m.detachAllLocked()
	m.mu.Unlock()
	for _, r := range replicas {
		r.close()
	}
}

// Feed records a write a client executed in database db.
func (m *Manager) Feed(db int, name string, args [][]byte) {
	m.feed([]command.ExecutedCommand{{Name: name, Args: args, DB: db}}, false)
}

// FeedTransaction records the writes an EXEC (or a script) executed, as one
// MULTI ... EXEC block.
func (m *Manager) FeedTransaction(cmds []command.ExecutedCommand) {
	if len(cmds) == 0 {
		return
	}
	m.feed(cmds, true)
}

// FeedDelete records the removal of key from store s, for an expiry or an
// eviction. It is called with the key's shard lock held, so it only touches
// the Manager's own state.
func (m *Manager) FeedDelete(s *store.Store, key string) {
	if !m.feeding.Load() {
		return
	}
	db := m.dbIndex(s)
	if db < 0 {
		return
	}
	m.feed([]command.ExecutedCommand{{Name: "DEL", Args: [][]byte{[]byte(key)}, DB: db}}, false)
}

func (m *Manager) dbIndex(s *store.Store) int {
	for i := 0; i < m.dbs.Count(); i++ {
		if m.dbs.Existing(i) == s {
			return i
		}
	}
	return -1
}

func (m *Manager) feed(cmds []command.ExecutedCommand, tx bool) {
	if !m.feeding.Load() {
		return
	}
	m.mu.Lock()
	if m.role != RoleMaster || m.backlog == nil || m.stopped {
		m.mu.Unlock()
		return
	}
	var buf []byte
	if tx {
		buf = appendCommand(buf, "MULTI", nil)
	}
	for _, c := range cmds {
		if c.DB != m.streamDB {
			buf = appendCommand(buf, "SELECT", [][]byte{strconv.AppendInt(nil, int64(c.DB), 10)})
			m.streamDB = c.DB
		}
		buf = appendCommand(buf, c.Name, c.Args)
	}
	if tx {
		buf = appendCommand(buf, "EXEC", nil)
	}
	dropped := m.appendLocked(buf)
	m.mu.Unlock()
	for _, r := range dropped {
		r.close()
	}
}

// feedRaw appends already encoded bytes to the stream of a master.
func (m *Manager) feedRaw(b []byte) {
	m.mu.Lock()
	if m.role != RoleMaster || m.backlog == nil || m.stopped {
		m.mu.Unlock()
		return
	}
	dropped := m.appendLocked(b)
	m.mu.Unlock()
	for _, r := range dropped {
		r.close()
	}
}

// appendLocked adds b to the stream: the offset, the backlog and the queue
// of every attached replica. It returns the replicas whose queue overflowed;
// the caller closes them once the lock is released.
func (m *Manager) appendLocked(b []byte) []*replica {
	if len(b) == 0 {
		return nil
	}
	if m.backlog != nil {
		size := int64(len(m.backlog))
		data := b
		start := m.offset
		if int64(len(data)) > size {
			start += int64(len(data)) - size
			data = data[int64(len(data))-size:]
		}
		pos := start % size
		n := copy(m.backlog[pos:], data)
		copy(m.backlog, data[n:])
		m.histLen += int64(len(b))
		if m.histLen > size {
			m.histLen = size
		}
	}
	m.offset += int64(len(b))
	var dropped []*replica
	for r := range m.replicas {
		if !r.enqueue(b, m.cfg.ReplicaBufferLimit) {
			delete(m.replicas, r)
			dropped = append(dropped, r)
		}
	}
	if len(dropped) > 0 {
		for _, r := range dropped {
			logger.Warn().Str("replica", r.addr).
				Int("limit", m.cfg.ReplicaBufferLimit).
				Msg("disconnecting replica: output buffer limit reached")
		}
	}
	return dropped
}

// createBacklogLocked starts a backlog at the current offset, if there is
// none.
func (m *Manager) createBacklogLocked() {
	if m.backlog == nil {
		m.backlog = make([]byte, m.cfg.BacklogSize)
		m.histLen = 0
	}
	m.feeding.Store(m.role == RoleMaster)
}

// backlogFromLocked returns the stream bytes from position from to the end,
// and whether the backlog still holds them.
func (m *Manager) backlogFromLocked(from int64) ([]byte, bool) {
	if m.backlog == nil || from > m.offset || from < m.offset-m.histLen {
		return nil, false
	}
	n := m.offset - from
	out := make([]byte, n)
	size := int64(len(m.backlog))
	pos := from % size
	c := copy(out, m.backlog[pos:])
	copy(out[c:], m.backlog)
	return out, true
}

// detachAllLocked removes every replica and returns them for closing.
func (m *Manager) detachAllLocked() []*replica {
	out := make([]*replica, 0, len(m.replicas))
	for r := range m.replicas {
		out = append(out, r)
		delete(m.replicas, r)
	}
	return out
}

func (m *Manager) pingLoop() {
	defer m.wg.Done()
	t := time.NewTicker(m.cfg.PingPeriod)
	defer t.Stop()
	ping := encodeCommand("PING")
	for {
		select {
		case <-m.stopCh:
			return
		case <-t.C:
			m.mu.Lock()
			n := len(m.replicas)
			m.mu.Unlock()
			if n > 0 {
				m.feedRaw(ping)
			}
		}
	}
}

// notifyAck wakes the WAIT callers.
func (m *Manager) notifyAck() {
	m.ackMu.Lock()
	close(m.ackCh)
	m.ackCh = make(chan struct{})
	m.ackMu.Unlock()
}

func (m *Manager) ackChan() <-chan struct{} {
	m.ackMu.Lock()
	defer m.ackMu.Unlock()
	return m.ackCh
}

// ackedReplicas counts the online replicas that acknowledged offset.
func (m *Manager) ackedReplicas(offset int64) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for r := range m.replicas {
		if r.online() && r.ackOffset.Load() >= offset {
			n++
		}
	}
	return n
}

// Wait blocks until numReplicas replicas acknowledged every write made
// before the call, timeout passes (0 waits forever) or done is closed, and
// returns how many did.
func (m *Manager) Wait(numReplicas int, timeout time.Duration, done <-chan struct{}) int {
	m.mu.Lock()
	target := m.offset
	m.mu.Unlock()
	if n := m.ackedReplicas(target); n >= numReplicas {
		return n
	}
	m.feedRaw(encodeCommand("REPLCONF", "GETACK", "*"))
	var expire <-chan time.Time
	if timeout > 0 {
		t := time.NewTimer(timeout)
		defer t.Stop()
		expire = t.C
	}
	for {
		ch := m.ackChan()
		n := m.ackedReplicas(target)
		if n >= numReplicas {
			return n
		}
		select {
		case <-ch:
		case <-expire:
			return m.ackedReplicas(target)
		case <-done:
			return m.ackedReplicas(target)
		case <-m.stopCh:
			return m.ackedReplicas(target)
		}
	}
}

// State is the part of a replica's replication state that survives a
// restart, so the replica can continue from its master's backlog.
type State struct {
	MasterHost string
	MasterPort int
	ReplID     string
	Offset     int64
	DB         int
}

// SavedState returns the state to persist next to the data at shutdown, and
// false when there is nothing safe to persist: the server is not a synced
// replica, or the master link stopped in the middle of a transaction.
func (m *Manager) SavedState() (State, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.role != RoleReplica || !m.cached || m.linkTx.IsActive() {
		return State{}, false
	}
	return State{ReplID: m.replid, Offset: m.offset, DB: m.linkDB, MasterHost: m.masterHost, MasterPort: m.masterPort}, true
}

// RestoreState adopts a state saved by SavedState for the dataset the server
// loaded, so the next PSYNC asks to continue from it.
func (m *Manager) RestoreState(st State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.replid = st.ReplID
	m.offset = st.Offset
	m.linkDB = st.DB
	m.streamDB = st.DB
	m.replid2 = zeroReplID
	m.secondOffset = -1
	m.cached = true
}

// discardWriter is a reply writer for commands whose replies nobody reads.
func discardWriter() *resp.Writer {
	return resp.NewWriter(io.Discard)
}
