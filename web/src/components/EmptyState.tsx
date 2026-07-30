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
        "relative flex flex-col items-center justify-center overflow-hidden text-center",
        compact ? "gap-3 px-4 py-10" : "gap-4 px-5 py-14",
        className,
      )}
    >
      {/* Soft geometric backdrop */}
      <div className="pointer-events-none absolute inset-0" aria-hidden>
        <span className="absolute left-1/2 top-4 h-20 w-20 -translate-x-1/2 rounded-none border-2 border-border bg-accent-soft opacity-50" />
        <span className="absolute left-[calc(50%-2.75rem)] top-8 h-10 w-10 rotate-12 border-2 border-border bg-accent-yellow opacity-70" />
        <span className="absolute left-[calc(50%+1.25rem)] top-10 h-8 w-8 -rotate-6 border-2 border-border bg-accent-teal opacity-60" />
      </div>

      <div
        className={cn(
          "relative z-[1] flex items-center justify-center rounded-none border-2 border-border bg-accent-yellow text-black",
          "shadow-[4px_4px_0_var(--border)]",
          compact ? "h-12 w-12" : "h-16 w-16",
        )}
      >
        <IconComp size={compact ? 22 : 28} weight="duotone" aria-hidden />
      </div>
      <div className="relative z-[1] max-w-sm">
        <p className={cn("font-semibold text-ink", compact ? "text-sm" : "text-[15px]")}>
          {title}
        </p>
        {description ? (
          <p className="mt-1.5 text-[13px] leading-relaxed text-ink-muted">
            {description}
          </p>
        ) : null}
      </div>
      {action ? <div className="relative z-[1] mt-0.5">{action}</div> : null}
    </div>
  );
}
