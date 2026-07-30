import { cn } from "@/lib/cn";

export type MetricRailItem = {
  readonly label: string;
  readonly value: string | number;
  readonly hint?: string;
  readonly tone?: "yellow" | "teal" | "mint" | "accent" | "white";
};

const toneClass: Record<NonNullable<MetricRailItem["tone"]>, string> = {
  yellow: "bg-accent-yellow",
  teal: "bg-accent-teal",
  mint: "bg-accent-mint",
  accent: "bg-accent text-black",
  white: "bg-paper-0",
};

export type MetricRailProps = {
  readonly items: ReadonlyArray<MetricRailItem>;
  readonly className?: string;
};

/** Colored metric strip for list pages with data (scheme B filled state). */
export function MetricRail({ items, className }: MetricRailProps) {
  return (
    <div
      className={cn(
        "grid grid-cols-2 border-2 border-border shadow-[4px_4px_0_var(--border)] sm:grid-cols-4",
        className,
      )}
    >
      {items.map((item, i) => (
        <div
          key={item.label}
          className={cn(
            "min-w-0 px-3.5 py-3",
            toneClass[item.tone ?? "white"],
            // mobile 2-col: right border on left cells, bottom on first row
            i % 2 === 0 && "border-r-2 border-border",
            i < 2 && items.length > 2 && "border-b-2 border-border sm:border-b-0",
            // desktop 4-col: right border except last
            i < items.length - 1 && "sm:border-r-2 sm:border-border",
            // left cells already have border-r on mobile; keep on sm for middle cells
            i % 2 === 1 && i < items.length - 1 && "sm:border-r-2",
          )}
        >
          <div className="text-[11px] font-bold uppercase tracking-wide text-ink">
            {item.label}
          </div>
          <div className="mt-0.5 font-mono text-[clamp(1.75rem,4vw,2rem)] font-black leading-none tabular-nums tracking-tight">
            {item.value}
          </div>
          {item.hint ? (
            <div className="mt-1 text-[11px] font-semibold text-ink-muted">{item.hint}</div>
          ) : null}
        </div>
      ))}
    </div>
  );
}
