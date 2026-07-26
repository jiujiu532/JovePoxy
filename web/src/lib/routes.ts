import type { MessageKey } from "@/lib/i18n/zh";

export const NAV_ROUTES = [
  { path: "/app/overview", labelKey: "nav.overview", id: "overview" },
  { path: "/app/models", labelKey: "nav.models", id: "models" },
  { path: "/app/key-pool", labelKey: "nav.keyPool", id: "key-pool" },
  { path: "/app/accounts", labelKey: "nav.accounts", id: "accounts" },
  { path: "/app/quotas", labelKey: "nav.quotas", id: "quotas" },
  { path: "/app/local-keys", labelKey: "nav.localKeys", id: "local-keys" },
  { path: "/app/proxies", labelKey: "nav.proxies", id: "proxies" },
  { path: "/app/logs", labelKey: "nav.logs", id: "logs" },
  { path: "/app/settings", labelKey: "nav.settings", id: "settings" },
] as const satisfies ReadonlyArray<{
  path: string;
  labelKey: MessageKey;
  id: string;
}>;

export type NavRouteId = (typeof NAV_ROUTES)[number]["id"];

export type ProviderTab = "opencode" | "ollama";

export function isProviderTab(value: string | null): value is ProviderTab {
  return value === "opencode" || value === "ollama";
}

/** Tabs inside 请求日志 hub */
export type LogsHubTab = "gateway" | "usage-oc" | "usage-ol";

export function isLogsHubTab(value: string | null): value is LogsHubTab {
  return value === "gateway" || value === "usage-oc" || value === "usage-ol";
}

/** 返回页面标题的 i18n key；由渲染处 t()。 */
export function pageTitleKeyForPath(pathname: string): MessageKey {
  const hit = NAV_ROUTES.find((r) => pathname.startsWith(r.path));
  return hit?.labelKey ?? "nav.console";
}
