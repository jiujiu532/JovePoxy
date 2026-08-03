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
