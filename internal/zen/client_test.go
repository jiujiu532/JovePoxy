package zen

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jovepoxy/internal/config"
)

func TestClient_ChatCompletions_sends_public_compatibility_request(t *testing.T) {
	// Given
	body := json.RawMessage(`{"model":"zen-free","stream":true}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", request.Method)
		}
		if request.URL.Path != "/zen/v1/chat/completions" {
			t.Errorf("path = %q, want /zen/v1/chat/completions", request.URL.Path)
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer public" {
			t.Errorf("Authorization = %q, want public authorization", got)
		}
		if got := request.Header.Get("User-Agent"); got != "opencode/9.9.9 ai-sdk/provider-utils/4.0.23 runtime/bun/1.3.13" {
			t.Errorf("User-Agent = %q", got)
		}
		if got := request.Header.Get("x-opencode-client"); got != "cli" {
			t.Errorf("x-opencode-client = %q, want cli", got)
		}
		if got := request.Header.Get("x-opencode-project"); got != "global" {
			t.Errorf("x-opencode-project = %q, want global", got)
		}
		if got := request.Header.Get("x-opencode-request"); !strings.HasPrefix(got, "msg_") || len(got) <= len("msg_") {
			t.Errorf("x-opencode-request = %q, want nonempty msg_ ID", got)
		}
		if got := request.Header.Get("x-opencode-session"); !strings.HasPrefix(got, "ses_") || len(got) <= len("ses_") {
			t.Errorf("x-opencode-session = %q, want nonempty ses_ ID", got)
		}
		gotBody, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		if string(gotBody) != string(body) {
			t.Errorf("body = %s, want %s", gotBody, body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: hello\n\n"))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL+"/zen/v1", time.Second)

	// When
	response, err := client.ChatCompletions(context.Background(), PublicAuth(), body, true)

	// Then
	if err != nil {
		t.Fatalf("ChatCompletions() error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", response.StatusCode)
	}
	got, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if string(got) != "data: hello\n\n" {
		t.Errorf("response body = %q", got)
	}
}

func TestClient_ChatCompletions_sends_supplied_API_key(t *testing.T) {
	// Given
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer paid-secret" {
			t.Errorf("Authorization = %q, want supplied API key", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, time.Second)
	auth, err := NewAPIKey("paid-secret")
	if err != nil {
		t.Fatalf("NewAPIKey() error = %v", err)
	}

	// When
	response, err := client.ChatCompletions(context.Background(), auth, json.RawMessage(`{}`), false)

	// Then
	if err != nil {
		t.Fatalf("ChatCompletions() error = %v", err)
	}
	defer response.Body.Close()
}

func TestClient_ChatCompletions_returns_context_error_when_cancelled(t *testing.T) {
	// Given
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-time.After(time.Second)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, 5*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan error, 1)
	go func() {
		response, err := client.ChatCompletions(ctx, PublicAuth(), json.RawMessage(`{}`), false)
		if response != nil {
			_ = response.Body.Close()
		}
		result <- err
	}()
	<-started

	// When
	cancel()

	// Then
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ChatCompletions() did not return after context cancellation")
	}
}

func TestClient_ChatCompletions_returns_typed_timeout_error_when_upstream_hangs(t *testing.T) {
	// Given
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-time.After(time.Second)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, 20*time.Millisecond)

	// When
	response, err := client.ChatCompletions(context.Background(), PublicAuth(), json.RawMessage(`{}`), false)

	// Then
	if response != nil {
		_ = response.Body.Close()
		t.Fatal("response must be nil on timeout")
	}
	var timeoutError *TimeoutError
	if !errors.As(err, &timeoutError) {
		t.Fatalf("error = %v, want TimeoutError", err)
	}
}

func TestClient_httpClientFor_stream_disables_overall_timeout(t *testing.T) {
	// Given
	const upstream = 120 * time.Millisecond
	client := newTestClient(t, "http://example.invalid", upstream)

	// When / Then
	if client.httpClient.Timeout != upstream {
		t.Fatalf("non-stream client Timeout = %v, want %v", client.httpClient.Timeout, upstream)
	}
	streamClient := client.httpClientFor(true)
	if streamClient.Timeout != 0 {
		t.Fatalf("stream client Timeout = %v, want 0 (no overall body deadline)", streamClient.Timeout)
	}
	if streamClient.Transport != client.httpClient.Transport {
		t.Fatal("stream client must reuse the same Transport (ResponseHeaderTimeout)")
	}
	if client.httpClientFor(false) != client.httpClient {
		t.Fatal("non-stream path must use the primary httpClient")
	}
	if client.upstreamTimeout != upstream {
		t.Fatalf("upstreamTimeout = %v, want %v", client.upstreamTimeout, upstream)
	}
}

func TestClient_ChatCompletions_stream_allows_body_after_overall_timeout_budget(t *testing.T) {
	// Given: headers flush immediately; body arrives after UpstreamTimeout would have fired.
	const upstream = 40 * time.Millisecond
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(3 * upstream)
		_, _ = w.Write([]byte("data: late-chunk\n\n"))
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, upstream)

	// When
	response, err := client.ChatCompletions(context.Background(), PublicAuth(), json.RawMessage(`{"stream":true}`), true)

	// Then
	if err != nil {
		t.Fatalf("stream ChatCompletions() error = %v", err)
	}
	defer response.Body.Close()
	got, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read stream body: %v", err)
	}
	if string(got) != "data: late-chunk\n\n" {
		t.Fatalf("stream body = %q, want late chunk after timeout budget", got)
	}
}

func TestClient_ChatCompletions_stream_still_times_out_waiting_for_headers(t *testing.T) {
	// Given: no response headers within ResponseHeaderTimeout (capped by UpstreamTimeout).
	const upstream = 30 * time.Millisecond
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(time.Second)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, upstream)

	// When
	response, err := client.ChatCompletions(context.Background(), PublicAuth(), json.RawMessage(`{"stream":true}`), true)

	// Then
	if response != nil {
		_ = response.Body.Close()
		t.Fatal("response must be nil when headers never arrive")
	}
	var timeoutError *TimeoutError
	if !errors.As(err, &timeoutError) {
		t.Fatalf("error = %v, want TimeoutError for header wait", err)
	}
}

func TestClient_ChatCompletions_returns_status_error_without_secret_when_upstream_rejects(t *testing.T) {
	// Given
	const secret = "paid-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`Bearer paid-secret`))
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, time.Second)
	auth, err := NewAPIKey(secret)
	if err != nil {
		t.Fatalf("NewAPIKey() error = %v", err)
	}

	// When
	response, err := client.ChatCompletions(context.Background(), auth, json.RawMessage(`{}`), false)

	// Then
	if response != nil {
		_ = response.Body.Close()
		t.Fatal("response must be nil on non-success status")
	}
	var statusError *StatusError
	if !errors.As(err, &statusError) {
		t.Fatalf("error = %v, want StatusError", err)
	}
	if statusError.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", statusError.StatusCode)
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error leaks API key: %q", err)
	}
}

func TestClient_ChatCompletions_returns_status_error_when_unauthorized(t *testing.T) {
	// Given
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, time.Second)

	// When
	response, err := client.ChatCompletions(context.Background(), PublicAuth(), json.RawMessage(`{}`), false)

	// Then
	if response != nil {
		_ = response.Body.Close()
		t.Fatal("response must be nil on non-success status")
	}
	var statusError *StatusError
	if !errors.As(err, &statusError) {
		t.Fatalf("error = %v, want StatusError", err)
	}
	if statusError.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", statusError.StatusCode)
	}
}

func TestClient_ChatCompletions_generates_unique_compatibility_IDs(t *testing.T) {
	// Given
	requestIDs := make(chan string, 2)
	sessionIDs := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requestIDs <- request.Header.Get("x-opencode-request")
		sessionIDs <- request.Header.Get("x-opencode-session")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, time.Second)

	// When
	for range 2 {
		response, err := client.ChatCompletions(context.Background(), PublicAuth(), json.RawMessage(`{}`), false)
		if err != nil {
			t.Fatalf("ChatCompletions() error = %v", err)
		}
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Fatalf("close response body: %v", closeErr)
		}
	}

	// Then
	if first, second := <-requestIDs, <-requestIDs; first == second {
		t.Errorf("request IDs must be unique, both were %q", first)
	}
	if first, second := <-sessionIDs, <-sessionIDs; first == second {
		t.Errorf("session IDs must be unique, both were %q", first)
	}
}

func TestNewAPIKey_rejects_blank_value(t *testing.T) {
	// Given
	// When
	_, err := NewAPIKey("  ")

	// Then
	if !errors.Is(err, ErrBlankAPIKey) {
		t.Errorf("error = %v, want ErrBlankAPIKey", err)
	}
}

func TestClient_plain_headers_skip_opencode_compat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer ollama-secret" {
			t.Errorf("Authorization = %q", got)
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		if got := request.Header.Get("x-opencode-client"); got != "" {
			t.Errorf("x-opencode-client = %q, want empty", got)
		}
		if got := request.Header.Get("x-opencode-project"); got != "" {
			t.Errorf("x-opencode-project = %q, want empty", got)
		}
		if got := request.Header.Get("x-opencode-request"); got != "" {
			t.Errorf("x-opencode-request = %q, want empty", got)
		}
		if got := request.Header.Get("x-opencode-session"); got != "" {
			t.Errorf("x-opencode-session = %q, want empty", got)
		}
		if strings.Contains(request.Header.Get("User-Agent"), "opencode/") {
			t.Errorf("User-Agent = %q, want no opencode UA", request.Header.Get("User-Agent"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewPlainClient(config.Config{UpstreamTimeout: time.Second, OCVersion: "9.9.9"}, server.URL)
	if err != nil {
		t.Fatalf("NewPlainClient() error = %v", err)
	}
	auth, err := NewAPIKey("ollama-secret")
	if err != nil {
		t.Fatalf("NewAPIKey() error = %v", err)
	}
	response, err := client.ChatCompletions(context.Background(), auth, json.RawMessage(`{}`), false)
	if err != nil {
		t.Fatalf("ChatCompletions() error = %v", err)
	}
	_ = response.Body.Close()
}

func TestClient_ModelsWithAuth_uses_supplied_authorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/models" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer pool-key" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"cloud-model"}]}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, time.Second)
	auth, err := NewAPIKey("pool-key")
	if err != nil {
		t.Fatalf("NewAPIKey() error = %v", err)
	}
	models, err := client.ModelsWithAuth(context.Background(), auth)
	if err != nil {
		t.Fatalf("ModelsWithAuth() error = %v", err)
	}
	if len(models) != 1 || models[0].ID != "cloud-model" {
		t.Fatalf("models = %#v", models)
	}
}

func newTestClient(t *testing.T, baseURL string, timeout time.Duration) *Client {
	t.Helper()
	client, err := NewClient(config.Config{
		ZenBase:         baseURL,
		OCVersion:       "9.9.9",
		UpstreamTimeout: timeout,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}
