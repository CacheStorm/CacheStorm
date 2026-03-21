<div align="center">
  <img src="https://avatars.githubusercontent.com/u/262622049?s=400&u=a2e56c80726cb8a3ae6fc8f8622be5173b7b2848&v=4" alt="CacheStorm Logo" width="180" height="180">

  # CacheStorm

  **High-Performance, Redis-Compatible In-Memory Database**

  [![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat&logo=go)](https://golang.org)
  [![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
  [![Redis Compatible](https://img.shields.io/badge/Redis-Compatible-DC382D?style=flat&logo=redis)](https://redis.io)
  [![Commands](https://img.shields.io/badge/Commands-1606+-green)](./docs/commands.md)
  [![Coverage](https://img.shields.io/badge/Coverage-89.1%25-brightgreen)](COVERAGE_REPORT.md)

  [![CI](https://github.com/cachestorm/cachestorm/actions/workflows/ci.yml/badge.svg)](https://github.com/cachestorm/cachestorm/actions/workflows/ci.yml)
  [![Release](https://github.com/cachestorm/cachestorm/actions/workflows/release.yml/badge.svg)](https://github.com/cachestorm/cachestorm/actions/workflows/release.yml)
  [![Nightly](https://github.com/cachestorm/cachestorm/actions/workflows/nightly.yml/badge.svg)](https://github.com/cachestorm/cachestorm/actions/workflows/nightly.yml)
  [![Go Report Card](https://goreportcard.com/badge/github.com/cachestorm/cachestorm)](https://goreportcard.com/report/github.com/cachestorm/cachestorm)
  [![Docker](https://img.shields.io/docker/v/cachestorm/cachestorm/latest?label=Docker)](https://hub.docker.com/r/cachestorm/cachestorm)
  [![Docker Pulls](https://img.shields.io/docker/pulls/cachestorm/cachestorm)](https://hub.docker.com/r/cachestorm/cachestorm)
</div>

---

A high-performance, Redis-compatible in-memory database written in Go with **1,606 commands** across 50+ modules. CacheStorm extends Redis with modern distributed systems features, data processing capabilities, and application-level abstractions.

**Redis Compatibility: ~99%** - Works with any Redis client out of the box!

**Test Coverage: 89.1%** with 100% test success rate across all 18 internal packages.

📚 **[Documentation](./docs/)** | 🚀 **[Getting Started](./docs/01-getting-started.md)** | 📖 **[Commands Reference](./docs/commands.md)** | 📊 **[Coverage Report](./COVERAGE_REPORT.md)** | 💬 **[Discussions](https://github.com/cachestorm/cachestorm/discussions)**

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Examples](#examples)
- [Command Modules](#command-modules)
- [Data Types](#data-types)
- [Performance](#performance)
- [Testing](#testing)
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
| **Machine Learning** | 80+ | Models, embeddings, tensors |
| **And more...** | 400+ | See [Commands Reference](./docs/commands.md) |

### Performance
- **High Throughput**: ~20M GET/sec, ~1.3M SET/sec (single thread)
- **Parallel Performance**: ~106M GET/sec, ~2.2M SET/sec (parallel)
- **256-Shard Architecture**: Concurrent access with minimal lock contention
- **Channel-Based Blocking**: BLPOP/BRPOP/BZPOPMIN/XREADGROUP use event-driven waiting, not polling
- **Zero Core Dependencies**: Core functionality implemented from scratch

### Enterprise Features
- **TLS Support**: TLS 1.2+ with configurable certificate/key files
- **Authentication**: `requirepass` with constant-time password validation
- **Lua Scripting**: Full EVAL/EVALSHA/SCRIPT support
- **Transactions**: MULTI/EXEC/DISCARD/WATCH support
- **Pub/Sub**: Subscribe, Publish, Pattern Subscribe, Sharded Pub/Sub (Redis 7)
- **Clustering**: Gossip-based cluster with hash slot routing
- **Persistence**: AOF with everysec/always sync policies, auto-replay on startup
- **Replication**: Master-Slave replication with Sentinel support
- **Memory Management**: Configurable maxmemory with LRU/LFU/Random eviction + OOM rejection
- **Access Control**: ACL support with per-command authentication enforcement
- **Graceful Shutdown**: Connection draining with configurable timeout
- **Panic Recovery**: Per-connection panic recovery with stack trace logging
- **Monitoring**: Prometheus `/metrics` endpoint, Slow Log, Latency monitoring, Hot key detection
- **HTTP API**: RESTful API with Read/Write/Idle timeouts on port 8080

## Quick Start

### One-Click Install

**Linux/macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/cachestorm/cachestorm/main/scripts/install.sh | bash
```

**Windows:**
```powershell
irm https://raw.githubusercontent.com/cachestorm/cachestorm/main/scripts/install.ps1 | iex
```

**Docker Compose:**
```bash
docker-compose up -d
```

### Using SDKs

**Go:**
```go
import (
	"context"
	cachestorm "github.com/cachestorm/cachestorm/clients/go"
)

ctx := context.Background()
client, _ := cachestorm.NewClient("localhost:6380")
client.Set(ctx, "key", "value", 0)
val, _ := client.Get(ctx, "key")
```

**TypeScript:**
```typescript
import { CacheStormClient } from '@cachestorm/client';

const client = new CacheStormClient({ host: 'localhost', port: 6380 });
await client.connect();
await client.set('key', 'value');
const val = await client.get('key');
```

**Python:**
```python
from cachestorm import CacheStormClient

client = CacheStormClient(host='localhost', port=6380)
client.set('key', 'value')
val = client.get('key')
```

**Redis CLI:**
```bash
redis-cli -p 6380 SET mykey myvalue
redis-cli -p 6380 GET mykey
```

## Installation Methods

### One-Click Installers

| Platform | Method | Command |
|----------|--------|---------|
| Linux/macOS | Docker (recommended) | `curl -fsSL .../install.sh \| bash -s -- docker` |
| Linux/macOS | Binary | `curl -fsSL .../install.sh \| bash -s -- binary` |
| Linux/macOS | Source | `curl -fsSL .../install.sh \| bash -s -- source` |
| Windows | Docker (recommended) | `irm .../install.ps1 \| iex -Method docker` |
| Windows | Binary | `irm .../install.ps1 \| iex -Method binary` |
| Windows | Source | `irm .../install.ps1 \| iex -Method source` |

### Manual Installation

#### From Source
```bash
git clone https://github.com/cachestorm/cachestorm
cd cachestorm
go build -o cachestorm ./cmd/cachestorm
./cachestorm
```

#### Using Go Install
```bash
go install github.com/cachestorm/cachestorm@latest
```

#### Using Docker
```bash
docker pull cachestorm/cachestorm:latest
docker run -d -p 6380:6380 -p 8080:8080 cachestorm/cachestorm
```

#### Using Docker Compose
```bash
# With monitoring (Prometheus + Grafana)
docker-compose --profile monitoring up -d

# With GUI (Redis Insight)
docker-compose --profile gui up -d

# All features
docker-compose --profile gui --profile monitoring up -d
```

#### From Releases
Download pre-built binaries from [GitHub Releases](https://github.com/cachestorm/cachestorm/releases).

### Uninstall

```bash
# Linux/macOS
curl -fsSL https://raw.githubusercontent.com/cachestorm/cachestorm/main/scripts/install.sh | bash -s -- uninstall

# Windows
irm https://raw.githubusercontent.com/cachestorm/cachestorm/main/scripts/install.ps1 | iex -Method uninstall
```

## Examples

Working examples for all SDKs:

```bash
# Start CacheStorm
docker-compose up -d

# Go example
cd examples/go
go run main.go

# Python example
cd examples/python
pip install -r requirements.txt
python demo.py

# TypeScript example
cd examples/typescript
npm install
npm run demo
```

See [examples/](./examples/) directory for complete working examples.

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

Benchmarks on AMD Ryzen 9 9950X3D, 64GB RAM, Windows 11 (in-memory, no network):

| Operation | ops/sec | ns/op | Allocs/op |
|-----------|---------|-------|-----------|
| GET (sequential) | 20M | 50 ns | 0 |
| GET (parallel, 32 cores) | 106M | 9.4 ns | 0 |
| SET (sequential) | 1.3M | 796 ns | 3 |
| SET (parallel, 32 cores) | 2.2M | 449 ns | 3 |
| RESP ReadCommand | 861K | 1,531 ns | 17 |
| RESP WriteBulkString (1KB) | 1M | 1,137 ns | 2 |
| Tag Invalidate (10K keys) | 597 | 1.7 ms | 19,982 |
| Tag Count | 52M | 23 ns | 0 |

E2E benchmarks (full TCP round-trip):

| Operation | ops/sec | ns/op |
|-----------|---------|-------|
| SET | 32K | 37 us |
| GET | 33K | 37 us |
| HSET | 33K | 38 us |
| ZADD | 31K | 36 us |
| XADD | 31K | 39 us |

## Testing

CacheStorm has comprehensive test coverage:

- **100% Test Success Rate**: All 18 internal packages pass
- **89.1% Average Coverage**: Industry-leading coverage
- **Integration Tests**: Full integration test suite
- **Benchmarks**: Performance benchmarks included

```bash
# Run all tests
go test ./internal/... -v

# Run with coverage
go test ./internal/... -cover

# Run benchmarks
go test ./internal/store/... -bench=.
```

See [COVERAGE_REPORT.md](./COVERAGE_REPORT.md) for detailed coverage information.

## Docker

### Quick Start with Docker
```bash
# Pull and run
docker run -d -p 6380:6380 -p 8080:8080 --name cachestorm cachestorm/cachestorm:latest
```

### Docker Compose (Full Stack)
```bash
# Clone repository
git clone https://github.com/cachestorm/cachestorm
cd cachestorm

# Basic setup
docker-compose up -d

# With monitoring (Prometheus + Grafana)
docker-compose --profile monitoring up -d

# With GUI (Redis Insight)
docker-compose --profile gui up -d

# Everything
docker-compose --profile gui --profile monitoring up -d
```

### Docker Commands
```bash
# Pull from Docker Hub
docker pull cachestorm/cachestorm:latest

# Run container
docker run -d \
  --name cachestorm \
  -p 6380:6380 \
  -p 8080:8080 \
  -v cachestorm-data:/data \
  cachestorm/cachestorm:latest

# With custom config
docker run -d \
  --name cachestorm \
  -p 6380:6380 \
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
      - "6380:6380"
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
- **Coverage**: Code coverage tracking

### Release Process
- **Automated Releases**: Triggered by version tags (`v*`) or manual dispatch
- **Multi-Architecture**: linux/amd64, linux/arm64, linux/386, linux/arm/v7, darwin/amd64, darwin/arm64, windows/amd64
- **Package Formats**: Binary archives (tar.gz, zip), DEB, RPM, APK
- **Docker Images**: Multi-arch images pushed to Docker Hub & GHCR with digest attestation
- **Package Managers**: Homebrew, Scoop, Snap
- **Nightly Builds**: Automated daily builds with `nightly` tag
- **Signed Releases**: GPG-signed binaries and checksums

### Creating a Release
```bash
# Method 1: Tag and push (automatic)
git tag v0.1.28
git push origin v0.1.28

# Method 2: Manual dispatch from GitHub Actions
# Go to Actions → Release → Run workflow
```

### CI/CD Workflows

| Workflow | Trigger | Description |
|----------|---------|-------------|
| [CI](.github/workflows/ci.yml) | Push, PR | Build, test, lint, security scan on all platforms |
| [Release](.github/workflows/release.yml) | Tag `v*` | Full release with binaries, Docker, package managers |
| [Nightly](.github/workflows/nightly.yml) | Daily 2AM | Nightly builds with latest changes |
| [Changelog](.github/workflows/changelog.yml) | Push | Auto-update CHANGELOG.md |
| [Dependencies](.github/workflows/dependency-update.yml) | Weekly | Automated dependency updates |
| [Docs](.github/workflows/docs.yml) | Push to docs | Documentation deployment |

### What CI/CD Does Automatically

1. **On Every Push/PR**:
   - Build on Linux, macOS, Windows
   - Run tests with race detection
   - Code coverage reporting (Codecov)
   - Linting (golangci-lint, go vet)
   - Security scanning (Gosec, Nancy, Trivy)
   - Benchmark tests
   - Docker build test

2. **On Release Tag**:
   - Run full test suite
   - Build binaries for all platforms
   - Create signed GitHub release
   - Build and push multi-arch Docker images
   - Update Homebrew formula
   - Update Scoop manifest
   - Publish Snap package
   - Create release discussion
   - Notify Slack/Discord

3. **Scheduled Tasks**:
   - Daily: Nightly builds
   - Weekly: Dependency updates
   - Daily: Changelog updates

## Configuration

```yaml
# cachestorm.yaml
server:
  bind: "0.0.0.0"
  port: 6380
  max_connections: 10000
  requirepass: ""              # Set password for AUTH
  tls_cert_file: ""            # Path to TLS certificate
  tls_key_file: ""             # Path to TLS private key
  read_timeout: "5m"
  write_timeout: "5m"

http:
  enabled: true
  port: 8080
  password: ""                 # HTTP API auth password

memory:
  max_memory: "0"              # 0 = unlimited, or e.g. "4gb"
  eviction_policy: "allkeys-lru"  # noeviction, allkeys-lru, allkeys-lfu, volatile-lru, allkeys-random
  pressure_warning: 70
  pressure_critical: 85
  eviction_sample_size: 5

persistence:
  enabled: false
  aof: true
  aof_sync: "everysec"         # always, everysec, no
  data_dir: "/var/lib/cachestorm"

replication:
  role: "master"               # master or slave
  master_host: ""
  master_port: 6380

cluster:
  enabled: false
  node_name: ""
  seeds: []

logging:
  level: "info"
  format: "json"
  output: "stdout"
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

## Official SDKs

CacheStorm provides official SDKs for popular programming languages:

| Language | Package | Installation | Documentation |
|----------|---------|--------------|---------------|
| **Go** | `github.com/cachestorm/cachestorm/clients/go` | `go get github.com/cachestorm/cachestorm/clients/go` | [Go SDK](./clients/go/README.md) |
| **TypeScript** | `@cachestorm/client` | `npm install @cachestorm/client` | [TypeScript SDK](./clients/typescript/README.md) |
| **Python** | `cachestorm` | `pip install cachestorm` | [Python SDK](./clients/python/README.md) |

### SDK Features

All official SDKs support:
- ✅ Full Redis protocol compatibility
- ✅ Connection pooling
- ✅ Pipeline support
- ✅ Pub/Sub support
- ✅ Automatic reconnection
- ✅ Type-safe APIs
- ✅ CacheStorm-specific features (tags, invalidation)

### Using Any Redis Client

Since CacheStorm is ~99% Redis compatible, you can use any existing Redis client:

```python
# Python with redis-py
import redis
r = redis.Redis(host='localhost', port=6380)

# Node.js with ioredis
const Redis = require('ioredis');
const redis = new Redis({ port: 6380 });

# Go with go-redis
import "github.com/redis/go-redis/v9"
rdb := redis.NewClient(&redis.Options{Addr: "localhost:6380"})

# Java with Jedis
Jedis jedis = new Jedis("localhost", 6380);

# C# with StackExchange.Redis
var redis = ConnectionMultiplexer.Connect("localhost:6380");
```

## Project Structure

```
cachestorm/
├── cmd/cachestorm/        # Main application entry point
├── internal/
│   ├── acl/               # Access control lists
│   ├── batch/             # Batch processing
│   ├── buffer/            # Buffer management
│   ├── cluster/           # Clustering
│   ├── command/           # Command handlers (1,606 commands)
│   ├── config/            # Configuration management
│   ├── graph/             # Graph operations
│   ├── logger/            # Logging (100% coverage)
│   ├── module/            # Module system
│   ├── persistence/       # AOF/RDB persistence
│   ├── plugin/            # Plugin system
│   ├── pool/              # Connection pooling
│   ├── replication/       # Master-slave replication
│   ├── resp/              # RESP protocol
│   ├── search/            # Search functionality
│   ├── sentinel/          # Redis Sentinel
│   ├── server/            # Server implementation
│   └── store/             # Data store (256-shard)
├── clients/               # Official SDKs
│   ├── go/                # Go client
│   ├── typescript/        # TypeScript/JavaScript client
│   └── python/            # Python client
├── plugins/               # Plugin implementations
├── scripts/               # Installation scripts
├── tests/                 # Integration tests
├── benchmarks/            # Performance benchmarks
├── docs/                  # Documentation
├── config/                # Configuration examples
├── docker/                # Docker files
└── web/                   # Web interface
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

<div align="center">
  <sub>Built with ❤️ by the CacheStorm Team</sub>
</div>
