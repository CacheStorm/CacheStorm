import { Link } from "react-router-dom";
import { Zap, Github } from "lucide-react";

const footerLinks = {
  Product: [
    { label: "Features", href: "/features" },
    { label: "Documentation", href: "/docs" },
    { label: "Changelog", href: "/changelog" },
  ],
  Docs: [
    { label: "Getting Started", href: "/docs/getting-started" },
    { label: "Configuration", href: "/docs/configuration" },
    { label: "Commands", href: "/docs/commands" },
    { label: "Security", href: "/docs/security" },
    { label: "Monitoring", href: "/docs/monitoring" },
    { label: "Clustering", href: "/docs/clustering" },
    { label: "HTTP API", href: "/docs/api" },
  ],
  Community: [
    { label: "GitHub", href: "https://github.com/CacheStorm/CacheStorm", external: true },
    { label: "Discussions", href: "https://github.com/CacheStorm/CacheStorm/discussions", external: true },
    { label: "Issues", href: "https://github.com/CacheStorm/CacheStorm/issues", external: true },
  ],
};

export function Footer() {
  return (
    <footer className="border-t" style={{ borderColor: "var(--color-border)", backgroundColor: "var(--color-bg-secondary)" }}>
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="grid grid-cols-2 gap-8 py-12 md:grid-cols-4">
          <div className="col-span-2 md:col-span-1">
            <Link to="/" className="flex items-center gap-2.5">
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-[var(--color-primary)]">
                <Zap className="h-4 w-4 text-white" />
              </div>
              <span className="text-lg font-bold tracking-tight" style={{ color: "var(--color-text)" }}>
                Cache<span style={{ color: "var(--color-primary)" }}>Storm</span>
              </span>
            </Link>
            <p className="mt-4 text-sm max-w-xs" style={{ color: "var(--color-text-secondary)" }}>
              High-performance, Redis-compatible caching server built in Go.
            </p>
            <div className="mt-4">
              <a
                href="https://github.com/CacheStorm/CacheStorm"
                target="_blank"
                rel="noopener noreferrer"
                className="flex h-9 w-9 items-center justify-center rounded-lg transition-colors hover:bg-[var(--color-surface)]"
                style={{ color: "var(--color-text-secondary)" }}
                aria-label="GitHub"
              >
                <Github className="h-4 w-4" />
              </a>
            </div>
          </div>

          {Object.entries(footerLinks).map(([title, links]) => (
            <div key={title}>
              <h3 className="text-sm font-semibold" style={{ color: "var(--color-text)" }}>{title}</h3>
              <ul className="mt-4 space-y-2.5">
                {links.map((link) => (
                  <li key={link.label}>
                    {"external" in link && link.external ? (
                      <a
                        href={link.href}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-sm transition-colors hover:underline"
                        style={{ color: "var(--color-text-secondary)" }}
                      >
                        {link.label}
                      </a>
                    ) : (
                      <Link
                        to={link.href}
                        className="text-sm transition-colors hover:underline"
                        style={{ color: "var(--color-text-secondary)" }}
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

        <div className="border-t py-6 text-center" style={{ borderColor: "var(--color-border)" }}>
          <p className="text-sm" style={{ color: "var(--color-text-tertiary)" }}>
            &copy; {new Date().getFullYear()} CacheStorm. Open source under MIT License.
          </p>
        </div>
      </div>
    </footer>
  );
}
