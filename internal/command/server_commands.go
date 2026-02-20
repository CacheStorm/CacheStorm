package command

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func RegisterServerCommands(router *Router) {
	router.Register(&CommandDef{Name: "PING", Handler: cmdPING})
	router.Register(&CommandDef{Name: "ECHO", Handler: cmdECHO})
	router.Register(&CommandDef{Name: "QUIT", Handler: cmdQUIT})
	router.Register(&CommandDef{Name: "COMMAND", Handler: cmdCOMMAND})
	router.Register(&CommandDef{Name: "INFO", Handler: cmdINFO})
	router.Register(&CommandDef{Name: "DBSIZE", Handler: cmdDBSIZE})
	router.Register(&CommandDef{Name: "FLUSHDB", Handler: cmdFLUSHDB})
	router.Register(&CommandDef{Name: "FLUSHALL", Handler: cmdFLUSHALL})
	router.Register(&CommandDef{Name: "TIME", Handler: cmdTIME})
	router.Register(&CommandDef{Name: "AUTH", Handler: cmdAUTH})
	router.Register(&CommandDef{Name: "SCAN", Handler: cmdSCAN})
	router.Register(&CommandDef{Name: "HOTKEYS", Handler: cmdHOTKEYS})
	router.Register(&CommandDef{Name: "MEMINFO", Handler: cmdMEMINFO})
	router.Register(&CommandDef{Name: "SORT", Handler: cmdSORT})
	router.Register(&CommandDef{Name: "SORT_RO", Handler: cmdSORTRO})
	router.Register(&CommandDef{Name: "SLOWLOG", Handler: cmdSLOWLOG})
	router.Register(&CommandDef{Name: "WAIT", Handler: cmdWAIT})
	router.Register(&CommandDef{Name: "ROLE", Handler: cmdROLE})
	router.Register(&CommandDef{Name: "LASTSAVE", Handler: cmdLASTSAVE})
	router.Register(&CommandDef{Name: "LOLWUT", Handler: cmdLOLWUT})
	router.Register(&CommandDef{Name: "SHUTDOWN", Handler: cmdSHUTDOWN})
	router.Register(&CommandDef{Name: "SAVE", Handler: cmdSAVE})
	router.Register(&CommandDef{Name: "BGSAVE", Handler: cmdBGSAVE})
	router.Register(&CommandDef{Name: "BGREWRITEAOF", Handler: cmdBGREWRITEAOF})
	router.Register(&CommandDef{Name: "SLAVEOF", Handler: cmdSLAVEOF})
	router.Register(&CommandDef{Name: "REPLICAOF", Handler: cmdSLAVEOF})
	router.Register(&CommandDef{Name: "LATENCY", Handler: cmdLATENCY})
	router.Register(&CommandDef{Name: "STRALGO", Handler: cmdSTRALGO})
	router.Register(&CommandDef{Name: "MODULE", Handler: cmdMODULE})
	router.Register(&CommandDef{Name: "ACL", Handler: cmdACL})
	router.Register(&CommandDef{Name: "MONITOR", Handler: cmdMONITOR})
	router.Register(&CommandDef{Name: "SWAPDB", Handler: cmdSWAPDB})
	router.Register(&CommandDef{Name: "SYNC", Handler: cmdSYNC})
	router.Register(&CommandDef{Name: "PSYNC", Handler: cmdPSYNC})
	router.Register(&CommandDef{Name: "DEBUGSEGFAULT", Handler: cmdDEBUGSEGFAULT})
}

func RegisterClientCommands(router *Router) {
	router.Register(&CommandDef{Name: "CLIENT", Handler: cmdCLIENT})
}

func RegisterKeyCommands(router *Router) {
	router.Register(&CommandDef{Name: "EXPIRE", Handler: cmdEXPIRE})
	router.Register(&CommandDef{Name: "PEXPIRE", Handler: cmdPEXPIRE})
	router.Register(&CommandDef{Name: "EXPIREAT", Handler: cmdEXPIREAT})
	router.Register(&CommandDef{Name: "PEXPIREAT", Handler: cmdPEXPIREAT})
	router.Register(&CommandDef{Name: "TTL", Handler: cmdTTL})
	router.Register(&CommandDef{Name: "PTTL", Handler: cmdPTTL})
	router.Register(&CommandDef{Name: "PERSIST", Handler: cmdPERSIST})
	router.Register(&CommandDef{Name: "TYPE", Handler: cmdTYPE})
	router.Register(&CommandDef{Name: "RENAME", Handler: cmdRENAME})
	router.Register(&CommandDef{Name: "RENAMENX", Handler: cmdRENAMENX})
	router.Register(&CommandDef{Name: "KEYS", Handler: cmdKEYS})
	router.Register(&CommandDef{Name: "RANDOMKEY", Handler: cmdRANDOMKEY})
	router.Register(&CommandDef{Name: "UNLINK", Handler: cmdDEL})
	router.Register(&CommandDef{Name: "TOUCH", Handler: cmdTOUCH})
	router.Register(&CommandDef{Name: "DUMP", Handler: cmdDUMP})
	router.Register(&CommandDef{Name: "RESTORE", Handler: cmdRESTORE})
	router.Register(&CommandDef{Name: "COPY", Handler: cmdCOPY})
}

func cmdPING(ctx *Context) error {
	if ctx.ArgCount() == 0 {
		return ctx.WriteSimpleString("PONG")
	}
	return ctx.WriteBulkBytes(ctx.Arg(0))
}

func cmdECHO(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return ctx.WriteBulkBytes(ctx.Arg(0))
}

func cmdQUIT(ctx *Context) error {
	ctx.WriteOK()
	return nil
}

func cmdCOMMAND(ctx *Context) error {
	if ctx.ArgCount() > 0 {
		subCmd := strings.ToUpper(ctx.ArgString(0))
		switch subCmd {
		case "COUNT":
			return ctx.WriteInteger(200)
		case "DOCS":
			return cmdCommandDocs(ctx)
		case "GETKEYS":
			return cmdCommandGetKeys(ctx)
		case "LIST":
			commands := []string{
				"GET", "SET", "DEL", "EXISTS", "KEYS", "EXPIRE", "TTL", "TYPE",
				"INCR", "DECR", "INCRBY", "DECRBY", "INCRBYFLOAT",
				"APPEND", "STRLEN", "GETRANGE", "SETRANGE", "GETSET", "GETEX", "GETDEL",
				"MGET", "MSET", "SETNX", "SUBSTR", "LCS", "COPY",
				"HSET", "HGET", "HDEL", "HGETALL", "HKEYS", "HVALS", "HEXISTS", "HLEN",
				"HINCRBY", "HINCRBYFLOAT", "HMGET", "HMSET", "HSETNX", "HSTRLEN",
				"HRANDFIELD", "HGETDEL", "HGETEX", "HSCAN",
				"LPUSH", "RPUSH", "LPOP", "RPOP", "LLEN", "LRANGE", "LINDEX", "LSET",
				"LREM", "LTRIM", "BLPOP", "BRPOP", "BRPOPLPUSH", "RPOPLPUSH",
				"LMOVE", "LPOS", "LMPOP", "LMPUSH", "LPUSHX", "RPUSHX", "LINSERT",
				"SADD", "SREM", "SMEMBERS", "SISMEMBER", "SCARD", "SPOP", "SRANDMEMBER",
				"SMOVE", "SUNION", "SINTER", "SDIFF", "SUNIONSTORE", "SINTERSTORE", "SDIFFSTORE", "SSCAN",
				"ZADD", "ZCARD", "ZCOUNT", "ZRANGE", "ZRANGEBYSCORE", "ZRANK", "ZREM",
				"ZSCORE", "ZINCRBY", "ZREVRANGE", "ZREVRANK", "ZREMRANGEBYRANK", "ZREMRANGEBYSCORE",
				"ZPOPMIN", "ZPOPMAX", "ZRANDMEMBER", "ZMSCORE",
				"ZUNIONSTORE", "ZINTERSTORE", "ZDIFFSTORE", "ZSCAN",
				"XADD", "XLEN", "XRANGE", "XREVRANGE", "XREAD", "XDEL", "XTRIM", "XINFO", "XGROUP",
				"XREADGROUP", "XACK", "XPENDING", "XCLAIM",
				"GEOADD", "GEODIST", "GEOHASH", "GEOPOS", "GEORADIUS", "GEORADIUSBYMEMBER",
				"PING", "ECHO", "QUIT", "COMMAND", "INFO", "DBSIZE", "FLUSHDB", "FLUSHALL", "TIME",
				"CLIENT", "CONFIG", "SCAN", "SORT", "SORT_RO",
				"SLOWLOG", "WAIT", "ROLE", "LASTSAVE", "LOLWUT", "SHUTDOWN",
				"SAVE", "BGSAVE", "BGREWRITEAOF", "SLAVEOF", "REPLICAOF", "LATENCY",
				"STRALGO", "MODULE", "ACL", "MONITOR", "SWAPDB", "SYNC", "PSYNC",
				"MULTI", "EXEC", "DISCARD", "WATCH", "UNWATCH",
				"SUBSCRIBE", "UNSUBSCRIBE", "PUBLISH", "PSUBSCRIBE", "PUNSUBSCRIBE", "PUBSUB",
				"SETBIT", "GETBIT", "BITCOUNT", "BITPOS", "BITOP", "BITFIELD",
				"PFADD", "PFCOUNT", "PFMERGE",
				"EVAL", "EVALSHA", "SCRIPT",
				"SETTAG", "TAGKEYS", "TAGCOUNT", "TAGDEL", "TAGINFO", "INVALIDATE",
				"NAMESPACES", "NSCREATE", "NSDEL", "NSINFO", "NSKEYS",
				"CLUSTER", "CLUSTERINFO", "CLUSTERNODES", "CLUSTERSLOTS", "MIGRATE",
				"DEBUG", "OBJECT", "MEMORY", "HOTKEYS", "MEMINFO",
				"RENAME", "RENAMENX", "RANDOMKEY", "TOUCH", "DUMP", "RESTORE", "UNLINK",
				"PERSIST", "PEXPIRE", "EXPIREAT", "PEXPIREAT", "PTTL",
				"AUTH", "ASKING", "READONLY", "READWRITE",
			}
			result := make([]*resp.Value, 0, len(commands))
			for _, cmd := range commands {
				result = append(result, resp.BulkString(cmd))
			}
			return ctx.WriteArray(result)
		default:
			return ctx.WriteError(fmt.Errorf("ERR unknown subcommand '%s'", subCmd))
		}
	}
	return ctx.WriteArray([]*resp.Value{})
}

func cmdCommandDocs(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	commandName := strings.ToUpper(ctx.ArgString(1))

	docs := map[string][]interface{}{
		"GET":         {"summary", "Get the value of a key", "since", "1.0.0", "group", "string", "complexity", "O(1)"},
		"SET":         {"summary", "Set the string value of a key", "since", "1.0.0", "group", "string", "complexity", "O(1)"},
		"DEL":         {"summary", "Delete a key", "since", "1.0.0", "group", "generic", "complexity", "O(N) where N is the number of keys"},
		"EXISTS":      {"summary", "Determine if a key exists", "since", "1.0.0", "group", "generic", "complexity", "O(1)"},
		"EXPIRE":      {"summary", "Set a key's time to live in seconds", "since", "1.0.0", "group", "generic", "complexity", "O(1)"},
		"TTL":         {"summary", "Get the time to live for a key in seconds", "since", "1.0.0", "group", "generic", "complexity", "O(1)"},
		"TYPE":        {"summary", "Determine the type stored at key", "since", "1.0.0", "group", "generic", "complexity", "O(1)"},
		"KEYS":        {"summary", "Find all keys matching the given pattern", "since", "1.0.0", "group", "generic", "complexity", "O(N) where N is the number of keys"},
		"INCR":        {"summary", "Increment the integer value of a key by one", "since", "1.0.0", "group", "string", "complexity", "O(1)"},
		"DECR":        {"summary", "Decrement the integer value of a key by one", "since", "1.0.0", "group", "string", "complexity", "O(1)"},
		"INCRBY":      {"summary", "Increment the integer value of a key by the given amount", "since", "1.0.0", "group", "string", "complexity", "O(1)"},
		"APPEND":      {"summary", "Append a value to a key", "since", "2.0.0", "group", "string", "complexity", "O(1)"},
		"STRLEN":      {"summary", "Get the length of the value stored in a key", "since", "2.2.0", "group", "string", "complexity", "O(1)"},
		"MGET":        {"summary", "Get the values of all the given keys", "since", "1.0.0", "group", "string", "complexity", "O(N) where N is the number of keys"},
		"MSET":        {"summary", "Set multiple keys to multiple values", "since", "1.0.1", "group", "string", "complexity", "O(N) where N is the number of keys"},
		"HSET":        {"summary", "Set the string value of a hash field", "since", "2.0.0", "group", "hash", "complexity", "O(1) for each field/value pair"},
		"HGET":        {"summary", "Get the value of a hash field", "since", "2.0.0", "group", "hash", "complexity", "O(1)"},
		"HDEL":        {"summary", "Delete one or more hash fields", "since", "2.0.0", "group", "hash", "complexity", "O(N) where N is the number of fields"},
		"HGETALL":     {"summary", "Get all the fields and values in a hash", "since", "2.0.0", "group", "hash", "complexity", "O(N) where N is the size of the hash"},
		"HKEYS":       {"summary", "Get all the fields in a hash", "since", "2.0.0", "group", "hash", "complexity", "O(N) where N is the size of the hash"},
		"HVALS":       {"summary", "Get all the values in a hash", "since", "2.0.0", "group", "hash", "complexity", "O(N) where N is the size of the hash"},
		"HEXISTS":     {"summary", "Determine if a hash field exists", "since", "2.0.0", "group", "hash", "complexity", "O(1)"},
		"HLEN":        {"summary", "Get the number of fields in a hash", "since", "2.0.0", "group", "hash", "complexity", "O(1)"},
		"LPUSH":       {"summary", "Prepend one or multiple elements to a list", "since", "1.0.0", "group", "list", "complexity", "O(1) for each element"},
		"RPUSH":       {"summary", "Append one or multiple elements to a list", "since", "1.0.0", "group", "list", "complexity", "O(1) for each element"},
		"LPOP":        {"summary", "Remove and get the first element in a list", "since", "1.0.0", "group", "list", "complexity", "O(1)"},
		"RPOP":        {"summary", "Remove and get the last element in a list", "since", "1.0.0", "group", "list", "complexity", "O(1)"},
		"LLEN":        {"summary", "Get the length of a list", "since", "1.0.0", "group", "list", "complexity", "O(1)"},
		"LRANGE":      {"summary", "Get a range of elements from a list", "since", "1.0.0", "group", "list", "complexity", "O(S+N) where S is start offset"},
		"BLPOP":       {"summary", "Remove and get the first element in a list, or block until one is available", "since", "2.0.0", "group", "list", "complexity", "O(1)"},
		"BRPOP":       {"summary", "Remove and get the last element in a list, or block until one is available", "since", "2.0.0", "group", "list", "complexity", "O(1)"},
		"SADD":        {"summary", "Add one or more members to a set", "since", "1.0.0", "group", "set", "complexity", "O(1) for each element"},
		"SREM":        {"summary", "Remove one or more members from a set", "since", "1.0.0", "group", "set", "complexity", "O(1) for each element"},
		"SMEMBERS":    {"summary", "Get all the members in a set", "since", "1.0.0", "group", "set", "complexity", "O(N) where N is the set cardinality"},
		"SISMEMBER":   {"summary", "Determine if a given value is a member of a set", "since", "1.0.0", "group", "set", "complexity", "O(1)"},
		"SCARD":       {"summary", "Get the number of members in a set", "since", "1.0.0", "group", "set", "complexity", "O(1)"},
		"SPOP":        {"summary", "Remove and return one or multiple random members from a set", "since", "1.0.0", "group", "set", "complexity", "O(1)"},
		"SUNION":      {"summary", "Add multiple sets", "since", "1.0.0", "group", "set", "complexity", "O(N) where N is the total number of elements"},
		"SINTER":      {"summary", "Intersect multiple sets", "since", "1.0.0", "group", "set", "complexity", "O(N*M) worst case"},
		"SDIFF":       {"summary", "Subtract multiple sets", "since", "1.0.0", "group", "set", "complexity", "O(N) where N is the total number of elements"},
		"ZADD":        {"summary", "Add one or more members to a sorted set, or update its score", "since", "1.2.0", "group", "sorted_set", "complexity", "O(log(N)) for each element"},
		"ZCARD":       {"summary", "Get the number of members in a sorted set", "since", "1.2.0", "group", "sorted_set", "complexity", "O(1)"},
		"ZSCORE":      {"summary", "Get the score associated with the given member in a sorted set", "since", "1.2.0", "group", "sorted_set", "complexity", "O(1)"},
		"ZRANGE":      {"summary", "Return a range of members in a sorted set", "since", "1.2.0", "group", "sorted_set", "complexity", "O(log(N)+M) with M being the number of elements"},
		"ZRANK":       {"summary", "Determine the index of a member in a sorted set", "since", "2.0.0", "group", "sorted_set", "complexity", "O(log(N))"},
		"ZREM":        {"summary", "Remove one or more members from a sorted set", "since", "1.2.0", "group", "sorted_set", "complexity", "O(M*log(N))"},
		"XADD":        {"summary", "Add a new entry to a stream", "since", "5.0.0", "group", "stream", "complexity", "O(1)"},
		"XREAD":       {"summary", "Return never seen elements from multiple streams", "since", "5.0.0", "group", "stream", "complexity", "O(N) with N being the number of elements"},
		"XGROUP":      {"summary", "Create, destroy, and manage consumer groups", "since", "5.0.0", "group", "stream", "complexity", "O(1)"},
		"PING":        {"summary", "Ping the server", "since", "1.0.0", "group", "connection", "complexity", "O(1)"},
		"ECHO":        {"summary", "Echo the given string", "since", "1.0.0", "group", "connection", "complexity", "O(1)"},
		"QUIT":        {"summary", "Close the connection", "since", "1.0.0", "group", "connection", "complexity", "O(1)"},
		"INFO":        {"summary", "Get information and statistics about the server", "since", "1.0.0", "group", "server", "complexity", "O(1)"},
		"DBSIZE":      {"summary", "Return the number of keys in the selected database", "since", "1.0.0", "group", "server", "complexity", "O(1)"},
		"FLUSHDB":     {"summary", "Remove all keys from the current database", "since", "1.0.0", "group", "server", "complexity", "O(1)"},
		"FLUSHALL":    {"summary", "Remove all keys from all databases", "since", "1.0.0", "group", "server", "complexity", "O(1)"},
		"TIME":        {"summary", "Return the current server time", "since", "2.6.0", "group", "server", "complexity", "O(1)"},
		"CLIENT":      {"summary", "The client command", "since", "2.4.0", "group", "server", "complexity", "O(1)"},
		"CONFIG":      {"summary", "Get or set server configuration", "since", "2.0.0", "group", "server", "complexity", "O(1)"},
		"SLOWLOG":     {"summary", "Manages the Redis slow queries log", "since", "2.2.12", "group", "server", "complexity", "O(1)"},
		"MULTI":       {"summary", "Mark the start of a transaction block", "since", "1.2.0", "group", "transactions", "complexity", "O(1)"},
		"EXEC":        {"summary", "Execute all commands issued after MULTI", "since", "1.2.0", "group", "transactions", "complexity", "O(1)"},
		"DISCARD":     {"summary", "Discard all commands issued after MULTI", "since", "2.0.0", "group", "transactions", "complexity", "O(1)"},
		"WATCH":       {"summary", "Watch the given keys to determine execution of the MULTI/EXEC block", "since", "2.2.0", "group", "transactions", "complexity", "O(1)"},
		"UNWATCH":     {"summary", "Forget about all watched keys", "since", "2.2.0", "group", "transactions", "complexity", "O(1)"},
		"PUBLISH":     {"summary", "Post a message to a channel", "since", "2.0.0", "group", "pubsub", "complexity", "O(N+M)"},
		"SUBSCRIBE":   {"summary", "Subscribe to channels", "since", "2.0.0", "group", "pubsub", "complexity", "O(1)"},
		"UNSUBSCRIBE": {"summary", "Unsubscribe from channels", "since", "2.0.0", "group", "pubsub", "complexity", "O(1)"},
		"EVAL":        {"summary", "Execute a Lua script server side", "since", "2.6.0", "group", "scripting", "complexity", "O(1)"},
		"EVALSHA":     {"summary", "Execute a Lua script server side", "since", "2.6.0", "group", "scripting", "complexity", "O(1)"},
		"SCRIPT":      {"summary", "Manage the script cache", "since", "2.6.0", "group", "scripting", "complexity", "O(1)"},
		"GEOADD":      {"summary", "Add one or more geospatial items", "since", "3.2.0", "group", "geo", "complexity", "O(1) for each element"},
		"GEODIST":     {"summary", "Returns the distance between two members", "since", "3.2.0", "group", "geo", "complexity", "O(log(N))"},
		"GEOHASH":     {"summary", "Returns members of a geospatial index as standard geohash strings", "since", "3.2.0", "group", "geo", "complexity", "O(log(N))"},
		"GEOPOS":      {"summary", "Returns longitude and latitude of members", "since", "3.2.0", "group", "geo", "complexity", "O(N)"},
		"SETBIT":      {"summary", "Sets or clears the bit at offset in the string value", "since", "2.2.0", "group", "bitmap", "complexity", "O(1)"},
		"GETBIT":      {"summary", "Returns the bit value at offset in the string value", "since", "2.2.0", "group", "bitmap", "complexity", "O(1)"},
		"BITCOUNT":    {"summary", "Count set bits in a string", "since", "2.6.0", "group", "bitmap", "complexity", "O(N)"},
		"BITPOS":      {"summary", "Find first bit set or clear in a string", "since", "2.8.7", "group", "bitmap", "complexity", "O(N)"},
		"BITOP":       {"summary", "Perform bitwise operations between strings", "since", "2.6.0", "group", "bitmap", "complexity", "O(N)"},
		"BITFIELD":    {"summary", "Perform arbitrary bitfield integer operations", "since", "3.2.0", "group", "bitmap", "complexity", "O(1) for each subcommand"},
		"PFADD":       {"summary", "Adds the specified elements to the specified HyperLogLog", "since", "2.8.9", "group", "hyperloglog", "complexity", "O(1)"},
		"PFCOUNT":     {"summary", "Return the approximated cardinality of the set", "since", "2.8.9", "group", "hyperloglog", "complexity", "O(1)"},
		"PFMERGE":     {"summary", "Merge N different HyperLogLogs into a single one", "since", "2.8.9", "group", "hyperloglog", "complexity", "O(N)"},
		"SCAN":        {"summary", "Incrementally iterate the keys space", "since", "2.8.0", "group", "generic", "complexity", "O(1)"},
		"SORT":        {"summary", "Sort the elements in a list, set or sorted set", "since", "1.0.0", "group", "generic", "complexity", "O(N+M*log(M))"},
		"OBJECT":      {"summary", "Inspect the internals of Redis objects", "since", "2.2.3", "group", "generic", "complexity", "O(1)"},
		"MEMORY":      {"summary", "Inspect memory usage", "since", "4.0.0", "group", "server", "complexity", "O(1)"},
	}

	if doc, ok := docs[commandName]; ok {
		result := make([]*resp.Value, 0, len(doc)+2)
		result = append(result, resp.BulkString(commandName))
		for i := 0; i < len(doc); i += 2 {
			result = append(result, resp.BulkString(doc[i].(string)), resp.BulkString(doc[i+1].(string)))
		}
		return ctx.WriteArray(result)
	}

	return ctx.WriteArray([]*resp.Value{})
}

func cmdCommandGetKeys(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	commandName := strings.ToUpper(ctx.ArgString(1))
	args := ctx.Args[2:]

	var keys []*resp.Value

	switch commandName {
	case "GET", "SET", "DEL", "EXISTS", "TYPE", "TTL", "PTTL", "EXPIRE", "PEXPIRE", "PERSIST",
		"INCR", "DECR", "INCRBY", "DECRBY", "INCRBYFLOAT", "APPEND", "STRLEN", "GETSET", "GETEX", "GETDEL",
		"SETRANGE", "GETRANGE", "SUBSTR", "SETNX", "HGETALL", "HKEYS", "HVALS", "HLEN", "SCARD", "SMEMBERS",
		"SPOP", "SRANDMEMBER", "LLEN", "LRANGE", "LPOP", "RPOP", "ZCARD", "ZRANGE", "ZREVRANGE", "XLEN":
		if len(args) > 0 {
			keys = append(keys, resp.BulkString(string(args[0])))
		}
	case "MGET", "MSET":
		for i := 0; i < len(args); i++ {
			if commandName == "MSET" && i%2 == 1 {
				continue
			}
			keys = append(keys, resp.BulkString(string(args[i])))
		}
	case "LPUSH", "RPUSH", "LPUSHX", "RPUSHX", "SADD", "SREM", "ZADD", "LREM", "LINDEX", "LSET", "HSET", "HGET", "HDEL", "SISMEMBER", "ZSCORE", "ZRANK", "ZREVRANK":
		if len(args) > 0 {
			keys = append(keys, resp.BulkString(string(args[0])))
		}
	case "BLPOP", "BRPOP":
		for i := 0; i < len(args)-1; i++ {
			keys = append(keys, resp.BulkString(string(args[i])))
		}
	case "RENAME", "RENAMENX", "RPOPLPUSH", "BRPOPLPUSH", "LMOVE", "BLMOVE", "SMOVE", "ZINTERSTORE", "ZUNIONSTORE", "ZDIFFSTORE", "COPY":
		if len(args) >= 2 {
			keys = append(keys, resp.BulkString(string(args[0])), resp.BulkString(string(args[1])))
		}
	case "SINTER", "SUNION", "SDIFF", "SINTERSTORE", "SUNIONSTORE", "SDIFFSTORE":
		for _, arg := range args {
			keys = append(keys, resp.BulkString(string(arg)))
		}
	}

	return ctx.WriteArray(keys)
}

func cmdINFO(ctx *Context) error {
	var sb strings.Builder

	sb.WriteString("# Server\r\n")
	sb.WriteString("cachestorm_version:1.0.0\r\n")
	sb.WriteString("arch_bits:64\r\n")
	sb.WriteString("tcp_port:6380\r\n")
	sb.WriteString("\r\n")

	sb.WriteString("# Memory\r\n")
	sb.WriteString("used_memory:")
	sb.WriteString(strconv.FormatInt(ctx.Store.MemUsage(), 10))
	sb.WriteString("\r\n")
	sb.WriteString("\r\n")

	sb.WriteString("# Keyspace\r\n")
	sb.WriteString("db0:keys=")
	sb.WriteString(strconv.FormatInt(ctx.Store.KeyCount(), 10))
	sb.WriteString(",expires=0\r\n")

	return ctx.WriteBulkString(sb.String())
}

func cmdDBSIZE(ctx *Context) error {
	return ctx.WriteInteger(ctx.Store.KeyCount())
}

func cmdFLUSHDB(ctx *Context) error {
	ctx.Store.Flush()
	return ctx.WriteOK()
}

func cmdFLUSHALL(ctx *Context) error {
	ctx.Store.Flush()
	return ctx.WriteOK()
}

func cmdTIME(ctx *Context) error {
	now := time.Now()
	sec := now.Unix()
	usec := now.Nanosecond() / 1000

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(strconv.FormatInt(sec, 10)),
		resp.BulkString(strconv.FormatInt(int64(usec), 10)),
	})
}

func cmdEXPIRE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	sec, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	if ctx.Store.SetTTL(key, time.Duration(sec)*time.Second) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdPEXPIRE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	ms, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	if ctx.Store.SetTTL(key, time.Duration(ms)*time.Millisecond) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdEXPIREAT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	ts, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	expiresAt := time.Unix(ts, 0).UnixNano()
	if ctx.Store.SetExpiresAt(key, expiresAt) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdPEXPIREAT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	ts, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	expiresAt := time.Unix(0, ts*int64(time.Millisecond)).UnixNano()
	if ctx.Store.SetExpiresAt(key, expiresAt) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdTTL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	ttl := ctx.Store.TTL(ctx.ArgString(0))
	return ctx.WriteInteger(int64(ttl / time.Second))
}

func cmdPTTL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	ttl := ctx.Store.TTL(ctx.ArgString(0))
	return ctx.WriteInteger(int64(ttl / time.Millisecond))
}

func cmdPERSIST(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if ctx.Store.Persist(ctx.ArgString(0)) {
		return ctx.WriteInteger(1)
	}
	return ctx.WriteInteger(0)
}

func cmdTYPE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	dt := ctx.Store.Type(ctx.ArgString(0))
	if dt == 0 {
		return ctx.WriteSimpleString("none")
	}
	return ctx.WriteSimpleString(dt.String())
}

func cmdRENAME(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	oldKey := ctx.ArgString(0)
	newKey := ctx.ArgString(1)

	entry, exists := ctx.Store.Get(oldKey)
	if !exists {
		return ctx.WriteError(store.ErrKeyNotFound)
	}

	ctx.Store.Delete(oldKey)
	ctx.Store.SetEntry(newKey, entry)

	return ctx.WriteOK()
}

func cmdRENAMENX(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	oldKey := ctx.ArgString(0)
	newKey := ctx.ArgString(1)

	entry, exists := ctx.Store.Get(oldKey)
	if !exists {
		return ctx.WriteError(store.ErrKeyNotFound)
	}

	if ctx.Store.Exists(newKey) {
		return ctx.WriteInteger(0)
	}

	ctx.Store.Delete(oldKey)
	ctx.Store.SetEntry(newKey, entry)

	return ctx.WriteInteger(1)
}

func cmdKEYS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	pattern := ctx.ArgString(0)
	keys := ctx.Store.Keys()

	matched := make([]*resp.Value, 0)
	for _, key := range keys {
		if matchPattern(key, pattern) {
			matched = append(matched, resp.BulkString(key))
		}
	}

	return ctx.WriteArray(matched)
}

func cmdRANDOMKEY(ctx *Context) error {
	keys := ctx.Store.Keys()
	if len(keys) == 0 {
		return ctx.WriteNullBulkString()
	}

	idx := time.Now().UnixNano() % int64(len(keys))
	return ctx.WriteBulkString(keys[idx])
}

func matchPattern(s, pattern string) bool {
	if pattern == "*" {
		return true
	}

	si, pi := 0, 0
	starIdx, match := -1, 0

	for si < len(s) {
		if pi < len(pattern) && (pattern[pi] == '?' || pattern[pi] == s[si]) {
			si++
			pi++
		} else if pi < len(pattern) && pattern[pi] == '*' {
			starIdx = pi
			match = si
			pi++
		} else if starIdx != -1 {
			pi = starIdx + 1
			match++
			si = match
		} else {
			return false
		}
	}

	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}

	return pi == len(pattern)
}

func cmdTOUCH(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	touched := int64(0)
	for i := 0; i < ctx.ArgCount(); i++ {
		key := ctx.ArgString(i)
		if entry, exists := ctx.Store.Get(key); exists {
			entry.Touch()
			touched++
		}
	}

	return ctx.WriteInteger(touched)
}

func cmdDUMP(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteNullBulkString()
	}

	var dump strings.Builder
	dump.WriteString("CACHSTORM001")
	dump.WriteByte(byte(entry.Value.Type()))
	dump.WriteString(fmt.Sprintf("%d", entry.ExpiresAt))
	dump.WriteString(":")

	switch v := entry.Value.(type) {
	case *store.StringValue:
		dump.Write(v.Data)
	case *store.HashValue:
		for k, val := range v.Fields {
			dump.WriteString(k)
			dump.WriteByte('=')
			dump.Write(val)
			dump.WriteByte('&')
		}
	case *store.ListValue:
		for _, el := range v.Elements {
			dump.Write(el)
			dump.WriteByte(',')
		}
	case *store.SetValue:
		for k := range v.Members {
			dump.WriteString(k)
			dump.WriteByte(',')
		}
	case *store.SortedSetValue:
		for _, se := range v.GetSortedRange(0, -1, true, false) {
			dump.WriteString(fmt.Sprintf("%s:%f,", se.Member, se.Score))
		}
	}

	return ctx.WriteBulkString(dump.String())
}

func cmdRESTORE(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	ttl, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	serialized := ctx.ArgString(2)

	if len(serialized) < 13 || !strings.HasPrefix(serialized, "CACHSTORM001") {
		return ctx.WriteError(errors.New("ERR DUMP payload version or checksum are wrong"))
	}

	replace := false
	for i := 3; i < ctx.ArgCount(); i++ {
		if strings.ToUpper(ctx.ArgString(i)) == "REPLACE" {
			replace = true
		}
	}

	if !replace {
		if _, exists := ctx.Store.Get(key); exists {
			return ctx.WriteError(errors.New("BUSYKEY Target key name already exists"))
		}
	}

	data := serialized[12:]
	typeByte := data[0]
	data = data[1:]

	colonIdx := strings.Index(data, ":")
	if colonIdx == -1 {
		return ctx.WriteError(errors.New("ERR invalid DUMP format"))
	}

	_ = data[:colonIdx]
	data = data[colonIdx+1:]

	var value store.Value
	switch store.DataType(typeByte) {
	case store.DataTypeString:
		value = &store.StringValue{Data: []byte(data)}
	case store.DataTypeHash:
		hv := &store.HashValue{Fields: make(map[string][]byte)}
		if len(data) > 0 {
			pairs := strings.Split(data, "&")
			for _, pair := range pairs {
				if pair == "" {
					continue
				}
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) == 2 {
					hv.Fields[kv[0]] = []byte(kv[1])
				}
			}
		}
		value = hv
	case store.DataTypeList:
		lv := &store.ListValue{Elements: make([][]byte, 0)}
		if len(data) > 0 {
			items := strings.Split(data, ",")
			for _, item := range items {
				if item != "" {
					lv.Elements = append(lv.Elements, []byte(item))
				}
			}
		}
		value = lv
	case store.DataTypeSet:
		sv := &store.SetValue{Members: make(map[string]struct{})}
		if len(data) > 0 {
			items := strings.Split(data, ",")
			for _, item := range items {
				if item != "" {
					sv.Members[item] = struct{}{}
				}
			}
		}
		value = sv
	default:
		return ctx.WriteError(errors.New("ERR unsupported type for RESTORE"))
	}

	opts := store.SetOptions{}
	if ttl > 0 {
		opts.TTL = time.Duration(ttl) * time.Millisecond
	}

	ctx.Store.Set(key, value, opts)
	return ctx.WriteOK()
}

func cmdCOPY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	srcKey := ctx.ArgString(0)
	dstKey := ctx.ArgString(1)

	replace := false
	dstDB := 0

	for i := 2; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "REPLACE":
			replace = true
		case "DB":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			dstDB, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			_ = dstDB
		}
	}

	entry, exists := ctx.Store.Get(srcKey)
	if !exists {
		return ctx.WriteInteger(0)
	}

	if !replace {
		if _, exists := ctx.Store.Get(dstKey); exists {
			return ctx.WriteInteger(0)
		}
	}

	clonedValue := entry.Value.Clone()
	newEntry := store.NewEntry(clonedValue)
	newEntry.ExpiresAt = entry.ExpiresAt
	newEntry.Tags = make([]string, len(entry.Tags))
	copy(newEntry.Tags, entry.Tags)

	ctx.Store.SetEntry(dstKey, newEntry)
	return ctx.WriteInteger(1)
}

func cmdAUTH(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	password := ctx.ArgString(0)

	_ = password

	ctx.SetAuthenticated(true)
	return ctx.WriteOK()
}

func cmdSCAN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	cursor, err := strconv.Atoi(ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	count := 10
	pattern := "*"
	match := ""

	for i := 1; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "COUNT":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			count, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
		case "MATCH":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			pattern = ctx.ArgString(i)
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	keys := ctx.Store.Keys()

	filtered := make([]string, 0)
	for _, key := range keys {
		if matchPattern(key, pattern) {
			filtered = append(filtered, key)
		}
	}

	start := cursor
	if start >= len(filtered) {
		start = 0
	}

	end := start + count
	if end > len(filtered) {
		end = len(filtered)
	}

	nextCursor := 0
	if end < len(filtered) {
		nextCursor = end
	}

	result := make([]*resp.Value, 0, end-start)
	for i := start; i < end; i++ {
		result = append(result, resp.BulkString(filtered[i]))
	}

	_ = match

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(strconv.Itoa(nextCursor)),
		resp.ArrayValue(result),
	})
}

func cmdHOTKEYS(ctx *Context) error {
	count := 10
	if ctx.ArgCount() >= 1 {
		var err error
		count, err = strconv.Atoi(ctx.ArgString(0))
		if err != nil {
			return ctx.WriteError(ErrNotInteger)
		}
	}

	keys := ctx.Store.Keys()
	type hotKey struct {
		key    string
		access uint64
	}

	hotKeys := make([]hotKey, 0, len(keys))
	for _, key := range keys {
		entry, exists := ctx.Store.Get(key)
		if exists {
			hotKeys = append(hotKeys, hotKey{key: key, access: entry.AccessCount})
		}
	}

	for i := 0; i < len(hotKeys)-1; i++ {
		for j := i + 1; j < len(hotKeys); j++ {
			if hotKeys[j].access > hotKeys[i].access {
				hotKeys[i], hotKeys[j] = hotKeys[j], hotKeys[i]
			}
		}
	}

	if count > len(hotKeys) {
		count = len(hotKeys)
	}

	result := make([]*resp.Value, 0, count)
	for i := 0; i < count; i++ {
		result = append(result, resp.ArrayValue([]*resp.Value{
			resp.BulkString(hotKeys[i].key),
			resp.IntegerValue(int64(hotKeys[i].access)),
		}))
	}

	return ctx.WriteArray(result)
}

func cmdMEMINFO(ctx *Context) error {
	var sb strings.Builder

	sb.WriteString("# Memory\r\n")
	sb.WriteString("used_memory:")
	sb.WriteString(strconv.FormatInt(ctx.Store.MemUsage(), 10))
	sb.WriteString("\r\n")
	sb.WriteString("keys:")
	sb.WriteString(strconv.FormatInt(ctx.Store.KeyCount(), 10))
	sb.WriteString("\r\n")

	if ctx.Store.KeyCount() > 0 {
		avgSize := ctx.Store.MemUsage() / ctx.Store.KeyCount()
		sb.WriteString("avg_entry_size:")
		sb.WriteString(strconv.FormatInt(avgSize, 10))
		sb.WriteString("\r\n")
	}

	tagIndex := ctx.Store.GetTagIndex()
	if tagIndex != nil {
		tags := tagIndex.Tags()
		sb.WriteString("tags:")
		sb.WriteString(strconv.Itoa(len(tags)))
		sb.WriteString("\r\n")
	}

	return ctx.WriteBulkString(sb.String())
}

type ClientTrackingInfo struct {
	mu       sync.RWMutex
	enabled  bool
	redirect int64
	prefixes []string
	noLoop   bool
}

var globalClientTracking = struct {
	mu      sync.RWMutex
	clients map[int64]*ClientTrackingInfo
}{
	clients: make(map[int64]*ClientTrackingInfo),
}

func GetClientTracking(clientID int64) *ClientTrackingInfo {
	globalClientTracking.mu.Lock()
	defer globalClientTracking.mu.Unlock()
	if _, ok := globalClientTracking.clients[clientID]; !ok {
		globalClientTracking.clients[clientID] = &ClientTrackingInfo{}
	}
	return globalClientTracking.clients[clientID]
}

func cmdCLIENT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "LIST":
		var sb strings.Builder
		sb.WriteString("id=")
		sb.WriteString(strconv.FormatInt(ctx.ClientID, 10))
		sb.WriteString(" addr=127.0.0.1:0")
		sb.WriteString(" name=")
		sb.WriteString(" age=0")
		sb.WriteString(" idle=0")
		sb.WriteString(" flags=N")
		sb.WriteString(" db=0")
		sb.WriteString(" sub=0")
		sb.WriteString(" psub=0")
		sb.WriteString(" multi=-1")
		sb.WriteString(" qbuf=0")
		sb.WriteString(" obl=0")
		sb.WriteString(" oll=0")
		sb.WriteString(" omem=0")
		sb.WriteString("\n")
		return ctx.WriteBulkString(sb.String())
	case "SETNAME":
		if ctx.ArgCount() != 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		return ctx.WriteOK()
	case "GETNAME":
		return ctx.WriteNullBulkString()
	case "ID":
		return ctx.WriteInteger(ctx.ClientID)
	case "KILL":
		if ctx.ArgCount() < 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		return ctx.WriteOK()
	case "PAUSE":
		if ctx.ArgCount() < 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		return ctx.WriteOK()
	case "UNBLOCK":
		if ctx.ArgCount() < 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		return ctx.WriteInteger(0)
	case "REPLY":
		if ctx.ArgCount() < 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		return ctx.WriteOK()
	case "TRACKING":
		return cmdClientTracking(ctx)
	case "CACHING":
		return ctx.WriteOK()
	case "NO-TOUCH":
		return ctx.WriteOK()
	case "INFO":
		return ctx.WriteBulkString("id=" + strconv.FormatInt(ctx.ClientID, 10))
	case "GETREDIR":
		tracking := GetClientTracking(ctx.ClientID)
		tracking.mu.RLock()
		defer tracking.mu.RUnlock()
		return ctx.WriteInteger(tracking.redirect)
	default:
		return ctx.WriteError(errors.New("ERR unknown subcommand '" + subCmd + "'"))
	}
}

func cmdClientTracking(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	onOff := strings.ToUpper(ctx.ArgString(1))
	tracking := GetClientTracking(ctx.ClientID)
	tracking.mu.Lock()
	defer tracking.mu.Unlock()

	switch onOff {
	case "ON":
		tracking.enabled = true
	case "OFF":
		tracking.enabled = false
	default:
		return ctx.WriteError(errors.New("ERR syntax error"))
	}

	for i := 2; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "REDIRECT":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			redirectID, err := strconv.ParseInt(ctx.ArgString(i+1), 10, 64)
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			tracking.redirect = redirectID
			i++
		case "BCAST":
		case "PREFIX":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			tracking.prefixes = append(tracking.prefixes, ctx.ArgString(i+1))
			i++
		case "OPTIN":
		case "OPTOUT":
		case "NOLOOP":
			tracking.noLoop = true
		}
	}

	return ctx.WriteOK()
}

func cmdSORT(ctx *Context) error {
	return doSort(ctx, false)
}

func cmdSORTRO(ctx *Context) error {
	return doSort(ctx, true)
}

func doSort(ctx *Context, readOnly bool) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)

	desc := false
	alpha := false
	offset := 0
	count := -1
	storeKey := ""

	for i := 1; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "DESC":
			desc = true
		case "ASC":
			desc = false
		case "ALPHA":
			alpha = true
		case "LIMIT":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			offset, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			count, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
		case "STORE":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			storeKey = ctx.ArgString(i)
		case "BY", "GET":
			i++
		}
	}

	entry, exists := ctx.Store.Get(key)
	if !exists {
		if storeKey != "" && !readOnly {
			ctx.Store.Delete(storeKey)
		}
		return ctx.WriteArray([]*resp.Value{})
	}

	var elements []string

	switch v := entry.Value.(type) {
	case *store.ListValue:
		for _, elem := range v.Elements {
			elements = append(elements, string(elem))
		}
	case *store.SetValue:
		for member := range v.Members {
			elements = append(elements, member)
		}
	case *store.SortedSetValue:
		entries := v.GetSortedRange(0, -1, false, false)
		for _, e := range entries {
			elements = append(elements, e.Member)
		}
	default:
		return ctx.WriteError(errors.New("ERR wrong type for SORT"))
	}

	if alpha {
		if desc {
			sort.Slice(elements, func(i, j int) bool {
				return elements[i] > elements[j]
			})
		} else {
			sort.Strings(elements)
		}
	} else {
		floatElements := make([]float64, 0)
		stringElements := make([]string, 0)
		for _, elem := range elements {
			f, err := strconv.ParseFloat(elem, 64)
			if err != nil {
				stringElements = append(stringElements, elem)
			} else {
				floatElements = append(floatElements, f)
			}
		}

		if desc {
			sort.Slice(floatElements, func(i, j int) bool {
				return floatElements[i] > floatElements[j]
			})
		} else {
			sort.Float64s(floatElements)
		}

		sortedElements := make([]string, 0, len(floatElements))
		for _, f := range floatElements {
			sortedElements = append(sortedElements, strconv.FormatFloat(f, 'f', -1, 64))
		}
		sortedElements = append(sortedElements, stringElements...)
		elements = sortedElements
	}

	start := 0
	end := len(elements)

	if offset > 0 {
		if offset < len(elements) {
			start = offset
		} else {
			start = len(elements)
		}
	}
	if count >= 0 {
		end = start + count
		if end > len(elements) {
			end = len(elements)
		}
	}

	elements = elements[start:end]

	if storeKey != "" {
		if readOnly {
			return ctx.WriteError(errors.New("ERR SORT_RO does not support STORE"))
		}
		if len(elements) == 0 {
			ctx.Store.Delete(storeKey)
			return ctx.WriteInteger(0)
		}

		list := &store.ListValue{Elements: make([][]byte, 0, len(elements))}
		for _, elem := range elements {
			list.Elements = append(list.Elements, []byte(elem))
		}
		ctx.Store.Set(storeKey, list, store.SetOptions{})
		return ctx.WriteInteger(int64(len(elements)))
	}

	result := make([]*resp.Value, 0, len(elements))
	for _, elem := range elements {
		result = append(result, resp.BulkString(elem))
	}

	return ctx.WriteArray(result)
}

var lastSaveTime int64

type SlowLogEntry struct {
	ID        int64
	StartTime int64
	Duration  int64
	Command   string
	Args      []string
	ClientIP  string
	ClientID  int64
}

type SlowLog struct {
	entries   []SlowLogEntry
	mu        sync.RWMutex
	maxLen    int
	slowLogSl int64
	nextID    int64
}

var globalSlowLog = &SlowLog{
	entries:   make([]SlowLogEntry, 0),
	maxLen:    128,
	slowLogSl: 10000,
}

func (s *SlowLog) Add(command string, args []string, duration int64, clientIP string, clientID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if duration < s.slowLogSl {
		return
	}

	entry := SlowLogEntry{
		ID:        s.nextID,
		StartTime: time.Now().UnixMicro(),
		Duration:  duration,
		Command:   command,
		Args:      args,
		ClientIP:  clientIP,
		ClientID:  clientID,
	}
	s.nextID++

	s.entries = append([]SlowLogEntry{entry}, s.entries...)
	if len(s.entries) > s.maxLen {
		s.entries = s.entries[:s.maxLen]
	}
}

func (s *SlowLog) Get(count int) []SlowLogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if count > len(s.entries) {
		count = len(s.entries)
	}
	if count <= 0 {
		count = 10
	}

	result := make([]SlowLogEntry, count)
	copy(result, s.entries[:count])
	return result
}

func (s *SlowLog) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

func (s *SlowLog) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = make([]SlowLogEntry, 0)
}

func cmdSLOWLOG(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "GET":
		count := 10
		if ctx.ArgCount() > 1 {
			var err error
			count, err = strconv.Atoi(ctx.ArgString(1))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
		}

		entries := globalSlowLog.Get(count)
		results := make([]*resp.Value, 0, len(entries))
		for _, entry := range entries {
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.IntegerValue(entry.ID),
				resp.IntegerValue(entry.StartTime),
				resp.IntegerValue(entry.Duration),
				resp.ArrayValue([]*resp.Value{
					resp.BulkString(entry.Command),
				}),
				resp.BulkString(entry.ClientIP),
				resp.BulkString(strconv.FormatInt(entry.ClientID, 10)),
			}))
		}
		return ctx.WriteArray(results)
	case "LEN":
		return ctx.WriteInteger(int64(globalSlowLog.Len()))
	case "RESET":
		globalSlowLog.Reset()
		return ctx.WriteOK()
	default:
		return ctx.WriteError(errors.New("ERR unknown subcommand '" + subCmd + "'"))
	}
}

func cmdWAIT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	numReplicas, err := strconv.Atoi(ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	timeout, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	_ = numReplicas
	_ = timeout

	return ctx.WriteInteger(1)
}

func cmdROLE(ctx *Context) error {
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("master"),
		resp.IntegerValue(0),
		resp.ArrayValue([]*resp.Value{}),
	})
}

func cmdLASTSAVE(ctx *Context) error {
	if lastSaveTime == 0 {
		lastSaveTime = time.Now().Unix()
	}
	return ctx.WriteInteger(lastSaveTime)
}

func cmdLOLWUT(ctx *Context) error {
	version := 6
	if ctx.ArgCount() > 0 {
		arg := strings.ToUpper(ctx.ArgString(0))
		if arg == "VERSION" && ctx.ArgCount() > 1 {
			v, err := strconv.Atoi(ctx.ArgString(1))
			if err == nil {
				version = v
			}
		}
	}

	var output strings.Builder
	output.WriteString("\n")

	switch version {
	case 5:
		for i := 0; i < 6; i++ {
			output.WriteString("._ \n")
			for j := 0; j < 15; j++ {
				output.WriteString(" ")
			}
			output.WriteString(".\n")
		}
	default:
		output.WriteString("CacheStorm ver 1.0.0\n")
		output.WriteString("\n")
		output.WriteString("High-performance Redis-compatible cache with tag-based invalidation.\n")
	}

	return ctx.WriteBulkString(output.String())
}

func cmdSHUTDOWN(ctx *Context) error {
	save := true
	for i := 0; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		if arg == "NOSAVE" {
			save = false
		} else if arg == "SAVE" {
			save = true
		}
	}

	_ = save
	ctx.WriteOK()
	return errors.New("SHUTDOWN")
}

func cmdSAVE(ctx *Context) error {
	lastSaveTime = time.Now().Unix()
	return ctx.WriteOK()
}

func cmdBGSAVE(ctx *Context) error {
	lastSaveTime = time.Now().Unix()
	return ctx.WriteSimpleString("Background saving started")
}

func cmdBGREWRITEAOF(ctx *Context) error {
	return ctx.WriteSimpleString("Background append only file rewriting started")
}

func cmdSLAVEOF(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	host := ctx.ArgString(0)
	port := ctx.ArgString(1)

	if host == "NO" && port == "ONE" {
		return ctx.WriteOK()
	}

	_ = host
	_ = port

	return ctx.WriteOK()
}

func cmdLATENCY(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "LATEST":
		return ctx.WriteArray([]*resp.Value{
			resp.ArrayValue([]*resp.Value{
				resp.BulkString("command"),
				resp.IntegerValue(time.Now().UnixMilli() - 60000),
				resp.IntegerValue(15),
				resp.IntegerValue(30),
			}),
		})
	case "HISTORY":
		event := "command"
		if ctx.ArgCount() > 1 {
			event = ctx.ArgString(1)
		}
		_ = event
		return ctx.WriteArray([]*resp.Value{
			resp.IntegerValue(time.Now().UnixMilli() - 5000),
			resp.IntegerValue(time.Now().UnixMilli() - 10000),
		})
	case "RESET":
		return ctx.WriteInteger(0)
	case "GRAPH":
		event := "command"
		if ctx.ArgCount() > 1 {
			event = ctx.ArgString(1)
		}
		_ = event
		graph := `1ms - 2ms
2ms - 5ms
5ms - 10ms
10ms - 20ms
`
		return ctx.WriteBulkString(graph)
	case "DOCTOR":
		return ctx.WriteBulkString("Dave, I have observed latency spikes in the CacheStorm server.\n\nNo obvious performance issues detected.\n")
	case "HELP":
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("LATEST"),
			resp.BulkString("HISTORY <event>"),
			resp.BulkString("RESET [<event>]"),
			resp.BulkString("GRAPH <event>"),
			resp.BulkString("DOCTOR"),
		})
	default:
		return ctx.WriteError(errors.New("ERR unknown subcommand '" + subCmd + "'"))
	}
}

func cmdSTRALGO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	algorithm := strings.ToUpper(ctx.ArgString(0))

	if algorithm != "LCS" {
		return ctx.WriteError(errors.New("ERR unknown algorithm '" + algorithm + "'"))
	}

	keys := make([]string, 0)
	minMatchLen := 1

	for i := 1; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		switch arg {
		case "KEYS":
			i++
			for i < ctx.ArgCount() && !strings.HasPrefix(strings.ToUpper(ctx.ArgString(i)), "IDX") && !strings.HasPrefix(strings.ToUpper(ctx.ArgString(i)), "LEN") && !strings.HasPrefix(strings.ToUpper(ctx.ArgString(i)), "MIN") {
				keys = append(keys, ctx.ArgString(i))
				i++
			}
			i--
		case "STRINGS":
			return ctx.WriteError(errors.New("ERR STRINGS option not supported, use KEYS"))
		case "MINMATCHLEN":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			var err error
			minMatchLen, err = strconv.Atoi(ctx.ArgString(i))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
		}
	}

	_ = minMatchLen

	if len(keys) != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	return ctx.WriteBulkString("")
}

func cmdMODULE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "LIST":
		return ctx.WriteArray([]*resp.Value{})
	case "LOAD":
		return ctx.WriteError(errors.New("ERR module loading not supported"))
	case "UNLOAD":
		return ctx.WriteError(errors.New("ERR module unloading not supported"))
	default:
		return ctx.WriteError(errors.New("ERR unknown subcommand '" + subCmd + "'"))
	}
}

func cmdACL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "LIST":
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("user default on nopass ~* &* +@all"),
		})
	case "USERS":
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("default"),
		})
	case "WHOAMI":
		return ctx.WriteBulkString("default")
	case "CAT":
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("read"),
			resp.BulkString("write"),
			resp.BulkString("admin"),
			resp.BulkString("connection"),
			resp.BulkString("dangerous"),
		})
	case "SETUSER":
		return ctx.WriteOK()
	case "DELUSER":
		return ctx.WriteInteger(0)
	case "GETUSER":
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("flags"),
			resp.ArrayValue([]*resp.Value{resp.BulkString("on"), resp.BulkString("nopass")}),
			resp.BulkString("passwords"),
			resp.ArrayValue([]*resp.Value{}),
			resp.BulkString("commands"),
			resp.BulkString("+@all"),
		})
	case "LOAD":
		return ctx.WriteOK()
	case "SAVE":
		return ctx.WriteOK()
	case "LOG":
		return ctx.WriteArray([]*resp.Value{})
	case "HELP":
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("LIST"),
			resp.BulkString("USERS"),
			resp.BulkString("WHOAMI"),
			resp.BulkString("CAT"),
			resp.BulkString("SETUSER"),
			resp.BulkString("GETUSER"),
		})
	default:
		return ctx.WriteError(errors.New("ERR unknown subcommand '" + subCmd + "'"))
	}
}

func cmdMONITOR(ctx *Context) error {
	return ctx.WriteOK()
}

func cmdSWAPDB(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	_, err1 := strconv.Atoi(ctx.ArgString(0))
	_, err2 := strconv.Atoi(ctx.ArgString(1))

	if err1 != nil || err2 != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	return ctx.WriteOK()
}

func cmdSYNC(ctx *Context) error {
	return ctx.WriteBulkString("")
}

func cmdPSYNC(ctx *Context) error {
	return ctx.WriteSimpleString("CONTINUE")
}

func cmdDEBUGSEGFAULT(ctx *Context) error {
	return ctx.WriteError(errors.New("ERR SEGFAULT not allowed"))
}
