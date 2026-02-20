<div align="center">
  <img src="https://avatars.githubusercontent.com/u/262622049?s=400&u=a2e56c80726cb8a3ae6fc8f8622be5173b7b2848&v=4" alt="CacheStorm Logo" width="180" height="180">
  
  # CacheStorm
  
  **High-Performance, Redis-Compatible In-Memory Cache**
  
  [![Go Version](https://img.shields.io/badge/Go-1.21%2B-00ADD8?style=flat&logo=go)](https://golang.org)
  [![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
  [![Redis Compatible](https://img.shields.io/badge/Redis-Compatible-DC382D?style=flat&logo=redis)](https://redis.io)
  [![Performance](https://img.shields.io/badge/Performance-77M%20ops%2Fsec-blue)](benchmarks/)
</div>

---

A high-performance, Redis-compatible in-memory cache server written in Go with **tag-based cache invalidation** as the killer feature.

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Configuration](#configuration)
- [Admin UI](#admin-ui)
- [Commands](#commands)
- [Data Types](#data-types)
- [Lua Scripting](#lua-scripting)
- [Tag-Based Invalidation](#tag-based-invalidation)
- [HTTP API](#http-api)
- [Performance](#performance)
- [Docker](#docker)
- [Architecture](#architecture)
- [Testing](#testing)
- [License](#license)

## Features

### Core Features
- **Redis Compatible**: Works with any Redis client (ioredis, go-redis, redis-py, jedis, redis-cli)
- **RESP3 Protocol**: Full Redis Serialization Protocol 3 support
- **180+ Commands**: Comprehensive Redis command coverage
- **9 Data Types**: String, Hash, List, Set, SortedSet, Stream, Geo, Bitmap, HyperLogLog

### Unique Features
- **Tag-Based Invalidation**: Native tag-based cache invalidation (killer feature!)
- **Named Namespaces**: Human-readable namespaces instead of numbered databases
- **Hot Key Detection**: Built-in hot key tracking and analysis

### Performance
- **High Performance**: ~14M GET/sec, ~1.5M SET/sec (single thread)
- **Parallel Performance**: ~77M GET/sec, ~15M SET/sec (parallel)
- **256-Shard Architecture**: Concurrent access with minimal lock contention
- **Zero Core Dependencies**: Core functionality implemented from scratch

### Enterprise Features
- **Lua Scripting**: Full EVAL/EVALSHA/SCRIPT support with gopher-lua
- **Transactions**: MULTI/EXEC/DISCARD/WATCH support
- **Pub/Sub**: Subscribe, Publish, Pattern Subscribe
- **Multi-node Clustering**: Gossip-based cluster with hash slot routing
- **Persistence**: AOF (Append-Only File) + RDB Snapshot
- **Eviction Policies**: LRU, LFU, TTL-based eviction
- **Replication**: Master-Slave replication support
- **Access Control**: ACL (Access Control List) support
- **Monitoring**: Slow Log, Latency monitoring, Hot key detection

## Quick Start

```bash
# Clone and build
git clone https://github.com/cachestorm/cachestorm
cd cachestorm
go build -o cachestorm ./cmd/cachestorm

# Run with default settings
./cachestorm

# Run with custom config
./cachestorm -config config.yaml -port 6380

# Test with redis-cli
redis-cli -p 6380 PING
```

## Installation

### From Source
```bash
go build -o cachestorm ./cmd/cachestorm
```

### Using Docker
```bash
docker pull cachestorm/cachestorm:latest
docker run -p 6380:6380 -p 9090:9090 cachestorm/cachestorm
```

### Using Docker Compose
```bash
docker-compose -f docker/docker-compose.yml up -d
```

## Configuration

### Command Line Options
```bash
./cachestorm -port 6380 -bind 0.0.0.0 -config config.yaml
```

### Configuration File (config.yaml)
```yaml
server:
  bind: "0.0.0.0"
  port: 6380
  max_connections: 10000
  read_timeout: "30s"
  write_timeout: "30s"

http:
  enabled: true
  port: 8080
  password: ""  # Optional: set to protect admin UI

memory:
  max_memory: "2gb"
  eviction_policy: "allkeys-lru"  # allkeys-lru, allkeys-lfu, volatile-lru, volatile-lfu, volatile-ttl

logging:
  level: "info"      # debug, info, warn, error
  format: "console"  # console, json

persistence:
  enabled: true
  aof: true
  aof_fsync: "everysec"  # always, everysec, no
  rdb: true
  rdb_interval: "5m"

plugins:
  stats:
    enabled: true
  metrics:
    enabled: true
    port: 9090
  auth:
    enabled: false
    password: "secret"
  slowlog:
    enabled: true
    threshold: "10ms"
    max_entries: 128

cluster:
  enabled: false
  gossip_port: 7946
  seeds: []
```

## Admin UI

CacheStorm includes a modern web-based admin interface for monitoring and management.

### Features
- **Dashboard**: Real-time metrics (keys, memory, tags, uptime)
- **Keys Browser**: Search, view, add, and delete keys
- **Tags Management**: View tags and invalidate cached data
- **Namespaces**: Manage multiple namespaces
- **Cluster View**: Monitor cluster nodes and join new nodes
- **Console**: Execute Redis commands directly
- **Slow Log**: View slow queries

### Access
```bash
# Start server (HTTP admin on port 8080)
./cachestorm

# Open in browser
http://localhost:8080
```

### Password Protection
```yaml
http:
  enabled: true
  port: 8080
  password: "your-secret-password"
```

When a password is set, the admin UI will show a login screen.

### Screenshots

**Dashboard**
```
┌─────────────────────────────────────────────────────────────┐
│  CacheStorm Admin                                           │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          │
│  │ Keys    │ │ Memory  │ │ Tags    │ │ Uptime  │          │
│  │ 12,345  │ │ 256 MB  │ │ 127     │ │ 2d 4h   │          │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘          │
│                                                             │
│  Recent Activity              Top Tags                     │
│  ─────────────────           ─────────                     │
│  ● SET user:1                 user:*  ========  4521      │
│  ● INCR counter               cache:* ======    3212      │
│  ● TAGKEYS session            sess:*  ====      1892      │
└─────────────────────────────────────────────────────────────┘
```

## Commands

### String Commands (22)
```
SET, GET, DEL, EXISTS, MSET, MGET, INCR, DECR, INCRBY, DECRBY, 
INCRBYFLOAT, APPEND, STRLEN, GETRANGE, SETRANGE, SETNX, GETSET, 
GETEX, GETDEL, SUBSTR, LCS, COPY
```

### Hash Commands (18)
```
HSET, HGET, HMSET, HMGET, HGETALL, HDEL, HEXISTS, HLEN, HKEYS, 
HVALS, HINCRBY, HINCRBYFLOAT, HSETNX, HSTRLEN, HRANDFIELD, 
HGETDEL, HGETEX, HSCAN
```

### List Commands (21)
```
LPUSH, RPUSH, LPUSHX, RPUSHX, LPOP, RPOP, LLEN, LRANGE, LINDEX, 
LSET, LREM, LTRIM, RPOPLPUSH, LINSERT, LMOVE, BLPOP, BRPOP, 
BRPOPLPUSH, LPOS, LMPOP, LMPUSH
```

### Set Commands (15)
```
SADD, SREM, SMEMBERS, SISMEMBER, SCARD, SPOP, SRANDMEMBER, SMOVE, 
SUNION, SINTER, SDIFF, SUNIONSTORE, SINTERSTORE, SDIFFSTORE, SSCAN
```

### Sorted Set Commands (25)
```
ZADD, ZCARD, ZCOUNT, ZRANGE, ZRANGEBYSCORE, ZRANK, ZREM, ZSCORE, 
ZINCRBY, ZREVRANGE, ZREVRANK, ZREMRANGEBYRANK, ZREMRANGEBYSCORE, 
ZPOPMIN, ZPOPMAX, ZRANDMEMBER, ZMSCORE, ZUNIONSTORE, ZINTERSTORE, 
ZDIFFSTORE, ZSCAN, ZLEXCOUNT, ZRANGEBYLEX, ZREMRANGEBYLEX, ZREVRANGEBYSCORE
```

### Stream Commands (13)
```
XADD, XLEN, XRANGE, XREVRANGE, XREAD, XDEL, XTRIM, XINFO, XGROUP, 
XREADGROUP, XACK, XPENDING, XCLAIM
```

### Geo Commands (6)
```
GEOADD, GEODIST, GEOHASH, GEOPOS, GEORADIUS, GEORADIUSBYMEMBER
```

### Bitmap Commands (6)
```
SETBIT, GETBIT, BITCOUNT, BITPOS, BITOP, BITFIELD
```

### HyperLogLog Commands (3)
```
PFADD, PFCOUNT, PFMERGE
```

### Key Commands (17)
```
EXPIRE, PEXPIRE, EXPIREAT, PEXPIREAT, TTL, PTTL, PERSIST, TYPE, 
RENAME, RENAMENX, KEYS, SCAN, RANDOMKEY, UNLINK, TOUCH, DUMP, RESTORE
```

### Server Commands (53)
```
PING, ECHO, QUIT, COMMAND, INFO, DBSIZE, FLUSHDB, FLUSHALL, TIME, 
AUTH, HOTKEYS, MEMINFO, SORT, SORT_RO, SLOWLOG, WAIT, ROLE, LASTSAVE, 
LOLWUT, SHUTDOWN, SAVE, BGSAVE, BGREWRITEAOF, SLAVEOF, REPLICAOF, 
LATENCY, STRALGO, MODULE, ACL, MONITOR, SWAPDB, SYNC, PSYNC, CLIENT, 
CONFIG, DEBUG, OBJECT, MEMORY, etc.
```

### Transaction Commands (5)
```
MULTI, EXEC, DISCARD, WATCH, UNWATCH
```

### Pub/Sub Commands (6)
```
SUBSCRIBE, UNSUBSCRIBE, PUBLISH, PSUBSCRIBE, PUNSUBSCRIBE, PUBSUB
```

### Scripting Commands (3)
```
EVAL, EVALSHA, SCRIPT (LOAD/EXISTS/FLUSH)
```

### Tag Commands (6) - Killer Feature!
```
SETTAG key value [tag1 tag2 ...]  - Set key with tags
TAGKEYS tag                        - Get all keys for a tag
TAGCOUNT tag                       - Count keys in a tag
TAGDEL tag                         - Delete a tag
TAGINFO tag                        - Get tag information
INVALIDATE tag                     - Delete all keys with tag
```

### Namespace Commands (5)
```
NAMESPACES                         - List all namespaces
NSCREATE name                      - Create namespace
NSDEL name                         - Delete namespace
NSINFO name                        - Get namespace info
NSKEYS name                        - List keys in namespace
```

### Cluster Commands (8)
```
CLUSTER INFO, CLUSTER NODES, CLUSTER SLOTS, CLUSTER MEET, 
CLUSTER FORGET, CLUSTER REPLICATE, MIGRATE, ASKING
```

## Data Types

### String
```bash
SET mykey "Hello World"
GET mykey
INCR counter
SETRANGE mykey 6 "Redis"
GETRANGE mykey 0 4
```

### Hash
```bash
HSET user:1 name "John" email "john@example.com"
HGET user:1 name
HGETALL user:1
HINCRBY user:1 visits 1
```

### List
```bash
LPUSH mylist "world"
LPUSH mylist "hello"
LRANGE mylist 0 -1
LPOP mylist
RPOP mylist
```

### Set
```bash
SADD myset "member1" "member2"
SMEMBERS myset
SISMEMBER myset "member1"
SPOP myset
```

### Sorted Set
```bash
ZADD leaderboard 100 "player1"
ZADD leaderboard 200 "player2"
ZRANGE leaderboard 0 -1 WITHSCORES
ZREVRANGE leaderboard 0 0
```

### Stream
```bash
XADD mystream * field1 value1
XLEN mystream
XRANGE mystream - +
XREAD COUNT 10 STREAMS mystream 0
```

### Bitmap
```bash
SETBIT mybitmap 0 1
SETBIT mybitmap 1 0
GETBIT mybitmap 0
BITCOUNT mybitmap
```

### HyperLogLog
```bash
PFADD hll a b c
PFCOUNT hll
PFMERGE hll2 hll
```

### Geo
```bash
GEOADD locations 13.361389 38.115556 "Palermo"
GEODIST locations Palermo Catania
GEOPOS locations Palermo
```

## Lua Scripting

### Basic Usage
```bash
# Simple script
EVAL "return redis.call('GET', KEYS[1])" 1 mykey

# With arguments
EVAL "return redis.call('SET', KEYS[1], ARGV[1])" 1 mykey myvalue

# Cache script
SCRIPT LOAD "return redis.call('INCR', KEYS[1])"
# Returns: sha1 hash

# Execute cached script
EVALSHA <sha1> 1 counter
```

### Available Commands in Lua
```lua
-- String
redis.call('SET', 'key', 'value')
redis.call('GET', 'key')
redis.call('DEL', 'key')
redis.call('EXISTS', 'key')
redis.call('INCR', 'key')
redis.call('DECR', 'key')
redis.call('MSET', 'k1', 'v1', 'k2', 'v2')
redis.call('MGET', 'k1', 'k2')

-- Hash
redis.call('HSET', 'hash', 'field', 'value')
redis.call('HGET', 'hash', 'field')
redis.call('HGETALL', 'hash')
redis.call('HDEL', 'hash', 'field')
redis.call('HEXISTS', 'hash', 'field')
redis.call('HLEN', 'hash')

-- List
redis.call('LPUSH', 'list', 'value')
redis.call('RPUSH', 'list', 'value')
redis.call('LPOP', 'list')
redis.call('RPOP', 'list')
redis.call('LLEN', 'list')
redis.call('LRANGE', 'list', 0, -1)

-- Set
redis.call('SADD', 'set', 'member')
redis.call('SISMEMBER', 'set', 'member')
redis.call('SCARD', 'set')

-- Sorted Set
redis.call('ZADD', 'zset', 1.0, 'member')
redis.call('ZSCORE', 'zset', 'member')
redis.call('ZCARD', 'zset')
redis.call('ZREM', 'zset', 'member')

-- Key
redis.call('EXPIRE', 'key', 3600)
redis.call('TTL', 'key')
redis.call('TYPE', 'key')

-- Server
redis.call('DBSIZE')
redis.call('FLUSHDB')
```

### Example Scripts
```lua
-- Atomic counter with limit
local current = redis.call('GET', KEYS[1])
if not current then
    current = 0
else
    current = tonumber(current)
end
if current >= tonumber(ARGV[1]) then
    return nil
end
return redis.call('INCR', KEYS[1])

-- Cache-aside pattern
local cached = redis.call('GET', KEYS[1])
if cached then
    return cached
end
-- Compute value...
local value = ARGV[1]
redis.call('SET', KEYS[1], value, 'EX', 3600)
return value
```

## Tag-Based Invalidation

Tag-based invalidation allows you to group keys by tags and invalidate all keys in a group with a single command.

### Basic Usage
```bash
# Set keys with tags
SETTAG user:1 "John Doe" users profile
SETTAG user:2 "Jane" users profile
SETTAG product:1 "Widget" products catalog

# Get all user keys
TAGKEYS users
# Returns: user:1, user:2

# Count keys in tag
TAGCOUNT users
# Returns: 2

# Invalidate all user profiles at once
INVALIDATE users
# Returns: 2 (keys deleted)
```

### Use Cases

#### Web Caching
```bash
# Cache user data with user tag
SETTAG user:123:profile "..." user:123
SETTAG user:123:settings "..." user:123

# Invalidate all cached data for user on update
INVALIDATE user:123
```

#### API Response Caching
```bash
# Cache API responses by endpoint
SETTAG api:/products:list "..." api products
SETTAG api:/products:123 "..." api product:123

# Invalidate on product update
INVALIDATE product:123
```

#### Session Management
```bash
# Tag sessions by user
SETTAG session:abc123 "..." session user:456

# Invalidate all sessions for user
INVALIDATE user:456
```

## Performance

### Benchmark Results (AMD Ryzen 7 PRO 6850H, Windows)

| Operation | Latency | Ops/sec | Allocations |
|-----------|---------|---------|-------------|
| GET | 71 ns/op | ~14M | 0 allocs |
| GET (parallel) | 13 ns/op | ~77M | 0 allocs |
| SET | 669 ns/op | ~1.5M | 3 allocs |
| SET (parallel) | 66 ns/op | ~15M | 3 allocs |
| DELETE | 941 ns/op | ~1M | 0 allocs |
| TAGCOUNT | 19 ns/op | ~53M | 0 allocs |
| TAGADD | 1.5 µs/op | ~660K | 2 allocs |

### Running Benchmarks
```bash
go test ./benchmarks/... -bench=. -benchmem
```

## API Reference

### TCP/RESP API
- **Port**: 6380 (default)
- **Protocol**: RESP3 (Redis Serialization Protocol 3)
- **Clients**: Any Redis client works

### HTTP Admin API
- **Port**: 9090 (default)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/info` | GET | Server info |
| `/keys` | GET | List keys |
| `/tags` | GET | List tags |
| `/memory` | GET | Memory info |
| `/metrics` | GET | Prometheus metrics |
| `/stats` | GET | Server statistics |
| `/config` | GET | Current configuration |

### Example HTTP Requests
```bash
# Health check
curl http://localhost:9090/health

# Get server info
curl http://localhost:9090/info

# List all keys
curl http://localhost:9090/keys

# Get memory usage
curl http://localhost:9090/memory

# Prometheus metrics
curl http://localhost:9090/metrics
```

## Docker

### Single Node
```bash
docker run -d \
  --name cachestorm \
  -p 6380:6380 \
  -p 9090:9090 \
  cachestorm/cachestorm:latest
```

### 3-Node Cluster
```bash
# Using docker-compose
docker-compose -f docker/docker-compose.yml up -d

# Scale
docker-compose -f docker/docker-compose.yml up -d --scale cachestorm=3
```

### Custom Configuration
```bash
docker run -d \
  --name cachestorm \
  -p 6380:6380 \
  -p 9090:9090 \
  -v /path/to/config.yaml:/app/config.yaml \
  cachestorm/cachestorm:latest \
  -config /app/config.yaml
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    CacheStorm Node                       │
│                                                          │
│  ┌────────────────────────────────────────────────────┐ │
│  │              TCP Server (:6380)                    │ │
│  │         Accepts RESP3 Protocol Connections         │ │
│  └──────────────────────┬─────────────────────────────┘ │
│                         │                                │
│  ┌──────────────────────▼─────────────────────────────┐ │
│  │            RESP3 Protocol Layer                    │ │
│  │   Reader/Writer for RESP types (String, Array,    │ │
│  │   Integer, Bulk, Null, Map, Set, etc.)            │ │
│  └──────────────────────┬─────────────────────────────┘ │
│                         │                                │
│  ┌──────────────────────▼─────────────────────────────┐ │
│  │              Command Router                        │ │
│  │   219 handlers for 180+ commands                   │ │
│  │   Middleware: Auth, Slowlog, Metrics              │ │
│  └──────────────────────┬─────────────────────────────┘ │
│                         │                                │
│  ┌──────────────────────▼─────────────────────────────┐ │
│  │               256-Shard Store                      │ │
│  │   ┌─────────────┐  ┌──────────────────────────┐   │ │
│  │   │  Tag Index  │  │   4-Level Timing Wheel   │   │ │
│  │   │ (Bidirect.) │  │   (for TTL management)   │   │ │
│  │   └─────────────┘  └──────────────────────────┘   │ │
│  │   ┌─────────────┐  ┌──────────────────────────┐   │ │
│  │   │   Memory    │  │    Namespace Manager     │   │ │
│  │   │   Tracker   │  │    (Named databases)     │   │ │
│  │   └─────────────┘  └──────────────────────────┘   │ │
│  │   ┌─────────────┐  ┌──────────────────────────┐   │ │
│  │   │   Pub/Sub   │  │    Lua Script Engine     │   │ │
│  │   │   System    │  │    (gopher-lua)          │   │ │
│  │   └─────────────┘  └──────────────────────────┘   │ │
│  └────────────────────────────────────────────────────┘ │
│                                                          │
│  ┌────────────────────────────────────────────────────┐ │
│  │              HTTP Server (:9090)                   │ │
│  │   Admin API + Prometheus Metrics                  │ │
│  └────────────────────────────────────────────────────┘ │
│                                                          │
│  ┌────────────────────────────────────────────────────┐ │
│  │           Cluster Gossip (:7946)                   │ │
│  │   Multi-node coordination via hash slots          │ │
│  └────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

## Testing

### Run All Tests
```bash
go test ./... -v
```

### Run Specific Package Tests
```bash
go test ./internal/store/... -v
go test ./internal/command/... -v
go test ./internal/resp/... -v
```

### Run Benchmarks
```bash
go test ./benchmarks/... -bench=. -benchmem
```

### Test Coverage
```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Integration Tests
```bash
# Start server
./cachestorm &

# Run integration tests
go test ./tests/... -v

# Or use redis-cli
redis-cli -p 6380 PING
```

## Project Statistics

| Metric | Value |
|--------|-------|
| Go Files | 64 |
| Total Lines | ~12,000 |
| Test Files | 10 |
| Test Lines | ~1,600 |
| Commands | 180+ |
| Data Types | 9 |
| Handlers | 219 |

## Ports

| Port | Service |
|------|---------|
| 6380 | TCP/RESP Server |
| 7946 | Cluster Gossip |
| 9090 | HTTP Admin API |

## Dependencies

### Runtime
- `github.com/rs/zerolog` - Logging
- `github.com/yuin/gopher-lua` - Lua scripting
- `gopkg.in/yaml.v3` - Configuration

### Development
- Go 1.22+
- Make (optional)

## Troubleshooting

### Connection Refused
```bash
# Check if server is running
netstat -an | grep 6380

# Check logs
./cachestorm -log-level debug
```

### Memory Issues
```bash
# Check memory usage
redis-cli -p 6380 INFO memory

# Set memory limit
redis-cli -p 6380 CONFIG SET maxmemory 1gb
```

### Slow Performance
```bash
# Check slow log
redis-cli -p 6380 SLOWLOG GET 10

# Check hot keys
redis-cli -p 6380 HOTKEYS
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing`)
5. Open a Pull Request

## License

Apache 2.0 - See [LICENSE](LICENSE) for details.

## Credits

CacheStorm is inspired by Redis and built from scratch in Go with a focus on tag-based cache invalidation.
