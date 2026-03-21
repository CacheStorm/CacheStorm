import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  Zap, Shield, Database, BarChart3, Network, Eye,
  ArrowRight, Terminal, Github,
} from "lucide-react";
import { Button } from "../components/ui/button";
import { Card, CardContent } from "../components/ui/card";

const terminalLines = [
  { prompt: "$ ", command: "redis-cli -p 6380", delay: 600 },
  { prompt: "127.0.0.1:6380> ", command: 'SET mykey "Hello CacheStorm"', delay: 800 },
  { prompt: "", command: "OK", delay: 400, isResponse: true },
  { prompt: "127.0.0.1:6380> ", command: "GET mykey", delay: 600 },
  { prompt: "", command: '"Hello CacheStorm"', delay: 400, isResponse: true },
  { prompt: "127.0.0.1:6380> ", command: "HSET user:1 name Alice age 30", delay: 700 },
  { prompt: "", command: "(integer) 2", delay: 400, isResponse: true },
];

function AnimatedTerminal() {
  const [visibleLines, setVisibleLines] = useState<Array<{ prompt: string; text: string; isResponse?: boolean }>>([]);
  const [currentLine, setCurrentLine] = useState(0);
  const [currentChar, setCurrentChar] = useState(0);
  const [isTyping, setIsTyping] = useState(true);

  useEffect(() => {
    if (currentLine >= terminalLines.length) {
      const timer = setTimeout(() => {
        setVisibleLines([]);
        setCurrentLine(0);
        setCurrentChar(0);
        setIsTyping(true);
      }, 4000);
      return () => clearTimeout(timer);
    }

    const line = terminalLines[currentLine];

    if (line.isResponse) {
      const timer = setTimeout(() => {
        setVisibleLines((prev) => [...prev, { prompt: line.prompt, text: line.command, isResponse: true }]);
        setCurrentLine((prev) => prev + 1);
        setCurrentChar(0);
      }, line.delay);
      return () => clearTimeout(timer);
    }

    if (currentChar === 0 && isTyping) {
      setVisibleLines((prev) => [...prev, { prompt: line.prompt, text: "" }]);
    }

    if (currentChar < line.command.length) {
      const timer = setTimeout(() => {
        setVisibleLines((prev) => {
          const updated = [...prev];
          updated[updated.length - 1] = { ...updated[updated.length - 1], text: line.command.slice(0, currentChar + 1) };
          return updated;
        });
        setCurrentChar((prev) => prev + 1);
      }, 30 + Math.random() * 40);
      return () => clearTimeout(timer);
    } else {
      const timer = setTimeout(() => {
        setCurrentLine((prev) => prev + 1);
        setCurrentChar(0);
        setIsTyping(true);
      }, line.delay);
      return () => clearTimeout(timer);
    }
  }, [currentLine, currentChar, isTyping]);

  const showCursor = currentLine < terminalLines.length && !terminalLines[currentLine]?.isResponse;

  return (
    <div className="rounded-xl border overflow-hidden" style={{ borderColor: "var(--color-border)", backgroundColor: "var(--color-terminal-bg)" }}>
      <div className="flex items-center gap-2 border-b px-4 py-3" style={{ borderColor: "var(--color-border)", backgroundColor: "var(--color-surface)" }}>
        <div className="flex gap-1.5">
          <div className="h-3 w-3 rounded-full bg-red-400" />
          <div className="h-3 w-3 rounded-full bg-yellow-400" />
          <div className="h-3 w-3 rounded-full bg-green-400" />
        </div>
        <span className="ml-2 text-xs font-mono" style={{ color: "var(--color-text-tertiary)" }}>terminal</span>
      </div>
      <div className="p-4 font-mono text-sm leading-relaxed h-[280px] overflow-hidden" style={{ backgroundColor: "var(--color-terminal-bg)" }}>
        {visibleLines.map((line, i) => (
          <div key={i} className="flex">
            {line.prompt && <span className="text-green-400 select-none shrink-0">{line.prompt}</span>}
            <span className={line.isResponse ? "text-gray-400" : "text-gray-200"}>{line.text}</span>
            {i === visibleLines.length - 1 && showCursor && <span className="terminal-cursor" />}
          </div>
        ))}
      </div>
    </div>
  );
}

const stats = [
  { value: "20M+", label: "Operations/sec" },
  { value: "99%", label: "Redis Compatible" },
  { value: "<1ms", label: "P99 Latency" },
  { value: "256", label: "Shards" },
];

const features = [
  { icon: Zap, title: "Extreme Performance", description: "Lock-free 256-shard architecture delivers 20M+ ops/sec with sub-millisecond latency." },
  { icon: Database, title: "Redis Compatible", description: "Drop-in replacement. Use redis-cli, existing client libraries, and tooling without changes." },
  { icon: Shield, title: "Enterprise Security", description: "TLS 1.2+, ACL system, password auth, command restrictions, and rate limiting." },
  { icon: BarChart3, title: "Persistence", description: "AOF and RDB persistence ensures data survives restarts. Configurable sync policies." },
  { icon: Network, title: "Clustering", description: "Built-in cluster support with automatic sharding, replication, and failover." },
  { icon: Eye, title: "Observability", description: "Prometheus metrics, Grafana dashboards, pprof profiling, and structured logging." },
];

const codeExample = `$ redis-cli -p 6380

127.0.0.1:6380> SET session:abc "user_data"
OK
127.0.0.1:6380> EXPIRE session:abc 3600
(integer) 1
127.0.0.1:6380> HSET product:1 name "Widget" price 29.99
(integer) 2
127.0.0.1:6380> HGETALL product:1
1) "name"
2) "Widget"
3) "price"
4) "29.99"`;

const configExample = `server:
  port: 6380
  requirepass: "your_password"
  tls_cert_file: "/path/to/cert.pem"
  tls_key_file: "/path/to/key.pem"

memory:
  max_memory: "4gb"
  eviction_policy: "allkeys-lru"

persistence:
  enabled: true
  aof: true
  aof_sync: "everysec"`;

export function Home() {
  return (
    <div>
      {/* Hero */}
      <section className="pt-32 pb-20 lg:pt-40 lg:pb-28">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="grid items-center gap-12 lg:grid-cols-2 lg:gap-16">
            <div className="max-w-xl">
              <div className="animate-fade-in-up">
                <span className="inline-flex items-center gap-1.5 px-3 py-1 text-xs font-medium rounded-full border" style={{ borderColor: "var(--color-border)", color: "var(--color-text-secondary)" }}>
                  <span className="h-1.5 w-1.5 rounded-full bg-green-500" />
                  Open Source
                </span>
              </div>

              <h1 className="animate-fade-in-up animation-delay-100 mt-6 text-4xl font-bold tracking-tight sm:text-5xl" style={{ color: "var(--color-text)" }}>
                High-Performance Redis-Compatible Cache
              </h1>

              <p className="animate-fade-in-up animation-delay-200 mt-5 text-lg leading-relaxed" style={{ color: "var(--color-text-secondary)" }}>
                A blazing-fast caching server written in Go. Drop-in Redis replacement with 256-shard architecture, sub-millisecond latency, and enterprise security.
              </p>

              <div className="animate-fade-in-up animation-delay-300 mt-8 flex flex-wrap gap-3">
                <Link to="/docs/getting-started">
                  <Button size="lg" className="gap-2">
                    Get Started <ArrowRight className="h-4 w-4" />
                  </Button>
                </Link>
                <a href="https://github.com/CacheStorm/CacheStorm" target="_blank" rel="noopener noreferrer">
                  <Button variant="outline" size="lg" className="gap-2">
                    <Github className="h-4 w-4" /> GitHub
                  </Button>
                </a>
              </div>

              <div className="mt-6 flex items-center gap-2 text-sm" style={{ color: "var(--color-text-tertiary)" }}>
                <Terminal className="h-4 w-4" />
                <code className="font-mono" style={{ color: "var(--color-text-secondary)" }}>docker pull cachestorm/cachestorm</code>
              </div>
            </div>

            <div className="animate-fade-in-up animation-delay-200 lg:ml-auto w-full max-w-lg">
              <AnimatedTerminal />
            </div>
          </div>
        </div>
      </section>

      {/* Stats */}
      <section className="border-y" style={{ borderColor: "var(--color-border)", backgroundColor: "var(--color-bg-secondary)" }}>
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="grid grid-cols-2 gap-8 py-10 md:grid-cols-4">
            {stats.map((stat) => (
              <div key={stat.label} className="text-center">
                <div className="text-2xl font-bold sm:text-3xl" style={{ color: "var(--color-text)" }}>{stat.value}</div>
                <div className="mt-1 text-sm" style={{ color: "var(--color-text-secondary)" }}>{stat.label}</div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Features */}
      <section className="py-24 lg:py-28">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="mx-auto max-w-2xl text-center">
            <h2 className="text-3xl font-bold tracking-tight sm:text-4xl" style={{ color: "var(--color-text)" }}>
              Built for production
            </h2>
            <p className="mt-4 text-lg" style={{ color: "var(--color-text-secondary)" }}>
              Everything you need to run a high-performance cache in production.
            </p>
          </div>

          <div className="mt-14 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            {features.map((f) => (
              <Card key={f.title} className="p-0">
                <CardContent className="p-6">
                  <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-lg" style={{ backgroundColor: "var(--color-surface)" }}>
                    <f.icon className="h-5 w-5" style={{ color: "var(--color-primary)" }} />
                  </div>
                  <h3 className="text-base font-semibold" style={{ color: "var(--color-text)" }}>{f.title}</h3>
                  <p className="mt-2 text-sm leading-relaxed" style={{ color: "var(--color-text-secondary)" }}>{f.description}</p>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      </section>

      {/* Code */}
      <section className="border-y py-24 lg:py-28" style={{ borderColor: "var(--color-border)", backgroundColor: "var(--color-bg-secondary)" }}>
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="mx-auto max-w-2xl text-center">
            <h2 className="text-3xl font-bold tracking-tight sm:text-4xl" style={{ color: "var(--color-text)" }}>
              Works with your existing tools
            </h2>
            <p className="mt-4 text-lg" style={{ color: "var(--color-text-secondary)" }}>
              Use redis-cli, any Redis client library, or the HTTP API.
            </p>
          </div>

          <div className="mt-14 grid gap-5 lg:grid-cols-2">
            <div className="rounded-xl border overflow-hidden" style={{ borderColor: "var(--color-border)" }}>
              <div className="flex items-center border-b px-4 py-2.5" style={{ borderColor: "var(--color-border)", backgroundColor: "var(--color-surface)" }}>
                <span className="text-xs font-mono" style={{ color: "var(--color-text-tertiary)" }}>redis-cli</span>
              </div>
              <pre className="p-4 text-sm font-mono leading-relaxed overflow-x-auto" style={{ backgroundColor: "var(--color-code-bg)", color: "var(--color-code-text)" }}>
                <code>{codeExample}</code>
              </pre>
            </div>
            <div className="rounded-xl border overflow-hidden" style={{ borderColor: "var(--color-border)" }}>
              <div className="flex items-center border-b px-4 py-2.5" style={{ borderColor: "var(--color-border)", backgroundColor: "var(--color-surface)" }}>
                <span className="text-xs font-mono" style={{ color: "var(--color-text-tertiary)" }}>cachestorm.yaml</span>
              </div>
              <pre className="p-4 text-sm font-mono leading-relaxed overflow-x-auto" style={{ backgroundColor: "var(--color-code-bg)", color: "var(--color-code-text)" }}>
                <code>{configExample}</code>
              </pre>
            </div>
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="py-24 lg:py-28">
        <div className="mx-auto max-w-2xl px-4 text-center sm:px-6 lg:px-8">
          <h2 className="text-3xl font-bold tracking-tight sm:text-4xl" style={{ color: "var(--color-text)" }}>
            Ready to get started?
          </h2>
          <p className="mx-auto mt-4 max-w-lg text-lg" style={{ color: "var(--color-text-secondary)" }}>
            Deploy in minutes. Compatible with your existing Redis setup.
          </p>
          <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
            <Link to="/docs/getting-started">
              <Button size="lg" className="gap-2">Get Started <ArrowRight className="h-4 w-4" /></Button>
            </Link>
            <Link to="/docs">
              <Button variant="outline" size="lg">Documentation</Button>
            </Link>
          </div>
          <p className="mt-6 text-sm" style={{ color: "var(--color-text-tertiary)" }}>
            Open source &middot; MIT License &middot; Free forever
          </p>
        </div>
      </section>
    </div>
  );
}
