export class ApiError extends Error {
  readonly status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export type ModelProvider = "opencode" | "ollama";

export type ModelDTO = {
  readonly id: string;
  readonly free: boolean;
  /** Primary chat route. Missing on older backends → treat as opencode. */
  readonly provider?: ModelProvider;
  /** All sources advertising this id (OpenCode Go ∩ Ollama overlap). */
  readonly providers?: ReadonlyArray<ModelProvider>;
  /** Ordered reasoning_effort labels accepted after gateway clamp. */
  readonly effort_levels?: ReadonlyArray<string>;
  /** Upstream may emit cache counters that the gateway logs. */
  readonly cache_usage?: boolean;
};
export type LocalKeyDTO = {
  readonly id: string;
  readonly label: string;
  readonly prefix: string;
  readonly enabled: boolean;
  readonly revoked: boolean;
  readonly rpm_limit: number;
  readonly daily_limit: number;
};
export type LocalKeyCreatedDTO = {
  readonly id: string;
  readonly prefix: string;
  readonly secret: string;
};
export type KeyProvider = "opencode" | "ollama";

export type ZenKeyStatus = "active" | "cooling" | "benched" | "disabled" | "probing";

export type ZenPoolProviderSummaryDTO = {
  readonly total: number;
  readonly enabled: number;
  readonly healthy: number;
  readonly cooled: number;
  readonly benched?: number;
  readonly disabled: number;
  /** Keys currently in controlled probe (0 or 1 concurrent per key). */
  readonly probing?: number;
  /** Keys that need attention (cooling / benched / probing). */
  readonly attention?: number;
};

export type ZenPoolSummaryDTO = {
  readonly total: number;
  readonly enabled: number;
  readonly healthy: number;
  readonly cooled: number;
  readonly benched?: number;
  readonly disabled: number;
  readonly probing?: number;
  readonly attention?: number;
  readonly by_provider?: Readonly<Record<string, ZenPoolProviderSummaryDTO>>;
};

export type ZenKeyDTO = {
  readonly id: string;
  readonly label: string;
  readonly prefix: string;
  /**
   * Legacy fixed weight. Still returned/accepted for API compatibility;
   * P0 selection ignores it — use health_score / selection_score instead.
   */
  readonly weight?: number;
  readonly enabled: boolean;
  readonly provider?: KeyProvider;
  readonly cooldown_until?: string;
  readonly created_at?: string;
  /** active | cooling | benched | disabled | probing — derived server-side */
  readonly status?: ZenKeyStatus;
  /**
   * Estimated dynamic traffic share within the same provider eligible set (0–100).
   * Not historical hits; not cross-provider routing.
   */
  readonly traffic_pct?: number;
  /** Seconds remaining until cooldown_until; 0 when not cooling. */
  readonly cooldown_remaining_sec?: number;
  /** Persistent 0–100 health score (cold start 70 when missing). */
  readonly health_score?: number;
  /** In-memory selection score after load penalty (drives weighted pick). */
  readonly selection_score?: number;
  readonly success_count?: number;
  readonly failure_count?: number;
  readonly consecutive_failures?: number;
  /** Coarse class e.g. unauthorized / rate_limited / upstream_5xx — never raw body. */
  readonly last_error_class?: string;
  readonly last_success_at?: string;
  readonly last_failure_at?: string;
  readonly health_updated_at?: string;
  readonly cooldown_reason?: string;
};
export type AccountDTO = {
  readonly id: string;
  readonly name: string;
  readonly workspace_id: string;
  readonly masked_cookie: string;
  readonly show_rolling: boolean;
  readonly show_weekly: boolean;
  readonly show_monthly: boolean;
  readonly enabled: boolean;
};
export type OllamaAccountDTO = {
  readonly id: string;
  readonly name: string;
  readonly masked_cookie: string;
  readonly show_session: boolean;
  readonly show_weekly: boolean;
  readonly enabled: boolean;
};
export type QuotaWindowDTO = {
  readonly label: string;
  readonly used: number;
  readonly remaining: number;
  readonly total: number;
  readonly unit: string;
  readonly reset_in_sec: number;
  readonly used_pct?: number;
  readonly headroom_pct?: number;
  readonly burn_per_day?: number | null;
  readonly days_to_empty?: number | null;
};
export type QuotaNarrativeDTO = {
  readonly primary_label?: string;
  readonly used_pct?: number;
  readonly headroom_pct?: number;
  readonly days_to_empty?: number | null;
  readonly note?: string;
};
export type AccountQuotaDTO = {
  readonly account_id: string;
  readonly name: string;
  readonly workspace_id: string;
  readonly success: boolean;
  readonly updated_at: string;
  readonly windows?: ReadonlyArray<QuotaWindowDTO>;
  readonly narrative?: QuotaNarrativeDTO;
  readonly error?: string;
};
export type UsageRecordDTO = {
  readonly id: string;
  readonly account_id: string;
  readonly usg_id: string;
  readonly model: string;
  readonly input_tokens: number;
  readonly output_tokens: number;
  readonly recorded_at: string;
};
export type UsageSyncResultDTO = {
  readonly inserted: number;
  readonly pages_fetched: number;
  readonly sync_at: string;
  readonly error?: string;
};
export type LogDTO = {
  readonly id: string;
  readonly key_id?: string;
  readonly model: string;
  readonly route: string;
  /** Data-plane channel: opencode_free | opencode_paid | ollama_paid. */
  readonly upstream?: string;
  /** Free-path egress proxy (secret-safe). Empty = direct / paid. */
  readonly proxy_id?: string;
  readonly proxy_label?: string;
  readonly proxy_host?: string;
  readonly status: number;
  readonly latency_ms: number;
  readonly ttft_ms?: number;
  readonly stream: boolean;
  readonly error_class?: string;
  readonly max_tokens?: number;
  readonly reasoning_effort?: string;
  readonly thinking_type?: string;
  readonly budget_tokens?: number;
  readonly input_tokens?: number;
  readonly output_tokens?: number;
  readonly cache_read_tokens?: number;
  readonly cache_creation_tokens?: number;
  readonly created_at: string;
};
export type VersionInfoDTO = {
  readonly current: string;
  readonly latest: string;
  readonly update_available: boolean;
  readonly image: string;
  readonly checked_at: string;
  readonly source: string;
  readonly note?: string;
};

export type SettingsDTO = {
  readonly model_cache_ttl_seconds: number;
  readonly show_all_models: boolean;
  readonly oc_version: string;
  readonly listen: string;
  readonly cookie_secure: boolean;
  readonly zen_base: string;
  readonly data_dir: string;
  readonly upstream_timeout_seconds: number;
  readonly session_ttl_hours: number;
  readonly password_custom: boolean;
  readonly http_proxy_configured: boolean;
  readonly https_proxy_configured: boolean;
  /** zenpool selection: spread | sticky */
  readonly load_policy?: "spread" | "sticky" | string;
  /** ProxyPaid attempts per request (2..4) */
  readonly max_failover_attempts?: number;
  /** process-memory 401 isolation minutes (1..60) */
  readonly bench_duration_minutes?: number;
};
export type OverviewQuotaNarrativeDTO = {
  readonly effective_remaining: number;
  readonly worst_used_pct?: number;
  readonly headroom_pct?: number;
  readonly days_to_empty?: number | null;
  readonly burn_per_day?: number | null;
  readonly note?: string;
};

/** Time-window ops KPIs from reqlog metadata (no bodies). Owned by overview-ops-kpis. */
export type OpsWindow = "1h" | "24h" | "7d";

export type OpsKPIsDTO = {
  readonly window: OpsWindow | string;
  readonly requests: number;
  /** 0..1; null/undefined when requests==0 */
  readonly success_rate?: number | null;
  readonly latency_p50_ms?: number | null;
  readonly latency_p95_ms?: number | null;
  readonly status_2xx: number;
  readonly status_429: number;
  /** 400–499 excluding 429 */
  readonly status_4xx: number;
  readonly status_5xx: number;
};

export type RoutingUpstreamKPI = {
  readonly upstream: string;
  readonly requests: number;
  /** 0..1; null/undefined when requests==0 */
  readonly success_rate?: number | null;
  readonly latency_p50_ms?: number | null;
  readonly latency_p95_ms?: number | null;
  readonly status_2xx: number;
  readonly status_429: number;
  /** 400–499 excluding 429; optional for older backends. */
  readonly status_4xx?: number;
  readonly status_5xx: number;
};

/** Final upstream-channel KPIs from request-log metadata, without bodies or secrets. */
export type RoutingKPIsDTO = {
  readonly window: OpsWindow | string;
  readonly requests: number;
  readonly by_upstream: ReadonlyArray<RoutingUpstreamKPI>;
};

export type OverviewDTO = {
  readonly requests_today: number;
  readonly tokens_today: number;
  readonly requests_total: number;
  readonly tokens_total: number;
  readonly by_model: ReadonlyArray<{
    readonly model: string;
    readonly requests: number;
    readonly input_tokens: number;
    readonly output_tokens: number;
  }>;
  readonly quota_effective_remaining: number;
  readonly quota_windows?: ReadonlyArray<{
    readonly label: string;
    readonly used: number;
    readonly remaining: number;
    readonly effective_remaining: number;
    readonly blocked: boolean;
    readonly blocked_by?: string;
  }>;
  /** Quota burn/headroom narrative (owned by quota surface; not zen_pool). */
  readonly quota_narrative?: OverviewQuotaNarrativeDTO;
  /** Zen key pool health summary (secret-free). */
  readonly zen_pool?: ZenPoolSummaryDTO;
  /** Time-window request KPIs (reqlog; owned by overview-ops-kpis). */
  readonly ops_kpis?: OpsKPIsDTO;
  /** Additive final-channel aggregates for the requested OpsWindow. */
  readonly routing_kpis?: RoutingKPIsDTO;
  readonly updated_at?: string;
};
export type MetricsDTO = {
  readonly total_requests: number;
  readonly status_429: number;
  /** 400–499 excluding 429 */
  readonly status_4xx: number;
  readonly status_5xx: number;
  readonly status_2xx: number;
  readonly stream_requests: number;
};

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const response = await fetch(path, { ...init, headers, credentials: "include" });
  const text = await response.text();
  let data: unknown = null;
  if (text) {
    try {
      data = JSON.parse(text) as unknown;
    } catch {
      throw new ApiError(response.status, "响应不是有效 JSON");
    }
  }
  if (!response.ok) {
    const message =
      typeof data === "object" &&
      data !== null &&
      "error" in data &&
      typeof (data as { error: unknown }).error === "string"
        ? (data as { error: string }).error
        : `请求失败 (${response.status})`;
    throw new ApiError(response.status, message);
  }
  return data as T;
}

export const api = {
  login: (password: string) =>
    request<{ ok: boolean; expires_at?: string }>("/api/admin/login", {
      method: "POST",
      body: JSON.stringify({ password }),
    }),
  logout: () => request<{ ok: boolean }>("/api/admin/logout", { method: "POST" }),
  me: () => request<{ ok: boolean; role: string }>("/api/admin/me"),
  overview: (window?: OpsWindow) =>
    request<OverviewDTO>(
      window
        ? `/api/admin/overview?window=${encodeURIComponent(window)}`
        : "/api/admin/overview",
    ),
  models: () => request<{ models: ModelDTO[]; stale: boolean }>("/api/admin/models"),
  refreshModels: () =>
    request<{ models: ModelDTO[]; stale: boolean }>("/api/admin/models/refresh", {
      method: "POST",
    }),
  localKeys: () => request<{ keys: LocalKeyDTO[] }>("/api/admin/local-keys"),
  createLocalKey: (label: string, rpmLimit = 0, dailyLimit = 0) =>
    request<LocalKeyCreatedDTO>("/api/admin/local-keys", {
      method: "POST",
      body: JSON.stringify({ label, rpm_limit: rpmLimit, daily_limit: dailyLimit }),
    }),
  updateLocalKey: (id: string, label: string, rpmLimit = 0, dailyLimit = 0) =>
    request<{ ok: boolean }>(`/api/admin/local-keys/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ label, rpm_limit: rpmLimit, daily_limit: dailyLimit }),
    }),
  setLocalKeyEnabled: (id: string, enabled: boolean) =>
    request<{ ok: boolean }>(
      `/api/admin/local-keys/${id}/${enabled ? "enable" : "disable"}`,
      { method: "POST" },
    ),
  revokeLocalKey: (id: string) =>
    request<{ ok: boolean }>(`/api/admin/local-keys/${id}/revoke`, { method: "POST" }),
  revealLocalKey: (id: string) =>
    request<{ secret: string }>(`/api/admin/local-keys/${id}/reveal`, { method: "POST" }),
  zenKeys: (provider?: KeyProvider) =>
    request<{ keys: ZenKeyDTO[]; summary?: ZenPoolSummaryDTO }>(
      provider
        ? `/api/admin/zen-keys?provider=${encodeURIComponent(provider)}`
        : "/api/admin/zen-keys",
    ),
  createZenKey: (
    label: string,
    secret: string,
    provider: KeyProvider = "opencode",
  ) =>
    request<ZenKeyDTO>("/api/admin/zen-keys", {
      method: "POST",
      // Omit weight: backend defaults to 1 for DB compat; selection uses health_score.
      body: JSON.stringify({ label, secret, provider }),
    }),
  updateZenKey: (id: string, body: { label: string; secret?: string }) =>
    request<ZenKeyDTO>(`/api/admin/zen-keys/${id}`, {
      method: "PATCH",
      body: JSON.stringify({
        label: body.label,
        // Do not send weight — backend preserves existing weight when omitted / ≤0.
        secret: body.secret ?? "",
      }),
    }),
  setZenKeyEnabled: (id: string, enabled: boolean) =>
    request<{ ok: boolean }>(
      `/api/admin/zen-keys/${id}/${enabled ? "enable" : "disable"}`,
      { method: "POST" },
    ),
  deleteZenKey: (id: string) =>
    request<{ ok: boolean }>(`/api/admin/zen-keys/${id}`, { method: "DELETE" }),
  proxies: () =>
    request<{
      proxies: ReadonlyArray<{
        id: string;
        label: string;
        scheme: string;
        host: string;
        weight: number;
        enabled: boolean;
        cooldown_until?: string;
      }>;
    }>("/api/admin/proxies"),
  createProxy: (label: string, url: string, weight = 1) =>
    request("/api/admin/proxies", {
      method: "POST",
      body: JSON.stringify({ label, url, weight }),
    }),
  updateProxy: (id: string, body: { label: string; weight: number; url?: string }) =>
    request(`/api/admin/proxies/${id}`, {
      method: "PATCH",
      body: JSON.stringify({
        label: body.label,
        weight: body.weight,
        url: body.url ?? "",
      }),
    }),
  setProxyEnabled: (id: string, enabled: boolean) =>
    request<{ ok: boolean }>(`/api/admin/proxies/${id}/${enabled ? "enable" : "disable"}`, {
      method: "POST",
    }),
  deleteProxy: (id: string) =>
    request<{ ok: boolean }>(`/api/admin/proxies/${id}`, { method: "DELETE" }),
  accounts: () => request<{ accounts: AccountDTO[] }>("/api/admin/opencode-accounts"),
  createAccount: (body: {
    name: string;
    workspace_id: string;
    auth_cookie: string;
    show_rolling: boolean;
    show_weekly: boolean;
    show_monthly: boolean;
    enabled: boolean;
  }) =>
    request<AccountDTO>("/api/admin/opencode-accounts", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  setAccountEnabled: (id: string, enabled: boolean) =>
    request<AccountDTO>(
      `/api/admin/opencode-accounts/${id}/${enabled ? "enable" : "disable"}`,
      { method: "POST" },
    ),
  getAccountCredential: (id: string) =>
    request<{ workspace_id: string; auth_cookie: string }>(
      `/api/admin/opencode-accounts/${id}/credential`,
    ),
  deleteAccount: (id: string) =>
    request<{ ok: boolean }>(`/api/admin/opencode-accounts/${id}`, { method: "DELETE" }),
  quotas: () => request<{ quotas: AccountQuotaDTO[] }>("/api/admin/quotas"),
  ollamaAccounts: () => request<{ accounts: OllamaAccountDTO[] }>("/api/admin/ollama-accounts"),
  createOllamaAccount: (body: {
    name: string;
    session_cookie: string;
    enabled?: boolean;
    show_session?: boolean;
    show_weekly?: boolean;
  }) =>
    request<OllamaAccountDTO>("/api/admin/ollama-accounts", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  setOllamaAccountEnabled: (id: string, enabled: boolean) =>
    request<OllamaAccountDTO>(
      `/api/admin/ollama-accounts/${id}/${enabled ? "enable" : "disable"}`,
      { method: "POST" },
    ),
  getOllamaAccountCredential: (id: string) =>
    request<{ session_cookie: string }>(`/api/admin/ollama-accounts/${id}/credential`),
  deleteOllamaAccount: (id: string) =>
    request<{ ok: boolean }>(`/api/admin/ollama-accounts/${id}`, { method: "DELETE" }),
  ollamaQuotas: () =>
    request<{
      quotas: ReadonlyArray<{
        account_id: string;
        name: string;
        success: boolean;
        plan?: string;
        error?: string;
        windows?: ReadonlyArray<{
          label: string;
          used: number;
          remaining: number;
          unit: string;
          status_text?: string;
          models?: ReadonlyArray<{ model: string; requests: number }>;
        }>;
      }>;
    }>("/api/admin/ollama-quotas"),
  usage: (opts?: {
    limit?: number;
    from?: string;
    to?: string;
    offset?: number;
    account_id?: string;
  }) => {
    const params = new URLSearchParams();
    params.set("limit", String(opts?.limit ?? 100));
    if (opts?.from) params.set("from", opts.from);
    if (opts?.to) params.set("to", opts.to);
    if (opts?.offset != null) params.set("offset", String(opts.offset));
    if (opts?.account_id) params.set("account_id", opts.account_id);
    return request<{
      records: UsageRecordDTO[];
      truncated?: boolean;
      limit?: number;
    }>(`/api/admin/usage?${params.toString()}`);
  },
  syncUsage: (accountId: string, maxPages = 3) =>
    request<UsageSyncResultDTO>("/api/admin/usage/sync", {
      method: "POST",
      body: JSON.stringify({ account_id: accountId, max_pages: maxPages }),
    }),
  backfillUsage: (accountId: string, maxPages = 5) =>
    request<UsageSyncResultDTO>("/api/admin/usage/backfill", {
      method: "POST",
      body: JSON.stringify({ account_id: accountId, max_pages: maxPages }),
    }),
  logs: (opts?: {
    limit?: number;
    from?: string;
    to?: string;
    offset?: number;
  }) => {
    const params = new URLSearchParams();
    params.set("limit", String(opts?.limit ?? 100));
    if (opts?.from) params.set("from", opts.from);
    if (opts?.to) params.set("to", opts.to);
    if (opts?.offset != null) params.set("offset", String(opts.offset));
    return request<{
      logs: LogDTO[];
      truncated?: boolean;
      limit?: number;
    }>(`/api/admin/logs?${params.toString()}`);
  },
  metrics: () => request<MetricsDTO>("/api/admin/metrics"),
  settings: () => request<SettingsDTO>("/api/admin/settings"),
  patchSettings: (body: {
    load_policy?: "spread" | "sticky";
    max_failover_attempts?: number;
    bench_duration_minutes?: number;
  }) =>
    request<SettingsDTO>("/api/admin/settings", {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  changePassword: (currentPassword: string, newPassword: string) =>
    request<{ ok: boolean }>("/api/admin/password", {
      method: "POST",
      body: JSON.stringify({
        current_password: currentPassword,
        new_password: newPassword,
      }),
    }),
  version: (refresh = false) =>
    request<VersionInfoDTO>(
      refresh ? "/api/admin/version?refresh=1" : "/api/admin/version",
    ),
};
