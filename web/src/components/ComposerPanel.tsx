import { X } from "@phosphor-icons/react";
import type { ReactNode } from "react";
import { Button } from "@/components/Button";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/cn";

export type ComposerPanelProps = {
  readonly title: string;
  readonly description?: string;
  readonly onClose?: () => void;
  readonly children: ReactNode;
  readonly footer?: ReactNode;
  readonly className?: string;
};

/**
 * Hard-edge insert form card used when "添加 / 发放" expands.
 * Keeps create UI distinct from data tables without soft chrome.
 */
export function ComposerPanel({
  title,
  description,
  onClose,
  children,
  footer,
  className,
}: ComposerPanelProps) {
  const { t } = useI18n();
  return (
    <section
      className={cn(
        "overflow-hidden rounded-none border-2 border-border bg-paper-1",
        "shadow-[var(--shadow-hard)]",
        className,
      )}
    >
      <header className="flex items-start justify-between gap-3 border-b-2 border-border px-4 py-3.5 sm:px-5">
        <div className="min-w-0">
          <h2 className="text-[14px] font-semibold tracking-tight text-ink">{title}</h2>
          {description ? (
            <p className="mt-0.5 text-[12px] leading-relaxed text-ink-muted">{description}</p>
          ) : null}
        </div>
        {onClose ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="!h-8 !w-8 !min-w-0 !px-0 shrink-0"
            aria-label={t("composer.collapse")}
            onClick={onClose}
          >
            <X size={16} weight="bold" />
          </Button>
        ) : null}
      </header>
      <div className="px-4 py-4 sm:px-5">{children}</div>
      {footer ? (
        <footer className="flex flex-wrap items-center justify-between gap-2 border-t-2 border-border bg-paper-0 px-4 py-3 sm:px-5">
          {footer}
        </footer>
      ) : null}
    </section>
  );
}

/** Number-ish field matching hard-edge inputs. */
export function CompactField({
  label,
  tip,
  children,
  className,
}: {
  readonly label: string;
  readonly tip?: ReactNode;
  readonly children: ReactNode;
  readonly className?: string;
}) {
  return (
    <div className={cn("min-w-0", className)}>
      <label className="mb-1.5 flex items-center gap-1 text-[12px] font-medium text-ink-muted">
        {label}
        {tip}
      </label>
      {children}
    </div>
  );
}

export const fieldInputClass =
  "h-10 w-full rounded-none border-2 border-border bg-paper-1 px-3 text-[13px] text-ink outline-none transition-[border-color,box-shadow] placeholder:text-ink-faint hover:border-border focus-visible:ring-2 focus-visible:ring-focus-ring focus-visible:ring-offset-1";
