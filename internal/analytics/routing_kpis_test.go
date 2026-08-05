package analytics_test

import (
	"testing"
	"time"

	"jovepoxy/internal/analytics"
	"jovepoxy/internal/reqlog"
)

func TestAggregateRoutingKPIs_empty(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	kpis := analytics.AggregateRoutingKPIs(nil, "24h", now)
	if kpis.Window != "24h" || kpis.Requests != 0 {
		t.Fatalf("empty = %+v", kpis)
	}
	if kpis.ByUpstream == nil || len(kpis.ByUpstream) != 0 {
		t.Fatalf("by_upstream must be empty non-nil slice: %#v", kpis.ByUpstream)
	}
}

func TestAggregateRoutingKPIs_groups_by_upstream(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	entries := []reqlog.Entry{
		{Upstream: "opencode_paid", Status: 200, LatencyMS: 10, CreatedAt: now.Add(-10 * time.Minute)},
		{Upstream: "opencode_paid", Status: 200, LatencyMS: 30, CreatedAt: now.Add(-9 * time.Minute)},
		{Upstream: "opencode_paid", Status: 500, LatencyMS: 40, CreatedAt: now.Add(-8 * time.Minute)},
		{Upstream: "ollama_paid", Status: 200, LatencyMS: 20, CreatedAt: now.Add(-7 * time.Minute)},
		{Upstream: "ollama_paid", Status: 429, LatencyMS: 15, CreatedAt: now.Add(-6 * time.Minute)},
		{Upstream: "opencode_free", Status: 200, LatencyMS: 5, CreatedAt: now.Add(-5 * time.Minute)},
		// outside 1h
		{Upstream: "opencode_paid", Status: 200, LatencyMS: 1, CreatedAt: now.Add(-2 * time.Hour)},
		// zero CreatedAt skipped
		{Upstream: "ollama_paid", Status: 200, LatencyMS: 99},
	}
	kpis := analytics.AggregateRoutingKPIs(entries, "1h", now)
	if kpis.Window != "1h" || kpis.Requests != 6 {
		t.Fatalf("totals = %+v", kpis)
	}
	if len(kpis.ByUpstream) != 3 {
		t.Fatalf("groups = %d want 3: %+v", len(kpis.ByUpstream), kpis.ByUpstream)
	}
	// Sorted by requests desc: opencode_paid(3), ollama_paid(2), opencode_free(1)
	if kpis.ByUpstream[0].Upstream != "opencode_paid" || kpis.ByUpstream[0].Requests != 3 {
		t.Fatalf("first = %+v", kpis.ByUpstream[0])
	}
	if kpis.ByUpstream[0].Status2xx != 2 || kpis.ByUpstream[0].Status5xx != 1 {
		t.Fatalf("opencode_paid buckets = %+v", kpis.ByUpstream[0])
	}
	if kpis.ByUpstream[0].SuccessRate == nil || *kpis.ByUpstream[0].SuccessRate != float64(2)/float64(3) {
		t.Fatalf("opencode_paid success = %v", kpis.ByUpstream[0].SuccessRate)
	}
	if kpis.ByUpstream[0].LatencyP50MS == nil || *kpis.ByUpstream[0].LatencyP50MS != 30 {
		// latencies 10,30,40 → nearest-rank p50 = ceil(0.5*3)=2nd → 30
		t.Fatalf("opencode_paid p50 = %v want 30", kpis.ByUpstream[0].LatencyP50MS)
	}
	if kpis.ByUpstream[1].Upstream != "ollama_paid" || kpis.ByUpstream[1].Requests != 2 {
		t.Fatalf("second = %+v", kpis.ByUpstream[1])
	}
	if kpis.ByUpstream[1].Status2xx != 1 || kpis.ByUpstream[1].Status429 != 1 {
		t.Fatalf("ollama_paid buckets = %+v", kpis.ByUpstream[1])
	}
	if kpis.ByUpstream[2].Upstream != "opencode_free" || kpis.ByUpstream[2].Requests != 1 {
		t.Fatalf("third = %+v", kpis.ByUpstream[2])
	}
}

func TestAggregateRoutingKPIs_empty_upstream_is_unknown(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	entries := []reqlog.Entry{
		{Upstream: "", Status: 200, LatencyMS: 10, CreatedAt: now.Add(-5 * time.Minute)},
		{Upstream: "   ", Status: 401, LatencyMS: 20, CreatedAt: now.Add(-4 * time.Minute)},
		{Upstream: "opencode_paid", Status: 200, LatencyMS: 15, CreatedAt: now.Add(-3 * time.Minute)},
	}
	kpis := analytics.AggregateRoutingKPIs(entries, "24h", now)
	if kpis.Requests != 3 {
		t.Fatalf("requests = %d", kpis.Requests)
	}
	var unknown, paid *analytics.UpstreamKPI
	for i := range kpis.ByUpstream {
		item := &kpis.ByUpstream[i]
		switch item.Upstream {
		case analytics.UnknownUpstream:
			unknown = item
		case "opencode_paid":
			paid = item
		}
	}
	if unknown == nil || unknown.Requests != 2 {
		t.Fatalf("unknown bucket = %+v", unknown)
	}
	if unknown.Status2xx != 1 || unknown.Status4xx != 1 {
		t.Fatalf("unknown buckets = %+v", unknown)
	}
	if paid == nil || paid.Requests != 1 {
		t.Fatalf("paid must stay independent: paid=%+v all=%+v", paid, kpis.ByUpstream)
	}
	// Empty upstream must not appear as "" or as a paid channel.
	for _, item := range kpis.ByUpstream {
		if item.Upstream == "" {
			t.Fatalf("empty upstream label leaked: %+v", item)
		}
	}
}

func TestAggregateRoutingKPIs_window_and_status_buckets(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	entries := []reqlog.Entry{
		{Upstream: "ollama_paid", Status: 200, LatencyMS: 100, CreatedAt: now.Add(-30 * time.Minute)},
		{Upstream: "ollama_paid", Status: 429, LatencyMS: 50, CreatedAt: now.Add(-20 * time.Minute)},
		{Upstream: "ollama_paid", Status: 503, LatencyMS: 200, CreatedAt: now.Add(-10 * time.Minute)},
		{Upstream: "ollama_paid", Status: 400, LatencyMS: 80, CreatedAt: now.Add(-5 * time.Minute)},
		// outside 1h
		{Upstream: "ollama_paid", Status: 200, LatencyMS: 1, CreatedAt: now.Add(-90 * time.Minute)},
	}
	kpis := analytics.AggregateRoutingKPIs(entries, "1h", now)
	if kpis.Requests != 4 || len(kpis.ByUpstream) != 1 {
		t.Fatalf("kpis = %+v", kpis)
	}
	item := kpis.ByUpstream[0]
	if item.Status2xx != 1 || item.Status429 != 1 || item.Status4xx != 1 || item.Status5xx != 1 {
		t.Fatalf("status buckets = %+v", item)
	}
	if item.SuccessRate == nil || *item.SuccessRate != 0.25 {
		t.Fatalf("success_rate = %v want 0.25", item.SuccessRate)
	}
}

func TestAggregateRoutingKPIs_default_window_and_tie_order(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	entries := []reqlog.Entry{
		{Upstream: "zulu", Status: 200, LatencyMS: 10, CreatedAt: now.Add(-2 * time.Hour)},
		{Upstream: "alpha", Status: 200, LatencyMS: 10, CreatedAt: now.Add(-2 * time.Hour)},
		// outside 24h default
		{Upstream: "alpha", Status: 200, LatencyMS: 10, CreatedAt: now.Add(-48 * time.Hour)},
	}
	kpis := analytics.AggregateRoutingKPIs(entries, "", now)
	if kpis.Window != "24h" || kpis.Requests != 2 {
		t.Fatalf("default window kpis = %+v", kpis)
	}
	// Equal requests → alphabetical: alpha before zulu
	if len(kpis.ByUpstream) != 2 || kpis.ByUpstream[0].Upstream != "alpha" || kpis.ByUpstream[1].Upstream != "zulu" {
		t.Fatalf("tie order = %+v", kpis.ByUpstream)
	}
}

func TestAggregateRoutingKPIs_single_sample_percentiles(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	entries := []reqlog.Entry{
		{Upstream: "opencode_paid", Status: 200, LatencyMS: 42, CreatedAt: now.Add(-1 * time.Minute)},
	}
	kpis := analytics.AggregateRoutingKPIs(entries, "24h", now)
	if len(kpis.ByUpstream) != 1 {
		t.Fatalf("groups = %+v", kpis.ByUpstream)
	}
	item := kpis.ByUpstream[0]
	if item.LatencyP50MS == nil || *item.LatencyP50MS != 42 {
		t.Fatalf("p50 = %v", item.LatencyP50MS)
	}
	if item.LatencyP95MS == nil || *item.LatencyP95MS != 42 {
		t.Fatalf("p95 = %v", item.LatencyP95MS)
	}
}
