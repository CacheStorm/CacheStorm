import { Link } from "react-router-dom";
import { Zap, Github, Twitter } from "lucide-react";
import { Separator } from "./ui/separator";

const footerLinks = {
  Product: [
    { label: "Features", href: "/features" },
    { label: "Pricing", href: "/#pricing" },
    { label: "Changelog", href: "/docs/getting-started" },
    { label: "Roadmap", href: "/docs/getting-started" },
  ],
  Documentation: [
    { label: "Getting Started", href: "/docs/getting-started" },
    { label: "Configuration", href: "/docs/configuration" },
    { label: "Commands", href: "/docs/commands" },
    { label: "HTTP API", href: "/docs/api" },
    { label: "Security", href: "/docs/security" },
    { label: "Monitoring", href: "/docs/monitoring" },
    { label: "Clustering", href: "/docs/clustering" },
  ],
  Community: [
    {
      label: "GitHub",
      href: "https://github.com/nicholasgasior/cachestorm",
      external: true,
    },
    {
      label: "Discussions",
      href: "https://github.com/nicholasgasior/cachestorm/discussions",
      external: true,
    },
    {
      label: "Issues",
      href: "https://github.com/nicholasgasior/cachestorm/issues",
      external: true,
    },
  ],
};

export function Footer() {
  return (
    <footer className="border-t border-slate-800/60 bg-slate-950">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        {/* Main footer content */}
        <div className="grid grid-cols-2 gap-8 py-12 md:grid-cols-4">
          {/* Brand column */}
          <div className="col-span-2 md:col-span-1">
            <Link to="/" className="flex items-center gap-2.5">
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-blue-600">
                <Zap className="h-4 w-4 text-white" />
              </div>
              <span className="text-lg font-bold tracking-tight text-white">
                Cache<span className="text-blue-400">Storm</span>
              </span>
            </Link>
            <p className="mt-4 text-sm text-slate-400 max-w-xs">
              High-performance, Redis-compatible caching server built for modern
              infrastructure.
            </p>
            <div className="mt-4 flex gap-3">
              <a
                href="https://github.com/nicholasgasior/cachestorm"
                target="_blank"
                rel="noopener noreferrer"
                className="flex h-9 w-9 items-center justify-center rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
                aria-label="GitHub"
              >
                <Github className="h-4 w-4" />
              </a>
              <a
                href="https://twitter.com/cachestorm"
                target="_blank"
                rel="noopener noreferrer"
                className="flex h-9 w-9 items-center justify-center rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
                aria-label="Twitter"
              >
                <Twitter className="h-4 w-4" />
              </a>
            </div>
          </div>

          {/* Link columns */}
          {Object.entries(footerLinks).map(([title, links]) => (
            <div key={title}>
              <h3 className="text-sm font-semibold text-white">{title}</h3>
              <ul className="mt-3 space-y-2.5">
                {links.map((link) => (
                  <li key={link.label}>
                    {"external" in link && link.external ? (
                      <a
                        href={link.href}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-sm text-slate-400 hover:text-white transition-colors"
                      >
                        {link.label}
                      </a>
                    ) : (
                      <Link
                        to={link.href}
                        className="text-sm text-slate-400 hover:text-white transition-colors"
                      >
                        {link.label}
                      </Link>
                    )}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        <Separator />

        {/* Bottom bar */}
        <div className="flex flex-col items-center justify-between gap-4 py-6 sm:flex-row">
          <p className="text-sm text-slate-500">
            &copy; {new Date().getFullYear()} CacheStorm. All rights reserved.
          </p>
          <p className="text-sm text-slate-500">
            Built with Go. Licensed under MIT.
          </p>
        </div>
      </div>
    </footer>
  );
}
