import type { Icon } from "@phosphor-icons/react";
import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export type StatCardProps = {
  readonly label: string;
  readonly value: ReactNode;
  readonly hint?: string;
  readonly icon?: Icon;
  readonly tone?: "default" | "accent" | "success" | "warning" | "error";
  readonly className?: string;
};

const toneIcon: Record<NonNullable<StatCardProps["tone"]>, string> = {
  default: "bg-paper-0 text-ink border-2 border-border",
  accent: "bg-accent-soft text-ink border-2 border-border",
  success: "bg-accent-mint text-black border-2 border-border",
  warning: "bg-accent-yellow text-black border-2 border-border",
  error: "bg-accent text-black border-2 border-border",
};

export function StatCard({
  label,
  value,
  hint,
  icon: IconComp,
  tone = "default",
  className,
}: StatCardProps) {
  return (
    <article
      className={cn(
        "relative border-4 border-border bg-paper-1 p-4 shadow-[var(--shadow-hard)] rounded-none",
        className,
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-caption font-semibold tracking-wide text-ink-muted">
            {label}
          </p>
          <p className="mt-2 text-[1.75rem] font-semibold leading-none tracking-tight text-ink tabular-nums">
            {value}
          </p>
          {hint ? (
            <p className="mt-2 text-[12px] leading-snug text-ink-faint">{hint}</p>
          ) : null}
        </div>
        {IconComp ? (
          <div
            className={cn(
              "flex h-10 w-10 shrink-0 items-center justify-center rounded-none",
              toneIcon[tone],
            )}
          >
            <IconComp size={18} weight="bold" />
          </div>
        ) : null}
      </div>
    </article>
  );
}
