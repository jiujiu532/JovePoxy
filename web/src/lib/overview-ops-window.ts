import type { DateRangeValue } from "@/components/DateRangePicker";
import type { OpsWindow } from "@/lib/api";

/**
 * Map UI date range onto backend rolling ops windows only when they match exactly.
 * today / week / custom / calendar-day ranges → undefined → client buildOpsFromLogs.
 * Backend windows are rolling (last 24h / last 7d from now), not calendar presets.
 */
export function opsWindowForRange(range: DateRangeValue): OpsWindow | undefined {
  if (
    range.preset === "today" ||
    range.preset === "week" ||
    range.preset === "month" ||
    range.preset === "custom"
  ) {
    return undefined;
  }
  if (range.preset === "7d") return "7d";
  if (range.preset === "30d") return undefined;
  return undefined;
}

/**
 * Nearest backend rolling window for routing KPIs when the page date range
 * is the only time control (no independent 1h/24h/7d segment).
 * Duration buckets: ≤90m → 1h, ≤36h → 24h, else → 7d (backend max).
 */
export function nearestOpsWindow(range: DateRangeValue): OpsWindow {
  const exact = opsWindowForRange(range);
  if (exact) return exact;
  const spanMs = Math.max(0, range.to.getTime() - range.from.getTime());
  const hour = 60 * 60 * 1000;
  if (spanMs <= 1.5 * hour) return "1h";
  if (spanMs <= 36 * hour) return "24h";
  return "7d";
}

/** Window query for overview: exact rolling match when possible, else nearest. */
export function overviewWindowForRange(range: DateRangeValue): OpsWindow {
  return opsWindowForRange(range) ?? nearestOpsWindow(range);
}
