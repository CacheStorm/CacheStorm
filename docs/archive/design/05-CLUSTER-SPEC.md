# CacheStorm — Cluster Specification

## 1. Overview

CacheStorm clustering provides horizontal scaling through data sharding across multiple nodes. The design is inspired by Redis Cluster but simplified for cache use cases where eventual consistency is acceptable.

**Key Decisions:**
- 16384 hash slots (same as Redis Cluster for compatibility)
- Gossip-based node discovery (HashiCorp memberlist)
- Async primary → replica replication
- Cross-node tag invalidation via broadcast
- Eventual consistency model (appropriate for cache)

## 2. Hash Slot Routing

### 2.1 Slot Calculation

```go
// CRC16 implementation (CCITT variant, same as Redis)
// Polynomial: x^16 + x^12 + x^5 + 1 (0x1021)
func crc16(data []byte) uint16 {
    crc := uint16(0)
    for _, b := range data {
        crc = (crc << 8) ^ crc16tab[byte(crc>>8)^b]
    }
    return crc
}

// crc16tab is precomputed lookup table for CRC16-CCITT
var crc16tab = [256]uint16{ /* 256 entries */ }

const TotalSlots = 16384

// KeySlot returns the hash slot for a given key.
// Supports hash tags: {tag}rest → slot based on "tag" only.
func KeySlot(key string) uint16 {
    // Check for hash tag: find first '{' and matching '}'
    if start := strings.IndexByte(key, '{'); start >= 0 {
        if end := strings.IndexByte(key[start+1:], '}'); end > 0 {
            key = key[start+1 : start+1+end]
        }
    }
    return crc16([]byte(key)) % TotalSlots
}
```

### 2.2 Slot Assignment

On cluster formation with N nodes, slots are divided evenly:
```
3 nodes:
  Node A: slots 0-5460      (5461 slots)
  Node B: slots 5461-10922   (5462 slots)
  Node C: slots 10923-16383  (5461 slots)
```

When a new node joins:
```
Before (3 nodes): A[0-5460], B[5461-10922], C[10923-16383]
After  (4 nodes): A[0-4095], B[4096-8191], C[8192-12287], D[12288-16383]
→ Migrate slots from existing nodes to new node
```

### 2.3 Slot Info Structure

```go
type SlotRange struct {
    Start uint16
    End   uint16
}

type SlotState uint8
const (
    SlotNormal    SlotState = iota
    SlotMigrating           // slot is being moved away from this node
    SlotImporting           // slot is being moved to this node
)

type SlotInfo struct {
    Primary     *Node
    Replicas    []*Node
    State       SlotState
    MigratingTo string // node ID when migrating
    ImportingFrom string // node ID when importing
}
```

## 3. Gossip Protocol

Using HashiCorp memberlist for zero-config node discovery.

### 3.1 Node Metadata

```go
// NodeMeta is serialized and shared via gossip.
// Must be compact — memberlist has metadata size limits.
type NodeMeta struct {
    ID        string   `json:"id"`        // unique node identifier (UUID)
    RESPPort  int      `json:"resp_port"` // RESP server port
    Role      string   `json:"role"`      // "primary" or "replica"
    ReplicaOf string   `json:"replica_of"` // primary node ID if replica
    SlotRanges []SlotRange `json:"slots"`  // owned slot ranges
    State     string   `json:"state"`     // "online", "loading", "syncing"
}
```

### 3.2 Memberlist Delegates

```go
type ClusterDelegate struct {
    cluster *Cluster
}

// NodeMeta returns serialized metadata for this node.
func (d *ClusterDelegate) NodeMeta(limit int) []byte {
    meta := d.cluster.self.Meta()
    data, _ := json.Marshal(meta)
    return data
}

// NotifyMsg handles incoming custom messages (tag invalidation, replication, etc.)
func (d *ClusterDelegate) NotifyMsg(msg []byte) {
    // Parse message type
    // Dispatch to appropriate handler
}

// GetBroadcasts returns pending messages to broadcast.
func (d *ClusterDelegate) GetBroadcasts(overhead, limit int) [][]byte {
    return d.cluster.broadcaster.GetPending(overhead, limit)
}

// LocalState is used for full state sync on join.
func (d *ClusterDelegate) LocalState(join bool) []byte {
    // Return full slot assignment table
}

// MergeRemoteState merges state from another node.
func (d *ClusterDelegate) MergeRemoteState(buf []byte, join bool) {
    // Update local slot table with remote state
}
```

### 3.3 Event Handlers

```go
type ClusterEventDelegate struct {
    cluster *Cluster
}

func (d *ClusterEventDelegate) NotifyJoin(node *memberlist.Node) {
    // Parse node metadata
    // Add to known nodes
    // If new node, trigger slot rebalance
    log.Info().Str("node", node.Name).Msg("node joined cluster")
}

func (d *ClusterEventDelegate) NotifyLeave(node *memberlist.Node) {
    // Remove from known nodes
    // If primary left, promote replica
    // If no replica, mark slots as unassigned
    log.Warn().Str("node", node.Name).Msg("node left cluster")
}

func (d *ClusterEventDelegate) NotifyUpdate(node *memberlist.Node) {
    // Update node metadata (role change, slot change, etc.)
}
```

## 4. Command Routing in Cluster Mode

### 4.1 Flow

```
Client sends: SET user:123 "data"
    │
    ├── Calculate slot: KeySlot("user:123") = 7843
    ├── Lookup: slots[7843].Primary = Node B
    │
    ├── If this IS Node B:
    │   └── Execute SET locally
    │       └── After: replicate to Node B's replicas
    │
    └── If this is NOT Node B:
        └── Return: -MOVED 7843 10.0.0.2:6380
            └── Client redirects to Node B and retries
```

### 4.2 MOVED and ASK Responses

```
MOVED: Permanent redirect. Key definitely belongs to another node.
  Format: -MOVED {slot} {ip}:{port}\r\n
  Client should update its slot→node mapping cache.

ASK: Temporary redirect during migration.
  Format: -ASK {slot} {ip}:{port}\r\n
  Client should redirect this one request only, prefixed with ASKING command.
```

### 4.3 Cluster-Aware Command Execution

```go
func (s *Server) executeInCluster(ctx *CommandContext) error {
    if !s.cluster.IsEnabled() {
        // Single node mode — execute directly
        return s.router.Execute(ctx)
    }

    // Extract key from command (first key argument)
    key := ctx.ExtractKey()
    if key == "" {
        // Commands without keys (PING, INFO, etc.) execute locally
        return s.router.Execute(ctx)
    }

    slot := cluster.KeySlot(key)
    owner := s.cluster.SlotOwner(slot)

    if owner.ID == s.cluster.Self().ID {
        // We own this slot — execute locally
        return s.router.Execute(ctx)
    }

    // Check migration state
    slotInfo := s.cluster.SlotInfo(slot)
    if slotInfo.State == SlotMigrating {
        // Key might still be here or already migrated
        // Try local first, if not found → ASK redirect
        if exists := s.store.Exists(ctx.Namespace, key); exists {
            return s.router.Execute(ctx)
        }
        return ctx.WriteError(fmt.Sprintf("ASK %d %s:%d", slot, owner.Addr, owner.Port))
    }

    // Not our slot — MOVED redirect
    return ctx.WriteError(fmt.Sprintf("MOVED %d %s:%d", slot, owner.Addr, owner.Port))
}
```

## 5. Replication

### 5.1 Replication Model

- Async replication (no wait by default)
- Primary → Replica direction only
- Replica is read-only

### 5.2 Full Sync (Initial)

When a replica connects to its primary for the first time:
1. Primary creates background snapshot
2. Primary sends snapshot to replica
3. Replica loads snapshot into its store
4. Primary starts streaming replication backlog from snapshot point
5. Replica enters "online" state

### 5.3 Streaming Replication

```go
type ReplicationStream struct {
    // Ring buffer of recent mutating commands
    backlog    []ReplicationEntry
    backlogMu  sync.RWMutex
    backlogIdx int64  // current position
    maxSize    int    // max entries in backlog
}

type ReplicationEntry struct {
    Offset    int64     // monotonically increasing
    Namespace string
    Command   string
    Args      [][]byte
    Timestamp time.Time
}

// After every mutating command on primary:
// 1. Append to replication backlog
// 2. Notify replicas of new entry
// 3. Replicas pull and apply
```

### 5.4 Replica Read-Only Mode

When cluster is enabled and this node is a replica:
- Write commands return: `-READONLY You can't write against a read only replica.`
- Read commands execute locally
- Exception: PING, AUTH, QUIT, INFO always allowed

## 6. Cross-Node Tag Invalidation

### 6.1 The Problem

When `INVALIDATE users` runs on Node A, keys tagged "users" may exist on Nodes B and C too. All nodes must invalidate their local keys.

### 6.2 Solution: Broadcast via Gossip

```go
type TagInvalidateMsg struct {
    Type      byte     // MsgTypeTagInvalidate = 0x01
    MessageID string   // UUID for dedup
    Tag       string
    Namespace string
    Cascade   bool
    Origin    string   // originating node ID
}

// When INVALIDATE command runs:
func (c *Cluster) BroadcastTagInvalidation(ns, tag string, cascade bool) {
    msg := TagInvalidateMsg{
        Type:      MsgTypeTagInvalidate,
        MessageID: uuid.New().String(),
        Tag:       tag,
        Namespace: ns,
        Cascade:   cascade,
        Origin:    c.self.ID,
    }
    data, _ := json.Marshal(msg)
    c.broadcaster.QueueBroadcast(data)
}

// When receiving invalidation from another node:
func (c *Cluster) handleTagInvalidation(msg TagInvalidateMsg) {
    // Dedup check
    if c.recentMessages.Has(msg.MessageID) {
        return
    }
    c.recentMessages.Add(msg.MessageID)

    // Invalidate local keys
    ns := c.store.NamespaceManager().Get(msg.Namespace)
    if ns != nil {
        if msg.Cascade {
            ns.Tags.InvalidateCascade(msg.Tag, ns.Store)
        } else {
            ns.Tags.Invalidate(msg.Tag, ns.Store)
        }
    }
}
```

### 6.3 Deduplication

Recent message IDs are tracked in a time-limited set:
```go
type RecentMessages struct {
    mu       sync.Mutex
    messages map[string]time.Time // messageID → timestamp
}

// Prune entries older than 60 seconds every 30 seconds.
// This prevents processing the same invalidation twice in case of gossip storms.
```

## 7. Slot Migration

### 7.1 Migration Process

When rebalancing slots from Node A to Node B:

```
Phase 1: Preparation
  - Node A marks slot as MIGRATING
  - Node B marks slot as IMPORTING

Phase 2: Key Transfer
  - Node A iterates all keys in the migrating slot
  - For each key:
    1. Send key + value + TTL + tags to Node B via internal protocol
    2. Node B imports the key
    3. Node A deletes the local copy
  - During migration:
    - New writes to migrating keys on Node A → ASK redirect to Node B
    - Reads on Node A: if key exists locally, serve it; else ASK redirect

Phase 3: Completion
  - All keys transferred
  - Node A removes slot from its ownership
  - Node B adds slot to its ownership
  - Both nodes update metadata via gossip
  - Clients receive MOVED for subsequent requests and update their cache
```

### 7.2 Internal Protocol for Migration

Between nodes, use a simple TCP protocol (not RESP, to avoid confusion):
```
Message format:
[4 bytes: message length]
[1 byte: message type]
[payload]

Types:
0x10: MIGRATE_KEY {namespace, key, type, value, ttl, tags}
0x11: MIGRATE_KEY_ACK {key, success}
0x12: MIGRATE_COMPLETE {slot}
0x20: FULL_SYNC_START
0x21: FULL_SYNC_DATA {snapshot chunk}
0x22: FULL_SYNC_END
0x30: REPL_ENTRY {offset, namespace, command, args}
```

## 8. Failure Handling

### 8.1 Node Failure Detection

Memberlist handles failure detection via gossip:
- Probes every 1 second
- Indirect probes via other nodes
- Suspicion mechanism before declaring dead
- Configurable timeouts

### 8.2 Failover

When a primary node fails:
1. Memberlist NotifyLeave fires
2. Check if failed node had replicas
3. If yes: promote the replica with highest replication offset
   - Promoted replica takes ownership of the primary's slots
   - Update cluster state via gossip
4. If no replica: slots become unassigned
   - Commands to those slots return CLUSTERDOWN
   - Log critical warning

### 8.3 Split Brain Prevention

Simple approach for cache:
- No split-brain prevention (cache can tolerate inconsistency)
- When partitions heal, nodes reconcile via gossip
- Stale data naturally expires via TTL
- Tag invalidation may be missed during partition — acceptable for cache

## 9. Cluster Configuration

```yaml
cluster:
  enabled: true
  node_name: "node-1"          # human-readable name (ID is auto-generated UUID)
  bind_addr: "0.0.0.0"         # gossip listener bind address
  bind_port: 7946              # gossip listener port
  advertise_addr: ""           # public address for NAT/Docker (auto-detect if empty)
  advertise_port: 0            # public port (same as bind_port if 0)
  seeds:                       # known nodes to bootstrap cluster
    - "10.0.0.2:7946"
    - "10.0.0.3:7946"
  replicas: 1                  # number of replicas per primary
  replication_backlog: 10000   # max entries in replication backlog
  migration_batch_size: 100    # keys per batch during slot migration
  gossip_interval: "200ms"     # gossip protocol interval
  probe_interval: "1s"         # failure detection probe interval
  probe_timeout: "500ms"       # probe response timeout
  suspicion_mult: 4            # suspicion multiplier before declaring dead
```

## 10. Cluster State Machine

```
Node States:
  INIT      → starting up, loading config
  JOINING   → connecting to seed nodes via gossip
  SYNCING   → receiving full sync from primary (if replica)
  ONLINE    → fully operational
  LEAVING   → gracefully leaving cluster
  FAILED    → detected as failed by other nodes

Transitions:
  INIT → JOINING      : cluster enabled, start gossip
  JOINING → SYNCING   : joined cluster, assigned as replica
  JOINING → ONLINE    : joined cluster, assigned as primary (no sync needed)
  SYNCING → ONLINE    : full sync complete
  ONLINE → LEAVING    : graceful shutdown initiated
  LEAVING → (exit)    : migration complete, left gossip
  ONLINE → FAILED     : detected by other nodes (this node doesn't know)
  FAILED → ONLINE     : node comes back, rejoins gossip
```
