# CacheStorm Changelog

All notable changes to this project will be documented in this file.

## [0.1.2] - 2026-02-21

### Added - Full-text Search Module

**Search Commands**
- FT.CREATE - Create search index with schema
- FT.DROPINDEX - Delete search index
- FT.INFO - Get index information
- FT.SEARCH - Search documents with query
- FT.ADD - Add document to index
- FT.DEL - Delete document from index
- FT.GET - Get document by ID
- FT._LIST - List all indexes
- FT.AGGREGATE - Aggregate search results
- FT.TAGVALS - Get tag values
- FT.ALIASADD, FT.ALIASDEL - Alias management

**Features**
- Inverted index for fast text search
- Field-based indexing
- Scoring and sorting
- LIMIT/OFFSET pagination
- RETURN fields selection
- NOCONTENT option

## [0.1.1] - 2026-02-21

### Added - JSON and Time Series Modules

**JSON Commands**
- JSON.GET, JSON.SET, JSON.DEL, JSON.TYPE
- JSON.NUMINCRBY, JSON.NUMMULTBY
- JSON.STRAPPEND, JSON.STRLEN
- JSON.ARRAPPEND, JSON.ARRLEN
- JSON.OBJLEN, JSON.OBJKEYS
- JSON.MGET, JSON.MSET
- JSON path support ($.field, $.array[0])

**Time Series Commands**
- TS.CREATE, TS.DEL, TS.ALTER
- TS.ADD, TS.MADD
- TS.RANGE, TS.REVRANGE
- TS.GET, TS.INFO
- TS.QUERYINDEX
- TS.INCRBY, TS.DECRBY
- Aggregation (avg, sum, min, max, count)
- Label-based querying

## [0.1.0] - 2026-02-20

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

#### High Availability
- Sentinel support for monitoring and failover
- Master/slave replication with RDB sync
- Cluster mode with gossip protocol
- Automatic failover and rebalancing

#### Persistence
- RDB save/load
- AOF append-only file with rewrite support
- Configurable sync policies

#### Proprietary Extensions
- CACHE.BULKGET, BULKDEL, STATS, PREFETCH, EXPORT, CLEAR
- Tag-based invalidation system
- Namespace support

#### Performance
- Connection pooling
- Batch processing
- Buffer pools for memory reuse

#### Extensibility
- Module system for custom commands
- Plugin architecture

### Test Coverage
- 40+ store tests
- 15+ cluster tests
- 20+ command tests
- Lua scripting tests
