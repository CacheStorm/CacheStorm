package replication

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/logger"
)

// Failover states, as INFO replication's master_failover_state reports them.
const (
	failoverNone       = "no-failover"
	failoverWaitSync   = "waiting-for-sync"
	failoverInProgress = "failover-in-progress"
)

// failoverHandshake bounds how long the old master tries to resume from the
// target before it gives up and stays master.
const failoverHandshake = 10 * time.Second

// WritePauser is implemented by a Host that can pause client writes, which a
// coordinated FAILOVER needs while the target replica catches up.
type WritePauser interface {
	PauseWrites(until time.Time)
	UnpauseWrites()
}

type failoverRun struct {
	state    string
	abort    chan struct{}
	abortOne sync.Once
}

// redisError is a Redis error reply, whose text may end with a period.
func redisError(text string) error { return errors.New(text) }

func (f *failoverRun) stop() { f.abortOne.Do(func() { close(f.abort) }) }

// failoverStateLocked is master_failover_state.
func (m *Manager) failoverStateLocked() string {
	if m.failover == nil {
		return failoverNone
	}
	return m.failover.state
}

// cmdFAILOVER implements FAILOVER [TO host port [FORCE]] [ABORT]
// [TIMEOUT ms], Redis' coordinated failover from a master to one of its
// replicas: writes pause until the target has the whole stream, then this
// server resumes from the target as its replica with PSYNC ... FAILOVER,
// which makes the target a master.
func (m *Manager) cmdFAILOVER(ctx *command.Context) error {
	if ctx.ClusterEnabled() {
		return ctx.WriteError(redisError("ERR FAILOVER not allowed in cluster mode.")) //nolint:staticcheck // Redis reply text
	}
	var host string
	var port int
	var timeout time.Duration
	force, abort, hasTo := false, false, false
	for i := 0; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch {
		case arg == "TO" && i+2 < ctx.ArgCount() && !hasTo:
			p, err := strconv.ParseInt(ctx.ArgString(i+2), 10, 64)
			if err != nil {
				return ctx.WriteError(errors.New("ERR value is not an integer or out of range"))
			}
			host, port, hasTo = ctx.ArgString(i+1), int(p), true
			i += 2
		case arg == "TIMEOUT" && i+1 < ctx.ArgCount() && timeout == 0:
			ms, err := strconv.ParseInt(ctx.ArgString(i+1), 10, 64)
			if err != nil {
				return ctx.WriteError(errors.New("ERR value is not an integer or out of range"))
			}
			if ms <= 0 {
				return ctx.WriteError(errors.New("ERR FAILOVER timeout must be greater than 0"))
			}
			timeout = time.Duration(ms) * time.Millisecond
			i++
		case arg == "FORCE" && !force:
			force = true
		case arg == "ABORT" && !abort:
			abort = true
		default:
			return ctx.WriteError(errors.New("ERR syntax error"))
		}
	}
	if abort {
		m.mu.Lock()
		f := m.failover
		m.mu.Unlock()
		if f == nil {
			return ctx.WriteError(redisError("ERR No failover in progress."))
		}
		f.stop()
		return ctx.WriteOK()
	}
	pauser, ok := m.host.(WritePauser)
	if !ok {
		return ctx.WriteError(errors.New("ERR FAILOVER is not supported by this server"))
	}

	m.mu.Lock()
	if m.role == RoleReplica {
		m.mu.Unlock()
		return ctx.WriteError(redisError("ERR FAILOVER is not valid when server is a replica."))
	}
	if len(m.replicas) == 0 {
		m.mu.Unlock()
		return ctx.WriteError(redisError("ERR FAILOVER requires connected replicas."))
	}
	if force && (timeout == 0 || !hasTo) {
		m.mu.Unlock()
		return ctx.WriteError(redisError("ERR FAILOVER with force option requires both a timeout and target HOST and IP."))
	}
	if m.failover != nil {
		m.mu.Unlock()
		return ctx.WriteError(redisError("ERR FAILOVER already in progress."))
	}
	var target *replica
	if hasTo {
		for r := range m.replicas {
			if r.ip == host && r.port == port {
				target = r
				break
			}
		}
		if target == nil {
			m.mu.Unlock()
			return ctx.WriteError(redisError("ERR FAILOVER target HOST and PORT is not a replica."))
		}
		if !target.online() {
			m.mu.Unlock()
			return ctx.WriteError(redisError("ERR FAILOVER target replica is not online."))
		}
	}
	f := &failoverRun{state: failoverWaitSync, abort: make(chan struct{})}
	m.failover = f
	m.mu.Unlock()

	until := time.Now().Add(24 * time.Hour)
	if timeout > 0 {
		until = time.Now().Add(timeout)
	}
	logger.Info().Str("target", host).Int("port", port).Msg("FAILOVER: waiting for a replica to catch up")
	go m.runFailover(f, pauser, until, target, timeout, force)
	return ctx.WriteOK()
}

func (m *Manager) runFailover(f *failoverRun, pauser WritePauser, until time.Time, target *replica, timeout time.Duration, force bool) {
	// The pause waits for the writes already running, so it cannot run on
	// the FAILOVER client's own goroutine.
	pauser.PauseWrites(until)
	finish := func(reason string) {
		m.mu.Lock()
		if m.failover == f {
			m.failover = nil
		}
		m.mu.Unlock()
		pauser.UnpauseWrites()
		if reason != "" {
			logger.Warn().Str("reason", reason).Msg("FAILOVER aborted")
		}
	}
	var deadline <-chan time.Time
	if timeout > 0 {
		t := time.NewTimer(timeout)
		defer t.Stop()
		deadline = t.C
	}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		m.mu.Lock()
		offset := m.offset
		if target == nil {
			for r := range m.replicas {
				if r.online() && r.ackOffset.Load() >= offset {
					target = r
					break
				}
			}
		}
		_, attached := m.replicas[target]
		caughtUp := target != nil && target.ackOffset.Load() >= offset
		m.mu.Unlock()
		if target != nil && !attached {
			finish("the target replica disconnected")
			return
		}
		if caughtUp {
			break
		}
		m.feedRaw(encodeCommand("REPLCONF", "GETACK", "*"))
		select {
		case <-f.abort:
			finish("failover manually aborted")
			return
		case <-deadline:
			if !force || target == nil {
				finish("replica never caught up before timeout")
				return
			}
		case <-tick.C:
			continue
		}
		break
	}

	m.mu.Lock()
	f.state = failoverInProgress
	m.mu.Unlock()
	host, port := target.ip, target.port
	logger.Info().Str("target", host).Int("port", port).Msg("FAILOVER: handing the master role to the replica")
	if err := m.replicaOf(host, port, true); err != nil {
		finish(err.Error())
		return
	}
	end := time.NewTimer(failoverHandshake)
	defer end.Stop()
	for {
		m.mu.Lock()
		connected := m.role == RoleReplica && m.linkState == linkConnected && m.masterHost == host && m.masterPort == port
		m.mu.Unlock()
		if connected {
			finish("")
			logger.Info().Str("master", host).Int("port", port).Msg("FAILOVER complete: now a replica of the new master")
			return
		}
		select {
		case <-f.abort:
			m.Promote()
			finish("failover manually aborted")
			return
		case <-end.C:
			m.Promote()
			finish("the target did not accept the failover in time")
			return
		case <-tick.C:
		}
	}
}
