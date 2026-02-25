# CacheStorm Commands Reference

CacheStorm implements **1,606 commands** across 50+ modules, providing ~99% Redis compatibility with extensive extensions.

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
ZREMZRANGEBYRANK key start stop
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
BITFIELD_RO key [GET type offset]
```

### HyperLogLog Commands

```
PFADD key element [element ...]
PFCOUNT key [key ...]
PFMERGE destkey sourcekey [sourcekey ...]
PFDEBUG subcommand key
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
OBJECT subcommand [arguments]
TOUCH key [key ...]
UNLINK key [key ...]
WAIT numreplicas timeout
WAITAOF numlocal numreplicas timeout
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
SUNSUBSCRIBE [shardchannel [shardchannel ...]]
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
JSON.FORGET key [path]
JSON.RESP key [path]
JSON.DEBUG MEMORY key [path]
JSON.DEBUG HELP
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
TS.CREATERULE sourceKey destKey AGGREGATION aggregationType timeBucket
TS.DELETERULE sourceKey destKey
TS.RANGE key fromTimestamp toTimestamp [COUNT count] [AGGREGATION aggregationType timeBucket [BUCKETTIMESTAMP bt] [EMPTY]]
TS.REVRANGE key fromTimestamp toTimestamp [COUNT count] [AGGREGATION aggregationType timeBucket [BUCKETTIMESTAMP bt] [EMPTY]]
TS.MRANGE fromTimestamp toTimestamp [COUNT count] [AGGREGATION aggregationType timeBucket [BUCKETTIMESTAMP bt] [EMPTY]] [WITHLABELS] [SELECTED_LABELS label [label ...]] [FILTER filter [filter ...]] [GROUPBY label REDUCE reducer]
TS.MREVRANGE fromTimestamp toTimestamp [COUNT count] [AGGREGATION aggregationType timeBucket [BUCKETTIMESTAMP bt] [EMPTY]] [WITHLABELS] [SELECTED_LABELS label [label ...]] [FILTER filter [filter ...]] [GROUPBY label REDUCE reducer]
TS.GET key
TS.MGET [WITHLABELS] [SELECTED_LABELS label [label ...]] FILTER filter [filter ...]
TS.INFO key [DEBUG]
TS.QUERYINDEX filter [filter ...]
```

---

## Search Commands

```
FT.CREATE index [ON HASH|JSON] [PREFIX count prefix [prefix ...]] [SCHEMA field type [field type ...]]
FT.DROPINDEX index [DD]
FT.LIST
FT.ADD index docId score [NOSAVE] [REPLACE] [PARTIAL] [LANGUAGE lang] [PAYLOAD payload] [FIELDS field value [field value ...]]
FT.DEL index docId [DD]
FT.GET index docId
FT.MGET index [index ...] docId [docId ...]
FT.SEARCH index query [NOCONTENT] [VERBATIM] [NOSTOPWORDS] [WITHSCORES] [WITHPAYLOADS] [WITHSORTKEYS] [FILTER numericField min max [FILTER numericField min max ...]] [GEOFILTER geoField lon lat radius M|KM|MI|FT] [INKEYS count key [key ...]] [INFIELDS count field [field ...]] [RETURN count field [field ...]] [SUMMARIZE [FIELDS count field [field ...]] [FRAGS num] [LEN fragsize] [SEPARATOR separator]] [HIGHLIGHT [FIELDS count field [field ...]] [TAGS open close]] [SLOP slop] [TIMEOUT timeout] [LIMIT offset num]
FT.AGGREGATE index query [VERBATIM] [LOAD count field [field ...]] [GROUPBY nargs property [property ...] [REDUCE func nargs arg [arg ...] [AS name]] [REDUCE ...]] [SORTBY nargs [property [ASC|DESC] [property [ASC|DESC] ...]] [MAX num] [AS name]] [APPLY expr AS name] [FILTER expr] [LIMIT offset num] [QUERY_EXECUTION_TIME] [TIMEOUT timeout]
FT.CURSOR READ index cursor [COUNT count]
FT.CURSOR DEL index cursor
FT.SUGADD index term score [INCR] [PAYLOAD payload]
FT.SUGGET index prefix [FUZZY] [WITHSCORES] [WITHPAYLOADS] [MAX max]
FT.SUGDEL index term
FT.SUGLEN index
FT.SYNUPDATE index synonymGroupId [SKIPINITIALSCAN] term [term ...]
FT.SYNDUMP index
FT.SPELLCHECK index query [DISTANCE dist] [TERMS {INCLUDE | EXCLUDE} dict [TERMS {INCLUDE | EXCLUDE} dict ...]] [FULLSCOREINFO] [TIMEOUT timeout]
FT.DICTADD dict term [term ...]
FT.DICTDEL dict term [term ...]
FT.DICTDUMP dict
FT.TAGVALS index field
FT.EXPLAIN index query [DIALECT dialect]
FT.EXPLAINCLI index query [DIALECT dialect]
FT.ALTER index SCHEMA ADD field type [field type ...]
FT.ALIASADD name index
FT.ALIASUPDATE name index
FT.ALIASDEL name
FT.INFO index
FT._LIST
FT.CONFIG GET option [option ...]
FT.CONFIG SET option value
FT.CONFIG HELP [option]
```

---

## Graph Commands

```
GRAPH.QUERY graphName query [TIMEOUT timeout] [READWRITE]
GRAPH.RO_QUERY graphName query [TIMEOUT timeout]
GRAPH.EXPLAIN graphName query
GRAPH.PROFILE graphName query [TIMEOUT timeout]
GRAPH.DELETE graphName
GRAPH.SLOWLOG graphName
GRAPH.CONFIG GET name
GRAPH.CONFIG SET name value
GRAPH.LIST
GRAPH.CONSTRAINT CREATE graphName constraintType nodeLabel|relType properties [entityType entityType]
GRAPH.CONSTRAINT DROP graphName constraintType nodeLabel|relType properties [entityType entityType]
```

---

## Probabilistic Commands

### Bloom Filter

```
BF.RESERVE key errorRate capacity [EXPANSION expansion] [NONSCALING]
BF.ADD key item
BF.MADD key item [item ...]
BF.INSERT key [CAPACITY capacity] [ERROR error] [EXPANSION expansion] [NOCREATE] [NONSCALING] ITEMS item [item ...]
BF.EXISTS key item
BF.MEXISTS key item [item ...]
BF.SCANDUMP key iterator
BF.LOADCHUNK key iterator data
BF.INFO key
BF.DEBUG key
```

### Cuckoo Filter

```
CF.RESERVE key capacity [BUCKETSIZE bucketsize] [MAXITERATIONS maxiterations] [EXPANSION expansion]
CF.ADD key item
CF.ADDNX key item
CF.INSERT key [CAPACITY capacity] [NOCREATE] ITEMS item [item ...]
CF.INSERTNX key [CAPACITY capacity] [NOCREATE] ITEMS item [item ...]
CF.DEL key item
CF.COUNT key item
CF.EXISTS key item
CF.MEXISTS key item [item ...]
CF.SCANDUMP key iterator
CF.LOADCHUNK key iterator data
CF.INFO key
```

### Count-Min Sketch

```
CMS.INITBYDIM key width depth
CMS.INITBYPROB key error probability
CMS.INCRBY key item increment [item increment ...]
CMS.QUERY key item [item ...]
CMS.MERGE dest numKeys key [key ...] [WEIGHTS weight [weight ...]]
CMS.INFO key
```

### Top-K

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
CIRCUITX.CREATE name [FAILURE_THRESHOLD threshold] [SUCCESS_THRESHOLD threshold] [TIMEOUT timeout] [HALF_OPEN_MAX max]
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
RETRY.CREATE name maxAttempts [BACKOFF_TYPE type] [INITIAL_DELAY delay] [MAX_DELAY delay]
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
WINDOWX.AGGREGATE name TYPE type
```

### Stream Joins

```
JOINX.CREATE name type [CONDITION condition]
JOINX.ADD stream value
JOINX.GET name
JOINX.DELETE name
```

### Partitioning

```
PARTITIONX.CREATE name count [STRATEGY strategy]
PARTITIONX.ADD name key value
PARTITIONX.GET name key
PARTITIONX.REBALANCE name
```

---

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
FEATURE.INCR key field [BY increment]
FEATURE.NORMALIZE key [METHOD method]
FEATURE.VECTOR key [FIELDS field [field ...]]
```

### Embedding Commands

```
EMBEDDING.CREATE name dimensions [METRIC metric]
EMBEDDING.GET name key
EMBEDDING.SEARCH name vector [K k] [THRESHOLD threshold]
EMBEDDING.SIMILAR name key [K k]
EMBEDDING.DELETE name key
```

### Tensor Commands

```
TENSOR.CREATE name shape [DTYPE dtype] [DATA data]
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
TAGDEL tag [tag ...]
INVALIDATE tag [tag ...]
TAGEXISTS tag
TAGTTL tag
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
NAMESPACE name                    # Switch to namespace
NAMESPACES                        # List all namespaces
NAMESPACEDEL name                 # Delete a namespace
NAMESPACEINFO [name]              # Get namespace info
SELECT index                      # Select database (alias for namespace)
```

---

## Workflow Commands

### Workflow Management

```
WORKFLOW.CREATE name [description]
WORKFLOW.DELETE name
WORKFLOW.GET name
WORKFLOW.LIST [pattern]
WORKFLOW.START name [vars...]
WORKFLOW.PAUSE id
WORKFLOW.COMPLETE id
WORKFLOW.FAIL id [reason]
WORKFLOW.RESET id
WORKFLOW.NEXT id
WORKFLOW.SETVAR id key value
WORKFLOW.GETVAR id key
WORKFLOW.ADDSTEP workflow name type [config]
```

### State Machine

```
STATEM.CREATE name [description]
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
TEMPLATE.INSTANTIATE name [vars...]
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
PARALLEL.REDUCE key command [INITIAL value]
PARALLEL.FILTER key command
```

### Secrets Management

```
SECRET.SET key value [TTL ttl]
SECRET.GET key
SECRET.DELETE key
SECRET.LIST [pattern]
SECRET.ROTATE key
SECRET.VERSION key
```

### Configuration

```
CONFIG.SET key value [NAMESPACE ns]
CONFIG.GET key [NAMESPACE ns]
CONFIG.DELETE key [NAMESPACE ns]
CONFIG.LIST [pattern] [NAMESPACE ns]
CONFIG.NAMESPACE ns command
CONFIG.IMPORT data [FORMAT format]
CONFIG.EXPORT [pattern] [FORMAT format]
```

### Data Structures

```
TRIE.ADD name key [value]
TRIE.SEARCH name prefix
TRIE.PREFIX name prefix [COUNT count]
TRIE.DELETE name key
TRIE.AUTOCOMPLETE name prefix [COUNT count]

RING.CREATE name [replicas]
RING.ADD name node [weight]
RING.GET name key
RING.NODES name
RING.REMOVE name node

SEM.CREATE name count
SEM.ACQUIRE name [timeout]
SEM.RELEASE name
SEM.TRYACQUIRE name
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

XML.ENCODE data [ROOT root]
XML.DECODE xml

YAML.ENCODE data
YAML.DECODE yaml

TOML.ENCODE data
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
TIMESTAMP.PARSE format string
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
EVENT.EMIT channel event [data]
EVENT.GET channel [cursor] [COUNT count]
EVENT.LIST [pattern]
EVENT.CLEAR channel
```

### Webhooks

```
WEBHOOK.CREATE name url [EVENTS events...] [SECRET secret]
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
RL.CREATE name rate interval [BURST burst]
RL.ALLOW name [COST cost]
RL.GET name
RL.DELETE name
RL.RESET name
```

### Distributed Locks

```
LOCK.TRY key ttl
LOCK.ACQUIRE key ttl [RETRY retry] [DELAY delay]
LOCK.RELEASE key
LOCK.RENEW key ttl
LOCK.INFO key
LOCK.ISLOCKED key
```

### ID Generation

```
ID.CREATE name [START start] [STEP step]
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
AUDIT.LOG command key [DETAILS details]
AUDIT.GET [COUNT count]
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
FLAG.GETVARIANT name [USER user]
FLAG.ADDRULE name rule value
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
BACKUP.CREATE [NAME name] [TYPE type]
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
CIRCUITBREAKER.CREATE name [THRESHOLD threshold] [TIMEOUT timeout]
CIRCUITBREAKER.STATE name
CIRCUITBREAKER.TRIP name
CIRCUITBREAKER.RESET name
```

### Rate Limit

```
RATELIMIT.CREATE name rate window
RATELIMIT.CHECK name [COST cost]
RATELIMIT.RESET name
RATELIMIT.DELETE name
```

### Cache Operations

```
CACHE.LOCK key ttl
CACHE.UNLOCK key
CACHE.LOCKED key
CACHE.REFRESH key
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
ARRAY.MERGE key1 key2 [OUT out]
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
OBJECT.FROMENTRIES entries
OBJECT.MERGE key1 key2 [OUT out]
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
MATH.STDDEV key
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

*Last updated: 2026-02-25*
*CacheStorm v0.1.27*
*Total Commands: 1,606*
