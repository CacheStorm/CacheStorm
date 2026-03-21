import DocsLayout, {
  DocHeading,
  CodeBlock,
  InfoBox,
  type TocItem,
} from "@/components/DocsLayout";
import { Download, Rocket, Settings, Cpu, Container, Code2 } from "lucide-react";

const toc: TocItem[] = [
  { id: "installation", text: "Installation", level: 2 },
  { id: "docker", text: "Docker", level: 3 },
  { id: "binary", text: "Pre-built Binaries", level: 3 },
  { id: "source", text: "Build from Source", level: 3 },
  { id: "quick-start", text: "Quick Start", level: 2 },
  { id: "connect", text: "Connecting", level: 3 },
  { id: "basic-ops", text: "Basic Operations", level: 3 },
  { id: "configuration", text: "Configuration", level: 2 },
  { id: "config-file", text: "Configuration File", level: 3 },
  { id: "env-vars", text: "Environment Variables", level: 3 },
  { id: "next-steps", text: "Next Steps", level: 2 },
];

export default function GettingStarted() {
  return (
    <DocsLayout toc={toc}>
      {/* Hero */}
      <div className="mb-10">
        <div className="flex items-center gap-2 text-[var(--color-primary)] text-sm font-medium mb-2">
          <Rocket className="w-4 h-4" />
          Getting Started
        </div>
        <h1 className="text-4xl font-extrabold text-[var(--color-text)] tracking-tight mb-4">
          Install &amp; Run CacheStorm
        </h1>
        <p className="text-lg text-[var(--color-text-secondary)] leading-relaxed max-w-2xl">
          CacheStorm is a high-performance, Redis-compatible caching server written in Go.
          Get up and running in under a minute with Docker, pre-built binaries, or from source.
        </p>
      </div>

      {/* ── Installation ─────────────────────────────────────── */}
      <DocHeading id="installation" level={2}>
        <Download className="w-5 h-5 text-[var(--color-primary)]" />
        Installation
      </DocHeading>

      <p className="mb-6 text-[var(--color-text-secondary)]">
        Choose the installation method that works best for your environment.
      </p>

      {/* Docker */}
      <DocHeading id="docker" level={3}>
        <Container className="w-4 h-4 text-cyan-400" />
        Docker
      </DocHeading>

      <p className="mb-3 text-[var(--color-text-secondary)]">
        The fastest way to try CacheStorm. The image is available on Docker Hub and GitHub Container Registry.
      </p>

      <CodeBlock
        language="bash"
        title="Pull & run"
        code={`# Pull the latest image
docker pull ghcr.io/nicktretyakov/cachestorm:latest

# Run CacheStorm on port 6380
docker run -d \\
  --name cachestorm \\
  -p 6380:6380 \\
  ghcr.io/nicktretyakov/cachestorm:latest

# With a custom config file
docker run -d \\
  --name cachestorm \\
  -p 6380:6380 \\
  -v $(pwd)/cachestorm.yaml:/etc/cachestorm/cachestorm.yaml \\
  ghcr.io/nicktretyakov/cachestorm:latest \\
  --config /etc/cachestorm/cachestorm.yaml`}
      />

      <InfoBox type="tip">
        Use <code className="text-xs bg-[var(--color-surface)] px-1 py-0.5 rounded">docker compose</code> for
        production deployments with persistent volumes and health checks.
      </InfoBox>

      <CodeBlock
        language="yaml"
        title="docker-compose.yml"
        code={`services:
  cachestorm:
    image: ghcr.io/nicktretyakov/cachestorm:latest
    ports:
      - "6380:6380"
      - "7280:7280"   # HTTP API
    volumes:
      - cachestorm-data:/data
      - ./cachestorm.yaml:/etc/cachestorm/cachestorm.yaml
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "cachestorm-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 3

volumes:
  cachestorm-data:`}
      />

      {/* Binary */}
      <DocHeading id="binary" level={3}>
        <Cpu className="w-4 h-4 text-cyan-400" />
        Pre-built Binaries
      </DocHeading>

      <p className="mb-3 text-[var(--color-text-secondary)]">
        Download the latest release for your platform from GitHub Releases.
      </p>

      <CodeBlock
        language="bash"
        title="Linux / macOS"
        code={`# Download (replace with your OS/arch)
curl -fsSL https://github.com/nicktretyakov/CacheStorm/releases/latest/download/cachestorm-linux-amd64.tar.gz \\
  | tar xz

# Move to PATH
sudo mv cachestorm /usr/local/bin/

# Verify installation
cachestorm --version`}
      />

      <CodeBlock
        language="bash"
        title="Windows (PowerShell)"
        code={`# Download the zip
Invoke-WebRequest -Uri "https://github.com/nicktretyakov/CacheStorm/releases/latest/download/cachestorm-windows-amd64.zip" -OutFile cachestorm.zip

# Extract
Expand-Archive cachestorm.zip -DestinationPath .

# Run
.\\cachestorm.exe --version`}
      />

      <div className="my-4 rounded-xl border border-[var(--color-border)] overflow-hidden">
        <div className="px-4 py-2 text-xs font-medium text-[var(--color-text-secondary)] bg-[var(--color-surface)] border-b border-[var(--color-border)]">
          Supported platforms
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-[var(--color-border)] text-left text-[var(--color-text-secondary)]">
                <th className="px-4 py-2 font-medium">OS</th>
                <th className="px-4 py-2 font-medium">Architecture</th>
                <th className="px-4 py-2 font-medium">File</th>
              </tr>
            </thead>
            <tbody className="text-[var(--color-text-secondary)]">
              <tr className="border-b border-[var(--color-border)]">
                <td className="px-4 py-2">Linux</td>
                <td className="px-4 py-2">amd64, arm64</td>
                <td className="px-4 py-2 font-mono text-xs">cachestorm-linux-*.tar.gz</td>
              </tr>
              <tr className="border-b border-[var(--color-border)]">
                <td className="px-4 py-2">macOS</td>
                <td className="px-4 py-2">amd64, arm64 (Apple Silicon)</td>
                <td className="px-4 py-2 font-mono text-xs">cachestorm-darwin-*.tar.gz</td>
              </tr>
              <tr>
                <td className="px-4 py-2">Windows</td>
                <td className="px-4 py-2">amd64</td>
                <td className="px-4 py-2 font-mono text-xs">cachestorm-windows-*.zip</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      {/* Source */}
      <DocHeading id="source" level={3}>
        <Code2 className="w-4 h-4 text-cyan-400" />
        Build from Source
      </DocHeading>

      <p className="mb-3 text-[var(--color-text-secondary)]">
        Requires Go 1.22+ and Git.
      </p>

      <CodeBlock
        language="bash"
        title="Build from source"
        code={`# Clone the repository
git clone https://github.com/nicktretyakov/CacheStorm.git
cd CacheStorm

# Build
go build -o cachestorm ./cmd/cachestorm

# Run tests
go test ./...

# Install globally
go install ./cmd/cachestorm`}
      />

      {/* ── Quick Start ──────────────────────────────────────── */}
      <DocHeading id="quick-start" level={2}>
        <Rocket className="w-5 h-5 text-[var(--color-primary)]" />
        Quick Start
      </DocHeading>

      <p className="mb-4 text-[var(--color-text-secondary)]">
        Start the server and begin storing data in seconds.
      </p>

      <CodeBlock
        language="bash"
        title="Start the server"
        code={`# Start with defaults (port 6380)
cachestorm

# Or with a config file
cachestorm --config cachestorm.yaml

# Or specify options via flags
cachestorm --port 6380 --maxmemory 256mb`}
      />

      <DocHeading id="connect" level={3}>
        Connecting
      </DocHeading>

      <p className="mb-3 text-[var(--color-text-secondary)]">
        CacheStorm speaks the Redis RESP protocol, so you can use any Redis client.
      </p>

      <CodeBlock
        language="bash"
        title="Using redis-cli"
        code={`# Connect with redis-cli
redis-cli -p 6380

# Ping the server
127.0.0.1:6380> PING
PONG

# Set and get a value
127.0.0.1:6380> SET greeting "Hello, CacheStorm!"
OK
127.0.0.1:6380> GET greeting
"Hello, CacheStorm!"`}
      />

      <CodeBlock
        language="bash"
        title="Using the HTTP API"
        code={`# Health check
curl http://localhost:7280/health

# Execute commands via HTTP
curl -X POST http://localhost:7280/api/v1/command \\
  -H "Content-Type: application/json" \\
  -d '{"command": "SET", "args": ["mykey", "myvalue"]}'`}
      />

      <DocHeading id="basic-ops" level={3}>
        Basic Operations
      </DocHeading>

      <CodeBlock
        language="bash"
        title="Common commands"
        code={`# Strings
SET user:1:name "Alice"
GET user:1:name
INCR page:views
MSET key1 "val1" key2 "val2"

# Hashes
HSET user:1 name "Alice" email "alice@example.com" age "30"
HGET user:1 name
HGETALL user:1

# Lists
LPUSH queue:tasks "task1" "task2" "task3"
RPOP queue:tasks
LRANGE queue:tasks 0 -1

# Sets
SADD tags:article:1 "go" "caching" "performance"
SMEMBERS tags:article:1
SINTER tags:article:1 tags:article:2

# Keys with TTL
SET session:abc123 "user_data" EX 3600
TTL session:abc123`}
      />

      {/* ── Configuration ────────────────────────────────────── */}
      <DocHeading id="configuration" level={2}>
        <Settings className="w-5 h-5 text-[var(--color-primary)]" />
        Configuration
      </DocHeading>

      <DocHeading id="config-file" level={3}>
        Configuration File
      </DocHeading>

      <p className="mb-3 text-[var(--color-text-secondary)]">
        CacheStorm uses a YAML configuration file. Create a{" "}
        <code className="text-xs bg-[var(--color-surface)] px-1 py-0.5 rounded">cachestorm.yaml</code> in
        your working directory or specify the path with <code className="text-xs bg-[var(--color-surface)] px-1 py-0.5 rounded">--config</code>.
      </p>

      <CodeBlock
        language="yaml"
        title="cachestorm.yaml (minimal)"
        code={`# Server settings
server:
  port: 6380
  bind: "0.0.0.0"

# Memory limits
memory:
  maxmemory: "512mb"
  eviction_policy: "allkeys-lru"

# Persistence
persistence:
  enabled: true
  directory: "/data"

# HTTP API
http:
  enabled: true
  port: 7280

# Logging
logging:
  level: "info"
  format: "json"`}
      />

      <DocHeading id="env-vars" level={3}>
        Environment Variables
      </DocHeading>

      <p className="mb-3 text-[var(--color-text-secondary)]">
        All config options can be set via environment variables with the{" "}
        <code className="text-xs bg-[var(--color-surface)] px-1 py-0.5 rounded">CACHESTORM_</code> prefix.
      </p>

      <CodeBlock
        language="bash"
        title="Environment variables"
        code={`export CACHESTORM_SERVER_PORT=6380
export CACHESTORM_MEMORY_MAXMEMORY="1gb"
export CACHESTORM_PERSISTENCE_ENABLED=true
export CACHESTORM_HTTP_PORT=7280
export CACHESTORM_LOGGING_LEVEL=debug

cachestorm`}
      />

      <InfoBox type="info">
        Environment variables take precedence over the configuration file, which takes
        precedence over default values.
      </InfoBox>

      {/* ── Next Steps ───────────────────────────────────────── */}
      <DocHeading id="next-steps" level={2}>
        Next Steps
      </DocHeading>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-4">
        {[
          { href: "/docs/configuration", label: "Configuration Reference", desc: "Full YAML config docs" },
          { href: "/docs/commands", label: "Command Reference", desc: "All supported commands" },
          { href: "/docs/security", label: "Security Guide", desc: "TLS, ACL, and auth setup" },
          { href: "/docs/clustering", label: "Clustering Guide", desc: "High-availability setup" },
        ].map((item) => (
          <a
            key={item.href}
            href={item.href}
            className="flex flex-col gap-1 p-4 rounded-xl border border-[var(--color-border)] hover:border-blue-500/40 hover:bg-blue-500/5 transition-all duration-200 group"
          >
            <span className="text-sm font-semibold text-[var(--color-text)] group-hover:text-[var(--color-primary)] transition-colors">
              {item.label} &rarr;
            </span>
            <span className="text-xs text-[var(--color-text-tertiary)]">{item.desc}</span>
          </a>
        ))}
      </div>
    </DocsLayout>
  );
}
