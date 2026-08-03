package httpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jovepoxy/internal/config"
	"jovepoxy/internal/httpserver"
	"jovepoxy/internal/keys"
	"jovepoxy/internal/models"
	"jovepoxy/internal/zen"
)

func TestServer_rejects_unknown_and_revoked_local_keys(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer upstream.Close()
	server := newServer(t, upstream.URL, []zen.Model{{ID: "demo-free"}})
	revoked, err := server.keys.Create(context.Background(), keys.CreateInput{Label: "revoked"})
	if err != nil {
		t.Fatalf("create revoked key: %v", err)
	}
	if err := server.keys.Revoke(context.Background(), revoked.ID); err != nil {
		t.Fatalf("revoke key: %v", err)
	}

	for _, credential := range []string{"Bearer sk-oc-" + strings.Repeat("0", 64), "Bearer " + revoked.Secret} {
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"demo-free","messages":[]}`))
		request.Header.Set("Authorization", credential)
		recorder := httptest.NewRecorder()

		// When
		server.handler.ServeHTTP(recorder, request)

		// Then
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
		}
	}
}

func TestServer_exposes_only_dynamic_free_models_and_public_health(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer upstream.Close()
	server := newServer(t, upstream.URL, []zen.Model{{ID: "dynamic-free"}, {ID: "paid-model"}})

	// When
	modelsRecorder := httptest.NewRecorder()
	server.handler.ServeHTTP(modelsRecorder, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	healthRecorder := httptest.NewRecorder()
	server.handler.ServeHTTP(healthRecorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	// Then
	if modelsRecorder.Code != http.StatusOK || healthRecorder.Code != http.StatusOK {
		t.Fatalf("public endpoint statuses = %d, %d", modelsRecorder.Code, healthRecorder.Code)
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(modelsRecorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode models response: %v", err)
	}
	if len(payload.Data) != 1 || payload.Data[0].ID != "dynamic-free" {
		t.Fatalf("models = %+v, want only dynamic-free", payload.Data)
	}
	var health struct {
		Status     string `json:"status"`
		Version    string `json:"version"`
		ModelCount int    `json:"model_count"`
		Upstream   struct {
			Status string `json:"status"`
		} `json:"upstream"`
	}
	if err := json.Unmarshal(healthRecorder.Body.Bytes(), &health); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if health.Status != "ok" || health.Version == "" || health.ModelCount != 2 || health.Upstream.Status != "ok" {
		t.Fatalf("health = %+v", health)
	}
	if strings.Contains(healthRecorder.Body.String(), "secret") {
		t.Fatal("health response exposed a secret")
	}
}

func TestServer_rejects_paid_model_without_zen_keys(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer upstream.Close()
	server := newServer(t, upstream.URL, []zen.Model{{ID: "paid-model"}})
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"paid-model","messages":[]}`))
	request.Header.Set("x-api-key", server.key)
	recorder := httptest.NewRecorder()

	// When
	server.handler.ServeHTTP(recorder, request)

	// Then
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "no zen API keys") {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestServer_maps_first_sse_error_and_empty_body(t *testing.T) {
	for _, scenario := range []struct {
		name       string
		response   string
		wantStatus int
	}{
		{name: "free usage error", response: `data: {"type":"FreeUsageLimitError"}` + "\n\n", wantStatus: http.StatusTooManyRequests},
		{name: "free usage error with comment line", response: ": keep-alive\ndata: {\"type\":\"FreeUsageLimitError\"}\n\n", wantStatus: http.StatusTooManyRequests},
		{name: "free usage error full first event", response: "event: error\ndata: {\"type\":\"FreeUsageLimitError\"}\n\n", wantStatus: http.StatusTooManyRequests},
		{name: "server_error is not rate limit", response: `data: {"error":{"type":"server_error","message":"boom"}}` + "\n\n", wantStatus: http.StatusBadGateway},
		{name: "empty response", response: "", wantStatus: http.StatusBadGateway},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			// Given
			upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Content-Type", "text/event-stream")
				_, _ = writer.Write([]byte(scenario.response))
			}))
			defer upstream.Close()
			server := newServer(t, upstream.URL, []zen.Model{{ID: "demo-free"}})
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"demo-free","messages":[],"stream":true}`))
			request.Header.Set("Authorization", "Bearer "+server.key)
			recorder := httptest.NewRecorder()

			// When
			server.handler.ServeHTTP(recorder, request)

			// Then
			if recorder.Code != scenario.wantStatus {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, scenario.wantStatus, recorder.Body.String())
			}
		})
	}
}

func TestServer_flushes_sse_chunks_and_maps_timeout(t *testing.T) {
	// Given
	streamUpstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte("data: first\n\ndata: second\n\n"))
	}))
	defer streamUpstream.Close()
	server := newServer(t, streamUpstream.URL, []zen.Model{{ID: "demo-free"}})
	streamRequest := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"demo-free","messages":[],"stream":true}`))
	streamRequest.Header.Set("Authorization", "Bearer "+server.key)
	streamRecorder := httptest.NewRecorder()

	// When
	server.handler.ServeHTTP(streamRecorder, streamRequest)

	// Then
	if !streamRecorder.Flushed || streamRecorder.Code != http.StatusOK || !bytes.Contains(streamRecorder.Body.Bytes(), []byte("data: second")) {
		t.Fatalf("stream response = %d flushed=%t body=%s", streamRecorder.Code, streamRecorder.Flushed, streamRecorder.Body.String())
	}

	// Given: upstream delays headers longer than the Zen client timeout
	timeoutUpstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"id":"too-late"}`))
	}))
	defer timeoutUpstream.Close()
	timeoutServer := newServerWithTimeout(t, timeoutUpstream.URL, 40*time.Millisecond)
	timeoutRequest := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"demo-free","messages":[]}`))
	timeoutRequest.Header.Set("Authorization", "Bearer "+timeoutServer.key)
	timeoutRecorder := httptest.NewRecorder()

	// When
	timeoutServer.handler.ServeHTTP(timeoutRecorder, timeoutRequest)

	// Then
	if timeoutRecorder.Code != http.StatusGatewayTimeout {
		t.Fatalf("timeout status = %d, want %d body=%s", timeoutRecorder.Code, http.StatusGatewayTimeout, timeoutRecorder.Body.String())
	}
}

func TestServer_cancels_upstream_when_client_disconnects(t *testing.T) {
	// Given: upstream blocks until the outbound request is abandoned.
	// On this host httptest may keep the server Request.Context alive after the
	// client cancels; the contract we assert is the proxy stops waiting.
	started := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		close(started)
		select {
		case <-request.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer upstream.Close()
	server := newServer(t, upstream.URL, []zen.Model{{ID: "demo-free"}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"demo-free","messages":[]}`))
	request.Header.Set("Authorization", "Bearer "+server.key)
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		server.handler.ServeHTTP(recorder, request)
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream did not receive the request")
	}

	// When
	cancel()

	// Then: handler returns promptly because request.Context() is passed to Zen.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return after client disconnect")
	}
}

func newServerWithTimeout(t *testing.T, upstreamURL string, timeout time.Duration) testServer {
	t.Helper()
	base := newServer(t, upstreamURL, []zen.Model{{ID: "demo-free"}})
	client, err := zen.NewClient(config.Config{ZenBase: upstreamURL, OCVersion: "test", UpstreamTimeout: timeout})
	if err != nil {
		t.Fatalf("new Zen client: %v", err)
	}
	catalog, err := models.NewCatalog(testModelSource{models: []zen.Model{{ID: "demo-free"}}}, models.Settings{TTL: time.Hour})
	if err != nil {
		t.Fatalf("new catalog: %v", err)
	}
	base.handler = httpserver.New(httpserver.Dependencies{
		Keys: base.keys, Catalog: catalog, Zen: client, Version: "test-version",
		// Pool intentionally omitted: timeout path only exercises free models.
	})
	return base
}
