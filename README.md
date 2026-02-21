<div align="center">
  <img src="https://avatars.githubusercontent.com/u/262622049?s=400&u=a2e56c80726cb8a3ae6fc8f8622be5173b7b2848&v=4" alt="CacheStorm Logo" width="180" height="180">
  
  # CacheStorm
  
  **High-Performance, Redis-Compatible In-Memory Database**
  
  [![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat&logo=go)](https://golang.org)
  [![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
  [![Redis Compatible](https://img.shields.io/badge/Redis-Compatible-DC382D?style=flat&logo=redis)](https://redis.io)
  [![Commands](https://img.shields.io/badge/Commands-1606+-green)](./docs/commands.md)
  
  [![CI](https://github.com/cachestorm/cachestorm/actions/workflows/ci.yml/badge.svg)](https://github.com/cachestorm/cachestorm/actions/workflows/ci.yml)
  [![Release](https://github.com/cachestorm/cachestorm/actions/workflows/release.yml/badge.svg)](https://github.com/cachestorm/cachestorm/actions/workflows/release.yml)
  [![Go Report Card](https://goreportcard.com/badge/github.com/cachestorm/cachestorm)](https://goreportcard.com/report/github.com/cachestorm/cachestorm)
  [![Docker](https://img.shields.io/docker/v/cachestorm/cachestorm/latest?label=Docker)](https://hub.docker.com/r/cachestorm/cachestorm)
</div>

---

A high-performance, Redis-compatible in-memory database written in Go with **1,606 commands** across 50+ modules. CacheStorm extends Redis with modern distributed systems features, data processing capabilities, and application-level abstractions.

**Redis Compatibility: ~99%** - Works with any Redis client out of the box!

📚 **[Documentation](./docs/)** | 🚀 **[Getting Started](./docs/01-getting-started.md)** | 📖 **[Commands Reference](./docs/commands.md)** | 💬 **[Discussions](https://github.com/cachestorm/cachestorm/discussions)**

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Command Modules](#command-modules)
- [Data Types](#data-types)
- [Performance](#performance)
- [Docker](#docker)
- [CI/CD & Releases](#cicd--releases)
- [Contributing](#contributing)
- [License](#license)

## Features

### Core Redis Compatibility
- **Full Redis Protocol**: RESP3 protocol support, works with any Redis client
- **289 Core Commands**: Complete Redis command set for strings, hashes, lists, sets, sorted sets, etc.
- **9 Core Data Types**: String, Hash, List, Set, SortedSet, Stream, Geo, Bitmap, HyperLogLog

### Extended Command Set (1,458+ Commands)

| Module | Commands | Description |
|--------|----------|-------------|
| **Core Redis** | 289 | Full Redis compatibility |
| **JSON** | 30+ | JSON document operations |
| **Time Series** | 40+ | Time-series data |
| **Search** | 50+ | Full-text search |
| **Graph** | 30+ | Graph database operations |
| **Probabilistic** | 20+ | Bloom/Cuckoo filters, Count-Min Sketch |
| **Distributed** | 100+ | Clustering, replication, failover |
| **Caching** | 50+ | Cache warming, invalidation |
| **Scheduling** | 40+ | Jobs, cron, timers |
| **Messaging** | 60+ | Pub/Sub, queues, streams |
| **Resilience** | 138 | Circuit breakers, rate limiters, retries |
| **Data Processing** | 100+ | Aggregation, windowing, joins |
| **Monitoring** | 80+ | Metrics, alerts, dashboards |
| **Security** | 40+ | ACL, secrets, encryption |
| **Workflows** | 50+ | DAGs, state machines |
| **And more...** | 400+ | See [Commands Reference](./docs/commands.md) |

### Performance
- **High Throughput**: ~14M GET/sec, ~1.5M SET/sec (single thread)
- **Parallel Performance**: ~77M GET/sec, ~15M SET/sec (parallel)
- **256-Shard Architecture**: Concurrent access with minimal lock contention
- **Zero Core Dependencies**: Core functionality implemented from scratch

### Enterprise Features
- **Lua Scripting**: Full EVAL/EVALSHA/SCRIPT support
- **Transactions**: MULTI/EXEC/DISCARD/WATCH support
- **Pub/Sub**: Subscribe, Publish, Pattern Subscribe
- **Clustering**: Gossip-based cluster with hash slot routing
- **Persistence**: AOF + RDB Snapshot
- **Replication**: Master-Slave replication
- **Access Control**: ACL support
- **Monitoring**: Slow Log, Latency monitoring, Hot key detection

## Quick Start

```bash
# Clone and build
git clone https://github.com/cachestorm/cachestorm
cd cachestorm
go build -o cachestorm ./cmd/cachestorm

# Run with default settings
./cachestorm

# Run with custom config
./cachestorm --config config.yaml --port 6379

# Test with redis-cli
redis-cli -p 6379 PING
```

## Installation

### From Source
```bash
go build -o cachestorm ./cmd/cachestorm
```

### Using Go Install
```bash
go install github.com/cachestorm/cachestorm@latest
```

### Using Docker
```bash
docker pull cachestorm/cachestorm:latest
docker run -d -p 6379:6379 -p 8080:8080 cachestorm/cachestorm
```

### From Releases
Download pre-built binaries from [GitHub Releases](https://github.com/cachestorm/cachestorm/releases).

## Command Modules

### Core Redis Commands
- **Strings**: SET, GET, INCR, DECR, APPEND, MSET, MGET, etc.
- **Hashes**: HSET, HGET, HINCRBY, HMSET, HGETALL, etc.
- **Lists**: LPUSH, RPUSH, LPOP, RPOP, LRANGE, etc.
- **Sets**: SADD, SREM, SINTER, SUNION, etc.
- **Sorted Sets**: ZADD, ZRANGE, ZINCRBY, etc.
- **Bitmaps**: SETBIT, GETBIT, BITCOUNT, BITOP, etc.
- **HyperLogLog**: PFADD, PFCOUNT, PFMERGE
- **Geo**: GEOADD, GEODIST, GEORADIUS, etc.
- **Streams**: XADD, XREAD, XGROUP, etc.

### Extended Commands

#### Resilience Patterns
```
CIRCUITX.CREATE/OPEN/CLOSE/STATUS/METRICS    - Circuit breaker pattern
RATELIMITER.CREATE/TRY/WAIT/RESET/STATUS     - Rate limiting
RETRY.CREATE/EXECUTE/STATUS/DELETE           - Retry with backoff
BULKHEAD.CREATE/ACQUIRE/RELEASE/STATUS       - Bulkhead isolation
TIMEOUT.CREATE/EXECUTE/DELETE                - Timeout handling
```

#### Async Processing
```
ASYNC.SUBMIT/STATUS/RESULT/CANCEL            - Async job execution
PROMISE.CREATE/RESOLVE/REJECT/AWAIT          - Promise pattern
FUTURE.CREATE/COMPLETE/GET/CANCEL            - Future pattern
OBSERVABLE.CREATE/NEXT/COMPLETE/SUBSCRIBE    - Observable streams
```

#### Data Processing
```
AGGREGATOR.CREATE/ADD/GET/RESET              - Real-time aggregation
WINDOWX.CREATE/ADD/GET/AGGREGATE             - Sliding/tumbling windows
JOINX.CREATE/ADD/GET/DELETE                  - Stream joins
PARTITIONX.CREATE/ADD/GET/REBALANCE          - Data partitioning
```

#### Event Sourcing
```
EVENTSOURCING.APPEND/REPLAY/SNAPSHOT/GET     - Event store
STREAMPROC.CREATE/PUSH/POP/PEEK              - Stream processing
```

#### Coordination
```
LOCKX.ACQUIRE/RELEASE/EXTEND/STATUS          - Distributed locks
SEMAPHOREX.CREATE/ACQUIRE/RELEASE/STATUS     - Semaphores
BATCHX.CREATE/ADD/EXECUTE/STATUS             - Batch processing
PIPELINEX.START/ADD/EXECUTE/CANCEL           - Pipelining
```

#### Monitoring
```
TELEMETRY.RECORD/QUERY/EXPORT                - Telemetry data
ALERTX.CREATE/FIRE/RESOLVE/LIST              - Alerting
DASHBOARD.CREATE/ADDWIDGET/GET               - Dashboards
METRICX.RECORD/QUERY/AGGREGATE               - Metrics
```

## Data Types

| Type | Description | Commands |
|------|-------------|----------|
| String | Binary-safe strings | SET, GET, INCR, etc. |
| Hash | Field-value maps | HSET, HGET, HINCRBY, etc. |
| List | Ordered collections | LPUSH, RPUSH, LRANGE, etc. |
| Set | Unordered unique sets | SADD, SREM, SINTER, etc. |
| Sorted Set | Scored ordered sets | ZADD, ZRANGE, ZINCRBY, etc. |
| Bitmap | Bit-level operations | SETBIT, GETBIT, BITCOUNT, etc. |
| HyperLogLog | Cardinality estimation | PFADD, PFCOUNT, PFMERGE |
| Geo | Geographic data | GEOADD, GEODIST, GEORADIUS, etc. |
| Stream | Log data structure | XADD, XREAD, XGROUP, etc. |
| JSON | JSON documents | JSON.GET, JSON.SET, etc. |
| Time Series | Time-stamped data | TS.CREATE, TS.ADD, etc. |

## Performance

Benchmarks run on AMD Ryzen 9 5900X, 64GB RAM:

| Operation | Single-thread | Multi-thread |
|-----------|---------------|--------------|
| GET | ~14M ops/sec | ~77M ops/sec |
| SET | ~1.5M ops/sec | ~15M ops/sec |
| HGET | ~12M ops/sec | ~65M ops/sec |
| HSET | ~1.2M ops/sec | ~12M ops/sec |
| LPUSH | ~800K ops/sec | ~8M ops/sec |
| ZADD | ~600K ops/sec | ~6M ops/sec |

## Docker

```bash
# Pull from Docker Hub
docker pull cachestorm/cachestorm:latest

# Run container
docker run -d \
  --name cachestorm \
  -p 6379:6379 \
  -p 8080:8080 \
  -v cachestorm-data:/data \
  cachestorm/cachestorm:latest

# With custom config
docker run -d \
  --name cachestorm \
  -p 6379:6379 \
  -v $(pwd)/config:/etc/cachestorm \
  cachestorm/cachestorm:latest \
  --config /etc/cachestorm/cachestorm.yaml
```

### Docker Compose

```yaml
version: '3.8'
services:
  cachestorm:
    image: cachestorm/cachestorm:latest
    ports:
      - "6379:6379"
      - "8080:8080"
    volumes:
      - cachestorm-data:/data
    environment:
      - CACHESTORM_MAX_MEMORY=4gb
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 3s
      retries: 3

volumes:
  cachestorm-data:
```

## CI/CD & Releases

### Continuous Integration
- **Build**: Multi-platform builds (Linux, macOS, Windows)
- **Test**: Unit tests, integration tests, benchmarks
- **Lint**: golangci-lint with comprehensive rules
- **Security**: Gosec vulnerability scanning
- **Coverage**: Codecov integration

### Release Process
- **Automated Releases**: Triggered by version tags
- **Multi-Architecture**: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- **Package Formats**: Binary archives, DEB, RPM, APK
- **Docker Images**: Multi-arch images pushed to Docker Hub & GHCR
- **Package Managers**: Homebrew, Scoop, Snap

### Creating a Release
```bash
# Tag and push
git tag v0.1.24
git push origin v0.1.24

# CI/CD will automatically:
# 1. Run all tests
# 2. Build binaries for all platforms
# 3. Create GitHub release
# 4. Push Docker images
# 5. Update package managers
```

## Configuration

```yaml
# config.yaml
server:
  port: 6379
  http_port: 8080
  max_clients: 10000

storage:
  max_memory: 4gb
  eviction_policy: allkeys-lru

persistence:
  enabled: true
  mode: aof
  aof_fsync: everysec
  rdb_interval: 300

cluster:
  enabled: false
  nodes: []
  
logging:
  level: info
  format: json
```

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Setup

```bash
# Clone your fork
git clone https://github.com/your-username/cachestorm
cd cachestorm

# Install dependencies
go mod download

# Run tests
go test ./internal/... -v -race -coverprofile=coverage.out

# Run linter
golangci-lint run

# Build
go build -o cachestorm ./cmd/cachestorm
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

<div align="center">
  <sub>Built with ❤️ by the CacheStorm Team</sub>
</div>
