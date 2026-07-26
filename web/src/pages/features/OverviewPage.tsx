import {
  ChartBar,
  ChartLineUp,
  Coins,
  Lightning,
  Path,
  Pulse,
  Stack,
  WarningCircle,
} from "@phosphor-icons/react";
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

function formatUpdatedAt(value?: string): string {
  if (!value) return "刚刚同步";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("zh-CN", {
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

/** 近 N 天日期骨架（含今天），保证图表横轴稳定。 */
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

/** 网关日志 → 按日请求数。 */
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

/** 用量记录 → 按日 token（输入+输出）。 */
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

export function OverviewPage() {
  const navigate = useNavigate();
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

      // 趋势数据为增强信息：失败时静默降级，不阻塞概览主体。
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
      setError(err instanceof Error ? err.message : "加载失败");
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
        title="概览加载失败"
        description={error}
        action={
          <Button variant="secondary" onClick={() => void load()}>
            重试
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
      label: "今日请求",
      value: data.requests_today,
      hint: "当天完成的代理请求",
      icon: Pulse,
      tone: "accent" as const,
    },
    {
      label: "今日 Token",
      value: data.tokens_today,
      hint: "输入 + 输出合计",
      icon: Lightning,
      tone: "default" as const,
    },
    {
      label: "累计请求",
      value: data.requests_total,
      hint: "历史总量",
      icon: ChartLineUp,
      tone: "default" as const,
    },
    {
      label: "累计 Token",
      value: data.tokens_total,
      hint: "全量 token 消耗",
      icon: Stack,
      tone: "default" as const,
    },
    {
      label: "额度有效剩余",
      value: `${Number(data.quota_effective_remaining ?? 0).toFixed(1)}%`,
      hint: "cascade 后的有效值",
      icon: Coins,
      tone: "success" as const,
    },
    {
      label: "进程 429",
      value: total429,
      hint: total5xx > 0 ? `另有 ${total5xx} 次 5xx` : "限流计数",
      icon: WarningCircle,
      tone: total429 > 0 ? ("warning" as const) : ("default" as const),
    },
  ];

  return (
    <div className="flex flex-col gap-5">
      <PageHeader
        title="概览"
        description="一眼看清网关健康度、额度状态与模型消耗。"
        meta={`更新于 ${formatUpdatedAt(data.updated_at)}`}
        actions={
          <Button variant="secondary" onClick={() => void load()}>
            刷新
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
          title="请求趋势"
          description={`近 ${TREND_DAYS} 天网关请求（最近 100 条日志聚合）`}
        >
          {requestTrend.some((p) => p.value > 0) ? (
            <HardLineChart
              points={requestTrend}
              ariaLabel={`近${TREND_DAYS}天请求趋势折线图`}
            />
          ) : (
            <EmptyState
              compact
              icon={ChartLineUp}
              title="暂无请求日志"
              description="发起代理请求后这里会出现近 7 天趋势。"
            />
          )}
        </SectionPanel>

        <SectionPanel
          title="Token 消耗"
          description={`近 ${TREND_DAYS} 天 token（最近 100 条用量记录聚合）`}
        >
          {tokenTrend.some((p) => p.value > 0) ? (
            <HardBarChart
              points={tokenTrend}
              formatValue={formatCompact}
              ariaLabel={`近${TREND_DAYS}天 token 消耗柱状图`}
            />
          ) : (
            <EmptyState
              compact
              icon={ChartBar}
              title="暂无用量记录"
              description="同步 OpenCode 用量后这里会出现 token 柱状图。"
            />
          )}
        </SectionPanel>
      </div>

      <div className="grid gap-3 lg:grid-cols-[1.2fr_0.8fr]">
        <SectionPanel
          title="网关健康"
          description="当前进程内的请求结果分布"
        >
          <StatusStackBar
            ariaLabel="请求状态分布"
            segments={[
              { label: "2xx", value: total2xx, color: "var(--accent-teal)" },
              { label: "429", value: total429, color: "var(--accent-yellow)" },
              { label: "5xx", value: total5xx, color: "var(--accent)" },
            ]}
          />
          <div className="mt-4 grid gap-3 sm:grid-cols-3">
            <div className="border-2 border-border bg-paper-0 px-3 py-3">
              <p className="text-caption text-ink-muted">成功率</p>
              <p className="mt-1.5 text-xl font-semibold tabular-nums text-ink">
                {successRate}
              </p>
            </div>
            <div className="border-2 border-border bg-paper-0 px-3 py-3">
              <p className="text-caption text-ink-muted">流式请求</p>
              <p className="mt-1.5 text-xl font-semibold tabular-nums text-ink">
                {metrics?.stream_requests ?? 0}
              </p>
            </div>
            <div className="border-2 border-border bg-paper-0 px-3 py-3">
              <p className="text-caption text-ink-muted">5xx</p>
              <p className="mt-1.5 text-xl font-semibold tabular-nums text-ink">
                {total5xx}
              </p>
            </div>
          </div>
        </SectionPanel>

        <SectionPanel title="快捷操作" description="常用配置入口">
          <div className="flex flex-col gap-2">
            {[
              { to: "/app/local-keys", label: "创建本地密钥", desc: "给客户端用" },
              { to: "/app/proxies", label: "配置出口代理", desc: "缓解 free IP 限流" },
              { to: "/app/quotas", label: "额度监控", desc: "OpenCode / Ollama 额度" },
            ].map((item) => (
              <Link
                key={item.to}
                to={item.to}
                className="flex items-center justify-between border-2 border-border bg-paper-0 px-3 py-2.5 transition-colors hover:bg-accent-soft"
              >
                <div>
                  <p className="text-[13px] font-medium text-ink">{item.label}</p>
                  <p className="text-[12px] text-ink-muted">{item.desc}</p>
                </div>
                <Path size={16} className="text-ink-faint" />
              </Link>
            ))}
          </div>
        </SectionPanel>
      </div>

      {data.quota_windows && data.quota_windows.length > 0 ? (
        <SectionPanel
          title="额度窗口"
          description="按 cascade 规则计算的有效剩余"
        >
          <div className="grid gap-3 sm:grid-cols-3">
            {data.quota_windows.map((window) => (
              <div
                key={window.label}
                className="border-2 border-border bg-paper-0 p-3.5"
              >
                <div className="flex items-center justify-between gap-2">
                  <p className="text-caption font-medium text-ink-muted">
                    {window.label}
                  </p>
                  {window.blocked ? (
                    <Badge kind="warning">受限</Badge>
                  ) : (
                    <Badge kind="healthy">可用</Badge>
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
                  已用 {Number(window.used ?? 0).toFixed(1)}%
                  {window.blocked_by ? ` · 受 ${window.blocked_by} 影响` : ""}
                </p>
              </div>
            ))}
          </div>
        </SectionPanel>
      ) : null}

      <SectionPanel
        title="按模型"
        description="来自已同步的用量记录"
        actions={
          <Button variant="ghost" onClick={() => void navigate("/app/logs?tab=usage")}>
            查看用量
          </Button>
        }
        {...(data.by_model.length === 0 ? { bodyClassName: "p-0" } : {})}
      >
        {data.by_model.length === 0 ? (
          <EmptyState
            compact
            icon={Stack}
            title="还没有模型用量"
            description="同步 OpenCode 用量，或先发起几条代理请求后这里会自动出现。"
            action={
              <Button variant="secondary" onClick={() => void navigate("/app/logs?tab=usage")}>
                去同步用量
              </Button>
            }
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[26rem] text-left text-sm md:min-w-[32rem]">
              <thead>
                <tr className="border-b border-border text-caption text-ink-muted">
                  <th className="pb-2 font-medium">模型</th>
                  <th className="pb-2 font-medium">份额</th>
                  <th className="pb-2 font-medium">请求</th>
                  <th className="pb-2 font-medium">输入</th>
                  <th className="pb-2 font-medium">输出</th>
                </tr>
              </thead>
              <tbody>
                {data.by_model.map((row) => (
                  <tr key={row.model} className="border-b border-border last:border-b-0">
                    <td className="py-2.5 font-mono text-[13px] text-ink">
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
