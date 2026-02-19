# CacheStorm — Master Project Prompt for Claude Code

## Project Overview

**CacheStorm** is a high-performance, Redis-compatible in-memory cache server written from scratch in Go. It is NOT a wrapper around any existing library. Every single component — RESP protocol parser, TCP server, storage engine, eviction, clustering — is implemented from scratch with zero external dependencies for core functionality.

**Repository:** github.com/cachestorm/cachestorm
**Website:** cachestorm.com
**License:** Apache 2.0
**Go Version:** 1.22+
**Final Target:** v1.0.0

## Core Philosophy

1. **Zero dependency core** — The core server has NO external dependencies. Only clustering (memberlist) and optional plugins may import third-party packages.
2. **Redis compatible** — Any Redis client (ioredis, go-redis, redis-py, jedis) can connect and use basic commands.
3. **Cache-first** — This is NOT a general-purpose database. It is optimized for caching use cases.
4. **Tag invalidation as first-class citizen** — The killer feature. Native tag-based cache invalidation that Redis lacks.
5. **Plugin-driven extensibility** — Core is minimal, everything else is a plugin.
6. **Production-ready from day one** — Proper error handling, logging, metrics, graceful shutdown, config validation.

## What Makes CacheStorm Different from Redis

| Feature | Redis | CacheStorm |
|---------|-------|------------|
| Tag-based invalidation | ❌ Manual workaround | ✅ Native, first-class |
| Plugin system | ❌ Modules (C only) | ✅ Go interface-based |
| Named namespaces | ❌ Numbered databases | ✅ Named namespaces |
| Hot key detection | ❌ Manual | ✅ Built-in |
| Memory inspector | ❌ Limited | ✅ Per-namespace, per-tag |
| Cascade invalidation | ❌ | ✅ Tag hierarchies |
| Config | redis.conf | YAML with validation |

## Tech Stack

- **Language:** Go 1.22+
- **Protocol:** RESP3 (Redis Serialization Protocol v3)
- **Storage:** Custom sharded concurrent hashmap (256 shards, RWMutex per shard)
- **TTL:** Hierarchical timing wheel
- **Eviction:** LRU (default), LFU, W-TinyLFU
- **Clustering:** Gossip protocol via HashiCorp memberlist
- **Persistence:** Custom AOF + snapshot (as plugin)
- **Config:** YAML (gopkg.in/yaml.v3)
- **Logging:** zerolog (rs/zerolog)
- **CLI flags:** stdlib flag package only
- **Testing:** stdlib testing + testify for assertions only
- **Build:** Standard Go toolchain, Makefile, GoReleaser

## CRITICAL RULES FOR IMPLEMENTATION

1. **NEVER use `interface{}` or `any` without type assertion safety.** Always use concrete types or well-defined interfaces.
2. **NEVER ignore errors.** Every error must be handled or explicitly documented why it's ignored.
3. **NEVER use global state.** Everything is passed through dependency injection.
4. **ALWAYS use context.Context for cancellation propagation.**
5. **ALWAYS implement graceful shutdown.** Every goroutine must be stoppable.
6. **ALWAYS validate config values at startup.** Fail fast with clear error messages.
7. **ALWAYS use structured logging (zerolog).** No fmt.Println in production code.
8. **Test every public function.** Minimum 80% coverage target.
9. **Benchmark critical paths:** RESP parsing, shard lookup, tag invalidation.
10. **Document every exported type and function with Go doc comments.**

## Naming Conventions

- Package names: lowercase, single word (`store`, `resp`, `cluster`)
- Interfaces: verb-noun or noun (`Commander`, `Store`, `Plugin`)
- Structs: PascalCase noun (`ShardMap`, `TimingWheel`, `TagIndex`)
- Methods: PascalCase verb (`Get`, `Set`, `Invalidate`)
- Private: camelCase (`shardIndex`, `lockAndGet`)
- Constants: PascalCase (`MaxShards`, `DefaultTTL`)
- Errors: `Err` prefix (`ErrKeyNotFound`, `ErrMemoryLimit`)
- Files: snake_case (`timing_wheel.go`, `tag_index.go`)
- Test files: `_test.go` suffix (`shard_test.go`)

## Port Assignments

- **6380** — Main RESP server (intentionally NOT 6379 to avoid Redis conflict)
- **7946** — Gossip/cluster communication
- **9090** — HTTP admin API + Prometheus metrics

## File References

Read these files in order for complete implementation details:

1. `01-ARCHITECTURE.md` — Detailed system architecture, data structures, memory layout
2. `02-PROTOCOL-SPEC.md` — Complete RESP3 protocol and all command specifications
3. `03-IMPLEMENTATION-PHASES.md` — Phase-by-phase implementation guide with exact steps
4. `04-PLUGIN-SYSTEM.md` — Plugin architecture, hooks, built-in plugins
5. `05-CLUSTER-SPEC.md` — Multi-node clustering, replication, tag broadcast
6. `06-CONFIG-AND-CLI.md` — Configuration schema, CLI flags, validation rules
7. `07-TESTING-AND-BENCHMARKS.md` — Test strategy, benchmark suite, CI/CD

## Project Structure

```
cachestorm/
├── cmd/
│   └── cachestorm/
│       └── main.go                    # Entry point
├── internal/
│   ├── server/
│   │   ├── server.go                  # TCP server, accept loop
│   │   ├── connection.go              # Per-client connection, read/write loop
│   │   ├── connection_pool.go         # Connection pool management
│   │   └── server_test.go
│   ├── resp/
│   │   ├── reader.go                  # RESP3 protocol reader/parser
│   │   ├── writer.go                  # RESP3 protocol writer/serializer
│   │   ├── types.go                   # RESP data types (SimpleString, Error, Integer, BulkString, Array, Map, etc.)
│   │   ├── reader_test.go
│   │   └── writer_test.go
│   ├── command/
│   │   ├── router.go                  # Command dispatch table
│   │   ├── context.go                 # CommandContext (shared state per command execution)
│   │   ├── string_commands.go         # SET, GET, MSET, MGET, DEL, EXISTS, INCR, DECR, APPEND, etc.
│   │   ├── hash_commands.go           # HSET, HGET, HMSET, HMGET, HDEL, HGETALL, HEXISTS, HLEN, HKEYS, HVALS, HINCRBY
│   │   ├── list_commands.go           # LPUSH, RPUSH, LPOP, RPOP, LLEN, LRANGE, LINDEX, LSET, LREM
│   │   ├── set_commands.go            # SADD, SREM, SMEMBERS, SISMEMBER, SCARD, SUNION, SINTER, SDIFF
│   │   ├── tag_commands.go            # SETTAG, TAGS, ADDTAG, REMTAG, INVALIDATE, TAGKEYS, TAGCOUNT
│   │   ├── namespace_commands.go      # NAMESPACE, NAMESPACES
│   │   ├── server_commands.go         # PING, ECHO, INFO, DBSIZE, FLUSHDB, FLUSHALL, SELECT, AUTH, COMMAND, HOTKEYS, MEMINFO
│   │   ├── key_commands.go            # EXPIRE, PEXPIRE, TTL, PTTL, PERSIST, RENAME, TYPE, KEYS, SCAN, RANDOMKEY
│   │   ├── cluster_commands.go        # CLUSTER INFO, CLUSTER NODES, CLUSTER SLOTS, CLUSTER MEET, CLUSTER REPLICATE
│   │   ├── string_commands_test.go
│   │   ├── hash_commands_test.go
│   │   ├── list_commands_test.go
│   │   ├── set_commands_test.go
│   │   └── tag_commands_test.go
│   ├── store/
│   │   ├── store.go                   # Store interface + main implementation
│   │   ├── namespace.go               # Namespace manager
│   │   ├── shard.go                   # Sharded concurrent hashmap
│   │   ├── entry.go                   # Entry struct + Value interface + concrete types (StringValue, HashValue, ListValue, SetValue)
│   │   ├── tag_index.go               # Bidirectional tag index (forward + reverse)
│   │   ├── timing_wheel.go            # Hierarchical timing wheel for TTL
│   │   ├── eviction.go                # Eviction controller (LRU, LFU)
│   │   ├── memory.go                  # Memory tracker + pressure monitoring
│   │   ├── shard_test.go
│   │   ├── tag_index_test.go
│   │   ├── timing_wheel_test.go
│   │   └── eviction_test.go
│   ├── cluster/
│   │   ├── cluster.go                 # Cluster manager
│   │   ├── gossip.go                  # Node discovery via memberlist
│   │   ├── hash_slots.go              # Hash slot allocation and routing (16384 slots)
│   │   ├── replication.go             # Primary-replica replication
│   │   ├── tag_broadcast.go           # Cross-node tag invalidation
│   │   ├── migration.go               # Slot migration during rebalance
│   │   └── cluster_test.go
│   ├── plugin/
│   │   ├── manager.go                 # Plugin lifecycle management (load, init, close)
│   │   ├── hooks.go                   # Hook interface definitions
│   │   ├── registry.go                # Plugin registration
│   │   ├── pipeline.go                # Hook execution pipeline (ordered)
│   │   └── manager_test.go
│   ├── config/
│   │   ├── config.go                  # Config struct + YAML parsing
│   │   ├── defaults.go                # Default values
│   │   ├── validation.go              # Config validation rules
│   │   └── config_test.go
│   └── logger/
│       └── logger.go                  # Zerolog wrapper + initialization
├── plugins/
│   ├── stats/
│   │   ├── stats.go                   # Hit/miss ratio, command counts, latency percentiles
│   │   └── stats_test.go
│   ├── persistence/
│   │   ├── aof.go                     # Append-only file writer + reader
│   │   ├── snapshot.go                # Periodic full dump (binary format)
│   │   ├── recovery.go                # Startup recovery from AOF/snapshot
│   │   ├── persistence.go             # Plugin entry point
│   │   └── persistence_test.go
│   ├── metrics/
│   │   ├── metrics.go                 # Prometheus metrics exporter
│   │   └── metrics_test.go
│   ├── auth/
│   │   ├── auth.go                    # Password + ACL authentication
│   │   └── auth_test.go
│   └── slowlog/
│       ├── slowlog.go                 # Slow query logger
│       └── slowlog_test.go
├── config/
│   └── cachestorm.example.yaml        # Example configuration file
├── scripts/
│   ├── build.sh
│   └── benchmark.sh
├── benchmarks/
│   ├── resp_bench_test.go
│   ├── store_bench_test.go
│   └── tag_bench_test.go
├── docker/
│   ├── Dockerfile
│   └── docker-compose.yml             # 3-node cluster example
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── release.yml
├── .goreleaser.yml
├── .gitignore
├── LICENSE
├── README.md
├── CHANGELOG.md
├── Makefile
├── go.mod
└── go.sum
```

## External Dependencies (Minimal)

### Core (only these):
- `github.com/rs/zerolog` — Structured logging
- `gopkg.in/yaml.v3` — Config parsing

### Cluster (optional, only when cluster enabled):
- `github.com/hashicorp/memberlist` — Gossip protocol

### Plugins (each plugin may have its own):
- `github.com/prometheus/client_golang` — metrics plugin only

### Testing:
- `github.com/stretchr/testify` — assertions only

**Total: 4 external dependencies for the entire project.** Everything else is implemented from scratch.
