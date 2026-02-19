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
- **Lua Scripting**: Full EVAL/EVALSHA/SCRIPT support with gopher-lua
- **All Redis Data Types**: String, Hash, List, Set, SortedSet, Stream, Geo, Bitmap, HyperLogLog

## Quick Start

```bash
# Build
go build -o cachestorm ./cmd/cachestorm

# Run
./cachestorm

# Or with custom config
./cachestorm -config config.yaml -port 6380
```

## Commands (180+)

### String Commands
`SET`, `GET`, `DEL`, `EXISTS`, `MSET`, `MGET`, `INCR`, `DECR`, `INCRBY`, `DECRBY`, `INCRBYFLOAT`, `APPEND`, `STRLEN`, `GETRANGE`, `SETRANGE`, `SETNX`, `GETSET`, `GETEX`, `GETDEL`, `SUBSTR`, `LCS`

### Hash Commands
`HSET`, `HGET`, `HMSET`, `HMGET`, `HGETALL`, `HDEL`, `HEXISTS`, `HLEN`, `HKEYS`, `HVALS`, `HINCRBY`, `HINCRBYFLOAT`, `HSETNX`, `HSTRLEN`, `HRANDFIELD`, `HGETDEL`, `HGETEX`, `HSCAN`

### List Commands
`LPUSH`, `RPUSH`, `LPUSHX`, `RPUSHX`, `LPOP`, `RPOP`, `LLEN`, `LRANGE`, `LINDEX`, `LSET`, `LREM`, `LTRIM`, `RPOPLPUSH`, `LINSERT`, `LMOVE`, `BLPOP`, `BRPOP`, `BRPOPLPUSH`, `LPOS`, `LMPOP`, `LMPUSH`

### Set Commands
`SADD`, `SREM`, `SMEMBERS`, `SISMEMBER`, `SCARD`, `SPOP`, `SRANDMEMBER`, `SMOVE`, `SUNION`, `SINTER`, `SDIFF`, `SUNIONSTORE`, `SINTERSTORE`, `SDIFFSTORE`, `SSCAN`

### Sorted Set Commands
`ZADD`, `ZCARD`, `ZCOUNT`, `ZRANGE`, `ZRANGEBYSCORE`, `ZRANK`, `ZREM`, `ZSCORE`, `ZINCRBY`, `ZREVRANGE`, `ZREVRANK`, `ZREMRANGEBYRANK`, `ZREMRANGEBYSCORE`, `ZPOPMIN`, `ZPOPMAX`, `ZRANDMEMBER`, `ZMSCORE`, `ZUNIONSTORE`, `ZINTERSTORE`, `ZDIFFSTORE`, `ZSCAN`

### Stream Commands
`XADD`, `XLEN`, `XRANGE`, `XREVRANGE`, `XREAD`, `XDEL`, `XTRIM`, `XINFO`, `XGROUP`, `XREADGROUP`, `XACK`, `XPENDING`, `XCLAIM`

### Geo Commands
`GEOADD`, `GEODIST`, `GEOHASH`, `GEOPOS`, `GEORADIUS`, `GEORADIUSBYMEMBER`

### Bitmap Commands
`SETBIT`, `GETBIT`, `BITCOUNT`, `BITPOS`, `BITOP`, `BITFIELD`

### HyperLogLog Commands
`PFADD`, `PFCOUNT`, `PFMERGE`

### Key Commands
`EXPIRE`, `PEXPIRE`, `EXPIREAT`, `PEXPIREAT`, `TTL`, `PTTL`, `PERSIST`, `TYPE`, `RENAME`, `RENAMENX`, `KEYS`, `SCAN`, `RANDOMKEY`, `UNLINK`, `TOUCH`, `DUMP`, `RESTORE`, `COPY`

### Server Commands
`PING`, `ECHO`, `QUIT`, `COMMAND`, `INFO`, `DBSIZE`, `FLUSHDB`, `FLUSHALL`, `TIME`, `AUTH`, `HOTKEYS`, `MEMINFO`, `SORT`, `SORT_RO`, `SLOWLOG`, `WAIT`, `ROLE`, `LASTSAVE`, `LOLWUT`, `SHUTDOWN`, `SAVE`, `BGSAVE`, `BGREWRITEAOF`, `SLAVEOF`, `REPLICAOF`, `LATENCY`, `STRALGO`, `MODULE`, `ACL`, `MONITOR`, `SWAPDB`, `SYNC`, `PSYNC`

### Transaction Commands
`MULTI`, `EXEC`, `DISCARD`, `WATCH`, `UNWATCH`

### Pub/Sub Commands
`SUBSCRIBE`, `UNSUBSCRIBE`, `PUBLISH`, `PSUBSCRIBE`, `PUNSUBSCRIBE`, `PUBSUB`

### Scripting Commands
`EVAL`, `EVALSHA`, `SCRIPT LOAD/EXISTS/FLUSH`

### Tag Commands (Killer Feature!)
```
SETTAG key value tag1 tag2 ...   # Set key with tags
TAGKEYS tag                       # Get all keys for a tag
TAGCOUNT tag                      # Count keys in a tag
TAGDEL tag                        # Delete tag
TAGINFO tag                       # Get tag info
INVALIDATE tag                    # Delete all keys with tag
```

### Namespace Commands
`NAMESPACES`, `NSCREATE`, `NSDEL`, `NSINFO`, `NSKEYS`

### Cluster Commands
`CLUSTER INFO`, `CLUSTER NODES`, `CLUSTER SLOTS`, `MIGRATE`, `ASKING`, `READONLY`, `READWRITE`

### Debug Commands
`DEBUG`, `OBJECT`, `MEMORY`

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
```

## Lua Scripting Example

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

## Performance

Benchmark results (AMD Ryzen 7 PRO 6850H, Windows):

```
GET:            72 ns/op (~14M ops/sec)
GET Parallel:   14 ns/op (~70M ops/sec)
SET:            750 ns/op (~1.3M ops/sec)
SET Parallel:   65 ns/op (~15M ops/sec)
TAGCOUNT:       19 ns/op
Delete:         997 ns/op
```

## Project Statistics

| Metric | Value |
|--------|-------|
| Go files | 64 |
| Lines of code | 11,255 |
| Commands | 180+ |
| Data types | 9 |

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
│  │  219 handlers registered      │  │
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
