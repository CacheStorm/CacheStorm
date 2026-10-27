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

## Results, 2026-10-27 (requests per second, pipelined)

`redis-benchmark` without `-r` sends every SET, GET and INCR to the single key
`key:__rand_int__`, so the earlier SET figures measured one hot key rather
than a workload. These runs add `-r 1000000` (one million distinct keys,
1,000,000 requests, 50 clients, `-P 16`):

```bash
docker run --rm --network host redis:7.4-alpine \
  redis-benchmark -p <port> -q -n 1000000 -r 1000000 -c 50 -P 16 -t set,get,incr,lpush,hset
```

| Command | CacheStorm | Redis 7.4.11 | CacheStorm p50 | Redis p50 |
|---|---|---|---|---|
| SET | 869,565 | 627,353 | 0.50 ms | 1.10 ms |
| GET | 1,016,260 | 753,580 | 0.38 ms | 0.91 ms |
| INCR | 933,707 | 634,518 | 0.45 ms | 1.08 ms |
| LPUSH (one key) | 476,872 | 445,038 | 1.48 ms | 1.66 ms |
| HSET (one key, ~630k fields) | 351,741 | 557,725 | 2.09 ms | 1.28 ms |

Across many keys CacheStorm uses several cores and outruns single-threaded
Redis with half its median latency. `redis-benchmark` sends LPUSH to `mylist`
and HSET to `myhash` whatever `-r` is, so those rows are a single hot key:
every write to one key holds that key's stripe of the write gate.

Two changes lifted the hot-key rows (LPUSH from 307k, HSET from 197k). A
connection keeps its stripe across consecutive pipelined commands on the same
stripes, so a 16-command pipeline hands the gate to another connection once
instead of 16 times; the hold is never kept across a socket write, a blocking
command or a read that would wait for the network, and at most 64 commands
share one hold. The per-command path also stopped allocating a transaction
and an address string and stopped taking a process-wide lock for the
connection registry.

A third change took LPUSH past Redis (429k to 477k) and HSET from 319k to
352k. With fifty connections queued on one stripe for over a millisecond, Go's
sync.Mutex hands the lock to parked goroutines in FIFO order, and each
wakeup cost more than the 16 commands the hold then covers. A connection now
retries the stripe a bounded number of times, yielding in between, before it
parks; 64 retries measured best, 16 left half the gain and 256 starved the
holder. HSET's own path also stopped allocating a closure per call, and a
write no longer looks its key up twice. HSET on one hash with hundreds of
thousands of fields remains the known gap: each write is a lookup in one
large Go map, which Redis' encodings handle with fewer cache misses, and a
single key's writes run one at a time on any server.

The table is the median of three runs.

## Results, 2026-10-25 (single key, superseded)

| Command | CacheStorm P1 | Redis P1 | CacheStorm P16 | Redis P16 |
|---|---|---|---|---|
| SET | 59,559 | 57,110 | 286,533 | 666,667 |
| GET | 57,703 | 59,032 | 641,026 | 636,943 |
| INCR | 57,670 | – | 236,407 | – |
| LPUSH | 57,937 | 53,050 | 297,619 | 350,877 |
| LPOP | 58,893 | – | 308,642 | – |

Unpipelined, both servers are bound by round trips and perform alike.

## Snapshot pauses, 2026-10-27

One million 100-byte strings (138 MB of AOF). Writes wait for the encode
step; the transfer to a replica and, for BGSAVE, the disk write and checksum
run after the gate is released.

| Operation | Before | After |
|---|---|---|
| AOF rewrite (encode, write, fsync, swap) | 3.4 s | 0.48 s |
| Replica full sync (encode) | 2.2 s | 0.5 s |
| BGSAVE (encode) | 1.4 s | 0.52 s |

The pause still grows linearly with the dataset (about 0.5 µs per key).

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
