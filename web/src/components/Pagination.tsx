import { CaretDown } from "@phosphor-icons/react";
import {
  useEffect,
  useId,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/cn";

/** Shared across every list table. */
export const DEFAULT_PAGE_SIZES = [5, 10, 20, 50, 100] as const;

function pageBtnClass(active = false, disabled = false) {
  return cn(
    "inline-flex h-8 min-w-8 shrink-0 items-center justify-center rounded-none",
    "border-2 border-border px-2.5 text-[12px] font-semibold tabular-nums",
    "transition-[transform,background-color,color,box-shadow] duration-150",
    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus-ring",
    disabled
      ? "cursor-not-allowed bg-paper-0 text-ink-faint opacity-45"
      : active
        ? "bg-accent-yellow text-black shadow-[2px_2px_0_var(--border)]"
        : "bg-paper-0 text-ink hover:bg-paper-1 active:translate-x-px active:translate-y-px active:shadow-none",
  );
}

function PageSizeSelect({
  value,
  options,
  onChange,
}: {
  readonly value: number;
  readonly options: ReadonlyArray<number>;
  readonly onChange: (size: number) => void;
}) {
  const { t } = useI18n();
  const [open, setOpen] = useState(false);
  const [coords, setCoords] = useState<{
    top: number;
    left: number;
    width: number;
    openUp: boolean;
  } | null>(null);
  const rootRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const listId = useId();

  const sizes = options.length > 0 ? [...options] : [...DEFAULT_PAGE_SIZES];
  const safeValue = sizes.includes(value)
    ? value
    : sizes.includes(20)
      ? 20
      : (sizes[0] ?? 20);

  function measure() {
    const el = rootRef.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    const menuH = Math.min(sizes.length * 34 + 8, 240);
    const spaceBelow = window.innerHeight - rect.bottom;
    const openUp = spaceBelow < menuH + 8 && rect.top > menuH;
    setCoords({
      top: openUp ? rect.top - 4 : rect.bottom + 4,
      left: rect.left,
      width: Math.max(rect.width, 88),
      openUp,
    });
  }

  useEffect(() => {
    if (!open) return;
    measure();
    function onDoc(event: MouseEvent) {
      const node = event.target as Node;
      if (rootRef.current?.contains(node) || listRef.current?.contains(node)) {
        return;
      }
      setOpen(false);
    }
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }
    function onReposition() {
      measure();
    }
    document.addEventListener("mousedown", onDoc);
    document.addEventListener("keydown", onKey);
    window.addEventListener("resize", onReposition);
    window.addEventListener("scroll", onReposition, true);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      document.removeEventListener("keydown", onKey);
      window.removeEventListener("resize", onReposition);
      window.removeEventListener("scroll", onReposition, true);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- measure uses sizes.length
  }, [open, sizes.length]);

  const menu =
    open && coords
      ? createPortal(
          <ul
            ref={listRef}
            id={listId}
            role="listbox"
            style={{
              position: "fixed",
              left: coords.left,
              width: coords.width,
              zIndex: 200,
              ...(coords.openUp
                ? { bottom: window.innerHeight - coords.top, top: "auto" }
                : { top: coords.top }),
            }}
            className={cn(
              "max-h-60 overflow-auto rounded-none border-2 border-border bg-paper-2 py-1",
              "shadow-[var(--shadow-hard)]",
            )}
          >
            {sizes.map((size) => {
              const active = size === safeValue;
              return (
                <li key={size} role="option" aria-selected={active}>
                  <button
                    type="button"
                    className={cn(
                      "flex w-full items-center px-3 py-1.5 text-left text-[12px] font-medium tabular-nums transition-colors",
                      active
                        ? "bg-accent-yellow text-black"
                        : "text-ink hover:bg-paper-0",
                    )}
                    onClick={() => {
                      onChange(size);
                      setOpen(false);
                    }}
                  >
                    {t("pagination.perPageOption", { n: size })}
                  </button>
                </li>
              );
            })}
          </ul>,
          document.body,
        )
      : null;

  return (
    <div className="inline-flex items-center gap-2">
      <span className="text-[12px] font-medium text-ink-muted">
        {t("pagination.perPage")}
      </span>
      <div ref={rootRef} className="relative inline-flex">
        <button
          type="button"
          aria-haspopup="listbox"
          aria-expanded={open}
          aria-controls={listId}
          aria-label={t("pagination.perPage")}
          className={cn(
            "inline-flex h-8 items-center gap-1.5 rounded-none border-2 border-border bg-paper-0 px-2.5",
            "text-[12px] font-semibold tabular-nums text-ink",
            "transition-[transform,background-color,box-shadow] duration-150",
            "hover:bg-paper-1 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus-ring",
            "active:translate-x-px active:translate-y-px",
            open && "bg-accent-yellow shadow-[2px_2px_0_var(--border)]",
          )}
          onClick={() => setOpen((v) => !v)}
        >
          <span>{t("pagination.perPageOption", { n: safeValue })}</span>
          <CaretDown
            size={12}
            weight="bold"
            className={cn(
              "shrink-0 transition-transform duration-150",
              open && "rotate-180",
            )}
            aria-hidden
          />
        </button>
        {menu}
      </div>
    </div>
  );
}

export type PaginationProps = {
  readonly total: number;
  readonly page: number;
  readonly pageSize: number;
  readonly onPageChange: (page: number) => void;
  readonly onPageSizeChange: (pageSize: number) => void;
  readonly pageSizeOptions?: ReadonlyArray<number>;
  readonly className?: string;
};

/** Build a compact page list with ellipsis gaps (1 … n-1 n n+1 … last). */
function pageWindow(current: number, totalPages: number): number[] {
  if (totalPages <= 7) {
    return Array.from({ length: totalPages }, (_, i) => i + 1);
  }
  const pages = new Set<number>([1, totalPages, current]);
  for (let delta = 1; delta <= 1; delta += 1) {
    pages.add(current - delta);
    pages.add(current + delta);
  }
  // Keep a denser head/tail when near edges (matches common table pagers).
  if (current <= 3) {
    pages.add(2);
    pages.add(3);
    pages.add(4);
  }
  if (current >= totalPages - 2) {
    pages.add(totalPages - 1);
    pages.add(totalPages - 2);
    pages.add(totalPages - 3);
  }
  return [...pages].filter((p) => p >= 1 && p <= totalPages).sort((a, b) => a - b);
}

function PageChip({
  children,
  active,
  disabled,
  onClick,
  ariaLabel,
  ariaCurrent,
}: {
  readonly children: ReactNode;
  readonly active?: boolean;
  readonly disabled?: boolean;
  readonly onClick?: () => void;
  readonly ariaLabel?: string;
  readonly ariaCurrent?: "page";
}) {
  return (
    <button
      type="button"
      className={pageBtnClass(active, disabled)}
      disabled={disabled}
      aria-label={ariaLabel}
      aria-current={ariaCurrent}
      onClick={onClick}
    >
      {children}
    </button>
  );
}

/**
 * Unified list pager — Neo-Brutalist hard-edge chrome.
 * Layout: [Prev] [1] [2] … [N] [Next]  summary          每页 [20 / 页 ▾]
 */
export function Pagination({
  total,
  page,
  pageSize,
  onPageChange,
  onPageSizeChange,
  pageSizeOptions = DEFAULT_PAGE_SIZES,
  className,
}: PaginationProps) {
  const { t } = useI18n();
  if (total <= 0) return null;

  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const safePage = Math.min(Math.max(1, page), totalPages);
  const pages = pageWindow(safePage, totalPages);

  return (
    <div
      className={cn(
        "flex flex-col gap-3 border-t-2 border-border px-3 py-3",
        "sm:flex-row sm:items-center sm:justify-between sm:gap-4",
        className,
      )}
    >
      <div className="flex min-w-0 flex-wrap items-center gap-2">
        <div className="inline-flex flex-wrap items-center gap-1.5">
          <PageChip
            disabled={safePage <= 1}
            ariaLabel={t("pagination.prev")}
            onClick={() => onPageChange(safePage - 1)}
          >
            {t("pagination.prev")}
          </PageChip>

          {pages.map((p, index) => {
            const prev = pages[index - 1];
            const showEllipsis = prev !== undefined && p - prev > 1;
            const active = p === safePage;
            return (
              <span key={p} className="inline-flex items-center gap-1.5">
                {showEllipsis ? (
                  <span
                    className="inline-flex h-8 min-w-6 items-center justify-center px-0.5 text-[12px] font-semibold text-ink-faint"
                    aria-hidden
                  >
                    …
                  </span>
                ) : null}
                <PageChip
                  active={active}
                  {...(active ? { ariaCurrent: "page" as const } : {})}
                  ariaLabel={t("pagination.goto", { page: p })}
                  onClick={() => onPageChange(p)}
                >
                  {p}
                </PageChip>
              </span>
            );
          })}

          <PageChip
            disabled={safePage >= totalPages}
            ariaLabel={t("pagination.next")}
            onClick={() => onPageChange(safePage + 1)}
          >
            {t("pagination.next")}
          </PageChip>
        </div>

        <p className="text-[12px] font-medium tabular-nums text-ink-muted">
          {t("pagination.summary", {
            page: safePage,
            pages: totalPages,
            total,
          })}
        </p>
      </div>

      <div className="flex shrink-0 items-center sm:justify-end">
        <PageSizeSelect
          value={pageSize}
          options={pageSizeOptions}
          onChange={(size) => {
            onPageSizeChange(size);
            // Reset to first page when page size changes so range stays valid.
            if (safePage !== 1) onPageChange(1);
          }}
        />
      </div>
    </div>
  );
}

export function slicePage<T>(
  items: ReadonlyArray<T>,
  page: number,
  pageSize: number,
): T[] {
  const start = (Math.max(1, page) - 1) * pageSize;
  return items.slice(start, start + pageSize);
}
