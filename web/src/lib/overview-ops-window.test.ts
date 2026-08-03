import { describe, expect, it } from "vitest";
import { presetRange } from "@/components/DateRangePicker";
import { opsWindowForRange } from "@/lib/overview-ops-window";

describe("opsWindowForRange", () => {
  it("uses backend 7d only for rolling 7d preset", () => {
    expect(opsWindowForRange(presetRange("7d"))).toBe("7d");
  });

  it("does not map today/week/custom/month/30d to backend rolling windows", () => {
    expect(opsWindowForRange(presetRange("today"))).toBeUndefined();
    expect(opsWindowForRange(presetRange("week"))).toBeUndefined();
    expect(opsWindowForRange(presetRange("month"))).toBeUndefined();
    expect(opsWindowForRange(presetRange("30d"))).toBeUndefined();
    expect(
      opsWindowForRange({
        from: new Date("2026-01-01T00:00:00"),
        to: new Date("2026-01-03T23:59:59"),
        preset: "custom",
      }),
    ).toBeUndefined();
  });
});
