import {
  ChartDonut,
  ChartLineUp,
  Lightning,
  Pulse,
} from "@phosphor-icons/react";
import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
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
import { OpsBoard } from "@/components/OpsBoard";
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
  type LogDTO,
  type OpsKPIsDTO,
  type OverviewDTO,
  type UsageRecordDTO,
} from "@/lib/api";
import { handleUnauthorized } from "@/lib/api-error";
import {
  opsWindowForRange,
  overviewWindowForRange,
} from "@/lib/overview-ops-window";
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

export function OverviewPage() {
  const navigate = useNavigate();
  const { t, lang } = useI18n();
  const [data, setData] = useState<OverviewDTO | null>(null);
  const [logs, setLogs] = useState<LogDTO[]>([]);
  const [usage, setUsage] = useState<UsageRecordDTO[]>([]);
  const [dataTruncated, setDataTruncated] = useState(false);
  const [error, setError] = useState<string | null>(null);
  /** Soft refresh failure banner; does not replace existing data with ErrorState. */
  const [softError, setSoftError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [dateRange, setDateRange] = useState<DateRangeValue>(() => presetRange("7d"));
  /** Monotonic sequence so fast range changes drop stale responses. */
  const loadSeqRef = useRef(0);

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

  // Prefer backend ops_kpis when we requested a matching ops window; else derive from logs.
  const opsKpis = useMemo(() => {
    const requested = opsWindowForRange(dateRange);
    if (requested && data?.ops_kpis) {
      return data.ops_kpis;
    }
    return buildOpsFromLogs(logs, dateRange);
  }, [data, logs, dateRange]);

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

  async function load(opts?: { soft?: boolean; seq?: number }) {
    const seq = opts?.seq ?? ++loadSeqRef.current;
    const soft = Boolean(opts?.soft);
    if (!soft) setLoading(true);
    try {
      // One overview call: exact rolling window when possible, else nearest for routing.
      const overview = await api.overview(overviewWindowForRange(dateRange));
      if (seq !== loadSeqRef.current) return;
      setData(overview);
      setError(null);
      setSoftError(null);

      const from = dateRange.from.toISOString();
      const to = dateRange.to.toISOString();
      const settled = await Promise.allSettled([
        api.logs({ limit: LOG_FETCH_LIMIT, from, to }),
        api.usage({ limit: USAGE_FETCH_LIMIT, from, to }),
      ]);
      if (seq !== loadSeqRef.current) return;
      const logsRes = settled[0]?.status === "fulfilled" ? settled[0].value : null;
      const usageRes = settled[1]?.status === "fulfilled" ? settled[1].value : null;
      // Prefer fresh results; if a partial soft fetch fails, keep previous series data.
      if (logsRes) {
        setLogs(logsRes.logs ?? []);
      } else if (!soft) {
        setLogs([]);
      }
      if (usageRes) {
        setUsage(usageRes.records ?? []);
      } else if (!soft) {
        setUsage([]);
      }
      setDataTruncated(Boolean(logsRes?.truncated || usageRes?.truncated));
      if (soft && (!logsRes || !usageRes)) {
        setSoftError(t("common.loadFailed"));
      }
    } catch (err) {
      if (seq !== loadSeqRef.current) return;
      if (handleUnauthorized(err, (to) => void navigate(to))) return;
      const msg = err instanceof Error ? err.message : t("common.loadFailed");
      if (soft && data != null) {
        // Soft refresh: keep existing overview/logs/usage, surface a banner.
        setSoftError(msg);
      } else {
        setError(msg);
      }
    } finally {
      if (seq === loadSeqRef.current) {
        setLoading(false);
      }
    }
  }

  useEffect(() => {
    const seq = ++loadSeqRef.current;
    // Soft re-fetch when range changes so the page does not flash full skeleton.
    void load({ soft: data != null, seq });
    // eslint-disable-next-line react-hooks/exhaustive-deps -- load closes over latest range via deps
  }, [navigate, rangeFromIso, rangeToIso]);

  // Full-page skeleton only on the initial hard load (no data yet).
  if (loading && data == null) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton className="h-14 w-full" />
        <Skeleton className="h-40 w-full" />
        <Skeleton className="h-56 w-full" />
        <Skeleton className="h-72 w-full" />
        <div className="grid gap-3 lg:grid-cols-2">
          <Skeleton className="h-56 w-full" />
          <Skeleton className="h-56 w-full" />
        </div>
      </div>
    );
  }

  // Full ErrorState only when we have nothing to show.
  if (error && data == null) {
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
    <div className="flex flex-col gap-3">
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
              onClick={() => void load({ soft: true })}
              className="h-8 min-h-8 px-2.5 text-[12px] shadow-none"
              title={t("common.refresh")}
            >
              <Pulse size={14} weight="bold" aria-hidden />
              <span className="sr-only sm:not-sr-only sm:inline">{t("common.refresh")}</span>
            </Button>
          </div>
        }
      />

      {softError ? (
        <p
          role="status"
          className="border-2 border-border bg-accent-yellow/20 px-3 py-2 font-mono text-[12px] text-ink"
        >
          {softError}
          <Button
            variant="ghost"
            size="sm"
            className="ml-2 h-7 min-h-7 px-2 text-[12px]"
            onClick={() => void load({ soft: true })}
          >
            {t("common.retry")}
          </Button>
        </p>
      ) : null}

      {/* 1. 体量（主信号）：区间请求/Token + 全量 */}
      <section aria-label={t("overview.volume.title")}>
        <div className="mb-1.5 flex items-end justify-between gap-3">
          <div>
            <h2 className="text-[12px] font-semibold uppercase tracking-wide text-ink-muted">
              {t("overview.volume.title")}
            </h2>
            <p className="mt-0.5 text-[11px] text-ink-faint">
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

      {/* 2. 运维总表：密钥池 + 全局 KPI + 最终付费通道 */}
      <OpsBoard
        opsKpis={opsKpis}
        rangeText={rangeText}
        pool={data.zen_pool}
        routingKpis={data.routing_kpis}
        onOpenPool={() => void navigate("/app/key-pool")}
        t={t}
      />

      {dataTruncated ? (
        <p
          role="status"
          className="border-2 border-border bg-accent-yellow/20 px-3 py-2 font-mono text-[12px] text-ink"
        >
          {t("overview.dataTruncated")}
        </p>
      ) : null}

      {/* 3. 模型分析（下钻） */}
      <SectionPanel
        title={t("overview.modelAnalytics.trendTitle")}
        description={t("overview.modelAnalytics.trendDesc", {
          range: rangeText,
          bucket: bucketText,
        })}
        icon={ChartLineUp}
        iconTone="yellow"
        bodyClassName={hasAnalytics ? "!p-3" : "p-0"}
        actions={
          <Button variant="ghost" onClick={() => void navigate("/app/logs")}>
            {t("overview.modelAnalytics.viewLogs")}
          </Button>
        }
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
        <div className="grid gap-3 lg:grid-cols-2">
          <SectionPanel
            title={t("overview.modelAnalytics.shareTitle")}
            description={t("overview.modelAnalytics.shareDesc", { range: rangeText })}
            icon={ChartDonut}
            iconTone="teal"
            bodyClassName="!p-3"
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
            bodyClassName="!p-3"
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
