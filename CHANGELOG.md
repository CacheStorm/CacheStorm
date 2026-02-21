# CacheStorm Changelog

All notable changes to this project will be documented in this file.

## [0.1.18] - 2026-02-21

### Added - Integration Commands (71 new commands - Total: 854)

**Circuit Breaker Commands**
- CIRCUITBREAKER.CREATE, STATE, TRIP, RESET

**Rate Limit Commands**
- RATELIMIT.CREATE, CHECK, RESET, DELETE

**Cache Lock Commands**
- CACHE.LOCK, UNLOCK, LOCKED, REFRESH

**Network Commands**
- NET.WHOIS, DNS, PING, PORT

**Array Commands**
- ARRAY.PUSH, POP, SHIFT, UNSHIFT, SLICE, SPLICE
- ARRAY.REVERSE, SORT, UNIQUE, FLATTEN, MERGE
- ARRAY.INTERSECT, DIFF, INDEXOF, LASTINDEXOF, INCLUDES

**Object Commands**
- OBJECT.KEYS, VALUES, ENTRIES, FROMENTRIES
- OBJECT.MERGE, PICK, OMIT, HAS, GET, SET, DELETE

**Math Commands**
- MATH.ADD, SUB, MUL, DIV, MOD, POW, SQRT, ABS
- MATH.MIN, MAX, FLOOR, CEIL, ROUND, RANDOM
- MATH.SUM, AVG, MEDIAN, STDDEV

**Geo Extended Commands**
- GEO.ENCODE, DECODE, DISTANCE, BOUNDINGBOX

**CAPTCHA Commands**
- CAPTCHA.GENERATE, VERIFY

**Sequence Commands**
- SEQUENCE.NEXT, CURRENT, RESET, SET

## [0.1.13] - 2026-02-21

### Added - Workflow, State Machine, Chained and Reactive

**Workflow Commands**
- WORKFLOW.CREATE, DELETE, GET, LIST, START, PAUSE, COMPLETE, FAIL, RESET
- WORKFLOW.NEXT, SETVAR, GETVAR, ADDSTEP

**Template Commands**
- TEMPLATE.CREATE, DELETE, GET, INSTANTIATE

**State Machine Commands**
- STATEM.CREATE, DELETE, ADDSTATE, ADDTRANS, TRIGGER
- STATEM.CURRENT, CANTRIGGER, EVENTS, RESET, ISFINAL, INFO, LIST

**Chained Commands**
- CHAINED.SET, GET, DEL

**Reactive Commands**
- REACTIVE.WATCH, UNWATCH, TRIGGER

## [0.1.12] - 2026-02-21

### Added - Expression, Validation, String manipulation

**Expression Commands**
- EVAL.EXPR, FORMAT, JSONPATH, TEMPLATE
- EVAL.REGEX, REGEXMATCH, REGEXREPLACE

**Validation Commands**
- VALIDATE.EMAIL, URL, IP, JSON
- VALIDATE.INT, FLOAT, ALPHA, ALPHANUM, LENGTH, RANGE

**String Commands**
- STR.FORMAT, TRUNCATE, PADLEFT, PADRIGHT, REVERSE, REPEAT
- STR.SPLIT, JOIN, CONTAINS, STARTSWITH, ENDSWITH
- STR.INDEX, LASTINDEX, REPLACE, TRIM, TRIMLEFT, TRIMRIGHT
- STR.TITLE, WORDS, LINES

## [0.1.11] - 2026-02-21

### Added - Audit Log, Feature Flags, Atomic Counter, Backup

**Audit Commands**
- AUDIT.LOG, GET, GETRANGE, GETBYCMD, GETBYKEY
- AUDIT.CLEAR, COUNT, STATS, ENABLE, DISABLE

**Feature Flag Commands**
- FLAG.CREATE, DELETE, GET, ENABLE, DISABLE, TOGGLE, ISENABLED
- FLAG.LIST, LISTENABLED, ADDVARIANT, GETVARIANT, ADDRULE

**Counter Commands**
- COUNTER.GET, SET, INCR, DECR, INCRBY, DECRBY
- COUNTER.DELETE, LIST, GETALL, RESET, RESETALL

**Backup Commands**
- BACKUP.CREATE, RESTORE, LIST, DELETE

**Memory Commands**
- MEMORY.TRIM, FRAG, PURGE, ALLOC

## [0.1.10] - 2026-02-21

### Added - Events, Webhooks, Compression, Queue, Stack

**Event Commands**
- EVENT.EMIT, GET, LIST, CLEAR

**Webhook Commands**
- WEBHOOK.CREATE, DELETE, GET, LIST, ENABLE, DISABLE, STATS

**Compression Commands**
- COMPRESS/DECOMPRESS RLE, LZ4, CUSTOM

**Queue Commands**
- QUEUE.CREATE, PUSH, POP, PEEK, LEN, CLEAR

**Stack Commands**
- STACK.CREATE, PUSH, POP, PEEK, LEN, CLEAR

## [0.1.9] - 2026-02-21

### Added - Job Scheduler, Circuit Breaker, Session Manager

**Job Commands**
- JOB.CREATE, DELETE, GET, LIST, ENABLE, DISABLE, RUN, STATS, RESET, UPDATE

**Circuit Breaker Commands**
- CIRCUIT.CREATE, DELETE, ALLOW, SUCCESS, FAILURE, STATE, RESET, STATS, LIST

**Session Commands**
- SESSION.CREATE, GET, SET, DEL, EXISTS, TTL, REFRESH, CLEAR
- SESSION.ALL, LIST, COUNT, CLEANUP

## [0.1.8] - 2026-02-21

### Added - Statistical Data Structures

**TDigest Commands**
- TDIGEST.CREATE, ADD, QUANTILE, CDF, MEAN, MIN, MAX, INFO, RESET, MERGE

**Sampler Commands**
- SAMPLE.CREATE, ADD, GET, RESET, INFO

**Histogram Commands**
- HISTOGRAM.CREATE, ADD, GET, MEAN, RESET, INFO

## [0.1.7] - 2026-02-21

### Added - Cache Warming and Key Management

**Cache Warming Commands**
- WARM.PRELOAD, PREFETCH, INVALIDATE, STATUS

**Batch Commands**
- BATCH.GET, SET, DEL, MGET, MSET, MDEL, EXEC, PIPELINE.EXEC

**Key Commands**
- KEY.RENAME, RENAMENX, COPY, MOVE, DUMP, RESTORE
- KEY.OBJECT, ENCODE, FREQ, IDLETIME, REFCOUNT

## [0.1.6] - 2026-02-21

### Added - Monitoring Commands

**Metrics Commands**
- METRICS.GET, RESET, CMD

**SlowLog Commands**
- SLOWLOG.GET, LEN, RESET, CONFIG

**Stats Commands**
- STATS.KEYSPACE, MEMORY, CPU, CLIENTS, ALL

**Health Commands**
- HEALTH.CHECK, LIVENESS, READINESS

## [0.1.5] - 2026-02-21

### Added - Utility Commands

**Rate Limiter Commands**
- RL.CREATE, ALLOW, GET, DELETE, RESET

**Distributed Lock Commands**
- LOCK.TRY, ACQUIRE, RELEASE, RENEW, INFO, ISLOCKED

**ID Generator Commands**
- ID.CREATE, NEXT, NEXTN, CURRENT, SET, DELETE

**Snowflake Commands**
- SNOWFLAKE.NEXT, PARSE

## [0.1.4] - 2026-02-21

### Added - Digest and Crypto Commands

**Digest Commands**
- DIGEST.MD5, SHA1, SHA256, SHA512
- DIGEST.HMAC, HMACMD5, HMACSHA1, HMACSHA256, HMACSHA512
- DIGEST.CRC32, ADLER32
- DIGEST.BASE64ENCODE, BASE64DECODE, HEXENCODE, HEXDECODE

**Crypto Commands**
- CRYPTO.HASH, CRYPTO.HMAC

## [0.1.3] - 2026-02-21

### Added - Probabilistic and Graph Modules

**Bloom Filter Commands**
- BF.ADD, EXISTS, INFO, RESERVE

**Cuckoo Filter Commands**
- CF.ADD, EXISTS, DEL, INFO

**Count-Min Sketch Commands**
- CMS.INCRBY, QUERY, INFO

**Top-K Commands**
- TOPK.ADD, QUERY, LIST, INFO

**Graph Commands**
- GRAPH.CREATE, DELETE, INFO, LIST
- GRAPH.ADDNODE, GETNODE, DELNODE
- GRAPH.ADDEDGE, GETEDGE, DELEDGE
- GRAPH.QUERY, NEIGHBORS

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
