import type { ZenKeyDTO, ZenKeyStatus } from "@/lib/api";
import { zenKeyStatus } from "@/lib/format";

/** Hard-edge palette for in-pool theoretical share segments (design tokens only). */
export const KEY_POOL_SHARE_COLORS = [
  "var(--accent-teal)",
  "var(--accent-yellow)",
  "var(--accent)",
  "var(--accent-mint)",
  "var(--accent-coral)",
] as const;

export function keyPoolShareColor(index: number): string {
  return KEY_POOL_SHARE_COLORS[index % KEY_POOL_SHARE_COLORS.length]!;
}

export type KeyPoolShareSlice = {
  readonly id: string;
  readonly label: string;
  readonly prefix: string;
  readonly weight: number;
  /** Theoretical share within eligible set, 0–100. */
  readonly sharePct: number;
  readonly status: ZenKeyStatus;
  readonly color: string;
};

export type KeyPoolShareSummary = {
  readonly eligibleCount: number;
  readonly coolingCount: number;
  readonly benchedCount: number;
  readonly eligibleWeight: number;
  /** Eligible keys only, ordered by share desc then label. */
  readonly slices: readonly KeyPoolShareSlice[];
};

/**
 * Build current-provider-tab theoretical weighted share for eligible keys.
 * Eligible = active (enabled, not cooling/benched) and weight > 0.
 * Prefers server `traffic_pct` when present; otherwise weight / eligibleWeight.
 * Does not represent historical request distribution or cross-provider routing.
 */
export function buildKeyPoolShare(
  keys: readonly ZenKeyDTO[],
  nowMs = Date.now(),
): KeyPoolShareSummary {
  let coolingCount = 0;
  let benchedCount = 0;
  const eligible: ZenKeyDTO[] = [];

  for (const key of keys) {
    const status = zenKeyStatus(key, nowMs);
    if (status === "cooling") coolingCount += 1;
    if (status === "benched") benchedCount += 1;
    if (status === "active" && key.weight > 0) {
      eligible.push(key);
    }
  }

  const eligibleWeight = eligible.reduce((sum, k) => sum + k.weight, 0);

  const slices: KeyPoolShareSlice[] = eligible
    .map((key, index) => {
      const fromServer =
        typeof key.traffic_pct === "number" && Number.isFinite(key.traffic_pct)
          ? Math.max(0, key.traffic_pct)
          : null;
      const sharePct =
        fromServer != null
          ? fromServer
          : eligibleWeight > 0
            ? (key.weight / eligibleWeight) * 100
            : 0;
      return {
        id: key.id,
        label: key.label,
        prefix: key.prefix,
        weight: key.weight,
        sharePct,
        status: "active" as const,
        color: keyPoolShareColor(index),
      };
    })
    .sort((a, b) => {
      if (b.sharePct !== a.sharePct) return b.sharePct - a.sharePct;
      return a.label.localeCompare(b.label);
    })
    // re-assign colors after sort so legend order matches bar order
    .map((slice, index) => ({
      ...slice,
      color: keyPoolShareColor(index),
    }));

  return {
    eligibleCount: eligible.length,
    coolingCount,
    benchedCount,
    eligibleWeight,
    slices,
  };
}
