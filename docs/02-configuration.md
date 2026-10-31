# Configuration

## Configuration File

CacheStorm reads the YAML file named by `--config` or, when that flag is
absent, by the `CACHESTORM_CONFIG` environment variable. Without either it
starts with the built-in defaults; no other locations are searched.

The loader is strict: an unknown or misspelt key stops startup with an error
naming the file, line and key, instead of silently leaving the default in
force. `config/cachestorm.example.yaml` and `config/cachestorm.production.yaml`
are kept loadable by a test.

### Redis-style options and redis.conf

CacheStorm takes the same command line as `redis-server`:

```bash
cachestorm [config-file] [--option value ...]
cachestorm --port 6379 --requirepass secret --maxmemory 1gb --maxmemory-policy allkeys-lru
cachestorm /etc/redis/redis.conf --appendonly yes
```

A config file whose name ends in `.conf` is read as a `redis.conf`, with its comments, quoting and repeated `save` lines. Any other file is YAML. Options on the command line are applied after the file, so they win, as with Redis.

- **Startup settings.** These directives are applied before the server starts: `port`, `bind` (the first address; `*` means all), `protected-mode`, `requirepass`, `databases`, `maxclients`, `tcp-keepalive`, `notify-keyspace-events`, `maxmemory`, `maxmemory-policy` and `maxmemory-samples`. So are `appendonly`, `appendfsync`, `dir`, `save`, `loglevel` and `logfile`, as well as `replicaof` (or `slaveof`), `masterauth`, the `replica-*` and `repl-*` settings, and the `cluster-*` settings.
- **Run-time settings.** Every other directive goes through `CONFIG SET` once the server exists, with the same validation. A `user` line becomes `ACL SETUSER`.
- **Ignored and refused directives.** A parameter Redis reads only at startup that CacheStorm does not use, such as `tcp-backlog` or `pidfile`, is ignored with a warning when it differs from the default. An unknown directive or an invalid value stops startup with the file and line, or the option, that caused it. `include`, `rename-command`, `aclfile`, `loadmodule` and `daemonize yes` are refused.

Persistence follows Redis' rules. A `redis.conf` without `save` lines gets Redis' default rules (`3600 1 300 100 60 10000`). A rule `save <seconds> <changes>` takes a snapshot once at least `<changes>` writes happened and `<seconds>` passed since the last save. `save ""` stops periodic snapshots, but SAVE and BGSAVE still work and `dump.rdb` is still loaded at startup. The append-only file is on only for `appendonly yes`. Without a `redis.conf` or a `save` option, persistence stays off, as CacheStorm's own configuration has it.

## Full Configuration Reference

```yaml
# Server Configuration
server:
  bind: "0.0.0.0"              # Bind address
  port: 6380                    # TCP port
  max_connections: 10000        # Max concurrent connections
  databases: 16                 # SELECT 0 .. databases-1
  tcp_keepalive: 300            # TCP keepalive period in seconds (0 = off)
  read_timeout: "0"             # Idle client timeout (0 = never, as Redis timeout 0)
  write_timeout: "0"            # Reply write timeout (0 = 5m default)
  read_buffer_size: 4096        # Per-connection read buffer in bytes
  write_buffer_size: 16384      # Per-connection write buffer in bytes
  requirepass: ""               # Password for AUTH (empty = no auth)
  protected_mode: true          # Without a password, serve only loopback clients
  tls_cert_file: ""             # TLS certificate (empty = plaintext)
  tls_key_file: ""              # TLS private key
  tls_ca_cert_file: ""          # CA that signs client (and master) certificates
  tls_auth_clients: ""          # yes | optional | no; empty = yes when a CA is set
  notify_keyspace_events: ""    # Keyspace notifications (notify-keyspace-events), e.g. "KEA"; empty = off

# HTTP/API Configuration
http:
  enabled: true                 # Enable HTTP server
  bind: ""                      # Listen address (empty = server.bind)
  port: 8080                    # HTTP port
  password: ""                  # Optional password for admin UI
  rate_limit: 100               # Requests per client per window
  rate_window: "1m"             # Rate limit window
  cors_origin: ""               # Allowed CORS origin (empty = none)

# Memory Management
memory:
  max_memory: "2gb"             # Max memory (0 = unlimited)
  eviction_policy: "allkeys-lru" # Eviction policy
  pressure_warning: 70          # Memory warning threshold %
  pressure_critical: 85         # Memory critical threshold %
  eviction_sample_size: 5       # Sample size for eviction

# Cluster mode (Redis Cluster semantics; see "Cluster mode" below)
cluster:
  enabled: false
  config_file: nodes.conf       # relative to persistence.data_dir
  node_timeout: 15000           # ms, as cluster-node-timeout
  bus_port: 0                   # cluster bus port; 0 = port + 10000
  announce_ip: ""               # as cluster-announce-ip
  announce_port: 0              # 0 = the listening port
  announce_bus_port: 0          # 0 = bus_port
  require_full_coverage: true   # down while any slot is unassigned
  allow_reads_when_down: false  # serve reads while the cluster is down
  replica_validity_factor: 10   # as cluster-replica-validity-factor; 0 = no data age limit
  replica_no_failover: false    # as cluster-replica-no-failover: never start an automatic failover
  allow_replica_migration: true # as cluster-allow-replica-migration
  migration_barrier: 1          # as cluster-migration-barrier
  tls: false                    # tls-cluster: mutual TLS on the bus (needs server.tls_* and tls_ca_cert_file)

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
  tls: false                    # dial the master over TLS, presenting tls_cert_file
                                # and verifying it with tls_ca_cert_file

# Persistence Configuration
persistence:
  enabled: false                # Enable persistence
  aof: true                     # Enable AOF
  aof_sync: "everysec"          # AOF sync: always, everysec, no
  snapshot_interval: "5m"       # Snapshot every interval when there were writes
  save: ""                      # Redis save rules, e.g. "3600 1 300 100"; replace snapshot_interval
  data_dir: "/var/lib/cachestorm" # Data directory
  aof_rewrite_percentage: 100   # Auto-rewrite after this much growth (0 = off)
  aof_rewrite_min_size: "64mb"  # ... once the AOF is at least this large

# Plugins Configuration
plugins:
  metrics:
    enabled: true               # Enable Prometheus metrics
    port: 9090                  # Metrics port
    path: "/metrics"            # Metrics endpoint path

  slowlog:
    enabled: true               # Enable slow log
    threshold: "10ms"           # Slow query threshold
    max_entries: 1000           # Max slow log entries

# Logging Configuration
logging:
  level: "info"                 # Log level: debug, info, warn, error
  format: "json"                # Log format: json, console
  output: "stdout"              # Output: stdout, stderr, or file path

# Experimental built-in sentinel role (off on data servers); see
# "High availability" below for automatic failover

sentinel:
  enabled: false
  id: "sentinel-1"
  addr: "127.0.0.1"
  port: 26379
  quorum: 2
  down_after: "30s"
  failover_time: "3m"
  seeds: []                     # addr:port of other sentinels
```

### High availability

Automatic failover uses **Redis Sentinel** (7.x) pointed at CacheStorm
servers, exactly as it would be pointed at Redis:

```
sentinel monitor mymaster 10.0.0.1 6380 2
sentinel auth-pass mymaster <requirepass>
sentinel down-after-milliseconds mymaster 5000
```

The sentinels discover the replicas from `INFO replication` and each other
over the `__sentinel__:hello` channel; when the master fails they elect a
leader, promote a replica with `MULTI`/`SLAVEOF NO ONE`/`EXEC`, repoint the
other replicas and demote the old master when it returns. Sentinel-aware
clients (go-redis `NewFailoverClient`, redis-py `Sentinel`, ioredis
`sentinels`) follow the switch. `scripts/sentinel-smoke.sh` runs this with
three CacheStorm processes and three sentinels and checks that no
acknowledged write is lost.

The `sentinel:` section starts CacheStorm's own sentinel role. It is
experimental: it tracks masters but does not reconfigure servers on
failover, so it is not a replacement for Redis Sentinel.

### Cluster mode

With `cluster.enabled: true` the server runs as a Redis Cluster node and
forms clusters of masters and replicas with other CacheStorm nodes, with
automatic and manual failover and `redis-cli --cluster` resharding. The
operator guide is [Running a cluster](./09-cluster.md).

- **Identity and nodes.conf.** On first start the node generates a 40-hex
  node ID and a shard ID and writes `nodes.conf` in `persistence.data_dir`.
  This happens whether or not persistence is enabled. The file uses Redis's
  format, including the `vars currentEpoch N lastVoteEpoch N` line. Every
  change is written atomically: a temporary file, fsync, rename, then an
  fsync of the directory. If the file exists at startup, it is loaded; a
  corrupt file stops startup.
- **Slots.** Assign slots with `CLUSTER ADDSLOTS`/`ADDSLOTSRANGE`. A single
  node serving everything needs `CLUSTER ADDSLOTSRANGE 0 16383`. Until every
  slot is assigned, `cluster_state` is `fail` (unless
  `require_full_coverage: false`) and keyed commands get `CLUSTERDOWN`.
- **Cluster bus.** The node listens on `bus_port` (default: the client
  port + 10000) on the `server.bind` address, for CacheStorm's own
  node-to-node protocol; open it between the nodes. `announce_bus_port` is
  the bus port the other nodes are told. Nodes join with `CLUSTER MEET ip
  port [bus-port]` and learn about each other through gossip, so
  `redis-cli --cluster create` works (see `scripts/cluster-smoke.sh`).
  `CLUSTER FORGET` removes a node and keeps it out for 60 seconds.
- **Failure detection.** A node that does not answer a ping within
  `node_timeout` is flagged `fail?` (PFAIL); once a majority of the masters
  serving slots report it, it becomes `fail` everywhere and, with
  `require_full_coverage`, the cluster goes down while its slots are not
  served. A master in the minority of a partition also reports the cluster
  down. A failed master that answers again is cleared after
  2 × `node_timeout`.
- **Pub/Sub.** `PUBLISH` reaches the subscribers of every node; `SPUBLISH`
  stays on the node serving the channel's slot.
- **Routing.** Keys hash with CRC16 modulo 16384 and `{hash tags}`. A keyed
  command that is not served here gets the reply Redis gives:
  - `MOVED slot host:port` when another node owns the slot;
  - `ASK` for a key missing from a migrating slot;
  - `TRYAGAIN` for a multi-key command that finds only some of its keys
    during a migration;
  - `CROSSSLOT` when the keys span slots.

  An importing slot is served only after `ASKING`. The same rules apply to
  the commands queued in `MULTI` (checked when they are queued and again at
  `EXEC`), to the declared keys of `EVAL`/`FCALL`, to the keys a script
  touches, and to `SPUBLISH`/`SSUBSCRIBE`. Commands without keys are never
  redirected.
- **Database 0 only.** `SELECT` with another index, `MOVE`, `SWAPDB` and
  `COPY ... DB n` are refused, as in Redis. A data directory whose snapshot
  or AOF holds keys in another database does not start in cluster mode.
  Keys found at startup in a slot no node serves are claimed by this node.
- `INFO cluster` reports `cluster_enabled:1`. `HELLO` and `INFO server`
  report the `cluster` mode. go-redis `ClusterClient` and redis-py
  `RedisCluster` are exercised by `scripts/client-smoke.sh`.
- **Replicas.** `CLUSTER REPLICATE <master-id>` makes an empty node without
  slots a replica of that master; `redis-cli --cluster create
  --cluster-replicas 1` uses it. The replica replicates through the normal
  replication link (`replication.master_auth` is the password it sends,
  so set it when the masters have `requirepass`), its `nodes.conf` line reads
  `slave <master-id>`, and `CLUSTER NODES`, `SLOTS`, `SHARDS` and
  `REPLICAS` list it as Redis does. A replica answers `MOVED` to its master
  unless the client sent `READONLY`, which lets it serve reads of its
  master's slots; writes are always redirected.
- **Automatic failover.** When a master is `fail`, its replicas wait 500 ms
  plus up to 500 ms plus 1 s per better-placed replica (by replication
  offset), then ask the masters for their vote in a new epoch. A master
  votes once per epoch and once per failed master in 2 × `node_timeout`. The
  replica that gets the votes of a majority of the slot-owning masters stops
  replicating, keeps its data, takes the slots with the election epoch and
  tells every node. A replica that has been cut off from its master for
  longer than `node_timeout` × `replica_validity_factor` + 10 s does not
  start an election. The old master, when it comes back, learns the newer
  configuration and becomes a replica of the new master.
- **Manual failover.** `CLUSTER FAILOVER` on a replica pauses the master's
  writers, lets the replica catch up with the master's offset and then runs
  the election, so no acknowledged write is lost. `FORCE` skips the
  master, and `TAKEOVER` also skips the vote, taking a new epoch on its own.
  A replica with `replica_no_failover` only fails over manually.
- **Replica migration.** When a master serving slots loses its last working
  replica, and another master has more than `migration_barrier` working
  replicas, the replica of that master with the smallest node ID becomes a
  replica of the orphaned master after 5 s, as in Redis. `CONFIG SET`
  changes `cluster-replica-no-failover`, `cluster-allow-replica-migration`,
  `cluster-migration-barrier`, `cluster-require-full-coverage` and
  `cluster-allow-reads-when-down` at run time.
- **Rejoin delays.** A master that was in the minority waits `node_timeout`
  (between 0.5 s and 5 s) after it sees a majority again before it serves
  writes, and a master restarted from `nodes.conf` next to other nodes waits
  2 s, so either one learns about a failover first, as in Redis.
- `REPLICAOF` is refused in cluster mode, and `replication.role: replica`
  cannot be combined with it.

### AOF rewrite

`BGREWRITEAOF` replaces the AOF with the smallest command stream that
rebuilds the current dataset. With `aof_rewrite_percentage` above zero the
server also rewrites automatically, as Redis' `auto-aof-rewrite-percentage`
and `auto-aof-rewrite-min-size` do: once the file is at least
`aof_rewrite_min_size` and has grown by the percentage since the last
rewrite (or startup). The rewrite holds the write gate while it encodes and
syncs the new file, so writes pause for its duration; reads that do not need
the gate continue. A failed automatic rewrite is retried only after the file
doubles again. `INFO persistence` reports `aof_rewrite_in_progress`,
`aof_last_bgrewrite_status`, `aof_rewrites`, `aof_current_size` and
`aof_base_size`.


### Snapshot and recovery behavior

With persistence enabled, the server takes an exclusive OS lock on `<data_dir>/.cachestorm.lock` during construction, before reading snapshots/AOF, truncating a torn log or consuming a replication checkpoint. A second CacheStorm server using the same directory fails immediately, including through relative paths or symlink aliases. The resolved absolute directory stays pinned for that server; changing the config object's path does not move its files. Construct a new server to change directories.

Embedded callers must call `Server.Stop` on a successfully constructed server even if `Start` was never called. Constructor failures release ownership. Failed persistent startup stops its writers and releases ownership; that object must be reconstructed with `New` before another start attempt. Normal shutdown joins the accept loop before draining connections, and retains the lock through HTTP/TCP drain, snapshot workers, AOF close and checkpoint publication. Repeated/concurrent Stop calls return the stored shutdown result. If workers remain after forced drain or HTTP shutdown times out, shutdown returns an error and retains ownership and the AOF writer until process exit, so an already-running request can still persist its result. An incomplete shutdown is terminal for that instance; terminate the process instead of reusing it. OS cleanup releases the lock after SIGKILL as well.

The lock file remains after exit; its presence does not mean a process owns the directory. Leave it in place so processes lock the same inode. Locking coordinates cooperating CacheStorm server instances; low-level persistence APIs and external tools do not take server ownership automatically. Linux process behavior is tested. Windows uses LockFileEx and Unix release targets use flock; other native platforms and shared/network filesystem behavior still require runtime validation.

With `persistence.enabled: true`, `SAVE` writes `<data_dir>/dump.rdb` synchronously. It captures every numbered database, expiry timestamps, key tags, stream entries and consumer-group state, function libraries, and search indexes with their documents and aliases. It also flushes AOF when AOF is enabled. The snapshot file and its directory are synced before a successful SAVE reply.

`BGSAVE` captures the same consistent state while writes are paused, then releases writes and performs disk I/O in a background worker. The capture uses additional memory proportional to the encoded dataset; it is not a fork or copy-on-write snapshot. A second SAVE/BGSAVE is rejected while that worker is active. `LASTSAVE` advances only after successful completion. `INFO persistence` exposes `rdb_bgsave_in_progress` and `rdb_last_bgsave_status`. Shutdown waits for an already-running snapshot worker.

`snapshot_interval: "5m"` starts a background save every interval in which there were writes, the same as the rule `save 300 1`. `save: "3600 1 300 100"` sets Redis save rules instead, and `CONFIG SET save` changes them at run time. Set the interval to `"0"` or an empty string, or `save` to `""` with `CONFIG SET`, to disable periodic saves while retaining explicit SAVE/BGSAVE. Invalid or negative values prevent startup.

With save rules configured, SHUTDOWN and a SIGTERM or SIGINT save a snapshot before the server exits, as in Redis. If that snapshot fails, a signal leaves the server running and logs the error; a second signal stops it without saving.

Startup prefers `appendonly.aof` when it exists, including an empty AOF. Otherwise it loads `dump.rdb` if present; if AOF is enabled, it seeds a new AOF from that restored dataset before accepting writes. New snapshots carry a CRC64 checksum; corruption or a truncated snapshot stops startup. Older snapshots with a zero checksum remain readable.

AOF startup validates transaction framing and replays only persistence commands plus SELECT. Unknown/disallowed commands, invalid arity, or a handler error reply stops startup at the first failure, before serving traffic. The error identifies the command position/name and selected database. A complete invalid log is preserved; replay validation also runs before trimming an otherwise torn suffix. An incomplete final command is removed; when the file ends inside MULTI, the entire unfinished block is removed from the MULTI boundary. Truncation is synced before opening the writer. Nested MULTI, stray EXEC/DISCARD and transaction marker arguments are corruption. A discarded transaction has no effects, including its SELECTs. Replay uses nonblocking transaction behavior for blocking commands. This does not establish deterministic replay for every extension or restore process-global registries atomically after a failed construction.


New AOF files use a private `CACHESTORM.AOF 2` header. Core string/expiry commands record effective mutations with absolute millisecond deadlines: rejected conditional operations are omitted, successful SET conditions are removed, and REST TTL writes use the stored deadline. During v2 replay, wall-clock expiry and eviction are deferred until history has been applied; actual primary expiry/eviction removals are journaled as DEL in removal order. Replicas hide expired values from readers but retain physical history until authoritative removal, allowing a delayed PERSIST or partial restart to preserve the primary's result. This is covered for core key/TTL history, transactions, scripts and REST; it does not establish deterministic replay for every extension, random operation or resource limit.

Headerless legacy AOFs are read using their existing expiry semantics, then atomically replaced by a v2 snapshot before writes are accepted. A legacy file lacks original execution timestamps and cannot recover expiry history that its old format did not preserve. Failed replay leaves a complete invalid log unchanged. Unsupported snapshot value types fail migration instead of silently becoming strings. Take a backup before upgrading: older binaries cannot read the v2 header, so downgrade requires a compatible export or a pre-upgrade backup. Converting a replica's legacy log invalidates its old partial-sync checkpoint and requires a full resync.

The AOF writer's snapshot replacement keeps appends serialized while preparing and syncing a separate file. It retains a backup before replacement and adopts the prepared file handle after success. A recoverable replacement failure preserves the old data and selected database, and subsequent appends continue on the restored file. If replacement and rollback both fail, the writer returns errors and reports the recovery path containing the previous data. AOF write, flush and fsync failures are retained until a new server process is started. TCP command, EXEC and script replies are buffered until their persistence hooks succeed; failure returns MISCONF instead of the computed reply. Later commands covered by the AOF write filter, including EXEC, are refused before execution. HTTP execute and direct REST writes return 503 on these errors. Read commands remain available, INFO persistence reports aof_last_write_status:err, and /api/ready returns 503. Repair the underlying storage problem before restarting. The first failing operation may already have changed memory or reached disk; MISCONF does not promise rollback, and retrying a non-idempotent operation can duplicate an effect. Background everysec/no-sync failure detection cannot revoke replies acknowledged before the failure was observed. Extension commands outside the persistence filter retain their existing persistence limitations. Buffering uses additional memory proportional to the write command's response. POSIX directory sync is checked; Windows replacement is cross-compiled but has no runtime or directory-durability verification. Successful shutdown now syncs AOF before publishing a matching replication checkpoint. Arbitrary power-loss recovery remains unverified.

`dump.rdb` is written in Redis' RDB format (version 11, or 12 when a hash has field expirations), so Redis 7.4, `redis-check-rdb` and other RDB tools read it, and a Redis `dump.rdb` can be copied in the other direction. Function libraries use Redis' own opcode. Under an LRU or LFU `maxmemory-policy` each key's idle time or LFU counter is saved too, as Redis does. What Redis has no type for is kept in ways Redis tolerates or refuses clearly:

- JSON documents and time series are saved as module values of the types `CStorm-JS` and `CStorm-TS`. Redis refuses to load such a file ("no matching module type 'CStorm-JS'"), just as it would refuse a RedisJSON file without the module.
- Key tags go in an aux field before the key, and search indexes in a `cachestorm-search` aux field; Redis ignores both.

CacheStorm still loads snapshots in its older native format. `redis-cli --rdb` also returns a Redis-format RDB, but leaves CacheStorm-only types out with a warning, so the file always loads in Redis.

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

Store.Set, GetOrCreate and planned bitmap/HLL/GEOADD mutations share atomic admission for the complete tracked footprint: value, key, entry and tags. Replacements credit the previous committed footprint, so an equal-size or smaller replacement does not need a second copy of dataset capacity. Admission retains the existing 95% headroom threshold and shares reservations across logical databases. NX/XX conditions use the locked live entry before reservation; KEEPTTL on a logically absent/expired key creates a persistent value. Input tag slices are copied, and expiry scheduling follows publication while the shard remains locked.

Noeviction rejection preserves the target and bookkeeping. Eviction policies can remove keys while attempting admission; those removals follow the existing journal and replica-checkpoint rules and are not rolled back if the requesting write ultimately fails. Victim selection still uses the issuing database. SetEntry also reserves the complete destination footprint and reports failures. RENAME/RENAMENX/MOVE/COPY reserve their net change while source and destination are locked; same-budget moves credit the removed source, and replacements credit the old destination. COPY clones the value only after admission. In-place collection/tag changes are not yet covered by this reservation invariant. max_memory tracks estimated dataset storage rather than allocator overhead, request/planning buffers or process RSS. Loading/replay retains its existing temporary admission bypass and accounting rules.

## Memory Limits

Memory limits can be specified with suffixes:

```yaml
memory:
  max_memory: "2gb"     # 2 gigabytes
  max_memory: "512mb"   # 512 megabytes
  max_memory: "1tb"     # 1 terabyte
  max_memory: "0"       # Unlimited
```

`used_memory` and `maxmemory` count each key's value, name and per-key bookkeeping, including its slot in the keyspace map. The process's RSS is higher, because it also includes client buffers, the Go runtime and the garbage collector's headroom. With `max_memory` set, the `cachestorm` binary gives the Go runtime a soft memory limit of 1.25 × max_memory + 64 MB. The garbage collector then works harder near the limit instead of letting the heap grow to twice the live data. On 600k small keys under a 100 MB limit, this halved RSS (383 MB → 199 MB). `CONFIG SET maxmemory` updates the limit.

State kept by CacheStorm's extension modules (queues, locks, rate limiters and the other `MODULE.VERB` commands that hold registries outside the keyspace) is not counted in `used_memory`. With `max_memory` set, an extension command that may add state is refused with Redis' OOM error when `used_memory` is over the limit, and also when the live Go heap is past 1.25 × max_memory + 64 MB. Extension commands that read or remove state (GET, LIST, LEN, POP, DELETE, CLEAR, RELEASE and similar verbs) keep working.

A `GOMEMLIMIT` environment variable overrides this. `GOGC` tunes the collector as usual: a lower value means a smaller heap and more GC CPU.

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

These environment variables override the file:

```bash
export CACHESTORM_CONFIG="/etc/cachestorm/cachestorm.yaml"
export CACHESTORM_BIND="0.0.0.0"
export CACHESTORM_PORT="6380"
export CACHESTORM_MAX_MEMORY="2gb"
export CACHESTORM_LOG_LEVEL="debug"
export CACHESTORM_REQUIREPASS="secret"
export CACHESTORM_HTTP_PASSWORD="secret"
export CACHESTORM_DATA_DIR="/data"
export CACHESTORM_PROTECTED_MODE="false"
```

The command line also accepts `--config`, `--bind`, `--port`, `--version` and any Redis directive as `--name value` (see "Redis-style options and redis.conf").

## Runtime Changes (CONFIG SET)

These parameters take effect immediately, for every connected client:

| Parameter | Effect |
|-----------|--------|
| `maxmemory` | New memory limit; accepts `100mb`, `4gb`, or bytes; `0` removes it |
| `maxmemory-policy` | Any of the eight Redis policies |
| `maxmemory-samples` | Keys sampled per eviction (1-64) |
| `maxclients` | Connection limit for new clients |
| `timeout` | Idle client timeout in seconds (`0` = never) |
| `requirepass` | New password; existing authenticated clients stay authenticated |
| `slowlog-log-slower-than`, `slowlog-max-len` | Slow log threshold (µs) and size |
| `notify-keyspace-events` | Keyspace notification classes (`K E g $ l s h z x e t m d n A`); see "Keyspace notifications" in commands-detailed.md |
| `save` | Save rules, `<seconds> <changes>` pairs as in Redis; `""` stops periodic snapshots. Needs `persistence.enabled` |

`port`, `bind`, `databases`, `dir`, `appendonly`, `appendfsync`,
`protected-mode` and `tcp-keepalive` are reported by `CONFIG GET` but are
immutable at runtime. Runtime changes are not written back to the YAML file.

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

HyperLogLog values persist as Redis HYLL binary strings through snapshots, AOF rewrite and replication. Single-key PFCOUNT may update its cached cardinality; that conditional cache effect is journaled but does not invalidate a replica's upstream checkpoint. PFCOUNT remains available on readonly replicas, in readonly scripts and after an existing AOF failure (without a cache write in the latter case). Previously persisted diagnostic strings cannot reconstruct lost registers; use original inputs or intact PFADD history. Unknown string-shaped values must provide an explicit RedisStringValue serializer; persistence rejects diagnostic-only representations.

Geo values created by GEOADD now use sorted-set geohash scores. Finite legacy native geo snapshots migrate on load, so coordinates become decoded cell centers and tracked capacity uses the sorted-set footprint. Unrepresentable historical NaN/out-of-range private values remain in their legacy type; retain backups and repair/export them before replacing those keys. Geo STORE effects persist the computed destination as RESTORE/DEL rather than rerunning a spatial query during recovery.

Full synchronization replaces the replica's function libraries and search indexes with the primary's, including an empty set. FUNCTION LOAD/DELETE and the FT.* writers also propagate through the live stream. If AOF is enabled on the replica, its rewritten AOF retains these libraries and indexes for restart without contacting the primary. Incoming full snapshots are decoded into isolated databases and a separate function registry. Only the native snapshot command set is accepted, and a configured memory budget is checked without evicting existing data. With AOF enabled, the candidate is persisted before replacing live data under the write gate. A truncated snapshot, rejected command or failed snapshot-to-AOF rewrite preserves the previous live keyspace and libraries; the failed snapshot is not accepted as a resumable replication position. The write gate remains held during snapshot reception and persistence, so command pause time still depends on transfer and disk speed.

A rejected live replication command ends the master session without advancing or forwarding that command's stream bytes. The replica discards resumable history, disconnects downstream replicas and requests a new full synchronization. While that state is invalid it refuses new downstream synchronization. This does not claim to undo partial in-memory effects of a live command that mutates data before reporting an error; that failure mode still needs separate handling.

A replica connects to its master's RESP port, loads a full copy of its
data, then applies every write the master executes. With AOF enabled a
replica persists what it applies and, after a clean restart, continues from
the master's backlog instead of resynchronizing everything.

The clean-shutdown checkpoint is versioned and checksummed. It records the upstream identity, stream database/offset, and a size/SHA-256 fingerprint of the closed AOF. Shutdown flushes and syncs AOF before publishing the checkpoint through a synced temporary file, rename and POSIX directory sync. Flush, fsync, close or checkpoint publication failure is returned from server shutdown. Startup removes the checkpoint and syncs that removal before accepting writes, then resumes only if its metadata and AOF fingerprint validate. A changed, missing or truncated AOF, corrupt/legacy checkpoint, changed upstream, or invalid database requires a full synchronization. If durable checkpoint invalidation fails, startup stops. Hashing adds a sequential read of the AOF at clean shutdown and when validating a checkpoint.

When local writes are allowed on a replica, an AOF-filtered local command, transaction, script effect or REST write invalidates resumable upstream history and disconnects downstream replicas. On upstream reconnection or restart it requires a full synchronization, which replaces those local changes. Promotion keeps the local data but discards the old partial-sync history. These rules do not establish persistence for extension state outside the AOF filter. Checkpoint I/O fault injection and real-process restart tests cover these paths; power-cut behavior, filesystem/hardware failure semantics and Windows directory durability still require separate validation.
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
