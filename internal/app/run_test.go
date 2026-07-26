package app_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jovepoxy/internal/app"
	"jovepoxy/internal/config"
)

func TestBootstrap_serves_public_health_without_secrets(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/models" && !strings.HasSuffix(request.URL.Path, "/models") {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"object":"list","data":[{"id":"demo-free"}]}`))
	}))
	defer upstream.Close()

	dataDir := t.TempDir()
	cfg := config.Config{
		Listen:          "127.0.0.1:0",
		DataDir:         dataDir,
		AdminPassword:   "test-admin-password",
		AdminSecret:     strings.Repeat("s", 32),
		ZenBase:         upstream.URL,
		ModelCacheTTL:   time.Hour,
		OCVersion:       "test",
		UpstreamTimeout: time.Second,
	}

	// When
	runtime, err := app.Bootstrap(context.Background(), cfg)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { _ = runtime.Close() })

	recorder := httptest.NewRecorder()
	runtime.Handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	// Then
	if recorder.Code != http.StatusOK {
		t.Fatalf("health status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if payload["version"] != app.Version {
		t.Fatalf("version = %v, want %s", payload["version"], app.Version)
	}
	if strings.Contains(recorder.Body.String(), cfg.AdminSecret) || strings.Contains(recorder.Body.String(), cfg.AdminPassword) {
		t.Fatal("health response leaked admin credentials")
	}
	if _, err := os.Stat(filepath.Join(dataDir, "jovepoxy.db")); err != nil {
		t.Fatalf("expected sqlite database file: %v", err)
	}
}

func TestRun_listens_and_shuts_down(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"object":"list","data":[{"id":"demo-free"}]}`))
	}))
	defer upstream.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()

	t.Setenv("ADMIN_PASSWORD", "test-admin-password")
	t.Setenv("ADMIN_SECRET", strings.Repeat("s", 32))
	t.Setenv("DATA_DIR", t.TempDir())
	t.Setenv("LISTEN", addr)
	t.Setenv("ZEN_BASE", upstream.URL)
	t.Setenv("MODEL_CACHE_TTL", "1h")
	t.Setenv("UPSTREAM_TIMEOUT", "2s")

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- app.Run(ctx) }()

	// When: wait until the server accepts connections
	deadline := time.Now().Add(5 * time.Second)
	var health *http.Response
	for time.Now().Before(deadline) {
		request, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/health", nil)
		health, err = http.DefaultClient.Do(request)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		cancel()
		t.Fatalf("health request failed: %v", err)
	}
	defer health.Body.Close()
	if health.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("health status = %d", health.StatusCode)
	}

	// Then: graceful shutdown returns
	cancel()
	select {
	case runErr := <-errCh:
		if runErr != nil {
			t.Fatalf("run returned error: %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down")
	}
}
