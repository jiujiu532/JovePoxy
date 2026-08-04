package models

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"jovepoxy/internal/config"
	"jovepoxy/internal/zen"
)

func TestCatalog_Refresh_classifies_openai_list_response(t *testing.T) {
	// Given: public Zen list keeps free only; public paid IDs are dropped.
	server := modelServer(t, http.StatusOK, `{"object":"list","data":[{"id":"deepseek-v4-flash-free"},{"id":"paid-model"}]}`)
	catalog := newCatalog(t, server.URL, Settings{TTL: time.Minute})

	// When
	result, err := catalog.Refresh(context.Background())

	// Then
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if result.Stale {
		t.Fatal("Refresh() returned stale result after a successful fetch")
	}
	if got := result.Models; len(got) != 1 || !got[0].Free || got[0].ID != "deepseek-v4-flash-free" {
		t.Fatalf("models = %#v, want only free models from public catalog", got)
	}
}

func TestCatalog_Refresh_marks_big_pickle_free(t *testing.T) {
	// Given
	server := modelServer(t, http.StatusOK, `{"object":"list","data":[{"id":"big-pickle"}]}`)
	catalog := newCatalog(t, server.URL, Settings{TTL: time.Minute})

	// When
	result, err := catalog.Refresh(context.Background())

	// Then
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if len(result.Models) != 1 || !result.Models[0].Free {
		t.Fatalf("models = %#v, want big-pickle to be free", result.Models)
	}
}

func TestCatalog_Refresh_applies_allow_and_deny_overrides_with_deny_winning(t *testing.T) {
	// Given
	server := modelServer(t, http.StatusOK, `{"object":"list","data":[{"id":"extra-free"},{"id":"suffix-free"},{"id":"big-pickle"}]}`)
	catalog := newCatalog(t, server.URL, Settings{
		TTL:           time.Minute,
		FreeAllowlist: []ModelID{"extra-free", "suffix-free"},
		FreeDenylist:  []ModelID{"suffix-free", "big-pickle"},
	})

	// When
	result, err := catalog.Refresh(context.Background())

	// Then
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	// Only free models remain; denylisted free models are dropped entirely.
	if got := result.Models; len(got) != 1 || got[0].ID != "extra-free" || !got[0].Free {
		t.Fatalf("models = %#v, want only allowlisted free model", got)
	}
}

func TestCatalog_List_uses_fresh_cache_without_second_request(t *testing.T) {
	// Given
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"object":"list","data":[{"id":"cached-free"}]}`))
	}))
	defer server.Close()
	catalog := newCatalog(t, server.URL, Settings{TTL: time.Minute})
	if _, err := catalog.Refresh(context.Background()); err != nil {
		t.Fatalf("initial Refresh() error = %v", err)
	}

	// When
	result, err := catalog.List(context.Background())

	// Then
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if result.Stale || calls.Load() != 1 {
		t.Fatalf("stale = %t, calls = %d, want false and 1", result.Stale, calls.Load())
	}
}

func TestCatalog_Refresh_returns_stale_snapshot_when_upstream_fails(t *testing.T) {
	// Given
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if fail.Load() {
			writer.WriteHeader(http.StatusBadGateway)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"object":"list","data":[{"id":"cached-free"}]}`))
	}))
	defer server.Close()
	catalog := newCatalog(t, server.URL, Settings{TTL: time.Minute})
	if _, err := catalog.Refresh(context.Background()); err != nil {
		t.Fatalf("initial Refresh() error = %v", err)
	}
	fail.Store(true)

	// When
	result, err := catalog.Refresh(context.Background())

	// Then
	if err != nil {
		t.Fatalf("Refresh() error = %v, want stale snapshot", err)
	}
	if !result.Stale || len(result.Models) != 1 || result.Models[0].ID != "cached-free" {
		t.Fatalf("result = %#v, want stale cached model", result)
	}
}

func TestCatalog_Refresh_returns_typed_error_when_no_snapshot_exists(t *testing.T) {
	// Given
	server := modelServer(t, http.StatusServiceUnavailable, "ignored")
	catalog := newCatalog(t, server.URL, Settings{TTL: time.Minute})

	// When
	_, err := catalog.Refresh(context.Background())

	// Then
	var refreshError *RefreshError
	if !errors.As(err, &refreshError) {
		t.Fatalf("error = %v, want RefreshError", err)
	}
	var statusError *zen.StatusError
	if !errors.As(err, &statusError) || statusError.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("error = %v, want Zen HTTP 503 status error", err)
	}
}

func TestCatalog_Refresh_returns_typed_error_for_malformed_body(t *testing.T) {
	// Given
	server := modelServer(t, http.StatusOK, `{"object":"list","data":[{"id":123}]}`)
	catalog := newCatalog(t, server.URL, Settings{TTL: time.Minute})

	// When
	_, err := catalog.Refresh(context.Background())

	// Then
	var refreshError *RefreshError
	if !errors.As(err, &refreshError) {
		t.Fatalf("error = %v, want RefreshError", err)
	}
	var schemaError *zen.ModelsSchemaError
	if !errors.As(err, &schemaError) {
		t.Fatalf("error = %v, want ModelsSchemaError", err)
	}
}

func modelServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/models" {
			t.Errorf("request = %s %s, want GET /models", request.Method, request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestCatalog_Refresh_merges_ollama_and_skips_id_conflicts(t *testing.T) {
	// Public source only keeps free IDs; shared-model is paid public → dropped before merge.
	zenSource := staticSource{models: []zen.Model{{ID: "shared-model"}, {ID: "zen-only-free"}}}
	ollamaSource := staticSource{models: []zen.Model{{ID: "shared-model"}, {ID: "ollama-only"}}}
	catalog, err := NewCatalog(zenSource, Settings{TTL: time.Minute, Ollama: ollamaSource})
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}

	result, err := catalog.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if result.Stale {
		t.Fatal("Refresh() should not be stale when both sources succeed")
	}
	if len(result.Models) != 3 {
		t.Fatalf("models = %#v, want 3 (free + shared ollama + ollama-only)", result.Models)
	}
	byID := map[ModelID]Model{}
	for _, model := range result.Models {
		byID[model.ID] = model
	}
	if got := byID["shared-model"]; got.Provider != ProviderOllama || got.Free {
		t.Fatalf("shared-model = %#v, want ollama paid (public paid dropped, ollama fills ID)", got)
	}
	if got := byID["ollama-only"]; got.Provider != ProviderOllama || got.Free {
		t.Fatalf("ollama-only = %#v, want ollama paid", got)
	}
	if got := byID["zen-only-free"]; got.Provider != ProviderOpenCode || !got.Free {
		t.Fatalf("zen-only-free = %#v", got)
	}
}

func TestCatalog_Refresh_annotates_ollama_on_go_overlap(t *testing.T) {
	// OpenCode Go wins primary route; Ollama same ID is annotated, not dropped from Ollama filter.
	zenSource := staticSource{models: []zen.Model{{ID: "zen-only-free"}}}
	goSource := staticSource{models: []zen.Model{{ID: "deepseek-v4-flash"}, {ID: "go-only"}}}
	ollamaSource := staticSource{models: []zen.Model{{ID: "deepseek-v4-flash"}, {ID: "ollama-only"}}}
	catalog, err := NewCatalog(zenSource, Settings{
		TTL: time.Minute, OpenCodePaid: goSource, Ollama: ollamaSource,
	})
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}
	result, err := catalog.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	byID := map[ModelID]Model{}
	for _, model := range result.Models {
		byID[model.ID] = model
	}
	overlap := byID["deepseek-v4-flash"]
	if overlap.Provider != ProviderOpenCode || overlap.Free {
		t.Fatalf("overlap primary = %#v, want paid opencode", overlap)
	}
	if !HasProvider(overlap, ProviderOpenCode) || !HasProvider(overlap, ProviderOllama) {
		t.Fatalf("overlap providers = %#v, want both opencode and ollama", ProvidersOf(overlap))
	}
	if got := byID["go-only"]; got.Provider != ProviderOpenCode || HasProvider(got, ProviderOllama) {
		t.Fatalf("go-only = %#v", got)
	}
	if got := byID["ollama-only"]; got.Provider != ProviderOllama || !HasProvider(got, ProviderOllama) {
		t.Fatalf("ollama-only = %#v", got)
	}
	// Still one row per ID.
	if len(result.Models) != 4 {
		t.Fatalf("models = %#v, want free + overlap + go-only + ollama-only", result.Models)
	}
}

func TestCatalog_Refresh_merges_go_paid_catalog(t *testing.T) {
	zenSource := staticSource{models: []zen.Model{{ID: "demo-free"}, {ID: "claude-haiku-4-5"}}}
	goSource := staticSource{models: []zen.Model{{ID: "deepseek-v4-flash"}, {ID: "demo-free"}}}
	catalog, err := NewCatalog(zenSource, Settings{TTL: time.Minute, OpenCodePaid: goSource})
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}
	result, err := catalog.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	byID := map[ModelID]Model{}
	for _, model := range result.Models {
		byID[model.ID] = model
	}
	if _, ok := byID["claude-haiku-4-5"]; ok {
		t.Fatalf("public paid model leaked into catalog: %#v", result.Models)
	}
	if got := byID["demo-free"]; !got.Free || got.Provider != ProviderOpenCode {
		t.Fatalf("demo-free = %#v, want free opencode", got)
	}
	if got := byID["deepseek-v4-flash"]; got.Free || got.Provider != ProviderOpenCode {
		t.Fatalf("deepseek-v4-flash = %#v, want paid opencode from Go catalog", got)
	}
	if len(result.Models) != 2 {
		t.Fatalf("models = %#v, want free + go paid only", result.Models)
	}
}

func TestCatalog_Refresh_marks_stale_when_go_fails_but_zen_ok(t *testing.T) {
	zenSource := staticSource{models: []zen.Model{{ID: "zen-only-free"}}}
	catalog, err := NewCatalog(zenSource, Settings{
		TTL:          time.Minute,
		OpenCodePaid: failingSource{err: errors.New("go down")},
	})
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}
	result, err := catalog.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if !result.Stale {
		t.Fatal("want stale when go paid source fails")
	}
	if len(result.Models) != 1 || result.Models[0].ID != "zen-only-free" {
		t.Fatalf("models = %#v, want free snapshot", result.Models)
	}
}

func TestCatalog_Refresh_marks_stale_when_ollama_fails_but_zen_ok(t *testing.T) {
	zenSource := staticSource{models: []zen.Model{{ID: "zen-only-free"}}}
	catalog, err := NewCatalog(zenSource, Settings{TTL: time.Minute, Ollama: failingSource{err: errors.New("ollama down")}})
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}

	result, err := catalog.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if !result.Stale {
		t.Fatal("want stale when ollama fails")
	}
	if len(result.Models) != 1 || result.Models[0].ID != "zen-only-free" || result.Models[0].Provider != ProviderOpenCode {
		t.Fatalf("models = %#v, want free snapshot", result.Models)
	}
}

func TestCatalog_Refresh_empty_ollama_is_not_error(t *testing.T) {
	zenSource := staticSource{models: []zen.Model{{ID: "zen-only-free"}}}
	catalog, err := NewCatalog(zenSource, Settings{TTL: time.Minute, Ollama: staticSource{}})
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}

	result, err := catalog.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if result.Stale || len(result.Models) != 1 {
		t.Fatalf("result = %#v, want non-stale free only", result)
	}
}

func TestNormalizeProvider(t *testing.T) {
	cases := []struct {
		in   Provider
		want Provider
	}{
		{in: "", want: ProviderOpenCode},
		{in: ProviderOpenCode, want: ProviderOpenCode},
		{in: ProviderOllama, want: ProviderOllama},
		{in: Provider("weird"), want: ProviderOpenCode},
	}
	for _, tc := range cases {
		if got := NormalizeProvider(tc.in); got != tc.want {
			t.Fatalf("NormalizeProvider(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCatalog_Refresh_skips_blank_ollama_ids(t *testing.T) {
	zenSource := staticSource{models: []zen.Model{{ID: "zen-only-free"}}}
	ollamaSource := staticSource{models: []zen.Model{{ID: "  "}, {ID: "ollama-ok"}}}
	catalog, err := NewCatalog(zenSource, Settings{TTL: time.Minute, Ollama: ollamaSource})
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}
	result, err := catalog.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if len(result.Models) != 2 {
		t.Fatalf("models = %#v, want free + one ollama", result.Models)
	}
}

func newCatalog(t *testing.T, baseURL string, settings Settings) *Catalog {
	t.Helper()
	client, err := zen.NewClient(config.Config{ZenBase: baseURL, OCVersion: "test", UpstreamTimeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	catalog, err := NewCatalog(client, settings)
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}
	return catalog
}

type staticSource struct {
	models []zen.Model
}

func (source staticSource) Models(context.Context) ([]zen.Model, error) {
	return append([]zen.Model(nil), source.models...), nil
}

type failingSource struct {
	err error
}

func (source failingSource) Models(context.Context) ([]zen.Model, error) {
	return nil, source.err
}
