import {
  ClipboardText,
  Database,
} from "@phosphor-icons/react";
import { Fragment, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import {
  Badge,
  Button,
  EmptyState,
  EntityMark,
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
import { api, type AccountDTO, type LocalKeyDTO, type LogDTO, type UsageRecordDTO } from "@/lib/api";
import { handleUnauthorized } from "@/lib/api-error";
import { formatDateTime } from "@/lib/format";
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

type UpstreamChannel = "opencode_free" | "opencode_paid" | "ollama_paid" | "";

function normalizeUpstream(raw: string | undefined): UpstreamChannel {
  switch ((raw ?? "").trim()) {
    case "opencode_free":
      return "opencode_free";
    case "opencode_paid":
      return "opencode_paid";
    case "ollama_paid":
      return "ollama_paid";
    default:
      return "";
  }
}

function upstreamLabel(raw: string | undefined, t: Translate): string {
  switch (normalizeUpstream(raw)) {
    case "opencode_free":
      return t("logs.upstreamOpenCodeFree");
    case "opencode_paid":
      return t("logs.upstreamOpenCodePaid");
    case "ollama_paid":
      return t("logs.upstreamOllamaPaid");
    default:
      return t("logs.upstreamUnknown");
  }
}

function upstreamBadgeKind(
  raw: string | undefined,
): "free" | "warning" | "healthy" | "neutral" {
  switch (normalizeUpstream(raw)) {
    case "opencode_free":
      return "free";
    case "opencode_paid":
      return "warning";
    case "ollama_paid":
      return "healthy";
    default:
      return "neutral";
  }
}

function formatLatency(ms: number): string {
  if (ms >= 1000) return `${(ms / 1000).toFixed(ms >= 10000 ? 1 : 2)} s`;
  return `${ms} ms`;
}

/** Mapped Zen reasoning_effort for display; empty means thinking not enabled. */
function effortLevel(row: LogDTO): string {
  const effort = (row.reasoning_effort ?? "").trim().toLowerCase();
  if (effort) return effort;
  const thinking = (row.thinking_type ?? "").trim().toLowerCase();
  if (thinking === "disabled") return "none";
  return "";
}

/** Title-case labels like the usage-record reference (Medium / High / XHigh). */
function effortLabel(level: string): string {
  switch (level) {
    case "none":
      return "None";
    case "minimal":
      return "Minimal";
    case "low":
      return "Low";
    case "medium":
      return "Medium";
    case "high":
      return "High";
    case "xhigh":
      return "XHigh";
    case "auto":
      return "Auto";
    default:
      if (!level) return "";
      return level.charAt(0).toUpperCase() + level.slice(1);
  }
}

function effortBadgeKind(
  level: string,
): "neutral" | "free" | "healthy" | "warning" | "error" {
  switch (level) {
    case "none":
      return "neutral";
    case "minimal":
    case "low":
      return "free";
    case "medium":
      return "healthy";
    case "high":
      return "warning";
    case "xhigh":
      return "error";
    default:
      return "neutral";
  }
}


function tokenCount(row: LogDTO, key: "input_tokens" | "output_tokens" | "cache_read_tokens" | "cache_creation_tokens"): number {
  const value = row[key];
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function hasUsage(row: LogDTO): boolean {
  return (
    tokenCount(row, "input_tokens") > 0 ||
    tokenCount(row, "output_tokens") > 0 ||
    tokenCount(row, "cache_read_tokens") > 0 ||
    tokenCount(row, "cache_creation_tokens") > 0
  );
}

/** Resolve local key id → display name (label). Never show raw key_… ids in cells. */
function keyDisplayName(
  keyId: string | undefined,
  names: ReadonlyMap<string, string>,
  t: Translate,
): string {
  if (!keyId) return t("common.none");
  const label = names.get(keyId)?.trim();
  if (label) return label;
  // Key was revoked / missing from catalog but still referenced in history.
  return t("logs.keyDeleted");
}

function DetailField({
  label,
  value,
  mono = false,
  title,
}: {
  readonly label: string;
  readonly value: string;
  readonly mono?: boolean;
  readonly title?: string;
}) {
  return (
    <div className="min-w-0">
      <div className="text-[10px] font-semibold uppercase tracking-wide text-ink-faint">
        {label}
      </div>
      <div
        className={
          mono
            ? "mt-0.5 truncate font-mono text-[12px] text-ink"
            : "mt-0.5 truncate text-[13px] font-medium text-ink"
        }
        title={title ?? value}
      >
        {value}
      </div>
    </div>
  );
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
  const [localKeys, setLocalKeys] = useState<LocalKeyDTO[]>([]);
  const [query, setQuery] = useState("");
  const [channelFilter, setChannelFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [streamFilter, setStreamFilter] = useState<StreamFilter>("all");
  const [sortKey, setSortKey] = useState<SortKey>("newest");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const keyNames = useMemo(() => {
    const map = new Map<string, string>();
    for (const key of localKeys) {
      const name = key.label.trim() || key.prefix || key.id;
      map.set(key.id, name);
    }
    return map;
  }, [localKeys]);

  async function load() {
    setLoading(true);
    try {
      const [logRes, keyRes] = await Promise.all([api.logs(), api.localKeys()]);
      setLogs(logRes.logs);
      setLocalKeys(keyRes.keys);
      setError(null);
    } catch (err) {
      if (handleUnauthorized(err, (to) => void navigate(to))) return;
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
  }, [query, channelFilter, statusFilter, streamFilter, sortKey]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    let rows = logs.filter((row) => {
      const channel = normalizeUpstream(row.upstream);
      if (channelFilter !== "all" && channel !== channelFilter) return false;
      if (!matchStatusBucket(row.status, statusFilter)) return false;
      if (streamFilter === "stream" && !row.stream) return false;
      if (streamFilter === "nonstream" && row.stream) return false;
      if (!q) return true;
      const keyName = keyDisplayName(row.key_id, keyNames, t).toLowerCase();
      const channelText = upstreamLabel(row.upstream, t).toLowerCase();
      return (
        channelText.includes(q) ||
        (row.upstream ?? "").toLowerCase().includes(q) ||
        row.route.toLowerCase().includes(q) ||
        row.model.toLowerCase().includes(q) ||
        String(row.status).includes(q) ||
        (row.error_class ?? "").toLowerCase().includes(q) ||
        keyName.includes(q) ||
        (row.reasoning_effort ?? "").toLowerCase().includes(q) ||
        (row.thinking_type ?? "").toLowerCase().includes(q)
      );
    });

    rows = [...rows].sort((a, b) => {
      if (sortKey === "newest") return b.created_at.localeCompare(a.created_at);
      if (sortKey === "oldest") return a.created_at.localeCompare(b.created_at);
      if (sortKey === "latency_desc") return b.latency_ms - a.latency_ms;
      return a.latency_ms - b.latency_ms;
    });
    return rows;
  }, [logs, query, channelFilter, statusFilter, streamFilter, sortKey, keyNames, t]);

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
    setChannelFilter("all");
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
          icon={ClipboardText}
          iconTone="accent"
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
                  value={channelFilter}
                  onChange={setChannelFilter}
                  options={[
                    { value: "all", label: t("logs.upstreamAll") },
                    { value: "opencode_free", label: t("logs.upstreamOpenCodeFree") },
                    { value: "opencode_paid", label: t("logs.upstreamOpenCodePaid") },
                    { value: "ollama_paid", label: t("logs.upstreamOllamaPaid") },
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
                <table className="w-full min-w-[36rem] md:min-w-[64rem] text-left text-sm">
                  <thead>
                    <tr className="border-b-2 border-border bg-paper-2 text-caption text-ink-muted">
                      <th className="px-4 py-2.5 font-medium whitespace-nowrap">{t("logs.colTime")}</th>
                      <th className="px-4 py-2.5 font-medium whitespace-nowrap">{t("logs.colModel")}</th>
                      <th className="px-4 py-2.5 font-medium whitespace-nowrap">{t("logs.colEffort")}</th>
                      <th className="px-4 py-2.5 font-medium whitespace-nowrap">{t("logs.routeLabel")}</th>
                      <th className="px-4 py-2.5 font-medium whitespace-nowrap">{t("logs.statusAria")}</th>
                      <th className="px-4 py-2.5 font-medium whitespace-nowrap">{t("logs.streamAria")}</th>
                      <th className="px-4 py-2.5 font-medium whitespace-nowrap">{t("logs.colTokens")}</th>
                      <th className="px-4 py-2.5 font-medium whitespace-nowrap">{t("logs.colLatency")}</th>
                      <th className="px-4 py-2.5 font-medium whitespace-nowrap">{t("logs.colKey")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {paged.map((row) => {
                      const level = effortLevel(row);
                      const label = effortLabel(level);
                      const input = tokenCount(row, "input_tokens");
                      const output = tokenCount(row, "output_tokens");
                      const cacheRead = tokenCount(row, "cache_read_tokens");
                      const cacheWrite = tokenCount(row, "cache_creation_tokens");
                      const expanded = expandedId === row.id;
                      const usageKnown = hasUsage(row);
                      const keyName = keyDisplayName(row.key_id, keyNames, t);
                      const total = input + output;
                      return (
                        <Fragment key={row.id}>
                          <tr
                            className="border-b-2 border-border last:border-b-0 hover:bg-accent-soft cursor-pointer"
                            onClick={() => setExpandedId(expanded ? null : row.id)}
                            aria-expanded={expanded}
                          >
                            <td
                              className="px-4 py-3 font-mono text-[12px] tabular-nums text-ink-muted whitespace-nowrap"
                              title={row.created_at}
                            >
                              {formatDateTime(row.created_at)}
                            </td>
                            <td className="px-4 py-3 font-mono text-[13px] text-ink">
                              <span className="inline-flex min-w-0 items-center gap-2.5">
                                <EntityMark name={row.model || row.route} size="sm" />
                                <span className="truncate">{row.model || t("common.none")}</span>
                              </span>
                            </td>
                            <td className="px-4 py-3 whitespace-nowrap">
                              {label ? (
                                <Badge kind={effortBadgeKind(level)}>{label}</Badge>
                              ) : (
                                <span className="text-ink-faint">{t("common.none")}</span>
                              )}
                            </td>
                            <td className="px-4 py-3 whitespace-nowrap">
                              <Badge kind={upstreamBadgeKind(row.upstream)}>
                                {upstreamLabel(row.upstream, t)}
                              </Badge>
                            </td>
                            <td className="px-4 py-3 whitespace-nowrap">
                              <Badge kind={statusKind(row.status)}>{row.status}</Badge>
                            </td>
                            <td className="px-4 py-3 whitespace-nowrap">
                              <Badge kind={row.stream ? "healthy" : "neutral"}>
                                {row.stream ? t("logs.streamBadgeYes") : t("logs.streamBadgeNo")}
                              </Badge>
                            </td>
                            <td className="px-4 py-3 font-mono text-[12px] tabular-nums text-ink">
                              <div className="flex flex-col gap-0.5 leading-tight whitespace-nowrap">
                                <span>
                                  ↓ {input.toLocaleString()} · ↑ {output.toLocaleString()}
                                </span>
                                {cacheRead > 0 ? (
                                  <span className="text-[11px] text-ink-muted">
                                    {t("logs.cacheReadShort", { n: cacheRead.toLocaleString() })}
                                  </span>
                                ) : null}
                              </div>
                            </td>
                            <td className="px-4 py-3 tabular-nums text-ink whitespace-nowrap">
                              <div className="flex flex-col gap-0.5 leading-tight">
                                <span className="font-medium">{formatLatency(row.latency_ms)}</span>
                                {(row.ttft_ms ?? 0) > 0 ? (
                                  <span className="text-[11px] text-ink-muted">
                                    {t("logs.colTTFT")} {formatLatency(row.ttft_ms ?? 0)}
                                  </span>
                                ) : null}
                              </div>
                            </td>
                            <td
                              className="max-w-[10rem] truncate px-4 py-3 text-[12px] font-medium text-ink whitespace-nowrap"
                              {...(row.key_id ? { title: row.key_id } : {})}
                            >
                              {keyName}
                            </td>
                          </tr>
                          {expanded ? (
                            <tr className="border-b-2 border-border bg-paper-2/40">
                              <td colSpan={9} className="px-3 py-3 sm:px-4">
                                <div className="overflow-hidden border-2 border-border bg-paper-1 shadow-[4px_4px_0_0_var(--border)]">
                                  {/* Header strip */}
                                  <div className="flex flex-wrap items-center justify-between gap-2 border-b-2 border-border bg-paper-2 px-3 py-2">
                                    <div className="flex min-w-0 items-center gap-2">
                                      <span
                                        className={
                                          usageKnown
                                            ? "inline-block size-2.5 shrink-0 border-2 border-border bg-accent-teal"
                                            : "inline-block size-2.5 shrink-0 border-2 border-border bg-accent-yellow"
                                        }
                                        aria-hidden
                                      />
                                      <span className="text-[13px] font-bold tracking-tight text-ink">
                                        {t("logs.detailTitle")}
                                      </span>
                                      <Badge kind={statusKind(row.status)}>{row.status}</Badge>
                                      <Badge kind={row.stream ? "healthy" : "neutral"}>
                                        {row.stream ? t("logs.streamBadgeYes") : t("logs.streamBadgeNo")}
                                      </Badge>
                                      {!usageKnown ? (
                                        <span className="text-[11px] font-medium text-ink-muted">
                                          {t("logs.detailNoUsage")}
                                        </span>
                                      ) : null}
                                    </div>
                                    <span
                                      className="max-w-full truncate font-mono text-[10px] text-ink-faint"
                                      title={row.id}
                                    >
                                      {row.id}
                                    </span>
                                  </div>

                                  {/* Token metrics — colored neo-brutal tiles */}
                                  <div className="grid grid-cols-2 border-b-2 border-border sm:grid-cols-4">
                                    <div className="border-b-2 border-r-2 border-border bg-accent-yellow p-3 text-black sm:border-b-0">
                                      <div className="text-[10px] font-bold uppercase tracking-wide text-black/70">
                                        {t("logs.detailCacheRead")}
                                      </div>
                                      <div className="mt-1 font-mono text-[1.5rem] font-black tabular-nums leading-none tracking-tight">
                                        {cacheRead.toLocaleString()}
                                      </div>
                                      {cacheWrite > 0 ? (
                                        <div className="mt-1 text-[11px] font-semibold text-black/65">
                                          {t("logs.detailCacheWrite")} {cacheWrite.toLocaleString()}
                                        </div>
                                      ) : null}
                                    </div>
                                    <div className="border-b-2 border-border bg-accent-mint p-3 text-black sm:border-b-0 sm:border-r-2">
                                      <div className="text-[10px] font-bold uppercase tracking-wide text-black/70">
                                        {t("logs.detailInput")}
                                      </div>
                                      <div className="mt-1 font-mono text-[1.5rem] font-black tabular-nums leading-none tracking-tight">
                                        {input.toLocaleString()}
                                      </div>
                                    </div>
                                    <div className="border-r-2 border-border bg-accent-teal p-3 text-black">
                                      <div className="text-[10px] font-bold uppercase tracking-wide text-black/70">
                                        {t("logs.detailOutput")}
                                      </div>
                                      <div className="mt-1 font-mono text-[1.5rem] font-black tabular-nums leading-none tracking-tight">
                                        {output.toLocaleString()}
                                      </div>
                                    </div>
                                    <div className="bg-accent-coral p-3 text-black">
                                      <div className="text-[10px] font-bold uppercase tracking-wide text-black/70">
                                        {t("logs.detailTotal")}
                                      </div>
                                      <div className="mt-1 font-mono text-[1.5rem] font-black tabular-nums leading-none tracking-tight">
                                        {total.toLocaleString()}
                                      </div>
                                    </div>
                                  </div>

                                  {/* Meta grid — label / value pairs, no run-on line */}
                                  <div className="grid grid-cols-2 gap-x-4 gap-y-3 px-3 py-3 sm:grid-cols-3 lg:grid-cols-4">
                                    <DetailField
                                      label={t("logs.detailTime")}
                                      value={formatDateTime(row.created_at)}
                                      mono
                                      title={row.created_at}
                                    />
                                    <DetailField
                                      label={t("logs.detailLatency")}
                                      value={formatLatency(row.latency_ms)}
                                      mono
                                    />
                                    <DetailField
                                      label={t("logs.detailTTFT")}
                                      value={(row.ttft_ms ?? 0) > 0 ? formatLatency(row.ttft_ms ?? 0) : t("common.none")}
                                      mono
                                    />
                                    <DetailField
                                      label={t("logs.detailModel")}
                                      value={row.model || t("common.none")}
                                      mono
                                    />
                                    <DetailField
                                      label={t("logs.detailUpstream")}
                                      value={upstreamLabel(row.upstream, t)}
                                    />
                                    <DetailField
                                      label={t("logs.detailRoute")}
                                      value={row.route}
                                      mono
                                    />
                                    <DetailField
                                      label={t("logs.detailKey")}
                                      value={keyName}
                                      {...(row.key_id ? { title: row.key_id } : {})}
                                    />
                                    <DetailField
                                      label={t("logs.detailEffort")}
                                      value={label || t("common.none")}
                                    />
                                    <DetailField
                                      label={t("logs.detailThinkingType")}
                                      value={row.thinking_type || t("common.none")}
                                      mono
                                    />
                                    <DetailField
                                      label={t("logs.detailMaxTokens")}
                                      value={
                                        (row.max_tokens ?? 0) > 0
                                          ? (row.max_tokens ?? 0).toLocaleString()
                                          : t("common.none")
                                      }
                                      mono
                                    />
                                    <DetailField
                                      label={t("logs.detailBudget")}
                                      value={
                                        (row.budget_tokens ?? 0) > 0
                                          ? (row.budget_tokens ?? 0).toLocaleString()
                                          : t("common.none")
                                      }
                                      mono
                                    />
                                    {row.error_class ? (
                                      <DetailField
                                        label={t("logs.detailError")}
                                        value={row.error_class}
                                        mono
                                      />
                                    ) : null}
                                  </div>

                                  {!usageKnown ? (
                                    <div className="border-t-2 border-border bg-accent-yellow px-3 py-2 text-[11px] font-medium leading-snug text-black">
                                      {t("logs.detailUsageHint")}
                                    </div>
                                  ) : null}
                                </div>
                              </td>
                            </tr>
                          ) : null}
                        </Fragment>
                      );
                    })}
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
  /** Drop stale responses when the user switches accounts quickly. */
  const loadSeqRef = useRef(0);

  async function load(selectedAccountId?: string) {
    const seq = ++loadSeqRef.current;
    setLoading(true);
    try {
      const accountList = await api.accounts();
      if (seq !== loadSeqRef.current) return;
      setAccounts(accountList.accounts);
      const resolvedId =
        selectedAccountId ??
        (accountId || accountList.accounts[0]?.id || "");
      if (resolvedId && resolvedId !== accountId) {
        setAccountId(resolvedId);
      }
      const usage = await api.usage(
        resolvedId ? { account_id: resolvedId, limit: 500 } : { limit: 500 },
      );
      if (seq !== loadSeqRef.current) return;
      setRecords(usage.records);
      setError(null);
    } catch (err) {
      if (seq !== loadSeqRef.current) return;
      if (handleUnauthorized(err, (to) => void navigate(to))) return;
      setError(err instanceof Error ? err.message : t("common.loadFailed"));
    } finally {
      if (seq === loadSeqRef.current) {
        setLoading(false);
      }
    }
  }

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // When the user switches account, re-scope the table via API + client filter.
  const accountFilterReady = useRef(false);
  useEffect(() => {
    if (!accountId) return;
    if (!accountFilterReady.current) {
      accountFilterReady.current = true;
      return;
    }
    void load(accountId);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [accountId]);

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
      await load(accountId);
    } catch (err) {
      if (handleUnauthorized(err, (to) => void navigate(to))) return;
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
      // Account filter must actually scope rows (mirrors OllamaUsagePanel).
      if (accountId && row.account_id !== accountId) return false;
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
  }, [records, query, modelFilter, sortKey, accountId]);

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
          icon={Database}
          iconTone="yellow"
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
                    <tr className="border-b-2 border-border bg-paper-2 text-caption text-ink-muted">
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
                        className="border-b-2 border-border last:border-b-0 hover:bg-accent-soft"
                      >
                        <td
                          className="px-4 py-3 font-mono text-[12px] tabular-nums text-ink-muted whitespace-nowrap"
                          title={row.recorded_at}
                        >
                          {formatDateTime(row.recorded_at)}
                        </td>
                        <td className="px-4 py-3 font-mono text-[13px] text-ink">
                          <span className="inline-flex min-w-0 items-center gap-2.5">
                            <EntityMark name={row.model} size="sm" />
                            <span className="truncate">{row.model}</span>
                          </span>
                        </td>
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
      if (handleUnauthorized(err, (to) => void navigate(to))) return;
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
          icon={Database}
          iconTone="teal"
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
                    <tr className="border-b-2 border-border bg-paper-2 text-caption text-ink-muted">
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
                        className="border-b-2 border-border last:border-b-0 hover:bg-accent-soft"
                      >
                        <td className="px-4 py-3 font-medium text-ink">
                          <span className="inline-flex min-w-0 items-center gap-2.5">
                            <EntityMark name={row.accountName} size="sm" />
                            <span className="truncate">{row.accountName}</span>
                          </span>
                        </td>
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

      <div className={tab === "gateway" ? "block" : "hidden"}>
        <GatewayLogsPanel t={t} />
      </div>
      <div className={tab === "usage-oc" ? "block" : "hidden"}>
        <UsagePanel t={t} />
      </div>
      <div className={tab === "usage-ol" ? "block" : "hidden"}>
        <OllamaUsagePanel t={t} />
      </div>
    </div>
  );
}
