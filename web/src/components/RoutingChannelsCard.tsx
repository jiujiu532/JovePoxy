/**
 * Dense final-channel routing strip for Overview.
 * One shared paid-share bar + two comparison rows (OpenCode / Ollama).
 * Does not claim cross-pool failover counts or per-key hits.
 */

import { GitBranch, Path } from "@phosphor-icons/react";
import {
  EmptyState,
  SectionPanel,
  SegmentedFilter,
  Skeleton,
} from "@/components";
import { StatusStackBar } from "@/components/charts";
import { cn } from "@/lib/cn";
import type { OpsWindow, RoutingKPIsDTO } from "@/lib/api";
import type { Translate } from "@/lib/i18n";
import {
  formatCompactCount,
  formatPaidShare,
  formatSuccessRatePct,
  summarizeRoutingKPIs,
  type RoutingChannelView,
} from "@/lib/routing-kpis";

export type RoutingChannelsCardProps = {
  readonly kpis: RoutingKPIsDTO | null | undefined;
  readonly window: OpsWindow;
  readonly onWindowChange: (window: OpsWindow) => void;
  readonly loading?: boolean;
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

function channelTitle(upstream: RoutingChannelView["upstream"], t: Translate): string {
  if (upstream === "ollama_paid") return t("overview.zenPool.ollama");
  return t("overview.zenPool.opencode");
}

function rateToneClass(rate: number | null, requests: number): string {
  if (rate == null || requests === 0) {
    return "bg-paper-2 text-ink-muted border-border";
  }
  if (rate >= 0.95) {
    return "bg-accent-teal text-black border-border";
  }
  if (rate >= 0.85) {
    return "bg-accent-yellow text-black border-border";
  }
  return "bg-accent text-black border-border";
}

function formatLatency(t: Translate, value: number | null): string {
  if (value == null) return t("common.none");
  return t("overview.opsKpis.ms", { n: value });
}

function ChannelRow({
  channel,
  t,
  accent,
}: {
  readonly channel: RoutingChannelView;
  readonly t: Translate;
  readonly accent: "teal" | "coral";
}) {
  const requests = channel.requests;
  const empty = requests <= 0;
  const title = channelTitle(channel.upstream, t);
  const accentDot = accent === "teal" ? "bg-accent-teal" : "bg-accent-coral";
  const rateText = formatSuccessRatePct(channel.success_rate, requests);

  return (
    <div className="grid min-w-0 grid-cols-1 gap-2 border-2 border-border bg-paper-2 px-2.5 py-2 sm:grid-cols-[minmax(7.5rem,0.9fr)_repeat(4,minmax(0,0.7fr))_minmax(0,1.4fr)] sm:items-center sm:gap-3">
      <div className="flex min-w-0 items-center gap-1.5">
        <span className={cn("h-2.5 w-2.5 shrink-0 border border-border", accentDot)} aria-hidden />
        <span className="truncate font-mono text-[12px] font-bold text-ink">{title}</span>
        <span className="shrink-0 border border-border bg-paper-1 px-1 py-px font-mono text-[10px] font-semibold text-ink-muted">
          {formatPaidShare(channel.share_of_paid)}
        </span>
      </div>

      <MetricCell label={t("overview.opsKpis.requests")} value={formatCompactCount(requests)} />
      <div className="min-w-0">
        <p className="font-mono text-[10px] font-bold uppercase tracking-wide text-ink-muted">
          {t("overview.opsKpis.successRate")}
        </p>
        <span
          className={cn(
            "mt-0.5 inline-block border px-1.5 py-0.5 font-mono text-[13px] font-extrabold tabular-nums leading-none",
            rateToneClass(channel.success_rate, requests),
          )}
        >
          {rateText}
        </span>
      </div>
      <MetricCell
        label={t("overview.opsKpis.latencyP50")}
        value={formatLatency(t, channel.latency_p50_ms)}
      />
      <MetricCell
        label={t("overview.opsKpis.latencyP95")}
        value={formatLatency(t, channel.latency_p95_ms)}
      />

      <div className="min-w-0">
        {empty ? (
          <div className="flex h-5 w-full items-center justify-center border border-border bg-paper-1 font-mono text-[10px] text-ink-faint">
            {t("charts.noRequests")}
          </div>
        ) : (
          <StatusStackBar
            ariaLabel={`${title} ${t("overview.health.ariaLabel")}`}
            segments={[
              {
                label: t("overview.opsKpis.status2xx"),
                value: channel.status_2xx,
                color: "var(--accent-teal)",
              },
              {
                label: t("overview.opsKpis.status429"),
                value: channel.status_429,
                color: "var(--accent-yellow)",
              },
              {
                label: t("overview.opsKpis.status4xx"),
                value: channel.status_4xx,
                color: "var(--accent-coral)",
              },
              {
                label: t("overview.opsKpis.status5xx"),
                value: channel.status_5xx,
                color: "var(--accent)",
              },
            ]}
          />
        )}
        {!empty ? (
          <p className="mt-0.5 font-mono text-[10px] tabular-nums text-ink-faint">
            {t("overview.opsKpis.status2xx")} {channel.status_2xx}
            {" · "}
            {t("overview.opsKpis.status429")} {channel.status_429}
            {" · "}
            {t("overview.opsKpis.status5xx")} {channel.status_5xx}
            {channel.status_4xx > 0
              ? ` · ${t("overview.opsKpis.status4xx")} ${channel.status_4xx}`
              : ""}
          </p>
        ) : null}
      </div>
    </div>
  );
}

function MetricCell({
  label,
  value,
}: {
  readonly label: string;
  readonly value: string;
}) {
  return (
    <div className="min-w-0">
      <p className="font-mono text-[10px] font-bold uppercase tracking-wide text-ink-muted">
        {label}
      </p>
      <p className="mt-0.5 font-mono text-[15px] font-extrabold tabular-nums leading-none text-ink">
        {value}
      </p>
    </div>
  );
}

/**
 * Overview final-channel routing panel.
 * Window is independent 1h|24h|7d (server routing_kpis), not the page date range.
 */
export function RoutingChannelsCard({
  kpis,
  window,
  onWindowChange,
  loading = false,
  t,
}: RoutingChannelsCardProps) {
  const summary = summarizeRoutingKPIs(kpis);
  const rangeText = windowLabel(window, t);
  const [oc, ol] = summary.channels;
  const ocShare =
    oc.share_of_paid == null ? 0 : Math.max(0, Math.min(100, oc.share_of_paid * 100));
  const olShare =
    ol.share_of_paid == null ? 0 : Math.max(0, Math.min(100, ol.share_of_paid * 100));

  return (
    <SectionPanel
      title={t("overview.routing.title")}
      description={t("overview.routing.description", { range: rangeText })}
      icon={GitBranch}
      iconTone="mint"
      bodyClassName="!p-3"
      actions={
        <SegmentedFilter
          aria-label={t("overview.opsKpis.window")}
          value={window}
          onChange={(value) => {
            if (value === "1h" || value === "24h" || value === "7d") {
              onWindowChange(value);
            }
          }}
          options={[
            { value: "1h", label: t("overview.opsKpis.window1h") },
            { value: "24h", label: t("overview.opsKpis.window24h") },
            { value: "7d", label: t("overview.opsKpis.window7d") },
          ]}
        />
      }
    >
      {loading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-6 w-full" />
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
        </div>
      ) : !summary.hasAnyData ? (
        <EmptyState
          compact
          icon={Path}
          title={t("overview.routing.empty", { range: rangeText })}
          description={t("overview.routing.description", { range: rangeText })}
        />
      ) : (
        <div className="flex flex-col gap-2">
          <div className="flex flex-wrap items-center gap-1.5 font-mono text-[11px] text-ink-muted">
            <span className="border-2 border-border bg-paper-1 px-2 py-0.5">
              {t("overview.routing.paidRequests")}{" "}
              <span className="font-bold tabular-nums text-ink">
                {formatCompactCount(summary.paidRequests)}
              </span>
              <span className="text-ink-faint">
                {" "}
                / {formatCompactCount(summary.totalRequests)}
              </span>
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

          {summary.hasPaidData ? (
            <div className="min-w-0">
              <div
                className="flex h-3 w-full overflow-hidden border-2 border-border bg-paper-1"
                role="img"
                aria-label={`${t("overview.zenPool.opencode")} ${formatPaidShare(oc.share_of_paid)}, ${t("overview.zenPool.ollama")} ${formatPaidShare(ol.share_of_paid)}`}
              >
                {ocShare > 0 ? (
                  <div
                    className={cn(
                      "h-full bg-accent-teal",
                      olShare > 0 && "border-r-2 border-border",
                    )}
                    style={{ width: `${ocShare}%`, minWidth: oc.requests > 0 ? 4 : 0 }}
                    title={`${t("overview.zenPool.opencode")} ${formatPaidShare(oc.share_of_paid)}`}
                  />
                ) : null}
                {olShare > 0 ? (
                  <div
                    className="h-full bg-accent-coral"
                    style={{ width: `${olShare}%`, minWidth: ol.requests > 0 ? 4 : 0 }}
                    title={`${t("overview.zenPool.ollama")} ${formatPaidShare(ol.share_of_paid)}`}
                  />
                ) : null}
              </div>
              <div className="mt-1 flex flex-wrap items-center justify-between gap-2 font-mono text-[11px] text-ink-muted">
                <span className="inline-flex items-center gap-1.5">
                  <span className="h-2 w-2 border border-border bg-accent-teal" aria-hidden />
                  {t("overview.zenPool.opencode")}{" "}
                  <span className="font-semibold tabular-nums text-ink">
                    {formatPaidShare(oc.share_of_paid)}
                  </span>
                  <span className="text-ink-faint">
                    · {formatCompactCount(oc.requests)}
                  </span>
                </span>
                <span className="inline-flex items-center gap-1.5">
                  <span className="h-2 w-2 border border-border bg-accent-coral" aria-hidden />
                  {t("overview.zenPool.ollama")}{" "}
                  <span className="font-semibold tabular-nums text-ink">
                    {formatPaidShare(ol.share_of_paid)}
                  </span>
                  <span className="text-ink-faint">
                    · {formatCompactCount(ol.requests)}
                  </span>
                </span>
              </div>
            </div>
          ) : null}

          <div className="flex flex-col gap-1.5">
            <ChannelRow channel={oc} t={t} accent="teal" />
            <ChannelRow channel={ol} t={t} accent="coral" />
          </div>

          {!summary.hasPaidData ? (
            <p className="font-mono text-[11px] text-ink-faint">
              {t("overview.routing.empty", { range: rangeText })}
            </p>
          ) : null}
        </div>
      )}
    </SectionPanel>
  );
}
