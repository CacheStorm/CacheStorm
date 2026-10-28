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
]

UNORDERED = {"SMEMBERS", "SINTER", "SUNION", "SDIFF", "HGETALL", "HKEYS", "HVALS", "KEYS"}
SKIP_RESULT = {"HRANDFIELD", "ZRANDMEMBER", "SRANDMEMBER"}


def norm(cmd, r):
    name = cmd[0]
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
    if name in UNORDERED and isinstance(r, list):
        if name == "HGETALL":
            pairs = sorted(zip(r[0::2], r[1::2]))
            return pairs
        return sorted(r, key=lambda x: (x is None, x))
    if name == "SINTERCARD":
        return r
    if isinstance(r, tuple) and r[0] == "-":
        return ("-", r[1].split(" ")[0])
    return r


def main():
    seed = int(sys.argv[1])
    count = int(sys.argv[2])
    full_err = len(sys.argv) > 3 and sys.argv[3] == "full"
    random.seed(seed)
    a, b = Conn(int(os.environ.get("CACHESTORM_PORT", "6390"))), Conn(int(os.environ.get("REDIS_PORT", "6391")))
    a.cmd("FLUSHALL")
    b.cmd("FLUSHALL")
    history = []
    for i in range(count):
        c = gen()
        ra, rb = a.cmd(*c), b.cmd(*c)
        history.append(c)
        na, nb = norm(c, ra), norm(c, rb)
        if full_err and isinstance(ra, tuple) and isinstance(rb, tuple):
            na, nb = ra, rb
        if na == nb:
            ta = [a.cmd("TYPE", x) for x in KEYS]
            tb = [b.cmd("TYPE", x) for x in KEYS]
            if ta != tb:
                na, nb = ("state", ta), ("state", tb)
        if na != nb:
            print("seed", seed, "step", i, "DIVERGE:", " ".join(c))
            print("  cachestorm:", ra)
            print("  redis     :", rb)
            for h in history[-8:-1]:
                print("    prev:", " ".join(h))
            return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
