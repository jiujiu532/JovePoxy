import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export type PageHeaderProps = {
  readonly title: string;
  readonly description?: string;
  readonly meta?: ReactNode;
  readonly actions?: ReactNode;
  /** Control strip under title (tabs / chips). */
  readonly toolbar?: ReactNode;
  readonly className?: string;
  /**
   * When true (default), hide the large in-page H1 — shell TopBar already shows the route title.
   * Set false only for standalone / test renders outside the shell.
   */
  readonly hideTitle?: boolean;
};

/**
 * Content-area page header.
 * Title lives in the fixed shell TopBar; this block keeps description / toolbar / meta / actions
 * inside the scrollable main region.
 */
export function PageHeader({
  title,
  description,
  meta,
  actions,
  toolbar,
  className,
  hideTitle = true,
}: PageHeaderProps) {
  const hasControlRow = Boolean(toolbar || meta || actions);
  const showTitleBlock = !hideTitle || Boolean(description);

  if (!showTitleBlock && !hasControlRow) {
    return null;
  }

  // Shell mode: no duplicate H1; description + control row only.
  if (hideTitle) {
    return (
      <div className={cn("flex flex-col gap-3", className)}>
        {description ? (
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <p className="min-w-0 max-w-2xl text-[13px] leading-relaxed text-ink-muted">
              {description}
            </p>
            {actions && !toolbar && !meta ? (
              <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>
            ) : null}
          </div>
        ) : null}

        {toolbar || meta || (actions && (toolbar || meta || !description)) ? (
          <div className="flex flex-col gap-2.5 lg:flex-row lg:items-center lg:justify-between">
            <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-2">
              {toolbar}
              {meta}
            </div>
            {actions && (toolbar || meta || !description) ? (
              <div className="flex w-full min-w-0 flex-wrap items-center gap-2 lg:w-auto lg:shrink-0 lg:justify-end">
                {actions}
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
    );
  }

  // Standalone mode (tests / outside shell): classic inline header with H1.
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
          <h1 className="text-[1.375rem] font-semibold tracking-tight text-ink">{title}</h1>
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

      {(toolbar || meta || (actions && !showTitleActions)) ? (
        <div className="flex flex-col gap-2.5 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-2">
            {toolbar}
            {meta}
          </div>
          {actions && !showTitleActions ? (
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
