import {
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { Badge } from "@/components/Badge";
import { ProgressBar, ProgressRing } from "@/components/ProgressRing";
import { viewModeGridClass } from "@/components/ViewModeToggle";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/cn";
import { assertNever } from "@/lib/assertNever";
import type { ViewMode } from "@/lib/view-mode";

export type QuotaWindowView = {
  readonly label: string;
  /** 0–100 usage percent for ring/bar */
  readonly percent: number;
  readonly primaryText: string;
  readonly hint?: string;
};

export type QuotaModelView = {
  readonly model: string;
  readonly requests: number;
};

export type QuotaAccountView = {
  readonly id: string;
  readonly name: string;
  readonly badge?: string;
  readonly subtitle?: string;
  readonly success: boolean;
  readonly error?: string;
  readonly windows: ReadonlyArray<QuotaWindowView>;
  readonly models?: ReadonlyArray<QuotaModelView>;
  readonly tableExtra?: ReactNode;
};

export type QuotaAccountViewsProps = {
  readonly mode: ViewMode;
  readonly items: ReadonlyArray<QuotaAccountView>;
  readonly tableHeaders?: ReadonlyArray<string>;
  readonly renderTableCells?: (item: QuotaAccountView) => ReactNode;
};

function modelsPreviewText(models: ReadonlyArray<QuotaModelView>, limit = 2): string {
  if (models.length === 0) return "-";
  const head = models
    .slice(0, limit)
    .map((m) => `${m.model}(${m.requests})`)
    .join(", ");
  if (models.length <= limit) return head;
  return `${head} +${models.length - limit}`;
}

type FloatPos = { readonly top: number; readonly left: number; readonly maxHeight: number };

/**
 * External floating model detail (portal + fixed).
 * Avoids clipping inside cards / table overflow ("外显" not "内显").
 */
function ModelsHover({
  models,
  children,
  className,
}: {
  readonly models: ReadonlyArray<QuotaModelView>;
  readonly children: ReactNode;
  readonly className?: string;
}) {
  const { t } = useI18n();
  const triggerRef = useRef<HTMLSpanElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const tooltipId = useId();
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<FloatPos | null>(null);
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clearCloseTimer = useCallback(() => {
    if (closeTimer.current) {
      clearTimeout(closeTimer.current);
      closeTimer.current = null;
    }
  }, []);

  const scheduleClose = useCallback(() => {
    clearCloseTimer();
    closeTimer.current = setTimeout(() => setOpen(false), 120);
  }, [clearCloseTimer]);

  const place = useCallback(() => {
    const el = triggerRef.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    const gap = 8;
    const panelW = Math.min(288, window.innerWidth - 16);
    const spaceBelow = window.innerHeight - rect.bottom - gap - 8;
    const spaceAbove = rect.top - gap - 8;
    const preferBelow = spaceBelow >= 160 || spaceBelow >= spaceAbove;
    const maxHeight = Math.max(120, Math.min(320, preferBelow ? spaceBelow : spaceAbove));
    let left = rect.left;
    if (left + panelW > window.innerWidth - 8) {
      left = window.innerWidth - 8 - panelW;
    }
    if (left < 8) left = 8;
    const top = preferBelow
      ? rect.bottom + gap
      : Math.max(8, rect.top - gap - maxHeight);
    setPos({ top, left, maxHeight });
  }, []);

  const openPanel = useCallback(() => {
    clearCloseTimer();
    place();
    setOpen(true);
  }, [clearCloseTimer, place]);

  useLayoutEffect(() => {
    if (!open) return;
    place();
  }, [open, place, models.length]);

  useEffect(() => {
    if (!open) return;
    const onScrollOrResize = () => place();
    window.addEventListener("scroll", onScrollOrResize, true);
    window.addEventListener("resize", onScrollOrResize);
    return () => {
      window.removeEventListener("scroll", onScrollOrResize, true);
      window.removeEventListener("resize", onScrollOrResize);
    };
  }, [open, place]);

  useEffect(() => () => clearCloseTimer(), [clearCloseTimer]);

  if (models.length === 0) {
    return <span className={className}>{children}</span>;
  }

  const panel =
    open && pos
      ? createPortal(
          <div
            ref={panelRef}
            id={tooltipId}
            role="tooltip"
            className={cn(
              "fixed z-[200] w-[min(18rem,calc(100vw-1rem))] border-2 border-border bg-paper-0",
              "p-3",
            )}
            style={{ top: pos.top, left: pos.left, maxHeight: pos.maxHeight }}
            onMouseEnter={openPanel}
            onMouseLeave={scheduleClose}
          >
            <p className="mb-2 text-[11px] font-medium text-ink-muted">
              {t("quotaviews.modelRequests", { count: models.length })}
            </p>
            <ul
              className="scrollbar-none space-y-1.5 overflow-y-auto"
              style={{ maxHeight: Math.max(80, pos.maxHeight - 40) }}
            >
              {models.map((m) => (
                <li
                  key={m.model}
                  className="flex items-start justify-between gap-3 text-[12px] text-ink-muted"
                >
                  <span className="min-w-0 break-all font-mono leading-snug text-ink">
                    {m.model}
                  </span>
                  <span className="shrink-0 tabular-nums text-ink-faint">{m.requests} req</span>
                </li>
              ))}
            </ul>
          </div>,
          document.body,
        )
      : null;

  return (
    <>
      <span
        ref={triggerRef}
        className={cn("inline-block max-w-full cursor-default", className)}
        aria-describedby={open ? tooltipId : undefined}
        onMouseEnter={openPanel}
        onMouseLeave={scheduleClose}
        onFocus={openPanel}
        onBlur={scheduleClose}
      >
        {children}
      </span>
      {panel}
    </>
  );
}

function ModelList({
  models,
  dense = false,
}: {
  readonly models: ReadonlyArray<QuotaModelView>;
  readonly dense?: boolean;
}) {
  const { t } = useI18n();
  if (models.length === 0) return null;
  const limit = dense ? 4 : 6;
  const visible = models.slice(0, limit);
  const hidden = models.length - visible.length;
  return (
    <ModelsHover models={models} className="mt-auto block w-full">
      <ul
        className={cn(
          "space-y-1 border-t border-border",
          dense ? "mt-2 pt-2" : "mt-3 pt-2.5",
        )}
      >
        {visible.map((m) => (
          <li
            key={m.model}
            className={cn(
              "flex items-center justify-between gap-2 text-ink-muted",
              dense ? "text-[11px]" : "text-[12px]",
            )}
          >
            <span className="min-w-0 truncate font-mono text-ink">{m.model}</span>
            <span className="shrink-0 tabular-nums">{m.requests} req</span>
          </li>
        ))}
        {hidden > 0 ? (
          <li className="text-[11px] font-medium text-accent">
            {t("quotaviews.moreModelsHover", { count: hidden })}
          </li>
        ) : null}
      </ul>
    </ModelsHover>
  );
}

function CardShell({
  item,
  children,
  dense = false,
}: {
  readonly item: QuotaAccountView;
  readonly children: ReactNode;
  readonly dense?: boolean;
}) {
  const { t } = useI18n();
  return (
    <article
      className={cn(
        "flex h-full flex-col border-2 border-border bg-paper-0",
        "transition-[border-color,box-shadow] duration-150 hover:border-border",
        dense ? "p-3" : "p-3.5 md:p-4",
      )}
    >
      <header className="mb-2.5 flex items-start justify-between gap-2">
        <div className="min-w-0">
          <h3
            className={cn(
              "truncate font-semibold tracking-tight text-ink",
              dense ? "text-[13px]" : "text-[14px]",
            )}
          >
            {item.name}
          </h3>
          {item.subtitle ? (
            <p className="mt-0.5 truncate font-mono text-[11px] text-ink-faint">{item.subtitle}</p>
          ) : null}
        </div>
        <div className="flex shrink-0 flex-wrap items-center justify-end gap-1">
          {item.badge ? <Badge kind="free">{item.badge}</Badge> : null}
          <Badge kind={item.success ? "healthy" : "error"}>
            {item.success ? t("quotaviews.success") : t("quotaviews.failed")}
          </Badge>
        </div>
      </header>
      {item.error ? (
        <p className="mb-2 border-2 border-border bg-paper-0 px-2.5 py-1.5 text-[12px] text-status-error">
          {item.error}
        </p>
      ) : null}
      {children}
    </article>
  );
}

function GridCard({ item }: { readonly item: QuotaAccountView }) {
  const { t } = useI18n();
  return (
    <CardShell item={item}>
      {item.windows.length > 0 ? (
        <div
          className={cn(
            "grid flex-1 gap-3",
            item.windows.length === 1
              ? "grid-cols-1 place-items-center"
              : item.windows.length === 2
                ? "grid-cols-2"
                : "grid-cols-3",
          )}
        >
          {item.windows.map((w) => (
            <ProgressRing
              key={w.label}
              percent={w.percent}
              label={w.label}
              valueText={w.primaryText}
              {...(w.hint !== undefined ? { hint: w.hint } : {})}
              size={item.windows.length >= 3 ? 72 : 88}
              strokeWidth={item.windows.length >= 3 ? 6 : 7}
            />
          ))}
        </div>
      ) : !item.error ? (
        <p className="text-sm text-ink-muted">{t("quotaviews.noWindows")}</p>
      ) : null}
      {item.models ? <ModelList models={item.models} /> : null}
    </CardShell>
  );
}

function CompactCard({ item }: { readonly item: QuotaAccountView }) {
  const { t } = useI18n();
  return (
    <CardShell item={item} dense>
      {item.windows.length > 0 ? (
        <div className="flex flex-col gap-2">
          {item.windows.map((w) => (
            <ProgressBar
              key={w.label}
              percent={w.percent}
              label={w.label}
              valueText={w.primaryText}
              {...(w.hint !== undefined ? { hint: w.hint } : {})}
              dense
            />
          ))}
        </div>
      ) : !item.error ? (
        <p className="text-[12px] text-ink-muted">{t("quotaviews.noWindowData")}</p>
      ) : null}
      {item.models ? <ModelList models={item.models} dense /> : null}
    </CardShell>
  );
}

function DefaultTableCells({ item }: { readonly item: QuotaAccountView }) {
  const { t } = useI18n();
  return (
    <>
      <td className="px-3 py-2.5 font-medium text-ink">{item.name}</td>
      <td className="px-3 py-2.5 text-ink-muted">
        {item.badge ? <Badge kind="free">{item.badge}</Badge> : item.subtitle ? (
          <span className="font-mono text-[12px]">{item.subtitle}</span>
        ) : (
          <span className="text-ink-faint">-</span>
        )}
      </td>
      {item.windows.map((w) => (
        <td key={w.label} className="px-3 py-2.5">
          <div className="min-w-[7rem]">
            <ProgressBar
              percent={w.percent}
              label={w.label}
              valueText={w.primaryText}
              {...(w.hint !== undefined ? { hint: w.hint } : {})}
              dense
            />
          </div>
        </td>
      ))}
      <td className="px-3 py-2.5">
        <Badge kind={item.success ? "healthy" : "error"}>
          {item.success ? t("quotaviews.success") : t("quotaviews.failed")}
        </Badge>
        {item.error ? (
          <p className="mt-1 max-w-[12rem] truncate text-[11px] text-status-error" title={item.error}>
            {item.error}
          </p>
        ) : null}
      </td>
      {item.models ? (
        <td className="px-3 py-2.5 text-[12px] text-ink-muted">
          {item.models.length === 0 ? (
            <span className="text-ink-faint">-</span>
          ) : (
            <ModelsHover models={item.models} className="max-w-[16rem]">
              <span className="block truncate tabular-nums text-ink-muted">
                {modelsPreviewText(item.models, 2)}
              </span>
            </ModelsHover>
          )}
        </td>
      ) : null}
      {item.tableExtra}
    </>
  );
}

function TableView({
  items,
  tableHeaders,
  renderTableCells,
}: {
  readonly items: ReadonlyArray<QuotaAccountView>;
  readonly tableHeaders?: ReadonlyArray<string>;
  readonly renderTableCells?: (item: QuotaAccountView) => ReactNode;
}) {
  const { t } = useI18n();
  const maxWindows = items.reduce((n, it) => Math.max(n, it.windows.length), 0);
  const windowLabels =
    items.find((it) => it.windows.length === maxWindows)?.windows.map((w) => w.label) ?? [];
  const hasModels = items.some((it) => it.models !== undefined);

  const headers =
    tableHeaders ??
    [
      t("quotaviews.colAccount"),
      t("quotaviews.colPlan"),
      ...windowLabels,
      t("quotaviews.colStatus"),
      ...(hasModels ? [t("quotaviews.colModel")] : []),
    ];

  return (
    <>
      {/* Mobile: compact cards — table columns do not fit phones */}
      <div className={`${viewModeGridClass("compact")} md:hidden`}>
        {items.map((item) => (
          <CompactCard key={item.id} item={item} />
        ))}
      </div>
      <div className="hidden border-2 border-border bg-paper-0 md:block">
        <div className="overflow-x-auto overflow-y-visible">
          <table className="w-full min-w-[40rem] text-left text-sm">
            <thead>
              <tr className="border-b-2 border-border bg-paper-0 text-caption text-ink">
                {headers.map((h) => (
                  <th key={h} className="whitespace-nowrap px-3 py-2.5 font-medium">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr
                  key={item.id}
                  className="border-b-2 border-border last:border-b-0 transition-colors hover:bg-accent-soft"
                >
                  {renderTableCells ? (
                    renderTableCells(item)
                  ) : (
                    <DefaultTableCells item={item} />
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </>
  );
}

export function QuotaAccountViews({
  mode,
  items,
  tableHeaders,
  renderTableCells,
}: QuotaAccountViewsProps) {
  switch (mode) {
    case "grid":
      return (
        <div className={viewModeGridClass("grid")}>
          {items.map((item) => (
            <GridCard key={item.id} item={item} />
          ))}
        </div>
      );
    case "compact":
      return (
        <div className={viewModeGridClass("compact")}>
          {items.map((item) => (
            <CompactCard key={item.id} item={item} />
          ))}
        </div>
      );
    case "table":
      return (
        <TableView
          items={items}
          {...(tableHeaders !== undefined ? { tableHeaders } : {})}
          {...(renderTableCells !== undefined ? { renderTableCells } : {})}
        />
      );
    default:
      return assertNever(mode);
  }
}
