import type { Icon } from "@phosphor-icons/react";
import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export type SectionPanelProps = {
  readonly title: string;
  readonly description?: string;
  readonly actions?: ReactNode;
  readonly children: ReactNode;
  readonly className?: string;
  readonly bodyClassName?: string;
  readonly icon?: Icon;
  readonly iconTone?: "default" | "accent" | "yellow" | "teal" | "mint";
};

const iconToneClass: Record<NonNullable<SectionPanelProps["iconTone"]>, string> = {
  default: "bg-paper-0 text-ink",
  accent: "bg-accent text-black",
  yellow: "bg-accent-yellow text-black",
  teal: "bg-accent-teal text-black",
  mint: "bg-accent-mint text-black",
};

export function SectionPanel({
  title,
  description,
  actions,
  children,
  className,
  bodyClassName,
  icon: IconComp,
  iconTone = "default",
}: SectionPanelProps) {
  return (
    <section
      className={cn(
        "overflow-hidden rounded-none border-2 border-border bg-paper-1 shadow-[var(--shadow-hard)]",
        className,
      )}
    >
      <header className="flex flex-wrap items-start justify-between gap-3 border-b-2 border-border px-4 py-3.5 md:px-5">
        <div className="flex min-w-0 items-start gap-3">
          {IconComp ? (
            <span
              className={cn(
                "mt-0.5 inline-flex h-9 w-9 shrink-0 items-center justify-center border-2 border-border",
                "shadow-[2px_2px_0_var(--border)]",
                iconToneClass[iconTone],
              )}
              aria-hidden
            >
              <IconComp size={18} weight="duotone" />
            </span>
          ) : null}
          <div className="min-w-0">
            <h2 className="text-[15px] font-semibold tracking-tight text-ink">
              {title}
            </h2>
            {description ? (
              <p className="mt-0.5 text-[12px] leading-relaxed text-ink-muted">
                {description}
              </p>
            ) : null}
          </div>
        </div>
        {actions ? (
          <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>
        ) : null}
      </header>
      <div className={cn("p-4 md:p-5", bodyClassName)}>{children}</div>
    </section>
  );
}
