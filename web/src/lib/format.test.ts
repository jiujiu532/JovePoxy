import { describe, expect, it } from "vitest";
import {
  formatCooldownRemaining,
  formatDateTime,
  formatModelId,
  formatTrafficPct,
  maskSecret,
  parseApiTime,
  validatePasswordInput,
  zenKeyStatus,
} from "./format";
import { translate } from "./i18n";

const t = (key: Parameters<typeof translate>[1]) => translate("zh", key);

describe("parseApiTime", () => {
  it("parses fixed-width nano UTC", () => {
    const d = parseApiTime("2026-08-03T11:10:33.678043900Z");
    expect(d).not.toBeNull();
    expect(d!.toISOString()).toBe("2026-08-03T11:10:33.678Z");
  });

  it("returns null for empty or garbage", () => {
    expect(parseApiTime("")).toBeNull();
    expect(parseApiTime("   ")).toBeNull();
    expect(parseApiTime("not-a-date")).toBeNull();
  });
});

describe("formatDateTime", () => {
  it("renders local calendar time without T/Z/nanos", () => {
    const raw = "2026-08-03T11:10:33.678043900Z";
    const d = parseApiTime(raw)!;
    const expected = [
      d.getFullYear(),
      String(d.getMonth() + 1).padStart(2, "0"),
      String(d.getDate()).padStart(2, "0"),
    ].join("-") + " " + [
      String(d.getHours()).padStart(2, "0"),
      String(d.getMinutes()).padStart(2, "0"),
      String(d.getSeconds()).padStart(2, "0"),
    ].join(":");

    expect(formatDateTime(raw)).toBe(expected);
    expect(formatDateTime(raw)).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/);
  });

  it("falls back to raw or hyphen when unparseable", () => {
    expect(formatDateTime("not-a-date")).toBe("not-a-date");
    expect(formatDateTime("  ")).toBe("-");
  });
});

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

describe("formatTrafficPct", () => {
  it("formats zero and integers", () => {
    expect(formatTrafficPct(0)).toBe("0%");
    expect(formatTrafficPct(100)).toBe("100%");
    expect(formatTrafficPct(25)).toBe("25%");
  });

  it("keeps one decimal when needed", () => {
    expect(formatTrafficPct(33.3)).toBe("33.3%");
  });
});

describe("zenKeyStatus", () => {
  const now = Date.parse("2026-07-30T12:00:00.000Z");

  it("classifies disabled / active / cooling / benched", () => {
    expect(zenKeyStatus({ enabled: false }, now)).toBe("disabled");
    expect(zenKeyStatus({ enabled: true }, now)).toBe("active");
    expect(
      zenKeyStatus(
        { enabled: true, cooldown_until: "2026-07-30T12:01:00.000Z" },
        now,
      ),
    ).toBe("cooling");
    expect(
      zenKeyStatus(
        { enabled: true, cooldown_until: "2026-07-30T11:59:00.000Z" },
        now,
      ),
    ).toBe("active");
    expect(zenKeyStatus({ enabled: true, status: "benched" }, now)).toBe("benched");
  });
});

describe("formatCooldownRemaining", () => {
  const now = Date.parse("2026-07-30T12:00:00.000Z");

  it("returns null when not cooling", () => {
    expect(formatCooldownRemaining({}, now)).toBeNull();
    expect(
      formatCooldownRemaining(
        { cooldown_until: "2026-07-30T11:59:00.000Z" },
        now,
      ),
    ).toBeNull();
  });

  it("formats seconds and minutes", () => {
    expect(
      formatCooldownRemaining(
        { cooldown_until: "2026-07-30T12:00:45.000Z" },
        now,
      ),
    ).toBe("45s");
    expect(
      formatCooldownRemaining(
        { cooldown_until: "2026-07-30T12:02:05.000Z" },
        now,
      ),
    ).toBe("2m 5s");
  });
});
