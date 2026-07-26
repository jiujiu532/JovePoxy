import { CaretLeft, CaretRight } from "@phosphor-icons/react";
import { FilterSelect } from "@/components/ListSelectBar";
import { cn } from "@/lib/cn";

/** Shared across every list table. */
export const DEFAULT_PAGE_SIZES = [5, 10, 20, 50, 100] as const;

function PageSizeSelect({
  value,
  options,
  onChange,
}: {
  readonly value: number;
  readonly options: ReadonlyArray<number>;
  readonly onChange: (size: number) => void;
}) {
  // Prefer opening upward — pagination sits at card bottom and was clipped.
  const sizes = options.length > 0 ? [...options] : [...DEFAULT_PAGE_SIZES];
  const safeValue = sizes.includes(value)
    ? value
    : sizes.includes(20)
      ? 20
      : (sizes[0] ?? 20);

  return (
    <FilterSelect
      label="每页"
      value={String(safeValue)}
      onChange={(v) => onChange(Number(v))}
      placement="top"
      options={sizes.map((size) => ({
        value: String(size),
        label: String(size),
      }))}
    />
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

function pageWindow(current: number, totalPages: number): number[] {
  if (totalPages <= 7) {
    return Array.from({ length: totalPages }, (_, i) => i + 1);
  }
  const pages = new Set<number>([1, totalPages, current]);
  for (let delta = 1; delta <= 2; delta += 1) {
    pages.add(current - delta);
    pages.add(current + delta);
  }
  return [...pages].filter((p) => p >= 1 && p <= totalPages).sort((a, b) => a - b);
}

export function Pagination({
  total,
  page,
  pageSize,
  onPageChange,
  onPageSizeChange,
  pageSizeOptions = DEFAULT_PAGE_SIZES,
  className,
}: PaginationProps) {
  if (total <= 0) return null;

  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const safePage = Math.min(Math.max(1, page), totalPages);
  const from = (safePage - 1) * pageSize + 1;
  const to = Math.min(total, safePage * pageSize);
  const pages = pageWindow(safePage, totalPages);

  return (
    <div
      className={cn(
        "flex flex-col gap-2 border-t border-border px-3 py-2.5 text-[12px] text-ink-muted sm:flex-row sm:items-center sm:justify-between",
        className,
      )}
    >
      <p>
        显示第 {from} 条 - 第 {to} 条，共 {total} 条
      </p>
      <div className="flex flex-wrap items-center gap-2">
        <span className="tabular-nums">总页数: {totalPages}</span>
        <div className="inline-flex items-center gap-0.5">
          <button
            type="button"
            className="inline-flex h-8 w-8 items-center justify-center rounded-none border border-border bg-paper-0 text-ink-muted transition-colors hover:border-border-strong hover:text-ink disabled:opacity-40"
            disabled={safePage <= 1}
            aria-label="上一页"
            onClick={() => onPageChange(safePage - 1)}
          >
            <CaretLeft size={14} weight="bold" />
          </button>
          {pages.map((p, index) => {
            const prev = pages[index - 1];
            const showEllipsis = prev !== undefined && p - prev > 1;
            return (
              <span key={p} className="inline-flex items-center">
                {showEllipsis ? <span className="px-1 text-ink-faint">...</span> : null}
                <button
                  type="button"
                  className={cn(
                    "inline-flex h-8 min-w-8 items-center justify-center rounded-none px-2 tabular-nums transition-colors",
                    p === safePage
                      ? "bg-accent text-accent-fg"
                      : "border border-border bg-paper-0 text-ink-muted hover:border-border-strong hover:text-ink",
                  )}
                  aria-current={p === safePage ? "page" : undefined}
                  onClick={() => onPageChange(p)}
                >
                  {p}
                </button>
              </span>
            );
          })}
          <button
            type="button"
            className="inline-flex h-8 w-8 items-center justify-center rounded-none border border-border bg-paper-0 text-ink-muted transition-colors hover:border-border-strong hover:text-ink disabled:opacity-40"
            disabled={safePage >= totalPages}
            aria-label="下一页"
            onClick={() => onPageChange(safePage + 1)}
          >
            <CaretRight size={14} weight="bold" />
          </button>
        </div>
        <PageSizeSelect
          value={pageSize}
          options={pageSizeOptions}
          onChange={onPageSizeChange}
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
