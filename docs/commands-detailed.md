# CacheStorm Commands Detailed Reference

Complete reference for all 1,606 CacheStorm commands with examples and use cases.

## Table of Contents

- [String Commands](#string-commands)
- [Hash Commands](#hash-commands)
- [List Commands](#list-commands)
- [Set Commands](#set-commands)
- [Sorted Set Commands](#sorted-set-commands)
- [Bitmap Commands](#bitmap-commands)
- [HyperLogLog Commands](#hyperloglog-commands)
- [Geo Commands](#geo-commands)
- [Stream Commands](#stream-commands)
- [JSON Commands](#json-commands)
- [Time Series Commands](#time-series-commands)
- [Search Commands](#search-commands)
- [Graph Commands](#graph-commands)
- [Pub/Sub Commands](#pubsub-commands)
- [Transaction Commands](#transaction-commands)
- [Connection Commands](#connection-commands)
- [Server Commands](#server-commands)
- [Cluster Commands](#cluster-commands)
- [Tag Commands](#tag-commands)
- [Resilience Commands](#resilience-commands)
- [ML Commands](#ml-commands)

---

## String Commands

### SET
Set key to hold the string value.

```
SET key value [EX seconds|PX milliseconds|EXAT timestamp|PXAT milliseconds-timestamp|KEEPTTL] [NX|XX] [GET] [TAGS tag ...]
```

**Parameters:**
- `key` - The key to set
- `value` - The value to store
- `EX seconds` - Set expiration in seconds
- `PX milliseconds` - Set expiration in milliseconds
- `EXAT timestamp` - Set absolute Unix timestamp (seconds)
- `PXAT milliseconds-timestamp` - Set absolute Unix timestamp (milliseconds)
- `KEEPTTL` - Retain the TTL associated with the key
- `NX` - Only set if key does not exist
- `XX` - Only set if key already exists
- `GET` - Return the old value
- `TAGS tag ...` - CacheStorm: Assign tags for invalidation

**Return Value:**
- OK on success
- Nil if NX/XX condition not met
- Old value if GET specified

**Examples:**
```redis
# Basic set
SET mykey "Hello"

# With expiration (60 seconds)
SET session:user:123 "active" EX 60

# Set only if not exists
SET lock:resource "locked" NX EX 10

# Set with tags (CacheStorm extension)
SET user:1 "{\"name\":\"John\"}" TAGS user session

# Compare-and-swap pattern
SET balance:1001 "1000" GET
```

**Complexity:** O(1)

---

### GET
Get the value of key.

```
GET key
```

**Parameters:**
- `key` - The key to get

**Return Value:**
- The value of key
- Nil if key does not exist

**Examples:**
```redis
GET mykey
```

**Complexity:** O(1)

---

### GETDEL
Get the value of key and delete the key.

```
GETDEL key
```

**Return Value:**
- The value of key
- Nil if key does not exist

**Examples:**
```redis
GETDEL temp:token:123
```

**Complexity:** O(1)

---

### GETEX
Get the value of key and optionally set its expiration.

```
GETEX key [EX seconds|PX milliseconds|EXAT timestamp|PXAT milliseconds-timestamp|PERSIST]
```

**Examples:**
```redis
# Get and extend expiration
GETEX session:123 EX 300

# Get and remove expiration
GETEX persistent:key PERSIST
```

**Complexity:** O(1)

---

### MSET
Set multiple keys to multiple values.

```
MSET key value [key value ...]
```

**Examples:**
```redis
MSET key1 "Hello" key2 "World" key3 "CacheStorm"
```

**Complexity:** O(N) where N is the number of keys

---

### MGET
Get the values of all the given keys.

```
MGET key [key ...]
```

**Examples:**
```redis
MGET key1 key2 key3
```

**Return Value:** Array of values, nil for non-existing keys

**Complexity:** O(N) where N is the number of keys

---

### INCR
Increment the integer value of a key by one.

```
INCR key
```

**Examples:**
```redis
SET counter 10
INCR counter    # Returns 11
```

**Complexity:** O(1)

---

### INCRBY
Increment the integer value of a key by the given amount.

```
INCRBY key increment
```

**Examples:**
```redis
INCRBY counter 5    # Add 5
INCRBY counter -3   # Subtract 3
```

---

### DECR / DECRBY
Decrement operations.

```
DECR key
DECRBY key decrement
```

---

### APPEND
Append a value to a key.

```
APPEND key value
```

**Examples:**
```redis
SET mykey "Hello"
APPEND mykey " World"    # Returns 11 (new length)
GET mykey                 # "Hello World"
```

---

### STRLEN
Get the length of the value stored in a key.

```
STRLEN key
```

---

### GETRANGE
Get a substring of the string stored at a key.

```
GETRANGE key start end
```

**Examples:**
```redis
SET mykey "This is a string"
GETRANGE mykey 0 3      # "This"
GETRANGE mykey -3 -1    # "ing"
```

---

### SETRANGE
Overwrite part of a string at key starting at the specified offset.

```
SETRANGE key offset value
```

---

## Hash Commands

### HSET
Set field in the hash stored at key to value.

```
HSET key field value [field value ...]
```

**Examples:**
```redis
# Single field
HSET user:1000 name "John Doe"

# Multiple fields
HSET user:1000 name "John Doe" email "john@example.com" age 30

# With tags (CacheStorm extension)
HSET user:1000 name "John" TAGS user
```

**Return Value:** Number of fields that were added

**Complexity:** O(1) for each field

---

### HGET
Get the value of a hash field.

```
HGET key field
```

---

### HMGET
Get the values of all the given hash fields.

```
HMGET key field [field ...]
```

---

### HGETALL
Get all the fields and values in a hash.

```
HGETALL key
```

**Return Value:** Array of fields and values alternating

**Examples:**
```redis
HGETALL user:1000
# 1) "name"
# 2) "John Doe"
# 3) "email"
# 4) "john@example.com"
```

---

### HDEL
Delete one or more hash fields.

```
HDEL key field [field ...]
```

---

### HEXISTS
Determine if a hash field exists.

```
HEXISTS key field
```

---

### HLEN
Get the number of fields in a hash.

```
HLEN key
```

---

### HKEYS / HVALS
Get all field names / values in a hash.

```
HKEYS key
HVALS key
```

---

### HINCRBY / HINCRBYFLOAT
Increment the integer/float value of a hash field.

```
HINCRBY key field increment
HINCRBYFLOAT key field increment
```

**Examples:**
```redis
HSET product:100 price 10.50
HINCRBYFLOAT product:100 price 5.25    # Returns "15.75"
```

---

### HSETNX
Set the value of a hash field, only if the field does not exist.

```
HSETNX key field value
```

---

### HSTRLEN
Get the length of the value of a hash field.

```
HSTRLEN key field
```

---

### HRANDFIELD
Get one or multiple random fields from a hash.

```
HRANDFIELD key [count [WITHVALUES]]
```

---

## List Commands

### LPUSH / RPUSH
Prepend/Append one or multiple elements to a list.

```
LPUSH key element [element ...]
RPUSH key element [element ...]
```

**Examples:**
```redis
LPUSH mylist "world"
LPUSH mylist "hello"        # List is now ["hello", "world"]
RPUSH mylist "!"            # List is now ["hello", "world", "!"]
```

**Return Value:** Length of the list after the push

---

### LPOP / RPOP
Remove and return the first/last element of a list.

```
LPOP key [count]
RPOP key [count]
```

**Examples:**
```redis
RPUSH mylist "one" "two" "three"
LPOP mylist                 # Returns "one"
LPOP mylist 2               # Returns ["two", "three"]
```

---

### LLEN
Get the length of a list.

```
LLEN key
```

---

### LRANGE
Get a range of elements from a list.

```
LRANGE key start stop
```

**Examples:**
```redis
RPUSH mylist "one" "two" "three" "four" "five"
LRANGE mylist 0 2           # ["one", "two", "three"]
LRANGE mylist -2 -1         # ["four", "five"]
LRANGE mylist 0 -1          # All elements
```

---

### LINDEX
Get an element from a list by its index.

```
LINDEX key index
```

---

### LSET
Set the value of an element in a list by its index.

```
LSET key index element
```

---

### LREM
Remove elements from a list.

```
LREM key count element
```

**Examples:**
```redis
RPUSH mylist "hello" "hello" "foo" "hello"
LREM mylist -2 "hello"      # Remove 2 "hello" from the end
```

---

### LTRIM
Trim a list to the specified range.

```
LTRIM key start stop
```

---

### LINSERT
Insert an element before or after another element in a list.

```
LINSERT key BEFORE|AFTER pivot element
```

---

### BLPOP / BRPOP
Remove and get the first/last element in a list, or block until one is available.

```
BLPOP key [key ...] timeout
BRPOP key [key ...] timeout
```

**Examples:**
```redis
# Block for up to 5 seconds
BLPOP mylist 5
```

---

### RPOPLPUSH / BRPOPLPUSH
Remove the last element in a list, prepend it to another list.

```
RPOPLPUSH source destination
BRPOPLPUSH source destination timeout
```

---

### LMOVE / BLMOVE
Pop an element from a list, push it to another list.

```
LMOVE source destination LEFT|RIGHT LEFT|RIGHT
BLMOVE source destination LEFT|RIGHT LEFT|RIGHT timeout
```

---

### LPOS
Return the index of matching elements in a list.

```
LPOS key element [RANK rank] [COUNT num-matches] [MAXLEN len]
```

---

## Set Commands

### SADD
Add members to a set.

```
SADD key member [member ...]
```

**Return Value:** Number of elements added

---

### SREM
Remove members from a set.

```
SREM key member [member ...]
```

---

### SMEMBERS
Get all the members in a set.

```
SMEMBERS key
```

---

### SISMEMBER
Determine if a given value is a member of a set.

```
SISMEMBER key member
```

---

### SCARD
Get the number of members in a set.

```
SCARD key
```

---

### SPOP
Remove and return one or multiple random members from a set.

```
SPOP key [count]
```

---

### SRANDMEMBER
Get one or multiple random members from a set.

```
SRANDMEMBER key [count]
```

---

### SINTER / SINTERSTORE
Intersect multiple sets.

```
SINTER key [key ...]
SINTERSTORE destination key [key ...]
```

---

### SUNION / SUNIONSTORE
Add multiple sets.

```
SUNION key [key ...]
SUNIONSTORE destination key [key ...]
```

---

### SDIFF / SDIFFSTORE
Subtract multiple sets.

```
SDIFF key [key ...]
SDIFFSTORE destination key [key ...]
```

---

### SMOVE
Move a member from one set to another.

```
SMOVE source destination member
```

---

### SSCAN
Incrementally iterate Set elements.

```
SSCAN key cursor [MATCH pattern] [COUNT count]
```

---

## Sorted Set Commands

### ZADD
Add members to a sorted set.

```
ZADD key [NX|XX] [GT|LT] [CH] [INCR] score member [score member ...]
```

**Options:**
- `NX` - Only add new elements
- `XX` - Only update existing elements
- `GT` - Only update if new score > current
- `LT` - Only update if new score < current
- `CH` - Return number of changed elements
- `INCR` - Increment score

**Examples:**
```redis
ZADD myzset 1 "one"
ZADD myzset 2 "two" 3 "three"

# Update only if greater
ZADD myzset GT 5 "one"
```

---

### ZRANGE
Return a range of members in a sorted set.

```
ZRANGE key start stop [WITHSCORES] [BYSCORE|BYLEX] [REV] [LIMIT offset count]
```

**Examples:**
```redis
ZADD myzset 1 "one" 2 "two" 3 "three" 4 "four"
ZRANGE myzset 0 -1                    # ["one", "two", "three", "four"]
ZRANGE myzset 0 -1 WITHSCORES         # With scores
ZRANGE myzset 0 -1 REV                # Reverse order
ZRANGE myzset (1 3 BYSCORE            # By score range
```

---

### ZREVRANGE
Return a range of members in a sorted set, ordered from highest to lowest score.

```
ZREVRANGE key start stop [WITHSCORES]
```

---

### ZRANGEBYSCORE / ZREVRANGEBYSCORE
Return a range of members by score.

```
ZRANGEBYSCORE key min max [WITHSCORES] [LIMIT offset count]
ZREVRANGEBYSCORE key max min [WITHSCORES] [LIMIT offset count]
```

---

### ZRANGEBYLEX / ZREVRANGEBYLEX
Return a range of members by lexicographical range.

```
ZRANGEBYLEX key min max [LIMIT offset count]
ZREVRANGEBYLEX key max min [LIMIT offset count]
```

---

### ZCOUNT
Count members in a sorted set with scores within the given values.

```
ZCOUNT key min max
```

---

### ZLEXCOUNT
Count members in a sorted set within a lexicographical range.

```
ZLEXCOUNT key min max
```

---

### ZREM
Remove members from a sorted set.

```
ZREM key member [member ...]
```

---

### ZREMRANGEBYRANK
Remove members from a sorted set within a rank range.

```
ZREMRANGEBYRANK key start stop
```

---

### ZREMRANGEBYSCORE
Remove members from a sorted set within a score range.

```
ZREMRANGEBYSCORE key min max
```

---

### ZREMRANGEBYLEX
Remove members from a sorted set within a lexicographical range.

```
ZREMRANGEBYLEX key min max
```

---

### ZPOPMIN / ZPOPMAX
Remove and return members with the lowest/highest scores.

```
ZPOPMIN key [count]
ZPOPMAX key [count]
```

---

### BZPOPMIN / BZPOPMAX
Remove and return members with the lowest/highest scores, or block until available.

```
BZPOPMIN key [key ...] timeout
BZPOPMAX key [key ...] timeout
```

---

### ZMPOP
Remove and return members from a sorted set.

```
ZMPOP numkeys key [key ...] MIN|MAX [COUNT count]
```

---

### ZMSCORE
Get the score associated with the given members.

```
ZMSCORE key member [member ...]
```

---

### ZRANK / ZREVRANK
Determine the index of a member in a sorted set.

```
ZRANK key member [WITHSCORE]
ZREVRANK key member [WITHSCORE]
```

---

### ZDIFF / ZDIFFSTORE
Subtract multiple sorted sets.

```
ZDIFF numkeys key [key ...] [WITHSCORES]
ZDIFFSTORE destination numkeys key [key ...]
```

---

### ZINTER / ZINTERSTORE
Intersect multiple sorted sets.

```
ZINTER numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX] [WITHSCORES]
ZINTERSTORE destination numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX]
```

---

### ZUNION / ZUNIONSTORE
Add multiple sorted sets.

```
ZUNION numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX] [WITHSCORES]
ZUNIONSTORE destination numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX]
```

---

### ZRANDMEMBER
Get one or multiple random elements from a sorted set.

```
ZRANDMEMBER key [count [WITHSCORES]]
```

---

### ZSCAN
Incrementally iterate sorted set elements.

```
ZSCAN key cursor [MATCH pattern] [COUNT count]
```

---

## Bitmap Commands

### SETBIT
Set or clear the bit at offset in the string value stored at key.

```
SETBIT key offset value
```

**Examples:**
```redis
SETBIT mykey 7 1          # Set bit 7 to 1
SETBIT mykey 7 0          # Clear bit 7
```

---

### GETBIT
Returns the bit value at offset in the string value stored at key.

```
GETBIT key offset
```

---

### BITCOUNT
Count set bits in a string.

```
BITCOUNT key [start end [BYTE|BIT]]
```

---

### BITPOS
Find first set or clear bit.

```
BITPOS key bit [start [end [BYTE|BIT]]]
```

---

### BITOP
Perform bitwise operations between strings.

```
BITOP operation destkey key [key ...]
```

**Operations:** AND, OR, XOR, NOT

---

### BITFIELD
Perform arbitrary bitfield integer operations.

```
BITFIELD key [GET type offset] [SET type offset value] [INCRBY type offset increment] [OVERFLOW WRAP|SAT|FAIL]
```

---

## Tag Commands (CacheStorm Extension)

### TAGS
Get tags associated with a key.

```
TAGS key
```

**Return Value:** Array of tags

**Examples:**
```redis
SET user:1 "data" TAGS user session
TAGS user:1
# 1) "user"
# 2) "session"
```

---

### TAGKEYS
Get all keys with a specific tag.

```
TAGKEYS tag
```

**Return Value:** Array of keys

**Examples:**
```redis
SET user:1 "data" TAGS user
SET user:2 "data" TAGS user
SET product:1 "data" TAGS product

TAGKEYS user
# 1) "user:1"
# 2) "user:2"
```

---

### INVALIDATE
Invalidate all keys with a specific tag.

```
INVALIDATE tag [tag ...]
```

**Return Value:** Number of keys invalidated

**Examples:**
```redis
SET user:1 "data" TAGS user
SET user:2 "data" TAGS user
SET session:1 "data" TAGS session

INVALIDATE user
# (integer) 2
```

---

### TAGDEL
Delete a tag (remove from all keys).

```
TAGDEL tag [tag ...]
```

---

### TAGEXISTS
Check if a tag exists.

```
TAGEXISTS tag
```

---

### TAGTTL
Get TTL information for a tag.

```
TAGTTL tag
```

---

## Pub/Sub Commands

### SUBSCRIBE
Subscribe to channels.

```
SUBSCRIBE channel [channel ...]
```

---

### UNSUBSCRIBE
Unsubscribe from channels.

```
UNSUBSCRIBE [channel [channel ...]]
```

---

### PSUBSCRIBE
Subscribe to channels matching patterns.

```
PSUBSCRIBE pattern [pattern ...]
```

---

### PUNSUBSCRIBE
Unsubscribe from patterns.

```
PUNSUBSCRIBE [pattern [pattern ...]]
```

---

### PUBLISH
Post a message to a channel.

```
PUBLISH channel message
```

---

### SPUBLISH
Post a message to a sharded channel.

```
SPUBLISH channel message
```

---

### SSUBSCRIBE / SUNSUBSCRIBE
Subscribe/unsubscribe to sharded channels.

```
SSUBSCRIBE shardchannel [shardchannel ...]
SUNSUBSCRIBE [shardchannel [shardchannel ...]]
```

---

### PUBSUB
Inspect the state of the Pub/Sub subsystem.

```
PUBSUB subcommand [argument [argument ...]]
```

**Subcommands:**
- `CHANNELS [pattern]` - List active channels
- `NUMSUB [channel ...]` - Count subscribers
- `NUMPAT` - Count pattern subscriptions
- `SHARDCHANNELS [pattern]` - List sharded channels
- `SHARDNUMSUB [shardchannel ...]` - Count sharded subscribers

---

## Transaction Commands

### MULTI
Mark the start of a transaction block.

```
MULTI
```

---

### EXEC
Execute all commands issued after MULTI.

```
EXEC
```

---

### DISCARD
Discard all commands issued after MULTI.

```
DISCARD
```

---

### WATCH
Watch the given keys to determine execution of the MULTI/EXEC block.

```
WATCH key [key ...]
```

---

### UNWATCH
Forget about all watched keys.

```
UNWATCH
```

---

## Server Commands

### INFO
Get information and statistics about the server.

```
INFO [section]
```

**Sections:** server, clients, memory, persistence, stats, replication, cpu, commandstats, cluster, keyspace, all, default

---

### CONFIG GET / SET
Get/Set configuration parameters.

```
CONFIG GET parameter [parameter ...]
CONFIG SET parameter value [parameter value ...]
CONFIG REWRITE
CONFIG RESETSTAT
```

---

### FLUSHDB / FLUSHALL
Remove all keys from the current/all database(s).

```
FLUSHDB [ASYNC|SYNC]
FLUSHALL [ASYNC|SYNC]
```

---

### DBSIZE
Return the number of keys in the selected database.

```
DBSIZE
```

---

### SAVE / BGSAVE
Synchronously/Asynchronously save the dataset to disk.

```
SAVE
BGSAVE [SCHEDULE]
```

---

### LASTSAVE
Get the UNIX time stamp of the last successful save to disk.

```
LASTSAVE
```

---

### SHUTDOWN
Synchronously save the dataset to disk and then shut down the server.

```
SHUTDOWN [NOSAVE|SAVE] [NOW] [FORCE] [ABORT]
```

---

### MONITOR
Listen for all requests received by the server in real time.

```
MONITOR
```

---

### SLOWLOG
Manage the slow log.

```
SLOWLOG subcommand [argument]
```

---

### TIME
Return the current server time.

```
TIME
```

---

### COMMAND
Get array of Redis command details.

```
COMMAND
COMMAND COUNT
COMMAND INFO command-name [command-name ...]
COMMAND LIST
COMMAND DOCS [command-name [command-name ...]]
COMMAND GETKEYS
COMMAND GETKEYSANDFLAGS
```

---

### LATENCY
Latency monitoring.

```
LATENCY DOCTOR
LATENCY GRAPH event
LATENCY HISTORY event
LATENCY LATEST
LATENCY RESET [event [event ...]]
LATENCY HELP
```

---

## Connection Commands

### AUTH
Authenticate to the server.

```
AUTH [username] password
```

---

### PING
Ping the server.

```
PING [message]
```

---

### SELECT
Change the selected database.

```
SELECT index
```

---

### SWAPDB
Swap two Redis databases.

```
SWAPDB index1 index2
```

---

### ECHO
Echo the given string.

```
ECHO message
```

---

### QUIT
Close the connection.

```
QUIT
```

---

## Cluster Commands

### CLUSTER INFO
Provides info about Redis Cluster node state.

```
CLUSTER INFO
```

---

### CLUSTER NODES
Get Cluster config for the node.

```
CLUSTER NODES
```

---

### CLUSTER SLOTS
Get array of Cluster slot to node mappings.

```
CLUSTER SLOTS
```

---

### CLUSTER KEYSLOT
Returns the hash slot of the specified key.

```
CLUSTER KEYSLOT key
```

---

### CLUSTER ADDSLOTS / DELSLOTS
Assign/remove slots to current node.

```
CLUSTER ADDSLOTS slot [slot ...]
CLUSTER DELSLOTS slot [slot ...]
```

---

### CLUSTER REPLICATE
Reconfigure a node as a replica of the specified master node.

```
CLUSTER REPLICATE node-id
```

---

### CLUSTER FAILOVER
Force a replica to perform a manual failover.

```
CLUSTER FAILOVER [FORCE|TAKEOVER]
```

---

### CLUSTER MEET
Force a node cluster to handshake with another cluster.

```
CLUSTER MEET ip port
```

---

### CLUSTER RESET
Reset a Redis Cluster node.

```
CLUSTER RESET [HARD|SOFT]
```

---

## Key Commands

### KEYS
Find all keys matching the given pattern.

```
KEYS pattern
```

**Warning:** Use in production with care, use SCAN instead for large datasets.

---

### SCAN
Incrementally iterate the keys space.

```
SCAN cursor [MATCH pattern] [COUNT count] [TYPE type]
```

**Examples:**
```redis
SCAN 0
SCAN 0 MATCH user:*
SCAN 0 COUNT 100
SCAN 0 TYPE hash
```

---

### DEL
Delete a key.

```
DEL key [key ...]
```

---

### UNLINK
Asynchronously delete a key.

```
UNLINK key [key ...]
```

---

### EXISTS
Determine if a key exists.

```
EXISTS key [key ...]
```

---

### EXPIRE / EXPIREAT
Set a key's time to live in seconds / at a timestamp.

```
EXPIRE key seconds [NX|XX|GT|LT]
EXPIREAT key timestamp [NX|XX|GT|LT]
```

---

### PEXPIRE / PEXPIREAT
Set a key's time to live in milliseconds.

```
PEXPIRE key milliseconds [NX|XX|GT|LT]
PEXPIREAT key milliseconds-timestamp [NX|XX|GT|LT]
```

---

### TTL / PTTL
Get the time to live for a key.

```
TTL key
PTTL key
```

---

### EXPIRETIME / PEXPIRETIME
Get the expiration Unix timestamp.

```
EXPIRETIME key
PEXPIRETIME key
```

---

### PERSIST
Remove the expiration from a key.

```
PERSIST key
```

---

### RENAME / RENAMENX
Rename a key.

```
RENAME key newkey
RENAMENX key newkey
```

---

### TYPE
Determine the type stored at key.

```
TYPE key
```

---

### DUMP / RESTORE
Serialize/Deserialize the value stored at key.

```
DUMP key
RESTORE key ttl serialized-value [REPLACE] [ABSTTL] [IDLETIME seconds] [FREQ frequency]
```

---

### MOVE
Move a key to another database.

```
MOVE key db
```

---

### COPY
Copy a key.

```
COPY source destination [DB destination-db] [REPLACE]
```

---

### SORT
Sort the elements in a list, set or sorted set.

```
SORT key [BY pattern] [LIMIT offset count] [GET pattern [GET pattern ...]] [ASC|DESC] [ALPHA] [STORE destination]
```

---

### TOUCH
Alters the last access time of a key(s).

```
TOUCH key [key ...]
```

---

### OBJECT
Inspect the internals of Redis objects.

```
OBJECT subcommand [arguments [arguments ...]]
```

---

### RANDOMKEY
Return a random key from the keyspace.

```
RANDOMKEY
```

---

### WAIT
Wait for the synchronous replication of all the write commands sent in the context of the current connection.

```
WAIT numreplicas timeout
```

---

### WAITAOF
Wait for local Redis buffers to be written to the AOF of local Redis and/or AOF of replicas.

```
WAITAOF numlocal numreplicas timeout
```

---

## Resilience Commands

### CIRCUITX.CREATE
Create a circuit breaker.

```
CIRCUITX.CREATE name [FAILURE_THRESHOLD threshold] [SUCCESS_THRESHOLD threshold] [TIMEOUT timeout] [HALF_OPEN_MAX max]
```

---

### RATELIMITER.CREATE
Create a rate limiter.

```
RATELIMITER.CREATE name rate burst [DURATION duration]
```

---

### RETRY.CREATE
Create a retry policy.

```
RETRY.CREATE name maxAttempts [BACKOFF_TYPE type] [INITIAL_DELAY delay] [MAX_DELAY delay]
```

---

## ML Commands

### MODEL.CREATE
Create an ML model.

```
MODEL.CREATE name type [PARAMS param value ...]
```

---

### MODEL.TRAIN
Train a model.

```
MODEL.TRAIN name dataKey [PARAMS param value ...]
```

---

### MODEL.PREDICT
Make a prediction.

```
MODEL.PREDICT name inputKey
```

---

### EMBEDDING.CREATE
Create an embedding index.

```
EMBEDDING.CREATE name dimensions [METRIC metric]
```

---

### EMBEDDING.SEARCH
Search embeddings.

```
EMBEDDING.SEARCH name vector [K k] [THRESHOLD threshold]
```

---

## Command Tips

### Common Patterns

#### Session Management
```redis
SET session:user:123 "{\"id\":123,\"name\":\"John\"}" EX 3600 TAGS session user:123
EXPIRE session:user:123 3600
TTL session:user:123
```

#### Rate Limiting
```redis
INCR rate_limit:ip:192.168.1.1
EXPIRE rate_limit:ip:192.168.1.1 60
```

#### Distributed Lock
```redis
SET lock:resource my_random_value NX EX 10
DEL lock:resource
```

#### Leaderboard
```redis
ZADD leaderboard 1000 "player1" 1500 "player2" 800 "player3"
ZREVRANGE leaderboard 0 9 WITHSCORES
ZINCRBY leaderboard 100 "player1"
```

#### Real-time Analytics
```redis
PFADD visitors:2024-01-01 "user1" "user2" "user3"
PFCOUNT visitors:2024-01-01
```

#### Tag-based Cache Invalidation
```redis
# Set multiple user data with tags
SET user:1 "data" TAGS user session
SET user:2 "data" TAGS user session
SET product:1 "data" TAGS product

# Invalidate all user data
INVALIDATE user

# Invalidate all session data
INVALIDATE session
```

---

*Last updated: 2026-02-25*
*CacheStorm v0.1.27*
*Total Commands: 1,606*
