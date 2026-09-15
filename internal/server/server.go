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
	"github.com/cachestorm/cachestorm/internal/store"
	"github.com/cachestorm/cachestorm/plugins/metrics"
)

// writeCommands lists commands that mutate state and should be persisted to AOF
var writeCommands = map[string]bool{
	"SET": true, "SETNX": true, "SETEX": true, "PSETEX": true, "MSET": true, "MSETNX": true,
	"APPEND": true, "INCR": true, "DECR": true, "INCRBY": true, "DECRBY": true, "INCRBYFLOAT": true,
	"DEL": true, "UNLINK": true, "RENAME": true, "RENAMENX": true, "EXPIRE": true, "EXPIREAT": true,
	"PEXPIRE": true, "PEXPIREAT": true, "PERSIST": true,
	"HSET": true, "HSETNX": true, "HMSET": true, "HDEL": true, "HINCRBY": true, "HINCRBYFLOAT": true,
	"LPUSH": true, "RPUSH": true, "LPOP": true, "RPOP": true, "LSET": true, "LREM": true,
	"LTRIM": true, "LINSERT": true, "RPOPLPUSH": true, "LMOVE": true,
	"SADD": true, "SREM": true, "SPOP": true, "SMOVE": true,
	"ZADD": true, "ZREM": true, "ZINCRBY": true, "ZRANGESTORE": true,
	"XADD": true, "XDEL": true, "XTRIM": true,
	"PFADD": true, "PFMERGE": true,
	"SETBIT": true, "BITOP": true, "BITFIELD": true,
	"GEOADD": true, "GEORADIUS": true,
	"SETTAG": true, "INVALIDATE": true,
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
}

func New(cfg *config.Config) (*Server, error) {
	s := &Server{
		cfg:    cfg,
		store:  store.NewStore(),
		router: command.NewRouter(),
		stopCh: make(chan struct{}),
	}

	// Wire the metrics plugin (command hooks + standalone Prometheus endpoint)
	// when enabled; see config: plugins.metrics.{enabled,port,path}.
	if cfg.Plugins.Metrics.Enabled {
		s.pluginMgr = plugin.NewManager()
		s.metricsPlugin = metrics.New(true)
		if err := s.pluginMgr.Register(s.metricsPlugin); err != nil {
			return nil, err
		}
	}

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
	command.RegisterReplicationCommands(s.router)
	command.RegisterFunctionCommands(s.router)
	command.RegisterModuleCommands(s.router)
	command.RegisterSentinelCommands(s.router)
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

	command.InitReplicationManager(s.store)

	if cfg.Server.RequirePass != "" {
		s.router.SetRequirePass(cfg.Server.RequirePass)
	}

	if cfg.HTTP.Enabled {
		httpCfg := &HTTPConfig{
			Enabled:  cfg.HTTP.Enabled,
			Port:     cfg.HTTP.Port,
			Password: cfg.HTTP.Password,
		}
		s.httpServer = NewHTTPServer(s.store, s.router, httpCfg)
		s.httpServer.connCount = func() int64 { return s.connCount.Load() }
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

		// Load existing AOF data
		aofPath := filepath.Join(dataDir, "appendonly.aof")
		if _, err := os.Stat(aofPath); err == nil {
			reader := persistence.NewAOFReader()
			commands, err := reader.Load(aofPath)
			if err != nil {
				logger.Warn().Err(err).Msg("failed to load AOF, starting fresh")
			} else if len(commands) > 0 {
				s.replayAOF(commands)
				logger.Info().Int("commands", len(commands)).Msg("AOF data restored")
			}
		}

		// Set post-execute hook on router to persist write commands
		s.router.SetPostExecute(func(cmd string, args [][]byte) {
			upperCmd := strings.ToUpper(cmd)
			if writeCommands[upperCmd] {
				if err := s.aof.Append(upperCmd, args); err != nil {
					logger.Error().Err(err).Str("cmd", cmd).Msg("AOF append failed")
				}
			}
		})
	}

	return s, nil
}

func (s *Server) Start(_ context.Context) error {
	// Start AOF writer if configured
	if s.aof != nil {
		if err := s.aof.Start(); err != nil {
			return err
		}
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

	// Active expiration: remove expired keys without requiring reads.
	s.store.StartExpiry()

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
	s.metricsPlugin.SetKeysTotal(int64(s.store.KeyCount()))
	s.metricsPlugin.SetMemoryBytes(s.store.MemUsage())
	s.metricsPlugin.SetConnectedClients(s.connCount.Load())
	s.metricsPlugin.SetHitCount(store.GlobalMetrics.TotalHits.Load())
	s.metricsPlugin.SetMissCount(store.GlobalMetrics.TotalMisses.Load())
	s.metricsPlugin.SetEvictedCount(store.GlobalMetrics.TotalEvictions.Load())
	s.metricsPlugin.SetExpiredCount(store.GlobalMetrics.TotalExpirations.Load())
}

func (s *Server) acceptLoop() {
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
			logger.Error().Err(err).Msg("accept error")
			continue
		}

		// Enforce MaxConnections
		maxConns := int64(s.cfg.Server.MaxConnections)
		if maxConns > 0 && s.connCount.Load() >= maxConns {
			conn.Close()
			continue
		}

		connID := s.connID.Add(1)
		s.connCount.Add(1)
		store.GlobalMetrics.RecordConnection()
		c := NewConnection(connID, conn, s.store, s.router, s.pluginMgr)

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

func (s *Server) Stop(ctx context.Context) error {
	s.stopping.Store(true)
	close(s.stopCh)

	// 1. Stop accepting new connections
	if s.listener != nil {
		s.listener.Close()
	}
	logger.Info().Msg("stopped accepting new connections")

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
	s.store.StopExpiry()

	// 3. Wait for in-flight requests to complete (with timeout)
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
	}

	logger.Info().Msg("CacheStorm server stopped")
	return nil
}

func (s *Server) Store() *store.Store {
	return s.store
}

func (s *Server) replayAOF(commands []persistence.Command) {
	replayed := 0
	failed := 0
	for _, cmd := range commands {
		ctx := command.NewContext(cmd.Name, cmd.Args, s.store, nil)
		if err := s.router.ExecuteSilent(ctx); err != nil {
			failed++
			if failed <= 10 {
				logger.Warn().Err(err).Str("cmd", cmd.Name).Msg("AOF replay command failed")
			}
		} else {
			replayed++
		}
	}
	if failed > 0 {
		logger.Warn().
			Int("total", len(commands)).
			Int("replayed", replayed).
			Int("failed", failed).
			Msg("AOF replay completed with errors")
	}
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
