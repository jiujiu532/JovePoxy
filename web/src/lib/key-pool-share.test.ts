import { describe, expect, it } from "vitest";
import type { ZenKeyDTO } from "@/lib/api";
import {
  buildKeyPoolShare,
  DEFAULT_HEALTH_SCORE,
  effectiveHealthScore,
  effectiveSelectionScore,
  estimateKeySharePct,
  formatCount,
  formatErrorClass,
  formatHealthScore,
} from "@/lib/key-pool-share";

function key(partial: Partial<ZenKeyDTO> & Pick<ZenKeyDTO, "id" | "label">): ZenKeyDTO {
  return {
    prefix: partial.prefix ?? "sk-ab…",
    enabled: partial.enabled ?? true,
    ...partial,
  };
}

const NOW = Date.parse("2026-08-05T12:00:00.000Z");

describe("effectiveHealthScore / selection", () => {
  it("uses cold-start 70 when health missing", () => {
    expect(effectiveHealthScore({ enabled: true })).toBe(DEFAULT_HEALTH_SCORE);
    expect(effectiveSelectionScore({ enabled: true })).toBe(DEFAULT_HEALTH_SCORE);
  });

  it("clamps health to 0–100 and prefers selection_score", () => {
    expect(effectiveHealthScore({ enabled: true, health_score: 120 })).toBe(100);
    expect(effectiveHealthScore({ enabled: true, health_score: -3 })).toBe(0);
    expect(
      effectiveSelectionScore({ enabled: true, health_score: 80, selection_score: 55 }),
    ).toBe(55);
  });

  it("formats scores and counts without inventing secrets", () => {
    expect(formatHealthScore(undefined)).toBe("—");
    expect(formatHealthScore(70)).toBe("70");
    expect(formatHealthScore(33.3)).toBe("33.3");
    expect(formatCount(undefined)).toBe("—");
    expect(formatCount(0)).toBe("0");
    expect(formatCount(12.9)).toBe("12");
    expect(formatErrorClass("")).toBe("—");
    expect(formatErrorClass("rate_limited")).toBe("rate_limited");
  });
});

describe("buildKeyPoolShare", () => {
  it("returns empty slices when no eligible keys", () => {
    const summary = buildKeyPoolShare(
      [
        key({ id: "1", label: "off", enabled: false, health_score: 90 }),
        key({
          id: "2",
          label: "cool",
          status: "cooling",
          cooldown_until: "2026-08-05T12:05:00.000Z",
          health_score: 40,
        }),
        key({ id: "3", label: "bench", status: "benched", health_score: 10 }),
        key({ id: "4", label: "probe", status: "probing", health_score: 55 }),
      ],
      NOW,
    );
    expect(summary.eligibleCount).toBe(0);
    expect(summary.eligibleSelectionTotal).toBe(0);
    expect(summary.coolingCount).toBe(1);
    expect(summary.benchedCount).toBe(1);
    expect(summary.probingCount).toBe(1);
    // cooling + benched + probing
    expect(summary.attentionCount).toBe(3);
    expect(summary.slices).toEqual([]);
  });

  it("computes selection-score based dynamic shares that sum to ~100", () => {
    const summary = buildKeyPoolShare(
      [
        key({ id: "a", label: "primary", selection_score: 75 }),
        key({ id: "b", label: "backup", selection_score: 25 }),
      ],
      NOW,
    );
    expect(summary.eligibleCount).toBe(2);
    expect(summary.eligibleSelectionTotal).toBe(100);
    expect(summary.slices).toHaveLength(2);
    expect(summary.slices[0]!.sharePct).toBe(75);
    expect(summary.slices[1]!.sharePct).toBe(25);
    const total = summary.slices.reduce((s, x) => s + x.sharePct, 0);
    expect(total).toBeCloseTo(100, 5);
  });

  it("prefers server traffic_pct when present", () => {
    const summary = buildKeyPoolShare(
      [
        key({ id: "a", label: "a", selection_score: 1, traffic_pct: 60 }),
        key({ id: "b", label: "b", selection_score: 1, traffic_pct: 40 }),
      ],
      NOW,
    );
    expect(summary.slices.map((s) => s.sharePct)).toEqual([60, 40]);
  });

  it("excludes cooling, benched, and probing from eligible share bar", () => {
    const summary = buildKeyPoolShare(
      [
        key({ id: "ok", label: "ok", selection_score: 50 }),
        key({
          id: "cool",
          label: "cool",
          status: "cooling",
          cooldown_until: "2026-08-05T12:10:00.000Z",
          selection_score: 90,
        }),
        key({ id: "bench", label: "bench", status: "benched", selection_score: 90 }),
        key({ id: "probe", label: "probe", status: "probing", selection_score: 90 }),
      ],
      NOW,
    );
    expect(summary.eligibleCount).toBe(1);
    expect(summary.coolingCount).toBe(1);
    expect(summary.benchedCount).toBe(1);
    expect(summary.probingCount).toBe(1);
    // cooling + benched + probing
    expect(summary.attentionCount).toBe(3);
    expect(summary.slices).toHaveLength(1);
    expect(summary.slices[0]!.id).toBe("ok");
    expect(summary.slices[0]!.sharePct).toBe(100);
    expect(summary.slices[0]).not.toHaveProperty("secret");
    expect(summary.slices[0]).not.toHaveProperty("weight");
  });

  it("orders slices by share descending and uses cold-start when scores missing", () => {
    const summary = buildKeyPoolShare(
      [
        key({ id: "low", label: "low", selection_score: 10 }),
        key({ id: "high", label: "high", selection_score: 40 }),
        key({ id: "mid", label: "mid" }), // cold start 70 → highest share
      ],
      NOW,
    );
    expect(summary.slices.map((s) => s.id)).toEqual(["mid", "high", "low"]);
    expect(summary.slices[0]!.healthScore).toBe(DEFAULT_HEALTH_SCORE);
  });

  it("does not use legacy weight for share", () => {
    const summary = buildKeyPoolShare(
      [
        key({ id: "heavy", label: "heavy", weight: 99, selection_score: 10 }),
        key({ id: "light", label: "light", weight: 1, selection_score: 90 }),
      ],
      NOW,
    );
    expect(summary.slices[0]!.id).toBe("light");
    expect(summary.slices[0]!.sharePct).toBe(90);
  });

  it("estimateKeySharePct returns null for non-active keys", () => {
    const keys = [
      key({ id: "ok", label: "ok", selection_score: 50 }),
      key({ id: "off", label: "off", enabled: false, selection_score: 90 }),
    ];
    expect(estimateKeySharePct(keys[0]!, keys, NOW)).toBe(100);
    expect(estimateKeySharePct(keys[1]!, keys, NOW)).toBeNull();
  });

  it("handles zero scores and empty pool without fabricating rates", () => {
    expect(buildKeyPoolShare([], NOW).slices).toEqual([]);
    const summary = buildKeyPoolShare(
      [key({ id: "z", label: "z", selection_score: 0, health_score: 0 })],
      NOW,
    );
    // selection score floor is 1 so a single active key still gets 100% estimate
    expect(summary.eligibleCount).toBe(1);
    expect(summary.slices[0]!.sharePct).toBe(100);
  });
});
