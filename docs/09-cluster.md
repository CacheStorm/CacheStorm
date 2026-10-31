# Running a Cluster

CacheStorm's cluster mode follows Redis Cluster. Unmodified cluster clients connect to it, and `redis-cli --cluster` manages it. The configuration reference is in [Configuration → Cluster mode](./02-configuration.md#cluster-mode), and the internals are in the [design notes](./cluster-design.md).

## Quick start: three masters, three replicas

Give every node its own port and data directory:

```yaml
# node-7000.yaml (repeat for 7001..7005)
server:
  bind: "0.0.0.0"
  port: 7000
persistence:
  enabled: true
  data_dir: /var/lib/cachestorm/7000
cluster:
  enabled: true
  node_timeout: 15000
```

Start the six processes, then let `redis-cli` assign slots and replicas:

```bash
redis-cli --cluster create \
  10.0.0.1:7000 10.0.0.1:7001 10.0.0.1:7002 \
  10.0.0.1:7003 10.0.0.1:7004 10.0.0.1:7005 \
  --cluster-replicas 1
redis-cli --cluster check 10.0.0.1:7000
```

Each node also listens on its cluster bus port: by default the client port plus 10000, so 17000 to 17005 here. Open those ports between the nodes, but not to clients. The bus is CacheStorm's own protocol, so CacheStorm nodes cannot join a Redis cluster and Redis nodes cannot join a CacheStorm cluster. Without TLS the bus has no encryption or authentication, which is also true of Redis's bus. Keep it on a private network, or set `cluster.tls: true` with `server.tls_cert_file`, `tls_key_file` and `tls_ca_cert_file`. The bus then uses mutual TLS, and nodes are verified against the CA. When the client port uses TLS, replicas and `MIGRATE` connect over TLS as well.

When the masters use `requirepass`, set `replication.master_auth` to the same password so their replicas can authenticate.

## Clients

Use your client library's cluster client: go-redis `ClusterClient`, redis-py `RedisCluster`, ioredis `Cluster`, Jedis `JedisCluster` or Lettuce. They read `CLUSTER SLOTS` or `CLUSTER SHARDS` and follow the `MOVED` and `ASK` redirections.

Keys that one command uses together must hash to the same slot. Use a hash tag, `{user:42}:profile` and `{user:42}:sessions`, or the command is refused with `CROSSSLOT`. Only database 0 exists. With `READONLY`, a client may read from replicas.

## Scaling and rebalancing

The standard `redis-cli --cluster` workflows work while clients keep reading and writing:

```bash
# move 1000 slots between two masters
redis-cli --cluster reshard 10.0.0.1:7000 --cluster-from <src-id> --cluster-to <dst-id> --cluster-slots 1000 --cluster-yes

# add a master, then give it slots
redis-cli --cluster add-node 10.0.0.2:7006 10.0.0.1:7000
redis-cli --cluster rebalance 10.0.0.1:7000 --cluster-use-empty-masters

# add a replica of a given master
redis-cli --cluster add-node 10.0.0.2:7007 10.0.0.1:7000 --cluster-slave --cluster-master-id <master-id>

# remove a master: move its slots away first, then delete it
redis-cli --cluster del-node 10.0.0.1:7000 <node-id>

# repair open slots after an interrupted migration
redis-cli --cluster fix 10.0.0.1:7000
```

Keys move with `MIGRATE`. Their TTLs and encodings are preserved, and so are streams with their consumer groups, JSON documents and time series.

## Failover

When a master stops answering for `node_timeout`, the other masters mark it failed. One of its replicas then wins an election and takes over its slots. Clients follow the new `MOVED` replies. When the old master comes back, it becomes a replica of the new one.

For planned maintenance, run `CLUSTER FAILOVER` on a replica. It pauses the master's writers, waits until the replica has caught up, and then switches roles, so no acknowledged write is lost. `FORCE` and `TAKEOVER` are for a master that cannot be reached.

## Verified behaviour

`scripts/cluster-smoke.sh` runs in CI. It does the following:
1. Creates a cluster of three masters with replicas using `redis-cli --cluster create --cluster-replicas 1`.
2. Kills a master and checks that its replica takes over and that every key survives.
3. Restarts the old master and checks that it rejoins as a replica.
4. Reshards, adds a master and a replica, rebalances, deletes a node, and fixes an open slot.

A go-redis `ClusterClient` reads and writes throughout. At the end the script checks that no write was lost and that no command failed.

## Replica migration and rejoin delays

When a master serving slots loses its last working replica, a replica of the master with the most working replicas (more than `cluster-migration-barrier`, default 1) moves to it after 5 s. Of those replicas, the one with the smallest node ID moves. `cluster-allow-replica-migration no` turns this off. A replica with `cluster-replica-no-failover yes` never starts an automatic failover; `CLUSTER FAILOVER` still works on it. All three settings change at run time with `CONFIG SET`.

A master that was on the minority side of a partition waits `cluster-node-timeout` (clamped to 0.5–5 s) after it sees a majority again before it serves writes. A master restarted from `nodes.conf` next to other nodes waits 2 s. In both cases the node first learns whether a replica took its place. Redis applies the same delays.

## Limitations

- **DUMP payloads:** payloads of CacheStorm-only types (JSON and time series) cannot be restored into Redis. Streams use Redis' encoding and move both ways.
- **Extension registries:** process-wide extension registries, such as queues, locks and counters outside the keyspace, are not partitioned by slot.
