# CacheStorm — Configuration & CLI Specification

## 1. CLI Flags

```
Usage: cachestorm [flags]

Flags:
  --config string      Path to config file (default: cachestorm.yaml in CWD, then /etc/cachestorm/cachestorm.yaml)
  --port int           Server port (overrides config, default: 6380)
  --bind string        Bind address (overrides config, default: 0.0.0.0)
  --maxmemory string   Max memory limit (overrides config, e.g., "2gb", "512mb")
  --loglevel string    Log level (overrides config: debug, info, warn, error)
  --logformat string   Log format (overrides config: json, console)
  --cluster            Enable cluster mode (overrides config)
  --cluster-seeds string  Comma-separated seed nodes (overrides config)
  --data-dir string    Persistence data directory (overrides config)
  --version            Print version and exit
  --help               Print help and exit
```

**Priority order:** CLI flags > Environment variables > Config file > Defaults

## 2. Environment Variables

Every config option can be set via environment variable with `CACHESTORM_` prefix:

```
CACHESTORM_PORT=6380
CACHESTORM_BIND=0.0.0.0
CACHESTORM_MAX_MEMORY=2gb
CACHESTORM_EVICTION_POLICY=allkeys-lru
CACHESTORM_LOG_LEVEL=info
CACHESTORM_LOG_FORMAT=json
CACHESTORM_CLUSTER_ENABLED=true
CACHESTORM_CLUSTER_NODE_NAME=node-1
CACHESTORM_CLUSTER_SEEDS=10.0.0.2:7946,10.0.0.3:7946
CACHESTORM_PERSISTENCE_ENABLED=true
CACHESTORM_PERSISTENCE_DATA_DIR=/var/lib/cachestorm
CACHESTORM_AUTH_PASSWORD=mysecret
CACHESTORM_METRICS_PORT=9090
```

## 3. Full Config File Reference

```yaml
# cachestorm.yaml — CacheStorm Configuration
# All values shown are defaults unless otherwise noted.

# ─── Server ───────────────────────────────────────────────
server:
  # Address to bind the RESP TCP server
  bind: "0.0.0.0"
  # Port for RESP protocol (Redis clients connect here)
  port: 6380
  # Maximum simultaneous client connections
  max_connections: 10000
  # TCP keepalive interval in seconds (0 = disabled)
  tcp_keepalive: 300
  # Read timeout per command (0 = no timeout)
  # Format: "30s", "5m", "0"
  read_timeout: "0"
  # Write timeout per response (0 = no timeout)
  write_timeout: "0"
  # Per-connection read buffer size in bytes
  read_buffer_size: 4096
  # Per-connection write buffer size in bytes
  write_buffer_size: 4096

# ─── Memory ───────────────────────────────────────────────
memory:
  # Maximum memory CacheStorm will use for data storage.
  # 0 = no limit (use all available system memory).
  # Supports: "100mb", "2gb", "1tb", or raw bytes as string.
  max_memory: "0"
  # Eviction policy when max_memory is reached.
  # Options:
  #   noeviction     — return error on write when full
  #   allkeys-lru    — evict least recently used key
  #   allkeys-lfu    — evict least frequently used key
  #   volatile-lru   — evict LRU among keys with TTL only
  #   allkeys-random — evict random key
  eviction_policy: "allkeys-lru"
  # Memory pressure thresholds (percentage of max_memory).
  # At warning: eviction starts (small batches).
  # At critical: aggressive eviction (large batches).
  # Above emergency (95): reject all writes.
  pressure_warning: 70
  pressure_critical: 85
  # Number of keys to sample when selecting eviction candidate.
  # Higher = more accurate LRU/LFU but slower.
  eviction_sample_size: 5

# ─── Namespaces ───────────────────────────────────────────
# Pre-configure namespaces with custom settings.
# The "default" namespace always exists.
# Namespaces not listed here are created with global defaults.
namespaces:
  default:
    # Default TTL for keys in this namespace.
    # 0 = no default TTL (keys live forever unless explicitly set).
    # Format: "30s", "5m", "1h", "24h", "7d"
    default_ttl: "0"
    # Per-namespace memory limit (0 = uses global max_memory proportionally)
    max_memory: "0"
  # Example: sessions namespace with 24h default TTL
  # sessions:
  #   default_ttl: "24h"
  #   max_memory: "512mb"

# ─── Cluster ──────────────────────────────────────────────
cluster:
  # Enable cluster mode for horizontal scaling.
  # When false, CacheStorm runs as a single standalone node.
  enabled: false
  # Human-readable node name (for logging). UUID is auto-generated.
  node_name: ""
  # Address to bind the gossip protocol listener.
  bind_addr: "0.0.0.0"
  # Port for gossip protocol communication.
  bind_port: 7946
  # Advertised address for NAT/Docker environments.
  # Empty = auto-detect from network interfaces.
  advertise_addr: ""
  # Advertised port (0 = same as bind_port).
  advertise_port: 0
  # Seed nodes to bootstrap cluster membership.
  # At least one seed should be reachable on startup.
  # Format: ["host:port", "host:port"]
  seeds: []
  # Number of replicas per primary node.
  replicas: 1
  # Maximum entries in replication backlog ring buffer.
  replication_backlog: 10000
  # Keys per batch during slot migration.
  migration_batch_size: 100
  # Gossip protocol interval.
  gossip_interval: "200ms"
  # Failure detection probe interval.
  probe_interval: "1s"
  # Probe response timeout.
  probe_timeout: "500ms"
  # Suspicion multiplier before declaring a node dead.
  suspicion_mult: 4

# ─── Persistence ──────────────────────────────────────────
persistence:
  # Enable disk persistence.
  # When false, all data is lost on restart.
  enabled: false
  # Enable Append-Only File logging.
  aof: true
  # AOF fsync policy:
  #   always   — fsync after every write (safest, slowest)
  #   everysec — fsync every second (recommended)
  #   no       — let OS decide (fastest, data loss risk)
  aof_sync: "everysec"
  # Interval between automatic snapshots.
  # Format: "5m", "1h", "0" (disabled)
  snapshot_interval: "5m"
  # Directory for AOF and snapshot files.
  data_dir: "/var/lib/cachestorm"
  # Maximum AOF file size before triggering rewrite.
  max_aof_size: "1gb"

# ─── Plugins ──────────────────────────────────────────────
plugins:
  # Stats plugin — tracks hit/miss ratios, command counts, latencies.
  stats:
    enabled: true

  # Metrics plugin — Prometheus metrics endpoint.
  metrics:
    enabled: true
    # HTTP port for metrics endpoint.
    port: 9090
    # URL path for Prometheus scraping.
    path: "/metrics"

  # Auth plugin — password-based authentication.
  auth:
    enabled: false
    # Password for AUTH command. Empty = no password.
    password: ""

  # SlowLog plugin — log slow queries.
  slowlog:
    enabled: true
    # Commands taking longer than this are logged.
    # Format: "1ms", "10ms", "100ms", "1s"
    threshold: "10ms"
    # Maximum number of slow log entries to keep (ring buffer).
    max_entries: 1000

# ─── Logging ──────────────────────────────────────────────
logging:
  # Log level: debug, info, warn, error
  level: "info"
  # Log format: json (structured, for production), console (human-readable, for dev)
  format: "json"
  # Log output: stdout, stderr, or file path
  output: "stdout"
```

## 4. Config Validation Rules

On startup, all config values are validated. Server refuses to start on invalid config.

```go
func (c *Config) Validate() error {
    var errs []error

    // Server
    if c.Server.Port < 1 || c.Server.Port > 65535 {
        errs = append(errs, fmt.Errorf("server.port must be 1-65535, got %d", c.Server.Port))
    }
    if c.Server.MaxConnections < 1 {
        errs = append(errs, fmt.Errorf("server.max_connections must be positive"))
    }
    if c.Server.ReadBufferSize < 512 {
        errs = append(errs, fmt.Errorf("server.read_buffer_size must be >= 512"))
    }

    // Memory
    if c.Memory.MaxMemory != "0" {
        if _, err := parseMemorySize(c.Memory.MaxMemory); err != nil {
            errs = append(errs, fmt.Errorf("memory.max_memory invalid: %w", err))
        }
    }
    validPolicies := map[string]bool{
        "noeviction": true, "allkeys-lru": true, "allkeys-lfu": true,
        "volatile-lru": true, "allkeys-random": true,
    }
    if !validPolicies[c.Memory.EvictionPolicy] {
        errs = append(errs, fmt.Errorf("memory.eviction_policy invalid: %s", c.Memory.EvictionPolicy))
    }
    if c.Memory.WarningPct < 0 || c.Memory.WarningPct > 100 {
        errs = append(errs, fmt.Errorf("memory.pressure_warning must be 0-100"))
    }
    if c.Memory.CriticalPct <= c.Memory.WarningPct {
        errs = append(errs, fmt.Errorf("memory.pressure_critical must be > pressure_warning"))
    }

    // Namespaces
    for name, ns := range c.Namespaces {
        if ns.DefaultTTL != "0" {
            if _, err := parseDuration(ns.DefaultTTL); err != nil {
                errs = append(errs, fmt.Errorf("namespaces.%s.default_ttl invalid: %w", name, err))
            }
        }
    }

    // Cluster
    if c.Cluster.Enabled {
        if c.Cluster.BindPort < 1 || c.Cluster.BindPort > 65535 {
            errs = append(errs, fmt.Errorf("cluster.bind_port must be 1-65535"))
        }
        if c.Cluster.Replicas < 0 {
            errs = append(errs, fmt.Errorf("cluster.replicas must be >= 0"))
        }
    }

    // Persistence
    if c.Persistence.Enabled {
        validSync := map[string]bool{"always": true, "everysec": true, "no": true}
        if !validSync[c.Persistence.AOFSync] {
            errs = append(errs, fmt.Errorf("persistence.aof_sync must be always/everysec/no"))
        }
        if c.Persistence.DataDir == "" {
            errs = append(errs, fmt.Errorf("persistence.data_dir required when persistence enabled"))
        }
    }

    // Plugins
    if c.Plugins.Auth.Enabled && c.Plugins.Auth.Password == "" {
        errs = append(errs, fmt.Errorf("plugins.auth.password required when auth enabled"))
    }
    if c.Plugins.Metrics.Enabled {
        if c.Plugins.Metrics.Port < 1 || c.Plugins.Metrics.Port > 65535 {
            errs = append(errs, fmt.Errorf("plugins.metrics.port must be 1-65535"))
        }
        if c.Plugins.Metrics.Port == c.Server.Port {
            errs = append(errs, fmt.Errorf("plugins.metrics.port must differ from server.port"))
        }
    }

    // Logging
    validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
    if !validLevels[c.Logging.Level] {
        errs = append(errs, fmt.Errorf("logging.level must be debug/info/warn/error"))
    }
    validFormats := map[string]bool{"json": true, "console": true}
    if !validFormats[c.Logging.Format] {
        errs = append(errs, fmt.Errorf("logging.format must be json/console"))
    }

    return errors.Join(errs...)
}
```

## 5. Memory Size Parser

```go
// parseMemorySize parses human-readable memory sizes.
// Supports: "100", "100b", "100kb", "100mb", "100gb", "100tb"
// Case insensitive.
func parseMemorySize(s string) (int64, error) {
    s = strings.TrimSpace(strings.ToLower(s))
    if s == "0" {
        return 0, nil
    }

    multipliers := map[string]int64{
        "b":  1,
        "kb": 1024,
        "mb": 1024 * 1024,
        "gb": 1024 * 1024 * 1024,
        "tb": 1024 * 1024 * 1024 * 1024,
    }

    for suffix, mult := range multipliers {
        if strings.HasSuffix(s, suffix) {
            numStr := strings.TrimSuffix(s, suffix)
            num, err := strconv.ParseFloat(numStr, 64)
            if err != nil {
                return 0, fmt.Errorf("invalid number: %s", numStr)
            }
            return int64(num * float64(mult)), nil
        }
    }

    // No suffix — treat as bytes
    return strconv.ParseInt(s, 10, 64)
}
```

## 6. Duration Parser

```go
// parseDuration parses extended duration strings.
// Supports Go standard durations ("5s", "10m", "1h") plus:
// "1d" = 24 hours, "7d" = 7 days
func parseDuration(s string) (time.Duration, error) {
    s = strings.TrimSpace(strings.ToLower(s))
    if s == "0" {
        return 0, nil
    }

    if strings.HasSuffix(s, "d") {
        numStr := strings.TrimSuffix(s, "d")
        days, err := strconv.ParseFloat(numStr, 64)
        if err != nil {
            return 0, err
        }
        return time.Duration(days * 24 * float64(time.Hour)), nil
    }

    return time.ParseDuration(s)
}
```

## 7. Config Loading Order

```go
func LoadConfig(flagConfigPath string) (*Config, error) {
    cfg := DefaultConfig()

    // 1. Try loading from file
    configPath := flagConfigPath
    if configPath == "" {
        // Try CWD first
        if _, err := os.Stat("cachestorm.yaml"); err == nil {
            configPath = "cachestorm.yaml"
        } else if _, err := os.Stat("/etc/cachestorm/cachestorm.yaml"); err == nil {
            configPath = "/etc/cachestorm/cachestorm.yaml"
        }
    }

    if configPath != "" {
        data, err := os.ReadFile(configPath)
        if err != nil {
            return nil, fmt.Errorf("read config: %w", err)
        }
        if err := yaml.Unmarshal(data, cfg); err != nil {
            return nil, fmt.Errorf("parse config: %w", err)
        }
    }

    // 2. Override with environment variables
    applyEnvOverrides(cfg)

    // 3. Override with CLI flags (caller does this after LoadConfig)

    // 4. Validate
    if err := cfg.Validate(); err != nil {
        return nil, fmt.Errorf("config validation: %w", err)
    }

    return cfg, nil
}
```

## 8. Startup Sequence

```go
func main() {
    // 1. Parse CLI flags
    flags := parseFlags()

    // 2. Print version if requested
    if flags.Version {
        fmt.Printf("CacheStorm %s (%s)\n", version, commit)
        os.Exit(0)
    }

    // 3. Load and validate config
    cfg, err := config.LoadConfig(flags.ConfigPath)
    handleFatal(err, "load config")

    // 4. Apply CLI flag overrides
    applyCLIOverrides(cfg, flags)

    // 5. Initialize logger
    log := logger.Init(cfg.Logging)

    // 6. Print startup banner
    log.Info().
        Str("version", version).
        Str("port", fmt.Sprintf("%d", cfg.Server.Port)).
        Str("pid", fmt.Sprintf("%d", os.Getpid())).
        Msg("CacheStorm starting")

    // 7. Create store (namespace manager + shards + tag index + TTL wheel + eviction)
    store := store.New(cfg)

    // 8. Create plugin manager + register enabled plugins
    plugins := plugin.NewManager()
    loadPlugins(cfg, plugins)

    // 9. Initialize plugins (OnStartup hooks run here, recovery happens here)
    err = plugins.InitAll(cfg)
    handleFatal(err, "init plugins")

    // 10. Create command router + register all commands
    router := command.NewRouter(store, plugins)

    // 11. Create and start cluster (if enabled)
    var clust *cluster.Cluster
    if cfg.Cluster.Enabled {
        clust, err = cluster.New(cfg.Cluster, store)
        handleFatal(err, "init cluster")
        err = clust.Start()
        handleFatal(err, "start cluster")
    }

    // 12. Create and start TCP server
    srv := server.New(cfg, router, store, plugins, clust)

    // 13. Setup graceful shutdown
    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    // 14. Start server (blocks until context cancelled)
    go func() {
        if err := srv.Start(ctx); err != nil {
            log.Fatal().Err(err).Msg("server failed")
        }
    }()

    log.Info().
        Str("bind", cfg.Server.Bind).
        Int("port", cfg.Server.Port).
        Msg("CacheStorm ready to accept connections")

    // 15. Wait for shutdown signal
    <-ctx.Done()
    log.Info().Msg("shutdown signal received")

    // 16. Graceful shutdown (30 second timeout)
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer shutdownCancel()

    srv.Stop(shutdownCtx)
    if clust != nil {
        clust.Stop()
    }
    plugins.CloseAll()
    store.Close()

    log.Info().Msg("CacheStorm shutdown complete")
}
```
