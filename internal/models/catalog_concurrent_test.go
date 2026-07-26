package models

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCatalog_Refresh_coalesces_concurrent_callers(t *testing.T) {
	// Given
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		close(started)
		<-release
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"object":"list","data":[{"id":"shared-free"}]}`))
	}))
	defer server.Close()
	catalog := newCatalog(t, server.URL, Settings{TTL: time.Minute})
	results := make(chan error, 2)

	// When
	go func() {
		_, err := catalog.Refresh(context.Background())
		results <- err
	}()
	<-started
	go func() {
		_, err := catalog.Refresh(context.Background())
		results <- err
	}()
	close(release)

	// Then
	for range 2 {
		if err := <-results; err != nil {
			t.Fatalf("Refresh() error = %v", err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("upstream calls = %d, want 1", calls.Load())
	}
}

func TestCatalog_Refresh_returns_callers_cancellation_while_waiting(t *testing.T) {
	// Given
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		close(started)
		<-release
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"object":"list","data":[{"id":"shared-free"}]}`))
	}))
	defer server.Close()
	catalog := newCatalog(t, server.URL, Settings{TTL: time.Minute})
	var group sync.WaitGroup
	group.Add(1)
	go func() {
		defer group.Done()
		_, _ = catalog.Refresh(context.Background())
	}()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	_, err := catalog.Refresh(ctx)
	close(release)
	group.Wait()

	// Then
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
