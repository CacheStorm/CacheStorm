package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/plugin"
	"github.com/cachestorm/cachestorm/internal/replication"
	"github.com/cachestorm/cachestorm/internal/sentinel"
	"github.com/cachestorm/cachestorm/internal/store"
	"github.com/cachestorm/cachestorm/plugins/metrics"
)

// writeCommands lists commands that mutate state and should be persisted to AOF.
//
// This gate decides durability, so a command that mutates data but is missing
// here is silently NOT appended: the AOF still holds the older command that
// created the value, and replaying it on restart RESURRECTS data the client
// already changed or deleted. Every registered command below was checked
// against the router; the list covers the core data-structure commands plus the
// scripting and JSON/time-series writers, because those mutate through their
// own handlers rather than the string/hash/list/set/zset paths.
//
// Over-inclusion is safe (replaying a read is a no-op) while under-inclusion
// loses data, so ambiguous writers (SORT without STORE) are deliberately listed.
var writeCommands = map[string]bool{
	"SET": true, "SETNX": true, "SETEX": true, "PSETEX": true, "MSET": true, "MSETNX": true,
	"APPEND": true, "INCR": true, "DECR": true, "INCRBY": true, "DECRBY": true, "INCRBYFLOAT": true,
	"GETDEL": true, "GETSET": true, "GETEX": true, "SETRANGE": true,
	"DEL": true, "UNLINK": true, "RENAME": true, "RENAMENX": true, "EXPIRE": true, "EXPIREAT": true,
	"PEXPIRE": true, "PEXPIREAT": true, "PERSIST": true,
	"COPY": true, "MOVE": true, "MIGRATE": true, "RESTORE": true, "SORT": true,
	"FLUSHALL": true, "FLUSHDB": true, "SWAPDB": true,
	"HSET": true, "HSETNX": true, "HMSET": true, "HDEL": true, "HINCRBY": true, "HINCRBYFLOAT": true,
	"HGETDEL": true, "HGETEX": true,
	"LPUSH": true, "RPUSH": true, "LPOP": true, "RPOP": true, "LSET": true, "LREM": true,
	"LTRIM": true, "LINSERT": true, "RPOPLPUSH": true, "LMOVE": true,
	"LPUSHX": true, "RPUSHX": true, "LMPUSH": true, "LMPOP": true,
	"BLPOP": true, "BRPOP": true, "BRPOPLPUSH": true, "BLMOVE": true, "BLMPOP": true,
	"SADD": true, "SREM": true, "SPOP": true, "SMOVE": true,
	"SDIFFSTORE": true, "SINTERSTORE": true, "SUNIONSTORE": true,
	"ZADD": true, "ZREM": true, "ZINCRBY": true, "ZRANGESTORE": true,
	"ZPOPMIN": true, "ZPOPMAX": true, "ZMPOP": true,
	"ZDIFFSTORE": true, "ZINTERSTORE": true, "ZUNIONSTORE": true,
	"ZREMRANGEBYSCORE": true, "ZREMRANGEBYRANK": true, "ZREMRANGEBYLEX": true,
	"BZPOPMIN": true, "BZPOPMAX": true,
	"XADD": true, "XDEL": true, "XTRIM": true,
	"XACK": true, "XCLAIM": true, "XAUTOCLAIM": true, "XGROUP": true, "XSETID": true,
	"PFADD": true, "PFMERGE": true,
	"SETBIT": true, "BITOP": true, "BITFIELD": true,
	"GEOADD": true, "GEORADIUS": true, "GEOSEARCHSTORE": true,
	"ADDTAG": true, "REMTAG": true, "TAGLINK": true, "TAGUNLINK": true,
	"SETTAG": true, "INVALIDATE": true,
	// Scripting mutates through redis.call inside the handler, so the script is
	// the only replayable record of the effect.
	"EVAL": true, "EVALSHA": true, "FCALL": true,
	"JSON.SET": true, "JSON.DEL": true, "JSON.MSET": true,
	"JSON.NUMINCRBY": true, "JSON.NUMMULTBY": true,
	"JSON.ARRAPPEND": true, "JSON.STRAPPEND": true,
	"TS.ADD": true, "TS.CREATE": true, "TS.DEL": true, "TS.ALTER": true,
	"TS.INCRBY": true, "TS.DECRBY": true, "TS.MADD": true,
}

// persistedCommand reports whether cmd is appended to the AOF. Besides the
// writeCommands, that is the SCRIPT and FUNCTION subcommands that change the
// script cache or the function libraries: EVALSHA and FCALL are persisted as
// themselves, so they replay only if the script or library they name is
// restored before them (and a flush must be replayed so a later failed call
// stays failed).
func persistedCommand(cmd string, args [][]byte) bool {
	if writeCommands[cmd] {
		return true
	}
	if len(args) == 0 || (cmd != "SCRIPT" && cmd != "FUNCTION") {
		return false
	}
	sub := strings.ToUpper(string(args[0]))
	switch cmd {
	case "SCRIPT":
		return sub == "LOAD" || sub == "FLUSH"
	case "FUNCTION":
		return sub == "LOAD" || sub == "CREATE" || sub == "DELETE" || sub == "FLUSH" || sub == "RESTORE"
	}
	return false
}

type Server struct {
	cfg           *config.Config
	listener      net.Listener
	router        *command.Router
	store         *store.Store
	httpServer    *HTTPServer
	pluginMgr     *plugin.Manager
	metricsPlugin *metrics.MetricsPlugin
	metricsServer *http.Server
	aof           *persistence.AOFWriter
	conns         sync.Map
	connID        atomic.Int64
	connCount     atomic.Int64
	stopping      atomic.Bool
	stopCh        chan struct{}
	wg            sync.WaitGroup
	// sentinelCancel stops the Sentinel peer listener started in Start().
	sentinelCancel context.CancelFunc
	// dbs holds the numbered databases; database 0 starts out as store.
	dbs *store.Databases
	// repl is the replication state: master side and master link.
	repl *replication.Manager
	// gate orders every write with its propagation to the AOF and the
	// replicas, and lets a full resync snapshot the data between writes.
	gate writeGate
}

func New(cfg *config.Config) (*Server, error) {
	if cfg.Cluster.Enabled {
		return nil, errClusterUnsupported
	}
	s := &Server{
		cfg:    cfg,
		store:  store.NewStore(),
		router: command.NewRouter(),
		stopCh: make(chan struct{}),
	}

	// The plugin manager exists unconditionally: command hooks and store
	// event hooks (evict/expire/tag-invalidate) dispatch to whatever is
	// registered; an empty manager is a no-op.
	s.pluginMgr = plugin.NewManager()

	// Wire the metrics plugin (command hooks + standalone Prometheus endpoint)
	// when enabled; see config: plugins.metrics.{enabled,port,path}.
	if cfg.Plugins.Metrics.Enabled {
		s.metricsPlugin = metrics.New(true)
		if err := s.pluginMgr.Register(s.metricsPlugin); err != nil {
			return nil, err
		}
	}

	// Bridge the store's lifecycle events (eviction, expiry, tag
	// invalidation) to the registered plugin consumers.
	s.store.SetHooks(s.storeHooks())

	// Slow log: honor the configured enabled flag, threshold, and size.
	store.GlobalSlowLog.SetEnabled(cfg.Plugins.SlowLog.Enabled)
	if d, err := time.ParseDuration(cfg.Plugins.SlowLog.Threshold); err == nil && d >= 0 {
		store.GlobalSlowLog.SetThreshold(d)
	}
	if cfg.Plugins.SlowLog.MaxEntries > 0 {
		store.GlobalSlowLog.MaxSize = cfg.Plugins.SlowLog.MaxEntries
	}

	// Configure memory limits and eviction
	if maxMem, err := config.ParseMemorySize(cfg.Memory.MaxMemory); err == nil && maxMem > 0 {
		policy := parseEvictionPolicy(cfg.Memory.EvictionPolicy)
		sampleSize := cfg.Memory.SampleSize
		if sampleSize <= 0 {
			sampleSize = 5
		}
		s.store.ConfigureMemory(maxMem, policy, cfg.Memory.WarningPct, cfg.Memory.CriticalPct, sampleSize)
		logger.Info().
			Int64("max_memory_bytes", maxMem).
			Str("eviction_policy", cfg.Memory.EvictionPolicy).
			Msg("memory limits configured")
	}

	command.RegisterStringCommands(s.router)
	command.RegisterServerCommands(s.router)
	command.RegisterKeyCommands(s.router)
	command.RegisterHashCommands(s.router)
	command.RegisterListCommands(s.router)
	command.RegisterSetCommands(s.router)
	command.RegisterTagCommands(s.router)
	command.RegisterNamespaceCommands(s.router)
	command.RegisterClusterCommands(s.router)
	command.RegisterClientCommands(s.router)
	command.RegisterConfigCommands(s.router)
	command.RegisterTransactionCommands(s.router)
	command.RegisterSortedSetCommands(s.router)
	command.RegisterPubSubCommands(s.router)
	command.RegisterBitmapCommands(s.router)
	command.RegisterHyperLogLogCommands(s.router)
	command.RegisterStreamCommands(s.router)
	command.RegisterGeoCommands(s.router)
	command.RegisterScriptCommands(s.router)
	command.RegisterDebugCommands(s.router)
	command.RegisterCacheCommands(s.router)
	command.RegisterFunctionCommands(s.router)
	command.RegisterModuleCommands(s.router)
	command.RegisterSentinelCommands(s.router)
	// Feed the loaded `sentinel` config section into the command layer, which
	// builds the singleton lazily on the first SENTINEL command. Without this
	// the values in config.Sentinel would never reach a Sentinel instance.
	command.ConfigureSentinel(sentinel.Config{
		ID:           cfg.Sentinel.ID,
		Addr:         cfg.Sentinel.Addr,
		Port:         cfg.Sentinel.Port,
		Quorum:       cfg.Sentinel.Quorum,
		DownAfter:    cfg.Sentinel.DownAfterDuration(),
		FailoverTime: cfg.Sentinel.FailoverTimeDuration(),
		Seeds:        cfg.Sentinel.Seeds,
	})
	command.RegisterJSONCommands(s.router)
	command.RegisterTSCommands(s.router)
	command.RegisterSearchCommands(s.router)
	command.RegisterProbabilisticCommands(s.router)
	command.RegisterGraphCommands(s.router)
	command.RegisterDigestCommands(s.router)
	command.RegisterUtilityCommands(s.router)
	command.RegisterMonitoringCommands(s.router)
	command.RegisterCacheWarmingCommands(s.router)
	command.RegisterStatsCommands(s.router)
	command.RegisterSchedulerCommands(s.router)
	command.RegisterEventCommands(s.router)
	command.RegisterUtilityExtCommands(s.router)
	command.RegisterTemplateCommands(s.router)
	command.RegisterWorkflowCommands(s.router)
	command.RegisterDataStructuresCommands(s.router)
	command.RegisterEncodingCommands(s.router)
	command.RegisterActorCommands(s.router)
	command.RegisterMVCCCommands(s.router)
	command.RegisterIntegrationCommands(s.router)
	command.RegisterExtendedCommands(s.router)
	command.RegisterMoreCommands(s.router)
	command.RegisterExtraCommands(s.router)
	command.RegisterAdvancedCommands2(s.router)
	command.RegisterResilienceCommands(s.router)
	command.RegisterMLCommands(s.router)

	// Database 0 is s.store; the others are created on first use and share
	// its Pub/Sub hub, memory tracker and hooks, configured above.
	s.dbs = store.NewDatabases(s.store, cfg.Server.Databases)

	s.newReplication(cfg)

	if cfg.Server.RequirePass != "" {
		s.router.SetRequirePass(cfg.Server.RequirePass)
	}

	if cfg.HTTP.Enabled {
		bind := cfg.HTTP.Bind
		if bind == "" {
			bind = cfg.Server.Bind
		}
		// An unparsable window falls back to the one-minute default.
		window, err := time.ParseDuration(cfg.HTTP.RateWindow)
		if err != nil {
			window = 0
		}
		httpCfg := &HTTPConfig{
			Enabled:    cfg.HTTP.Enabled,
			Bind:       bind,
			Port:       cfg.HTTP.Port,
			Password:   cfg.HTTP.Password,
			CORSOrigin: cfg.HTTP.CORSOrigin,
			RateLimit:  cfg.HTTP.RateLimit,
			RateWindow: window,
			Protected:  cfg.Server.ProtectedMode,
		}
		s.httpServer = NewHTTPServer(s.store, s.router, httpCfg)
		s.httpServer.dbs = s.dbs
		s.httpServer.connCount = func() int64 { return s.connCount.Load() }
		s.httpServer.srv = s
	}

	// Configure AOF persistence
	if cfg.Persistence.Enabled && cfg.Persistence.AOF {
		dataDir := cfg.Persistence.DataDir
		if dataDir == "" {
			dataDir = "."
		}
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return nil, err
		}
		aofCfg := persistence.AOFConfig{
			Enabled:    true,
			Filename:   "appendonly.aof",
			DataDir:    dataDir,
			SyncPolicy: persistence.SyncPolicyFromString(cfg.Persistence.AOFSync),
		}
		s.aof = persistence.NewAOFWriter(aofCfg)

		// Load existing AOF data. A torn tail left by a crash is cut off so
		// the next append does not land after it; anything else that fails to
		// parse is corruption, and starting empty would hide the loss while
		// new writes went on being appended to the unreadable file.
		aofPath := filepath.Join(dataDir, "appendonly.aof")
		if info, err := os.Stat(aofPath); err == nil {
			reader := persistence.NewAOFReader()
			commands, valid, err := reader.LoadValid(aofPath)
			if err != nil {
				return nil, fmt.Errorf("AOF %s is corrupt: %w", aofPath, err)
			}
			if valid < info.Size() {
				logger.Warn().
					Int64("size", info.Size()).
					Int64("valid", valid).
					Msg("AOF ends with an incomplete command, truncating it")
				if err := os.Truncate(aofPath, valid); err != nil {
					return nil, fmt.Errorf("truncate AOF %s: %w", aofPath, err)
				}
			}
			if len(commands) > 0 {
				lastDB := s.replayAOF(commands)
				// Appends continue where the file left off, so the writer
				// starts in the database the replay ended in.
				s.aof.SetCurrentDB(lastDB)
				logger.Info().Int("commands", len(commands)).Msg("AOF data restored")
			}
		}

		s.loadReplicationState(dataDir)
	}

	// Every write the router executes goes to the AOF, when it is enabled,
	// and to the replication stream.
	s.router.SetPostExecuteDB(func(db int, cmd string, args [][]byte) {
		upperCmd := strings.ToUpper(cmd)
		if !persistedCommand(upperCmd, args) {
			return
		}
		if s.aof != nil {
			if err := s.aof.AppendDB(db, upperCmd, args); err != nil {
				logger.Error().Err(err).Str("cmd", cmd).Msg("AOF append failed")
			}
		}
		s.repl.Feed(db, upperCmd, args)
	})

	// EXEC runs its queue outside the per-command hook; its writes are
	// recorded here as one MULTI ... EXEC block, as are a script's effects.
	s.router.SetPostExec(func(cmds []command.ExecutedCommand) {
		writes := make([]command.ExecutedCommand, 0, len(cmds))
		for _, c := range cmds {
			upperCmd := strings.ToUpper(c.Name)
			if persistedCommand(upperCmd, c.Args) {
				writes = append(writes, command.ExecutedCommand{Name: upperCmd, Args: c.Args, DB: c.DB})
			}
		}
		if len(writes) == 0 {
			return
		}
		if s.aof != nil {
			aofWrites := make([]persistence.Command, len(writes))
			for i, c := range writes {
				aofWrites[i] = persistence.Command{Name: c.Name, Args: c.Args, DB: c.DB}
			}
			if err := s.aof.AppendTransaction(aofWrites); err != nil {
				logger.Error().Err(err).Msg("AOF transaction append failed")
			}
		}
		s.repl.FeedTransaction(writes)
	})

	// Give SAVE/BGSAVE something real to do. They used to stamp a timestamp and
	// reply +OK while performing no I/O, so a client treating SAVE as a
	// durability barrier got a guarantee the server never kept. s.aof is read
	// through the pointer at call time, so this is correct whether or not AOF is
	// configured; when it is not, the hook reports that instead of pretending.
	command.SetSaveHook(func() error {
		if s.aof == nil {
			return errors.New("ERR persistence is not configured")
		}
		return s.aof.Flush()
	})

	return s, nil
}

func (s *Server) Start(_ context.Context) error {
	startupComplete := false
	// Start AOF writer if configured
	if s.aof != nil {
		if err := s.aof.Start(); err != nil {
			return err
		}
		defer func() {
			if !startupComplete {
				s.aof.Stop()
			}
		}()
	}

	addr := net.JoinHostPort(s.cfg.Server.Bind, strconv.Itoa(s.cfg.Server.Port))

	var listener net.Listener
	var err error

	if s.cfg.Server.TLSCertFile != "" && s.cfg.Server.TLSKeyFile != "" {
		cert, tlsErr := tls.LoadX509KeyPair(s.cfg.Server.TLSCertFile, s.cfg.Server.TLSKeyFile)
		if tlsErr != nil {
			return tlsErr
		}
		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
			CurvePreferences: []tls.CurveID{
				tls.X25519,
				tls.CurveP256,
			},
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			},
		}
		listener, err = tls.Listen("tcp", addr, tlsCfg)
		if err != nil {
			return err
		}
		logger.Info().Str("addr", addr).Msg("CacheStorm server started (TLS)")
	} else {
		listener, err = net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		logger.Info().Str("addr", addr).Msg("CacheStorm server started")
	}

	s.listener = listener
	s.router.SetPort(s.ListeningPort())

	// Active expiration: remove expired keys without requiring reads.
	s.dbs.StartExpiry()

	s.repl.Start()
	if rc := s.cfg.Replication; rc.IsReplica() {
		if err := s.repl.ReplicaOf(rc.MasterHost, rc.MasterPort); err != nil {
			listener.Close()
			s.repl.Stop()
			return fmt.Errorf("replication: %w", err)
		}
	}

	go s.acceptLoop()

	if s.httpServer != nil {
		go func() {
			defer logger.RecoverPanic("http-server")
			logger.Info().
				Int("port", s.cfg.HTTP.Port).
				Msg("HTTP admin server started")
			if err := s.httpServer.Start(); err != nil {
				logger.Error().Err(err).Msg("HTTP server error")
			}
		}()
	}

	if s.metricsPlugin != nil {
		s.startMetricsServer()
	}

	if s.cfg.Sentinel.Enabled {
		s.startSentinel()
	}

	startupComplete = true
	return nil
}

// startMetricsServer serves the metrics plugin's Prometheus exposition on
// the configured plugins.metrics.{port,path} and keeps its gauges current.
func (s *Server) startMetricsServer() {
	mux := http.NewServeMux()
	mux.HandleFunc(s.cfg.Plugins.Metrics.Path, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		fmt.Fprint(w, s.metricsPlugin.ExportPrometheus())
	})

	s.metricsServer = &http.Server{
		Addr:    net.JoinHostPort(s.cfg.Server.Bind, strconv.Itoa(s.cfg.Plugins.Metrics.Port)),
		Handler: mux,
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer logger.RecoverPanic("metrics-server")
		logger.Info().
			Int("port", s.cfg.Plugins.Metrics.Port).
			Str("path", s.cfg.Plugins.Metrics.Path).
			Msg("metrics server started")
		if err := s.metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error().Err(err).Msg("metrics server error")
		}
	}()

	// Refresh the advertised counters and gauges periodically.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				s.updateMetricsGauges()
			}
		}
	}()
}

// updateMetricsGauges pushes current store and connection counters into the
// metrics plugin so the Prometheus exposition reflects live state.
func (s *Server) updateMetricsGauges() {
	s.metricsPlugin.SetKeysTotal(s.dbs.KeyCount())
	s.metricsPlugin.SetMemoryBytes(s.dbs.MemUsage())
	s.metricsPlugin.SetConnectedClients(s.connCount.Load())
	s.metricsPlugin.SetHitCount(store.GlobalMetrics.TotalHits.Load())
	s.metricsPlugin.SetMissCount(store.GlobalMetrics.TotalMisses.Load())
	s.metricsPlugin.SetEvictedCount(store.GlobalMetrics.TotalEvictions.Load())
	s.metricsPlugin.SetExpiredCount(store.GlobalMetrics.TotalExpirations.Load())
}

func (s *Server) acceptLoop() {
	// backoff paces retries after a failed Accept. A persistent failure such
	// as running out of file descriptors returns at once on every call, so
	// retrying immediately spun a core and flooded the log.
	var backoff time.Duration
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			if s.stopping.Load() {
				return
			}
			if backoff == 0 {
				backoff = 5 * time.Millisecond
			} else if backoff *= 2; backoff > time.Second {
				backoff = time.Second
			}
			logger.Error().Err(err).Dur("retry_in", backoff).Msg("accept error")
			select {
			case <-s.stopCh:
				return
			case <-time.After(backoff):
			}
			continue
		}
		backoff = 0

		// Enforce MaxConnections. Redis tells the client why it is refused.
		maxConns := int64(s.cfg.Server.MaxConnections)
		if maxConns > 0 && s.connCount.Load() >= maxConns {
			_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
			if _, err := conn.Write([]byte("-ERR max number of clients reached\r\n")); err != nil {
				logger.Debug().Err(err).Msg("refused connection closed before the error was sent")
			}
			conn.Close()
			continue
		}

		if s.cfg.Server.ProtectedMode && s.router.RequirePass() == "" && !isLoopbackAddr(conn.RemoteAddr().String()) {
			_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
			if _, err := conn.Write([]byte("-DENIED " + protectedModeMessage + "\r\n")); err != nil {
				logger.Debug().Err(err).Msg("refused connection closed before the error was sent")
			}
			conn.Close()
			continue
		}

		connID := s.connID.Add(1)
		s.connCount.Add(1)
		store.GlobalMetrics.RecordConnection()
		c := NewConnection(connID, conn, s.store, s.router, s.pluginMgr)
		c.dbs = s.dbs
		c.srv = s

		// Apply configured timeouts
		if rt := s.cfg.Server.ReadTimeoutDuration(); rt > 0 {
			c.readTimeout = rt
		}
		if wt := s.cfg.Server.WriteTimeoutDuration(); wt > 0 {
			c.writeTimeout = wt
		}

		s.conns.Store(connID, c)
		s.wg.Add(1)

		go func() {
			defer s.wg.Done()
			defer s.conns.Delete(connID)
			defer s.connCount.Add(-1)
			defer store.GlobalMetrics.RecordDisconnection()
			c.Handle()
		}()
	}
}

// startSentinel launches the Sentinel monitor and gossip loops and listens for
// Sentinel peer traffic (SENTINEL MONITOR, PING, INFO from other sentinels) on
// the configured sentinel port. Serve blocks, so it runs on its own goroutine
// and closes its listener when its context is cancelled in Stop(). A bind
// failure is logged rather than fatal: the data server works without peer
// gossip.
func (s *Server) startSentinel() {
	if err := command.StartSentinel(); err != nil {
		logger.Warn().Err(err).Msg("sentinel not started")
	}
	sentinelCtx, sentinelCancel := context.WithCancel(context.Background())
	s.sentinelCancel = sentinelCancel
	go func() {
		defer logger.RecoverPanic("sentinel-serve")
		// Serve only returns when the listener dies: either a bind failure or
		// the Accept error raised when shutdown closes it. Either way, log it.
		logger.Warn().Err(command.ServeSentinel(sentinelCtx)).Msg("sentinel peer listener stopped")
	}()
}

func (s *Server) Stop(ctx context.Context) error {
	s.stopping.Store(true)
	close(s.stopCh)

	// 1. Stop accepting new connections
	if s.listener != nil {
		s.listener.Close()
	}
	logger.Info().Msg("stopped accepting new connections")

	// 1a. Stop replication: the master link and every attached replica, so
	// no write arrives from the master while the server drains.
	s.repl.Stop()

	// 1b. Stop the Sentinel monitor and gossip loops. Stop joins both via
	// wg.Wait, so they cannot outlive the server.
	command.StopSentinel()

	// 1c. Close the Sentinel peer listener so Serve returns and its port is
	// released before the server goes away.
	if s.sentinelCancel != nil {
		s.sentinelCancel()
	}

	// 2. Stop HTTP server
	if s.httpServer != nil {
		if err := s.httpServer.Stop(); err != nil {
			logger.Error().Err(err).Msg("HTTP server stop error")
		}
	}

	// 2b. Stop the metrics endpoint and plugins
	if s.metricsServer != nil {
		metricsCtx, metricsCancel := context.WithTimeout(ctx, 5*time.Second)
		if err := s.metricsServer.Shutdown(metricsCtx); err != nil {
			logger.Error().Err(err).Msg("metrics server stop error")
		}
		metricsCancel()
	}
	if s.pluginMgr != nil {
		if err := s.pluginMgr.CloseAll(); err != nil {
			logger.Error().Err(err).Msg("plugin close error")
		}
	}

	// Stop active expiration with the server.
	s.dbs.StopExpiry()

	// 3. Let every connection finish the command it is running, then close.
	// Idle clients would otherwise hold the drain for its whole timeout.
	s.conns.Range(func(_, value any) bool {
		if c, ok := value.(*Connection); ok {
			c.Drain()
		}
		return true
	})

	// Wait for in-flight requests to complete (with timeout)
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	drainTimeout := 10 * time.Second
	drainCtx, drainCancel := context.WithTimeout(ctx, drainTimeout)
	defer drainCancel()

	select {
	case <-done:
		logger.Info().Msg("all connections drained gracefully")
	case <-drainCtx.Done():
		// 4. Force close remaining connections after drain timeout
		logger.Warn().Msg("drain timeout reached, force closing connections")
		s.conns.Range(func(_, value any) bool {
			if c, ok := value.(*Connection); ok {
				c.Close()
			}
			return true
		})

		// Fresh context: parent ctx is already expired (that's why we're here), so a new base is needed
		forceCtx, forceCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer forceCancel()
		select {
		case <-done:
		case <-forceCtx.Done():
			logger.Error().Msg("some connections did not close cleanly")
		}
	}

	// 5. Stop AOF writer (flush remaining data)
	if s.aof != nil {
		s.aof.Stop()
		// A replica records where in its master's stream the AOF ends.
		s.saveReplicationState()
	}

	logger.Info().Msg("CacheStorm server stopped")
	return nil
}

// Store returns database 0.
func (s *Server) Store() *store.Store {
	return s.dbs.DB(0)
}

// Databases returns every database the server serves.
func (s *Server) Databases() *store.Databases {
	return s.dbs
}

// replayAOF applies the AOF's commands, following its SELECTs, and returns
// the database the last command left selected.
func (s *Server) replayAOF(commands []persistence.Command) int {
	replayed := 0
	failed := 0
	db := 0
	apply := func(cmd persistence.Command) {
		ctx := command.NewContext(cmd.Name, cmd.Args, s.dbs.DB(db), nil)
		ctx.DB = db
		ctx.DBs = s.dbs
		err := s.router.ExecuteSilent(ctx)
		db = ctx.DB
		if err != nil {
			failed++
			if failed <= 10 {
				logger.Warn().Err(err).Str("cmd", cmd.Name).Msg("AOF replay command failed")
			}
		} else {
			replayed++
		}
	}

	// A MULTI ... EXEC block is applied only once its EXEC marker is read, so
	// a transaction the crash cut short is dropped as a whole.
	var pending []persistence.Command
	inTx := false
	for _, cmd := range commands {
		switch strings.ToUpper(cmd.Name) {
		case "MULTI":
			pending = pending[:0]
			inTx = true
			continue
		case "EXEC":
			for _, c := range pending {
				apply(c)
			}
			pending = pending[:0]
			inTx = false
			continue
		}
		if inTx {
			pending = append(pending, cmd)
			continue
		}
		apply(cmd)
	}
	if inTx {
		logger.Warn().
			Int("commands", len(pending)).
			Msg("AOF ends inside a transaction, discarding the incomplete transaction")
	}
	if failed > 0 {
		logger.Warn().
			Int("total", len(commands)).
			Int("replayed", replayed).
			Int("failed", failed).
			Msg("AOF replay completed with errors")
	}
	return db
}

func parseEvictionPolicy(name string) store.EvictionPolicy {
	switch name {
	case "noeviction":
		return store.EvictionNoEviction
	case "allkeys-lru":
		return store.EvictionAllKeysLRU
	case "allkeys-lfu":
		return store.EvictionAllKeysLFU
	case "volatile-lru":
		return store.EvictionVolatileLRU
	case "allkeys-random":
		return store.EvictionAllKeysRandom
	default:
		return store.EvictionAllKeysLRU
	}
}
