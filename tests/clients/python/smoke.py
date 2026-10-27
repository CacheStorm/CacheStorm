"""Exercises CacheStorm with redis-py over RESP2 and RESP3, including
RESP3 client-side caching driven by CLIENT TRACKING invalidations."""
import os, threading, time, redis
from redis.cache import CacheConfig

host, port = os.environ["CACHESTORM_ADDR"].rsplit(":", 1)
password = os.environ.get("CACHESTORM_PASSWORD")

for proto in (2, 3):
    r = redis.Redis(host=host, port=int(port), password=password, db=1, protocol=proto, decode_responses=True)
    r.ping()
    r.delete("n", "h", "l", "q", "s", "st")
    r.set("k", "v", ex=60)
    assert r.get("k") == "v"
    assert r.get("missing") is None
    p = r.pipeline()
    p.incr("n"); p.hset("h", mapping={"a": 1, "b": 2}); p.hgetall("h")
    res = p.execute()
    assert res[2] == {"a": "1", "b": "2"}, res
    with r.pipeline(transaction=True) as t:
        t.lpush("l", "x"); t.llen("l"); out = t.execute()
    ps = r.pubsub(); ps.subscribe("ch"); ps.get_message(timeout=1)
    r.publish("ch", "hello")
    m = ps.get_message(timeout=2)
    assert m and m["data"] == "hello", m
    assert r.eval("return redis.call('GET', KEYS[1])", 1, "k") == "v"
    threading.Timer(0.2, lambda: r.rpush("q", "job")).start()
    got = r.blpop("q", timeout=2)
    assert got and tuple(got) == ("q", "job"), got
    r.sadd("s", *range(100))
    assert len(set(r.sscan_iter("s", count=10))) == 100
    r.xadd("st", {"f": "v"}); r.xgroup_create("st", "g", id="0")
    got = r.xreadgroup("g", "c", {"st": ">"})
    # RESP2 returns [[stream, entries]], RESP3 a map {stream: [entries]}.
    entries = got["st"][0] if proto == 3 else got[0][1]
    assert entries and entries[0][1] == {"f": "v"}, got
    cmds = r.command()
    assert cmds["get"]["arity"] == 2 and "readonly" in cmds["get"]["flags"], cmds["get"]
    assert r.command_getkeys("MSET", "a", "1", "b", "2") == ["a", "b"]
    docs = r.execute_command("COMMAND DOCS", "get", "client|kill")
    assert docs, docs
    info = r.execute_command("COMMAND INFO", "get", "client|kill")
    assert len(info) == 2, info
    kf = r.execute_command("COMMAND GETKEYSANDFLAGS", "LMOVE", "a", "b", "LEFT", "RIGHT")
    assert len(kf) == 2 and kf[0][0] == "a", kf
    print(f"redis-py protocol={proto}: all checks passed")

# Client-side caching: the cached GET must be refreshed once another client
# changes the key, which only works if the server sends the invalidation.
cached = redis.Redis(host=host, port=int(port), password=password, db=1, protocol=3,
                     decode_responses=True, cache_config=CacheConfig())
writer = redis.Redis(host=host, port=int(port), password=password, db=1, decode_responses=True)
writer.set("csc", "one")
assert cached.get("csc") == "one"
assert cached.get("csc") == "one"
writer.set("csc", "two")
deadline = time.time() + 3
while cached.get("csc") != "two":
    assert time.time() < deadline, "client-side cache kept a stale value"
    time.sleep(0.05)
print("redis-py client-side caching: invalidation received")
