/**
 * Unified ops board for Overview: live key pool + global KPI + final paid channels
 * in one dense table. Collapses the previous three SectionPanels.
 */

import { ChartLineUp, Coins, Path } from "@phosphor-icons/react";
import type { ReactNode } from "react";
import {
  Button,
  EmptyState,
  SectionPanel,
  SegmentedFilter,
  Skeleton,
} from "@/components";
import { StatusStackBar } from "@/components/charts";
import { cn } from "@/lib/cn";
import type {
  OpsKPIsDTO,
  OpsWindow,
  RoutingKPIsDTO,
  ZenPoolSummaryDTO,
} from "@/lib/api";
import type { Translate } from "@/lib/i18n";
import {
  formatCompactCount,
  formatPaidShare,
  formatSuccessRatePct,
  summarizeRoutingKPIs,
  type RoutingChannelView,
} from "@/lib/routing-kpis";

export type OpsBoardProps = {
  readonly opsKpis: OpsKPIsDTO;
  readonly rangeText: string;
  readonly pool?: ZenPoolSummaryDTO | undefined;
  readonly routingKpis: RoutingKPIsDTO | null | undefined;
  readonly routingWindow: OpsWindow;
  readonly onRoutingWindowChange: (window: OpsWindow) => void;
  readonly routingLoading?: boolean;
  readonly onOpenPool: () => void;
  readonly t: Translate;
};

function windowLabel(window: OpsWindow, t: Translate): string {
  switch (window) {
    case "1h":
      return t("overview.opsKpis.window1h");
    case "7d":
      return t("overview.opsKpis.window7d");
    case "24h":
    default:
      return t("overview.opsKpis.window24h");
  }
}

function formatLatency(t: Translate, value: number | null | undefined): string {
  if (value == null) return t("common.none");
  return t("overview.opsKpis.ms", { n: value });
}

function formatSuccessRate(rate: number | null | undefined, requests: number): string {
  if (requests <= 0 || rate == null) return "-";
  return `${(rate * 100).toFixed(1)}%`;
}

function rateToneClass(rate: number | null | undefined, requests: number): string {
  if (rate == null || requests === 0) {
    return "bg-paper-2 text-ink-muted border-border";
  }
  if (rate >= 0.95) return "bg-accent-teal text-black border-border";
  if (rate >= 0.85) return "bg-accent-yellow text-black border-border";
  return "bg-accent text-black border-border";
}

type StatusCounts = {
  readonly s2xx: number;
  readonly s429: number;
  readonly s4xx: number;
  readonly s5xx: number;
};

function StatusCell({
  empty,
  counts,
  ariaLabel,
  t,
}: {
  readonly empty: boolean;
  readonly counts: StatusCounts;
  readonly ariaLabel: string;
  readonly t: Translate;
}) {
  if (empty) {
    return (
      <div className="flex h-4 w-full min-w-[7rem] items-center justify-center border border-border bg-paper-1 font-mono text-[10px] text-ink-faint">
        {t("charts.noRequests")}
      </div>
    );
  }
  return (
    <div className="min-w-[7rem]">
      <StatusStackBar
        ariaLabel={ariaLabel}
        segments={[
          {
            label: t("overview.opsKpis.status2xx"),
            value: counts.s2xx,
            color: "var(--accent-teal)",
          },
          {
            label: t("overview.opsKpis.status429"),
            value: counts.s429,
            color: "var(--accent-yellow)",
          },
          {
            label: t("overview.opsKpis.status4xx"),
            value: counts.s4xx,
            color: "var(--accent-coral)",
          },
          {
            label: t("overview.opsKpis.status5xx"),
            value: counts.s5xx,
            color: "var(--accent)",
          },
        ]}
      />
      <p className="mt-0.5 font-mono text-[10px] tabular-nums leading-none text-ink-faint">
        {t("overview.opsKpis.status2xx")} {counts.s2xx}
        {" · "}
        {t("overview.opsKpis.status429")} {counts.s429}
        {" · "}
        {t("overview.opsKpis.status5xx")} {counts.s5xx}
        {counts.s4xx > 0
          ? ` · ${t("overview.opsKpis.status4xx")} ${counts.s4xx}`
          : ""}
      </p>
    </div>
  );
}

function RateBadge({
  rate,
  requests,
  text,
}: {
  readonly rate: number | null | undefined;
  readonly requests: number;
  readonly text: string;
}) {
  return (
    <span
      className={cn(
        "inline-block border px-1.5 py-0.5 font-mono text-[12px] font-extrabold tabular-nums leading-none",
        rateToneClass(rate, requests),
      )}
    >
      {text}
    </span>
  );
}

function ChannelName({
  accent,
  title,
  share,
  sub,
}: {
  readonly accent: "ink" | "teal" | "coral";
  readonly title: string;
  readonly share?: string | undefined;
  readonly sub?: string | undefined;
}) {
  const dot =
    accent === "teal"
      ? "bg-accent-teal"
      : accent === "coral"
        ? "bg-accent-coral"
        : "bg-ink";
  return (
    <div className="flex min-w-0 flex-col gap-0.5">
      <div className="flex min-w-0 items-center gap-1.5">
        <span className={cn("h-2.5 w-2.5 shrink-0 border border-border", dot)} aria-hidden />
        <span className="truncate font-mono text-[12px] font-bold text-ink">{title}</span>
        {share ? (
          <span className="shrink-0 border border-border bg-paper-1 px-1 py-px font-mono text-[10px] font-semibold text-ink-muted">
            {share}
          </span>
        ) : null}
      </div>
      {sub ? (
        <span className="pl-4 font-mono text-[10px] text-ink-faint">{sub}</span>
      ) : null}
    </div>
  );
}

function MetricTd({ children }: { readonly children: ReactNode }) {
  return (
    <td className="px-2 py-1.5 align-middle font-mono text-[13px] font-extrabold tabular-nums text-ink">
      {children}
    </td>
  );
}

function channelTitle(upstream: RoutingChannelView["upstream"], t: Translate): string {
  if (upstream === "ollama_paid") return t("overview.zenPool.ollama");
  return t("overview.zenPool.opencode");
}

/**
 * Single Overview panel: key-pool strip + 3-row ops table (global / OC / OL).
 */
export function OpsBoard({
  opsKpis,
  rangeText,
  pool,
  routingKpis,
  routingWindow,
  onRoutingWindowChange,
  routingLoading = false,
  onOpenPool,
  t,
}: OpsBoardProps) {
  const summary = summarizeRoutingKPIs(routingKpis);
  const routingRange = windowLabel(routingWindow, t);
  const [oc, ol] = summary.channels;

  const total = pool?.total ?? 0;
  const healthy = pool?.healthy ?? 0;
  const cooled = pool?.cooled ?? 0;
  const disabled = pool?.disabled ?? 0;
  const benched = pool?.benched ?? 0;
  const abnormal = cooled + benched + disabled;
  const by = pool?.by_provider;
  const ocPool = by?.["opencode"] ?? {
    total: 0,
    healthy: 0,
    enabled: 0,
    cooled: 0,
    disabled: 0,
  };
  const olPool = by?.["ollama"] ?? {
    total: 0,
    healthy: 0,
    enabled: 0,
    cooled: 0,
    disabled: 0,
  };

  const globalRequests = opsKpis.requests ?? 0;
  const globalRate = opsKpis.success_rate;
  const globalCounts: StatusCounts = {
    s2xx: opsKpis.status_2xx ?? 0,
    s429: opsKpis.status_429 ?? 0,
    s4xx: opsKpis.status_4xx ?? 0,
    s5xx: opsKpis.status_5xx ?? 0,
  };

  const ocShare = formatPaidShare(oc.share_of_paid);
  const olShare = formatPaidShare(ol.share_of_paid);

  return (
    <SectionPanel
      title={t("overview.opsBoard.title")}
      description={t("overview.opsBoard.description", {
        range: rangeText,
        routing: routingRange,
      })}
      icon={ChartLineUp}
      iconTone="teal"
      bodyClassName="!p-0"
      actions={
        <div className="flex flex-wrap items-center gap-1.5">
          <SegmentedFilter
            aria-label={t("overview.opsKpis.window")}
            value={routingWindow}
            onChange={(value) => {
              if (value === "1h" || value === "24h" || value === "7d") {
                onRoutingWindowChange(value);
              }
            }}
            options={[
              { value: "1h", label: t("overview.opsKpis.window1h") },
              { value: "24h", label: t("overview.opsKpis.window24h") },
              { value: "7d", label: t("overview.opsKpis.window7d") },
            ]}
          />
          <Button variant="ghost" size="sm" onClick={onOpenPool}>
            {t("overview.zenPool.openPool")}
          </Button>
        </div>
      }
    >
      {/* Live key-pool strip */}
      <div className="flex flex-col gap-1.5 border-b-2 border-border bg-paper-2 px-3 py-2">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
          <span className="inline-flex items-center gap-1.5 font-mono text-[11px] font-bold uppercase tracking-wide text-ink-muted">
            <Coins size={12} weight="bold" aria-hidden />
            {t("overview.zenPool.title")}
          </span>
          <span className="font-mono text-[12px] text-ink-muted">
            {t("overview.zenPool.totalKeys")}{" "}
            <span className="font-extrabold tabular-nums text-ink">{total}</span>
          </span>
          <span className="font-mono text-[12px] text-ink-muted">
            {t("overview.zenPool.healthy")}{" "}
            <span className="font-extrabold tabular-nums text-accent-teal">
              {healthy}
            </span>
          </span>
          <span className="font-mono text-[12px] text-ink-muted">
            {t("overview.zenPool.abnormal")}{" "}
            <span
              className={cn(
                "font-extrabold tabular-nums",
                abnormal > 0 ? "text-accent-coral" : "text-ink-muted",
              )}
            >
              {abnormal}
            </span>
          </span>
          <span className="inline-flex items-center gap-1 border border-border bg-paper-1 px-1.5 py-0.5 font-mono text-[11px]">
            <span
              className={cn(
                "h-2 w-2",
                ocPool.healthy > 0 ? "bg-accent-teal" : "bg-ink-faint",
              )}
              aria-hidden
            />
            {t("overview.zenPool.opencode")}{" "}
            <span className="font-bold tabular-nums">
              {ocPool.healthy}/{ocPool.total}
            </span>
          </span>
          <span className="inline-flex items-center gap-1 border border-border bg-paper-1 px-1.5 py-0.5 font-mono text-[11px]">
            <span
              className={cn(
                "h-2 w-2",
                olPool.healthy > 0 ? "bg-accent-teal" : "bg-ink-faint",
              )}
              aria-hidden
            />
            {t("overview.zenPool.ollama")}{" "}
            <span className="font-bold tabular-nums">
              {olPool.healthy}/{olPool.total}
            </span>
          </span>
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
      </div>

      {/* Routing meta + table */}
      <div className="px-3 py-2">
        {routingLoading && !summary.hasAnyData ? (
          <div className="flex flex-col gap-2">
            <Skeleton className="h-5 w-full" />
            <Skeleton className="h-24 w-full" />
          </div>
        ) : (
          <div className="flex flex-col gap-2">
            <div className="flex flex-wrap items-center gap-1.5 font-mono text-[11px] text-ink-muted">
              <span className="border border-border bg-paper-1 px-2 py-0.5">
                {t("overview.routing.paidRequests")}{" "}
                <span className="font-bold tabular-nums text-ink">
                  {formatCompactCount(summary.paidRequests)}
                </span>
                <span className="text-ink-faint">
                  {" "}
                  / {formatCompactCount(summary.totalRequests)}
                </span>
                <span className="text-ink-faint"> · {routingRange}</span>
              </span>
              {summary.freeRequests > 0 ? (
                <span className="border border-border bg-paper-1 px-2 py-0.5 text-ink-faint">
                  {t("overview.routing.freeHint", {
                    n: formatCompactCount(summary.freeRequests),
                  })}
                </span>
              ) : null}
              {summary.unknownRequests > 0 ? (
                <span className="border border-border bg-paper-1 px-2 py-0.5 text-ink-faint">
                  {t("overview.routing.unknownHint", {
                    n: formatCompactCount(summary.unknownRequests),
                  })}
                </span>
              ) : null}
            </div>

            <div className="overflow-x-auto border-2 border-border">
              <table className="w-full min-w-[40rem] border-collapse text-left">
                <thead>
                  <tr className="border-b-2 border-border bg-paper-2 font-mono text-[10px] font-bold uppercase tracking-wide text-ink-muted">
                    <th className="px-2 py-1.5 font-bold">
                      {t("overview.opsBoard.col.channel")}
                    </th>
                    <th className="px-2 py-1.5 font-bold">
                      {t("overview.opsKpis.requests")}
                    </th>
                    <th className="px-2 py-1.5 font-bold">
                      {t("overview.opsKpis.successRate")}
                    </th>
                    <th className="px-2 py-1.5 font-bold">
                      {t("overview.opsKpis.latencyP50")}
                    </th>
                    <th className="px-2 py-1.5 font-bold">
                      {t("overview.opsKpis.latencyP95")}
                    </th>
                    <th className="px-2 py-1.5 font-bold">
                      {t("overview.opsKpis.statusDist")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr className="border-b border-border bg-paper-0">
                    <td className="px-2 py-1.5 align-middle">
                      <ChannelName
                        accent="ink"
                        title={t("overview.opsBoard.global")}
                        sub={rangeText}
                      />
                    </td>
                    <MetricTd>
                      {globalRequests === 0 ? "-" : formatCompactCount(globalRequests)}
                    </MetricTd>
                    <td className="px-2 py-1.5 align-middle">
                      <RateBadge
                        rate={globalRate}
                        requests={globalRequests}
                        text={formatSuccessRate(globalRate, globalRequests)}
                      />
                    </td>
                    <MetricTd>{formatLatency(t, opsKpis.latency_p50_ms)}</MetricTd>
                    <MetricTd>{formatLatency(t, opsKpis.latency_p95_ms)}</MetricTd>
                    <td className="px-2 py-1.5 align-middle">
                      <StatusCell
                        empty={globalRequests === 0}
                        counts={globalCounts}
                        ariaLabel={t("overview.opsBoard.global")}
                        t={t}
                      />
                    </td>
                  </tr>

                  <ChannelTableRow
                    accent="teal"
                    channel={oc}
                    title={channelTitle(oc.upstream, t)}
                    share={ocShare}
                    sub={routingRange}
                    t={t}
                    dim={!summary.hasPaidData && oc.requests === 0}
                  />
                  <ChannelTableRow
                    accent="coral"
                    channel={ol}
                    title={channelTitle(ol.upstream, t)}
                    share={olShare}
                    sub={routingRange}
                    t={t}
                    dim={!summary.hasPaidData && ol.requests === 0}
                    last
                  />
                </tbody>
              </table>
            </div>

            {!summary.hasAnyData && globalRequests === 0 ? (
              <EmptyState
                compact
                icon={Path}
                title={t("overview.opsBoard.empty")}
                description={t("overview.opsBoard.description", {
                  range: rangeText,
                  routing: routingRange,
                })}
              />
            ) : null}
          </div>
        )}
      </div>
    </SectionPanel>
  );
}

function ChannelTableRow({
  channel,
  title,
  share,
  sub,
  accent,
  t,
  dim = false,
  last = false,
}: {
  readonly channel: RoutingChannelView;
  readonly title: string;
  readonly share: string;
  readonly sub: string;
  readonly accent: "teal" | "coral";
  readonly t: Translate;
  readonly dim?: boolean;
  readonly last?: boolean;
}) {
  const requests = channel.requests;
  const rateText = formatSuccessRatePct(channel.success_rate, requests);
  return (
    <tr
      className={cn(
        "bg-paper-0",
        !last && "border-b border-border",
        dim && "opacity-70",
      )}
    >
      <td className="px-2 py-1.5 align-middle">
        <ChannelName
          accent={accent}
          title={title}
          share={share === "-" ? undefined : share}
          sub={sub}
        />
      </td>
      <MetricTd>{requests === 0 ? "-" : formatCompactCount(requests)}</MetricTd>
      <td className="px-2 py-1.5 align-middle">
        <RateBadge rate={channel.success_rate} requests={requests} text={rateText} />
      </td>
      <MetricTd>{formatLatency(t, channel.latency_p50_ms)}</MetricTd>
      <MetricTd>{formatLatency(t, channel.latency_p95_ms)}</MetricTd>
      <td className="px-2 py-1.5 align-middle">
        <StatusCell
          empty={requests === 0}
          counts={{
            s2xx: channel.status_2xx,
            s429: channel.status_429,
            s4xx: channel.status_4xx,
            s5xx: channel.status_5xx,
          }}
          ariaLabel={title}
          t={t}
        />
      </td>
    </tr>
  );
}
