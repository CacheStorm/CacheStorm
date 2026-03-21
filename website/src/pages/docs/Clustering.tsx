import DocsLayout, {
  DocHeading,
  CodeBlock,
  InfoBox,
  type TocItem,
} from "@/components/DocsLayout";
import { Network, Server, GitBranch, Shield, Activity, RefreshCw } from "lucide-react";

const toc: TocItem[] = [
  { id: "overview", text: "Overview", level: 2 },
  { id: "architecture", text: "Architecture", level: 2 },
  { id: "replication", text: "Replication", level: 2 },
  { id: "replication-setup", text: "Setup", level: 3 },
  { id: "replication-config", text: "Configuration", level: 3 },
  { id: "replication-monitoring", text: "Monitoring", level: 3 },
  { id: "sentinel", text: "Sentinel Mode", level: 2 },
  { id: "sentinel-setup", text: "Sentinel Setup", level: 3 },
  { id: "sentinel-failover", text: "Failover", level: 3 },
  { id: "cluster-mode", text: "Cluster Mode", level: 2 },
  { id: "cluster-setup", text: "Cluster Setup", level: 3 },
  { id: "cluster-management", text: "Management", level: 3 },
  { id: "production", text: "Production Topology", level: 2 },
];

export default function Clustering() {
  return (
    <DocsLayout toc={toc}>
      {/* Hero */}
      <div className="mb-10">
        <div className="flex items-center gap-2 text-blue-400 text-sm font-medium mb-2">
          <Network className="w-4 h-4" />
          Operations
        </div>
        <h1 className="text-4xl font-extrabold text-white tracking-tight mb-4">
          Clustering &amp; High Availability
        </h1>
        <p className="text-lg text-slate-400 leading-relaxed max-w-2xl">
          Scale CacheStorm with replication, automatic failover via Sentinel mode,
          and horizontal scaling with cluster mode. This guide covers all high-availability
          deployment patterns.
        </p>
      </div>

      {/* ── Overview ─────────────────────────────────────────── */}
      <DocHeading id="overview" level={2}>
        Overview
      </DocHeading>

      <p className="mb-4 text-slate-400">
        CacheStorm supports three high-availability modes:
      </p>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 mb-6">
        {[
          {
            icon: <GitBranch className="w-5 h-5 text-blue-400" />,
            title: "Replication",
            desc: "Master-replica for read scaling and data redundancy",
          },
          {
            icon: <Shield className="w-5 h-5 text-emerald-400" />,
            title: "Sentinel",
            desc: "Automatic failover and service discovery",
          },
          {
            icon: <Network className="w-5 h-5 text-amber-400" />,
            title: "Cluster",
            desc: "Horizontal sharding across multiple nodes",
          },
        ].map((item) => (
          <div
            key={item.title}
            className="flex flex-col items-center gap-2 p-4 rounded-xl border border-slate-800 bg-slate-900/50 text-center"
          >
            {item.icon}
            <p className="text-sm font-semibold text-white">{item.title}</p>
            <p className="text-xs text-slate-500">{item.desc}</p>
          </div>
        ))}
      </div>

      {/* ── Architecture ─────────────────────────────────────── */}
      <DocHeading id="architecture" level={2}>
        Architecture
      </DocHeading>

      <p className="mb-4 text-slate-400">
        CacheStorm uses asynchronous replication with a single master accepting writes and one
        or more replicas receiving updates. This provides:
      </p>

      <ul className="list-disc list-inside text-slate-400 space-y-1 mb-4 ml-2">
        <li>Read scaling by distributing reads across replicas</li>
        <li>Data redundancy with automatic re-sync on reconnection</li>
        <li>Geographic distribution for lower latency</li>
        <li>Online backup without impacting the master</li>
      </ul>

      <div className="my-6 p-6 rounded-xl border border-slate-800 bg-slate-900/50 font-mono text-sm text-slate-400">
        <pre>{`                    ┌─────────────────┐
                    │   Application   │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  Load Balancer  │
                    └────────┬────────┘
                             │
               ┌─────────────┼─────────────┐
               │             │             │
        ┌──────▼──────┐ ┌───▼───┐ ┌───────▼──────┐
        │   Master    │ │Replica│ │   Replica    │
        │  (writes)   │ │ (read)│ │   (read)     │
        │ 10.0.1.10   │ │.1.11  │ │  10.0.1.12   │
        └──────┬──────┘ └───▲───┘ └───────▲──────┘
               │            │             │
               └────────────┴─────────────┘
                    Replication Stream`}</pre>
      </div>

      {/* ── Replication ──────────────────────────────────────── */}
      <DocHeading id="replication" level={2}>
        <GitBranch className="w-5 h-5 text-blue-400" />
        Replication
      </DocHeading>

      <DocHeading id="replication-setup" level={3}>
        Setup
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Set up a master with two replicas using Docker Compose:
      </p>

      <CodeBlock
        language="yaml"
        title="docker-compose.replication.yml"
        code={`services:
  master:
    image: ghcr.io/nicktretyakov/cachestorm:latest
    ports:
      - "6380:6380"
    volumes:
      - ./master.yaml:/etc/cachestorm/cachestorm.yaml
      - master-data:/data

  replica-1:
    image: ghcr.io/nicktretyakov/cachestorm:latest
    ports:
      - "6381:6380"
    volumes:
      - ./replica.yaml:/etc/cachestorm/cachestorm.yaml
      - replica1-data:/data
    depends_on:
      - master

  replica-2:
    image: ghcr.io/nicktretyakov/cachestorm:latest
    ports:
      - "6382:6380"
    volumes:
      - ./replica.yaml:/etc/cachestorm/cachestorm.yaml
      - replica2-data:/data
    depends_on:
      - master

volumes:
  master-data:
  replica1-data:
  replica2-data:`}
      />

      <DocHeading id="replication-config" level={3}>
        Configuration
      </DocHeading>

      <CodeBlock
        language="yaml"
        title="master.yaml"
        code={`server:
  port: 6380
  bind: "0.0.0.0"

cluster:
  replication:
    role: "master"
    # Optional: require password for replica connections
    repl_password: "\${CACHESTORM_REPL_PASSWORD}"

memory:
  maxmemory: "2gb"
  eviction_policy: "allkeys-lru"`}
      />

      <CodeBlock
        language="yaml"
        title="replica.yaml"
        code={`server:
  port: 6380
  bind: "0.0.0.0"

cluster:
  replication:
    role: "replica"
    master_addr: "master:6380"
    # Password to authenticate with master
    repl_password: "\${CACHESTORM_REPL_PASSWORD}"
    # Read-only mode (recommended for replicas)
    read_only: true

memory:
  maxmemory: "2gb"
  eviction_policy: "allkeys-lru"`}
      />

      <InfoBox type="info">
        Replicas automatically perform a full sync on first connection, then apply incremental
        changes from the master's replication stream.
      </InfoBox>

      <DocHeading id="replication-monitoring" level={3}>
        Monitoring
      </DocHeading>

      <CodeBlock
        language="bash"
        title="Monitor replication"
        code={`# On master: check connected replicas
redis-cli -p 6380 INFO replication
# role: master
# connected_replicas: 2
# replica0: ip=10.0.1.11,port=6380,state=online,lag=0
# replica1: ip=10.0.1.12,port=6380,state=online,lag=0

# On replica: check replication status
redis-cli -p 6381 INFO replication
# role: replica
# master_host: 10.0.1.10
# master_port: 6380
# master_link_status: up
# master_last_io_seconds_ago: 1
# repl_offset: 1234567`}
      />

      {/* ── Sentinel ─────────────────────────────────────────── */}
      <DocHeading id="sentinel" level={2}>
        <Shield className="w-5 h-5 text-blue-400" />
        Sentinel Mode
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Sentinel provides automatic failover when the master becomes unavailable. It monitors
        master and replica nodes, and promotes a replica to master when needed.
      </p>

      <DocHeading id="sentinel-setup" level={3}>
        Sentinel Setup
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Deploy at least 3 sentinel instances for reliable quorum-based failover.
      </p>

      <CodeBlock
        language="yaml"
        title="sentinel.yaml"
        code={`sentinel:
  enabled: true
  port: 26380

  # Monitor master instances
  monitors:
    - name: "cachestorm-primary"
      host: "10.0.1.10"
      port: 6380
      quorum: 2                    # Sentinels needed to agree on failure
      down_after_ms: 5000          # Time before marking as down
      failover_timeout_ms: 60000   # Failover timeout
      parallel_syncs: 1            # Replicas to sync simultaneously
      auth_password: "\${CACHESTORM_PASSWORD}"`}
      />

      <CodeBlock
        language="yaml"
        title="docker-compose.sentinel.yml"
        code={`services:
  sentinel-1:
    image: ghcr.io/nicktretyakov/cachestorm:latest
    command: ["cachestorm-sentinel", "--config", "/etc/cachestorm/sentinel.yaml"]
    ports:
      - "26380:26380"
    volumes:
      - ./sentinel.yaml:/etc/cachestorm/sentinel.yaml

  sentinel-2:
    image: ghcr.io/nicktretyakov/cachestorm:latest
    command: ["cachestorm-sentinel", "--config", "/etc/cachestorm/sentinel.yaml"]
    ports:
      - "26381:26380"
    volumes:
      - ./sentinel.yaml:/etc/cachestorm/sentinel.yaml

  sentinel-3:
    image: ghcr.io/nicktretyakov/cachestorm:latest
    command: ["cachestorm-sentinel", "--config", "/etc/cachestorm/sentinel.yaml"]
    ports:
      - "26382:26380"
    volumes:
      - ./sentinel.yaml:/etc/cachestorm/sentinel.yaml`}
      />

      <DocHeading id="sentinel-failover" level={3}>
        <RefreshCw className="w-4 h-4 text-cyan-400" />
        Failover
      </DocHeading>

      <CodeBlock
        language="bash"
        title="Sentinel commands"
        code={`# Connect to sentinel
redis-cli -p 26380

# Get master address
SENTINEL get-master-addr-by-name cachestorm-primary
# => ["10.0.1.10", "6380"]

# List monitored masters
SENTINEL masters

# List replicas for a master
SENTINEL replicas cachestorm-primary

# Force a failover (manual)
SENTINEL failover cachestorm-primary

# Check sentinel status
SENTINEL ckquorum cachestorm-primary
# => OK 3 usable sentinels. Quorum and failover authorization is possible.`}
      />

      <InfoBox type="tip">
        Client libraries with Sentinel support (like Lettuce or go-redis) will automatically
        discover the current master and reconnect after failover.
      </InfoBox>

      <CodeBlock
        language="go"
        title="Go client with Sentinel"
        code={`import "github.com/redis/go-redis/v9"

client := redis.NewFailoverClient(&redis.FailoverOptions{
    MasterName:    "cachestorm-primary",
    SentinelAddrs: []string{
        "10.0.1.20:26380",
        "10.0.1.21:26380",
        "10.0.1.22:26380",
    },
    Password: "your-password",
    DB:       0,
})`}
      />

      {/* ── Cluster Mode ─────────────────────────────────────── */}
      <DocHeading id="cluster-mode" level={2}>
        <Network className="w-5 h-5 text-blue-400" />
        Cluster Mode
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Cluster mode distributes data across multiple master nodes using hash slots.
        Each master owns a subset of the 16,384 hash slots, and data is sharded based on the key's
        hash slot.
      </p>

      <DocHeading id="cluster-setup" level={3}>
        Cluster Setup
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Minimum recommended cluster: 3 masters, each with 1 replica (6 nodes total).
      </p>

      <CodeBlock
        language="yaml"
        title="cluster-node.yaml"
        code={`server:
  port: 6380
  bind: "0.0.0.0"

cluster:
  enabled: true
  node_id: ""  # auto-generated
  announce_addr: ""  # set to the node's public IP

  # Cluster bus port (default: server port + 10000)
  bus_port: 16380

  # Node timeout before marking as failed
  node_timeout_ms: 15000

  # Require full slot coverage for the cluster to accept writes
  require_full_coverage: true

memory:
  maxmemory: "4gb"
  eviction_policy: "allkeys-lru"`}
      />

      <CodeBlock
        language="yaml"
        title="docker-compose.cluster.yml"
        code={`services:
  node-1:
    image: ghcr.io/nicktretyakov/cachestorm:latest
    ports: ["6380:6380", "16380:16380"]
    environment:
      CACHESTORM_CLUSTER_ENABLED: "true"
      CACHESTORM_CLUSTER_ANNOUNCE_ADDR: "node-1:6380"
    volumes:
      - node1-data:/data

  node-2:
    image: ghcr.io/nicktretyakov/cachestorm:latest
    ports: ["6381:6380", "16381:16380"]
    environment:
      CACHESTORM_CLUSTER_ENABLED: "true"
      CACHESTORM_CLUSTER_ANNOUNCE_ADDR: "node-2:6380"
    volumes:
      - node2-data:/data

  node-3:
    image: ghcr.io/nicktretyakov/cachestorm:latest
    ports: ["6382:6380", "16382:16380"]
    environment:
      CACHESTORM_CLUSTER_ENABLED: "true"
      CACHESTORM_CLUSTER_ANNOUNCE_ADDR: "node-3:6380"
    volumes:
      - node3-data:/data

  node-4:
    image: ghcr.io/nicktretyakov/cachestorm:latest
    ports: ["6383:6380", "16383:16380"]
    environment:
      CACHESTORM_CLUSTER_ENABLED: "true"
      CACHESTORM_CLUSTER_ANNOUNCE_ADDR: "node-4:6380"
    volumes:
      - node4-data:/data

  node-5:
    image: ghcr.io/nicktretyakov/cachestorm:latest
    ports: ["6384:6380", "16384:16380"]
    environment:
      CACHESTORM_CLUSTER_ENABLED: "true"
      CACHESTORM_CLUSTER_ANNOUNCE_ADDR: "node-5:6380"
    volumes:
      - node5-data:/data

  node-6:
    image: ghcr.io/nicktretyakov/cachestorm:latest
    ports: ["6385:6380", "16385:16380"]
    environment:
      CACHESTORM_CLUSTER_ENABLED: "true"
      CACHESTORM_CLUSTER_ANNOUNCE_ADDR: "node-6:6380"
    volumes:
      - node6-data:/data

volumes:
  node1-data:
  node2-data:
  node3-data:
  node4-data:
  node5-data:
  node6-data:`}
      />

      <CodeBlock
        language="bash"
        title="Initialize the cluster"
        code={`# Create the cluster with 3 masters and 3 replicas
cachestorm-cli --cluster create \\
  node-1:6380 node-2:6380 node-3:6380 \\
  node-4:6380 node-5:6380 node-6:6380 \\
  --cluster-replicas 1

# Verify cluster status
cachestorm-cli --cluster info -p 6380

# Check slot distribution
cachestorm-cli --cluster slots -p 6380

# Example output:
# Master 1: slots 0-5460     (node-1, replica: node-4)
# Master 2: slots 5461-10922 (node-2, replica: node-5)
# Master 3: slots 10923-16383 (node-3, replica: node-6)`}
      />

      <DocHeading id="cluster-management" level={3}>
        Management
      </DocHeading>

      <CodeBlock
        language="bash"
        title="Cluster management commands"
        code={`# Add a new node
cachestorm-cli --cluster add-node new-node:6380 existing-node:6380

# Add a replica
cachestorm-cli --cluster add-node new-node:6380 existing-node:6380 \\
  --cluster-slave --cluster-master-id <master-node-id>

# Remove a node (must be empty or a replica)
cachestorm-cli --cluster del-node existing-node:6380 <node-id>

# Reshard slots between nodes
cachestorm-cli --cluster reshard existing-node:6380

# Rebalance slots evenly
cachestorm-cli --cluster rebalance existing-node:6380

# Check cluster health
cachestorm-cli --cluster check existing-node:6380

# Fix cluster (resolve open/migrating slots)
cachestorm-cli --cluster fix existing-node:6380`}
      />

      <InfoBox type="info">
        When using cluster mode, use hash tags <code className="text-xs bg-slate-800 px-1 py-0.5 rounded">{"{tag}"}</code> to
        ensure related keys are stored on the same node. For example:{" "}
        <code className="text-xs bg-slate-800 px-1 py-0.5 rounded">{"{user:1}:name"}</code> and{" "}
        <code className="text-xs bg-slate-800 px-1 py-0.5 rounded">{"{user:1}:email"}</code> will
        share the same slot.
      </InfoBox>

      {/* ── Production Topology ──────────────────────────────── */}
      <DocHeading id="production" level={2}>
        <Activity className="w-5 h-5 text-blue-400" />
        Production Topology
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Recommended production topology for different workload sizes:
      </p>

      <div className="my-4 rounded-xl border border-slate-800 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-slate-800 text-left text-slate-400">
                <th className="px-4 py-2 font-medium">Scale</th>
                <th className="px-4 py-2 font-medium">Topology</th>
                <th className="px-4 py-2 font-medium">Nodes</th>
                <th className="px-4 py-2 font-medium">Best For</th>
              </tr>
            </thead>
            <tbody className="text-slate-300">
              {[
                ["Small", "1 master + 1 replica", "2", "Dev/staging, low traffic"],
                ["Medium", "1 master + 2 replicas + 3 sentinels", "6", "Production with auto-failover"],
                ["Large", "3 masters + 3 replicas (cluster)", "6", "High throughput, large datasets"],
                ["Enterprise", "6 masters + 6 replicas + 3 sentinels", "15", "Mission-critical, multi-AZ"],
              ].map(([scale, topology, nodes, best], i, arr) => (
                <tr key={scale} className={i < arr.length - 1 ? "border-b border-slate-800/60" : ""}>
                  <td className="px-4 py-2 font-medium text-white">{scale}</td>
                  <td className="px-4 py-2 text-slate-400">{topology}</td>
                  <td className="px-4 py-2 text-center text-blue-300">{nodes}</td>
                  <td className="px-4 py-2 text-slate-500">{best}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <div className="space-y-3 mt-6 mb-4">
        {[
          {
            title: "Spread across availability zones",
            desc: "Place master and replicas in different AZs to survive zone failures.",
          },
          {
            title: "Use dedicated instances",
            desc: "Avoid noisy-neighbor issues by running CacheStorm on dedicated VMs or bare metal.",
          },
          {
            title: "Size memory appropriately",
            desc: "Set maxmemory to 75% of available RAM to leave room for fragmentation and forks.",
          },
          {
            title: "Monitor replication lag",
            desc: "Alert if replication lag exceeds 1 second to catch network or performance issues early.",
          },
        ].map((item) => (
          <div
            key={item.title}
            className="flex items-start gap-3 p-3 rounded-lg border border-slate-800 bg-slate-900/30"
          >
            <div className="w-1.5 h-1.5 rounded-full bg-blue-400 mt-2 shrink-0" />
            <div>
              <p className="text-sm font-medium text-white">{item.title}</p>
              <p className="text-xs text-slate-500 mt-0.5">{item.desc}</p>
            </div>
          </div>
        ))}
      </div>
    </DocsLayout>
  );
}
