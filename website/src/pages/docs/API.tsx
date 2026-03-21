import DocsLayout, {
  DocHeading,
  CodeBlock,
  InfoBox,
  type TocItem,
} from "@/components/DocsLayout";
import { Globe, Heart, Terminal, Database, BarChart3, Shield, Settings } from "lucide-react";

const toc: TocItem[] = [
  { id: "overview", text: "Overview", level: 2 },
  { id: "authentication", text: "Authentication", level: 2 },
  { id: "health", text: "Health Endpoints", level: 2 },
  { id: "command-exec", text: "Command Execution", level: 2 },
  { id: "key-ops", text: "Key Operations", level: 2 },
  { id: "info", text: "Server Info", level: 2 },
  { id: "metrics-api", text: "Metrics", level: 2 },
  { id: "config-api", text: "Configuration", level: 2 },
  { id: "errors", text: "Error Handling", level: 2 },
  { id: "rate-limiting", text: "Rate Limiting", level: 2 },
  { id: "sdks", text: "Client SDKs", level: 2 },
];

function EndpointBadge({ method }: { method: "GET" | "POST" | "PUT" | "DELETE" }) {
  const colors = {
    GET: "text-green-600 dark:text-green-400 bg-emerald-500/10 border-emerald-500/30",
    POST: "text-[var(--color-primary)] bg-[var(--color-surface)] border-blue-500/30",
    PUT: "text-amber-400 bg-amber-500/10 border-amber-500/30",
    DELETE: "text-red-400 bg-red-500/10 border-red-500/30",
  };

  return (
    <span className={`text-xs font-mono font-bold px-2 py-0.5 rounded border ${colors[method]}`}>
      {method}
    </span>
  );
}

function Endpoint({
  method,
  path,
  desc,
}: {
  method: "GET" | "POST" | "PUT" | "DELETE";
  path: string;
  desc: string;
}) {
  return (
    <div className="py-3 border-b border-[var(--color-border)] last:border-0">
      <div className="flex items-center gap-2.5 flex-wrap mb-1">
        <EndpointBadge method={method} />
        <code className="text-sm font-bold text-[var(--color-text)]">{path}</code>
      </div>
      <p className="text-sm text-[var(--color-text-secondary)] ml-0.5">{desc}</p>
    </div>
  );
}

export default function API() {
  return (
    <DocsLayout toc={toc}>
      {/* Hero */}
      <div className="mb-10">
        <div className="flex items-center gap-2 text-[var(--color-primary)] text-sm font-medium mb-2">
          <Globe className="w-4 h-4" />
          Reference
        </div>
        <h1 className="text-4xl font-extrabold text-[var(--color-text)] tracking-tight mb-4">
          HTTP API Reference
        </h1>
        <p className="text-lg text-[var(--color-text-secondary)] leading-relaxed max-w-2xl">
          CacheStorm provides a RESTful HTTP API for management, monitoring, and executing
          commands. The API runs on a separate port (default: 7280) alongside the RESP protocol.
        </p>
      </div>

      {/* ── Overview ─────────────────────────────────────────── */}
      <DocHeading id="overview" level={2}>
        Overview
      </DocHeading>

      <p className="mb-3 text-[var(--color-text-secondary)]">
        The HTTP API provides a JSON-based interface for all CacheStorm operations. It runs
        alongside the main RESP server on a configurable port.
      </p>

      <CodeBlock
        language="yaml"
        title="Enable the HTTP API"
        code={`http:
  enabled: true
  port: 7280
  bind: "0.0.0.0"
  cors_origins: ["*"]`}
      />

      <p className="mb-3 text-[var(--color-text-secondary)]">Base URL:</p>
      <CodeBlock
        language="bash"
        code={`http://localhost:7280`}
      />

      <p className="mb-4 text-[var(--color-text-secondary)]">All endpoints summary:</p>

      <div className="rounded-xl border border-[var(--color-border)] overflow-hidden px-4 mb-6">
        <Endpoint method="GET" path="/health" desc="Health check" />
        <Endpoint method="GET" path="/health/ready" desc="Readiness check" />
        <Endpoint method="POST" path="/api/v1/command" desc="Execute any CacheStorm command" />
        <Endpoint method="GET" path="/api/v1/keys/:key" desc="Get the value of a key" />
        <Endpoint method="PUT" path="/api/v1/keys/:key" desc="Set a key value" />
        <Endpoint method="DELETE" path="/api/v1/keys/:key" desc="Delete a key" />
        <Endpoint method="GET" path="/api/v1/info" desc="Server information" />
        <Endpoint method="GET" path="/api/v1/info/:section" desc="Specific info section" />
        <Endpoint method="GET" path="/api/v1/metrics" desc="Prometheus metrics" />
        <Endpoint method="GET" path="/api/v1/config" desc="Get configuration" />
        <Endpoint method="PUT" path="/api/v1/config" desc="Update configuration at runtime" />
      </div>

      {/* ── Authentication ───────────────────────────────────── */}
      <DocHeading id="authentication" level={2}>
        <Shield className="w-5 h-5 text-[var(--color-primary)]" />
        Authentication
      </DocHeading>

      <p className="mb-3 text-[var(--color-text-secondary)]">
        When a server password is configured, the HTTP API requires authentication via
        Bearer token or Basic auth.
      </p>

      <CodeBlock
        language="bash"
        title="Authentication methods"
        code={`# Bearer token (using the server password)
curl -H "Authorization: Bearer your-password" \\
  http://localhost:7280/api/v1/info

# Basic auth (username:password)
curl -u "default:your-password" \\
  http://localhost:7280/api/v1/info

# ACL user authentication
curl -u "admin:admin-password" \\
  http://localhost:7280/api/v1/info`}
      />

      <InfoBox type="warning">
        Always use TLS when authenticating over HTTP to prevent credential interception.
        Configure the HTTP server with the same TLS certificates as the RESP server.
      </InfoBox>

      {/* ── Health ───────────────────────────────────────────── */}
      <DocHeading id="health" level={2}>
        <Heart className="w-5 h-5 text-[var(--color-primary)]" />
        Health Endpoints
      </DocHeading>

      <p className="mb-3 text-[var(--color-text-secondary)]">
        Use these endpoints for load balancer health checks and Kubernetes probes.
      </p>

      <CodeBlock
        language="bash"
        title="Health check"
        code={`# Liveness probe
curl http://localhost:7280/health
# => {"status": "ok", "uptime": 3600}

# Readiness probe (checks if server is accepting commands)
curl http://localhost:7280/health/ready
# => {"status": "ready", "connections": 42, "memory_usage": 0.45}`}
      />

      <CodeBlock
        language="json"
        title="Response: /health"
        code={`{
  "status": "ok",
  "version": "1.2.0",
  "uptime": 86400,
  "uptime_human": "1d 0h 0m"
}`}
      />

      <CodeBlock
        language="json"
        title="Response: /health/ready"
        code={`{
  "status": "ready",
  "connections": 42,
  "memory_usage": 0.45,
  "memory_used": "460 MB",
  "memory_max": "1024 MB",
  "keys": 125000,
  "ops_per_sec": 15000
}`}
      />

      {/* ── Command Execution ────────────────────────────────── */}
      <DocHeading id="command-exec" level={2}>
        <Terminal className="w-5 h-5 text-[var(--color-primary)]" />
        Command Execution
      </DocHeading>

      <p className="mb-3 text-[var(--color-text-secondary)]">
        Execute any CacheStorm command through the HTTP API.
      </p>

      <CodeBlock
        language="bash"
        title="POST /api/v1/command"
        code={`# Simple command
curl -X POST http://localhost:7280/api/v1/command \\
  -H "Content-Type: application/json" \\
  -d '{"command": "PING"}'
# => {"result": "PONG"}

# SET command
curl -X POST http://localhost:7280/api/v1/command \\
  -H "Content-Type: application/json" \\
  -d '{"command": "SET", "args": ["mykey", "myvalue", "EX", "3600"]}'
# => {"result": "OK"}

# GET command
curl -X POST http://localhost:7280/api/v1/command \\
  -H "Content-Type: application/json" \\
  -d '{"command": "GET", "args": ["mykey"]}'
# => {"result": "myvalue"}

# Hash operations
curl -X POST http://localhost:7280/api/v1/command \\
  -H "Content-Type: application/json" \\
  -d '{"command": "HGETALL", "args": ["user:1"]}'
# => {"result": {"name": "Alice", "email": "alice@example.com"}}

# Pipeline (multiple commands)
curl -X POST http://localhost:7280/api/v1/command/pipeline \\
  -H "Content-Type: application/json" \\
  -d '{
    "commands": [
      {"command": "SET", "args": ["key1", "val1"]},
      {"command": "SET", "args": ["key2", "val2"]},
      {"command": "MGET", "args": ["key1", "key2"]}
    ]
  }'
# => {"results": ["OK", "OK", ["val1", "val2"]]}`}
      />

      <CodeBlock
        language="json"
        title="Request body schema"
        code={`{
  "command": "string (required)",
  "args": ["array of string arguments (optional)"],
  "db": 0
}`}
      />

      {/* ── Key Operations ───────────────────────────────────── */}
      <DocHeading id="key-ops" level={2}>
        <Database className="w-5 h-5 text-[var(--color-primary)]" />
        Key Operations
      </DocHeading>

      <p className="mb-3 text-[var(--color-text-secondary)]">
        RESTful CRUD operations for key-value pairs.
      </p>

      <CodeBlock
        language="bash"
        title="Key CRUD operations"
        code={`# GET a key
curl http://localhost:7280/api/v1/keys/mykey
# => {"key": "mykey", "value": "myvalue", "type": "string", "ttl": 3540}

# SET a key
curl -X PUT http://localhost:7280/api/v1/keys/mykey \\
  -H "Content-Type: application/json" \\
  -d '{"value": "newvalue", "ttl": 3600}'
# => {"status": "ok"}

# SET only if not exists
curl -X PUT http://localhost:7280/api/v1/keys/mykey \\
  -H "Content-Type: application/json" \\
  -d '{"value": "newvalue", "ttl": 3600, "nx": true}'
# => {"status": "ok", "created": true}

# DELETE a key
curl -X DELETE http://localhost:7280/api/v1/keys/mykey
# => {"status": "ok", "deleted": 1}

# Check if key exists (HEAD request)
curl -I http://localhost:7280/api/v1/keys/mykey
# => HTTP/1.1 200 OK (exists) or 404 (not found)`}
      />

      <CodeBlock
        language="json"
        title="Response: GET /api/v1/keys/:key"
        code={`{
  "key": "mykey",
  "value": "myvalue",
  "type": "string",
  "ttl": 3540,
  "encoding": "embstr",
  "size": 7
}`}
      />

      {/* ── Server Info ──────────────────────────────────────── */}
      <DocHeading id="info" level={2}>
        <BarChart3 className="w-5 h-5 text-[var(--color-primary)]" />
        Server Info
      </DocHeading>

      <CodeBlock
        language="bash"
        title="Server information endpoints"
        code={`# Full server info
curl http://localhost:7280/api/v1/info

# Specific section
curl http://localhost:7280/api/v1/info/memory
curl http://localhost:7280/api/v1/info/clients
curl http://localhost:7280/api/v1/info/stats
curl http://localhost:7280/api/v1/info/replication
curl http://localhost:7280/api/v1/info/keyspace`}
      />

      <CodeBlock
        language="json"
        title="Response: GET /api/v1/info/memory"
        code={`{
  "used_memory": 134217728,
  "used_memory_human": "128.00M",
  "used_memory_peak": 268435456,
  "used_memory_peak_human": "256.00M",
  "maxmemory": 1073741824,
  "maxmemory_human": "1.00G",
  "maxmemory_policy": "allkeys-lru",
  "mem_fragmentation_ratio": 1.05,
  "mem_allocator": "go-runtime"
}`}
      />

      {/* ── Metrics ──────────────────────────────────────────── */}
      <DocHeading id="metrics-api" level={2}>
        Metrics
      </DocHeading>

      <p className="mb-3 text-[var(--color-text-secondary)]">
        Prometheus-format metrics are available at the metrics endpoint.
      </p>

      <CodeBlock
        language="bash"
        title="Metrics endpoint"
        code={`# Prometheus text format
curl http://localhost:7280/api/v1/metrics
# => cachestorm_uptime_seconds 3600
# => cachestorm_connected_clients 42
# => ...

# JSON format
curl -H "Accept: application/json" http://localhost:7280/api/v1/metrics
# => { "uptime_seconds": 3600, "connected_clients": 42, ... }`}
      />

      {/* ── Configuration ────────────────────────────────────── */}
      <DocHeading id="config-api" level={2}>
        <Settings className="w-5 h-5 text-[var(--color-primary)]" />
        Configuration
      </DocHeading>

      <CodeBlock
        language="bash"
        title="Runtime configuration"
        code={`# Get current configuration
curl http://localhost:7280/api/v1/config
# => { "server": { "port": 6380, ... }, "memory": { ... } }

# Get specific setting
curl http://localhost:7280/api/v1/config/memory.maxmemory
# => { "key": "memory.maxmemory", "value": "1gb" }

# Update a setting at runtime
curl -X PUT http://localhost:7280/api/v1/config \\
  -H "Content-Type: application/json" \\
  -d '{"memory.maxmemory": "2gb", "memory.eviction_policy": "allkeys-lfu"}'
# => {"status": "ok", "updated": ["memory.maxmemory", "memory.eviction_policy"]}`}
      />

      <InfoBox type="info">
        Not all settings can be changed at runtime. Server port, bind address, and TLS settings
        require a restart.
      </InfoBox>

      {/* ── Error Handling ───────────────────────────────────── */}
      <DocHeading id="errors" level={2}>
        Error Handling
      </DocHeading>

      <p className="mb-3 text-[var(--color-text-secondary)]">
        The API uses standard HTTP status codes and returns errors in a consistent JSON format.
      </p>

      <div className="my-4 rounded-xl border border-[var(--color-border)] overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-[var(--color-border)] text-left text-[var(--color-text-secondary)]">
                <th className="px-4 py-2 font-medium">Status</th>
                <th className="px-4 py-2 font-medium">Meaning</th>
              </tr>
            </thead>
            <tbody className="text-[var(--color-text-secondary)]">
              {[
                ["200", "Success"],
                ["201", "Created (key didn't exist before)"],
                ["400", "Bad request (invalid JSON, missing fields)"],
                ["401", "Unauthorized (missing or invalid auth)"],
                ["403", "Forbidden (ACL permission denied)"],
                ["404", "Key not found"],
                ["429", "Rate limited"],
                ["500", "Internal server error"],
                ["503", "Service unavailable (server loading or shutting down)"],
              ].map(([code, meaning], i, arr) => (
                <tr key={code} className={i < arr.length - 1 ? "border-b border-[var(--color-border)]" : ""}>
                  <td className="px-4 py-2 font-mono text-[var(--color-primary)]">{code}</td>
                  <td className="px-4 py-2 text-[var(--color-text-secondary)]">{meaning}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <CodeBlock
        language="json"
        title="Error response format"
        code={`{
  "error": {
    "code": "KEY_NOT_FOUND",
    "message": "Key 'mykey' does not exist",
    "status": 404
  }
}`}
      />

      {/* ── Rate Limiting ────────────────────────────────────── */}
      <DocHeading id="rate-limiting" level={2}>
        Rate Limiting
      </DocHeading>

      <p className="mb-3 text-[var(--color-text-secondary)]">
        The HTTP API supports optional rate limiting per IP or per authenticated user.
      </p>

      <CodeBlock
        language="yaml"
        title="Rate limit configuration"
        code={`http:
  rate_limit:
    enabled: true
    requests_per_second: 1000
    burst: 100
    # Per-user limits (overrides global)
    user_limits:
      admin: 0        # unlimited
      app: 5000
      reader: 1000`}
      />

      <p className="mb-3 text-[var(--color-text-secondary)]">
        Rate limit headers are included in all responses:
      </p>

      <CodeBlock
        language="bash"
        title="Rate limit headers"
        code={`X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 995
X-RateLimit-Reset: 1705000060
Retry-After: 1  # Only included when rate limited (429)`}
      />

      {/* ── SDKs ─────────────────────────────────────────────── */}
      <DocHeading id="sdks" level={2}>
        Client SDKs
      </DocHeading>

      <p className="mb-4 text-[var(--color-text-secondary)]">
        Since CacheStorm is Redis-compatible, you can use any Redis client library. For the HTTP
        API specifically, use standard HTTP clients:
      </p>

      <CodeBlock
        language="go"
        title="Go"
        code={`package main

import (
    "context"
    "github.com/redis/go-redis/v9"
)

func main() {
    // RESP protocol (recommended for performance)
    rdb := redis.NewClient(&redis.Options{
        Addr:     "localhost:6380",
        Password: "your-password",
        DB:       0,
    })

    ctx := context.Background()
    rdb.Set(ctx, "key", "value", 0)
    val, _ := rdb.Get(ctx, "key").Result()
    println(val) // "value"
}`}
      />

      <CodeBlock
        language="python"
        title="Python"
        code={`import redis
import requests

# RESP protocol (recommended for performance)
r = redis.Redis(host='localhost', port=6380, password='your-password')
r.set('key', 'value')
print(r.get('key'))  # b'value'

# HTTP API
resp = requests.post('http://localhost:7280/api/v1/command',
    json={'command': 'SET', 'args': ['key', 'value']},
    headers={'Authorization': 'Bearer your-password'})
print(resp.json())  # {'result': 'OK'}`}
      />

      <CodeBlock
        language="javascript"
        title="Node.js / TypeScript"
        code={`import Redis from 'ioredis';

// RESP protocol (recommended for performance)
const redis = new Redis({
  host: 'localhost',
  port: 6380,
  password: 'your-password',
});

await redis.set('key', 'value');
const val = await redis.get('key');
console.log(val); // "value"

// HTTP API
const resp = await fetch('http://localhost:7280/api/v1/command', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': 'Bearer your-password',
  },
  body: JSON.stringify({ command: 'GET', args: ['key'] }),
});
const data = await resp.json();
console.log(data.result); // "value"`}
      />
    </DocsLayout>
  );
}
