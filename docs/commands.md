# CacheStorm Commands Reference

CacheStorm registers about 1,200 commands across 50+ modules: the Redis 7.4 command set plus CacheStorm extensions. Compatibility with Redis is measured by the differential harness in `tests/compat`.

Every command name in this reference is verified against the registered command
set: 1,238 documented, 0 phantoms, 0 missing.

Counts in the table below are the commands listed in each section; `SET` and `COMMAND` are also repeated
under Tag-Based Invalidation and Command Help but counted only once, under Core Redis.

## Command Categories

| Category | Commands | Description |
|----------|----------|-------------|
| [Core Redis](#core-redis-commands) | 208 | Standard Redis commands |
| [JSON](#json-commands) | 13 | JSON document operations |
| [Time Series](#time-series-commands) | 12 | Time-series data |
| [Search](#search-commands) | 13 | Full-text search |
| [Graph](#graph-commands) | 12 | Graph database |
| [Probabilistic](#probabilistic-commands) | 19 | Bloom filters, sketches |
| [Resilience](#resilience-commands) | 21 | Circuit breakers, rate limits |
| [Data Processing](#data-processing-commands) | 15 | Aggregation, joins |
| [Machine Learning](#machine-learning-commands) | 13 | Models, embeddings |
| [Namespace](#namespace-commands) | 1 | Multi-tenant namespaces |
| [Tag Management](#tag-based-invalidation-commands) | 3 | Tag-based cache invalidation |
| [Workflow](#workflow-commands) | 12 | State machines |
| [Advanced](#advanced-commands) | 48 | Advanced data structures |
| [Encoding](#encoding-commands) | 31 | Data encoding/decoding |
| [Digest](#digest-commands) | 17 | Hashing and crypto |
| [Events](#event-commands) | 21 | Event sourcing |
| [Utilities](#utility-commands) | 54 | Various utilities |
| [Integration](#integration-commands) | 66 | External integrations |
| [String Processing](#string-processing-commands) | 20 | String analysis and text manipulation |
| [Sessions](#session-commands) | 13 | Server-side session storage |
| [CRDT](#crdt-commands) | 14 | Conflict-free replicated data types |
| [Validation](#validation-commands) | 14 | Reusable value validation rules |
| [Key Management](#key-management-commands) | 6 | Key lifecycle operations |
| [Batch](#batch-commands) | 7 | Grouped operation submission |
| [Vector](#vector-commands) | 10 | Vector embeddings and similarity search |
| [T-Digest](#t-digest-commands) | 15 | Streaming quantile sketches |
| [Message Queue](#message-queue-commands) | 10 | Queue with acknowledgements and dead letters |
| [Documents](#document-commands) | 10 | Document storage and queries |
| [Analytics](#analytics-commands) | 9 | Counters and aggregates |
| [Circuit Breakers](#circuit-breaker-commands) | 9 | Failure/success state tracking |
| [Service Discovery](#service-discovery-commands) | 8 | Service registration and discovery |
| [Profiles](#profile-commands) | 5 | User profile records |
| [Extended families](#extended-command-families) | 509 | 58 unprefixed commands and 101 further families |
| **Total registered** | **1,238** | |

---

## Core Redis Commands

### String Commands

```
SET key value [EX seconds|PX milliseconds|EXAT timestamp|PXAT milliseconds-timestamp|KEEPTTL] [NX|XX] [GET] [TAGS tag ...]
GET key
DEL key [key ...]
EXISTS key [key ...]
EXPIRE key seconds [NX|XX|GT|LT]
TTL key
PERSIST key
INCR key
DECR key
INCRBY key increment
DECRBY key decrement
APPEND key value
GETRANGE key start end
SETRANGE key offset value
STRLEN key
MSET key value [key value ...]
MGET key [key ...]
SETNX key value
SETEX key seconds value
PSETEX key milliseconds value
GETSET key value
```

### Hash Commands

```
HSET key field value [field value ...]
HGET key field
HMSET key field value [field value ...]
HMGET key field [field ...]
HGETALL key
HDEL key field [field ...]
HEXISTS key field
HLEN key
HKEYS key
HVALS key
HINCRBY key field increment
HINCRBYFLOAT key field increment
HSETNX key field value
HSTRLEN key field
HRANDFIELD key [count [WITHVALUES]]
HEXPIRE key seconds [NX|XX|GT|LT] FIELDS numfields field [field ...]
HPEXPIRE key milliseconds [NX|XX|GT|LT] FIELDS numfields field [field ...]
HEXPIREAT key unix-time-seconds [NX|XX|GT|LT] FIELDS numfields field [field ...]
HPEXPIREAT key unix-time-milliseconds [NX|XX|GT|LT] FIELDS numfields field [field ...]
HTTL key FIELDS numfields field [field ...]
HPTTL key FIELDS numfields field [field ...]
HEXPIRETIME key FIELDS numfields field [field ...]
HPEXPIRETIME key FIELDS numfields field [field ...]
HPERSIST key FIELDS numfields field [field ...]
HGETEX key [EX seconds|PX milliseconds|EXAT unix-time-seconds|PXAT unix-time-milliseconds|PERSIST] FIELDS numfields field [field ...]
HSETEX key [FNX|FXX] [EX seconds|PX milliseconds|EXAT unix-time-seconds|PXAT unix-time-milliseconds|KEEPTTL] FIELDS numfields field value [field value ...]
```

Hash fields can expire on their own (Redis 7.4 hash field expiration).
HEXPIRE and friends reply per field: `-2` no such field, `0` the NX/XX/GT/LT
condition was not met, `1` the expiry was set, `2` the field was deleted
because the time is already past. HTTL/HPTTL/HEXPIRETIME/HPEXPIRETIME reply
`-2` for a missing field and `-1` for a field without expiry; HPERSIST replies
`-2`, `-1` or `1`. An expired field is never returned; it is removed on access
or by the background expiry, and the key goes away with its last field. HSET,
HMSET and HSETEX without KEEPTTL clear a field's expiry; HINCRBY,
HINCRBYFLOAT and HSETNX keep it. HGETEX and HSETEX follow Redis 8.

### List Commands

```
LPUSH key element [element ...]
RPUSH key element [element ...]
LPOP key [count]
RPOP key [count]
LLEN key
LRANGE key start stop
LINDEX key index
LSET key index element
LREM key count element
LTRIM key start stop
LINSERT key BEFORE|AFTER pivot element
BLPOP key [key ...] timeout
BRPOP key [key ...] timeout
RPOPLPUSH source destination
BRPOPLPUSH source destination timeout
LMOVE source destination LEFT|RIGHT LEFT|RIGHT
BLMOVE source destination LEFT|RIGHT LEFT|RIGHT timeout
LPOS key element [RANK rank] [COUNT num-matches] [MAXLEN len]
```

### Set Commands

```
SADD key member [member ...]
SREM key member [member ...]
SMEMBERS key
SISMEMBER key member
SCARD key
SPOP key [count]
SRANDMEMBER key [count]
SINTER key [key ...]
SINTERSTORE destination key [key ...]
SUNION key [key ...]
SUNIONSTORE destination key [key ...]
SDIFF key [key ...]
SDIFFSTORE destination key [key ...]
SMOVE source destination member
SSCAN key cursor [MATCH pattern] [COUNT count]
```

### Sorted Set Commands

```
ZADD key [NX|XX] [GT|LT] [CH] [INCR] score member [score member ...]
ZREM key member [member ...]
ZSCORE key member
ZINCRBY key increment member
ZCARD key
ZRANGE key start stop [WITHSCORES] [BYSCORE|BYLEX] [REV] [LIMIT offset count]
ZREVRANGE key start stop [WITHSCORES]
ZRANGEBYSCORE key min max [WITHSCORES] [LIMIT offset count]
ZREVRANGEBYSCORE key max min [WITHSCORES] [LIMIT offset count]
ZRANGEBYLEX key min max [LIMIT offset count]
ZREVRANGEBYLEX key max min [LIMIT offset count]
ZCOUNT key min max
ZLEXCOUNT key min max
ZREMRANGEBYSCORE key min max
ZREMRANGEBYLEX key min max
ZPOPMIN key [count]
ZPOPMAX key [count]
BZPOPMIN key [key ...] timeout
BZPOPMAX key [key ...] timeout
ZMPOP numkeys key [key ...] MIN|MAX [COUNT count]
ZMSCORE key member [member ...]
ZRANK key member [WITHSCORE]
ZREVRANK key member [WITHSCORE]
ZDIFF numkeys key [key ...] [WITHSCORES]
ZDIFFSTORE destination numkeys key [key ...]
ZINTER numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX] [WITHSCORES]
ZINTERSTORE destination numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX]
ZINTERCARD numkeys key [key ...] [LIMIT limit]
ZUNION numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX] [WITHSCORES]
ZUNIONSTORE destination numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX]
ZRANDMEMBER key [count [WITHSCORES]]
ZSCAN key cursor [MATCH pattern] [COUNT count]
```

### Bitmap Commands

```
SETBIT key offset value
GETBIT key offset
BITCOUNT key [start end [BYTE|BIT]]
BITPOS key bit [start [end [BYTE|BIT]]]
BITOP operation destkey key [key ...]
BITFIELD key [GET type offset] [SET type offset value] [INCRBY type offset increment] [OVERFLOW WRAP|SAT|FAIL]
BITFIELD_RO key [GET type offset ...]
```

### HyperLogLog Commands

```
PFADD key [element ...]
PFCOUNT key [key ...]
PFMERGE destkey [sourcekey ...]
PFDEBUG GETREG|ENCODING|TODENSE|DECODE key
PFSELFTEST
```

### Geo Commands

```
GEOADD key [NX|XX] [CH] longitude latitude member [longitude latitude member ...]
GEODIST key member1 member2 [M|KM|FT|MI]
GEOHASH key member [member ...]
GEOPOS key member [member ...]
GEORADIUS key longitude latitude radius M|KM|FT|MI [WITHCOORD] [WITHDIST] [WITHHASH] [COUNT count [ANY]] [ASC|DESC] [STORE key] [STOREDIST key]
GEORADIUSBYMEMBER key member radius M|KM|FT|MI [WITHCOORD] [WITHDIST] [WITHHASH] [COUNT count [ANY]] [ASC|DESC] [STORE key] [STOREDIST key]
GEORADIUS_RO key longitude latitude radius M|KM|FT|MI [WITHCOORD] [WITHDIST] [WITHHASH] [COUNT count [ANY]] [ASC|DESC]
GEORADIUSBYMEMBER_RO key member radius M|KM|FT|MI [WITHCOORD] [WITHDIST] [WITHHASH] [COUNT count [ANY]] [ASC|DESC]
GEOSEARCH key [FROMMEMBER m] [FROMLONLAT lon lat] [BYRADIUS rad M|KM|FT|MI] [BYBOX w h M|KM|FT|MI] [ASC|DESC] [COUNT count [ANY]] [WITHCOORD] [WITHDIST] [WITHHASH]
GEOSEARCHSTORE destination source [FROMMEMBER m] [FROMLONLAT lon lat] [BYRADIUS rad M|KM|FT|MI] [BYBOX w h M|KM|FT|MI] [ASC|DESC] [COUNT count [ANY]] [STOREDIST]
```

### Stream Commands

```
XADD key [NOMKSTREAM] [MAXLEN|MINID [=|~] threshold [LIMIT count]] id field value [field value ...]
XDEL key id [id ...]
XLEN key
XRANGE key start end [COUNT count]
XREVRANGE key end start [COUNT count]
XREAD [COUNT count] [BLOCK milliseconds] STREAMS key [key ...] id [id ...]
XGROUP CREATE key group id|$ [MKSTREAM]
XGROUP CREATECONSUMER key group consumer
XGROUP DELCONSUMER key group consumer
XGROUP DESTROY key group
XGROUP SETID key group id|$
XREADGROUP GROUP group consumer [COUNT count] [BLOCK milliseconds] [NOACK] STREAMS key [key ...] id [id ...]
XACK key group id [id ...]
XCLAIM key group consumer min-idle-time id [id ...] [IDLE ms] [TIME ms-unix-time] [RETRYCOUNT count] [FORCE] [JUSTID]
XAUTOCLAIM key group consumer min-idle-time start [COUNT count] [JUSTID]
XPENDING key group [[IDLE min-idle-time] start end count [consumer]]
XTRIM key MAXLEN|MINID [=|~] threshold [LIMIT count]
XINFO STREAM key [FULL [COUNT count]]
XINFO GROUPS key
XINFO CONSUMERS key group
XSETID key id|$ [ENTRIESADDED entries-added] [MAXDELETEDID max-deleted-id]
```

### Key Commands

```
KEYS pattern
SCAN cursor [MATCH pattern] [COUNT count] [TYPE type]
TYPE key
DUMP key
RESTORE key ttl serialized-value [REPLACE] [ABSTTL] [IDLETIME seconds] [FREQ frequency]
RESTORE-ASKING key ttl serialized-value [REPLACE] [ABSTTL] [IDLETIME seconds] [FREQ frequency]
EXPIRETIME key
PEXPIRETIME key
EXPIRE key seconds [NX|XX|GT|LT]
PEXPIRE key milliseconds [NX|XX|GT|LT]
RENAME key newkey
RENAMENX key newkey
MOVE key db
COPY source destination [DB db] [REPLACE]
OBJECT subcommand key [arguments ...]
TOUCH key [key ...]
UNLINK key [key ...]
WAIT numreplicas timeout
WAITAOF numlocal numreplicas [timeout]
RANDOMKEY
```

### Transaction Commands

```
MULTI
EXEC
DISCARD
WATCH key [key ...]
UNWATCH
```

### Pub/Sub Commands

```
SUBSCRIBE channel [channel ...]
UNSUBSCRIBE [channel [channel ...]]
PSUBSCRIBE pattern [pattern ...]
PUNSUBSCRIBE [pattern [pattern ...]]
PUBLISH channel message
SPUBLISH channel message
SSUBSCRIBE shardchannel [shardchannel ...]
SUNSUBSCRIBE shardchannel [shardchannel ...]
PUBSUB subcommand [argument [argument ...]]
```

### Server Commands

```
BGREWRITEAOF
BGSAVE
SAVE
LASTSAVE
CONFIG GET parameter [parameter ...]
CONFIG SET parameter value [parameter value ...]
CONFIG REWRITE
CONFIG RESETSTAT
DBSIZE
DEBUG OBJECT key
DEBUG SEGFAULT
FLUSHALL [ASYNC|SYNC]
FLUSHDB [ASYNC|SYNC]
INFO [section]
LOLWUT [VERSION version]
MEMORY DOCTOR
MEMORY HELP
MEMORY MALLOC-STATS
MEMORY PURGE
MEMORY STATS
MEMORY USAGE key [SAMPLES count]
MODULE LIST
MODULE LOAD path [arg [arg ...]]
MODULE UNLOAD name
MONITOR
FAILOVER [TO host port [FORCE]] [ABORT] [TIMEOUT milliseconds]
REPLICAOF host port
ROLE
SHUTDOWN [NOSAVE|SAVE] [NOW] [FORCE] [ABORT]
SLAVEOF host port
SLOWLOG subcommand [argument]
SWAPDB index index
SYNC
TIME
COMMAND
COMMAND COUNT
COMMAND DOCS [command-name [command-name ...]]
COMMAND INFO command-name [command-name ...]
COMMAND LIST
LATENCY DOCTOR
LATENCY GRAPH event
LATENCY HELP
LATENCY HISTORY event
LATENCY LATEST
LATENCY RESET [event [event ...]]
ACL LIST
ACL USERS
ACL GETUSER username
ACL SETUSER username [rule [rule ...]]
ACL DELUSER username [username ...]
ACL CAT [category]
ACL GENPASS [bits]
ACL WHOAMI
ACL LOG [count|RESET]
ACL HELP
ACL LOAD
ACL SAVE
```

---

## JSON Commands

These follow RedisJSON, the JSON module built into Redis 8, and were checked against Redis 8.2. A document is a key of type `ReJSON-RL`, saved, replicated and expired like any key.

A path that starts with `$` is JSONPath: `.key`, `['key']`, `["a","b"]`, `*`, `..` (descendants), `[n]` and negative indexes, `[n,m]`, slices `[start:end:step]`, and filters such as `[?(@.price < 10 && @.tag == 'x')]` with `==`, `!=`, `<`, `<=`, `>`, `>=`, `=~` (regular expression), `&&` and `||`. Such a path answers per match: JSON.GET returns a JSON array of the matches, other commands an array with one reply per match (nil for a match of the wrong type). Any other path is the legacy syntax (`.a.b[0]`, `a.b`, `.` for the document) and answers for its first match, with an error when nothing matches.

Documents keep their keys in insertion order; deleting a key moves the last key into its place, as RedisJSON does. Integers and floats stay distinct (`1` and `1.0`), integer arithmetic wraps at 64 bits like RedisJSON's, and floats print as RedisJSON prints them (`1e16`, `0.000015`, `1e-6`). Values nest at most 128 levels.

```
JSON.SET key path value [NX|XX]
JSON.GET key [INDENT indent] [NEWLINE newline] [SPACE space] [path [path ...]]
JSON.MGET key [key ...] path
JSON.MSET key path value [key path value ...]
JSON.MERGE key path value                # RFC 7386 merge patch
JSON.DEL key [path]
JSON.FORGET key [path]
JSON.TYPE key [path]
JSON.NUMINCRBY key path number
JSON.NUMMULTBY key path number
JSON.STRAPPEND key [path] value
JSON.STRLEN key [path]
JSON.ARRAPPEND key path value [value ...]
JSON.ARRINDEX key path value [start [stop]]
JSON.ARRINSERT key path index value [value ...]
JSON.ARRLEN key [path]
JSON.ARRPOP key [path [index]]
JSON.ARRTRIM key path start stop
JSON.OBJKEYS key [path]
JSON.OBJLEN key [path]
JSON.TOGGLE key path
JSON.CLEAR key [path]
JSON.RESP key [path]
JSON.DEBUG MEMORY key [path] | HELP
```

---

## Time Series Commands

These follow RedisTimeSeries, the time series module built into Redis 8, and were checked against Redis 8.2. A series is a key of type `TSDB-TYPE`: it lives in the selected database, expires, is deleted, renamed and flushed like any key, and is saved in snapshots and the AOF (with its settings and compaction rules) and replicated.

```
TS.CREATE key [RETENTION ms] [ENCODING COMPRESSED|UNCOMPRESSED] [CHUNK_SIZE size] [DUPLICATE_POLICY policy] [IGNORE maxTimeDiff maxValDiff] [LABELS label value ...]
TS.ALTER key [RETENTION ms] [CHUNK_SIZE size] [DUPLICATE_POLICY policy] [IGNORE maxTimeDiff maxValDiff] [LABELS label value ...]
TS.ADD key timestamp|* value [RETENTION ms] [ENCODING ...] [CHUNK_SIZE size] [DUPLICATE_POLICY policy] [ON_DUPLICATE policy] [IGNORE ...] [LABELS ...]
TS.MADD key timestamp value [key timestamp value ...]
TS.INCRBY key addend [TIMESTAMP timestamp] [RETENTION ms] [UNCOMPRESSED] [CHUNK_SIZE size] [LABELS ...]
TS.DECRBY key subtrahend [TIMESTAMP timestamp] [...]
TS.DEL key fromTimestamp toTimestamp
TS.GET key [LATEST]
TS.MGET [LATEST] [WITHLABELS | SELECTED_LABELS label ...] FILTER filter ...
TS.RANGE key from to [LATEST] [FILTER_BY_TS ts ...] [FILTER_BY_VALUE min max] [COUNT count] [[ALIGN align] AGGREGATION aggregator bucketDuration [BUCKETTIMESTAMP -|+|~] [EMPTY]]
TS.REVRANGE key from to [same options]
TS.MRANGE from to [range options] [WITHLABELS | SELECTED_LABELS label ...] FILTER filter ... [GROUPBY label REDUCE reducer]
TS.MREVRANGE from to [same options]
TS.CREATERULE sourceKey destKey AGGREGATION aggregator bucketDuration [alignTimestamp]
TS.DELETERULE sourceKey destKey
TS.INFO key [DEBUG]
TS.QUERYINDEX filter ...
```

Samples stay in timestamp order; an older sample is inserted in place unless it falls outside the retention period. A duplicate timestamp follows the series' `DUPLICATE_POLICY` (`BLOCK` by default; `FIRST`, `LAST`, `MIN`, `MAX`, `SUM`) or `ON_DUPLICATE`. Aggregators are `avg`, `sum`, `min`, `max`, `range`, `count`, `first`, `last`, `std.p`, `std.s`, `var.p`, `var.s` and `twa`. A compaction rule aggregates the source's samples per bucket into the destination when a sample of a later bucket arrives. Filters are `label=value`, `label!=value`, `label=`, `label!=` and `label=(v1,v2)`; at least one must be a `label=value` matcher, and `TS.MGET`, `TS.MRANGE` and `TS.QUERYINDEX` scan the database's keys. Sample values reply as simple strings in RedisTimeSeries' format (`1.5`, `5E-1`, `1E16`), in RESP3 as well.

---

## Search Commands

```
FT.CREATE index [ON HASH|JSON] [PREFIX count prefix [prefix ...]] [SCHEMA field type [field type ...]]
FT.DROPINDEX index [DD]
FT.ADD index docId score [NOSAVE] [REPLACE] [PARTIAL] [LANGUAGE lang] [PAYLOAD payload] [FIELDS field value [field value ...]]
FT.DEL index docId [DD]
FT.GET index docId
FT.SEARCH index query [NOCONTENT] [VERBATIM] [NOSTOPWORDS] [WITHSCORES] [WITHPAYLOADS] [WITHSORTKEYS] [FILTER numericField min max [FILTER numericField min max ...]] [GEOFILTER geoField lon lat radius M|KM|MI|FT] [INKEYS count key [key ...]] [INFIELDS count field [field ...]] [RETURN count field [field ...]] [SUMMARIZE [FIELDS count field [field ...]] [FRAGS num] [LEN fragsize] [SEPARATOR separator]] [HIGHLIGHT [FIELDS count field [field ...]] [TAGS open close]] [SLOP slop] [TIMEOUT timeout] [LIMIT offset num]
FT.AGGREGATE index query [VERBATIM] [LOAD count field [field ...]] [GROUPBY nargs property [property ...] [REDUCE func nargs arg [arg ...] [AS name]] [REDUCE ...]] [SORTBY nargs [property [ASC|DESC] [property [ASC|DESC] ...]] [MAX num] [AS name]] [APPLY expr AS name] [FILTER expr] [LIMIT offset num] [QUERY_EXECUTION_TIME] [TIMEOUT timeout]
FT.TAGVALS index field
FT.ALIASADD name index
FT.ALIASDEL name
FT.ALIASUPDATE name index
FT.INFO index
FT._LIST
```

---

## Graph Commands

```
GRAPH.CREATE graphName
GRAPH.LIST
GRAPH.DELETE graphName
GRAPH.INFO graphName
GRAPH.ADDNODE graphName label [key value ...]
GRAPH.GETNODE graphName nodeID
GRAPH.DELNODE graphName nodeID
GRAPH.ADDEDGE graphName fromID toID relation [key value ...]
GRAPH.GETEDGE graphName edgeID
GRAPH.DELEDGE graphName edgeID
GRAPH.NEIGHBORS graphName nodeID [relation]
GRAPH.QUERY graphName query
```

> **Not implemented:** `GRAPH.QUERY` is registered and returns an empty
> result set for every query; the query engine is not implemented yet. Use the
> explicit graph commands above to read and write graphs.

---

## Probabilistic Commands

These follow RedisBloom, the probabilistic module built into Redis 8: the same syntax, replies and error texts (some of which, as in RedisBloom, carry no `ERR` prefix). Each structure is a key of its own type (`TYPE` reports `MBbloom--`, `MBbloomCF`, `CMSk-TYPE`, `TopK-TYPE` and `TDIS-TYPE` for t-digests). They live in the selected database, expire, are deleted and flushed like any key, and are saved in snapshots and the AOF, replicated, and moved by DUMP/RESTORE and MIGRATE. Hashes are deterministic, so a filter answers the same after a restart; they differ from RedisBloom's, so which item a top-k list drops on a tie, or which rare item is a false positive, can differ from Redis. One structure may take at most 512 MB; a larger request fails with `ERR Insufficient memory to create filter`.

### Bloom Filter

A scalable filter: when the newest sub-filter holds its capacity, a larger one (by the expansion factor, 2 by default) with half the error rate is added. `NONSCALING` or `EXPANSION 0` makes a full filter refuse new items. `BF.ADD`/`BF.MADD` create a filter of capacity 100 at error rate 0.01 when the key is absent.

```
BF.RESERVE key error_rate capacity [EXPANSION expansion] [NONSCALING]
BF.ADD key item                       # 1 if added, 0 if it may already exist
BF.MADD key item [item ...]
BF.INSERT key [CAPACITY c] [ERROR e] [EXPANSION x] [NOCREATE] [NONSCALING] ITEMS item [item ...]
BF.EXISTS key item
BF.MEXISTS key item [item ...]
BF.CARD key
BF.INFO key [CAPACITY | SIZE | FILTERS | ITEMS | EXPANSION]
```

### Cuckoo Filter

One-byte fingerprints in buckets of `BUCKETSIZE` (2 by default); the bucket count is rounded up to a power of two. When an item finds no place after `MAXITERATIONS` relocations, a filter `EXPANSION` times larger is added, or with `EXPANSION 0` the add fails with `Filter is full`. `CF.ADD` creates a filter of capacity 1000 when the key is absent.

```
CF.RESERVE key capacity [BUCKETSIZE bucketsize] [MAXITERATIONS maxiterations] [EXPANSION expansion]
CF.ADD key item
CF.ADDNX key item
CF.INSERT key [CAPACITY capacity] [NOCREATE] ITEMS item [item ...]
CF.INSERTNX key [CAPACITY capacity] [NOCREATE] ITEMS item [item ...]
CF.EXISTS key item
CF.MEXISTS key item [item ...]
CF.COUNT key item
CF.DEL key item
CF.INFO key
```

### Count-Min Sketch

32-bit counters; an increment past the maximum leaves the counter there and reports `CMS: INCRBY overflow` for that item.

```
CMS.INITBYDIM key width depth
CMS.INITBYPROB key error probability
CMS.INCRBY key item increment [item increment ...]
CMS.QUERY key item [item ...]
CMS.MERGE destination numkeys source [source ...] [WEIGHTS weight [weight ...]]
CMS.INFO key
```

### Top-K

HeavyKeeper with `depth` rows of `width` decaying counters (defaults 8, 7 and 0.9). `TOPK.ADD` and `TOPK.INCRBY` reply with the item each addition pushed out of the list, or nil.

```
TOPK.RESERVE key topk [width depth decay]
TOPK.ADD key item [item ...]
TOPK.INCRBY key item increment [item increment ...]
TOPK.QUERY key item [item ...]
TOPK.COUNT key item [item ...]
TOPK.LIST key [WITHCOUNT]
TOPK.INFO key
```

---

## Resilience Commands

### Circuit Breaker

```
CIRCUITX.CREATE name failure_threshold timeout_ms [success_threshold]   # create or replace a closed circuit; OK
CIRCUITX.ALLOW name                  # 1 if a call may proceed, 0 while open (counts a rejection); an open circuit turns half-open after timeout_ms
CIRCUITX.RECORD name SUCCESS|FAILURE # record a call outcome; replies with the resulting state
CIRCUITX.OPEN name                   # force open; OK
CIRCUITX.CLOSE name                  # force closed; OK
CIRCUITX.HALFOPEN name               # force half-open; OK
CIRCUITX.STATUS name                 # name, state, failures, successes, failure_threshold, success_threshold, timeout, retry_after
CIRCUITX.METRICS name                # total_failures, total_successes, rejected, failure_rate
CIRCUITX.RESET name                  # close and clear counters; OK
CIRCUITX.DELETE name                 # 1 if deleted, 0 if absent
```

Failures in the closed state open the circuit at `failure_threshold`; any failure in half-open reopens it, and `success_threshold` successes in half-open close it.

### Rate Limiter

```
RATELIMITER.CREATE name limit window_ms [strategy]   # limit requests per sliding window; OK
RATELIMITER.TRY name                 # [allowed(1|0), remaining, window_ms]
RATELIMITER.WAIT name timeout_ms     # blocks until a slot frees; ms waited, or null on timeout
RATELIMITER.RESET name               # clear recorded requests; OK
RATELIMITER.STATUS name              # name, limit, current, strategy
RATELIMITER.DELETE name               # 1 if deleted, 0 if absent
```

### Bulkhead

```
BULKHEAD.CREATE name max_concurrent  # OK
BULKHEAD.ACQUIRE name [timeout_ms]   # 1 when a permit is taken; without a timeout replies 0 at once if full, with one blocks up to timeout_ms, then 0
BULKHEAD.RELEASE name                # OK; error if no permit is held
BULKHEAD.STATUS name                 # name, current, max, waiting, rejected
BULKHEAD.DELETE name                 # 1 if deleted, 0 if absent
```

The blocking commands in this section (and `PROMISE.AWAIT`, `FUTURE.GET`) treat a `timeout_ms` of 0 as the 5-minute cap, never wait longer than that, and do not block inside `MULTI`/`EXEC`.

---

## Data Processing Commands

### Aggregation

```
AGGREGATOR.CREATE name windowSize [TYPE type]
AGGREGATOR.ADD name value [TIMESTAMP timestamp]
AGGREGATOR.GET name [TYPE type]
AGGREGATOR.RESET name
```

### Windowing

```
WINDOWX.CREATE name size [TYPE sliding|tumbling|session] [GAP gap]
WINDOWX.ADD name value
WINDOWX.GET name
WINDOWX.AGGREGATE name aggType
```

### Stream Joins

```
JOINX.CREATE name                    # inner join on a key; type/condition args are rejected
JOINX.ADD name LEFT|RIGHT key value  # add a value to one side under key; OK
JOINX.GET name [key]                 # one [key k left l right r] row per matching pair; optional key limits the rows
JOINX.DELETE name                    # 1 if deleted, 0 if absent
```

### Partitioning

```
PARTITIONX.CREATE name count
PARTITIONX.ADD name key value
PARTITIONX.GET name index
```

---


## Machine Learning Commands

### Feature Commands

```
FEATURE.SET key field value
FEATURE.GET key field
FEATURE.DEL key field
FEATURE.INCR entity key delta
FEATURE.VECTOR key [FIELDS field [field ...]]
```

### Embedding Commands

```
EMBEDDING.CREATE name dimensions [METRIC metric]
EMBEDDING.GET id
EMBEDDING.DELETE id
```

### Tensor Commands

```
TENSOR.CREATE name dim dim [dim ...] [| value ...]
TENSOR.GET name
TENSOR.ADD name1 name2 [OUT out]
TENSOR.RESHAPE name shape
TENSOR.DELETE name
```

---

## Tag-Based Invalidation Commands

```
SET key value [EX seconds] [PX milliseconds] [NX|XX] [TAGS tag [tag ...]]
TAGS key [tag [tag ...]]
TAGKEYS tag
INVALIDATE tag [tag ...]
```

---

## Command Help

For detailed help on any command:

```redis
COMMAND INFO SET
COMMAND DOCS GET
```

Or visit the [CacheStorm documentation](https://github.com/cachestorm/cachestorm/docs) for comprehensive guides.

---

## Notes

- All Redis commands are fully compatible with standard Redis clients
- Extended commands follow the same RESP protocol
- Commands with `.` prefix are CacheStorm-specific extensions
- Use `COMMAND LIST` to see all available commands at runtime

---

## Namespace Commands

CacheStorm supports multi-tenant namespaces for data isolation.

```
SELECT index
```

---

## Workflow Commands

### State Machine

```
STATEM.CREATE name initial
STATEM.DELETE name
STATEM.ADDSTATE machine state [type]
STATEM.ADDTRANS machine from to event [condition]
STATEM.TRIGGER machine event [data]
STATEM.CURRENT machine
STATEM.CANTRIGGER machine event
STATEM.EVENTS machine
STATEM.RESET machine [state]
STATEM.ISFINAL machine
STATEM.INFO machine
STATEM.LIST [pattern]
```

---

## Advanced Commands

### Actor Model

```
ACTOR.CREATE name [mailbox_size]
ACTOR.DELETE name
ACTOR.SEND name message
ACTOR.RECV name [timeout]
ACTOR.POKE name
ACTOR.PEEK name
ACTOR.LEN name
ACTOR.LIST [pattern]
ACTOR.CLEAR name
```

### DAG (Directed Acyclic Graph)

```
DAG.CREATE name                      # create or replace an empty DAG; OK
DAG.DELETE name                      # 1 if deleted, 0 if absent
DAG.ADDNODE name node                # 1 if added, 0 if it exists
DAG.ADDEDGE name from to             # both nodes must exist; an edge that would close a cycle is rejected; OK
DAG.TOPO name                        # topological order, ties broken by name
DAG.PARENTS name node                # sorted direct parents
DAG.CHILDREN name node               # sorted direct children
DAG.LIST                             # DAG names
```

### Parallel Processing

```
PARALLEL.MAP key command
PARALLEL.REDUCE operation arg2 value [value ...]
PARALLEL.FILTER key command
```

### Secrets Management

```
SECRET.SET key value [TTL ttl]
SECRET.GET key
SECRET.DELETE key
SECRET.LIST [pattern]
SECRET.ROTATE key newValue
SECRET.VERSION key
```

### Configuration

```
CONFIG.SET ns key value
CONFIG.GET ns key
CONFIG.DELETE ns key
CONFIG.LIST [pattern] [NAMESPACE ns]
CONFIG.NAMESPACE ns [command]
CONFIG.IMPORT ns key value [key value ...]
CONFIG.EXPORT ns [pattern] [FORMAT format]
```

### Data Structures

```
TRIE.ADD name key [value]
TRIE.SEARCH name prefix
TRIE.PREFIX name prefix [COUNT count]
TRIE.DELETE name key
TRIE.AUTOCOMPLETE name prefix [COUNT count]

RING.CREATE name replicas            # consistent-hash ring, replicas = virtual nodes per node (1-1024); OK
RING.ADD name node                   # node count after the add
RING.GET name key                    # node that owns key; null if the ring is empty
RING.NODES name                      # nodes sorted by name
RING.REMOVE name node                # 1 if removed, 0 if absent

SEM.CREATE name max                  # create or replace with max permits; OK
SEM.ACQUIRE name permits [timeout_ms] # blocks until permits are free: 1 acquired, 0 on timeout; 0 or omitted waits indefinitely; no blocking inside MULTI/EXEC
SEM.TRYACQUIRE name permits          # never blocks; 1 acquired, 0 otherwise
SEM.RELEASE name permits             # OK; error if it would exceed max
SEM.VALUE name                       # value, max, waiters
```

---

## Encoding Commands

```
MSGPACK.ENCODE json                  # one JSON document to MessagePack bytes
MSGPACK.DECODE msgpack               # raw MessagePack bytes to JSON text


URL.ENCODE string
URL.DECODE string

XML.ENCODE tag value                 # <tag>escaped value</tag>
XML.DECODE xml                       # one text-only element; [tag, text]

YAML.ENCODE json                     # one JSON document to YAML
YAML.DECODE yaml                     # one YAML document to JSON text

CBOR.ENCODE json                     # one JSON document to CBOR bytes
CBOR.DECODE cbor                     # raw CBOR bytes to JSON text

CSV.ENCODE rows [DELIMITER delim]
CSV.DECODE csv [DELIMITER delim]

UUID.GEN [VERSION version]
UUID.VALIDATE uuid
UUID.VERSION uuid

ULID.GEN
ULID.EXTRACT ulid

TIMESTAMP.NOW                        # unix, unix_milli, iso
TIMESTAMP.PARSE input                # unix seconds/ms, RFC 3339, 2006-01-02 or "2006-01-02 15:04:05" (UTC); unix, iso
TIMESTAMP.FORMAT unix_seconds layout # Go reference layout, server-local time zone
TIMESTAMP.ADD unix_seconds unit amount   # unit: second|minute|hour|day|week|month|year
TIMESTAMP.DIFF unix1 unix2 unit      # unix2 - unix1 in second|minute|hour|day|millisecond|microsecond|nanosecond
TIMESTAMP.STARTOF unix_seconds unit  # UTC start of the second|minute|hour|day|week (Sunday)|month|year
TIMESTAMP.ENDOF unix_seconds unit    # last second of that UTC period

DIFF.TEXT text1 text2                # Myers line diff: added, removed, unchanged counts and the diff lines (" ", "-", "+")
DIFF.JSON json1 json2                # added, removed, changed top-level keys

POOL.CREATE name size [FACTORY factory]
POOL.GET name [TIMEOUT timeout]
POOL.PUT name item
POOL.CLEAR name
POOL.STATS name
```

---

## Digest Commands

```
DIGEST.MD5 data
DIGEST.SHA1 data
DIGEST.SHA256 data
DIGEST.SHA512 data
DIGEST.HMAC algorithm key data
DIGEST.HMACMD5 key data
DIGEST.HMACSHA1 key data
DIGEST.HMACSHA256 key data
DIGEST.HMACSHA512 key data
DIGEST.CRC32 data
DIGEST.ADLER32 data
DIGEST.BASE64ENCODE data
DIGEST.BASE64DECODE data
DIGEST.HEXENCODE data
DIGEST.HEXDECODE data

CRYPTO.HASH algorithm data
CRYPTO.HMAC algorithm key data
```

---

## Event Commands

### Event Sourcing

```
EVENT.EMIT name [field value ...]    # id, name, timestamp
EVENT.GET name [limit]               # newest events for name (limit defaults to 10), each with id, name, timestamp and data
EVENT.LIST                           # newest 100 events of any name
EVENT.CLEAR                          # drop every event; OK
```

### Compression

```
COMPRESS.RLE data
DECOMPRESS.RLE data
COMPRESS.LZ4 data
DECOMPRESS.LZ4 data
COMPRESS.CUSTOM algorithm data [LEVEL level]
```

### Queue & Stack

```
QUEUE.CREATE name [MAX max]
QUEUE.PUSH name item
QUEUE.POP name [TIMEOUT timeout]
QUEUE.PEEK name
QUEUE.LEN name
QUEUE.CLEAR name

STACK.CREATE name [MAX max]
STACK.PUSH name item
STACK.POP name
STACK.PEEK name
STACK.LEN name
STACK.CLEAR name
```

---

## Utility Commands

### Rate Limiting

```
RL.CREATE key maxTokens refillRate intervalMs [BURST burst]
RL.ALLOW key tokens
RL.GET name
RL.DELETE name
RL.RESET name
```

### Distributed Locks

```
LOCK.TRY key holder token ttlMs
LOCK.ACQUIRE key holder token ttlMs timeoutMs [RETRY retry] [DELAY delay]
LOCK.RELEASE key holder token
LOCK.RENEW key holder token ttlMs
LOCK.INFO key
LOCK.ISLOCKED key
```

### ID Generation

```
ID.CREATE name start increment
ID.NEXT name
ID.NEXTN name count
ID.CURRENT name
ID.SET name value
ID.DELETE name

SNOWFLAKE.NEXT [NODE node]
SNOWFLAKE.PARSE id
```

### Audit Logging

```
AUDIT.LOG command [key] [arg ...]    # add a manual entry; replies its id
AUDIT.GET id                         # id, timestamp, command, key, args, client_ip, user, success, duration_us; null if absent
AUDIT.GETRANGE start_ms end_ms       # entries whose timestamp (ms) is in the inclusive range
AUDIT.GETBYCMD command [limit]       # newest first, limit defaults to 100; case-sensitive (recorded names are uppercase)
AUDIT.GETBYKEY key [limit]           # newest first, limit defaults to 100
AUDIT.CLEAR                          # OK
AUDIT.COUNT                          # number of entries
AUDIT.STATS                          # total, success, failed, enabled, max_size, per-command counts
AUDIT.ENABLE                         # record every dispatched command (off by default); OK
AUDIT.DISABLE                        # OK
```

When enabled, every dispatched command is recorded with its first argument as the key and up to 16 further arguments (each capped at 128 bytes); `success` is false when the command replied with an error. AUTH, HELLO, ACL, CONFIG and MIGRATE are recorded by name only, without arguments. The log keeps the newest 10,000 entries.

### Feature Flags

```
FLAG.CREATE name [DEFAULT default]
FLAG.DELETE name
FLAG.GET name
FLAG.ENABLE name
FLAG.DISABLE name
FLAG.TOGGLE name
FLAG.ISENABLED name
FLAG.LIST [pattern]
FLAG.LISTENABLED
FLAG.ADDVARIANT name variant value [WEIGHT weight]
FLAG.GETVARIANT name key
```

### Counters

```
COUNTER.GET name
COUNTER.SET name value
COUNTER.INCR name [BY increment]
COUNTER.DECR name [BY decrement]
COUNTER.INCRBY name increment
COUNTER.DECRBY name decrement
COUNTER.DELETE name
COUNTER.LIST [pattern]
COUNTER.GETALL [pattern]
COUNTER.RESET name
COUNTER.RESETALL [pattern]
```

### Memory Management

```
MEMORY.TRIM
MEMORY.FRAG
MEMORY.PURGE
```

---

## Integration Commands

### Circuit Breaker

```
CIRCUITBREAKER.CREATE name threshold timeoutMs
CIRCUITBREAKER.STATE name
CIRCUITBREAKER.TRIP name
CIRCUITBREAKER.RESET name
```

### Rate Limit

```
RATELIMIT.CREATE name rate window
RATELIMIT.CHECK name cost
RATELIMIT.RESET name
RATELIMIT.DELETE name
```

### Cache Operations

```
CACHE.LOCK key holder ttlMs
CACHE.UNLOCK key holder
CACHE.LOCKED key
CACHE.REFRESH key holder ttlMs
```

### Array Operations

```
ARRAY.PUSH key value [value ...]
ARRAY.POP key
ARRAY.SHIFT key
ARRAY.UNSHIFT key value [value ...]
ARRAY.SLICE key start end
ARRAY.SPLICE key start deleteCount [value ...]
ARRAY.REVERSE key
ARRAY.SORT key [ASC|DESC]
ARRAY.UNIQUE key
ARRAY.MERGE dest src
ARRAY.INTERSECT key1 key2 [OUT out]
ARRAY.DIFF key1 key2 [OUT out]
ARRAY.INDEXOF key value
ARRAY.LASTINDEXOF key value
ARRAY.INCLUDES key value
```

### Object Operations

```
OBJECT.KEYS key
OBJECT.VALUES key
OBJECT.ENTRIES key
OBJECT.FROMENTRIES name key value [key value ...]
OBJECT.MERGE dest src
OBJECT.PICK key fields...
OBJECT.OMIT key fields...
OBJECT.HAS key field
OBJECT.GET key field [DEFAULT default]
OBJECT.SET key field value
OBJECT.DELETE key field
```

### Math Operations

```
MATH.ADD a b
MATH.SUB a b
MATH.MUL a b
MATH.DIV a b
MATH.MOD a b
MATH.POW base exp
MATH.SQRT x
MATH.ABS x
MATH.MIN a b [c ...]
MATH.MAX a b [c ...]
MATH.FLOOR x
MATH.CEIL x
MATH.ROUND x [DECIMALS decimals]
MATH.RANDOM [MIN min] [MAX max]
MATH.SUM key
MATH.AVG key
MATH.MEDIAN key
MATH.STDDEV value value [value ...]
```

### Geo Operations

```
GEO.ENCODE lat lon [PRECISION precision]
GEO.DECODE hash
GEO.DISTANCE lat1 lon1 lat2 lon2 [UNIT unit]
GEO.BOUNDINGBOX lat lon radius [UNIT unit]
```

### Captcha

```
CAPTCHA.GENERATE [WIDTH width] [HEIGHT height] [LENGTH length]
CAPTCHA.VERIFY id code
```

### Sequence

```
SEQUENCE.NEXT name
SEQUENCE.CURRENT name
SEQUENCE.RESET name [VALUE value]
SEQUENCE.SET name value
```

---

## String Processing Commands

String manipulation and text analysis on stored values.

```
STR.CONTAINS key substring
STR.ENDSWITH key suffix
STR.FORMAT format [arg ...]
STR.INDEX key substring
STR.JOIN separator part [part ...]
STR.LASTINDEX key substring
STR.LINES key
STR.PADLEFT key width padChar
STR.PADRIGHT key width padChar
STR.REPEAT key count
STR.REPLACE key old new
STR.REVERSE key
STR.SPLIT key delimiter
STR.STARTSWITH key prefix
STR.TITLE key
STR.TRIM key
STR.TRIMLEFT key
STR.TRIMRIGHT key
STR.TRUNCATE key length
STR.WORDS key
```

---

## Session Commands

CacheStorm-native server-side session storage.

```
SESSION.CREATE id ttlMs
SESSION.GET id key
SESSION.SET id key value
SESSION.DEL id
SESSION.EXISTS id
SESSION.TTL id
SESSION.REFRESH id ttlMs
SESSION.ALL
SESSION.LIST
SESSION.COUNT
SESSION.CLEAR
SESSION.CLEANUP
SESSION.DELETE id
```

---

## CRDT Commands

Conflict-free replicated data types for multi-node convergence.

```
CRDT.GCOUNTER.INCR name node_id [amount]  # that node's new count
CRDT.GCOUNTER.GET name               # sum over nodes
CRDT.PNCOUNTER.INCR name node_id     # +1 for node_id; OK
CRDT.PNCOUNTER.DECR name node_id     # -1 for node_id; OK
CRDT.PNCOUNTER.GET name              # increments minus decrements
CRDT.GSET.ADD name value             # 1 if added, 0 if present
CRDT.GSET.GET name                   # members
CRDT.ORSET.ADD name value [tag]      # add an observed tag (generated if omitted); replies the tag
CRDT.ORSET.REMOVE name value [tag ...] # tombstone the given tags, or every observed tag; number of live tags removed
CRDT.ORSET.GET name                  # sorted values with at least one live tag
CRDT.ORSET.TAGS name value           # sorted live tags of value
CRDT.LWW.SET key value [timestamp]   # written only if timestamp (default now, ns) is newer; OK
CRDT.LWW.GET key                     # value or null
CRDT.LWW.DELETE key                  # 1 if deleted, 0 if absent
```

---

## Vector Commands

Vector embeddings: storage, similarity search, and index management.

```
VECTOR.CREATE name dim
VECTOR.ADD name id dim value [value ...]
VECTOR.GET name id
VECTOR.DELETE name id
VECTOR.SEARCH name k dim [vectorValue ...]
VECTOR.SIMILARITY name id1 id2
VECTOR.MERGE name id1 id2
VECTOR.NORMALIZE name id
VECTOR.DIMENSIONS name
VECTOR.STATS name
```

---

## T-Digest Commands

Streaming quantile sketches using RedisBloom syntax and replies (doubles as bulk strings). A missing key is an error. A t-digest is a key of type `TDIS-TYPE`, saved, replicated and expired like the other probabilistic structures.

```
TDIGEST.CREATE key [COMPRESSION compression]   # default 100; error if key exists; OK
TDIGEST.ADD key value [value ...]    # OK
TDIGEST.MERGE destination_key numkeys source_key [source_key ...] [COMPRESSION compression] [OVERRIDE]  # OK
TDIGEST.QUANTILE key quantile [quantile ...]   # one double per quantile (0..1)
TDIGEST.CDF key value [value ...]    # one double per value
TDIGEST.RANK key value [value ...]   # estimated rank per value (-1 below min, -2 if empty)
TDIGEST.REVRANK key value [value ...] # estimated reverse rank per value
TDIGEST.BYRANK key rank [rank ...]   # value per rank (inf past the end, nan if empty)
TDIGEST.BYREVRANK key reverse_rank [reverse_rank ...]  # value per reverse rank (-inf past the end)
TDIGEST.TRIMMED_MEAN key low_cut_quantile high_cut_quantile  # mean between the two quantiles
TDIGEST.MIN key                      # nan if empty
TDIGEST.MAX key                      # nan if empty
TDIGEST.MEAN key                     # CacheStorm extension; nan if empty
TDIGEST.RESET key                    # empty the sketch, keep compression; OK
TDIGEST.INFO key                     # Compression, Capacity, Merged/Unmerged nodes and weight, Observations, Total compressions, Memory usage
```

---

## Message Queue Commands

In-memory queue with acknowledgements, a visibility timeout, retries and a dead-letter list. None of the commands block.

```
MSGQUEUE.CREATE name [max_retries]   # create or replace (default 3 retries); OK
MSGQUEUE.PUBLISH name body           # replies the message id
MSGQUEUE.CONSUME name [visibility_timeout_ms]  # take the oldest message (default 5000 ms visibility): id, body, retries; null if empty
MSGQUEUE.ACK name id                 # 1 if acked, 0 if not in flight
MSGQUEUE.NACK name id                # 1 requeued, 2 dead-lettered after max_retries, 0 if not in flight
MSGQUEUE.REQUEUE name id             # move a dead letter back with retries reset; 1 or 0
MSGQUEUE.DEADLETTER name             # dead-lettered messages
MSGQUEUE.PURGE name                  # drop pending messages; number dropped
MSGQUEUE.DELETE name                 # 1 if deleted, 0 if absent
MSGQUEUE.STATS name                  # name, pending, processing, dead_letters, max_retries
```

A consumed message that is not acknowledged within its visibility timeout is treated as NACKed.

---

## Document Commands

Document storage with indexing and aggregation queries.

```
DOC.INSERT name id field value [field value ...]  # insert or replace a document; replies the id
DOC.FIND name [field value]          # all documents, or those where field equals value, sorted by id
DOC.FINDONE name field value         # first match by id, or null
DOC.UPDATE name id [field value ...] # merge fields into the document; OK
DOC.DELETE name id                   # 1 if deleted, 0 if absent
DOC.COUNT name                       # number of documents
DOC.AGGREGATE name count|fields      # document count, or sorted field names in use
DOC.DISTINCT name field              # sorted distinct values of field
DOC.INDEX name field                 # index field (kept current on insert, update and delete); OK
DOC.DROPINDEX name field             # 1 if dropped, 0 if no such index
```

---

## Circuit Breaker Commands

Circuit breaker state and failure/success counters for resilient calls.

```
CIRCUIT.CREATE name failureThreshold successThreshold timeoutMs
CIRCUIT.STATE name
CIRCUIT.ALLOW name
CIRCUIT.SUCCESS name
CIRCUIT.FAILURE name
CIRCUIT.RESET name
CIRCUIT.DELETE name
CIRCUIT.LIST
CIRCUIT.STATS name
```

---

## Validation Commands

Reusable validation rules and value checkers.

```
VALIDATE.CREATE name regex           # store an RE2 rule (unanchored unless ^...$); OK
VALIDATE.LIST                        # rule names
VALIDATE.DELETE name                 # 1 if deleted, 0 if absent
VALIDATE.CHECK name value            # 1 if value matches the rule
VALIDATE.ALPHA value                 # non-empty ASCII letters
VALIDATE.ALPHANUM value              # non-empty ASCII letters and digits
VALIDATE.EMAIL value                 # bare addr-spec with a dotted domain, at most 254 bytes
VALIDATE.URL value                   # absolute scheme://host URL
VALIDATE.IP value                    # IPv4 or IPv6, no zones
VALIDATE.FLOAT value                 # optional sign, digits, at most one '.'; no exponent
VALIDATE.INT value                   # optional sign and digits
VALIDATE.LENGTH value min max        # byte length within min..max
VALIDATE.RANGE value min max         # numeric value within min..max (exact comparison)
VALIDATE.JSON value                  # valid JSON
```

The checkers reply 1 when the value is valid and 0 otherwise.

---

## Key Management Commands

Key lifecycle operations that go beyond the core Redis surface.

```
KEY.COPY source destination [REPLACE]
KEY.RENAME key newkey
KEY.RENAMENX key newkey
KEY.MOVE key db
KEY.IDLETIME key
KEY.FREQ key
```

---

## Batch Commands

Submit and inspect grouped operations.

```
BATCH.GET id
BATCH.EXEC id
BATCH.MGET name [key ...]
BATCH.MSET key value [key value ...]
BATCH.MDEL name [key ...]
BATCH.SET key value [key value ...]
BATCH.DEL name [key ...]
```

---

## Analytics Commands

Counters and aggregates for tracking usage over time.

```
ANALYTICS.INCR name value
ANALYTICS.DECR name value
ANALYTICS.GET name
ANALYTICS.COUNT name
ANALYTICS.SUM name
ANALYTICS.AVG name
ANALYTICS.MIN name
ANALYTICS.MAX name
ANALYTICS.CLEAR name
```

---

## Service Discovery Commands

Register services and instances for discovery and health tracking.

```
SERVICE.REGISTER name id address port
SERVICE.DEREGISTER name id
SERVICE.DISCOVER name
SERVICE.HEALTHY name
SERVICE.HEARTBEAT name id
SERVICE.TAGS name id tag [tag ...]
SERVICE.WEIGHT name id weight
SERVICE.LIST
```

---

## Profile Commands

Store and read user profile records.

```
PROFILE.GET id
PROFILE.SET id key value
PROFILE.LIST
PROFILE.CREATE user
PROFILE.DELETE id
```

---

## Extended Command Families

The sections below list every remaining registered command family. Unlike the command groups above, these keep their state in the server process, outside the keyspace: it is not saved in snapshots or the AOF, not replicated, shared by every database, and not removed by `DEL` or `FLUSHALL`. Use them for process-local coordination, not for data that must survive a restart.

Command names are reproduced exactly as registered in `internal/command`.

Argument signatures are given for some families; for the rest, check the command handler for syntax.

---
## Unprefixed Commands

Commands without a dotted namespace prefix (58 commands).

```
ADDTAG
ASKING
AUTH
BLMPOP
BZMPOP
CLIENT
CLUSTER
CLUSTERINFO
CLUSTERNODES
CLUSTERSLOTS
ECHO
EVAL
EVAL_RO
EVALSHA
EVALSHA_RO
EXPIREAT
FCALL
FCALL_RO
FUNCTION
GETDEL
GETEX
HELLO
HGETDEL
HOTKEYS
HSCAN
INCRBYFLOAT
LCS
LMPOP
LMPUSH
LPUSHX
MEMINFO
MIGRATE
MSETNX
PEXPIREAT
PING
PSYNC
PTTL
QUIT
READONLY
READWRITE
REMTAG
REPLCONF
RESET
RPUSHX
SCRIPT
SENTINEL
SETTAG
SINTERCARD
SMISMEMBER
SORT
SORT_RO
SUBSTR
TAGCHILDREN
TAGCOUNT
TAGLINK
TAGUNLINK
ZRANGESTORE
ZREMRANGEBYRANK
```

---
## Aggregation Commands

`AGG.*` command family (7 commands).

```
AGG.AVG
AGG.CLEAR
AGG.COUNT
AGG.MAX
AGG.MIN
AGG.PUSH
AGG.SUM
```

---
## Alerting Commands

`ALERT.*` command family (6 commands).

```
ALERT.CREATE name message            # create or replace with status active; OK
ALERT.TRIGGER name                   # status firing; OK
ALERT.ACKNOWLEDGE name               # status acknowledged; OK
ALERT.RESOLVE name                   # status resolved; OK
ALERT.LIST                           # name and status of every alert
ALERT.HISTORY name                   # id, name, message, status, created_at and every status transition (last 1000)
```

---
## API Keys Commands

`APIKEY.*` command family (5 commands).

```
APIKEY.CREATE
APIKEY.LIST
APIKEY.REVOKE
APIKEY.USAGE
APIKEY.VALIDATE
```

---
## Auditx Commands

`AUDITX.*` command family (4 commands).

```
AUDITX.GET
AUDITX.LIST
AUDITX.LOG
AUDITX.SEARCH
```

---
## Backpressure Commands

`BACKPRESSURE.*` command family (3 commands).

```
BACKPRESSURE.CHECK
BACKPRESSURE.CREATE
BACKPRESSURE.STATUS
```

---
## Bloom Filters Commands

`BLOOMX.*` command family (5 commands).

```
BLOOMX.ADD
BLOOMX.CHECK
BLOOMX.CREATE
BLOOMX.DELETE
BLOOMX.INFO
```

---
## Bookmark Commands

`BOOKMARK.*` command family (4 commands).

```
BOOKMARK.DELETE
BOOKMARK.GET
BOOKMARK.LIST
BOOKMARK.SET
```

---
## Buckets Commands

`BUCKETX.*` command family (5 commands).

```
BUCKETX.CREATE
BUCKETX.DELETE
BUCKETX.REFILL
BUCKETX.RETURN
BUCKETX.TAKE
```

---
## Bufferx Commands

`BUFFERX.*` command family (4 commands).

```
BUFFERX.CREATE
BUFFERX.DELETE
BUFFERX.READ
BUFFERX.WRITE
```

---
## Cache Management Commands

`CACHE.*` command family (5 commands).

```
CACHE.BULKDEL
CACHE.BULKGET
CACHE.CLEAR
CACHE.EXPORT
CACHE.STATS
```

---
## Cachex Commands

`CACHEX.*` command family (5 commands).

```
CACHEX.CREATE
CACHEX.DELETE
CACHEX.GET
CACHEX.LIST
CACHEX.SET
```

---
## Chaining Commands

`CHAIN.*` command family (6 commands).

```
CHAIN.CREATE name                    # create or replace an empty chain; OK
CHAIN.ADD name data                  # append a SHA-256 linked block; replies its index
CHAIN.GET name index                 # index, timestamp, data, hash, prev_hash; null if out of range
CHAIN.LAST name                      # index, data, hash of the last block; null if empty
CHAIN.LENGTH name                    # number of blocks
CHAIN.VALIDATE name                  # 1 if every block's hash and link recompute correctly, else 0
```

---
## Chained Commands

`CHAINED.*` command family (3 commands).

```
CHAINED.DEL
CHAINED.GET
CHAINED.SET
```

---
## Coalescing Commands

`COALESCE.*` command family (4 commands).

```
COALESCE.ADD
COALESCE.CLEAR
COALESCE.CREATE
COALESCE.GET
```

---
## Connection Commands

`CONNECTION.*` command family (4 commands).

```
CONNECTION.COUNT
CONNECTION.INFO
CONNECTION.KILL
CONNECTION.LIST
```

---
## Context Commands

`CONTEXT.*` command family (5 commands).

```
CONTEXT.CREATE
CONTEXT.DELETE
CONTEXT.GET
CONTEXT.LIST
CONTEXT.SET
```

---
## Cooldown Commands

`COOLDOWN.*` command family (5 commands).

```
COOLDOWN.CHECK
COOLDOWN.DELETE
COOLDOWN.LIST
COOLDOWN.RESET
COOLDOWN.SET
```

---
## Counter Metrics Commands

`COUNTERX.*` command family (6 commands).

```
COUNTERX.CREATE
COUNTERX.DECR
COUNTERX.DELETE
COUNTERX.GET
COUNTERX.INCR
COUNTERX.RESET
```

---
## Counterx2 Commands

`COUNTERX2.*` command family (5 commands).

```
COUNTERX2.CREATE
COUNTERX2.DECR
COUNTERX2.GET
COUNTERX2.INCR
COUNTERX2.LIST
```

---
## Datasets Commands

`DATASET.*` command family (5 commands).

```
DATASET.CREATE name                  # create or replace an empty dataset; OK
DATASET.ADD name item                # new length
DATASET.GET name                     # items; null if absent
DATASET.SPLIT name ratio [SEED seed] # train and test item arrays, floor(len*ratio) in train; SEED shuffles deterministically
DATASET.DELETE name                  # 1 if deleted, 0 if absent
```

---
## Deadline Commands

`DEADLINE.*` command family (4 commands).

```
DEADLINE.CANCEL
DEADLINE.CHECK
DEADLINE.LIST
DEADLINE.SET
```

---
## Debounce Commands

`DEBOUNCE.*` command family (4 commands).

```
DEBOUNCE.CALL
DEBOUNCE.DELETE
DEBOUNCE.GET
DEBOUNCE.SET
```

---
## Debouncex Commands

`DEBOUNCEX.*` command family (4 commands).

```
DEBOUNCEX.CREATE name delay_ms       # OK
DEBOUNCEX.CALL name arg              # remember arg as the latest call and restart the delay; OK
DEBOUNCEX.CANCEL name                # drop the pending call; OK
DEBOUNCEX.FLUSH name [FORCE]         # the pending arg once delay_ms has passed since the last CALL (at once with FORCE); null otherwise
```

---
## Deduplication Commands

`DEDUP.*` command family (4 commands).

```
DEDUP.ADD
DEDUP.CHECK
DEDUP.CLEAR
DEDUP.EXPIRE
```

---
## Entity Commands

`ENTITY.*` command family (5 commands).

```
ENTITY.CREATE
ENTITY.DELETE
ENTITY.GET
ENTITY.LIST
ENTITY.SET
```

---
## Script Evaluation Commands

`EVAL.*` command family (7 commands).

```
EVAL.EXPR expression                 # arithmetic with + - * / % ^, unary signs and parentheses; no variables or functions
EVAL.JSONPATH json path              # $ with .name, ['name'] and [index] steps; JSON text, or null if not found
EVAL.FORMAT format [arg ...]         # {} takes the next arg, {N} the Nth
EVAL.TEMPLATE template name value [name value ...]  # replace {{ name }} placeholders
EVAL.REGEX pattern text              # 1 if the RE2 pattern matches, else 0
EVAL.REGEXMATCH pattern text         # all matches
EVAL.REGEXREPLACE pattern replacement text  # replace every match; $1 and ${name} expand
```

---
## Event Sourcing Commands

`EVENTSOURCING.*` command family (3 commands).

```
EVENTSOURCING.APPEND
EVENTSOURCING.GET
EVENTSOURCING.REPLAY
```

---
## Experimentation Commands

`EXPERIMENT.*` command family (6 commands).

```
EXPERIMENT.CREATE name variant [variant ...]  # create or replace; OK
EXPERIMENT.ASSIGN name user_id       # sticky, hash-based variant
EXPERIMENT.TRACK name user_id event  # record an event for an assigned user; OK
EXPERIMENT.RESULTS name              # assignments, then per variant its users and, per event, count, users and conversion_rate
EXPERIMENT.LIST                      # experiment names
EXPERIMENT.DELETE name               # 1 if deleted, 0 if absent
```

---
## Freq Commands

`FREQ.*` command family (5 commands).

```
FREQ.ADD
FREQ.COUNT
FREQ.CREATE
FREQ.DELETE
FREQ.TOP
```

---
## Future Commands

`FUTURE.*` command family (4 commands).

```
FUTURE.CREATE                        # new pending future; replies its id
FUTURE.COMPLETE id value             # OK; error unless pending
FUTURE.CANCEL id                     # OK; error unless pending
FUTURE.GET id [timeout_ms]           # value when completed, error when cancelled, null while pending; with timeout_ms blocks up to that long
```

---
## Gateway Commands

`GATEWAY.*` command family (4 commands).

```
GATEWAY.CREATE
GATEWAY.DELETE
GATEWAY.LIST
GATEWAY.ROUTE
```

---
## Gauge Metrics Commands

`GAUGE.*` command family (6 commands).

```
GAUGE.CREATE
GAUGE.DECR
GAUGE.DELETE
GAUGE.GET
GAUGE.INCR
GAUGE.SET
```

---
## Ghost Entries Commands

`GHOST.*` command family (4 commands).

```
GHOST.CREATE
GHOST.DELETE
GHOST.READ
GHOST.WRITE
```

---
## Grant Commands

`GRANT.*` command family (4 commands).

```
GRANT.CHECK
GRANT.CREATE
GRANT.DELETE
GRANT.LIST
```

---
## Grid Index Commands

`GRID.*` command family (6 commands).

```
GRID.CLEAR
GRID.CREATE
GRID.DELETE
GRID.GET
GRID.QUERY
GRID.SET
```

---
## Health Commands

`HEALTH.*` command family (3 commands).

```
HEALTH.CHECK                         # status, timestamp, uptime_seconds
HEALTH.LIVENESS                      # OK
HEALTH.READINESS                     # OK, or a NOTREADY error while a replica's master link is down or a sync is in progress
```

---
## Heap Commands

`HEAP.*` command family (7 commands).

```
HEAP.DELETE
HEAP.GC
HEAP.PEEK
HEAP.POP
HEAP.PUSH
HEAP.SIZE
HEAP.STATS
```

---
## Histogram Commands

`HISTOGRAM.*` command family (6 commands).

```
HISTOGRAM.ADD
HISTOGRAM.CREATE
HISTOGRAM.GET
HISTOGRAM.INFO
HISTOGRAM.MEAN
HISTOGRAM.RESET
```

---
## Hyperparam Commands

`HYPERPARAM.*` command family (3 commands).

```
HYPERPARAM.DELETE
HYPERPARAM.GET
HYPERPARAM.SET
```

---
## Idempotency Commands

`IDEMPOTENCY.*` command family (5 commands).

```
IDEMPOTENCY.CHECK
IDEMPOTENCY.DELETE
IDEMPOTENCY.GET
IDEMPOTENCY.LIST
IDEMPOTENCY.SET
```

---
## Indexing Commands

`INDEX.*` command family (5 commands).

```
INDEX.ADD
INDEX.CREATE
INDEX.DELETE
INDEX.LIST
INDEX.SEARCH
```

---
## Json Commands

`JSON.*` command family (1 command).

```
JSON.MSET
```

---
## Leader Commands

`LEADER.*` command family (5 commands).

```
LEADER.CURRENT
LEADER.ELECT
LEADER.HISTORY
LEADER.RENEW
LEADER.RESIGN
```

---
## Leaky Bucket Commands

`LEAKYBUCKET.*` command family (4 commands).

```
LEAKYBUCKET.ADD
LEAKYBUCKET.AVAILABLE
LEAKYBUCKET.CREATE
LEAKYBUCKET.DELETE
```

---
## Leases Commands

`LEASE.*` command family (5 commands).

```
LEASE.CREATE
LEASE.GET
LEASE.LIST
LEASE.RENEW
LEASE.REVOKE
```

---
## Level Commands

`LEVEL.*` command family (5 commands).

```
LEVEL.CREATE name max                # create or replace at level 0; OK
LEVEL.SET name level                 # level must be 0..max; OK
LEVEL.GET name                       # current, max
LEVEL.LIST                           # sorted level names
LEVEL.DELETE name                    # 1 if deleted, 0 if absent
```

---
## Distributed Locks Commands

`LOCKX.*` command family (4 commands).

```
LOCKX.ACQUIRE
LOCKX.EXTEND
LOCKX.RELEASE
LOCKX.STATUS
```

---
## Logx Commands

`LOGX.*` command family (5 commands).

```
LOGX.CLEAR
LOGX.READ
LOGX.SEARCH
LOGX.STATS
LOGX.WRITE
```

---
## LRU Cache Commands

`LRU.*` command family (7 commands).

```
LRU.CLEAR
LRU.CREATE
LRU.DEL
LRU.GET
LRU.KEYS
LRU.SET
LRU.STATS
```

---
## Masking Commands

`MASK.*` command family (4 commands).

```
MASK.CARD
MASK.EMAIL
MASK.IP
MASK.PHONE
```

---
## Meter Commands

`METER.*` command family (4 commands).

```
METER.CREATE
METER.DELETE
METER.GET
METER.RECORD
```

---
## Metrics Commands

`METRICS.*` command family (3 commands).

```
METRICS.CMD
METRICS.GET
METRICS.RESET
```

---
## ML Experimentation Commands

`MLXPERIMENT.*` command family (4 commands).

```
MLXPERIMENT.CREATE
MLXPERIMENT.DELETE
MLXPERIMENT.LOG
MLXPERIMENT.METRICS
```

---
## Partitioning Commands

`PARTITION.*` command family (5 commands).

```
PARTITION.ADD
PARTITION.CREATE
PARTITION.DELETE
PARTITION.GET
PARTITION.LIST
```

---
## Permit Commands

`PERMIT.*` command family (4 commands).

```
PERMIT.CHECK
PERMIT.GRANT
PERMIT.LIST
PERMIT.REVOKE
```

---
## Pipeline Commands

`PIPELINE.*` command family (1 commands).

```
PIPELINE.EXEC
```

---
## Poolx Commands

`POOLX.*` command family (5 commands).

```
POOLX.ACQUIRE
POOLX.CREATE
POOLX.DELETE
POOLX.RELEASE
POOLX.STATUS
```

---
## Priority Queue Commands

`PQ.*` command family (7 commands).

```
PQ.CLEAR
PQ.CREATE
PQ.GETALL
PQ.LEN
PQ.PEEK
PQ.POP
PQ.PUSH
```

---
## Profilex Commands

`PROFILEX.*` command family (1 command).

```
PROFILEX.LIST
```

---
## Promise Tracking Commands

`PROMISE.*` command family (5 commands).

```
PROMISE.CREATE                       # new pending promise; replies its id
PROMISE.RESOLVE id value             # OK; error unless pending
PROMISE.REJECT id reason             # OK; error unless pending
PROMISE.STATUS id                    # id, status (pending|resolved|rejected)
PROMISE.AWAIT id timeout_ms          # blocks until settled: value, error "promise rejected: reason", or null on timeout
```

---
## Quota Control Commands

`QUOTA.*` command family (6 commands).

```
QUOTA.CHECK
QUOTA.DELETE
QUOTA.LIST
QUOTA.RESET
QUOTA.SET
QUOTA.USE
```

---
## Quotax Commands

`QUOTAX.*` command family (5 commands).

```
QUOTAX.CHECK
QUOTAX.CREATE
QUOTAX.DELETE
QUOTAX.RESET
QUOTAX.USAGE
```

---
## Record Commands

`RECORD.*` command family (4 commands).

```
RECORD.ADD
RECORD.CREATE
RECORD.DELETE
RECORD.GET
```

---
## Relation Commands

`RELATION.*` command family (4 commands).

```
RELATION.CREATE
RELATION.DELETE
RELATION.GET
RELATION.LIST
```

---
## Ring Buffer Commands

`RINGBUFFER.*` command family (5 commands).

```
RINGBUFFER.CREATE
RINGBUFFER.DELETE
RINGBUFFER.READ
RINGBUFFER.SIZE
RINGBUFFER.WRITE
```

---
## Rollout Commands

`ROLLOUT.*` command family (5 commands).

```
ROLLOUT.CHECK
ROLLOUT.CREATE
ROLLOUT.DELETE
ROLLOUT.LIST
ROLLOUT.PERCENTAGE
```

---
## Rollups Commands

`ROLLUP.*` command family (4 commands).

```
ROLLUP.ADD
ROLLUP.CREATE
ROLLUP.DELETE
ROLLUP.GET
```

---
## Rollupx Commands

`ROLLUPX.*` command family (4 commands).

```
ROLLUPX.ADD
ROLLUPX.CREATE
ROLLUPX.DELETE
ROLLUPX.GET
```

---
## Route Commands

`ROUTE.*` command family (4 commands).

```
ROUTE.ADD
ROUTE.LIST
ROUTE.MATCH
ROUTE.REMOVE
```

---
## Sample Commands

`SAMPLE.*` command family (5 commands).

```
SAMPLE.ADD
SAMPLE.CREATE
SAMPLE.GET
SAMPLE.INFO
SAMPLE.RESET
```

---
## Sanitize Commands

`SANITIZE.*` command family (4 commands).

```
SANITIZE.HTML
SANITIZE.JSON
SANITIZE.SQL
SANITIZE.STRING
```

---
## Semaphores Commands

`SEMAPHOREX.*` command family (4 commands).

```
SEMAPHOREX.ACQUIRE
SEMAPHOREX.CREATE
SEMAPHOREX.RELEASE
SEMAPHOREX.STATUS
```

---
## Sessionx Commands

`SESSIONX.*` command family (5 commands).

```
SESSIONX.CREATE user ttl_ms          # replies the session id
SESSIONX.SET id field value [field value ...]  # merge data; does not extend the TTL; OK
SESSIONX.GET id                      # id, user, created_at, expires_at, ttl, data; error once expired
SESSIONX.LIST                        # sorted live session ids
SESSIONX.DELETE id                   # 1 if deleted, 0 if absent or expired
```

---
## Shuffle Commands

`SHUFFLE.*` command family (3 commands).

```
SHUFFLE.ADD
SHUFFLE.CREATE
SHUFFLE.GET
```

---
## Sketch Commands

`SKETCH.*` command family (4 commands).

```
SKETCH.CREATE
SKETCH.DELETE
SKETCH.QUERY
SKETCH.UPDATE
```

---
## Slice Commands

`SLICE.*` command family (4 commands).

```
SLICE.APPEND
SLICE.CREATE
SLICE.DELETE
SLICE.GET
```

---
## Sliding Commands

`SLIDING.*` command family (5 commands).

```
SLIDING.CHECK
SLIDING.CREATE
SLIDING.DELETE
SLIDING.RESET
SLIDING.STATS
```

---
## Slidingwindow Commands

`SLIDINGWINDOW.*` command family (5 commands).

```
SLIDINGWINDOW.COUNT
SLIDINGWINDOW.CREATE
SLIDINGWINDOW.DELETE
SLIDINGWINDOW.INCR
SLIDINGWINDOW.RESET
```

---
## Slow Log Commands

`SLOWLOG.*` command family (4 commands).

```
SLOWLOG.CONFIG
SLOWLOG.GET
SLOWLOG.LEN
SLOWLOG.RESET
```

---
## Spatial Index Commands

`SPATIAL.*` command family (6 commands).

```
SPATIAL.ADD
SPATIAL.CREATE
SPATIAL.DELETE
SPATIAL.LIST
SPATIAL.NEARBY
SPATIAL.WITHIN
```

---
## Stage Commands

`STAGE.*` command family (5 commands).

```
STAGE.CREATE
STAGE.DELETE
STAGE.LIST
STAGE.NEXT
STAGE.PREV
```

---
## Stats Commands

`STATS.*` command family (5 commands).

```
STATS.ALL                            # uptime, connections, command, hit/miss, key, memory, traffic, error, eviction and expiration totals
STATS.CLIENTS                        # connected_clients, total_connections
STATS.CPU                            # used_cpu_sys, used_cpu_user
STATS.KEYSPACE                       # total_keys, per-type key counts, expires, avg_ttl (ms)
STATS.MEMORY                         # used_memory, used_memory_human, peak_memory, used_memory_rss, total_allocated, fragmentation_ratio
```

---
## Storex Commands

`STOREX.*` command family (5 commands).

```
STOREX.CREATE
STOREX.DELETE
STOREX.GET
STOREX.LIST
STOREX.PUT
```

---
## Streamproc Commands

`STREAMPROC.*` command family (5 commands).

```
STREAMPROC.CREATE
STREAMPROC.DELETE
STREAMPROC.PEEK
STREAMPROC.POP
STREAMPROC.PUSH
```

---
## Streamx Commands

`STREAMX.*` command family (4 commands).

```
STREAMX.CREATE
STREAMX.DELETE
STREAMX.READ
STREAMX.WRITE
```

---
## Switching Commands

`SWITCH.*` command family (5 commands).

```
SWITCH.LIST
SWITCH.OFF
SWITCH.ON
SWITCH.STATE
SWITCH.TOGGLE
```

---
## Append-only Tape Log Commands

`TAPE.*` command family (5 commands).

```
TAPE.CREATE
TAPE.DELETE
TAPE.READ
TAPE.SEEK
TAPE.WRITE
```

---
## Telemetry Commands

`TELEMETRY.*` command family (2 commands).

```
TELEMETRY.QUERY
TELEMETRY.RECORD
```

---
## Multi-tenancy Commands

`TENANT.*` command family (5 commands).

```
TENANT.CONFIG
TENANT.CREATE
TENANT.DELETE
TENANT.GET
TENANT.LIST
```

---
## Threshold Commands

`THRESHOLD.*` command family (4 commands).

```
THRESHOLD.CHECK
THRESHOLD.DELETE
THRESHOLD.LIST
THRESHOLD.SET
```

---
## Throttling Commands

`THROTTLE.*` command family (4 commands).

```
THROTTLE.CALL
THROTTLE.DELETE
THROTTLE.RESET
THROTTLE.SET
```

---
## Throttlex Commands

`THROTTLEX.*` command family (3 commands).

```
THROTTLEX.CHECK
THROTTLEX.CREATE
THROTTLEX.STATUS
```

---
## Timers Commands

`TIMER.*` command family (4 commands).

```
TIMER.CREATE
TIMER.DELETE
TIMER.LIST
TIMER.STATUS
```

---
## Token Management Commands

`TOKEN.*` command family (5 commands).

```
TOKEN.CREATE
TOKEN.DELETE
TOKEN.LIST
TOKEN.REFRESH
TOKEN.VALIDATE
```

---
## Token Bucket Commands

`TOKENBUCKET.*` command family (5 commands).

```
TOKENBUCKET.AVAILABLE
TOKENBUCKET.CONSUME
TOKENBUCKET.CREATE
TOKENBUCKET.DELETE
TOKENBUCKET.RESET
```

---
## Trace Commands

`TRACE.*` command family (5 commands).

```
TRACE.END
TRACE.GET
TRACE.LIST
TRACE.SPAN
TRACE.START
```

---
## Vector Clocks Commands

`VECTOR_CLOCK.*` command family (5 commands).

```
VECTOR_CLOCK.COMPARE
VECTOR_CLOCK.CREATE
VECTOR_CLOCK.GET
VECTOR_CLOCK.INCREMENT
VECTOR_CLOCK.MERGE
```

---
## Views Commands

`VIEW.*` command family (4 commands).

```
VIEW.CREATE
VIEW.DELETE
VIEW.GET
VIEW.LIST
```

---
## Cache Warming Commands

`WARM.*` command family (1 commands).

```
WARM.INVALIDATE
```

---
## Window Functions Commands

`WINDOW.*` command family (5 commands).

```
WINDOW.ADD
WINDOW.AGGREGATE
WINDOW.CREATE
WINDOW.DELETE
WINDOW.GET
```

*Last updated: 2026-10-12*
*CacheStorm v0.3.0*
*Total Commands: 1,238 (verified against the registered command set)*
