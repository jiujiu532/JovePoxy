import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export type SectionPanelProps = {
  readonly title: string;
  readonly description?: string;
  readonly actions?: ReactNode;
  readonly children: ReactNode;
  readonly className?: string;
  readonly bodyClassName?: string;
};

export function SectionPanel({
  title,
  description,
  actions,
  children,
  className,
  bodyClassName,
}: SectionPanelProps) {
  return (
    <section
      className={cn(
        "overflow-hidden rounded-none border-2 border-border bg-paper-1 shadow-[var(--shadow-hard)]",
        className,
      )}
    >
      <header className="flex flex-wrap items-start justify-between gap-3 border-b border-border px-4 py-3.5 md:px-5">
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
        {actions ? (
          <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>
        ) : null}
      </header>
      <div className={cn("p-4 md:p-5", bodyClassName)}>{children}</div>
    </section>
  );
}
