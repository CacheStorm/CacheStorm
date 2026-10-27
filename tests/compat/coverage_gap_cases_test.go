//go:build compat

package compat

// coverageGapCases cover command families the rest of the corpus does not:
// SORT, the bit commands, LCS, FUNCTION/FCALL, SCRIPT, OBJECT, COPY/DUMP
// corners, string edge cases and EXPIRE on collections.
func coverageGapCases() []Case {
	lib := "#!lua name=mylib\nredis.register_function('myget', function(keys, args) return redis.call('GET', keys[1]) end)\nredis.register_function{function_name='ro', callback=function(keys, args) return args[1] end, flags={'no-writes'}}"
	return []Case{
		k("sort", "numeric-alpha-limit", c("RPUSH", "l", "3", "1", "2", "10"), c("SORT", "l"), c("SORT", "l", "DESC"), c("SORT", "l", "ALPHA"),
			c("SORT", "l", "LIMIT", "1", "2"), c("SORT", "l", "LIMIT", "0", "-1"), c("RPUSH", "w", "b", "a"), c("SORT", "w"), c("SORT", "w", "ALPHA", "DESC")),
		k("sort", "by-get-store", c("RPUSH", "ids", "1", "2", "3"), c("MSET", "w_1", "30", "w_2", "10", "w_3", "20", "n_1", "one", "n_2", "two"),
			c("SORT", "ids", "BY", "w_*"), c("SORT", "ids", "BY", "w_*", "GET", "n_*", "GET", "#"), c("SORT", "ids", "BY", "nosort", "GET", "n_*"),
			c("SORT", "ids", "BY", "w_*", "STORE", "out"), c("LRANGE", "out", "0", "-1"), c("SORT", "missing", "STORE", "out"), c("EXISTS", "out")),
		k("sort", "hash-by-and-sets", c("SADD", "s", "c", "a", "b"), c("SORT", "s", "ALPHA"), c("ZADD", "z", "3", "x", "1", "y"), c("SORT", "z", "ALPHA"),
			c("RPUSH", "ids", "1", "2"), c("HSET", "h_1", "f", "9"), c("HSET", "h_2", "f", "4"), c("SORT", "ids", "BY", "h_*->f"), c("SORT", "ids", "BY", "h_*->f", "GET", "h_*->f")),
		k("sort", "errors", c("SET", "str", "x"), c("SORT", "str"), c("RPUSH", "l", "a", "1"), c("SORT", "l"), c("SORT", "l", "LIMIT", "x", "1"),
			c("SORT", "l", "BOGUS"), c("SORT_RO", "l", "ALPHA"), c("SORT_RO", "l", "STORE", "x")),
		k("bits", "bitcount-bitpos", c("SET", "b", "\xff\xf0\x00"), c("BITCOUNT", "b"), c("BITCOUNT", "b", "0", "0"), c("BITCOUNT", "b", "1", "1", "BIT"),
			c("BITCOUNT", "b", "-1", "-1"), c("BITPOS", "b", "0"), c("BITPOS", "b", "1", "1"), c("BITPOS", "b", "0", "0", "0"),
			c("BITPOS", "b", "1", "2"), c("BITPOS", "b", "1", "7", "15", "BIT"), c("BITPOS", "missing", "0"), c("BITPOS", "missing", "1"),
			c("BITCOUNT", "b", "0"), c("BITCOUNT", "b", "0", "1", "BOGUS")),
		k("bits", "bitop", c("SET", "a", "\x0f\xf0"), c("SET", "b", "\xff"), c("BITOP", "AND", "d", "a", "b"), c("GET", "d"), c("BITOP", "OR", "d", "a", "b"), c("GET", "d"),
			c("BITOP", "XOR", "d", "a", "b"), c("GET", "d"), c("BITOP", "NOT", "d", "a"), c("GET", "d"), c("BITOP", "NOT", "d", "a", "b"),
			c("BITOP", "AND", "d", "missing"), c("EXISTS", "d"), c("BITOP", "BOGUS", "d", "a")),
		k("strings", "lcs-forms", c("MSET", "a", "ohmytext", "b", "mynewtext"), c("LCS", "a", "b"), c("LCS", "a", "b", "LEN"), c("LCS", "a", "b", "IDX"),
			c("LCS", "a", "b", "IDX", "MINMATCHLEN", "4", "WITHMATCHLEN"), c("LCS", "a", "missing"), c("LCS", "a", "b", "LEN", "IDX")),
		k("strings", "setrange-getrange-edges", c("SETRANGE", "k", "5", "x"), c("GET", "k"), c("SETRANGE", "k", "0", ""), c("SETRANGE", "missing", "3", ""),
			c("EXISTS", "missing"), c("GETRANGE", "k", "-3", "-1"), c("GETRANGE", "k", "10", "20"), c("GETRANGE", "k", "3", "1"), c("SETRANGE", "k", "-1", "x"),
			c("SETRANGE", "k", "536870912", "x")),
		k("strings", "incr-decr-edges", c("SET", "n", " 1"), c("INCR", "n"), c("SET", "n", "1.0"), c("INCR", "n"), c("SET", "n", "-9223372036854775808"),
			c("DECR", "n"), c("DECRBY", "x", "-9223372036854775808"), c("INCRBY", "x", "abc"), c("SET", "f", "1e3"), c("INCRBYFLOAT", "f", "0.5"),
			c("INCRBYFLOAT", "f", "inf"), c("SET", "f", "5.0e3"), c("INCRBYFLOAT", "f", "1")),
		k("function", "load-call-list", c("FUNCTION", "LOAD", lib), c("SET", "k", "v"), c("FCALL", "myget", "1", "k"), c("FCALL_RO", "ro", "0", "x"),
			c("FCALL_RO", "myget", "1", "k"), c("FUNCTION", "LOAD", lib), c("FUNCTION", "LOAD", "REPLACE", lib), c("FCALL", "nope", "0"),
			c("FUNCTION", "LIST", "LIBRARYNAME", "nomatch"), c("FUNCTION", "DELETE", "mylib"), c("FUNCTION", "DELETE", "mylib"), c("FCALL", "myget", "1", "k")),
		k("function", "load-errors", c("FUNCTION", "LOAD", "return 1"), c("FUNCTION", "LOAD", "#!lua name=x\nreturn 1"),
			c("FUNCTION", "LOAD", "#!lua\nredis.register_function('f', function() end)"), c("FUNCTION", "LOAD", "#!py name=x\n"),
			c("FUNCTION", "FLUSH"), c("FUNCTION", "BOGUS")),
		k("scripting", "script-commands", c("SCRIPT", "LOAD", "return 1"), c("SCRIPT", "EXISTS", "e0e1f9fabfc9d4800c877a703b823ac0578ff8db", "ffffffffffffffffffffffffffffffffffffffff"),
			c("SCRIPT", "FLUSH"), c("SCRIPT", "EXISTS", "e0e1f9fabfc9d4800c877a703b823ac0578ff8db"), c("SCRIPT", "FLUSH", "ASYNC"), c("SCRIPT", "KILL"), c("SCRIPT", "BOGUS")),
		k("scripting", "eval-ro-and-flags", c("SET", "k", "v"), c("EVAL_RO", "return redis.call('GET', KEYS[1])", "1", "k"),
			c("EVAL_RO", "return redis.call('SET', KEYS[1], 'x')", "1", "k"), c("EVAL", "#!lua flags=no-writes\nreturn redis.call('SET', KEYS[1], 'x')", "1", "k"),
			c("EVAL", "#!lua\nreturn redis.call('GET', KEYS[1])", "1", "k"), c("GET", "k")),
		k("keys", "expire-collections-and-persist", c("RPUSH", "l", "a"), c("EXPIRE", "l", "100"), ttl(1, "TTL", "l"), c("PERSIST", "l"), c("TTL", "l"),
			c("PERSIST", "l"), c("PERSIST", "missing"), c("PEXPIRE", "l", "0"), c("EXISTS", "l"), c("EXPIRE", "missing", "10")),
		k("keys", "object-freq-and-encoding-growth", c("RPUSH", "l", "a"), c("OBJECT", "ENCODING", "l"), c("SADD", "s", "1"), c("SADD", "s", "x"), c("OBJECT", "ENCODING", "s"),
			c("OBJECT", "HELP")),
		k("keys", "dump-restore-collections", c("RPUSH", "l", "a", "b"), c("SADD", "s", "x", "y"), c("ZADD", "z", "1.5", "m"),
			c("EVAL", "for _, k in ipairs(KEYS) do local d = redis.call('DUMP', k); redis.call('DEL', k); redis.call('RESTORE', k, 0, d) end return 1", "3", "l", "s", "z"),
			c("LRANGE", "l", "0", "-1"), srt("SMEMBERS", "s"), c("ZRANGE", "z", "0", "-1", "WITHSCORES"), c("DUMP", "missing")),
		k("keys", "type-and-unlink", c("SET", "a", "1"), c("RPUSH", "b", "x"), c("TYPE", "a"), c("TYPE", "b"), c("TYPE", "missing"), c("UNLINK", "a", "b", "missing"), c("EXISTS", "a", "b")),
	}
}

// encodingAndEdgeCases cover encoding conversions at Redis' default
// thresholds and argument edge cases of the collection commands.
func encodingAndEdgeCases() []Case {
	many := func(prefix string, n int) []string {
		out := make([]string, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, prefix+string(rune('a'+i%26))+string(rune('a'+i/26%26))+string(rune('a'+i/676)))
		}
		return out
	}
	zargs := []string{"ZADD", "z"}
	for i, m := range many("m", 130) {
		zargs = append(zargs, string(rune('0'+i%10)), m)
	}
	hargs := []string{"HSET", "h"}
	for _, f := range many("f", 513) {
		hargs = append(hargs, f, "v")
	}
	return []Case{
		k("encoding", "thresholds", c(append([]string{"RPUSH", "l"}, many("e", 129)...)...), c("OBJECT", "ENCODING", "l"),
			c(append([]string{"SADD", "s"}, many("e", 129)...)...), c("OBJECT", "ENCODING", "s"), c(zargs...), c("OBJECT", "ENCODING", "z"),
			c(hargs...), c("OBJECT", "ENCODING", "h"), c("SET", "big", string(make([]byte, 45))), c("OBJECT", "ENCODING", "big"),
			c("SET", "int", "-9223372036854775808"), c("OBJECT", "ENCODING", "int")),
		k("encoding", "long-element-conversion", c("RPUSH", "l", "a"), c("RPUSH", "l", string(make([]byte, 65))), c("OBJECT", "ENCODING", "l"),
			c("HSET", "h", "f", string(make([]byte, 65))), c("OBJECT", "ENCODING", "h"), c("ZADD", "z", "1", string(make([]byte, 65))), c("OBJECT", "ENCODING", "z"),
			c("SADD", "s", "1", "2", "3"), c("OBJECT", "ENCODING", "s"), c("SADD", "s", "x"), c("OBJECT", "ENCODING", "s")),
		k("lists", "lpos-and-linsert-edges", c("RPUSH", "l", "a", "b", "a", "c", "a"), c("LPOS", "l", "a", "RANK", "-1"), c("LPOS", "l", "a", "RANK", "0"),
			c("LPOS", "l", "a", "COUNT", "0"), c("LPOS", "l", "a", "COUNT", "2", "RANK", "2"), c("LPOS", "l", "a", "MAXLEN", "2", "COUNT", "0"),
			c("LINSERT", "l", "MIDDLE", "a", "x"), c("LINSERT", "l", "BEFORE", "zz", "x"), c("LINSERT", "missing", "BEFORE", "a", "x"),
			c("LSET", "l", "99", "x"), c("LSET", "missing", "0", "x"), c("LRANGE", "l", "-100", "100")),
		k("zsets", "range-and-store-edges", c("ZADD", "z", "1", "a", "2", "b", "3", "c"), c("ZRANGE", "z", "(1", "3", "BYSCORE", "LIMIT", "0", "1"),
			c("ZRANGE", "z", "[a", "(c", "BYLEX"), c("ZRANGE", "z", "a", "c", "BYLEX"), c("ZRANGE", "z", "0", "1", "LIMIT", "0", "1"),
			c("ZRANGESTORE", "dst", "z", "0", "-1"), c("ZRANGESTORE", "dst", "z", "5", "6"), c("EXISTS", "dst"), c("ZINTERCARD", "1", "z", "LIMIT", "2"),
			c("ZADD", "z", "NX", "XX", "1", "a"), c("ZADD", "z", "GT", "LT", "1", "a"), c("ZADD", "z", "INCR", "1", "a", "2", "b"), c("ZADD", "z", "nan", "a"),
			c("ZRANGEBYSCORE", "z", "-inf", "+inf", "LIMIT", "1", "-1"), c("ZREVRANGEBYLEX", "z", "+", "-", "LIMIT", "0", "2")),
		k("sets", "intercard-and-misc-errors", c("SADD", "a", "1", "2"), c("SINTERCARD", "2", "a"), c("SINTERCARD", "1", "a", "LIMIT", "-1"),
			c("SINTERCARD", "x", "a"), c("SRANDMEMBER", "a", "0"), c("SRANDMEMBER", "missing", "5"), c("SPOP", "a", "-1"), c("SMOVE", "a", "a", "1")),
		k("keys", "scan-type-and-patterns", c("MSET", "a*b", "1", "a\\b", "2", "aab", "3"), c("RPUSH", "list", "x"), srt("KEYS", "a\\*b"),
			srt("KEYS", "a[ab]b"), srt("KEYS", "a?b"), srt("KEYS", "[^a]*"), scanAll("SCAN", "0", "TYPE", "list"), scanAll("SCAN", "0", "MATCH", "a*", "TYPE", "string")),
		k("strings", "getex-persist-and-set-get-wrongtype", c("SET", "k", "v", "EX", "100"), c("GETEX", "k", "PERSIST"), c("TTL", "k"),
			c("RPUSH", "l", "x"), c("SET", "l", "v", "GET"), c("SET", "k", "w", "NX", "GET"), c("SET", "k", "w", "XX", "GET"), c("GET", "k")),
	}
}

// adminCases cover the introspection and administration replies client
// libraries parse: ACL, COMMAND, CLIENT and CONFIG.
func adminCases() []Case {
	return []Case{
		k("acl", "setuser-getuser-list", c("ACL", "SETUSER", "u1", "on", ">pw", "~k*", "&ch", "+get", "+set", "-del"),
			c("ACL", "GETUSER", "u1"), c("ACL", "LIST"), c("ACL", "USERS"), c("ACL", "SETUSER", "u1", "reset"), c("ACL", "GETUSER", "u1"),
			c("ACL", "GETUSER", "nouser"), c("ACL", "DELUSER", "u1", "nouser"), c("ACL", "SETUSER", "u2", "bogus"), c("ACL", "USERS")),
		k("acl", "dryrun-cat-whoami", c("ACL", "SETUSER", "d", "on", "nopass", "~a*", "+get"), c("ACL", "DRYRUN", "d", "GET", "a1"),
			c("ACL", "DRYRUN", "d", "GET", "b1"), c("ACL", "DRYRUN", "d", "SET", "a1", "v"), c("ACL", "DRYRUN", "nouser", "GET", "a"),
			c("ACL", "WHOAMI"), srt("ACL", "CAT"), srt("ACL", "CAT", "hyperloglog"), c("ACL", "CAT", "nosuchcat"), c("ACL", "DELUSER", "d")),
		k("command", "info-and-getkeys", c("COMMAND", "INFO", "get", "set", "nosuch"), c("COMMAND", "GETKEYS", "SET", "a", "b"),
			c("COMMAND", "GETKEYS", "MSET", "a", "1", "b", "2"), c("COMMAND", "GETKEYS", "EVAL", "return 1", "2", "x", "y", "z"),
			c("COMMAND", "GETKEYS", "PING"), c("COMMAND", "GETKEYSANDFLAGS", "LMOVE", "a", "b", "LEFT", "RIGHT"), c("COMMAND", "INFO", "client|kill")),
		k("command", "count-and-list-filters", kind("COMMAND", "COUNT"), srt("COMMAND", "LIST", "FILTERBY", "PATTERN", "xa*"),
			srt("COMMAND", "LIST", "FILTERBY", "ACLCAT", "hyperloglog"), c("COMMAND", "DOCS", "get")),
		k("client", "info-reply-no-evict", c("CLIENT", "NO-EVICT", "on"), c("CLIENT", "NO-EVICT", "maybe"), c("CLIENT", "NO-TOUCH", "on"),
			c("CLIENT", "NO-TOUCH", "off"), c("CLIENT", "GETREDIR"), c("CLIENT", "BOGUS"), c("CLIENT", "SETINFO", "LIB-NAME", "x y")),
		k("config", "get-set-errors", c("CONFIG", "GET", "maxmemory-samples"), c("CONFIG", "SET", "maxmemory-samples", "7"), c("CONFIG", "GET", "maxmemory-samples"),
			c("CONFIG", "SET", "maxmemory-samples", "x"), c("CONFIG", "SET", "nosuchparam", "1"), c("CONFIG", "GET", "nosuchparam"),
			c("CONFIG", "SET", "maxmemory-samples", "5"), c("CONFIG", "SET", "notify-keyspace-events", "Kx"), c("CONFIG", "GET", "notify-keyspace-events"),
			c("CONFIG", "SET", "notify-keyspace-events", "")),
		k("config", "set-error-texts", c("CONFIG", "SET", "maxmemory-samples", "0"), c("CONFIG", "SET", "maxmemory", "x"),
			c("CONFIG", "SET", "maxmemory-policy", "bogus"), c("CONFIG", "SET", "maxclients", "0"), c("CONFIG", "SET", "timeout", "-1"),
			c("CONFIG", "SET", "appendonly", "maybe"), c("CONFIG", "SET", "loglevel", "bogus"), c("CONFIG", "SET", "latency-monitor-threshold", "-1"),
			c("CONFIG", "SET", "lazyfree-lazy-eviction", "maybe"), c("CONFIG", "SET", "hash-max-listpack-entries", "x"),
			c("CONFIG", "SET", "databases", "3"), c("CONFIG", "SET", "dir", "/x"), c("CONFIG", "SET", "maxmemory", "1", "maxmemory", "2"),
			c("CONFIG", "SET", "maxmemory-samples", "7", "nosuch", "1"), c("CONFIG", "GET", "maxmemory-samples"), c("CONFIG", "SET", "maxmemory"),
			c("CONFIG", "GET"), c("CONFIG", "RESETSTAT", "x"), c("CONFIG", "BOGUS"), c("CONFIG", "HELP")),
		k("acl", "rule-rendering", c("ACL", "SETUSER", "r1", "on", "nopass", "+@all", "-@dangerous", "+flushdb", "-get", "+config|get", "~x*", "~y*", "&c1"),
			c("ACL", "GETUSER", "r1"), c("ACL", "SETUSER", "r1", "+config", "-config|set", "+@read", "-@read", "+@hash"), c("ACL", "GETUSER", "r1"),
			c("ACL", "SETUSER", "r2", "allkeys", "allchannels", "allcommands", "-@all", "+client|kill", "+client", "+get|x"),
			c("ACL", "SETUSER", "r3", "on", ">p1", ">p2", "<p1", "skip-sanitize-payload", "nocommands", "+@string", "-set"),
			c("ACL", "LIST"), c("ACL", "USERS"), c("ACL", "GETUSER", "r3"), c("ACL", "DELUSER", "r1", "r2", "r3")),
		k("acl", "dryrun-channels-subcommands", c("ACL", "SETUSER", "e", "on", "nopass", "~a*", "&ch1", "+publish", "+subscribe", "+config|get"),
			c("ACL", "DRYRUN", "e", "PUBLISH", "ch1", "x"), c("ACL", "DRYRUN", "e", "PUBLISH", "ch2", "x"), c("ACL", "DRYRUN", "e", "SUBSCRIBE", "ch1", "ch2"),
			c("ACL", "DRYRUN", "e", "CONFIG", "GET", "x"), c("ACL", "DRYRUN", "e", "CONFIG", "SET", "x", "y"), c("ACL", "DRYRUN", "e", "CONFIG", "NOSUCH"),
			c("ACL", "DRYRUN", "e", "NOSUCH"), c("ACL", "DRYRUN", "e", "GET"), c("ACL", "DRYRUN", "e"), c("ACL", "DELUSER", "e")),
		k("command", "getkeys-forms", c("COMMAND", "GETKEYS", "NOSUCH", "a"), c("COMMAND", "GETKEYS", "GET"), c("COMMAND", "GETKEYS"),
			c("COMMAND", "GETKEYS", "CONFIG", "GET", "x"), c("COMMAND", "GETKEYS", "ZUNION", "0"), c("COMMAND", "GETKEYS", "EVAL", "x", "0"),
			c("COMMAND", "GETKEYSANDFLAGS", "SET", "a", "b", "GET"), c("COMMAND", "GETKEYSANDFLAGS", "SET", "a", "b"),
			c("COMMAND", "GETKEYSANDFLAGS", "BITFIELD", "a", "GET", "u8", "0"), c("COMMAND", "GETKEYSANDFLAGS", "SORT", "a", "BY", "w_*", "STORE", "b"),
			c("COMMAND", "GETKEYSANDFLAGS", "XREAD", "STREAMS", "a", "b", "0", "0"), c("COMMAND", "GETKEYSANDFLAGS", "ZUNIONSTORE", "d", "2", "a", "b"),
			c("COMMAND", "GETKEYSANDFLAGS", "MIGRATE", "h", "1", "", "0", "0", "KEYS", "a", "b"), c("COMMAND", "GETKEYSANDFLAGS", "OBJECT", "ENCODING", "k"),
			c("COMMAND", "GETKEYSANDFLAGS", "GEORADIUS", "a", "0", "0", "1", "km", "STORE", "b"), c("COMMAND", "GETKEYS", "LMPOP", "2", "a", "b", "LEFT"),
			c("COMMAND", "GETKEYS", "SPUBLISH", "ch", "msg"), c("COMMAND", "INFO", "config|get", "config|nosuch", "nosuch|x"),
			c("COMMAND", "DOCS", "set", "client|kill", "nosuch"), c("COMMAND", "COUNT", "x"), c("COMMAND", "BOGUS"), c("COMMAND", "HELP")),
		k("admin", "container-subcommand-errors", c("MODULE", "BOGUS"), c("MODULE", "HELP"), c("MODULE", "LIST", "x"),
			c("SLOWLOG", "BOGUS"), c("SLOWLOG", "GET", "1", "2"), c("SLOWLOG", "GET", "x"), c("SLOWLOG", "LEN", "x"), c("SLOWLOG", "HELP"),
			c("MEMORY", "BOGUS"), c("MEMORY", "USAGE"), c("MEMORY", "HELP"), c("MEMORY", "DOCTOR", "x"), c("MEMORY", "USAGE", "k", "BOGUS"),
			c("OBJECT", "BOGUS"), c("OBJECT", "ENCODING"), c("OBJECT", "HELP", "x"), c("ACL", "BOGUS"), c("ACL", "CAT", "a", "b"),
			c("ACL", "LOG", "x", "y"), c("ACL", "GENPASS", "0"), c("ACL", "GENPASS", "x"), c("ACL", "HELP"), c("CLIENT", "HELP"),
			c("CLIENT", "NO-TOUCH", "maybe"), c("CLIENT", "SETINFO", "lib-ver", "a b"), c("CLIENT", "SETINFO", "bogus", "x")),
	}
}
