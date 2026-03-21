# CacheStorm Documentation

Complete documentation for CacheStorm - High-Performance, Redis-Compatible In-Memory Database.

## Table of Contents

1. [Getting Started](./01-getting-started.md) - Installation and quick start
2. [Configuration](./02-configuration.md) - Configuration options
3. [Data Types](./03-data-types.md) - Supported data types
4. [Commands Reference](./04-commands.md) - All 1,606 commands
5. [Tag-Based Invalidation](./05-tags.md) - Cache tagging system
6. [Lua Scripting](./06-lua-scripting.md) - Lua scripting guide
7. [HTTP API](./07-http-api.md) - RESTful API reference
8. [Admin UI](./08-admin-ui.md) - Web interface guide
9. [Clustering](./09-clustering.md) - Cluster setup
10. [Persistence](./10-persistence.md) - AOF and RDB
11. [Performance Tuning](./11-performance.md) - Optimization
12. [Deployment](./12-deployment.md) - Production deployment
13. [Troubleshooting](./13-troubleshooting.md) - Common issues

## Quick Links

- [README](../README.md) - Project overview
- [CHANGELOG](../CHANGELOG.md) - Version history
- [CONTRIBUTING](../CONTRIBUTING.md) - Contribution guidelines
- [COVERAGE_REPORT](../COVERAGE_REPORT.md) - Test coverage
- [Examples](../examples/) - Code examples

## Overview

CacheStorm is a high-performance, Redis-compatible in-memory database written in Go with **1,606 commands** across 50+ modules.

### Key Features

- **~99% Redis Compatibility**: Works with any Redis client out of the box
- **1,606 Commands**: 289 core Redis + 1,317 extended commands
- **11 Data Types**: String, Hash, List, Set, SortedSet, Stream, Geo, Bitmap, HyperLogLog, JSON, TimeSeries
- **Tag-Based Invalidation**: Native support for cache tagging
- **Lua Scripting**: Full EVAL/EVALSHA/SCRIPT support
- **Modern Admin UI**: Web-based management interface
- **RESTful HTTP API**: Easy integration on port 8080
- **Clustering**: Gossip-based cluster with hash slot routing
- **Persistence**: AOF and RDB snapshot support
- **Replication**: Master-slave replication with Sentinel
- **89.1% Test Coverage**: 100% test success rate

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      CacheStorm Server                       │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │  TCP/RESP   │  │  HTTP API   │  │  Admin UI   │         │
│  │  :6380      │  │  :8080      │  │  :8080      │         │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘         │
│         │                │                │                 │
│         └────────────────┼────────────────┘                 │
│                          │                                  │
│  ┌───────────────────────┴───────────────────────┐         │
│  │           Command Router (1,606 cmds)         │         │
│  └───────────────────────┬───────────────────────┘         │
│                          │                                  │
│  ┌───────────────────────┴───────────────────────┐         │
│  │              Store (256 Shards)                │         │
│  │  ┌─────┐ ┌─────┐ ┌─────┐     ┌─────┐          │         │
│  │  │ S0  │ │ S1  │ │ S2  │ ... │ S255│          │         │
│  │  └─────┘ └─────┘ └─────┘     └─────┘          │         │
│  └───────────────────────┬───────────────────────┘         │
│                          │                                  │
│  ┌───────────┐  ┌───────┴────────┐  ┌───────────────┐     │
│  │ Tag Index │  │ Namespace Mgr  │  │ Timing Wheel  │     │
│  └───────────┘  └────────────────┘  └───────────────┘     │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │    AOF      │  │    RDB      │  │   Cluster   │         │
│  │ Persistence │  │  Snapshot   │  │   Manager   │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ Replication │  │   Sentinel  │  │    ACL      │         │
│  │  Master/    │  │   Failover  │  │   Auth      │         │
│  │  Slave      │  │             │  │             │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
└─────────────────────────────────────────────────────────────┘
```

## Performance

Benchmarks on AMD Ryzen 9 5900X, 64GB RAM:

| Operation | Single-thread | Multi-thread |
|-----------|---------------|--------------|
| GET | ~14M ops/sec | ~77M ops/sec |
| SET | ~1.5M ops/sec | ~15M ops/sec |
| HGET | ~12M ops/sec | ~65M ops/sec |
| HSET | ~1.2M ops/sec | ~12M ops/sec |
| LPUSH | ~800K ops/sec | ~8M ops/sec |
| ZADD | ~600K ops/sec | ~6M ops/sec |

## Support

- GitHub Issues: https://github.com/cachestorm/cachestorm/issues
- Discussions: https://github.com/cachestorm/cachestorm/discussions
- Documentation: https://cachestorm.com/docs

## License

MIT License - See [LICENSE](../LICENSE) for details.
