import { Link } from "react-router-dom";
import { ArrowLeft } from "lucide-react";
import { Button } from "../components/ui/button";

export default function NotFound() {
  return (
    <div className="min-h-screen flex items-center justify-center px-4" style={{ backgroundColor: "var(--color-bg)" }}>
      <div className="text-center max-w-md">
        <p className="text-6xl font-bold" style={{ color: "var(--color-primary)" }}>404</p>
        <h1 className="mt-4 text-2xl font-bold" style={{ color: "var(--color-text)" }}>
          Page not found
        </h1>
        <p className="mt-3 text-base" style={{ color: "var(--color-text-secondary)" }}>
          The page you're looking for doesn't exist or has been moved.
        </p>
        <div className="mt-8 flex justify-center gap-3">
          <Link to="/">
            <Button className="gap-2">
              <ArrowLeft className="h-4 w-4" /> Back to Home
            </Button>
          </Link>
          <Link to="/docs">
            <Button variant="outline">Documentation</Button>
          </Link>
        </div>
      </div>
    </div>
  );
}
