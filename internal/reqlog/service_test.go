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
		Model: "demo-free", Route: "/v1/chat/completions", Status: 200, LatencyMS: 12, Stream: true,
	})
	service.Record(context.Background(), reqlog.Entry{
		Model: "demo-free", Route: "/v1/messages", Status: 429, LatencyMS: 3,
	})

	// Then
	snapshot := service.Snapshot()
	if snapshot.TotalRequests != 2 || snapshot.Status429 != 1 || snapshot.Status2xx != 1 || snapshot.StreamRequests != 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	list, err := service.List(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list len = %d", len(list))
	}
	recent := service.Recent(1)
	if len(recent) != 1 || recent[0].Status != 429 {
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

type failingStore struct{}

func (failingStore) Insert(context.Context, reqlog.Entry) error { return context.Canceled }
func (failingStore) List(context.Context, int, int) ([]reqlog.Entry, error) {
	return nil, context.Canceled
}
