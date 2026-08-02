package analytics

import (
	"math"
	"sort"
	"strings"
	"time"

	"jovepoxy/internal/reqlog"
)

// OpsKPIs is time-windowed request health from reqlog metadata (no bodies).
// SuccessRate is nil when Requests==0 (documented empty-window behavior).
// Latency percentiles are omitted when there are no samples in the window.
type OpsKPIs struct {
	Window       string   `json:"window"`
	Requests     int64    `json:"requests"`
	SuccessRate  *float64 `json:"success_rate"` // 0..1; nil when requests==0
	LatencyP50MS *int64   `json:"latency_p50_ms,omitempty"`
	LatencyP95MS *int64   `json:"latency_p95_ms,omitempty"`
	Status2xx    int64    `json:"status_2xx"`
	Status429    int64    `json:"status_429"`
	Status4xx    int64    `json:"status_4xx"` // 400–499 excluding 429
	Status5xx    int64    `json:"status_5xx"`
}

// NormalizeWindow maps query values to 1h|24h|7d. Unknown/empty → 24h.
func NormalizeWindow(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1h", "1hour", "hour":
		return "1h"
	case "7d", "7day", "week":
		return "7d"
	case "24h", "1d", "day", "":
		return "24h"
	default:
		return "24h"
	}
}

// WindowDuration returns the lookback duration for a normalized window label.
func WindowDuration(window string) time.Duration {
	switch NormalizeWindow(window) {
	case "1h":
		return time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

// AggregateOpsKPIs filters entries by CreatedAt >= now-window and aggregates KPIs.
// Process-lifetime Snapshot counters must NOT be used as a time window.
// Entries with zero CreatedAt are skipped.
func AggregateOpsKPIs(entries []reqlog.Entry, window string, now time.Time) OpsKPIs {
	window = NormalizeWindow(window)
	now = now.UTC()
	since := now.Add(-WindowDuration(window))
	out := OpsKPIs{Window: window}
	if len(entries) == 0 {
		return out
	}

	latencies := make([]int64, 0, len(entries))
	for _, entry := range entries {
		if entry.CreatedAt.IsZero() {
			continue
		}
		created := entry.CreatedAt.UTC()
		if created.Before(since) {
			continue
		}
		out.Requests++
		switch {
		case entry.Status == 429:
			out.Status429++
		case entry.Status >= 500:
			out.Status5xx++
		case entry.Status >= 400 && entry.Status < 500:
			out.Status4xx++
		case entry.Status >= 200 && entry.Status < 300:
			out.Status2xx++
		}
		latencies = append(latencies, entry.LatencyMS)
	}

	if out.Requests > 0 {
		rate := float64(out.Status2xx) / float64(out.Requests)
		out.SuccessRate = &rate
	}
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		p50 := percentileNearestRank(latencies, 0.50)
		p95 := percentileNearestRank(latencies, 0.95)
		out.LatencyP50MS = &p50
		out.LatencyP95MS = &p95
	}
	return out
}

// percentileNearestRank uses the classic nearest-rank method:
// rank = ceil(p * n), 1-indexed. Stable for small samples (n=1 → only value).
func percentileNearestRank(sorted []int64, p float64) int64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 || p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[n-1]
	}
	rank := int(math.Ceil(p * float64(n)))
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	return sorted[rank-1]
}
