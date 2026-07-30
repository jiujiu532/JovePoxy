import {
  ArrowRight,
  ChartBar,
  ChartLineUp,
  Coins,
  Lightning,
  Path,
  Pulse,
  Stack,
  WarningCircle,
  Heartbeat,
  Key,
  Globe,
  ChartLine,
} from "@phosphor-icons/react";
import type { Icon } from "@phosphor-icons/react";
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Badge,
  Button,
  EmptyState,
  ErrorState,
  PageHeader,
  SectionPanel,
  Skeleton,
  StatCard,
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
  type MetricsDTO,
  type OverviewDTO,
  type UsageRecordDTO,
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

type QuickAction = {
  readonly to: string;
  readonly label: string;
  readonly desc: string;
  readonly icon: Icon;
  readonly tone: string;
};

export function OverviewPage() {
  const navigate = useNavigate();
  const { t, lang } = useI18n();
  const [data, setData] = useState<OverviewDTO | null>(null);
  const [metrics, setMetrics] = useState<MetricsDTO | null>(null);
  const [requestTrend, setRequestTrend] = useState<TrendPoint[]>([]);
  const [tokenTrend, setTokenTrend] = useState<TrendPoint[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  async function load() {
    setLoading(true);
    try {
      const [overview, metricsPayload] = await Promise.all([
        api.overview(),
        api.metrics(),
      ]);
      setData(overview);
      setMetrics(metricsPayload);
      setError(null);

      const [logsRes, usageRes] = await Promise.allSettled([
        api.logs(),
        api.usage(),
      ]);
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

  useEffect(() => {
    void load();
  }, [navigate]);

  if (loading) {
    return (
      <div className="flex flex-col gap-5">
        <Skeleton className="h-16 w-full" />
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 6 }).map((_, index) => (
            <Skeleton key={index} className="h-28 w-full" />
          ))}
        </div>
        <div className="grid gap-3 lg:grid-cols-2">
          <Skeleton className="h-56 w-full" />
          <Skeleton className="h-56 w-full" />
        </div>
        <Skeleton className="h-48 w-full" />
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

  const total2xx = metrics?.status_2xx ?? 0;
  const total429 = metrics?.status_429 ?? 0;
  const total5xx = metrics?.status_5xx ?? 0;
  const totalReq = metrics?.total_requests ?? data.requests_total;
  const successRate =
    totalReq > 0 ? `${((total2xx / totalReq) * 100).toFixed(1)}%` : "-";
  const modelMaxRequests = Math.max(1, ...data.by_model.map((m) => m.requests));

  const cards = [
    {
      label: t("overview.card.requestsToday"),
      value: data.requests_today,
      hint: t("overview.card.requestsTodayHint"),
      icon: Pulse,
      tone: "accent" as const,
    },
    {
      label: t("overview.card.tokensToday"),
      value: data.tokens_today,
      hint: t("overview.card.tokensTodayHint"),
      icon: Lightning,
      tone: "yellow" as const,
    },
    {
      label: t("overview.card.requestsTotal"),
      value: data.requests_total,
      hint: t("overview.card.requestsTotalHint"),
      icon: ChartLineUp,
      tone: "teal" as const,
    },
    {
      label: t("overview.card.tokensTotal"),
      value: data.tokens_total,
      hint: t("overview.card.tokensTotalHint"),
      icon: Stack,
      tone: "default" as const,
    },
    {
      label: t("overview.card.quotaRemaining"),
      value: `${Number(data.quota_effective_remaining ?? 0).toFixed(1)}%`,
      hint: t("overview.card.quotaRemainingHint"),
      icon: Coins,
      tone: "success" as const,
    },
    {
      label: t("overview.card.status429"),
      value: total429,
      hint:
        total5xx > 0
          ? t("overview.card.status429HintWith5xx", { n: total5xx })
          : t("overview.card.status429Hint"),
      icon: WarningCircle,
      tone: total429 > 0 ? ("warning" as const) : ("default" as const),
    },
  ];

  const quickActions: QuickAction[] = [
    {
      to: "/app/local-keys",
      label: t("overview.quickActions.createLocalKeyLabel"),
      desc: t("overview.quickActions.createLocalKeyDesc"),
      icon: Key,
      tone: "bg-accent-yellow text-black",
    },
    {
      to: "/app/proxies",
      label: t("overview.quickActions.configureProxyLabel"),
      desc: t("overview.quickActions.configureProxyDesc"),
      icon: Globe,
      tone: "bg-accent-teal text-black",
    },
    {
      to: "/app/quotas",
      label: t("overview.quickActions.quotaMonitorLabel"),
      desc: t("overview.quickActions.quotaMonitorDesc"),
      icon: ChartLine,
      tone: "bg-accent-mint text-black",
    },
  ];

  return (
    <div className="flex flex-col gap-5">
      <PageHeader
        title={t("overview.title")}
        meta={t("overview.updatedAt", {
          time: formatUpdatedAt(lang, t, data.updated_at),
        })}
        actions={
          <Button variant="secondary" onClick={() => void load()}>
            <Pulse size={16} weight="bold" className="mr-1.5" aria-hidden />
            {t("common.refresh")}
          </Button>
        }
      />

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {cards.map((card) => (
          <StatCard
            key={card.label}
            label={card.label}
            value={card.value}
            hint={card.hint}
            icon={card.icon}
            tone={card.tone}
          />
        ))}
      </div>

      <div className="grid gap-3 lg:grid-cols-2">
        <SectionPanel
          title={t("overview.requestTrend.title")}
          description={t("overview.requestTrend.description", { days: TREND_DAYS })}
          icon={ChartLineUp}
          iconTone="yellow"
        >
          {requestTrend.some((p) => p.value > 0) ? (
            <HardLineChart
              points={requestTrend}
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
          description={t("overview.tokenTrend.description", { days: TREND_DAYS })}
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

      <div className="grid gap-3 lg:grid-cols-[1.2fr_0.8fr]">
        <SectionPanel
          title={t("overview.health.title")}
          description={t("overview.health.description")}
          icon={Heartbeat}
          iconTone="accent"
          actions={<Badge kind="healthy">{t("overview.health.live")}</Badge>}
        >
          <StatusStackBar
            ariaLabel={t("overview.health.ariaLabel")}
            segments={[
              { label: "2xx", value: total2xx, color: "var(--accent-teal)" },
              { label: "429", value: total429, color: "var(--accent-yellow)" },
              { label: "5xx", value: total5xx, color: "var(--accent)" },
            ]}
          />
          <div className="mt-4 grid gap-3 sm:grid-cols-3">
            {[
              {
                label: t("overview.health.successRate"),
                value: successRate,
                tone: "bg-accent-mint",
              },
              {
                label: t("overview.health.streamRequests"),
                value: metrics?.stream_requests ?? 0,
                tone: "bg-accent-yellow",
              },
              {
                label: t("overview.health.status5xx"),
                value: total5xx,
                tone: "bg-accent-soft",
              },
            ].map((item) => (
              <div
                key={item.label}
                className="relative overflow-hidden border-2 border-border bg-paper-0 px-3 py-3 shadow-[2px_2px_0_var(--border)]"
              >
                <span
                  className={cn("absolute inset-y-0 left-0 w-1", item.tone)}
                  aria-hidden
                />
                <p className="pl-2 text-caption text-ink-muted">{item.label}</p>
                <p className="mt-1.5 pl-2 text-xl font-semibold tabular-nums text-ink">
                  {item.value}
                </p>
              </div>
            ))}
          </div>
        </SectionPanel>

        <SectionPanel
          title={t("overview.quickActions.title")}
          description={t("overview.quickActions.description")}
          icon={Path}
          iconTone="yellow"
        >
          <div className="flex flex-col gap-2.5">
            {quickActions.map((item) => {
              const IconComp = item.icon;
              return (
                <Link
                  key={item.to}
                  to={item.to}
                  className={cn(
                    "group flex items-center gap-3 border-2 border-border bg-paper-0 px-3 py-3",
                    "shadow-[2px_2px_0_var(--border)]",
                    "transition-[transform,background-color,box-shadow] duration-150",
                    "hover:-translate-x-px hover:-translate-y-px hover:bg-accent-soft hover:shadow-[3px_3px_0_var(--border)]",
                    "active:translate-x-px active:translate-y-px active:shadow-none",
                  )}
                >
                  <span
                    className={cn(
                      "inline-flex h-10 w-10 shrink-0 items-center justify-center border-2 border-border",
                      "shadow-[2px_2px_0_var(--border)]",
                      item.tone,
                    )}
                    aria-hidden
                  >
                    <IconComp size={18} weight="duotone" />
                  </span>
                  <div className="min-w-0 flex-1">
                    <p className="text-[13px] font-semibold text-ink">{item.label}</p>
                    <p className="mt-0.5 text-[12px] text-ink-muted">{item.desc}</p>
                  </div>
                  <span className="inline-flex items-center gap-1 text-[11px] font-semibold uppercase tracking-wide text-ink-faint transition-colors group-hover:text-ink">
                    {t("overview.quickActions.open")}
                    <ArrowRight
                      size={14}
                      weight="bold"
                      className="transition-transform group-hover:translate-x-0.5"
                      aria-hidden
                    />
                  </span>
                </Link>
              );
            })}
          </div>
        </SectionPanel>
      </div>

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
                  <p className="text-caption font-medium text-ink-muted">
                    {window.label}
                  </p>
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
                      {row.input_tokens}
                    </td>
                    <td className="py-2.5 tabular-nums text-ink-muted">
                      {row.output_tokens}
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
