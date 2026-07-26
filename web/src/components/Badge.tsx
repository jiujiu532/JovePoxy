import type { ReactNode } from "react";
import { cn } from "@/lib/cn";
import { assertNever } from "@/lib/assertNever";

export type BadgeKind =
  | "free"
  | "paid"
  | "healthy"
  | "warning"
  | "error"
  | "neutral";

export type BadgeProps = {
  readonly kind?: BadgeKind;
  readonly children: ReactNode;
  readonly className?: string;
};

function kindClass(kind: BadgeKind): string {
  switch (kind) {
    case "free":
      return "bg-accent-soft text-ink border-border";
    case "paid":
      return "bg-accent-yellow text-black border-border";
    case "healthy":
      return "bg-accent-mint text-black border-border";
    case "warning":
      return "bg-accent-yellow text-black border-border";
    case "error":
      return "bg-accent text-black border-border";
    case "neutral":
      return "bg-paper-0 text-ink border-border";
    default:
      return assertNever(kind);
  }
}

export function Badge({ kind = "neutral", children, className }: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-none border-2 px-2 py-0.5 text-caption font-semibold",
        kindClass(kind),
        className,
      )}
    >
      {children}
    </span>
  );
}
