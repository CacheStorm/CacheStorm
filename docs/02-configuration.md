# Configuration

## Configuration File

CacheStorm uses YAML for configuration. Default locations:
- `./cachestorm.yaml`
- `./config/cachestorm.yaml`
- `/etc/cachestorm/cachestorm.yaml`

## Full Configuration Reference

```yaml
# Server Configuration
server:
  bind: "0.0.0.0"              # Bind address
  port: 6380                    # TCP port
  max_connections: 10000        # Max concurrent connections
  tcp_keepalive: 300            # TCP keepalive in seconds
  read_timeout: "30s"           # Read timeout
  write_timeout: "30s"          # Write timeout
  read_buffer_size: 4096        # Read buffer size in bytes
  write_buffer_size: 4096       # Write buffer size in bytes

# HTTP/API Configuration
http:
  enabled: true                 # Enable HTTP server
  port: 8080                    # HTTP port
  password: ""                  # Optional password for admin UI

# Memory Management
memory:
  max_memory: "2gb"             # Max memory (0 = unlimited)
  eviction_policy: "allkeys-lru" # Eviction policy
  pressure_warning: 70          # Memory warning threshold %
  pressure_critical: 85         # Memory critical threshold %
  eviction_sample_size: 5       # Sample size for eviction

# Namespaces
namespaces:
  default:
    default_ttl: "0"            # Default TTL (0 = no expiry)
    max_memory: "0"             # Namespace memory limit
  cache:
    default_ttl: "1h"
    max_memory: "512mb"

# Cluster mode is not supported in this release: cluster.enabled: true makes
# startup fail, and CLUSTER commands answer
# "ERR This instance has cluster support disabled".
cluster:
  enabled: false

# Replication Configuration (master-replica, Redis semantics)
replication:
  role: "master"                # master, or replica (alias: slave)
  master_host: ""               # required for a replica
  master_port: 0                # required for a replica
  master_auth: ""               # password sent with AUTH to the master
  read_only: true               # a replica refuses client writes with -READONLY
  repl_timeout: 60              # seconds of silence before a link is dropped
  backlog_size: 1048576         # bytes kept for partial resynchronization
  replica_buffer_limit: 67108864 # bytes queued per replica before it is dropped

# Persistence Configuration
persistence:
  enabled: false                # Enable persistence
  aof: true                     # Enable AOF
  aof_sync: "everysec"          # AOF sync: always, everysec, no
  snapshot_interval: "5m"       # RDB snapshot interval
  data_dir: "/var/lib/cachestorm" # Data directory
  max_aof_size: "1gb"           # Max AOF file size before rewrite

# Plugins Configuration
plugins:
  stats:
    enabled: true               # Enable stats collection

  metrics:
    enabled: true               # Enable Prometheus metrics
    port: 9090                  # Metrics port
    path: "/metrics"            # Metrics endpoint path

  auth:
    enabled: false              # Enable authentication
    password: ""                # Auth password

  slowlog:
    enabled: true               # Enable slow log
    threshold: "10ms"           # Slow query threshold
    max_entries: 1000           # Max slow log entries

# Logging Configuration
logging:
  level: "info"                 # Log level: debug, info, warn, error
  format: "json"                # Log format: json, console
  output: "stdout"              # Output: stdout, stderr, or file path
```

## Eviction Policies

| Policy | Description |
|--------|-------------|
| `allkeys-lru` | Evict least recently used keys from all keys |
| `allkeys-lfu` | Evict least frequently used keys from all keys |
| `allkeys-random` | Evict random keys from all keys |
| `volatile-lru` | Evict LRU keys with TTL set |
| `volatile-lfu` | Evict LFU keys with TTL set |
| `volatile-ttl` | Evict keys with shortest TTL |
| `volatile-random` | Evict random keys with TTL set |
| `noeviction` | Return errors when memory limit reached |

## Memory Limits

Memory limits can be specified with suffixes:

```yaml
memory:
  max_memory: "2gb"     # 2 gigabytes
  max_memory: "512mb"   # 512 megabytes
  max_memory: "1tb"     # 1 terabyte
  max_memory: "0"       # Unlimited
```

## Time Durations

Time durations can be specified with suffixes:

```yaml
server:
  read_timeout: "30s"    # 30 seconds
  read_timeout: "5m"     # 5 minutes
  read_timeout: "1h"     # 1 hour
  read_timeout: "500ms"  # 500 milliseconds
```

## Environment Variables

All configuration options can be overridden with environment variables:

```bash
# Server
export CACHESTORM_BIND="0.0.0.0"
export CACHESTORM_PORT="6380"
export CACHESTORM_MAX_MEMORY="2gb"
export CACHESTORM_LOG_LEVEL="debug"

# HTTP
export CACHESTORM_HTTP_PORT="8080"
export CACHESTORM_HTTP_PASSWORD="secret"
```

## Configuration Validation

```bash
# Validate configuration
./cachestorm -config config.yaml -validate
```

## Hot Reloading

Some configuration options can be reloaded without restart:

```redis
CONFIG SET maxmemory 4gb
CONFIG SET maxmemory-policy allkeys-lfu
CONFIG SET slowlog-log-slower-than 5000
```

## Example Configurations

### Development

```yaml
server:
  port: 6380

http:
  enabled: true
  port: 8080

logging:
  level: debug
  format: console

memory:
  max_memory: "256mb"
```

### Production

```yaml
server:
  port: 6380
  max_connections: 50000

http:
  enabled: true
  port: 8080
  password: "${ADMIN_PASSWORD}"

logging:
  level: info
  format: json

memory:
  max_memory: "8gb"
  eviction_policy: "allkeys-lru"

persistence:
  enabled: true
  aof: true
  aof_sync: "everysec"
  data_dir: "/data/cachestorm"

plugins:
  slowlog:
    enabled: true
    threshold: "5ms"
```

### Replica

A replica connects to its master's RESP port, loads a full copy of its
data, then applies every write the master executes. With AOF enabled a
replica persists what it applies and, after a clean restart, continues from
the master's backlog instead of resynchronizing everything.
`REPLICAOF host port` and `REPLICAOF NO ONE` change the role at runtime.

```yaml
server:
  port: 6381

replication:
  role: replica
  master_host: "10.0.0.1"
  master_port: 6380
  master_auth: "the-masters-requirepass"

persistence:
  enabled: true
  data_dir: "/data/cachestorm"
```
