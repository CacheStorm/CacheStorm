package server

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/replication"
	"github.com/cachestorm/cachestorm/internal/store"
)

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
	stripes [gateStripes]sync.RWMutex
}

// gateHold is one acquisition of the gate.
type gateHold struct {
	g         *writeGate
	exclusive bool
	read      bool
	stripes   []int
	// inline backs stripes for the common case of a command with few keys,
	// so acquiring the gate does not allocate.
	inline [4]int
}

// exclusiveWrites are writes that reach keys they do not name, or several
// databases, so they take the whole gate.
var exclusiveWrites = map[string]bool{
	"SAVE": true, "BGSAVE": true, "SHUTDOWN": true,
	"EXEC": true, "EVAL": true, "EVALSHA": true, "FCALL": true,
	"FLUSHALL": true, "FLUSHDB": true, "SWAPDB": true, "MOVE": true,
	"RENAME": true, "RENAMENX": true, "COPY": true,
	"MSET": true, "MSETNX": true, "SMOVE": true,
	"SCRIPT": true, "FUNCTION": true, "MIGRATE": true,
	// Read-only scripts take the gate exclusively too: a script is atomic in
	// Redis, so it must not see a write land between two of its reads.
	"EVAL_RO": true, "EVALSHA_RO": true, "FCALL_RO": true,
	"INVALIDATE": true, "ADDTAG": true, "REMTAG": true, "TAGLINK": true,
	"TAGUNLINK": true, "SETTAG": true,
	// The search indexes are shared by every database and an index is
	// reachable through its aliases, so the index name does not order writes.
	"FT.CREATE": true, "FT.DROPINDEX": true, "FT.ADD": true, "FT.DEL": true,
	"FT.ALIASADD": true, "FT.ALIASDEL": true, "FT.ALIASUPDATE": true,
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

// planWrite works out the hold a write of cmd needs, without locking.
func (g *writeGate) planWrite(h *gateHold, ctx *command.Context) *gateHold {
	cmd, args := ctx.Command, ctx.Args
	*h = gateHold{g: g}
	if exclusiveWrites[cmd] || globalRead(cmd, args) {
		h.exclusive = true
	} else {
		keys := ctx.CommandKeys()
		if len(keys) == 0 {
			h.exclusive = true
		} else {
			h.stripes = h.inline[:0]
			for _, k := range keys {
				h.stripes = appendStripe(h.stripes, stripeOf(k))
			}
		}
	}
	return h
}

func globalRead(cmd string, args [][]byte) bool {
	switch cmd {
	case "KEYS", "SCAN", "DBSIZE", "RANDOMKEY":
		return true
	case "SORT", "SORT_RO":
		for i := 1; i+1 < len(args); i++ {
			if strings.EqualFold(string(args[i]), "BY") && !strings.EqualFold(string(args[i+1]), "nosort") {
				return true
			}
			if strings.EqualFold(string(args[i]), "GET") && string(args[i+1]) != "#" {
				return true
			}
		}
	}
	return false
}

// planRead works out the hold a read of cmd needs, without locking.
func (g *writeGate) planRead(h *gateHold, ctx *command.Context) *gateHold {
	*h = gateHold{g: g, read: true, exclusive: globalRead(ctx.Command, ctx.Args)}
	if !h.exclusive {
		h.stripes = h.inline[:0]
		for _, key := range ctx.CommandKeys() {
			h.stripes = appendStripe(h.stripes, stripeOf(key))
		}
	}
	return h
}

// covers reports whether holding h already gives what want needs: the same
// mode and the same stripes. Exclusive holds are never shared, so a pipeline
// cannot keep the whole server waiting.
func (h *gateHold) covers(want *gateHold) bool {
	if h == nil || want == nil || h.exclusive || want.exclusive || h.read != want.read || len(h.stripes) != len(want.stripes) {
		return false
	}
	for i := range h.stripes {
		if h.stripes[i] != want.stripes[i] {
			return false
		}
	}
	return true
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
		if h.read {
			h.g.stripes[s].RLock()
		} else {
			lockStripe(&h.g.stripes[s])
		}
	}
}

// stripeSpins bounds how often lockStripe retries before it parks.
const stripeSpins = 64

// lockStripe write-locks mu, retrying briefly before blocking. Pipelined
// writes to one hot key hand a stripe between connections every few
// microseconds; once waiters have queued for a millisecond sync.Mutex hands
// it over in FIFO order to parked goroutines, and the wakeup then costs more
// than the commands the hold covers.
func lockStripe(mu *sync.RWMutex) {
	for i := 0; i < stripeSpins; i++ {
		if mu.TryLock() {
			return
		}
		runtime.Gosched()
	}
	mu.Lock()
}

func (h *gateHold) unlock() {
	if h.exclusive {
		h.g.global.Unlock()
		return
	}
	for i := len(h.stripes) - 1; i >= 0; i-- {
		if h.read {
			h.g.stripes[h.stripes[i]].RUnlock()
		} else {
			h.g.stripes[h.stripes[i]].Unlock()
		}
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
	case "PFCOUNT":
		return false
	case "EVAL", "EVALSHA", "FCALL", "SCRIPT":
		return false
	}
	return persistedCommand(cmd, args) || command.IsWriteCommand(cmd)
}

// PauseWrites implements replication.WritePauser for FAILOVER: client
// writes pause until the given time, and the writes already running finish
// first.
func (s *Server) PauseWrites(until time.Time) { s.pauseWritesForFailover(until) }

// UnpauseWrites implements replication.WritePauser.
func (s *Server) UnpauseWrites() { s.unpauseClients() }

// LockWrites implements replication.Host.
func (s *Server) LockWrites() { s.gate.global.Lock() }

// UnlockWrites implements replication.Host.
func (s *Server) UnlockWrites() { s.gate.global.Unlock() }

func (s *Server) CommitFullSync(dbs *store.Databases, replay *command.Router) error {
	if dbs.Count() != s.dbs.Count() {
		return errors.New("replication database count mismatch")
	}
	functions := replay.FunctionRegistry().CloneForStore(s.Store())
	searchIndexes := replay.SearchIndexes()
	if s.aof != nil {
		if err := s.aof.Rewrite(replication.DBSource{DBs: dbs, Functions: functions.DumpPayload(), Search: searchIndexes.Commands()}); err != nil {
			return err
		}
	}
	if err := s.dbs.ReplaceFrom(dbs); err != nil {
		return err
	}
	s.router.SetFunctionRegistry(functions)
	s.router.SetSearchIndexes(searchIndexes)
	return nil
}

func (s *Server) FunctionPayload() ([]byte, error) {
	return s.snapshotFunctionCommand([][]byte{[]byte("DUMP")})
}

// SearchCommands implements replication.Host.
func (s *Server) SearchCommands() [][]string {
	return s.router.SearchIndexes().Commands()
}

// persistenceSource is dbs with the state an AOF rewrite or a snapshot
// carries besides the keys: the function libraries and the search indexes.
func (s *Server) persistenceSource(dbs *store.Databases) (replication.DBSource, error) {
	functions, err := s.FunctionPayload()
	if err != nil {
		return replication.DBSource{}, err
	}
	return replication.DBSource{DBs: dbs, Functions: functions, Search: s.SearchCommands()}, nil
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
func (s *Server) newReplication(cfg *config.Config) error {
	rc := cfg.Replication
	var replTLS *tls.Config
	// In cluster mode a node's client port is the one replicas sync from, so
	// when it speaks TLS the replicas dial it with TLS too.
	if rc.TLS || (cfg.Cluster.Enabled && cfg.Server.TLSCertFile != "") {
		var err error
		if replTLS, err = peerTLSConfig(&cfg.Server); err != nil {
			return fmt.Errorf("replication TLS: %w", err)
		}
	}
	s.repl = replication.New(replication.Config{
		TLS:                replTLS,
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
	return nil
}

// recordWrites hands writes that ran outside the router (the REST API) to the
// AOF and the replication stream, as one transaction.
func (s *Server) recordWrites(cmds []command.ExecutedCommand) error {
	if len(cmds) == 0 {
		return nil
	}
	if hasLogicalWrites(cmds) {
		s.repl.MarkLocalWrite()
	}
	if s.aof != nil {
		writes := make([]persistence.Command, len(cmds))
		for i, c := range cmds {
			writes[i] = persistence.Command{Name: c.Name, Args: c.Args, DB: c.DB}
		}
		if err := s.aof.AppendTransaction(writes); err != nil {
			logger.Error().Err(err).Msg("AOF append failed")
			return errAOFUnavailable
		}
	}
	s.repl.FeedTransaction(cmds)
	return nil
}

// storeHooks returns the store hooks: the plugin dispatchers, and the
// replication of expirations and evictions as DEL.
func (s *Server) storeHooks() store.StoreHooks {
	return store.StoreHooks{
		OnEvict:         s.pluginMgr.RunEvictHooks,
		OnExpire:        s.pluginMgr.RunExpireHooks,
		OnTagInvalidate: s.pluginMgr.RunTagInvalidateHooks,
		OnExpireLocked: func(st *store.Store, key string) {
			s.recordRemoval(st, key)
			s.router.TouchSearchKey(st, key)
		},
		OnEvictLocked: func(st *store.Store, key string) {
			s.recordRemoval(st, key)
			s.router.TouchSearchKey(st, key)
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
	if s.router.ClusterRedirects(ctx) {
		// In cluster mode the redirection comes first, as in Redis.
		return false
	}
	if s.router.RequirePass() != "" && !ctx.IsAuthenticated() {
		// The router answers NOAUTH first.
		return false
	}
	if ctx.Transaction != nil && ctx.Transaction.IsActive() {
		ctx.Transaction.Abort()
	}
	command.RecordRejected(ctx.Command, ctx.Args)
	_ = ctx.Writer.WriteError(readOnlyReplicaError)
	return true
}

func (s *Server) holdGate(ctx *command.Context) func() {
	h := s.planGate(ctx, &gateHold{})
	if h == nil {
		return func() {}
	}
	h.lock()
	ctx.BeforeWait = h.unlock
	ctx.AfterWait = h.lock
	return h.unlock
}

// planGate fills h with the hold ctx's command needs and returns it,
// unlocked, or returns nil when the command runs without the gate.
func (s *Server) planGate(ctx *command.Context, h *gateHold) *gateHold {
	if s == nil {
		return nil
	}
	if ctx.Command != "EXEC" && ctx.Transaction != nil && ctx.Transaction.IsActive() {
		// Inside MULTI the command is only queued; EXEC runs it.
		return nil
	}
	if ungatedCommands[ctx.Command] {
		return nil
	}
	if gated(ctx.Command, ctx.Args) || exclusiveWrites[ctx.Command] {
		return s.gate.planWrite(h, ctx)
	}
	return s.gate.planRead(h, ctx)
}

func hasLogicalWrites(cmds []command.ExecutedCommand) bool {
	for _, cmd := range cmds {
		if cmd.Name != "PFCOUNT" && cmd.Name != "SELECT" {
			return true
		}
	}
	return false
}
