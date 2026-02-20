# Getting Started

## Installation

### From Binary

Download the latest release for your platform:

```bash
# Linux
wget https://github.com/cachestorm/cachestorm/releases/latest/download/cachestorm-linux-amd64.tar.gz
tar -xzf cachestorm-linux-amd64.tar.gz
sudo mv cachestorm /usr/local/bin/

# macOS
wget https://github.com/cachestorm/cachestorm/releases/latest/download/cachestorm-darwin-amd64.tar.gz
tar -xzf cachestorm-darwin-amd64.tar.gz
sudo mv cachestorm /usr/local/bin/

# Windows (PowerShell)
Invoke-WebRequest -Uri "https://github.com/cachestorm/cachestorm/releases/latest/download/cachestorm-windows-amd64.exe.tar.gz" -OutFile "cachestorm.tar.gz"
tar -xzf cachestorm.tar.gz
```

### From Source

```bash
git clone https://github.com/cachestorm/cachestorm
cd cachestorm
go build -o cachestorm ./cmd/cachestorm
```

### Using Docker

```bash
docker pull cachestorm/cachestorm:latest
docker run -d -p 6380:6380 -p 8080:8080 cachestorm/cachestorm
```

### Using Docker Compose

```yaml
version: '3.8'
services:
  cachestorm:
    image: cachestorm/cachestorm:latest
    ports:
      - "6380:6380"
      - "8080:8080"
    volumes:
      - ./data:/var/lib/cachestorm
      - ./config:/etc/cachestorm
    environment:
      - CACHESTORM_LOG_LEVEL=info
```

## Quick Start

### 1. Start the Server

```bash
./cachestorm
```

Output:
```
2024-01-15T10:00:00Z INFO CacheStorm server started addr=0.0.0.0:6380
2024-01-15T10:00:00Z INFO HTTP admin server started port=8080
```

### 2. Connect with redis-cli

```bash
redis-cli -p 6380
```

```redis
127.0.0.1:6380> PING
PONG

127.0.0.1:6380> SET mykey "Hello CacheStorm"
OK

127.0.0.1:6380> GET mykey
"Hello CacheStorm"
```

### 3. Use Tag-Based Invalidation

```redis
127.0.0.1:6380> SET user:1 '{"name":"John"}' TAGS "user" "session"
OK

127.0.0.1:6380> SET user:2 '{"name":"Jane"}' TAGS "user"
OK

127.0.0.1:6380> TAGKEYS user
1) "user:1"
2) "user:2"

127.0.0.1:6380> INVALIDATE user
1) "user:1"
2) "user:2"
```

### 4. Access Admin UI

Open your browser to `http://localhost:8080`

## Basic Usage Examples

### Python (redis-py)

```python
import redis

r = redis.Redis(host='localhost', port=6380, decode_responses=True)

# Basic operations
r.set('key', 'value')
print(r.get('key'))  # 'value'

# Hash operations
r.hset('user:1', mapping={'name': 'John', 'email': 'john@example.com'})
print(r.hgetall('user:1'))

# List operations
r.lpush('queue', 'task1', 'task2')
print(r.rpop('queue'))  # 'task1'
```

### Node.js (ioredis)

```javascript
const Redis = require('ioredis');
const redis = new Redis({ port: 6380 });

// Basic operations
await redis.set('key', 'value');
console.log(await redis.get('key')); // 'value'

// Hash operations
await redis.hset('user:1', 'name', 'John', 'email', 'john@example.com');
console.log(await redis.hgetall('user:1'));

// List operations
await redis.lpush('queue', 'task1', 'task2');
console.log(await redis.rpop('queue')); // 'task1'
```

### Go (go-redis)

```go
package main

import (
    "context"
    "fmt"
    "github.com/redis/go-redis/v9"
)

func main() {
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6380",
    })

    ctx := context.Background()

    // Basic operations
    rdb.Set(ctx, "key", "value", 0)
    val, _ := rdb.Get(ctx, "key").Result()
    fmt.Println(val) // "value"

    // Hash operations
    rdb.HSet(ctx, "user:1", "name", "John", "email", "john@example.com")
    data, _ := rdb.HGetAll(ctx, "user:1").Result()
    fmt.Println(data)
}
```

## Command Line Options

```bash
./cachestorm [options]

Options:
  -config string
        Path to configuration file
  -bind string
        Bind address (default "0.0.0.0")
  -port int
        Server port (default 6380)
  -version
        Print version and exit
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CACHESTORM_CONFIG` | Path to config file | |
| `CACHESTORM_BIND` | Bind address | `0.0.0.0` |
| `CACHESTORM_PORT` | Server port | `6380` |
| `CACHESTORM_MAX_MEMORY` | Max memory limit | `0` (unlimited) |
| `CACHESTORM_LOG_LEVEL` | Log level | `info` |

## Verifying Installation

```bash
# Check version
./cachestorm -version

# Health check
curl http://localhost:8080/api/health

# Metrics
curl http://localhost:8080/api/metrics
```

## Next Steps

- [Configuration](./02-configuration.md) - Configure CacheStorm for your needs
- [Commands Reference](./04-commands.md) - Learn all available commands
- [Tag-Based Invalidation](./05-tags.md) - Master the killer feature
- [Admin UI](./08-admin-ui.md) - Explore the web interface
