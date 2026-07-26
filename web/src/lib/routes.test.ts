import { describe, expect, it } from "vitest";
import { NAV_ROUTES, isLogsHubTab, isProviderTab, pageTitleForPath } from "./routes";

describe("NAV_ROUTES", () => {
  it("reserves all operator destinations", () => {
    const ids = NAV_ROUTES.map((r) => r.id);
    expect(ids).toEqual([
      "overview",
      "models",
      "key-pool",
      "accounts",
      "quotas",
      "local-keys",
      "proxies",
      "logs",
      "settings",
    ]);
  });
});

describe("pageTitleForPath", () => {
  it("returns Chinese label for known path", () => {
    expect(pageTitleForPath("/app/local-keys")).toBe("分发管理");
    expect(pageTitleForPath("/app/accounts")).toBe("账号统计");
    expect(pageTitleForPath("/app/quotas")).toBe("额度监控");
    expect(pageTitleForPath("/app/logs")).toBe("请求日志");
  });

  it("falls back for unknown path", () => {
    expect(pageTitleForPath("/app/unknown")).toBe("管理台");
  });
});

describe("isProviderTab", () => {
  it("accepts opencode and ollama only", () => {
    expect(isProviderTab("opencode")).toBe(true);
    expect(isProviderTab("ollama")).toBe(true);
    expect(isProviderTab("other")).toBe(false);
    expect(isProviderTab(null)).toBe(false);
  });
});

describe("isLogsHubTab", () => {
  it("accepts gateway and usage tabs", () => {
    expect(isLogsHubTab("gateway")).toBe(true);
    expect(isLogsHubTab("usage-oc")).toBe(true);
    expect(isLogsHubTab("usage-ol")).toBe(true);
    expect(isLogsHubTab("usage")).toBe(false);
    expect(isLogsHubTab("other")).toBe(false);
  });
});
