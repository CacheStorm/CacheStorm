//go:build compat

package compat

import (
	"fmt"
	"time"
)

func k(group, name string, steps ...Step) Case {
	return Case{Group: group, Name: name, Steps: steps}
}

func corpus() []Case {
	var cs []Case
	cs = append(cs, stringCases()...)
	cs = append(cs, msetBatchCases()...)
	cs = append(cs, keyCases()...)
	cs = append(cs, restoreCases()...)
	cs = append(cs, bitmapMutationCases()...)
	cs = append(cs, hllCases()...)
	cs = append(cs, geoAddValidationCases()...)
	cs = append(cs, geoZsetCases()...)
	cs = append(cs, entryTransferCases()...)
	cs = append(cs, hashCases()...)
	cs = append(cs, hashGrowthCases()...)
	cs = append(cs, hashTTLCases()...)
	cs = append(cs, listCases()...)
	cs = append(cs, blockingCases()...)
	cs = append(cs, setCases()...)
	cs = append(cs, setAdmissionCases()...)
	cs = append(cs, readIsolationCases()...)
	cs = append(cs, zsetCases()...)
	cs = append(cs, streamCases()...)
	cs = append(cs, txCases()...)
	cs = append(cs, wrongTypeCases()...)
	cs = append(cs, arityCases()...)
	cs = append(cs, pubsubCases()...)
	cs = append(cs, dbCases()...)
	cs = append(cs, miscCases()...)
	cs = append(cs, luaCases()...)
	multi := map[string]bool{"transactions/multi-select": true, "transactions/watch-other-db-no-abort": true, "lua/lua-select-isolated": true}
	for i := range cs {
		if cs[i].Group == "db" || multi[cs[i].Group+"/"+cs[i].Name] {
			cs[i].MultiDB = true
		}
	}
	return cs
}

func stringCases() []Case {
	g := "strings"
	return []Case{
		k(g, "set-get", c("SET", "k", "v"), c("GET", "k"), c("GET", "missing")),
		k(g, "set-nx-xx", c("SET", "k", "v", "NX"), c("SET", "k", "v2", "NX"), c("GET", "k"), c("SET", "m", "v", "XX"), c("SET", "k", "v3", "XX"), c("GET", "k"), c("EXISTS", "m")),
		k(g, "set-get-option", c("SET", "k", "v", "GET"), c("SET", "k", "v2", "GET"), c("GET", "k"), c("RPUSH", "l", "a"), c("SET", "l", "x", "GET"), c("TYPE", "l")),
		k(g, "set-nx-get", c("SET", "k", "v", "NX", "GET"), c("SET", "k", "v2", "NX", "GET"), c("GET", "k")),
		k(g, "set-ex", c("SET", "k", "v", "EX", "100"), ttl(1, "TTL", "k"), ttl(1500, "PTTL", "k")),
		k(g, "set-px", c("SET", "k", "v", "PX", "100000"), ttl(1, "TTL", "k"), ttl(1500, "PTTL", "k")),
		k(g, "set-keepttl", c("SET", "k", "v", "EX", "100"), c("SET", "k", "v2", "KEEPTTL"), ttl(1, "TTL", "k"), c("SET", "k", "v3"), c("TTL", "k")),
		k(g, "set-ex-invalid", c("SET", "k", "v", "EX", "0"), c("SET", "k", "v", "EX", "-1"), c("SET", "k", "v", "EX", "abc"), c("SET", "k", "v", "PX", "0"), c("EXISTS", "k")),
		k(g, "set-syntax-errors", c("SET", "k", "v", "NX", "XX"), c("SET", "k", "v", "EX", "10", "PX", "100"), c("SET", "k", "v", "EX"), c("SET", "k", "v", "BOGUS"), c("SET", "k", "v", "KEEPTTL", "EX", "10"), c("EXISTS", "k")),
		k(g, "set-pxat-max", c("SET", "k", "v", "PXAT", "9223372036854775807"), c("PEXPIRETIME", "k"), c("EXPIRETIME", "k"), ttl(1500, "PTTL", "k"), ttl(1, "TTL", "k"), c("SET", "k", "new", "KEEPTTL"), c("PEXPIRETIME", "k")),
		k(g, "getex-pxat-max", c("SET", "k", "v"), c("GETEX", "k", "PXAT", "9223372036854775807"), c("PEXPIRETIME", "k"), ttl(1500, "PTTL", "k"), c("GETEX", "k", "PERSIST"), c("TTL", "k")),
		k(g, "set-exat-pxat-past", c("SET", "k", "v", "PXAT", "1"), c("GET", "k"), c("SET", "k2", "v", "EXAT", "1"), c("EXISTS", "k2"), c("SET", "k3", "v", "EXAT", "99999999999"), c("TTL", "k3")),
		k(g, "set-ex-overflow", c("SET", "k", "v", "EX", "9223372036854775807"), c("SET", "k", "v", "PX", "9223372036854775807"), c("EXISTS", "k")),
		k(g, "getex", c("SET", "k", "v"), c("GETEX", "k", "EX", "100"), ttl(1, "TTL", "k"), c("GETEX", "k", "PERSIST"), c("TTL", "k"), c("GETEX", "missing"), c("GETEX", "k", "PX", "50000"), ttl(1500, "PTTL", "k"), c("GETEX", "k")),
		k(g, "getex-errors", c("SET", "k", "v"), c("GETEX", "k", "EX", "0"), c("GETEX", "k", "EX", "abc"), c("GETEX", "k", "EX", "10", "PX", "100"), c("GETEX", "k", "BOGUS"), c("RPUSH", "l", "a"), c("GETEX", "l")),
		k(g, "getex-exat-past", c("SET", "k", "v"), c("GETEX", "k", "EXAT", "1"), c("EXISTS", "k")),
		k(g, "getdel", c("SET", "k", "v"), c("GETDEL", "k"), c("GETDEL", "k"), c("EXISTS", "k"), c("RPUSH", "l", "a"), c("GETDEL", "l"), c("EXISTS", "l")),
		k(g, "incr-basic", c("INCR", "n"), c("INCR", "n"), c("INCRBY", "n", "10"), c("INCRBY", "n", "-5"), c("DECR", "n"), c("DECRBY", "n", "3"), c("GET", "n")),
		k(g, "incr-not-int", c("SET", "s", "abc"), c("INCR", "s"), c("SET", "f", "1.5"), c("INCR", "f"), c("SET", "sp", " 1"), c("INCR", "sp"), c("SET", "z", "01"), c("INCR", "z"), c("SET", "e", ""), c("INCR", "e"), c("INCRBY", "n", "abc"), c("INCRBY", "n", "1.0")),
		k(g, "incr-overflow", c("SET", "n", "9223372036854775807"), c("INCR", "n"), c("GET", "n"), c("SET", "m", "-9223372036854775808"), c("DECR", "m"), c("INCRBY", "x", "9223372036854775808"), c("DECRBY", "y", "-9223372036854775808"), c("SET", "p", "1"), c("INCRBY", "p", "9223372036854775807")),
		k(g, "incrbyfloat", c("INCRBYFLOAT", "f", "0.1"), c("INCRBYFLOAT", "f", "0.2"), c("SET", "g", "10.5"), c("INCRBYFLOAT", "g", "0.1"), c("SET", "h", "5.0e3"), c("INCRBYFLOAT", "h", "2.0e2"), c("SET", "i", "3"), c("INCRBYFLOAT", "i", "1.1"), c("INCRBYFLOAT", "i", "-4.1"), c("GET", "i")),
		k(g, "incrbyfloat-errors", c("INCRBYFLOAT", "f", "abc"), c("INCRBYFLOAT", "f", "inf"), c("INCRBYFLOAT", "f", "nan"), c("SET", "s", "abc"), c("INCRBYFLOAT", "s", "1"), c("SET", "big", "1.7e308"), c("INCRBYFLOAT", "big", "1.7e308"), c("GET", "big"), c("EXISTS", "f")),
		k(g, "incrbyfloat-format", c("INCRBYFLOAT", "a", "1e20"), c("INCRBYFLOAT", "b", "0.0001"), c("INCRBYFLOAT", "c", "-0"), c("INCRBYFLOAT", "d", "3.0"), c("INCRBYFLOAT", "e", "1.23456789012345678"), c("INCRBYFLOAT", "ff", "1e-20")),
		k(g, "append-strlen", c("APPEND", "k", "hello"), c("APPEND", "k", " world"), c("GET", "k"), c("STRLEN", "k"), c("STRLEN", "missing"), c("SET", "n", "10"), c("APPEND", "n", "5"), c("INCR", "n")),
		k(g, "setrange", c("SETRANGE", "k", "5", "hi"), c("GET", "k"), c("SET", "s", "Hello World"), c("SETRANGE", "s", "6", "Redis"), c("GET", "s"), c("SETRANGE", "s", "-1", "x"), c("SETRANGE", "e", "0", ""), c("EXISTS", "e"), c("SETRANGE", "e", "10", ""), c("EXISTS", "e"), c("SETRANGE", "s", "536870912", "x")),
		k(g, "getrange", c("SET", "k", "This is a string"), c("GETRANGE", "k", "0", "3"), c("GETRANGE", "k", "-3", "-1"), c("GETRANGE", "k", "0", "-1"), c("GETRANGE", "k", "10", "100"), c("GETRANGE", "k", "5", "2"), c("GETRANGE", "k", "-100", "2"), c("GETRANGE", "missing", "0", "-1"), c("GETRANGE", "k", "a", "b"), c("SUBSTR", "k", "0", "3")),
		k(g, "mset-mget", c("MSET", "a", "1", "b", "2"), c("MGET", "a", "b", "c"), c("MSET", "a"), c("MSET", "a", "1", "b"), c("RPUSH", "l", "x"), c("MGET", "a", "l")),
		k(g, "msetnx", c("MSETNX", "a", "1", "b", "2"), c("MSETNX", "b", "3", "c", "4"), c("MGET", "a", "b", "c"), c("MSETNX", "a", "1", "a", "2"), c("MSETNX", "d", "1", "d", "2"), c("GET", "d")),
		k(g, "binary-values", c("SET", "bin", "\x00\xff\r\n\x01abc"), c("GET", "bin"), c("STRLEN", "bin"), c("APPEND", "bin", "\x00"), c("GETRANGE", "bin", "0", "2"), c("SET", "k\x00\r\nkey", "v"), c("GET", "k\x00\r\nkey")),
		k(g, "utf8-values", c("SET", "u", "héllo 日本 🎉"), c("GET", "u"), c("STRLEN", "u"), c("GETRANGE", "u", "0", "1"), c("SET", "ключ", "значение"), c("GET", "ключ")),
		k(g, "large-value", c("SET", "big", string(make([]byte, 100000))), c("STRLEN", "big"), c("GETRANGE", "big", "99990", "-1")),
		k(g, "setnx-setex-psetex-getset", c("SETNX", "a", "1"), c("SETNX", "a", "2"), c("GET", "a"), c("SETEX", "b", "100", "v"), ttl(1, "TTL", "b"), c("SETEX", "b", "0", "v"), c("PSETEX", "c", "100000", "v"), ttl(1500, "PTTL", "c"), c("GETSET", "a", "9"), c("GETSET", "zz", "1"), c("GET", "a")),
		k(g, "lcs", c("MSET", "k1", "ohmytext", "k2", "mynewtext"), c("LCS", "k1", "k2"), c("LCS", "k1", "k2", "LEN")),
	}
}

func keyCases() []Case {
	g := "keys"
	return []Case{
		k(g, "del-multi", c("MSET", "a", "1", "b", "2"), c("RPUSH", "l", "x"), c("DEL", "a", "b", "l", "c"), c("DEL", "a"), c("DBSIZE")),
		k(g, "exists-multi", c("MSET", "a", "1", "b", "2"), c("EXISTS", "a", "a", "b", "c"), c("EXISTS", "c"), c("UNLINK", "a", "zz"), c("EXISTS", "a", "b")),
		k(g, "expire-basic", c("SET", "k", "v"), c("EXPIRE", "k", "100"), ttl(1, "TTL", "k"), c("EXPIRE", "missing", "100"), c("PERSIST", "k"), c("PERSIST", "k"), c("TTL", "k")),
		k(g, "expire-nx-xx", c("SET", "k", "v"), c("EXPIRE", "k", "100", "XX"), c("EXPIRE", "k", "100", "NX"), c("EXPIRE", "k", "200", "NX"), ttl(1, "TTL", "k"), c("EXPIRE", "k", "300", "XX"), ttl(1, "TTL", "k")),
		k(g, "expire-gt-lt", c("SET", "k", "v"), c("EXPIRE", "k", "100", "GT"), c("TTL", "k"), c("EXPIRE", "k", "100", "LT"), ttl(1, "TTL", "k"), c("EXPIRE", "k", "50", "GT"), c("EXPIRE", "k", "200", "GT"), ttl(1, "TTL", "k"), c("EXPIRE", "k", "300", "LT"), c("EXPIRE", "k", "10", "LT"), ttl(1, "TTL", "k")),
		k(g, "expire-option-conflicts", c("SET", "k", "v"), c("EXPIRE", "k", "100", "NX", "XX"), c("EXPIRE", "k", "100", "GT", "LT"), c("EXPIRE", "k", "100", "NX", "GT"), c("EXPIRE", "k", "100", "BOGUS"), c("EXPIRE", "k", "abc"), c("TTL", "k")),
		k(g, "expire-negative-deletes", c("SET", "k", "v"), c("EXPIRE", "k", "-1"), c("EXISTS", "k"), c("SET", "k2", "v"), c("PEXPIRE", "k2", "0"), c("EXISTS", "k2"), c("SET", "k3", "v"), c("EXPIREAT", "k3", "1"), c("EXISTS", "k3")),
		k(g, "expire-actually-expires", c("SET", "k", "v"), c("PEXPIRE", "k", "50"), sleep(150), c("GET", "k"), c("EXISTS", "k"), c("TTL", "k"), c("DBSIZE")),
		k(g, "ttl-missing-nottl", c("SET", "k", "v"), c("TTL", "k"), c("PTTL", "k"), c("TTL", "missing"), c("PTTL", "missing"), c("EXPIRETIME", "k"), c("EXPIRETIME", "missing"), c("PEXPIRETIME", "missing")),
		k(g, "expiretime-rounding", c("SET", "k", "v", "PXAT", "4102444800499"), c("EXPIRETIME", "k"), c("PEXPIREAT", "k", "4102444800500"), c("EXPIRETIME", "k"), c("PEXPIREAT", "k", "4102444800999"), c("EXPIRETIME", "k")),
		k(g, "expiretime-max", c("SET", "k", "v"), c("PEXPIREAT", "k", "9223372036854775807"), c("PEXPIRETIME", "k"), c("EXPIRETIME", "k"), ttl(1500, "PTTL", "k"), ttl(1, "TTL", "k")),
		k(g, "expire-gt-lt-far-future", c("SET", "k", "v", "PXAT", "16725225600000"), c("PEXPIREAT", "k", "16725225600001", "GT"), c("PEXPIREAT", "k", "16725225600000", "GT"), c("PEXPIREAT", "k", "16725225600000", "LT"), c("PEXPIRETIME", "k"), c("COPY", "k", "copy"), c("PEXPIRETIME", "copy")),
		k(g, "expire-relative-far-future", c("SET", "k", "v"), c("EXPIRE", "k", "99999999999999"), ttl(1, "TTL", "k"), c("PEXPIRE", "k", "9000000000000000000"), ttl(1500, "PTTL", "k"), ttl(1, "TTL", "k")),
		k(g, "expiretime", c("SET", "k", "v"), c("EXPIREAT", "k", "33177117420"), c("EXPIRETIME", "k"), c("PEXPIRETIME", "k"), c("PEXPIREAT", "k", "33177117420000"), c("TTL", "k")),
		k(g, "rename", c("SET", "a", "1"), c("RENAME", "a", "b"), c("GET", "a"), c("GET", "b"), c("RENAME", "missing", "x"), c("RENAME", "b", "b"), c("GET", "b"), c("SET", "c", "3"), c("RENAME", "b", "c"), c("GET", "c")),
		k(g, "rename-keeps-ttl", c("SET", "a", "1", "EX", "100"), c("RENAME", "a", "b"), ttl(1, "TTL", "b"), c("SET", "c", "1", "EX", "100"), c("SET", "d", "2"), c("RENAME", "d", "c"), c("TTL", "c")),
		k(g, "renamenx", c("MSET", "a", "1", "b", "2"), c("RENAMENX", "a", "b"), c("RENAMENX", "a", "c"), c("RENAMENX", "missing", "z"), c("MGET", "a", "b", "c")),
		k(g, "type", c("SET", "s", "v"), c("RPUSH", "l", "a"), c("HSET", "h", "f", "v"), c("SADD", "st", "m"), c("ZADD", "z", "1", "m"), c("XADD", "x", "1-1", "f", "v"), c("TYPE", "s"), c("TYPE", "l"), c("TYPE", "h"), c("TYPE", "st"), c("TYPE", "z"), c("TYPE", "x"), c("TYPE", "missing")),
		ks(g, "scan-match-count", scanAll("SCAN", "0", "MATCH", "user:*", "COUNT", "5"), scanAll("SCAN", "0", "COUNT", "3"), scanAll("SCAN", "0", "MATCH", "nomatch*")),
		ks(g, "scan-type", scanAll("SCAN", "0", "TYPE", "hash"), scanAll("SCAN", "0", "TYPE", "string", "MATCH", "user:1*"), scanAll("SCAN", "0", "TYPE", "list"), scanAll("SCAN", "0", "TYPE", "bogus")),
		k(g, "scan-errors", c("SCAN", "abc"), c("SCAN", "0", "COUNT", "0"), c("SCAN", "0", "COUNT", "abc"), c("SCAN", "0", "MATCH"), c("SCAN", "0", "BOGUS", "1")),
		k(g, "scan-empty", c("SCAN", "0")),
		ks(g, "keys-pattern", srt("KEYS", "*"), srt("KEYS", "user:?"), srt("KEYS", "user:[1-3]*"), srt("KEYS", "h*"), srt("KEYS", "user:\\*")),
		k(g, "randomkey", c("RANDOMKEY"), c("SET", "only", "1"), c("RANDOMKEY")),
		k(g, "copy", c("SET", "a", "1"), c("COPY", "a", "b"), c("COPY", "a", "b"), c("COPY", "a", "b", "REPLACE"), c("RPUSH", "l", "x", "y"), c("COPY", "l", "l2"), c("LRANGE", "l2", "0", "-1"), c("COPY", "missing", "z")),
		k(g, "touch", c("MSET", "a", "1", "b", "2"), c("TOUCH", "a", "b", "c")),
		k(g, "del-then-type", c("RPUSH", "k", "a"), c("DEL", "k"), c("SET", "k", "x"), c("TYPE", "k"), c("GET", "k")),
	}
}

func ks(group, name string, steps ...Step) Case {
	seed := []Step{c(rangeArgs([]string{"MSET"}, 1, 12, "user:%d", "%d")...)}
	for i := 1; i <= 6; i++ {
		n := fmt.Sprint(i)
		seed = append(seed, c("HSET", "hash:"+n, "f", n), c("RPUSH", "list:"+n, n), c("SADD", "set:"+n, n))
	}
	return k(group, name, append(seed, steps...)...)
}

func rangeArgs(prefix []string, from, to int, formats ...string) []string {
	out := append([]string(nil), prefix...)
	for i := from; i <= to; i++ {
		for _, f := range formats {
			out = append(out, fmt.Sprintf(f, i))
		}
	}
	return out
}

func noState(c Case) Case {
	c.NoState = true
	return c
}

func hashCases() []Case {
	g := "hashes"
	return []Case{
		k(g, "hset-hget", c("HSET", "h", "a", "1", "b", "2"), c("HSET", "h", "a", "3", "c", "4"), c("HGET", "h", "a"), c("HGET", "h", "zz"), c("HGET", "missing", "a"), c("HLEN", "h"), c("HMGET", "h", "a", "zz", "c"), c("HMGET", "missing", "a")),
		k(g, "hgetall-keys-vals", c("HSET", "h", "a", "1", "b", "2", "c", "3"), pairs("HGETALL", "h"), srt("HKEYS", "h"), srt("HVALS", "h"), c("HGETALL", "missing"), c("HKEYS", "missing")),
		k(g, "hdel-removes-key", c("HSET", "h", "a", "1", "b", "2"), c("HDEL", "h", "a", "zz"), c("HDEL", "h", "b", "b"), c("EXISTS", "h"), c("HDEL", "h", "a")),
		k(g, "hexists-hstrlen", c("HSET", "h", "a", "hello"), c("HEXISTS", "h", "a"), c("HEXISTS", "h", "b"), c("HSTRLEN", "h", "a"), c("HSTRLEN", "h", "b"), c("HSTRLEN", "missing", "a")),
		k(g, "hsetnx", c("HSETNX", "h", "a", "1"), c("HSETNX", "h", "a", "2"), c("HGET", "h", "a")),
		k(g, "hincrby", c("HINCRBY", "h", "n", "5"), c("HINCRBY", "h", "n", "-10"), c("HSET", "h", "s", "abc"), c("HINCRBY", "h", "s", "1"), c("HSET", "h", "big", "9223372036854775807"), c("HINCRBY", "h", "big", "1"), c("HINCRBY", "h", "n", "abc"), pairs("HGETALL", "h")),
		k(g, "hincrbyfloat", c("HINCRBYFLOAT", "h", "f", "1.5"), c("HINCRBYFLOAT", "h", "f", "0.1"), c("HSET", "h", "i", "3"), c("HINCRBYFLOAT", "h", "i", "2.0"), c("HINCRBYFLOAT", "h", "f", "abc"), c("HINCRBYFLOAT", "h", "f", "inf"), pairs("HGETALL", "h")),
		k(g, "hmset", c("HMSET", "h", "a", "1", "b", "2"), pairs("HGETALL", "h"), c("HMSET", "h", "a")),
		k(g, "hset-odd-args", c("HSET", "h", "a", "1", "b"), c("HSET", "h", "a"), c("EXISTS", "h")),
		k(g, "hrandfield", c("HSET", "h", "a", "1", "b", "2", "c", "3"), length("HRANDFIELD", "h", "2"), length("HRANDFIELD", "h", "-5"), length("HRANDFIELD", "h", "10"), length("HRANDFIELD", "h", "2", "WITHVALUES"), c("HRANDFIELD", "missing"), c("HRANDFIELD", "missing", "2"), c("HRANDFIELD", "h", "0")),
		k(g, "hscan", c(rangeArgs([]string{"HSET", "h"}, 1, 50, "f%d", "%d")...), scanAllPairs("HSCAN", "h", "0", "COUNT", "7"), scanAllPairs("HSCAN", "h", "0", "MATCH", "f1*"), c("HSCAN", "missing", "0")),
	}
}

func listCases() []Case {
	g := "lists"
	return []Case{
		k(g, "push-range", c("RPUSH", "l", "a", "b", "c"), c("LPUSH", "l", "z", "y"), c("LRANGE", "l", "0", "-1"), c("LRANGE", "l", "1", "2"), c("LRANGE", "l", "-2", "-1"), c("LRANGE", "l", "5", "10"), c("LRANGE", "l", "-100", "100"), c("LRANGE", "missing", "0", "-1"), c("LLEN", "l"), c("LLEN", "missing")),
		k(g, "lindex-lset", c("RPUSH", "l", "a", "b", "c"), c("LINDEX", "l", "0"), c("LINDEX", "l", "-1"), c("LINDEX", "l", "10"), c("LSET", "l", "1", "B"), c("LSET", "l", "10", "x"), c("LSET", "missing", "0", "x"), c("LSET", "l", "-1", "C"), c("LRANGE", "l", "0", "-1")),
		k(g, "linsert", c("RPUSH", "l", "a", "c"), c("LINSERT", "l", "BEFORE", "c", "b"), c("LINSERT", "l", "AFTER", "c", "d"), c("LINSERT", "l", "AFTER", "zz", "x"), c("LINSERT", "missing", "AFTER", "a", "x"), c("LINSERT", "l", "MIDDLE", "a", "x"), c("LRANGE", "l", "0", "-1")),
		k(g, "lrem", c("RPUSH", "l", "a", "b", "a", "c", "a", "b"), c("LREM", "l", "2", "a"), c("LRANGE", "l", "0", "-1"), c("LREM", "l", "-1", "b"), c("LRANGE", "l", "0", "-1"), c("LREM", "l", "0", "a"), c("LRANGE", "l", "0", "-1"), c("LREM", "missing", "0", "a")),
		k(g, "ltrim", c("RPUSH", "l", "a", "b", "c", "d", "e"), c("LTRIM", "l", "1", "-2"), c("LRANGE", "l", "0", "-1"), c("LTRIM", "l", "5", "10"), c("EXISTS", "l")),
		k(g, "pop-count", c("RPUSH", "l", "a", "b", "c", "d"), c("LPOP", "l"), c("LPOP", "l", "2"), c("RPOP", "l", "5"), c("EXISTS", "l"), c("LPOP", "l"), c("LPOP", "l", "2"), c("RPOP", "missing", "1"), c("RPUSH", "m", "x"), c("LPOP", "m", "0"), c("LPOP", "m", "-1")),
		k(g, "pushx", c("LPUSHX", "l", "a"), c("RPUSHX", "l", "a"), c("EXISTS", "l"), c("RPUSH", "l", "a"), c("LPUSHX", "l", "b", "c"), c("RPUSHX", "l", "d"), c("LRANGE", "l", "0", "-1")),
		k(g, "lmove-rpoplpush", c("RPUSH", "a", "1", "2", "3"), c("LMOVE", "a", "b", "LEFT", "RIGHT"), c("LMOVE", "a", "b", "RIGHT", "LEFT"), c("RPOPLPUSH", "a", "b"), c("LRANGE", "b", "0", "-1"), c("EXISTS", "a"), c("LMOVE", "a", "b", "LEFT", "LEFT"), c("RPUSH", "r", "x", "y"), c("LMOVE", "r", "r", "LEFT", "RIGHT"), c("LRANGE", "r", "0", "-1"), c("LMOVE", "r", "b", "UP", "LEFT")),
		k(g, "lpos", c("RPUSH", "l", "a", "b", "c", "1", "2", "3", "c", "c"), c("LPOS", "l", "c"), c("LPOS", "l", "c", "RANK", "2"), c("LPOS", "l", "c", "RANK", "-1"), c("LPOS", "l", "c", "COUNT", "0"), c("LPOS", "l", "c", "COUNT", "2"), c("LPOS", "l", "c", "RANK", "-1", "COUNT", "2"), c("LPOS", "l", "c", "COUNT", "0", "MAXLEN", "7"), c("LPOS", "l", "zz"), c("LPOS", "l", "zz", "COUNT", "0"), c("LPOS", "missing", "a"), c("LPOS", "l", "c", "RANK", "0"), c("LPOS", "l", "c", "COUNT", "-1"), c("LPOS", "l", "c", "MAXLEN", "-1"), c("LPOS", "l", "c", "RANK", "-9223372036854775808")),
		k(g, "lmpop", c("RPUSH", "b", "1", "2", "3"), c("LMPOP", "2", "a", "b", "LEFT"), c("LMPOP", "2", "a", "b", "RIGHT", "COUNT", "5"), c("LMPOP", "2", "a", "b", "LEFT"), c("EXISTS", "b")),
		k(g, "lmpop-errors", c("LMPOP", "0", "a", "LEFT"), c("LMPOP", "2", "a", "LEFT"), c("LMPOP", "1", "a", "MIDDLE"), c("LMPOP", "1", "a", "LEFT", "COUNT", "0"), c("LMPOP", "abc", "a", "LEFT"), c("LMPOP", "1", "a", "LEFT", "COUNT")),
		k(g, "big-list-order", c(rangeArgs([]string{"RPUSH", "l"}, 1, 1000, "%d")...), c("LINDEX", "l", "500"), c("LRANGE", "l", "995", "-1"), c("LTRIM", "l", "100", "199"), c("LLEN", "l"), c("LINDEX", "l", "0")),
	}
}

func blockingCases() []Case {
	g := "blocking"
	t0 := func(cmd ...string) Step { s := send(0, cmd...); return s }
	return []Case{
		k(g, "blpop-present", c("RPUSH", "b", "1", "2"), c("BLPOP", "a", "b", "0"), c("BLPOP", "b", "0.5"), c("EXISTS", "b")),
		k(g, "blpop-absent-fractional", c("BLPOP", "a", "0.1"), c("BRPOP", "a", "b", "0.1"), c("EXISTS", "a")),
		k(g, "blpop-timeout0-wakeup", t0("BLPOP", "q", "0"), sleep(200), on(1, "RPUSH", "q", "x", "y"), recv(0), c("LRANGE", "q", "0", "-1")),
		k(g, "brpop-timeout0-wakeup", t0("BRPOP", "q1", "q2", "0"), sleep(200), on(1, "LPUSH", "q2", "a", "b"), recv(0), c("LRANGE", "q2", "0", "-1")),
		k(g, "blpop-fractional-wakeup", t0("BLPOP", "q", "1.5"), sleep(200), on(1, "RPUSH", "q", "x"), recv(0), c("EXISTS", "q")),
		k(g, "blpop-timeout-errors", c("BLPOP", "q", "-1"), c("BLPOP", "q", "abc"), c("BLPOP", "q", "inf"), c("BLPOP", "q")),
		k(g, "blpop-wrongtype", c("SET", "s", "v"), c("BLPOP", "s", "0.1")),
		k(g, "blmove-present", c("RPUSH", "a", "1", "2"), c("BLMOVE", "a", "b", "LEFT", "RIGHT", "0"), c("BLMOVE", "a", "b", "RIGHT", "LEFT", "0.5"), c("LRANGE", "b", "0", "-1"), c("EXISTS", "a")),
		k(g, "blmove-absent", c("BLMOVE", "a", "b", "LEFT", "RIGHT", "0.1"), c("BRPOPLPUSH", "a", "b", "0.1"), c("EXISTS", "b")),
		k(g, "blmove-timeout0-wakeup", t0("BLMOVE", "a", "b", "LEFT", "LEFT", "0"), sleep(200), on(1, "RPUSH", "a", "x"), recv(0), c("LRANGE", "b", "0", "-1"), c("EXISTS", "a")),
		k(g, "brpoplpush-wakeup", t0("BRPOPLPUSH", "a", "b", "0"), sleep(200), on(1, "RPUSH", "a", "x"), recv(0), c("LRANGE", "b", "0", "-1")),
		k(g, "blmpop-present", c("RPUSH", "b", "1", "2", "3"), c("BLMPOP", "0", "2", "a", "b", "LEFT", "COUNT", "2"), c("BLMPOP", "0.5", "1", "b", "RIGHT"), c("EXISTS", "b")),
		k(g, "blmpop-absent", c("BLMPOP", "0.1", "1", "a", "LEFT"), c("BLMPOP", "0.1", "2", "a", "b", "RIGHT", "COUNT", "3")),
		k(g, "blmpop-timeout0-wakeup", t0("BLMPOP", "0", "2", "a", "b", "RIGHT", "COUNT", "2"), sleep(200), on(1, "RPUSH", "b", "1", "2", "3"), recv(0), c("LRANGE", "b", "0", "-1")),
		k(g, "bzpopmin-present", c("ZADD", "z", "1", "a", "2", "b"), c("BZPOPMIN", "y", "z", "0"), c("BZPOPMAX", "z", "0.5"), c("EXISTS", "z")),
		k(g, "bzpopmin-absent", c("BZPOPMIN", "z", "0.1"), c("BZPOPMAX", "z", "0.1"), c("BZMPOP", "0.1", "1", "z", "MIN")),
		k(g, "bzpopmin-timeout0-wakeup", t0("BZPOPMIN", "z", "0"), sleep(200), on(1, "ZADD", "z", "5", "m", "3", "n"), recv(0), c("ZRANGE", "z", "0", "-1", "WITHSCORES")),
		k(g, "bzmpop-wakeup", t0("BZMPOP", "0", "1", "z", "MAX", "COUNT", "2"), sleep(200), on(1, "ZADD", "z", "1", "a", "2", "b", "3", "c"), recv(0), c("ZRANGE", "z", "0", "-1")),
		k(g, "blpop-inside-multi", c("MULTI"), c("BLPOP", "q", "0"), c("EXEC")),
		k(g, "blpop-fifo-two-waiters", send(0, "BLPOP", "q", "0"), sleep(100), send(2, "BLPOP", "q", "0"), sleep(200), on(1, "RPUSH", "q", "first", "second"), recv(0), recv(2), c("EXISTS", "q")),
		k(g, "blpop-wakeup-after-multi-push", t0("BLPOP", "q", "0"), sleep(200), on(1, "MULTI"), on(1, "RPUSH", "q", "a"), on(1, "RPUSH", "q", "b"), on(1, "EXEC"), recv(0), c("LRANGE", "q", "0", "-1")),
		k(g, "blpop-lmove-target", t0("BLPOP", "dst", "0"), sleep(200), on(1, "RPUSH", "src", "v"), on(1, "LMOVE", "src", "dst", "LEFT", "LEFT"), recv(0), c("EXISTS", "src", "dst")),
	}
}

func setCases() []Case {
	g := "sets"
	return []Case{
		k(g, "sadd-smembers", c("SADD", "s", "a", "b", "a", "c"), c("SADD", "s", "a"), srt("SMEMBERS", "s"), c("SCARD", "s"), c("SISMEMBER", "s", "a"), c("SISMEMBER", "s", "z"), c("SMISMEMBER", "s", "a", "z", "c"), c("SMEMBERS", "missing"), c("SCARD", "missing")),
		k(g, "srem-removes-key", c("SADD", "s", "a", "b"), c("SREM", "s", "a", "z"), c("SREM", "s", "b"), c("EXISTS", "s"), c("SREM", "s", "a")),
		noState(k(g, "spop-srandmember", c("SADD", "s", "a", "b", "c", "d"), kind("SPOP", "s"), length("SPOP", "s", "2"), c("SCARD", "s"), length("SRANDMEMBER", "s", "5"), length("SRANDMEMBER", "s", "-5"), c("SPOP", "missing"), c("SPOP", "missing", "2"), c("SRANDMEMBER", "missing"), c("SRANDMEMBER", "s", "0"), c("SPOP", "s", "-1"))),
		k(g, "spop-all", c("SADD", "s", "a", "b"), length("SPOP", "s", "10"), c("EXISTS", "s")),
		k(g, "set-algebra", c("SADD", "a", "1", "2", "3", "4"), c("SADD", "b", "3", "4", "5"), srt("SINTER", "a", "b"), srt("SUNION", "a", "b"), srt("SDIFF", "a", "b"), srt("SDIFF", "b", "a"), srt("SINTER", "a", "missing"), srt("SUNION", "a", "missing"), c("SINTERCARD", "2", "a", "b"), c("SINTERCARD", "2", "a", "b", "LIMIT", "1"), c("SINTERCARD", "0", "a")),
		k(g, "set-store", c("SADD", "a", "1", "2", "3"), c("SADD", "b", "2", "3", "4"), c("SINTERSTORE", "d", "a", "b"), c("SUNIONSTORE", "u", "a", "b"), c("SDIFFSTORE", "df", "a", "b"), c("SINTERSTORE", "d", "a", "missing"), c("EXISTS", "d"), c("SET", "str", "x"), c("SUNIONSTORE", "str", "a"), c("TYPE", "str")),
		k(g, "smove", c("SADD", "a", "x", "y"), c("SMOVE", "a", "b", "x"), c("SMOVE", "a", "b", "zz"), c("SMOVE", "a", "b", "y"), c("EXISTS", "a"), srt("SMEMBERS", "b"), c("SET", "str", "v"), c("SMOVE", "b", "str", "x")),
		k(g, "sscan", c(rangeArgs([]string{"SADD", "s"}, 1, 40, "m%d")...), scanAll("SSCAN", "s", "0", "COUNT", "5"), scanAll("SSCAN", "s", "0", "MATCH", "m3*")),
		k(g, "intset-members", c("SADD", "s", "5", "1", "3", "-2"), srt("SMEMBERS", "s"), c("SISMEMBER", "s", "-2")),
	}
}

func zsetCases() []Case {
	g := "zsets"
	return []Case{
		k(g, "zadd-basic", c("ZADD", "z", "1", "a", "2", "b", "3", "c"), c("ZADD", "z", "4", "a"), c("ZRANGE", "z", "0", "-1", "WITHSCORES"), c("ZCARD", "z"), c("ZSCORE", "z", "a"), c("ZSCORE", "z", "zz"), c("ZMSCORE", "z", "a", "zz", "b")),
		k(g, "zadd-nx-xx", c("ZADD", "z", "1", "a"), c("ZADD", "z", "NX", "5", "a", "2", "b"), c("ZADD", "z", "XX", "5", "a", "3", "c"), c("ZRANGE", "z", "0", "-1", "WITHSCORES")),
		k(g, "zadd-gt-lt-ch", c("ZADD", "z", "5", "a", "5", "b"), c("ZADD", "z", "GT", "CH", "3", "a", "7", "b", "1", "c"), c("ZADD", "z", "LT", "CH", "1", "a", "9", "b"), c("ZADD", "z", "CH", "1", "a", "100", "b"), c("ZRANGE", "z", "0", "-1", "WITHSCORES")),
		k(g, "zadd-incr", c("ZADD", "z", "INCR", "5", "a"), c("ZADD", "z", "INCR", "2.5", "a"), c("ZADD", "z", "XX", "INCR", "1", "missing"), c("ZADD", "z", "NX", "INCR", "1", "a"), c("ZADD", "z", "GT", "INCR", "-1", "a"), c("ZADD", "z", "INCR", "1", "a", "2", "b"), c("ZSCORE", "z", "a")),
		k(g, "zadd-errors", c("ZADD", "z", "NX", "XX", "1", "a"), c("ZADD", "z", "GT", "LT", "1", "a"), c("ZADD", "z", "NX", "GT", "1", "a"), c("ZADD", "z", "abc", "a"), c("ZADD", "z", "1"), c("ZADD", "z", "1", "a", "2"), c("ZADD", "z", "nan", "a"), c("EXISTS", "z")),
		k(g, "zadd-inf-scores", c("ZADD", "z", "-inf", "lo", "+inf", "hi", "0", "mid"), c("ZRANGE", "z", "0", "-1", "WITHSCORES"), c("ZSCORE", "z", "hi"), c("ZINCRBY", "z", "1", "hi"), c("ZINCRBY", "z", "-inf", "hi")),
		k(g, "score-format", c("ZADD", "z", "1.5", "a", "3.0", "b", "1e20", "c", "0.1", "d", "-0", "e", "123456789012345678", "f"), c("ZINCRBY", "z", "0.2", "d"), c("ZRANGE", "z", "0", "-1", "WITHSCORES"), c("ZSCORE", "z", "f")),
		k(g, "zrange-byscore", zseed(), c("ZRANGE", "z", "2", "4", "BYSCORE"), c("ZRANGE", "z", "(2", "4", "BYSCORE", "WITHSCORES"), c("ZRANGE", "z", "-inf", "+inf", "BYSCORE", "LIMIT", "1", "2"), c("ZRANGE", "z", "4", "2", "BYSCORE", "REV"), c("ZRANGE", "z", "+inf", "-inf", "BYSCORE", "REV", "LIMIT", "0", "2", "WITHSCORES"), c("ZRANGE", "z", "abc", "2", "BYSCORE"), c("ZRANGE", "z", "0", "-1", "LIMIT", "0", "1")),
		k(g, "zrange-bylex", c("ZADD", "z", "0", "a", "0", "b", "0", "c", "0", "d", "0", "e"), c("ZRANGE", "z", "[b", "(d", "BYLEX"), c("ZRANGE", "z", "-", "+", "BYLEX", "LIMIT", "1", "2"), c("ZRANGE", "z", "(d", "[b", "BYLEX", "REV"), c("ZRANGE", "z", "b", "d", "BYLEX"), c("ZRANGEBYLEX", "z", "-", "[c"), c("ZREVRANGEBYLEX", "z", "+", "(c"), c("ZLEXCOUNT", "z", "[b", "+")),
		k(g, "zrange-rev-index", zseed(), c("ZRANGE", "z", "0", "1", "REV"), c("ZRANGE", "z", "-2", "-1", "WITHSCORES"), c("ZREVRANGE", "z", "0", "-1", "WITHSCORES"), c("ZRANGE", "z", "10", "20"), c("ZRANGE", "missing", "0", "-1")),
		k(g, "zrangebyscore-legacy", zseed(), c("ZRANGEBYSCORE", "z", "-inf", "+inf", "WITHSCORES", "LIMIT", "1", "3"), c("ZREVRANGEBYSCORE", "z", "+inf", "(3"), c("ZCOUNT", "z", "(1", "3"), c("ZCOUNT", "z", "-inf", "+inf")),
		k(g, "zrank", zseed(), c("ZRANK", "z", "c"), c("ZREVRANK", "z", "c"), c("ZRANK", "z", "zz"), c("ZRANK", "z", "c", "WITHSCORE"), c("ZREVRANK", "z", "a", "WITHSCORE"), c("ZRANK", "z", "zz", "WITHSCORE")),
		k(g, "zrangestore", zseed(), c("ZRANGESTORE", "dst", "z", "0", "1"), c("ZRANGE", "dst", "0", "-1", "WITHSCORES"), c("ZRANGESTORE", "dst2", "z", "2", "4", "BYSCORE", "REV"), c("ZRANGESTORE", "dst3", "z", "(5", "+inf", "BYSCORE"), c("EXISTS", "dst3"), c("ZRANGESTORE", "dst", "missing", "0", "-1"), c("EXISTS", "dst")),
		k(g, "zunion-zinter", c("ZADD", "a", "1", "x", "2", "y"), c("ZADD", "b", "3", "y", "4", "z"), c("ZUNION", "2", "a", "b", "WITHSCORES"), c("ZINTER", "2", "a", "b", "WITHSCORES"), c("ZUNION", "2", "a", "b", "WEIGHTS", "2", "3", "WITHSCORES"), c("ZUNION", "2", "a", "b", "AGGREGATE", "MAX", "WITHSCORES"), c("ZINTER", "2", "a", "b", "AGGREGATE", "MIN", "WITHSCORES"), c("ZUNION", "2", "a", "b"), c("ZDIFF", "2", "a", "b", "WITHSCORES"), c("ZINTERCARD", "2", "a", "b")),
		k(g, "zunion-with-set", c("ZADD", "a", "1", "x"), c("SADD", "s", "x", "y"), c("ZUNION", "2", "a", "s", "WITHSCORES"), c("ZINTERSTORE", "d", "2", "a", "s"), c("ZRANGE", "d", "0", "-1", "WITHSCORES")),
		k(g, "zunionstore", c("ZADD", "a", "1", "x", "2", "y"), c("ZADD", "b", "3", "y"), c("ZUNIONSTORE", "d", "2", "a", "b"), c("ZRANGE", "d", "0", "-1", "WITHSCORES"), c("ZINTERSTORE", "e", "2", "a", "missing"), c("EXISTS", "e"), c("ZUNIONSTORE", "d", "0", "a"), c("ZUNION", "3", "a", "b")),
		k(g, "zmpop", zseed(), c("ZMPOP", "1", "z", "MIN"), c("ZMPOP", "1", "z", "MAX", "COUNT", "2"), c("ZMPOP", "2", "missing", "z", "MIN", "COUNT", "10"), c("ZMPOP", "1", "z", "MIN"), c("EXISTS", "z"), c("ZMPOP", "0", "z", "MIN"), c("ZMPOP", "1", "z", "BOTH")),
		k(g, "zpopmin-max", zseed(), c("ZPOPMIN", "z"), c("ZPOPMAX", "z", "2"), c("ZPOPMIN", "z", "0"), c("ZPOPMIN", "missing"), c("ZPOPMIN", "z", "-1")),
		k(g, "zrem-ranges", zseed(), c("ZREM", "z", "a", "zz"), c("ZREMRANGEBYSCORE", "z", "(2", "3"), c("ZREMRANGEBYRANK", "z", "-1", "-1"), c("ZRANGE", "z", "0", "-1", "WITHSCORES"), c("ZREM", "z", "b", "d"), c("EXISTS", "z")),
		k(g, "zincrby", c("ZINCRBY", "z", "1.5", "a"), c("ZINCRBY", "z", "abc", "a"), c("ZINCRBY", "z", "-1.5", "a"), c("ZSCORE", "z", "a")),
		k(g, "zrandmember", zseed(), length("ZRANDMEMBER", "z", "3"), length("ZRANDMEMBER", "z", "-10", "WITHSCORES"), c("ZRANDMEMBER", "missing"), c("ZRANDMEMBER", "z", "0")),
		k(g, "zscan", zseed(), scanAllPairs("ZSCAN", "z", "0"), scanAllPairs("ZSCAN", "z", "0", "MATCH", "[ab]")),
		k(g, "same-score-lex-order", c("ZADD", "z", "1", "c", "1", "a", "1", "b"), c("ZRANGE", "z", "0", "-1"), c("ZREVRANGE", "z", "0", "-1")),
	}
}

func zseed() Step {
	return c("ZADD", "z", "1", "a", "2", "b", "3", "c", "4", "d", "5", "e")
}

func streamCases() []Case {
	g := "streams"
	return []Case{
		k(g, "xadd-explicit", c("XADD", "s", "1-1", "f", "v"), c("XADD", "s", "1-1", "f", "v"), c("XADD", "s", "0-1", "f", "v"), c("XADD", "s", "1-*", "f", "v2"), c("XADD", "s", "5", "a", "b", "c", "d"), c("XLEN", "s"), c("XRANGE", "s", "-", "+")),
		k(g, "xadd-zero-id", c("XADD", "s", "0-0", "f", "v"), c("XADD", "s", "abc", "f", "v"), c("XADD", "s", "1-1", "f"), c("EXISTS", "s")),
		noState(k(g, "xadd-auto", kind("XADD", "s", "*", "f", "v"), c("XLEN", "s"), kind("XADD", "s", "NOMKSTREAM", "*", "f", "v"), c("XLEN", "s"), length("XRANGE", "s", "-", "+"), c("XADD", "missing", "NOMKSTREAM", "*", "f", "v"), c("EXISTS", "missing"))),
		kx(g, "xrange-count-rev", c("XRANGE", "s", "-", "+", "COUNT", "2"), c("XRANGE", "s", "2", "3"), c("XRANGE", "s", "(1-0", "+"), c("XREVRANGE", "s", "+", "-", "COUNT", "2"), c("XRANGE", "s", "5", "1"), c("XRANGE", "missing", "-", "+")),
		kx(g, "xread", c("XREAD", "COUNT", "2", "STREAMS", "s", "0"), c("XREAD", "STREAMS", "s", "2-0"), c("XREAD", "STREAMS", "s", "$"), c("XREAD", "STREAMS", "s", "missing", "0", "0"), c("XREAD", "BLOCK", "100", "STREAMS", "s", "$"), c("XREAD", "STREAMS", "s")),
		kx(g, "xread-block-wakeup", send(0, "XREAD", "BLOCK", "0", "STREAMS", "s", "$"), sleep(200), on(1, "XADD", "s", "10-1", "new", "1"), recv(0)),
		kx(g, "xgroup-create", c("XGROUP", "CREATE", "s", "g", "0"), c("XGROUP", "CREATE", "s", "g", "0"), c("XGROUP", "CREATE", "missing", "g", "$"), c("XGROUP", "CREATE", "new", "g", "$", "MKSTREAM"), c("XLEN", "new"), c("XGROUP", "DESTROY", "s", "g"), c("XGROUP", "DESTROY", "s", "g")),
		kx(g, "xreadgroup-ack-pending", c("XGROUP", "CREATE", "s", "g", "0"), c("XREADGROUP", "GROUP", "g", "c1", "COUNT", "2", "STREAMS", "s", ">"), c("XREADGROUP", "GROUP", "g", "c2", "STREAMS", "s", ">"), c("XREADGROUP", "GROUP", "g", "c2", "STREAMS", "s", ">"), c("XPENDING", "s", "g"), c("XACK", "s", "g", "1-0", "2-0", "99-0"), c("XPENDING", "s", "g"), c("XREADGROUP", "GROUP", "g", "c1", "STREAMS", "s", "0"), c("XACK", "s", "g", "1-0")),
		kx(g, "xreadgroup-errors", c("XREADGROUP", "GROUP", "nog", "c", "STREAMS", "s", ">"), c("XREADGROUP", "GROUP", "g", "c", "STREAMS", "missing", ">")),
		kx(g, "xpending-extended-shape", c("XGROUP", "CREATE", "s", "g", "0"), c("XREADGROUP", "GROUP", "g", "c1", "STREAMS", "s", ">"), length("XPENDING", "s", "g", "-", "+", "10"), length("XPENDING", "s", "g", "-", "+", "10", "c2")),
		kx(g, "xdel-xtrim", c("XDEL", "s", "2-0", "99-0"), c("XLEN", "s"), c("XTRIM", "s", "MAXLEN", "1"), c("XRANGE", "s", "-", "+"), c("XADD", "s", "MAXLEN", "2", "10-0", "f", "v"), c("XADD", "s", "MAXLEN", "2", "11-0", "f", "v"), c("XRANGE", "s", "-", "+")),
		noState(kx(g, "xadd-smaller-after-delete", c("XDEL", "s", "3-0"), c("XADD", "s", "3-0", "f", "v"), kind("XADD", "s", "*", "f", "v"), c("XLEN", "s"))),
		k(g, "xadd-trim-options", c("XADD", "s", "MAXLEN", "5", "MINID", "3", "*", "f", "v"), c("XADD", "s", "LIMIT", "5", "*", "f", "v"), c("XADD", "s", "MAXLEN", "5", "LIMIT", "5", "*", "f", "v"), c("XADD", "s", "MAXLEN", "-1", "*", "f", "v"), c("XADD", "s", "MAXLEN", "~", "5", "LIMIT", "-1", "*", "f", "v"), c("XADD", "s", "MAXLEN", "abc", "*", "f", "v"), c("XADD", "s", "MINID", "abc", "*", "f", "v"), c("XADD", "s", "MAXLEN", "5"), c("EXISTS", "s"), c("XADD", "s", "1-0", "a", "1"), c("XADD", "s", "2-0", "b", "2"), c("XADD", "s", "3-0", "c", "3"), c("XADD", "s", "MAXLEN", "=", "2", "4-0", "d", "4"), c("XRANGE", "s", "-", "+"), c("XADD", "s", "MAXLEN", "~", "1", "5-0", "e", "5"), c("XLEN", "s"), c("XADD", "s", "MINID", "4", "6-0", "f", "6"), c("XRANGE", "s", "-", "+"), c("XADD", "s", "nomkstream", "maxlen", "~", "0", "LIMIT", "0", "7-0", "g", "7"), c("XLEN", "s")),
		k(g, "xadd-partial-ids", c("XADD", "s", "0-*", "f", "v"), c("XADD", "s", "0-*", "f", "v"), c("XADD", "s", "5-18446744073709551615", "f", "v"), c("XADD", "s", "5-*", "f", "v"), c("XADD", "s", "4-*", "f", "v"), c("XADD", "s", "-", "f", "v"), c("XADD", "s", "+", "f", "v"), c("XADD", "s", "18446744073709551616", "f", "v"), c("XADD", "s", "1-2-3", "f", "v"), c("XADD", "s", "-*", "f", "v"), c("XADD", "s", "0", "f", "v"), c("XADD", "s", "18446744073709551615-*", "f", "v"), c("XADD", "s", "18446744073709551615-*", "f", "v"), c("XADD", "s", "18446744073709551615-18446744073709551615", "f", "v"), c("XADD", "s", "*", "f", "v"), c("XADD", "s", "18446744073709551615-*", "f", "v"), c("XRANGE", "s", "-", "+")),
		k(g, "xadd-field-order", c("XADD", "s", "1-0", "zeta", "1", "alpha", "2", "zeta", "3", "", "empty"), c("XRANGE", "s", "-", "+"), c("XREAD", "STREAMS", "s", "0"), c("XREVRANGE", "s", "+", "-")),
		xbig(g, "xtrim-approx-nodes", 250, false, c("XTRIM", "s", "MAXLEN", "~", "10", "LIMIT", "30"), c("XLEN", "s"), c("XTRIM", "s", "MAXLEN", "~", "10", "LIMIT", "150"), c("XLEN", "s"), c("XTRIM", "s", "MAXLEN", "~", "10"), c("XLEN", "s"), c("XTRIM", "s", "MINID", "~", "240"), c("XLEN", "s"), c("XTRIM", "s", "MINID", "240"), c("XLEN", "s"), c("XRANGE", "s", "-", "+", "COUNT", "1")),
		xbig(g, "xtrim-approx-node-bytes", 120, true, c("XTRIM", "s", "MAXLEN", "~", "1"), c("XLEN", "s"), kind("XADD", "s", "MAXLEN", "~", "0", "*", "f", "v"), c("XLEN", "s")),
		xbig(g, "xadd-approx-after-xdel", 150, false, c("XDEL", "s", "1-0", "2-0", "50-0"), c("XADD", "s", "MAXLEN", "~", "40", "151-0", "f", "v"), c("XLEN", "s"), c("XDEL", "s", "151-0"), c("XTRIM", "s", "MAXLEN", "~", "0"), c("XLEN", "s"), c("XADD", "s", "152-0", "f", "v"), c("XTRIM", "s", "MAXLEN", "~", "0"), c("XLEN", "s")),
		kx(g, "xtrim-errors", c("XTRIM", "s"), c("XTRIM", "s", "MAXLEN"), c("XTRIM", "s", "FOO", "1"), c("XTRIM", "s", "MAXLEN", "1", "LIMIT", "1"), c("XTRIM", "s", "MAXLEN", "1", "BOGUS"), c("XTRIM", "s", "MINID", "abc"), c("XTRIM", "s", "MINID", "-"), c("XTRIM", "s", "MAXLEN", "-5"), c("XTRIM", "missing", "MAXLEN", "1"), c("EXISTS", "missing"), c("XTRIM", "s", "MINID", "=", "2"), c("XRANGE", "s", "-", "+"), c("XTRIM", "s", "MAXLEN", "~", "0"), c("XLEN", "s"), c("EXISTS", "s")),
		kx(g, "xrange-exclusive", c("XRANGE", "s", "(-", "+"), c("XRANGE", "s", "(", "+"), c("XRANGE", "s", "-", "(+"), c("XRANGE", "s", "(18446744073709551615-18446744073709551615", "+"), c("XRANGE", "s", "-", "(0-0"), c("XRANGE", "s", "-", "(2"), c("XRANGE", "s", "(1", "+"), c("XRANGE", "s", "(1-0", "(3-0"), c("XREVRANGE", "s", "(3-0", "-"), c("XREVRANGE", "s", "+", "(1-0", "COUNT", "1"), c("XRANGE", "s", "-", "+", "COUNT", "0"), c("XRANGE", "s", "-", "+", "COUNT", "-1"), c("XRANGE", "missing", "-", "+", "COUNT", "0"), c("XRANGE", "s", "-", "+", "COUNT"), c("XRANGE", "s", "-", "+", "FOO", "1"), c("XRANGE", "s", "-", "+", "COUNT", "abc"), c("XRANGE", "s", "abc", "+"), c("XRANGE", "s", "2-0", "2-0"), c("XRANGE", "s", "1-1", "1-9")),
		kx(g, "xread-errors", c("XREAD", "STREAMS", "s", "a", "b"), c("XREAD", "FOO", "STREAMS", "s", "0"), c("XREAD", "GROUP", "g", "c", "STREAMS", "s", "0"), c("XREAD", "NOACK", "STREAMS", "s", "0"), c("XREAD", "STREAMS", "s", ">"), c("XREAD", "BLOCK", "-1", "STREAMS", "s", "0"), c("XREAD", "BLOCK", "abc", "STREAMS", "s", "0"), c("XREAD", "COUNT", "abc", "STREAMS", "s", "0"), c("XREAD", "STREAMS", "s", "abc"), c("XREAD", "STREAMS", "s", "-"), c("XREAD", "STREAMS", "s", "+"), c("XREAD", "COUNT", "0", "STREAMS", "s", "1"), c("XREAD", "COUNT", "-5", "STREAMS", "s", "1-0"), c("XREAD", "COUNT", "1", "STREAMS", "s", "missing", "0", "0"), c("XREAD", "STREAMS", "missing", "+"), c("SET", "str", "x"), c("XREAD", "STREAMS", "s", "str", "0", "0")),
		kx(g, "xread-plus-empty", c("XGROUP", "CREATE", "e", "g", "$", "MKSTREAM"), c("XREAD", "STREAMS", "e", "+"), c("XREAD", "STREAMS", "e", "$"), c("XDEL", "s", "3-0"), c("XREAD", "STREAMS", "s", "+"), c("XREAD", "STREAMS", "s", "$")),
		kx(g, "xread-block-in-multi", c("MULTI"), c("XREAD", "BLOCK", "0", "STREAMS", "s", "$"), c("XREADGROUP", "GROUP", "g", "c", "BLOCK", "0", "STREAMS", "s", ">"), c("EXEC")),
		kx(g, "xreadgroup-block-wakeup", c("XGROUP", "CREATE", "s", "g", "$"), send(0, "XREADGROUP", "GROUP", "g", "c", "BLOCK", "0", "STREAMS", "s", ">"), sleep(200), on(1, "XADD", "s", "4-0", "d", "4"), recv(0), c("XPENDING", "s", "g")),
		kx(g, "xreadgroup-block-group-destroyed", c("XGROUP", "CREATE", "s", "g", "$"), send(0, "XREADGROUP", "GROUP", "g", "c", "BLOCK", "0", "STREAMS", "s", ">"), sleep(200), on(1, "XGROUP", "DESTROY", "s", "g"), recv(0)),
		kx(g, "xreadgroup-block-key-deleted", c("XGROUP", "CREATE", "s", "g", "$"), send(0, "XREADGROUP", "GROUP", "g", "c", "BLOCK", "1500", "STREAMS", "s", ">"), sleep(200), on(1, "DEL", "s"), recv(0)),
		k(g, "xread-block-new-key", send(0, "XREAD", "COUNT", "1", "BLOCK", "0", "STREAMS", "a", "b", "$", "$"), sleep(200), on(1, "XADD", "b", "7-0", "f", "v"), recv(0)),
		kx(g, "xreadgroup-block-count", c("XGROUP", "CREATE", "s", "g", "$"), send(0, "XREADGROUP", "GROUP", "g", "c", "COUNT", "1", "BLOCK", "0", "STREAMS", "s", ">"), sleep(200), on(1, "MULTI"), on(1, "XADD", "s", "4-0", "d", "4"), on(1, "XADD", "s", "5-0", "e", "5"), on(1, "EXEC"), recv(0), c("XPENDING", "s", "g"), c("XINFO", "GROUPS", "s")),
		kx(g, "xreadgroup-block-timeout", c("XGROUP", "CREATE", "s", "g", "$"), c("XREADGROUP", "GROUP", "g", "c", "BLOCK", "50", "STREAMS", "s", ">"), c("XREADGROUP", "GROUP", "g", "c", "BLOCK", "50", "STREAMS", "s", "0")),
		kx(g, "xreadgroup-id-errors", c("XGROUP", "CREATE", "s", "g", "0"), c("XREADGROUP", "GROUP", "g", "c", "STREAMS", "s", "$"), c("XREADGROUP", "GROUP", "g", "c", "STREAMS", "s", "+"), c("XREADGROUP", "GROUP", "g", "c", "STREAMS", "s", "a", "b"), c("XREADGROUP", "GROUP", "g", "c", "STREAMS", "s", "abc"), c("XREADGROUP", "COUNT", "1", "NOACK", "STREAMS", "s", ">"), c("XREADGROUP", "GROUP", "g", "STREAMS", "s", ">"), c("XREADGROUP", "GROUP", "nog", "c", "STREAMS", "s", "$"), c("XREADGROUP", "GROUP", "g", "c", "COUNT", "x", "STREAMS", "s", ">"), c("XREADGROUP", "GROUP", "g", "c", "BOGUS", "STREAMS", "s", ">"), c("XINFO", "GROUPS", "s")),
		kx(g, "xreadgroup-history", c("XADD", "s", "4-0", "d", "4"), c("XGROUP", "CREATE", "s", "g", "0"), c("XREADGROUP", "GROUP", "g", "c1", "COUNT", "2", "STREAMS", "s", ">"), c("XREADGROUP", "GROUP", "g", "c2", "COUNT", "1", "STREAMS", "s", ">"), c("XDEL", "s", "1-0"), c("XREADGROUP", "GROUP", "g", "c1", "STREAMS", "s", "0"), c("XREADGROUP", "GROUP", "g", "c1", "STREAMS", "s", "1-0"), c("XREADGROUP", "GROUP", "g", "c1", "COUNT", "1", "STREAMS", "s", "0"), c("XREADGROUP", "GROUP", "g", "c3", "STREAMS", "s", "0"), c("XREADGROUP", "GROUP", "g", "c1", "STREAMS", "s", "0", "missing"), c("XPENDING", "s", "g"), c("XREADGROUP", "GROUP", "g", "c3", "NOACK", "STREAMS", "s", ">"), c("XREADGROUP", "GROUP", "g", "c3", "NOACK", "STREAMS", "s", ">"), c("XPENDING", "s", "g"), c("XINFO", "GROUPS", "s"), c("XGROUP", "CREATECONSUMER", "s", "g", "c4"), c("XPENDING", "s", "g")),
		kx(g, "xreadgroup-multi-stream", c("XADD", "t", "5-0", "x", "1"), c("XGROUP", "CREATE", "s", "g", "0"), c("XGROUP", "CREATE", "t", "g", "$"), c("XREADGROUP", "GROUP", "g", "c", "COUNT", "1", "STREAMS", "s", "t", ">", ">"), c("XREADGROUP", "GROUP", "g", "c", "STREAMS", "s", "t", "0", "0"), c("XREADGROUP", "GROUP", "g", "c", "STREAMS", "s", "t", ">", "0")),
		kx(g, "xreadgroup-setid-redeliver", c("XGROUP", "CREATE", "s", "g", "0"), c("XREADGROUP", "GROUP", "g", "c1", "STREAMS", "s", ">"), c("XGROUP", "SETID", "s", "g", "1-0"), c("XREADGROUP", "GROUP", "g", "c2", "STREAMS", "s", ">"), c("XPENDING", "s", "g"), c("XINFO", "GROUPS", "s")),
		kx(g, "xpending-forms", c("XGROUP", "CREATE", "s", "g", "0"), c("XGROUP", "CREATE", "s", "empty", "$"), c("XPENDING", "s", "empty"), length("XPENDING", "s", "empty", "-", "+", "10"), c("XREADGROUP", "GROUP", "g", "c1", "COUNT", "2", "STREAMS", "s", ">"), c("XREADGROUP", "GROUP", "g", "c2", "STREAMS", "s", ">"), c("XPENDING", "s", "g", "-", "+"), c("XPENDING", "s", "g", "IDLE", "10"), c("XPENDING", "s", "g", "IDLE", "abc", "-", "+", "10"), c("XPENDING", "s", "g", "-", "+", "abc"), c("XPENDING", "s", "g", "-", "+", "0"), c("XPENDING", "s", "g", "-", "+", "-1"), c("XPENDING", "s", "g", "abc", "+", "10"), c("XPENDING", "s", "g", "(18446744073709551615-18446744073709551615", "+", "10"), c("XPENDING", "s", "nog"), c("XPENDING", "missing", "g"), c("XPENDING", "missing", "g", "-", "+", "10"), length("XPENDING", "s", "g", "(1-0", "+", "10"), length("XPENDING", "s", "g", "-", "(3-0", "10"), length("XPENDING", "s", "g", "-", "+", "10", "c1", "extra"), length("XPENDING", "s", "g", "-", "+", "10", "nosuch"), length("XPENDING", "s", "g", "IDLE", "3600000", "-", "+", "10"), length("XPENDING", "s", "g", "IDLE", "0", "-", "+", "10", "c2"), length("XPENDING", "s", "g", "-", "+", "1")),
		kx(g, "xclaim-forms", c("XGROUP", "CREATE", "s", "g", "0"), c("XREADGROUP", "GROUP", "g", "c1", "STREAMS", "s", ">"), c("XCLAIM", "s", "g", "c2", "0", "1-0", "JUSTID"), c("XCLAIM", "s", "g", "c2", "0", "2-0", "99-0"), c("XDEL", "s", "3-0"), c("XCLAIM", "s", "g", "c2", "0", "3-0"), c("XPENDING", "s", "g"), c("XCLAIM", "s", "g", "c2", "3600000", "1-0", "JUSTID"), c("XCLAIM", "s", "g", "c2", "0", "abc"), c("XCLAIM", "s", "g", "c2", "abc", "1-0"), c("XCLAIM", "s", "g", "c2", "-5", "1-0", "JUSTID"), c("XCLAIM", "s", "g", "c2", "0", "1-0", "BOGUS"), c("XCLAIM", "s", "g", "c2", "0", "1-0", "IDLE", "abc"), c("XCLAIM", "s", "g", "c2", "0", "1-0", "TIME", "abc"), c("XCLAIM", "s", "g", "c2", "0", "1-0", "RETRYCOUNT", "abc"), c("XCLAIM", "s", "g", "c2", "0", "1-0", "LASTID", "abc"), c("XCLAIM", "s", "nog", "c2", "0", "1-0"), c("XCLAIM", "missing", "g", "c2", "0", "1-0"), c("XADD", "s", "4-0", "d", "4"), c("XCLAIM", "s", "g", "c3", "0", "4-0", "JUSTID"), c("XCLAIM", "s", "g", "c3", "0", "4-0", "FORCE", "RETRYCOUNT", "5", "JUSTID"), c("XCLAIM", "s", "g", "c3", "0", "99-0", "FORCE"), c("XPENDING", "s", "g"), c("XINFO", "GROUPS", "s"), c("XCLAIM", "s", "g", "c3", "0", "4-0", "LASTID", "9-0", "JUSTID"), c("XINFO", "GROUPS", "s"), c("XCLAIM", "s", "g", "c4", "0", "1-0", "2-0", "4-0", "IDLE", "5000", "TIME", "1", "JUSTID"), c("XPENDING", "s", "g")),
		kx(g, "xautoclaim-forms", c("XADD", "s", "4-0", "d", "4"), c("XGROUP", "CREATE", "s", "g", "0"), c("XREADGROUP", "GROUP", "g", "c1", "STREAMS", "s", ">"), c("XDEL", "s", "2-0"), c("XAUTOCLAIM", "s", "g", "c2", "0", "0", "COUNT", "1"), c("XAUTOCLAIM", "s", "g", "c2", "0", "2-0", "COUNT", "1"), c("XAUTOCLAIM", "s", "g", "c2", "0", "0", "JUSTID"), c("XAUTOCLAIM", "s", "g", "c3", "3600000", "0"), c("XAUTOCLAIM", "s", "g", "c3", "0", "(1-0", "COUNT", "1", "JUSTID"), c("XAUTOCLAIM", "s", "g", "c3", "0", "0", "COUNT", "0"), c("XAUTOCLAIM", "s", "g", "c3", "0", "0", "COUNT", "abc"), c("XAUTOCLAIM", "s", "g", "c3", "0", "abc"), c("XAUTOCLAIM", "s", "g", "c3", "0", "0", "BOGUS"), c("XAUTOCLAIM", "s", "g", "c3", "abc", "0"), c("XAUTOCLAIM", "s", "nog", "c3", "0", "0"), c("XAUTOCLAIM", "missing", "g", "c3", "0", "0"), c("XPENDING", "s", "g")),
		kx(g, "xgroup-subcommands", c("XGROUP", "FOO", "s"), c("XGROUP", "CREATE", "s"), c("XGROUP", "CREATE", "s", "g", "0", "BOGUS"), c("XGROUP", "CREATE", "s", "g", "abc"), c("XGROUP", "CREATE", "s", "g", "-"), c("XGROUP", "CREATE", "s", "g", "0", "ENTRIESREAD", "-2"), c("XGROUP", "CREATE", "s", "g", "0", "ENTRIESREAD", "x"), c("XGROUP", "CREATE", "s", "g", "0", "MKSTREAM", "ENTRIESREAD", "1", "extra"), c("XGROUP", "CREATE", "s", "ge", "0", "ENTRIESREAD", "2"), c("XGROUP", "CREATE", "s", "gd", "$"), c("XGROUP", "CREATE", "s", "gj", "99-0"), c("XGROUP", "CREATE", "s", "gp", "2"), c("XGROUP", "SETID", "s", "nog", "0"), c("XGROUP", "SETID", "missing", "g", "0"), c("XGROUP", "SETID", "s", "ge", "1-0", "ENTRIESREAD", "1"), c("XGROUP", "SETID", "s", "ge", "0", "FOO"), c("XGROUP", "SETID", "s", "ge"), c("XGROUP", "SETID", "s", "gd", "0", "MKSTREAM"), c("XGROUP", "SETID", "s", "gj", "$"), c("XGROUP", "CREATECONSUMER", "s", "ge", "alice"), c("XGROUP", "CREATECONSUMER", "s", "ge", "alice"), c("XGROUP", "CREATECONSUMER", "s", "nog", "alice"), c("XGROUP", "CREATECONSUMER", "missing", "ge", "alice"), c("XGROUP", "DELCONSUMER", "s", "ge", "bob"), c("XGROUP", "DELCONSUMER", "s", "nog", "bob"), c("XGROUP", "DELCONSUMER", "s", "ge", "alice"), c("XGROUP", "DESTROY", "missing", "g"), c("XGROUP", "DESTROY", "s", "nog"), c("XGROUP", "HELP"), c("XGROUP", "HELP", "extra"), c("XGROUP"), c("XGROUP", "create", "s"), c("XINFO", "GROUPS", "s")),
		kx(g, "xgroup-delconsumer-pending", c("XGROUP", "CREATE", "s", "g", "0"), c("XREADGROUP", "GROUP", "g", "c1", "COUNT", "2", "STREAMS", "s", ">"), c("XREADGROUP", "GROUP", "g", "c2", "STREAMS", "s", ">"), c("XGROUP", "DELCONSUMER", "s", "g", "c1"), c("XPENDING", "s", "g"), c("XINFO", "GROUPS", "s"), c("XGROUP", "DESTROY", "s", "g"), c("XINFO", "GROUPS", "s"), c("XREADGROUP", "GROUP", "g", "c", "STREAMS", "s", ">")),
		kx(g, "xinfo-shapes", c("XINFO", "STREAM", "s"), c("XINFO", "STREAM", "missing"), c("XINFO", "GROUPS", "missing"), c("XINFO", "CONSUMERS", "missing", "g"), c("XINFO", "CONSUMERS", "s", "nog"), c("XINFO", "STREAM", "s", "BOGUS"), c("XINFO", "STREAM", "s", "FULL", "BOGUS"), c("XINFO", "STREAM", "s", "FULL", "COUNT"), c("XINFO", "GROUPS", "s", "extra"), c("XINFO", "CONSUMERS", "s"), c("XINFO", "FOO"), c("XINFO", "HELP"), c("XINFO"), c("XINFO", "STREAM", "s", "FULL"), c("XINFO", "STREAM", "s", "FULL", "COUNT", "1"), c("XGROUP", "CREATE", "e", "g", "$", "MKSTREAM"), c("XINFO", "STREAM", "e"), c("XINFO", "STREAM", "e", "FULL"), c("XINFO", "GROUPS", "e"), c("XINFO", "CONSUMERS", "e", "g"), c("SET", "str", "x"), c("XINFO", "STREAM", "str")),
		kx(g, "xinfo-after-deletes", c("XDEL", "s", "1-0"), c("XINFO", "STREAM", "s"), c("XDEL", "s", "3-0"), c("XINFO", "STREAM", "s"), c("XTRIM", "s", "MAXLEN", "0"), c("XINFO", "STREAM", "s"), c("EXISTS", "s")),
		kx(g, "xinfo-lag", c("XGROUP", "CREATE", "s", "gx", "1-0", "ENTRIESREAD", "0"), c("XGROUP", "CREATE", "s", "gy", "2-0", "ENTRIESREAD", "10"), c("XGROUP", "CREATE", "s", "gz", "5-0", "ENTRIESREAD", "1"), c("XGROUP", "CREATE", "s", "gw", "2-5"), c("XGROUP", "CREATE", "s", "g0", "0"), c("XGROUP", "CREATE", "s", "gd", "$"), c("XINFO", "GROUPS", "s"), c("XREADGROUP", "GROUP", "gx", "c", "STREAMS", "s", ">"), c("XREADGROUP", "GROUP", "g0", "c", "COUNT", "1", "STREAMS", "s", ">"), c("XINFO", "GROUPS", "s"), c("XTRIM", "s", "MAXLEN", "1"), c("XGROUP", "CREATE", "s", "ga", "0"), c("XGROUP", "CREATE", "s", "gc", "2-0", "ENTRIESREAD", "1"), c("XINFO", "GROUPS", "s"), c("XADD", "s", "4-0", "d", "4"), c("XADD", "s", "5-0", "e", "5"), c("XDEL", "s", "4-0"), c("XINFO", "GROUPS", "s"), c("XREADGROUP", "GROUP", "g0", "c", "STREAMS", "s", ">"), c("XINFO", "GROUPS", "s")),
		kx(g, "xinfo-lag-tombstone", c("XDEL", "s", "2-0"), c("XGROUP", "CREATE", "s", "g", "0"), c("XINFO", "GROUPS", "s"), c("XREADGROUP", "GROUP", "g", "c", "COUNT", "1", "STREAMS", "s", ">"), c("XINFO", "GROUPS", "s"), c("XREADGROUP", "GROUP", "g", "c", "COUNT", "1", "STREAMS", "s", ">"), c("XINFO", "GROUPS", "s"), c("XGROUP", "CREATE", "s", "n", "$", "ENTRIESREAD", "3"), c("XADD", "s", "4-0", "d", "4"), c("XINFO", "GROUPS", "s")),
		kx(g, "xsetid-forms", c("XSETID", "s", "1-0"), c("XSETID", "s", "2-0", "ENTRIESADDED", "1"), c("XSETID", "s", "4-0", "ENTRIESADDED", "2", "MAXDELETEDID", "5-0"), c("XSETID", "s", "4-0", "ENTRIESADDED", "3", "MAXDELETEDID", "2-5"), c("XSETID", "s", "3-0", "ENTRIESADDED", "-1"), c("XSETID", "s", "3-0", "FOO"), c("XSETID", "s", "abc"), c("XSETID", "missing", "1-0"), c("XSETID", "s", "3-0", "ENTRIESADDED"), c("XSETID", "s", "1-0", "MAXDELETEDID", "abc"), c("XINFO", "STREAM", "s"), c("XSETID", "s", "2-1"), c("XSETID", "s", "5"), c("XADD", "s", "5-0", "f", "v"), c("XADD", "s", "5-*", "f", "v"), c("XINFO", "STREAM", "s"), c("XDEL", "s", "5-1"), c("XSETID", "s", "4-0"), c("XSETID", "s", "5-1")),
		kx(g, "xack-xdel-forms", c("XGROUP", "CREATE", "s", "g", "0"), c("XREADGROUP", "GROUP", "g", "c", "STREAMS", "s", ">"), c("XACK", "s", "g", "1-0", "abc"), c("XPENDING", "s", "g"), c("XACK", "s", "g", "1", "2-0", "2-0"), c("XACK", "s", "nog", "abc"), c("XACK", "missing", "g", "abc"), c("XDEL", "s", "abc"), c("XDEL", "s", "1-0", "-"), c("XDEL", "missing", "1-0"), c("XDEL", "s", "1-0", "1-0"), c("XDEL", "s", "2"), c("XLEN", "s"), c("XLEN", "missing"), c("XPENDING", "s", "g")),
		k(g, "wrongtype-family", c("SET", "str", "x"), c("XLEN", "str"), c("XRANGE", "str", "-", "+"), c("XREVRANGE", "str", "+", "-"), c("XREAD", "STREAMS", "str", "0"), c("XREADGROUP", "GROUP", "g", "c", "STREAMS", "str", ">"), c("XGROUP", "CREATE", "str", "g", "0"), c("XINFO", "STREAM", "str"), c("XPENDING", "str", "g"), c("XCLAIM", "str", "g", "c", "0", "1-0"), c("XAUTOCLAIM", "str", "g", "c", "0", "0"), c("XACK", "str", "g", "1-0"), c("XDEL", "str", "1-0"), c("XTRIM", "str", "MAXLEN", "1"), c("XSETID", "str", "1-0"), c("XADD", "str", "NOMKSTREAM", "*", "f", "v")),
	}
}

// xbig seeds a stream with n entries "<i>-0"; wide fields make Redis split
// nodes by stream-node-max-bytes instead of the entry count.
func xbig(group, name string, n int, wide bool, steps ...Step) Case {
	seed := make([]Step, 0, n+len(steps))
	for i := 1; i <= n; i++ {
		id := fmt.Sprintf("%d-0", i)
		if wide {
			seed = append(seed, c("XADD", "s", id, fmt.Sprintf("field%d", i), fmt.Sprintf("value-with-some-length-%d", i)))
		} else {
			seed = append(seed, c("XADD", "s", id, "f", "v"))
		}
	}
	return k(group, name, append(seed, steps...)...)
}

func kx(group, name string, steps ...Step) Case {
	seed := []Step{c("XADD", "s", "1-0", "a", "1"), c("XADD", "s", "2-0", "b", "2"), c("XADD", "s", "3-0", "c", "3")}
	return k(group, name, append(seed, steps...)...)
}

func txCases() []Case {
	g := "transactions"
	return []Case{
		k(g, "multi-exec", c("MULTI"), c("SET", "a", "1"), c("INCR", "a"), c("GET", "a"), c("EXEC"), c("GET", "a")),
		k(g, "multi-discard", c("SET", "a", "0"), c("MULTI"), c("SET", "a", "1"), c("DISCARD"), c("GET", "a"), c("DISCARD"), c("EXEC")),
		k(g, "execabort-unknown-command", c("MULTI"), c("SET", "a", "1"), c("NOTACOMMAND", "x"), c("SET", "b", "2"), c("EXEC"), c("EXISTS", "a", "b")),
		k(g, "execabort-wrong-arity", c("MULTI"), c("SET", "a", "1"), c("GET"), c("EXEC"), c("EXISTS", "a")),
		k(g, "runtime-error-inside-exec", c("SET", "s", "x"), c("MULTI"), c("SET", "a", "1"), c("INCR", "s"), c("LPUSH", "s", "v"), c("SET", "b", "2"), c("EXEC"), c("MGET", "a", "b", "s")),
		k(g, "empty-exec", c("MULTI"), c("EXEC")),
		k(g, "nested-multi", c("MULTI"), c("MULTI"), c("SET", "a", "1"), c("EXEC")),
		k(g, "watch-inside-multi", c("MULTI"), c("WATCH", "a"), c("SET", "a", "1"), c("EXEC"), c("GET", "a")),
		k(g, "watch-touched-aborts", c("SET", "a", "1"), c("WATCH", "a"), on(1, "SET", "a", "2"), c("MULTI"), c("SET", "a", "3"), c("EXEC"), c("GET", "a")),
		k(g, "watch-untouched-commits", c("SET", "a", "1"), c("WATCH", "a"), c("MULTI"), c("SET", "a", "3"), c("EXEC"), c("GET", "a")),
		k(g, "watch-missing-key-created", c("WATCH", "a"), on(1, "SET", "a", "2"), c("MULTI"), c("SET", "a", "3"), c("EXEC"), c("GET", "a")),
		k(g, "watch-deleted-key", c("SET", "a", "1"), c("WATCH", "a"), on(1, "DEL", "a"), c("MULTI"), c("SET", "b", "1"), c("EXEC"), c("EXISTS", "b")),
		k(g, "watch-expire-touch", c("SET", "a", "1"), c("WATCH", "a"), on(1, "EXPIRE", "a", "100"), c("MULTI"), c("SET", "b", "1"), c("EXEC")),
		k(g, "unwatch", c("SET", "a", "1"), c("WATCH", "a"), c("UNWATCH"), on(1, "SET", "a", "2"), c("MULTI"), c("SET", "a", "3"), c("EXEC"), c("GET", "a")),
		k(g, "watch-flushall", c("SET", "a", "1"), c("WATCH", "a"), on(1, "FLUSHALL"), c("MULTI"), c("SET", "b", "1"), c("EXEC")),
		k(g, "watch-other-db-no-abort", c("SET", "a", "1"), c("WATCH", "a"), on(1, "SELECT", "1"), on(1, "SET", "a", "2"), c("MULTI"), c("SET", "b", "1"), c("EXEC")),
		k(g, "exec-discard-without-multi", c("EXEC"), c("DISCARD"), c("WATCH")),
		k(g, "multi-queued-replies", c("MULTI"), c("SET", "a", "1"), c("PING"), c("ECHO", "hi"), c("LPUSH", "l", "x"), c("EXEC")),
		k(g, "multi-select", c("MULTI"), c("SELECT", "1"), c("SET", "a", "1"), c("EXEC"), c("GET", "a"), c("SELECT", "0"), c("GET", "a")),
		k(g, "exec-after-execabort-resets", c("MULTI"), c("BOGUS"), c("EXEC"), c("SET", "a", "1"), c("GET", "a")),
		k(g, "multi-incr-wrongtype-then-ok", c("RPUSH", "l", "x"), c("MULTI"), c("INCR", "l"), c("INCR", "n"), c("EXEC")),
	}
}

var wrongTypeCmds = [][]string{
	{"GET", "L"}, {"INCR", "L"}, {"APPEND", "L", "x"}, {"STRLEN", "L"}, {"GETRANGE", "L", "0", "1"}, {"SETRANGE", "L", "0", "x"}, {"INCRBYFLOAT", "L", "1"}, {"GETDEL", "L"}, {"DECRBY", "L", "1"}, {"GETSET", "L", "x"},
	{"LPUSH", "S", "x"}, {"RPUSH", "S", "x"}, {"LRANGE", "S", "0", "-1"}, {"LLEN", "S"}, {"LPOP", "S"}, {"LINDEX", "S", "0"}, {"LSET", "S", "0", "x"}, {"LPOS", "S", "x"}, {"LMOVE", "S", "x", "LEFT", "LEFT"}, {"LMPOP", "1", "S", "LEFT"},
	{"HSET", "S", "f", "v"}, {"HGET", "S", "f"}, {"HGETALL", "S"}, {"HDEL", "S", "f"}, {"HLEN", "S"}, {"HINCRBY", "S", "f", "1"}, {"HKEYS", "S"},
	{"SADD", "S", "m"}, {"SMEMBERS", "S"}, {"SISMEMBER", "S", "m"}, {"SCARD", "S"}, {"SINTER", "S", "L"}, {"SREM", "S", "m"},
	{"ZADD", "S", "1", "m"}, {"ZRANGE", "S", "0", "-1"}, {"ZSCORE", "S", "m"}, {"ZCARD", "S"}, {"ZINCRBY", "S", "1", "m"}, {"ZUNION", "1", "S"}, {"ZRANK", "S", "m"},
	{"XADD", "S", "*", "f", "v"}, {"XRANGE", "S", "-", "+"}, {"XLEN", "S"},
	{"SUNIONSTORE", "dst", "S"}, {"PFADD", "L", "a"}, {"SETBIT", "L", "1", "1"}, {"BITCOUNT", "L"},
}

func wrongTypeCases() []Case {
	var out []Case
	for _, cmd := range wrongTypeCmds {
		out = append(out, k("wrongtype", cmd[0], c("SET", "S", "str"), c("RPUSH", "L", "a"), c(cmd...)))
	}
	return out
}

var arityCmds = [][]string{
	{"GET"}, {"GET", "a", "b"}, {"SET", "a"}, {"DEL"}, {"EXISTS"}, {"INCR"}, {"INCRBY", "a"}, {"APPEND", "a"}, {"MGET"}, {"MSET"}, {"STRLEN"}, {"GETRANGE", "a", "0"}, {"SETEX", "a", "1"},
	{"EXPIRE", "a"}, {"TTL"}, {"RENAME", "a"}, {"TYPE"}, {"KEYS"}, {"SCAN"},
	{"HSET", "h"}, {"HGET", "h"}, {"HGETALL"}, {"HDEL", "h"}, {"HMGET", "h"}, {"HINCRBY", "h", "f"},
	{"LPUSH", "l"}, {"RPUSH"}, {"LRANGE", "l", "0"}, {"LPOP"}, {"LINDEX", "l"}, {"LREM", "l", "0"}, {"BLPOP", "l"},
	{"SADD", "s"}, {"SMEMBERS"}, {"SISMEMBER", "s"}, {"SINTER"},
	{"ZADD", "z"}, {"ZRANGE", "z", "0"}, {"ZSCORE", "z"}, {"ZINCRBY", "z", "1"},
	{"XADD", "s"}, {"XRANGE", "s", "-"}, {"XLEN"},
	{"PUBLISH", "ch"}, {"SUBSCRIBE"}, {"SELECT"}, {"ECHO"}, {"ECHO", "a", "b"}, {"PING", "a", "b"}, {"EVAL"}, {"EVAL", "return 1"}, {"MULTI", "x"}, {"WATCH"}, {"OBJECT"}, {"DBSIZE", "x"},
}

func arityCases() []Case {
	var out []Case
	for _, cmd := range arityCmds {
		name := cmd[0]
		if len(cmd) > 1 {
			name += "-" + itoa(len(cmd)-1)
		}
		out = append(out, k("arity", name, c(cmd...)))
	}
	return out
}

func itoa(n int) string {
	return string(rune('0' + n))
}

func pubsubCases() []Case {
	g := "pubsub"
	slow := func(s Step) Step { s.Timeout = 2 * time.Second; return s }
	return []Case{
		k(g, "subscribe-publish-deliver", on(1, "SUBSCRIBE", "ch"), c("PUBLISH", "ch", "hello"), slow(recv(1)), c("PUBLISH", "other", "x")),
		k(g, "psubscribe-deliver", on(1, "PSUBSCRIBE", "news.*"), c("PUBLISH", "news.tech", "hi"), slow(recv(1)), c("PUBLISH", "sports", "no")),
		k(g, "subscribe-multi-channel", on(1, "SUBSCRIBE", "a", "b"), recv(1), c("PUBLISH", "b", "m1"), slow(recv(1)), c("PUBLISH", "a", "m2"), slow(recv(1))),
		k(g, "publish-no-subscribers", c("PUBLISH", "nobody", "x")),
		k(g, "two-subscribers-count", on(1, "SUBSCRIBE", "ch"), on(2, "SUBSCRIBE", "ch"), c("PUBLISH", "ch", "x"), slow(recv(1)), slow(recv(2))),
		k(g, "pattern-and-channel-both", on(1, "SUBSCRIBE", "news.a"), on(1, "PSUBSCRIBE", "news.*"), c("PUBLISH", "news.a", "x"), slow(recv(1)), slow(recv(1))),
		k(g, "unsubscribe", on(1, "SUBSCRIBE", "a", "b"), recv(1), on(1, "UNSUBSCRIBE", "a"), c("PUBLISH", "a", "x"), c("PUBLISH", "b", "y"), slow(recv(1)), on(1, "UNSUBSCRIBE"), c("PUBLISH", "b", "z")),
		k(g, "unsubscribe-not-subscribed", c("UNSUBSCRIBE", "x"), c("PUNSUBSCRIBE")),
		k(g, "ping-in-subscribe-mode", on(1, "SUBSCRIBE", "ch"), on(1, "PING"), on(1, "PING", "hi"), on(1, "GET", "x")),
		k(g, "pubsub-introspection", on(1, "SUBSCRIBE", "a", "b"), recv(1), on(2, "PSUBSCRIBE", "p*"), srt("PUBSUB", "CHANNELS"), c("PUBSUB", "CHANNELS", "a*"), c("PUBSUB", "NUMSUB", "a", "zz"), c("PUBSUB", "NUMPAT")),
		k(g, "binary-message", on(1, "SUBSCRIBE", "ch"), c("PUBLISH", "ch", "a\x00b\r\nc"), slow(recv(1))),
		k(g, "publish-from-multi", on(1, "SUBSCRIBE", "ch"), c("MULTI"), c("PUBLISH", "ch", "tx"), c("EXEC"), slow(recv(1))),
		k(g, "publish-from-lua", on(1, "SUBSCRIBE", "ch"), c("EVAL", "return redis.call('PUBLISH','ch','lua')", "0"), slow(recv(1))),
		k(g, "ssubscribe-spublish", on(1, "SSUBSCRIBE", "sh"), c("SPUBLISH", "sh", "x"), slow(recv(1))),
	}
}

func dbCases() []Case {
	g := "db"
	return []Case{
		k(g, "select-dbsize", c("SET", "a", "1"), c("DBSIZE"), c("SELECT", "1"), c("DBSIZE"), c("GET", "a"), c("SET", "b", "2"), c("SET", "c", "3"), c("DBSIZE"), c("SELECT", "0"), c("DBSIZE"), c("GET", "b")),
		k(g, "select-errors", c("SELECT", "16"), c("SELECT", "-1"), c("SELECT", "abc"), c("SELECT", "15"), c("SELECT", "0")),
		k(g, "move", c("SET", "a", "1"), c("MOVE", "a", "1"), c("EXISTS", "a"), c("MOVE", "a", "1"), c("SELECT", "1"), c("GET", "a"), c("SET", "x", "1"), c("SELECT", "0"), c("SET", "x", "0"), c("MOVE", "x", "1"), c("MOVE", "x", "0")),
		k(g, "flushdb-only-current", c("SET", "a", "1"), c("SELECT", "1"), c("SET", "b", "2"), c("FLUSHDB"), c("DBSIZE"), c("SELECT", "0"), c("DBSIZE")),
		k(g, "swapdb", c("SET", "a", "0"), c("SELECT", "1"), c("SET", "b", "1"), c("SWAPDB", "0", "1"), c("GET", "a"), c("GET", "b"), c("SELECT", "0"), c("GET", "b")),
		k(g, "select-per-connection", c("SELECT", "1"), c("SET", "k", "db1"), on(1, "GET", "k"), on(1, "SET", "k", "db0"), c("GET", "k")),
		k(g, "copy-db", c("SET", "a", "1"), c("COPY", "a", "a", "DB", "1"), c("SELECT", "1"), c("GET", "a")),
	}
}

func miscCases() []Case {
	g := "misc"
	return []Case{
		k(g, "ping-echo", c("PING"), c("PING", "hello"), c("ECHO", "hi"), c("ECHO", ""), c("ECHO", "a\x00b")),
		k(g, "object-encoding-strings", c("SET", "i", "12345"), c("SET", "e", "short"), c("SET", "r", string(make([]byte, 64))), c("OBJECT", "ENCODING", "i"), c("OBJECT", "ENCODING", "e"), c("OBJECT", "ENCODING", "r"), c("OBJECT", "ENCODING", "missing")),
		k(g, "object-encoding-collections", c("RPUSH", "l", "a"), c("HSET", "h", "f", "v"), c("SADD", "si", "1", "2"), c("SADD", "ss", "a"), c("ZADD", "z", "1", "a"), c("XADD", "x", "1-1", "f", "v"), c("OBJECT", "ENCODING", "l"), c("OBJECT", "ENCODING", "h"), c("OBJECT", "ENCODING", "si"), c("OBJECT", "ENCODING", "ss"), c("OBJECT", "ENCODING", "z"), c("OBJECT", "ENCODING", "x")),
		k(g, "object-misc", c("SET", "a", "x"), c("OBJECT", "REFCOUNT", "a"), ttl(1, "OBJECT", "IDLETIME", "a"), c("OBJECT", "FREQ", "a"), c("OBJECT", "REFCOUNT", "missing"), c("OBJECT", "BOGUS", "a")),
		k(g, "unknown-command", c("NOTACOMMAND"), c("notacommand", "a", "b")),
		k(g, "case-insensitive", c("set", "a", "1"), c("GeT", "a"), c("set", "b", "1", "ex", "100", "nx"), ttl(1, "ttl", "b")),
		k(g, "dbsize-flushall", c("MSET", "a", "1", "b", "2"), c("DBSIZE"), c("FLUSHALL"), c("DBSIZE"), c("FLUSHALL", "ASYNC"), c("FLUSHDB", "SYNC")),
		k(g, "inline-commands", Step{Raw: "PING\r\n"}, Step{Raw: "SET inl v\r\n"}, c("GET", "inl")),
		k(g, "pipelined-burst", pipeline()...),
		k(g, "client-setname", c("CLIENT", "SETNAME", "foo"), c("CLIENT", "GETNAME"), c("CLIENT", "SETNAME", "a b")),
		k(g, "hello-2", Step{Args: []string{"HELLO", "2"}, Norm: NormKind}),
	}
}

func pipeline() []Step {
	var s []Step
	for i := 0; i < 20; i++ {
		s = append(s, send(0, "INCR", "ctr"))
	}
	for i := 0; i < 20; i++ {
		s = append(s, recv(0))
	}
	s = append(s, c("GET", "ctr"))
	return s
}

func luaCases() []Case {
	g := "lua"
	return []Case{
		k(g, "return-types", c("EVAL", "return 1", "0"), c("EVAL", "return 'str'", "0"), c("EVAL", "return {1,2,{3,'x'}}", "0"), c("EVAL", "return {ok='FINE'}", "0"), c("EVAL", "return {err='MYERR boom'}", "0"), c("EVAL", "return nil", "0"), c("EVAL", "return false", "0"), c("EVAL", "return true", "0"), c("EVAL", "return 3.99", "0"), c("EVAL", "return {1,nil,3}", "0"), c("EVAL", "return redis.status_reply('X')", "0"), c("EVAL", "return redis.error_reply('E1 nope')", "0")),
		k(g, "redis-call-types", c("SET", "s", "v"), c("RPUSH", "l", "a", "b"), c("EVAL", "return redis.call('GET',KEYS[1])", "1", "s"), c("EVAL", "return redis.call('GET','missing')", "0"), c("EVAL", "return redis.call('LRANGE',KEYS[1],0,-1)", "1", "l"), c("EVAL", "return redis.call('SET','x','1')", "0"), c("EVAL", "return redis.call('INCR','x')", "0"), c("EVAL", "return type(redis.call('GET','missing'))", "0"), c("EVAL", "return redis.call('PING')", "0"), c("EVAL", "local r = redis.call('SET','y','1') return type(r)", "0")),
		k(g, "redis-call-errors", c("SET", "s", "v"), c("EVAL", "return redis.call('INCR',KEYS[1])", "1", "s"), c("EVAL", "return redis.pcall('INCR',KEYS[1])", "1", "s"), c("EVAL", "local r = redis.pcall('INCR',KEYS[1]) return type(r)", "1", "s"), c("EVAL", "return redis.call('NOSUCHCMD')", "0"), c("EVAL", "syntax error here", "0"), c("EVAL", "error('custom')", "0")),
		k(g, "keys-argv", c("EVAL", "return {KEYS[1],KEYS[2],ARGV[1],ARGV[2],#KEYS,#ARGV}", "2", "k1", "k2", "a1", "a2"), c("EVAL", "return 1", "-1"), c("EVAL", "return 1", "5", "a"), c("EVAL", "return 1", "abc")),
		k(g, "evalsha", c("SCRIPT", "LOAD", "return ARGV[1]"), c("EVALSHA", "098e0f0d1448c0a81dafe820f66d460eb09263da", "0", "x"), c("EVALSHA", "0000000000000000000000000000000000000000", "0"), c("SCRIPT", "EXISTS", "0000000000000000000000000000000000000000")),
		k(g, "evalsha-loaded", Step{Args: []string{"SCRIPT", "LOAD", "return 42"}}, c("EVALSHA", "1fa00e76656cc152ad327c13fe365858fd7be306", "0")),
		k(g, "lua-writes-state", c("EVAL", "redis.call('HSET',KEYS[1],'a',1,'b',2) redis.call('EXPIRE',KEYS[1],100) return redis.call('HLEN',KEYS[1])", "1", "h"), pairs("HGETALL", "h")),
		k(g, "lua-float-int-conv", c("EVAL", "return tonumber(ARGV[1]) * 2", "0", "2.5"), c("EVAL", "return tostring(1.5)", "0"), c("EVAL", "return redis.call('INCRBYFLOAT','f','1.5')", "0"), c("EVAL", "return cjson.encode({a=1})", "0")),
		k(g, "lua-select-isolated", c("EVAL", "redis.call('SELECT',1) redis.call('SET','a','1') return 1", "0"), c("GET", "a"), c("SELECT", "1"), c("GET", "a")),
	}
}
