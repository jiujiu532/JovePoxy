/**
 * Pure helpers for overview.routing_kpis (final upstream channel view).
 * Does not infer cross-pool failover counts; only surfaces final upstream buckets.
 */

import type { RoutingKPIsDTO, RoutingUpstreamKPI } from "@/lib/api";

/** Paid final channels shown on the overview routing card. */
export const PAID_UPSTREAMS = ["opencode_paid", "ollama_paid"] as const;
export type PaidUpstream = (typeof PAID_UPSTREAMS)[number];

export const FREE_UPSTREAM = "opencode_free";
export const UNKNOWN_UPSTREAM = "unknown";

export type RoutingChannelView = {
  readonly upstream: PaidUpstream;
  readonly requests: number;
  /** 0..1; null when requests==0 (do not show 0% as a fake conclusion). */
  readonly success_rate: number | null;
  readonly latency_p50_ms: number | null;
  readonly latency_p95_ms: number | null;
  readonly status_2xx: number;
  readonly status_429: number;
  readonly status_4xx: number;
  readonly status_5xx: number;
  /**
   * Share of paid traffic only (OC+OL). null when paidRequests==0.
   * Never includes free/unknown/empty upstream.
   */
  readonly share_of_paid: number | null;
};

export type RoutingChannelsSummary = {
  readonly window: string;
  /** Sum of all by_upstream requests (incl. free/unknown). */
  readonly totalRequests: number;
  /** opencode_paid + ollama_paid only. */
  readonly paidRequests: number;
  readonly freeRequests: number;
  /** Empty / unknown / unrecognized buckets (never folded into paid). */
  readonly unknownRequests: number;
  readonly channels: readonly [RoutingChannelView, RoutingChannelView];
  /** true when any sample exists in the window (any upstream). */
  readonly hasAnyData: boolean;
  /** true when at least one paid channel has requests. */
  readonly hasPaidData: boolean;
};

function emptyBucket(upstream: string): RoutingUpstreamKPI {
  return {
    upstream,
    requests: 0,
    status_2xx: 0,
    status_429: 0,
    status_4xx: 0,
    status_5xx: 0,
  };
}

/** Normalize server/legacy upstream labels into stable keys. */
export function normalizeUpstream(raw: string | null | undefined): string {
  const value = (raw ?? "").trim().toLowerCase();
  if (!value || value === "none" || value === "null" || value === "undefined") {
    return UNKNOWN_UPSTREAM;
  }
  if (value === "unknown" || value === "legacy" || value === "history") {
    return UNKNOWN_UPSTREAM;
  }
  return value;
}

export function isPaidUpstream(upstream: string): upstream is PaidUpstream {
  return upstream === "opencode_paid" || upstream === "ollama_paid";
}

function coalesceRate(
  requests: number,
  status2xx: number,
  rate: number | null | undefined,
): number | null {
  if (requests <= 0) return null;
  if (typeof rate === "number" && Number.isFinite(rate)) return rate;
  return status2xx / requests;
}

function coalesceLatency(value: number | null | undefined): number | null {
  if (value == null) return null;
  if (typeof value !== "number" || !Number.isFinite(value)) return null;
  return value;
}

/** Pick one upstream bucket; empty buckets stay zero (not failures). */
export function pickUpstream(
  kpis: RoutingKPIsDTO | null | undefined,
  upstream: string,
): RoutingUpstreamKPI {
  const want = normalizeUpstream(upstream);
  const list = kpis?.by_upstream ?? [];
  let requests = 0;
  let s2xx = 0;
  let s429 = 0;
  let sOther4 = 0;
  let s5xx = 0;
  let rate: number | null | undefined;
  let p50: number | null | undefined;
  let p95: number | null | undefined;
  let found = false;

  for (const row of list) {
    if (normalizeUpstream(row.upstream) !== want) continue;
    found = true;
    requests += row.requests ?? 0;
    s2xx += row.status_2xx ?? 0;
    s429 += row.status_429 ?? 0;
    sOther4 += row.status_4xx ?? 0;
    s5xx += row.status_5xx ?? 0;
    if (rate == null && row.success_rate != null) rate = row.success_rate;
    if (p50 == null && row.latency_p50_ms != null) p50 = row.latency_p50_ms;
    if (p95 == null && row.latency_p95_ms != null) p95 = row.latency_p95_ms;
  }

  if (!found) return emptyBucket(want);

  const out: RoutingUpstreamKPI = {
    upstream: want,
    requests,
    status_2xx: s2xx,
    status_429: s429,
    status_4xx: sOther4,
    status_5xx: s5xx,
  };
  const success = coalesceRate(requests, s2xx, rate);
  if (success != null) {
    return {
      ...out,
      success_rate: success,
      ...(coalesceLatency(p50) != null ? { latency_p50_ms: coalesceLatency(p50)! } : {}),
      ...(coalesceLatency(p95) != null ? { latency_p95_ms: coalesceLatency(p95)! } : {}),
    };
  }
  return {
    ...out,
    ...(coalesceLatency(p50) != null ? { latency_p50_ms: coalesceLatency(p50)! } : {}),
    ...(coalesceLatency(p95) != null ? { latency_p95_ms: coalesceLatency(p95)! } : {}),
  };
}

function toChannelView(
  bucket: RoutingUpstreamKPI,
  paidTotal: number,
  upstream: PaidUpstream,
): RoutingChannelView {
  const requests = bucket.requests ?? 0;
  const s2xx = bucket.status_2xx ?? 0;
  return {
    upstream,
    requests,
    success_rate: coalesceRate(requests, s2xx, bucket.success_rate),
    latency_p50_ms: coalesceLatency(bucket.latency_p50_ms),
    latency_p95_ms: coalesceLatency(bucket.latency_p95_ms),
    status_2xx: s2xx,
    status_429: bucket.status_429 ?? 0,
    status_4xx: bucket.status_4xx ?? 0,
    status_5xx: bucket.status_5xx ?? 0,
    share_of_paid: paidTotal > 0 ? requests / paidTotal : null,
  };
}

/**
 * Build a UI-ready summary for paid OC/OL final channels.
 * Unknown/empty/free are counted separately and never folded into paid share.
 */
export function summarizeRoutingKPIs(
  kpis: RoutingKPIsDTO | null | undefined,
): RoutingChannelsSummary {
  const window = (kpis?.window ?? "").trim() || "24h";
  const oc = pickUpstream(kpis, "opencode_paid");
  const ol = pickUpstream(kpis, "ollama_paid");
  const free = pickUpstream(kpis, FREE_UPSTREAM);

  let listedTotal = 0;
  let unknownRequests = 0;
  for (const row of kpis?.by_upstream ?? []) {
    const key = normalizeUpstream(row.upstream);
    const n = row.requests ?? 0;
    listedTotal += n;
    if (key === FREE_UPSTREAM || isPaidUpstream(key)) continue;
    unknownRequests += n;
  }

  // Prefer server total when present; fall back to sum of buckets.
  const totalRequests =
    typeof kpis?.requests === "number" && Number.isFinite(kpis.requests)
      ? Math.max(0, kpis.requests)
      : listedTotal;

  const paidRequests = (oc.requests ?? 0) + (ol.requests ?? 0);
  const freeRequests = free.requests ?? 0;

  return {
    window,
    totalRequests,
    paidRequests,
    freeRequests,
    unknownRequests,
    channels: [
      toChannelView(oc, paidRequests, "opencode_paid"),
      toChannelView(ol, paidRequests, "ollama_paid"),
    ],
    hasAnyData: totalRequests > 0 || listedTotal > 0,
    hasPaidData: paidRequests > 0,
  };
}

/** Compact integer display for dense KPI tiles. */
export function formatCompactCount(value: number): string {
  if (!Number.isFinite(value)) return "0";
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}k`;
  return String(Math.round(value));
}

/** Success rate text; empty samples stay "-" (no fake 0%). */
export function formatSuccessRatePct(
  rate: number | null | undefined,
  requests: number,
): string {
  if (requests <= 0 || rate == null || !Number.isFinite(rate)) return "-";
  return `${(rate * 100).toFixed(1)}%`;
}

/** Share of paid traffic; empty paid total stays "-". */
export function formatPaidShare(share: number | null | undefined): string {
  if (share == null || !Number.isFinite(share)) return "-";
  return `${(share * 100).toFixed(1)}%`;
}
