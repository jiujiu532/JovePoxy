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

function GatewayLogsPanel() {
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
      setError(err instanceof Error ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
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
      {!loading && error ? <ErrorState title="加载失败" description={error} /> : null}
      {!loading && !error ? (
        <SectionPanel
          title="网关请求"
          description={`${filtered.length} / ${logs.length} · 成功 ${ok} · 429 ${rateLimited} · 均延迟 ${logs.length ? formatLatency(avgLatency) : "-"}`}
          bodyClassName="p-0"
          actions={
            <Button variant="secondary" size="sm" onClick={() => void load()}>
              刷新
            </Button>
          }
        >
          <FilterStrip
            search={query}
            onSearchChange={setQuery}
            searchPlaceholder="模型 / 路由 / key"
            filters={
              <>
                <SegmentedFilter
                  aria-label="状态"
                  value={statusFilter}
                  onChange={(v) => setStatusFilter(v as StatusFilter)}
                  options={[
                    { value: "all", label: "全部" },
                    { value: "2xx", label: "2xx" },
                    { value: "429", label: "429" },
                    { value: "4xx", label: "4xx" },
                    { value: "5xx", label: "5xx" },
                  ]}
                />
                <SegmentedFilter
                  aria-label="流式"
                  value={streamFilter}
                  onChange={(v) => setStreamFilter(v as StreamFilter)}
                  options={[
                    { value: "all", label: "全部" },
                    { value: "stream", label: "流式" },
                    { value: "nonstream", label: "非流式" },
                  ]}
                />
              </>
            }
            trailing={
              <>
                <FilterSelect
                  label="路由"
                  value={routeFilter}
                  onChange={setRouteFilter}
                  options={[
                    { value: "all", label: "全部" },
                    ...routes.map((route) => ({ value: route, label: route })),
                  ]}
                />
                <FilterSelect
                  label="排序"
                  value={sortKey}
                  onChange={(v) => setSortKey(v as SortKey)}
                  options={[
                    { value: "newest", label: "最新优先" },
                    { value: "oldest", label: "最旧优先" },
                    { value: "latency_desc", label: "延迟高→低" },
                    { value: "latency_asc", label: "延迟低→高" },
                  ]}
                />
                <Button variant="secondary" size="sm" onClick={resetFilters}>
                  重置
                </Button>
              </>
            }
          />
          {logs.length === 0 ? (
            <EmptyState
              icon={ClipboardText}
              title="暂无请求"
              description="对 /v1/chat/completions 或 /v1/messages 发起请求后会出现在这里。"
            />
          ) : filtered.length === 0 ? (
            <EmptyState compact title="没有匹配日志" description="放宽筛选条件试试。" />
          ) : (
            <div className="overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full min-w-[28rem] md:min-w-[52rem] text-left text-sm">
                  <thead>
                    <tr className="border-b border-border bg-paper-0/60 text-caption text-ink-muted">
                      <th className="px-4 py-2.5 font-medium">时间</th>
                      <th className="px-4 py-2.5 font-medium">路由</th>
                      <th className="px-4 py-2.5 font-medium">模型</th>
                      <th className="px-4 py-2.5 font-medium">状态</th>
                      <th className="px-4 py-2.5 font-medium">延迟</th>
                      <th className="px-4 py-2.5 font-medium">流式</th>
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
                          {row.model || "-"}
                        </td>
                        <td className="px-4 py-3">
                          <Badge kind={statusKind(row.status)}>{row.status}</Badge>
                        </td>
                        <td className="px-4 py-3 tabular-nums text-ink">
                          {formatLatency(row.latency_ms)}
                        </td>
                        <td className="px-4 py-3 text-ink-muted">{row.stream ? "是" : "否"}</td>
                        <td className="px-4 py-3 font-mono text-[11px] text-ink-faint">
                          {row.key_id ?? "-"}
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

function UsagePanel() {
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
      setError(err instanceof Error ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  useEffect(() => {
    setPage(1);
  }, [query, modelFilter, sortKey, accountId]);

  async function runSync(backfill: boolean) {
    if (!accountId) {
      setError("请先选择账号");
      return;
    }
    setSyncing(true);
    try {
      const result = backfill
        ? await api.backfillUsage(accountId, 5)
        : await api.syncUsage(accountId, 3);
      setMessage(`写入 ${result.inserted} 条，抓取 ${result.pages_fetched} 页`);
      setError(null);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "同步失败");
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
      {!loading && error ? <ErrorState title="加载失败" description={error} /> : null}
      {!loading && !error ? (
        <SectionPanel
          title="OpenCode 用量"
          description={`${filtered.length} / ${records.length} · 入 ${totalIn} · 出 ${totalOut}`}
          bodyClassName="p-0"
          actions={
            <div className="flex flex-wrap gap-1.5">
              <Button
                variant="secondary"
                size="sm"
                loading={syncing}
                onClick={() => void runSync(false)}
              >
                增量同步
              </Button>
              <Button
                variant="secondary"
                size="sm"
                loading={syncing}
                onClick={() => void runSync(true)}
              >
                补拉
              </Button>
            </div>
          }
        >
          <FilterStrip
            search={query}
            onSearchChange={setQuery}
            searchPlaceholder="模型 / usg_id / 时间"
            filters={
              <FilterSelect
                label="账号"
                value={accountId || ""}
                onChange={setAccountId}
                options={[
                  { value: "", label: "选择账号" },
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
                  label="模型"
                  value={modelFilter}
                  onChange={setModelFilter}
                  options={[
                    { value: "all", label: "全部" },
                    ...models.map((model) => ({ value: model, label: model })),
                  ]}
                />
                <FilterSelect
                  label="排序"
                  value={sortKey}
                  onChange={(v) =>
                    setSortKey(v as "newest" | "oldest" | "tokens_desc")
                  }
                  options={[
                    { value: "newest", label: "最新优先" },
                    { value: "oldest", label: "最旧优先" },
                    { value: "tokens_desc", label: "Token 多→少" },
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
                  重置
                </Button>
              </>
            }
          />
          {records.length === 0 ? (
            <EmptyState
              icon={Database}
              title="暂无用量记录"
              description={
                accounts.length === 0
                  ? "先添加 OpenCode 账号，再执行同步。"
                  : "选择账号后执行增量同步或补拉。"
              }
              action={
                accounts.length === 0 ? (
                  <Button variant="secondary" onClick={() => void navigate("/app/accounts")}>
                    去添加账号
                  </Button>
                ) : (
                  <Button
                    variant="secondary"
                    loading={syncing}
                    onClick={() => void runSync(false)}
                  >
                    立即同步
                  </Button>
                )
              }
            />
          ) : filtered.length === 0 ? (
            <EmptyState compact title="没有匹配记录" description="放宽筛选条件试试。" />
          ) : (
            <div className="overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full min-w-[28rem] md:min-w-[44rem] text-left text-sm">
                  <thead>
                    <tr className="border-b border-border bg-paper-0/60 text-caption text-ink-muted">
                      <th className="px-4 py-2.5 font-medium">时间</th>
                      <th className="px-4 py-2.5 font-medium">模型</th>
                      <th className="px-4 py-2.5 font-medium">输入</th>
                      <th className="px-4 py-2.5 font-medium">输出</th>
                      <th className="px-4 py-2.5 font-medium">合计</th>
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

function OllamaUsagePanel() {
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
      setError(err instanceof Error ? err.message : "加载失败");
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }

  useEffect(() => {
    void load();
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
      {!loading && error ? <ErrorState title="加载失败" description={error} /> : null}
      {!loading && !error ? (
        <SectionPanel
          title="Ollama 用量"
          description={`${filtered.length} 条模型记录 · 请求合计 ${totalReq}（来自控制面额度快照）`}
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
              刷新
            </Button>
          }
        >
          <FilterStrip
            search={query}
            onSearchChange={setQuery}
            searchPlaceholder="模型 / 账号 / 窗口"
            filters={
              <FilterSelect
                label="账号"
                value={accountFilter}
                onChange={setAccountFilter}
                options={[
                  { value: "all", label: "全部" },
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
                重置
              </Button>
            }
          />
          {rows.length === 0 ? (
            <EmptyState
              icon={Database}
              title="暂无 Ollama 用量"
              description="额度抓取成功后，会在此汇总各账号窗口下的模型请求数。可先到账号统计添加 Ollama 账号。"
              action={
                <Button variant="secondary" onClick={() => void navigate("/app/accounts?tab=ollama")}>
                  去添加账号
                </Button>
              }
            />
          ) : filtered.length === 0 ? (
            <EmptyState compact title="没有匹配记录" description="放宽筛选条件试试。" />
          ) : (
            <div className="overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full min-w-[28rem] md:min-w-[36rem] text-left text-sm">
                  <thead>
                    <tr className="border-b border-border bg-paper-0/60 text-caption text-ink-muted">
                      <th className="px-4 py-2.5 font-medium">账号</th>
                      <th className="px-4 py-2.5 font-medium">窗口</th>
                      <th className="px-4 py-2.5 font-medium">模型</th>
                      <th className="px-4 py-2.5 font-medium">请求数</th>
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

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="请求日志"
        description="网关代理请求，以及 OpenCode / Ollama 控制面用量。"
        toolbar={
          <Tabs
            aria-label="日志类型"
            value={tab}
            onChange={(id) => setTab(id as LogsHubTab)}
            items={[
              { id: "gateway", label: "网关请求" },
              { id: "usage-oc", label: "OpenCode 用量" },
              { id: "usage-ol", label: "Ollama 用量" },
            ]}
          />
        }
      />

      {tab === "gateway" ? (
        <GatewayLogsPanel />
      ) : tab === "usage-oc" ? (
        <UsagePanel />
      ) : (
        <OllamaUsagePanel />
      )}
    </div>
  );
}
