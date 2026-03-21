import { useState, useEffect, useCallback, type ReactNode } from "react";
import { Link, useLocation } from "react-router-dom";
import {
  BookOpen,
  Settings,
  Terminal,
  Shield,
  BarChart3,
  Network,
  Globe,
  ChevronDown,
  ChevronRight,
  Menu,
  X,
  ExternalLink,
  Hash,
} from "lucide-react";
import { cn } from "@/lib/utils";

/* ------------------------------------------------------------------ */
/*  Types                                                             */
/* ------------------------------------------------------------------ */

interface TocItem {
  id: string;
  text: string;
  level: number;
}

interface NavSection {
  title: string;
  items: NavItem[];
}

interface NavItem {
  title: string;
  href: string;
  icon: ReactNode;
}

/* ------------------------------------------------------------------ */
/*  Sidebar navigation data                                           */
/* ------------------------------------------------------------------ */

const navSections: NavSection[] = [
  {
    title: "Getting Started",
    items: [
      {
        title: "Introduction",
        href: "/docs/getting-started",
        icon: <BookOpen className="w-4 h-4" />,
      },
      {
        title: "Configuration",
        href: "/docs/configuration",
        icon: <Settings className="w-4 h-4" />,
      },
    ],
  },
  {
    title: "Usage",
    items: [
      {
        title: "Commands",
        href: "/docs/commands",
        icon: <Terminal className="w-4 h-4" />,
      },
      {
        title: "HTTP API",
        href: "/docs/api",
        icon: <Globe className="w-4 h-4" />,
      },
    ],
  },
  {
    title: "Operations",
    items: [
      {
        title: "Security",
        href: "/docs/security",
        icon: <Shield className="w-4 h-4" />,
      },
      {
        title: "Monitoring",
        href: "/docs/monitoring",
        icon: <BarChart3 className="w-4 h-4" />,
      },
      {
        title: "Clustering",
        href: "/docs/clustering",
        icon: <Network className="w-4 h-4" />,
      },
    ],
  },
];

/* ------------------------------------------------------------------ */
/*  Collapsible sidebar section                                       */
/* ------------------------------------------------------------------ */

function SidebarSection({
  section,
  currentPath,
  onNavigate,
}: {
  section: NavSection;
  currentPath: string;
  onNavigate?: () => void;
}) {
  const hasActive = section.items.some((i) => currentPath === i.href);
  const [open, setOpen] = useState(true);

  // auto-open section that contains the active page
  useEffect(() => {
    if (hasActive) setOpen(true);
  }, [hasActive]);

  return (
    <div className="mb-1">
      <button
        onClick={() => setOpen((v) => !v)}
        className="flex items-center justify-between w-full px-3 py-2 text-xs font-semibold uppercase tracking-wider text-[var(--color-text-secondary)] hover:text-[var(--color-text)] transition-colors"
      >
        {section.title}
        {open ? (
          <ChevronDown className="w-3.5 h-3.5" />
        ) : (
          <ChevronRight className="w-3.5 h-3.5" />
        )}
      </button>

      {open && (
        <ul className="space-y-0.5">
          {section.items.map((item) => {
            const active = currentPath === item.href;
            return (
              <li key={item.href}>
                <Link
                  to={item.href}
                  onClick={onNavigate}
                  className={cn(
                    "flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-all duration-150",
                    active
                      ? "bg-blue-600/20 text-[var(--color-primary)] font-medium border-l-2 border-blue-500 ml-0 pl-2.5"
                      : "text-[var(--color-text-secondary)] hover:text-[var(--color-text)] hover:bg-white/5"
                  )}
                >
                  {item.icon}
                  {item.title}
                </Link>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Table-of-contents (right sidebar)                                 */
/* ------------------------------------------------------------------ */

function TableOfContents({ items }: { items: TocItem[] }) {
  const [activeId, setActiveId] = useState<string>("");

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            setActiveId(entry.target.id);
          }
        }
      },
      { rootMargin: "-80px 0px -60% 0px", threshold: 0.1 }
    );

    for (const item of items) {
      const el = document.getElementById(item.id);
      if (el) observer.observe(el);
    }

    return () => observer.disconnect();
  }, [items]);

  if (items.length === 0) return null;

  return (
    <nav className="space-y-1">
      <p className="text-xs font-semibold uppercase tracking-wider text-[var(--color-text-secondary)] mb-3">
        On this page
      </p>
      {items.map((item) => (
        <a
          key={item.id}
          href={`#${item.id}`}
          className={cn(
            "block text-sm py-1 transition-colors duration-150 border-l-2",
            item.level === 2 ? "pl-3" : "pl-6",
            activeId === item.id
              ? "text-[var(--color-primary)] border-blue-500"
              : "text-[var(--color-text-tertiary)] hover:text-[var(--color-text-secondary)] border-transparent"
          )}
        >
          {item.text}
        </a>
      ))}
    </nav>
  );
}

/* ------------------------------------------------------------------ */
/*  DocsLayout (exported)                                             */
/* ------------------------------------------------------------------ */

export default function DocsLayout({
  children,
  toc = [],
}: {
  children: ReactNode;
  toc?: TocItem[];
}) {
  const { pathname } = useLocation();
  const [sidebarOpen, setSidebarOpen] = useState(false);

  const closeSidebar = useCallback(() => setSidebarOpen(false), []);

  // close mobile sidebar on route change
  useEffect(() => {
    setSidebarOpen(false);
  }, [pathname]);

  // prevent body scroll when mobile sidebar open
  useEffect(() => {
    if (sidebarOpen) {
      document.body.style.overflow = "hidden";
    } else {
      document.body.style.overflow = "";
    }
    return () => {
      document.body.style.overflow = "";
    };
  }, [sidebarOpen]);

  return (
    <div className="min-h-screen bg-[var(--color-bg)] text-[var(--color-text-secondary)]">
      {/* ── Top bar (mobile) ────────────────────────────────────── */}
      <div className="lg:hidden sticky top-0 z-40 flex items-center gap-3 px-4 py-3 bg-[var(--color-bg-secondary)] backdrop-blur border-b border-[var(--color-border)]">
        <button
          onClick={() => setSidebarOpen(true)}
          className="p-1.5 rounded-lg hover:bg-white/10 transition-colors"
          aria-label="Open navigation"
        >
          <Menu className="w-5 h-5" />
        </button>
        <Link to="/" className="text-lg font-bold text-[var(--color-text)] tracking-tight">
          CacheStorm
        </Link>
      </div>

      <div className="max-w-[90rem] mx-auto flex">
        {/* ── Backdrop ─────────────────────────────────────────── */}
        {sidebarOpen && (
          <div
            className="fixed inset-0 z-40 bg-black/60 lg:hidden"
            onClick={closeSidebar}
          />
        )}

        {/* ── Left sidebar ─────────────────────────────────────── */}
        <aside
          className={cn(
            "fixed top-0 left-0 z-50 h-screen w-72 bg-[var(--color-bg-secondary)] border-r border-[var(--color-border)] flex flex-col transition-transform duration-300 lg:sticky lg:translate-x-0 lg:z-0",
            sidebarOpen ? "translate-x-0" : "-translate-x-full"
          )}
        >
          {/* logo / brand */}
          <div className="flex items-center justify-between px-4 py-4 border-b border-[var(--color-border)]">
            <Link
              to="/"
              className="flex items-center gap-2 text-lg font-bold text-[var(--color-text)] tracking-tight"
            >
              <div className="w-7 h-7 rounded-lg bg-gradient-to-br from-blue-500 to-cyan-400 flex items-center justify-center">
                <span className="text-[var(--color-text)] text-xs font-black">CS</span>
              </div>
              CacheStorm
            </Link>
            <button
              onClick={closeSidebar}
              className="lg:hidden p-1.5 rounded-lg hover:bg-white/10 transition-colors"
              aria-label="Close navigation"
            >
              <X className="w-5 h-5" />
            </button>
          </div>

          {/* nav */}
          <nav className="flex-1 overflow-y-auto px-3 py-4 space-y-1 scrollbar-thin scrollbar-thumb-slate-700">
            {navSections.map((section) => (
              <SidebarSection
                key={section.title}
                section={section}
                currentPath={pathname}
                onNavigate={closeSidebar}
              />
            ))}
          </nav>

          {/* footer links */}
          <div className="border-t border-[var(--color-border)] px-4 py-3 space-y-2">
            <a
              href="https://github.com/nicktretyakov/CacheStorm"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 text-sm text-[var(--color-text-tertiary)] hover:text-[var(--color-text-secondary)] transition-colors"
            >
              GitHub
              <ExternalLink className="w-3.5 h-3.5" />
            </a>
          </div>
        </aside>

        {/* ── Main content ─────────────────────────────────────── */}
        <main className="flex-1 min-w-0 px-6 py-10 lg:px-12 lg:py-12">
          <article className="max-w-3xl mx-auto prose-docs">{children}</article>
        </main>

        {/* ── Right sidebar (TOC) ──────────────────────────────── */}
        {toc.length > 0 && (
          <aside className="hidden xl:block w-56 shrink-0 py-12 pr-6">
            <div className="sticky top-20">
              <TableOfContents items={toc} />
            </div>
          </aside>
        )}
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Reusable doc building-blocks                                      */
/* ------------------------------------------------------------------ */

export function DocHeading({
  id,
  level = 2,
  children,
}: {
  id: string;
  level?: 2 | 3;
  children: ReactNode;
}) {
  const Tag = level === 2 ? "h2" : "h3";
  return (
    <Tag
      id={id}
      className={cn(
        "group scroll-mt-24 flex items-center gap-2",
        level === 2
          ? "text-2xl font-bold text-[var(--color-text)] mt-12 mb-4 pb-2 border-b border-[var(--color-border)]"
          : "text-xl font-semibold text-[var(--color-text)] mt-8 mb-3"
      )}
    >
      {children}
      <a
        href={`#${id}`}
        className="opacity-0 group-hover:opacity-100 transition-opacity text-[var(--color-primary)]"
        aria-label={`Link to ${typeof children === "string" ? children : id}`}
      >
        <Hash className="w-4 h-4" />
      </a>
    </Tag>
  );
}

export function CodeBlock({
  code,
  language = "bash",
  title,
}: {
  code: string;
  language?: string;
  title?: string;
}) {
  return (
    <div className="my-4 rounded-xl overflow-hidden border border-[var(--color-border)] bg-[var(--color-bg-secondary)]">
      {title && (
        <div className="px-4 py-2 text-xs font-medium text-[var(--color-text-secondary)] bg-[var(--color-surface)] border-b border-[var(--color-border)]">
          {title}
        </div>
      )}
      <pre className="p-4 overflow-x-auto text-sm leading-relaxed">
        <code className={`language-${language} text-[var(--color-text-secondary)]`}>{code}</code>
      </pre>
    </div>
  );
}

export function InfoBox({
  type = "info",
  children,
}: {
  type?: "info" | "warning" | "tip";
  children: ReactNode;
}) {
  const styles = {
    info: "border-blue-500/40 bg-[var(--color-surface)] text-[var(--color-primary)]",
    warning: "border-amber-500/40 bg-amber-500/10 text-amber-300",
    tip: "border-emerald-500/40 bg-emerald-500/10 text-emerald-300",
  };

  const labels = { info: "Info", warning: "Warning", tip: "Tip" };

  return (
    <div
      className={cn(
        "my-4 rounded-xl border-l-4 px-4 py-3 text-sm leading-relaxed",
        styles[type]
      )}
    >
      <p className="font-semibold mb-1">{labels[type]}</p>
      {children}
    </div>
  );
}

export type { TocItem };
