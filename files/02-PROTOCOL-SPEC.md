# CacheStorm — Protocol & Command Specification

## 1. RESP3 Protocol Implementation

CacheStorm implements the RESP3 protocol (Redis Serialization Protocol v3). This ensures compatibility with all existing Redis client libraries.

### 1.1 RESP3 Data Types

```
Type        Prefix  Example Wire Format              Go Representation
───────────────────────────────────────────────────────────────────────
Simple String  +    +OK\r\n                           string
Simple Error   -    -ERR unknown command\r\n          error
Integer        :    :1000\r\n                         int64
Bulk String    $    $5\r\nhello\r\n                   []byte
Array          *    *2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n  []RESPValue
Null           _    _\r\n                             nil
Boolean        #    #t\r\n or #f\r\n                  bool
Map            %    %2\r\n+key1\r\n+val1\r\n...       map
Null Bulk Str  $    $-1\r\n                           nil (for GET miss)
Null Array     *    *-1\r\n                           nil
```

### 1.2 RESP Reader Implementation

```go
package resp

import (
    "bufio"
    "fmt"
    "io"
    "strconv"
)

type Type byte

const (
    TypeSimpleString Type = '+'
    TypeError        Type = '-'
    TypeInteger      Type = ':'
    TypeBulkString   Type = '$'
    TypeArray        Type = '*'
    TypeNull         Type = '_'
    TypeBoolean      Type = '#'
    TypeMap          Type = '%'
)

// Value represents a parsed RESP value.
type Value struct {
    Type    Type
    Str     string   // for SimpleString, Error
    Int     int64    // for Integer
    Bulk    []byte   // for BulkString
    Array   []Value  // for Array
    IsNull  bool     // for Null types
    Boolean bool     // for Boolean
}

// Reader reads and parses RESP messages from a buffered reader.
type Reader struct {
    rd *bufio.Reader
}

func NewReader(rd io.Reader) *Reader {
    return &Reader{rd: bufio.NewReaderSize(rd, 4096)}
}

// ReadValue reads the next complete RESP value.
// Returns io.EOF when connection closes.
func (r *Reader) ReadValue() (Value, error) {
    // Read type byte
    // Read until \r\n
    // Parse based on type
    // For arrays: recursively read N values
    // For bulk strings: read exact N bytes + \r\n
    // Return parsed Value
}

// ReadCommand reads a RESP array as a command (common case).
// Returns the command name (uppercase) and args as [][]byte.
func (r *Reader) ReadCommand() (cmd string, args [][]byte, err error) {
    val, err := r.ReadValue()
    if err != nil {
        return "", nil, err
    }
    if val.Type != TypeArray || len(val.Array) == 0 {
        return "", nil, fmt.Errorf("expected array, got %c", val.Type)
    }
    // First element is command name
    cmd = strings.ToUpper(string(val.Array[0].Bulk))
    // Rest are args
    for _, v := range val.Array[1:] {
        args = append(args, v.Bulk)
    }
    return cmd, args, nil
}
```

### 1.3 RESP Writer Implementation

```go
// Writer serializes RESP values to a buffered writer.
type Writer struct {
    wr *bufio.Writer
}

func NewWriter(wr io.Writer) *Writer {
    return &Writer{wr: bufio.NewWriterSize(wr, 4096)}
}

// Core write methods:
func (w *Writer) WriteSimpleString(s string) error   // +OK\r\n
func (w *Writer) WriteError(msg string) error         // -ERR ...\r\n
func (w *Writer) WriteInteger(n int64) error           // :1000\r\n
func (w *Writer) WriteBulkString(b []byte) error       // $5\r\nhello\r\n
func (w *Writer) WriteNull() error                     // $-1\r\n
func (w *Writer) WriteArray(n int) error               // *N\r\n (then write N values)
func (w *Writer) WriteNullArray() error                // *-1\r\n
func (w *Writer) Flush() error                         // flush buffer to wire
```

## 2. Command Specifications

### 2.1 Command Router

```go
type CommandHandler func(ctx *CommandContext) error

type CommandDef struct {
    Name     string
    Handler  CommandHandler
    MinArgs  int   // minimum number of args (excluding command name)
    MaxArgs  int   // maximum args, -1 = unlimited
    ReadOnly bool  // true = read-only command (can run on replica)
    NoAuth   bool  // true = can run without AUTH (PING, AUTH, QUIT)
}

// CommandContext is passed to every command handler.
type CommandContext struct {
    Conn      *Connection
    Args      [][]byte        // command arguments (NOT including command name)
    Namespace *Namespace      // resolved namespace for this connection
    Server    *Server         // reference for server-level commands
    StartTime time.Time       // for latency tracking
}

// Helper methods on CommandContext:
func (ctx *CommandContext) ArgString(i int) string     // args[i] as string
func (ctx *CommandContext) ArgInt64(i int) (int64, error)  // parse args[i] as int64
func (ctx *CommandContext) ArgFloat64(i int) (float64, error)
func (ctx *CommandContext) WriteOK() error             // shorthand for +OK
func (ctx *CommandContext) WriteError(msg string) error
func (ctx *CommandContext) WriteInteger(n int64) error
func (ctx *CommandContext) WriteBulk(b []byte) error
func (ctx *CommandContext) WriteNull() error
func (ctx *CommandContext) WriteArray(items [][]byte) error
func (ctx *CommandContext) WriteStringArray(items []string) error
```

### 2.2 String Commands

```
┌─────────────────┬──────────────────────────────────────────────────────────────┐
│ Command         │ Specification                                                │
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ SET key value   │ SET key value [EX seconds] [PX milliseconds] [NX|XX]       │
│                 │ [KEEPTTL] [GET]                                              │
│                 │ Returns: OK, or bulk string (with GET), or null (NX/XX fail)│
│                 │ NX = only set if not exists                                  │
│                 │ XX = only set if exists                                      │
│                 │ GET = return old value                                        │
│                 │ KEEPTTL = preserve existing TTL                              │
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ GET key         │ Returns: bulk string value, or null if key doesn't exist    │
│                 │ If key type != string → WRONGTYPE error                     │
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ MSET k v [k v]  │ Set multiple keys. Always returns OK.                       │
│                 │ Atomic: all or nothing (within same shard, best-effort cross)│
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ MGET k [k ...]  │ Returns: array of values (null for missing keys)            │
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ DEL key [key]   │ Delete one or more keys. Returns: integer (count deleted)   │
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ EXISTS key [key]│ Returns: integer count of existing keys                     │
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ INCR key        │ Increment by 1. Returns new value. Creates key with 0 if    │
│                 │ missing. Error if value not parseable as integer.            │
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ DECR key        │ Decrement by 1. Same rules as INCR.                         │
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ INCRBY key n    │ Increment by n (integer). Returns new value.                │
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ DECRBY key n    │ Decrement by n. Returns new value.                          │
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ INCRBYFLOAT k n │ Increment by float. Returns new value as bulk string.       │
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ APPEND key val  │ Append to string. Returns: new length. Creates if missing.  │
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ STRLEN key      │ Returns: integer length of string value. 0 if missing.      │
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ GETRANGE k s e  │ Returns substring [start, end] inclusive. Negative index ok.│
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ SETRANGE k off v│ Overwrite at offset. Returns: new length.                   │
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ SETNX key value │ SET if Not eXists. Returns: 1 if set, 0 if not.            │
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ SETEX key sec v │ SET with EXpire in seconds. Returns: OK.                   │
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ PSETEX key ms v │ SET with EXpire in milliseconds. Returns: OK.              │
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ GETSET key val  │ Set new value, return old. Deprecated but supported.        │
├─────────────────┼──────────────────────────────────────────────────────────────┤
│ GETDEL key      │ Get value and delete key. Returns: value or null.           │
└─────────────────┴──────────────────────────────────────────────────────────────┘
```

### 2.3 Key Commands

```
┌──────────────────────┬────────────────────────────────────────────────────────┐
│ Command              │ Specification                                          │
├──────────────────────┼────────────────────────────────────────────────────────┤
│ EXPIRE key seconds   │ Set TTL in seconds. Returns: 1 if set, 0 if no key.  │
├──────────────────────┼────────────────────────────────────────────────────────┤
│ PEXPIRE key ms       │ Set TTL in milliseconds.                              │
├──────────────────────┼────────────────────────────────────────────────────────┤
│ EXPIREAT key ts      │ Set expire at unix timestamp (seconds).               │
├──────────────────────┼────────────────────────────────────────────────────────┤
│ PEXPIREAT key ts     │ Set expire at unix timestamp (milliseconds).          │
├──────────────────────┼────────────────────────────────────────────────────────┤
│ TTL key              │ Returns: seconds remaining, -1 no TTL, -2 no key.    │
├──────────────────────┼────────────────────────────────────────────────────────┤
│ PTTL key             │ Returns: milliseconds remaining, -1 no TTL, -2 no key│
├──────────────────────┼────────────────────────────────────────────────────────┤
│ PERSIST key          │ Remove TTL. Returns: 1 if removed, 0 if no TTL/key.  │
├──────────────────────┼────────────────────────────────────────────────────────┤
│ TYPE key             │ Returns: string type name ("string","hash","list","set")│
│                      │ or "none" if key doesn't exist.                        │
├──────────────────────┼────────────────────────────────────────────────────────┤
│ RENAME key newkey    │ Rename key. Error if source doesn't exist.            │
├──────────────────────┼────────────────────────────────────────────────────────┤
│ KEYS pattern         │ Return all keys matching glob pattern.                │
│                      │ Patterns: * ? [abc] [a-z] supported.                  │
│                      │ WARNING: blocks shard — use SCAN for production.       │
├──────────────────────┼────────────────────────────────────────────────────────┤
│ SCAN cursor [MATCH p]│ Incrementally iterate keys.                           │
│ [COUNT n]            │ cursor=0 starts, returns new cursor + keys.            │
│                      │ cursor=0 in response means complete.                   │
│                      │ Implement cursor as: shard_index:offset encoded.       │
├──────────────────────┼────────────────────────────────────────────────────────┤
│ RANDOMKEY            │ Return a random key from current namespace.            │
├──────────────────────┼────────────────────────────────────────────────────────┤
│ UNLINK key [key...]  │ Same as DEL but async (in Go, just use DEL logic).    │
└──────────────────────┴────────────────────────────────────────────────────────┘
```

### 2.4 Hash Commands

```
All WRONGTYPE if key exists but is not a hash.

┌──────────────────────────┬──────────────────────────────────────────────────┐
│ HSET key f v [f v ...]   │ Set field(s). Returns: count of NEW fields.     │
├──────────────────────────┼──────────────────────────────────────────────────┤
│ HGET key field           │ Returns: value or null.                         │
├──────────────────────────┼──────────────────────────────────────────────────┤
│ HMSET key f v [f v ...]  │ Set multiple fields. Always returns OK.         │
├──────────────────────────┼──────────────────────────────────────────────────┤
│ HMGET key f [f ...]      │ Returns: array of values (null for missing).    │
├──────────────────────────┼──────────────────────────────────────────────────┤
│ HDEL key f [f ...]       │ Delete field(s). Returns: count deleted.        │
│                          │ If all fields deleted, key is removed.           │
├──────────────────────────┼──────────────────────────────────────────────────┤
│ HGETALL key              │ Returns: flat array [f1, v1, f2, v2, ...].      │
├──────────────────────────┼──────────────────────────────────────────────────┤
│ HEXISTS key field        │ Returns: 1 or 0.                                │
├──────────────────────────┼──────────────────────────────────────────────────┤
│ HLEN key                 │ Returns: number of fields. 0 if missing.        │
├──────────────────────────┼──────────────────────────────────────────────────┤
│ HKEYS key                │ Returns: array of field names.                  │
├──────────────────────────┼──────────────────────────────────────────────────┤
│ HVALS key                │ Returns: array of values.                       │
├──────────────────────────┼──────────────────────────────────────────────────┤
│ HINCRBY key field n      │ Increment field by integer n. Creates if needed.│
├──────────────────────────┼──────────────────────────────────────────────────┤
│ HINCRBYFLOAT key field n │ Increment field by float n.                     │
├──────────────────────────┼──────────────────────────────────────────────────┤
│ HSETNX key field value   │ Set field only if not exists. Returns: 1 or 0.  │
└──────────────────────────┴──────────────────────────────────────────────────┘
```

### 2.5 List Commands

```
All WRONGTYPE if key exists but is not a list.
Lists are implemented as slice-based deque (append/prepend are O(1) amortized).
Empty list = key auto-deleted.

┌──────────────────────────────┬──────────────────────────────────────────┐
│ LPUSH key element [element]  │ Prepend. Returns: list length after.     │
├──────────────────────────────┼──────────────────────────────────────────┤
│ RPUSH key element [element]  │ Append. Returns: list length after.      │
├──────────────────────────────┼──────────────────────────────────────────┤
│ LPOP key [count]             │ Remove+return from head. Null if empty.  │
│                              │ With count: returns array.                │
├──────────────────────────────┼──────────────────────────────────────────┤
│ RPOP key [count]             │ Remove+return from tail.                 │
├──────────────────────────────┼──────────────────────────────────────────┤
│ LLEN key                     │ Returns: list length. 0 if missing.      │
├──────────────────────────────┼──────────────────────────────────────────┤
│ LRANGE key start stop        │ Returns: sublist [start, stop] inclusive. │
│                              │ Negative indices supported (-1 = last).   │
├──────────────────────────────┼──────────────────────────────────────────┤
│ LINDEX key index             │ Returns: element at index, or null.      │
├──────────────────────────────┼──────────────────────────────────────────┤
│ LSET key index element       │ Set element at index. Error if OOB.      │
├──────────────────────────────┼──────────────────────────────────────────┤
│ LREM key count element       │ Remove count occurrences of element.      │
│                              │ count>0: from head, count<0: from tail,   │
│                              │ count=0: all. Returns: count removed.     │
├──────────────────────────────┼──────────────────────────────────────────┤
│ LPOS key element [RANK r]    │ Returns: index of element (first match). │
│ [COUNT c] [MAXLEN m]        │ null if not found.                        │
└──────────────────────────────┴──────────────────────────────────────────┘
```

### 2.6 Set Commands

```
All WRONGTYPE if key exists but is not a set.
Empty set = key auto-deleted.

┌────────────────────────────────┬────────────────────────────────────────┐
│ SADD key member [member ...]   │ Add members. Returns: count of NEW.    │
├────────────────────────────────┼────────────────────────────────────────┤
│ SREM key member [member ...]   │ Remove members. Returns: count removed.│
├────────────────────────────────┼────────────────────────────────────────┤
│ SMEMBERS key                   │ Returns: array of all members.         │
├────────────────────────────────┼────────────────────────────────────────┤
│ SISMEMBER key member           │ Returns: 1 or 0.                      │
├────────────────────────────────┼────────────────────────────────────────┤
│ SMISMEMBER key m [m ...]       │ Returns: array of 1/0 for each member.│
├────────────────────────────────┼────────────────────────────────────────┤
│ SCARD key                      │ Returns: set cardinality. 0 if missing.│
├────────────────────────────────┼────────────────────────────────────────┤
│ SUNION key [key ...]           │ Returns: union of all sets.            │
├────────────────────────────────┼────────────────────────────────────────┤
│ SINTER key [key ...]           │ Returns: intersection of all sets.     │
├────────────────────────────────┼────────────────────────────────────────┤
│ SDIFF key [key ...]            │ Returns: difference (first - others).  │
├────────────────────────────────┼────────────────────────────────────────┤
│ SUNIONSTORE dest key [key ...] │ Store union in dest. Returns: count.   │
├────────────────────────────────┼────────────────────────────────────────┤
│ SINTERSTORE dest key [key ...] │ Store intersection in dest. Returns: n.│
├────────────────────────────────┼────────────────────────────────────────┤
│ SDIFFSTORE dest key [key ...]  │ Store difference in dest. Returns: n.  │
├────────────────────────────────┼────────────────────────────────────────┤
│ SRANDMEMBER key [count]        │ Return random member(s). No removal.   │
├────────────────────────────────┼────────────────────────────────────────┤
│ SPOP key [count]               │ Remove+return random member(s).        │
└────────────────────────────────┴────────────────────────────────────────┘
```

### 2.7 CacheStorm Tag Commands (Custom — NOT in Redis)

```
These are CacheStorm-specific commands. Redis clients can still send them
as regular commands since RESP protocol allows any command name.

┌──────────────────────────────────────┬──────────────────────────────────────────────────────┐
│ SETTAG key value TAG t1 [t2...]      │ SET key with tags in one atomic operation.           │
│ [EX s] [PX ms] [NX|XX]              │ Same options as SET but with TAG clause.              │
│                                      │ Returns: OK or null (NX/XX fail).                    │
│                                      │ Implementation: SET the key, then register tags.     │
│                                      │ Must be atomic within the shard.                      │
├──────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ TAGS key                             │ Get all tags associated with a key.                  │
│                                      │ Returns: array of tag names, or empty array.         │
│                                      │ Returns null if key doesn't exist.                   │
├──────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ ADDTAG key tag [tag ...]             │ Add tags to an existing key.                         │
│                                      │ Returns: count of NEW tags added.                    │
│                                      │ Error if key doesn't exist.                          │
├──────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ REMTAG key tag [tag ...]             │ Remove tags from a key.                              │
│                                      │ Returns: count of tags removed.                      │
│                                      │ Error if key doesn't exist.                          │
├──────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ INVALIDATE tag [tag ...]             │ Delete ALL keys associated with given tag(s).        │
│                                      │ This is the killer feature.                           │
│                                      │ Returns: total count of keys deleted.                │
│                                      │ Process: for each tag →                               │
│                                      │   1. Get all keys from reverse index                 │
│                                      │   2. Delete each key from store                      │
│                                      │   3. Clean up other tag associations                 │
│                                      │   4. Remove tag from reverse index                   │
│                                      │   5. In cluster: broadcast to other nodes            │
│                                      │ Must trigger OnEvict hooks for each deleted key.      │
├──────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ TAGKEYS tag                          │ List all keys belonging to a tag.                    │
│                                      │ Returns: array of key names.                         │
│                                      │ Empty array if tag doesn't exist.                    │
├──────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ TAGCOUNT tag                         │ Returns: integer count of keys in the tag.           │
│                                      │ 0 if tag doesn't exist.                              │
├──────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ TAGINVALIDATE tag [CASCADE]          │ Like INVALIDATE but with optional cascade.           │
│                                      │ CASCADE: also invalidate child tags.                 │
│                                      │ Returns: total count of keys deleted.                │
├──────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ TAGLINK parent child                 │ Create tag hierarchy (parent → child).               │
│                                      │ When parent is invalidated with CASCADE,              │
│                                      │ child tags are also invalidated.                     │
│                                      │ Returns: OK.                                          │
├──────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ TAGUNLINK parent child               │ Remove tag hierarchy link.                           │
│                                      │ Returns: 1 if removed, 0 if link didn't exist.      │
├──────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ TAGCHILDREN tag                      │ List child tags of a parent tag.                     │
│                                      │ Returns: array of child tag names.                   │
└──────────────────────────────────────┴──────────────────────────────────────────────────────┘
```

### 2.8 Namespace Commands (Custom)

```
┌──────────────────────────────┬──────────────────────────────────────────────────┐
│ NAMESPACE name               │ Switch current connection to named namespace.    │
│                              │ Creates namespace if it doesn't exist.           │
│                              │ Returns: OK.                                      │
│                              │ This replaces Redis SELECT command functionality. │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ NAMESPACES                   │ List all namespace names.                        │
│                              │ Returns: array of strings.                       │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ NAMESPACEDEL name            │ Delete a namespace and all its data.             │
│                              │ Cannot delete "default".                          │
│                              │ Returns: OK or error.                             │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ NAMESPACEINFO [name]         │ Returns: key count, memory usage, tag count.     │
│                              │ If name omitted, shows current namespace.        │
└──────────────────────────────┴──────────────────────────────────────────────────┘
```

### 2.9 Server Commands

```
┌──────────────────────────────┬──────────────────────────────────────────────────┐
│ PING [message]               │ Returns: PONG or echo message.                  │
│                              │ NoAuth: true (works before AUTH).                │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ ECHO message                 │ Returns: the message back.                      │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ QUIT                         │ Close connection. Returns: OK before closing.    │
│                              │ NoAuth: true.                                     │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ AUTH password                │ Authenticate. Returns: OK or error.              │
│                              │ NoAuth: true.                                     │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ SELECT index                 │ For Redis compat: SELECT 0 = "default" ns.      │
│                              │ SELECT n>0 = namespace "db{n}".                  │
│                              │ Recommend: use NAMESPACE instead.                │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ INFO [section]               │ Returns: bulk string with server info.           │
│                              │ Sections: server, memory, stats, clients,        │
│                              │ keyspace, cluster, namespaces, tags, plugins.    │
│                              │ No section = all. Format: "key:value\r\n".       │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ DBSIZE                       │ Returns: integer key count in current namespace. │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ FLUSHDB [ASYNC]              │ Delete all keys in current namespace.            │
│                              │ Returns: OK.                                      │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ FLUSHALL [ASYNC]             │ Delete all keys in ALL namespaces.               │
│                              │ Returns: OK.                                      │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ COMMAND                      │ Returns: array of supported command names.       │
│ COMMAND COUNT                │ Returns: integer count of commands.              │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ CONFIG GET pattern           │ Get config values matching pattern.              │
│ CONFIG SET key value         │ Set runtime config (limited subset).             │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ HOTKEYS [count]              │ CacheStorm custom: top N most accessed keys.     │
│                              │ Default count: 10. Max: 100.                     │
│                              │ Returns: array of [key, access_count] pairs.     │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ MEMINFO [namespace|tag name] │ CacheStorm custom: memory breakdown.             │
│                              │ No args: global memory info.                      │
│                              │ namespace name: memory for that namespace.        │
│                              │ tag name: memory for keys in that tag.            │
│                              │ Returns: bulk string with detailed breakdown.    │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ TIME                         │ Returns: [unix_seconds, microseconds] array.     │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ WAIT                         │ (cluster) Wait for replication. Returns: count.  │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ CLIENT LIST                  │ Returns: connected client info.                  │
│ CLIENT GETNAME               │ Returns: client name or null.                    │
│ CLIENT SETNAME name          │ Set client name. Returns: OK.                    │
│ CLIENT ID                    │ Returns: integer client ID.                      │
└──────────────────────────────┴──────────────────────────────────────────────────┘
```

### 2.10 Cluster Commands

```
These are only available when cluster mode is enabled.

┌──────────────────────────────┬──────────────────────────────────────────────────┐
│ CLUSTER INFO                 │ Returns: bulk string with cluster state.         │
│                              │ cluster_enabled, cluster_state, cluster_slots,   │
│                              │ cluster_known_nodes, cluster_size.               │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ CLUSTER NODES                │ Returns: bulk string. One line per node.         │
│                              │ Format: id ip:port@gossipport role slots         │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ CLUSTER SLOTS                │ Returns: array of slot ranges + nodes.           │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ CLUSTER MEET ip port         │ Add a node to the cluster.                       │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ CLUSTER REPLICATE node-id    │ Make current node replica of given node.         │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ CLUSTER MYID                 │ Returns: this node's ID.                         │
├──────────────────────────────┼──────────────────────────────────────────────────┤
│ CLUSTER RESET [HARD|SOFT]    │ Reset cluster state.                             │
└──────────────────────────────┴──────────────────────────────────────────────────┘
```

## 3. Wire Protocol Examples

### Simple SET/GET:
```
Client → Server:
*3\r\n$3\r\nSET\r\n$4\r\nname\r\n$5\r\nErsin\r\n

Server → Client:
+OK\r\n

Client → Server:
*2\r\n$3\r\nGET\r\n$4\r\nname\r\n

Server → Client:
$5\r\nErsin\r\n
```

### SETTAG (Custom):
```
Client → Server:
*7\r\n$6\r\nSETTAG\r\n$8\r\nuser:123\r\n$15\r\n{"name":"Ersin"}\r\n$3\r\nTAG\r\n$5\r\nusers\r\n$7\r\npremium\r\n$2\r\nEX\r\n$4\r\n3600\r\n

Server → Client:
+OK\r\n
```

### INVALIDATE:
```
Client → Server:
*2\r\n$10\r\nINVALIDATE\r\n$5\r\nusers\r\n

Server → Client:
:42\r\n
(42 keys were deleted)
```
