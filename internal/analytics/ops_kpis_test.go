package analytics_test

import (
	"testing"
	"time"

	"jovepoxy/internal/analytics"
	"jovepoxy/internal/reqlog"
)

func TestNormalizeWindow(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "24h"},
		{"24h", "24h"},
		{"1d", "24h"},
		{"1h", "1h"},
		{"7d", "7d"},
		{"WEEK", "7d"},
		{"bogus", "24h"},
		{" 1H ", "1h"},
	}
	for _, tc := range cases {
		if got := analytics.NormalizeWindow(tc.in); got != tc.want {
			t.Fatalf("NormalizeWindow(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAggregateOpsKPIs_empty(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	kpis := analytics.AggregateOpsKPIs(nil, "24h", now)
	if kpis.Window != "24h" || kpis.Requests != 0 || kpis.SuccessRate != nil {
		t.Fatalf("empty = %+v", kpis)
	}
	if kpis.LatencyP50MS != nil || kpis.LatencyP95MS != nil {
		t.Fatalf("empty should omit percentiles: %+v", kpis)
	}
}

func TestAggregateOpsKPIs_all_2xx(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	entries := []reqlog.Entry{
		{Status: 200, LatencyMS: 10, CreatedAt: now.Add(-30 * time.Minute)},
		{Status: 201, LatencyMS: 20, CreatedAt: now.Add(-20 * time.Minute)},
		{Status: 204, LatencyMS: 40, CreatedAt: now.Add(-10 * time.Minute)},
	}
	kpis := analytics.AggregateOpsKPIs(entries, "1h", now)
	if kpis.Requests != 3 || kpis.Status2xx != 3 {
		t.Fatalf("counts = %+v", kpis)
	}
	if kpis.SuccessRate == nil || *kpis.SuccessRate != 1.0 {
		t.Fatalf("success_rate = %v", kpis.SuccessRate)
	}
	if kpis.LatencyP50MS == nil || *kpis.LatencyP50MS != 20 {
		t.Fatalf("p50 = %v", kpis.LatencyP50MS)
	}
	if kpis.LatencyP95MS == nil || *kpis.LatencyP95MS != 40 {
		t.Fatalf("p95 = %v", kpis.LatencyP95MS)
	}
}

func TestAggregateOpsKPIs_mixed_and_window_filter(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	entries := []reqlog.Entry{
		// outside 1h window
		{Status: 200, LatencyMS: 1, CreatedAt: now.Add(-2 * time.Hour)},
		// in window
		{Status: 200, LatencyMS: 100, CreatedAt: now.Add(-30 * time.Minute)},
		{Status: 429, LatencyMS: 50, CreatedAt: now.Add(-20 * time.Minute)},
		{Status: 500, LatencyMS: 200, CreatedAt: now.Add(-10 * time.Minute)},
		{Status: 401, LatencyMS: 80, CreatedAt: now.Add(-5 * time.Minute)},
		// zero CreatedAt skipped
		{Status: 200, LatencyMS: 999},
	}
	kpis := analytics.AggregateOpsKPIs(entries, "1h", now)
	if kpis.Requests != 4 {
		t.Fatalf("requests = %d want 4; kpis=%+v", kpis.Requests, kpis)
	}
	if kpis.Status2xx != 1 || kpis.Status429 != 1 || kpis.Status5xx != 1 {
		t.Fatalf("status buckets = %+v", kpis)
	}
	// success_rate = 1/4
	if kpis.SuccessRate == nil || *kpis.SuccessRate != 0.25 {
		t.Fatalf("success_rate = %v want 0.25", kpis.SuccessRate)
	}
}

func TestAggregateOpsKPIs_quantile_boundary(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	// single sample → p50 == p95
	one := []reqlog.Entry{
		{Status: 200, LatencyMS: 42, CreatedAt: now.Add(-1 * time.Minute)},
	}
	kpis := analytics.AggregateOpsKPIs(one, "24h", now)
	if kpis.LatencyP50MS == nil || *kpis.LatencyP50MS != 42 {
		t.Fatalf("single p50 = %v", kpis.LatencyP50MS)
	}
	if kpis.LatencyP95MS == nil || *kpis.LatencyP95MS != 42 {
		t.Fatalf("single p95 = %v", kpis.LatencyP95MS)
	}

	// 20 samples: nearest-rank p50 = ceil(0.5*20)=10th (0-index 9), p95 = ceil(0.95*20)=19th (0-index 18)
	entries := make([]reqlog.Entry, 0, 20)
	for i := 1; i <= 20; i++ {
		entries = append(entries, reqlog.Entry{
			Status: 200, LatencyMS: int64(i * 10), CreatedAt: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	kpis = analytics.AggregateOpsKPIs(entries, "24h", now)
	if kpis.LatencyP50MS == nil || *kpis.LatencyP50MS != 100 {
		t.Fatalf("p50 = %v want 100", kpis.LatencyP50MS)
	}
	if kpis.LatencyP95MS == nil || *kpis.LatencyP95MS != 190 {
		t.Fatalf("p95 = %v want 190", kpis.LatencyP95MS)
	}
}

func TestAggregateOpsKPIs_default_window(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	entries := []reqlog.Entry{
		{Status: 200, LatencyMS: 10, CreatedAt: now.Add(-2 * time.Hour)},  // in 24h
		{Status: 200, LatencyMS: 10, CreatedAt: now.Add(-48 * time.Hour)}, // out of 24h
	}
	kpis := analytics.AggregateOpsKPIs(entries, "", now)
	if kpis.Window != "24h" || kpis.Requests != 1 {
		t.Fatalf("default window kpis = %+v", kpis)
	}
}
