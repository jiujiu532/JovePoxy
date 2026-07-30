import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export type PosterInfoBar = {
  readonly label: string;
  readonly detail: string;
  readonly tone: "accent" | "teal" | "mint" | "yellow" | "coral";
};

export type PosterEmptyProps = {
  readonly stamp: string;
  readonly stampSub?: string;
  readonly title: string;
  readonly description: string;
  readonly action: ReactNode;
  readonly note?: ReactNode;
  readonly bars?: ReadonlyArray<PosterInfoBar>;
  readonly giant?: string;
  readonly className?: string;
};

const barTone: Record<PosterInfoBar["tone"], string> = {
  accent: "bg-accent text-black",
  teal: "bg-accent-teal text-black",
  mint: "bg-accent-mint text-black",
  yellow: "bg-accent-yellow text-black",
  coral: "bg-accent-coral text-black",
};

/** Full-bleed neo-brutal empty poster (scheme B). */
export function PosterEmpty({
  stamp,
  stampSub,
  title,
  description,
  action,
  note,
  bars,
  giant = "0",
  className,
}: PosterEmptyProps) {
  return (
    <div
      className={cn(
        "relative isolate min-h-[22rem] overflow-hidden border-2 border-border bg-paper-0",
        "shadow-[4px_4px_0_var(--border)]",
        className,
      )}
    >
      <div
        className="pointer-events-none absolute inset-[-20%_-10%] z-0 opacity-95"
        style={{
          background:
            "linear-gradient(115deg, transparent 0 28%, var(--accent-yellow) 28% 42%, transparent 42% 58%, var(--accent-mint) 58% 66%, transparent 66% 100%)",
          transform: "skewY(-6deg)",
        }}
        aria-hidden
      />
      <div
        className="pointer-events-none absolute inset-0 z-0 opacity-40 mix-blend-multiply"
        style={{
          backgroundImage:
            "linear-gradient(to right, rgba(0,0,0,0.05) 1px, transparent 1px), linear-gradient(to bottom, rgba(0,0,0,0.05) 1px, transparent 1px)",
          backgroundSize: "28px 28px",
        }}
        aria-hidden
      />

      <div className="relative z-[1] flex min-h-[22rem] flex-col gap-3 p-5 sm:p-6">
        <div className="flex items-start justify-between gap-3">
          <div
            className={cn(
              "inline-block -rotate-[8deg] border-[3px] border-border bg-accent-yellow px-3 py-2",
              "font-mono text-[11px] font-black uppercase leading-tight tracking-[0.12em]",
              "shadow-[4px_4px_0_var(--border)]",
            )}
          >
            {stamp}
            {stampSub ? (
              <span className="mt-0.5 block text-[10px] tracking-[0.16em] opacity-85">
                {stampSub}
              </span>
            ) : null}
          </div>
          <span
            className="select-none font-mono text-[clamp(3.5rem,10vw,5.5rem)] font-black leading-none tracking-tighter text-ink"
            style={{ textShadow: "4px 4px 0 var(--accent)" }}
            aria-hidden
          >
            {giant}
          </span>
        </div>

        <div className="grid flex-1 grid-cols-1 items-end gap-4 md:grid-cols-[1.4fr_0.9fr]">
          <div>
            <h2 className="m-0 text-[clamp(2.25rem,7vw,4rem)] font-black leading-[0.95] tracking-tight text-ink">
              {title}
            </h2>
            <p className="mt-3 max-w-[36ch] text-[15px] font-semibold leading-snug text-ink-muted">
              {description}
            </p>
          </div>
          <div className="flex flex-col items-stretch gap-2 md:items-end">
            {action}
            {note ? (
              <div className="border-2 border-border bg-paper-0 px-2 py-1 font-mono text-[11px] font-bold text-ink">
                {note}
              </div>
            ) : null}
          </div>
        </div>

        {bars && bars.length > 0 ? (
          <div className="mt-1 grid grid-cols-1 border-2 border-border shadow-[4px_4px_0_var(--border)] sm:grid-cols-3">
            {bars.map((bar, i) => (
              <div
                key={bar.label}
                className={cn(
                  "flex min-h-[4.5rem] flex-col justify-center gap-1 px-3.5 py-3",
                  barTone[bar.tone],
                  i < bars.length - 1 && "border-b-2 border-border sm:border-b-0 sm:border-r-2",
                )}
              >
                <span className="text-[11px] font-bold uppercase tracking-wide opacity-80">
                  {bar.label}
                </span>
                <span className="text-[13px] font-extrabold leading-snug">{bar.detail}</span>
              </div>
            ))}
          </div>
        ) : null}
      </div>
    </div>
  );
}
