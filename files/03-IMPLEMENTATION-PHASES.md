# CacheStorm — Implementation Phases

All phases must be completed to reach v1.0.0. Each phase builds on the previous one.
DO NOT skip any phase. DO NOT start a phase without completing and testing the previous one.

---

## Phase 1: Foundation — RESP3 Protocol + TCP Server (MUST COMPLETE FIRST)

### Goal
A bare TCP server that speaks RESP3 and responds to PING, SET, GET, DEL.

### Steps

#### 1.1 Initialize Go Module
```bash
mkdir cachestorm && cd cachestorm
go mod init github.com/cachestorm/cachestorm
```
Create .gitignore, LICENSE (Apache 2.0), README.md stub.

#### 1.2 Implement RESP3 Reader (`internal/resp/reader.go`)
- `type Reader struct` with `*bufio.Reader`
- `func NewReader(rd io.Reader) *Reader`
- `func (r *Reader) ReadValue() (Value, error)` — the main parser
  - Read first byte to determine type
  - For `+` (SimpleString): read until \r\n
  - For `-` (Error): read until \r\n
  - For `:` (Integer): read until \r\n, parse int64
  - For `$` (BulkString): read length, then read N bytes + \r\n. Handle $-1 as null.
  - For `*` (Array): read count, then recursively read N values. Handle *-1 as null.
- `func (r *Reader) ReadCommand() (string, [][]byte, error)` — convenience wrapper
- Edge cases: empty bulk string ($0\r\n\r\n), max bulk string size (512MB like Redis)

#### 1.3 Implement RESP3 Writer (`internal/resp/writer.go`)
- `type Writer struct` with `*bufio.Writer`
- `func NewWriter(wr io.Writer) *Writer`
- Methods: WriteSimpleString, WriteError, WriteInteger, WriteBulkString, WriteBulkStringBytes, WriteNull, WriteNullArray, WriteArray (start), Flush
- All methods must handle \r\n properly
- WriteBulkString: `$len\r\ndata\r\n`

#### 1.4 RESP Types (`internal/resp/types.go`)
- Define Type constants
- Define Value struct
- Helper constructors: `SimpleString(s)`, `ErrorValue(s)`, `IntegerValue(n)`, `BulkBytes(b)`, `NullValue()`

#### 1.5 Write RESP Tests (`internal/resp/reader_test.go`, `writer_test.go`)
- Test every type parsing
- Test edge cases: null, empty string, large bulk string, nested arrays
- Test malformed input (missing \r\n, invalid length, etc.)
- Test round-trip: write → read → compare
- Benchmark: `BenchmarkReadBulkString`, `BenchmarkReadArray`

#### 1.6 TCP Server (`internal/server/server.go`)
```go
type Server struct {
    config    *config.Config
    listener  net.Listener
    router    *command.Router
    store     *store.Store
    plugins   *plugin.Manager
    conns     sync.Map // trackconnected clients
    connID    atomic.Int64
    stopping  atomic.Bool
    stopCh    chan struct{}
    wg        sync.WaitGroup
}

func New(cfg *config.Config) *Server
func (s *Server) Start(ctx context.Context) error   // bind + accept loop
func (s *Server) Stop(ctx context.Context) error     // graceful shutdown
func (s *Server) handleConnection(conn net.Conn)     // per-client goroutine
```

- Accept loop in goroutine
- Each connection gets its own goroutine with `handleConnection`
- Track all connections for graceful shutdown
- Respect context cancellation

#### 1.7 Connection Handler (`internal/server/connection.go`)
```go
type Connection struct {
    ID        int64
    conn      net.Conn
    reader    *resp.Reader
    writer    *resp.Writer
    namespace string    // current namespace, default "default"
    clientName string
    authenticated bool
    createdAt time.Time
}
```
- Read loop: ReadCommand → Router.Execute → write response → repeat
- Handle io.EOF (client disconnect) gracefully
- Handle QUIT command inline

#### 1.8 Basic Config (`internal/config/config.go`)
- Just ServerConfig for now: bind, port, max_connections
- Load from YAML file if exists, else use defaults
- Parse command-line flags: `--config`, `--port`, `--bind`

#### 1.9 Logger Setup (`internal/logger/logger.go`)
- Initialize zerolog with config (level, format, output)
- Provide package-level `log` variable
- Structured fields: component, connection_id, command

#### 1.10 Store — Minimal (`internal/store/`)
- `store.go`: Store interface with Get, Set, Del, Exists methods
- `entry.go`: Entry struct with StringValue only for now
- `shard.go`: ShardMap with 256 shards, fnv32a hash, RWMutex per shard
- Implement: Get(key), Set(key, value, ttl), Del(key), Exists(key)
- Lazy expiry check on Get (if expired, delete and return nil)

#### 1.11 Command Router (`internal/command/router.go`)
```go
type Router struct {
    commands map[string]*CommandDef
}

func NewRouter() *Router
func (r *Router) Register(def *CommandDef)
func (r *Router) Execute(ctx *CommandContext) error
```

#### 1.12 Initial Commands (`internal/command/string_commands.go`, `server_commands.go`)
- PING, ECHO, QUIT
- SET (basic: key value, with EX, PX, NX, XX)
- GET
- DEL
- EXISTS

#### 1.13 Main Entry Point (`cmd/cachestorm/main.go`)
- Parse flags
- Load config
- Initialize logger
- Create store
- Create router, register commands
- Create server
- Start server
- Handle SIGINT/SIGTERM for graceful shutdown

#### 1.14 Integration Test
- Start server programmatically
- Connect with `net.Dial` and send raw RESP
- Test PING → PONG
- Test SET/GET round-trip
- Test DEL
- Verify with `redis-cli -p 6380` (manual test)

### Phase 1 Deliverables
- [ ] `redis-cli -p 6380 PING` returns PONG
- [ ] `redis-cli -p 6380 SET foo bar` + `GET foo` works
- [ ] `redis-cli -p 6380 DEL foo` works
- [ ] All unit tests pass
- [ ] RESP benchmarks exist

---

## Phase 2: Complete String Operations + TTL + Memory

### Goal
All string commands, proper TTL with timing wheel, memory tracking and eviction.

### Steps

#### 2.1 Complete String Commands
Implement remaining string commands (see 02-PROTOCOL-SPEC.md section 2.2):
- MSET, MGET
- INCR, DECR, INCRBY, DECRBY, INCRBYFLOAT
- APPEND, STRLEN
- GETRANGE, SETRANGE
- SETNX, SETEX, PSETEX
- GETSET, GETDEL

Each command needs:
- Handler function
- Registration in router
- Unit test
- Edge case handling (wrong type, missing key, overflow)

#### 2.2 Key Commands
- EXPIRE, PEXPIRE, EXPIREAT, PEXPIREAT
- TTL, PTTL
- PERSIST
- TYPE
- RENAME
- KEYS (with glob pattern matching — implement simple glob: *, ?, [abc])
- SCAN (cursor-based iteration)
- RANDOMKEY
- UNLINK (same as DEL in our implementation)

#### 2.3 Timing Wheel (`internal/store/timing_wheel.go`)
- Implement hierarchical timing wheel as specified in 01-ARCHITECTURE.md
- Level 0: 1s resolution, 3600 slots
- Level 1: 1m resolution, 1440 slots
- Level 2: 1h resolution, 720 slots
- Level 3: 1d resolution, 365 slots
- Ticker goroutine: every 100ms, check current slot, expire keys
- On SET with TTL → add to wheel
- On DEL → remove from wheel
- On EXPIRE → reschedule in wheel
- Test: set key with 1s TTL, wait 1.5s, verify expired

#### 2.4 Memory Tracking (`internal/store/memory.go`)
- MemoryTracker with atomic int64
- Track on every Set (add), Del (subtract), Update (diff)
- Entry.MemoryUsage() calculation

#### 2.5 Eviction Controller (`internal/store/eviction.go`)
- Implement LRU eviction:
  - Sample N random keys from random shards
  - Pick the one with oldest LastAccess
  - Delete it
  - Repeat until under memory pressure
- Update Entry.LastAccess on every Get
- Implement policies: NoEviction, AllKeysLRU, VolatileLRU, AllKeysRandom
- CheckAndEvict() called after every write
- Memory pressure levels as specified in architecture

#### 2.6 MemoryConfig
- Parse max_memory string ("2gb", "512mb", "0") into int64 bytes
- Validate eviction_policy string to enum

#### 2.7 INFO Command
- Implement INFO with sections: server, memory, stats, keyspace
- Format: `# Section\r\nkey:value\r\n`

#### 2.8 Tests
- Unit tests for every command
- Timing wheel: test add, remove, expire, reschedule
- Eviction: test LRU ordering, memory pressure levels
- Integration: SET with EX, verify TTL countdown, verify expiration

### Phase 2 Deliverables
- [ ] All string commands work via redis-cli
- [ ] TTL works: SET with EX → TTL decreases → key expires automatically
- [ ] Memory limit works: set max_memory, fill it, verify eviction occurs
- [ ] KEYS pattern matching works
- [ ] SCAN cursor iteration works
- [ ] INFO returns meaningful data

---

## Phase 3: Data Structures — Hash, List, Set

### Goal
Full Hash, List, and Set command support.

### Steps

#### 3.1 Value Types (`internal/store/entry.go`)
- Implement HashValue, ListValue, SetValue as specified in architecture
- Ensure SizeOf() is accurate for each
- Ensure Clone() creates deep copies

#### 3.2 Type Checking
- Before executing Hash/List/Set commands, verify key type matches
- Return WRONGTYPE error if mismatch
- No type conflict for new keys

#### 3.3 Hash Commands (`internal/command/hash_commands.go`)
Implement all hash commands from 02-PROTOCOL-SPEC.md section 2.4.
- Auto-create HashValue on first HSET
- Auto-delete key when all fields removed via HDEL
- Memory tracking: update on field add/remove

#### 3.4 List Commands (`internal/command/list_commands.go`)
Implement all list commands from 02-PROTOCOL-SPEC.md section 2.5.
- Use slice-based deque (for simplicity in Phase 1, optimize later if needed)
- LPUSH prepends (for slice: use append + copy, or maintain as reverse internally)
  - Better approach: use a deque with head/tail pointers in a circular buffer
  - Simplest: just use Go slice with append for RPUSH and prepend for LPUSH
- Handle negative indices in LRANGE, LINDEX
- Auto-delete key when list becomes empty

#### 3.5 Set Commands (`internal/command/set_commands.go`)
Implement all set commands from 02-PROTOCOL-SPEC.md section 2.6.
- SUNION, SINTER, SDIFF: work across multiple keys
- SRANDMEMBER: use math/rand for random selection from map (iterate + skip)
- Auto-delete key when set becomes empty

#### 3.6 Type-Aware DEL and EXPIRE
- DEL should work on all types
- EXPIRE/TTL should work on all types
- TYPE command returns correct type string

#### 3.7 Tests
- Full test suite for each data type
- Test type conflicts (SET string, then HSET on same key → WRONGTYPE)
- Test auto-deletion of empty collections
- Test memory tracking accuracy with different types

### Phase 3 Deliverables
- [ ] All Hash commands work via redis-cli: HSET/HGET/HGETALL/etc.
- [ ] All List commands work: LPUSH/RPUSH/LPOP/RPOP/LRANGE/etc.
- [ ] All Set commands work: SADD/SMEMBERS/SUNION/SINTER/etc.
- [ ] WRONGTYPE errors on type mismatch
- [ ] Empty collections auto-delete

---

## Phase 4: Tag System — The Killer Feature

### Goal
Full tag-based invalidation with bidirectional index, cascade, and tag hierarchy.

### Steps

#### 4.1 Tag Index (`internal/store/tag_index.go`)
- Implement sharded reverse index as specified in architecture
- Methods: AddTags, RemoveTags, RemoveKey, GetKeys, Invalidate, Count
- Thread-safe with per-shard RWMutex

#### 4.2 Forward Index
- Tags stored directly in Entry.Tags
- On SET: if tags provided, register in tag index
- On DEL: clean up all tag associations
- On EXPIRE/Eviction: clean up all tag associations

#### 4.3 Tag Commands (`internal/command/tag_commands.go`)
Implement all tag commands from 02-PROTOCOL-SPEC.md section 2.7:
- SETTAG: atomic SET + tag registration within same shard lock
- TAGS: lookup entry, return Tags field
- ADDTAG: add tags to existing entry + update reverse index
- REMTAG: remove tags from entry + update reverse index
- INVALIDATE: the main feature — bulk delete by tag
- TAGKEYS: list all keys for a tag
- TAGCOUNT: count keys in a tag

#### 4.4 Tag Hierarchy
- Store parent→children mapping in TagIndex
- TAGLINK parent child → register hierarchy
- TAGUNLINK parent child → remove hierarchy
- TAGCHILDREN tag → list children
- TAGINVALIDATE tag CASCADE → invalidate tag + recursively invalidate all children

Data structure:
```go
type TagIndex struct {
    shards   [TagShards]*tagShard
    // hierarchy: parent tag → set of child tags
    hierarchy    sync.RWMutex
    childrenMap  map[string]map[string]struct{} // parent → {child1, child2}
    parentMap    map[string]string              // child → parent (for lookup)
}
```

#### 4.5 Integration with Store
- Modify store.Set() to accept optional tags
- Modify store.Del() to clean up tag associations
- Modify eviction to clean up tag associations
- Modify timing wheel expiration to clean up tag associations

#### 4.6 Tests
- Tag CRUD: SETTAG, TAGS, ADDTAG, REMTAG
- Invalidation: set 1000 keys with same tag, INVALIDATE, verify all gone
- Cross-tag: key in multiple tags, delete key, verify removed from all tags
- Hierarchy: TAGLINK, CASCADE invalidation, verify children invalidated
- Concurrent: parallel SETTAG + INVALIDATE from multiple goroutines
- Benchmark: INVALIDATE with 10K keys, measure latency

### Phase 4 Deliverables
- [ ] SETTAG + INVALIDATE flow works end-to-end
- [ ] Tag hierarchy with CASCADE works
- [ ] No memory leaks after invalidation (tag index fully cleaned)
- [ ] Concurrent access is safe
- [ ] Benchmark shows <10ms for invalidating 10K keys

---

## Phase 5: Namespace System

### Goal
Named namespaces as independent keyspaces, replacing Redis numbered databases.

### Steps

#### 5.1 Namespace Manager (`internal/store/namespace.go`)
- Each Namespace contains: ShardMap, TagIndex, TimingWheel, EvictionController
- NamespaceManager holds all namespaces
- "default" namespace always exists
- Thread-safe creation/deletion/lookup

#### 5.2 Namespace Commands
- NAMESPACE name → switch connection to namespace
- NAMESPACES → list all
- NAMESPACEDEL name → flush and remove (except "default")
- NAMESPACEINFO → stats for a namespace

#### 5.3 SELECT Compatibility
- SELECT 0 → "default"
- SELECT N → namespace "db{N}"
- Creates namespace if not exists (for Redis compat)

#### 5.4 Per-Namespace Config
- DefaultTTL per namespace
- MaxMemory per namespace
- Read from YAML config on startup

#### 5.5 Connection State
- Connection tracks current namespace name
- Commands route to correct namespace through Connection → NamespaceManager → Namespace → ShardMap

#### 5.6 FLUSHDB / FLUSHALL
- FLUSHDB: flush current namespace only
- FLUSHALL: flush ALL namespaces

#### 5.7 Tests
- Create namespace, switch, set key, switch back, verify isolation
- Per-namespace TTL defaults
- Per-namespace memory limits
- FLUSHDB only affects current namespace
- SELECT compatibility

### Phase 5 Deliverables
- [ ] NAMESPACE create/switch works
- [ ] Keys are isolated between namespaces
- [ ] Per-namespace config works
- [ ] SELECT N backward compatibility works

---

## Phase 6: Plugin System

### Goal
Extensible hook-based plugin system with built-in plugins.

### Steps

#### 6.1 Plugin Interfaces (`internal/plugin/hooks.go`)
Define all hook interfaces as specified in architecture:
- Plugin (base: Name, Version, Init, Close)
- BeforeCommandHook
- AfterCommandHook
- OnEvictHook
- OnExpireHook
- OnTagInvalidateHook
- OnStartupHook
- OnShutdownHook
- CustomCommandProvider
- HTTPEndpointProvider

#### 6.2 Plugin Manager (`internal/plugin/manager.go`)
```go
type Manager struct {
    plugins    []Plugin
    beforeHooks []BeforeCommandHook
    afterHooks  []AfterCommandHook
    evictHooks  []OnEvictHook
    expireHooks []OnExpireHook
    tagHooks    []OnTagInvalidateHook
    httpRoutes  []HTTPRoute
}

func (m *Manager) Register(p Plugin) error  // register + type-assert for hooks
func (m *Manager) InitAll(cfg) error         // init all plugins in order
func (m *Manager) CloseAll() error           // close all in reverse order
func (m *Manager) RunBeforeHooks(ctx) error  // run all before hooks, stop on error
func (m *Manager) RunAfterHooks(ctx)         // run all after hooks (no stop)
func (m *Manager) RunEvictHooks(key, entry)
func (m *Manager) RunExpireHooks(key, entry)
func (m *Manager) RunTagInvalidateHooks(tag, keys)
```

#### 6.3 Hook Pipeline Integration
- In connection handler: before command execution → RunBeforeHooks
- After command execution → RunAfterHooks
- In store eviction → RunEvictHooks
- In timing wheel expiration → RunExpireHooks
- In tag invalidation → RunTagInvalidateHooks

#### 6.4 Stats Plugin (`plugins/stats/stats.go`)
- Track: total_commands, commands_per_type, hit_count, miss_count, hit_ratio
- Track: keys_created, keys_deleted, keys_expired, keys_evicted
- Track: latency percentiles (p50, p95, p99) using HDR histogram or simple approximation
- Expose via INFO stats section
- Implement: BeforeCommandHook (start timer), AfterCommandHook (record latency + counts)

#### 6.5 Auth Plugin (`plugins/auth/auth.go`)
- Password-based authentication
- Track `authenticated` flag on Connection
- BeforeCommandHook: if not authenticated and command requires auth → return ErrAuthRequired
- NoAuth commands: PING, AUTH, QUIT

#### 6.6 SlowLog Plugin (`plugins/slowlog/slowlog.go`)
- Ring buffer of slow queries
- AfterCommandHook: if latency > threshold → add to ring buffer
- Custom command: SLOWLOG GET [count], SLOWLOG RESET, SLOWLOG LEN

#### 6.7 Metrics Plugin (`plugins/metrics/metrics.go`)
- HTTP server on configured port
- Prometheus metrics:
  - `cachestorm_commands_total{command="SET"}` counter
  - `cachestorm_command_duration_seconds{command="SET"}` histogram
  - `cachestorm_keys_total{namespace="default"}` gauge
  - `cachestorm_memory_bytes{namespace="default"}` gauge
  - `cachestorm_hit_total` counter
  - `cachestorm_miss_total` counter
  - `cachestorm_evicted_total` counter
  - `cachestorm_expired_total` counter
  - `cachestorm_connected_clients` gauge
  - `cachestorm_tag_invalidations_total` counter
- HTTPEndpointProvider: registers /metrics route

#### 6.8 Plugin Loading from Config
- Read plugins section from YAML
- Register enabled plugins
- Pass plugin-specific config

#### 6.9 Tests
- Test plugin lifecycle: register → init → close
- Test hook ordering
- Test stats accuracy after N operations
- Test auth flow: connect → command fails → AUTH → command succeeds
- Test slowlog: send slow command, verify logged
- Test metrics endpoint returns valid Prometheus format

### Phase 6 Deliverables
- [ ] Plugin system loads and initializes plugins from config
- [ ] Stats plugin tracks hit/miss/latency
- [ ] Auth plugin blocks unauthenticated commands
- [ ] SlowLog plugin captures slow queries
- [ ] Metrics plugin serves Prometheus endpoint
- [ ] All hooks fire correctly

---

## Phase 7: Persistence

### Goal
Optional disk persistence via AOF and snapshots, with crash recovery.

### Steps

#### 7.1 AOF Writer (`plugins/persistence/aof.go`)
- Append RESP commands to file (like Redis AOF)
- Only write mutating commands: SET, DEL, HSET, LPUSH, SADD, SETTAG, INVALIDATE, EXPIRE, etc.
- Three sync modes:
  - `always`: fsync after every write (safest, slowest)
  - `everysec`: fsync every second (default, good balance)
  - `no`: let OS handle (fastest, riskiest)
- File format: standard RESP commands, one per entry
- AOF rewrite: when file exceeds max_aof_size, create new compact AOF from current state

#### 7.2 Snapshot (`plugins/persistence/snapshot.go`)
- Periodic full dump of all namespaces
- Binary format for efficiency:
  ```
  [Header: magic "CSDB" + version + timestamp]
  [Namespace count]
  For each namespace:
    [Namespace name length + name]
    [Key count]
    For each key:
      [Key length + key]
      [Data type byte]
      [Value bytes (type-specific encoding)]
      [Tag count + tags]
      [ExpiresAt int64]
  [CRC32 checksum]
  ```
- Run in background goroutine
- Use Entry.Clone() to avoid holding locks during write

#### 7.3 Recovery Manager (`plugins/persistence/recovery.go`)
- On startup: check for snapshot file + AOF file
- Recovery order:
  1. Load latest snapshot (faster, approximate state)
  2. Replay AOF commands after snapshot timestamp (brings to exact state)
  3. If no snapshot, replay entire AOF
- Validate CRC32 on snapshot load
- Handle corrupted AOF: stop at first invalid command, log warning

#### 7.4 Persistence Plugin Entry Point (`plugins/persistence/persistence.go`)
- Implements: AfterCommandHook (AOF write), OnStartupHook (recovery), OnShutdownHook (final flush)
- Config: enabled, aof, aof_sync, snapshot_interval, data_dir, max_aof_size

#### 7.5 Tests
- Set data → stop → restart → verify data recovered
- AOF: write 1000 commands, replay, verify state matches
- Snapshot: create snapshot, load, verify
- Snapshot + AOF: snapshot at T1, more writes, crash, recover, verify all data
- Corrupted AOF: inject bad bytes, verify partial recovery
- AOF rewrite: trigger rewrite, verify compact file is correct

### Phase 7 Deliverables
- [ ] AOF logs all mutations
- [ ] Snapshot creates valid binary dump
- [ ] Recovery restores state from snapshot + AOF
- [ ] Data survives restart
- [ ] Corrupted files handled gracefully

---

## Phase 8: Multi-Node Clustering

### Goal
Gossip-based cluster with hash slot routing, replication, and cross-node tag invalidation.

### Steps

#### 8.1 Cluster Manager (`internal/cluster/cluster.go`)
```go
type Cluster struct {
    config      *config.ClusterConfig
    self        *Node
    nodes       sync.Map  // nodeID → *Node
    slots       [16384]*SlotInfo
    memberlist  *memberlist.Memberlist
    store       *store.Store
    broadcaster *TagBroadcaster
    replicator  *Replicator
    stopCh      chan struct{}
}

type Node struct {
    ID        string
    Addr      string
    Port      int
    GossipPort int
    Role      NodeRole // Primary or Replica
    Slots     []SlotRange
    ReplicaOf string   // primary node ID if replica
    State     NodeState
    LastSeen  time.Time
}

type SlotInfo struct {
    Primary  *Node
    Replicas []*Node
}
```

#### 8.2 Hash Slot Routing (`internal/cluster/hash_slots.go`)
- CRC16 implementation (from scratch, not imported)
- `func KeySlot(key string) uint16` → CRC16(key) % 16384
- Handle hash tags: `{user}.profile` → slot based on "user" only
- Slot range assignment: on cluster init, divide 16384 slots evenly among nodes

#### 8.3 Gossip Protocol (`internal/cluster/gossip.go`)
- Use HashiCorp memberlist for node discovery
- Implement memberlist.Delegate interface:
  - NodeMeta: return node ID, role, slot ranges
  - NotifyMsg: handle custom messages (tag invalidation broadcasts)
  - GetBroadcasts: pending broadcasts to send
  - LocalState/MergeRemoteState: cluster state sync
- Implement memberlist.EventDelegate:
  - NotifyJoin: new node joined → rebalance slots
  - NotifyLeave: node left → failover
  - NotifyUpdate: node metadata changed

#### 8.4 Command Routing in Cluster Mode
- Before executing command, check if key's slot belongs to this node
- If not: return MOVED error with correct node address
  - `"-MOVED {slot} {ip}:{port}\r\n"`
- This is how Redis Cluster works — clients handle MOVED by redirecting

#### 8.5 Replication (`internal/cluster/replication.go`)
- Primary → Replica async replication
- Replication stream: replicate every mutating command to replicas
- Full sync: when replica joins, send snapshot + AOF stream
- Partial sync: after temporary disconnect, send missed commands (backlog buffer)

#### 8.6 Tag Invalidation Broadcast (`internal/cluster/tag_broadcast.go`)
- When INVALIDATE runs on one node:
  1. Delete local keys for the tag
  2. Broadcast invalidation to all other nodes via memberlist
  3. Each receiving node deletes its local keys for the tag
- Message format: `{type: TAG_INVALIDATE, tag: "users", originNode: "node-1"}`
- Deduplication: track recent invalidation IDs to prevent loops

#### 8.7 Slot Migration (`internal/cluster/migration.go`)
- When adding/removing nodes, slots need to move
- Migration process:
  1. Mark slot as MIGRATING on source, IMPORTING on target
  2. Iterate all keys in slot on source
  3. For each key: send to target, delete from source
  4. Update slot assignment
  5. Clear MIGRATING/IMPORTING flags

#### 8.8 Cluster Commands
- CLUSTER INFO, CLUSTER NODES, CLUSTER SLOTS, CLUSTER MEET, CLUSTER REPLICATE, CLUSTER MYID, CLUSTER RESET
- Implement in `internal/command/cluster_commands.go`

#### 8.9 Docker Compose for Testing
```yaml
# docker/docker-compose.yml - 3 node cluster
version: '3.8'
services:
  cachestorm-1:
    build: ..
    ports: ["6380:6380"]
    environment:
      CACHESTORM_CLUSTER_ENABLED: "true"
      CACHESTORM_CLUSTER_NODE_NAME: "node-1"
      CACHESTORM_CLUSTER_SEEDS: "cachestorm-2:7946,cachestorm-3:7946"
  cachestorm-2:
    build: ..
    ports: ["6381:6380"]
    environment:
      CACHESTORM_CLUSTER_ENABLED: "true"
      CACHESTORM_CLUSTER_NODE_NAME: "node-2"
      CACHESTORM_CLUSTER_SEEDS: "cachestorm-1:7946,cachestorm-3:7946"
  cachestorm-3:
    build: ..
    ports: ["6382:6380"]
    environment:
      CACHESTORM_CLUSTER_ENABLED: "true"
      CACHESTORM_CLUSTER_NODE_NAME: "node-3"
      CACHESTORM_CLUSTER_SEEDS: "cachestorm-1:7946,cachestorm-2:7946"
```

#### 8.10 Tests
- 3-node cluster formation via gossip
- Key routing: SET on node-1, GET redirected to correct node
- MOVED response handling
- Tag invalidation across nodes
- Node join: verify rebalance
- Node leave: verify failover (replica promoted)
- Replication: write on primary, read from replica

### Phase 8 Deliverables
- [ ] 3-node cluster forms automatically via gossip
- [ ] Keys route to correct node by hash slot
- [ ] MOVED responses work correctly
- [ ] Tag invalidation propagates to all nodes
- [ ] Replication: primary → replica sync works
- [ ] Docker compose spins up working cluster

---

## Phase 9: Admin HTTP API + Hot Keys + Memory Inspector

### Goal
HTTP admin API, hot key detection, detailed memory analysis.

### Steps

#### 9.1 Admin HTTP Server
- Separate HTTP server on port 9090 (configurable)
- Endpoints:
  - `GET /health` → `{"status":"ok","uptime":"..."}`
  - `GET /info` → same as INFO command in JSON format
  - `GET /namespaces` → list namespaces with stats
  - `GET /namespaces/{name}` → namespace detail
  - `GET /tags` → list all tags with key counts
  - `GET /tags/{name}` → tag detail with keys
  - `GET /hotkeys?n=10` → top N hottest keys
  - `GET /memory` → memory breakdown
  - `GET /cluster` → cluster state (if enabled)
  - `GET /plugins` → loaded plugins
  - `GET /metrics` → Prometheus (from metrics plugin)

#### 9.2 Hot Key Detection
- Track top-K keys by access count
- Use approximate algorithm: maintain min-heap of size K
- On every Get/Set: if access_count > min of heap → replace
- HOTKEYS command returns current top K
- Also exposed via HTTP `/hotkeys`

#### 9.3 Memory Inspector (MEMINFO)
- Global: total used, max, per-namespace breakdown
- Per namespace: key count, memory by type, tag count, avg entry size
- Per tag: key count, total memory, avg TTL
- Returns structured text format

#### 9.4 CLIENT Commands
- CLIENT LIST: connected clients with info (id, addr, age, namespace, last_cmd)
- CLIENT GETNAME / SETNAME
- CLIENT ID

#### 9.5 CONFIG GET/SET
- Limited runtime config changes:
  - maxmemory
  - eviction_policy
  - slowlog threshold
  - loglevel

#### 9.6 Tests
- HTTP endpoints return correct JSON
- Hot key tracking accuracy
- Memory inspector numbers match actual usage

### Phase 9 Deliverables
- [ ] HTTP admin API fully functional
- [ ] Hot key detection works
- [ ] MEMINFO shows accurate memory breakdown
- [ ] CLIENT commands work
- [ ] CONFIG GET/SET for runtime changes

---

## Phase 10: Polish, Benchmarks, Documentation, Release

### Goal
Production-ready v1.0.0 release.

### Steps

#### 10.1 Comprehensive Benchmark Suite (`benchmarks/`)
- RESP parsing throughput
- SET/GET single key throughput (ops/sec)
- SET/GET with 100 byte / 1KB / 10KB values
- MSET/MGET with 10/100 keys
- Hash: HSET/HGET throughput
- List: LPUSH/RPUSH/LPOP throughput
- Set: SADD/SMEMBERS throughput
- Tag: SETTAG + INVALIDATE with 1K/10K/100K keys
- Eviction under memory pressure
- Concurrent: 100/1000 goroutines hitting SET/GET
- Compare with Redis using redis-benchmark tool

#### 10.2 Documentation
- README.md: project overview, quick start, docker, features
- docs/commands.md: all commands with examples
- docs/tags.md: tag system guide with use cases
- docs/clustering.md: cluster setup and operation
- docs/plugins.md: plugin development guide
- docs/config.md: all config options reference
- docs/migration-from-redis.md: how to migrate from Redis

#### 10.3 Dockerfile
```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o cachestorm ./cmd/cachestorm

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/cachestorm /usr/local/bin/cachestorm
COPY config/cachestorm.example.yaml /etc/cachestorm/cachestorm.yaml
EXPOSE 6380 7946 9090
ENTRYPOINT ["cachestorm"]
CMD ["--config", "/etc/cachestorm/cachestorm.yaml"]
```

#### 10.4 Makefile
```makefile
.PHONY: build test bench lint docker

build:
	go build -o bin/cachestorm ./cmd/cachestorm

test:
	go test ./... -race -count=1

bench:
	go test ./... -bench=. -benchmem

lint:
	golangci-lint run

docker:
	docker build -t cachestorm:latest -f docker/Dockerfile .

docker-cluster:
	docker compose -f docker/docker-compose.yml up

clean:
	rm -rf bin/

release:
	goreleaser release --clean
```

#### 10.5 CI/CD (`.github/workflows/ci.yml`)
- On push: lint + test + build
- On tag: GoReleaser → GitHub Release + Docker Hub

#### 10.6 GoReleaser Config
- Build for: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- Create .tar.gz and .zip archives
- Docker images: latest + version tagged
- Homebrew formula (optional)

#### 10.7 Example Config File
- Create `config/cachestorm.example.yaml` with ALL options documented

#### 10.8 CHANGELOG.md
- Document all features for v1.0.0

#### 10.9 Final Testing
- Run full test suite
- Run benchmarks, document results
- Test Docker single node
- Test Docker 3-node cluster
- Test with real Redis clients: ioredis (Node.js), go-redis (Go), redis-py (Python)
- Test graceful shutdown under load
- Test crash recovery with persistence
- Memory leak check: run for 1 hour with continuous SET/DEL, verify stable memory

### Phase 10 Deliverables (v1.0.0)
- [ ] All tests pass with `go test ./... -race`
- [ ] Benchmarks documented and competitive
- [ ] Docker image builds and runs
- [ ] 3-node cluster works via docker compose
- [ ] Documentation complete
- [ ] CI/CD pipeline working
- [ ] GoReleaser produces binaries
- [ ] CHANGELOG written
- [ ] README is polished and complete
- [ ] Tagged as v1.0.0
