# Cluster Mode Design

CacheStorm's cluster mode is meant to be used by unmodified Redis Cluster clients and tooling. Clients include go-redis `ClusterClient`, redis-py `RedisCluster` and Jedis. Tooling means `redis-cli --cluster create/check/reshard/rebalance/add-node/del-node`.

## Compatibility boundary

- **Client-facing (Redis 7.4-compatible).** This covers the commands and replies clients and `redis-cli` depend on:
  - the `CLUSTER` subcommands, `ASKING`, `READONLY` and `READWRITE`;
  - the redirections `MOVED`, `ASK`, `CROSSSLOT`, `TRYAGAIN` and `CLUSTERDOWN`;
  - key hashing with CRC16 modulo 16384 and `{hash tags}`;
  - the `CLUSTER NODES`, `CLUSTER SLOTS` and `CLUSTER SHARDS` formats, and the `nodes.conf` semantics.
- **Node-to-node (CacheStorm's own).** The cluster bus is CacheStorm's own protocol on the bus port, which defaults to client port + 10000. CacheStorm nodes form clusters only with other CacheStorm nodes; they cannot join a Redis cluster.

## Configuration

```yaml
cluster:
  enabled: true
  config_file: nodes.conf      # relative to persistence.data_dir
  node_timeout: 15000          # ms, as cluster-node-timeout
  bus_port: 0                  # 0 = port + 10000
  announce_ip: ""              # as cluster-announce-ip
  announce_port: 0
  announce_bus_port: 0
  require_full_coverage: true
  allow_reads_when_down: false
  replica_validity_factor: 10
```

In cluster mode only database 0 exists, as in Redis: `SELECT` with any other index fails.

## Phases

Each phase ends with every test suite passing and a commit. Tests stay bounded in memory, and each server they start is stopped.

### Phase 1: single-node cluster semantics

- Node identity: a 40-hex node ID, generated once and persisted.
- `nodes.conf` is loaded and saved, with atomic writes and an fsync. It holds the epoch, the slot owners, and the importing/migrating state.
- The slot table: 16384 entries mapping each slot to its owner, plus migrating and importing targets.
- A per-slot key index in the store, kept only in cluster mode. With it, `COUNTKEYSINSLOT` is O(1) and `GETKEYSINSLOT` is O(count) rather than a keyspace scan.
- The `CLUSTER` subcommands:
  - INFO, MYID, MYSHARDID, NODES, SLOTS, SHARDS;
  - KEYSLOT, COUNTKEYSINSLOT, GETKEYSINSLOT;
  - ADDSLOTS, ADDSLOTSRANGE, DELSLOTS, DELSLOTSRANGE, FLUSHSLOTS;
  - SETSLOT IMPORTING/MIGRATING/NODE/STABLE;
  - RESET, SAVECONFIG, BUMPEPOCH, SET-CONFIG-EPOCH;
  - LINKS, COUNT-FAILURE-REPORTS, HELP.
- Routing in the router, before a command runs:
  - Keys must hash to one slot, otherwise `CROSSSLOT`.
  - A slot owned by another node gets `MOVED slot host:port`.
  - A key in a migrating slot that is missing here gets `ASK`.
  - An importing slot is served only after `ASKING`.
  - A multi-key command that finds only some of its keys during a migration gets `TRYAGAIN`.
  - When the cluster is down, commands get `CLUSTERDOWN`.
  - The same rules apply to MULTI/EXEC (checked at queue time and at EXEC), to scripts (declared keys) and to `SPUBLISH`/`SSUBSCRIBE`.
- `READONLY`/`READWRITE` connection flags.
- `INFO cluster` reports `cluster_enabled:1`.
- Done when one node with `CLUSTER ADDSLOTSRANGE 0 16383` serves go-redis `ClusterClient` and redis-py `RedisCluster`.

### Phase 2: the cluster bus (done)

- A TCP bus on the bus port, carrying ping/pong gossip of node state and slot ownership, with config epochs.
- `CLUSTER MEET`/`FORGET`, and a forget blacklist.
- Conflict resolution by config epoch, and a collision-handling bump.
- Failure detection, as in Redis:
  - a node that misses its pong for longer than `node_timeout` is marked PFAIL;
  - once a majority of masters report PFAIL, it becomes FAIL, and the FAIL message is broadcast.
- Cluster state ok/fail from slot coverage and reachable masters.
- `PUBLISH` is broadcast over the bus.
- Done when `redis-cli --cluster create` with 3 masters succeeds and `redis-cli --cluster check` reports OK.

#### Bus protocol (version 1)

The bus lives in `internal/cluster` (`msg.go`, `bus.go`, `gossip.go`) and follows the model of Redis's `cluster_legacy.c`.

- **Frames.** Each frame starts with the magic `CSCB`, its total length (uint32, at most 16 MiB), the protocol version (uint16, 1), the message type (uint16, numbered as Redis numbers its types) and message flags (uint16, reserved). Integers are big endian. A frame of another version or with a bad magic closes the link.
- **Header.** Every message carries the sender's state:
  - its ID, current epoch, config epoch and replication offset;
  - its flags, port, bus port and TLS port, and whether its cluster state is ok;
  - its announced IP (empty means "use the connection's address"), its host name and its shard ID;
  - its master's ID (none for a master);
  - its 2048-byte slot bitmap (a replica sends its master's).
- **Bodies.**
  - PING, PONG and MEET carry a gossip section. It names a few random nodes (a tenth of the known nodes, at least 3, at most all but two) and every node in PFAIL. For each it gives the ID, IP, ports, flags, ping-sent and pong-received.
  - FAIL names the failing node.
  - UPDATE names a node with its config epoch and slot bitmap.
  - PUBLISH carries the channel and the message.
  - Types 5, 6 and 8 (failover auth request/ack, manual failover start) are reserved for phase 3. Unknown types are ignored after their header is processed.
- **Links.** Each node opens one outbound link to every known node and answers on the links it accepts; PONGs to a PING go back on the link it came on. Every link has a reader and a writer goroutine. Writes go through a queue capped at 64 MiB, which closes the link when exceeded. Reads and writes have deadlines. A failed dial is retried with a backoff doubling from 100 ms up to `node_timeout`/2 (at most 2 s).
- **Cron.** Every 100 ms the node:
  - drops handshakes older than max(`node_timeout`, 1 s);
  - opens the missing links;
  - once a second, pings the node with the oldest pong among 5 random ones;
  - pings every node whose pong is older than `node_timeout`/2;
  - recycles a link that has waited more than `node_timeout`/2 for a pong;
  - flags PFAIL a node whose ping has been pending for more than `node_timeout`.
- **Packet processing** follows `clusterProcessPacket`:
  - **Epochs.** The current and config epochs only grow.
  - **MEET.** A MEET from an unknown node adds it in handshake. The handshake node is renamed to the ID in its first PONG.
  - **Slots.** A master claiming slots is applied with `clusterUpdateSlotsConfigWith`: the greater config epoch wins, unassigned slots are taken, and importing slots are left alone. A node that loses a slot it holds keys for deletes them, and the deletions go to the AOF and the replicas. A sender with a stale claim gets an UPDATE.
  - **Epoch collisions.** Two masters with the same config epoch are resolved by `clusterHandleConfigEpochCollision`: the node with the smaller ID takes a new epoch.
  - **Gossip.** The gossip section adds nodes, unless they are blacklisted by `CLUSTER FORGET` for 60 s. It also carries failure reports from masters, which are valid for 2 × `node_timeout`.
  - **FAIL.** A PFAIL node reported by a majority of the slot-owning masters (this node counted) becomes FAIL, and a FAIL message is broadcast. A FAIL master that answers again is cleared after 2 × `node_timeout`; a replica, or a master with no slots, is cleared at once.
  - **PUBLISH.** A PUBLISH from a known node is delivered to the local subscribers only, so it is never forwarded again.
- **Cluster state.** The state is computed at every change: slot coverage (when `require_full_coverage` is set) and a majority of reachable slot-owning masters. Redis's writable delay after startup and its rejoin delay after a partition are not implemented.
- **nodes.conf.** The file is saved after every config change: a node added, renamed or removed, an epoch change, a slot change, a FAIL set or cleared, or an address change.
- **Tests.** The tests in `internal/server/cluster_bus_test.go` start in-process clusters with a 400 ms `node_timeout`. `scripts/cluster-smoke.sh` runs `redis-cli --cluster create`/`check` (Redis 7.4) against three processes.

### Phase 3: replicas and failover

- `CLUSTER REPLICATE` attaches a node as a replica, through the existing replication manager.
- `CLUSTER REPLICAS`/`SLAVES`.
- Automatic failover:
  - a replica whose master is in FAIL waits a delay based on its rank, then requests votes;
  - masters grant one vote per epoch;
  - with a majority, the replica promotes itself, takes the slots with a new config epoch and broadcasts.
- `CLUSTER FAILOVER [FORCE|TAKEOVER]`.
- Replica migration is not in scope.
- Done when `redis-cli --cluster create --cluster-replicas 1` with 6 nodes succeeds, killing a master promotes its replica, and clients keep working.

### Phase 4: resharding

- `MIGRATE ... KEYS` (already implemented) together with the SETSLOT flow.
- Done when `redis-cli --cluster reshard`, `rebalance`, `add-node` and `del-node` work on a loaded cluster without losing keys.

### Phase 5: hardening

- `scripts/cluster-smoke.sh` brings up 6 nodes and covers create, client traffic, reshard and failover.
- CI wiring for that script.
- Documentation: a cluster chapter.
