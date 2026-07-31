import { describe, expect, it } from "vitest";
import {
  bucketKeyFor,
  buildBucketAxis,
  resolveBucketKind,
  startOfWeekMonday,
} from "./time-buckets";

function local(
  y: number,
  m: number,
  d: number,
  h = 0,
  min = 0,
  s = 0,
  ms = 0,
): Date {
  return new Date(y, m - 1, d, h, min, s, ms);
}

describe("resolveBucketKind", () => {
  it("uses 15m for ~2h span", () => {
    const from = local(2026, 7, 31, 10, 0);
    const to = local(2026, 7, 31, 12, 0);
    expect(resolveBucketKind(from, to)).toBe("15m");
  });

  it("uses 30m for ~6h span", () => {
    const from = local(2026, 7, 31, 8, 0);
    const to = local(2026, 7, 31, 14, 0);
    expect(resolveBucketKind(from, to)).toBe("30m");
  });

  it("uses 1h for today-like span (from midnight to afternoon)", () => {
    const from = local(2026, 7, 31, 0, 0);
    const to = local(2026, 7, 31, 15, 30);
    expect(resolveBucketKind(from, to)).toBe("1h");
  });

  it("uses 1d for ~7 day span", () => {
    const from = local(2026, 7, 25, 0, 0);
    const to = local(2026, 7, 31, 12, 0);
    expect(resolveBucketKind(from, to)).toBe("1d");
  });

  it("uses 1w for ~90 day span", () => {
    const from = local(2026, 5, 1, 0, 0);
    const to = local(2026, 7, 30, 23, 59);
    expect(resolveBucketKind(from, to)).toBe("1w");
  });

  it("uses 1M for multi-year / very long span", () => {
    const from = local(2024, 1, 1, 0, 0);
    const to = local(2026, 7, 31, 0, 0);
    expect(resolveBucketKind(from, to)).toBe("1M");
  });

  it("promotes when base kind would exceed 48 points", () => {
    // 47h → base 1h would be ~48 points; 49h still 1h base but check promote path
    // 3.5 days of hours would be 1d base (≤31d). Force: 40h is still 1h.
    // 49 hours is still ≤48h → 1h; 50h → 1d base.
    const from = local(2026, 7, 1, 0, 0);
    const to = local(2026, 7, 3, 2, 0); // ~50h
    expect(resolveBucketKind(from, to)).toBe("1d");
  });
});

describe("startOfWeekMonday", () => {
  it("aligns Sunday to previous Monday", () => {
    // 2026-07-26 is Sunday
    const mon = startOfWeekMonday(local(2026, 7, 26, 15));
    expect(mon.getFullYear()).toBe(2026);
    expect(mon.getMonth()).toBe(6);
    expect(mon.getDate()).toBe(20);
    expect(mon.getDay()).toBe(1);
  });

  it("keeps Monday as-is", () => {
    const mon = startOfWeekMonday(local(2026, 7, 20, 9));
    expect(mon.getDate()).toBe(20);
    expect(mon.getDay()).toBe(1);
  });
});

describe("bucketKeyFor + buildBucketAxis", () => {
  it("builds continuous 15m axis for 2h window", () => {
    const from = local(2026, 7, 31, 14, 5);
    const to = local(2026, 7, 31, 16, 0);
    const kind = resolveBucketKind(from, to);
    expect(kind).toBe("15m");
    const axis = buildBucketAxis(from, to, kind);
    expect(axis.length).toBeGreaterThan(1);
    expect(axis[0]!.key).toBe(bucketKeyFor(local(2026, 7, 31, 14, 0), "15m"));
    // keys unique and sorted
    const keys = axis.map((a) => a.key);
    expect(new Set(keys).size).toBe(keys.length);
    expect([...keys].sort()).toEqual(keys);
  });

  it("assigns events to same hour bucket", () => {
    const a = bucketKeyFor(local(2026, 7, 31, 9, 12), "1h");
    const b = bucketKeyFor(local(2026, 7, 31, 9, 59), "1h");
    const c = bucketKeyFor(local(2026, 7, 31, 10, 0), "1h");
    expect(a).toBe(b);
    expect(a).not.toBe(c);
  });

  it("day axis covers full 7 calendar days", () => {
    const from = local(2026, 7, 25, 0, 0);
    const to = local(2026, 7, 31, 18, 0);
    const axis = buildBucketAxis(from, to, "1d");
    expect(axis.length).toBe(7);
    expect(axis[0]!.label).toMatch(/07\/25/);
    expect(axis[axis.length - 1]!.label).toMatch(/07\/31/);
  });

  it("week axis for 90d has multiple week buckets", () => {
    const from = local(2026, 5, 1, 0, 0);
    const to = local(2026, 7, 30, 12, 0);
    const kind = resolveBucketKind(from, to);
    expect(kind).toBe("1w");
    const axis = buildBucketAxis(from, to, kind);
    expect(axis.length).toBeGreaterThan(8);
    expect(axis.length).toBeLessThanOrEqual(48);
  });

  it("includes empty buckets (axis length independent of data)", () => {
    const from = local(2026, 7, 31, 0, 0);
    const to = local(2026, 7, 31, 5, 0);
    const axis = buildBucketAxis(from, to, "1h");
    // 00:00 .. 05:00 inclusive → 6 points
    expect(axis.length).toBe(6);
  });

  it("same-day hour axis uses HH:mm labels without date prefix", () => {
    const from = local(2026, 7, 31, 0, 0);
    const to = local(2026, 7, 31, 23, 59);
    const axis = buildBucketAxis(from, to, "1h");
    expect(axis.length).toBeGreaterThan(12);
    expect(axis[0]!.label).toBe("00:00");
    expect(axis[15]!.label).toBe("15:00");
    for (const item of axis) {
      expect(item.label).toMatch(/^\d{2}:\d{2}$/);
    }
  });

  it("multi-day hour axis keeps date in label", () => {
    const from = local(2026, 7, 30, 20, 0);
    const to = local(2026, 7, 31, 8, 0);
    const axis = buildBucketAxis(from, to, "1h");
    expect(axis.some((a) => a.label.includes("/"))).toBe(true);
  });
});
