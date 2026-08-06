/**
 * Unified ops board for Overview: live key pool + global KPI + final paid channels
 * in one dense glanceable table.
 */

import { Coins, Path } from "@phosphor-icons/react";
import type { ReactNode } from "react";
import {
  Button,
  EmptyState,
  SectionPanel,
  Skeleton,
} from "@/components";
import { cn } from "@/lib/cn";
import type {
  OpsKPIsDTO,
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
  readonly routingLoading?: boolean;
  readonly onOpenPool: () => void;
  readonly t: Translate;
};

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

function latencyToneClass(ms: number | null | undefined, requests: number): string {
  if (ms == null || requests === 0) return "text-ink-muted";
  if (ms >= 10000) return "text-accent";
  if (ms >= 3000) return "text-accent-coral";
  return "text-ink";
}

type StatusCounts = {
  readonly s2xx: number;
  readonly s429: number;
  readonly s4xx: number;
  readonly s5xx: number;
};

/** Compact HTTP status stack: bar + one dense legend line. */
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
      <div className="flex h-5 w-full min-w-[8rem] items-center justify-center border border-border bg-paper-1 font-mono text-[11px] font-semibold text-ink-muted">
        {t("charts.noRequests")}
      </div>
    );
  }
  const segments = [
    { label: t("overview.opsKpis.status2xx"), value: counts.s2xx, color: "var(--accent-teal)" },
    { label: t("overview.opsKpis.status429"), value: counts.s429, color: "var(--accent-yellow)" },
    { label: t("overview.opsKpis.status4xx"), value: counts.s4xx, color: "var(--accent-coral)" },
    { label: t("overview.opsKpis.status5xx"), value: counts.s5xx, color: "var(--accent)" },
  ];
  const total = segments.reduce((sum, s) => sum + s.value, 0);
  return (
    <div className="min-w-[8rem]" role="img" aria-label={ariaLabel}>
      <div className="flex h-5 w-full overflow-hidden border-2 border-border bg-paper-2">
        {segments
          .filter((s) => s.value > 0)
          .map((s, i) => (
            <div
              key={s.label}
              className={cn("h-full", i > 0 && "border-l border-border")}
              style={{
                width: `${(s.value / total) * 100}%`,
                backgroundColor: s.color,
                minWidth: 4,
              }}
              title={`${s.label}: ${s.value}`}
            />
          ))}
      </div>
      <p className="mt-0.5 flex flex-wrap gap-x-2 font-mono text-[11px] font-semibold tabular-nums leading-none text-ink">
        {segments
          .filter((s) => s.value > 0)
          .map((s) => (
            <span key={s.label} className="inline-flex items-center gap-1">
              <span
                className="inline-block h-2 w-2 border border-border"
                style={{ backgroundColor: s.color }}
                aria-hidden
              />
              {s.label}
              <span className="text-ink">{s.value}</span>
            </span>
          ))}
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
        "inline-block border-2 px-2 py-0.5 font-mono text-[14px] font-extrabold tabular-nums leading-none",
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
}: {
  readonly accent: "ink" | "teal" | "coral";
  readonly title: string;
  readonly share?: string | undefined;
}) {
  const dot =
    accent === "teal"
      ? "bg-accent-teal"
      : accent === "coral"
        ? "bg-accent-coral"
        : "bg-ink";
  return (
    <div className="flex min-w-0 items-center gap-1.5">
      <span className={cn("h-3 w-3 shrink-0 border border-border", dot)} aria-hidden />
      <span className="truncate font-mono text-[13px] font-extrabold text-ink">{title}</span>
      {share ? (
        <span className="shrink-0 border-2 border-border bg-ink px-1.5 py-px font-mono text-[11px] font-extrabold tabular-nums text-paper-0">
          {share}
        </span>
      ) : null}
    </div>
  );
}

function MetricTd({
  children,
  className,
}: {
  readonly children: ReactNode;
  readonly className?: string;
}) {
  return (
    <td
      className={cn(
        "px-2 py-1.5 align-middle font-mono text-[15px] font-extrabold tabular-nums text-ink",
        className,
      )}
    >
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
  routingLoading = false,
  onOpenPool,
  t,
}: OpsBoardProps) {
  const summary = summarizeRoutingKPIs(routingKpis);
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
      description={t("overview.opsBoard.description", { range: rangeText })}
      icon={Coins}
      iconTone="teal"
      bodyClassName="!p-0"
      actions={
        <Button variant="ghost" size="sm" onClick={onOpenPool}>
          {t("overview.zenPool.openPool")}
        </Button>
      }
    >
      {/* Live key-pool strip — numbers only, no decorative bar */}
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b-2 border-border bg-paper-2 px-3 py-1.5">
        <span className="font-mono text-[11px] font-extrabold uppercase tracking-wide text-ink">
          {t("overview.zenPool.title")}
        </span>
        <span className="font-mono text-[12px] font-bold text-ink">
          {t("overview.zenPool.totalKeys")}{" "}
          <span className="tabular-nums">{total}</span>
        </span>
        <span className="font-mono text-[12px] font-bold text-ink">
          {t("overview.zenPool.healthy")}{" "}
          <span className="tabular-nums text-accent-teal">{healthy}</span>
        </span>
        <span className="font-mono text-[12px] font-bold text-ink">
          {t("overview.zenPool.abnormal")}{" "}
          <span
            className={cn(
              "tabular-nums",
              abnormal > 0 ? "text-accent-coral" : "text-ink-muted",
            )}
          >
            {abnormal}
          </span>
        </span>
        <span className="inline-flex items-center gap-1 border-2 border-border bg-paper-0 px-1.5 py-0.5 font-mono text-[11px] font-extrabold text-ink">
          <span
            className={cn("h-2 w-2", ocPool.healthy > 0 ? "bg-accent-teal" : "bg-ink-faint")}
            aria-hidden
          />
          {t("overview.zenPool.opencode")} {ocPool.healthy}/{ocPool.total}
        </span>
        <span className="inline-flex items-center gap-1 border-2 border-border bg-paper-0 px-1.5 py-0.5 font-mono text-[11px] font-extrabold text-ink">
          <span
            className={cn("h-2 w-2", olPool.healthy > 0 ? "bg-accent-teal" : "bg-ink-faint")}
            aria-hidden
          />
          {t("overview.zenPool.ollama")} {olPool.healthy}/{olPool.total}
        </span>
      </div>

      {/* Table */}
      <div className="px-2 py-1.5">
        {routingLoading && !summary.hasAnyData ? (
          <div className="flex flex-col gap-1.5">
            <Skeleton className="h-5 w-full" />
            <Skeleton className="h-20 w-full" />
          </div>
        ) : (
          <div className="flex flex-col gap-1.5">
            {(summary.paidRequests > 0 ||
              summary.freeRequests > 0 ||
              summary.unknownRequests > 0) && (
              <div className="flex flex-wrap items-center gap-1.5 px-1 font-mono text-[12px] font-bold text-ink">
                <span className="border-2 border-border bg-paper-1 px-2 py-0.5">
                  {t("overview.routing.paidRequests")}{" "}
                  <span className="tabular-nums">
                    {formatCompactCount(summary.paidRequests)}
                  </span>
                  <span className="text-ink-muted">
                    {" "}
                    / {formatCompactCount(summary.totalRequests)}
                  </span>
                </span>
                {summary.freeRequests > 0 ? (
                  <span className="border-2 border-border bg-paper-1 px-2 py-0.5 text-ink-muted">
                    {t("overview.routing.freeHint", {
                      n: formatCompactCount(summary.freeRequests),
                    })}
                  </span>
                ) : null}
                {summary.unknownRequests > 0 ? (
                  <span className="border-2 border-border bg-paper-1 px-2 py-0.5 text-ink-muted">
                    {t("overview.routing.unknownHint", {
                      n: formatCompactCount(summary.unknownRequests),
                    })}
                  </span>
                ) : null}
              </div>
            )}

            <div className="overflow-x-auto border-2 border-border">
              <table className="w-full min-w-[40rem] border-collapse text-left">
                <thead>
                  <tr className="border-b-2 border-border bg-paper-2 font-mono text-[11px] font-extrabold uppercase tracking-wide text-ink">
                    <th className="px-2 py-1 font-extrabold">
                      {t("overview.opsBoard.col.channel")}
                    </th>
                    <th className="px-2 py-1 font-extrabold">
                      {t("overview.opsKpis.requests")}
                    </th>
                    <th className="px-2 py-1 font-extrabold">
                      {t("overview.opsKpis.successRate")}
                    </th>
                    <th className="px-2 py-1 font-extrabold">
                      {t("overview.opsKpis.latencyP50")}
                    </th>
                    <th className="px-2 py-1 font-extrabold">
                      {t("overview.opsKpis.latencyP95")}
                    </th>
                    <th className="px-2 py-1 font-extrabold">
                      {t("overview.opsKpis.statusDist")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr className="border-b border-border bg-paper-0">
                    <td className="px-2 py-1.5 align-middle">
                      <ChannelName accent="ink" title={t("overview.opsBoard.global")} />
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
                    <MetricTd
                      className={latencyToneClass(opsKpis.latency_p50_ms, globalRequests)}
                    >
                      {formatLatency(t, opsKpis.latency_p50_ms)}
                    </MetricTd>
                    <MetricTd
                      className={latencyToneClass(opsKpis.latency_p95_ms, globalRequests)}
                    >
                      {formatLatency(t, opsKpis.latency_p95_ms)}
                    </MetricTd>
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
                    t={t}
                    dim={!summary.hasPaidData && oc.requests === 0}
                  />
                  <ChannelTableRow
                    accent="coral"
                    channel={ol}
                    title={channelTitle(ol.upstream, t)}
                    share={olShare}
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
                description={t("overview.opsBoard.description", { range: rangeText })}
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
  accent,
  t,
  dim = false,
  last = false,
}: {
  readonly channel: RoutingChannelView;
  readonly title: string;
  readonly share: string;
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
        />
      </td>
      <MetricTd>{requests === 0 ? "-" : formatCompactCount(requests)}</MetricTd>
      <td className="px-2 py-1.5 align-middle">
        <RateBadge rate={channel.success_rate} requests={requests} text={rateText} />
      </td>
      <MetricTd className={latencyToneClass(channel.latency_p50_ms, requests)}>
        {formatLatency(t, channel.latency_p50_ms)}
      </MetricTd>
      <MetricTd className={latencyToneClass(channel.latency_p95_ms, requests)}>
        {formatLatency(t, channel.latency_p95_ms)}
      </MetricTd>
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
