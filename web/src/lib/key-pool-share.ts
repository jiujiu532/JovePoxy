import type { ZenKeyDTO, ZenKeyStatus } from "@/lib/api";
import { zenKeyStatus } from "@/lib/format";

/** Cold-start health when backend has not yet emitted health fields. */
export const DEFAULT_HEALTH_SCORE = 70;

/** Hard-edge palette for in-pool dynamic share segments (design tokens only). */
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
  readonly healthScore: number;
  readonly selectionScore: number;
  /** Estimated dynamic share within eligible set, 0–100. */
  readonly sharePct: number;
  readonly status: ZenKeyStatus;
  readonly color: string;
};

export type KeyPoolShareSummary = {
  readonly eligibleCount: number;
  readonly probingCount: number;
  readonly coolingCount: number;
  readonly benchedCount: number;
  readonly attentionCount: number;
  /** Sum of selection scores used for estimate (active only). */
  readonly eligibleSelectionTotal: number;
  /** Eligible (active) keys only, ordered by share desc then label. */
  readonly slices: readonly KeyPoolShareSlice[];
};

/** Clamp numeric score-like fields; invalid → null. */
export function readOptionalScore(value: number | undefined | null): number | null {
  if (value == null || Number.isNaN(value) || !Number.isFinite(value)) return null;
  return value;
}

/** Display health score: server value or cold-start 70 for enabled keys; null when disabled/unknown. */
export function effectiveHealthScore(key: Pick<ZenKeyDTO, "health_score" | "enabled">): number {
  const raw = readOptionalScore(key.health_score);
  if (raw != null) return Math.max(0, Math.min(100, raw));
  return DEFAULT_HEALTH_SCORE;
}

/**
 * Selection score for dynamic share estimates.
 * Prefers server selection_score; falls back to health_score / cold-start.
 * Non-schedulable keys should not call this for share (filtered by status).
 */
export function effectiveSelectionScore(
  key: Pick<ZenKeyDTO, "selection_score" | "health_score" | "enabled">,
): number {
  const sel = readOptionalScore(key.selection_score);
  if (sel != null) return Math.max(1, sel);
  return Math.max(1, Math.round(effectiveHealthScore(key)));
}

/** Format a 0–100 score for table cells (one decimal only when needed). */
export function formatHealthScore(score: number | undefined | null): string {
  const raw = readOptionalScore(score);
  if (raw == null) return "—";
  const clamped = Math.max(0, Math.min(100, raw));
  const rounded = Math.round(clamped * 10) / 10;
  if (Number.isInteger(rounded)) return String(rounded);
  return rounded.toFixed(1);
}

/** Format success/failure counts without inventing values. */
export function formatCount(value: number | undefined | null): string {
  if (value == null || Number.isNaN(value) || !Number.isFinite(value)) return "—";
  return String(Math.max(0, Math.floor(value)));
}

/** Human-readable last error class; empty → dash. Never shows secrets. */
export function formatErrorClass(value: string | undefined | null): string {
  const trimmed = (value ?? "").trim();
  return trimmed.length > 0 ? trimmed : "—";
}

/**
 * Build current-provider-tab estimated dynamic share for eligible keys.
 * Eligible = active (enabled, not cooling/benched/disabled). Probing is tracked separately
 * and does not split normal share with active keys.
 * Prefers server `traffic_pct` when present; otherwise selection_score / eligible total.
 * Does not represent historical request distribution or cross-provider routing.
 * Never includes secret-bearing fields.
 */
export function buildKeyPoolShare(
  keys: readonly ZenKeyDTO[],
  nowMs = Date.now(),
): KeyPoolShareSummary {
  let coolingCount = 0;
  let benchedCount = 0;
  let probingCount = 0;
  const eligible: ZenKeyDTO[] = [];

  for (const key of keys) {
    const status = zenKeyStatus(key, nowMs);
    if (status === "cooling") coolingCount += 1;
    if (status === "benched") benchedCount += 1;
    if (status === "probing") probingCount += 1;
    if (status === "active") {
      eligible.push(key);
    }
  }

  // Align with backend: attention = cooling + benched + probing.
  const attentionCount = coolingCount + benchedCount + probingCount;

  const scored = eligible.map((key) => ({
    key,
    selectionScore: effectiveSelectionScore(key),
  }));
  const eligibleSelectionTotal = scored.reduce((sum, row) => sum + row.selectionScore, 0);

  const slices: KeyPoolShareSlice[] = scored
    .map((row, index) => {
      const fromServer =
        typeof row.key.traffic_pct === "number" && Number.isFinite(row.key.traffic_pct)
          ? Math.max(0, row.key.traffic_pct)
          : null;
      const sharePct =
        fromServer != null
          ? fromServer
          : eligibleSelectionTotal > 0
            ? (row.selectionScore / eligibleSelectionTotal) * 100
            : 0;
      return {
        id: row.key.id,
        label: row.key.label,
        prefix: row.key.prefix,
        healthScore: effectiveHealthScore(row.key),
        selectionScore: row.selectionScore,
        sharePct,
        status: "active" as const,
        color: keyPoolShareColor(index),
      };
    })
    .sort((a, b) => {
      if (b.sharePct !== a.sharePct) return b.sharePct - a.sharePct;
      return a.label.localeCompare(b.label);
    })
    .map((slice, index) => ({
      ...slice,
      color: keyPoolShareColor(index),
    }));

  return {
    eligibleCount: eligible.length,
    probingCount,
    coolingCount,
    benchedCount,
    attentionCount,
    eligibleSelectionTotal,
    slices,
  };
}

/**
 * Estimated dynamic share label for a single row (table/mobile).
 * Returns null when the key is not in the active eligible set.
 */
export function estimateKeySharePct(
  key: ZenKeyDTO,
  keys: readonly ZenKeyDTO[],
  nowMs = Date.now(),
): number | null {
  const status = zenKeyStatus(key, nowMs);
  if (status !== "active") return null;
  if (typeof key.traffic_pct === "number" && Number.isFinite(key.traffic_pct)) {
    return Math.max(0, key.traffic_pct);
  }
  const summary = buildKeyPoolShare(keys, nowMs);
  const hit = summary.slices.find((s) => s.id === key.id);
  return hit ? hit.sharePct : null;
}
