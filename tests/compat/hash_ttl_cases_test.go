//go:build compat

package compat

import "encoding/hex"

// hashTTLCases cover Redis 7.4's hash field expiration: the HEXPIRE family,
// the HTTL family, HPERSIST, and how field expiry interacts with the other
// hash commands, persistence encodings and transactions.
func hashTTLCases() []Case {
	payload := func(h string) string {
		raw, err := hex.DecodeString(h)
		if err != nil {
			panic(err)
		}
		return string(raw)
	}
	// DUMP payloads Redis 7.4 produced for {a: 70 x "v" (TTL 1893456000000),
	// b: "x"} (RDB_TYPE_HASH_METADATA) and {a: "1" (same TTL), b: "x"}
	// (RDB_TYPE_HASH_LISTPACK_EX).
	metadata := payload("1800b4c5dab8010000020001620178010161c3094046017676e039000176760c00cfec07cb326caa26")
	listpackEx := payload("1900b4c5dab80100001e1e00000006008161020101f400b4c5dab8010000098162028178020001ff0c008872b61dee49f947")
	roundTrip := "local d = redis.call('DUMP', KEYS[1]); redis.call('DEL', KEYS[1]); return redis.call('RESTORE', KEYS[1], 0, d)"
	const far = "4102444800"
	const farMS = "4102444800000"
	return []Case{
		k("hash-ttl", "hexpire-basic", c("HSET", "h", "a", "1", "b", "2", "c", "3"),
			c("HEXPIREAT", "h", far, "FIELDS", "2", "a", "z"), c("HEXPIRETIME", "h", "FIELDS", "3", "a", "b", "z"),
			c("HPEXPIRETIME", "h", "FIELDS", "2", "a", "b"), c("HEXPIRE", "h", "100", "FIELDS", "1", "b"), c("HTTL", "h", "FIELDS", "2", "b", "c")),
		k("hash-ttl", "units", c("HSET", "h", "a", "1", "b", "2", "c", "3", "d", "4"),
			c("HEXPIREAT", "h", far, "FIELDS", "1", "a"), c("HPEXPIREAT", "h", "4102444800123", "FIELDS", "1", "b"),
			c("HEXPIRETIME", "h", "FIELDS", "2", "a", "b"), c("HPEXPIRETIME", "h", "FIELDS", "2", "a", "b"),
			c("HPEXPIRE", "h", "100000", "FIELDS", "1", "c"), c("HTTL", "h", "FIELDS", "1", "c")),
		k("hash-ttl", "conditions", c("HSET", "h", "a", "1", "b", "2"),
			c("HEXPIREAT", "h", "4102444800", "NX", "FIELDS", "1", "a"), c("HEXPIREAT", "h", "4102444900", "NX", "FIELDS", "2", "a", "b"),
			c("HEXPIREAT", "h", "4102444700", "GT", "FIELDS", "2", "a", "b"), c("HEXPIREAT", "h", "4102445000", "GT", "FIELDS", "2", "a", "b"),
			c("HEXPIREAT", "h", "4102445000", "LT", "FIELDS", "2", "a", "b"), c("HEXPIREAT", "h", "4102444600", "LT", "FIELDS", "2", "a", "b"),
			c("HPERSIST", "h", "FIELDS", "1", "b"), c("HEXPIREAT", "h", "4102444500", "xx", "FIELDS", "2", "a", "b"),
			c("HEXPIREAT", "h", "4102444500", "lt", "fields", "1", "b"), c("HEXPIRETIME", "h", "FIELDS", "2", "a", "b")),
		k("hash-ttl", "equal-and-duplicate", c("HSET", "h", "a", "1"), c("HEXPIREAT", "h", far, "FIELDS", "2", "a", "a"),
			c("HEXPIREAT", "h", far, "GT", "FIELDS", "1", "a"), c("HEXPIREAT", "h", far, "LT", "FIELDS", "1", "a"), c("HEXPIRETIME", "h", "FIELDS", "2", "a", "a")),
		k("hash-ttl", "past-deletes", c("HSET", "h", "a", "1", "b", "2", "c", "3"),
			c("HEXPIRE", "h", "0", "FIELDS", "1", "a"), c("HPEXPIREAT", "h", "1", "FIELDS", "2", "b", "nope"), c("HGETALL", "h"),
			c("HEXPIREAT", "h", "0", "FIELDS", "1", "c"), c("EXISTS", "h"), c("HTTL", "h", "FIELDS", "1", "c")),
		k("hash-ttl", "past-condition-first", c("HSET", "h", "a", "1"), c("HEXPIRE", "h", "0", "XX", "FIELDS", "1", "a"),
			c("HEXPIRE", "h", "0", "GT", "FIELDS", "1", "a"), c("HGET", "h", "a")),
		k("hash-ttl", "missing-key", c("HEXPIRE", "nokey", "10", "FIELDS", "2", "a", "b"), c("HTTL", "nokey", "FIELDS", "1", "a"),
			c("HPTTL", "nokey", "FIELDS", "1", "a"), c("HEXPIRETIME", "nokey", "FIELDS", "1", "a"), c("HPERSIST", "nokey", "FIELDS", "2", "a", "b"),
			c("HEXPIRE", "nokey", "10", "NX", "FIELDS", "1", "a"), c("EXISTS", "nokey")),
		k("hash-ttl", "wrongtype", c("SET", "s", "v"), c("HEXPIRE", "s", "10", "FIELDS", "1", "a"), c("HEXPIRE", "s", "abc", "FIELDS", "1", "a"),
			c("HEXPIRE", "s", "10", "FOO", "1", "a"), c("HTTL", "s", "FIELDS", "1", "a"), c("HPTTL", "s", "FIELDS", "x", "a"),
			c("HPERSIST", "s", "FIELDS", "1", "a"), c("HPEXPIRETIME", "s", "FOO", "1", "a")),
		k("hash-ttl", "numfields-errors", c("HSET", "h", "a", "1", "b", "2"),
			c("HEXPIRE", "h", "100", "FIELDS", "0", "a"), c("HEXPIRE", "h", "100", "FIELDS", "-1", "a"), c("HEXPIRE", "h", "100", "FIELDS", "x", "a"),
			c("HEXPIRE", "h", "100", "FIELDS", "99999999999999999999", "a"), c("HEXPIRE", "h", "100", "FIELDS", "3", "a"),
			c("HEXPIRE", "h", "100", "FIELDS", "1", "a", "b"), c("HEXPIRE", "h", "100", "FIELDS", "9223372036854775807", "a"),
			c("HTTL", "h", "FIELDS", "0", "a"), c("HTTL", "h", "FIELDS", "abc", "a"), c("HTTL", "h", "FIELDS", "2", "a"),
			c("HTTL", "h", "FIELDS", "1", "a", "b"), c("HPERSIST", "h", "FIELDS", "0", "a"), c("HPERSIST", "h", "FIELDS", "3", "a", "b"),
			c("HEXPIRE", "nokey", "10", "FIELDS", "0", "a"), c("HTTL", "nokey", "FIELDS", "2", "a"), c("HPERSIST", "nokey", "FIELDS", "x", "a")),
		k("hash-ttl", "fields-keyword-errors", c("HSET", "h", "a", "1"),
			c("HEXPIRE", "h", "100", "FOO", "1", "a"), c("HEXPIRE", "h", "100", "NX", "1", "a"), c("HEXPIRE", "h", "100", "NX", "XX", "FIELDS", "1", "a"),
			c("HEXPIRE", "h", "100", "XX", "GT", "FIELDS", "1", "a"), c("HTTL", "h", "FOO", "1", "a"), c("HPERSIST", "h", "FOO", "1", "a"),
			c("HEXPIRE", "nokey", "10", "FOO", "1", "a"), c("HPERSIST", "nokey", "FOO", "1", "a")),
		k("hash-ttl", "time-errors", c("HSET", "h", "a", "1"),
			c("HEXPIRE", "h", "abc", "FIELDS", "1", "a"), c("HEXPIRE", "h", "-1", "FIELDS", "1", "a"), c("HPEXPIREAT", "h", "-5", "FIELDS", "1", "a"),
			c("HEXPIRE", "h", "9223372036854775807", "FIELDS", "1", "a"), c("HPEXPIRE", "h", "9223372036854775807", "FIELDS", "1", "a"),
			c("HEXPIREAT", "h", "70368744178", "FIELDS", "1", "a"), c("HEXPIREAT", "h", "70368744177", "FIELDS", "1", "a"),
			c("HPEXPIREAT", "h", "70368744177664", "FIELDS", "1", "a"), c("HPEXPIREAT", "h", "70368744177663", "FIELDS", "1", "a"),
			c("HPEXPIRE", "h", "70368744177663", "FIELDS", "1", "a"), c("HEXPIRE", "h", "1.5", "FIELDS", "1", "a"),
			c("HEXPIRE", "nokey", "abc", "FIELDS", "1", "a"), c("HEXPIRE", "nokey", "-1", "FIELDS", "1", "a"), c("HPEXPIRETIME", "h", "FIELDS", "1", "a")),
		k("hash-ttl", "arity", c("HEXPIRE", "h", "10", "FIELDS", "1"), c("HEXPIRE", "h", "10", "FIELDS"), c("HTTL", "h", "FIELDS", "1"),
			c("HPERSIST", "h", "FIELDS"), c("HPEXPIREAT", "h"), c("HPTTL", "h", "FIELDS")),
		k("hash-ttl", "hpersist", c("HSET", "h", "a", "1", "b", "2"), c("HEXPIREAT", "h", far, "FIELDS", "1", "a"),
			c("HPERSIST", "h", "FIELDS", "3", "a", "b", "z"), c("HPERSIST", "h", "FIELDS", "1", "a"), c("HTTL", "h", "FIELDS", "2", "a", "b")),
		k("hash-ttl", "writes-and-ttl", c("HSET", "h", "set", "1", "mset", "1", "incr", "1", "fincr", "1.5", "nx", "1", "del", "1"),
			c("HPEXPIREAT", "h", farMS, "FIELDS", "6", "set", "mset", "incr", "fincr", "nx", "del"),
			c("HSET", "h", "set", "2"), c("HMSET", "h", "mset", "2"), c("HINCRBY", "h", "incr", "5"), c("HINCRBYFLOAT", "h", "fincr", "1"),
			c("HSETNX", "h", "nx", "2"), c("HDEL", "h", "del"), c("HSET", "h", "del", "3"),
			c("HPEXPIRETIME", "h", "FIELDS", "6", "set", "mset", "incr", "fincr", "nx", "del"), pairs("HGETALL", "h")),
		k("hash-ttl", "lazy-expiry-reads", c("HSET", "h", "a", "1", "b", "2", "c", "3"), c("HPEXPIRE", "h", "50", "FIELDS", "2", "a", "b"),
			sleep(300), c("HGET", "h", "a"), c("HMGET", "h", "a", "b", "c"), c("HEXISTS", "h", "b"), c("HSTRLEN", "h", "a"), c("HLEN", "h"),
			pairs("HGETALL", "h"), srt("HKEYS", "h"), srt("HVALS", "h"), c("HRANDFIELD", "h"), c("HRANDFIELD", "h", "5", "WITHVALUES"),
			scanAllPairs("HSCAN", "h", "0"), c("HTTL", "h", "FIELDS", "3", "a", "b", "c"), c("HPERSIST", "h", "FIELDS", "1", "a")),
		k("hash-ttl", "last-field-expiry-deletes-key", c("HSET", "h", "a", "1", "b", "2"), c("HPEXPIRE", "h", "50", "FIELDS", "2", "a", "b"),
			sleep(300), c("EXISTS", "h"), c("TYPE", "h"), c("HLEN", "h"), c("HGETALL", "h"), c("HTTL", "h", "FIELDS", "1", "a")),
		k("hash-ttl", "expired-field-writes", c("HSET", "h", "a", "1", "b", "2", "keep", "x"),
			c("HPEXPIRE", "h", "50", "FIELDS", "2", "a", "b"), sleep(300), c("HSETNX", "h", "a", "new"), c("HINCRBY", "h", "b", "5"),
			c("HTTL", "h", "FIELDS", "2", "a", "b"), pairs("HGETALL", "h")),
		k("hash-ttl", "object-encoding", c("HSET", "h", "a", "1"), c("OBJECT", "ENCODING", "h"), c("HEXPIREAT", "h", far, "FIELDS", "1", "a"),
			c("OBJECT", "ENCODING", "h"), c("HPERSIST", "h", "FIELDS", "1", "a"), c("OBJECT", "ENCODING", "h")),
		k("hash-ttl", "restore-redis-payloads", c("RESTORE", "m", "0", metadata), c("RESTORE", "l", "0", listpackEx),
			c("HPEXPIRETIME", "m", "FIELDS", "2", "a", "b"), c("HPEXPIRETIME", "l", "FIELDS", "2", "a", "b"), pairs("HGETALL", "m"), pairs("HGETALL", "l")),
		k("hash-ttl", "dump-restore-roundtrip", c("HSET", "h", "a", "1", "b", "2", "c", "3"),
			c("HPEXPIREAT", "h", farMS, "FIELDS", "1", "a"), c("HPEXPIREAT", "h", "4102444900000", "FIELDS", "1", "b"),
			c("EVAL", roundTrip, "1", "h"), c("HPEXPIRETIME", "h", "FIELDS", "3", "a", "b", "c"), pairs("HGETALL", "h")),
		k("hash-ttl", "rename-copy-keep-field-ttls", c("HSET", "h", "a", "1", "b", "2"), c("HPEXPIREAT", "h", farMS, "FIELDS", "1", "a"),
			c("RENAME", "h", "h2"), c("HPEXPIRETIME", "h2", "FIELDS", "2", "a", "b"), c("COPY", "h2", "h3"), c("HPEXPIRETIME", "h3", "FIELDS", "2", "a", "b"),
			c("HPERSIST", "h3", "FIELDS", "1", "a"), c("HPEXPIRETIME", "h2", "FIELDS", "1", "a")),
		k("hash-ttl", "key-ttl-independent", c("HSET", "h", "a", "1"), c("EXPIREAT", "h", "4102444900"), c("HEXPIREAT", "h", far, "FIELDS", "1", "a"),
			c("EXPIRETIME", "h"), c("PERSIST", "h"), c("HEXPIRETIME", "h", "FIELDS", "1", "a"), c("TTL", "h")),
		k("hash-ttl", "multi-exec", c("HSET", "h", "a", "1", "b", "2"), c("MULTI"), c("HEXPIREAT", "h", far, "FIELDS", "1", "a"),
			c("HTTL", "h", "FIELDS", "1", "zz"), c("HPERSIST", "h", "FIELDS", "1", "b"), c("HEXPIRE", "h", "0", "FIELDS", "1", "b"), c("EXEC"),
			c("HEXPIRETIME", "h", "FIELDS", "2", "a", "b")),
		k("hash-ttl", "watch-hexpire-aborts", c("HSET", "h", "a", "1"), on(1, "WATCH", "h"), c("HEXPIREAT", "h", far, "FIELDS", "1", "a"),
			on(1, "MULTI"), on(1, "HGET", "h", "a"), on(1, "EXEC")),
		k("hash-ttl", "watch-noop-keeps-transaction", c("HSET", "h", "a", "1"), c("HEXPIREAT", "h", far, "FIELDS", "1", "a"), on(1, "WATCH", "h"),
			c("HEXPIREAT", "h", far, "NX", "FIELDS", "2", "a", "zz"), c("HPERSIST", "h", "FIELDS", "1", "zz"), c("HTTL", "h", "FIELDS", "1", "a"),
			on(1, "MULTI"), on(1, "HGET", "h", "a"), on(1, "EXEC")),
		k("hash-ttl", "watch-hpersist-aborts", c("HSET", "h", "a", "1"), c("HEXPIREAT", "h", far, "FIELDS", "1", "a"), on(1, "WATCH", "h"),
			c("HPERSIST", "h", "FIELDS", "1", "a"), on(1, "MULTI"), on(1, "HGET", "h", "a"), on(1, "EXEC")),
		k("hash-ttl", "lua-hexpire", c("HSET", "h", "a", "1"),
			c("EVAL", "return {redis.call('HPEXPIREAT', KEYS[1], ARGV[1], 'FIELDS', 1, 'a'), redis.call('HPEXPIRETIME', KEYS[1], 'FIELDS', 2, 'a', 'b')}", "1", "h", farMS)),
		k("hash-ttl", "hrandfield-after-partial-expiry", c("HSET", "h", "gone", "1", "kept", "2"), c("HPEXPIRE", "h", "50", "FIELDS", "1", "gone"),
			sleep(300), c("HRANDFIELD", "h", "-3"), c("HRANDFIELD", "h", "1"), c("HLEN", "h")),
	}
}
