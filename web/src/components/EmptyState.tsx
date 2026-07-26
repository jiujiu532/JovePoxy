import { Tray } from "@phosphor-icons/react";
import type { Icon } from "@phosphor-icons/react";
import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export type EmptyStateProps = {
  readonly title: string;
  readonly description?: string;
  readonly action?: ReactNode;
  readonly icon?: Icon;
  readonly className?: string;
  readonly compact?: boolean;
};

export function EmptyState({
  title,
  description,
  action,
  icon: IconComp = Tray,
  className,
  compact = false,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center text-center",
        compact ? "gap-2.5 px-3 py-8" : "gap-3.5 px-4 py-14",
        className,
      )}
    >
      <div
        className={cn(
          "flex items-center justify-center rounded-none border-4 border-border bg-accent-yellow text-black shadow-[var(--shadow-hard)]",
          compact ? "h-11 w-11" : "h-14 w-14",
        )}
      >
        <IconComp size={compact ? 20 : 24} weight="bold" />
      </div>
      <div className="max-w-sm">
        <p className="text-sm font-semibold text-ink">{title}</p>
        {description ? (
          <p className="mt-1.5 text-[13px] leading-relaxed text-ink-muted">
            {description}
          </p>
        ) : null}
      </div>
      {action ? <div className="mt-1">{action}</div> : null}
    </div>
  );
}
