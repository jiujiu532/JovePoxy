package adminapi

import (
	"encoding/json"
	"net/http"
	"time"

	"jovepoxy/internal/analytics"
	"jovepoxy/internal/auth"
	"jovepoxy/internal/config"
	"jovepoxy/internal/keys"
	"jovepoxy/internal/models"
	"jovepoxy/internal/ollama"
	"jovepoxy/internal/proxypool"
	"jovepoxy/internal/quota"
	"jovepoxy/internal/reqlog"
	"jovepoxy/internal/usage"
	"jovepoxy/internal/version"
	"jovepoxy/internal/zenpool"
)

// Dependencies wires control-plane services for /api/admin/*.
// Config is a read-only snapshot used by settings; DB is intentionally absent.
type Dependencies struct {
	Auth           *auth.Service
	Keys           *keys.Service
	Pool           *zenpool.Service
	Proxies        *proxypool.Service
	Accounts       *quota.AccountService
	Quotas         *quota.SnapshotService
	OllamaAccounts *ollama.AccountService
	OllamaScraper  *ollama.Scraper
	Usage          *usage.Service
	Analytics      *analytics.Service
	Logs           *reqlog.Service
	Catalog        *models.Catalog
	Versions       *version.Checker
	Config         config.Config
	CookieSecure   bool
}

type server struct {
	auth           *auth.Service
	keys           *keys.Service
	pool           *zenpool.Service
	proxies        *proxypool.Service
	accounts       *quota.AccountService
	quotas         *quota.SnapshotService
	ollamaAccounts *ollama.AccountService
	ollamaScraper  *ollama.Scraper
	usage          *usage.Service
	analytics      *analytics.Service
	logs           *reqlog.Service
	catalog        *models.Catalog
	versions       *version.Checker
	cfg            config.Config
	cookieSecure   bool
}

// New mounts authenticated admin routes.
func New(dependencies Dependencies) http.Handler {
	server := server{
		auth: dependencies.Auth, keys: dependencies.Keys, pool: dependencies.Pool, proxies: dependencies.Proxies,
		accounts: dependencies.Accounts, quotas: dependencies.Quotas,
		ollamaAccounts: dependencies.OllamaAccounts, ollamaScraper: dependencies.OllamaScraper,
		usage: dependencies.Usage, analytics: dependencies.Analytics, logs: dependencies.Logs,
		catalog: dependencies.Catalog, versions: dependencies.Versions,
		cfg: dependencies.Config, cookieSecure: dependencies.CookieSecure,
	}
	if server.versions == nil {
		server.versions = version.NewChecker()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/admin/login", server.login)
	mux.HandleFunc("POST /api/admin/logout", server.requireAuth(server.logout))
	mux.HandleFunc("GET /api/admin/me", server.requireAuth(server.me))
	mux.HandleFunc("POST /api/admin/password", server.requireAuth(server.changePassword))
	mux.HandleFunc("GET /api/admin/version", server.requireAuth(server.getVersion))
	mux.HandleFunc("GET /api/admin/overview", server.requireAuth(server.overview))
	mux.HandleFunc("GET /api/admin/models", server.requireAuth(server.listModels))
	mux.HandleFunc("POST /api/admin/models/refresh", server.requireAuth(server.refreshModels))
	mux.HandleFunc("GET /api/admin/local-keys", server.requireAuth(server.listLocalKeys))
	mux.HandleFunc("POST /api/admin/local-keys", server.requireAuth(server.createLocalKey))
	mux.HandleFunc("PATCH /api/admin/local-keys/{id}", server.requireAuth(server.updateLocalKey))
	mux.HandleFunc("POST /api/admin/local-keys/{id}/enable", server.requireAuth(server.enableLocalKey))
	mux.HandleFunc("POST /api/admin/local-keys/{id}/disable", server.requireAuth(server.disableLocalKey))
	mux.HandleFunc("POST /api/admin/local-keys/{id}/revoke", server.requireAuth(server.revokeLocalKey))
	mux.HandleFunc("GET /api/admin/zen-keys", server.requireAuth(server.listZenKeys))
	mux.HandleFunc("POST /api/admin/zen-keys", server.requireAuth(server.createZenKey))
	mux.HandleFunc("PATCH /api/admin/zen-keys/{id}", server.requireAuth(server.updateZenKey))
	mux.HandleFunc("POST /api/admin/zen-keys/{id}/enable", server.requireAuth(server.enableZenKey))
	mux.HandleFunc("POST /api/admin/zen-keys/{id}/disable", server.requireAuth(server.disableZenKey))
	mux.HandleFunc("DELETE /api/admin/zen-keys/{id}", server.requireAuth(server.deleteZenKey))
	mux.HandleFunc("GET /api/admin/proxies", server.requireAuth(server.listProxies))
	mux.HandleFunc("POST /api/admin/proxies", server.requireAuth(server.createProxy))
	mux.HandleFunc("PATCH /api/admin/proxies/{id}", server.requireAuth(server.updateProxy))
	mux.HandleFunc("POST /api/admin/proxies/{id}/enable", server.requireAuth(server.enableProxy))
	mux.HandleFunc("POST /api/admin/proxies/{id}/disable", server.requireAuth(server.disableProxy))
	mux.HandleFunc("DELETE /api/admin/proxies/{id}", server.requireAuth(server.deleteProxy))
	mux.HandleFunc("GET /api/admin/opencode-accounts", server.requireAuth(server.listAccounts))
	mux.HandleFunc("POST /api/admin/opencode-accounts", server.requireAuth(server.createAccount))
	mux.HandleFunc("POST /api/admin/opencode-accounts/{id}/enable", server.requireAuth(server.enableAccount))
	mux.HandleFunc("POST /api/admin/opencode-accounts/{id}/disable", server.requireAuth(server.disableAccount))
	mux.HandleFunc("GET /api/admin/opencode-accounts/{id}/credential", server.requireAuth(server.getAccountCredential))
	mux.HandleFunc("DELETE /api/admin/opencode-accounts/{id}", server.requireAuth(server.deleteAccount))
	mux.HandleFunc("GET /api/admin/quotas", server.requireAuth(server.listQuotas))
	mux.HandleFunc("GET /api/admin/ollama-accounts", server.requireAuth(server.listOllamaAccounts))
	mux.HandleFunc("POST /api/admin/ollama-accounts", server.requireAuth(server.createOllamaAccount))
	mux.HandleFunc("POST /api/admin/ollama-accounts/{id}/enable", server.requireAuth(server.enableOllamaAccount))
	mux.HandleFunc("POST /api/admin/ollama-accounts/{id}/disable", server.requireAuth(server.disableOllamaAccount))
	mux.HandleFunc("GET /api/admin/ollama-accounts/{id}/credential", server.requireAuth(server.getOllamaAccountCredential))
	mux.HandleFunc("DELETE /api/admin/ollama-accounts/{id}", server.requireAuth(server.deleteOllamaAccount))
	mux.HandleFunc("GET /api/admin/ollama-quotas", server.requireAuth(server.listOllamaQuotas))
	mux.HandleFunc("GET /api/admin/usage", server.requireAuth(server.listUsage))
	mux.HandleFunc("POST /api/admin/usage/sync", server.requireAuth(server.syncUsage))
	mux.HandleFunc("POST /api/admin/usage/backfill", server.requireAuth(server.backfillUsage))
	mux.HandleFunc("GET /api/admin/logs", server.requireAuth(server.listLogs))
	mux.HandleFunc("GET /api/admin/metrics", server.requireAuth(server.metrics))
	mux.HandleFunc("GET /api/admin/settings", server.requireAuth(server.getSettings))
	mux.HandleFunc("PATCH /api/admin/settings", server.requireAuth(server.patchSettings))
	return mux
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, ErrorBody{Error: message})
}

func (server server) cookieName() string { return "jovepoxy_admin" }

func (server server) setSessionCookie(writer http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(writer, &http.Cookie{
		Name: server.cookieName(), Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: server.cookieSecure, Expires: expires,
	})
}

func (server server) clearSessionCookie(writer http.ResponseWriter) {
	http.SetCookie(writer, &http.Cookie{
		Name: server.cookieName(), Value: "", Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: server.cookieSecure, MaxAge: -1,
	})
}
