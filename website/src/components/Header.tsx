import { useState } from "react";
import { Link, useLocation } from "react-router-dom";
import { Menu, X, Sun, Moon, Zap, Github } from "lucide-react";
import { Button } from "./ui/button";
import { useTheme } from "./ThemeProvider";
import { cn } from "../lib/utils";

const navLinks = [
  { href: "/features", label: "Features" },
  { href: "/docs", label: "Docs" },
  { href: "/changelog", label: "Changelog" },
];

export function Header() {
  const [mobileOpen, setMobileOpen] = useState(false);
  const { theme, toggleTheme } = useTheme();
  const location = useLocation();

  return (
    <header
      className="fixed top-0 left-0 right-0 z-50 backdrop-blur-md border-b"
      style={{
        backgroundColor: "var(--color-header-bg)",
        borderColor: "var(--color-header-border)",
      }}
    >
      <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8">
        <Link to="/" className="flex items-center gap-2.5">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-[var(--color-primary)]">
            <Zap className="h-4 w-4 text-white" />
          </div>
          <span className="text-lg font-bold tracking-tight" style={{ color: "var(--color-text)" }}>
            Cache<span style={{ color: "var(--color-primary)" }}>Storm</span>
          </span>
        </Link>

        <nav className="hidden md:flex items-center gap-1">
          {navLinks.map((link) => (
            <Link
              key={link.href}
              to={link.href}
              className={cn(
                "px-4 py-2 text-sm font-medium rounded-lg transition-colors",
                location.pathname.startsWith(link.href)
                  ? "bg-[var(--color-surface)]"
                  : "hover:bg-[var(--color-surface)]"
              )}
              style={{ color: location.pathname.startsWith(link.href) ? "var(--color-text)" : "var(--color-text-secondary)" }}
            >
              {link.label}
            </Link>
          ))}
          <a
            href="https://github.com/CacheStorm/CacheStorm"
            target="_blank"
            rel="noopener noreferrer"
            className="px-4 py-2 text-sm font-medium rounded-lg transition-colors hover:bg-[var(--color-surface)] inline-flex items-center gap-1.5"
            style={{ color: "var(--color-text-secondary)" }}
          >
            <Github className="h-4 w-4" />
            GitHub
          </a>
        </nav>

        <div className="flex items-center gap-2">
          <button
            onClick={toggleTheme}
            className="flex h-9 w-9 items-center justify-center rounded-lg transition-colors hover:bg-[var(--color-surface)] cursor-pointer"
            style={{ color: "var(--color-text-secondary)" }}
            aria-label="Toggle theme"
          >
            {theme === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
          </button>

          <Link to="/docs/getting-started" className="hidden sm:block">
            <Button size="sm">Get Started</Button>
          </Link>

          <button
            onClick={() => setMobileOpen(!mobileOpen)}
            className="flex h-9 w-9 items-center justify-center rounded-lg transition-colors hover:bg-[var(--color-surface)] md:hidden cursor-pointer"
            style={{ color: "var(--color-text-secondary)" }}
            aria-label="Toggle menu"
          >
            {mobileOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
          </button>
        </div>
      </div>

      {mobileOpen && (
        <div
          className="border-t backdrop-blur-md md:hidden"
          style={{ backgroundColor: "var(--color-header-bg)", borderColor: "var(--color-header-border)" }}
        >
          <nav className="mx-auto max-w-7xl px-4 py-4 space-y-1">
            {navLinks.map((link) => (
              <Link
                key={link.href}
                to={link.href}
                onClick={() => setMobileOpen(false)}
                className="block px-4 py-2.5 text-sm font-medium rounded-lg transition-colors hover:bg-[var(--color-surface)]"
                style={{ color: "var(--color-text-secondary)" }}
              >
                {link.label}
              </Link>
            ))}
            <a
              href="https://github.com/CacheStorm/CacheStorm"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 px-4 py-2.5 text-sm font-medium rounded-lg transition-colors hover:bg-[var(--color-surface)]"
              style={{ color: "var(--color-text-secondary)" }}
            >
              <Github className="h-4 w-4" />
              GitHub
            </a>
          </nav>
        </div>
      )}
    </header>
  );
}
