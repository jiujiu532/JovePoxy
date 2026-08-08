package reqlog_test

import (
	"context"
	"testing"
	"time"

	"jovepoxy/internal/db"
	"jovepoxy/internal/reqlog"
)

func TestService_record_persists_and_counts(t *testing.T) {
	// Given
	database, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	service := reqlog.NewService(database, nil)

	// When
	service.Record(context.Background(), reqlog.Entry{
		Model: "demo-free", Route: "/v1/chat/completions", Upstream: "opencode_free", ProxyID: "px_demo", ProxyLabel: "edge-a", ProxyHost: "1.2.3.4:2260", Status: 200, LatencyMS: 12, TTFTMS: 5, Stream: true,
	})
	service.Record(context.Background(), reqlog.Entry{
		Model: "demo-free", Route: "/v1/messages", Status: 429, LatencyMS: 3,
	})
	service.Record(context.Background(), reqlog.Entry{
		Model: "demo-free", Route: "/v1/chat/completions", Status: 401, LatencyMS: 4,
	})

	// Then
	snapshot := service.Snapshot()
	if snapshot.TotalRequests != 3 || snapshot.Status429 != 1 || snapshot.Status4xx != 1 || snapshot.Status2xx != 1 || snapshot.StreamRequests != 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	list, err := service.List(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("list len = %d", len(list))
	}
	var foundTTFT bool
	for _, item := range list {
		if item.Route == "/v1/chat/completions" && item.Status == 200 {
			if item.TTFTMS != 5 {
				t.Fatalf("ttft_ms = %d, want 5", item.TTFTMS)
			}
			if item.Upstream != "opencode_free" {
				t.Fatalf("upstream = %q, want opencode_free", item.Upstream)
			}
			if item.ProxyID != "px_demo" || item.ProxyLabel != "edge-a" || item.ProxyHost != "1.2.3.4:2260" {
				t.Fatalf("proxy fields = id=%q label=%q host=%q", item.ProxyID, item.ProxyLabel, item.ProxyHost)
			}
			foundTTFT = true
		}
	}
	if !foundTTFT {
		t.Fatal("expected stream 200 entry with ttft_ms")
	}
	recent := service.Recent(1)
	if len(recent) != 1 || recent[0].Status != 401 {
		t.Fatalf("recent = %+v", recent)
	}
}

func TestService_record_ignores_store_failure(t *testing.T) {
	// Given
	service := reqlog.NewServiceWithStore(failingStore{}, nil, 8)

	// When / Then: must not panic
	service.Record(context.Background(), reqlog.Entry{
		Model: "x", Route: "/v1/chat/completions", Status: 500, CreatedAt: time.Now(),
	})
	if service.Snapshot().TotalRequests != 1 || service.Snapshot().Status5xx != 1 {
		t.Fatalf("counters not updated on store failure: %+v", service.Snapshot())
	}
}

func TestService_list_filters_by_created_at(t *testing.T) {
	// Given
	database, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	service := reqlog.NewService(database, nil)

	t1 := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 7, 30, 14, 0, 0, 0, time.UTC)
	service.Record(context.Background(), reqlog.Entry{
		ID: "rl_early", Model: "m1", Route: "/v1/chat/completions", Status: 200, CreatedAt: t1,
	})
	service.Record(context.Background(), reqlog.Entry{
		ID: "rl_mid", Model: "m2", Route: "/v1/chat/completions", Status: 200, CreatedAt: t2,
	})
	service.Record(context.Background(), reqlog.Entry{
		ID: "rl_late", Model: "m3", Route: "/v1/chat/completions", Status: 200, CreatedAt: t3,
	})

	// When: closed interval [t2, t3]
	list, err := service.ListFiltered(context.Background(), reqlog.ListFilter{
		From: t2, To: t3, Limit: 10,
	})
	if err != nil {
		t.Fatalf("list filtered: %v", err)
	}

	// Then
	if len(list) != 2 {
		t.Fatalf("filtered len = %d want 2; list=%+v", len(list), list)
	}
	ids := map[string]bool{}
	for _, entry := range list {
		ids[entry.ID] = true
	}
	if !ids["rl_mid"] || !ids["rl_late"] || ids["rl_early"] {
		t.Fatalf("unexpected ids: %+v", ids)
	}

	// When: only lower bound
	fromOnly, err := service.ListFiltered(context.Background(), reqlog.ListFilter{
		From: t3, Limit: 10,
	})
	if err != nil {
		t.Fatalf("from-only list: %v", err)
	}
	if len(fromOnly) != 1 || fromOnly[0].ID != "rl_late" {
		t.Fatalf("from-only = %+v", fromOnly)
	}

	// When: only upper bound
	toOnly, err := service.ListFiltered(context.Background(), reqlog.ListFilter{
		To: t1, Limit: 10,
	})
	if err != nil {
		t.Fatalf("to-only list: %v", err)
	}
	if len(toOnly) != 1 || toOnly[0].ID != "rl_early" {
		t.Fatalf("to-only = %+v", toOnly)
	}

	// When: unfiltered List still works
	all, err := service.List(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("list all len = %d", len(all))
	}
}

func TestService_list_filters_fractional_seconds(t *testing.T) {
	// Given: times that break variable-width RFC3339Nano string order
	// (e.g. "...00.5Z" would sort before "...00Z" without fixed fractional padding).
	database, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	service := reqlog.NewService(database, nil)

	tWhole := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	tHalf := time.Date(2026, 7, 30, 12, 0, 0, 500_000_000, time.UTC) // 12:00:00.5
	tNext := time.Date(2026, 7, 30, 12, 0, 1, 0, time.UTC)
	service.Record(context.Background(), reqlog.Entry{
		ID: "rl_whole", Model: "m1", Route: "/v1/chat/completions", Status: 200, CreatedAt: tWhole,
	})
	service.Record(context.Background(), reqlog.Entry{
		ID: "rl_half", Model: "m2", Route: "/v1/chat/completions", Status: 200, CreatedAt: tHalf,
	})
	service.Record(context.Background(), reqlog.Entry{
		ID: "rl_next", Model: "m3", Route: "/v1/chat/completions", Status: 200, CreatedAt: tNext,
	})

	// When: lower bound at whole second must still include the half-second row
	list, err := service.ListFiltered(context.Background(), reqlog.ListFilter{
		From: tWhole, To: tHalf, Limit: 10,
	})
	if err != nil {
		t.Fatalf("list filtered: %v", err)
	}

	// Then
	if len(list) != 2 {
		t.Fatalf("filtered len = %d want 2; list=%+v", len(list), list)
	}
	ids := map[string]bool{}
	for _, entry := range list {
		ids[entry.ID] = true
	}
	if !ids["rl_whole"] || !ids["rl_half"] || ids["rl_next"] {
		t.Fatalf("unexpected ids: %+v", ids)
	}

	// Newest-first order must be chronological (half then whole)
	if list[0].ID != "rl_half" || list[1].ID != "rl_whole" {
		t.Fatalf("order = [%s, %s] want half then whole", list[0].ID, list[1].ID)
	}
}

type failingStore struct{}

func (failingStore) Insert(context.Context, reqlog.Entry) error { return context.Canceled }
func (failingStore) List(context.Context, reqlog.ListFilter) ([]reqlog.Entry, error) {
	return nil, context.Canceled
}

func TestService_record_persists_generation_meta(t *testing.T) {
	database, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	service := reqlog.NewService(database, nil)

	service.Record(context.Background(), reqlog.Entry{
		Model: "deepseek-v4-flash-free", Route: "/v1/messages", Status: 200, LatencyMS: 100,
		Stream: true, MaxTokens: 128000, ReasoningEffort: "xhigh", ThinkingType: "enabled", BudgetTokens: 32000,
	})
	list, err := service.List(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len=%d", len(list))
	}
	entry := list[0]
	if entry.MaxTokens != 128000 || entry.ReasoningEffort != "xhigh" || entry.ThinkingType != "enabled" || entry.BudgetTokens != 32000 {
		t.Fatalf("meta = %+v", entry)
	}
}

