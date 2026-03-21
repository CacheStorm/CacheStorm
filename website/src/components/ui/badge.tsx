import type { HTMLAttributes } from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "../../lib/utils";

const badgeVariants = cva(
  "inline-flex items-center rounded-full px-3 py-1 text-xs font-medium border",
  {
    variants: {
      variant: {
        default: "bg-[color-mix(in_srgb,var(--color-primary)_10%,transparent)] text-[var(--color-primary)] border-[color-mix(in_srgb,var(--color-primary)_20%,transparent)]",
        secondary: "bg-[var(--color-surface)] text-[var(--color-text-secondary)] border-[var(--color-border)]",
        success: "bg-green-500/10 text-green-600 dark:text-green-400 border-green-500/20",
        destructive: "bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/20",
      },
    },
    defaultVariants: { variant: "default" },
  }
);

interface BadgeProps extends HTMLAttributes<HTMLSpanElement>, VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ variant, className }))} {...props} />;
}

export { Badge, badgeVariants };
export type { BadgeProps };
