"""Differential fuzzer: runs the same random command sequence against
CacheStorm and Redis and stops at the first reply or key-type difference.

    CACHESTORM_PORT=6390 REDIS_PORT=6391 python3 scripts/diff-fuzz.py SEED COUNT

Both servers are flushed first. Replies that are unordered or random by
definition are normalized; OBJECT ENCODING is not compared.
"""
import os
import random
import socket
import sys


class Conn:
    def __init__(self, port):
        self.s = socket.create_connection(("127.0.0.1", port))
        self.f = self.s.makefile("rb")

    def cmd(self, *args):
        out = [b"*%d\r\n" % len(args)]
        for a in args:
            a = a if isinstance(a, bytes) else str(a).encode()
            out.append(b"$%d\r\n%s\r\n" % (len(a), a))
        self.s.sendall(b"".join(out))
        return self.read()

    def read(self):
        line = self.f.readline()
        t, body = line[:1], line[1:-2]
        if t in (b"+", b"-"):
            return (t.decode(), body.decode(errors="replace"))
        if t == b":":
            return int(body)
        if t == b"$":
            n = int(body)
            if n < 0:
                return None
            d = self.f.read(n + 2)[:-2]
            return d
        if t == b"*":
            n = int(body)
            if n < 0:
                return None
            return [self.read() for _ in range(n)]
        # RESP3 types keep a tag so a type difference is a difference.
        if t == b"_":
            return ("null",)
        if t == b"#":
            return ("bool", body == b"t")
        if t == b",":
            return ("double", body.decode())
        if t == b"(":
            return ("big", body.decode())
        if t in (b"=", b"!"):
            d = self.f.read(int(body) + 2)[:-2]
            return ("verbatim" if t == b"=" else "-", d.decode(errors="replace"))
        if t in (b"%", b"|"):
            n = int(body)
            items = [(self.read(), self.read()) for _ in range(n)]
            if t == b"|":
                return ("attr", items, self.read())
            return ("map", items)
        if t in (b"~", b">"):
            return ("set" if t == b"~" else "push", [self.read() for _ in range(int(body))])
        raise RuntimeError("bad reply %r" % line)


KEYS = ["k1", "k2", "k3", "k4"]
VALS = ["a", "b", "c", "1", "2", "-3", "10", "3.5", "", "abc", "xyz", "0"]
FIELDS = ["f1", "f2", "f3"]
NUMS = ["0", "1", "-1", "2", "-2", "3", "100", "-100", "x", "1.5"]
SCORES = ["1", "2", "-1", "0", "1.5", "inf", "-inf", "+inf", "nan", "x"]


def k():
    return random.choice(KEYS)


def v():
    return random.choice(VALS)


def n():
    return random.choice(NUMS)


def gen():
    c = random.choice(GENS)
    return c()


GENS = [
    lambda: ["SET", k(), v()] + random.choice([[], ["NX"], ["XX"], ["GET"], ["KEEPTTL"], ["EX", "100"], ["NX", "GET"]]),
    lambda: ["GET", k()],
    lambda: ["GETDEL", k()],
    lambda: ["APPEND", k(), v()],
    lambda: ["INCR", k()],
    lambda: ["DECRBY", k(), n()],
    lambda: ["INCRBYFLOAT", k(), random.choice(NUMS + ["1e3", "inf"])],
    lambda: ["STRLEN", k()],
    lambda: ["GETRANGE", k(), n(), n()],
    lambda: ["SETRANGE", k(), random.choice(["0", "1", "3", "-1"]), v()],
    lambda: ["MSET", k(), v(), k(), v()],
    lambda: ["MGET", k(), k(), k()],
    lambda: ["MSETNX", k(), v(), k(), v()],
    lambda: ["SETNX", k(), v()],
    lambda: ["GETSET", k(), v()],
    lambda: ["DEL", k(), k()],
    lambda: ["EXISTS", k(), k()],
    lambda: ["TYPE", k()],
    lambda: ["RENAME", k(), k()],
    lambda: ["RENAMENX", k(), k()],
    lambda: ["COPY", k(), k()] + random.choice([[], ["REPLACE"]]),
    lambda: ["EXPIRE", k(), random.choice(["100", "200", "-1", "0", "x"])] + random.choice([[], ["NX"], ["XX"], ["GT"], ["LT"]]),
    lambda: ["PERSIST", k()],
    lambda: ["TTL", k()],
    lambda: ["LPUSH", k(), v(), v()],
    lambda: ["RPUSH", k(), v()],
    lambda: ["LPUSHX", k(), v()],
    lambda: ["LPOP", k()] + random.choice([[], ["2"], ["0"], ["-1"]]),
    lambda: ["RPOP", k()] + random.choice([[], ["3"]]),
    lambda: ["LRANGE", k(), n(), n()],
    lambda: ["LINDEX", k(), n()],
    lambda: ["LSET", k(), n(), v()],
    lambda: ["LREM", k(), n(), v()],
    lambda: ["LTRIM", k(), n(), n()],
    lambda: ["LINSERT", k(), random.choice(["BEFORE", "AFTER", "MIDDLE"]), v(), v()],
    lambda: ["LLEN", k()],
    lambda: ["LPOS", k(), v()] + random.choice([[], ["RANK", "-1"], ["COUNT", "0"], ["RANK", "0"], ["MAXLEN", "1"]]),
    lambda: ["LMOVE", k(), k(), random.choice(["LEFT", "RIGHT"]), random.choice(["LEFT", "RIGHT"])],
    lambda: ["RPOPLPUSH", k(), k()],
    lambda: ["LMPOP", "2", k(), k(), random.choice(["LEFT", "RIGHT"])] + random.choice([[], ["COUNT", "2"]]),
    lambda: ["SADD", k(), v(), v()],
    lambda: ["SREM", k(), v()],
    lambda: ["SISMEMBER", k(), v()],
    lambda: ["SMISMEMBER", k(), v(), v()],
    lambda: ["SCARD", k()],
    lambda: ["SMEMBERS", k()],
    lambda: ["SINTER", k(), k()],
    lambda: ["SUNION", k(), k()],
    lambda: ["SDIFF", k(), k()],
    lambda: ["SINTERSTORE", k(), k(), k()],
    lambda: ["SUNIONSTORE", k(), k(), k()],
    lambda: ["SDIFFSTORE", k(), k(), k()],
    lambda: ["SINTERCARD", "2", k(), k()] + random.choice([[], ["LIMIT", "1"]]),
    lambda: ["SMOVE", k(), k(), v()],
    lambda: ["HSET", k(), random.choice(FIELDS), v()],
    lambda: ["HSETNX", k(), random.choice(FIELDS), v()],
    lambda: ["HGET", k(), random.choice(FIELDS)],
    lambda: ["HMGET", k(), random.choice(FIELDS), random.choice(FIELDS)],
    lambda: ["HDEL", k(), random.choice(FIELDS)],
    lambda: ["HLEN", k()],
    lambda: ["HGETALL", k()],
    lambda: ["HKEYS", k()],
    lambda: ["HINCRBY", k(), random.choice(FIELDS), n()],
    lambda: ["HINCRBYFLOAT", k(), random.choice(FIELDS), n()],
    lambda: ["HSTRLEN", k(), random.choice(FIELDS)],
    lambda: ["HEXISTS", k(), random.choice(FIELDS)],
    lambda: ["ZADD", k()] + random.choice([[], ["NX"], ["XX"], ["GT"], ["LT"], ["CH"], ["INCR"], ["XX", "CH"]]) + [random.choice(SCORES), v()],
    lambda: ["ZINCRBY", k(), random.choice(SCORES), v()],
    lambda: ["ZSCORE", k(), v()],
    lambda: ["ZMSCORE", k(), v(), v()],
    lambda: ["ZRANK", k(), v()] + random.choice([[], ["WITHSCORE"]]),
    lambda: ["ZREVRANK", k(), v()],
    lambda: ["ZCARD", k()],
    lambda: ["ZCOUNT", k(), random.choice(SCORES + ["(1", "(2"]), random.choice(SCORES + ["(1"])],
    lambda: ["ZRANGE", k(), n(), n()] + random.choice([[], ["WITHSCORES"], ["REV"], ["REV", "WITHSCORES"]]),
    lambda: ["ZRANGE", k(), random.choice(SCORES + ["(1"]), random.choice(SCORES), "BYSCORE"] + random.choice([[], ["LIMIT", "0", "1"], ["REV"], ["WITHSCORES"]]),
    lambda: ["ZRANGE", k(), random.choice(["-", "[a", "(b", "+", "x"]), random.choice(["+", "[c", "(z", "-"]), "BYLEX"],
    lambda: ["ZRANGEBYSCORE", k(), random.choice(SCORES), random.choice(SCORES)],
    lambda: ["ZREM", k(), v()],
    lambda: ["ZREMRANGEBYRANK", k(), n(), n()],
    lambda: ["ZREMRANGEBYSCORE", k(), random.choice(SCORES), random.choice(SCORES)],
    lambda: ["ZPOPMIN", k()] + random.choice([[], ["2"], ["-1"]]),
    lambda: ["ZPOPMAX", k()],
    lambda: ["ZUNIONSTORE", k(), "2", k(), k()] + random.choice([[], ["WEIGHTS", "2", "3"], ["AGGREGATE", "MAX"]]),
    lambda: ["ZINTERSTORE", k(), "2", k(), k()],
    lambda: ["ZUNION", "2", k(), k(), "WITHSCORES"],
    lambda: ["ZINTER", "2", k(), k()],
    lambda: ["ZDIFF", "2", k(), k()],
    lambda: ["ZRANGESTORE", k(), k(), "0", "-1"],
    lambda: ["ZLEXCOUNT", k(), "-", "+"],
    lambda: ["ZMPOP", "2", k(), k(), random.choice(["MIN", "MAX"])],
    lambda: ["SORT", k()] + random.choice([[], ["ALPHA"], ["DESC", "ALPHA"], ["LIMIT", "0", "2"]]),
    lambda: ["SETBIT", k(), random.choice(["0", "7", "100"]), random.choice(["0", "1"])],
    lambda: ["GETBIT", k(), random.choice(["0", "7", "100"])],
    lambda: ["BITCOUNT", k()] + random.choice([[], ["0", "-1"], ["1", "2", "BIT"]]),
    lambda: ["BITPOS", k(), random.choice(["0", "1"])],
    lambda: ["BITOP", random.choice(["AND", "OR", "XOR", "NOT"]), k(), k()] + random.choice([[], [k()]]),
    lambda: ["PFADD", k(), v()],
    lambda: ["PFCOUNT", k()],
    lambda: ["LCS", k(), k()] + random.choice([[], ["LEN"]]),
    lambda: ["DBSIZE"],
    lambda: ["XADD", k(), "*", "f", v()],
    lambda: ["XLEN", k()],
    lambda: ["XTRIM", k(), "MAXLEN", random.choice(["0", "1", "5"])],
    lambda: ["GETEX", k()] + random.choice([[], ["PERSIST"], ["EX", "100"]]),
    lambda: ["INCRBY", k(), n()],
    lambda: ["HRANDFIELD", k(), "-5"],
    lambda: ["GEOADD", k()] + random.choice([[], ["NX"], ["XX"], ["CH"]]) + [random.choice(["13.36", "15.08", "200", "-180", "x"]), random.choice(["38.11", "37.5", "86", "-85.05112878"]), v()],
    lambda: ["GEODIST", k(), v(), v()] + random.choice([[], ["km"], ["mi"], ["xx"]]),
    lambda: ["GEOPOS", k(), v(), v()],
    lambda: ["GEOHASH", k(), v()],
    lambda: ["GEOSEARCH", k(), "FROMLONLAT", "15", "37", "BYRADIUS", random.choice(["100", "500", "-1"]), "km"] + random.choice([[], ["ASC"], ["WITHDIST", "ASC"], ["COUNT", "1", "ASC"], ["WITHCOORD", "WITHHASH", "DESC"]]),
    lambda: ["BITFIELD", k()] + random.choice([["GET", "u8", "0"], ["SET", "i8", "#1", n()], ["INCRBY", "u4", "2", n()], ["OVERFLOW", "SAT", "INCRBY", "i8", "0", "200"], ["OVERFLOW", "FAIL", "INCRBY", "u2", "0", "5"], ["GET", "i64", "0"], ["GET", "u64", "0"]]),
    lambda: ["BITFIELD_RO", k(), "GET", "u8", random.choice(["0", "8", "#2"])],
    lambda: ["HEXPIRE", k(), random.choice(["100", "0", "-1"])] + random.choice([[], ["NX"], ["XX"], ["GT"]]) + ["FIELDS", "1", random.choice(FIELDS)],
    lambda: ["HTTL", k(), "FIELDS", "2", random.choice(FIELDS), random.choice(FIELDS)],
    lambda: ["HPERSIST", k(), "FIELDS", "1", random.choice(FIELDS)],
    lambda: ["XADD", k()] + random.choice([[], ["NOMKSTREAM"], ["MAXLEN", "2"]]) + [random.choice(["1-1", "2-0", "2-*", "5", "0-0", "1-0"]), "f", v()],
    lambda: ["XRANGE", k(), "-", "+"] + random.choice([[], ["COUNT", "1"]]),
    lambda: ["XREVRANGE", k(), "+", "-"],
    lambda: ["XDEL", k(), random.choice(["1-1", "2-0", "5-0"])],
    lambda: ["XSETID", k(), random.choice(["10-0", "1-0", "0-0"])],
    lambda: ["ZRANGEBYLEX", k(), random.choice(["-", "[a", "(b"]), random.choice(["+", "[c", "(z"])],
    lambda: ["ZREVRANGEBYSCORE", k(), random.choice(SCORES), random.choice(SCORES), "WITHSCORES"],
    lambda: ["ZADD", k(), random.choice(SCORES), v(), random.choice(SCORES), v()],
    lambda: ["SUBSTR", k(), n(), n()],
    lambda: ["SETEX", k(), random.choice(["100", "0", "-5", "x"]), v()],
    lambda: ["PSETEX", k(), random.choice(["100000", "0"]), v()],
    lambda: ["EXPIRE", k(), "1000"],
    lambda: ["PTTL", k()],
    lambda: ["UNLINK", k()],
    lambda: ["TOUCH", k(), k()],
    lambda: ["RPUSHX", k(), v(), v()],
    lambda: ["ZRANDMEMBER", k(), "-3"],
    lambda: ["SRANDMEMBER", k(), "-3"],
    lambda: ["DUMP", k()],
    lambda: ["HVALS", k()],
    lambda: ["HMSET", k(), random.choice(FIELDS), v(), random.choice(FIELDS), v()],
    lambda: ["HPEXPIRE", k(), random.choice(["100000", "0"]), "FIELDS", "2", random.choice(FIELDS), random.choice(FIELDS)],
    lambda: ["HEXPIRETIME", k(), "FIELDS", "1", random.choice(FIELDS)],
    lambda: ["SPOP", k(), random.choice(["0", "-1"])],
    lambda: ["ZINTERCARD", "2", k(), k()] + random.choice([[], ["LIMIT", "1"], ["LIMIT", "-1"]]),
    lambda: ["ZDIFFSTORE", k(), "2", k(), k()],
    lambda: ["ZINTERSTORE", k(), "2", k(), k(), "WEIGHTS", random.choice(SCORES), "1", "AGGREGATE", random.choice(["SUM", "MIN", "MAX", "X"])],
    lambda: ["ZREVRANGEBYLEX", k(), random.choice(["+", "[c", "(z"]), random.choice(["-", "[a", "(b"])],
    lambda: ["ZREMRANGEBYLEX", k(), random.choice(["-", "[a", "(b"]), random.choice(["+", "[c", "(z"])],
    lambda: ["ZRANGESTORE", k(), k(), random.choice(SCORES), random.choice(SCORES), "BYSCORE"] + random.choice([[], ["REV"], ["LIMIT", "1", "1"]]),
    lambda: ["ZADD", k(), "GT", "LT", "1", v()],
    lambda: ["ZADD", k(), "NX", "INCR", random.choice(SCORES), v()],
    lambda: ["EXPIREAT", k(), random.choice(["1", "9999999999", "x"])],
    lambda: ["PEXPIRE", k(), random.choice(["100000", "-1"])] + random.choice([[], ["GT"], ["LT"], ["NX", "XX"]]),
    lambda: ["EXPIRETIME", k()],
    lambda: ["PFMERGE", k(), k(), k()],
    lambda: ["PFCOUNT", k(), k()],
    lambda: ["SETRANGE", k(), random.choice(["5", "20", "x", "-1"]), random.choice(VALS)],
    lambda: ["GETRANGE", k(), random.choice(["0", "-100", "5"]), random.choice(["-1", "100", "-100"])],
    lambda: ["XGROUP", "CREATE", k(), "g", random.choice(["$", "0", "x"])] + random.choice([[], ["MKSTREAM"], ["ENTRIESREAD", "1"]]),
    lambda: ["XREADGROUP", "GROUP", "g", random.choice(["c1", "c2"]), "COUNT", "2", "STREAMS", k(), random.choice([">", "0"])],
    lambda: ["XACK", k(), "g", random.choice(["1-1", "2-0", "5-0"])],
    lambda: ["XPENDING", k(), "g"],
    lambda: ["XPENDING", k(), "g", "-", "+", "10"] + random.choice([[], ["c1"]]),
    lambda: ["XCLAIM", k(), "g", "c2", "0", random.choice(["1-1", "2-0"])] + random.choice([[], ["JUSTID"], ["FORCE"]]),
    lambda: ["XAUTOCLAIM", k(), "g", "c2", "0", "0"] + random.choice([[], ["COUNT", "1"], ["JUSTID"]]),
    lambda: ["XGROUP", random.choice(["DESTROY", "CREATECONSUMER", "DELCONSUMER"]), k(), "g"] + random.choice([[], ["c1"]]),
    lambda: ["XGROUP", "SETID", k(), "g", random.choice(["0", "$", "1-1"])],
    lambda: ["XTRIM", k(), "MINID", random.choice(["1-1", "2", "0"])],
    lambda: ["XREAD", "COUNT", "2", "STREAMS", k(), random.choice(["0", "1-1", "$"])],
    lambda: ["LMPOP", random.choice(["0", "1", "x"]), k(), "LEFT"],
    lambda: ["SINTERCARD", random.choice(["0", "3", "x"]), k(), k()],
    lambda: ["GETEX", k(), random.choice(["EX", "PX", "EXAT"]), random.choice(["0", "-1", "100000", "x"])],
    lambda: ["SET", k(), v(), random.choice(["EX", "PX"]), random.choice(["0", "-1", "x"])],
    lambda: ["SET", k(), v(), "NX", "XX"],
    lambda: ["BITCOUNT", k(), random.choice(["0", "-2"]), random.choice(["-1", "5"]), random.choice(["BYTE", "BIT", "X"])],
    lambda: ["BITPOS", k(), random.choice(["0", "1"]), random.choice(["0", "2", "-1"]), random.choice(["-1", "10"])] + random.choice([[], ["BIT"], ["BYTE"]]),
    lambda: ["LINSERT", k(), "BEFORE", v(), v()],
    lambda: ["LPOS", k(), v(), "RANK", random.choice(["1", "-2", "2"]), "COUNT", random.choice(["0", "2"]), "MAXLEN", random.choice(["0", "2"])],
    lambda: ["SORT", k(), "BY", "nosort"] + random.choice([[], ["DESC"], ["STORE", k()]]),
    lambda: ["SORT_RO", k()] + random.choice([[], ["ALPHA"]]),
    lambda: ["RENAME", k(), k()],
    lambda: ["MOVE", k(), "1"],
    lambda: ["INCRBYFLOAT", k(), random.choice(["1e308", "-1e308", "0.1", "nan"])],
    lambda: ["DECR", k()],
    lambda: ["LRANGE", k(), "0", "-1"],
    lambda: ["SMEMBERS", k()],
    lambda: ["ZRANGE", k(), "0", "-1", "WITHSCORES"],
    lambda: ["HGETALL", k()],
    lambda: ["XRANGE", k(), "-", "+"],
    lambda: ["MULTI"],
    lambda: ["EXEC"],
    lambda: ["DISCARD"],
    lambda: ["WATCH", k()],
    lambda: ["UNWATCH"],
    lambda: ["BLPOP", k(), k(), "0.01"],
    lambda: ["BRPOP", k(), "0.01"],
    lambda: ["BLMOVE", k(), k(), "LEFT", "RIGHT", "0.01"],
    lambda: ["BZPOPMIN", k(), "0.01"],
    lambda: ["BZMPOP", "0.01", "1", k(), "MAX", "COUNT", "2"],
    lambda: ["BLMPOP", "0.01", "2", k(), k(), "LEFT"],
    lambda: ["BLPOP", k(), random.choice(["-1", "x", "0.000001"])],
    lambda: ["EVAL", random.choice(LUA), "1", k()],
    lambda: ["EVAL_RO", random.choice(LUA), "1", k()],
    lambda: ["LCS", k(), k(), "IDX"] + random.choice([[], ["MINMATCHLEN", "2"], ["WITHMATCHLEN"]]),
    lambda: ["SET", k(), random.choice(["12345", "-7", "1e5", " 1", "abcdefghij"])],
]

LUA = [
    "return redis.call('incr', KEYS[1])",
    "return redis.pcall('lpush', KEYS[1], 'x')",
    "return redis.call('get', KEYS[1])",
    "return {1, 2, {3, 'x'}, nil, 4}",
    "return redis.status_reply('FINE')",
    "return redis.error_reply('MYERR bad')",
    "return 3.99",
    "return true",
    "return false",
    "return {err='E1 x'}",
    "return redis.call('hgetall', KEYS[1])",
    "return redis.call('zrange', KEYS[1], 0, -1, 'WITHSCORES')",
    "return redis.call('type', KEYS[1])",
    "return redis.pcall('nosuch')",
    "return redis.call('set', KEYS[1], 'v', 'GET')",
    "return tostring(redis.call('exists', KEYS[1]))",
    "return redis.call('smembers', KEYS[1])",
    "redis.call('del', KEYS[1]); return redis.call('rpush', KEYS[1], 1, 2.5, 'z')",
    "return tostring(tonumber(redis.call('get', KEYS[1])))",
    "return redis.call('set', KEYS[1], tostring(1/3) .. (2^53))",
    "return string.format('%5.2f|%d|%x|%q', 1/3, 7, -1, redis.call('type', KEYS[1])['ok'])",
    "return struct.unpack('>I2', redis.call('get', KEYS[1]) or 'zz')",
    "local v = redis.call('get', KEYS[1]) return v + 1",
    "local t = redis.call('lrange', KEYS[1], 0, -1) table.sort(t) return t",
    "math.randomseed(#KEYS[1]) return {math.random(100), tostring(math.random())}",
]

UNORDERED = {"SMEMBERS", "SINTER", "SUNION", "SDIFF", "HGETALL", "HKEYS", "HVALS", "KEYS"}
SKIP_RESULT = {"HRANDFIELD", "ZRANDMEMBER", "SRANDMEMBER"}


def auto_ids(r):
    # IDs that XADD * took from the clock differ by a millisecond or so.
    if isinstance(r, bytes) and len(r) > 13 and r[:13].isdigit() and r[13:14] == b"-":
        return b"<ms>" + r[13:]
    if isinstance(r, list):
        return [auto_ids(x) for x in r]
    return r


# FUZZ_ORDER=1 compares the order of hash and set replies too: the fuzzer's
# collections are small enough for Redis' listpack and intset encodings, whose
# order CacheStorm follows.
KEEP_ORDER = os.environ.get("FUZZ_ORDER") == "1"


def unordered3(r):
    # RESP3 maps and sets have no order.
    if KEEP_ORDER:
        return r
    if isinstance(r, tuple) and r and r[0] == "map":
        return ("map", sorted(((unordered3(k), unordered3(v)) for k, v in r[1]), key=repr))
    if isinstance(r, tuple) and r and r[0] == "set":
        return ("set", sorted((unordered3(x) for x in r[1]), key=repr))
    if isinstance(r, list):
        return [unordered3(x) for x in r]
    return r


def norm(cmd, r):
    name = cmd[0]
    r = unordered3(auto_ids(r))
    if name in SKIP_RESULT and isinstance(r, tuple) and r[0] in ("map", "set"):
        return (r[0], len(r[1]))
    if name in SKIP_RESULT and isinstance(r, list):
        return ("list", len(r))
    if name == "XADD" and isinstance(r, bytes) and cmd[-3] in ("*", "2-*"):
        return "id"
    if name == "DUMP":
        return r is None
    if name in ("TTL", "PTTL", "HTTL"):
        if isinstance(r, int):
            return r if r < 0 else "pos"
        if isinstance(r, list):
            return [x if x < 0 else "pos" for x in r]
    if name == "OBJECT":
        return r
    if name in ("EVAL", "EVAL_RO") and isinstance(r, list):
        if "hgetall" in cmd[1]:
            name = "HGETALL"
        elif "smembers" in cmd[1]:
            name = "SMEMBERS"
    if name in UNORDERED and isinstance(r, list) and not (KEEP_ORDER and name != "KEYS"):
        if name == "HGETALL":
            pairs = sorted(zip(r[0::2], r[1::2]))
            return pairs
        return sorted(r, key=lambda x: (x is None, x))
    if name == "SORT" and "nosort" in cmd and "STORE" not in cmd and isinstance(r, list) and not KEEP_ORDER:
        # A set has no native order.
        return sorted(r)
    if name == "SINTERCARD":
        return r
    if isinstance(r, tuple) and r[0] == "-":
        return ("-", r[1].split(" ")[0])
    return r


def drain(sc):
    out = []
    sc.s.sendall(b"*1\r\n$4\r\nPING\r\n")
    while True:
        m = sc.read()
        if m == [b"pong", b""]:
            return out
        out.append((m[2], m[3]))


def main():
    seed = int(sys.argv[1])
    count = int(sys.argv[2])
    full_err = len(sys.argv) > 3 and sys.argv[3] == "full"
    random.seed(seed)
    a, b = Conn(int(os.environ.get("CACHESTORM_PORT", "6390"))), Conn(int(os.environ.get("REDIS_PORT", "6391")))
    if os.environ.get("FUZZ_RESP3"):
        a.cmd("HELLO", "3")
        b.cmd("HELLO", "3")
    a.cmd("FLUSHALL")
    b.cmd("FLUSHALL")
    subs = None
    if os.environ.get("FUZZ_NOTIFY"):
        # Keyspace notifications are compared too: each subscriber is
        # drained up to the reply of a PING sent after the command.
        subs = []
        for c, port in ((a, os.environ.get("CACHESTORM_PORT", "6390")), (b, os.environ.get("REDIS_PORT", "6391"))):
            c.cmd("CONFIG", "SET", "notify-keyspace-events", "KEA")
            sc = Conn(int(port))
            sc.cmd("PSUBSCRIBE", "__key*__:*")
            subs.append(sc)
    history = []
    in_multi = False
    queued = []
    for i in range(count):
        c = gen()
        ra, rb = a.cmd(*c), b.cmd(*c)
        history.append(c)
        if subs:
            events = [drain(sc) for sc in subs]
            if events[0] != events[1]:
                print("seed", seed, "step", i, "NOTIFY DIVERGE:", " ".join(c))
                print("  cachestorm:", events[0])
                print("  redis     :", events[1])
                for h in history[-int(os.environ.get("FUZZ_HISTORY", "8")):-1]:
                    print("    prev:", " ".join(h))
                return 1
        if c[0] == "EXEC" and isinstance(ra, list) and isinstance(rb, list) and len(ra) == len(queued) == len(rb):
            na = [norm(q, x) for q, x in zip(queued, ra)]
            nb = [norm(q, x) for q, x in zip(queued, rb)]
        else:
            na, nb = norm(c, ra), norm(c, rb)
        if in_multi and ra == ("+", "QUEUED"):
            queued.append(c)
        if full_err and isinstance(ra, tuple) and isinstance(rb, tuple) and ra[0] == rb[0] == "-":
            na, nb = ra, rb
        if c[0] == "MULTI" and ra == ("+", "OK"):
            in_multi, queued = True, []
        elif c[0] in ("EXEC", "DISCARD"):
            in_multi = False
        if na == nb and not in_multi:
            ta = [a.cmd("TYPE", x) for x in KEYS]
            tb = [b.cmd("TYPE", x) for x in KEYS]
            if ta != tb:
                na, nb = ("state", ta), ("state", tb)
        if na != nb:
            print("seed", seed, "step", i, "DIVERGE:", " ".join(c))
            print("  cachestorm:", ra)
            print("  redis     :", rb)
            for h in history[-int(os.environ.get("FUZZ_HISTORY", "8")):-1]:
                print("    prev:", " ".join(h))
            return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
