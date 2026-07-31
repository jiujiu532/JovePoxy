import { CaretDown, MagnifyingGlass } from "@phosphor-icons/react";
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

export type SegmentOption = {
  readonly value: string;
  readonly label: string;
};

/** Dense hard-edge segment control. */
export function SegmentedFilter({
  value,
  onChange,
  options,
  className,
  "aria-label": ariaLabel,
}: {
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly options: ReadonlyArray<SegmentOption>;
  readonly className?: string;
  readonly "aria-label"?: string;
}) {
  return (
    <div
      role="group"
      aria-label={ariaLabel}
      className={cn(
        "inline-flex h-8 flex-wrap items-center gap-0.5 rounded-none border-2 border-border bg-paper-1 p-0.5",
        className,
      )}
    >
      {options.map((opt) => {
        const active = value === opt.value;
        return (
          <button
            key={opt.value}
            type="button"
            aria-pressed={active}
            className={cn(
              "h-7 shrink-0 rounded-none px-2.5 text-[12px] font-medium transition-[background-color,color,box-shadow] duration-150",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus-ring",
              active
                ? "bg-accent text-accent-fg shadow-[2px_2px_0_var(--border)]"
                : "text-ink-muted hover:bg-paper-2 hover:text-ink",
            )}
            onClick={() => onChange(opt.value)}
          >
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}

/** Custom dropdown panel (no native select chrome). Portal avoids overflow clipping. */
export function FilterSelect({
  label,
  value,
  onChange,
  options,
  className,
  placement = "auto",
}: {
  readonly label: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly options: ReadonlyArray<SegmentOption>;
  readonly className?: string;
  /** auto = flip above when near bottom of viewport. */
  readonly placement?: "auto" | "top" | "bottom";
}) {
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
  const current = options.find((o) => o.value === value)?.label ?? value;

  function measure() {
    const el = rootRef.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    const menuH = Math.min(options.length * 32 + 8, 240);
    const spaceBelow = window.innerHeight - rect.bottom;
    const openUp =
      placement === "top" ||
      (placement === "auto" && spaceBelow < menuH + 8 && rect.top > menuH);
    setCoords({
      top: openUp ? rect.top - 4 : rect.bottom + 4,
      left: rect.left,
      width: Math.max(rect.width, 96),
      openUp,
    });
  }

  useEffect(() => {
    if (!open) return;
    measure();
    function onDoc(event: MouseEvent) {
      const t = event.target as Node;
      if (rootRef.current?.contains(t) || listRef.current?.contains(t)) return;
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
    // eslint-disable-next-line react-hooks/exhaustive-deps -- measure uses options.length
  }, [open, options.length, placement]);

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
            {options.map((opt) => {
              const active = opt.value === value;
              return (
                <li key={opt.value} role="option" aria-selected={active}>
                  <button
                    type="button"
                    className={cn(
                      "flex w-full items-center px-3 py-1.5 text-left text-[12px] transition-colors",
                      active
                        ? "bg-accent font-medium text-accent-fg"
                        : "text-ink-muted hover:bg-paper-0 hover:text-ink",
                    )}
                    onClick={() => {
                      onChange(opt.value);
                      setOpen(false);
                    }}
                  >
                    {opt.label}
                  </button>
                </li>
              );
            })}
          </ul>,
          document.body,
        )
      : null;

  return (
    <div ref={rootRef} className={cn("relative inline-flex", className)}>
      <button
        type="button"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={listId}
        className={cn(
          "inline-flex h-8 items-center gap-1.5 rounded-none border-2 border-border bg-paper-1 pl-2.5 pr-2",
          "text-[12px] font-medium text-ink transition-colors",
          "hover:bg-paper-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus-ring",
          open && "ring-2 ring-focus-ring",
        )}
        onClick={() => setOpen((v) => !v)}
      >
        <span className="text-[11px] font-normal text-ink-faint">{label}</span>
        <span className="max-w-[7.5rem] truncate">{current}</span>
        <CaretDown
          size={12}
          weight="bold"
          className={cn(
            "shrink-0 text-ink-faint transition-transform duration-150",
            open && "rotate-180",
          )}
          aria-hidden
        />
      </button>
      {menu}
    </div>
  );
}

export function SearchField({
  value,
  onChange,
  placeholder,
  className,
}: {
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly placeholder?: string;
  readonly className?: string;
}) {
  const { t } = useI18n();
  return (
    <label className={cn("relative w-full min-w-0 sm:w-56", className)}>
      <MagnifyingGlass
        size={14}
        className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-ink-faint"
        aria-hidden
      />
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder ?? t("common.search")}
        className={cn(
          "h-8 w-full rounded-none border-2 border-border bg-paper-1 pl-8 pr-2.5 text-[12px] text-ink",
          "placeholder:text-ink-faint outline-none transition-[border-color,box-shadow] duration-150",
          "hover:bg-paper-0 focus-visible:ring-2 focus-visible:ring-focus-ring",
        )}
      />
    </label>
  );
}

export type ListToolbarProps = {
  readonly search: string;
  readonly onSearchChange: (value: string) => void;
  readonly searchPlaceholder?: string;
  /** Primary filters (status chips, etc.) — sits next to search. */
  readonly filters?: ReactNode;
  /** Sort / reset / page tools — right side of row 1. */
  readonly trailing?: ReactNode;
  readonly selectedCount: number;
  readonly totalVisible: number;
  readonly allSelected: boolean;
  readonly onSelectAll: () => void;
  readonly onInvert: () => void;
  readonly onClear: () => void;
  readonly bulkActions?: ReactNode;
  /** Hide selection row entirely (e.g. read-only lists). */
  readonly hideSelection?: boolean;
  readonly className?: string;
};

/**
 * Ordered toolbar:
 *  1) [search + filters] ........ [trailing]
 *  2) selection strip — only when items are checked (hard full-width bar)
 */
export function ListToolbar({
  search,
  onSearchChange,
  searchPlaceholder,
  filters,
  trailing,
  selectedCount,
  totalVisible,
  allSelected,
  onSelectAll,
  onInvert,
  onClear,
  bulkActions,
  hideSelection = false,
  className,
}: ListToolbarProps) {
  return (
    <div
      className={cn(
        "flex flex-col gap-2 border-b-2 border-border bg-paper-2 px-3 py-2.5",
        className,
      )}
    >
      <div className="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
        <div className="flex min-w-0 flex-wrap items-center gap-1.5">
          <SearchField
            value={search}
            onChange={onSearchChange}
            {...(searchPlaceholder !== undefined
              ? { placeholder: searchPlaceholder }
              : {})}
          />
          {filters}
        </div>
        {trailing ? (
          <div className="flex flex-wrap items-center gap-1.5 lg:justify-end">
            {trailing}
          </div>
        ) : null}
      </div>

      {!hideSelection && selectedCount > 0 ? (
        <SelectionStrip
          selectedCount={selectedCount}
          totalVisible={totalVisible}
          allSelected={allSelected}
          onSelectAll={onSelectAll}
          onInvert={onInvert}
          onClear={onClear}
          bulkActions={bulkActions}
        />
      ) : null}
    </div>
  );
}

/** Hard full-width bar: left meta/links · right bulk actions. */
export function SelectionStrip({
  selectedCount,
  totalVisible,
  allSelected,
  onSelectAll,
  onInvert,
  onClear,
  bulkActions,
  className,
}: {
  readonly selectedCount: number;
  readonly totalVisible: number;
  readonly allSelected: boolean;
  readonly onSelectAll: () => void;
  readonly onInvert: () => void;
  readonly onClear: () => void;
  readonly bulkActions?: ReactNode;
  readonly className?: string;
}) {
  const { t } = useI18n();
  return (
    <div
      role="status"
      aria-live="polite"
      className={cn(
        "flex flex-col gap-2 rounded-none border-2 border-border sm:flex-row sm:items-center sm:justify-between",
        "bg-accent-soft px-3 py-2 shadow-[3px_3px_0_var(--border)]",
        className,
      )}
    >
      <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
        <span className="inline-flex items-center gap-1.5 text-[13px] font-medium text-ink">
          <span
            className="inline-flex h-5 min-w-5 items-center justify-center rounded-none border-2 border-border bg-accent px-1.5 text-[11px] font-semibold tabular-nums text-accent-fg"
            aria-hidden
          >
            {selectedCount}
          </span>
          {t("listselect.selected")}
          <span className="font-normal text-ink-faint">
            {t("listselect.ofPage", { total: totalVisible })}
          </span>
        </span>
        <span className="hidden h-3.5 w-px bg-ink sm:inline-block" aria-hidden />
        <div className="flex flex-wrap items-center gap-x-2.5 text-[12px]">
          <button
            type="button"
            className="font-medium text-ink-muted transition-colors hover:text-ink"
            onClick={onSelectAll}
          >
            {allSelected ? t("listselect.deselectAll") : t("listselect.selectAll")}
          </button>
          <button
            type="button"
            className="font-medium text-ink-muted transition-colors hover:text-ink"
            onClick={onInvert}
          >
            {t("listselect.invert")}
          </button>
          <button
            type="button"
            className="font-medium text-ink-faint transition-colors hover:text-ink"
            onClick={onClear}
          >
            {t("listselect.clear")}
          </button>
        </div>
      </div>
      {bulkActions ? (
        <div className="flex flex-wrap items-center gap-1.5 sm:justify-end">
          {bulkActions}
        </div>
      ) : null}
    </div>
  );
}

/** Lightweight filter strip for read-only lists (logs / models). */
export function FilterStrip({
  search,
  onSearchChange,
  searchPlaceholder,
  filters,
  trailing,
  className,
}: {
  readonly search: string;
  readonly onSearchChange: (value: string) => void;
  readonly searchPlaceholder?: string;
  readonly filters?: ReactNode;
  readonly trailing?: ReactNode;
  readonly className?: string;
}) {
  return (
    <div
      className={cn(
        "flex flex-col gap-2 border-b-2 border-border bg-paper-2 px-3 py-2.5 sm:flex-row sm:items-center sm:justify-between",
        className,
      )}
    >
      <div className="flex min-w-0 flex-wrap items-center gap-1.5">
        <SearchField
          value={search}
          onChange={onSearchChange}
          {...(searchPlaceholder !== undefined
            ? { placeholder: searchPlaceholder }
            : {})}
        />
        {filters}
      </div>
      {trailing ? (
        <div className="flex flex-wrap items-center gap-1.5 sm:justify-end">
          {trailing}
        </div>
      ) : null}
    </div>
  );
}

/** @deprecated */
export function ListSelectBar({
  selectedCount,
  totalVisible,
  allSelected,
  onSelectAll,
  onInvert,
  onClear,
  children,
  className,
}: {
  readonly selectedCount: number;
  readonly totalVisible: number;
  readonly allSelected: boolean;
  readonly onSelectAll: () => void;
  readonly onInvert: () => void;
  readonly onClear: () => void;
  readonly children?: ReactNode;
  readonly className?: string;
}) {
  return (
    <ListToolbar
      search=""
      onSearchChange={() => undefined}
      selectedCount={selectedCount}
      totalVisible={totalVisible}
      allSelected={allSelected}
      onSelectAll={onSelectAll}
      onInvert={onInvert}
      onClear={onClear}
      {...(children !== undefined ? { bulkActions: children } : {})}
      {...(className !== undefined ? { className } : {})}
    />
  );
}
