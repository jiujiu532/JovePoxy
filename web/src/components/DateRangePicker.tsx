import { CalendarBlank, CaretLeft, CaretRight } from "@phosphor-icons/react";
import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { cn } from "@/lib/cn";

export type DateRangePreset = "today" | "7d" | "week" | "30d" | "month" | "custom";

export type DateRangeValue = {
  readonly from: Date;
  readonly to: Date;
  readonly preset: DateRangePreset;
};

export type DateRangeLabels = {
  readonly today: string;
  readonly last7d: string;
  readonly thisWeek: string;
  readonly last30d: string;
  readonly thisMonth: string;
  readonly apply: string;
  readonly clear: string;
  readonly start: string;
  readonly end: string;
  readonly placeholder: string;
};

function startOfDay(d: Date): Date {
  const x = new Date(d);
  x.setHours(0, 0, 0, 0);
  return x;
}

function endOfDay(d: Date): Date {
  const x = new Date(d);
  x.setHours(23, 59, 59, 999);
  return x;
}

function sameDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

function clampRange(from: Date, to: Date): { from: Date; to: Date } {
  const a = startOfDay(from);
  const b = endOfDay(to);
  if (a.getTime() <= b.getTime()) return { from: a, to: b };
  return { from: startOfDay(to), to: endOfDay(from) };
}

export function formatDateYMD(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

export function formatDateMD(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${m}/${day}`;
}

/** Inclusive day count between from/to (local calendar days). */
export function rangeDayCount(from: Date, to: Date): number {
  const a = startOfDay(from).getTime();
  const b = startOfDay(to).getTime();
  return Math.floor(Math.abs(b - a) / 86_400_000) + 1;
}

/** Build preset range relative to `now` (local). */
export function presetRange(preset: Exclude<DateRangePreset, "custom">, now = new Date()): DateRangeValue {
  const today = startOfDay(now);
  switch (preset) {
    case "today":
      return { from: today, to: endOfDay(now), preset };
    case "7d": {
      const from = new Date(today);
      from.setDate(today.getDate() - 6);
      return { from, to: endOfDay(now), preset };
    }
    case "week": {
      // Monday-start week
      const day = today.getDay(); // 0 Sun
      const offset = day === 0 ? 6 : day - 1;
      const from = new Date(today);
      from.setDate(today.getDate() - offset);
      return { from, to: endOfDay(now), preset };
    }
    case "30d": {
      const from = new Date(today);
      from.setDate(today.getDate() - 29);
      return { from, to: endOfDay(now), preset };
    }
    case "month": {
      const from = new Date(today.getFullYear(), today.getMonth(), 1);
      return { from, to: endOfDay(now), preset };
    }
    default:
      return { from: today, to: endOfDay(now), preset: "today" };
  }
}

function monthLabel(year: number, month: number, lang: "zh" | "en"): string {
  if (lang === "zh") return `${year}年${month + 1}月`;
  const names = [
    "Jan",
    "Feb",
    "Mar",
    "Apr",
    "May",
    "Jun",
    "Jul",
    "Aug",
    "Sep",
    "Oct",
    "Nov",
    "Dec",
  ];
  return `${names[month]} ${year}`;
}

function weekdayLabels(lang: "zh" | "en"): string[] {
  return lang === "zh"
    ? ["日", "一", "二", "三", "四", "五", "六"]
    : ["Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"];
}

type Cell = {
  readonly date: Date;
  readonly inMonth: boolean;
};

function buildMonthCells(year: number, month: number): Cell[] {
  const first = new Date(year, month, 1);
  const startPad = first.getDay();
  const daysInMonth = new Date(year, month + 1, 0).getDate();
  const cells: Cell[] = [];
  for (let i = 0; i < startPad; i += 1) {
    const d = new Date(year, month, 1 - (startPad - i));
    cells.push({ date: d, inMonth: false });
  }
  for (let day = 1; day <= daysInMonth; day += 1) {
    cells.push({ date: new Date(year, month, day), inMonth: true });
  }
  while (cells.length % 7 !== 0) {
    const last = cells[cells.length - 1]!.date;
    const d = new Date(last);
    d.setDate(last.getDate() + 1);
    cells.push({ date: d, inMonth: false });
  }
  return cells;
}

function MonthGrid({
  year,
  month,
  lang,
  draftFrom,
  draftTo,
  hover,
  onPick,
  onHover,
  onPrev,
  onNext,
  showPrev,
  showNext,
}: {
  readonly year: number;
  readonly month: number;
  readonly lang: "zh" | "en";
  readonly draftFrom: Date | null;
  readonly draftTo: Date | null;
  readonly hover: Date | null;
  readonly onPick: (d: Date) => void;
  readonly onHover: (d: Date | null) => void;
  readonly onPrev?: (() => void) | undefined;
  readonly onNext?: (() => void) | undefined;
  readonly showPrev: boolean;
  readonly showNext: boolean;
}) {
  const cells = useMemo(() => buildMonthCells(year, month), [year, month]);
  const weeks = weekdayLabels(lang);
  const today = startOfDay(new Date());

  const rangeStart = draftFrom ? startOfDay(draftFrom) : null;
  const rangeEnd = draftTo
    ? startOfDay(draftTo)
    : hover && rangeStart
      ? startOfDay(hover)
      : null;
  let selA = rangeStart;
  let selB = rangeEnd;
  if (selA && selB && selA.getTime() > selB.getTime()) {
    const t = selA;
    selA = selB;
    selB = t;
  }

  return (
    <div className="min-w-[240px] flex-1">
      <div className="mb-2 flex items-center justify-between gap-1">
        <button
          type="button"
          aria-label="prev-month"
          disabled={!showPrev || !onPrev}
          onClick={onPrev}
          className={cn(
            "inline-flex h-8 w-8 items-center justify-center border-2 border-transparent text-ink",
            showPrev
              ? "hover:border-border hover:bg-paper-2"
              : "opacity-0 pointer-events-none",
          )}
        >
          <CaretLeft size={14} weight="bold" />
        </button>
        <span className="font-mono text-[13px] font-semibold text-ink">
          {monthLabel(year, month, lang)}
        </span>
        <button
          type="button"
          aria-label="next-month"
          disabled={!showNext || !onNext}
          onClick={onNext}
          className={cn(
            "inline-flex h-8 w-8 items-center justify-center border-2 border-transparent text-ink",
            showNext
              ? "hover:border-border hover:bg-paper-2"
              : "opacity-0 pointer-events-none",
          )}
        >
          <CaretRight size={14} weight="bold" />
        </button>
      </div>
      <div className="mb-1 grid grid-cols-7 gap-0.5">
        {weeks.map((w) => (
          <div
            key={w}
            className="py-1 text-center font-mono text-[11px] font-medium text-ink-faint"
          >
            {w}
          </div>
        ))}
      </div>
      <div className="grid grid-cols-7 gap-0.5">
        {cells.map((cell) => {
          const day = startOfDay(cell.date);
          const isToday = sameDay(day, today);
          const isStart = selA ? sameDay(day, selA) : false;
          const isEnd = selB ? sameDay(day, selB) : false;
          const inRange =
            selA && selB
              ? day.getTime() >= selA.getTime() && day.getTime() <= selB.getTime()
              : false;
          const edge = isStart || isEnd;
          return (
            <button
              key={`${day.getTime()}-${cell.inMonth ? "m" : "o"}`}
              type="button"
              disabled={!cell.inMonth}
              onMouseEnter={() => onHover(day)}
              onMouseLeave={() => onHover(null)}
              onClick={() => onPick(day)}
              className={cn(
                "relative h-8 w-full font-mono text-[12px] tabular-nums transition-colors",
                !cell.inMonth && "pointer-events-none text-transparent",
                cell.inMonth && !edge && !inRange && "text-ink hover:bg-paper-2",
                cell.inMonth && inRange && !edge && "bg-accent-soft text-ink",
                edge && "bg-accent font-semibold text-accent-fg",
                isToday && cell.inMonth && !edge && "ring-1 ring-inset ring-border",
              )}
            >
              {cell.date.getDate()}
            </button>
          );
        })}
      </div>
    </div>
  );
}

export function DateRangePicker({
  value,
  onChange,
  labels,
  lang = "zh",
  className,
}: {
  readonly value: DateRangeValue;
  readonly onChange: (next: DateRangeValue) => void;
  readonly labels: DateRangeLabels;
  readonly lang?: "zh" | "en";
  readonly className?: string;
}) {
  const panelId = useId();
  const rootRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [draftFrom, setDraftFrom] = useState<Date | null>(value.from);
  const [draftTo, setDraftTo] = useState<Date | null>(value.to);
  const [hover, setHover] = useState<Date | null>(null);
  const [leftCursor, setLeftCursor] = useState(() => {
    const d = startOfDay(value.from);
    return { year: d.getFullYear(), month: d.getMonth() };
  });

  const rightCursor = useMemo(() => {
    const m = leftCursor.month + 1;
    if (m > 11) return { year: leftCursor.year + 1, month: 0 };
    return { year: leftCursor.year, month: m };
  }, [leftCursor]);

  useEffect(() => {
    if (!open) return;
    setDraftFrom(startOfDay(value.from));
    setDraftTo(startOfDay(value.to));
    const d = startOfDay(value.from);
    setLeftCursor({ year: d.getFullYear(), month: d.getMonth() });
  }, [open, value.from, value.to]);

  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const pick = useCallback(
    (day: Date) => {
      const d = startOfDay(day);
      if (!draftFrom || (draftFrom && draftTo)) {
        setDraftFrom(d);
        setDraftTo(null);
        return;
      }
      // second click
      if (d.getTime() < draftFrom.getTime()) {
        setDraftTo(draftFrom);
        setDraftFrom(d);
      } else {
        setDraftTo(d);
      }
    },
    [draftFrom, draftTo],
  );

  const applyDraft = useCallback(() => {
    if (!draftFrom) return;
    const to = draftTo ?? draftFrom;
    const clamped = clampRange(draftFrom, to);
    onChange({ ...clamped, preset: "custom" });
    setOpen(false);
  }, [draftFrom, draftTo, onChange]);

  const applyPreset = useCallback(
    (preset: Exclude<DateRangePreset, "custom">) => {
      onChange(presetRange(preset));
      setOpen(false);
    },
    [onChange],
  );

  const triggerText = `${formatDateYMD(value.from)}  ~  ${formatDateYMD(value.to)}`;

  const presets: Array<{ key: Exclude<DateRangePreset, "custom">; label: string }> = [
    { key: "today", label: labels.today },
    { key: "7d", label: labels.last7d },
    { key: "week", label: labels.thisWeek },
    { key: "30d", label: labels.last30d },
    { key: "month", label: labels.thisMonth },
  ];

  return (
    <div ref={rootRef} className={cn("relative", className)}>
      <button
        type="button"
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={() => setOpen((v) => !v)}
        className={cn(
          "inline-flex h-9 max-w-full items-center gap-2 border-2 border-border bg-paper-0 px-2.5",
          "font-mono text-[12px] text-ink shadow-[2px_2px_0_var(--border)]",
          "hover:bg-paper-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus-ring",
        )}
      >
        <CalendarBlank size={14} weight="bold" className="shrink-0" aria-hidden />
        <span className="truncate">{triggerText}</span>
      </button>

      {open ? (
        <div
          id={panelId}
          role="dialog"
          aria-label={labels.placeholder}
          className={cn(
            "absolute right-0 z-40 mt-2 w-[min(100vw-1.5rem,560px)] border-2 border-border bg-paper-0",
            "p-3 shadow-[4px_4px_0_var(--border)]",
          )}
        >
          <div className="flex flex-col gap-3 sm:flex-row">
            <MonthGrid
              year={leftCursor.year}
              month={leftCursor.month}
              lang={lang}
              draftFrom={draftFrom}
              draftTo={draftTo}
              hover={hover}
              onPick={pick}
              onHover={setHover}
              showPrev
              showNext={false}
              onPrev={() =>
                setLeftCursor((c) =>
                  c.month === 0
                    ? { year: c.year - 1, month: 11 }
                    : { year: c.year, month: c.month - 1 },
                )
              }
            />
            <div className="hidden w-px bg-border sm:block" aria-hidden />
            <MonthGrid
              year={rightCursor.year}
              month={rightCursor.month}
              lang={lang}
              draftFrom={draftFrom}
              draftTo={draftTo}
              hover={hover}
              onPick={pick}
              onHover={setHover}
              showPrev={false}
              showNext
              onNext={() =>
                setLeftCursor((c) =>
                  c.month === 11
                    ? { year: c.year + 1, month: 0 }
                    : { year: c.year, month: c.month + 1 },
                )
              }
            />
          </div>

          <div className="mt-3 flex flex-wrap items-center gap-2 border-t-2 border-border pt-3">
            <span className="font-mono text-[11px] text-ink-muted">
              {labels.start} {draftFrom ? formatDateYMD(draftFrom) : "—"}
            </span>
            <span className="text-ink-faint">→</span>
            <span className="font-mono text-[11px] text-ink-muted">
              {labels.end} {draftTo ? formatDateYMD(draftTo) : draftFrom ? formatDateYMD(draftFrom) : "—"}
            </span>
            <div className="ml-auto flex items-center gap-2">
              <button
                type="button"
                className="h-8 border-2 border-border bg-paper-0 px-3 text-[12px] font-medium text-ink hover:bg-paper-2"
                onClick={() => setOpen(false)}
              >
                {labels.clear}
              </button>
              <button
                type="button"
                disabled={!draftFrom}
                className={cn(
                  "h-8 border-2 border-border bg-accent px-3 text-[12px] font-semibold text-accent-fg",
                  "shadow-[2px_2px_0_var(--border)] hover:bg-accent-hover disabled:opacity-50",
                )}
                onClick={applyDraft}
              >
                {labels.apply}
              </button>
            </div>
          </div>

          <div className="mt-3 flex flex-wrap gap-1.5">
            {presets.map((p) => {
              const active = value.preset === p.key;
              return (
                <button
                  key={p.key}
                  type="button"
                  onClick={() => applyPreset(p.key)}
                  className={cn(
                    "h-8 border-2 px-2.5 text-[12px] font-medium",
                    active
                      ? "border-border bg-accent-yellow text-black shadow-[2px_2px_0_var(--border)]"
                      : "border-border bg-paper-0 text-ink hover:bg-paper-2",
                  )}
                >
                  {p.label}
                </button>
              );
            })}
          </div>
        </div>
      ) : null}
    </div>
  );
}

/** Small helper for section headers that only need a label node. */
export function DateRangeSummary({
  value,
  children,
}: {
  readonly value: DateRangeValue;
  readonly children?: ReactNode;
}) {
  return (
    <span className="font-mono text-[12px] text-ink-muted">
      {formatDateMD(value.from)} – {formatDateMD(value.to)}
      {children}
    </span>
  );
}
