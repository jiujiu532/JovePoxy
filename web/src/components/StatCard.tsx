import type { Icon } from "@phosphor-icons/react";
import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export type StatCardProps = {
  readonly label: string;
  readonly value: ReactNode;
  readonly hint?: string;
  readonly icon?: Icon;
  readonly tone?: "default" | "accent" | "success" | "warning" | "error" | "teal" | "yellow";
  readonly className?: string;
  readonly footer?: ReactNode;
};

const toneIcon: Record<NonNullable<StatCardProps["tone"]>, string> = {
  /* paper-2：dark 下勿用 paper-0 图标底（与 canvas 融成黑洞） */
  default: "bg-paper-2 text-ink",
  accent: "bg-accent text-black",
  success: "bg-accent-mint text-black",
  warning: "bg-accent-yellow text-black",
  error: "bg-accent text-black",
  teal: "bg-accent-teal text-black",
  yellow: "bg-accent-yellow text-black",
};

const toneShadow: Record<NonNullable<StatCardProps["tone"]>, string> = {
  default: "shadow-[4px_4px_0_var(--border)]",
  accent: "shadow-[4px_4px_0_var(--accent)]",
  success: "shadow-[4px_4px_0_var(--accent-mint)]",
  warning: "shadow-[4px_4px_0_var(--accent-yellow)]",
  error: "shadow-[4px_4px_0_var(--accent)]",
  teal: "shadow-[4px_4px_0_var(--accent-teal)]",
  yellow: "shadow-[4px_4px_0_var(--accent-yellow)]",
};

const toneBar: Record<NonNullable<StatCardProps["tone"]>, string> = {
  default: "bg-border",
  accent: "bg-accent",
  success: "bg-accent-mint",
  warning: "bg-accent-yellow",
  error: "bg-accent",
  teal: "bg-accent-teal",
  yellow: "bg-accent-yellow",
};

export function StatCard({
  label,
  value,
  hint,
  icon: IconComp,
  tone = "default",
  className,
  footer,
}: StatCardProps) {
  return (
    <article
      className={cn(
        "group relative overflow-hidden rounded-none border-2 border-border bg-paper-1 p-4",
        "transition-[transform,box-shadow] duration-200 ease-out",
        "hover:-translate-x-px hover:-translate-y-px",
        toneShadow[tone],
        className,
      )}
    >
      {/* Left accent bar */}
      <span
        className={cn("absolute inset-y-0 left-0 w-1.5", toneBar[tone])}
        aria-hidden
      />
      {/* Decorative corner block */}
      <span
        className={cn(
          "pointer-events-none absolute -right-3 -top-3 h-12 w-12 rotate-12 border-2 border-border opacity-[0.12]",
          toneBar[tone],
        )}
        aria-hidden
      />

      <div className="relative flex items-start justify-between gap-3 pl-2">
        <div className="min-w-0">
          <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted">
            {label}
          </p>
          <p className="mt-2 text-[1.85rem] font-semibold leading-none tracking-tight text-ink tabular-nums">
            {value}
          </p>
          {hint ? (
            <p className="mt-2 text-[12px] leading-snug text-ink-faint">{hint}</p>
          ) : null}
          {footer ? <div className="mt-3">{footer}</div> : null}
        </div>
        {IconComp ? (
          <div
            className={cn(
              "flex h-11 w-11 shrink-0 items-center justify-center rounded-none border-2 border-border",
              "shadow-[2px_2px_0_var(--border)]",
              "transition-transform duration-200 ease-out group-hover:scale-105",
              toneIcon[tone],
            )}
          >
            <IconComp size={22} weight="duotone" aria-hidden />
          </div>
        ) : null}
      </div>
    </article>
  );
}
