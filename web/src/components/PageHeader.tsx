import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export type PageHeaderProps = {
  readonly title: string;
  readonly description?: string;
  readonly meta?: ReactNode;
  readonly actions?: ReactNode;
  /** Control strip under title (tabs / chips). Actions sit on this row when present. */
  readonly toolbar?: ReactNode;
  readonly className?: string;
};

/**
 * Page chrome.
 * - Title block stays clean.
 * - When toolbar/meta exist: [tabs · chips …… actions] on one row.
 * - Otherwise actions stay beside the title (overview / settings).
 */
export function PageHeader({
  title,
  description,
  meta,
  actions,
  toolbar,
  className,
}: PageHeaderProps) {
  const hasControlRow = Boolean(toolbar || meta);
  const showTitleActions = Boolean(actions) && !hasControlRow;

  return (
    <header className={cn("flex flex-col gap-3.5", className)}>
      <div
        className={cn(
          "flex flex-col gap-3",
          showTitleActions && "sm:flex-row sm:items-start sm:justify-between",
        )}
      >
        <div className="min-w-0">
          <h1 className="text-[1.375rem] font-semibold tracking-tight text-ink">
            {title}
          </h1>
          {description ? (
            <p className="mt-1 max-w-2xl text-[13px] leading-relaxed text-ink-muted">
              {description}
            </p>
          ) : null}
        </div>
        {showTitleActions ? (
          <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>
        ) : null}
      </div>

      {hasControlRow ? (
        <div className="flex flex-col gap-2.5 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-2">
            {toolbar}
            {meta}
          </div>
          {actions ? (
            <div className="flex w-full min-w-0 flex-wrap items-center gap-2 lg:w-auto lg:shrink-0 lg:justify-end">
              {actions}
            </div>
          ) : null}
        </div>
      ) : null}
    </header>
  );
}

/** Compact stat chip for page-header meta row. */
export function MetaChip({
  children,
  className,
}: {
  readonly children: ReactNode;
  readonly className?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex h-7 items-center rounded-none border-2 border-border bg-paper-0 px-2.5 text-[11px] font-medium tabular-nums text-ink-muted",
        className,
      )}
    >
      {children}
    </span>
  );
}
