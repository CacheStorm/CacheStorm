# CacheStorm Changelog

All notable changes to this project will be documented in this file.

## [1.0.0] - 2026-02-20

### Added

#### Core Server
- TCP server with RESP3 protocol support
- Graceful shutdown with SIGINT/SIGTERM handling
- Connection pooling and management
- Configurable bind address, port, and timeouts
- 256-shard concurrent hashmap architecture

#### Data Structures (9 Types)

**String (22 commands)**
- SET, GET, DEL, EXISTS, MSET, MGET
- INCR, DECR, INCRBY, DECRBY, INCRBYFLOAT
- APPEND, STRLEN, GETRANGE, SETRANGE
- SETNX, GETSET, GETEX, GETDEL, SUBSTR, LCS, COPY

**Hash (18 commands)**
- HSET, HGET, HMSET, HMGET, HGETALL, HDEL
- HEXISTS, HLEN, HKEYS, HVALS
- HINCRBY, HINCRBYFLOAT, HSETNX, HSTRLEN
- HRANDFIELD, HGETDEL, HGETEX, HSCAN

**List (21 commands)**
- LPUSH, RPUSH, LPUSHX, RPUSHX, LPOP, RPOP
- LLEN, LRANGE, LINDEX, LSET, LREM, LTRIM
- RPOPLPUSH, LINSERT, LMOVE
- BLPOP, BRPOP, BRPOPLPUSH, LPOS, LMPOP, LMPUSH

**Set (15 commands)**
- SADD, SREM, SMEMBERS, SISMEMBER, SCARD
- SPOP, SRANDMEMBER, SMOVE
- SUNION, SINTER, SDIFF
- SUNIONSTORE, SINTERSTORE, SDIFFSTORE, SSCAN

**Sorted Set (25 commands)**
- ZADD, ZCARD, ZCOUNT, ZRANGE, ZRANGEBYSCORE
- ZRANK, ZREM, ZSCORE, ZINCRBY
- ZREVRANGE, ZREVRANK, ZREMRANGEBYRANK, ZREMRANGEBYSCORE
- ZPOPMIN, ZPOPMAX, ZRANDMEMBER, ZMSCORE
- ZUNIONSTORE, ZINTERSTORE, ZDIFFSTORE, ZSCAN
- ZLEXCOUNT, ZRANGEBYLEX, ZREMRANGEBYLEX, ZREVRANGEBYSCORE

**Stream (13 commands)**
- XADD, XLEN, XRANGE, XREVRANGE, XREAD
- XDEL, XTRIM, XINFO, XGROUP
- XREADGROUP, XACK, XPENDING, XCLAIM

**Geo (6 commands)**
- GEOADD, GEODIST, GEOHASH, GEOPOS
- GEORADIUS, GEORADIUSBYMEMBER

**Bitmap (6 commands)**
- SETBIT, GETBIT, BITCOUNT, BITPOS
- BITOP (AND/OR/XOR/NOT), BITFIELD

**HyperLogLog (3 commands)**
- PFADD, PFCOUNT, PFMERGE

#### Tag System (Killer Feature!)
- SETTAG: Set key with associated tags
- TAGKEYS: Get all keys for a tag
- TAGCOUNT: Count keys in a tag
- TAGDEL: Delete a tag
- TAGINFO: Get tag information
- INVALIDATE: Bulk delete by tag
- Bidirectional tag index for O(1) lookups

#### Lua Scripting
- EVAL: Execute Lua script
- EVALSHA: Execute cached script by SHA
- SCRIPT LOAD/EXISTS/FLUSH: Script management
- 30+ Redis commands available in Lua
- Full KEYS/ARGV support
- redis.call() and redis.pcall() support

#### Transactions
- MULTI/EXEC/DISCARD: Transaction support
- WATCH/UNWATCH: Optimistic locking

#### Pub/Sub
- SUBSCRIBE/UNSUBSCRIBE: Channel subscription
- PUBLISH: Message publishing
- PSUBSCRIBE/PUNSUBSCRIBE: Pattern subscription
- PUBSUB: Pub/Sub introspection

#### Server Commands (53)
- PING, ECHO, QUIT, COMMAND, INFO, DBSIZE
- FLUSHDB, FLUSHALL, TIME, AUTH
- SLOWLOG, WAIT, ROLE, LASTSAVE, LOLWUT
- SHUTDOWN, SAVE, BGSAVE, BGREWRITEAOF
- SLAVEOF, REPLICAOF, LATENCY
- STRALGO, MODULE, ACL, MONITOR, SWAPDB, SYNC, PSYNC
- CLIENT (LIST, SETNAME, GETNAME, ID, KILL, etc.)
- CONFIG (GET, SET, REWRITE)
- SORT, SORT_RO

#### Key Commands (17)
- EXPIRE, PEXPIRE, EXPIREAT, PEXPIREAT
- TTL, PTTL, PERSIST, TYPE
- RENAME, RENAMENX, KEYS, SCAN
- RANDOMKEY, UNLINK, TOUCH, DUMP, RESTORE

#### Debug Commands
- DEBUG (SLEEP, OBJECT, RELOAD, etc.)
- OBJECT (ENCODING, REFCOUNT, IDLETIME)
- MEMORY (USAGE, STATS)
- HOTKEYS, MEMINFO

#### Storage Engine
- 256-shard concurrent hashmap
- FNV-1a hash for key distribution
- Per-shard RWMutex for minimal contention
- Incremental memory tracking
- Entry types: String, Hash, List, Set, SortedSet, Stream, Geo

#### TTL & Eviction
- Hierarchical 4-level timing wheel
- LRU, LFU, and random eviction policies
- Configurable memory limits and pressure thresholds
- Lazy expiration on read

#### Namespace System
- Named namespaces (not numbered databases)
- Per-namespace configuration
- NSCREATE, NSDEL, NSINFO, NSKEYS
- NAMESPACES: List all namespaces

#### Clustering
- Hash slot routing (16384 slots)
- CRC16 key distribution
- Tag invalidation broadcast
- Node management (MEET, FORGET, REPLICATE)
- MIGRATE command for key migration
- ASKING, READONLY, READWRITE

#### Plugin System
- Hook-based extensibility
- BeforeCommand/AfterCommand hooks
- OnEvict/OnExpire/OnTagInvalidate hooks
- OnStartup/OnShutdown hooks

#### Built-in Plugins
- Stats: Command counts, hit/miss ratio, latency
- Persistence: AOF + Snapshot
- Auth: Password authentication
- SlowLog: Slow query logging
- Metrics: Prometheus exporter

#### HTTP Admin API
- GET /health - Health check
- GET /info - Server information
- GET /keys - Key listing
- GET /tags - Tag information
- GET /memory - Memory stats
- GET /metrics - Prometheus metrics
- GET /stats - Server statistics
- GET /config - Current configuration

#### Infrastructure
- Docker support with Dockerfile
- Docker Compose for 3-node cluster
- GitHub Actions CI/CD
- GoReleaser configuration
- Comprehensive test suite (10 test files, 50+ tests)
- Benchmark suite (14 benchmarks)

### Test Coverage
- `internal/resp/reader_test.go` - RESP protocol tests
- `internal/store/store_test.go` - Store operations
- `internal/store/entry_test.go` - Entry types
- `internal/store/memory_test.go` - Memory tracking
- `internal/store/tag_index_test.go` - Tag system
- `internal/command/lua_test.go` - Lua scripting (30 tests)

### Performance
| Operation | Latency | Ops/sec | Allocations |
|-----------|---------|---------|-------------|
| GET | 71 ns/op | ~14M | 0 allocs |
| GET (parallel) | 13 ns/op | ~77M | 0 allocs |
| SET | 669 ns/op | ~1.5M | 3 allocs |
| SET (parallel) | 66 ns/op | ~15M | 3 allocs |
| DELETE | 941 ns/op | ~1M | 0 allocs |
| TAGCOUNT | 19 ns/op | ~53M | 0 allocs |

### Project Statistics
- Go Files: 64
- Total Lines: ~12,000
- Commands: 180+
- Handlers: 219
- Data Types: 9
- Test Files: 10

### Dependencies
- `github.com/rs/zerolog` - Structured logging
- `github.com/yuin/gopher-lua` - Lua scripting engine
- `gopkg.in/yaml.v3` - Configuration parsing

### Ports
- 6380: TCP/RESP Server
- 7946: Cluster Gossip
- 9090: HTTP Admin API

## [0.1.0] - 2026-02-19

### Added
- Initial project structure
- Basic RESP protocol implementation
- Simple key-value store
- Basic TCP server
