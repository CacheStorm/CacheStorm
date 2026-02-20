# CacheStorm Documentation

## Table of Contents

1. [Getting Started](./01-getting-started.md)
2. [Configuration](./02-configuration.md)
3. [Data Types](./03-data-types.md)
4. [Commands Reference](./04-commands.md)
5. [Tag-Based Invalidation](./05-tags.md)
6. [Lua Scripting](./06-lua-scripting.md)
7. [HTTP API](./07-http-api.md)
8. [Admin UI](./08-admin-ui.md)
9. [Clustering](./09-clustering.md)
10. [Persistence](./10-persistence.md)
11. [Performance Tuning](./11-performance.md)
12. [Deployment](./12-deployment.md)
13. [Troubleshooting](./13-troubleshooting.md)

## Quick Links

- [README](../README.md)
- [CHANGELOG](../CHANGELOG.md)
- [Examples](../examples/)

## Overview

CacheStorm is a high-performance, Redis-compatible in-memory cache server written in Go. It provides:

- **Full Redis Compatibility**: Works with any Redis client
- **180+ Commands**: Comprehensive Redis command coverage
- **9 Data Types**: String, Hash, List, Set, SortedSet, Stream, Geo, Bitmap, HyperLogLog
- **Tag-Based Invalidation**: Native support for tag-based cache invalidation
- **Lua Scripting**: Full EVAL/EVALSHA support
- **Modern Admin UI**: Web-based management interface
- **RESTful API**: HTTP API for easy integration
- **Clustering**: Built-in cluster support with gossip protocol
- **Persistence**: AOF and RDB snapshot support

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
│  │                  Command Router                 │         │
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
└─────────────────────────────────────────────────────────────┘
```

## Support

- GitHub Issues: https://github.com/cachestorm/cachestorm/issues
- Documentation: https://cachestorm.com/docs

## License

Apache 2.0 - See [LICENSE](../LICENSE) for details.
