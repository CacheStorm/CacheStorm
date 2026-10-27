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

### Phase 2: the cluster bus

- A TCP bus on the bus port, carrying ping/pong gossip of node state and slot ownership, with config epochs.
- `CLUSTER MEET`/`FORGET`, and a forget blacklist.
- Conflict resolution by config epoch, and a collision-handling bump.
- Failure detection, as in Redis:
  - a node that misses its pong for longer than `node_timeout` is marked PFAIL;
  - once a majority of masters report PFAIL, it becomes FAIL, and the FAIL message is broadcast.
- Cluster state ok/fail from slot coverage and reachable masters.
- `PUBLISH` is broadcast over the bus.
- Done when `redis-cli --cluster create` with 3 masters succeeds and `redis-cli --cluster check` reports OK.

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
