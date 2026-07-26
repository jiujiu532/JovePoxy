import { ClipboardText, Database } from "@phosphor-icons/react";
import { useEffect, useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import {
  Badge,
  Button,
  EmptyState,
  ErrorState,
  FilterSelect,
  FilterStrip,
  PageHeader,
  Pagination,
  SectionPanel,
  SegmentedFilter,
  Skeleton,
  Tabs,
  slicePage,
} from "@/components";
import { api, ApiError, type AccountDTO, type LogDTO, type UsageRecordDTO } from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";
import { useI18n, type Translate } from "@/lib/i18n";
import { isLogsHubTab, type LogsHubTab } from "@/lib/routes";

type StatusFilter = "all" | "2xx" | "429" | "4xx" | "5xx";
type StreamFilter = "all" | "stream" | "nonstream";
type SortKey = "newest" | "oldest" | "latency_desc" | "latency_asc";

function statusKind(status: number): "healthy" | "warning" | "error" | "neutral" {
  if (status >= 200 && status < 300) return "healthy";
  if (status === 429) return "warning";
  if (status >= 400) return "error";
  return "neutral";
}

function formatLatency(ms: number): string {
  if (ms >= 1000) return `${(ms / 1000).toFixed(ms >= 10000 ? 1 : 2)} s`;
  return `${ms} ms`;
}

function matchStatusBucket(status: number, bucket: StatusFilter): boolean {
  if (bucket === "all") return true;
  if (bucket === "2xx") return status >= 200 && status < 300;
  if (bucket === "429") return status === 429;
  if (bucket === "4xx") return status >= 400 && status < 500 && status !== 429;
  if (bucket === "5xx") return status >= 500;
  return true;
}

function useLogsTab(): readonly [LogsHubTab, (tab: LogsHubTab) => void] {
  const [params, setParams] = useSearchParams();
  const raw = params.get("tab");
  // legacy ?tab=usage → OpenCode 用量
  const normalized =
    raw === "usage" ? "usage-oc" : isLogsHubTab(raw) ? raw : "gateway";
  const tab: LogsHubTab = normalized;
  function setTab(next: LogsHubTab) {
    setParams(next === "gateway" ? {} : { tab: next }, { replace: true });
  }
  return [tab, setTab] as const;
}

function GatewayLogsPanel({ t }: { readonly t: Translate }) {
  const navigate = useNavigate();
  const [logs, setLogs] = useState<LogDTO[]>([]);
  const [query, setQuery] = useState("");
  const [routeFilter, setRouteFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [streamFilter, setStreamFilter] = useState<StreamFilter>("all");
  const [sortKey, setSortKey] = useState<SortKey>("newest");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  async function load() {
    setLoading(true);
    try {
      const res = await api.logs();
      setLogs(res.logs);
      setError(null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setSessionHint(false);
        void navigate("/login");
        return;
      }
      setError(err instanceof Error ? err.message : t("common.loadFailed"));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    setPage(1);
  }, [query, routeFilter, statusFilter, streamFilter, sortKey]);

  const routes = useMemo(() => {
    const set = new Set(logs.map((l) => l.route).filter(Boolean));
    return [...set].sort();
  }, [logs]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    let rows = logs.filter((row) => {
      if (routeFilter !== "all" && row.route !== routeFilter) return false;
      if (!matchStatusBucket(row.status, statusFilter)) return false;
      if (streamFilter === "stream" && !row.stream) return false;
      if (streamFilter === "nonstream" && row.stream) return false;
      if (!q) return true;
      return (
        row.route.toLowerCase().includes(q) ||
        row.model.toLowerCase().includes(q) ||
        String(row.status).includes(q) ||
        (row.error_class ?? "").toLowerCase().includes(q) ||
        (row.key_id ?? "").toLowerCase().includes(q)
      );
    });

    rows = [...rows].sort((a, b) => {
      if (sortKey === "newest") return b.created_at.localeCompare(a.created_at);
      if (sortKey === "oldest") return a.created_at.localeCompare(b.created_at);
      if (sortKey === "latency_desc") return b.latency_ms - a.latency_ms;
      return a.latency_ms - b.latency_ms;
    });
    return rows;
  }, [logs, query, routeFilter, statusFilter, streamFilter, sortKey]);

  const paged = useMemo(
    () => slicePage(filtered, page, pageSize),
    [filtered, page, pageSize],
  );

  const ok = logs.filter((l) => l.status >= 200 && l.status < 300).length;
  const rateLimited = logs.filter((l) => l.status === 429).length;
  const avgLatency =
    logs.length === 0
      ? 0
      : Math.round(logs.reduce((sum, row) => sum + row.latency_ms, 0) / logs.length);

  function resetFilters() {
    setQuery("");
    setRouteFilter("all");
    setStatusFilter("all");
    setStreamFilter("all");
    setSortKey("newest");
    setPage(1);
  }

  return (
    <>
      {loading ? <Skeleton className="h-48 w-full" /> : null}
      {!loading && error ? <ErrorState title={t("common.loadFailed")} description={error} /> : null}
      {!loading && !error ? (
        <SectionPanel
          title={t("logs.gatewayTitle")}
          description={t("logs.gatewayStats", {
            filtered: filtered.length,
            total: logs.length,
            ok,
            rateLimited,
            latency: logs.length ? formatLatency(avgLatency) : t("common.none"),
          })}
          bodyClassName="p-0"
          actions={
            <Button variant="secondary" size="sm" onClick={() => void load()}>
              {t("common.refresh")}
            </Button>
          }
        >
          <FilterStrip
            search={query}
            onSearchChange={setQuery}
            searchPlaceholder={t("logs.gatewaySearchPlaceholder")}
            filters={
              <>
                <SegmentedFilter
                  aria-label={t("logs.statusAria")}
                  value={statusFilter}
                  onChange={(v) => setStatusFilter(v as StatusFilter)}
                  options={[
                    { value: "all", label: t("common.all") },
                    { value: "2xx", label: "2xx" },
                    { value: "429", label: "429" },
                    { value: "4xx", label: "4xx" },
                    { value: "5xx", label: "5xx" },
                  ]}
                />
                <SegmentedFilter
                  aria-label={t("logs.streamAria")}
                  value={streamFilter}
                  onChange={(v) => setStreamFilter(v as StreamFilter)}
                  options={[
                    { value: "all", label: t("common.all") },
                    { value: "stream", label: t("logs.streamYes") },
                    { value: "nonstream", label: t("logs.streamNo") },
                  ]}
                />
              </>
            }
            trailing={
              <>
                <FilterSelect
                  label={t("logs.routeLabel")}
                  value={routeFilter}
                  onChange={setRouteFilter}
                  options={[
                    { value: "all", label: t("common.all") },
                    ...routes.map((route) => ({ value: route, label: route })),
                  ]}
                />
                <FilterSelect
                  label={t("logs.sortLabel")}
                  value={sortKey}
                  onChange={(v) => setSortKey(v as SortKey)}
                  options={[
                    { value: "newest", label: t("logs.sortNewest") },
                    { value: "oldest", label: t("logs.sortOldest") },
                    { value: "latency_desc", label: t("logs.sortLatencyDesc") },
                    { value: "latency_asc", label: t("logs.sortLatencyAsc") },
                  ]}
                />
                <Button variant="secondary" size="sm" onClick={resetFilters}>
                  {t("logs.reset")}
                </Button>
              </>
            }
          />
          {logs.length === 0 ? (
            <EmptyState
              icon={ClipboardText}
              title={t("logs.emptyGatewayTitle")}
              description={t("logs.emptyGatewayDescription")}
            />
          ) : filtered.length === 0 ? (
            <EmptyState compact title={t("logs.noMatchTitle")} description={t("logs.noMatchDescription")} />
          ) : (
            <div className="overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full min-w-[28rem] md:min-w-[52rem] text-left text-sm">
                  <thead>
                    <tr className="border-b border-border bg-paper-0/60 text-caption text-ink-muted">
                      <th className="px-4 py-2.5 font-medium">{t("logs.colTime")}</th>
                      <th className="px-4 py-2.5 font-medium">{t("logs.routeLabel")}</th>
                      <th className="px-4 py-2.5 font-medium">{t("logs.colModel")}</th>
                      <th className="px-4 py-2.5 font-medium">{t("logs.statusAria")}</th>
                      <th className="px-4 py-2.5 font-medium">{t("logs.colLatency")}</th>
                      <th className="px-4 py-2.5 font-medium">{t("logs.streamAria")}</th>
                      <th className="px-4 py-2.5 font-medium">Key</th>
                    </tr>
                  </thead>
                  <tbody>
                    {paged.map((row) => (
                      <tr
                        key={row.id}
                        className="border-b border-border last:border-b-0 hover:bg-paper-0/50"
                      >
                        <td className="px-4 py-3 text-[12px] text-ink-muted whitespace-nowrap">
                          {row.created_at}
                        </td>
                        <td className="px-4 py-3 font-mono text-[12px] text-ink-muted">
                          {row.route}
                        </td>
                        <td className="px-4 py-3 font-mono text-[13px] text-ink">
                          {row.model || t("common.none")}
                        </td>
                        <td className="px-4 py-3">
                          <Badge kind={statusKind(row.status)}>{row.status}</Badge>
                        </td>
                        <td className="px-4 py-3 tabular-nums text-ink">
                          {formatLatency(row.latency_ms)}
                        </td>
                        <td className="px-4 py-3 text-ink-muted">
                          {row.stream ? t("logs.yes") : t("logs.no")}
                        </td>
                        <td className="px-4 py-3 font-mono text-[11px] text-ink-faint">
                          {row.key_id ?? t("common.none")}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <Pagination
                total={filtered.length}
                page={page}
                pageSize={pageSize}
                onPageChange={setPage}
                onPageSizeChange={(size) => {
                  setPageSize(size);
                  setPage(1);
                }}
              />
            </div>
          )}
        </SectionPanel>
      ) : null}
    </>
  );
}

function UsagePanel({ t }: { readonly t: Translate }) {
  const navigate = useNavigate();
  const [records, setRecords] = useState<UsageRecordDTO[]>([]);
  const [accounts, setAccounts] = useState<AccountDTO[]>([]);
  const [accountId, setAccountId] = useState("");
  const [query, setQuery] = useState("");
  const [modelFilter, setModelFilter] = useState("all");
  const [sortKey, setSortKey] = useState<"newest" | "oldest" | "tokens_desc">("newest");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);

  async function load() {
    setLoading(true);
    try {
      const [usage, accountList] = await Promise.all([api.usage(), api.accounts()]);
      setRecords(usage.records);
      setAccounts(accountList.accounts);
      if (!accountId && accountList.accounts[0]) {
        setAccountId(accountList.accounts[0].id);
      }
      setError(null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setSessionHint(false);
        void navigate("/login");
        return;
      }
      setError(err instanceof Error ? err.message : t("common.loadFailed"));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    setPage(1);
  }, [query, modelFilter, sortKey, accountId]);

  async function runSync(backfill: boolean) {
    if (!accountId) {
      setError(t("logs.selectAccountFirst"));
      return;
    }
    setSyncing(true);
    try {
      const result = backfill
        ? await api.backfillUsage(accountId, 5)
        : await api.syncUsage(accountId, 3);
      setMessage(t("logs.syncResult", { inserted: result.inserted, pages: result.pages_fetched }));
      setError(null);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("logs.syncFailed"));
    } finally {
      setSyncing(false);
    }
  }

  const models = useMemo(() => {
    const set = new Set(records.map((r) => r.model).filter(Boolean));
    return [...set].sort();
  }, [records]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    let rows = records.filter((row) => {
      if (modelFilter !== "all" && row.model !== modelFilter) return false;
      if (!q) return true;
      return (
        row.model.toLowerCase().includes(q) ||
        row.usg_id.toLowerCase().includes(q) ||
        row.recorded_at.toLowerCase().includes(q)
      );
    });
    rows = [...rows].sort((a, b) => {
      if (sortKey === "newest") return b.recorded_at.localeCompare(a.recorded_at);
      if (sortKey === "oldest") return a.recorded_at.localeCompare(b.recorded_at);
      return b.input_tokens + b.output_tokens - (a.input_tokens + a.output_tokens);
    });
    return rows;
  }, [records, query, modelFilter, sortKey]);

  const paged = useMemo(
    () => slicePage(filtered, page, pageSize),
    [filtered, page, pageSize],
  );

  const totalIn = filtered.reduce((sum, row) => sum + row.input_tokens, 0);
  const totalOut = filtered.reduce((sum, row) => sum + row.output_tokens, 0);

  return (
    <>
      {message ? (
        <p className="rounded-none border border-border bg-paper-0 px-3 py-2 text-[12px] text-status-success">
          {message}
        </p>
      ) : null}

      {loading ? <Skeleton className="h-48 w-full" /> : null}
      {!loading && error ? <ErrorState title={t("common.loadFailed")} description={error} /> : null}
      {!loading && !error ? (
        <SectionPanel
          title={t("logs.usageOcTitle")}
          description={t("logs.usageOcStats", { filtered: filtered.length, total: records.length, in: totalIn, out: totalOut })}
          bodyClassName="p-0"
          actions={
            <div className="flex flex-wrap gap-1.5">
              <Button
                variant="secondary"
                size="sm"
                loading={syncing}
                onClick={() => void runSync(false)}
              >
                {t("logs.syncIncremental")}
              </Button>
              <Button
                variant="secondary"
                size="sm"
                loading={syncing}
                onClick={() => void runSync(true)}
              >
                {t("logs.syncBackfill")}
              </Button>
            </div>
          }
        >
          <FilterStrip
            search={query}
            onSearchChange={setQuery}
            searchPlaceholder={t("logs.usageOcSearchPlaceholder")}
            filters={
              <FilterSelect
                label={t("logs.accountLabel")}
                value={accountId || ""}
                onChange={setAccountId}
                options={[
                  { value: "", label: t("logs.selectAccount") },
                  ...accounts.map((account) => ({
                    value: account.id,
                    label: account.name,
                  })),
                ]}
              />
            }
            trailing={
              <>
                <FilterSelect
                  label={t("logs.colModel")}
                  value={modelFilter}
                  onChange={setModelFilter}
                  options={[
                    { value: "all", label: t("common.all") },
                    ...models.map((model) => ({ value: model, label: model })),
                  ]}
                />
                <FilterSelect
                  label={t("logs.sortLabel")}
                  value={sortKey}
                  onChange={(v) =>
                    setSortKey(v as "newest" | "oldest" | "tokens_desc")
                  }
                  options={[
                    { value: "newest", label: t("logs.sortNewest") },
                    { value: "oldest", label: t("logs.sortOldest") },
                    { value: "tokens_desc", label: t("logs.sortTokensDesc") },
                  ]}
                />
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => {
                    setQuery("");
                    setModelFilter("all");
                    setSortKey("newest");
                    setPage(1);
                  }}
                >
                  {t("logs.reset")}
                </Button>
              </>
            }
          />
          {records.length === 0 ? (
            <EmptyState
              icon={Database}
              title={t("logs.emptyUsageOcTitle")}
              description={
                accounts.length === 0
                  ? t("logs.emptyUsageOcNoAccounts")
                  : t("logs.emptyUsageOcHasAccounts")
              }
              action={
                accounts.length === 0 ? (
                  <Button variant="secondary" onClick={() => void navigate("/app/accounts")}>
                    {t("logs.goAddAccount")}
                  </Button>
                ) : (
                  <Button
                    variant="secondary"
                    loading={syncing}
                    onClick={() => void runSync(false)}
                  >
                    {t("logs.syncNow")}
                  </Button>
                )
              }
            />
          ) : filtered.length === 0 ? (
            <EmptyState compact title={t("logs.noMatchRecordsTitle")} description={t("logs.noMatchDescription")} />
          ) : (
            <div className="overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full min-w-[28rem] md:min-w-[44rem] text-left text-sm">
                  <thead>
                    <tr className="border-b border-border bg-paper-0/60 text-caption text-ink-muted">
                      <th className="px-4 py-2.5 font-medium">{t("logs.colTime")}</th>
                      <th className="px-4 py-2.5 font-medium">{t("logs.colModel")}</th>
                      <th className="px-4 py-2.5 font-medium">{t("logs.colInput")}</th>
                      <th className="px-4 py-2.5 font-medium">{t("logs.colOutput")}</th>
                      <th className="px-4 py-2.5 font-medium">{t("common.total")}</th>
                      <th className="px-4 py-2.5 font-medium">usg_id</th>
                    </tr>
                  </thead>
                  <tbody>
                    {paged.map((row) => (
                      <tr
                        key={row.id}
                        className="border-b border-border last:border-b-0 hover:bg-paper-0/50"
                      >
                        <td className="px-4 py-3 text-[12px] text-ink-muted">{row.recorded_at}</td>
                        <td className="px-4 py-3 font-mono text-[13px] text-ink">{row.model}</td>
                        <td className="px-4 py-3 tabular-nums text-ink">{row.input_tokens}</td>
                        <td className="px-4 py-3 tabular-nums text-ink">{row.output_tokens}</td>
                        <td className="px-4 py-3 tabular-nums text-ink-muted">
                          {row.input_tokens + row.output_tokens}
                        </td>
                        <td className="px-4 py-3 font-mono text-[12px] text-ink-muted">
                          {row.usg_id}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <Pagination
                total={filtered.length}
                page={page}
                pageSize={pageSize}
                onPageChange={setPage}
                onPageSizeChange={(size) => {
                  setPageSize(size);
                  setPage(1);
                }}
              />
            </div>
          )}
        </SectionPanel>
      ) : null}
    </>
  );
}

type OllamaModelUsageRow = {
  readonly accountId: string;
  readonly accountName: string;
  readonly windowLabel: string;
  readonly model: string;
  readonly requests: number;
};

function OllamaUsagePanel({ t }: { readonly t: Translate }) {
  const navigate = useNavigate();
  const [rows, setRows] = useState<OllamaModelUsageRow[]>([]);
  const [query, setQuery] = useState("");
  const [accountFilter, setAccountFilter] = useState("all");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  async function load() {
    setLoading(true);
    try {
      const res = await api.ollamaQuotas();
      const next: OllamaModelUsageRow[] = [];
      for (const q of res.quotas) {
        if (!q.success) continue;
        for (const w of q.windows ?? []) {
          for (const m of w.models ?? []) {
            next.push({
              accountId: q.account_id,
              accountName: q.name,
              windowLabel: w.label,
              model: m.model,
              requests: m.requests,
            });
          }
        }
      }
      next.sort((a, b) => b.requests - a.requests || a.model.localeCompare(b.model));
      setRows(next);
      setError(null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setSessionHint(false);
        void navigate("/login");
        return;
      }
      setError(err instanceof Error ? err.message : t("common.loadFailed"));
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    setPage(1);
  }, [query, accountFilter]);

  const accounts = useMemo(() => {
    const map = new Map<string, string>();
    for (const r of rows) map.set(r.accountId, r.accountName);
    return [...map.entries()].sort((a, b) => a[1].localeCompare(b[1]));
  }, [rows]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return rows.filter((row) => {
      if (accountFilter !== "all" && row.accountId !== accountFilter) return false;
      if (!q) return true;
      return (
        row.model.toLowerCase().includes(q) ||
        row.accountName.toLowerCase().includes(q) ||
        row.windowLabel.toLowerCase().includes(q)
      );
    });
  }, [rows, query, accountFilter]);

  const paged = useMemo(
    () => slicePage(filtered, page, pageSize),
    [filtered, page, pageSize],
  );
  const totalReq = filtered.reduce((s, r) => s + r.requests, 0);

  return (
    <>
      {loading ? <Skeleton className="h-48 w-full" /> : null}
      {!loading && error ? <ErrorState title={t("common.loadFailed")} description={error} /> : null}
      {!loading && !error ? (
        <SectionPanel
          title={t("logs.usageOlTitle")}
          description={t("logs.usageOlStats", { count: filtered.length, total: totalReq })}
          bodyClassName="p-0"
          actions={
            <Button
              variant="secondary"
              size="sm"
              loading={refreshing}
              onClick={() => {
                setRefreshing(true);
                void load();
              }}
            >
              {t("common.refresh")}
            </Button>
          }
        >
          <FilterStrip
            search={query}
            onSearchChange={setQuery}
            searchPlaceholder={t("logs.usageOlSearchPlaceholder")}
            filters={
              <FilterSelect
                label={t("logs.accountLabel")}
                value={accountFilter}
                onChange={setAccountFilter}
                options={[
                  { value: "all", label: t("common.all") },
                  ...accounts.map(([id, name]) => ({ value: id, label: name })),
                ]}
              />
            }
            trailing={
              <Button
                variant="secondary"
                size="sm"
                onClick={() => {
                  setQuery("");
                  setAccountFilter("all");
                  setPage(1);
                }}
              >
                {t("logs.reset")}
              </Button>
            }
          />
          {rows.length === 0 ? (
            <EmptyState
              icon={Database}
              title={t("logs.emptyUsageOlTitle")}
              description={t("logs.emptyUsageOlDescription")}
              action={
                <Button variant="secondary" onClick={() => void navigate("/app/accounts?tab=ollama")}>
                  {t("logs.goAddAccount")}
                </Button>
              }
            />
          ) : filtered.length === 0 ? (
            <EmptyState compact title={t("logs.noMatchRecordsTitle")} description={t("logs.noMatchDescription")} />
          ) : (
            <div className="overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full min-w-[28rem] md:min-w-[36rem] text-left text-sm">
                  <thead>
                    <tr className="border-b border-border bg-paper-0/60 text-caption text-ink-muted">
                      <th className="px-4 py-2.5 font-medium">{t("logs.accountLabel")}</th>
                      <th className="px-4 py-2.5 font-medium">{t("logs.colWindow")}</th>
                      <th className="px-4 py-2.5 font-medium">{t("logs.colModel")}</th>
                      <th className="px-4 py-2.5 font-medium">{t("logs.colRequests")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {paged.map((row) => (
                      <tr
                        key={`${row.accountId}-${row.windowLabel}-${row.model}`}
                        className="border-b border-border last:border-b-0 hover:bg-paper-0/50"
                      >
                        <td className="px-4 py-3 font-medium text-ink">{row.accountName}</td>
                        <td className="px-4 py-3 text-ink-muted">{row.windowLabel}</td>
                        <td className="px-4 py-3 font-mono text-[13px] text-ink">{row.model}</td>
                        <td className="px-4 py-3 tabular-nums text-ink">{row.requests}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <Pagination
                total={filtered.length}
                page={page}
                pageSize={pageSize}
                onPageChange={setPage}
                onPageSizeChange={(size) => {
                  setPageSize(size);
                  setPage(1);
                }}
              />
            </div>
          )}
        </SectionPanel>
      ) : null}
    </>
  );
}

export function LogsPage() {
  const [tab, setTab] = useLogsTab();
  const { t } = useI18n();

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title={t("logs.pageTitle")}
        description={t("logs.pageDescription")}
        toolbar={
          <Tabs
            aria-label={t("logs.tabsAria")}
            value={tab}
            onChange={(id) => setTab(id as LogsHubTab)}
            items={[
              { id: "gateway", label: t("logs.gatewayTitle") },
              { id: "usage-oc", label: t("logs.usageOcTitle") },
              { id: "usage-ol", label: t("logs.usageOlTitle") },
            ]}
          />
        }
      />

      {tab === "gateway" ? (
        <GatewayLogsPanel t={t} />
      ) : tab === "usage-oc" ? (
        <UsagePanel t={t} />
      ) : (
        <OllamaUsagePanel t={t} />
      )}
    </div>
  );
}
