/** Adaptive time buckets for overview model analytics (local timezone). */

export type BucketKind = "15m" | "30m" | "1h" | "1d" | "1w" | "1M";

const MS_MIN = 60_000;
const MS_HOUR = 60 * MS_MIN;
const MS_DAY = 24 * MS_HOUR;

/** Soft cap on axis points; promote to coarser kind when exceeded. */
export const MAX_BUCKET_POINTS = 48;

const KIND_ORDER: readonly BucketKind[] = [
  "15m",
  "30m",
  "1h",
  "1d",
  "1w",
  "1M",
];

function startOfLocalDay(d: Date): Date {
  const x = new Date(d);
  x.setHours(0, 0, 0, 0);
  return x;
}

/** Monday-start week (local), matching DateRangePicker week preset. */
export function startOfWeekMonday(d: Date): Date {
  const day = startOfLocalDay(d);
  const dow = day.getDay(); // 0 Sun
  const offset = dow === 0 ? 6 : dow - 1;
  day.setDate(day.getDate() - offset);
  return day;
}

function startOfMonth(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), 1);
}

function floorToMinutes(d: Date, stepMin: number): Date {
  const x = new Date(d);
  const totalMin = x.getHours() * 60 + x.getMinutes();
  const floored = Math.floor(totalMin / stepMin) * stepMin;
  x.setHours(Math.floor(floored / 60), floored % 60, 0, 0);
  return x;
}

function pad2(n: number): string {
  return String(n).padStart(2, "0");
}

function formatHM(d: Date): string {
  return `${pad2(d.getHours())}:${pad2(d.getMinutes())}`;
}

function formatYMD(d: Date): string {
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
}

/** Stable sort/identity key for a bucket start (local wall time). */
export function bucketKeyFor(date: Date, kind: BucketKind): string {
  const t = new Date(date);
  if (Number.isNaN(t.getTime())) return "";

  switch (kind) {
    case "15m": {
      const f = floorToMinutes(t, 15);
      return `${formatYMD(f)}T${formatHM(f)}`;
    }
    case "30m": {
      const f = floorToMinutes(t, 30);
      return `${formatYMD(f)}T${formatHM(f)}`;
    }
    case "1h": {
      const f = floorToMinutes(t, 60);
      return `${formatYMD(f)}T${formatHM(f)}`;
    }
    case "1d":
      return formatYMD(startOfLocalDay(t));
    case "1w":
      return `W:${formatYMD(startOfWeekMonday(t))}`;
    case "1M": {
      const m = startOfMonth(t);
      return `${m.getFullYear()}-${pad2(m.getMonth() + 1)}`;
    }
    default:
      return formatYMD(startOfLocalDay(t));
  }
}

export type BucketLabelOpts = {
  /** When true (same local calendar day), hour/minute axes use HH:mm only. */
  readonly sameDay?: boolean;
};

/** Display label for axis / tooltip (local). */
export function bucketLabel(
  key: string,
  kind: BucketKind,
  opts: BucketLabelOpts = {},
): string {
  if (!key) return key;
  switch (kind) {
    case "15m":
    case "30m":
    case "1h": {
      // key: YYYY-MM-DDTHH:mm
      const [datePart, timePart] = key.split("T");
      if (!datePart || !timePart) return key;
      const [, m, d] = datePart.split("-");
      // Single calendar day: short clock only so ~24 hourly ticks don't pile up.
      if (opts.sameDay || kind === "15m" || kind === "30m") {
        return timePart;
      }
      // Multi-day hour axis: compact MM/DD HH (drop :00 noise when on the hour)
      const hm = timePart.endsWith(":00") ? timePart.slice(0, 2) : timePart;
      return `${m}/${d} ${hm}`;
    }
    case "1d": {
      const [, m, d] = key.split("-");
      return m && d ? `${m}/${d}` : key;
    }
    case "1w": {
      // W:YYYY-MM-DD
      const ymd = key.startsWith("W:") ? key.slice(2) : key;
      const [, m, d] = ymd.split("-");
      return m && d ? `${m}/${d}` : key;
    }
    case "1M":
      return key;
    default:
      return key;
  }
}

function estimatePointCount(from: Date, to: Date, kind: BucketKind): number {
  const span = Math.max(0, to.getTime() - from.getTime());
  switch (kind) {
    case "15m":
      return Math.ceil(span / (15 * MS_MIN)) + 1;
    case "30m":
      return Math.ceil(span / (30 * MS_MIN)) + 1;
    case "1h":
      return Math.ceil(span / MS_HOUR) + 1;
    case "1d":
      return Math.ceil(span / MS_DAY) + 1;
    case "1w":
      return Math.ceil(span / (7 * MS_DAY)) + 1;
    case "1M": {
      const months =
        (to.getFullYear() - from.getFullYear()) * 12 +
        (to.getMonth() - from.getMonth()) +
        1;
      return Math.max(1, months);
    }
    default:
      return 1;
  }
}

function baseKindForSpan(spanMs: number): BucketKind {
  if (spanMs <= 3 * MS_HOUR) return "15m";
  if (spanMs <= 12 * MS_HOUR) return "30m";
  if (spanMs <= 48 * MS_HOUR) return "1h";
  if (spanMs <= 31 * MS_DAY) return "1d";
  if (spanMs <= 180 * MS_DAY) return "1w";
  return "1M";
}

function promote(kind: BucketKind): BucketKind | null {
  const i = KIND_ORDER.indexOf(kind);
  if (i < 0 || i >= KIND_ORDER.length - 1) return null;
  return KIND_ORDER[i + 1]!;
}

/**
 * Choose bucket width from span; promote coarser while estimated points > 48.
 */
export function resolveBucketKind(from: Date, to: Date): BucketKind {
  const a = from.getTime() <= to.getTime() ? from : to;
  const b = from.getTime() <= to.getTime() ? to : from;
  const span = Math.max(0, b.getTime() - a.getTime());
  let kind = baseKindForSpan(span);
  while (estimatePointCount(a, b, kind) > MAX_BUCKET_POINTS) {
    const next = promote(kind);
    if (!next) break;
    kind = next;
  }
  return kind;
}

function nextBucketStart(d: Date, kind: BucketKind): Date {
  const x = new Date(d);
  switch (kind) {
    case "15m":
      x.setMinutes(x.getMinutes() + 15);
      return x;
    case "30m":
      x.setMinutes(x.getMinutes() + 30);
      return x;
    case "1h":
      x.setHours(x.getHours() + 1);
      return x;
    case "1d":
      x.setDate(x.getDate() + 1);
      return x;
    case "1w":
      x.setDate(x.getDate() + 7);
      return x;
    case "1M":
      x.setMonth(x.getMonth() + 1);
      return x;
    default:
      x.setDate(x.getDate() + 1);
      return x;
  }
}

function alignBucketStart(from: Date, kind: BucketKind): Date {
  switch (kind) {
    case "15m":
      return floorToMinutes(from, 15);
    case "30m":
      return floorToMinutes(from, 30);
    case "1h":
      return floorToMinutes(from, 60);
    case "1d":
      return startOfLocalDay(from);
    case "1w":
      return startOfWeekMonday(from);
    case "1M":
      return startOfMonth(from);
    default:
      return startOfLocalDay(from);
  }
}

export type BucketAxisItem = {
  /** Stable key for aggregation (bucketKeyFor). */
  readonly key: string;
  /** Axis / tooltip label. */
  readonly label: string;
};

/**
 * Ordered continuous axis covering [from, to] (inclusive of buckets that
 * intersect the range). Empty buckets are included so the axis stays continuous.
 */
function sameLocalCalendarDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

export function buildBucketAxis(
  from: Date,
  to: Date,
  kind: BucketKind,
): BucketAxisItem[] {
  const a = from.getTime() <= to.getTime() ? from : to;
  const b = from.getTime() <= to.getTime() ? to : from;
  const endMs = b.getTime();
  const labelOpts: BucketLabelOpts = {
    sameDay: sameLocalCalendarDay(a, b),
  };

  let cursor = alignBucketStart(a, kind);
  // If alignment went before `from` for week/month, still include that bucket
  // when it intersects [from,to]; start walking from it.
  const items: BucketAxisItem[] = [];
  let guard = 0;
  const maxGuard = MAX_BUCKET_POINTS + 8;

  while (cursor.getTime() <= endMs && guard < maxGuard) {
    const key = bucketKeyFor(cursor, kind);
    items.push({ key, label: bucketLabel(key, kind, labelOpts) });
    cursor = nextBucketStart(cursor, kind);
    guard += 1;
  }

  // Ensure last partial bucket that contains `to` is present even if cursor
  // alignment skipped edge cases (e.g. to exactly on boundary already covered).
  if (items.length === 0) {
    const key = bucketKeyFor(a, kind);
    items.push({ key, label: bucketLabel(key, kind, labelOpts) });
  }

  return items;
}

/** i18n message key for bucket hint subtitle. */
export function bucketHintKey(kind: BucketKind): `overview.bucket.${BucketKind}` {
  return `overview.bucket.${kind}`;
}

/** Convenience: resolve kind + axis in one call. */
export function resolveBuckets(from: Date, to: Date): {
  kind: BucketKind;
  axis: BucketAxisItem[];
} {
  const kind = resolveBucketKind(from, to);
  return { kind, axis: buildBucketAxis(from, to, kind) };
}
