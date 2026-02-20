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
	masterOffset   int64
	masterID       string
	replicas       map[string]*Replica
	replicaID      string
	replBacklog    []byte
	replBacklogIdx int64
	stopCh         chan struct{}
	wg             sync.WaitGroup
	onRoleChange   func(Role)
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
	close(m.stopCh)
	if m.masterConn != nil {
		m.masterConn.Close()
	}
	m.mu.Lock()
	for _, r := range m.replicas {
		r.Conn.Close()
	}
	m.mu.Unlock()
	m.wg.Wait()
}

func (m *Manager) GetRole() Role {
	return Role(m.role.Load())
}

func (m *Manager) GetReplicaID() string {
	return m.replicaID
}

func (m *Manager) GetMasterOffset() int64 {
	return m.masterOffset
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
		sb.WriteString(fmt.Sprintf("connected_replicas:%d\r\n", len(m.replicas)))

		i := 0
		for _, r := range m.replicas {
			if r.State == StateConnected {
				sb.WriteString(fmt.Sprintf("slave%d:ip=%s,port=%d,state=online,offset=%d,lag=%d\r\n",
					i, r.IP, r.Port, r.Offset, int(time.Since(r.LastAckTime).Seconds())))
				i++
			}
		}
		sb.WriteString(fmt.Sprintf("master_replid:%s\r\n", m.replicaID))
		sb.WriteString(fmt.Sprintf("master_repl_offset:%d\r\n", m.replBacklogIdx))
		sb.WriteString("repl_backlog_active:1\r\n")
		sb.WriteString("repl_backlog_size:1048576\r\n")
		sb.WriteString(fmt.Sprintf("repl_backlog_first_byte_offset:%d\r\n", m.replBacklogIdx))
	} else {
		sb.WriteString("# Replication\r\n")
		sb.WriteString("role:slave\r\n")
		sb.WriteString(fmt.Sprintf("master_host:%s\r\n", m.cfg.MasterHost))
		sb.WriteString(fmt.Sprintf("master_port:%d\r\n", m.cfg.MasterPort))
		sb.WriteString(fmt.Sprintf("master_link_status:%s\r\n", m.getMasterLinkStatus()))
		sb.WriteString(fmt.Sprintf("master_last_io_seconds_ago:%d\r\n", m.getSecondsSinceMasterIO()))
		sb.WriteString(fmt.Sprintf("master_sync_in_progress:%d\r\n", m.getSyncInProgress()))
		sb.WriteString(fmt.Sprintf("slave_repl_offset:%d\r\n", m.masterOffset))
		sb.WriteString(fmt.Sprintf("slave_priority:100\r\n"))
		sb.WriteString(fmt.Sprintf("slave_read_only:%d\r\n", boolToInt(m.cfg.ReadOnly)))
	}

	return sb.String()
}

func (m *Manager) connectToMaster() error {
	if m.cfg.MasterHost == "" || m.cfg.MasterPort == 0 {
		return fmt.Errorf("master host/port not configured")
	}

	addr := net.JoinHostPort(m.cfg.MasterHost, strconv.Itoa(m.cfg.MasterPort))
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		logger.Error().Err(err).Msg("failed to connect to master")
		return err
	}

	m.masterConn = conn
	logger.Info().Str("addr", addr).Msg("connected to master")

	m.wg.Add(1)
	go m.syncWithMaster()

	return nil
}

func (m *Manager) syncWithMaster() {
	defer m.wg.Done()
	defer m.masterConn.Close()

	reader := bufio.NewReader(m.masterConn)
	writer := bufio.NewWriter(m.masterConn)

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
	writer.Flush()

	if _, err := fmt.Fprintf(writer, "*3\r\n$5\r\nREPLCONF\r\n$8\r\nlistening-port\r\n$4\r\n%d\r\n",
		m.cfg.ReplicaAnnouncePort); err != nil {
		return err
	}
	writer.Flush()

	if _, err := writer.WriteString("*3\r\n$5\r\nREPLCONF\r\n$4\r\ncapa\r\n$6\r\npsync2\r\n"); err != nil {
		return err
	}
	writer.Flush()

	psyncCmd := fmt.Sprintf("*3\r\n$5\r\nPSYNC\r\n$40\r\n%s\r\n$1\r\n%d\r\n",
		strings.Repeat("?", 40), -1)
	if _, err := writer.WriteString(psyncCmd); err != nil {
		return err
	}
	writer.Flush()

	return nil
}

func (m *Manager) handleMasterResponse(line string, reader *bufio.Reader) {
	if strings.HasPrefix(line, "+FULLRESYNC") {
		parts := strings.Fields(line[11:])
		if len(parts) >= 2 {
			m.mu.Lock()
			m.masterID = parts[0]
			if offset, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				m.masterOffset = offset
			}
			m.mu.Unlock()
			logger.Info().Str("master_id", m.masterID).Msg("full sync started")
		}
		m.receiveRDB(reader)
	} else if strings.HasPrefix(line, "+CONTINUE") {
		logger.Info().Msg("partial sync continued")
	} else if strings.HasPrefix(line, "$") {
		length, _ := strconv.Atoi(line[1:])
		if length > 0 {
			buf := make([]byte, length)
			if _, err := io.ReadFull(reader, buf); err == nil {
				m.processWriteCommand(buf)
			}
		}
	}

	m.mu.Lock()
	m.masterOffset++
	m.mu.Unlock()
}

func (m *Manager) receiveRDB(reader *bufio.Reader) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	line = strings.TrimSpace(line)

	if strings.HasPrefix(line, "$") {
		length, _ := strconv.ParseInt(line[1:], 10, 64)
		if length > 0 {
			buf := make([]byte, length)
			io.ReadFull(reader, buf)
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

	m.replBacklogIdx++
	idx := int(m.replBacklogIdx) % len(m.replBacklog)
	copy(m.replBacklog[idx:], cmd)

	for _, replica := range m.replicas {
		if replica.State == StateConnected && replica.Writer != nil {
			replica.Writer.Write(cmd)
			replica.Writer.Flush()
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
	if m.onRoleChange != nil {
		m.onRoleChange(role)
	}
}

func (m *Manager) OnRoleChange(fn func(Role)) {
	m.onRoleChange = fn
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

func (w *SyncWriter) WriteRDBHeader() {
	w.writer.Write([]byte("REDIS0011\xfe\x00"))
}

func (w *SyncWriter) WriteDatabaseSelect(db int) {
	buf := make([]byte, 2)
	buf[0] = 0xFE
	buf[1] = byte(db)
	w.writer.Write(buf)
}

func (w *SyncWriter) WriteKeyValuePair(key string, value interface{}, ttl time.Duration, expireAt int64) {
	keyBytes := []byte(key)
	var buf []byte

	if ttl > 0 {
		buf = make([]byte, 1+8+2+len(keyBytes))
		buf[0] = 0xFC
		binary.LittleEndian.PutUint64(buf[1:9], uint64(expireAt))
		buf[9] = 0x00
		buf[10] = byte(len(keyBytes))
		copy(buf[11:], keyBytes)
	} else {
		buf = make([]byte, 2+len(keyBytes))
		buf[0] = 0x00
		buf[1] = byte(len(keyBytes))
		copy(buf[2:], keyBytes)
	}
	w.writer.Write(buf)

	switch v := value.(type) {
	case string:
		w.writeStringValue(v)
	case []byte:
		w.writeStringValue(string(v))
	}
}

func (w *SyncWriter) writeStringValue(s string) {
	data := []byte(s)
	buf := make([]byte, 1+4+len(data))
	buf[0] = '$'
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(data)))
	copy(buf[5:], data)
	w.writer.Write(buf)
}

func (w *SyncWriter) WriteEnd() {
	w.writer.Write([]byte{0xFF})
}
