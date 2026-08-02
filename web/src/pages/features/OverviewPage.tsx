import {
  ChartDonut,
  ChartLineUp,
  Coins,
  Lightning,
  Pulse,
} from "@phosphor-icons/react";
import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { cn } from "@/lib/cn";
import {
  Button,
  DateRangePicker,
  EmptyState,
  ErrorState,
  MetricRail,
  PageHeader,
  presetRange,
  rangeDayCount,
  SectionPanel,
  Skeleton,
  type DateRangeValue,
} from "@/components";
import { StatusStackBar } from "@/components/charts";
import {
  ModelCallTrendChart,
  ModelRankBars,
  ModelShareDonut,
  modelColor,
  type ModelRankItem,
  type ModelSeriesPoint,
  type ModelShareSlice,
} from "@/components/model-charts";
import {
  api,
  ApiError,
  type LogDTO,
  type OpsKPIsDTO,
  type OverviewDTO,
  type UsageRecordDTO,
  type ZenPoolSummaryDTO,
} from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";
import { useI18n, type Lang, type Translate } from "@/lib/i18n";
import {
  bucketHintKey,
  bucketKeyFor,
  buildBucketAxis,
  resolveBucketKind,
  type BucketKind,
} from "@/lib/time-buckets";

function formatUpdatedAt(lang: Lang, t: Translate, value?: string): string {
  if (!value) return t("overview.justSynced");
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString(lang === "zh" ? "zh-CN" : "en-US", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/** Overview pulls a larger window; server may still truncate. */
const LOG_FETCH_LIMIT = 5000;
const USAGE_FETCH_LIMIT = 5000;

function formatCompact(value: number): string {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}k`;
  return String(value);
}

function rangeLabel(range: DateRangeValue, t: Translate): string {
  switch (range.preset) {
    case "today":
      return t("overview.range.today");
    case "7d":
      return t("overview.range.7d");
    case "week":
      return t("overview.range.week");
    case "30d":
      return t("overview.range.30d");
    case "month":
      return t("overview.range.month");
    default:
      return t("overview.range.days", {
        n: rangeDayCount(range.from, range.to),
      });
  }
}

function inRange(iso: string, range: DateRangeValue): boolean {
  const t = new Date(iso);
  if (Number.isNaN(t.getTime())) return false;
  const ms = t.getTime();
  return ms >= range.from.getTime() && ms <= range.to.getTime();
}

function filterLogs(logs: ReadonlyArray<LogDTO>, range: DateRangeValue): LogDTO[] {
  return logs.filter((log) => inRange(log.created_at, range));
}

/** Client-side ops KPIs from request logs in the selected range. */
function buildOpsFromLogs(logs: ReadonlyArray<LogDTO>, range: DateRangeValue): OpsKPIsDTO {
  const scoped = filterLogs(logs, range);
  let s2xx = 0;
  let s429 = 0;
  let s4xx = 0;
  let s5xx = 0;
  const latencies: number[] = [];
  for (const log of scoped) {
    if (log.status === 429) s429 += 1;
    else if (log.status >= 500) s5xx += 1;
    else if (log.status >= 400 && log.status < 500) s4xx += 1;
    else if (log.status >= 200 && log.status < 300) s2xx += 1;
    if (typeof log.latency_ms === "number" && Number.isFinite(log.latency_ms)) {
      latencies.push(log.latency_ms);
    }
  }
  const requests = scoped.length;
  latencies.sort((a, b) => a - b);
  const percentile = (p: number): number | null => {
    if (latencies.length === 0) return null;
    if (latencies.length === 1) return latencies[0]!;
    const rank = Math.ceil(p * latencies.length);
    const idx = Math.min(latencies.length, Math.max(1, rank)) - 1;
    return latencies[idx]!;
  };
  const p50 = percentile(0.5);
  const p95 = percentile(0.95);
  const result: OpsKPIsDTO = {
    window: "range",
    requests,
    status_2xx: s2xx,
    status_429: s429,
    status_4xx: s4xx,
    status_5xx: s5xx,
  };
  if (requests > 0) {
    return {
      ...result,
      success_rate: s2xx / requests,
      ...(p50 != null ? { latency_p50_ms: p50 } : {}),
      ...(p95 != null ? { latency_p95_ms: p95 } : {}),
    };
  }
  return result;
}

function periodTokens(records: ReadonlyArray<UsageRecordDTO>, range: DateRangeValue): number {
  let sum = 0;
  for (const r of records) {
    if (!inRange(r.recorded_at, range)) continue;
    sum += (r.input_tokens ?? 0) + (r.output_tokens ?? 0);
  }
  return sum;
}

type ModelAnalytics = {
  readonly series: ModelSeriesPoint[];
  readonly models: string[];
  readonly colors: string[];
  readonly slices: ModelShareSlice[];
  readonly ranks: ModelRankItem[];
  readonly totalCalls: number;
  readonly bucketKind: BucketKind;
};

/** R-B: all models in range; adaptive time buckets (no Top-N / synthetic "other"). */
function buildModelAnalytics(
  logs: ReadonlyArray<LogDTO>,
  range: DateRangeValue,
): ModelAnalytics {
  const fromMs = range.from.getTime();
  const toMs = range.to.getTime();
  const bucketKind = resolveBucketKind(range.from, range.to);
  const axis = buildBucketAxis(range.from, range.to, bucketKind);
  const axisKeys = axis.map((a) => a.key);
  const labelByKey = new Map(axis.map((a) => [a.key, a.label]));
  const keySet = new Set(axisKeys);

  const modelTotals = new Map<string, number>();
  const bucketModel = new Map<string, Map<string, number>>();
  for (const k of axisKeys) bucketModel.set(k, new Map());

  for (const log of logs) {
    const t = new Date(log.created_at);
    if (Number.isNaN(t.getTime())) continue;
    const ms = t.getTime();
    if (ms < fromMs || ms > toMs) continue;
    const key = bucketKeyFor(t, bucketKind);
    if (!keySet.has(key)) continue;
    const model = (log.model || "unknown").trim() || "unknown";
    modelTotals.set(model, (modelTotals.get(model) ?? 0) + 1);
    const bucket = bucketModel.get(key);
    if (!bucket) continue;
    bucket.set(model, (bucket.get(model) ?? 0) + 1);
  }

  // Rank by call volume for stable color index; do not drop any model (R-B).
  const rankedAll = [...modelTotals.entries()].sort((a, b) => b[1] - a[1]);
  const seriesModels = rankedAll.map(([m]) => m);
  const seriesModelSet = new Set(seriesModels);
  const colors = seriesModels.map((_, i) => modelColor(i));
  const colorByModel = new Map(seriesModels.map((m, i) => [m, colors[i]!]));

  const series: ModelSeriesPoint[] = axisKeys.map((key) => {
    const bucket = bucketModel.get(key) ?? new Map();
    const label = labelByKey.get(key) ?? key;
    const row: Record<string, string | number> = { day: label, total: 0 };
    for (const m of seriesModels) row[m] = 0;
    let total = 0;
    for (const [model, count] of bucket) {
      total += count;
      if (seriesModelSet.has(model)) {
        row[model] = (row[model] as number) + count;
      }
    }
    row["total"] = total;
    return row as ModelSeriesPoint;
  });

  const slices: ModelShareSlice[] = rankedAll.map(([model, value], i) => ({
    model,
    value,
    color: colorByModel.get(model) ?? modelColor(i),
  }));

  const ranks: ModelRankItem[] = rankedAll.map(([model, value], i) => ({
    model,
    value,
    color: colorByModel.get(model) ?? modelColor(i),
  }));

  const totalCalls = rankedAll.reduce((s, [, n]) => s + n, 0);

  return {
    series,
    models: seriesModels,
    colors,
    slices,
    ranks,
    totalCalls,
    bucketKind,
  };
}

function formatSuccessRate(rate: number | null | undefined, requests: number): string {
  if (requests <= 0 || rate == null) return "-";
  return `${(rate * 100).toFixed(1)}%`;
}

function formatLatencyMs(t: Translate, value: number | null | undefined): string {
  if (value == null) return "-";
  return t("overview.opsKpis.ms", { n: value });
}

/** 运维 KPI：跟随全局日期范围，无独立时窗切换。 */
function HealthBlock({
  kpis,
  rangeText,
  t,
}: {
  readonly kpis: OpsKPIsDTO;
  readonly rangeText: string;
  readonly t: Translate;
}) {
  const requests = kpis.requests ?? 0;
  const s2xx = kpis.status_2xx ?? 0;
  const s429 = kpis.status_429 ?? 0;
  const s4xx = kpis.status_4xx ?? 0;
  const s5xx = kpis.status_5xx ?? 0;
  const rate = kpis.success_rate;

  const rateToneClass =
    rate == null || requests === 0
      ? "bg-paper-2 text-ink-muted border-border"
      : rate >= 0.95
        ? "bg-accent-teal text-black border-border shadow-[1px_1px_0_var(--border)]"
        : rate >= 0.85
          ? "bg-accent-yellow text-black border-border shadow-[1px_1px_0_var(--border)]"
          : "bg-accent text-black border-border shadow-[1px_1px_0_var(--border)]";

  return (
    <SectionPanel
      title={t("overview.opsKpis.title")}
      description={t("overview.opsKpis.description", { range: rangeText })}
      icon={ChartLineUp}
      iconTone="teal"
    >
      {requests === 0 ? (
        <EmptyState
          compact
          icon={ChartLineUp}
          title={t("overview.opsKpis.noData", { range: rangeText })}
        />
      ) : (
        <div className="flex flex-col gap-4">
          {/* 4 Neo-Brutalist Metric Cards */}
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
            <div className="flex flex-col justify-between border-2 border-border bg-paper-2 p-2.5 shadow-[2px_2px_0_var(--border)]">
              <span className="font-mono text-[10px] font-bold uppercase tracking-wide text-ink-muted">
                {t("overview.opsKpis.requests")}
              </span>
              <span className="mt-1 font-mono text-[1.4rem] font-extrabold tabular-nums leading-none text-ink">
                {formatCompact(requests)}
              </span>
              <span className="mt-2 text-[10px] font-mono text-ink-faint">
                {t("overview.modelAnalytics.totalCalls")}
              </span>
            </div>

            <div className="flex flex-col justify-between border-2 border-border bg-paper-2 p-2.5 shadow-[2px_2px_0_var(--border)]">
              <span className="font-mono text-[10px] font-bold uppercase tracking-wide text-ink-muted">
                {t("overview.opsKpis.successRate")}
              </span>
              <span className="mt-1 font-mono text-[1.4rem] font-extrabold tabular-nums leading-none text-ink">
                {formatSuccessRate(kpis.success_rate, requests)}
              </span>
              <span
                className={cn(
                  "mt-2 self-start inline-block border px-1.5 py-0.2 font-mono text-[10px] font-bold",
                  rateToneClass,
                )}
              >
                {rate == null ? "-" : rate >= 0.95 ? "优秀" : rate >= 0.85 ? "良好" : "偏低"}
              </span>
            </div>

            <div className="flex flex-col justify-between border-2 border-border bg-paper-2 p-2.5 shadow-[2px_2px_0_var(--border)]">
              <span className="font-mono text-[10px] font-bold uppercase tracking-wide text-ink-muted">
                {t("overview.opsKpis.latencyP50")}
              </span>
              <span className="mt-1 font-mono text-[1.4rem] font-extrabold tabular-nums leading-none text-ink">
                {formatLatencyMs(t, kpis.latency_p50_ms)}
              </span>
              <span className="mt-2 text-[10px] font-mono text-ink-faint">
                中位数
              </span>
            </div>

            <div className="flex flex-col justify-between border-2 border-border bg-paper-2 p-2.5 shadow-[2px_2px_0_var(--border)]">
              <span className="font-mono text-[10px] font-bold uppercase tracking-wide text-ink-muted">
                {t("overview.opsKpis.latencyP95")}
              </span>
              <span className="mt-1 font-mono text-[1.4rem] font-extrabold tabular-nums leading-none text-ink">
                {formatLatencyMs(t, kpis.latency_p95_ms)}
              </span>
              <span className="mt-2 text-[10px] font-mono text-ink-faint">
                长尾 95%
              </span>
            </div>
          </div>

          {/* HTTP Status Code Breakdown */}
          <div className="border-2 border-border bg-paper-2 p-2.5">
            <div className="mb-1.5 flex items-center justify-between font-mono text-[11px]">
              <span className="font-bold uppercase tracking-wider text-ink-muted">
                HTTP 状态码分布
              </span>
              <span className="text-[10px] text-ink-faint tabular-nums">
                2xx: {s2xx} · 429: {s429} · 4xx: {s4xx} · 5xx: {s5xx}
              </span>
            </div>
            <StatusStackBar
              ariaLabel={t("overview.opsKpis.title")}
              segments={[
                {
                  label: t("overview.opsKpis.status2xx"),
                  value: s2xx,
                  color: "var(--accent-teal)",
                },
                {
                  label: t("overview.opsKpis.status429"),
                  value: s429,
                  color: "var(--accent-yellow)",
                },
                {
                  label: t("overview.opsKpis.status4xx"),
                  value: s4xx,
                  color: "var(--accent-coral)",
                },
                {
                  label: t("overview.opsKpis.status5xx"),
                  value: s5xx,
                  color: "var(--accent)",
                },
              ]}
            />
          </div>
        </div>
      )}
    </SectionPanel>
  );
}

function ZenPoolStrip({
  pool,
  t,
  onOpen,
}: {
  readonly pool?: ZenPoolSummaryDTO | undefined;
  readonly t: Translate;
  readonly onOpen: () => void;
}) {
  // Always render: empty pool still fills the right column so the health row stays balanced.
  const total = pool?.total ?? 0;
  const healthy = pool?.healthy ?? 0;
  const cooled = pool?.cooled ?? 0;
  const disabled = pool?.disabled ?? 0;
  const benched = pool?.benched ?? 0;
  const abnormal = cooled + benched + disabled;
  const by = pool?.by_provider;
  const oc = by?.["opencode"] ?? { total: 0, healthy: 0, enabled: 0, cooled: 0, disabled: 0 };
  const ol = by?.["ollama"] ?? { total: 0, healthy: 0, enabled: 0, cooled: 0, disabled: 0 };

  return (
    <SectionPanel
      title={t("overview.zenPool.title")}
      description={t("overview.zenPool.liveHint")}
      icon={Coins}
      iconTone="yellow"
      actions={
        <Button variant="ghost" size="sm" onClick={onOpen}>
          {t("overview.zenPool.openPool")}
        </Button>
      }
    >
      <div className="flex flex-col gap-3.5">
        <div className="grid grid-cols-3 gap-2">
          <div className="border-2 border-border bg-paper-2 p-2 shadow-[2px_2px_0_var(--border)]">
            <p className="font-mono text-[10px] font-bold uppercase tracking-wide text-ink-muted">
              {t("overview.zenPool.totalKeys")}
            </p>
            <p className="mt-0.5 font-mono text-[1.3rem] font-extrabold tabular-nums leading-none text-ink">
              {total}
            </p>
          </div>

          <div className="border-2 border-border bg-paper-2 p-2 shadow-[2px_2px_0_var(--border)]">
            <p className="font-mono text-[10px] font-bold uppercase tracking-wide text-ink-muted">
              {t("overview.zenPool.healthy")}
            </p>
            <p className="mt-0.5 font-mono text-[1.3rem] font-extrabold tabular-nums leading-none text-accent-teal">
              {healthy}
            </p>
          </div>

          <div className="border-2 border-border bg-paper-2 p-2 shadow-[2px_2px_0_var(--border)]">
            <p className="font-mono text-[10px] font-bold uppercase tracking-wide text-ink-muted">
              {t("overview.zenPool.abnormal")}
            </p>
            <p
              className={cn(
                "mt-0.5 font-mono text-[1.3rem] font-extrabold tabular-nums leading-none",
                abnormal > 0 ? "text-accent-coral" : "text-ink-muted",
              )}
            >
              {abnormal}
            </p>
          </div>
        </div>

        <StatusStackBar
          ariaLabel={t("overview.zenPool.title")}
          emptyLabel={t("overview.zenPool.empty")}
          segments={[
            {
              label: t("overview.zenPool.healthy"),
              value: healthy,
              color: "var(--accent-teal)",
            },
            {
              label: t("overview.zenPool.cooled"),
              value: cooled,
              color: "var(--accent-yellow)",
            },
            {
              label: t("overview.zenPool.benched"),
              value: benched,
              color: "var(--accent-coral)",
            },
            {
              label: t("overview.zenPool.disabled"),
              value: disabled,
              color: "var(--border)",
            },
          ]}
        />

        <div className="flex flex-wrap items-center gap-2 border-t-2 border-border/20 pt-2.5 font-mono text-[11px]">
          <span className="font-bold text-ink-muted">{t("overview.zenPool.channelAvailability")}</span>
          <span className="inline-flex items-center gap-1.5 border-2 border-border bg-paper-2 px-2 py-0.5 shadow-[1px_1px_0_var(--border)]">
            <span
              className={cn(
                "h-2 w-2 rounded-full",
                oc.healthy > 0 ? "bg-accent-teal" : "bg-ink-faint",
              )}
            />
            <span className="font-semibold">{t("overview.zenPool.opencode")}</span>
            <span className="font-bold tabular-nums">
              {oc.healthy}/{oc.total}
            </span>
          </span>
          <span className="inline-flex items-center gap-1.5 border-2 border-border bg-paper-2 px-2 py-0.5 shadow-[1px_1px_0_var(--border)]">
            <span
              className={cn(
                "h-2 w-2 rounded-full",
                ol.healthy > 0 ? "bg-accent-teal" : "bg-ink-faint",
              )}
            />
            <span className="font-semibold">{t("overview.zenPool.ollama")}</span>
            <span className="font-bold tabular-nums">
              {ol.healthy}/{ol.total}
            </span>
          </span>
        </div>
      </div>
    </SectionPanel>
  );
}

export function OverviewPage() {
  const navigate = useNavigate();
  const { t, lang } = useI18n();
  const [data, setData] = useState<OverviewDTO | null>(null);
  const [logs, setLogs] = useState<LogDTO[]>([]);
  const [usage, setUsage] = useState<UsageRecordDTO[]>([]);
  const [dataTruncated, setDataTruncated] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [dateRange, setDateRange] = useState<DateRangeValue>(() => presetRange("7d"));

  const rangeText = useMemo(() => rangeLabel(dateRange, t), [dateRange, t]);
  const rangePickerLabels = useMemo(
    () => ({
      today: t("overview.range.today"),
      last7d: t("overview.range.7d"),
      thisWeek: t("overview.range.week"),
      last30d: t("overview.range.30d"),
      thisMonth: t("overview.range.month"),
      apply: t("overview.range.apply"),
      clear: t("overview.range.cancel"),
      start: t("overview.range.start"),
      end: t("overview.range.end"),
      placeholder: t("overview.range.placeholder"),
    }),
    [t],
  );

  const analytics = useMemo(
    () => buildModelAnalytics(logs, dateRange),
    [logs, dateRange],
  );

  const bucketText = useMemo(
    () => t(bucketHintKey(analytics.bucketKind)),
    [analytics.bucketKind, t],
  );

  const opsKpis = useMemo(() => buildOpsFromLogs(logs, dateRange), [logs, dateRange]);

  const periodRequestCount = useMemo(
    () => filterLogs(logs, dateRange).length,
    [logs, dateRange],
  );
  const periodTokenCount = useMemo(
    () => periodTokens(usage, dateRange),
    [usage, dateRange],
  );

  const rangeFromIso = dateRange.from.toISOString();
  const rangeToIso = dateRange.to.toISOString();

  async function load(opts?: { soft?: boolean }) {
    if (!opts?.soft) setLoading(true);
    try {
      const overview = await api.overview();
      setData(overview);
      setError(null);

      const from = dateRange.from.toISOString();
      const to = dateRange.to.toISOString();
      const settled = await Promise.allSettled([
        api.logs({ limit: LOG_FETCH_LIMIT, from, to }),
        api.usage({ limit: USAGE_FETCH_LIMIT, from, to }),
      ]);
      const logsRes = settled[0]?.status === "fulfilled" ? settled[0].value : null;
      const usageRes = settled[1]?.status === "fulfilled" ? settled[1].value : null;
      setLogs(logsRes?.logs ?? []);
      setUsage(usageRes?.records ?? []);
      setDataTruncated(Boolean(logsRes?.truncated || usageRes?.truncated));
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
    // Soft re-fetch when range changes so the page does not flash full skeleton.
    void load({ soft: data != null });
    // eslint-disable-next-line react-hooks/exhaustive-deps -- load closes over latest range via deps
  }, [navigate, rangeFromIso, rangeToIso]);

  if (loading) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton className="h-14 w-full" />
        <Skeleton className="h-40 w-full" />
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-72 w-full" />
        <div className="grid gap-3 lg:grid-cols-2">
          <Skeleton className="h-56 w-full" />
          <Skeleton className="h-56 w-full" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <ErrorState
        title={t("overview.loadFailed")}
        description={error}
        action={
          <Button variant="secondary" onClick={() => void load()}>
            {t("common.retry")}
          </Button>
        }
      />
    );
  }

  if (!data) return null;

  const quotaHint = (() => {
    const n = data.quota_narrative;
    if (!n) return t("overview.card.quotaRemainingHint");
    if (n.note === "sample_insufficient" || n.worst_used_pct == null) {
      return t("overview.card.quotaNarrativeSample");
    }
    return t("overview.card.quotaNarrativeHint", {
      pct: Number(n.worst_used_pct).toFixed(1),
      headroom: Number(n.headroom_pct ?? Math.max(0, 100 - n.worst_used_pct)).toFixed(1),
    });
  })();

  const volumeRail = [
    {
      label: t("overview.card.requestsPeriod", { range: rangeText }),
      value: periodRequestCount,
      hint: t("overview.card.requestsPeriodHint", { range: rangeText }),
      tone: "accent" as const,
    },
    {
      label: t("overview.card.tokensPeriod", { range: rangeText }),
      value: formatCompact(periodTokenCount),
      hint: t("overview.card.tokensPeriodHint"),
      tone: "yellow" as const,
    },
    {
      label: t("overview.card.requestsTotal"),
      value: formatCompact(data.requests_total),
      hint: t("overview.card.requestsTotalHint"),
      tone: "teal" as const,
    },
    {
      label: t("overview.card.tokensTotal"),
      value: formatCompact(data.tokens_total),
      hint: t("overview.card.tokensTotalHint"),
      tone: "white" as const,
    },
  ];

  const hasAnalytics = analytics.totalCalls > 0;

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title={t("overview.title")}
        meta={t("overview.updatedAt", {
          time: formatUpdatedAt(lang, t, data.updated_at),
        })}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <DateRangePicker
              value={dateRange}
              onChange={setDateRange}
              labels={rangePickerLabels}
              lang={lang}
            />
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void load()}
              className="h-8 min-h-8 px-2.5 text-[12px] shadow-none"
              title={t("common.refresh")}
            >
              <Pulse size={14} weight="bold" aria-hidden />
              <span className="sr-only sm:not-sr-only sm:inline">{t("common.refresh")}</span>
            </Button>
          </div>
        }
      />

      {/* 1. 流量体积：区间指标随全局日期变化，累计为全量 */}
      <section aria-label={t("overview.volume.title")}>
        <div className="mb-2 flex items-end justify-between gap-3">
          <div>
            <h2 className="text-[13px] font-semibold uppercase tracking-wide text-ink-muted">
              {t("overview.volume.title")}
            </h2>
            <p className="mt-0.5 text-[12px] text-ink-faint">
              {t("overview.volume.hint", { range: rangeText })}
            </p>
          </div>
          <div className="flex items-center gap-2 text-[12px] text-ink-muted">
            <Lightning size={14} weight="fill" className="text-accent-yellow" aria-hidden />
            <span>
              {t("overview.card.quotaRemaining")}{" "}
              <span className="font-mono font-semibold tabular-nums text-ink">
                {Number(data.quota_effective_remaining ?? 0).toFixed(1)}%
              </span>
            </span>
            <span className="hidden text-ink-faint sm:inline">· {quotaHint}</span>
          </div>
        </div>
        <MetricRail items={volumeRail} />
      </section>

      {/* 2. 健康：KPI 跟区间；密钥池为实时态 */}
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <HealthBlock kpis={opsKpis} rangeText={rangeText} t={t} />
        <ZenPoolStrip
          pool={data.zen_pool}
          t={t}
          onOpen={() => void navigate("/app/key-pool")}
        />
      </div>

      {dataTruncated ? (
        <p
          role="status"
          className="border-2 border-border bg-accent-yellow/20 px-3 py-2 font-mono text-[12px] text-ink"
        >
          {t("overview.dataTruncated")}
        </p>
      ) : null}

      {/* 3. 模型调用分析：同一区间 + 自适应时间桶 */}
      <SectionPanel
        title={t("overview.modelAnalytics.trendTitle")}
        description={t("overview.modelAnalytics.trendDesc", {
          range: rangeText,
          bucket: bucketText,
        })}
        icon={ChartLineUp}
        iconTone="yellow"
        actions={
          <Button variant="ghost" onClick={() => void navigate("/app/logs")}>
            {t("overview.modelAnalytics.viewLogs")}
          </Button>
        }
        {...(!hasAnalytics ? { bodyClassName: "p-0" } : {})}
      >
        {hasAnalytics ? (
          <ModelCallTrendChart
            data={analytics.series}
            models={analytics.models}
            colors={analytics.colors}
            totalLabel={t("overview.modelAnalytics.totalCalls")}
          />
        ) : (
          <EmptyState
            compact
            icon={ChartLineUp}
            title={t("overview.modelAnalytics.emptyTitle")}
            description={t("overview.modelAnalytics.emptyDescription", {
              range: rangeText,
            })}
            action={
              <Button variant="secondary" onClick={() => void navigate("/app/logs")}>
                {t("overview.modelAnalytics.viewLogs")}
              </Button>
            }
          />
        )}
      </SectionPanel>

      {hasAnalytics ? (
        <div className="grid gap-4 lg:grid-cols-2">
          <SectionPanel
            title={t("overview.modelAnalytics.shareTitle")}
            description={t("overview.modelAnalytics.shareDesc", { range: rangeText })}
            icon={ChartDonut}
            iconTone="teal"
          >
            <ModelShareDonut
              slices={analytics.slices}
              centerLabel={t("overview.modelAnalytics.centerLabel")}
              centerValue={formatCompact(analytics.totalCalls)}
              callsUnit={t("overview.modelAnalytics.callsUnit")}
              otherLabel={t("overview.modelAnalytics.other")}
            />
          </SectionPanel>

          <SectionPanel
            title={t("overview.modelAnalytics.rankTitle")}
            description={t("overview.modelAnalytics.rankDesc", { range: rangeText })}
            icon={ChartLineUp}
            iconTone="default"
          >
            <ModelRankBars
              items={analytics.ranks}
              unitLabel={t("overview.modelAnalytics.callsUnit")}
            />
          </SectionPanel>
        </div>
      ) : null}
    </div>
  );
}
