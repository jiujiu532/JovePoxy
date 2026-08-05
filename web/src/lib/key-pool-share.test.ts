import { describe, expect, it } from "vitest";
import type { ZenKeyDTO } from "@/lib/api";
import { buildKeyPoolShare } from "@/lib/key-pool-share";

function key(partial: Partial<ZenKeyDTO> & Pick<ZenKeyDTO, "id" | "label">): ZenKeyDTO {
  return {
    prefix: partial.prefix ?? "sk-ab…",
    weight: partial.weight ?? 1,
    enabled: partial.enabled ?? true,
    ...partial,
  };
}

const NOW = Date.parse("2026-08-05T12:00:00.000Z");

describe("buildKeyPoolShare", () => {
  it("returns empty slices when no eligible keys", () => {
    const summary = buildKeyPoolShare(
      [
        key({ id: "1", label: "off", enabled: false, weight: 3 }),
        key({
          id: "2",
          label: "cool",
          status: "cooling",
          cooldown_until: "2026-08-05T12:05:00.000Z",
          weight: 5,
        }),
        key({ id: "3", label: "bench", status: "benched", weight: 2 }),
        key({ id: "4", label: "zero", weight: 0 }),
      ],
      NOW,
    );
    expect(summary.eligibleCount).toBe(0);
    expect(summary.eligibleWeight).toBe(0);
    expect(summary.coolingCount).toBe(1);
    expect(summary.benchedCount).toBe(1);
    expect(summary.slices).toEqual([]);
  });

  it("computes weight-based theoretical shares that sum to ~100", () => {
    const summary = buildKeyPoolShare(
      [
        key({ id: "a", label: "primary", weight: 3 }),
        key({ id: "b", label: "backup", weight: 1 }),
      ],
      NOW,
    );
    expect(summary.eligibleCount).toBe(2);
    expect(summary.eligibleWeight).toBe(4);
    expect(summary.slices).toHaveLength(2);
    expect(summary.slices[0]!.sharePct).toBe(75);
    expect(summary.slices[1]!.sharePct).toBe(25);
    const total = summary.slices.reduce((s, x) => s + x.sharePct, 0);
    expect(total).toBeCloseTo(100, 5);
  });

  it("prefers server traffic_pct when present", () => {
    const summary = buildKeyPoolShare(
      [
        key({ id: "a", label: "a", weight: 1, traffic_pct: 60 }),
        key({ id: "b", label: "b", weight: 1, traffic_pct: 40 }),
      ],
      NOW,
    );
    expect(summary.slices.map((s) => s.sharePct)).toEqual([60, 40]);
  });

  it("excludes cooling and benched from eligible share bar", () => {
    const summary = buildKeyPoolShare(
      [
        key({ id: "ok", label: "ok", weight: 2 }),
        key({
          id: "cool",
          label: "cool",
          status: "cooling",
          cooldown_until: "2026-08-05T12:10:00.000Z",
          weight: 9,
        }),
        key({ id: "bench", label: "bench", status: "benched", weight: 9 }),
      ],
      NOW,
    );
    expect(summary.eligibleCount).toBe(1);
    expect(summary.coolingCount).toBe(1);
    expect(summary.benchedCount).toBe(1);
    expect(summary.slices).toHaveLength(1);
    expect(summary.slices[0]!.id).toBe("ok");
    expect(summary.slices[0]!.sharePct).toBe(100);
    // never exposes secret fields
    expect(summary.slices[0]).not.toHaveProperty("secret");
  });

  it("orders slices by share descending", () => {
    const summary = buildKeyPoolShare(
      [
        key({ id: "low", label: "low", weight: 1 }),
        key({ id: "high", label: "high", weight: 4 }),
        key({ id: "mid", label: "mid", weight: 2 }),
      ],
      NOW,
    );
    expect(summary.slices.map((s) => s.id)).toEqual(["high", "mid", "low"]);
  });
});
