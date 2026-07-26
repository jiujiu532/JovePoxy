import type { KeyboardEvent } from "react";
import { cn } from "@/lib/cn";

export type TabItem = {
  readonly id: string;
  readonly label: string;
  /** Optional count badge (subtle, not glued into the label text). */
  readonly count?: number;
};

export type TabsProps = {
  readonly items: readonly TabItem[];
  readonly value: string;
  readonly onChange: (id: string) => void;
  readonly className?: string;
  /**
   * pill — hard segment control (default, neo-brutalist blocks)
   * line — classic underline tabs
   */
  readonly variant?: "pill" | "line";
  readonly "aria-label"?: string;
};

export function Tabs({
  items,
  value,
  onChange,
  className,
  variant = "pill",
  "aria-label": ariaLabel = "分区",
}: TabsProps) {
  function onKeyNav(event: KeyboardEvent, currentId: string) {
    if (event.key !== "ArrowRight" && event.key !== "ArrowLeft") return;
    event.preventDefault();
    const idx = items.findIndex((t) => t.id === currentId);
    if (idx < 0) return;
    const delta = event.key === "ArrowRight" ? 1 : -1;
    const next = items[(idx + delta + items.length) % items.length];
    if (next) onChange(next.id);
  }

  if (variant === "line") {
    return (
      <div
        role="tablist"
        aria-label={ariaLabel}
        className={cn("flex gap-0.5 border-b border-border", className)}
      >
        {items.map((item) => {
          const selected = item.id === value;
          return (
            <button
              key={item.id}
              type="button"
              role="tab"
              id={`tab-${item.id}`}
              aria-selected={selected}
              tabIndex={selected ? 0 : -1}
              className={cn(
                "relative -mb-px inline-flex items-center gap-1.5 px-3.5 py-2 text-[13px] font-medium transition-colors",
                selected ? "text-ink" : "text-ink-muted hover:text-ink",
              )}
              onClick={() => onChange(item.id)}
              onKeyDown={(e) => onKeyNav(e, item.id)}
            >
              {item.label}
              {item.count !== undefined ? (
                <span
                  className={cn(
                    "tabular-nums text-[11px]",
                    selected ? "text-ink-muted" : "text-ink-faint",
                  )}
                >
                  {item.count}
                </span>
              ) : null}
              {selected ? (
                <span
                  className="absolute inset-x-2.5 bottom-0 h-[2px] rounded-none bg-accent"
                  aria-hidden
                />
              ) : null}
            </button>
          );
        })}
      </div>
    );
  }

  return (
    <div
      role="tablist"
      aria-label={ariaLabel}
      className={cn(
        "inline-flex h-9 max-w-full flex-wrap items-center gap-0.5 rounded-none border-2 border-border bg-paper-0 p-0.5",
        className,
      )}
    >
      {items.map((item) => {
        const selected = item.id === value;
        return (
          <button
            key={item.id}
            type="button"
            role="tab"
            id={`tab-${item.id}`}
            aria-selected={selected}
            tabIndex={selected ? 0 : -1}
            className={cn(
              "inline-flex h-8 items-center gap-1.5 rounded-none px-3 text-[13px] font-medium transition-[background-color,color,box-shadow] duration-150",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus-ring",
              selected
                ? "bg-paper-2 text-ink shadow-[2px_2px_0_var(--border)] ring-1 ring-border"
                : "text-ink-muted hover:bg-paper-1/80 hover:text-ink",
            )}
            onClick={() => onChange(item.id)}
            onKeyDown={(e) => onKeyNav(e, item.id)}
          >
            {item.label}
            {item.count !== undefined ? (
              <span
                className={cn(
                  "inline-flex min-w-[1.15rem] items-center justify-center rounded-none px-1 py-px text-[10px] font-semibold tabular-nums leading-none",
                  selected
                    ? "bg-accent/15 text-accent"
                    : "bg-paper-2 text-ink-faint",
                )}
              >
                {item.count}
              </span>
            ) : null}
          </button>
        );
      })}
    </div>
  );
}
