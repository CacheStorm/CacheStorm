import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  Zap,
  Shield,
  Database,
  BarChart3,
  Network,
  Eye,
  ArrowRight,
  Terminal,
  Github,
  ChevronRight,
} from "lucide-react";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { Card, CardContent } from "../components/ui/card";

/* ────────────────────────────────────────────────────────────── */
/*  Terminal animation                                            */
/* ────────────────────────────────────────────────────────────── */

const terminalLines = [
  { prompt: "$ ", command: "redis-cli -p 6380", delay: 600 },
  { prompt: "127.0.0.1:6380> ", command: 'SET mykey "Hello CacheStorm"', delay: 800 },
  { prompt: "", command: "OK", delay: 400, isResponse: true },
  { prompt: "127.0.0.1:6380> ", command: "GET mykey", delay: 600 },
  { prompt: "", command: '"Hello CacheStorm"', delay: 400, isResponse: true },
  { prompt: "127.0.0.1:6380> ", command: "HSET user:1 name Alice age 30", delay: 700 },
  { prompt: "", command: "(integer) 2", delay: 400, isResponse: true },
  { prompt: "127.0.0.1:6380> ", command: "INFO server | head -3", delay: 600 },
  { prompt: "", command: "# Server", delay: 200, isResponse: true },
  { prompt: "", command: "cachestorm_version:1.0.0", delay: 200, isResponse: true },
  { prompt: "", command: "redis_version:7.0.0 (compatible)", delay: 200, isResponse: true },
];

function AnimatedTerminal() {
  const [visibleLines, setVisibleLines] = useState<
    Array<{ prompt: string; text: string; isResponse?: boolean }>
  >([]);
  const [currentLine, setCurrentLine] = useState(0);
  const [currentChar, setCurrentChar] = useState(0);
  const [isTyping, setIsTyping] = useState(true);

  useEffect(() => {
    if (currentLine >= terminalLines.length) {
      // Reset after a pause
      const timer = setTimeout(() => {
        setVisibleLines([]);
        setCurrentLine(0);
        setCurrentChar(0);
        setIsTyping(true);
      }, 3000);
      return () => clearTimeout(timer);
    }

    const line = terminalLines[currentLine];

    if (line.isResponse) {
      // Show response lines instantly after a short delay
      const timer = setTimeout(() => {
        setVisibleLines((prev) => [
          ...prev,
          { prompt: line.prompt, text: line.command, isResponse: true },
        ]);
        setCurrentLine((prev) => prev + 1);
        setCurrentChar(0);
      }, line.delay);
      return () => clearTimeout(timer);
    }

    if (currentChar === 0 && isTyping) {
      // Add new line entry
      setVisibleLines((prev) => [
        ...prev,
        { prompt: line.prompt, text: "" },
      ]);
    }

    if (currentChar < line.command.length) {
      const timer = setTimeout(
        () => {
          setVisibleLines((prev) => {
            const updated = [...prev];
            const last = updated[updated.length - 1];
            updated[updated.length - 1] = {
              ...last,
              text: line.command.slice(0, currentChar + 1),
            };
            return updated;
          });
          setCurrentChar((prev) => prev + 1);
        },
        30 + Math.random() * 40
      );
      return () => clearTimeout(timer);
    } else {
      // Finished typing this line
      const timer = setTimeout(() => {
        setCurrentLine((prev) => prev + 1);
        setCurrentChar(0);
        setIsTyping(true);
      }, line.delay);
      return () => clearTimeout(timer);
    }
  }, [currentLine, currentChar, isTyping]);

  const showCursor =
    currentLine < terminalLines.length &&
    !terminalLines[currentLine]?.isResponse;

  return (
    <div className="rounded-xl border border-slate-700/50 bg-slate-900 shadow-2xl overflow-hidden">
      {/* Title bar */}
      <div className="flex items-center gap-2 border-b border-slate-700/50 bg-slate-800/50 px-4 py-3">
        <div className="flex gap-1.5">
          <div className="h-3 w-3 rounded-full bg-red-500/80" />
          <div className="h-3 w-3 rounded-full bg-yellow-500/80" />
          <div className="h-3 w-3 rounded-full bg-green-500/80" />
        </div>
        <span className="ml-2 text-xs text-slate-400 font-mono">
          terminal
        </span>
      </div>
      {/* Terminal body */}
      <div className="p-4 font-mono text-sm leading-relaxed h-[340px] overflow-hidden">
        {visibleLines.map((line, i) => (
          <div key={i} className="flex">
            {line.prompt && (
              <span className="text-emerald-400 select-none shrink-0">
                {line.prompt}
              </span>
            )}
            <span
              className={
                line.isResponse ? "text-slate-400" : "text-slate-200"
              }
            >
              {line.text}
            </span>
            {i === visibleLines.length - 1 && showCursor && (
              <span className="terminal-cursor" />
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

/* ────────────────────────────────────────────────────────────── */
/*  Stats bar                                                     */
/* ────────────────────────────────────────────────────────────── */

const stats = [
  { value: "20M+", label: "ops/sec" },
  { value: "99%", label: "Redis Compatible" },
  { value: "Sub-ms", label: "Latency" },
  { value: "256", label: "Shard Architecture" },
];

/* ────────────────────────────────────────────────────────────── */
/*  Features                                                      */
/* ────────────────────────────────────────────────────────────── */

const features = [
  {
    icon: Zap,
    title: "Extreme Performance",
    description:
      "Lock-free 256-shard architecture delivers 20M+ ops/sec with sub-millisecond P99 latency. Built from the ground up for speed.",
    color: "text-yellow-400",
    bg: "bg-yellow-400/10",
  },
  {
    icon: Database,
    title: "Redis Compatible",
    description:
      "Drop-in replacement for Redis. Use your existing redis-cli, client libraries, and tooling without any code changes.",
    color: "text-blue-400",
    bg: "bg-blue-400/10",
  },
  {
    icon: Shield,
    title: "Enterprise Security",
    description:
      "ACL-based authentication, IP whitelisting, command restrictions, and TLS encryption to protect your data in production.",
    color: "text-emerald-400",
    bg: "bg-emerald-400/10",
  },
  {
    icon: BarChart3,
    title: "Persistence & Durability",
    description:
      "Configurable AOF and snapshot persistence ensures data survives restarts. Tune durability vs. performance to your needs.",
    color: "text-purple-400",
    bg: "bg-purple-400/10",
  },
  {
    icon: Network,
    title: "Clustering Ready",
    description:
      "Horizontal scaling with built-in cluster support, automatic sharding, and replica failover for high availability.",
    color: "text-cyan-400",
    bg: "bg-cyan-400/10",
  },
  {
    icon: Eye,
    title: "Observability",
    description:
      "Built-in HTTP metrics endpoint, Prometheus-compatible exports, and detailed logging for complete operational visibility.",
    color: "text-pink-400",
    bg: "bg-pink-400/10",
  },
];

/* ────────────────────────────────────────────────────────────── */
/*  Code comparison                                               */
/* ────────────────────────────────────────────────────────────── */

const redisCode = `# Connect with standard redis-cli
$ redis-cli -p 6380

# All your favorite commands work
127.0.0.1:6380> SET session:abc123 "user_data"
OK
127.0.0.1:6380> EXPIRE session:abc123 3600
(integer) 1
127.0.0.1:6380> TTL session:abc123
(integer) 3599

# Hash operations
127.0.0.1:6380> HSET product:42 name "Widget" price 29.99
(integer) 2
127.0.0.1:6380> HGETALL product:42
1) "name"
2) "Widget"
3) "price"
4) "29.99"`;

const cacheStormConfig = `# cachestorm.conf
# Simple, Redis-like configuration

bind 0.0.0.0
port 6380

# 256-shard lock-free architecture
shards 256

# Enterprise security
requirepass your_secure_password
acl-file /etc/cachestorm/users.acl

# Persistence
appendonly yes
appendfsync everysec

# Performance tuning
maxmemory 4gb
maxmemory-policy allkeys-lru

# Observability
http-enabled true
http-port 8080`;

/* ────────────────────────────────────────────────────────────── */
/*  Home page                                                     */
/* ────────────────────────────────────────────────────────────── */

export function Home() {
  return (
    <div className="relative">
      {/* ── Hero ───────────────────────────────────────────── */}
      <section className="relative overflow-hidden pt-32 pb-20 lg:pt-40 lg:pb-28">
        {/* Background effects */}
        <div className="absolute inset-0 bg-grid" />
        <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[800px] h-[600px] bg-blue-600/5 rounded-full blur-3xl pointer-events-none" />
        <div className="absolute top-20 right-0 w-[400px] h-[400px] bg-purple-600/5 rounded-full blur-3xl pointer-events-none" />

        <div className="relative mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="grid items-center gap-12 lg:grid-cols-2 lg:gap-16">
            {/* Left: text content */}
            <div className="max-w-2xl">
              <div className="animate-fade-in-up">
                <Badge variant="success" className="mb-6">
                  <span className="mr-1.5 inline-block h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse" />
                  v1.0 Now Available
                </Badge>
              </div>

              <h1 className="animate-fade-in-up animation-delay-100 text-4xl font-extrabold tracking-tight sm:text-5xl lg:text-6xl">
                <span className="gradient-text">High-Performance</span>
                <br />
                <span className="text-white">
                  Redis-Compatible Cache
                </span>
              </h1>

              <p className="animate-fade-in-up animation-delay-200 mt-6 text-lg text-slate-400 leading-relaxed max-w-xl">
                CacheStorm is a blazing-fast, Redis-compatible caching server
                written in Go. Drop-in replacement with 256-shard architecture,
                sub-millisecond latency, and enterprise-grade security.
              </p>

              <div className="animate-fade-in-up animation-delay-300 mt-8 flex flex-wrap gap-4">
                <Link to="/docs/getting-started">
                  <Button size="lg" className="gap-2">
                    Get Started
                    <ArrowRight className="h-4 w-4" />
                  </Button>
                </Link>
                <a
                  href="https://github.com/nicholasgasior/cachestorm"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <Button variant="outline" size="lg" className="gap-2">
                    <Github className="h-4 w-4" />
                    View on GitHub
                  </Button>
                </a>
              </div>

              <div className="animate-fade-in-up animation-delay-400 mt-8 flex items-center gap-2 text-sm text-slate-500">
                <Terminal className="h-4 w-4" />
                <code className="font-mono text-slate-400">
                  go install github.com/nicholasgasior/cachestorm@latest
                </code>
              </div>
            </div>

            {/* Right: animated terminal */}
            <div className="animate-fade-in-up animation-delay-200 lg:ml-auto w-full max-w-lg">
              <AnimatedTerminal />
            </div>
          </div>
        </div>
      </section>

      {/* ── Stats bar ──────────────────────────────────────── */}
      <section className="relative border-y border-slate-800/60 bg-slate-900/30">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="grid grid-cols-2 gap-8 py-12 md:grid-cols-4">
            {stats.map((stat) => (
              <div key={stat.label} className="text-center">
                <div className="text-3xl font-bold gradient-text sm:text-4xl">
                  {stat.value}
                </div>
                <div className="mt-1 text-sm text-slate-400">{stat.label}</div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Features grid ──────────────────────────────────── */}
      <section id="features" className="relative py-24 lg:py-32">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="mx-auto max-w-2xl text-center">
            <Badge className="mb-4">Features</Badge>
            <h2 className="text-3xl font-bold tracking-tight text-white sm:text-4xl">
              Everything you need for{" "}
              <span className="gradient-text">production caching</span>
            </h2>
            <p className="mt-4 text-lg text-slate-400">
              Built from scratch in Go for maximum performance, reliability, and
              operational simplicity.
            </p>
          </div>

          <div className="mt-16 grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
            {features.map((feature) => (
              <Card
                key={feature.title}
                className="group hover:border-slate-700 hover:bg-slate-800/50 p-0"
              >
                <CardContent className="p-6 pt-6">
                  <div
                    className={`mb-4 flex h-10 w-10 items-center justify-center rounded-lg ${feature.bg}`}
                  >
                    <feature.icon className={`h-5 w-5 ${feature.color}`} />
                  </div>
                  <h3 className="text-base font-semibold text-white">
                    {feature.title}
                  </h3>
                  <p className="mt-2 text-sm leading-relaxed text-slate-400">
                    {feature.description}
                  </p>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      </section>

      {/* ── Code comparison ────────────────────────────────── */}
      <section className="relative border-y border-slate-800/60 bg-slate-900/20 py-24 lg:py-32">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="mx-auto max-w-2xl text-center">
            <Badge className="mb-4">Compatibility</Badge>
            <h2 className="text-3xl font-bold tracking-tight text-white sm:text-4xl">
              Your Redis workflow,{" "}
              <span className="gradient-text">supercharged</span>
            </h2>
            <p className="mt-4 text-lg text-slate-400">
              Use your existing tools and client libraries. CacheStorm speaks
              the Redis protocol natively.
            </p>
          </div>

          <div className="mt-16 grid gap-6 lg:grid-cols-2">
            {/* Redis CLI panel */}
            <div className="rounded-xl border border-slate-700/50 bg-slate-900 overflow-hidden">
              <div className="flex items-center gap-2 border-b border-slate-700/50 bg-slate-800/50 px-4 py-3">
                <div className="flex gap-1.5">
                  <div className="h-3 w-3 rounded-full bg-red-500/80" />
                  <div className="h-3 w-3 rounded-full bg-yellow-500/80" />
                  <div className="h-3 w-3 rounded-full bg-green-500/80" />
                </div>
                <span className="ml-2 text-xs text-slate-400 font-mono">
                  redis-cli
                </span>
              </div>
              <pre className="p-4 text-sm font-mono text-slate-300 leading-relaxed overflow-x-auto">
                <code>{redisCode}</code>
              </pre>
            </div>

            {/* Config panel */}
            <div className="rounded-xl border border-slate-700/50 bg-slate-900 overflow-hidden">
              <div className="flex items-center gap-2 border-b border-slate-700/50 bg-slate-800/50 px-4 py-3">
                <div className="flex gap-1.5">
                  <div className="h-3 w-3 rounded-full bg-red-500/80" />
                  <div className="h-3 w-3 rounded-full bg-yellow-500/80" />
                  <div className="h-3 w-3 rounded-full bg-green-500/80" />
                </div>
                <span className="ml-2 text-xs text-slate-400 font-mono">
                  cachestorm.conf
                </span>
              </div>
              <pre className="p-4 text-sm font-mono text-slate-300 leading-relaxed overflow-x-auto">
                <code>{cacheStormConfig}</code>
              </pre>
            </div>
          </div>
        </div>
      </section>

      {/* ── CTA section ────────────────────────────────────── */}
      <section className="relative py-24 lg:py-32">
        <div className="absolute inset-0 bg-grid" />
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[400px] bg-blue-600/5 rounded-full blur-3xl pointer-events-none" />

        <div className="relative mx-auto max-w-3xl px-4 text-center sm:px-6 lg:px-8">
          <h2 className="text-3xl font-bold tracking-tight text-white sm:text-4xl lg:text-5xl">
            Ready for{" "}
            <span className="gradient-text">Production</span>?
          </h2>
          <p className="mx-auto mt-6 max-w-xl text-lg text-slate-400 leading-relaxed">
            Deploy CacheStorm in minutes. Compatible with your existing Redis
            setup, no migration required.
          </p>

          <div className="mt-10 flex flex-wrap items-center justify-center gap-4">
            <Link to="/docs/getting-started">
              <Button size="lg" className="gap-2">
                Download CacheStorm
                <ArrowRight className="h-4 w-4" />
              </Button>
            </Link>
            <Link to="/docs">
              <Button variant="outline" size="lg" className="gap-2">
                Read the Docs
                <ChevronRight className="h-4 w-4" />
              </Button>
            </Link>
          </div>

          <div className="mt-8">
            <p className="text-sm text-slate-500">
              Open source under MIT license. Free forever.
            </p>
          </div>
        </div>
      </section>
    </div>
  );
}
