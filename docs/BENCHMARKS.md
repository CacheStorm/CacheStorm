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

## Tags against Redis 7.4, 2026-11-26

The tag workload on Redis 7.4.12 (`redis:7.4-alpine`, `--network host`, no persistence) the way it is usually done there — a set of keys per tag, `SET` plus `SADD` for a tagged write, and a Lua script running `SMEMBERS` then `DEL` in chunks of 5,000 for an invalidation — against `SET ... TAGS` and `INVALIDATE` on CacheStorm, from the same Go load generator on the same host (8-core Ryzen 7 PRO 6850H, loaded by other work, so single runs vary by ±15%).

| Workload | Redis 7.4.12 | CacheStorm |
|---|---|---|
| Tagged writes, 8 connections × 32 pipelined, one of 1,000 tags | 425k/s | 713k/s |
| Cache mix per 10 operations: 8 `GET`, 1 tagged write, 1 invalidation of a 10-key entity tag (8 connections) | 686k ops/s | 1,180k ops/s |
| Invalidate a 10-key tag, median of 301 / p90 | 122 µs / 182 µs | 92 µs / 126 µs |
| Invalidate a 1,000-key tag, median | 625 µs | 332 µs |
| Invalidate a 50,000-key tag, median | 31.8 ms | 4.7 ms |
| Invalidate a 200,000-key tag, median | 149 ms | 23 ms |
| Memory per key, 16-byte value, untagged: allocated / RSS | 87 B / 86 B | 124 B / 136 B |
| Memory per key, 16-byte value, one of 200 tags: allocated / RSS | 120 B / 117 B | 151 B / 186 B |

While Redis runs the script every other client waits, as it waits while CacheStorm's whole-gate invalidation runs; the waits follow the invalidation times above. Memory is where Redis is still ahead, on the key itself: a tag costs a key 25 bytes on CacheStorm and 33 on Redis, but an untagged string key 124 bytes against 87 (Go's map slot, the 48-byte entry and the separately allocated key and value), and the Go heap keeps more free memory around the live data than jemalloc does. Memory is measured after `MEMORY PURGE`, as the change in the allocator's figure (`used_memory_heap` on CacheStorm, `allocator_allocated` on Redis) and in RSS while 200,000 keys are added.

`redis-benchmark` (from the same image, one million random keys, 50 clients, pipelines of 16) on the same run: SET 830k vs 436k/s, GET 878k vs 626k/s, INCR 876k vs 650k/s, LPUSH 552k vs 443k/s; `HSET`, which `redis-benchmark` sends to one hash, 450k vs 580k/s. On one hot key the 50 connections queue on one stripe of the write gate and most of the time goes to handing it from goroutine to goroutine, which single-threaded Redis never does.

The tag set of each tag was a Go `map[string]struct{}`, about 55 bytes per member at the sizes tags have; it is now a slice of the key strings, open-addressed and grown by half (a plain slice up to 8 members), about 25 bytes per member. That also made `SET ... TAGS` 36% faster and invalidating 50,000 keys a third faster, as the members are read from one array.

## Large invalidations, 2026-11-26

An `INVALIDATE` of more than 256 keys holds the whole write gate, so its length is how long every other client waits. Measured as the longest `GET` of four clients running beside it, which matched the invalidation's own time:

| Tag size | 0.5.0 | Now |
|---|---|---|
| 1,000 keys | 0.9 ms | 0.25 ms (alone) |
| 10,000 keys | 3.1 ms | 1.5 ms |
| 50,000 keys | 10.5–13 ms | 5.3–6.5 ms |
| 200,000 keys | 44–49 ms | 21–24 ms |

What changed: bucketing the keys by shard runs on the worker goroutines too (it was a sequential pass hashing every key while everything else waited); the workers' result slices are sized up front; the keys are not looked for again in the tag that was just invalidated (the index dropped it already, and a key with that one tag now costs no tag-index work at all); batches from 512 keys (was 2,048) use the workers, as below that the sequential path was slower; and the `DEL`s recorded for replicas and the AOF use the key strings' bytes instead of copying them. An `INVALIDATE` reaching up to 256 keys (was 32) and at most three quarters of the 256 gate stripes holds only those stripes, so other clients keep running beside it: with two clients invalidating 50-key tags in a loop, six other pipelining clients wrote 12% more.

## Tags, 2026-11-23

Tag writes and invalidation (0.4.0 and 0.5.0), from a Go load generator with 8 connections each pipelining 32 commands against a server on the same host; operations per second unless noted.

| Operation | Before | After |
|---|---|---|
| `SETTAG key value t u` | 290k | ~800k |
| `ADDTAG key t` | 340k | ~950k |
| `INVALIDATE` of a 10-key tag | 88k | ~270k |
| 8 `GET` + `SETTAG` + `INVALIDATE` per 10 commands | 635k commands/s | 1.45M commands/s |
| `INVALIDATE` of a 50,000-key tag | 37 ms | ~9 ms |
| `DEL` of untagged keys with 1,050 tags in the database | 61k | 1.24M |
| Memory a tag costs a key | ~110 B | ~55 B |

For comparison, the usual Redis pattern — a set per tag, and a Lua script running `SMEMBERS` then `DEL` — took about 36 ms for the 50,000-key tag when run on CacheStorm itself (no Redis server was available for this round; the script's work is the same on both).

An `INVALIDATE` reaching at most 32 keys locks only those keys' stripes of the write gate, so other clients keep writing while it runs; a larger one holds the whole gate, and a batch of 2,048 keys or more is deleted by up to 8 goroutines over separate shards. In-process equivalents:

```bash
go test ./internal/command/ -run '^$' -bench 'Invalidate|TagWrites' -benchmem
```

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
