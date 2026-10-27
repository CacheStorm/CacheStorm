package replication

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/logger"
)

// Master link states, as ROLE reports them.
const (
	linkConnect    = "connect"
	linkConnecting = "connecting"
	linkSync       = "sync"
	linkConnected  = "connected"
)

// ErrAlreadyConnected is returned by ReplicaOf when the server already
// follows that master.
var ErrAlreadyConnected = errors.New("already connected to specified master")

// link is the connection of a replica to its master. One goroutine runs it:
// connect, handshake, resync, apply the stream, and start over with backoff
// when anything fails.
type link struct {
	m    *Manager
	host string
	port int

	stop chan struct{}
	done chan struct{}

	mu   sync.Mutex
	conn net.Conn
}

func (l *link) setConn(c net.Conn) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	select {
	case <-l.stop:
		return false
	default:
	}
	l.conn = c
	return true
}

func (l *link) stopAndWait() {
	l.mu.Lock()
	select {
	case <-l.stop:
	default:
		close(l.stop)
	}
	if l.conn != nil {
		_ = l.conn.Close()
	}
	l.mu.Unlock()
	<-l.done
}

func (l *link) stopped() bool {
	select {
	case <-l.stop:
		return true
	default:
		return false
	}
}

// ReplicaOf makes the server a replica of host:port. Its replicas are
// disconnected; the link starts with a partial resync from the server's own
// stream position, which the new master accepts if it shares the history.
func (m *Manager) ReplicaOf(host string, port int) error {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return errors.New("server is stopping")
	}
	if m.role == RoleReplica && m.masterHost == host && m.masterPort == port {
		m.mu.Unlock()
		return ErrAlreadyConnected
	}
	old := m.link
	m.link = nil
	m.mu.Unlock()
	if old != nil {
		old.stopAndWait()
	}

	m.host.LockWrites()
	m.mu.Lock()
	if m.role == RoleMaster {
		// The server's own stream is the history it brings to the new
		// master: the stream database is where the master client starts.
		m.linkDB = m.streamDB
		if m.linkDB < 0 {
			m.linkDB = 0
		}
		m.linkTx = command.NewTransaction()
		m.cached = true
	}
	m.role = RoleReplica
	m.dbs.SetPassiveExpiry(true)
	m.feeding.Store(false)
	m.masterHost, m.masterPort = host, port
	m.linkState = linkConnect
	m.linkDownSince = time.Now()
	replicas := m.detachAllLocked()
	l := &link{m: m, host: host, port: port, stop: make(chan struct{}), done: make(chan struct{})}
	m.link = l
	m.mu.Unlock()
	m.host.UnlockWrites()

	// A replica never expires keys itself: the master streams a DEL for
	// every key that expires, in order with the other writes.
	m.dbs.StopExpiry()
	for _, r := range replicas {
		r.close()
	}
	logger.Info().Str("master", net.JoinHostPort(host, strconv.Itoa(port))).Msg("replicating from master")
	go l.run()
	return nil
}

// Promote makes a replica a master that keeps its data: REPLICAOF NO ONE. The
// stream gets a new id; the old one stays valid up to the current offset, so
// the other replicas of the former master can continue from this server.
func (m *Manager) Promote() {
	m.mu.Lock()
	if m.role == RoleMaster || m.stopped {
		m.mu.Unlock()
		return
	}
	l := m.link
	m.link = nil
	m.mu.Unlock()
	if l != nil {
		l.stopAndWait()
	}

	m.host.LockWrites()
	m.mu.Lock()
	if m.linkTx.IsActive() {
		// The master link stopped inside a transaction: its replicas received
		// the MULTI and the queued commands, which must not run.
		m.linkTx = command.NewTransaction()
		m.appendLocked(encodeCommand("DISCARD"))
	}
	m.replid2 = m.replid
	m.secondOffset = m.offset + 1
	if !m.cached {
		m.replid2 = zeroReplID
		m.secondOffset = -1
		if m.backlog != nil {
			m.backlog = nil
			m.createBacklogLocked()
		}
	}
	m.replid = NewReplID()
	m.role = RoleMaster
	m.feeding.Store(m.backlog != nil)
	m.streamDB = m.linkDB
	m.masterHost, m.masterPort = "", 0
	m.linkState = linkConnect
	m.syncInProgress = false
	m.cached = true
	m.mu.Unlock()
	m.dbs.SetPassiveExpiry(false)
	m.host.UnlockWrites()
	m.dbs.StartExpiry()
	logger.Info().Msg("promoted to master")
}

func (l *link) run() {
	defer close(l.done)
	m := l.m
	backoff := 50 * time.Millisecond
	for {
		if l.stopped() {
			return
		}
		synced, err := l.session()
		if l.stopped() {
			return
		}
		m.mu.Lock()
		if m.link == l {
			m.linkState = linkConnect
			m.syncInProgress = false
			if m.linkDownSince.IsZero() {
				m.linkDownSince = time.Now()
			}
		}
		m.mu.Unlock()
		if err != nil {
			logger.Warn().Err(err).Str("master", net.JoinHostPort(l.host, strconv.Itoa(l.port))).
				Msg("master link lost, reconnecting")
		}
		if synced {
			backoff = 50 * time.Millisecond
		}
		select {
		case <-l.stop:
			return
		case <-time.After(backoff):
		}
		if backoff *= 2; backoff > m.cfg.MaxBackoff {
			backoff = m.cfg.MaxBackoff
		}
	}
}

// session runs one connection to the master. It reports whether the link got
// as far as streaming.
func (l *link) session() (bool, error) {
	m := l.m
	m.mu.Lock()
	m.linkState = linkConnecting
	m.mu.Unlock()

	addr := net.JoinHostPort(l.host, strconv.Itoa(l.port))
	conn, err := net.DialTimeout("tcp", addr, m.cfg.Timeout)
	if err != nil {
		return false, err
	}
	if !l.setConn(conn) {
		_ = conn.Close()
		return false, nil
	}
	defer conn.Close()

	br := bufio.NewReaderSize(&deadlineReader{conn: conn, timeout: m.cfg.Timeout}, 64*1024)
	var wmu sync.Mutex
	send := func(parts ...string) error {
		wmu.Lock()
		defer wmu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(m.cfg.Timeout))
		_, err := conn.Write(encodeCommand(parts...))
		return err
	}
	call := func(parts ...string) (string, error) {
		if err := send(parts...); err != nil {
			return "", err
		}
		return readLine(br)
	}

	if m.cfg.MasterAuth != "" {
		args := []string{"AUTH"}
		if m.cfg.MasterUser != "" {
			args = append(args, m.cfg.MasterUser)
		}
		args = append(args, m.cfg.MasterAuth)
		reply, err := call(args...)
		if err != nil {
			return false, err
		}
		if !strings.HasPrefix(reply, "+") {
			return false, fmt.Errorf("master refused AUTH: %s", reply)
		}
	}
	reply, err := call("PING")
	if err != nil {
		return false, err
	}
	if !strings.HasPrefix(reply, "+") {
		return false, fmt.Errorf("master refused PING: %s", reply)
	}
	port := m.cfg.AnnouncePort
	if port == 0 {
		port = m.host.ListeningPort()
	}
	if reply, err = call("REPLCONF", "listening-port", strconv.Itoa(port)); err != nil {
		return false, err
	}
	if !strings.HasPrefix(reply, "+") {
		return false, fmt.Errorf("master refused REPLCONF listening-port: %s", reply)
	}
	if m.cfg.AnnounceIP != "" {
		if reply, err = call("REPLCONF", "ip-address", m.cfg.AnnounceIP); err != nil {
			return false, err
		}
		if !strings.HasPrefix(reply, "+") {
			return false, fmt.Errorf("master refused REPLCONF ip-address: %s", reply)
		}
	}
	if reply, err = call("REPLCONF", "capa", "eof", "capa", "psync2"); err != nil {
		return false, err
	}
	if !strings.HasPrefix(reply, "+") {
		return false, fmt.Errorf("master refused REPLCONF capa: %s", reply)
	}

	m.mu.Lock()
	replid, next := "?", int64(-1)
	if m.cached && m.replid != "" {
		replid, next = m.replid, m.offset+1
	}
	m.mu.Unlock()
	if reply, err = call("PSYNC", replid, strconv.FormatInt(next, 10)); err != nil {
		return false, err
	}
	switch {
	case strings.HasPrefix(reply, "+FULLRESYNC"):
		id, off, err := parseFullResync(reply)
		if err != nil {
			return false, err
		}
		if err := m.loadSnapshot(br, id, off); err != nil {
			return false, err
		}
	case strings.HasPrefix(reply, "+CONTINUE"):
		m.continueFrom(parseContinue(reply))
	default:
		return false, fmt.Errorf("master refused PSYNC: %s", reply)
	}

	m.mu.Lock()
	m.linkState = linkConnected
	m.linkDownSince = time.Time{}
	m.lastIO = time.Now()
	m.mu.Unlock()
	logger.Info().Str("master", addr).Msg("master link established")

	ackStop := make(chan struct{})
	ackDone := make(chan struct{})
	ack := func() error {
		_, off := m.ReplID()
		return send("REPLCONF", "ACK", strconv.FormatInt(off, 10))
	}
	go func() {
		defer close(ackDone)
		t := time.NewTicker(m.cfg.AckPeriod)
		defer t.Stop()
		for {
			select {
			case <-ackStop:
				return
			case <-t.C:
				if ack() != nil {
					return
				}
			}
		}
	}()
	defer func() {
		close(ackStop)
		<-ackDone
	}()
	if err := ack(); err != nil {
		return true, err
	}

	for {
		args, raw, err := readCommand(br)
		if err != nil {
			if l.stopped() {
				return true, nil
			}
			if errors.Is(err, io.EOF) {
				return true, errors.New("master closed the connection")
			}
			return true, err
		}
		getack, err := m.applyFromMaster(args, raw)
		if err != nil {
			return true, err
		}
		if getack {
			if err := ack(); err != nil {
				return true, err
			}
		}
	}
}

// applyFromMaster executes one command of the master's stream and appends it
// to this server's own stream. It reports whether the master asked for an
// acknowledgement.
func (m *Manager) applyFromMaster(args [][]byte, raw []byte) (bool, error) {
	name := command.CanonicalName(string(args[0]))
	getack := name == "REPLCONF" && len(args) > 1 && strings.EqualFold(string(args[1]), "GETACK")

	m.host.LockWrites()
	m.mu.Lock()
	db, tx := m.linkDB, m.linkTx
	m.lastIO = time.Now()
	m.mu.Unlock()

	if !getack && name != "PING" && name != "REPLCONF" {
		st := m.dbs.DB(db)
		if st == nil {
			db, st = 0, m.dbs.DB(0)
		}
		ctx := command.NewContext(name, args[1:], st, discardWriter())
		ctx.DB = db
		ctx.DBs = m.dbs
		ctx.Transaction = tx
		ctx.SetAuthenticated(true)
		ctx.InExec = true
		ctx.FromMaster = true
		ctx.Replaying = true
		m.dbs.BeginAuthoritativeApply()
		err := m.router.Execute(ctx)
		m.dbs.EndAuthoritativeApply()
		if err == nil && ctx.Writer.Errors() > 0 {
			err = fmt.Errorf("command returned an error reply")
		}
		if err != nil {
			tx.ClearWatch()
			m.mu.Lock()
			m.cached = false
			m.backlog, m.histLen = nil, 0
			m.linkTx = command.NewTransaction()
			dropped := m.detachAllLocked()
			m.mu.Unlock()
			m.host.UnlockWrites()
			for _, replica := range dropped {
				replica.close()
			}
			return false, fmt.Errorf("command from master %s failed: %w", name, err)
		}
		db = ctx.DB
	}

	m.mu.Lock()
	m.linkDB = db
	dropped := m.appendLocked(raw)
	m.mu.Unlock()
	m.host.UnlockWrites()
	for _, r := range dropped {
		r.close()
	}
	return getack, nil
}

// loadSnapshot replaces the dataset with the master's snapshot.
func (m *Manager) loadSnapshot(br *bufio.Reader, replid string, offset int64) error {
	var size int64
	for {
		line, err := readLine(br)
		if err != nil {
			return err
		}
		if line == "" {
			continue
		}
		if size, err = parseBulkHeader(line); err != nil {
			return err
		}
		break
	}

	m.mu.Lock()
	m.linkState = linkSync
	m.syncInProgress = true
	// Until the load completes the data matches no stream position.
	m.cached = false
	m.mu.Unlock()

	m.host.LockWrites()
	defer m.host.UnlockWrites()

	staging := m.dbs.NewStaging()
	staging.BeginAuthoritativeApply()
	replay := m.router.ForkForReplay(staging.DB(0), []string{
		"SET", "HSET", "RPUSH", "SADD", "ZADD", "GEOADD", "JSON.SET", "XGROUP", "XADD", "XSETID", "XCLAIM", "TS.CREATE", "TS.ADD", "ADDTAG", "PEXPIREAT", "SELECT", "FUNCTION",
	})
	payload := bufio.NewReaderSize(io.LimitReader(br, size), 64*1024)
	db := 0
	var applied int64
	for {
		args, raw, err := readCommand(payload)
		if err != nil {
			if errors.Is(err, io.EOF) && applied == size {
				break
			}
			if errors.Is(err, io.EOF) {
				return fmt.Errorf("snapshot truncated after %d of %d bytes", applied, size)
			}
			return err
		}
		applied += int64(len(raw))
		name := command.CanonicalName(string(args[0]))
		if name == "FUNCTION" && (len(args) < 2 || !strings.EqualFold(string(args[1]), "RESTORE")) {
			return fmt.Errorf("unsupported snapshot FUNCTION operation")
		}
		st := staging.DB(db)
		if st == nil {
			return fmt.Errorf("snapshot database %d is out of range", db)
		}
		ctx := command.NewContext(name, args[1:], st, nil)
		ctx.DB = db
		ctx.DBs = staging
		ctx.Replaying = true
		if err := replay.ExecuteSilent(ctx); err != nil {
			return fmt.Errorf("snapshot command %s failed: %w", ctx.Command, err)
		}
		if ctx.Writer.Errors() > 0 {
			return fmt.Errorf("snapshot command %s returned an error reply", ctx.Command)
		}
		if tracker := staging.DB(0).MemoryTracker(); tracker != nil && tracker.Max() > 0 && tracker.Usage() > tracker.Max() {
			return fmt.Errorf("snapshot exceeds maxmemory")
		}
		db = ctx.DB
	}
	if err := m.host.CommitFullSync(staging, replay); err != nil {
		return fmt.Errorf("persisting the full resynchronization failed: %w", err)
	}

	m.mu.Lock()
	m.replid = replid
	m.offset = offset
	m.replid2 = zeroReplID
	m.secondOffset = -1
	m.backlog = nil
	m.createBacklogLocked()
	m.linkDB = db
	m.linkTx = command.NewTransaction()
	m.syncInProgress = false
	m.cached = true
	// Replicas of this server hold the old history.
	replicas := m.detachAllLocked()
	m.mu.Unlock()
	for _, r := range replicas {
		r.close()
	}
	logger.Info().Str("replid", replid).Int64("offset", offset).Int64("bytes", size).
		Msg("full resynchronization loaded")
	return nil
}

// continueFrom adopts the id a +CONTINUE announced.
func (m *Manager) continueFrom(replid string) {
	m.mu.Lock()
	var replicas []*replica
	if replid != "" && replid != m.replid {
		m.replid2 = m.replid
		m.secondOffset = m.offset + 1
		m.replid = replid
		replicas = m.detachAllLocked()
	}
	m.createBacklogLocked()
	off := m.offset
	m.mu.Unlock()
	for _, r := range replicas {
		r.close()
	}
	logger.Info().Int64("offset", off).Msg("partial resynchronization accepted by master")
}
