package command

import (
	"strconv"
	"strings"
	"sync"

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
	"UNWATCH": true, "WAIT": true, "RANDOMKEY": true, "TS.QUERYINDEX": true,
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
	"PEXPIRETIME": true, "RENAME": true, "RENAMENX": true, "RESTORE": true, "RESTORE-ASKING": true,
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
	"HEXPIRE": true, "HPEXPIRE": true, "HEXPIREAT": true, "HPEXPIREAT": true, "HTTL": true,
	"HPTTL": true, "HEXPIRETIME": true, "HPEXPIRETIME": true, "HPERSIST": true, "HGETEX": true,
	"HSETEX": true,
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

// sortLookupKeys returns the key patterns named by SORT's BY and GET options.
// sortExpandPattern turns "w_*" plus an element into "w_<elem>" and
// sortLookupValue then does ctx.Store.Get on it, so these name real keys and
// must be permission-checked. "BY nosort" and "GET #" reference no key.
func sortLookupKeys(args [][]byte) []string {
	var keys []string
	for i := 0; i+1 < len(args); i++ {
		switch strings.ToUpper(string(args[i])) {
		case "BY":
			p := string(args[i+1])
			if !strings.EqualFold(p, "nosort") {
				keys = append(keys, p)
			}
		case "GET":
			if p := string(args[i+1]); p != "#" {
				keys = append(keys, p)
			}
		}
	}
	return keys
}

// storeKeyAfter returns every destination that follows a STORE/STOREDIST
// keyword. GEORADIUSBYMEMBER and GEORADIUSSTORE accept BOTH keywords and write
// to both, so matching only "STORE" left a STOREDIST destination unchecked --
// a ~user:* user could write the distance set into any key.
func storeKeyAfter(args [][]byte) []string {
	var keys []string
	for i, a := range args {
		kw := strings.ToUpper(string(a))
		if (kw == "STORE" || kw == "STOREDIST") && i+1 < len(args) {
			keys = append(keys, string(args[i+1]))
		}
	}
	return keys
}

// aclKeysForCommand returns the keys a command operates on, for key-pattern
// enforcement.
//
// A command that is not classified here falls back to its first argument, so a
// key-pattern ACL such as ~user:* still applies to it — an unlisted data
// command must not become a way to read or write outside the sandbox. The
// table covers the commands whose keys are not simply args[0] (multi-key,
// NUMKEYS-counted, source/destination, keyword-separated) plus explicit no-key
// entries for admin commands, so an argument like "GET" in CONFIG GET is never
// mistaken for a key.
func aclKeysForCommand(cmd string, args [][]byte) []string {
	switch cmd {
	case "EVAL", "EVALSHA", "EVAL_RO", "EVALSHA_RO":
		// EVAL script numkeys key [key ...]
		return keysAfterNumkeys(args, 1)
	case "ZUNIONSTORE", "ZINTERSTORE", "ZDIFFSTORE":
		// ZUNIONSTORE dest numkeys key [key ...] — args[0] IS dest, the key this
		// command WRITES. Omitting it let a user confined to ~user:* compute
		// into any key outside the sandbox, so dest is checked alongside the
		// NUMKEYS-counted sources.
		if len(args) < 1 {
			return nil
		}
		keys := keysAfterNumkeys(args, 1)
		if keys == nil {
			// Malformed count: keep the existing contract of returning no keys
			// rather than guessing a shape.
			return nil
		}
		return append([]string{string(args[0])}, keys...)
	case "ZUNION", "ZINTER", "ZDIFF":
		// ZUNION numkeys key [key ...] — the READ-ONLY forms take NO
		// destination, so the count sits at args[0]. Reading it from args[1]
		// parsed a key as the count, atoi failed, and nil came back — leaving
		// these commands with no key check at all.
		return keysAfterNumkeys(args, 0)
	case "BITOP":
		// BITOP op destkey key [key ...]
		// args[0] is the operation word (AND/OR/XOR/NOT), NOT a key.
		// args[1] IS destkey — the key this command WRITES — and args[2:]
		// are the source keys. Both sides must be permission-checked: leaving
		// destkey out let a user confined to ~user:* compute into any key
		// outside the sandbox.
		if len(args) >= 3 {
			return toStrings(args[1:])
		}
		return nil
	case "SORT", "SORT_RO":
		// SORT key ... [STORE destination] [BY pattern] [GET pattern ...]
		keys := []string{}
		if len(args) >= 1 {
			keys = append(keys, string(args[0]))
		}
		keys = append(keys, storeKeyAfter(args)...)
		return append(keys, sortLookupKeys(args)...)
	case "SINTERCARD":
		return keysAfterNumkeys(args, 0)
	case "CMS.MERGE", "TDIGEST.MERGE":
		// CMS.MERGE destination numkeys source [source ...] [WEIGHTS ...]
		if len(args) < 1 {
			return nil
		}
		return append([]string{string(args[0])}, keysAfterNumkeys(args, 1)...)
	case "TS.MADD", "JSON.MSET":
		// TS.MADD key timestamp value [key timestamp value ...]
		var keys []string
		for i := 0; i < len(args); i += 3 {
			keys = append(keys, string(args[i]))
		}
		return keys
	case "JSON.MGET":
		// JSON.MGET key [key ...] path
		if len(args) < 2 {
			return nil
		}
		return toStrings(args[:len(args)-1])
	case "SINTER", "SDIFF", "SUNION", "SDIFFSTORE", "SINTERSTORE", "SUNIONSTORE":
		return toStrings(args)
	case "MIGRATE":
		// MIGRATE host port key|"" db timeout ... [KEYS key ...]
		var keys []string
		if len(args) > 2 && len(args[2]) > 0 {
			keys = append(keys, string(args[2]))
		}
		for i := 5; i < len(args); i++ {
			if strings.EqualFold(string(args[i]), "KEYS") {
				keys = append(keys, toStrings(args[i+1:])...)
				break
			}
		}
		return keys
	case "GEOSEARCHSTORE":
		// GEOSEARCHSTORE destination source <FROMMEMBER m | FROMLONLAT lon lat>
		// ... — BOTH keys must be checked. There was no entry for this command,
		// so it fell through to the fail-closed default and returned args[0]
		// only: the destination. The SOURCE at args[1] was then read
		// unchecked, letting a ~user:* user copy any geo set in the store.
		if len(args) >= 2 {
			return toStrings(args[:2])
		}
		if len(args) >= 1 {
			return toStrings(args[:1])
		}
		return nil
	case "LMPOP", "ZMPOP", "ZINTERCARD":
		// LMPOP numkeys key [key ...] LEFT|RIGHT [COUNT n]: the count sits
		// at args[0]. These fell through to the args[0] default, so the count
		// was checked as a key and the real keys never were.
		return keysAfterNumkeys(args, 0)
	case "BLMPOP", "BZMPOP":
		// BLMPOP timeout numkeys key [key ...] ...: BLMPOP was treated as
		// "source destination" and returned the timeout and the count.
		return keysAfterNumkeys(args, 1)
	case "BLPOP", "BRPOP", "BZPOPMIN", "BZPOPMAX":
		// BLPOP key [key ...] timeout: every argument but the timeout.
		// BZPOPMIN/BZPOPMAX checked only their first key.
		if len(args) >= 2 {
			return toStrings(args[:len(args)-1])
		}
		return toStrings(args)
	case "RENAME", "RENAMENX", "SMOVE", "LMOVE", "RPOPLPUSH", "BLMOVE", "BRPOPLPUSH",
		"COPY":
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
	case "GEORADIUS", "GEORADIUSBYMEMBER":
		if len(args) == 0 {
			return nil
		}
		keys := []string{string(args[0])}
		offset := 5
		if cmd == "GEORADIUSBYMEMBER" {
			offset = 4
		}
		if len(args) > offset {
			keys = append(keys, storeKeyAfter(args[offset:])...)
		}
		return keys
	case "GEORADIUSSTORE":
		// source ... [STORE destination]
		keys := []string{}
		if len(args) >= 1 {
			keys = append(keys, string(args[0]))
		}
		return append(keys, storeKeyAfter(args)...)
	case "XREAD", "XREADGROUP":
		// XREAD [COUNT n] [BLOCK ms] STREAMS key [key ...] id [id ...]
		// XREADGROUP GROUP g c [COUNT n] [BLOCK ms] [NOACK] STREAMS key ...
		// STREAMS is followed by an equal number of keys and IDs, keys
		// first. An odd remainder is malformed, so return every remaining
		// argument rather than miss a key.
		for i, a := range args {
			if strings.EqualFold(string(a), "STREAMS") {
				rest := args[i+1:]
				if len(rest)%2 != 0 {
					return toStrings(rest)
				}
				return toStrings(rest[:len(rest)/2])
			}
		}
		return nil
	case "XINFO", "XGROUP":
		// XINFO STREAM|GROUPS|CONSUMERS key, XGROUP CREATE|SETID|DESTROY
		// key ... — both take their subcommand first, key second.
		if len(args) >= 2 {
			return toStrings(args[1:2])
		}
		return nil
	case "FCALL", "FCALL_RO":
		// FCALL f-name numkeys key [key ...] arg [arg ...] — the key count
		// sits between the function name and the keys, so the function name
		// itself is not a key.
		if len(args) < 2 {
			return nil
		}
		n := atoiSafe(args[1])
		if n < 0 || 2+n > len(args) {
			return nil
		}
		return toStrings(args[2 : 2+n])
	}

	switch keyShapeOf(cmd) {
	case keyShapeMSET:
		keys := make([]string, 0, (len(args)+1)/2)
		for i := 0; i+1 < len(args); i += 2 {
			keys = append(keys, string(args[i]))
		}
		return keys
	case keyShapeAll:
		return toStrings(args)
	case keyShapeNone:
		return nil
	}
	// Fail closed for a command the tables do not classify. Falling through
	// with no keys let a user confined to ~user:* read and write any other
	// key through an unlisted command — XADD was the proof (round r27). Data
	// commands overwhelmingly take the key as their first argument, so that
	// is the safe assumption; a command that truly takes no keys belongs in
	// aclNoKeyCommands.
	if len(args) >= 1 {
		return toStrings(args[:1])
	}
	return nil
}

type keyShape uint8

const (
	keyShapeFirst keyShape = iota
	keyShapeMSET
	keyShapeAll
	keyShapeNone
)

// keyShapes caches keyShapeOf: the tables it reads never change, and a
// command's keys are worked out for every command a connection runs.
var keyShapes sync.Map

// keyShapeOf classifies where the keys of a command the switch in
// aclKeysForCommand does not name sit in its arguments.
func keyShapeOf(cmd string) keyShape {
	if v, ok := keyShapes.Load(cmd); ok {
		return v.(keyShape)
	}
	shape := keyShapeFirst
	switch {
	case aclMSETCommands[cmd]:
		shape = keyShapeMSET
	case aclAllArgsCommands[cmd]:
		shape = keyShapeAll
	case aclNoKeyCommands[cmd], strings.HasPrefix(cmd, "FT."):
		// A search index is named by its first argument, which is no key.
		shape = keyShapeNone
	default:
		if spec, ok := redisCommandSpecs[cmd]; ok && spec.FirstKey == 0 && !commandHasFlag(cmd, "movablekeys") {
			shape = keyShapeNone
		}
	}
	if _, ok := redisCommandSpecs[cmd]; ok || aclSingleKeyCommands[cmd] || shape != keyShapeFirst {
		keyShapes.Store(cmd, shape)
	}
	return shape
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

// AuthenticateACL resolves a username/password pair against the ACL registry
// that backs the TCP path, so alternative entry points such as the HTTP API
// can map a client to the same per-user identity. The errors are the ones
// ACL.Authenticate returns (unknown user, invalid password).
func AuthenticateACL(username, password string) (*acl.User, error) {
	return globalACL.Authenticate(username, password)
}

// ACLUserByName resolves a username from the ACL registry for callers that
// hold a username rather than credentials, such as a session token bound to
// an authenticated identity.
func ACLUserByName(username string) (*acl.User, bool) {
	return globalACL.GetUser(username)
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

	denial, item := aclCheck(user, upperCmd, ctx.Args)
	switch denial {
	case acl.DeniedCommand:
		shown := aclShownName(upperCmd, ctx.Args)
		recordACLDenial(ctx, "command", shown, user.Name)
		_ = ctx.Writer.WriteError("NOPERM User " + user.Name + " has no permissions to run the '" + shown + "' command")
		return true
	case acl.DeniedKey:
		recordACLDenial(ctx, "key", item, user.Name)
		_ = ctx.Writer.WriteError("NOPERM No permissions to access a key")
		return true
	case acl.DeniedChannel:
		recordACLDenial(ctx, "channel", item, user.Name)
		_ = ctx.Writer.WriteError("NOPERM No permissions to access a channel")
		return true
	}
	return false
}

// aclShownName is the command name a NOPERM reply shows: "cmd" or
// "cmd|sub".
func aclShownName(upperCmd string, args [][]byte) string {
	shown := strings.ToLower(upperCmd)
	if hasSubcommands(upperCmd) && len(args) > 0 {
		shown += "|" + strings.ToLower(string(args[0]))
	}
	return shown
}

// aclCheck checks whether user may run upperCmd with args, with its keys
// and channels, across the user's selectors.
func aclCheck(user *acl.User, upperCmd string, args [][]byte) (acl.Denial, string) {
	sub := ""
	if hasSubcommands(upperCmd) && len(args) > 0 {
		sub = string(args[0])
	}
	return user.Check(upperCmd, sub,
		func() []acl.KeyRef { return aclKeyRefs(upperCmd, args) },
		func() []acl.ChannelRef { return aclChannelRefs(upperCmd, args) })
}

// aclKeyRefs returns the keys of a command with the access each needs,
// taken from Redis' key specs: a key the command reads needs read access, one
// it inserts, deletes or updates needs write access, and one it neither reads
// nor changes (EXISTS, TYPE) only has to match a pattern. A command without
// usable specs needs both on every key aclKeysForCommand finds.
func aclKeyRefs(upperCmd string, args [][]byte) []acl.KeyRef {
	argv := make([]string, 0, len(args)+1)
	argv = append(argv, upperCmd)
	for _, a := range args {
		argv = append(argv, string(a))
	}
	if flags, ok := aclVariableKeyFlags(upperCmd, args); ok {
		return []acl.KeyRef{{Name: string(args[0]), Flags: flags}}
	}
	var out []acl.KeyRef
	meta := lookupRedisCommandMeta(argv)
	if meta != nil {
		if !commandHasKeys(meta) {
			return nil
		}
		if refs, ok := keysFromSpecs(meta, argv); ok && upperCmd != "SORT" && upperCmd != "SORT_RO" {
			out = make([]acl.KeyRef, 0, len(refs))
			for _, r := range refs {
				out = append(out, acl.KeyRef{Name: r.name, Flags: aclKeyFlags(r.flags)})
			}
			return out
		}
		for _, r := range keysFromSpecsPartial(meta, argv) {
			if r.name != "" || upperCmd != "MIGRATE" {
				out = append(out, acl.KeyRef{Name: r.name, Flags: aclKeyFlags(r.flags)})
			}
		}
	}
	if upperCmd == "SORT" {
		for _, k := range storeKeyAfter(args) {
			out = append(out, acl.KeyRef{Name: k, Flags: acl.KeyWrite})
		}
	}
	// Keys the specs could not place, or SORT's BY and GET patterns, need
	// both permissions.
	for _, k := range aclKeysForCommand(upperCmd, args) {
		found := false
		for _, r := range out {
			if r.Name == k {
				found = true
				break
			}
		}
		if !found {
			out = append(out, acl.KeyRef{Name: k, Flags: acl.KeyReadWrite})
		}
	}
	return out
}

func aclVariableKeyFlags(upperCmd string, args [][]byte) (uint8, bool) {
	if len(args) == 0 {
		return 0, false
	}
	switch upperCmd {
	case "SET":
		for i := 2; i < len(args); i++ {
			a := args[i]
			if strings.EqualFold(string(a), "GET") {
				return acl.KeyReadWrite, true
			}
		}
		return acl.KeyWrite, true
	case "BITFIELD":
		for j := 1; j < len(args); j++ {
			rem := len(args) - j - 1
			switch op := string(args[j]); {
			case strings.EqualFold(op, "GET") && rem >= 2:
				j += 2
			case (strings.EqualFold(op, "SET") || strings.EqualFold(op, "INCRBY")) && rem >= 3:
				return acl.KeyReadWrite, true
			case strings.EqualFold(op, "OVERFLOW") && rem >= 1:
				j++
			default:
				return acl.KeyReadWrite, true
			}
		}
		return acl.KeyRead, true
	}
	return 0, false
}

func aclKeyFlags(specFlags []string) uint8 {
	var flags uint8
	for _, f := range specFlags {
		switch f {
		case "access":
			flags |= acl.KeyRead
		case "insert", "delete", "update":
			flags |= acl.KeyWrite
		}
	}
	return flags
}

func aclChannelRefs(upperCmd string, args [][]byte) []acl.ChannelRef {
	chans := aclChannelsForCommand(upperCmd, args)
	out := make([]acl.ChannelRef, len(chans))
	for i, c := range chans {
		out[i] = acl.ChannelRef{Name: c, Pattern: upperCmd == "PSUBSCRIBE"}
	}
	return out
}

// CommandKeys returns the keys cmd names in args, the same ones the ACL key
// checks inspect. A command the tables do not classify reports its first
// argument.
func CommandKeys(cmd string, args [][]byte) []string {
	return aclKeysForCommand(CanonicalName(cmd), args)
}
