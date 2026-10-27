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
  - FAILOVER_AUTH_REQUEST (5), FAILOVER_AUTH_ACK (6) and MFSTART (8) carry only the header. The message flags carry PAUSED (bit 0, a master paused for a manual failover) and FORCEACK (bit 1, a vote request for a manual failover). Unknown types are ignored after their header is processed.
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

### Phase 3: replicas and failover (done)

- `CLUSTER REPLICATE` attaches a node as a replica, through the existing replication manager.
- `CLUSTER REPLICAS`/`SLAVES`.
- Automatic failover:
  - a replica whose master is in FAIL waits a delay based on its rank, then requests votes;
  - masters grant one vote per epoch;
  - with a majority, the replica promotes itself, takes the slots with a new config epoch and broadcasts.
- `CLUSTER FAILOVER [FORCE|TAKEOVER]`.
- Replica migration is not in scope.
- Done when `redis-cli --cluster create --cluster-replicas 1` with 6 nodes succeeds, killing a master promotes its replica, and clients keep working.

#### How it works

The failover code is in `internal/cluster/failover.go` and follows `cluster_legacy.c`.

- **Replicas.** `CLUSTER REPLICATE` is `clusterSetMaster`. A master needs no slots and an empty database 0 ("To set a master the node must be empty and without assigned slots."); the master can't be the node itself or a replica. The node takes its master's shard ID and closes its open slots. Its PINGs carry the replica flag, its master's ID, its master's slot bitmap and config epoch, and its replication offset. Every node records the offsets it hears.
- **Replication.** The cluster package never calls the replication manager under its lock. A role goroutine started with the bus compares the role in the cluster state with the one it last applied, and points replication at the master's address (`ReplicaOf`) or stops it (`Promote`, as `REPLICAOF NO ONE`). It runs when the role changes and once a second, so a master that changes address is followed too. A node restarted from a nodes.conf that says `slave` resumes replicating.
- **Reads.** A replica serves a keyed command for a slot of its master when the client sent `READONLY` (or for a shard channel) and the command, or a queued command of an EXEC, does not write; anything else gets `MOVED`. In cluster mode the redirection comes before the `READONLY You can't write against a read only replica` refusal, as in Redis; a write without keys, such as FLUSHALL, still gets that refusal.
- **Replies.** `CLUSTER SLOTS` lists a replica after its master once it is not FAIL and has a non-zero replication offset (`isReplicaAvailable`). `CLUSTER SHARDS` reports `role`, `replication-offset` and `health` (`fail`, `loading` for a replica at offset 0, `online`).
- **Election** (`clusterHandleSlaveFailover`, run by the cron and on every vote):
  - It runs on a replica whose master is FAIL and serves slots, or during a manual failover.
  - Unless the failover is manual, the replica gives up when its data is older than `repl-ping-replica-period` (10 s) + `node_timeout` × `replica_validity_factor`. The age is the time since the last read from the master, or since the link went down, less `node_timeout`.
  - It schedules the election at 500 ms + random(0–500 ms) + rank × 1 s; the rank counts sibling replicas with a greater offset. It then sends a PONG to those siblings, and adds delay if its rank worsens before it asks.
  - It bumps `currentEpoch` and broadcasts FAILOVER_AUTH_REQUEST (type 5). A manual failover sets the FORCEACK flag.
  - An attempt expires after max(2 × `node_timeout`, 2 s) and is retried after twice that.
- **Votes** (`clusterSendFailoverAuthIfNeeded`). Only a master that serves slots votes. It refuses:
  - a request whose epoch is older than its `currentEpoch`;
  - a second vote in one epoch (`lastVoteEpoch`, saved to nodes.conf before the FAILOVER_AUTH_ACK, type 6, goes out);
  - a requester that is not a replica, or whose master is unknown, or up without FORCEACK;
  - a second vote for the same master within 2 × `node_timeout`;
  - a claimed slot whose owner has a newer config epoch.

  The replica counts an ACK only from a slot-owning master whose epoch is at least the election epoch.
- **Promotion** (`clusterFailoverReplaceYourMaster`). With votes from a majority of the slot-owning masters, the replica sets its config epoch to the election epoch. The role goroutine then stops replication first, keeping the data, so the node never serves its new slots as a read-only replica. Only then does it take the old master's slots, save nodes.conf and broadcast a PONG.
- **Demotion.** In `clusterUpdateSlotsConfigWith`, a master left without slots by a newer config becomes a replica of the node that took them, as does a replica whose master was emptied (with `cluster-allow-replica-migration` on, its Redis default). A sub-replica moves to its grandmaster. A returning old master claims its slots with its old epoch; the other nodes answer with an UPDATE, and it becomes a replica of the new master. When a node that was a master announces itself as a replica of a master of the same shard, the other nodes give that master its slots.
- **Manual failover** (`CLUSTER FAILOVER`):
  - The replica sends MFSTART (type 8) to its master.
  - The master pauses client writes for 10 s and waits for the writes in flight. A write that took the write gate after the pause began goes back to waiting, so the offset it then reads is final. It pings the replica every 100 ms with the PAUSED flag and that offset.
  - Once the replica has processed up to that offset, the election starts at once and without the data age check.
  - The demoted master resumes its writers; their commands then get `MOVED`.
  - A manual failover that has not finished in 5 s is abandoned and the master resumes.
  - `FORCE` skips the master: the election starts at once.
  - `TAKEOVER` bumps the config epoch without consensus (`clusterBumpConfigEpochWithoutConsensus`) and promotes without an election.
- **Not implemented.** Replica migration to orphaned masters, `cluster-replica-no-failover` (the `nofailover` flag is carried but cannot be set) and the writable delay after a failover.
- **Tests.**
  - `internal/server/cluster_failover_test.go` runs 3 masters + 3 replicas in process with a 400 ms `node_timeout`. It covers the REPLICATE errors, NODES/SLOTS/SHARDS/REPLICAS shape, READONLY/READWRITE and MOVED, write propagation, automatic failover after a master is stopped (about 1.5 s) with the old master rejoining as a replica, and a manual failover under a concurrent writer that loses no acknowledged write. It also covers FORCE, TAKEOVER and the FORCE-needed error.
  - `internal/cluster/failover_test.go` checks the vote rules and the replica rank.
  - `scripts/cluster-smoke.sh` creates a 6-node cluster with `--cluster-replicas 1`, kills a master process, and checks that `redis-cli --cluster check` is OK again, the keys survive and go-redis ClusterClient still works.

### Phase 4: resharding (done)

- `MIGRATE ... KEYS` together with the SETSLOT flow.
- Done when `redis-cli --cluster reshard`, `rebalance`, `add-node` and `del-node` work on a loaded cluster without losing keys.

#### How it works

redis-cli moves a slot with `clusterManagerMoveSlot`: `SETSLOT IMPORTING` on the target, `SETSLOT MIGRATING` on the source, then `CLUSTER GETKEYSINSLOT slot <pipeline>` and `MIGRATE host port "" 0 timeout [REPLACE] [AUTH ...] KEYS ...` until the source holds no key of the slot, then `SETSLOT NODE <target>` on the target, the source and every other master.

- **SETSLOT** (`internal/cluster/cluster.go`, `SetSlot`) follows Redis 7.4's `clusterCommandSetSlot`:
  - MIGRATING needs the slot to be this node's and IMPORTING needs it not to be; both need a known master. They are saved to nodes.conf (`[slot->-id]`, `[slot-<-id]`).
  - NODE is refused while this node owns the slot and still holds keys for it; it never deletes keys. With no keys left, the MIGRATING state clears.
  - A master that gives away its last slot becomes a replica of the new owner (`cluster-allow-replica-migration`, on in Redis by default). redis-cli's next SETSLOT to it gets `Please use SETSLOT only with masters.`, which redis-cli accepts.
  - The importing node that takes the slot bumps its config epoch without consensus (`clusterBumpConfigEpochWithoutConsensus`), clears IMPORTING and broadcasts a PONG. The other nodes take the slot through `clusterUpdateSlotsConfigWith` (the greater config epoch wins); a node with a stale claim gets an UPDATE. Gossip never moves a slot this node is importing.
- **MIGRATE** (`internal/command/cluster_commands.go`) sends `AUTH`, `SELECT db` and one `RESTORE-ASKING` (`RESTORE` outside cluster mode) per key with its DUMP payload and remaining TTL, in 64 KiB chunks, while a reader collects the replies; the timeout applies to each read and write. The keys the target accepted are deleted and recorded as one DEL in the AOF and the replication stream. MIGRATE takes the write gate exclusively, so a write to a key cannot land between its dump and its deletion, and routing for a write waiting on the gate is decided after MIGRATE is done.
- **RESTORE-ASKING** is RESTORE with Redis' `asking` command flag: routing serves it for an importing slot without a preceding `ASKING`. It is recorded as `RESTORE ... ABSTTL`, so the target's replicas and AOF get the keys with their absolute expiry.
- **DUMP payloads.** Strings, lists, sets, hashes and sorted sets use Redis's RDB encodings. Streams (with their consumer groups), JSON documents and time series, which have none, use private type bytes 200–202, so they migrate whole between CacheStorm nodes; Redis refuses those payloads. Each key keeps its encoding after RESTORE because CacheStorm derives the encoding from the value.
- **Redirections** during a migration (`clusterRoute`, as `getNodeByQuery`): on the source, a command whose keys are all missing gets `ASK slot target`, one that finds only some of them gets `TRYAGAIN`; on the target, a command after `ASKING` (or RESTORE-ASKING) is served unless it names several keys and some are missing (`TRYAGAIN`); without ASKING it gets `MOVED` to the source. MIGRATE itself is always served on a migrating or importing slot. After SETSLOT NODE, every node answers `MOVED` to the new owner.
- **Tests.** `internal/server/cluster_migration_test.go` migrates a slot holding strings (with and without TTL, integer-encoded), small and large hashes, int and string sets, a sorted set, a list and a stream between two masters of a 3 + 3 cluster, checking ASK/ASKING/TRYAGAIN/RESTORE-ASKING in the middle, the MOVED replies, DUMP and OBJECT ENCODING and TTLs after, the keys on both replicas, the epochs and nodes.conf; it also checks that a master emptied by SETSLOT NODE becomes a replica. `scripts/cluster-smoke.sh` runs reshard, add-node (master and replica), rebalance `--cluster-use-empty-masters`, del-node and fix with redis-cli 7.4 under go-redis ClusterClient traffic.
- **Not implemented.** Redis' cached MIGRATE connections (each MIGRATE dials the target); `SETSLOT` replication to replicas, which arrived after Redis 7.4.

### Phase 5: hardening (done)

- `scripts/cluster-smoke.sh` brings up 6 nodes and covers create, client traffic, reshard and failover.
- CI wiring for that script.
- Documentation: a cluster chapter, [Running a cluster](./09-cluster.md).
