package analytics

import (
	"sort"
	"strings"
	"time"

	"jovepoxy/internal/reqlog"
)

// UnknownUpstream is the bucket label for empty/missing Entry.Upstream values
// (legacy rows before channel logging). It must never be merged into a paid provider.
const UnknownUpstream = "unknown"

// UpstreamKPI is one final-channel bucket inside a routing window.
// SuccessRate is nil when Requests==0. Latency percentiles omit when no samples.
type UpstreamKPI struct {
	Upstream     string   `json:"upstream"`
	Requests     int64    `json:"requests"`
	SuccessRate  *float64 `json:"success_rate"` // 0..1; nil when requests==0
	LatencyP50MS *int64   `json:"latency_p50_ms,omitempty"`
	LatencyP95MS *int64   `json:"latency_p95_ms,omitempty"`
	Status2xx    int64    `json:"status_2xx"`
	Status429    int64    `json:"status_429"`
	Status4xx    int64    `json:"status_4xx"` // 400–499 excluding 429
	Status5xx    int64    `json:"status_5xx"`
}

// RoutingKPIs is time-windowed request health grouped by final upstream channel.
// Only final recorded channels are counted — no cross-pool failover inference.
type RoutingKPIs struct {
	Window     string        `json:"window"`
	Requests   int64         `json:"requests"`
	ByUpstream []UpstreamKPI `json:"by_upstream"`
}

// AggregateRoutingKPIs filters entries by CreatedAt >= now-window and groups by Upstream.
// Empty/whitespace upstream becomes UnknownUpstream and stays independent of paid channels.
// Entries with zero CreatedAt are skipped. ByUpstream is always non-nil (empty when no rows).
func AggregateRoutingKPIs(entries []reqlog.Entry, window string, now time.Time) RoutingKPIs {
	window = NormalizeWindow(window)
	now = now.UTC()
	since := now.Add(-WindowDuration(window))
	out := RoutingKPIs{Window: window, ByUpstream: []UpstreamKPI{}}
	if len(entries) == 0 {
		return out
	}

	type acc struct {
		requests               int64
		s2xx, s429, s4xx, s5xx int64
		latencies              []int64
	}
	groups := make(map[string]*acc)

	for _, entry := range entries {
		if entry.CreatedAt.IsZero() {
			continue
		}
		created := entry.CreatedAt.UTC()
		if created.Before(since) {
			continue
		}
		key := normalizeUpstream(entry.Upstream)
		bucket, ok := groups[key]
		if !ok {
			bucket = &acc{}
			groups[key] = bucket
		}
		bucket.requests++
		out.Requests++
		switch {
		case entry.Status == 429:
			bucket.s429++
		case entry.Status >= 500:
			bucket.s5xx++
		case entry.Status >= 400 && entry.Status < 500:
			bucket.s4xx++
		case entry.Status >= 200 && entry.Status < 300:
			bucket.s2xx++
		}
		bucket.latencies = append(bucket.latencies, entry.LatencyMS)
	}

	if len(groups) == 0 {
		return out
	}

	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	// Stable order: requests desc, then upstream name asc (deterministic JSON).
	sort.Slice(keys, func(i, j int) bool {
		ai, aj := groups[keys[i]], groups[keys[j]]
		if ai.requests != aj.requests {
			return ai.requests > aj.requests
		}
		return keys[i] < keys[j]
	})

	out.ByUpstream = make([]UpstreamKPI, 0, len(keys))
	for _, key := range keys {
		bucket := groups[key]
		item := UpstreamKPI{
			Upstream:  key,
			Requests:  bucket.requests,
			Status2xx: bucket.s2xx,
			Status429: bucket.s429,
			Status4xx: bucket.s4xx,
			Status5xx: bucket.s5xx,
		}
		if bucket.requests > 0 {
			rate := float64(bucket.s2xx) / float64(bucket.requests)
			item.SuccessRate = &rate
		}
		if len(bucket.latencies) > 0 {
			sort.Slice(bucket.latencies, func(i, j int) bool {
				return bucket.latencies[i] < bucket.latencies[j]
			})
			p50 := percentileNearestRank(bucket.latencies, 0.50)
			p95 := percentileNearestRank(bucket.latencies, 0.95)
			item.LatencyP50MS = &p50
			item.LatencyP95MS = &p95
		}
		out.ByUpstream = append(out.ByUpstream, item)
	}
	return out
}

func normalizeUpstream(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return UnknownUpstream
	}
	return raw
}
