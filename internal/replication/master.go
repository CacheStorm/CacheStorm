package replication

import (
	"bytes"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/store"
)

// capaCacheStorm is the REPLCONF capa a CacheStorm replica announces to
// take the full resynchronization as CacheStorm's command stream.
const capaCacheStorm = "cachestorm"

// Replica states, as INFO reports them.
const (
	stateWaitBgsave = "wait_bgsave"
	stateSendBulk   = "send_bulk"
	stateOnline     = "online"
)

// replica is one replica attached to this server. Its queue is filled under
// the Manager's lock and drained by the replica's own goroutine, which does
// the socket writes without holding the Manager's lock.
type replica struct {
	m    *Manager
	conn net.Conn
	addr string
	ip   string
	port int
	// cachestorm is set for a replica that announced REPLCONF capa
	// cachestorm: it takes the snapshot as CacheStorm's command stream,
	// which carries what Redis' RDB cannot. Others get an RDB in Redis'
	// format.
	cachestorm bool

	mu      sync.Mutex
	cond    *sync.Cond
	pending []byte
	closed  bool
	state   string

	ackOffset atomic.Int64
	lastAck   atomic.Int64
	closeOnce sync.Once
}

// enqueue queues b for the replica. It returns false, and marks the replica
// closed, when the queue would exceed limit.
func (r *replica) enqueue(b []byte, limit int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return true
	}
	if len(r.pending)+len(b) > limit {
		r.closed = true
		r.cond.Broadcast()
		return false
	}
	r.pending = append(r.pending, b...)
	r.cond.Signal()
	return true
}

func (r *replica) online() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state == stateOnline
}

func (r *replica) setState(s string) {
	r.mu.Lock()
	r.state = s
	r.mu.Unlock()
}

func (r *replica) getState() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

// close detaches the replica and closes its connection; the writer goroutine
// and the connection's read loop then end.
func (r *replica) close() {
	r.closeOnce.Do(func() {
		r.m.mu.Lock()
		delete(r.m.replicas, r)
		r.m.mu.Unlock()
		r.mu.Lock()
		r.closed = true
		r.cond.Broadcast()
		r.mu.Unlock()
		_ = r.conn.Close()
		logger.Info().Str("replica", r.addr).Msg("replica disconnected")
	})
}

// Command handles what the replica sends after PSYNC: REPLCONF ACK.
func (r *replica) Command(cmd string, args [][]byte) {
	if !strings.EqualFold(cmd, "REPLCONF") || len(args) < 2 {
		return
	}
	if !strings.EqualFold(string(args[0]), "ACK") {
		return
	}
	off, err := strconv.ParseInt(string(args[1]), 10, 64)
	if err != nil {
		return
	}
	r.ackOffset.Store(off)
	r.lastAck.Store(time.Now().UnixNano())
	r.m.notifyAck()
}

// Closed is called when the replica's connection is gone.
func (r *replica) Closed() {
	r.close()
}

// writeLoop sends the queued stream until the replica is closed.
func (r *replica) writeLoop() {
	var buf []byte
	for {
		r.mu.Lock()
		for len(r.pending) == 0 && !r.closed {
			r.cond.Wait()
		}
		if r.closed {
			r.mu.Unlock()
			return
		}
		buf, r.pending = r.pending, buf[:0]
		r.mu.Unlock()
		if err := r.write(buf); err != nil {
			logger.Warn().Err(err).Str("replica", r.addr).Msg("write to replica failed")
			r.close()
			return
		}
	}
}

func (r *replica) write(b []byte) error {
	_ = r.conn.SetWriteDeadline(time.Now().Add(r.m.cfg.Timeout))
	_, err := r.conn.Write(b)
	return err
}

// syncRequest is a PSYNC (or SYNC) a connection made.
type syncRequest struct {
	replid string
	offset int64
	// legacy is SYNC: no +FULLRESYNC line, always a full resync.
	legacy bool
}

// attach takes over conn for a replica that asked for the stream. It answers
// +CONTINUE with the missing bytes when the backlog has them, and otherwise
// sends a full resync from the replica's goroutine.
func (m *Manager) attach(conn net.Conn, conf *command.ReplicaConf, req syncRequest) command.ConnHandler {
	r := &replica{m: m, conn: conn, addr: conn.RemoteAddr().String()}
	r.cond = sync.NewCond(&r.mu)
	r.ip = hostOf(r.addr)
	if conf != nil {
		r.port = conf.ListeningPort
		if conf.IP != "" {
			r.ip = conf.IP
		}
		r.cachestorm = slices.Contains(conf.Capa, capaCacheStorm)
	}
	r.lastAck.Store(time.Now().UnixNano())

	m.mu.Lock()
	if m.stopped || (m.role == RoleReplica && !m.cached) {
		m.mu.Unlock()
		_ = conn.Close()
		return r
	}
	if !req.legacy {
		data, ok := m.continuableLocked(req)
		if !ok && req.replid != "?" {
			m.syncPartialErr++
		}
		if ok {
			m.syncPartialOK++
			r.state = stateOnline
			r.pending = append([]byte("+CONTINUE "+m.replid+"\r\n"), data...)
			r.ackOffset.Store(req.offset - 1)
			m.replicas[r] = struct{}{}
			m.mu.Unlock()
			logger.Info().Str("replica", r.addr).Int64("offset", req.offset-1).
				Int("bytes", len(data)).Msg("partial resynchronization accepted")
			m.wg.Add(1)
			go func() {
				defer m.wg.Done()
				r.writeLoop()
			}()
			return r
		}
	}
	r.state = stateWaitBgsave
	m.syncFull++
	m.mu.Unlock()

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		if err := m.fullSync(r, req.legacy); err != nil {
			logger.Warn().Err(err).Str("replica", r.addr).Msg("full resynchronization failed")
			r.close()
			return
		}
		r.writeLoop()
	}()
	return r
}

// continuableLocked reports whether req can continue from the backlog and
// returns the bytes the replica is missing.
func (m *Manager) continuableLocked(req syncRequest) ([]byte, bool) {
	if req.replid == "" || req.replid == "?" || req.offset < 1 {
		return nil, false
	}
	from := req.offset - 1
	switch {
	case req.replid == m.replid:
	case req.replid == m.replid2 && m.secondOffset >= 0 && req.offset <= m.secondOffset:
	default:
		return nil, false
	}
	return m.backlogFromLocked(from)
}

// fullSync snapshots the dataset with writes locked, attaches the replica at
// the snapshot's offset, and sends +FULLRESYNC and the snapshot.
func (m *Manager) fullSync(r *replica, legacy bool) error {
	var snap bytes.Buffer
	replid, offset, err := m.snapshotFor(r, &snap)
	if err != nil {
		return err
	}

	var header []byte
	if !legacy {
		header = append(header, "+FULLRESYNC "+replid+" "+strconv.FormatInt(offset, 10)+"\r\n"...)
	}
	header = append(header, '$')
	header = strconv.AppendInt(header, int64(snap.Len()), 10)
	header = append(header, '\r', '\n')
	if err := r.write(header); err != nil {
		return err
	}
	if err := r.write(snap.Bytes()); err != nil {
		return err
	}
	r.ackOffset.Store(offset)
	r.setState(stateOnline)
	logger.Info().Str("replica", r.addr).Int64("offset", offset).
		Int("bytes", snap.Len()).Msg("full resynchronization sent")
	return nil
}

// snapshotFor encodes the dataset for r with writes locked and attaches r
// at the snapshot's offset. A CacheStorm replica gets the command stream an
// AOF rewrite writes, ending by selecting the stream's database; any other
// gets an RDB in Redis' format, whose aux fields name the stream's position
// and database. It returns that position.
func (m *Manager) snapshotFor(r *replica, snap *bytes.Buffer) (string, int64, error) {
	m.host.LockWrites()
	defer m.host.UnlockWrites()
	m.mu.Lock()
	invalid := m.role == RoleReplica && !m.cached
	replicaSource := m.role == RoleReplica
	m.mu.Unlock()
	if invalid {
		return "", 0, fmt.Errorf("replica has no synchronized dataset")
	}
	functions, err := m.host.FunctionPayload()
	if err != nil {
		return "", 0, err
	}
	if replicaSource {
		m.dbs.BeginAuthoritativeApply()
		defer m.dbs.EndAuthoritativeApply()
	}
	if r.cachestorm {
		if err := persistence.WriteSnapshot(snap, DBSource{DBs: m.dbs, Functions: functions, Search: m.host.SearchCommands()}); err != nil {
			return "", 0, err
		}
	}
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return "", 0, fmt.Errorf("server is stopping")
	}
	m.createBacklogLocked()
	replid, offset := m.replid, m.offset
	db := m.streamDB
	if m.role == RoleReplica {
		db = m.linkDB
	}
	r.state = stateSendBulk
	m.replicas[r] = struct{}{}
	m.mu.Unlock()
	if r.cachestorm {
		if db > 0 {
			snap.Write(encodeCommand("SELECT", strconv.Itoa(db)))
		}
		return replid, offset, nil
	}
	skipped, err := persistence.WriteRedisRDB(snap, DBSource{DBs: m.dbs}, persistence.RedisRDBInfo{
		Functions: persistence.FunctionCodes(functions), StreamDB: db, ReplID: replid, Offset: offset})
	if skipped > 0 {
		logger.Warn().Int("keys", skipped).Str("replica", r.addr).Msg("keys Redis has no type for were left out of the RDB")
	}
	return replid, offset, err
}

func hostOf(addr string) string {
	h, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return h
}

// DBSource exposes databases as a snapshot source: every database, each as
// its live entries.
type DBSource struct {
	DBs       *store.Databases
	Functions []byte
	// Search holds the commands that rebuild the search indexes.
	Search [][]string
}

func (d DBSource) FunctionPayload() []byte { return d.Functions }

func (d DBSource) SearchCommands() [][]string { return d.Search }

// ForEachEntry implements persistence.EntryStreamer.
func (d DBSource) ForEachEntry(fn func(db int, key string, entry interface{}) error) error {
	for i := 0; i < d.DBs.Count(); i++ {
		s := d.DBs.Existing(i)
		if s == nil {
			continue
		}
		db := i
		if err := s.ForEachLive(func(key string, e *store.Entry) error {
			return fn(db, key, e)
		}); err != nil {
			return err
		}
	}
	return nil
}

// DatabaseEntries implements persistence.MultiDBSource.
func (d DBSource) DatabaseEntries() []map[string]interface{} {
	out := make([]map[string]interface{}, d.DBs.Count())
	for i := range out {
		entries := make(map[string]interface{})
		if s := d.DBs.Existing(i); s != nil {
			for k, e := range s.GetAll() {
				entries[k] = e
			}
		}
		out[i] = entries
	}
	return out
}
