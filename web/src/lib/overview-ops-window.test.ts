import { describe, expect, it } from "vitest";
import { presetRange } from "@/components/DateRangePicker";
import {
  nearestOpsWindow,
  opsWindowForRange,
  overviewWindowForRange,
} from "@/lib/overview-ops-window";

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

describe("nearestOpsWindow", () => {
  it("keeps exact 7d rolling preset", () => {
    expect(nearestOpsWindow(presetRange("7d"))).toBe("7d");
  });

  it("maps short custom span to 1h, day-ish to 24h, multi-day to 7d", () => {
    expect(
      nearestOpsWindow({
        from: new Date("2026-08-06T10:00:00"),
        to: new Date("2026-08-06T11:00:00"),
        preset: "custom",
      }),
    ).toBe("1h");
    expect(
      nearestOpsWindow({
        from: new Date("2026-08-05T12:00:00"),
        to: new Date("2026-08-06T12:00:00"),
        preset: "custom",
      }),
    ).toBe("24h");
    expect(
      nearestOpsWindow({
        from: new Date("2026-07-31T00:00:00"),
        to: new Date("2026-08-06T23:59:59"),
        preset: "custom",
      }),
    ).toBe("7d");
  });
});

describe("overviewWindowForRange", () => {
  it("prefers exact rolling match then nearest", () => {
    expect(overviewWindowForRange(presetRange("7d"))).toBe("7d");
    expect(overviewWindowForRange(presetRange("today"))).toBe(
      nearestOpsWindow(presetRange("today")),
    );
  });
});
