import { Link, useParams } from "react-router-dom";
import { Button } from "../components/ui/button";
import { Card, CardContent } from "../components/ui/card";
import { Badge } from "../components/ui/badge";
import {
  BookOpen,
  Settings,
  Terminal,
  Code,
  ArrowRight,
} from "lucide-react";

const docSections = [
  {
    slug: "getting-started",
    title: "Getting Started",
    description: "Install CacheStorm and run your first commands in under 5 minutes.",
    icon: Terminal,
    color: "text-emerald-400",
    bg: "bg-emerald-400/10",
  },
  {
    slug: "configuration",
    title: "Configuration",
    description: "Learn how to configure CacheStorm for your specific use case.",
    icon: Settings,
    color: "text-blue-400",
    bg: "bg-blue-400/10",
  },
  {
    slug: "api",
    title: "API Reference",
    description: "Complete reference of all supported Redis commands and extensions.",
    icon: Code,
    color: "text-purple-400",
    bg: "bg-purple-400/10",
  },
  {
    slug: "cli",
    title: "CLI Reference",
    description: "Command-line options and flags for the CacheStorm server.",
    icon: BookOpen,
    color: "text-cyan-400",
    bg: "bg-cyan-400/10",
  },
];

export function DocsIndex() {
  return (
    <div className="relative pt-32 pb-24">
      <div className="mx-auto max-w-4xl px-4 sm:px-6 lg:px-8">
        <Badge className="mb-4">Documentation</Badge>
        <h1 className="text-3xl font-bold tracking-tight text-white sm:text-4xl">
          CacheStorm <span className="gradient-text">Documentation</span>
        </h1>
        <p className="mt-4 text-lg text-slate-400 max-w-2xl">
          Everything you need to install, configure, and operate CacheStorm in production.
        </p>

        <div className="mt-12 grid gap-4 sm:grid-cols-2">
          {docSections.map((section) => (
            <Link key={section.slug} to={`/docs/${section.slug}`}>
              <Card className="group h-full hover:border-slate-700 hover:bg-slate-800/50 p-0 transition-all duration-200">
                <CardContent className="p-6 pt-6 flex flex-col h-full">
                  <div className={`mb-4 flex h-10 w-10 items-center justify-center rounded-lg ${section.bg}`}>
                    <section.icon className={`h-5 w-5 ${section.color}`} />
                  </div>
                  <h2 className="text-base font-semibold text-white group-hover:text-blue-400 transition-colors">
                    {section.title}
                  </h2>
                  <p className="mt-2 text-sm text-slate-400 flex-1">
                    {section.description}
                  </p>
                  <div className="mt-4 flex items-center text-sm text-blue-400 opacity-0 group-hover:opacity-100 transition-opacity">
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

export function DocPage() {
  const { slug } = useParams<{ slug: string }>();
  const section = docSections.find((s) => s.slug === slug);

  return (
    <div className="relative pt-32 pb-24">
      <div className="mx-auto max-w-4xl px-4 sm:px-6 lg:px-8">
        <Link
          to="/docs"
          className="text-sm text-slate-400 hover:text-white transition-colors mb-6 inline-block"
        >
          &larr; Back to Documentation
        </Link>

        <h1 className="text-3xl font-bold tracking-tight text-white sm:text-4xl">
          {section?.title ?? slug}
        </h1>
        <p className="mt-4 text-lg text-slate-400">
          {section?.description ?? "Documentation page"}
        </p>

        <Card className="mt-12 p-0">
          <CardContent className="p-8 pt-8">
            <div className="prose prose-invert max-w-none">
              <p className="text-slate-400 leading-relaxed">
                This documentation page is coming soon. Check back later or
                visit the{" "}
                <a
                  href="https://github.com/nicholasgasior/cachestorm"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-blue-400 hover:text-blue-300 underline underline-offset-4"
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
