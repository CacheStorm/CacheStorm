package command

import (
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/acl"
)

// aclBypassCommands are always permitted, whatever a user's permissions say.
// A client must always be able to authenticate, probe and disconnect. This
// mirrors the router's existing noAuthCommands set.
var aclBypassCommands = map[string]bool{
	"AUTH":    true,
	"HELLO":   true,
	"PING":    true,
	"QUIT":    true,
	"RESET":   true,
	"COMMAND": true,
}

// aclNoKeyCommands operate on connection/server state rather than keys. Listed
// explicitly so a key-pattern check never mistakes an admin argument for a key.
var aclNoKeyCommands = map[string]bool{
	"ACL": true, "AUTH": true, "BGREWRITEAOF": true, "BGSAVE": true, "CLIENT": true,
	"CLUSTER": true, "COMMAND": true, "CONFIG": true, "DBSIZE": true, "DEBUG": true,
	"DISCARD": true, "ECHO": true, "EXEC": true, "FLUSHALL": true, "FLUSHDB": true,
	"FUNCTION": true, "HELLO": true, "INFO": true, "LASTSAVE": true, "LOLWUT": true,
	"MODULE": true, "MONITOR": true, "MULTI": true, "OBJECT": true, "PING": true,
	"PSYNC": true, "PUBLISH": true, "PUBSUB": true, "QUIT": true,
	"REPLICAOF": true, "RESET": true, "ROLE": true, "SAVE": true, "SCAN": true,
	"SCRIPT": true, "SELECT": true, "SHUTDOWN": true, "SLAVEOF": true, "SLOWLOG": true,
	"SUBSCRIBE": true, "SWAPDB": true, "TIME": true, "UNSUBSCRIBE": true,
	"UNWATCH": true, "WAIT": true, "RANDOMKEY": true,
}

// aclSingleKeyCommands take the key as their first argument.
var aclSingleKeyCommands = map[string]bool{
	// strings
	"APPEND": true, "DECR": true, "DECRBY": true, "GET": true, "GETDEL": true,
	"GETEX": true, "GETRANGE": true, "GETSET": true, "INCR": true, "INCRBY": true,
	"INCRBYFLOAT": true, "PSETEX": true, "SET": true, "SETEX": true, "SETNX": true,
	"SETRANGE": true, "STRLEN": true, "SUBSTR": true,
	// bitmaps
	"BITCOUNT": true, "BITFIELD": true, "BITFIELD_RO": true, "BITPOS": true,
	"GETBIT": true, "SETBIT": true,
	// generic key + expiry
	"COPY": true, "DUMP": true, "EXPIRE": true, "EXPIREAT": true, "EXPIRETIME": true,
	"EXISTS_": true, "MOVE": true, "PERSIST": true, "PEXPIRE": true, "PEXPIREAT": true,
	"PEXPIRETIME": true, "RENAME": true, "RENAMENX": true, "RESTORE": true,
	"TOUCH": true, "TTL": true, "TYPE": true, "UNLINK": true,
	// lists
	"BLMOVE": true, "BLMPOP": true, "BRPOP": true, "BRPOPLPUSH": true, "LINDEX": true,
	"LLEN": true, "LMOVE": true, "LPOP": true, "LPOS": true, "LPUSH": true, "LPUSHX": true,
	"RPOP": true, "RPOPLPUSH": true, "RPUSH": true, "RPUSHX": true,
	"LSET": true, "LTRIM": true, "LREM": true, "LRANGE": true,
	// hashes
	"HDEL": true, "HEXISTS": true, "HGET": true, "HGETALL": true, "HINCRBY": true,
	"HINCRBYFLOAT": true, "HKEYS": true, "HLEN": true, "HMSET": true, "HRANDFIELD": true,
	"HSCAN": true, "HSET": true, "HSETNX": true, "HSTRLEN": true, "HVALS": true,
	// sets
	"SADD": true, "SCARD": true, "SDIFF": true, "SINTER": true, "SINTERSTORE": true,
	"SISMEMBER": true, "SMEMBERS": true, "SMISMEMBER": true, "SMOVE": true, "SPOP": true,
	"SRANDMEMBER": true, "SREM": true, "SSCAN": true, "SUNION": true, "SUNIONSTORE": true,
	// sorted sets
	"ZADD": true, "ZCARD": true, "ZCOUNT": true, "ZDIFF": true, "ZDIFFSTORE": true,
	"ZINCRBY": true, "ZINTER": true, "ZINTERSTORE": true, "ZLEXCOUNT": true,
	"ZMSCORE": true, "ZPOPMAX": true, "ZPOPMIN": true, "ZRANDMEMBER": true, "ZRANGE": true,
	"ZRANGEBYLEX": true, "ZRANGEBYSCORE": true, "ZRANK": true, "ZREM": true,
	"ZREMRANGEBYLEX": true, "ZREMRANGEBYRANK": true, "ZREMRANGEBYSCORE": true, "ZREVRANGE": true,
	"ZREVRANGEBYLEX": true, "ZREVRANGEBYSCORE": true, "ZREVRANK": true, "ZSCAN": true,
	"ZSCORE": true, "ZUNION": true, "ZUNIONSTORE": true,
	// geo
	"GEOADD": true, "GEODIST": true, "GEOHASH": true, "GEOPOS": true, "GEOSEARCH": true,
}

// aclAllArgsCommands take a variable list of keys as the whole argument list.
var aclAllArgsCommands = map[string]bool{
	"DEL": true, "EXISTS": true, "MGET": true, "PFMERGE": true, "PFCOUNT": true,
	"SDIFFSTORE": true, "SINTERSTORE": true, "SUNIONSTORE": true, "UNLINK": true,
	"WATCH": true, "BLPOP": true, "BRPOP": true,
}

// aclMSETCommands alternate key/value, so only the even indices are keys.
var aclMSETCommands = map[string]bool{
	"MSET": true, "MSETNX": true,
}

func toStrings(args [][]byte) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		out = append(out, string(a))
	}
	return out
}

func atoiSafe(b []byte) int {
	n, err := strconv.Atoi(string(b))
	if err != nil {
		return -1
	}
	return n
}

// keysAfterNumkeys returns the args following a leading count, e.g.
// ZUNIONSTORE dest numkeys key [key ...]. base is the index of the count.
func keysAfterNumkeys(args [][]byte, base int) []string {
	if len(args) < base+1 {
		return nil
	}
	n := atoiSafe(args[base])
	if n < 0 || base+1+n > len(args) {
		return nil
	}
	return toStrings(args[base+1 : base+1+n])
}

// storeKeyAfter returns the destination that follows a STORE/any-store keyword.
func storeKeyAfter(args [][]byte) []string {
	for i, a := range args {
		if strings.EqualFold(string(a), "STORE") && i+1 < len(args) {
			return []string{string(args[i+1])}
		}
	}
	return nil
}

// aclKeysForCommand returns the keys a command operates on, for key-pattern
// enforcement.
//
// A command that is not covered here is still subject to COMMAND permission
// enforcement; only its key-pattern check is skipped. The table below covers
// the data commands — which is where a key-pattern ACL such as ~user:* carries
// its security weight — plus explicit no-key entries for admin commands, so an
// argument like "GET" in CONFIG GET is never mistaken for a key.
func aclKeysForCommand(cmd string, args [][]byte) []string {
	switch cmd {
	case "EVAL", "EVALSHA":
		// EVAL script numkeys key [key ...]
		return keysAfterNumkeys(args, 1)
	case "ZUNIONSTORE", "ZINTERSTORE", "ZDIFFSTORE", "ZUNION", "ZINTER", "ZDIFF":
		// dest numkeys key [key ...]
		return keysAfterNumkeys(args, 1)
	case "BITOP":
		// BITOP op destkey key [key ...]
		if len(args) >= 3 {
			return toStrings(args[2:])
		}
		return nil
	case "SORT", "SORT_RO":
		// SORT key ... [STORE destination]
		keys := []string{}
		if len(args) >= 1 {
			keys = append(keys, string(args[0]))
		}
		return append(keys, storeKeyAfter(args)...)
	case "RENAME", "RENAMENX", "SMOVE", "LMOVE", "RPOPLPUSH", "BLMOVE", "BRPOPLPUSH",
		"BLMPOP", "COPY", "GEORADIUSBYMEMBER_RO":
		// source destination [member ...] — BOTH sides must be permitted.
		// Omitting the destination lets a user move a key out of the
		// key-pattern they are confined to.
		if len(args) >= 2 {
			return toStrings(args[:2])
		}
		if len(args) == 1 {
			return toStrings(args[:1])
		}
		return nil
	case "GEORADIUSSTORE", "GEORADIUSBYMEMBER":
		// source ... [STORE destination]
		keys := []string{}
		if len(args) >= 1 {
			keys = append(keys, string(args[0]))
		}
		return append(keys, storeKeyAfter(args)...)
	}

	if aclMSETCommands[cmd] {
		keys := make([]string, 0, (len(args)+1)/2)
		for i := 0; i+1 < len(args); i += 2 {
			keys = append(keys, string(args[i]))
		}
		return keys
	}
	if aclAllArgsCommands[cmd] {
		return toStrings(args)
	}
	if aclNoKeyCommands[cmd] {
		return nil
	}
	if aclSingleKeyCommands[cmd] {
		if len(args) >= 1 {
			return toStrings(args[:1])
		}
		return nil
	}
	return nil
}

// aclChannelsForCommand returns the channels a pub/sub command names, for
// channel-pattern (&pattern) enforcement.
//
// PUBLISH and SPUBLISH name exactly one channel, with the message payload after
// it — treating every argument as a channel would refuse a perfectly legal
// publish merely because the payload text does not match the allowed pattern.
// SUBSCRIBE/PSUBSCRIBE and the sharded SUBSCRIBE variants take one or more
// channels or patterns, so every argument is a channel.
//
// A user with no &rule has an empty AllowedChannels list, which CanAccessChannel
// treats as unrestricted — matching Redis, so a user limited by ~keys only is
// not newly confined to no channels.
func aclChannelsForCommand(cmd string, args [][]byte) []string {
	switch cmd {
	case "PUBLISH", "SPUBLISH":
		// PUBLISH <channel> <message>
		if len(args) >= 1 {
			return []string{string(args[0])}
		}
		return nil
	case "SUBSCRIBE", "PSUBSCRIBE", "SSUBSCRIBE":
		// SUBSCRIBE <channel> [channel ...]
		return toStrings(args)
	}
	return nil
}

// aclAdminCommands administer the security and tunables of the server itself.
// They are reserved for the default user: a connection carrying an ACL user is
// refused before the command permission check runs, because granting a user ACL
// access is enough to defeat key- and command-level restrictions entirely —
// `ACL SETUSER self +@all ~*` widens every rule at once. Refusing the whole
// command is the safe subset; Redis's finer model, where a user may edit its own
// rules but never beyond its own permissions, is deliberately not implemented.
//
// CONFIG is included for the same reason. A user allowed to run it could reshape
// maxmemory, maxclients, the append/persistence mode and the server timeout —
// a denial-of-service surface that no key or command pattern can bound.
// Note that the specific `CONFIG SET requirepass ""` attack is NOT the reason:
// requirepass is not a settable parameter in this implementation (cmdConfigSet
// has no such case, and no default case either, so the call silently no-ops), so
// authentication cannot be removed this way today. The restriction is
// defence-in-depth against the settable parameters that do exist.
var aclAdminCommands = map[string]bool{
	"ACL":    true,
	"CONFIG": true,
}

// enforceACL applies the authenticated ACL user's permissions to one command.
//
// It reports whether the command was refused, having already written the NOPERM
// reply itself. It deliberately returns a bool rather than an error: the
// handlers in this package follow the convention `return ctx.WriteError(...)`,
// which writes the reply and then returns nil, so an `if err != nil` guard
// would not short-circuit and the command would still execute and emit a
// second reply. The router therefore stops on the bool.
//
// A connection that has not authenticated as an ACL user has ACLUser == nil and
// keeps the default (unrestricted) behaviour, which matches Redis applying the
// permissive default user to unauthenticated clients.
func enforceACL(ctx *Context, upperCmd string) bool {
	user := ctx.ACLUser
	if user == nil || aclBypassCommands[upperCmd] {
		return false
	}

	// ACL administration is checked before the command permission, so that
	// explicitly granting "+acl" (or "+@all") does not become a privilege
	// escalation. This runs only when ctx.ACLUser != nil, so the default user
	// keeps full ACL administration.
	if aclAdminCommands[upperCmd] {
		_ = ctx.WriteError(acl.ErrPermissionDenied)
		return true
	}

	if !user.CanExecuteCommand(upperCmd) {
		_ = ctx.WriteError(acl.ErrPermissionDenied)
		return true
	}
	for _, key := range aclKeysForCommand(upperCmd, ctx.Args) {
		if !user.CanAccessKey(key) {
			_ = ctx.WriteError(acl.ErrPermissionDenied)
			return true
		}
	}
	for _, channel := range aclChannelsForCommand(upperCmd, ctx.Args) {
		if !user.CanAccessChannel(channel) {
			_ = ctx.WriteError(acl.ErrPermissionDenied)
			return true
		}
	}
	return false
}
