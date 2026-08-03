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
	"jovepoxy/internal/crypto"
	"jovepoxy/internal/db"
	"jovepoxy/internal/httpserver"
	"jovepoxy/internal/keys"
	"jovepoxy/internal/models"
	"jovepoxy/internal/reqlog"
	"jovepoxy/internal/zen"
	"jovepoxy/internal/zenpool"
)

func TestServer_routes_ollama_model_to_ollama_pool_and_base(t *testing.T) {
	var zenHits, ollamaHits int
	var ollamaAuth string
	var ollamaHasOpenCodeHeader bool

	zenUpstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		zenHits++
	}))
	defer zenUpstream.Close()

	ollamaUpstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ollamaHits++
		ollamaAuth = request.Header.Get("Authorization")
		if request.Header.Get("x-opencode-client") != "" {
			ollamaHasOpenCodeHeader = true
		}
		if !strings.HasSuffix(request.URL.Path, "/chat/completions") {
			t.Errorf("ollama path = %q", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"chatcmpl_ollama","object":"chat.completion"}`))
	}))
	defer ollamaUpstream.Close()

	ctx := context.Background()
	database, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	keyService := keys.NewService(database, nil)
	created, err := keyService.Create(ctx, keys.CreateInput{Label: "local"})
	if err != nil {
		t.Fatalf("create local key: %v", err)
	}
	box, err := crypto.NewBox("test-admin-secret-32-bytes-minimum!!")
	if err != nil {
		t.Fatalf("new box: %v", err)
	}
	pool := zenpool.NewService(database, box, nil)
	if _, err := pool.Create(ctx, zenpool.CreateInput{
		Label: "ollama-key", Secret: "ollama-secret-value", Provider: zenpool.ProviderOllama,
	}); err != nil {
		t.Fatalf("create ollama pool key: %v", err)
	}
	// OpenCode key must not be used for ollama model.
	if _, err := pool.Create(ctx, zenpool.CreateInput{
		Label: "zen-key", Secret: "zen-secret-value", Provider: zenpool.ProviderOpenCode,
	}); err != nil {
		t.Fatalf("create opencode pool key: %v", err)
	}

	zenClient, err := zen.NewClient(config.Config{ZenBase: zenUpstream.URL, OCVersion: "test", UpstreamTimeout: time.Second})
	if err != nil {
		t.Fatalf("zen client: %v", err)
	}
	ollamaClient, err := zen.NewPlainClient(config.Config{UpstreamTimeout: time.Second, OCVersion: "test"}, ollamaUpstream.URL)
	if err != nil {
		t.Fatalf("ollama client: %v", err)
	}
	catalog, err := models.NewCatalog(
		testModelSource{models: []zen.Model{{ID: "demo-free"}}},
		models.Settings{
			TTL:    time.Hour,
			Ollama: testModelSource{models: []zen.Model{{ID: "cloud-llama"}}},
		},
	)
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if _, err := catalog.Refresh(ctx); err != nil {
		t.Fatalf("refresh catalog: %v", err)
	}

	handler := httpserver.New(httpserver.Dependencies{
		Keys: keyService, Catalog: catalog, Zen: zenClient, Ollama: ollamaClient,
		Pool: pool, Logs: reqlog.NewService(database, nil), Version: "test",
		ShowAllModels: true,
	})

	// owned_by for dual catalog
	modelsRecorder := httptest.NewRecorder()
	handler.ServeHTTP(modelsRecorder, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if modelsRecorder.Code != http.StatusOK {
		t.Fatalf("list models status = %d", modelsRecorder.Code)
	}
	var listed struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal(modelsRecorder.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode models: %v", err)
	}
	owned := map[string]string{}
	for _, item := range listed.Data {
		owned[item.ID] = item.OwnedBy
	}
	if owned["demo-free"] != "zen" {
		t.Fatalf("demo-free owned_by = %q, want zen", owned["demo-free"])
	}
	if owned["cloud-llama"] != "ollama" {
		t.Fatalf("cloud-llama owned_by = %q, want ollama", owned["cloud-llama"])
	}

	// chat ollama model
	body := []byte(`{"model":"cloud-llama","messages":[{"role":"user","content":"hi"}]}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+created.Secret)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("chat status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if zenHits != 0 {
		t.Fatalf("zen upstream hits = %d, want 0", zenHits)
	}
	if ollamaHits != 1 {
		t.Fatalf("ollama upstream hits = %d, want 1", ollamaHits)
	}
	if ollamaAuth != "Bearer ollama-secret-value" {
		t.Fatalf("ollama Authorization = %q", ollamaAuth)
	}
	if ollamaHasOpenCodeHeader {
		t.Fatal("ollama request carried OpenCode compatibility headers")
	}
}

func TestServer_ollama_model_without_keys_returns_clear_error(t *testing.T) {
	zenUpstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer zenUpstream.Close()
	ollamaUpstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("ollama upstream should not be dialed without pool keys")
	}))
	defer ollamaUpstream.Close()

	ctx := context.Background()
	database, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	keyService := keys.NewService(database, nil)
	created, err := keyService.Create(ctx, keys.CreateInput{Label: "local"})
	if err != nil {
		t.Fatalf("create local key: %v", err)
	}
	box, err := crypto.NewBox("test-admin-secret-32-bytes-minimum!!")
	if err != nil {
		t.Fatalf("new box: %v", err)
	}
	pool := zenpool.NewService(database, box, nil)

	zenClient, err := zen.NewClient(config.Config{ZenBase: zenUpstream.URL, OCVersion: "test", UpstreamTimeout: time.Second})
	if err != nil {
		t.Fatalf("zen client: %v", err)
	}
	ollamaClient, err := zen.NewPlainClient(config.Config{UpstreamTimeout: time.Second, OCVersion: "test"}, ollamaUpstream.URL)
	if err != nil {
		t.Fatalf("ollama client: %v", err)
	}
	catalog, err := models.NewCatalog(
		testModelSource{models: []zen.Model{{ID: "demo-free"}}},
		models.Settings{TTL: time.Hour, Ollama: testModelSource{models: []zen.Model{{ID: "cloud-llama"}}}},
	)
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if _, err := catalog.Refresh(ctx); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	handler := httpserver.New(httpserver.Dependencies{
		Keys: keyService, Catalog: catalog, Zen: zenClient, Ollama: ollamaClient,
		Pool: pool, Logs: reqlog.NewService(database, nil), Version: "test",
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"cloud-llama","messages":[]}`))
	request.Header.Set("Authorization", "Bearer "+created.Secret)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "no ollama API keys configured") {
		t.Fatalf("body = %s, want ollama key message", recorder.Body.String())
	}
}

func TestServer_missing_ollama_dialer_does_not_hit_zen(t *testing.T) {
	zenHits := 0
	zenUpstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		zenHits++
	}))
	defer zenUpstream.Close()

	ctx := context.Background()
	database, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	keyService := keys.NewService(database, nil)
	created, err := keyService.Create(ctx, keys.CreateInput{Label: "local"})
	if err != nil {
		t.Fatalf("create local key: %v", err)
	}
	box, err := crypto.NewBox("test-admin-secret-32-bytes-minimum!!")
	if err != nil {
		t.Fatalf("new box: %v", err)
	}
	pool := zenpool.NewService(database, box, nil)
	if _, err := pool.Create(ctx, zenpool.CreateInput{
		Label: "ollama-key", Secret: "ollama-secret-value", Provider: zenpool.ProviderOllama,
	}); err != nil {
		t.Fatalf("create ollama key: %v", err)
	}

	zenClient, err := zen.NewClient(config.Config{ZenBase: zenUpstream.URL, OCVersion: "test", UpstreamTimeout: time.Second})
	if err != nil {
		t.Fatalf("zen client: %v", err)
	}
	catalog, err := models.NewCatalog(
		testModelSource{models: []zen.Model{{ID: "demo-free"}}},
		models.Settings{TTL: time.Hour, Ollama: testModelSource{models: []zen.Model{{ID: "cloud-llama"}}}},
	)
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if _, err := catalog.Refresh(ctx); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	// Ollama dialer intentionally omitted — must not fall back to Zen.
	handler := httpserver.New(httpserver.Dependencies{
		Keys: keyService, Catalog: catalog, Zen: zenClient, Ollama: nil,
		Pool: pool, Logs: reqlog.NewService(database, nil), Version: "test",
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"cloud-llama","messages":[]}`))
	request.Header.Set("Authorization", "Bearer "+created.Secret)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if zenHits != 0 {
		t.Fatalf("zen hits = %d, want 0 when ollama dialer missing", zenHits)
	}
	if recorder.Code == http.StatusOK {
		t.Fatalf("expected non-OK when ollama dialer missing, body=%s", recorder.Body.String())
	}
}

func TestServer_routes_opencode_paid_model_to_zen_go_not_public(t *testing.T) {
	var publicHits, goHits int
	var goAuth string

	publicUpstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		publicHits++
	}))
	defer publicUpstream.Close()

	goUpstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		goHits++
		goAuth = request.Header.Get("Authorization")
		if !strings.HasSuffix(request.URL.Path, "/chat/completions") {
			t.Errorf("go path = %q", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"chatcmpl_go","object":"chat.completion"}`))
	}))
	defer goUpstream.Close()

	ctx := context.Background()
	database, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	keyService := keys.NewService(database, nil)
	created, err := keyService.Create(ctx, keys.CreateInput{Label: "local"})
	if err != nil {
		t.Fatalf("create local key: %v", err)
	}
	box, err := crypto.NewBox("test-admin-secret-32-bytes-minimum!!")
	if err != nil {
		t.Fatalf("new box: %v", err)
	}
	pool := zenpool.NewService(database, box, nil)
	if _, err := pool.Create(ctx, zenpool.CreateInput{
		Label: "go-key", Secret: "go-secret-value", Provider: zenpool.ProviderOpenCode,
	}); err != nil {
		t.Fatalf("create go pool key: %v", err)
	}

	publicClient, err := zen.NewClient(config.Config{ZenBase: publicUpstream.URL, OCVersion: "test", UpstreamTimeout: time.Second})
	if err != nil {
		t.Fatalf("public client: %v", err)
	}
	goClient, err := zen.NewClient(config.Config{ZenBase: goUpstream.URL, OCVersion: "test", UpstreamTimeout: time.Second})
	if err != nil {
		t.Fatalf("go client: %v", err)
	}
	catalog, err := models.NewCatalog(
		testModelSource{models: []zen.Model{{ID: "demo-free"}}},
		models.Settings{
			TTL:          time.Hour,
			OpenCodePaid: testModelSource{models: []zen.Model{{ID: "deepseek-v4-flash"}}},
		},
	)
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if _, err := catalog.Refresh(ctx); err != nil {
		t.Fatalf("refresh catalog: %v", err)
	}

	handler := httpserver.New(httpserver.Dependencies{
		Keys: keyService, Catalog: catalog, Zen: publicClient, ZenGo: goClient,
		Pool: pool, Logs: reqlog.NewService(database, nil), Version: "test",
		ShowAllModels: true,
	})

	modelsRecorder := httptest.NewRecorder()
	handler.ServeHTTP(modelsRecorder, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	var listed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(modelsRecorder.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode models: %v", err)
	}
	ids := map[string]bool{}
	for _, item := range listed.Data {
		ids[item.ID] = true
	}
	if !ids["demo-free"] || !ids["deepseek-v4-flash"] {
		t.Fatalf("listed ids = %#v, want free + go paid", ids)
	}

	body := []byte(`{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"hi"}]}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+created.Secret)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("chat status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if publicHits != 0 {
		t.Fatalf("public zen hits = %d, want 0", publicHits)
	}
	if goHits != 1 {
		t.Fatalf("go upstream hits = %d, want 1", goHits)
	}
	if goAuth != "Bearer go-secret-value" {
		t.Fatalf("go Authorization = %q", goAuth)
	}
}
