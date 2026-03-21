import DocsLayout, {
  DocHeading,
  CodeBlock,
  InfoBox,
  type TocItem,
} from "@/components/DocsLayout";
import { Terminal, Type, Hash, List, Layers, SortAsc, Radio, MessageSquare, Key, Clock } from "lucide-react";

const toc: TocItem[] = [
  { id: "overview", text: "Overview", level: 2 },
  { id: "strings", text: "Strings", level: 2 },
  { id: "hashes", text: "Hashes", level: 2 },
  { id: "lists", text: "Lists", level: 2 },
  { id: "sets", text: "Sets", level: 2 },
  { id: "sorted-sets", text: "Sorted Sets", level: 2 },
  { id: "streams", text: "Streams", level: 2 },
  { id: "pubsub", text: "Pub/Sub", level: 2 },
  { id: "keys", text: "Keys & Expiration", level: 2 },
  { id: "server-cmds", text: "Server Commands", level: 2 },
  { id: "scripting", text: "Scripting", level: 2 },
];

function CommandBadge({ complexity }: { complexity: string }) {
  const color = complexity.startsWith("O(1)")
    ? "text-emerald-400 bg-emerald-500/10 border-emerald-500/30"
    : complexity.startsWith("O(N)")
    ? "text-amber-400 bg-amber-500/10 border-amber-500/30"
    : "text-slate-400 bg-slate-500/10 border-slate-500/30";

  return (
    <span className={`text-[10px] font-mono px-1.5 py-0.5 rounded border ${color}`}>
      {complexity}
    </span>
  );
}

function CommandEntry({
  name,
  syntax,
  desc,
  complexity,
}: {
  name: string;
  syntax: string;
  desc: string;
  complexity: string;
}) {
  return (
    <div className="py-3 border-b border-slate-800/60 last:border-0">
      <div className="flex items-center gap-2 flex-wrap mb-1">
        <code className="text-sm font-bold text-blue-300">{name}</code>
        <CommandBadge complexity={complexity} />
      </div>
      <p className="text-xs font-mono text-slate-500 mb-1">{syntax}</p>
      <p className="text-sm text-slate-400">{desc}</p>
    </div>
  );
}

export default function Commands() {
  return (
    <DocsLayout toc={toc}>
      {/* Hero */}
      <div className="mb-10">
        <div className="flex items-center gap-2 text-blue-400 text-sm font-medium mb-2">
          <Terminal className="w-4 h-4" />
          Reference
        </div>
        <h1 className="text-4xl font-extrabold text-white tracking-tight mb-4">
          Command Reference
        </h1>
        <p className="text-lg text-slate-400 leading-relaxed max-w-2xl">
          CacheStorm supports 200+ Redis-compatible commands across all major data structures.
          This page covers the most commonly used commands with examples.
        </p>
      </div>

      <DocHeading id="overview" level={2}>
        Overview
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Commands follow the standard Redis protocol (RESP). Use any Redis client library or{" "}
        <code className="text-xs bg-slate-800 px-1 py-0.5 rounded">redis-cli</code> to interact
        with CacheStorm.
      </p>

      <InfoBox type="info">
        All commands are case-insensitive.{" "}
        <code className="text-xs bg-slate-800 px-1 py-0.5 rounded">SET</code>,{" "}
        <code className="text-xs bg-slate-800 px-1 py-0.5 rounded">set</code>, and{" "}
        <code className="text-xs bg-slate-800 px-1 py-0.5 rounded">Set</code> are all equivalent.
      </InfoBox>

      {/* ── Strings ──────────────────────────────────────────── */}
      <DocHeading id="strings" level={2}>
        <Type className="w-5 h-5 text-blue-400" />
        Strings
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Strings are the most basic data type. They can hold any data: text, serialized objects,
        or binary data up to 512 MB.
      </p>

      <div className="rounded-xl border border-slate-800 overflow-hidden px-4 mb-4">
        <CommandEntry name="SET" syntax="SET key value [EX seconds] [PX milliseconds] [NX|XX]" desc="Set a key to a string value with optional expiration and condition." complexity="O(1)" />
        <CommandEntry name="GET" syntax="GET key" desc="Get the string value of a key. Returns nil if the key does not exist." complexity="O(1)" />
        <CommandEntry name="MSET" syntax="MSET key value [key value ...]" desc="Set multiple keys to multiple values atomically." complexity="O(N)" />
        <CommandEntry name="MGET" syntax="MGET key [key ...]" desc="Get the values of multiple keys." complexity="O(N)" />
        <CommandEntry name="INCR" syntax="INCR key" desc="Increment the integer value of a key by one." complexity="O(1)" />
        <CommandEntry name="DECR" syntax="DECR key" desc="Decrement the integer value of a key by one." complexity="O(1)" />
        <CommandEntry name="INCRBY" syntax="INCRBY key increment" desc="Increment the integer value of a key by the given amount." complexity="O(1)" />
        <CommandEntry name="APPEND" syntax="APPEND key value" desc="Append a value to a key. Creates the key if it does not exist." complexity="O(1)" />
        <CommandEntry name="STRLEN" syntax="STRLEN key" desc="Get the length of the value stored at a key." complexity="O(1)" />
        <CommandEntry name="SETNX" syntax="SETNX key value" desc="Set a key only if it does not already exist." complexity="O(1)" />
        <CommandEntry name="GETSET" syntax="GETSET key value" desc="Set a key and return its previous value." complexity="O(1)" />
      </div>

      <CodeBlock
        language="bash"
        title="Strings example"
        code={`# Basic set/get
SET user:1:name "Alice"
GET user:1:name
# => "Alice"

# Atomic counter
SET page:views 0
INCR page:views
INCR page:views
GET page:views
# => "2"

# Set with TTL (expires in 1 hour)
SET session:token "abc123" EX 3600

# Set only if key doesn't exist (distributed lock)
SET lock:resource "owner1" NX EX 30

# Multiple operations
MSET key1 "val1" key2 "val2" key3 "val3"
MGET key1 key2 key3
# => ["val1", "val2", "val3"]`}
      />

      {/* ── Hashes ───────────────────────────────────────────── */}
      <DocHeading id="hashes" level={2}>
        <Hash className="w-5 h-5 text-blue-400" />
        Hashes
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Hashes are maps of field-value pairs, ideal for representing objects.
      </p>

      <div className="rounded-xl border border-slate-800 overflow-hidden px-4 mb-4">
        <CommandEntry name="HSET" syntax="HSET key field value [field value ...]" desc="Set one or more fields in a hash." complexity="O(N)" />
        <CommandEntry name="HGET" syntax="HGET key field" desc="Get the value of a hash field." complexity="O(1)" />
        <CommandEntry name="HGETALL" syntax="HGETALL key" desc="Get all fields and values of a hash." complexity="O(N)" />
        <CommandEntry name="HDEL" syntax="HDEL key field [field ...]" desc="Delete one or more hash fields." complexity="O(N)" />
        <CommandEntry name="HEXISTS" syntax="HEXISTS key field" desc="Check if a hash field exists." complexity="O(1)" />
        <CommandEntry name="HLEN" syntax="HLEN key" desc="Get the number of fields in a hash." complexity="O(1)" />
        <CommandEntry name="HINCRBY" syntax="HINCRBY key field increment" desc="Increment the integer value of a hash field." complexity="O(1)" />
        <CommandEntry name="HKEYS" syntax="HKEYS key" desc="Get all field names in a hash." complexity="O(N)" />
        <CommandEntry name="HVALS" syntax="HVALS key" desc="Get all values in a hash." complexity="O(N)" />
      </div>

      <CodeBlock
        language="bash"
        title="Hashes example"
        code={`# Store user object
HSET user:1 name "Alice" email "alice@example.com" age "30" role "admin"

# Get specific field
HGET user:1 name
# => "Alice"

# Get all fields
HGETALL user:1
# => {name: "Alice", email: "alice@example.com", age: "30", role: "admin"}

# Increment a numeric field
HINCRBY user:1 age 1
HGET user:1 age
# => "31"

# Check if field exists
HEXISTS user:1 phone
# => 0 (false)`}
      />

      {/* ── Lists ────────────────────────────────────────────── */}
      <DocHeading id="lists" level={2}>
        <List className="w-5 h-5 text-blue-400" />
        Lists
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Lists are linked lists of string values, useful for queues, stacks, and recent item tracking.
      </p>

      <div className="rounded-xl border border-slate-800 overflow-hidden px-4 mb-4">
        <CommandEntry name="LPUSH" syntax="LPUSH key value [value ...]" desc="Prepend one or more values to a list." complexity="O(N)" />
        <CommandEntry name="RPUSH" syntax="RPUSH key value [value ...]" desc="Append one or more values to a list." complexity="O(N)" />
        <CommandEntry name="LPOP" syntax="LPOP key [count]" desc="Remove and return elements from the head of a list." complexity="O(N)" />
        <CommandEntry name="RPOP" syntax="RPOP key [count]" desc="Remove and return elements from the tail of a list." complexity="O(N)" />
        <CommandEntry name="LRANGE" syntax="LRANGE key start stop" desc="Get a range of elements from a list." complexity="O(N)" />
        <CommandEntry name="LLEN" syntax="LLEN key" desc="Get the length of a list." complexity="O(1)" />
        <CommandEntry name="LINDEX" syntax="LINDEX key index" desc="Get an element by its index." complexity="O(N)" />
        <CommandEntry name="LSET" syntax="LSET key index value" desc="Set the value of an element by its index." complexity="O(N)" />
        <CommandEntry name="BLPOP" syntax="BLPOP key [key ...] timeout" desc="Blocking pop from the head of a list." complexity="O(N)" />
        <CommandEntry name="BRPOP" syntax="BRPOP key [key ...] timeout" desc="Blocking pop from the tail of a list." complexity="O(N)" />
      </div>

      <CodeBlock
        language="bash"
        title="Lists example"
        code={`# Task queue (FIFO)
RPUSH queue:jobs "job:1" "job:2" "job:3"
LPOP queue:jobs
# => "job:1"

# Recent activity feed (keep last 100)
LPUSH feed:user:1 "posted a photo"
LPUSH feed:user:1 "liked a comment"
LTRIM feed:user:1 0 99

# Get latest 10 items
LRANGE feed:user:1 0 9

# Blocking queue (wait up to 30s for new item)
BLPOP queue:tasks 30`}
      />

      {/* ── Sets ─────────────────────────────────────────────── */}
      <DocHeading id="sets" level={2}>
        <Layers className="w-5 h-5 text-blue-400" />
        Sets
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Sets are unordered collections of unique strings, perfect for tags, unique visitors, and set operations.
      </p>

      <div className="rounded-xl border border-slate-800 overflow-hidden px-4 mb-4">
        <CommandEntry name="SADD" syntax="SADD key member [member ...]" desc="Add one or more members to a set." complexity="O(N)" />
        <CommandEntry name="SREM" syntax="SREM key member [member ...]" desc="Remove one or more members from a set." complexity="O(N)" />
        <CommandEntry name="SMEMBERS" syntax="SMEMBERS key" desc="Get all members of a set." complexity="O(N)" />
        <CommandEntry name="SISMEMBER" syntax="SISMEMBER key member" desc="Check if a value is a member of a set." complexity="O(1)" />
        <CommandEntry name="SCARD" syntax="SCARD key" desc="Get the number of members in a set." complexity="O(1)" />
        <CommandEntry name="SINTER" syntax="SINTER key [key ...]" desc="Return the intersection of multiple sets." complexity="O(N*M)" />
        <CommandEntry name="SUNION" syntax="SUNION key [key ...]" desc="Return the union of multiple sets." complexity="O(N)" />
        <CommandEntry name="SDIFF" syntax="SDIFF key [key ...]" desc="Return the difference between the first set and all successive sets." complexity="O(N)" />
        <CommandEntry name="SRANDMEMBER" syntax="SRANDMEMBER key [count]" desc="Get one or more random members from a set." complexity="O(N)" />
      </div>

      <CodeBlock
        language="bash"
        title="Sets example"
        code={`# Tag system
SADD tags:article:1 "go" "caching" "performance" "databases"
SADD tags:article:2 "go" "networking" "performance"

# Common tags between articles
SINTER tags:article:1 tags:article:2
# => ["go", "performance"]

# All tags across articles
SUNION tags:article:1 tags:article:2
# => ["go", "caching", "performance", "databases", "networking"]

# Unique visitor tracking
SADD visitors:2024-01-15 "user:1" "user:2" "user:3" "user:1"
SCARD visitors:2024-01-15
# => 3 (duplicates ignored)`}
      />

      {/* ── Sorted Sets ──────────────────────────────────────── */}
      <DocHeading id="sorted-sets" level={2}>
        <SortAsc className="w-5 h-5 text-blue-400" />
        Sorted Sets
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Sorted sets are sets where each member has an associated score, maintaining order by score.
        Ideal for leaderboards, rate limiting, and priority queues.
      </p>

      <div className="rounded-xl border border-slate-800 overflow-hidden px-4 mb-4">
        <CommandEntry name="ZADD" syntax="ZADD key [NX|XX] [GT|LT] [CH] score member [score member ...]" desc="Add members with scores to a sorted set." complexity="O(log N)" />
        <CommandEntry name="ZREM" syntax="ZREM key member [member ...]" desc="Remove one or more members from a sorted set." complexity="O(log N)" />
        <CommandEntry name="ZSCORE" syntax="ZSCORE key member" desc="Get the score of a member." complexity="O(1)" />
        <CommandEntry name="ZRANK" syntax="ZRANK key member" desc="Get the rank (index) of a member, ordered low to high." complexity="O(log N)" />
        <CommandEntry name="ZRANGE" syntax="ZRANGE key min max [BYSCORE|BYLEX] [REV] [LIMIT offset count] [WITHSCORES]" desc="Return a range of members from a sorted set." complexity="O(log N + M)" />
        <CommandEntry name="ZRANGEBYSCORE" syntax="ZRANGEBYSCORE key min max [WITHSCORES] [LIMIT offset count]" desc="Return members with scores within the given range." complexity="O(log N + M)" />
        <CommandEntry name="ZCARD" syntax="ZCARD key" desc="Get the number of members in a sorted set." complexity="O(1)" />
        <CommandEntry name="ZINCRBY" syntax="ZINCRBY key increment member" desc="Increment the score of a member." complexity="O(log N)" />
        <CommandEntry name="ZCOUNT" syntax="ZCOUNT key min max" desc="Count members with scores within the given range." complexity="O(log N)" />
      </div>

      <CodeBlock
        language="bash"
        title="Sorted sets example"
        code={`# Leaderboard
ZADD leaderboard 1500 "alice" 1200 "bob" 1800 "charlie" 900 "dave"

# Top 3 players (highest score first)
ZRANGE leaderboard 0 2 REV WITHSCORES
# => charlie:1800, alice:1500, bob:1200

# Player rank (0-indexed)
ZRANK leaderboard "alice"
# => 2 (third from bottom)

# Update score
ZINCRBY leaderboard 500 "bob"
ZSCORE leaderboard "bob"
# => 1700

# Rate limiting (sliding window)
ZADD rate:user:1 1705000000 "req:1"
ZADD rate:user:1 1705000001 "req:2"
ZCOUNT rate:user:1 1704999900 1705000100
# => 2 requests in window`}
      />

      {/* ── Streams ──────────────────────────────────────────── */}
      <DocHeading id="streams" level={2}>
        <Radio className="w-5 h-5 text-blue-400" />
        Streams
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Streams are append-only log data structures for event sourcing, message queues, and real-time data.
      </p>

      <div className="rounded-xl border border-slate-800 overflow-hidden px-4 mb-4">
        <CommandEntry name="XADD" syntax="XADD key [MAXLEN|MINID [=|~] threshold] *|id field value [field value ...]" desc="Append a new entry to a stream." complexity="O(1)" />
        <CommandEntry name="XREAD" syntax="XREAD [COUNT count] [BLOCK milliseconds] STREAMS key [key ...] id [id ...]" desc="Read entries from one or more streams." complexity="O(N)" />
        <CommandEntry name="XRANGE" syntax="XRANGE key start end [COUNT count]" desc="Return a range of entries from a stream." complexity="O(N)" />
        <CommandEntry name="XLEN" syntax="XLEN key" desc="Get the number of entries in a stream." complexity="O(1)" />
        <CommandEntry name="XGROUP CREATE" syntax="XGROUP CREATE key group id|$ [MKSTREAM]" desc="Create a consumer group." complexity="O(1)" />
        <CommandEntry name="XREADGROUP" syntax="XREADGROUP GROUP group consumer [COUNT count] [BLOCK ms] STREAMS key [key ...] id [id ...]" desc="Read entries from a stream via consumer group." complexity="O(N)" />
        <CommandEntry name="XACK" syntax="XACK key group id [id ...]" desc="Acknowledge one or more messages in a consumer group." complexity="O(N)" />
      </div>

      <CodeBlock
        language="bash"
        title="Streams example"
        code={`# Append events to a stream
XADD events * type "page_view" url "/home" user_id "42"
XADD events * type "click" target "signup_btn" user_id "42"

# Read all events
XRANGE events - +

# Create consumer group
XGROUP CREATE events analytics $ MKSTREAM

# Read as consumer in group
XREADGROUP GROUP analytics worker-1 COUNT 10 STREAMS events >

# Acknowledge processed messages
XACK events analytics "1705000000000-0"

# Blocking read (wait for new events)
XREADGROUP GROUP analytics worker-1 COUNT 1 BLOCK 5000 STREAMS events >`}
      />

      {/* ── Pub/Sub ──────────────────────────────────────────── */}
      <DocHeading id="pubsub" level={2}>
        <MessageSquare className="w-5 h-5 text-blue-400" />
        Pub/Sub
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Publish/Subscribe messaging for real-time communication between clients.
      </p>

      <div className="rounded-xl border border-slate-800 overflow-hidden px-4 mb-4">
        <CommandEntry name="SUBSCRIBE" syntax="SUBSCRIBE channel [channel ...]" desc="Subscribe to one or more channels." complexity="O(N)" />
        <CommandEntry name="UNSUBSCRIBE" syntax="UNSUBSCRIBE [channel [channel ...]]" desc="Unsubscribe from channels." complexity="O(N)" />
        <CommandEntry name="PUBLISH" syntax="PUBLISH channel message" desc="Post a message to a channel." complexity="O(N+M)" />
        <CommandEntry name="PSUBSCRIBE" syntax="PSUBSCRIBE pattern [pattern ...]" desc="Subscribe to channels matching a pattern." complexity="O(N)" />
        <CommandEntry name="PUNSUBSCRIBE" syntax="PUNSUBSCRIBE [pattern [pattern ...]]" desc="Unsubscribe from pattern-matched channels." complexity="O(N)" />
      </div>

      <CodeBlock
        language="bash"
        title="Pub/Sub example"
        code={`# Terminal 1: Subscribe to channels
SUBSCRIBE notifications:user:42
PSUBSCRIBE events:*

# Terminal 2: Publish messages
PUBLISH notifications:user:42 "You have a new message"
PUBLISH events:login "user:42 logged in"
PUBLISH events:purchase "user:42 bought item:99"

# Terminal 1 receives:
# 1) "message"
# 2) "notifications:user:42"
# 3) "You have a new message"
# 1) "pmessage"
# 2) "events:*"
# 3) "events:login"
# 4) "user:42 logged in"`}
      />

      {/* ── Keys ─────────────────────────────────────────────── */}
      <DocHeading id="keys" level={2}>
        <Key className="w-5 h-5 text-blue-400" />
        Keys &amp; Expiration
      </DocHeading>

      <div className="rounded-xl border border-slate-800 overflow-hidden px-4 mb-4">
        <CommandEntry name="DEL" syntax="DEL key [key ...]" desc="Delete one or more keys." complexity="O(N)" />
        <CommandEntry name="EXISTS" syntax="EXISTS key [key ...]" desc="Check if keys exist." complexity="O(N)" />
        <CommandEntry name="EXPIRE" syntax="EXPIRE key seconds" desc="Set a timeout on a key (seconds)." complexity="O(1)" />
        <CommandEntry name="PEXPIRE" syntax="PEXPIRE key milliseconds" desc="Set a timeout on a key (milliseconds)." complexity="O(1)" />
        <CommandEntry name="TTL" syntax="TTL key" desc="Get the remaining time to live in seconds." complexity="O(1)" />
        <CommandEntry name="PTTL" syntax="PTTL key" desc="Get the remaining time to live in milliseconds." complexity="O(1)" />
        <CommandEntry name="PERSIST" syntax="PERSIST key" desc="Remove the expiration from a key." complexity="O(1)" />
        <CommandEntry name="RENAME" syntax="RENAME key newkey" desc="Rename a key." complexity="O(1)" />
        <CommandEntry name="TYPE" syntax="TYPE key" desc="Get the type of a key." complexity="O(1)" />
        <CommandEntry name="SCAN" syntax="SCAN cursor [MATCH pattern] [COUNT count] [TYPE type]" desc="Incrementally iterate the keyspace." complexity="O(N)" />
        <CommandEntry name="KEYS" syntax="KEYS pattern" desc="Find all keys matching a pattern (use SCAN in production)." complexity="O(N)" />
      </div>

      <CodeBlock
        language="bash"
        title="Keys example"
        code={`# Set expiration
SET cache:page:home "<html>..." EX 300

# Check TTL
TTL cache:page:home
# => 298

# Remove expiration
PERSIST cache:page:home

# Iterate keys safely
SCAN 0 MATCH "user:*" COUNT 100
# => [cursor, [key1, key2, ...]]

# Check key type
TYPE user:1
# => "hash"`}
      />

      {/* ── Server Commands ──────────────────────────────────── */}
      <DocHeading id="server-cmds" level={2}>
        <Clock className="w-5 h-5 text-blue-400" />
        Server Commands
      </DocHeading>

      <div className="rounded-xl border border-slate-800 overflow-hidden px-4 mb-4">
        <CommandEntry name="PING" syntax="PING [message]" desc="Test server connectivity. Returns PONG or the given message." complexity="O(1)" />
        <CommandEntry name="INFO" syntax="INFO [section]" desc="Get server information and statistics." complexity="O(1)" />
        <CommandEntry name="DBSIZE" syntax="DBSIZE" desc="Return the number of keys in the current database." complexity="O(1)" />
        <CommandEntry name="FLUSHDB" syntax="FLUSHDB [ASYNC]" desc="Remove all keys from the current database." complexity="O(N)" />
        <CommandEntry name="FLUSHALL" syntax="FLUSHALL [ASYNC]" desc="Remove all keys from all databases." complexity="O(N)" />
        <CommandEntry name="CONFIG GET" syntax="CONFIG GET parameter" desc="Get a configuration parameter value." complexity="O(1)" />
        <CommandEntry name="CONFIG SET" syntax="CONFIG SET parameter value" desc="Set a configuration parameter at runtime." complexity="O(1)" />
        <CommandEntry name="CLIENT LIST" syntax="CLIENT LIST" desc="List connected clients." complexity="O(N)" />
        <CommandEntry name="SLOWLOG" syntax="SLOWLOG GET [count]" desc="Get the slow log entries." complexity="O(N)" />
        <CommandEntry name="SAVE" syntax="SAVE" desc="Synchronously save the dataset to disk." complexity="O(N)" />
        <CommandEntry name="BGSAVE" syntax="BGSAVE" desc="Asynchronously save the dataset to disk." complexity="O(1)" />
      </div>

      {/* ── Scripting ────────────────────────────────────────── */}
      <DocHeading id="scripting" level={2}>
        Scripting
      </DocHeading>

      <p className="mb-4 text-slate-400">
        CacheStorm supports Lua scripting for atomic multi-step operations.
      </p>

      <div className="rounded-xl border border-slate-800 overflow-hidden px-4 mb-4">
        <CommandEntry name="EVAL" syntax="EVAL script numkeys key [key ...] arg [arg ...]" desc="Execute a Lua script." complexity="O(N)" />
        <CommandEntry name="EVALSHA" syntax="EVALSHA sha1 numkeys key [key ...] arg [arg ...]" desc="Execute a cached Lua script by SHA1 hash." complexity="O(N)" />
        <CommandEntry name="SCRIPT LOAD" syntax="SCRIPT LOAD script" desc="Load a Lua script into the script cache." complexity="O(N)" />
        <CommandEntry name="SCRIPT EXISTS" syntax="SCRIPT EXISTS sha1 [sha1 ...]" desc="Check if scripts exist in the cache." complexity="O(N)" />
      </div>

      <CodeBlock
        language="bash"
        title="Lua scripting example"
        code={`# Atomic check-and-set
EVAL "
  local current = redis.call('GET', KEYS[1])
  if current == ARGV[1] then
    redis.call('SET', KEYS[1], ARGV[2])
    return 1
  end
  return 0
" 1 mykey "expected_value" "new_value"

# Rate limiter script
EVAL "
  local key = KEYS[1]
  local limit = tonumber(ARGV[1])
  local window = tonumber(ARGV[2])
  local current = redis.call('INCR', key)
  if current == 1 then
    redis.call('EXPIRE', key, window)
  end
  if current > limit then
    return 0
  end
  return 1
" 1 "rate:api:user:42" 100 60`}
      />

      <InfoBox type="tip">
        Use <code className="text-xs bg-slate-800 px-1 py-0.5 rounded">EVALSHA</code> in production
        to avoid sending the full script text on every call. Load scripts once with{" "}
        <code className="text-xs bg-slate-800 px-1 py-0.5 rounded">SCRIPT LOAD</code> and call them
        by their SHA1 hash.
      </InfoBox>
    </DocsLayout>
  );
}
