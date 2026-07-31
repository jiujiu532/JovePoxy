import {
  ChartBar,
  ChartLineUp,
  Coins,
  Lightning,
  Pulse,
  Stack,
} from "@phosphor-icons/react";
import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Badge,
  Button,
  EmptyState,
  ErrorState,
  MetricRail,
  PageHeader,
  SectionPanel,
  Skeleton,
} from "@/components";
import {
  HardBarChart,
  HardLineChart,
  ShareBar,
  StatusStackBar,
  type TrendPoint,
} from "@/components/charts";
import {
  api,
  ApiError,
  type LogDTO,
  type OpsKPIsDTO,
  type OpsWindow,
  type OverviewDTO,
  type UsageRecordDTO,
  type ZenPoolSummaryDTO,
} from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";
import { cn } from "@/lib/cn";
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

const TREND_DAYS = 7;

function dayKey(date: Date): string {
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${m}/${d}`;
}

function recentDays(): string[] {
  const days: string[] = [];
  const now = new Date();
  for (let i = TREND_DAYS - 1; i >= 0; i -= 1) {
    const d = new Date(now);
    d.setDate(now.getDate() - i);
    days.push(dayKey(d));
  }
  return days;
}

function buildRequestTrend(logs: ReadonlyArray<LogDTO>): TrendPoint[] {
  const bucket = new Map<string, number>(recentDays().map((d) => [d, 0]));
  for (const log of logs) {
    const t = new Date(log.created_at);
    if (Number.isNaN(t.getTime())) continue;
    const key = dayKey(t);
    if (bucket.has(key)) bucket.set(key, (bucket.get(key) ?? 0) + 1);
  }
  return [...bucket.entries()].map(([label, value]) => ({ label, value }));
}

function buildTokenTrend(records: ReadonlyArray<UsageRecordDTO>): TrendPoint[] {
  const bucket = new Map<string, number>(recentDays().map((d) => [d, 0]));
  for (const r of records) {
    const t = new Date(r.recorded_at);
    if (Number.isNaN(t.getTime())) continue;
    const key = dayKey(t);
    if (bucket.has(key)) {
      bucket.set(key, (bucket.get(key) ?? 0) + r.input_tokens + r.output_tokens);
    }
  }
  return [...bucket.entries()].map(([label, value]) => ({ label, value }));
}

function formatCompact(value: number): string {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}k`;
  return String(value);
}

const OPS_WINDOWS: ReadonlyArray<OpsWindow> = ["1h", "24h", "7d"];

function windowLabelKey(
  window: OpsWindow,
): "overview.opsKpis.window1h" | "overview.opsKpis.window24h" | "overview.opsKpis.window7d" {
  switch (window) {
    case "1h":
      return "overview.opsKpis.window1h";
    case "7d":
      return "overview.opsKpis.window7d";
    default:
      return "overview.opsKpis.window24h";
  }
}

function formatSuccessRate(rate: number | null | undefined, requests: number): string {
  if (requests <= 0 || rate == null) return "-";
  return `${(rate * 100).toFixed(1)}%`;
}

function formatLatencyMs(t: Translate, value: number | null | undefined): string {
  if (value == null) return "-";
  return t("overview.opsKpis.ms", { n: value });
}

function WindowToggle({
  window,
  onWindowChange,
  loading,
  t,
}: {
  readonly window: OpsWindow;
  readonly onWindowChange: (w: OpsWindow) => void;
  readonly loading: boolean;
  readonly t: Translate;
}) {
  return (
    <div
      role="group"
      aria-label={t("overview.opsKpis.window")}
      className="inline-flex items-center gap-0.5 rounded-none border-2 border-border bg-paper-0 p-0.5"
    >
      {OPS_WINDOWS.map((w) => {
        const active = window === w;
        return (
          <button
            key={w}
            type="button"
            aria-pressed={active}
            disabled={loading}
            className={cn(
              "inline-flex h-8 items-center rounded-none px-2.5 text-[12px] font-medium transition-[background-color,color] duration-150",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus-ring focus-visible:ring-offset-1 focus-visible:ring-offset-paper-0",
              active
                ? "bg-paper-1 text-ink shadow-[2px_2px_0_var(--border)] ring-1 ring-border"
                : "text-ink-muted hover:bg-paper-1/70 hover:text-ink",
              loading && "opacity-60",
            )}
            onClick={() => onWindowChange(w)}
          >
            {t(windowLabelKey(w))}
          </button>
        );
      })}
    </div>
  );
}

/** 第一眼：健康 + 延迟，时窗可切换。 */
function HealthBlock({
  kpis,
  window,
  onWindowChange,
  loading,
  t,
}: {
  readonly kpis?: OpsKPIsDTO | undefined;
  readonly window: OpsWindow;
  readonly onWindowChange: (w: OpsWindow) => void;
  readonly loading: boolean;
  readonly t: Translate;
}) {
  const requests = kpis?.requests ?? 0;
  const s2xx = kpis?.status_2xx ?? 0;
  const s429 = kpis?.status_429 ?? 0;
  const s5xx = kpis?.status_5xx ?? 0;

  return (
    <SectionPanel
      title={t("overview.opsKpis.title")}
      icon={ChartLineUp}
      iconTone="teal"
      actions={
        <WindowToggle
          window={window}
          onWindowChange={onWindowChange}
          loading={loading}
          t={t}
        />
      }
    >
      {requests === 0 ? (
        <EmptyState compact icon={ChartLineUp} title={t("overview.opsKpis.noData")} />
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
                value: formatSuccessRate(kpis?.success_rate, requests),
              },
              {
                label: t("overview.opsKpis.latencyP50"),
                value: formatLatencyMs(t, kpis?.latency_p50_ms),
              },
              {
                label: t("overview.opsKpis.latencyP95"),
                value: formatLatencyMs(t, kpis?.latency_p95_ms),
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
  const [requestTrend, setRequestTrend] = useState<TrendPoint[]>([]);
  const [tokenTrend, setTokenTrend] = useState<TrendPoint[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [opsWindow, setOpsWindow] = useState<OpsWindow>("24h");
  const [opsLoading, setOpsLoading] = useState(false);

  async function load(window: OpsWindow = opsWindow) {
    setLoading(true);
    try {
      const overview = await api.overview(window);
      setData(overview);
      setError(null);

      const [logsRes, usageRes] = await Promise.allSettled([api.logs(), api.usage()]);
      setRequestTrend(
        logsRes.status === "fulfilled" ? buildRequestTrend(logsRes.value.logs) : [],
      );
      setTokenTrend(
        usageRes.status === "fulfilled"
          ? buildTokenTrend(usageRes.value.records)
          : [],
      );
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

  async function changeOpsWindow(next: OpsWindow) {
    if (next === opsWindow) return;
    setOpsWindow(next);
    setOpsLoading(true);
    try {
      const overview = await api.overview(next);
      setData(overview);
      setError(null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setSessionHint(false);
        void navigate("/login");
        return;
      }
      setError(err instanceof Error ? err.message : t("common.loadFailed"));
    } finally {
      setOpsLoading(false);
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
          <Button variant="secondary" onClick={() => void load(opsWindow)}>
            {t("common.retry")}
          </Button>
        }
      />
    );
  }

  if (!data) return null;

  const modelMaxRequests = Math.max(1, ...data.by_model.map((m) => m.requests));

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
      label: t("overview.card.requestsToday"),
      value: data.requests_today,
      hint: t("overview.card.requestsTodayHint"),
      tone: "accent" as const,
    },
    {
      label: t("overview.card.tokensToday"),
      value: formatCompact(data.tokens_today),
      hint: t("overview.card.tokensTodayHint"),
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

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title={t("overview.title")}
        meta={t("overview.updatedAt", {
          time: formatUpdatedAt(lang, t, data.updated_at),
        })}
        actions={
          <Button variant="secondary" onClick={() => void load(opsWindow)}>
            <Pulse size={16} weight="bold" className="mr-1.5" aria-hidden />
            {t("common.refresh")}
          </Button>
        }
      />

      {/* 1. 流量体积置顶：今日 + 累计，一条轨道 */}
      <section aria-label={t("overview.volume.title")}>
        <div className="mb-2 flex items-end justify-between gap-3">
          <div>
            <h2 className="text-[13px] font-semibold uppercase tracking-wide text-ink-muted">
              {t("overview.volume.title")}
            </h2>
            <p className="mt-0.5 text-[12px] text-ink-faint">{t("overview.volume.hint")}</p>
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

      {/* 2. 健康：KPI + 密钥池并排 */}
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <HealthBlock
          kpis={data.ops_kpis}
          window={opsWindow}
          onWindowChange={(w) => void changeOpsWindow(w)}
          loading={opsLoading}
          t={t}
        />
        <ZenPoolStrip
          pool={data.zen_pool}
          t={t}
          onOpen={() => void navigate("/app/key-pool")}
        />
      </div>

      {/* 3. 趋势：请求 + token 同屏 */}
      <div className="grid gap-4 lg:grid-cols-2">
        <SectionPanel
          title={t("overview.requestTrend.title")}
          description={t("overview.requestTrend.shortDesc", { days: TREND_DAYS })}
          icon={ChartLineUp}
          iconTone="yellow"
        >
          {requestTrend.some((p) => p.value > 0) ? (
            <HardLineChart
              points={requestTrend}
              formatValue={formatCompact}
              ariaLabel={t("overview.requestTrend.ariaLabel", { days: TREND_DAYS })}
            />
          ) : (
            <EmptyState
              compact
              icon={ChartLineUp}
              title={t("overview.requestTrend.emptyTitle")}
              description={t("overview.requestTrend.emptyDescription")}
            />
          )}
        </SectionPanel>

        <SectionPanel
          title={t("overview.tokenTrend.title")}
          description={t("overview.tokenTrend.shortDesc", { days: TREND_DAYS })}
          icon={ChartBar}
          iconTone="teal"
        >
          {tokenTrend.some((p) => p.value > 0) ? (
            <HardBarChart
              points={tokenTrend}
              formatValue={formatCompact}
              ariaLabel={t("overview.tokenTrend.ariaLabel", { days: TREND_DAYS })}
            />
          ) : (
            <EmptyState
              compact
              icon={ChartBar}
              title={t("overview.tokenTrend.emptyTitle")}
              description={t("overview.tokenTrend.emptyDescription")}
            />
          )}
        </SectionPanel>
      </div>

      {/* 4. 额度窗口（有数据才显示；主路径在流量体积旁注） */}
      {data.quota_windows && data.quota_windows.length > 0 ? (
        <SectionPanel
          title={t("overview.quotaWindows.title")}
          description={t("overview.quotaWindows.description")}
          icon={Coins}
          iconTone="mint"
        >
          <div className="grid gap-3 sm:grid-cols-3">
            {data.quota_windows.map((window) => (
              <div
                key={window.label}
                className="border-2 border-border bg-paper-0 p-3.5 shadow-[2px_2px_0_var(--border)]"
              >
                <div className="flex items-center justify-between gap-2">
                  <p className="text-caption font-medium text-ink-muted">{window.label}</p>
                  {window.blocked ? (
                    <Badge kind="warning">{t("overview.quotaWindows.blocked")}</Badge>
                  ) : (
                    <Badge kind="healthy">{t("overview.quotaWindows.available")}</Badge>
                  )}
                </div>
                <p className="mt-2 text-2xl font-semibold tabular-nums text-ink">
                  {Number(window.effective_remaining ?? 0).toFixed(1)}%
                </p>
                <ShareBar
                  className="mt-2 max-w-none"
                  ratio={Number(window.effective_remaining ?? 0) / 100}
                  color={
                    window.blocked ? "var(--accent-yellow)" : "var(--accent-teal)"
                  }
                />
                <p className="mt-2 text-[12px] text-ink-faint">
                  {t("overview.quotaWindows.used", {
                    percent: Number(window.used ?? 0).toFixed(1),
                  })}
                  {window.blocked_by
                    ? t("overview.quotaWindows.blockedBy", { name: window.blocked_by })
                    : ""}
                </p>
              </div>
            ))}
          </div>
        </SectionPanel>
      ) : null}

      {/* 5. 按模型明细 */}
      <SectionPanel
        title={t("overview.byModel.title")}
        description={t("overview.byModel.description")}
        icon={Stack}
        iconTone="default"
        actions={
          <Button variant="ghost" onClick={() => void navigate("/app/logs?tab=usage")}>
            {t("overview.byModel.viewUsage")}
          </Button>
        }
        {...(data.by_model.length === 0 ? { bodyClassName: "p-0" } : {})}
      >
        {data.by_model.length === 0 ? (
          <EmptyState
            compact
            icon={Stack}
            title={t("overview.byModel.emptyTitle")}
            description={t("overview.byModel.emptyDescription")}
            action={
              <Button variant="secondary" onClick={() => void navigate("/app/logs?tab=usage")}>
                {t("overview.byModel.emptyAction")}
              </Button>
            }
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[26rem] text-left text-sm md:min-w-[32rem]">
              <thead>
                <tr className="border-b-2 border-border text-caption text-ink-muted">
                  <th className="pb-2 font-medium">{t("overview.byModel.table.model")}</th>
                  <th className="pb-2 font-medium">{t("overview.byModel.table.share")}</th>
                  <th className="pb-2 font-medium">{t("overview.byModel.table.requests")}</th>
                  <th className="pb-2 font-medium">{t("overview.byModel.table.input")}</th>
                  <th className="pb-2 font-medium">{t("overview.byModel.table.output")}</th>
                </tr>
              </thead>
              <tbody>
                {data.by_model.map((row) => (
                  <tr
                    key={row.model}
                    className="border-b border-border last:border-b-0 transition-colors hover:bg-paper-0"
                  >
                    <td className="py-2.5 font-mono text-[13px] font-medium text-ink">
                      {row.model}
                    </td>
                    <td className="py-2.5 pr-3">
                      <ShareBar ratio={row.requests / modelMaxRequests} />
                    </td>
                    <td className="py-2.5 tabular-nums text-ink">{row.requests}</td>
                    <td className="py-2.5 tabular-nums text-ink-muted">
                      {formatCompact(row.input_tokens)}
                    </td>
                    <td className="py-2.5 tabular-nums text-ink-muted">
                      {formatCompact(row.output_tokens)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </SectionPanel>
    </div>
  );
}
