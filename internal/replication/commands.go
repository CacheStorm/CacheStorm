package replication

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/resp"
)

// RegisterCommands registers the replication commands, bound to m, on router:
// REPLCONF, PSYNC, SYNC, ROLE, REPLICAOF, SLAVEOF and WAIT. It also installs
// the INFO replication section.
func (m *Manager) RegisterCommands(router *command.Router) {
	router.Register(&command.CommandDef{Name: "REPLCONF", Handler: m.cmdREPLCONF})
	router.Register(&command.CommandDef{Name: "PSYNC", Handler: m.cmdPSYNC})
	router.Register(&command.CommandDef{Name: "SYNC", Handler: m.cmdSYNC})
	router.Register(&command.CommandDef{Name: "ROLE", Handler: m.cmdROLE})
	router.Register(&command.CommandDef{Name: "REPLICAOF", Handler: m.cmdREPLICAOF})
	router.Register(&command.CommandDef{Name: "SLAVEOF", Handler: m.cmdREPLICAOF})
	router.Register(&command.CommandDef{Name: "WAIT", Handler: m.cmdWAIT})
	router.SetReplicationInfo(m.Info)
}

func (m *Manager) cmdREPLCONF(ctx *command.Context) error {
	if ctx.ArgCount()%2 != 0 {
		return ctx.WriteError(errors.New("ERR syntax error"))
	}
	for i := 0; i < ctx.ArgCount(); i += 2 {
		opt := strings.ToLower(ctx.ArgString(i))
		val := ctx.ArgString(i + 1)
		switch opt {
		case "listening-port":
			port, err := strconv.Atoi(val)
			if err != nil || port < 0 || port > 65535 {
				return ctx.WriteError(errors.New("ERR value is out of range"))
			}
			if ctx.ReplConf != nil {
				ctx.ReplConf.ListeningPort = port
			}
		case "ip-address":
			if ctx.ReplConf != nil {
				ctx.ReplConf.IP = val
			}
		case "capa":
			if ctx.ReplConf != nil {
				ctx.ReplConf.Capa = append(ctx.ReplConf.Capa, strings.ToLower(val))
			}
		case "ack", "getack", "fack":
			// Only meaningful on a replication link, which never reaches
			// the router; Redis does not reply to them either.
			return nil
		case "rdb-only", "rdb-filter-only", "rdb-channel":
		default:
			return ctx.WriteError(fmt.Errorf("ERR Unrecognized REPLCONF option: %s", ctx.ArgString(i)))
		}
	}
	return ctx.WriteOK()
}

func (m *Manager) cmdPSYNC(ctx *command.Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(errors.New("ERR wrong number of arguments for 'psync' command"))
	}
	off, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(errors.New("ERR value is not an integer or out of range"))
	}
	return m.handoff(ctx, syncRequest{replid: ctx.ArgString(0), offset: off})
}

func (m *Manager) cmdSYNC(ctx *command.Context) error {
	return m.handoff(ctx, syncRequest{legacy: true})
}

func (m *Manager) handoff(ctx *command.Context, req syncRequest) error {
	if ctx.InExec {
		return ctx.WriteError(errors.New("ERR Command is not allowed inside a transaction"))
	}
	m.mu.Lock()
	unsynced := m.role == RoleReplica && m.linkState != linkConnected
	m.mu.Unlock()
	if unsynced {
		return ctx.WriteError(errors.New("NOMASTERLINK Can't SYNC while not connected with my master"))
	}
	conf := ctx.ReplConf
	ctx.Handoff = func(conn net.Conn) command.ConnHandler {
		return m.attach(conn, conf, req)
	}
	return nil
}

func (m *Manager) cmdROLE(ctx *command.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.role == RoleMaster {
		replicas := make([]*resp.Value, 0, len(m.replicas))
		for r := range m.replicas {
			if !r.online() {
				continue
			}
			replicas = append(replicas, resp.ArrayValue([]*resp.Value{
				resp.BulkString(r.ip),
				resp.BulkString(strconv.Itoa(r.port)),
				resp.BulkString(strconv.FormatInt(r.ackOffset.Load(), 10)),
			}))
		}
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("master"),
			resp.IntegerValue(m.offset),
			resp.ArrayValue(replicas),
		})
	}
	off := m.offset
	if !m.cached {
		off = -1
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("slave"),
		resp.BulkString(m.masterHost),
		resp.IntegerValue(int64(m.masterPort)),
		resp.BulkString(m.linkState),
		resp.IntegerValue(off),
	})
}

func (m *Manager) cmdREPLICAOF(ctx *command.Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(fmt.Errorf("ERR wrong number of arguments for '%s' command", strings.ToLower(ctx.Command)))
	}
	if ctx.ClusterEnabled() {
		return ctx.WriteError(errors.New("ERR REPLICAOF not allowed in cluster mode.")) //nolint:staticcheck // Redis reply text
	}
	host, portArg := ctx.ArgString(0), ctx.ArgString(1)
	promote := strings.EqualFold(host, "no") && strings.EqualFold(portArg, "one")
	port := 0
	if !promote {
		var err error
		port, err = strconv.Atoi(portArg)
		if err != nil || port < 0 || port > 65535 {
			return ctx.WriteError(errors.New("ERR Invalid master port"))
		}
	}
	if ctx.InExec {
		// A role change takes the write gate exclusively, and EXEC is holding
		// it, so the change runs once EXEC has released it. Redis Sentinel
		// promotes a replica with MULTI, SLAVEOF NO ONE, CONFIG REWRITE,
		// CLIENT KILL, EXEC and then polls INFO for the new role.
		go func() {
			if promote {
				m.Promote()
				return
			}
			if err := m.ReplicaOf(host, port); err != nil && !errors.Is(err, ErrAlreadyConnected) {
				logger.Warn().Err(err).Str("host", host).Int("port", port).Msg("REPLICAOF inside a transaction failed")
			}
		}()
		return ctx.WriteOK()
	}
	if promote {
		m.Promote()
		return ctx.WriteOK()
	}
	switch err := m.ReplicaOf(host, port); {
	case errors.Is(err, ErrAlreadyConnected):
		return ctx.WriteSimpleString("OK Already connected to specified master")
	case err != nil:
		return ctx.WriteError(fmt.Errorf("ERR %v", err))
	}
	return ctx.WriteOK()
}

func (m *Manager) cmdWAIT(ctx *command.Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(errors.New("ERR wrong number of arguments for 'wait' command"))
	}
	num, err := strconv.ParseInt(ctx.ArgString(0), 10, 64)
	if err != nil {
		return ctx.WriteError(errors.New("ERR value is not an integer or out of range"))
	}
	ms, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(errors.New("ERR timeout is not an integer or out of range"))
	}
	if ms < 0 {
		return ctx.WriteError(errors.New("ERR timeout is negative"))
	}
	if m.Role() == RoleReplica {
		_ = ctx.Writer.WriteError("ERR WAIT cannot be used with replica instances. Please also note that since Redis 4.0 if a replica is configured to be writable (which is not the default) writes to replicas are just local and are not propagated.")
		return nil
	}
	if ctx.InExec {
		m.mu.Lock()
		target := m.offset
		m.mu.Unlock()
		return ctx.WriteInteger(int64(m.ackedReplicas(target)))
	}
	if num > 1<<31 {
		num = 1 << 31
	}
	return ctx.WriteInteger(int64(m.Wait(int(num), time.Duration(ms)*time.Millisecond, ctx.Done)))
}

// Info renders the INFO replication section.
func (m *Manager) Info() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var sb strings.Builder
	sb.WriteString("# Replication\r\n")
	fmt.Fprintf(&sb, "role:%s\r\n", m.role)
	if m.role == RoleReplica {
		fmt.Fprintf(&sb, "master_host:%s\r\n", m.masterHost)
		fmt.Fprintf(&sb, "master_port:%d\r\n", m.masterPort)
		status := "down"
		if m.linkState == linkConnected {
			status = "up"
		}
		fmt.Fprintf(&sb, "master_link_status:%s\r\n", status)
		lastIO := -1
		if !m.lastIO.IsZero() {
			lastIO = int(time.Since(m.lastIO).Seconds())
		}
		fmt.Fprintf(&sb, "master_last_io_seconds_ago:%d\r\n", lastIO)
		fmt.Fprintf(&sb, "master_sync_in_progress:%d\r\n", boolInt(m.syncInProgress))
		fmt.Fprintf(&sb, "slave_read_repl_offset:%d\r\n", m.offset)
		fmt.Fprintf(&sb, "slave_repl_offset:%d\r\n", m.offset)
		if status == "down" && !m.linkDownSince.IsZero() {
			fmt.Fprintf(&sb, "master_link_down_since_seconds:%d\r\n", int(time.Since(m.linkDownSince).Seconds()))
		}
		sb.WriteString("slave_priority:100\r\n")
		fmt.Fprintf(&sb, "slave_read_only:%d\r\n", boolInt(m.cfg.ReadOnly))
		sb.WriteString("replica_announced:1\r\n")
	}
	fmt.Fprintf(&sb, "connected_slaves:%d\r\n", len(m.replicas))
	i := 0
	for r := range m.replicas {
		lag := int(time.Since(time.Unix(0, r.lastAck.Load())).Seconds())
		fmt.Fprintf(&sb, "slave%d:ip=%s,port=%d,state=%s,offset=%d,lag=%d\r\n",
			i, r.ip, r.port, r.getState(), r.ackOffset.Load(), lag)
		i++
	}
	sb.WriteString("master_failover_state:no-failover\r\n")
	fmt.Fprintf(&sb, "master_replid:%s\r\n", m.replid)
	fmt.Fprintf(&sb, "master_replid2:%s\r\n", m.replid2)
	fmt.Fprintf(&sb, "master_repl_offset:%d\r\n", m.offset)
	fmt.Fprintf(&sb, "second_repl_offset:%d\r\n", m.secondOffset)
	fmt.Fprintf(&sb, "repl_backlog_active:%d\r\n", boolInt(m.backlog != nil))
	fmt.Fprintf(&sb, "repl_backlog_size:%d\r\n", m.cfg.BacklogSize)
	first := int64(0)
	if m.backlog != nil {
		first = m.offset - m.histLen + 1
	}
	fmt.Fprintf(&sb, "repl_backlog_first_byte_offset:%d\r\n", first)
	fmt.Fprintf(&sb, "repl_backlog_histlen:%d\r\n", m.histLen)
	fmt.Fprintf(&sb, "sync_full:%d\r\n", m.syncFull)
	fmt.Fprintf(&sb, "sync_partial_ok:%d\r\n", m.syncPartialOK)
	fmt.Fprintf(&sb, "sync_partial_err:%d\r\n", m.syncPartialErr)
	sb.WriteString("\r\n")
	return sb.String()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
