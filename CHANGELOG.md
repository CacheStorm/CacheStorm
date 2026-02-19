# CacheStorm Changelog

All notable changes to this project will be documented in this file.

## [1.0.0] - 2026-02-19

### Added
- **Core Server**
  - TCP server with RESP3 protocol support
  - Graceful shutdown with SIGINT/SIGTERM handling
  - Connection pooling and management
  - Configurable bind address, port, and timeouts

- **Data Structures**
  - String: SET, GET, MSET, MGET, INCR, DECR, APPEND, etc.
  - Hash: HSET, HGET, HGETALL, HMSET, HMGET, HDEL, etc.
  - List: LPUSH, RPUSH, LPOP, RPOP, LRANGE, LLEN, etc.
  - Set: SADD, SREM, SMEMBERS, SUNION, SINTER, SDIFF, etc.

- **Tag System (Killer Feature!)**
  - SETTAG: Set key with associated tags
  - TAGS: Get tags for a key
  - ADDTAG/REMTAG: Modify tags on existing keys
  - INVALIDATE: Bulk delete by tag
  - TAGKEYS/TAGCOUNT: Query tag information
  - Tag hierarchy with CASCADE invalidation
  - Bidirectional tag index for O(1) lookups

- **Storage Engine**
  - 256-shard concurrent hashmap
  - FNV-1a hash for key distribution
  - Per-shard RWMutex for minimal contention
  - Incremental memory tracking

- **TTL & Eviction**
  - Hierarchical 4-level timing wheel
  - LRU, LFU, and random eviction policies
  - Configurable memory limits and pressure thresholds
  - Lazy expiration on read

- **Namespace System**
  - Named namespaces (not numbered databases)
  - Per-namespace configuration
  - SELECT N backward compatibility

- **Clustering**
  - Hash slot routing (16384 slots)
  - CRC16 key distribution
  - Tag invalidation broadcast
  - Node management

- **Plugin System**
  - Hook-based extensibility
  - BeforeCommand/AfterCommand hooks
  - OnEvict/OnExpire/OnTagInvalidate hooks
  - OnStartup/OnShutdown hooks

- **Built-in Plugins**
  - Stats: Command counts, hit/miss ratio, latency
  - Persistence: AOF + Snapshot
  - Auth: Password authentication
  - SlowLog: Slow query logging
  - Metrics: Prometheus exporter

- **Commands**
  - Server: PING, ECHO, INFO, DBSIZE, FLUSHDB, FLUSHALL, TIME
  - Keys: EXPIRE, TTL, PERSIST, TYPE, RENAME, KEYS, SCAN, RANDOMKEY
  - Admin: HOTKEYS, MEMINFO, CLIENT
  - Cluster: CLUSTER INFO, CLUSTER NODES, CLUSTER SLOTS

- **HTTP Admin API**
  - /health - Health check
  - /info - Server information
  - /keys - Key listing
  - /tags - Tag information
  - /memory - Memory stats
  - /metrics - Prometheus metrics

- **Infrastructure**
  - Docker support with Dockerfile
  - Docker Compose for 3-node cluster
  - GitHub Actions CI/CD
  - GoReleaser configuration
  - Comprehensive test suite
  - Benchmark suite

### Performance
- GET: ~10M ops/sec
- SET: ~1.4M ops/sec
- Parallel GET: ~70M ops/sec
- Parallel SET: ~13M ops/sec

### Dependencies
- github.com/rs/zerolog - Structured logging
- gopkg.in/yaml.v3 - Configuration parsing
- github.com/stretchr/testify - Testing assertions
