# CacheStorm Commands Reference

CacheStorm registers about 1,200 commands across 50+ modules: the Redis 7.4 command set plus CacheStorm extensions. Compatibility with Redis is measured by the differential harness in `tests/compat`.

The stateless utility namespaces (`STR`, `MATH`, `TIMESTAMP`, `UUID`, `ULID`, `SNOWFLAKE`, `DIGEST`, `CRYPTO`, `COMPRESS`, `DECOMPRESS`, `CBOR`, `MSGPACK`, `CSV`, `XML`, `YAML`, `URL`, `DIFF`, `MASK`, `SANITIZE`, `EVAL`, `PARALLEL`) are served only with `extension_commands: all`; see "Extension commands" in 02-configuration.md.

Every command name in this reference is verified against the registered command
set: 1,289 documented, 0 phantoms, 0 missing.

Counts in the table below are the commands listed in each section; `SET` and `COMMAND` are also repeated
under Tag-Based Invalidation and Command Help but counted only once, under Core Redis.

## Command Categories

| Category | Commands | Description |
|----------|----------|-------------|
| [Core Redis](#core-redis-commands) | 208 | Standard Redis commands |
| [JSON](#json-commands) | 13 | JSON document operations |
| [Time Series](#time-series-commands) | 12 | Time-series data |
| [Search](#search-commands) | 38 | Full-text and vector search, auto-complete, spelling |
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
| **Total registered** | **1,289** | |

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

Redis 8's module parameters are there too: `search-*` (the RediSearch settings `FT.CONFIG` reads and writes, kept in step with it), `ts-*` (RedisTimeSeries: the retention, chunk size, duplicate policy, encoding and ignore thresholds new series take, and `ts-compaction-policy`, whose rules `TS.ADD` and `TS.INCRBY` give the series they create) and `bf-*`/`cf-*` (RedisBloom: the error rate, capacity, bucket size, iterations and expansions of the filters `BF.ADD` and `CF.ADD` create), with Redis 8.2's defaults, ranges and errors.

---

## JSON Commands

These follow RedisJSON, the JSON module built into Redis 8, and were checked against Redis 8.2. A document is a key of type `ReJSON-RL`, saved, replicated and expired like any key.

A path that starts with `$` is JSONPath: `.key`, `['key']`, `["a","b"]`, `*`, `..` (descendants), `[n]` and negative indexes, `[n,m]`, slices `[start:end:step]`, and filters such as `[?(@.price < 10 && @.tag == 'x')]` with `==`, `!=`, `<`, `<=`, `>`, `>=`, `=~` (regular expression), `&&` and `||`. Such a path answers per match: JSON.GET returns a JSON array of the matches, other commands an array with one reply per match (nil for a match of the wrong type). Any other path is the legacy syntax (`.a.b[0]`, `a.b`, `.` for the document) and answers for its first match, with an error when nothing matches. As in RedisJSON, an index before the start of an array selects its first element, and a filter applied to a value that is not an array or object tests the value itself.

Writes follow RedisJSON's rules. `JSON.SET`, `JSON.MERGE` and `JSON.MSET` on a path that selects nothing create the value only when the path is static (keys and single indexes) and ends in a key of an existing object; a path with wildcards, descent, slices or filters fails with `Err wrong static path`, one ending in an index with `ERR array index out of range`, and `XX` simply replies nil. A command writing several matches reaches each one again from the document root, so when an earlier match's change removed or retyped a later one it stops with `ERR Path does not exist` or `WRONGTYPE`, keeping the changes already made. `JSON.DEL` counts only the outermost of nested matches, and a deletion that leaves the document an empty object or array deletes the key. `JSON.CLEAR` counts only non-empty containers and non-zero numbers.

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

Samples stay in timestamp order; an older sample is inserted in place unless it falls outside the retention period. A duplicate timestamp follows the series' `DUPLICATE_POLICY` (`BLOCK` by default; `FIRST`, `LAST`, `MIN`, `MAX`, `SUM`) or `ON_DUPLICATE`. Aggregators are `avg`, `sum`, `min`, `max`, `range`, `count`, `first`, `last`, `std.p`, `std.s`, `var.p`, `var.s` and `twa`. A compaction rule aggregates the source's samples per bucket into the destination when a sample of a later bucket arrives; a sample added at or before the newest one, and `TS.DEL`, recompute the buckets they touch, as in RedisTimeSeries. Rules do not chain: a destination cannot be the source of another rule. `twa` interpolates each bucket's edges from the neighbouring samples, even outside the query range; with `EMPTY` it fills gaps from the line between the samples around them, stopping at the series' last sample where RedisTimeSeries would fill an open range until it runs out of memory. `TS.REVRANGE` aggregates each bucket newest first, so `last` there is the bucket's oldest sample, as RedisTimeSeries returns it. Filters are `label=value`, `label!=value`, `label=`, `label!=` and `label=(v1,v2)`; at least one must be a `label=value` matcher, and `TS.MGET`, `TS.MRANGE` and `TS.QUERYINDEX` scan the database's keys. Sample values reply as simple strings in RedisTimeSeries' format (`1.5`, `5E-1`, `1E16`) to RESP2 clients and as doubles to RESP3 clients, where `TS.MGET`, `TS.MRANGE` and `TS.INFO` reply with maps and `TS.QUERYINDEX` with a set. `TS.MRANGE` takes `FILTER` after its other options and `GROUPBY` last.

---

## Search Commands

These follow RediSearch, the search module built into Redis 8, and were checked against Redis 8.2. An index covers the HASH keys (or, with `ON JSON`, the JSON documents) under its prefixes: a document is searchable as soon as it is written and disappears when its key is deleted, expires or is evicted. As in Redis, indexes are created in database 0 only (`Cannot create index on db != 0`), and `FLUSHALL` or a `FLUSHDB` of any database drops every index. Index definitions and aliases are saved in snapshots and the AOF and replicated; the documents are the keys themselves.

```
FT.CREATE index [ON HASH|JSON] [PREFIX count prefix ...] [LANGUAGE lang] [SCORE score] [STOPWORDS count word ...]
    SCHEMA {identifier [AS attribute] {TEXT [WEIGHT w] [NOSTEM] | NUMERIC | TAG [SEPARATOR c] [CASESENSITIVE] | GEO
            | VECTOR {FLAT|HNSW} count TYPE {FLOAT32|FLOAT64|FLOAT16|BFLOAT16|INT8|UINT8} DIM n DISTANCE_METRIC {L2|IP|COSINE} [name value ...]}
            [SORTABLE] [NOINDEX] [INDEXMISSING]} ...
FT.ALTER index [SKIPINITIALSCAN] SCHEMA ADD field options ...
FT.DROPINDEX index [DD]
FT.DROP index [KEEPDOCS]
FT._CREATEIFNX | FT._ALTERIFNX | FT._DROPIFX | FT._DROPINDEXIFX | FT._ALIASADDIFNX | FT._ALIASDELIFX ...
FT.SEARCH index query [NOCONTENT] [VERBATIM] [NOSTOPWORDS] [WITHSCORES] [WITHPAYLOADS] [WITHSORTKEYS] [FILTER field min max] [GEOFILTER field lon lat radius m|km|mi|ft]
    [INKEYS count key ...] [INFIELDS count field ...] [RETURN count field [AS name] ...] [SORTBY field [ASC|DESC]] [LIMIT offset num] [PARAMS count name value ...] [DIALECT n]
    [WITHCURSOR [COUNT n] [MAXIDLE ms]]
FT.AGGREGATE index query [VERBATIM] [ADDSCORES] [LOAD count field [AS name] ... | LOAD *] [GROUPBY count property ... [REDUCE fn count arg ... [AS name]] ...]
    [SORTBY count property [ASC|DESC] ... [MAX n]] [APPLY expression AS name] [FILTER expression] [LIMIT offset num] [PARAMS count name value ...] [DIALECT n]
    [WITHCURSOR [COUNT n] [MAXIDLE ms]]
FT.CURSOR READ index cursor [COUNT n] | DEL index cursor | GC index 0
FT.SYNUPDATE index group [SKIPINITIALSCAN] term ...
FT.SYNDUMP index
FT.SYNADD index term ...                 # refused, as in Redis 8.2: FT.SYNUPDATE replaces it
FT.SPELLCHECK index query [DISTANCE 1-4] [TERMS INCLUDE|EXCLUDE dict] ... [DIALECT n] [PARAMS count name value ...]
FT.DICTADD dict term [term ...]
FT.DICTDEL dict term [term ...]
FT.DICTDUMP dict
FT.SUGADD key string score [INCR] [PAYLOAD payload]
FT.SUGGET key prefix [FUZZY] [MAX n] [WITHSCORES] [WITHPAYLOADS] [TRIM]
FT.SUGDEL key string
FT.SUGLEN key
FT.EXPLAIN index query [DIALECT n] [PARAMS count name value ...]
FT.EXPLAINCLI index query [DIALECT n] [PARAMS count name value ...]
FT.PROFILE index SEARCH|AGGREGATE [LIMITED] QUERY query [args ...]
FT.MGET index key [key ...]
FT.CONFIG GET|HELP option|* | SET option value
FT.TAGVALS index field
FT.ALIASADD alias index
FT.ALIASUPDATE alias index
FT.ALIASDEL alias
FT.INFO index
FT._LIST
```

The default dialect (1) is parsed with RediSearch's own grammar: its LALR tables are generated from the RediSearch 8.2 grammar with lemon's precedence rules (`scripts/gen_search_parser.py`), so queries group as in Redis — `a b|c` is `(a b)|c`, `-a b` is `NOT(a b)`, `@f:a b` scopes both words — and syntax errors name the same offset and text. It has terms, `a|b`, `-expr`, `~expr` (optional), `"exact phrase"`, `prefix*`, `*suffix`, `*infix*`, `%fuzzy%` (one to three `%` for the edit distance), parentheses, `@field:expr`, `@a|b:expr`, `@num:[min max]` with `(` for exclusive bounds and `-inf`/`+inf`, `@tag:{a | b}`, `@geo:[lon lat radius unit]`, `expr => {$weight: n}`, and `*` for every document. `FT.EXPLAIN` prints the parse tree as Redis does. `DIALECT 2` and later are parsed the same way from RediSearch's second grammar (checked against Redis on 4,000 random queries): `$name` parameters from `PARAMS`, `@num>5`-style comparisons, `ismissing(@field)`, `w'pattern'` wildcards, quoted phrases that keep their stop words, and vector queries. A VECTOR field indexes a blob of `DIM` numbers of its `TYPE` (or a JSON array); a key whose blob has the wrong size, like one whose NUMERIC field is not a number, is left out of the index, as in Redis. `*=>[KNN k @field $blob [AS name]]` returns the k nearest documents, or `(filter)=>[KNN ...]` the k nearest of those the filter matches, with the distance as `__field_score` (or `name`); `@field:[VECTOR_RANGE radius $blob]` matches the documents within a distance and yields it with `=>{$YIELD_DISTANCE_AS: name}`. Distances are squared L2, `1 - inner product`, or cosine distance of unit vectors, computed as RediSearch's vector library computes them — in order below 16 dimensions and in the AVX or AVX-512 kernels' lane order above — so they agree with Redis on the same CPU to the last bit. FLAT and HNSW indexes are both searched exactly. Terms are lower-cased and English stop words dropped; a stop word may not open a quoted phrase, as in Redis. Words are expanded with their synonym groups (`FT.SYNUPDATE`; a document indexes `~group` with each word of a group, and a query adds it to the word's union) and their Snowball English (Porter2) stems unless `VERBATIM`, as RediSearch's expander adds them (`+stem` and the stem). Without `SKIPINITIALSCAN`, `FT.SYNUPDATE` indexes the documents again; `FT.SYNDUMP` lists the terms in the order they were added, where Redis uses its hash table's order. Results are ranked with RediSearch's default BM25STD scorer, computed as Redis computes it (with glibc's correctly rounded logarithm for the IDF): field-weighted term frequencies and document lengths, single-precision constants, a prefix or fuzzy term scoring its first matching term in term order, and tags scored per value. Documents with equal scores, and equal SORTBY values, come in document id order, the order their keys were last written. A document's fields are replied in the order the query first met them, the SORTBY field first, and RESP3 clients get RediSearch's map replies. Aggregation reducers are `COUNT`, `COUNT_DISTINCT`, `COUNT_DISTINCTISH`, `SUM`, `AVG`, `MIN`, `MAX`, `STDDEV`, `QUANTILE`, `TOLIST`, `FIRST_VALUE [BY property [ASC|DESC]]` and `RANDOM_SAMPLE`, ported from RediSearch (its quantile stream, 8-bit HyperLogLog and Welford deviation), and groups come in the order RediSearch's hash table holds them. `APPLY` and `FILTER` expressions are lexed, parsed and evaluated as RediSearch does: a signed number is one token (`@n-1` is a syntax error, `@n - 1` is not), unknown punctuation is skipped, comparisons do not chain, `NULL` is a value, strings compare as strings and against numbers as numbers (or fail with `Error converting string 'x' to number`), `%` is `fmod`, and the functions are RediSearch's: `lower`, `upper`, `substr`, `format`, `split`, `to_number`, `to_str`, `exists`, `case`, `startswith`, `contains`, `strlen`, `log`, `log2`, `exp`, `floor`, `ceil`, `abs`, `sqrt`, `timefmt`, `parsetime`, `hour`, `minute`, `day`, `month`, `monthofyear`, `year`, `dayofmonth`, `dayofweek`, `dayofyear`, `geodistance` and `matched_terms` (the query terms the document matched, null after a sort or a group). The evaluator's error status carries from row to row, as in RediSearch: an error set before the first row is replied comes back as the reply, a later one ends the rows. The SORTBY and LIMIT between two GROUPBYs form one step, run where the first of them stands, as in RediSearch: a SORTBY keeps the first offset+num rows, or 10 without a LIMIT or MAX, and a FILTER after a LIMIT filters the page. A step may read only what earlier steps loaded or made, the query's yields, and (before the first GROUPBY) SORTABLE fields, or any field after `LOAD *`; SORTBY, GROUPBY and reducers also load schema fields themselves. Anything else is refused before the query runs with RediSearch's errors (``Property `x` not loaded nor in pipeline``, ``No such property `x` ``), and a row's fields come in the order the pipeline made them, the fields a step read first. An expression that reads a property a row lacks stops the reply at that row, as in Redis. `WITHCURSOR` replies with the first `COUNT` rows (1000 by default) and a cursor id that `FT.CURSOR READ` continues from; as in RediSearch, a streamed aggregation's chunk counts the documents read for it, a sorted or grouped one and FT.SEARCH send their total with the first chunk, documents written or deleted between reads are skipped, an index keeps at most 128 cursors, and idle cursors close after `MAXIDLE` (at most 300 seconds) when cursors are next swept or on `FT.CURSOR GC`. Scores of queries that combine clauses are computed as RediSearch's iterators compute them (a port of its union, intersection, NOT, optional and term readers): an intersection sums its children in the order of their estimated sizes, and a union that a skip passed keeps one child's match, so a document matching a word and its stem can score as either alone, as in Redis. Known differences: `FT._LIST` and `FT.INFO`'s statistics come in a different order or form, the date functions of a timestamp C cannot convert (infinite or NaN) return null where RediSearch reads uninitialized memory; `EXPLAINSCORE` is ignored.

Each index keeps a compact parsed form of its documents (index terms with their weighted frequencies, numbers, tags, and the raw values of SORTABLE fields) with term and tag postings, built on the first query and updated as keys are written, so a query reads only the documents its terms and tags select; a numeric or geo filter alone, or a query that matches most documents, still examines every document of the index. That takes roughly 600–800 bytes per small document; `INFO search` reports it as `search_used_memory_indexes`. It is not counted in `used_memory`, so `maxmemory` does not cover it. Sorting by a field that is neither SORTABLE nor NUMERIC reads each matching key. `FT.CONFIG` has RediSearch 8.2's 59 options with their help texts, defaults and validation; `DEFAULT_DIALECT`, `MINPREFIX`, `MAXPREFIXEXPANSIONS`, `MINSTEMLEN`, `UNION_ITERATOR_HEAP`, `CURSOR_MAX_IDLE`, `INDEX_CURSOR_LIMIT`, `MAXSEARCHRESULTS` and `MAXAGGREGATERESULTS` take effect, the others are kept for clients that read them. `FT.PROFILE` runs a query and describes it as RediSearch does: the iterator tree with each iterator's reads and estimated matches, and the result processors with the results each passed on, counted as RediSearch's pull-driven pipeline counts them (a `LIMIT` or a failing `FILTER` stops the reading early); times are measured for the whole command and its parsing only. `FT.DROP` deletes the documents unless `KEEPDOCS`; the `_IFNX`/`_IFX` variants succeed when there is nothing to do, as RediSearch's coordinator expects. `FT.SPELLCHECK` suggests, for each query term the index lacks (stop words included, as RediSearch parses the query without a stop word list), the index terms and INCLUDE dictionary terms within DISTANCE edits, as RediSearch's Levenshtein automaton finds them (a word whose last letter is one edit too many still matches), scored by the documents holding them; EXCLUDE dictionaries silence a term. Dictionaries (`FT.DICTADD`) belong to the server, persist and replicate, and FLUSHALL clears them, as in Redis. `FT.SUGADD` keeps an auto-complete dictionary in a key of type `trietype0`, RediSearch's trie: `FT.SUGGET` returns the best five (or `MAX`) strings starting with the prefix, compared case-folded, or within one edit with `FUZZY`, scored and ordered as Redis scores and orders them (an exact match first, longer strings lower, fuzzy matches lower again); checked against Redis 8.2 on random dictionaries. The dictionary is saved, replicated and dumped like other keys. `FT.ADD`, `FT.DEL` and `FT.GET`, which RediSearch 2 removed, remain for old AOF files: `FT.ADD` writes the document as a hash at its id.

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

These follow RedisBloom, the probabilistic module built into Redis 8: the same syntax, replies and error texts (some of which, as in RedisBloom, carry no `ERR` prefix). Each structure is a key of its own type (`TYPE` reports `MBbloom--`, `MBbloomCF`, `CMSk-TYPE`, `TopK-TYPE` and `TDIS-TYPE` for t-digests). They live in the selected database, expire, are deleted and flushed like any key, and are saved in snapshots and the AOF, replicated, and moved by DUMP/RESTORE and MIGRATE. The structures are RedisBloom's own: the same MurmurHash functions, sizes, layouts, eviction and compression, so each false positive, each item a top-k list drops and each t-digest centroid is the one Redis 8.2 gives (top-k decay draws random numbers, so with a decay below 1 the drops can still differ). RESP3 clients get booleans, maps and doubles where RedisBloom sends them. One structure may take at most 512 MB; a larger request fails with `ERR Insufficient memory to create filter`.

### Bloom Filter

A scalable filter: the first sub-filter has half the requested error rate (capped at 0.25) and, when the newest holds its capacity, a larger one (by the expansion factor, 2 by default) with half the error rate of the last is added. `NONSCALING` or `EXPANSION 0` makes a full filter refuse new items. `BF.ADD`/`BF.MADD` create a filter of capacity 100 at error rate 0.01 when the key is absent.

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

One-byte fingerprints in buckets of `BUCKETSIZE` (2 by default); the bucket count is capacity divided by the bucket size, rounded to a power of two, and the expansion is rounded up to one too. When an item finds no place after `MAXITERATIONS` relocations, a filter `EXPANSION` times larger is added, or with `EXPANSION 0` the add fails with `Filter is full` (`CF.INSERT` and `CF.INSERTNX` reply `-1` for that item instead). Deleting compacts later filters into earlier ones once deletions pass a tenth of the items; a filter may have at most 32 sub-filters. `CF.ADD` creates a filter of capacity 1000 when the key is absent.

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

32-bit counters; an increment past the maximum leaves the counter there, and an item whose estimate reaches the maximum is reported as `CMS: INCRBY overflow`. A `CMS.MERGE` whose weighted sums leave the counters' range fails with `CMS: MERGE overflow` and changes nothing.

```
CMS.INITBYDIM key width depth
CMS.INITBYPROB key error probability
CMS.INCRBY key item increment [item increment ...]
CMS.QUERY key item [item ...]
CMS.MERGE destination numkeys source [source ...] [WEIGHTS weight [weight ...]]
CMS.INFO key
```

### Top-K

HeavyKeeper with `depth` rows of `width` decaying counters (defaults 8, 7 and 0.9) and RedisBloom's min-heap of the `topk` items. `TOPK.ADD` and `TOPK.INCRBY` reply with the item each addition pushed out of the list, or nil. `TOPK.COUNT` of a listed item counts only the counters at or above the list's smallest count.

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
RATELIMITER.CREATE name limit window_ms [strategy]   # at most limit (1-100000) requests in any window; creates or replaces; OK
RATELIMITER.TRY name                 # [allowed(1|0), remaining, window_ms]
RATELIMITER.WAIT name timeout_ms     # blocks until a slot frees; ms waited, or null on timeout
RATELIMITER.RESET name               # clear recorded requests; OK
RATELIMITER.STATUS name              # name, limit, current, strategy
RATELIMITER.DELETE name               # 1 if deleted, 0 if absent
# A RATELIMITER is a hash key (limit, window, strategy, next, and one field per
# slot holding a request time in ms): per database, persisted, replicated, and
# removed by DEL and FLUSHALL. A request is recorded as one small HSET.
# WAIT does not wait inside MULTI or scripts.
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

A queue or a stack is a list key: a queue pushes at the tail and pops at the head, a stack pushes and pops at the tail. Being keys, they belong to the selected database, are saved in the RDB and the AOF, are replicated, and go with `DEL`, `FLUSHDB` and `FLUSHALL`; the list commands (`LRANGE`, `LLEN`, `BLPOP`, ...) work on them, and a name holding another type answers `WRONGTYPE`. `CREATE` only checks the name, since a list exists while it has elements; `CLEAR` deletes it.

```
QUEUE.CREATE name                # OK
QUEUE.PUSH name item [item ...]  # length after the push (RPUSH)
QUEUE.POP name                   # head item or null (LPOP)
QUEUE.PEEK name                  # head item or null
QUEUE.LEN name
QUEUE.CLEAR name                 # OK

STACK.CREATE name
STACK.PUSH name item [item ...]  # RPUSH
STACK.POP name                   # RPOP
STACK.PEEK name                  # top item or null
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

A lock is a hash key with the fields `holder`, `token` and `renewals` that expires with the lock, as a Redis lock is a key set with `NX PX`. It belongs to the selected database, is saved in the RDB and the AOF with its absolute deadline (recorded as `HSET` and `PEXPIREAT`, so a restart or a replica keeps the deadline it was given), is replicated, and is released by `DEL`, `FLUSHDB` and `FLUSHALL`; `PTTL` and `HGETALL` read it. A key holding anything but a lock answers `WRONGTYPE`. TTLs are positive milliseconds.

```
LOCK.TRY key holder token ttlMs                # 1 taken (or renewed by its holder), 0 held by another
LOCK.ACQUIRE key holder token ttlMs timeoutMs  # as TRY, waiting up to timeoutMs (0 = no wait; no wait inside MULTI or scripts)
LOCK.RELEASE key holder token                  # 1 released, 0 not held by this holder and token
LOCK.RENEW key holder token ttlMs              # 1 renewed, 0 not held by this holder and token
LOCK.INFO key                                  # holder, expires_at (Unix s), ttl_ms, renewals; null when free
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

Server-side session storage. A session is a hash key, named by its id, that expires with the session: its data fields sit beside a marker field, `__session_created` (the creation time in ms), which `SESSION.ALL` and `SESSION.GET` hide and `SESSION.SET` refuses. It belongs to the selected database, is saved in the RDB and the AOF with its absolute deadline (recorded as `HSET`, `HDEL`, `DEL` and `PEXPIREAT`), is replicated, and goes away at its deadline or with `DEL` and `FLUSHALL`. A key holding anything but a session answers `WRONGTYPE`. TTLs are positive milliseconds.

```
SESSION.CREATE id ttlMs        # create or replace, empty; [id, ttl]
SESSION.GET id field           # value or null
SESSION.SET id field value     # OK; keeps the deadline
SESSION.DELETE id [field]      # 1 removed, 0 not there (SESSION.DEL is the same)
SESSION.EXISTS id
SESSION.TTL id                 # ms left, -2 for no session
SESSION.REFRESH id ttlMs       # new deadline; OK
SESSION.ALL id                 # field, value, ...; null for no session
SESSION.CLEAR id               # remove the data, keep the session and deadline; OK
SESSION.LIST                   # sessions of the selected database (walks its keys, like KEYS)
SESSION.COUNT
SESSION.CLEANUP                # 0: expired sessions are removed by their expiry
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

Streaming quantile sketches using RedisBloom syntax, replies and error texts (doubles as bulk strings, or RESP3 doubles), computed as RedisBloom's t-digest computes them. A missing key is an error. A t-digest is a key of type `TDIS-TYPE`, saved, replicated and expired like the other probabilistic structures.

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
*Total Commands: 1,289 (verified against the registered command set)*
