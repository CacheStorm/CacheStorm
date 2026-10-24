package server

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/replication"
	"github.com/cachestorm/cachestorm/internal/store"
)

// errClusterUnsupported is returned by New when the configuration enables
// cluster mode, which this release does not implement.
var errClusterUnsupported = errors.New("cluster mode is not supported in this release: set cluster.enabled to false (master-replica replication is configured in the replication section)")

// readOnlyReplicaError is the reply of a read-only replica to a write.
const readOnlyReplicaError = "READONLY You can't write against a read only replica."

// replicationStateFile holds a replica's stream position next to its AOF, so
// a restarted replica can continue from its master's backlog.
const replicationStateFile = "replication.state"

// gateStripes is the number of key stripes of the write gate.
const gateStripes = 256

// writeGate orders writes with their propagation. A write holds the stripes
// of the keys it names (shared with every other write) from before it runs
// until it has been handed to the AOF and the replication stream, so two
// writes to the same key reach the stream in the order they were applied.
// Writes whose keys are unknown, and the full-resync snapshot, take the gate
// exclusively.
type writeGate struct {
	global  sync.RWMutex
	stripes [gateStripes]sync.Mutex
}

// gateHold is one acquisition of the gate.
type gateHold struct {
	g         *writeGate
	exclusive bool
	stripes   []int
	// inline backs stripes for the common case of a command with few keys,
	// so acquiring the gate does not allocate.
	inline [4]int
}

// exclusiveWrites are writes that reach keys they do not name, or several
// databases, so they take the whole gate.
var exclusiveWrites = map[string]bool{
	"EXEC": true, "EVAL": true, "EVALSHA": true, "FCALL": true,
	"FLUSHALL": true, "FLUSHDB": true, "SWAPDB": true, "MOVE": true,
	"SCRIPT": true, "FUNCTION": true, "MIGRATE": true,
	// Read-only scripts take the gate exclusively too: a script is atomic in
	// Redis, so it must not see a write land between two of its reads.
	"EVAL_RO": true, "EVALSHA_RO": true, "FCALL_RO": true,
	"INVALIDATE": true, "ADDTAG": true, "REMTAG": true, "TAGLINK": true,
	"TAGUNLINK": true, "SETTAG": true,
}

// stripeOf hashes key with FNV-1a without allocating a hasher.
// ungatedCommands run without the gate. They do not read the keyspace, and
// they either wait (WAIT for replica acks) or take the gate themselves
// (PSYNC/SYNC snapshot the data with it held exclusively; REPLICAOF loads a
// snapshot the same way), so holding it shared around them would deadlock or
// stall every transaction behind them.
var ungatedCommands = map[string]bool{
	"PSYNC": true, "SYNC": true, "REPLCONF": true, "REPLICAOF": true,
	"SLAVEOF": true, "WAIT": true, "WAITAOF": true, "ROLE": true,
	"QUIT": true, "RESET": true, "AUTH": true, "HELLO": true,
}

func stripeOf(key string) int {
	h := uint32(2166136261)
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return int(h % gateStripes)
}

// acquire takes the gate for cmd.
func (g *writeGate) acquire(cmd string, args [][]byte) *gateHold {
	h := &gateHold{g: g}
	if exclusiveWrites[cmd] {
		h.exclusive = true
	} else {
		keys := command.CommandKeys(cmd, args)
		if len(keys) == 0 {
			h.exclusive = true
		} else {
			h.stripes = h.inline[:0]
			for _, k := range keys {
				h.stripes = appendStripe(h.stripes, stripeOf(k))
			}
		}
	}
	h.lock()
	return h
}

// appendStripe inserts stripe into the sorted, duplicate-free list. Locking
// the stripes in ascending order keeps two multi-key writes from deadlocking.
func appendStripe(stripes []int, stripe int) []int {
	i := sort.SearchInts(stripes, stripe)
	if i < len(stripes) && stripes[i] == stripe {
		return stripes
	}
	stripes = append(stripes, 0)
	copy(stripes[i+1:], stripes[i:])
	stripes[i] = stripe
	return stripes
}

// acquireKey takes the gate for a write of key alone.
func (g *writeGate) acquireKey(key string) *gateHold {
	h := &gateHold{g: g}
	h.stripes = append(h.inline[:0], stripeOf(key))
	h.lock()
	return h
}

func (h *gateHold) lock() {
	if h.exclusive {
		h.g.global.Lock()
		return
	}
	h.g.global.RLock()
	for _, s := range h.stripes {
		h.g.stripes[s].Lock()
	}
}

func (h *gateHold) unlock() {
	if h.exclusive {
		h.g.global.Unlock()
		return
	}
	for i := len(h.stripes) - 1; i >= 0; i-- {
		h.g.stripes[h.stripes[i]].Unlock()
	}
	h.g.global.RUnlock()
}

// gated reports whether cmd takes the write gate: every command recorded in
// the AOF and the replication stream, and EXEC, which runs them.
func gated(cmd string, args [][]byte) bool {
	return cmd == "EXEC" || persistedCommand(cmd, args)
}

// replicaRefuses reports whether a read-only replica refuses cmd from a
// client. Scripts run, but in read-only mode (see Context.ReplicaReadOnly).
func replicaRefuses(cmd string, args [][]byte) bool {
	switch cmd {
	case "EVAL", "EVALSHA", "FCALL", "SCRIPT":
		return false
	}
	return persistedCommand(cmd, args) || command.IsWriteCommand(cmd)
}

// LockWrites implements replication.Host.
func (s *Server) LockWrites() { s.gate.global.Lock() }

// UnlockWrites implements replication.Host.
func (s *Server) UnlockWrites() { s.gate.global.Unlock() }

// AfterFullSync implements replication.Host: the AOF is rewritten from the
// dataset the full resync loaded, so it describes that dataset alone.
func (s *Server) AfterFullSync() error {
	if s.aof == nil {
		return nil
	}
	return s.aof.Rewrite(replication.DBSource{DBs: s.dbs})
}

// ListeningPort implements replication.Host.
func (s *Server) ListeningPort() int {
	if s.listener != nil {
		if addr, ok := s.listener.Addr().(*net.TCPAddr); ok {
			return addr.Port
		}
	}
	return s.cfg.Server.Port
}

// Replication returns the server's replication manager.
func (s *Server) Replication() *replication.Manager {
	return s.repl
}

// newReplication builds the replication manager and connects it to the
// router, the store hooks and the persistence hooks.
func (s *Server) newReplication(cfg *config.Config) {
	rc := cfg.Replication
	s.repl = replication.New(replication.Config{
		MasterAuth:         rc.MasterAuth,
		ReadOnly:           rc.ReadOnly,
		Timeout:            time.Duration(rc.ReplTimeout) * time.Second,
		BacklogSize:        rc.BacklogSize,
		ReplicaBufferLimit: rc.ReplicaBufferLimit,
		AnnounceIP:         rc.ReplicaAnnounceIP,
		AnnouncePort:       rc.ReplicaAnnouncePort,
	}, s, s.router, s.dbs)
	s.repl.RegisterCommands(s.router)
	s.router.SetPersistFilter(persistedCommand)
}

// recordWrites hands writes that ran outside the router (the REST API) to the
// AOF and the replication stream, as one transaction.
func (s *Server) recordWrites(cmds []command.ExecutedCommand) {
	if len(cmds) == 0 {
		return
	}
	if s.aof != nil {
		writes := make([]persistence.Command, len(cmds))
		for i, c := range cmds {
			writes[i] = persistence.Command{Name: c.Name, Args: c.Args, DB: c.DB}
		}
		if err := s.aof.AppendTransaction(writes); err != nil {
			logger.Error().Err(err).Msg("AOF append failed")
		}
	}
	s.repl.FeedTransaction(cmds)
}

// loadReplicationState adopts the stream position a replica saved at its last
// shutdown, when it restarts as a replica with the AOF that matches it. The
// file is removed either way: once the server runs, the AOF moves on.
func (s *Server) loadReplicationState(dataDir string) {
	path := filepath.Join(dataDir, replicationStateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if err := os.Remove(path); err != nil {
		logger.Warn().Err(err).Msg("cannot remove the replication state file")
	}
	if !s.cfg.Replication.IsReplica() {
		return
	}
	var st replication.State
	if err := json.Unmarshal(data, &st); err != nil || st.ReplID == "" || st.Offset < 0 {
		logger.Warn().Msg("ignoring an unreadable replication state file")
		return
	}
	s.repl.RestoreState(st)
	logger.Info().Str("replid", st.ReplID).Int64("offset", st.Offset).Msg("replication state restored")
}

// saveReplicationState records a replica's stream position next to its AOF
// at shutdown.
func (s *Server) saveReplicationState() {
	if s.aof == nil {
		return
	}
	st, ok := s.repl.SavedState()
	if !ok {
		return
	}
	data, err := json.Marshal(st)
	if err != nil {
		return
	}
	dataDir := s.cfg.Persistence.DataDir
	if dataDir == "" {
		dataDir = "."
	}
	tmp := filepath.Join(dataDir, replicationStateFile+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		logger.Warn().Err(err).Msg("cannot save the replication state")
		return
	}
	if err := os.Rename(tmp, filepath.Join(dataDir, replicationStateFile)); err != nil {
		logger.Warn().Err(err).Msg("cannot save the replication state")
	}
}

// storeHooks returns the store hooks: the plugin dispatchers, and the
// replication of expirations and evictions as DEL.
func (s *Server) storeHooks() store.StoreHooks {
	return store.StoreHooks{
		OnEvict:         s.pluginMgr.RunEvictHooks,
		OnExpire:        s.pluginMgr.RunExpireHooks,
		OnTagInvalidate: s.pluginMgr.RunTagInvalidateHooks,
		OnExpireLocked: func(st *store.Store, key string) {
			if s.repl != nil {
				s.repl.FeedDelete(st, key)
			}
		},
		OnEvictKey: func(st *store.Store, key string) {
			if s.repl != nil {
				s.repl.FeedDelete(st, key)
			}
		},
		ExpiryGuard: func(_ *store.Store, key string) func() {
			return s.gate.acquireKey(key).unlock
		},
	}
}

// refuseOnReplica answers a client's write on a read-only replica with the
// READONLY error, and reports whether it did. Inside MULTI the transaction is
// aborted, as Redis does for a command that fails to queue.
func (s *Server) refuseOnReplica(ctx *command.Context) bool {
	if s == nil || s.repl == nil || !s.repl.ReadOnly() {
		return false
	}
	ctx.ReplicaReadOnly = true
	if !replicaRefuses(ctx.Command, ctx.Args) {
		return false
	}
	if s.router.RequirePass() != "" && !ctx.IsAuthenticated() {
		// The router answers NOAUTH first.
		return false
	}
	if ctx.Transaction != nil && ctx.Transaction.IsActive() {
		ctx.Transaction.Abort()
	}
	_ = ctx.Writer.WriteError(readOnlyReplicaError)
	return true
}

// holdGate takes the write gate for the command ctx is about to run and
// returns its release. A write holds the stripes of its keys, EXEC and the
// scripts hold the whole gate, and every other command holds it shared, so
// no command observes a transaction or a script half applied. A blocking
// command releases it while it waits.
func (s *Server) holdGate(ctx *command.Context) func() {
	if s == nil {
		return func() {}
	}
	if ctx.Command != "EXEC" && ctx.Transaction != nil && ctx.Transaction.IsActive() {
		// Inside MULTI the command is only queued; EXEC runs it.
		return func() {}
	}
	if ungatedCommands[ctx.Command] {
		return func() {}
	}
	var h *gateHold
	if gated(ctx.Command, ctx.Args) || exclusiveWrites[ctx.Command] {
		h = s.gate.acquire(ctx.Command, ctx.Args)
	} else {
		h = &gateHold{g: &s.gate}
		h.lock()
	}
	ctx.BeforeWait = h.unlock
	ctx.AfterWait = h.lock
	return h.unlock
}
