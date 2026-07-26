import { describe, expect, it } from "vitest";
import { formatModelId, maskSecret, validatePasswordInput } from "./format";
import { translate } from "./i18n";

const t = (key: Parameters<typeof translate>[1]) => translate("zh", key);

describe("formatModelId", () => {
  it("returns hyphen when empty", () => {
    expect(formatModelId("   ")).toBe("-");
  });

  it("returns trimmed id", () => {
    expect(formatModelId("  gpt-free  ")).toBe("gpt-free");
  });
});

describe("maskSecret", () => {
  it("masks after visible prefix", () => {
    expect(maskSecret("sk-oc-abcdefghijklmnop", 8)).toBe("sk-oc-ab************");
  });

  it("returns empty for empty input", () => {
    expect(maskSecret("")).toBe("");
  });
});

describe("validatePasswordInput", () => {
  it("rejects empty password", () => {
    expect(validatePasswordInput("", t)).toBe("请输入管理员密码");
  });

  it("rejects short password", () => {
    expect(validatePasswordInput("ab", t)).toBe("密码至少 4 位");
  });

  it("accepts valid password", () => {
    expect(validatePasswordInput("admin", t)).toBeNull();
  });
});
