# Benchmarks

End-to-end throughput over TCP, measured with `redis-benchmark` from the `redis:7.4-alpine` image. The figures compare builds against each other on one machine; they are not a capacity guarantee for other hardware.

## Environment

- AMD Ryzen 7 PRO 6850H (8 cores, 16 threads), Linux, Go 1.26.10
- Server and benchmark client on the same host, loopback, `--network host`
- CacheStorm with HTTP, metrics and persistence off, `protected_mode: false`, log level `warn`
- Redis 7.4.11 with `--save '' --appendonly no`
- 50 clients. SET, GET and INCR send 100,000 requests; LPUSH and LPOP send 50,000. P1 is unpipelined, P16 pipelines 16 commands

```bash
docker run --rm --network host redis:7.4-alpine \
  redis-benchmark -p <port> -q -n 100000 -c 50 -P 16 -t set,get,incr
docker run --rm --network host redis:7.4-alpine \
  redis-benchmark -p <port> -q -n 50000 -c 50 -P 16 -t lpush,lpop
```

## Results, 2026-10-25 (requests per second)

| Command | CacheStorm P1 | Redis P1 | CacheStorm P16 | Redis P16 |
|---|---|---|---|---|
| SET | 59,559 | 57,110 | 286,533 | 666,667 |
| GET | 57,703 | 59,032 | 641,026 | 636,943 |
| INCR | 57,670 | – | 236,407 | – |
| LPUSH | 57,937 | 53,050 | 297,619 | 350,877 |
| LPOP | 58,893 | – | 308,642 | – |

Unpipelined, both servers are bound by round trips and perform alike. Pipelined reads match Redis; pipelined writes reach 40–85% of Redis, because every write holds a stripe of the write gate that orders it with the AOF and the replication stream.

## History

| Change | SET P16 | GET P16 | LPUSH P1 | LPUSH P16 |
|---|---|---|---|---|
| 58d78e2 (before the production-readiness work) | 215,054 | 258,398 | 1,574 | 2,297 |
| LPUSH prepends in amortised O(1) (6942e2a) | 313,480 | 371,747 | 55,556 | 352,113 |
| Replication write gate (6799516) | 71,429 | 404,858 | 54,054 | 73,421 |
| Gate without allocations, replies written after the gate (64155a3) | 286,533 | 641,026 | 57,937 | 297,619 |

LPUSH used to copy the whole list on every call, which made a list built with LPUSH quadratic. The replication merge put a socket write and several allocations inside every write's critical section; moving the reply out of the gate and batching a pipeline's replies into one write recovered that and more.

## Microbenchmarks

`benchmarks/` holds Go benchmarks for the RESP codec, the store and tags, and `internal/command` has `BenchmarkRPUSHOnLargeList`, which checks that a push stays constant-time on a 200,000-element list:

```bash
go test ./benchmarks/ -bench . -benchmem
go test ./internal/command/ -run '^$' -bench RPUSHOnLargeList -benchmem
```
