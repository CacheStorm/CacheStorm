package command

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/acl"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
	buildversion "github.com/cachestorm/cachestorm/internal/version"
)

var globalACL = acl.NewACL()

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
	router.Register(&CommandDef{Name: "LASTSAVE", Handler: cmdLASTSAVE})
	router.Register(&CommandDef{Name: "LOLWUT", Handler: cmdLOLWUT})
	router.Register(&CommandDef{Name: "SHUTDOWN", Handler: cmdSHUTDOWN})
	router.Register(&CommandDef{Name: "SAVE", Handler: cmdSAVE})
	router.Register(&CommandDef{Name: "BGSAVE", Handler: cmdBGSAVE})
	router.Register(&CommandDef{Name: "BGREWRITEAOF", Handler: cmdBGREWRITEAOF})
	// SLAVEOF and REPLICAOF used to be registered here pointing at cmdSLAVEOF,
	// a stub that discards both arguments and replies +OK unconditionally.
	// Router.Register is last-write-wins, so those entries only worked because
	// RegisterReplicationCommands happens to run later in
	// internal/server/server.go and overwrote them with the functional
	// cmdREPLICAOF. Reorder those two lines and both commands would silently
	// resolve to the stub and stop configuring replication, with no error at
	// all. The functional handler owns both names; do not register them twice.
	router.Register(&CommandDef{Name: "LATENCY", Handler: cmdLATENCY})
	router.Register(&CommandDef{Name: "STRALGO", Handler: cmdSTRALGO})
	router.Register(&CommandDef{Name: "ACL", Handler: cmdACL})
	router.Register(&CommandDef{Name: "MONITOR", Handler: cmdMONITOR})
	router.Register(&CommandDef{Name: "SWAPDB", Handler: cmdSWAPDB})
	router.Register(&CommandDef{Name: "DEBUGSEGFAULT", Handler: cmdDEBUGSEGFAULT})
	RegisterHelloCommands(router)
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
	router.Register(&CommandDef{Name: "PEXPIRETIME", Handler: cmdPEXPIRETIME})
	router.Register(&CommandDef{Name: "EXPIRETIME", Handler: cmdEXPIRETIME})
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
	router.Register(&CommandDef{Name: "MOVE", Handler: cmdMOVE})
	router.Register(&CommandDef{Name: "WAITAOF", Handler: cmdWAITAOF})
}

func cmdPING(ctx *Context) error {
	if ctx.ArgCount() > 1 {
		return ctx.WriteError(arityError("PING"))
	}
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
	if ctx.ArgCount() == 0 {
		return cmdCommandInfo(ctx, nil)
	}
	subCmd := strings.ToUpper(ctx.ArgString(0))
	switch subCmd {
	case "COUNT":
		return ctx.WriteInteger(int64(len(registeredCommandNames(ctx))))
	case "DOCS":
		return cmdCommandDocs(ctx)
	case "GETKEYS":
		return cmdCommandGetKeys(ctx)
	case "INFO":
		names := make([]string, 0, ctx.ArgCount()-1)
		for i := 1; i < ctx.ArgCount(); i++ {
			names = append(names, ctx.ArgString(i))
		}
		if len(names) == 0 {
			names = nil
		}
		return cmdCommandInfo(ctx, names)
	case "LIST":
		return cmdCommandList(ctx)
	default:
		return ctx.WriteError(fmt.Errorf("ERR unknown subcommand '%s'", subCmd))
	}
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
			k, _ := doc[i].(string)
			v, _ := doc[i+1].(string)
			result = append(result, resp.BulkString(k), resp.BulkString(v))
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
	r := ctx.dispatcher()
	sections := make(map[string]bool)
	for i := 0; i < ctx.ArgCount(); i++ {
		switch name := strings.ToLower(ctx.ArgString(i)); name {
		case "all", "everything", "default":
		default:
			sections[name] = true
		}
	}
	want := func(name string) bool {
		return len(sections) == 0 || sections[name]
	}

	port := globalConfig.port
	if r != nil && r.Port() > 0 {
		port = r.Port()
	}

	var st ServerStatus
	if r != nil {
		st = r.getServerStatus()
	} else {
		st.Started = store.GlobalMetrics.StartTime
	}
	var w infoWriter
	if want("server") {
		infoServer(&w, st, port)
	}
	if want("clients") {
		infoClients(&w, st)
	}
	if want("memory") {
		infoMemory(&w, ctx)
	}
	sb := &w.sb

	if want("persistence") && r != nil {
		_, _, info := r.snapshotHooks()
		if info != nil {
			sb.WriteString(info())
		}
	}

	if want("stats") {
		infoStats(&w, st)
	}

	if want("replication") {
		var fn func() string
		if r != nil {
			fn = r.replicationInfo()
		}
		if fn != nil {
			sb.WriteString(fn())
		} else {
			sb.WriteString("# Replication\r\nrole:master\r\nconnected_slaves:0\r\n\r\n")
		}
	}

	if want("cpu") {
		infoCPU(&w)
	}

	if want("keyspace") {
		sb.WriteString(keyspaceInfo(ctx))
	}

	return ctx.WriteBulkString(sb.String())
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
	return expireGeneric(ctx, time.Now().UnixMilli(), true, "expire")
}

func cmdPEXPIRE(ctx *Context) error {
	return expireGeneric(ctx, time.Now().UnixMilli(), false, "pexpire")
}

func cmdEXPIREAT(ctx *Context) error {
	return expireGeneric(ctx, 0, true, "expireat")
}

func cmdPEXPIREAT(ctx *Context) error {
	return expireGeneric(ctx, 0, false, "pexpireat")
}

const (
	maxExpireAtSeconds = 9223372036    // MaxInt64 / time.Second
	maxExpireAtMillis  = 9223372036854 // MaxInt64 / time.Millisecond
)

var errInvalidExpireTime = errors.New("ERR invalid expire time")

const (
	expireFlagNX = 1 << iota
	expireFlagXX
	expireFlagGT
	expireFlagLT
)

// parseExpireFlags is Redis' parseExtendedExpireArgumentsOrReply.
func parseExpireFlags(args [][]byte) (int, error) {
	flags := 0
	for _, a := range args {
		switch strings.ToUpper(string(a)) {
		case "NX":
			flags |= expireFlagNX
		case "XX":
			flags |= expireFlagXX
		case "GT":
			flags |= expireFlagGT
		case "LT":
			flags |= expireFlagLT
		default:
			return 0, errors.New("ERR Unsupported option " + string(a))
		}
	}
	if flags&expireFlagNX != 0 && flags&(expireFlagXX|expireFlagGT|expireFlagLT) != 0 {
		return 0, errors.New("ERR NX and XX, GT or LT options at the same time are not compatible")
	}
	if flags&expireFlagGT != 0 && flags&expireFlagLT != 0 {
		return 0, errors.New("ERR GT and LT options at the same time are not compatible")
	}
	return flags, nil
}

// expireGeneric is Redis' expireGenericCommand: the instant is computed in
// milliseconds with overflow checks, NX/XX/GT/LT compare it against the
// current expiry (no expiry counts as infinite), and an instant that is not
// in the future deletes the key.
func expireGeneric(ctx *Context, basetimeMs int64, seconds bool, cmd string) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	flags, err := parseExpireFlags(ctx.Args[2:])
	if err != nil {
		return ctx.WriteError(err)
	}
	when, ok := parseRedisInt(ctx.Arg(1))
	if !ok {
		return ctx.WriteError(ErrNotInteger)
	}
	if seconds {
		if when > math.MaxInt64/1000 || when < math.MinInt64/1000 {
			return ctx.WriteError(errInvalidExpireIn(cmd))
		}
		when *= 1000
	}
	if when > math.MaxInt64-basetimeMs {
		return ctx.WriteError(errInvalidExpireIn(cmd))
	}
	when += basetimeMs

	entry, exists := ctx.Store.Get(key)
	if !exists {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	if flags != 0 {
		current := entry.ExpiresAt.Load()
		volatile := current != 0
		if flags&expireFlagNX != 0 && volatile {
			ctx.PropagateAs()
			return ctx.WriteInteger(0)
		}
		if flags&expireFlagXX != 0 && !volatile {
			ctx.PropagateAs()
			return ctx.WriteInteger(0)
		}
		if flags&expireFlagGT != 0 && (!volatile || when <= current) {
			ctx.PropagateAs()
			return ctx.WriteInteger(0)
		}
		if flags&expireFlagLT != 0 && volatile && when >= current {
			ctx.PropagateAs()
			return ctx.WriteInteger(0)
		}
	}

	if when <= time.Now().UnixMilli() && !ctx.Replaying {
		ctx.Store.Delete(key)
		ctx.propagateDelete(key)
		return ctx.WriteInteger(1)
	}
	if ctx.Store.SetExpiresAt(key, when) {
		ctx.propagateExpiry(key, when)
		return ctx.WriteInteger(1)
	}
	ctx.PropagateAs()
	return ctx.WriteInteger(0)
}

func cmdTTL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	ttl := ctx.Store.TTLMillis(ctx.ArgString(0))
	if ttl < 0 {
		return ctx.WriteInteger(ttl)
	}
	return ctx.WriteInteger((ttl + 500) / 1000)
}

func cmdPTTL(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	return ctx.WriteInteger(ctx.Store.TTLMillis(ctx.ArgString(0)))
}

func cmdPEXPIRETIME(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteInteger(-2)
	}
	if entry.ExpiresAt.Load() == 0 {
		return ctx.WriteInteger(-1)
	}
	return ctx.WriteInteger(entry.ExpiresAt.Load())
}

func cmdEXPIRETIME(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return ctx.WriteInteger(-2)
	}
	if entry.ExpiresAt.Load() == 0 {
		return ctx.WriteInteger(-1)
	}
	at := entry.ExpiresAt.Load()
	seconds := at / 1000
	if at%1000 >= 500 {
		seconds++
	}
	return ctx.WriteInteger(seconds)
}

func cmdPERSIST(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	if ctx.Store.Persist(ctx.ArgString(0)) {
		return ctx.WriteInteger(1)
	}
	ctx.PropagateAs()
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

var errNoSuchKey = errors.New("ERR no such key")

func cmdRENAME(ctx *Context) error {
	ctx.writeEffectsHandled = true
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	changed, err := ctx.Store.Transfer(ctx.ArgString(0), ctx.Store, ctx.ArgString(1), store.TransferOptions{Replace: true})
	if err == store.ErrKeyNotFound {
		return ctx.WriteError(errNoSuchKey)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	if !changed {
		ctx.PropagateAs()
	}
	return ctx.WriteOK()
}

func cmdRENAMENX(ctx *Context) error {
	ctx.writeEffectsHandled = true
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	changed, err := ctx.Store.Transfer(ctx.ArgString(0), ctx.Store, ctx.ArgString(1), store.TransferOptions{})
	if err == store.ErrKeyNotFound {
		return ctx.WriteError(errNoSuchKey)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	if !changed {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(1)
}

func cmdKEYS(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	const maxKeysResult = 100000 // Safety cap to prevent OOM on large databases

	pattern := ctx.ArgString(0)
	matched := make([]*resp.Value, 0)
	for key, entry := range ctx.Store.GetAll() {
		if entry.IsExpired() || !matchPattern(key, pattern) {
			continue
		}
		matched = append(matched, resp.BulkString(key))
		if len(matched) >= maxKeysResult {
			break
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

// matchPattern is Redis' glob matcher (stringmatchlen): *, ?, [abc], [a-z],
// [^a] and backslash escapes.
func matchPattern(s, pattern string) bool {
	return store.MatchPattern(s, pattern)
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

func cmdAUTH(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	// AUTH <username> <password> selects an ACL user. This is checked BEFORE
	// the requirepass gate so ACL users can authenticate on a server that has
	// no requirepass configured.
	if ctx.ArgCount() >= 2 {
		username := ctx.ArgString(0)
		password := ctx.ArgString(1)

		user, err := globalACL.Authenticate(username, password)
		if err != nil {
			recordACLDenial(ctx, "auth", "AUTH", username)
			ctx.SetAuthenticated(false)
			ctx.ACLUser = nil
			ctx.Username = ""
			return ctx.Writer.WriteError("WRONGPASS invalid username-password pair or user is disabled.")
		}

		ctx.SetAuthenticated(true)
		ctx.ACLUser = user
		ctx.Username = username
		return ctx.WriteOK()
	}

	password := ctx.ArgString(0)

	// The router dispatching this command holds the password: several
	// servers can share a process, each with its own router.
	r := ctx.dispatcher()
	if r == nil || r.RequirePass() == "" {
		return ctx.Writer.WriteError("ERR Client sent AUTH, but no password is set. Did you mean ACL SETUSER with >password?")
	}

	if !r.ValidatePassword(password) {
		recordACLDenial(ctx, "auth", "AUTH", "default")
		ctx.SetAuthenticated(false)
		return ctx.Writer.WriteError("WRONGPASS invalid username-password pair or user is disabled.")
	}

	ctx.SetAuthenticated(true)
	return ctx.WriteOK()
}

// SCAN walks the keyspace in the order of a 63-bit hash of each key, and the
// cursor is one more than the hash where the next call resumes. A key that is
// present for the whole iteration is therefore returned regardless of what is
// added or removed in between, which an index into an unordered key list
// could not guarantee.
func cmdSCAN(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	cursor, err := strconv.ParseUint(ctx.ArgString(0), 10, 64)
	if err != nil {
		return ctx.WriteError(errors.New("ERR invalid cursor"))
	}

	count := 10
	pattern := ""
	typeName := ""
	for i := 1; i < ctx.ArgCount(); i++ {
		arg := strings.ToUpper(ctx.ArgString(i))
		if i+1 >= ctx.ArgCount() {
			return ctx.WriteError(ErrSyntaxError)
		}
		switch arg {
		case "COUNT":
			i++
			n, ok := parseRedisInt(ctx.Arg(i))
			if !ok {
				return ctx.WriteError(ErrNotInteger)
			}
			if n < 1 {
				return ctx.WriteError(ErrSyntaxError)
			}
			count = int(min(n, 1<<30))
		case "MATCH":
			i++
			pattern = ctx.ArgString(i)
		case "TYPE":
			i++
			typeName = strings.ToLower(ctx.ArgString(i))
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}

	type scanKey struct {
		hash  uint64
		key   string
		entry *store.Entry
	}
	var start uint64
	if cursor > 0 {
		start = cursor - 1
	}
	candidates := make([]scanKey, 0)
	for key, entry := range ctx.Store.GetAll() {
		if h := scanHash(key); h >= start && !entry.IsExpired() {
			candidates = append(candidates, scanKey{h, key, entry})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].hash != candidates[j].hash {
			return candidates[i].hash < candidates[j].hash
		}
		return candidates[i].key < candidates[j].key
	})

	// Take count keys, then the rest of the last key's hash bucket so that a
	// resumed scan never splits keys that share a hash.
	end := min(count, len(candidates))
	for end > 0 && end < len(candidates) && candidates[end].hash == candidates[end-1].hash {
		end++
	}
	next := uint64(0)
	if end < len(candidates) {
		next = candidates[end].hash + 1
	}

	result := make([]*resp.Value, 0, end)
	for _, c := range candidates[:end] {
		if pattern != "" && !matchPattern(c.key, pattern) {
			continue
		}
		if typeName != "" && redisTypeName(c.entry.Value) != typeName {
			continue
		}
		result = append(result, resp.BulkString(c.key))
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(strconv.FormatUint(next, 10)),
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
			hotKeys = append(hotKeys, hotKey{key: key, access: entry.AccessCount.Load()})
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

func cmdCLIENT(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "LIST":
		return cmdClientList(ctx)
	case "SETNAME":
		if ctx.ArgCount() != 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		// Redis stores the name for this connection and echoes it from GETNAME.
		// This was a stub that replied +OK and discarded the value, so a named
		// client still reported no name -- and SLOWLOG GET element 6 could never
		// carry one. CLIENT SETNAME with an empty argument clears the name.
		if !validClientName(ctx.ArgString(1)) {
			return ctx.WriteError(errClientNameChars)
		}
		ctx.ClientName = ctx.ArgString(1)
		return ctx.WriteOK()
	case "GETNAME":
		if ctx.ClientName == "" {
			return ctx.WriteNullBulkString()
		}
		return ctx.WriteBulkString(ctx.ClientName)
	case "ID":
		return ctx.WriteInteger(ctx.ClientID)
	case "KILL":
		return cmdClientKill(ctx)
	case "PAUSE":
		return cmdClientPause(ctx)
	case "UNPAUSE":
		return cmdClientUnpause(ctx)
	case "UNBLOCK":
		return cmdClientUnblock(ctx)
	case "REPLY":
		return cmdClientReply(ctx)
	case "TRACKING":
		return cmdClientTracking(ctx)
	case "SETINFO":
		if ctx.ArgCount() != 3 {
			return ctx.WriteError(arityError("client|setinfo"))
		}
		attr, value := strings.ToUpper(ctx.ArgString(1)), ctx.ArgString(2)
		if attr != "LIB-NAME" && attr != "LIB-VER" {
			return ctx.WriteError(errors.New("ERR Unrecognized option '" + ctx.ArgString(1) + "'"))
		}
		if strings.ContainsAny(value, " \r\n") {
			return ctx.WriteError(errors.New("ERR " + strings.ToLower(attr) + " cannot contain spaces, newlines or special characters."))
		}
		if attr == "LIB-NAME" {
			ctx.ClientLibName = value
		} else {
			ctx.ClientLibVer = value
		}
		return ctx.WriteOK()
	case "CACHING":
		return cmdClientCaching(ctx)
	case "TRACKINGINFO":
		return cmdClientTrackingInfo(ctx)
	case "NO-TOUCH":
		return ctx.WriteOK()
	case "INFO":
		return cmdClientInfo(ctx)
	case "GETREDIR":
		if h := trackingHooksFor(ctx); h != nil {
			return ctx.WriteInteger(h.Redirect(ctx.ClientID))
		}
		return ctx.WriteInteger(-1)
	default:
		return ctx.WriteError(errors.New("ERR unknown subcommand '" + subCmd + "'"))
	}
}

func cmdSORT(ctx *Context) error {
	return doSort(ctx, false)
}

func cmdSORTRO(ctx *Context) error {
	return doSort(ctx, true)
}

// sortExpandPattern expands a SORT BY/GET pattern for one element: the first
// '*' is replaced by the element. A pattern with no '*' expands to nothing:
// Redis skips sorting for such a BY pattern and misses such a GET pattern.
func sortExpandPattern(pattern, elem string) (string, bool) {
	i := strings.IndexByte(pattern, '*')
	if i < 0 {
		return pattern, false
	}
	return pattern[:i] + elem + pattern[i+1:], true
}

// sortLookupValue reads the external key a BY/GET pattern expands to. A
// pattern with no '*' is a miss; hash-field patterns (key->field) resolve the
// named hash field. Only string values and hash fields are usable; anything
// else is a miss, which BY treats as weight 0 (skipping the sort when every
// element misses) and GET reports as a nil element.
func sortLookupValue(ctx *Context, pattern, elem string) (string, bool) {
	expanded, ok := sortExpandPattern(pattern, elem)
	if !ok {
		return "", false
	}
	lookup, field, hasField := strings.Cut(expanded, "->")
	entry, exists := ctx.Store.Get(lookup)
	if !exists {
		return "", false
	}
	if hasField {
		hv, ok := entry.Value.(*store.HashValue)
		if !ok {
			return "", false
		}
		// cmdHSET mutates Fields under the hash write lock.
		hv.RLock()
		v, ok := hv.Fields[field]
		hv.RUnlock()
		if !ok {
			return "", false
		}
		return string(v), true
	}
	sv, ok := entry.Value.(*store.StringValue)
	if !ok {
		return "", false
	}
	return string(sv.Data), true
}

// sortProjectElements applies each GET clause to every element, in the order
// the clauses were given, so each element yields len(patterns) values.
func sortProjectElements(ctx *Context, elements []string, patterns []string) []*resp.Value {
	out := make([]*resp.Value, 0, len(elements)*len(patterns))
	for _, elem := range elements {
		for _, p := range patterns {
			if p == "#" {
				out = append(out, resp.BulkString(elem))
				continue
			}
			if v, ok := sortLookupValue(ctx, p, elem); ok {
				out = append(out, resp.BulkString(v))
			} else {
				out = append(out, resp.NullBulkString())
			}
		}
	}
	return out
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
	byPattern := ""
	nosort := false
	var getPatterns []string

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
		case "BY":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			byPattern = ctx.ArgString(i)
			if strings.EqualFold(byPattern, "nosort") {
				nosort = true
			}
		case "GET":
			i++
			if i >= ctx.ArgCount() {
				return ctx.WriteError(ErrSyntaxError)
			}
			getPatterns = append(getPatterns, ctx.ArgString(i))
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
	// The command handlers mutate these collections under the value write
	// lock (cmdRPUSH/cmdLPOP, cmdSADD/cmdSREM), so the copy must be taken
	// under the matching read lock. No Store call happens inside these arms,
	// so holding RLock across the copy cannot self-deadlock.
	case *store.ListValue:
		v.RLock()
		for _, elem := range v.Elements {
			elements = append(elements, string(elem))
		}
		v.RUnlock()
	case *store.SetValue:
		v.RLock()
		for member := range v.Members {
			elements = append(elements, member)
		}
		v.RUnlock()
	case *store.SortedSetValue:
		entries := v.GetSortedRange(0, -1, false, false)
		for _, e := range entries {
			elements = append(elements, e.Member)
		}
	default:
		return ctx.WriteError(errors.New("ERR wrong type for SORT"))
	}

	if byPattern != "" && !nosort {
		// BY <pattern>: order the elements by the value of the external key
		// each one expands to, rather than by the element itself. A missing
		// or non-numeric weight counts as 0, matching Redis.
		type sortItem struct {
			elem    string
			weight  float64
			sweight string
		}
		items := make([]sortItem, 0, len(elements))
		for _, elem := range elements {
			it := sortItem{elem: elem}
			if w, ok := sortLookupValue(ctx, byPattern, elem); ok {
				it.sweight = w
				if f, err := strconv.ParseFloat(w, 64); err == nil {
					it.weight = f
				}
			}
			items = append(items, it)
		}
		if alpha {
			sort.SliceStable(items, func(i, j int) bool {
				if desc {
					return items[i].sweight > items[j].sweight
				}
				return items[i].sweight < items[j].sweight
			})
		} else {
			sort.SliceStable(items, func(i, j int) bool {
				if desc {
					return items[i].weight > items[j].weight
				}
				return items[i].weight < items[j].weight
			})
		}
		sorted := make([]string, 0, len(items))
		for _, it := range items {
			sorted = append(sorted, it.elem)
		}
		elements = sorted
	} else if alpha {
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

	// Redis treats a negative LIMIT offset as a count back from the end
	// (-1 is the last element); a non-negative one is clamped to the list.
	if offset < 0 {
		if -offset < len(elements) {
			start = len(elements) + offset
		} else {
			start = len(elements)
		}
	} else if offset >= len(elements) {
		start = len(elements)
	} else {
		start = offset
	}
	if count >= 0 {
		end = start + count
		if end > len(elements) {
			end = len(elements)
		}
	}

	elements = elements[start:end]

	// GET <pattern>: project each element through the external key its pattern
	// expands to, after LIMIT, so paging is decided on the sorted elements.
	if len(getPatterns) > 0 {
		projected := sortProjectElements(ctx, elements, getPatterns)

		if storeKey != "" {
			if readOnly {
				return ctx.WriteError(errors.New("ERR SORT_RO does not support STORE"))
			}
			if len(projected) == 0 {
				ctx.Store.Delete(storeKey)
				return ctx.WriteInteger(0)
			}
			list := &store.ListValue{Elements: make([][]byte, 0, len(projected))}
			for _, p := range projected {
				if p.Type == resp.TypeNull || p.IsNull {
					list.Elements = append(list.Elements, nil)
					continue
				}
				list.Elements = append(list.Elements, p.Bulk)
			}
			ctx.Store.Set(storeKey, list, store.SetOptions{})
			return ctx.WriteInteger(int64(len(projected)))
		}

		return ctx.WriteArray(projected)
	}

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

// lastSaveTime is the Unix second of the most recent SAVE/BGSAVE, reported by
// LASTSAVE. It is touched by three command handlers (LASTSAVE reads it and
// lazily initialises it; SAVE and BGSAVE write it), so it must be atomic -- it
// was a bare int64, and one client's SAVE raced another's LASTSAVE.
var lastSaveTime atomic.Int64

// saveHook performs the real persistence work behind SAVE and BGSAVE. The
// composition root installs it during New() -- before the listener starts --
// pointing at the configured AOF flush, so SAVE is an actual durability barrier.
//
// Without it both handlers only stamped lastSaveTime and replied +OK: a client
// that issued SAVE as a durability barrier got a false guarantee, and with
// aof_sync "no" the buffered AOF was never moved to disk at all (Append syncs
// only for AOFAlways and the background syncLoop runs only for
// AOFEverySecond). nil means persistence is not configured, and SAVE then says
// so instead of claiming a success it did not perform.
//
// Startup-only, exactly like ConfigureSentinel: installed once before traffic.
var saveHook func() error

// SetSaveHook installs the durability action SAVE and BGSAVE invoke.
func SetSaveHook(fn func() error) {
	saveHook = fn
}

// runSaveHook executes the installed durability action, reporting plainly when
// there is nothing configured to persist to.
func runSaveHook() error {
	if saveHook == nil {
		return errors.New("ERR persistence is not configured")
	}
	return saveHook()
}

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

	if count < 0 {
		count = len(s.entries)
	}
	if count > len(s.entries) {
		count = len(s.entries)
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

		// Read the recorder that production actually writes to. This handler
		// used to read command.globalSlowLog, a second SlowLog type that
		// nothing in production ever appended to (internal/server/
		// connection.go records into store.GlobalSlowLog), so SLOWLOG GET
		// and LEN always reported zero and RESET cleared nothing.
		//
		// store.SlowLog.Get treats n <= 0 as "all entries", but SLOWLOG's
		// documented contract is the opposite for 0: a count of -1 means all
		// entries, while 0 means "at most count entries", i.e. an empty
		// array. Normalise here so delegating does not silently change it.
		var entries []store.SlowLogEntry
		switch {
		case count == 0:
			entries = nil
		case count < 0:
			entries = store.GlobalSlowLog.Get(-1)
		default:
			entries = store.GlobalSlowLog.Get(count)
		}
		results := make([]*resp.Value, 0, len(entries))
		for _, entry := range entries {
			args := make([]*resp.Value, 0, len(entry.Args)+1)
			args = append(args, resp.BulkString(entry.Command))
			for _, a := range entry.Args {
				args = append(args, resp.BulkBytes(a))
			}
			results = append(results, resp.ArrayValue([]*resp.Value{
				resp.IntegerValue(entry.ID),
				// Redis reports field 2 as a Unix timestamp in SECONDS while
				// field 3 is in MICROSECONDS. Emitting UnixMicro() here
				// inflated the timestamp by 1e6, so a client rendering it as
				// a wall-clock date landed ~50,000 years in the future.
				resp.IntegerValue(entry.Timestamp.Unix()),
				resp.IntegerValue(entry.Duration.Microseconds()),
				resp.ArrayValue(args),
				resp.BulkString(entry.ClientIP),
				resp.BulkString(entry.ClientName),
			}))
		}
		return ctx.WriteArray(results)
	case "LEN":
		return ctx.WriteInteger(int64(store.GlobalSlowLog.Len()))
	case "RESET":
		store.GlobalSlowLog.Clear()
		return ctx.WriteOK()
	default:
		return ctx.WriteError(errors.New("ERR unknown subcommand '" + subCmd + "'"))
	}
}

func cmdLASTSAVE(ctx *Context) error {
	if r := ctx.dispatcher(); r != nil {
		_, last, _ := r.snapshotHooks()
		if last != nil {
			return ctx.WriteInteger(last())
		}
	}
	if lastSaveTime.Load() == 0 {
		lastSaveTime.Store(time.Now().Unix())
	}
	return ctx.WriteInteger(lastSaveTime.Load())
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
		output.WriteString("CacheStorm ver. " + buildversion.Version + "\n")
		output.WriteString("\n")
		output.WriteString("High-performance Redis-compatible cache with tag-based invalidation.\n")
	}

	return ctx.WriteBulkString(output.String())
}

// ShutdownOptions are the modifiers of SHUTDOWN.
type ShutdownOptions struct {
	Save   bool
	NoSave bool
	Now    bool
	Force  bool
}

// SetShutdownHook installs the function SHUTDOWN stops the server with.
func (r *Router) SetShutdownHook(fn func(ShutdownOptions) error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.shutdown = fn
}

// cmdSHUTDOWN implements SHUTDOWN [NOSAVE|SAVE] [NOW] [FORCE] [ABORT]. On
// success it sends no reply: the connection closes as the server stops.
func cmdSHUTDOWN(ctx *Context) error {
	var o ShutdownOptions
	abort := false
	for i := 0; i < ctx.ArgCount(); i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "NOSAVE":
			o.NoSave = true
		case "SAVE":
			o.Save = true
		case "NOW":
			o.Now = true
		case "FORCE":
			o.Force = true
		case "ABORT":
			abort = true
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}
	if (o.Save && o.NoSave) || (abort && ctx.ArgCount() > 1) {
		return ctx.WriteError(ErrSyntaxError)
	}
	if abort {
		return ctx.Writer.WriteError("ERR No shutdown in progress.")
	}
	var hook func(ShutdownOptions) error
	if r := ctx.dispatcher(); r != nil {
		r.mu.RLock()
		hook = r.shutdown
		r.mu.RUnlock()
	}
	if hook == nil {
		return ctx.WriteError(errors.New("ERR SHUTDOWN is not available in this context"))
	}
	if err := hook(o); err != nil {
		return ctx.WriteError(err)
	}
	ctx.CloseConnection = true
	return nil
}

func cmdSAVE(ctx *Context) error {
	if r := ctx.dispatcher(); r != nil {
		save, _, _ := r.snapshotHooks()
		if save != nil {
			if err := save(false); err != nil {
				return ctx.WriteError(err)
			}
			return ctx.WriteOK()
		}
	}
	// SAVE is the explicit durability barrier, so it must actually persist
	// before reporting success. Stamping the timestamp and replying +OK while
	// doing no I/O gave clients a guarantee the server had not kept, and
	// lastSaveTime then recorded a save that never happened.
	if err := runSaveHook(); err != nil {
		return ctx.WriteError(err)
	}
	lastSaveTime.Store(time.Now().Unix())
	return ctx.WriteOK()
}

func cmdBGSAVE(ctx *Context) error {
	if ctx.ArgCount() > 1 || (ctx.ArgCount() == 1 && !strings.EqualFold(ctx.ArgString(0), "SCHEDULE")) {
		return ctx.WriteError(ErrSyntaxError)
	}
	if r := ctx.dispatcher(); r != nil {
		save, _, _ := r.snapshotHooks()
		if save != nil {
			if err := save(true); err != nil {
				return ctx.WriteError(err)
			}
			return ctx.WriteSimpleString("Background saving started")
		}
	}
	// BGSAVE is the background form of the same barrier. The configured flush
	// is cheap and idempotent, so it runs inline: reporting "started" without a
	// guaranteed write would reintroduce the same false guarantee.
	if err := runSaveHook(); err != nil {
		return ctx.WriteError(err)
	}
	lastSaveTime.Store(time.Now().Unix())
	return ctx.WriteSimpleString("Background saving started")
}

func cmdBGREWRITEAOF(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	r := ctx.dispatcher()
	if r == nil {
		return ctx.WriteError(errors.New("ERR append only file is not enabled"))
	}
	rewrite := r.aofRewriteHook()
	if rewrite == nil {
		return ctx.WriteError(errors.New("ERR append only file is not enabled"))
	}
	if err := rewrite(); err != nil {
		return ctx.WriteError(err)
	}
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

func cmdACL(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	subCmd := strings.ToUpper(ctx.ArgString(0))

	switch subCmd {
	case "LIST":
		users := globalACL.ListUsers()
		results := make([]*resp.Value, 0, len(users))
		for _, name := range users {
			if user, ok := globalACL.GetUser(name); ok {
				results = append(results, resp.BulkString(user.ToACLString()))
			}
		}
		return ctx.WriteArray(results)
	case "USERS":
		users := globalACL.ListUsers()
		results := make([]*resp.Value, 0, len(users))
		for _, name := range users {
			results = append(results, resp.BulkString(name))
		}
		return ctx.WriteArray(results)
	case "WHOAMI":
		if ctx.Username != "" {
			return ctx.WriteBulkString(ctx.Username)
		}
		return ctx.WriteBulkString("default")
	case "CAT":
		return cmdACLCat(ctx)
	case "SETUSER":
		if ctx.ArgCount() < 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		username := ctx.ArgString(1)
		user, exists := globalACL.GetUser(username)
		if !exists {
			var err error
			user, err = globalACL.CreateUser(username)
			if err != nil {
				return ctx.WriteError(err)
			}
		}
		ruleParts := make([]string, 0, len(ctx.Args)-2)
		for i := 2; i < len(ctx.Args); i++ {
			ruleParts = append(ruleParts, string(ctx.Args[i]))
		}
		rule := strings.Join(ruleParts, " ")
		if err := acl.ParseACLRule(rule, user); err != nil {
			return ctx.WriteError(err)
		}
		if !user.Enabled {
			user.Enable()
		}
		return ctx.WriteOK()
	case "DELUSER":
		if ctx.ArgCount() < 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		deleted := int64(0)
		for i := 1; i < ctx.ArgCount(); i++ {
			username := ctx.ArgString(i)
			if err := globalACL.DeleteUser(username); err == nil {
				deleted++
			}
		}
		return ctx.WriteInteger(deleted)
	case "GETUSER":
		if ctx.ArgCount() < 2 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		username := ctx.ArgString(1)
		user, exists := globalACL.GetUser(username)
		if !exists {
			return ctx.WriteNull()
		}

		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("flags"),
			resp.ArrayValue([]*resp.Value{
				resp.BulkString(func() string {
					if user.IsEnabled() {
						return "on"
					}
					return "off"
				}()),
				resp.BulkString(func() string {
					if user.IsNoPassword() {
						return "nopass"
					}
					return ""
				}()),
			}),
			resp.BulkString("passwords"),
			resp.ArrayValue([]*resp.Value{}),
			resp.BulkString("commands"),
			resp.BulkString("+@all"),
			resp.BulkString("keys"),
			resp.ArrayValue([]*resp.Value{resp.BulkString("~*")}),
		})
	case "LOAD", "SAVE":
		return ctx.Writer.WriteError("ERR This Redis instance is not configured to use an ACL file. You may want to specify users via the ACL SETUSER command and then issue a CONFIG REWRITE (assuming you have a Redis configuration file set) in order to store users in the Redis configuration.")
	case "LOG":
		return cmdACLLog(ctx)
	case "DRYRUN":
		if ctx.ArgCount() < 3 {
			return ctx.WriteError(ErrWrongArgCount)
		}
		username := ctx.ArgString(1)
		cmd := ctx.ArgString(2)
		user, exists := globalACL.GetUser(username)
		if !exists {
			return ctx.WriteError(acl.ErrUserNotFound)
		}
		if user.CanExecuteCommand(cmd) {
			return ctx.WriteOK()
		}
		return ctx.WriteError(acl.ErrPermissionDenied)
	case "GENPASS":
		bits := 256
		if ctx.ArgCount() > 1 {
			var err error
			bits, err = strconv.Atoi(ctx.ArgString(1))
			if err != nil {
				return ctx.WriteError(ErrNotInteger)
			}
		}
		return ctx.WriteBulkString(generatePassword(bits))
	case "HELP":
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("LIST"),
			resp.BulkString("USERS"),
			resp.BulkString("WHOAMI"),
			resp.BulkString("CAT"),
			resp.BulkString("SETUSER <username> [rule ...]"),
			resp.BulkString("GETUSER <username>"),
			resp.BulkString("DELUSER <username> [username ...]"),
			resp.BulkString("DRYRUN <username> <command>"),
			resp.BulkString("GENPASS [bits]"),
			resp.BulkString("LOAD"),
			resp.BulkString("SAVE"),
		})
	default:
		return ctx.WriteError(errors.New("ERR unknown subcommand '" + subCmd + "'"))
	}
}

func generatePassword(bits int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	length := bits / 6
	if length < 8 {
		length = 8
	}
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			n = big.NewInt(0) // crypto/rand failure is practically impossible; fall back to first char
		}
		result[i] = chars[n.Int64()]
	}
	return string(result)
}

// cmdMONITOR puts the connection in monitor mode: the server streams every
// command other clients run to it (see MonitorVisible).
func cmdMONITOR(ctx *Context) error {
	ctx.GetSubscriber()
	ctx.Monitor = true
	return ctx.WriteOK()
}

// MonitorVisible reports whether MONITOR shows cmd. As in Redis, admin
// commands and those that carry credentials are left out.
func MonitorVisible(cmd string) bool {
	switch cmd {
	case "AUTH", "HELLO", "MIGRATE":
		return false
	}
	return !commandHasFlag(cmd, "admin")
}

func cmdDEBUGSEGFAULT(ctx *Context) error {
	return ctx.WriteError(errors.New("ERR SEGFAULT not allowed"))
}

// SetWaitAOFHook installs the function WAITAOF waits with. It returns how
// many local AOFs (0 or 1) and replicas acknowledged the writes so far.
func (r *Router) SetWaitAOFHook(fn func(numLocal, numReplicas int64, timeout time.Duration, done <-chan struct{}) (int64, int64, error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.waitAOF = fn
}

// cmdWAITAOF implements WAITAOF numlocal numreplicas timeout.
func cmdWAITAOF(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(arityError("waitaof"))
	}
	numLocal, err1 := strconv.ParseInt(ctx.ArgString(0), 10, 64)
	numReplicas, err2 := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err1 != nil || err2 != nil || numLocal < 0 || numReplicas < 0 {
		return ctx.WriteError(ErrNotInteger)
	}
	ms, err := strconv.ParseInt(ctx.ArgString(2), 10, 64)
	if err != nil {
		return ctx.WriteError(errors.New("ERR timeout is not an integer or out of range"))
	}
	if ms < 0 {
		return ctx.WriteError(errors.New("ERR timeout is negative"))
	}
	var hook func(int64, int64, time.Duration, <-chan struct{}) (int64, int64, error)
	if r := ctx.dispatcher(); r != nil {
		r.mu.RLock()
		hook = r.waitAOF
		r.mu.RUnlock()
	}
	if hook == nil {
		if numLocal > 0 {
			return ctx.Writer.WriteError("ERR WAITAOF cannot be used when numlocal is set but appendonly is disabled.")
		}
		return ctx.WriteArray([]*resp.Value{resp.IntegerValue(0), resp.IntegerValue(0)})
	}
	timeout := time.Duration(ms) * time.Millisecond
	if ctx.InExec {
		timeout = -1
	}
	local, replicas, err := hook(numLocal, numReplicas, timeout, ctx.Done)
	if err != nil {
		return ctx.Writer.WriteError(err.Error())
	}
	return ctx.WriteArray([]*resp.Value{resp.IntegerValue(local), resp.IntegerValue(replicas)})
}
