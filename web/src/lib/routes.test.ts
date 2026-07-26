import { describe, expect, it } from "vitest";
import { NAV_ROUTES, isLogsHubTab, isProviderTab, pageTitleKeyForPath } from "./routes";

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

describe("pageTitleKeyForPath", () => {
  it("maps known paths to nav label keys", () => {
    expect(pageTitleKeyForPath("/app/local-keys")).toBe("nav.localKeys");
    expect(pageTitleKeyForPath("/app/accounts")).toBe("nav.accounts");
    expect(pageTitleKeyForPath("/app/quotas")).toBe("nav.quotas");
    expect(pageTitleKeyForPath("/app/logs")).toBe("nav.logs");
  });

  it("falls back to console key for unknown path", () => {
    expect(pageTitleKeyForPath("/app/unknown")).toBe("nav.console");
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
