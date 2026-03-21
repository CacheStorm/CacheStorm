# CacheStorm Examples

This directory contains working examples for CacheStorm in multiple languages.

## Quick Start

### 1. Start CacheStorm Server

**Using Docker (Recommended):**
```bash
cd docker
docker-compose -f docker-compose.simple.yml up -d
```

**Or using the one-click installer:**
```bash
# Linux/macOS
curl -fsSL https://raw.githubusercontent.com/cachestorm/cachestorm/main/scripts/install.sh | bash

# Windows
irm https://raw.githubusercontent.com/cachestorm/cachestorm/main/scripts/install.ps1 | iex
```

### 2. Run Examples

#### Go Example
```bash
cd go
go mod tidy
go run main.go
```

#### Python Example
```bash
cd python
pip install -r requirements.txt
python demo.py
```

#### TypeScript Example
```bash
cd typescript
npm install
npm run demo
```

## What's Demonstrated

Each example demonstrates:

1. **Connection** - Connecting to CacheStorm server
2. **String Operations** - SET, GET, INCR, expiration
3. **Hash Operations** - HSET, HGET, HGETALL
4. **List Operations** - LPUSH, LRANGE, LPOP
5. **Set Operations** - SADD, SMEMBERS, SISMEMBER
6. **Sorted Set Operations** - ZADD, ZRANGE
7. **CacheStorm Tags** - Unique tag-based invalidation feature
8. **Pipeline** - Batch command execution
9. **Pub/Sub** - Real-time messaging
10. **Cleanup** - Proper resource management

## Docker Examples

### Simple Setup
```bash
cd docker
docker-compose -f docker-compose.simple.yml up -d
```

### With Monitoring (Prometheus + Grafana)
```bash
cd ../..  # Root directory
docker-compose --profile monitoring up -d
```

### With GUI (Redis Insight)
```bash
cd ../..  # Root directory
docker-compose --profile gui up -d
```

## SDK Features

All examples showcase:

- ✅ Full Redis protocol compatibility
- ✅ Connection pooling
- ✅ Pipeline support
- ✅ Pub/Sub support
- ✅ Automatic reconnection
- ✅ Type-safe APIs (Go/TypeScript)
- ✅ CacheStorm-specific features (tags, invalidation)
- ✅ Both sync and async APIs (Python)

## Troubleshooting

### Connection Refused
Make sure CacheStorm is running:
```bash
docker ps | grep cachestorm
# or
cachestorm-cli -p 6380 PING
```

### Port Already in Use
Change the port mapping in docker-compose:
```yaml
ports:
  - "6380:6380"  # Use 6380 on host
```

Then update examples to use port 6380.

## More Information

- [Main Documentation](../docs/)
- [Command Reference](../docs/commands.md)
- [Go SDK](../clients/go/README.md)
- [TypeScript SDK](../clients/typescript/README.md)
- [Python SDK](../clients/python/README.md)
