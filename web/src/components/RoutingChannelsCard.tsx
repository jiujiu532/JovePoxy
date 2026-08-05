/**
 * Compact final-channel routing card for Overview.
 * Shows paid OpenCode / paid Ollama request quality from routing_kpis.
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
import { Badge } from "@/components/Badge";
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
    return "bg-accent-teal text-black border-border shadow-[1px_1px_0_var(--border)]";
  }
  if (rate >= 0.85) {
    return "bg-accent-yellow text-black border-border shadow-[1px_1px_0_var(--border)]";
  }
  return "bg-accent text-black border-border shadow-[1px_1px_0_var(--border)]";
}

function ChannelTile({
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
  const shareBar =
    channel.share_of_paid == null
      ? 0
      : Math.max(0, Math.min(100, channel.share_of_paid * 100));

  return (
    <div className="flex min-w-0 flex-col gap-2.5 border-2 border-border bg-paper-2 p-2.5 shadow-[2px_2px_0_var(--border)]">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-1.5">
          <span className={cn("h-2.5 w-2.5 shrink-0 border border-border", accentDot)} aria-hidden />
          <span className="truncate font-mono text-[12px] font-bold text-ink">{title}</span>
          <Badge kind="paid" className="text-[10px]">
            {t("models.kpi.paid")}
          </Badge>
        </div>
        <span className="font-mono text-[11px] tabular-nums text-ink-muted">
          {formatPaidShare(channel.share_of_paid)}
        </span>
      </div>

      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <div className="min-w-0">
          <p className="font-mono text-[10px] font-bold uppercase tracking-wide text-ink-muted">
            {t("overview.opsKpis.requests")}
          </p>
          <p className="mt-0.5 font-mono text-[1.25rem] font-extrabold tabular-nums leading-none text-ink">
            {formatCompactCount(requests)}
          </p>
        </div>
        <div className="min-w-0">
          <p className="font-mono text-[10px] font-bold uppercase tracking-wide text-ink-muted">
            {t("overview.opsKpis.successRate")}
          </p>
          <p className="mt-0.5 font-mono text-[1.25rem] font-extrabold tabular-nums leading-none text-ink">
            {formatSuccessRatePct(channel.success_rate, requests)}
          </p>
          <span
            className={cn(
              "mt-1 inline-block border px-1.5 py-0.5 font-mono text-[10px] font-bold",
              rateToneClass(channel.success_rate, requests),
            )}
          >
            {empty
              ? t("common.none")
              : formatSuccessRatePct(channel.success_rate, requests)}
          </span>
        </div>
        <div className="min-w-0">
          <p className="font-mono text-[10px] font-bold uppercase tracking-wide text-ink-muted">
            {t("overview.opsKpis.latencyP50")}
          </p>
          <p className="mt-0.5 font-mono text-[1.1rem] font-extrabold tabular-nums leading-none text-ink">
            {channel.latency_p50_ms == null
              ? t("common.none")
              : t("overview.opsKpis.ms", { n: channel.latency_p50_ms })}
          </p>
        </div>
        <div className="min-w-0">
          <p className="font-mono text-[10px] font-bold uppercase tracking-wide text-ink-muted">
            {t("overview.opsKpis.latencyP95")}
          </p>
          <p className="mt-0.5 font-mono text-[1.1rem] font-extrabold tabular-nums leading-none text-ink">
            {channel.latency_p95_ms == null
              ? t("common.none")
              : t("overview.opsKpis.ms", { n: channel.latency_p95_ms })}
          </p>
        </div>
      </div>

      {/* Paid-share hard bar (OC vs OL only; not theoretical key weight). */}
      <div className="h-2 w-full border border-border bg-paper-1" aria-hidden>
        <div
          className={cn("h-full", accent === "teal" ? "bg-accent-teal" : "bg-accent-coral")}
          style={{ width: empty ? "0%" : `${Math.max(shareBar, requests > 0 ? 4 : 0)}%` }}
        />
      </div>

      <div className="border-t border-border/30 pt-2">
        <div className="mb-1 flex items-center justify-between font-mono text-[10px] text-ink-faint">
          <span className="font-bold uppercase tracking-wide text-ink-muted">
            {t("overview.health.ariaLabel")}
          </span>
          <span className="tabular-nums">
            {t("overview.opsKpis.status2xx")} {channel.status_2xx}
            {" · "}
            {t("overview.opsKpis.status429")} {channel.status_429}
            {" · "}
            {t("overview.opsKpis.status5xx")} {channel.status_5xx}
            {channel.status_4xx > 0
              ? ` · ${t("overview.opsKpis.status4xx")} ${channel.status_4xx}`
              : ""}
          </span>
        </div>
        {empty ? (
          <div className="flex h-6 w-full items-center justify-center border-2 border-border bg-paper-1 font-mono text-[11px] text-ink-faint">
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
      </div>
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

  const title = t("overview.routing.title");
  const description = t("overview.routing.description", { range: rangeText });

  return (
    <SectionPanel
      title={title}
      description={description}
      icon={GitBranch}
      iconTone="mint"
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
        <div className="grid gap-3 lg:grid-cols-2">
          <Skeleton className="h-40 w-full" />
          <Skeleton className="h-40 w-full" />
        </div>
      ) : !summary.hasAnyData ? (
        <EmptyState
          compact
          icon={Path}
          title={t("overview.routing.empty", { range: rangeText })}
          description={t("overview.routing.description", { range: rangeText })}
        />
      ) : (
        <div className="flex flex-col gap-3">
          <div className="flex flex-wrap items-center gap-2 font-mono text-[11px] text-ink-muted">
            <span className="border-2 border-border bg-paper-1 px-2 py-0.5 shadow-[1px_1px_0_var(--border)]">
              {t("overview.opsKpis.requests")}{" "}
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
                {t("overview.routing.freeHint", { n: formatCompactCount(summary.freeRequests) })}
              </span>
            ) : null}
            {summary.unknownRequests > 0 ? (
              <span className="border border-border bg-paper-1 px-2 py-0.5 text-ink-faint">
                {t("overview.routing.unknownHint", { n: formatCompactCount(summary.unknownRequests) })}
              </span>
            ) : null}
          </div>

          <div className="grid gap-3 lg:grid-cols-2">
            <ChannelTile channel={oc} t={t} accent="teal" />
            <ChannelTile channel={ol} t={t} accent="coral" />
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
