# CacheStorm — Plugin System Specification

## 1. Plugin Architecture

The plugin system is designed around Go interface composition. A plugin implements the base `Plugin` interface and optionally implements additional hook interfaces for the behaviors it needs.

### 1.1 Base Interface

```go
package plugin

// Plugin is the base interface every plugin must implement.
type Plugin interface {
    // Name returns a unique identifier for this plugin (e.g., "stats", "auth").
    Name() string
    // Version returns semver string (e.g., "1.0.0").
    Version() string
    // Init is called once during server startup with plugin-specific config.
    Init(cfg map[string]interface{}) error
    // Close is called during server shutdown. Cleanup resources here.
    Close() error
}
```

### 1.2 Hook Interfaces

```go
// BeforeCommandHook runs before a command is executed.
// Returning an error cancels the command and sends the error to the client.
type BeforeCommandHook interface {
    BeforeCommand(ctx *HookContext) error
}

// AfterCommandHook runs after a command completes (regardless of success/failure).
type AfterCommandHook interface {
    AfterCommand(ctx *HookContext)
}

// OnEvictHook is called when a key is evicted due to memory pressure.
type OnEvictHook interface {
    OnEvict(namespace string, key string, entry *store.Entry)
}

// OnExpireHook is called when a key expires via TTL.
type OnExpireHook interface {
    OnExpire(namespace string, key string, entry *store.Entry)
}

// OnTagInvalidateHook is called when a tag is invalidated.
type OnTagInvalidateHook interface {
    OnTagInvalidate(namespace string, tag string, keysDeleted []string)
}

// OnStartupHook is called after server starts and store is ready.
type OnStartupHook interface {
    OnStartup(s ServerInfo) error
}

// OnShutdownHook is called before server stops. Cleanup persistence etc.
type OnShutdownHook interface {
    OnShutdown() error
}

// CustomCommandProvider lets a plugin register its own RESP commands.
type CustomCommandProvider interface {
    Commands() []CustomCommand
}

// HTTPEndpointProvider lets a plugin register HTTP endpoints on the admin API.
type HTTPEndpointProvider interface {
    Routes() []HTTPRoute
}
```

### 1.3 Hook Context

```go
// HookContext provides command information to hooks.
type HookContext struct {
    // Command name (uppercase): "SET", "GET", etc.
    Command string
    // Raw arguments (NOT including command name)
    Args [][]byte
    // Current namespace name
    Namespace string
    // Connection ID
    ConnectionID int64
    // Client address
    ClientAddr string
    // Command start time (set before BeforeCommand hooks)
    StartTime time.Time
    // Duration (set before AfterCommand hooks)
    Duration time.Duration
    // Error from command execution (nil if success, set before AfterCommand hooks)
    Err error
    // Whether command modified data
    IsWrite bool
}
```

### 1.4 Custom Command Registration

```go
type CustomCommand struct {
    Name     string
    Handler  func(ctx *CommandContext) error
    MinArgs  int
    MaxArgs  int
    ReadOnly bool
}

type HTTPRoute struct {
    Method  string // "GET", "POST", etc.
    Path    string // "/my-plugin/endpoint"
    Handler http.HandlerFunc
}

type ServerInfo struct {
    Version     string
    StartTime   time.Time
    Config      *config.Config
    Store       StoreAccessor // limited interface for plugins
}

// StoreAccessor provides controlled access to the store for plugins.
type StoreAccessor interface {
    Get(namespace, key string) (*store.Entry, error)
    Set(namespace, key string, value store.Value, ttl time.Duration, tags []string) error
    Del(namespace, key string) (bool, error)
    Keys(namespace, pattern string) ([]string, error)
    DBSize(namespace string) int64
    NamespaceNames() []string
}
```

## 2. Plugin Manager

```go
type Manager struct {
    mu sync.RWMutex

    // All registered plugins in registration order
    plugins []Plugin

    // Categorized hook lists (populated during Register via type assertion)
    beforeHooks    []BeforeCommandHook
    afterHooks     []AfterCommandHook
    evictHooks     []OnEvictHook
    expireHooks    []OnExpireHook
    tagHooks       []OnTagInvalidateHook
    startupHooks   []OnStartupHook
    shutdownHooks  []OnShutdownHook
    cmdProviders   []CustomCommandProvider
    httpProviders  []HTTPEndpointProvider
}

func NewManager() *Manager {
    return &Manager{}
}

// Register adds a plugin and categorizes its hooks.
func (m *Manager) Register(p Plugin) {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.plugins = append(m.plugins, p)

    // Type-assert for each hook interface
    if h, ok := p.(BeforeCommandHook); ok {
        m.beforeHooks = append(m.beforeHooks, h)
    }
    if h, ok := p.(AfterCommandHook); ok {
        m.afterHooks = append(m.afterHooks, h)
    }
    if h, ok := p.(OnEvictHook); ok {
        m.evictHooks = append(m.evictHooks, h)
    }
    // ... same for all hook types
}

// InitAll initializes all plugins in registration order.
func (m *Manager) InitAll(configs map[string]map[string]interface{}) error {
    for _, p := range m.plugins {
        cfg := configs[p.Name()] // may be nil
        if err := p.Init(cfg); err != nil {
            return fmt.Errorf("plugin %s init failed: %w", p.Name(), err)
        }
    }
    return nil
}

// CloseAll closes all plugins in reverse registration order.
func (m *Manager) CloseAll() error {
    var errs []error
    for i := len(m.plugins) - 1; i >= 0; i-- {
        if err := m.plugins[i].Close(); err != nil {
            errs = append(errs, fmt.Errorf("plugin %s close: %w", m.plugins[i].Name(), err))
        }
    }
    if len(errs) > 0 {
        return errors.Join(errs...)
    }
    return nil
}

// RunBeforeHooks executes all BeforeCommand hooks.
// Returns first error (short-circuits).
func (m *Manager) RunBeforeHooks(ctx *HookContext) error {
    for _, h := range m.beforeHooks {
        if err := h.BeforeCommand(ctx); err != nil {
            return err
        }
    }
    return nil
}

// RunAfterHooks executes all AfterCommand hooks (never short-circuits).
func (m *Manager) RunAfterHooks(ctx *HookContext) {
    for _, h := range m.afterHooks {
        h.AfterCommand(ctx)
    }
}

// Similarly for RunEvictHooks, RunExpireHooks, RunTagInvalidateHooks, etc.
```

## 3. Built-in Plugins — Detailed Specifications

### 3.1 Stats Plugin

**Package:** `plugins/stats`

**Purpose:** Track server-wide statistics: command counts, hit/miss ratios, latency percentiles.

**Implementation:**
```go
type StatsPlugin struct {
    // Command counters
    totalCommands  atomic.Int64
    commandCounts  sync.Map // command name → *atomic.Int64

    // Cache hit/miss
    hits   atomic.Int64
    misses atomic.Int64

    // Keys lifecycle
    keysCreated  atomic.Int64
    keysDeleted  atomic.Int64
    keysExpired  atomic.Int64
    keysEvicted  atomic.Int64

    // Latency tracking (simple approach: track last N latencies per command)
    latencies sync.Map // command name → *LatencyTracker
}

type LatencyTracker struct {
    mu      sync.Mutex
    samples []time.Duration // circular buffer, size 1000
    idx     int
    count   int64
    total   time.Duration
}

// Implements: BeforeCommandHook, AfterCommandHook, OnEvictHook, OnExpireHook
// BeforeCommand: record start time (already in HookContext)
// AfterCommand: increment counters, record latency, track hit/miss for GET commands
// OnEvict: increment keysEvicted
// OnExpire: increment keysExpired
```

**Exposed Data (via INFO stats):**
```
# Stats
total_commands_processed:1234567
commands_per_second:1234
hit_count:100000
miss_count:5000
hit_ratio:0.9524
keys_created:50000
keys_deleted:10000
keys_expired:5000
keys_evicted:2000
cmd_set_count:30000
cmd_set_avg_latency_us:45
cmd_set_p99_latency_us:120
cmd_get_count:70000
cmd_get_avg_latency_us:20
cmd_get_p99_latency_us:80
```

### 3.2 Auth Plugin

**Package:** `plugins/auth`

**Purpose:** Password-based authentication. Block commands until AUTH succeeds.

**Implementation:**
```go
type AuthPlugin struct {
    password       string
    authenticated  sync.Map // connectionID → bool
}

// Implements: BeforeCommandHook

// BeforeCommand:
//   if command is AUTH → check password, set authenticated flag, return nil
//   if command is PING, QUIT → allow without auth
//   if not authenticated → return ErrAuthRequired
//   else → allow
```

### 3.3 SlowLog Plugin

**Package:** `plugins/slowlog`

**Purpose:** Log commands that exceed a latency threshold.

**Implementation:**
```go
type SlowLogPlugin struct {
    threshold  time.Duration
    maxEntries int

    mu      sync.Mutex
    entries []SlowLogEntry // ring buffer
    idx     int
    seq     int64
}

type SlowLogEntry struct {
    ID        int64
    Timestamp time.Time
    Duration  time.Duration
    Command   string
    Args      []string // first 5 args only (truncated for safety)
}

// Implements: AfterCommandHook, CustomCommandProvider

// AfterCommand: if duration > threshold → add entry
// Custom commands: SLOWLOG GET [count], SLOWLOG RESET, SLOWLOG LEN
```

### 3.4 Metrics Plugin

**Package:** `plugins/metrics`

**Purpose:** Expose Prometheus metrics via HTTP endpoint.

**Implementation:**
```go
type MetricsPlugin struct {
    httpServer *http.Server
    port       int
    path       string

    // Prometheus metrics
    commandsTotal    *prometheus.CounterVec   // labels: command
    commandDuration  *prometheus.HistogramVec  // labels: command
    keysTotal        *prometheus.GaugeVec      // labels: namespace
    memoryBytes      *prometheus.GaugeVec      // labels: namespace
    hitsTotal        prometheus.Counter
    missesTotal      prometheus.Counter
    evictedTotal     prometheus.Counter
    expiredTotal     prometheus.Counter
    connectedClients prometheus.Gauge
    tagInvalidations prometheus.Counter

    store StoreAccessor // for periodic gauge updates
}

// Implements: AfterCommandHook, OnEvictHook, OnExpireHook, OnTagInvalidateHook,
//             OnStartupHook, OnShutdownHook, HTTPEndpointProvider

// OnStartup: start HTTP server, register Prometheus collectors
// AfterCommand: increment commandsTotal, observe commandDuration
// OnEvict: increment evictedTotal
// OnExpire: increment expiredTotal
// OnTagInvalidate: increment tagInvalidations
// OnShutdown: stop HTTP server

// Background goroutine: every 10 seconds, update gauge metrics (keys count, memory)
```

### 3.5 Persistence Plugin

**Package:** `plugins/persistence`

Detailed in 03-IMPLEMENTATION-PHASES.md Phase 7. Key interfaces:

```go
// Implements: AfterCommandHook (AOF write), OnStartupHook (recovery),
//             OnShutdownHook (final flush + snapshot)

type PersistencePlugin struct {
    config   PersistenceConfig
    aofFile  *os.File
    aofMu    sync.Mutex
    aofBuf   *bufio.Writer

    snapshotTicker *time.Ticker
    store          StoreAccessor

    stopCh chan struct{}
}
```

## 4. Plugin Configuration Mapping

From YAML config to plugin Init():

```yaml
plugins:
  stats:
    enabled: true
  auth:
    enabled: true
    password: "mysecret"
  slowlog:
    enabled: true
    threshold: "10ms"
    max_entries: 1000
  metrics:
    enabled: true
    port: 9090
    path: "/metrics"
```

Mapping logic in server startup:
```go
func loadPlugins(cfg *config.Config, mgr *plugin.Manager) {
    if cfg.Plugins.Stats.Enabled {
        mgr.Register(stats.New())
    }
    if cfg.Plugins.Auth.Enabled {
        mgr.Register(auth.New(cfg.Plugins.Auth.Password))
    }
    if cfg.Plugins.SlowLog.Enabled {
        mgr.Register(slowlog.New(cfg.Plugins.SlowLog.Threshold, cfg.Plugins.SlowLog.MaxEntries))
    }
    if cfg.Plugins.Metrics.Enabled {
        mgr.Register(metrics.New(cfg.Plugins.Metrics.Port, cfg.Plugins.Metrics.Path))
    }
    if cfg.Persistence.Enabled {
        mgr.Register(persistence.New(cfg.Persistence))
    }
}
```
