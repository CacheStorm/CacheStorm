# CacheStorm — Architecture Specification

## 1. System Architecture Overview

```
                           ┌──────────────────────────────────────────┐
                           │            CacheStorm Node               │
                           │                                          │
    Redis Clients          │  ┌──────────────────────────────────┐   │
    (ioredis, go-redis,    │  │       TCP Server (:6380)          │   │
     redis-py, jedis,      │  │  ┌────────────────────────────┐  │   │
     redis-cli)        ────┼──│  │   Connection Handler        │  │   │
                           │  │  │   (goroutine per client)    │  │   │
                           │  │  └─────────┬──────────────────┘  │   │
                           │  └────────────┼─────────────────────┘   │
                           │               │                          │
                           │  ┌────────────▼─────────────────────┐   │
                           │  │       RESP3 Protocol Layer        │   │
                           │  │  ┌──────────┐  ┌──────────────┐  │   │
                           │  │  │  Reader   │  │   Writer     │  │   │
                           │  │  │  (parse)  │  │  (serialize) │  │   │
                           │  │  └──────────┘  └──────────────┘  │   │
                           │  └────────────┬─────────────────────┘   │
                           │               │                          │
                           │  ┌────────────▼─────────────────────┐   │
                           │  │       Command Router              │   │
                           │  │  dispatch table: map[string]Cmd   │   │
                           │  └────────────┬─────────────────────┘   │
                           │               │                          │
                           │  ┌────────────▼─────────────────────┐   │
                           │  │     Plugin Pipeline               │   │
                           │  │  BeforeCmd → Execute → AfterCmd   │   │
                           │  └────────────┬─────────────────────┘   │
                           │               │                          │
                           │  ┌────────────▼─────────────────────┐   │
                           │  │       Storage Engine              │   │
                           │  │                                    │   │
                           │  │  ┌──────────────────────────────┐ │   │
                           │  │  │     Namespace Manager         │ │   │
                           │  │  │  "default"  "sessions"  ...   │ │   │
                           │  │  └──────────┬───────────────────┘ │   │
                           │  │             │                      │   │
                           │  │  ┌──────────▼───────────────────┐ │   │
                           │  │  │     ShardMap (256 shards)     │ │   │
                           │  │  │  ┌─────┐┌─────┐     ┌─────┐ │ │   │
                           │  │  │  │ S0  ││ S1  │ ... │S255 │ │ │   │
                           │  │  │  │RWMtx││RWMtx│     │RWMtx│ │ │   │
                           │  │  │  └─────┘└─────┘     └─────┘ │ │   │
                           │  │  └──────────────────────────────┘ │   │
                           │  │                                    │   │
                           │  │  ┌──────────────┐ ┌────────────┐ │   │
                           │  │  │  Tag Index    │ │  TTL Wheel │ │   │
                           │  │  │  (sharded)    │ │  (4 level) │ │   │
                           │  │  └──────────────┘ └────────────┘ │   │
                           │  │                                    │   │
                           │  │  ┌──────────────┐ ┌────────────┐ │   │
                           │  │  │  Eviction     │ │  Memory    │ │   │
                           │  │  │  Controller   │ │  Tracker   │ │   │
                           │  │  └──────────────┘ └────────────┘ │   │
                           │  └──────────────────────────────────┘   │
                           │                                          │
                           │  ┌──────────────────────────────────┐   │
                           │  │       Cluster Layer               │   │
                           │  │  Gossip (:7946) │ Slot Router    │   │
                           │  │  Replication    │ Tag Broadcast  │   │
                           │  └──────────────────────────────────┘   │
                           │                                          │
                           │  ┌──────────────────────────────────┐   │
                           │  │       Admin HTTP API (:9090)      │   │
                           │  │  /health  /metrics  /info  /debug │   │
                           │  └──────────────────────────────────┘   │
                           └──────────────────────────────────────────┘
```

## 2. Data Structures — Detailed Specifications

### 2.1 Value Interface and Concrete Types

```go
package store

import "time"

// DataType identifies the type of value stored in an entry.
type DataType uint8

const (
    DataTypeString DataType = iota + 1
    DataTypeHash
    DataTypeList
    DataTypeSet
)

// String representation for INFO and TYPE commands
func (dt DataType) String() string {
    switch dt {
    case DataTypeString:
        return "string"
    case DataTypeHash:
        return "hash"
    case DataTypeList:
        return "list"
    case DataTypeSet:
        return "set"
    default:
        return "unknown"
    }
}

// Value is the interface all stored values must implement.
type Value interface {
    // Type returns the data type identifier
    Type() DataType
    // SizeOf returns the approximate memory usage in bytes
    SizeOf() int64
    // Clone creates a deep copy (used for snapshots and replication)
    Clone() Value
}

// StringValue holds a simple byte slice value.
type StringValue struct {
    Data []byte
}

func (v *StringValue) Type() DataType { return DataTypeString }
func (v *StringValue) SizeOf() int64  { return int64(len(v.Data)) + 24 } // 24 = slice header overhead
func (v *StringValue) Clone() Value {
    cloned := make([]byte, len(v.Data))
    copy(cloned, v.Data)
    return &StringValue{Data: cloned}
}

// HashValue holds a map of field→value pairs.
type HashValue struct {
    Fields map[string][]byte
}

func (v *HashValue) Type() DataType { return DataTypeHash }
func (v *HashValue) SizeOf() int64 {
    var size int64 = 48 // map header
    for k, val := range v.Fields {
        size += int64(len(k)) + int64(len(val)) + 80 // key + value + map entry overhead
    }
    return size
}
func (v *HashValue) Clone() Value {
    cloned := &HashValue{Fields: make(map[string][]byte, len(v.Fields))}
    for k, val := range v.Fields {
        cv := make([]byte, len(val))
        copy(cv, val)
        cloned.Fields[k] = cv
    }
    return cloned
}

// ListValue holds a doubly-linked list for O(1) push/pop at both ends.
// Internally uses a slice-based deque for memory efficiency.
type ListValue struct {
    Elements [][]byte
}

func (v *ListValue) Type() DataType { return DataTypeList }
func (v *ListValue) SizeOf() int64 {
    var size int64 = 24 // slice header
    for _, el := range v.Elements {
        size += int64(len(el)) + 24
    }
    return size
}
func (v *ListValue) Clone() Value {
    cloned := &ListValue{Elements: make([][]byte, len(v.Elements))}
    for i, el := range v.Elements {
        cel := make([]byte, len(el))
        copy(cel, el)
        cloned.Elements[i] = cel
    }
    return cloned
}

// SetValue holds a set of unique string members.
type SetValue struct {
    Members map[string]struct{}
}

func (v *SetValue) Type() DataType { return DataTypeSet }
func (v *SetValue) SizeOf() int64 {
    var size int64 = 48
    for k := range v.Members {
        size += int64(len(k)) + 48
    }
    return size
}
func (v *SetValue) Clone() Value {
    cloned := &SetValue{Members: make(map[string]struct{}, len(v.Members))}
    for k := range v.Members {
        cloned.Members[k] = struct{}{}
    }
    return cloned
}
```

### 2.2 Entry Structure

```go
// Entry represents a single cached item with metadata.
type Entry struct {
    Value       Value     // the stored value
    Tags        []string  // associated cache tags
    ExpiresAt   int64     // unix nanosecond timestamp, 0 = no expiration
    CreatedAt   int64     // unix nanosecond when created
    LastAccess  int64     // unix nanosecond of last access (for LRU)
    AccessCount uint64    // total access count (for LFU)
}

// IsExpired checks if the entry has passed its TTL.
func (e *Entry) IsExpired() bool {
    if e.ExpiresAt == 0 {
        return false
    }
    return time.Now().UnixNano() > e.ExpiresAt
}

// MemoryUsage returns total memory footprint of this entry in bytes.
func (e *Entry) MemoryUsage() int64 {
    var size int64 = 64 // Entry struct overhead (pointers, ints)
    size += e.Value.SizeOf()
    for _, tag := range e.Tags {
        size += int64(len(tag)) + 16
    }
    return size
}
```

### 2.3 Shard Structure

```go
const (
    NumShards    = 256              // must be power of 2
    ShardMask    = NumShards - 1    // for bitwise AND
)

// Shard is a single partition of the keyspace.
type Shard struct {
    mu      sync.RWMutex
    data    map[string]*Entry
    keyCount int64
    memUsage int64  // tracked incrementally, not recalculated
}

// ShardMap is the main key-value storage, distributed across shards.
type ShardMap struct {
    shards [NumShards]*Shard
}

// shardIndex determines which shard a key belongs to.
// Uses FNV-1a hash for speed and distribution.
func (sm *ShardMap) shardIndex(key string) uint32 {
    h := fnv32a(key)
    return h & ShardMask
}

// fnv32a is a fast, non-cryptographic hash function.
// Inline implementation — no external dependency.
func fnv32a(s string) uint32 {
    const (
        offset32 = uint32(2166136261)
        prime32  = uint32(16777619)
    )
    h := offset32
    for i := 0; i < len(s); i++ {
        h ^= uint32(s[i])
        h *= prime32
    }
    return h
}
```

### 2.4 Tag Index Structure

```go
const (
    TagShards    = 64
    TagShardMask = TagShards - 1
)

// TagIndex maintains a bidirectional mapping between tags and keys.
type TagIndex struct {
    // reverse index: tag → set of keys
    shards [TagShards]*tagShard
}

type tagShard struct {
    mu   sync.RWMutex
    // tag name → set of keys belonging to that tag
    index map[string]map[string]struct{}
}

// Forward index (key → tags) is stored directly in Entry.Tags

// Operations:
// AddTags(key, tags)       — register key under given tags in reverse index
// RemoveTags(key, tags)    — remove key from given tags in reverse index
// RemoveKey(key, tags)     — remove key from ALL its tags (called on DEL/eviction)
// GetKeys(tag) []string    — return all keys for a tag
// Invalidate(tag) []string — return all keys, then remove the tag entry
// Count(tag) int           — return count of keys in a tag
```

### 2.5 Timing Wheel for TTL

```go
// TimingWheel implements a hierarchical timing wheel for efficient TTL expiration.
// Instead of checking every key on every tick, keys are bucketed into time slots.
//
// Level 0: 1-second resolution, 3600 slots (1 hour)
// Level 1: 1-minute resolution, 1440 slots (24 hours)
// Level 2: 1-hour resolution, 720 slots (30 days)
// Level 3: 1-day resolution, 365 slots (1 year)
//
// A single goroutine ticks every second, processing expired slots.
// Keys with TTL > 1 year use a fallback "far future" bucket checked daily.

type TimingWheel struct {
    levels    [4]*wheelLevel
    farFuture *wheelBucket // for TTL > 1 year
    store     *ShardMap    // reference to delete expired keys
    tagIndex  *TagIndex    // reference to clean up tags
    stopCh    chan struct{}
}

type wheelLevel struct {
    mu        sync.Mutex
    slots     []*wheelBucket
    current   int
    tickSize  time.Duration
    numSlots  int
}

type wheelBucket struct {
    keys map[string]int64 // key → expiresAt (for validation)
}

// Add(key, expiresAt) — schedule key for expiration
// Remove(key)         — cancel scheduled expiration (when TTL is removed or key deleted)
// Start()             — begin the ticker goroutine
// Stop()              — graceful shutdown of ticker
```

### 2.6 Eviction Controller

```go
type EvictionPolicy uint8

const (
    EvictionNoEviction   EvictionPolicy = iota // reject writes when memory full
    EvictionAllKeysLRU                          // evict least recently used
    EvictionAllKeysLFU                          // evict least frequently used
    EvictionVolatileLRU                         // evict LRU among keys with TTL
    EvictionAllKeysRandom                       // evict random keys
)

// EvictionController manages memory pressure and eviction.
type EvictionController struct {
    policy     EvictionPolicy
    maxMemory  int64          // max allowed memory in bytes
    store      *ShardMap
    tagIndex   *TagIndex
    onEvict    func(key string, entry *Entry) // plugin hook callback
}

// Memory pressure levels:
// 0-70%   = Normal      → no action
// 70-85%  = Warning     → start evicting (batch of 10 keys per check)
// 85-95%  = Critical    → aggressive eviction (batch of 100)
// 95%+    = Emergency   → reject all writes, evict until below 85%

// CheckAndEvict() — called after every write operation
// ForceEvict(n)   — evict n keys immediately
// CurrentUsage() (used, max int64, percent float64)
```

### 2.7 Namespace Manager

```go
// Namespace wraps a ShardMap + TagIndex + TimingWheel + EvictionController
// Each namespace is an independent keyspace.
type Namespace struct {
    Name       string
    Store      *ShardMap
    Tags       *TagIndex
    TTLWheel   *TimingWheel
    Eviction   *EvictionController
    DefaultTTL time.Duration // 0 = no default TTL
    MaxMemory  int64         // 0 = uses global limit
    CreatedAt  time.Time
}

// NamespaceManager manages all namespaces.
type NamespaceManager struct {
    mu         sync.RWMutex
    namespaces map[string]*Namespace
    defaultNS  *Namespace // quick reference to "default"
    globalMem  *MemoryTracker
}

// Get(name) *Namespace          — get or nil
// GetOrCreate(name) *Namespace  — get or create with defaults
// Delete(name) error            — flush and remove namespace
// List() []string               — list all namespace names
// Default() *Namespace          — return the "default" namespace
```

## 3. Connection Lifecycle

```
Client connects to :6380 (TCP)
    │
    ├── Server accepts connection
    ├── Create Connection object (bufio.Reader + bufio.Writer)
    ├── Spawn goroutine for this connection
    │
    ▼
Connection Read Loop:
    │
    ├── Read RESP3 message from wire
    ├── Parse into []RESPValue (command + args)
    ├── Check if AUTH required (if auth plugin enabled, client must AUTH first)
    │
    ├── Resolve namespace (from connection state, default = "default")
    │
    ├── Create CommandContext {
    │       Conn: *Connection
    │       Args: [][]byte
    │       Namespace: *Namespace
    │       StartTime: time.Now()
    │   }
    │
    ├── Plugin Pipeline: BeforeCommand hooks
    │   └── If any hook returns error → send error to client, skip execution
    │
    ├── Command Router: lookup command handler
    │   └── If unknown command → send ERR unknown command
    │
    ├── Execute command handler
    │   └── Handler interacts with Namespace.Store / Namespace.Tags / etc.
    │
    ├── Plugin Pipeline: AfterCommand hooks
    │   └── Stats tracking, slow log, etc.
    │
    ├── Write RESP3 response to client
    │
    └── Loop back to read next command
    
Client disconnects or error:
    ├── Close connection
    ├── Remove from connection pool
    └── Goroutine exits
```

## 4. Memory Management Strategy

### 4.1 Tracking

Memory is tracked incrementally, NOT by periodic scanning:

- On SET: `shard.memUsage += entry.MemoryUsage()`
- On DEL: `shard.memUsage -= entry.MemoryUsage()`
- On UPDATE: `shard.memUsage += (newEntry.MemoryUsage() - oldEntry.MemoryUsage())`

Global usage = sum of all shard.memUsage across all namespaces.

### 4.2 Memory Tracker

```go
type MemoryTracker struct {
    maxMemory    int64
    currentUsage atomic.Int64
    
    // Pressure thresholds
    warningPct   float64 // default 0.70
    criticalPct  float64 // default 0.85
    emergencyPct float64 // default 0.95
}

func (mt *MemoryTracker) Add(bytes int64)      { mt.currentUsage.Add(bytes) }
func (mt *MemoryTracker) Sub(bytes int64)       { mt.currentUsage.Add(-bytes) }
func (mt *MemoryTracker) Usage() int64          { return mt.currentUsage.Load() }
func (mt *MemoryTracker) Pressure() PressureLevel { ... }
func (mt *MemoryTracker) CanAllocate(bytes int64) bool { ... }
```

## 5. Graceful Shutdown Sequence

```
SIGINT / SIGTERM received
    │
    ├── 1. Stop accepting new connections
    ├── 2. Set server state to "shutting_down"
    ├── 3. Stop timing wheel ticker
    ├── 4. Stop cluster gossip
    ├── 5. Trigger OnShutdown hooks for all plugins
    │       ├── Persistence plugin: flush AOF buffer, write final snapshot
    │       ├── Metrics plugin: final metrics push
    │       └── Stats plugin: log final stats
    ├── 6. Wait for in-flight commands to complete (with 30s timeout)
    ├── 7. Close all client connections
    ├── 8. Close TCP listener
    ├── 9. Log "CacheStorm shutdown complete"
    └── 10. Exit(0)
```

## 6. Error Types

```go
package store

import "errors"

var (
    // Key errors
    ErrKeyNotFound     = errors.New("key not found")
    ErrKeyExists       = errors.New("key already exists")
    ErrKeyExpired      = errors.New("key expired")
    
    // Type errors
    ErrWrongType       = errors.New("WRONGTYPE Operation against a key holding the wrong kind of value")
    ErrInvalidDataType = errors.New("invalid data type")
    
    // Memory errors
    ErrMemoryLimit     = errors.New("OOM command not allowed when used memory > 'maxmemory'")
    ErrEvictionFailed  = errors.New("failed to evict enough keys")
    
    // Namespace errors
    ErrNamespaceNotFound = errors.New("namespace not found")
    ErrNamespaceExists   = errors.New("namespace already exists")
    
    // Tag errors
    ErrTagNotFound     = errors.New("tag not found")
    ErrInvalidTagName  = errors.New("invalid tag name")
    
    // Auth errors
    ErrAuthRequired    = errors.New("NOAUTH Authentication required")
    ErrAuthFailed      = errors.New("ERR invalid password")
    
    // Command errors
    ErrUnknownCommand  = errors.New("ERR unknown command")
    ErrWrongArgCount   = errors.New("ERR wrong number of arguments")
    ErrInvalidArg      = errors.New("ERR invalid argument")
    ErrSyntaxError     = errors.New("ERR syntax error")
    ErrNotInteger      = errors.New("ERR value is not an integer or out of range")
    ErrIndexOutOfRange = errors.New("ERR index out of range")
    
    // Cluster errors
    ErrClusterDown     = errors.New("CLUSTERDOWN The cluster is down")
    ErrMoved           = errors.New("MOVED")
    ErrAsk             = errors.New("ASK")
    
    // Server errors
    ErrServerShutdown  = errors.New("ERR server is shutting down")
)
```

## 7. Configuration Structure

```go
type Config struct {
    Server    ServerConfig    `yaml:"server"`
    Memory    MemoryConfig    `yaml:"memory"`
    Namespaces map[string]NamespaceConfig `yaml:"namespaces"`
    Cluster   ClusterConfig   `yaml:"cluster"`
    Persistence PersistenceConfig `yaml:"persistence"`
    Plugins   PluginsConfig   `yaml:"plugins"`
    Logging   LoggingConfig   `yaml:"logging"`
}

type ServerConfig struct {
    Bind           string `yaml:"bind" default:"0.0.0.0"`
    Port           int    `yaml:"port" default:"6380"`
    MaxConnections int    `yaml:"max_connections" default:"10000"`
    TCPKeepAlive   int    `yaml:"tcp_keepalive" default:"300"` // seconds
    ReadTimeout    string `yaml:"read_timeout" default:"0"`     // 0 = no timeout
    WriteTimeout   string `yaml:"write_timeout" default:"0"`
    ReadBufferSize int    `yaml:"read_buffer_size" default:"4096"`
    WriteBufferSize int   `yaml:"write_buffer_size" default:"4096"`
}

type MemoryConfig struct {
    MaxMemory      string `yaml:"max_memory" default:"0"`      // 0 = no limit, "2gb", "512mb"
    EvictionPolicy string `yaml:"eviction_policy" default:"allkeys-lru"`
    WarningPct     int    `yaml:"pressure_warning" default:"70"`
    CriticalPct    int    `yaml:"pressure_critical" default:"85"`
    SampleSize     int    `yaml:"eviction_sample_size" default:"5"` // keys to sample for LRU/LFU
}

type NamespaceConfig struct {
    DefaultTTL string `yaml:"default_ttl" default:"0"` // "1h", "30m", "0" = none
    MaxMemory  string `yaml:"max_memory" default:"0"`
}

type ClusterConfig struct {
    Enabled     bool     `yaml:"enabled" default:"false"`
    NodeName    string   `yaml:"node_name"`
    BindAddr    string   `yaml:"bind_addr" default:"0.0.0.0"`
    BindPort    int      `yaml:"bind_port" default:"7946"`
    AdvertiseAddr string `yaml:"advertise_addr"` // for NAT/Docker
    AdvertisePort int    `yaml:"advertise_port"`
    Seeds       []string `yaml:"seeds"`           // initial nodes
    Replicas    int      `yaml:"replicas" default:"1"`
}

type PersistenceConfig struct {
    Enabled          bool   `yaml:"enabled" default:"false"`
    AOF              bool   `yaml:"aof" default:"true"`
    AOFSync          string `yaml:"aof_sync" default:"everysec"` // always, everysec, no
    SnapshotInterval string `yaml:"snapshot_interval" default:"5m"`
    DataDir          string `yaml:"data_dir" default:"/var/lib/cachestorm"`
    MaxAOFSize       string `yaml:"max_aof_size" default:"1gb"` // triggers rewrite
}

type PluginsConfig struct {
    Stats     StatsPluginConfig     `yaml:"stats"`
    Metrics   MetricsPluginConfig   `yaml:"metrics"`
    Auth      AuthPluginConfig      `yaml:"auth"`
    SlowLog   SlowLogPluginConfig   `yaml:"slowlog"`
}

type StatsPluginConfig struct {
    Enabled bool `yaml:"enabled" default:"true"`
}

type MetricsPluginConfig struct {
    Enabled bool   `yaml:"enabled" default:"true"`
    Port    int    `yaml:"port" default:"9090"`
    Path    string `yaml:"path" default:"/metrics"`
}

type AuthPluginConfig struct {
    Enabled  bool   `yaml:"enabled" default:"false"`
    Password string `yaml:"password"`
}

type SlowLogPluginConfig struct {
    Enabled    bool   `yaml:"enabled" default:"true"`
    Threshold  string `yaml:"threshold" default:"10ms"`
    MaxEntries int    `yaml:"max_entries" default:"1000"`
}

type LoggingConfig struct {
    Level  string `yaml:"level" default:"info"`   // debug, info, warn, error
    Format string `yaml:"format" default:"json"`  // json, console
    Output string `yaml:"output" default:"stdout"` // stdout, stderr, filepath
}
```

## 8. Thread Safety Rules

1. **ShardMap shards**: Each shard has its own `sync.RWMutex`. Read operations use `RLock`, write operations use `Lock`. NEVER hold locks across shards simultaneously — this prevents deadlocks.

2. **TagIndex shards**: Same pattern as ShardMap. Tag shard selected by hash of tag name.

3. **TimingWheel levels**: Each level has its own `sync.Mutex`. Only the ticker goroutine modifies slots.

4. **NamespaceManager**: `sync.RWMutex` for the namespace map. Individual namespace operations don't need the manager lock.

5. **Connection**: Each connection is owned by a single goroutine. No concurrent access.

6. **Plugin hooks**: Plugins must be thread-safe internally. The plugin pipeline calls hooks sequentially (not concurrently) for a single command, but different commands may call hooks concurrently.

7. **MemoryTracker**: Uses `atomic.Int64` — no locks needed.

8. **Config**: Read-only after startup. No locks needed.
