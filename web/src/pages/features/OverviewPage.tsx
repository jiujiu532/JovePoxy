import {
  ChartDonut,
  ChartLineUp,
  Coins,
  Lightning,
  Pulse,
} from "@phosphor-icons/react";
import { useEffect, useMemo, useState } from "react";
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

/** Max distinct model series on the trend chart (rest → "other"). */
const TREND_TOP_MODELS = 6;
const LOG_FETCH_LIMIT = 2000;
const USAGE_FETCH_LIMIT = 2000;

function dayKey(date: Date): string {
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${m}/${d}`;
}

function startOfLocalDay(d: Date): Date {
  const x = new Date(d);
  x.setHours(0, 0, 0, 0);
  return x;
}

/** Inclusive calendar-day labels between from/to (local). Cap at 62 for chart density. */
function daysInRange(from: Date, to: Date): string[] {
  const start = startOfLocalDay(from);
  const end = startOfLocalDay(to);
  const days: string[] = [];
  const cursor = new Date(start);
  let guard = 0;
  while (cursor.getTime() <= end.getTime() && guard < 62) {
    days.push(dayKey(cursor));
    cursor.setDate(cursor.getDate() + 1);
    guard += 1;
  }
  return days;
}

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
  let s5xx = 0;
  const latencies: number[] = [];
  for (const log of scoped) {
    if (log.status === 429) s429 += 1;
    else if (log.status >= 500) s5xx += 1;
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
};

function buildModelAnalytics(
  logs: ReadonlyArray<LogDTO>,
  otherLabel: string,
  range: DateRangeValue,
): ModelAnalytics {
  const days = daysInRange(range.from, range.to);
  const daySet = new Set(days);
  const fromMs = range.from.getTime();
  const toMs = range.to.getTime();

  const modelTotals = new Map<string, number>();
  const dayModel = new Map<string, Map<string, number>>();
  for (const d of days) dayModel.set(d, new Map());

  for (const log of logs) {
    const t = new Date(log.created_at);
    if (Number.isNaN(t.getTime())) continue;
    const ms = t.getTime();
    if (ms < fromMs || ms > toMs) continue;
    const key = dayKey(t);
    if (!daySet.has(key)) continue;
    const model = (log.model || "unknown").trim() || "unknown";
    modelTotals.set(model, (modelTotals.get(model) ?? 0) + 1);
    const bucket = dayModel.get(key);
    if (!bucket) continue;
    bucket.set(model, (bucket.get(model) ?? 0) + 1);
  }

  const rankedAll = [...modelTotals.entries()].sort((a, b) => b[1] - a[1]);
  const top = rankedAll.slice(0, TREND_TOP_MODELS).map(([m]) => m);
  const topSet = new Set(top);
  const hasOther = rankedAll.length > top.length;
  const seriesModels = hasOther ? [...top, otherLabel] : top;

  const colors = seriesModels.map((_, i) => modelColor(i));
  const colorByModel = new Map(seriesModels.map((m, i) => [m, colors[i]!]));

  const series: ModelSeriesPoint[] = days.map((day) => {
    const bucket = dayModel.get(day) ?? new Map();
    const row: Record<string, string | number> = { day, total: 0 };
    for (const m of seriesModels) row[m] = 0;
    let total = 0;
    for (const [model, count] of bucket) {
      total += count;
      if (topSet.has(model)) {
        row[model] = (row[model] as number) + count;
      } else if (hasOther) {
        row[otherLabel] = (row[otherLabel] as number) + count;
      }
    }
    row["total"] = total;
    return row as ModelSeriesPoint;
  });

  const shareSource = rankedAll.slice(0, 10);
  const shareOther = rankedAll.slice(10).reduce((s, [, n]) => s + n, 0);
  const slices: ModelShareSlice[] = shareSource.map(([model, value], i) => ({
    model,
    value,
    color: modelColor(i),
  }));
  if (shareOther > 0) {
    slices.push({
      model: otherLabel,
      value: shareOther,
      color: modelColor(slices.length),
    });
  }

  const ranks: ModelRankItem[] = rankedAll.slice(0, 8).map(([model, value], i) => ({
    model,
    value,
    color: colorByModel.get(model) ?? modelColor(i),
  }));

  const totalCalls = rankedAll.reduce((s, [, n]) => s + n, 0);

  return { series, models: seriesModels, colors, slices, ranks, totalCalls };
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
  const s5xx = kpis.status_5xx ?? 0;

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
        <div className="flex flex-col gap-3">
          <div className="grid grid-cols-2 gap-px overflow-hidden border-2 border-border bg-border sm:grid-cols-4">
            {[
              {
                label: t("overview.opsKpis.requests"),
                value: requests,
              },
              {
                label: t("overview.opsKpis.successRate"),
                value: formatSuccessRate(kpis.success_rate, requests),
              },
              {
                label: t("overview.opsKpis.latencyP50"),
                value: formatLatencyMs(t, kpis.latency_p50_ms),
              },
              {
                label: t("overview.opsKpis.latencyP95"),
                value: formatLatencyMs(t, kpis.latency_p95_ms),
              },
            ].map((cell) => (
              <div key={cell.label} className="bg-paper-0 px-3 py-2.5">
                <p className="text-[11px] font-medium uppercase tracking-wide text-ink-muted">
                  {cell.label}
                </p>
                <p className="mt-1 font-mono text-[1.35rem] font-bold leading-none tabular-nums text-ink">
                  {cell.value}
                </p>
              </div>
            ))}
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
                label: t("overview.opsKpis.status5xx"),
                value: s5xx,
                color: "var(--accent)",
              },
            ]}
          />
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
  const total = pool?.total ?? 0;
  if (total === 0) return null;

  const healthy = pool?.healthy ?? 0;
  const cooled = pool?.cooled ?? 0;
  const disabled = pool?.disabled ?? 0;
  const benched = pool?.benched ?? 0;
  const by = pool?.by_provider;
  const oc = by?.["opencode"];
  const ol = by?.["ollama"];

  return (
    <SectionPanel
      title={t("overview.zenPool.title")}
      description={t("overview.zenPool.liveHint")}
      icon={Coins}
      iconTone="yellow"
      actions={
        <Button variant="ghost" onClick={onOpen}>
          {t("overview.zenPool.openPool")}
        </Button>
      }
    >
      <StatusStackBar
        ariaLabel={t("overview.zenPool.title")}
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
            color: "var(--accent)",
          },
          {
            label: t("overview.zenPool.disabled"),
            value: disabled,
            color: "var(--border)",
          },
        ]}
      />
      <p className="mt-2 text-[12px] text-ink-muted">
        {t("overview.zenPool.total", { n: total })}
        {oc || ol ? (
          <>
            {oc ? ` · ${t("overview.zenPool.opencode")} ${oc.healthy}/${oc.total}` : null}
            {ol ? ` · ${t("overview.zenPool.ollama")} ${ol.healthy}/${ol.total}` : null}
          </>
        ) : null}
      </p>
    </SectionPanel>
  );
}

export function OverviewPage() {
  const navigate = useNavigate();
  const { t, lang } = useI18n();
  const [data, setData] = useState<OverviewDTO | null>(null);
  const [logs, setLogs] = useState<LogDTO[]>([]);
  const [usage, setUsage] = useState<UsageRecordDTO[]>([]);
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
    () => buildModelAnalytics(logs, t("overview.modelAnalytics.other"), dateRange),
    [logs, t, dateRange],
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

  async function load() {
    setLoading(true);
    try {
      const overview = await api.overview();
      setData(overview);
      setError(null);

      const settled = await Promise.allSettled([
        api.logs(LOG_FETCH_LIMIT),
        api.usage(USAGE_FETCH_LIMIT),
      ]);
      setLogs(settled[0]?.status === "fulfilled" ? settled[0].value.logs : []);
      setUsage(settled[1]?.status === "fulfilled" ? settled[1].value.records : []);
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
    // eslint-disable-next-line react-hooks/exhaustive-deps -- initial mount only
  }, [navigate]);

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
            <Button variant="secondary" onClick={() => void load()}>
              <Pulse size={16} weight="bold" className="mr-1.5" aria-hidden />
              {t("common.refresh")}
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

      {/* 3. 模型调用分析：同一区间 */}
      <SectionPanel
        title={t("overview.modelAnalytics.trendTitle")}
        description={t("overview.modelAnalytics.trendDesc", { range: rangeText })}
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
