import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export type MobileField = {
  readonly label: string;
  readonly value: ReactNode;
};

export type MobileEntityCardProps = {
  readonly title: ReactNode;
  readonly subtitle?: ReactNode;
  readonly badge?: ReactNode;
  readonly fields?: ReadonlyArray<MobileField>;
  readonly actions?: ReactNode;
  readonly leading?: ReactNode;
  readonly muted?: boolean;
  readonly className?: string;
};

/**
 * Mobile-first list row: stacked card instead of wide table columns.
 */
export function MobileEntityCard({
  title,
  subtitle,
  badge,
  fields,
  actions,
  leading,
  muted,
  className,
}: MobileEntityCardProps) {
  return (
    <article
      className={cn(
        "border-b-2 border-border px-3 py-3 last:border-b-0",
        muted && "bg-paper-0 opacity-70",
        className,
      )}
    >
      <div className="flex items-start gap-2.5">
        {leading ? <div className="pt-0.5">{leading}</div> : null}
        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0">
              <p className="truncate text-[13px] font-semibold text-ink">{title}</p>
              {subtitle ? (
                <div className="mt-0.5 truncate text-[12px] text-ink-muted">
                  {subtitle}
                </div>
              ) : null}
            </div>
            {badge ? <div className="shrink-0">{badge}</div> : null}
          </div>

          {fields && fields.length > 0 ? (
            <dl className="mt-2 grid grid-cols-2 gap-x-3 gap-y-1.5">
              {fields.map((field) => (
                <div key={field.label} className="min-w-0">
                  <dt className="text-[10px] font-medium uppercase tracking-wide text-ink-faint">
                    {field.label}
                  </dt>
                  <dd className="mt-0.5 break-words text-[12px] text-ink-muted">
                    {field.value}
                  </dd>
                </div>
              ))}
            </dl>
          ) : null}

          {actions ? (
            <div className="mt-2.5 flex flex-wrap items-center gap-1.5">
              {actions}
            </div>
          ) : null}
        </div>
      </div>
    </article>
  );
}

/** Desktop table / mobile cards shell. */
export function ResponsiveList({
  desktop,
  mobile,
  className,
}: {
  readonly desktop: ReactNode;
  readonly mobile: ReactNode;
  readonly className?: string;
}) {
  return (
    <div className={className}>
      <div className="hidden md:block">{desktop}</div>
      <div className="md:hidden">{mobile}</div>
    </div>
  );
}
