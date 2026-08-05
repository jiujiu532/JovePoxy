package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"jovepoxy/internal/adminapi"
	"jovepoxy/internal/analytics"
	"jovepoxy/internal/auth"
	"jovepoxy/internal/config"
	"jovepoxy/internal/crypto"
	"jovepoxy/internal/db"
	"jovepoxy/internal/httpserver"
	"jovepoxy/internal/keys"
	"jovepoxy/internal/models"
	"jovepoxy/internal/ollama"
	"jovepoxy/internal/proxypool"
	"jovepoxy/internal/quota"
	"jovepoxy/internal/reqlog"
	"jovepoxy/internal/usage"
	"jovepoxy/internal/version"
	"jovepoxy/internal/webui"
	"jovepoxy/internal/zen"
	"jovepoxy/internal/zenpool"
)

// Version is reported by --version and /health (prefix + product version).
var Version = "jovepoxy " + version.Current

// Runtime owns process resources that must be closed after the HTTP handler is no longer used.
type Runtime struct {
	Handler http.Handler
	close   func() error
}

// Close releases database resources.
func (runtime *Runtime) Close() error {
	if runtime == nil || runtime.close == nil {
		return nil
	}
	return runtime.close()
}

// Bootstrap loads configuration-backed services and builds the public handler without listening.
func Bootstrap(ctx context.Context, cfg config.Config) (*Runtime, error) {
	database, err := db.Open(ctx, cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// Free path: public Zen + Bearer public.
	zenClient, err := zen.NewClient(cfg)
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("create zen client: %w", err)
	}
	// Paid OpenCode Go suite: /zen/go/v1 with pool sk keys (not public Zen paid list).
	zenGoClient, err := zen.NewClient(config.Config{
		ZenBase:         openCodeGoAPIBase(cfg.ZenGoBase),
		OCVersion:       cfg.OCVersion,
		UpstreamTimeout: cfg.UpstreamTimeout,
		HTTPProxy:       cfg.HTTPProxy,
		HTTPSProxy:      cfg.HTTPSProxy,
	})
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("create zen go client: %w", err)
	}
	ollamaClient, err := zen.NewPlainClient(cfg, ollamaAPIBase(cfg.OllamaBase))
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("create ollama client: %w", err)
	}
	box, err := crypto.NewBox(cfg.AdminSecret)
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("create secret box: %w", err)
	}
	pool := zenpool.NewService(database, box, nil)
	// Optional process env for paid-pool scheduling (runtime still mutable via PATCH /settings).
	if policy := strings.TrimSpace(os.Getenv("ZEN_LOAD_POLICY")); policy != "" {
		pool.SetLoadPolicy(zenpool.LoadPolicy(policy))
	}
	if raw := strings.TrimSpace(os.Getenv("ZEN_MAX_ATTEMPTS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			pool.SetMaxAttempts(n)
		}
	}
	if raw := strings.TrimSpace(os.Getenv("ZEN_BENCH_MINUTES")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			pool.SetBenchMinutes(n)
		}
	}
	catalog, err := models.NewCatalog(zenClient, models.Settings{
		TTL:          cfg.ModelCacheTTL,
		OpenCodePaid: goModelsSource{pool: pool, client: zenGoClient},
		Ollama:       ollamaModelsSource{pool: pool, client: ollamaClient},
	})
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("create model catalog: %w", err)
	}
	keyService := keys.NewService(database, nil)
	proxies := proxypool.NewService(database, box, nil)
	logs := reqlog.NewService(database, nil)
	authService, err := auth.NewService(auth.Config{Database: database, Password: cfg.AdminPassword})
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("create auth service: %w", err)
	}
	accounts, err := quota.NewAccountService(database, box)
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("create account service: %w", err)
	}
	scraper, err := quota.NewScraper(quota.ScraperConfig{
		HTTPClient: &http.Client{Timeout: 15 * time.Second}, Timeout: 15 * time.Second,
	})
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("create quota scraper: %w", err)
	}
	ollamaAccounts, err := ollama.NewAccountService(database, box)
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("create ollama account service: %w", err)
	}
	ollamaScraper, err := ollama.NewScraper(ollama.ScraperConfig{
		HTTPClient: &http.Client{Timeout: 20 * time.Second}, Timeout: 20 * time.Second,
	})
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("create ollama scraper: %w", err)
	}
	usageFetcher, err := usage.NewHTTPFetcher(usage.FetcherConfig{
		HTTPClient: &http.Client{Timeout: 20 * time.Second}, Timeout: 20 * time.Second,
	})
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("create usage fetcher: %w", err)
	}
	usageStore := usage.NewSQLiteStore(database)
	usageService := usage.NewService(usageStore, usageFetcher)
	dataPlane := httpserver.New(httpserver.Dependencies{
		Keys: keyService, Catalog: catalog, Zen: zenClient, ZenGo: zenGoClient, Ollama: ollamaClient,
		Pool: pool, Proxies: proxies, Logs: logs, Version: Version, ShowAllModels: cfg.ShowAllModels,
	})
	quotaSnapshots := quota.NewSnapshotService(accounts, scraper, 30*time.Second)
	admin := adminapi.New(adminapi.Dependencies{
		Auth: authService, Keys: keyService, Pool: pool, Proxies: proxies,
		Accounts: accounts, Quotas: quotaSnapshots,
		OllamaAccounts: ollamaAccounts, OllamaScraper: ollamaScraper,
		Usage: usageService, Analytics: analytics.NewService(usageStore), Logs: logs, Catalog: catalog,
		Versions: version.NewChecker(), Config: cfg, CookieSecure: cfg.CookieSecure,
	})
	spa, err := webui.Handler()
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("load embedded web UI: %w", err)
	}
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path := request.URL.Path
		switch {
		case strings.HasPrefix(path, "/api/admin"):
			admin.ServeHTTP(writer, request)
		case strings.HasPrefix(path, "/v1"), path == "/health", path == "/metrics":
			dataPlane.ServeHTTP(writer, request)
		default:
			spa.ServeHTTP(writer, request)
		}
	})
	return &Runtime{Handler: handler, close: database.Close}, nil
}

// Run loads configuration, serves the public API, and shuts down on context cancel.
func Run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	runtime, err := Bootstrap(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = runtime.Close() }()

	server := &http.Server{
		Addr:              cfg.Listen,
		Handler:           runtime.Handler,
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}
		err := <-errCh
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// openCodeGoAPIBase ensures OpenAI-compatible paths under /v1 when ZEN_GO_BASE is the Go host root.
func openCodeGoAPIBase(base string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(base), "/")
	if trimmed == "" {
		return "https://opencode.ai/zen/go/v1"
	}
	if strings.HasSuffix(trimmed, "/v1") {
		return trimmed
	}
	return trimmed + "/v1"
}

// ollamaAPIBase ensures OpenAI-compatible paths under /v1 when OLLAMA_BASE is the host root.
func ollamaAPIBase(base string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(base), "/")
	if trimmed == "" {
		return "https://ollama.com/v1"
	}
	if strings.HasSuffix(trimmed, "/v1") {
		return trimmed
	}
	return trimmed + "/v1"
}

// goModelsSource pulls OpenCode Go suite models with a healthy OpenCode pool key.
// Empty pool is not an error (returns no paid Go models).
type goModelsSource struct {
	pool   *zenpool.Service
	client *zen.Client
}

func (source goModelsSource) Models(ctx context.Context) ([]zen.Model, error) {
	if source.pool == nil || source.client == nil {
		return nil, nil
	}
	selected, err := source.pool.AcquireFor(ctx, zenpool.AcquireOptions{Provider: zenpool.ProviderOpenCode})
	if err != nil {
		if errors.Is(err, zenpool.ErrNoHealthyKey) {
			return nil, nil
		}
		return nil, err
	}
	auth, err := zen.NewAPIKey(selected.Secret)
	if err != nil {
		return nil, err
	}
	return source.client.ModelsWithAuth(ctx, auth)
}

// ollamaModelsSource pulls Cloud models with a healthy Ollama pool key; empty pool is not an error.
type ollamaModelsSource struct {
	pool   *zenpool.Service
	client *zen.Client
}

func (source ollamaModelsSource) Models(ctx context.Context) ([]zen.Model, error) {
	if source.pool == nil || source.client == nil {
		return nil, nil
	}
	selected, err := source.pool.AcquireFor(ctx, zenpool.AcquireOptions{Provider: zenpool.ProviderOllama})
	if err != nil {
		if errors.Is(err, zenpool.ErrNoHealthyKey) {
			return nil, nil
		}
		return nil, err
	}
	auth, err := zen.NewAPIKey(selected.Secret)
	if err != nil {
		return nil, err
	}
	return source.client.ModelsWithAuth(ctx, auth)
}
