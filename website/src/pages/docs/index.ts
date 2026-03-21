import { lazy } from "react";

/**
 * Docs routing index.
 *
 * Maps URL slugs to page components (lazy-loaded for code-splitting).
 * Import this map from your router configuration to wire up the /docs/* routes.
 */

const GettingStarted = lazy(() => import("./GettingStarted"));
const Configuration = lazy(() => import("./Configuration"));
const Commands = lazy(() => import("./Commands"));
const Security = lazy(() => import("./Security"));
const Monitoring = lazy(() => import("./Monitoring"));
const Clustering = lazy(() => import("./Clustering"));
const API = lazy(() => import("./API"));

export interface DocRoute {
  slug: string;
  title: string;
  component: React.LazyExoticComponent<React.ComponentType>;
}

/**
 * Ordered list of doc routes. The first entry is the default docs landing page.
 */
export const docRoutes: DocRoute[] = [
  { slug: "getting-started", title: "Getting Started", component: GettingStarted },
  { slug: "configuration", title: "Configuration", component: Configuration },
  { slug: "commands", title: "Commands", component: Commands },
  { slug: "security", title: "Security", component: Security },
  { slug: "monitoring", title: "Monitoring", component: Monitoring },
  { slug: "clustering", title: "Clustering", component: Clustering },
  { slug: "api", title: "HTTP API", component: API },
];

/**
 * Lookup a doc page component by slug.
 * Returns undefined if the slug does not match any route.
 */
export function getDocBySlug(slug: string): DocRoute | undefined {
  return docRoutes.find((r) => r.slug === slug);
}

/**
 * Default doc slug (used when navigating to /docs with no sub-path).
 */
export const defaultDocSlug = "getting-started";
