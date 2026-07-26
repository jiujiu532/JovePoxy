export const NAV_ROUTES = [
  { path: "/app/overview", label: "概览", id: "overview" },
  { path: "/app/models", label: "模型", id: "models" },
  { path: "/app/key-pool", label: "密钥池", id: "key-pool" },
  { path: "/app/accounts", label: "账号统计", id: "accounts" },
  { path: "/app/quotas", label: "额度监控", id: "quotas" },
  { path: "/app/local-keys", label: "分发管理", id: "local-keys" },
  { path: "/app/proxies", label: "出口代理池", id: "proxies" },
  { path: "/app/logs", label: "请求日志", id: "logs" },
  { path: "/app/settings", label: "设置", id: "settings" },
] as const;

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

export function pageTitleForPath(pathname: string): string {
  const hit = NAV_ROUTES.find((r) => pathname.startsWith(r.path));
  return hit?.label ?? "管理台";
}
