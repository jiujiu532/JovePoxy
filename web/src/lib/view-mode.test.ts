import { describe, expect, it } from "vitest";
import { isViewMode, viewModeLabelKey } from "@/lib/view-mode";

describe("viewMode", () => {
  it("accepts known modes only", () => {
    expect(isViewMode("grid")).toBe(true);
    expect(isViewMode("compact")).toBe(true);
    expect(isViewMode("table")).toBe(true);
    expect(isViewMode("list")).toBe(false);
  });

  it("maps modes to i18n label keys", () => {
    expect(viewModeLabelKey("grid")).toBe("viewmode.grid");
    expect(viewModeLabelKey("compact")).toBe("viewmode.compact");
    expect(viewModeLabelKey("table")).toBe("viewmode.table");
  });
});
