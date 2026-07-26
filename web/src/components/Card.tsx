import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/cn";

export type CardProps = {
  readonly title?: string;
  readonly description?: string;
  readonly actions?: ReactNode;
  readonly children?: ReactNode;
} & HTMLAttributes<HTMLDivElement>;

export function Card({
  title,
  description,
  actions,
  children,
  className,
  ...rest
}: CardProps) {
  return (
    <section
      className={cn(
        "rounded-none border-4 border-border bg-paper-1 p-4 shadow-[var(--shadow-hard)] md:p-6",
        className,
      )}
      {...rest}
    >
      {(title || actions) && (
        <header className="mb-4 flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            {title ? (
              <h2 className="text-lg font-semibold tracking-tight text-ink">
                {title}
              </h2>
            ) : null}
            {description ? (
              <p className="mt-1 text-sm text-ink-muted">{description}</p>
            ) : null}
          </div>
          {actions ? <div className="flex shrink-0 items-center gap-2">{actions}</div> : null}
        </header>
      )}
      {children}
    </section>
  );
}
