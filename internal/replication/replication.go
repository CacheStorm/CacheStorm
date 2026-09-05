package replication

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/store"
)

type Role int

const (
	RoleMaster Role = iota
	RoleReplica
)

type ReplicaState int

const (
	StateConnecting ReplicaState = iota
	StateSyncing
	StateConnected
	StateDisconnected
)

type Replica struct {
	ID           string
	Conn         net.Conn
	IP           string
	Port         int
	State        ReplicaState
	Offset       int64
	LastAckTime  time.Time
	ConnectedAt  time.Time
	Writer       *bufio.Writer
	Capabilities map[string]bool
}

type Manager struct {
	mu             sync.RWMutex
	cfg            *config.ReplicationConfig
	store          *store.Store
	role           atomic.Int32
	masterConn     net.Conn
	masterOffset   atomic.Int64
	masterID       string
	replicas       map[string]*Replica
	replicaID      string
	replBacklog    []byte
	replBacklogIdx atomic.Int64
	stopCh         chan struct{}
	wg             sync.WaitGroup
	onRoleChange   atomic.Value // func(Role); accessed atomically — SetRole fires from under m.mu (ReplicaOf), so locking here would self-deadlock
	stopped        atomic.Bool
}

var globalManager *Manager
var managerOnce sync.Once

func GetManager() *Manager {
	return globalManager
}

func InitManager(cfg *config.ReplicationConfig, s *store.Store) *Manager {
	managerOnce.Do(func() {
		globalManager = &Manager{
			cfg:         cfg,
			store:       s,
			replicas:    make(map[string]*Replica),
			replBacklog: make([]byte, 1024*1024),
			replicaID:   generateReplicaID(),
			stopCh:      make(chan struct{}),
		}
		if cfg.Role == "replica" || cfg.Role == "slave" {
			globalManager.role.Store(int32(RoleReplica))
		} else {
			globalManager.role.Store(int32(RoleMaster))
		}
	})
	return globalManager
}

func generateReplicaID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

func (m *Manager) Start() error {
	if m.GetRole() == RoleReplica {
		return m.connectToMaster()
	}
	return nil
}

func (m *Manager) Stop() {
	if m.stopped.Swap(true) {
		return
	}
	close(m.stopCh)
	m.mu.Lock()
	conn := m.masterConn
	m.masterConn = nil
	for _, r := range m.replicas {
		r.Conn.Close()
	}
	m.mu.Unlock()
	if conn != nil {
		conn.Close()
	}
	m.wg.Wait()
}

func (m *Manager) GetRole() Role {
	return Role(m.role.Load())
}

func (m *Manager) GetReplicaID() string {
	return m.replicaID
}

func (m *Manager) GetMasterOffset() int64 {
	return m.masterOffset.Load()
}

func (m *Manager) GetReplicas() []*Replica {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Replica, 0, len(m.replicas))
	for _, r := range m.replicas {
		result = append(result, r)
	}
	return result
}

func (m *Manager) GetReplicaCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.replicas)
}

func (m *Manager) GetInfo() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sb strings.Builder
	role := m.GetRole()

	if role == RoleMaster {
		sb.WriteString("# Replication\r\n")
		sb.WriteString("role:master\r\n")
		fmt.Fprintf(&sb, "connected_replicas:%d\r\n", len(m.replicas))

		i := 0
		for _, r := range m.replicas {
			if r.State == StateConnected {
				fmt.Fprintf(&sb, "slave%d:ip=%s,port=%d,state=online,offset=%d,lag=%d\r\n",
					i, r.IP, r.Port, r.Offset, int(time.Since(r.LastAckTime).Seconds()))
				i++
			}
		}
		fmt.Fprintf(&sb, "master_replid:%s\r\n", m.replicaID)
		fmt.Fprintf(&sb, "master_repl_offset:%d\r\n", m.replBacklogIdx.Load())
		sb.WriteString("repl_backlog_active:1\r\n")
		sb.WriteString("repl_backlog_size:1048576\r\n")
		fmt.Fprintf(&sb, "repl_backlog_first_byte_offset:%d\r\n", m.replBacklogIdx.Load())
	} else {
		sb.WriteString("# Replication\r\n")
		sb.WriteString("role:slave\r\n")
		fmt.Fprintf(&sb, "master_host:%s\r\n", m.cfg.MasterHost)
		fmt.Fprintf(&sb, "master_port:%d\r\n", m.cfg.MasterPort)
		fmt.Fprintf(&sb, "master_link_status:%s\r\n", m.getMasterLinkStatus())
		fmt.Fprintf(&sb, "master_last_io_seconds_ago:%d\r\n", m.getSecondsSinceMasterIO())
		fmt.Fprintf(&sb, "master_sync_in_progress:%d\r\n", m.getSyncInProgress())
		fmt.Fprintf(&sb, "slave_repl_offset:%d\r\n", m.masterOffset.Load())
		sb.WriteString("slave_priority:100\r\n")
		fmt.Fprintf(&sb, "slave_read_only:%d\r\n", boolToInt(m.cfg.ReadOnly))
	}

	return sb.String()
}

func (m *Manager) connectToMaster() error {
	// Snapshot the target under the lock: a concurrent ReplicaOf may
	// reconfigure or clear the master at any time (previously these reads
	// raced ReplicaOf's mu-held writes).
	m.mu.RLock()
	host, port := m.cfg.MasterHost, m.cfg.MasterPort
	m.mu.RUnlock()

	if host == "" || port == 0 {
		return fmt.Errorf("master host/port not configured")
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		logger.Error().Err(err).Msg("failed to connect to master")
		return err
	}

	m.mu.Lock()
	// The dial is slow; the master may have been reconfigured or cleared
	// while it was in flight. Publishing the connection then would leak a
	// stale link to a master this node has already left.
	if m.cfg.MasterHost != host || m.cfg.MasterPort != port || m.masterConn != nil {
		conn.Close()
		m.mu.Unlock()
		return fmt.Errorf("master reconfigured during connect")
	}
	m.masterConn = conn
	m.mu.Unlock()

	logger.Info().Str("addr", addr).Msg("connected to master")

	m.wg.Add(1)
	go m.syncWithMaster()

	return nil
}

func (m *Manager) syncWithMaster() {
	defer m.wg.Done()

	// Snapshot the connection once: ReplicaOf may clear m.masterConn
	// concurrently (previously these reads raced that mu-held write).
	m.mu.RLock()
	conn := m.masterConn
	m.mu.RUnlock()
	if conn == nil {
		return
	}
	defer conn.Close()
	defer logger.RecoverPanic("replication-sync")

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	if err := m.sendHandshake(writer); err != nil {
		logger.Error().Err(err).Msg("handshake failed")
		return
	}

	for {
		select {
		case <-m.stopCh:
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				logger.Error().Err(err).Msg("read from master failed")
			}
			return
		}

		line = strings.TrimSpace(line)
		m.handleMasterResponse(line, reader)
	}
}

func (m *Manager) sendHandshake(writer *bufio.Writer) error {
	if _, err := writer.WriteString("*1\r\n$4\r\nPING\r\n"); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(writer, "*3\r\n$5\r\nREPLCONF\r\n$8\r\nlistening-port\r\n$4\r\n%d\r\n",
		m.cfg.ReplicaAnnouncePort); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	if _, err := writer.WriteString("*3\r\n$5\r\nREPLCONF\r\n$4\r\ncapa\r\n$6\r\npsync2\r\n"); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	psyncCmd := fmt.Sprintf("*3\r\n$5\r\nPSYNC\r\n$40\r\n%s\r\n$1\r\n%d\r\n",
		strings.Repeat("?", 40), -1)
	if _, err := writer.WriteString(psyncCmd); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	return nil
}

func (m *Manager) handleMasterResponse(line string, reader *bufio.Reader) {
	if strings.HasPrefix(line, "+FULLRESYNC") {
		parts := strings.Fields(line[11:])
		if len(parts) >= 2 {
			m.mu.Lock()
			m.masterID = parts[0]
			if offset, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				m.masterOffset.Store(offset)
			}
			m.mu.Unlock()
			logger.Info().Str("master_id", parts[0]).Msg("full sync started")
		}
		m.receiveRDB(reader)
	} else if strings.HasPrefix(line, "+CONTINUE") {
		logger.Info().Msg("partial sync continued")
	} else if strings.HasPrefix(line, "$") {
		length, err := strconv.Atoi(line[1:])
		if err != nil {
			logger.Error().Err(err).Str("line", line).Msg("invalid bulk length in replication stream")
		} else if length > 0 {
			buf := make([]byte, length)
			if _, err := io.ReadFull(reader, buf); err == nil {
				m.processWriteCommand(buf)
			}
		}
	}

	m.masterOffset.Add(1)
}

func (m *Manager) receiveRDB(reader *bufio.Reader) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	line = strings.TrimSpace(line)

	if strings.HasPrefix(line, "$") {
		length, err := strconv.ParseInt(line[1:], 10, 64)
		if err != nil {
			logger.Error().Err(err).Str("line", line).Msg("invalid RDB length")
			return
		}
		if length > 0 {
			buf := make([]byte, length)
			if _, err := io.ReadFull(reader, buf); err != nil {
				logger.Error().Err(err).Msg("failed to read RDB data")
				return
			}
			logger.Info().Int64("size", length).Msg("RDB received, syncing complete")
		}
	}
}

func (m *Manager) processWriteCommand(data []byte) {
}

func (m *Manager) AddReplica(conn net.Conn, ip string, port int, capabilities map[string]bool) *Replica {
	m.mu.Lock()
	defer m.mu.Unlock()

	replica := &Replica{
		ID:           generateReplicaID(),
		Conn:         conn,
		IP:           ip,
		Port:         port,
		State:        StateConnected,
		ConnectedAt:  time.Now(),
		LastAckTime:  time.Now(),
		Writer:       bufio.NewWriter(conn),
		Capabilities: capabilities,
	}

	m.replicas[replica.ID] = replica

	logger.Info().
		Str("id", replica.ID).
		Str("ip", ip).
		Int("port", port).
		Msg("replica connected")

	return replica
}

func (m *Manager) RemoveReplica(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if replica, ok := m.replicas[id]; ok {
		replica.Conn.Close()
		delete(m.replicas, id)
		logger.Info().Str("id", id).Msg("replica disconnected")
	}
}

func (m *Manager) UpdateReplicaOffset(id string, offset int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if replica, ok := m.replicas[id]; ok {
		replica.Offset = offset
		replica.LastAckTime = time.Now()
	}
}

func (m *Manager) PropagateCommand(cmd []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	newIdx := m.replBacklogIdx.Add(1)
	idx := int(newIdx) % len(m.replBacklog)
	copy(m.replBacklog[idx:], cmd)

	for id, replica := range m.replicas {
		if replica.State == StateConnected && replica.Writer != nil {
			if _, err := replica.Writer.Write(cmd); err != nil {
				logger.Error().Err(err).Str("replica", id).Msg("failed to write to replica")
				continue
			}
			if err := replica.Writer.Flush(); err != nil {
				logger.Error().Err(err).Str("replica", id).Msg("failed to flush replica writer")
				continue
			}
			replica.Offset++
		}
	}
}

func (m *Manager) getMasterLinkStatus() string {
	if m.masterConn == nil {
		return "down"
	}
	return "up"
}

func (m *Manager) getSecondsSinceMasterIO() int {
	return 0
}

func (m *Manager) getSyncInProgress() int {
	return 0
}

func (m *Manager) SetRole(role Role) {
	m.role.Store(int32(role))
	if fn, ok := m.onRoleChange.Load().(func(Role)); ok && fn != nil {
		fn(role)
	}
}

func (m *Manager) OnRoleChange(fn func(Role)) {
	m.onRoleChange.Store(fn)
}

func (m *Manager) ReplicaOf(host string, port int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if host == "no" && port == 1 {
		m.SetRole(RoleMaster)
		if m.masterConn != nil {
			m.masterConn.Close()
			m.masterConn = nil
		}
		m.cfg.MasterHost = ""
		m.cfg.MasterPort = 0
		logger.Info().Msg("promoted to master")
		return nil
	}

	m.cfg.MasterHost = host
	m.cfg.MasterPort = port
	m.SetRole(RoleReplica)

	go func() {
		if err := m.connectToMaster(); err != nil {
			logger.Error().Err(err).Msg("failed to connect to new master")
		}
	}()

	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

type SyncWriter struct {
	writer io.Writer
}

func NewSyncWriter(w io.Writer) *SyncWriter {
	return &SyncWriter{writer: w}
}

func (w *SyncWriter) WriteRDBHeader() error {
	if _, err := w.writer.Write([]byte("REDIS0011")); err != nil {
		return err
	}
	if _, err := w.writer.Write([]byte{0xFE}); err != nil {
		return err
	}
	return persistence.WriteRDBLength(w.writer, 0)
}

func (w *SyncWriter) WriteDatabaseSelect(db int) error {
	if _, err := w.writer.Write([]byte{0xFE}); err != nil {
		return err
	}
	return persistence.WriteRDBLength(w.writer, db)
}

func (w *SyncWriter) WriteKeyValuePair(key string, value interface{}, ttl time.Duration, expireAt int64) error {
	if ttl > 0 {
		if err := w.writeByte(0xFC); err != nil {
			return err
		}
		var exp [8]byte
		binary.LittleEndian.PutUint64(exp[:], uint64(expireAt))
		if _, err := w.writer.Write(exp[:]); err != nil {
			return err
		}
	}
	// Value type: string. The key length uses the canonical RDB length
	// encoding so entries parse symmetrically with persistence.RDBReader
	// (a raw byte(len) misparses for lengths >= 64).
	if err := w.writeByte(0x00); err != nil {
		return err
	}
	if err := persistence.WriteRDBLength(w.writer, len(key)); err != nil {
		return err
	}
	if _, err := w.writer.Write([]byte(key)); err != nil {
		return err
	}

	switch v := value.(type) {
	case string:
		return w.writeStringValue(v)
	case []byte:
		return w.writeStringValue(string(v))
	}
	return nil
}

// writeStringValue writes a string value using the canonical RDB length
// encoding (replaces the former RESP-style "$" + 4-byte raw length, which
// the RDB reader could not parse for any length).
func (w *SyncWriter) writeStringValue(s string) error {
	if err := persistence.WriteRDBLength(w.writer, len(s)); err != nil {
		return err
	}
	_, err := w.writer.Write([]byte(s))
	return err
}

func (w *SyncWriter) writeByte(b byte) error {
	_, err := w.writer.Write([]byte{b})
	return err
}

func (w *SyncWriter) WriteEnd() error {
	_, err := w.writer.Write([]byte{0xFF})
	return err
}
