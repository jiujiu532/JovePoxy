import { describe, expect, it } from "vitest";
import type { RoutingKPIsDTO } from "@/lib/api";
import {
  formatPaidShare,
  formatSuccessRatePct,
  normalizeUpstream,
  pickUpstream,
  summarizeRoutingKPIs,
} from "@/lib/routing-kpis";

describe("normalizeUpstream", () => {
  it("maps empty / legacy labels to unknown", () => {
    expect(normalizeUpstream("")).toBe("unknown");
    expect(normalizeUpstream("  ")).toBe("unknown");
    expect(normalizeUpstream(undefined)).toBe("unknown");
    expect(normalizeUpstream("NONE")).toBe("unknown");
    expect(normalizeUpstream("Unknown")).toBe("unknown");
  });

  it("keeps paid / free channels stable", () => {
    expect(normalizeUpstream("opencode_paid")).toBe("opencode_paid");
    expect(normalizeUpstream("OLLAMA_PAID")).toBe("ollama_paid");
    expect(normalizeUpstream("opencode_free")).toBe("opencode_free");
  });
});

describe("pickUpstream", () => {
  it("returns zeros when missing and never invents success", () => {
    const empty = pickUpstream(undefined, "opencode_paid");
    expect(empty.requests).toBe(0);
    expect(empty.success_rate).toBeUndefined();
    expect(empty.status_2xx).toBe(0);
  });

  it("merges duplicate upstream labels without folding unknown", () => {
    const kpis: RoutingKPIsDTO = {
      window: "1h",
      requests: 5,
      by_upstream: [
        {
          upstream: "opencode_paid",
          requests: 2,
          status_2xx: 2,
          status_429: 0,
          status_5xx: 0,
        },
        {
          upstream: "OpenCode_Paid",
          requests: 1,
          status_2xx: 0,
          status_429: 1,
          status_5xx: 0,
        },
        {
          upstream: "",
          requests: 2,
          status_2xx: 2,
          status_429: 0,
          status_5xx: 0,
        },
      ],
    };
    const paid = pickUpstream(kpis, "opencode_paid");
    expect(paid.requests).toBe(3);
    expect(paid.status_2xx).toBe(2);
    expect(paid.status_429).toBe(1);
    const unknown = pickUpstream(kpis, "unknown");
    expect(unknown.requests).toBe(2);
  });
});

describe("summarizeRoutingKPIs", () => {
  it("returns empty paid channels without fake 0% conclusions", () => {
    const summary = summarizeRoutingKPIs({
      window: "24h",
      requests: 0,
      by_upstream: [],
    });
    expect(summary.hasAnyData).toBe(false);
    expect(summary.hasPaidData).toBe(false);
    expect(summary.paidRequests).toBe(0);
    expect(summary.channels[0].success_rate).toBeNull();
    expect(summary.channels[0].share_of_paid).toBeNull();
    expect(summary.channels[1].requests).toBe(0);
  });

  it("keeps unknown and free out of paid share", () => {
    const summary = summarizeRoutingKPIs({
      window: "7d",
      requests: 20,
      by_upstream: [
        {
          upstream: "opencode_paid",
          requests: 6,
          success_rate: 1,
          status_2xx: 6,
          status_429: 0,
          status_4xx: 0,
          status_5xx: 0,
          latency_p50_ms: 100,
          latency_p95_ms: 200,
        },
        {
          upstream: "ollama_paid",
          requests: 4,
          success_rate: 0.5,
          status_2xx: 2,
          status_429: 1,
          status_4xx: 1,
          status_5xx: 0,
          latency_p50_ms: 80,
          latency_p95_ms: 400,
        },
        {
          upstream: "opencode_free",
          requests: 7,
          status_2xx: 7,
          status_429: 0,
          status_5xx: 0,
        },
        {
          upstream: "",
          requests: 3,
          status_2xx: 0,
          status_429: 0,
          status_5xx: 3,
        },
      ],
    });

    expect(summary.totalRequests).toBe(20);
    expect(summary.paidRequests).toBe(10);
    expect(summary.freeRequests).toBe(7);
    expect(summary.unknownRequests).toBe(3);
    expect(summary.hasPaidData).toBe(true);
    expect(summary.channels[0].upstream).toBe("opencode_paid");
    expect(summary.channels[0].share_of_paid).toBeCloseTo(0.6);
    expect(summary.channels[1].share_of_paid).toBeCloseTo(0.4);
    expect(summary.channels[1].status_4xx).toBe(1);
    // paid share must not include free/unknown
    const paidShareSum =
      (summary.channels[0].share_of_paid ?? 0) +
      (summary.channels[1].share_of_paid ?? 0);
    expect(paidShareSum).toBeCloseTo(1);
  });

  it("treats missing routing_kpis as empty aggregation", () => {
    const summary = summarizeRoutingKPIs(null);
    expect(summary.hasAnyData).toBe(false);
    expect(summary.unknownRequests).toBe(0);
    expect(summary.channels).toHaveLength(2);
  });
});

describe("formatters", () => {
  it("avoids 0% when there are no samples", () => {
    expect(formatSuccessRatePct(null, 0)).toBe("-");
    expect(formatSuccessRatePct(0, 0)).toBe("-");
    expect(formatSuccessRatePct(0.5, 10)).toBe("50.0%");
    expect(formatPaidShare(null)).toBe("-");
    expect(formatPaidShare(0.25)).toBe("25.0%");
  });
});
