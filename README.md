# CacheStorm

A high-performance, Redis-compatible in-memory cache server written in Go.

## Features

- **Redis Compatible**: Works with any Redis client (ioredis, go-redis, redis-py, jedis, redis-cli)
- **Tag-based Invalidation**: Native tag-based cache invalidation (killer feature!)
- **Zero Core Dependencies**: Core functionality implemented from scratch
- **High Performance**: ~10M+ ops/sec for GET, ~1M+ ops/sec for SET
- **256-Shard Architecture**: Concurrent access with minimal lock contention
- **Plugin System**: Extensible via Go interfaces
- **Named Namespaces**: Instead of numbered databases
- **Hot Key Detection**: Built-in hot key tracking
- **Multi-node Clustering**: Gossip-based cluster with hash slot routing

## Quick Start

```bash
# Build
make build

# Run
./bin/cachestorm

# Or with Docker
docker run -p 6380:6380 cachestorm/cachestorm:latest
```

## Commands

### String Commands
`SET`, `GET`, `DEL`, `MSET`, `MGET`, `INCR`, `DECR`, `INCRBY`, `DECRBY`, `APPEND`, `STRLEN`, `GETRANGE`, `SETRANGE`, `SETNX`, `GETSET`, `GETDEL`

### Hash Commands
`HSET`, `HGET`, `HMSET`, `HMGET`, `HGETALL`, `HDEL`, `HEXISTS`, `HLEN`, `HKEYS`, `HVALS`, `HINCRBY`, `HINCRBYFLOAT`, `HSETNX`, `HSTRLEN`

### List Commands
`LPUSH`, `RPUSH`, `LPUSHX`, `RPUSHX`, `LPOP`, `RPOP`, `LLEN`, `LRANGE`, `LINDEX`, `LSET`, `LREM`, `LTRIM`, `RPOPLPUSH`

### Set Commands
`SADD`, `SREM`, `SMEMBERS`, `SISMEMBER`, `SCARD`, `SPOP`, `SRANDMEMBER`, `SMOVE`, `SUNION`, `SINTER`, `SDIFF`, `SUNIONSTORE`, `SINTERSTORE`, `SDIFFSTORE`

### Key Commands
`EXPIRE`, `PEXPIRE`, `EXPIREAT`, `PEXPIREAT`, `TTL`, `PTTL`, `PERSIST`, `TYPE`, `RENAME`, `RENAMENX`, `KEYS`, `SCAN`, `RANDOMKEY`, `UNLINK`

### Tag Commands (Killer Feature!)
```
SETTAG key value tag1 tag2 ...   # Set key with tags
TAGS key                          # Get tags for key
ADDTAG key tag ...               # Add tags to existing key
REMTAG key tag ...               # Remove tags from key
INVALIDATE tag [CASCADE]         # Delete all keys with tag (and children if CASCADE)
TAGKEYS tag                       # List all keys for a tag
TAGCOUNT tag                      # Count keys in a tag
TAGLINK parent child             # Create tag hierarchy
TAGUNLINK parent child           # Remove tag hierarchy link
TAGCHILDREN tag                   # List child tags
```

### Server Commands
`PING`, `ECHO`, `QUIT`, `COMMAND`, `INFO`, `DBSIZE`, `FLUSHDB`, `FLUSHALL`, `TIME`, `AUTH`, `HOTKEYS`, `MEMINFO`

### Namespace Commands
`NAMESPACE`, `NAMESPACES`, `NAMESPACEDEL`, `NAMESPACEINFO`, `SELECT`

### Cluster Commands
`CLUSTER INFO`, `CLUSTER NODES`, `CLUSTER SLOTS`

### Client Commands
`CLIENT LIST`, `CLIENT SETNAME`, `CLIENT GETNAME`, `CLIENT ID`

## Tag-Based Invalidation Example

```bash
# Set keys with tags
SETTAG user:1 "John Doe" users profile
SETTAG user:2 "Jane" users profile
SETTAG product:1 "Widget" products catalog

# Get all user keys
TAGKEYS users
# Returns: user:1, user:2

# Invalidate all user profiles at once
INVALIDATE users
# Returns: 2 (keys deleted)

# With hierarchy
TAGLINK users admin
SETTAG admin:1 "Admin" admin users

INVALIDATE users CASCADE
# Also invalidates admin:1
```

## Configuration

```yaml
server:
  bind: "0.0.0.0"
  port: 6380
  max_connections: 10000

memory:
  max_memory: "2gb"
  eviction_policy: "allkeys-lru"

logging:
  level: "info"
  format: "console"

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
```

## Ports

- **6380**: Main RESP server
- **7946**: Cluster gossip (when cluster enabled)
- **9090**: HTTP admin API + Prometheus metrics

## HTTP Admin API

```
GET /health      - Health check
GET /info        - Server info
GET /keys        - List keys
GET /tags        - List tags
GET /memory      - Memory info
GET /metrics     - Prometheus metrics
```

## Docker

```bash
# Single node
docker run -p 6380:6380 -p 9090:9090 cachestorm/cachestorm

# 3-node cluster
docker-compose -f docker/docker-compose.yml up
```

## Performance

Benchmark results (AMD Ryzen 7, Windows):

```
GET:            97.80 ns/op (~10M ops/sec)
GET Parallel:   14.17 ns/op (~70M ops/sec)
SET:            735.8 ns/op (~1.4M ops/sec)
SET Parallel:   73.77 ns/op (~13M ops/sec)
TAGCOUNT:       23.65 ns/op
```

## Architecture

```
┌─────────────────────────────────────┐
│          CacheStorm Node            │
│  ┌───────────────────────────────┐  │
│  │     TCP Server (:6380)        │  │
│  └───────────┬───────────────────┘  │
│              │                       │
│  ┌───────────▼───────────────────┐  │
│  │     RESP3 Protocol Layer      │  │
│  └───────────┬───────────────────┘  │
│              │                       │
│  ┌───────────▼───────────────────┐  │
│  │     Command Router            │  │
│  └───────────┬───────────────────┘  │
│              │                       │
│  ┌───────────▼───────────────────┐  │
│  │     256-Shard Store           │  │
│  │  ┌─────────┐ ┌─────────┐      │  │
│  │  │ Tag     │ │ TTL     │      │  │
│  │  │ Index   │ │ Wheel   │      │  │
│  │  └─────────┘ └─────────┘      │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
```

## License

Apache 2.0
