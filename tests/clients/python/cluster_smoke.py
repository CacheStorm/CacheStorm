"""Exercises a single cluster-mode CacheStorm node with redis-py RedisCluster."""
import os, redis
from redis.cluster import RedisCluster

host, port = os.environ["CACHESTORM_CLUSTER_ADDR"].rsplit(":", 1)
password = os.environ.get("CACHESTORM_PASSWORD")

plain = redis.Redis(host=host, port=int(port), password=password, decode_responses=True)
if int(plain.cluster("info")["cluster_slots_assigned"]) == 0:
    plain.execute_command("CLUSTER", "ADDSLOTSRANGE", 0, 16383)

for proto in (2, 3):
    rc = RedisCluster(host=host, port=int(port), password=password, protocol=proto, decode_responses=True)
    rc.set("{py}.a", "1")
    rc.set("{py}.b", "2")
    assert rc.get("{py}.a") == "1"
    assert rc.mget_nonatomic("{py}.a", "{py}.b", "elsewhere") == ["1", "2", None]
    p = rc.pipeline()
    p.incr("{py}.n"); p.get("{py}.b"); p.set("other", "x")
    res = p.execute()
    assert res[1] == "2" and res[2] is True, res
    assert rc.keyslot("foo") == 12182
    assert len(rc.cluster_slots()) == 1
    rc.delete("{py}.a", "{py}.b", "{py}.n")
    rc.delete("other")
    rc.close()
    print(f"redis-py RedisCluster protocol={proto}: all checks passed")
