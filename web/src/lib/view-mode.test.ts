import { describe, expect, it } from "vitest";
import { isViewMode, viewModeLabel } from "@/lib/view-mode";

describe("viewMode", () => {
  it("accepts known modes only", () => {
    expect(isViewMode("grid")).toBe(true);
    expect(isViewMode("compact")).toBe(true);
    expect(isViewMode("table")).toBe(true);
    expect(isViewMode("list")).toBe(false);
  });

  it("labels modes in Chinese", () => {
    expect(viewModeLabel("grid")).toBe("网格视图");
    expect(viewModeLabel("compact")).toBe("紧凑视图");
    expect(viewModeLabel("table")).toBe("表格视图");
  });
});
