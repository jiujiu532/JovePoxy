package models

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCatalog_List_keeps_stale_snapshot_cached_until_TTL_expires_after_failed_refresh(t *testing.T) {
	// Given
	var calls atomic.Int32
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		if fail.Load() {
			writer.WriteHeader(http.StatusBadGateway)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"object":"list","data":[{"id":"cached-free"}]}`))
	}))
	defer server.Close()
	const ttl = time.Minute
	catalog := newCatalog(t, server.URL, Settings{TTL: ttl})
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	catalog.now = func() time.Time { return now }
	if _, err := catalog.Refresh(context.Background()); err != nil {
		t.Fatalf("initial Refresh() error = %v", err)
	}
	fail.Store(true)
	if result, err := catalog.Refresh(context.Background()); err != nil || !result.Stale {
		t.Fatalf("failed Refresh() = %#v, %v, want stale snapshot", result, err)
	}

	// When
	withinTTL, withinTTLErr := catalog.List(context.Background())
	now = now.Add(ttl)
	afterTTL, afterTTLErr := catalog.List(context.Background())

	// Then
	if withinTTLErr != nil || !withinTTL.Stale {
		t.Fatalf("List() within TTL = %#v, %v, want stale snapshot", withinTTL, withinTTLErr)
	}
	if afterTTLErr != nil || !afterTTL.Stale {
		t.Fatalf("List() after TTL = %#v, %v, want stale snapshot", afterTTL, afterTTLErr)
	}
	if calls.Load() != 3 {
		t.Fatalf("upstream calls = %d, want 3: success, forced failure, expired retry", calls.Load())
	}
}
