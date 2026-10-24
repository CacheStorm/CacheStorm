"""Exercises CacheStorm with redis-py over RESP2 and checks RESP3 is refused."""
import os, threading, redis

host, port = os.environ["CACHESTORM_ADDR"].rsplit(":", 1)
password = os.environ.get("CACHESTORM_PASSWORD")
resp3_refused = False

for proto in (2, 3):
    try:
        r = redis.Redis(host=host, port=int(port), password=password, db=1, protocol=proto, decode_responses=True)
        r.ping()
    except redis.ResponseError as e:
        # CacheStorm speaks RESP2 only; HELLO 3 is refused with NOPROTO.
        assert proto == 3 and "NOPROTO" in str(e), e
        resp3_refused = True
        print("redis-py protocol=3: refused with NOPROTO as expected")
        continue
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
    assert r.blpop("q", timeout=2) == ("q", "job")
    r.sadd("s", *range(100))
    assert len(set(r.sscan_iter("s", count=10))) == 100
    r.xadd("st", {"f": "v"}); r.xgroup_create("st", "g", id="0")
    got = r.xreadgroup("g", "c", {"st": ">"})
    assert got and got[0][1][0][1] == {"f": "v"}, got
    print(f"redis-py protocol={proto}: all checks passed")
assert resp3_refused, "HELLO 3 was not refused"
