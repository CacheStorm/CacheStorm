import { Suspense } from "react";
import { Link, useParams, Navigate } from "react-router-dom";
import { Button } from "../components/ui/button";
import { Card, CardContent } from "../components/ui/card";
import { Badge } from "../components/ui/badge";
import {
  Settings,
  Terminal,
  Shield,
  BarChart3,
  Network,
  Globe,
  ArrowRight,
  Loader2,
} from "lucide-react";
import { getDocBySlug, defaultDocSlug } from "./docs/index";

/* ------------------------------------------------------------------ */
/*  Cards for the docs index page                                     */
/* ------------------------------------------------------------------ */

const docSections = [
  {
    slug: "getting-started",
    title: "Getting Started",
    description:
      "Install CacheStorm and run your first commands in under 5 minutes.",
    icon: Terminal,
    color: "text-green-600 dark:text-green-400",
    bg: "bg-emerald-400/10",
  },
  {
    slug: "configuration",
    title: "Configuration",
    description:
      "Full configuration reference with YAML examples for every setting.",
    icon: Settings,
    color: "text-[var(--color-primary)]",
    bg: "bg-blue-400/10",
  },
  {
    slug: "commands",
    title: "Commands",
    description:
      "Complete command reference: Strings, Hashes, Lists, Sets, Sorted Sets, Streams, and more.",
    icon: Terminal,
    color: "text-amber-400",
    bg: "bg-amber-400/10",
  },
  {
    slug: "api",
    title: "HTTP API",
    description:
      "RESTful HTTP API for management, monitoring, and executing commands.",
    icon: Globe,
    color: "text-purple-400",
    bg: "bg-purple-400/10",
  },
  {
    slug: "security",
    title: "Security",
    description:
      "TLS encryption, ACL system, and authentication setup for production.",
    icon: Shield,
    color: "text-cyan-400",
    bg: "bg-cyan-400/10",
  },
  {
    slug: "monitoring",
    title: "Monitoring",
    description:
      "Prometheus metrics, Grafana dashboards, and pprof profiling setup.",
    icon: BarChart3,
    color: "text-pink-400",
    bg: "bg-pink-400/10",
  },
  {
    slug: "clustering",
    title: "Clustering",
    description:
      "Replication, Sentinel failover, and horizontal scaling with cluster mode.",
    icon: Network,
    color: "text-orange-400",
    bg: "bg-orange-400/10",
  },
];

/* ------------------------------------------------------------------ */
/*  Loading fallback                                                  */
/* ------------------------------------------------------------------ */

function DocLoader() {
  return (
    <div className="flex items-center justify-center min-h-[60vh]">
      <div className="flex flex-col items-center gap-3 text-[var(--color-text-tertiary)]">
        <Loader2 className="w-6 h-6 animate-spin" />
        <span className="text-sm">Loading documentation...</span>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Docs index page (/docs)                                           */
/* ------------------------------------------------------------------ */

export function DocsIndex() {
  return (
    <div className="relative pt-32 pb-24">
      <div className="mx-auto max-w-4xl px-4 sm:px-6 lg:px-8">
        <Badge className="mb-4">Documentation</Badge>
        <h1 className="text-3xl font-bold tracking-tight text-[var(--color-text)] sm:text-4xl">
          CacheStorm <span className="gradient-text">Documentation</span>
        </h1>
        <p className="mt-4 text-lg text-[var(--color-text-secondary)] max-w-2xl">
          Everything you need to install, configure, and operate CacheStorm in
          production.
        </p>

        <div className="mt-12 grid gap-4 sm:grid-cols-2">
          {docSections.map((section) => (
            <Link key={section.slug} to={`/docs/${section.slug}`}>
              <Card className="group h-full hover:border-[var(--color-border)] hover:bg-[var(--color-surface)] p-0 transition-all duration-200">
                <CardContent className="p-6 pt-6 flex flex-col h-full">
                  <div
                    className={`mb-4 flex h-10 w-10 items-center justify-center rounded-lg ${section.bg}`}
                  >
                    <section.icon
                      className={`h-5 w-5 ${section.color}`}
                    />
                  </div>
                  <h2 className="text-base font-semibold text-[var(--color-text)] group-hover:text-[var(--color-primary)] transition-colors">
                    {section.title}
                  </h2>
                  <p className="mt-2 text-sm text-[var(--color-text-secondary)] flex-1">
                    {section.description}
                  </p>
                  <div className="mt-4 flex items-center text-sm text-[var(--color-primary)] opacity-0 group-hover:opacity-100 transition-opacity">
                    Read more <ArrowRight className="ml-1 h-3.5 w-3.5" />
                  </div>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Individual doc page (/docs/:slug)                                 */
/* ------------------------------------------------------------------ */

export function DocPage() {
  const { slug } = useParams<{ slug: string }>();

  // If no slug or invalid slug, redirect to default doc
  if (!slug) {
    return <Navigate to={`/docs/${defaultDocSlug}`} replace />;
  }

  const route = getDocBySlug(slug);

  // If we have a matching doc route, render the lazy component
  if (route) {
    const Component = route.component;
    return (
      <Suspense fallback={<DocLoader />}>
        <Component />
      </Suspense>
    );
  }

  // Fallback for unknown slugs
  return (
    <div className="relative pt-32 pb-24">
      <div className="mx-auto max-w-4xl px-4 sm:px-6 lg:px-8">
        <Link
          to="/docs"
          className="text-sm text-[var(--color-text-secondary)] hover:text-[var(--color-text)] transition-colors mb-6 inline-block"
        >
          &larr; Back to Documentation
        </Link>

        <h1 className="text-3xl font-bold tracking-tight text-[var(--color-text)] sm:text-4xl">
          Page Not Found
        </h1>
        <p className="mt-4 text-lg text-[var(--color-text-secondary)]">
          The documentation page "{slug}" could not be found.
        </p>

        <Card className="mt-12 p-0">
          <CardContent className="p-8 pt-8">
            <div className="prose prose-invert max-w-none">
              <p className="text-[var(--color-text-secondary)] leading-relaxed">
                This page does not exist. Browse the available documentation
                sections below or visit the{" "}
                <a
                  href="https://github.com/nicktretyakov/CacheStorm"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-[var(--color-primary)] hover:text-[var(--color-primary)] underline underline-offset-4"
                >
                  GitHub repository
                </a>{" "}
                for the latest information.
              </p>
            </div>

            <div className="mt-8">
              <Link to="/docs">
                <Button variant="outline" size="sm">
                  Browse all docs
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
