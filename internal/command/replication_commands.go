package command

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

type ReplicaInfo struct {
	ClientID     int64
	RemoteAddr   string
	IP           string
	Port         int
	Offset       int64
	Capabilities map[string]bool
	ConnectedAt  time.Time
	LastAckTime  time.Time
	State        string
}

var (
	replicaMu   sync.RWMutex
	replicas    = make(map[int64]*ReplicaInfo)
	replManager *ReplicationManager
)

type ReplicationManager struct {
	store      *store.Store
	role       string
	masterHost string
	masterPort int
	masterID   string
	masterOff  int64
	replicaID  string
}

func InitReplicationManager(s *store.Store) {
	replManager = &ReplicationManager{
		store:     s,
		role:      "master",
		replicaID: fmt.Sprintf("%040x", time.Now().UnixNano()),
	}
}

func GetReplicationManager() *ReplicationManager {
	return replManager
}

func (m *ReplicationManager) GetRole() string {
	return m.role
}

func (m *ReplicationManager) GetReplicaID() string {
	return m.replicaID
}

func (m *ReplicationManager) GetMasterOffset() int64 {
	return m.masterOff
}

func (m *ReplicationManager) GetMasterHost() string {
	return m.masterHost
}

func (m *ReplicationManager) GetMasterPort() int {
	return m.masterPort
}

func (m *ReplicationManager) GetReplicas() []*ReplicaInfo {
	replicaMu.RLock()
	defer replicaMu.RUnlock()
	result := make([]*ReplicaInfo, 0, len(replicas))
	for _, r := range replicas {
		result = append(result, r)
	}
	return result
}

func (m *ReplicationManager) GetReplicaCount() int {
	replicaMu.RLock()
	defer replicaMu.RUnlock()
	return len(replicas)
}

func (m *ReplicationManager) AddReplica(clientID int64, ip string, port int, capa map[string]bool) {
	replicaMu.Lock()
	defer replicaMu.Unlock()
	replicas[clientID] = &ReplicaInfo{
		ClientID:     clientID,
		IP:           ip,
		Port:         port,
		Capabilities: capa,
		ConnectedAt:  time.Now(),
		LastAckTime:  time.Now(),
		State:        "online",
	}
}

func (m *ReplicationManager) UpdateReplicaAck(clientID int64, offset int64) {
	replicaMu.Lock()
	defer replicaMu.Unlock()
	if r, ok := replicas[clientID]; ok {
		r.Offset = offset
		r.LastAckTime = time.Now()
	}
}

func (m *ReplicationManager) RemoveReplica(clientID int64) {
	replicaMu.Lock()
	defer replicaMu.Unlock()
	delete(replicas, clientID)
}

func (m *ReplicationManager) ReplicaOf(host string, port int) {
	m.masterHost = host
	m.masterPort = port
	if host == "" || (host == "no" && port == 1) {
		m.role = "master"
	} else {
		m.role = "slave"
	}
}

func (m *ReplicationManager) GetInfo() string {
	var sb strings.Builder
	sb.WriteString("# Replication\r\n")

	if m.role == "master" {
		sb.WriteString("role:master\r\n")
		sb.WriteString(fmt.Sprintf("connected_replicas:%d\r\n", m.GetReplicaCount()))
		sb.WriteString(fmt.Sprintf("master_replid:%s\r\n", m.replicaID))
		sb.WriteString(fmt.Sprintf("master_repl_offset:%d\r\n", m.masterOff))
		sb.WriteString("repl_backlog_active:1\r\n")
		sb.WriteString("repl_backlog_size:1048576\r\n")

		i := 0
		for _, r := range m.GetReplicas() {
			sb.WriteString(fmt.Sprintf("slave%d:ip=%s,port=%d,state=%s,offset=%d,lag=%d\r\n",
				i, r.IP, r.Port, r.State, r.Offset, int(time.Since(r.LastAckTime).Seconds())))
			i++
		}
	} else {
		sb.WriteString("role:slave\r\n")
		sb.WriteString(fmt.Sprintf("master_host:%s\r\n", m.masterHost))
		sb.WriteString(fmt.Sprintf("master_port:%d\r\n", m.masterPort))
		sb.WriteString("master_link_status:up\r\n")
		sb.WriteString("master_last_io_seconds_ago:0\r\n")
		sb.WriteString("master_sync_in_progress:0\r\n")
		sb.WriteString(fmt.Sprintf("slave_repl_offset:%d\r\n", m.masterOff))
		sb.WriteString("slave_priority:100\r\n")
		sb.WriteString("slave_read_only:1\r\n")
	}

	return sb.String()
}

func generateRDB(s *store.Store) []byte {
	var buf bytes.Buffer

	buf.WriteString("REDIS0011")
	buf.WriteByte(0xFE)
	buf.WriteByte(0x00)

	entries := s.GetAll()
	for key, entry := range entries {
		if entry == nil {
			continue
		}

		var expireAt int64
		if entry.TTL() > 0 {
			expireAt = time.Now().Add(entry.TTL()).UnixMilli()
		}

		keyBytes := []byte(key)
		valueBytes := []byte(entry.Value.String())

		if expireAt > 0 {
			buf.WriteByte(0xFC)
			writeUint64LE(&buf, uint64(expireAt))
		}

		buf.WriteByte(0x00)
		writeRDBString(&buf, keyBytes)
		writeRDBString(&buf, valueBytes)
	}

	buf.WriteByte(0xFF)
	buf.Write(make([]byte, 8))

	return buf.Bytes()
}

func writeRDBString(buf *bytes.Buffer, data []byte) {
	length := len(data)
	if length < 64 {
		buf.WriteByte(byte(length))
	} else if length < 16384 {
		buf.WriteByte(byte((length >> 8) | 0x40))
		buf.WriteByte(byte(length & 0xFF))
	} else {
		buf.WriteByte(byte((length >> 24) | 0x80))
		buf.WriteByte(byte((length >> 16) & 0xFF))
		buf.WriteByte(byte((length >> 8) & 0xFF))
		buf.WriteByte(byte(length & 0xFF))
	}
	buf.Write(data)
}

func writeUint64LE(buf *bytes.Buffer, v uint64) {
	for i := 0; i < 8; i++ {
		buf.WriteByte(byte(v >> (i * 8)))
	}
}

func cmdREPLCONF(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if replManager == nil {
		return ctx.WriteError(fmt.Errorf("ERR replication not initialized"))
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "LISTENING-PORT":
		if ctx.ArgCount() < 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		port, err := strconv.Atoi(ctx.ArgString(1))
		if err != nil {
			return ctx.WriteError(ErrInvalidArg)
		}
		ip := "unknown"
		if parts := strings.Split(ctx.RemoteAddr, ":"); len(parts) > 0 {
			ip = parts[0]
		}
		replManager.AddReplica(ctx.ClientID, ip, port, map[string]bool{})
		return ctx.WriteOK()

	case "IP-ADDRESS":
		if ctx.ArgCount() < 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		replicaMu.Lock()
		if r, ok := replicas[ctx.ClientID]; ok {
			r.IP = ctx.ArgString(1)
		}
		replicaMu.Unlock()
		return ctx.WriteOK()

	case "CAPA":
		capa := make(map[string]bool)
		for i := 1; i < ctx.ArgCount(); i++ {
			capa[strings.ToLower(ctx.ArgString(i))] = true
		}
		replicaMu.Lock()
		if r, ok := replicas[ctx.ClientID]; ok {
			for k, v := range capa {
				r.Capabilities[k] = v
			}
		} else {
			ip := "unknown"
			if parts := strings.Split(ctx.RemoteAddr, ":"); len(parts) > 0 {
				ip = parts[0]
			}
			replicas[ctx.ClientID] = &ReplicaInfo{
				ClientID:     ctx.ClientID,
				IP:           ip,
				Capabilities: capa,
				ConnectedAt:  time.Now(),
				LastAckTime:  time.Now(),
				State:        "online",
			}
		}
		replicaMu.Unlock()
		return ctx.WriteOK()

	case "ACK":
		if ctx.ArgCount() >= 2 {
			offset, _ := strconv.ParseInt(ctx.ArgString(1), 10, 64)
			replManager.UpdateReplicaAck(ctx.ClientID, offset)
		}
		return nil

	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown subcommand '%s'", subCmd))
	}
}

func cmdSYNC(ctx *Context) error {
	if replManager == nil {
		return ctx.WriteError(fmt.Errorf("ERR replication not initialized"))
	}

	if replManager.GetRole() != "master" {
		return ctx.WriteError(fmt.Errorf("ERR can't sync from a replica"))
	}

	rdbData := generateRDB(ctx.Store)

	ctx.Writer.WriteBulkBytes(rdbData)

	return nil
}

func cmdPSYNC(ctx *Context) error {
	if replManager == nil {
		return ctx.WriteError(fmt.Errorf("ERR replication not initialized"))
	}

	if replManager.GetRole() != "master" {
		return ctx.WriteError(fmt.Errorf("ERR can't sync from a replica"))
	}

	var masterReplID string
	var offset int64 = -1

	if ctx.ArgCount() >= 1 {
		masterReplID = ctx.ArgString(0)
	}
	if ctx.ArgCount() >= 2 {
		offset, _ = strconv.ParseInt(ctx.ArgString(1), 10, 64)
	}

	currentReplID := replManager.GetReplicaID()
	currentOffset := replManager.GetMasterOffset()

	if masterReplID == currentReplID && offset >= 0 {
		ctx.Writer.WriteSimpleString(fmt.Sprintf("CONTINUE %s", currentReplID))
	} else {
		ctx.Writer.WriteSimpleString(fmt.Sprintf("FULLRESYNC %s %d", currentReplID, currentOffset))
		rdbData := generateRDB(ctx.Store)
		ctx.Writer.WriteBulkBytes(rdbData)
	}

	return nil
}

func cmdROLE(ctx *Context) error {
	if replManager == nil {
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("master"),
			resp.ArrayValue([]*resp.Value{}),
		})
	}

	if replManager.GetRole() == "master" {
		rs := replManager.GetReplicas()
		replicaVals := make([]*resp.Value, len(rs))
		for i, r := range rs {
			replicaVals[i] = resp.ArrayValue([]*resp.Value{
				resp.BulkString(r.IP),
				resp.BulkString(strconv.Itoa(r.Port)),
				resp.BulkString(strconv.FormatInt(r.Offset, 10)),
			})
		}
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("master"),
			resp.ArrayValue(replicaVals),
		})
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("slave"),
		resp.BulkString(replManager.GetMasterHost()),
		resp.BulkString(strconv.Itoa(replManager.GetMasterPort())),
		resp.BulkString(strconv.FormatInt(replManager.GetMasterOffset(), 10)),
	})
}

func cmdREPLICAOF(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if replManager == nil {
		return ctx.WriteError(fmt.Errorf("ERR replication not initialized"))
	}

	host := strings.ToLower(ctx.ArgString(0))
	arg2 := strings.ToLower(ctx.ArgString(1))

	if host == "no" && arg2 == "one" {
		replManager.ReplicaOf("", 0)
		return ctx.WriteOK()
	}

	port, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrInvalidArg)
	}

	replManager.ReplicaOf(host, port)
	return ctx.WriteOK()
}

func RegisterReplicationCommands(router *Router) {
	router.Register(&CommandDef{Name: "REPLCONF", Handler: cmdREPLCONF})
	router.Register(&CommandDef{Name: "SYNC", Handler: cmdSYNC})
	router.Register(&CommandDef{Name: "PSYNC", Handler: cmdPSYNC})
	router.Register(&CommandDef{Name: "ROLE", Handler: cmdROLE})
	router.Register(&CommandDef{Name: "REPLICAOF", Handler: cmdREPLICAOF})
	router.Register(&CommandDef{Name: "SLAVEOF", Handler: cmdREPLICAOF})
}
