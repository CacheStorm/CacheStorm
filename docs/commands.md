# CacheStorm Commands Reference

CacheStorm registers about 1,600 commands across 50+ modules: the Redis 7.4 command set plus CacheStorm extensions. Compatibility with Redis is measured by the differential harness in `tests/compat`.

Every command name in this reference is verified against the registered command
set: 1,596 documented, 0 phantoms, 0 missing.

## Command Categories

| Category | Commands | Description |
|----------|----------|-------------|
| [Core Redis](#core-redis-commands) | 289 | Standard Redis commands |
| [JSON](#json-commands) | 30+ | JSON document operations |
| [Time Series](#time-series-commands) | 40+ | Time-series data |
| [Search](#search-commands) | 50+ | Full-text search |
| [Graph](#graph-commands) | 30+ | Graph database |
| [Probabilistic](#probabilistic-commands) | 20+ | Bloom filters, sketches |
| [Distributed](#distributed-commands) | 100+ | Cluster, replication |
| [Caching](#caching-commands) | 50+ | Cache management |
| [Scheduling](#scheduling-commands) | 40+ | Jobs, cron, timers |
| [Messaging](#messaging-commands) | 60+ | Pub/Sub, queues |
| [Resilience](#resilience-commands) | 138 | Circuit breakers, rate limits |
| [Data Processing](#data-processing-commands) | 100+ | Aggregation, joins |
| [Monitoring](#monitoring-commands) | 80+ | Metrics, alerts |
| [Security](#security-commands) | 40+ | ACL, encryption |
| [Machine Learning](#machine-learning-commands) | 80+ | Models, embeddings |
| [Namespace](#namespace-commands) | 5 | Multi-tenant namespaces |
| [Tag Management](#tag-based-invalidation-commands) | 10 | Tag-based cache invalidation |
| [Workflow](#workflow-commands) | 30+ | Workflows and state machines |
| [Advanced](#advanced-commands) | 100+ | Advanced data structures |
| [Encoding](#encoding-commands) | 40+ | Data encoding/decoding |
| [Digest](#digest-commands) | 20+ | Hashing and crypto |
| [Events](#event-commands) | 30+ | Event sourcing and webhooks |
| [Utilities](#utility-commands) | 100+ | Various utilities |
| [Integration](#integration-commands) | 100+ | External integrations |
| [String Processing](#string-processing-commands) | 20 | String analysis and text manipulation |
| [Sessions](#session-commands) | 13 | Server-side session storage |
| [CRDT](#crdt-commands) | 13 | Conflict-free replicated data types |
| [Validation](#validation-commands) | 14 | Reusable value validation rules |
| [Key Management](#key-management-commands) | 11 | Key lifecycle operations |
| [Batch](#batch-commands) | 11 | Grouped operation submission |
| [Vector](#vector-commands) | 10 | Vector embeddings and similarity search |
| [T-Digest](#t-digest-commands) | 10 | Streaming quantile sketches |
| [Message Queue](#message-queue-commands) | 10 | Durable queue with acknowledgements |
| [Jobs](#job-commands) | 10 | Background job scheduling |
| [Documents](#document-commands) | 10 | Document storage and queries |
| [Analytics](#analytics-commands) | 9 | Counters and aggregates |
| [Circuit Breakers](#circuit-breaker-commands) | 9 | Failure/success state tracking |
| [WebSocket](#websocket-commands) | 8 | WebSocket connections and rooms |
| [Service Discovery](#service-discovery-commands) | 8 | Service registration and discovery |
| [Profiler](#profiler-commands) | 8 | Command execution profiling |
| [MVCC](#mvcc-commands) | 8 | Multi-version concurrency control |

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
```

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
```

### HyperLogLog Commands

```
PFADD key [element ...]
PFCOUNT key [key ...]
PFMERGE destkey [sourcekey ...]
```

### Geo Commands

```
GEOADD key [NX|XX] [CH] longitude latitude member [longitude latitude member ...]
GEODIST key member1 member2 [M|KM|FT|MI]
GEOHASH key member [member ...]
GEOPOS key member [member ...]
GEORADIUS key longitude latitude radius M|KM|FT|MI [WITHCOORD] [WITHDIST] [WITHHASH] [COUNT count [ANY]] [ASC|DESC] [STORE key] [STOREDIST key]
GEORADIUSBYMEMBER key member radius M|KM|FT|MI [WITHCOORD] [WITHDIST] [WITHHASH] [COUNT count [ANY]] [ASC|DESC] [STORE key] [STOREDIST key]
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

```
JSON.SET key path value [NX|XX]
JSON.GET key [INDENT indent] [NEWLINE newline] [SPACE space] [path [path ...]]
JSON.DEL key [path]
JSON.MGET key [key ...] path
JSON.TYPE key [path]
JSON.NUMINCRBY key path number
JSON.NUMMULTBY key path number
JSON.STRAPPEND key path value
JSON.STRLEN key [path]
JSON.ARRAPPEND key path value [value ...]
JSON.ARRLEN key [path]
JSON.OBJKEYS key [path]
JSON.OBJLEN key [path]
```

---

## Time Series Commands

```
TS.CREATE key [RETENTION retentionTime] [ENCODING encoding] [CHUNK_SIZE size] [DUPLICATE_POLICY policy] [LABELS label value [label value ...]]
TS.ALTER key [RETENTION retentionTime] [CHUNK_SIZE size] [DUPLICATE_POLICY policy] [LABELS label value [label value ...]]
TS.ADD key timestamp value [RETENTION retentionTime] [ENCODING encoding] [CHUNK_SIZE size] [ON_DUPLICATE policy] [LABELS label value [label value ...]]
TS.MADD key timestamp value [key timestamp value ...]
TS.INCRBY key value [TIMESTAMP timestamp] [RETENTION retentionTime] [UNCOMPRESSED]
TS.DECRBY key value [TIMESTAMP timestamp] [RETENTION retentionTime] [UNCOMPRESSED]
TS.DEL key fromTimestamp toTimestamp
TS.RANGE key fromTimestamp toTimestamp [COUNT count] [AGGREGATION aggregationType timeBucket [BUCKETTIMESTAMP bt] [EMPTY]]
TS.REVRANGE key fromTimestamp toTimestamp [COUNT count] [AGGREGATION aggregationType timeBucket [BUCKETTIMESTAMP bt] [EMPTY]]
TS.GET key
TS.INFO key [DEBUG]
TS.QUERYINDEX filter [filter ...]
```

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

### Bloom Filter

```
BF.RESERVE key capacity [errorRate] [EXPANSION expansion] [NONSCALING]
BF.ADD key item
BF.MADD key item [item ...]
BF.EXISTS key item
BF.MEXISTS key item [item ...]
BF.INFO key
```

### Cuckoo Filter

```
CF.RESERVE key capacity [BUCKETSIZE bucketsize] [MAXITERATIONS maxiterations] [EXPANSION expansion]
CF.ADD key item
CF.DEL key item
CF.EXISTS key item
CF.INFO key
```

### Count-Min Sketch

```
CMS.INCRBY key item increment [item increment ...]
CMS.QUERY key item [item ...]
CMS.INFO key
```

### Top-K

```
TOPK.RESERVE key topk [width depth decay]
TOPK.ADD key item [item ...]
TOPK.QUERY key item [item ...]
TOPK.LIST key [WITHCOUNT]
TOPK.INFO key
```

---

## Resilience Commands

### Circuit Breaker

```
CIRCUITX.CREATE name threshold timeoutMs [halfOpenMax]
CIRCUITX.OPEN name
CIRCUITX.CLOSE name
CIRCUITX.STATUS name
CIRCUITX.METRICS name
CIRCUITX.RESET name
CIRCUITX.DELETE name
```

### Rate Limiter

```
RATELIMITER.CREATE name rate burst [DURATION duration]
RATELIMITER.TRY name [COST cost]
RATELIMITER.WAIT name [COST cost] [TIMEOUT timeout]
RATELIMITER.RESET name
RATELIMITER.STATUS name
RATELIMITER.DELETE name
```

### Retry

```
RETRY.CREATE name maxRetry backoffMs
RETRY.EXECUTE name command [args ...]
RETRY.STATUS name
RETRY.DELETE name
```

### Bulkhead

```
BULKHEAD.CREATE name maxConcurrent [MAX_WAIT maxWait]
BULKHEAD.ACQUIRE name [TIMEOUT timeout]
BULKHEAD.RELEASE name
BULKHEAD.STATUS name
BULKHEAD.DELETE name
```

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
JOINX.CREATE name   # type/condition args are rejected
JOINX.ADD name side value
JOINX.GET name
JOINX.DELETE name
```

### Partitioning

```
PARTITIONX.CREATE name count
PARTITIONX.ADD name key value
PARTITIONX.GET name index
PARTITIONX.REBALANCE name arg2
```

---


> **Not implemented:** `PARTITIONX.REBALANCE` always answers with an error.

## Machine Learning Commands

### Model Commands

```
MODEL.CREATE name type [PARAMS param value [param value ...]]
MODEL.TRAIN name dataKey [PARAMS param value ...]
MODEL.PREDICT name inputKey
MODEL.DELETE name
MODEL.LIST [PATTERN pattern]
MODEL.STATUS name
```

### Feature Commands

```
FEATURE.SET key field value
FEATURE.GET key field
FEATURE.DEL key field
FEATURE.INCR entity key delta
FEATURE.NORMALIZE entity method
FEATURE.VECTOR key [FIELDS field [field ...]]
```

### Embedding Commands

```
EMBEDDING.CREATE name dimensions [METRIC metric]
EMBEDDING.GET id
EMBEDDING.SEARCH name vector [K k] [THRESHOLD threshold]
EMBEDDING.SIMILAR name key [K k]
EMBEDDING.DELETE id
```

### Tensor Commands

```
TENSOR.CREATE name dim dim [dim ...] [| value ...]
TENSOR.GET name
TENSOR.ADD name1 name2 [OUT out]
TENSOR.MATMUL name1 name2 [OUT out]
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

### Workflow Management

```
WORKFLOW.CREATE id name
WORKFLOW.DELETE name
WORKFLOW.GET name
WORKFLOW.LIST [pattern]
WORKFLOW.START name [vars...]
WORKFLOW.PAUSE id
WORKFLOW.COMPLETE id
WORKFLOW.FAIL id errMsg
WORKFLOW.RESET id
WORKFLOW.NEXT id
WORKFLOW.SETVAR id key value
WORKFLOW.GETVAR id key
WORKFLOW.ADDSTEP id stepID name command timeout
```

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

### Templates

```
TEMPLATE.CREATE name content
TEMPLATE.DELETE name
TEMPLATE.GET name
TEMPLATE.INSTANTIATE templateName workflowID
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
DAG.CREATE name
DAG.DELETE name
DAG.ADDNODE name node [data]
DAG.ADDEDGE name from to [weight]
DAG.TOPO name
DAG.PARENTS name node
DAG.CHILDREN name node
DAG.LIST [pattern]
```

### Parallel Processing

```
PARALLEL.EXEC commands... [MAX max]
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

RING.CREATE name replicas
RING.ADD name node [weight]
RING.GET name key
RING.NODES name
RING.REMOVE name node

SEM.CREATE name count
SEM.ACQUIRE name permits
SEM.RELEASE name permits
SEM.TRYACQUIRE name permits
SEM.VALUE name
```

---

## Encoding Commands

```
MSGPACK.ENCODE data
MSGPACK.DECODE data

BSON.ENCODE data
BSON.DECODE data

URL.ENCODE string
URL.DECODE string

XML.ENCODE tag value [ROOT root]
XML.DECODE xml

YAML.ENCODE data
YAML.DECODE yaml

TOML.ENCODE key value
TOML.DECODE toml

CBOR.ENCODE data
CBOR.DECODE data

CSV.ENCODE rows [DELIMITER delim]
CSV.DECODE csv [DELIMITER delim]

UUID.GEN [VERSION version]
UUID.VALIDATE uuid
UUID.VERSION uuid

ULID.GEN
ULID.EXTRACT ulid

TIMESTAMP.NOW [UNIT unit]
TIMESTAMP.PARSE input
TIMESTAMP.FORMAT timestamp format
TIMESTAMP.ADD timestamp amount unit
TIMESTAMP.DIFF t1 t2 unit
TIMESTAMP.STARTOF timestamp unit
TIMESTAMP.ENDOF timestamp unit

DIFF.TEXT text1 text2
DIFF.JSON json1 json2

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
EVENT.EMIT name [key value ...]
EVENT.GET channel [cursor] [COUNT count]
EVENT.LIST [pattern]
EVENT.CLEAR channel
```

### Webhooks

```
WEBHOOK.CREATE id url method [EVENTS events...] [SECRET secret]
WEBHOOK.DELETE name
WEBHOOK.GET name
WEBHOOK.LIST [pattern]
WEBHOOK.ENABLE name
WEBHOOK.DISABLE name
WEBHOOK.STATS name
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
AUDIT.LOG command [key] [DETAILS details]
AUDIT.GET id [COUNT count]
AUDIT.GETRANGE start end
AUDIT.GETBYCMD command [COUNT count]
AUDIT.GETBYKEY key [COUNT count]
AUDIT.CLEAR [BEFORE timestamp]
AUDIT.COUNT [COMMAND command] [KEY key]
AUDIT.STATS
AUDIT.ENABLE
AUDIT.DISABLE
```

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
FLAG.ADDRULE name attribute operator value
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

### Backup

```
BACKUP.CREATE name [TYPE type]
BACKUP.RESTORE name [OPTIONS options]
BACKUP.LIST [pattern]
BACKUP.DELETE name
```

### Memory Management

```
MEMORY.TRIM
MEMORY.FRAG
MEMORY.PURGE
MEMORY.ALLOC size
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

### Network

```
NET.WHOIS domain
NET.DNS domain [TYPE type]
NET.PING host [COUNT count]
NET.PORT host port [TIMEOUT timeout]
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
ARRAY.FLATTEN key [DEPTH depth]
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
CRDT.GCOUNTER.INCR name nodeID [amount]
CRDT.GCOUNTER.GET key
CRDT.PNCounter.INCR name nodeID [amount]
CRDT.PNCounter.DECR name nodeID [amount]
CRDT.PNCounter.GET key
CRDT.GSET.ADD key member [member ...]
CRDT.GSET.GET key
CRDT.ORSET.ADD key member [member ...]
CRDT.ORSET.REMOVE key member
CRDT.ORSET.GET key
CRDT.LWW.SET key value [timestamp]
CRDT.LWW.GET key
CRDT.LWW.DELETE key
```

---

## Vector Commands

Vector embeddings: storage, similarity search, and index management.

```
VECTOR.CREATE name dim
VECTOR.ADD name id vector
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

Streaming quantile sketches and distribution summaries.

```
TDIGEST.CREATE key [COMPRESSION compression]
TDIGEST.ADD key value [value ...]
TDIGEST.MERGE destination source [source ...]
TDIGEST.QUANTILE key quantile
TDIGEST.CDF key value
TDIGEST.MIN key
TDIGEST.MAX key
TDIGEST.MEAN key
TDIGEST.RESET key
TDIGEST.INFO key
```

---

## Message Queue Commands

Durable publish/subscribe with explicit acknowledgement.

```
MSGQUEUE.CREATE name [MAXLEN count]
MSGQUEUE.PUBLISH name message
MSGQUEUE.CONSUME name [COUNT count]
MSGQUEUE.ACK name msgID
MSGQUEUE.NACK name msgID
MSGQUEUE.REQUEUE name msgID
MSGQUEUE.DEADLETTER name
MSGQUEUE.PURGE name
MSGQUEUE.DELETE name
MSGQUEUE.STATS name
```

---

## Job Commands

Background job scheduling and execution control.

```
JOB.CREATE id name command intervalMs
JOB.RUN name
JOB.GET name
JOB.LIST
JOB.UPDATE id intervalMs
JOB.DELETE name
JOB.ENABLE name
JOB.DISABLE name
JOB.RESET name
JOB.STATS name
```

---

## Document Commands

Document storage with indexing and aggregation queries.

```
DOC.INSERT name id field value [field value ...]
DOC.FIND name [filterField filterValue]
DOC.FINDONE name field value
DOC.UPDATE name id [FIELD field value ...]
DOC.DELETE name id
DOC.COUNT name [query]
DOC.AGGREGATE name pipeline
DOC.DISTINCT name field
DOC.INDEX name field [UNIQUE]
DOC.DROPINDEX name field
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
VALIDATE.CREATE name rule
VALIDATE.LIST
VALIDATE.DELETE name
VALIDATE.CHECK name value
VALIDATE.ALPHA value
VALIDATE.ALPHANUM value
VALIDATE.EMAIL value
VALIDATE.URL value
VALIDATE.IP value
VALIDATE.FLOAT value [min] [max]
VALIDATE.INT value [min] [max]
VALIDATE.LENGTH value min max
VALIDATE.RANGE value min max
VALIDATE.JSON value
```

---

## Key Management Commands

Key lifecycle operations that go beyond the core Redis surface.

```
KEY.COPY source destination [REPLACE]
KEY.RENAME key newkey
KEY.RENAMENX key newkey
KEY.MOVE key db
KEY.DUMP key
KEY.RESTORE key value [REPLACE] [TTL milliseconds]
KEY.ENCODE key
KEY.OBJECT subcommand key
KEY.REFCOUNT key
KEY.IDLETIME key
KEY.FREQ key
```

---

## Batch Commands

Submit and inspect grouped operations.

```
BATCH.SUBMIT name [PIPELINED]
BATCH.GET id
BATCH.STATUS id
BATCH.EXEC id
BATCH.CANCEL id
BATCH.MGET name [key ...]
BATCH.MSET key value [key value ...]
BATCH.MDEL name [key ...]
BATCH.SET key value [key value ...]
BATCH.DEL name [key ...]
BATCH.LIST
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

## WebSocket Commands

WebSocket connection and room management.

```
WS.CONNECT id [path]
WS.DISCONNECT id
WS.SEND id message
WS.BROADCAST room message
WS.JOIN id room
WS.LEAVE id room
WS.ROOMS
WS.LIST
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

## Profiler Commands

Capture and inspect command execution profiles.

```
PROFILE.START name [SAMPLE rate]
PROFILE.STOP
PROFILE.GET id
PROFILE.RESULT
PROFILE.SET id key value
PROFILE.LIST
PROFILE.CREATE user
PROFILE.DELETE id
```

---

## MVCC Commands

Multi-version concurrency control: snapshot reads and transactional writes.

```
MVCC.BEGIN [key key ...]
MVCC.COMMIT
MVCC.ROLLBACK
MVCC.SNAPSHOT
MVCC.STATUS
MVCC.GET id key
MVCC.SET id key value
MVCC.DELETE id key
```

---

## Extended Command Families

The sections below list every remaining registered command family.

Command names are reproduced exactly as registered in `internal/command`.

Argument signatures are not included; check the command handler for syntax.

---
## Unprefixed Commands

Commands without a dotted namespace prefix (57 commands).

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
DEBUGSEGFAULT
ECHO
EVAL
EVALSHA
EXPIREAT
FCALL
FCALL_RO
FUNCTION
GETDEL
GETEX
HGETDEL
HGETEX
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
RPUSHX
SCRIPT
SENTINEL
SETTAG
SINTERCARD
SMISMEMBER
SORT
SORT_RO
STRALGO
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
ALERT.ACKNOWLEDGE
ALERT.CREATE
ALERT.HISTORY
ALERT.LIST
ALERT.RESOLVE
ALERT.TRIGGER
```

---
## Anomaly Commands

`ANOMALY.*` command family (4 commands).

```
ANOMALY.CREATE
ANOMALY.DELETE
ANOMALY.DETECT
ANOMALY.LEARN
```

---
## Anti entropy Commands

`ANTI_ENTROPY.*` command family (4 commands).

```
ANTI_ENTROPY.DIFF
ANTI_ENTROPY.MERGE
ANTI_ENTROPY.STATUS
ANTI_ENTROPY.SYNC
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
## Async Execution Commands

`ASYNC.*` command family (4 commands).

```
ASYNC.CANCEL
ASYNC.RESULT
ASYNC.STATUS
ASYNC.SUBMIT
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
## Backupx Commands

`BACKUPX.*` command family (4 commands).

```
BACKUPX.CREATE
BACKUPX.DELETE
BACKUPX.LIST
BACKUPX.RESTORE
```

---
## Batchx Commands

`BATCHX.*` command family (5 commands).

```
BATCHX.ADD
BATCHX.CREATE
BATCHX.DELETE
BATCHX.EXECUTE
BATCHX.STATUS
```

---
## Beacon Commands

`BEACON.*` command family (4 commands).

```
BEACON.CHECK
BEACON.LIST
BEACON.START
BEACON.STOP
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

`CACHE.*` command family (7 commands).

```
CACHE.BULKDEL
CACHE.BULKGET
CACHE.CLEAR
CACHE.EXPORT
CACHE.IMPORT
CACHE.PREFETCH
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
## Canary Commands

`CANARY.*` command family (5 commands).

```
CANARY.CHECK
CANARY.CREATE
CANARY.DELETE
CANARY.LIST
CANARY.STATUS
```

---
## Chaining Commands

`CHAIN.*` command family (6 commands).

```
CHAIN.ADD
CHAIN.CREATE
CHAIN.GET
CHAIN.LAST
CHAIN.LENGTH
CHAIN.VALIDATE
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
## Chainx Commands

`CHAINX.*` command family (4 commands).

```
CHAINX.CREATE
CHAINX.DELETE
CHAINX.EXECUTE
CHAINX.LIST
```

---
## Circuitx Commands

`CIRCUITX.*` command family (1 commands).

```
CIRCUITX.HALFOPEN
```

---
## Classifier Commands

`CLASSIFIER.*` command family (4 commands).

```
CLASSIFIER.CREATE
CLASSIFIER.DELETE
CLASSIFIER.PREDICT
CLASSIFIER.TRAIN
```

---
## Clustering Commands

`CLUSTER.*` command family (5 commands).

```
CLUSTER.CENTROIDS
CLUSTER.CREATE
CLUSTER.DELETE
CLUSTER.FIT
CLUSTER.PREDICT
```

---
## Cms Commands

`CMS.*` command family (1 commands).

```
CMS.INIT
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
## Compact Commands

`COMPACT.*` command family (2 commands).

```
COMPACT.MERGE
COMPACT.STATUS
```

---
## Compression Commands

`COMPRESSION.*` command family (3 commands).

```
COMPRESSION.COMPRESS
COMPRESSION.DECOMPRESS
COMPRESSION.INFO
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
## Connectionx Commands

`CONNECTIONX.*` command family (4 commands).

```
CONNECTIONX.CREATE
CONNECTIONX.DELETE
CONNECTIONX.LIST
CONNECTIONX.STATUS
```

---
## Connection Pooling Commands

`CONPOOL.*` command family (5 commands).

```
CONPOOL.CREATE
CONPOOL.DELETE
CONPOOL.GET
CONPOOL.RETURN
CONPOOL.STATUS
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
## Cron Scheduler Commands

`CRON.*` command family (8 commands).

```
CRON.ADD
CRON.HISTORY
CRON.LIST
CRON.NEXT
CRON.PAUSE
CRON.REMOVE
CRON.RESUME
CRON.TRIGGER
```

---
## Datasets Commands

`DATASET.*` command family (5 commands).

```
DATASET.ADD
DATASET.CREATE
DATASET.DELETE
DATASET.GET
DATASET.SPLIT
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
DEBOUNCEX.CALL
DEBOUNCEX.CANCEL
DEBOUNCEX.CREATE
DEBOUNCEX.FLUSH
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
## Diagnostic Commands

`DIAGNOSTIC.*` command family (3 commands).

```
DIAGNOSTIC.LIST
DIAGNOSTIC.RESULT
DIAGNOSTIC.RUN
```

---
## Enrichment Commands

`ENRICH.*` command family (4 commands).

```
ENRICH.APPLY
ENRICH.CREATE
ENRICH.DELETE
ENRICH.LIST
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
EVAL.EXPR
EVAL.FORMAT
EVAL.JSONPATH
EVAL.REGEX
EVAL.REGEXMATCH
EVAL.REGEXREPLACE
EVAL.TEMPLATE
```

---
## Evaluator Commands

`EVALUATOR.*` command family (4 commands).

```
EVALUATOR.CREATE
EVALUATOR.DELETE
EVALUATOR.METRICS
EVALUATOR.RUN
```

---
## Event Sourcing Commands

`EVENTSOURCING.*` command family (4 commands).

```
EVENTSOURCING.APPEND
EVENTSOURCING.GET
EVENTSOURCING.REPLAY
EVENTSOURCING.SNAPSHOT
```

---
## Eventx Commands

`EVENTX.*` command family (5 commands).

```
EVENTX.CREATE
EVENTX.DELETE
EVENTX.EMIT
EVENTX.LIST
EVENTX.SUBSCRIBE
```

---
## Experimentation Commands

`EXPERIMENT.*` command family (6 commands).

```
EXPERIMENT.ASSIGN
EXPERIMENT.CREATE
EXPERIMENT.DELETE
EXPERIMENT.LIST
EXPERIMENT.RESULTS
EXPERIMENT.TRACK
```

---
## Fallback Commands

`FALLBACK.*` command family (3 commands).

```
FALLBACK.CREATE
FALLBACK.DELETE
FALLBACK.EXECUTE
```

---
## Filter Commands

`FILTER.*` command family (4 commands).

```
FILTER.APPLY
FILTER.CREATE
FILTER.DELETE
FILTER.LIST
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
FUTURE.CANCEL
FUTURE.COMPLETE
FUTURE.CREATE
FUTURE.GET
```

---
## Gateway Commands

`GATEWAY.*` command family (5 commands).

```
GATEWAY.CREATE
GATEWAY.DELETE
GATEWAY.LIST
GATEWAY.METRICS
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
## Gossip Protocol Commands

`GOSSIP.*` command family (5 commands).

```
GOSSIP.BROADCAST
GOSSIP.GET
GOSSIP.JOIN
GOSSIP.LEAVE
GOSSIP.MEMBERS
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
## Guard Commands

`GUARD.*` command family (4 commands).

```
GUARD.CHECK
GUARD.CREATE
GUARD.DELETE
GUARD.LIST
```

---
## Health Commands

`HEALTH.*` command family (3 commands).

```
HEALTH.CHECK
HEALTH.LIVENESS
HEALTH.READINESS
```

---
## Healthx Commands

`HEALTHX.*` command family (6 commands).

```
HEALTHX.CHECK
HEALTHX.HISTORY
HEALTHX.LIST
HEALTHX.REGISTER
HEALTHX.STATUS
HEALTHX.UNREGISTER
```

---
## Heap Commands

`HEAP.*` command family (8 commands).

```
HEAP.DELETE
HEAP.DUMP
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
## Hook Commands

`HOOK.*` command family (4 commands).

```
HOOK.CREATE
HOOK.DELETE
HOOK.LIST
HOOK.TRIGGER
```

---
## Hyperparam Commands

`HYPERPARAM.*` command family (4 commands).

```
HYPERPARAM.DELETE
HYPERPARAM.GET
HYPERPARAM.SEARCH
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
## Interceptor Commands

`INTERCEPTOR.*` command family (4 commands).

```
INTERCEPTOR.CHECK
INTERCEPTOR.CREATE
INTERCEPTOR.DELETE
INTERCEPTOR.LIST
```

---
## Jobx Commands

`JOBX.*` command family (5 commands).

```
JOBX.CREATE
JOBX.DELETE
JOBX.LIST
JOBX.RUN
JOBX.STATUS
```

---
## Json Commands

`JSON.*` command family (1 commands).

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
LEVEL.CREATE
LEVEL.DELETE
LEVEL.GET
LEVEL.LIST
LEVEL.SET
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
## Memo Commands

`MEMO.*` command family (5 commands).

```
MEMO.CACHE
MEMO.CLEAR
MEMO.INVALIDATE
MEMO.STATS
MEMO.WARM
```

---
## Memory Management Commands

`MEMORYX.*` command family (4 commands).

```
MEMORYX.ALLOC
MEMORYX.FREE
MEMORYX.STATS
MEMORYX.TRACK
```

---
## Merkle Trees Commands

`MERKLE.*` command family (5 commands).

```
MERKLE.ADD
MERKLE.CREATE
MERKLE.PROOF
MERKLE.ROOT
MERKLE.VERIFY
```

---
## Meter Commands

`METER.*` command family (5 commands).

```
METER.BILLING
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
## Middleware Commands

`MIDDLEWARE.*` command family (4 commands).

```
MIDDLEWARE.CREATE
MIDDLEWARE.DELETE
MIDDLEWARE.EXECUTE
MIDDLEWARE.LIST
```

---
## ML Experimentation Commands

`MLXPERIMENT.*` command family (5 commands).

```
MLXPERIMENT.COMPARE
MLXPERIMENT.CREATE
MLXPERIMENT.DELETE
MLXPERIMENT.LOG
MLXPERIMENT.METRICS
```

---
## Nlp Commands

`NLP.*` command family (4 commands).

```
NLP.ENTITIES
NLP.KEYWORDS
NLP.SUMMARIZE
NLP.TOKENIZE
```

---
## Notify Commands

`NOTIFY.*` command family (5 commands).

```
NOTIFY.CREATE
NOTIFY.DELETE
NOTIFY.LIST
NOTIFY.SEND
NOTIFY.TEMPLATE
```

---
## Observability Commands

`OBSERVABILITY.*` command family (4 commands).

```
OBSERVABILITY.LOG
OBSERVABILITY.METRIC
OBSERVABILITY.SPAN
OBSERVABILITY.TRACE
```

---
## Observable Commands

`OBSERVABLE.*` command family (5 commands).

```
OBSERVABLE.COMPLETE
OBSERVABLE.CREATE
OBSERVABLE.ERROR
OBSERVABLE.NEXT
OBSERVABLE.SUBSCRIBE
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

`PIPELINE.*` command family (7 commands).

```
PIPELINE.ADDSTAGE
PIPELINE.CREATE
PIPELINE.DELETE
PIPELINE.EXEC
PIPELINE.EXECUTE
PIPELINE.LIST
PIPELINE.STATUS
```

---
## Pipelineml Commands

`PIPELINEML.*` command family (4 commands).

```
PIPELINEML.ADD
PIPELINEML.CREATE
PIPELINEML.DELETE
PIPELINEML.RUN
```

---
## Pipelinex Commands

`PIPELINEX.*` command family (4 commands).

```
PIPELINEX.ADD
PIPELINEX.CANCEL
PIPELINEX.EXECUTE
PIPELINEX.START
```

---
## Plugins Commands

`PLUGIN.*` command family (5 commands).

```
PLUGIN.CALL
PLUGIN.INFO
PLUGIN.LIST
PLUGIN.LOAD
PLUGIN.UNLOAD
```

---
## Policy Commands

`POLICY.*` command family (4 commands).

```
POLICY.CHECK
POLICY.CREATE
POLICY.DELETE
POLICY.LIST
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
## Health Probes Commands

`PROBE.*` command family (5 commands).

```
PROBE.CREATE
PROBE.DELETE
PROBE.LIST
PROBE.RESULTS
PROBE.RUN
```

---
## Profilex Commands

`PROFILEX.*` command family (1 commands).

```
PROFILEX.LIST
```

---
## Promise Tracking Commands

`PROMISE.*` command family (5 commands).

```
PROMISE.AWAIT
PROMISE.CREATE
PROMISE.REJECT
PROMISE.RESOLVE
PROMISE.STATUS
```

---
## Proxy Commands

`PROXY.*` command family (4 commands).

```
PROXY.CREATE
PROXY.DELETE
PROXY.LIST
PROXY.ROUTE
```

---
## Query Engine Commands

`QUERY.*` command family (4 commands).

```
QUERY.CREATE
QUERY.DELETE
QUERY.EXECUTE
QUERY.LIST
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
## Raft Consensus Commands

`RAFT.*` command family (6 commands).

```
RAFT.APPEND
RAFT.COMMIT
RAFT.LEADER
RAFT.STATE
RAFT.TERM
RAFT.VOTE
```

---
## Rage Commands

`RAGE.*` command family (4 commands).

```
RAGE.RESET
RAGE.STATS
RAGE.STOP
RAGE.TEST
```

---
## Reactive Commands

`REACTIVE.*` command family (3 commands).

```
REACTIVE.TRIGGER
REACTIVE.UNWATCH
REACTIVE.WATCH
```

---
## Recommend Commands

`RECOMMEND.*` command family (4 commands).

```
RECOMMEND.CREATE
RECOMMEND.DELETE
RECOMMEND.GET
RECOMMEND.TRAIN
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
## Regressor Commands

`REGRESSOR.*` command family (4 commands).

```
REGRESSOR.CREATE
REGRESSOR.DELETE
REGRESSOR.PREDICT
REGRESSOR.TRAIN
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
## Replay Commands

`REPLAY.*` command family (5 commands).

```
REPLAY.SEEK
REPLAY.SPEED
REPLAY.START
REPLAY.STATUS
REPLAY.STOP
```

---
## Replayx Commands

`REPLAYX.*` command family (4 commands).

```
REPLAYX.PAUSE
REPLAYX.SPEED
REPLAYX.START
REPLAYX.STOP
```

---
## Report Commands

`REPORT.*` command family (4 commands).

```
REPORT.CREATE
REPORT.DELETE
REPORT.GENERATE
REPORT.LIST
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
## Rolex Commands

`ROLEX.*` command family (5 commands).

```
ROLEX.ASSIGN
ROLEX.CHECK
ROLEX.CREATE
ROLEX.DELETE
ROLEX.LIST
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
## Rules Commands

`RULE.*` command family (4 commands).

```
RULE.CREATE
RULE.DELETE
RULE.EVAL
RULE.LIST
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
## Schemas Commands

`SCHEMA.*` command family (4 commands).

```
SCHEMA.DELETE
SCHEMA.LIST
SCHEMA.REGISTER
SCHEMA.VALIDATE
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
## Sentiment Commands

`SENTIMENT.*` command family (2 commands).

```
SENTIMENT.ANALYZE
SENTIMENT.BATCH
```

---
## Sentinel Extension Commands

`SENTINELX.*` command family (5 commands).

```
SENTINELX.ALERTS
SENTINELX.CONFIG
SENTINELX.STATUS
SENTINELX.UNWATCH
SENTINELX.WATCH
```

---
## Sessionx Commands

`SESSIONX.*` command family (5 commands).

```
SESSIONX.CREATE
SESSIONX.DELETE
SESSIONX.GET
SESSIONX.LIST
SESSIONX.SET
```

---
## Sharding Commands

`SHARD.*` command family (5 commands).

```
SHARD.LIST
SHARD.MAP
SHARD.MOVE
SHARD.REBALANCE
SHARD.STATUS
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
## Similarity Commands

`SIMILARITY.*` command family (4 commands).

```
SIMILARITY.COSINE
SIMILARITY.DOTPRODUCT
SIMILARITY.EUCLIDEAN
SIMILARITY.JACCARD
```

---
## Sketch Commands

`SKETCH.*` command family (5 commands).

```
SKETCH.CREATE
SKETCH.DELETE
SKETCH.MERGE
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
STATS.ALL
STATS.CLIENTS
STATS.CPU
STATS.KEYSPACE
STATS.MEMORY
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
## SWIM Membership Commands

`SWIM.*` command family (5 commands).

```
SWIM.JOIN
SWIM.LEAVE
SWIM.MEMBERS
SWIM.PING
SWIM.SUSPECT
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
## Taskx Commands

`TASKX.*` command family (4 commands).

```
TASKX.CREATE
TASKX.DELETE
TASKX.LIST
TASKX.RUN
```

---
## Telemetry Commands

`TELEMETRY.*` command family (3 commands).

```
TELEMETRY.EXPORT
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
## Time Forecasting Commands

`TIMEFORECAST.*` command family (4 commands).

```
TIMEFORECAST.CREATE
TIMEFORECAST.DELETE
TIMEFORECAST.PREDICT
TIMEFORECAST.TRAIN
```

---
## Timeout Commands

`TIMEOUT.*` command family (3 commands).

```
TIMEOUT.CREATE
TIMEOUT.DELETE
TIMEOUT.EXECUTE
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
## Pub/Sub Topics Commands

`TOPIC.*` command family (6 commands).

```
TOPIC.HISTORY
TOPIC.LIST
TOPIC.PUBLISH
TOPIC.SUBSCRIBE
TOPIC.SUBSCRIBERS
TOPIC.UNSUBSCRIBE
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
## Transform Commands

`TRANSFORM.*` command family (4 commands).

```
TRANSFORM.APPLY
TRANSFORM.CREATE
TRANSFORM.DELETE
TRANSFORM.LIST
```

---
## Transx Commands

`TRANSX.*` command family (4 commands).

```
TRANSX.BEGIN
TRANSX.COMMIT
TRANSX.ROLLBACK
TRANSX.STATUS
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

`WARM.*` command family (4 commands).

```
WARM.INVALIDATE
WARM.PREFETCH
WARM.PRELOAD
WARM.STATUS
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
*Total Commands: 1,596 (verified against the registered command set)*
