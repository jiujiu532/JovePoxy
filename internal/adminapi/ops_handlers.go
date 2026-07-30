package adminapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"jovepoxy/internal/analytics"
	"jovepoxy/internal/auth"
	"jovepoxy/internal/quota"
	"jovepoxy/internal/reqlog"
	"jovepoxy/internal/zenpool"
)

func (server server) overview(writer http.ResponseWriter, request *http.Request) {
	window := analytics.NormalizeWindow(request.URL.Query().Get("window"))
	if server.analytics == nil {
		resp := overviewResponse{ByModel: nil}
		resp.ZenPool = server.zenPoolSummary(request)
		resp.OpsKPIs = server.buildOpsKPIs(request, window)
		writeJSON(writer, http.StatusOK, resp)
		return
	}
	var windows []quota.Window
	if server.quotas != nil {
		windows = server.quotas.Windows(request.Context())
	}
	overview, err := server.analytics.Overview(request.Context(), windows)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "overview failed")
		return
	}
	resp := mapOverview(overview)
	resp.ZenPool = server.zenPoolSummary(request)
	resp.OpsKPIs = server.buildOpsKPIs(request, window)
	writeJSON(writer, http.StatusOK, resp)
}

// buildOpsKPIs aggregates reqlog metadata for the requested window.
// Prefer persisted List; fall back to in-memory Recent. Never panics on empty logs.
func (server server) buildOpsKPIs(request *http.Request, window string) *analytics.OpsKPIs {
	now := time.Now().UTC()
	const limit = 5000
	var entries []reqlog.Entry
	if server.logs != nil {
		listed, err := server.logs.List(request.Context(), limit, 0)
		if err != nil {
			entries = server.logs.Recent(limit)
		} else {
			entries = listed
		}
	}
	kpis := analytics.AggregateOpsKPIs(entries, window, now)
	return &kpis
}

func (server server) zenPoolSummary(request *http.Request) *zenPoolSummaryDTO {
	if server.pool == nil {
		empty := mapZenPoolSummary(zenpool.PoolSummary{})
		return &empty
	}
	list, err := server.pool.List(request.Context())
	if err != nil {
		// Surface empty counts rather than failing the whole overview.
		empty := mapZenPoolSummary(zenpool.PoolSummary{})
		return &empty
	}
	now := time.Now().UTC()
	sum := mapZenPoolSummary(zenpool.Summarize(list, now, server.pool.BenchedSnapshot(now)))
	return &sum
}

func (server server) listModels(writer http.ResponseWriter, request *http.Request) {
	if server.catalog == nil {
		writeError(writer, http.StatusBadGateway, "catalog unavailable")
		return
	}
	result, err := server.catalog.List(request.Context())
	if err != nil {
		writeError(writer, http.StatusBadGateway, "catalog unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, mapModels(result))
}

func (server server) refreshModels(writer http.ResponseWriter, request *http.Request) {
	if server.catalog == nil {
		writeError(writer, http.StatusBadGateway, "catalog unavailable")
		return
	}
	result, err := server.catalog.Refresh(request.Context())
	if err != nil {
		writeError(writer, http.StatusBadGateway, "refresh failed")
		return
	}
	writeJSON(writer, http.StatusOK, mapModels(result))
}

func (server server) listUsage(writer http.ResponseWriter, request *http.Request) {
	if server.usage == nil {
		writeJSON(writer, http.StatusOK, usageResponse{Records: []usageRecordDTO{}})
		return
	}
	limit, _ := strconv.Atoi(request.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(request.URL.Query().Get("offset"))
	accountID := strings.TrimSpace(request.URL.Query().Get("account_id"))
	records, err := server.usage.List(request.Context(), accountID, limit, offset)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "list usage failed")
		return
	}
	writeJSON(writer, http.StatusOK, mapUsage(records))
}

type usageSyncRequest struct {
	AccountID string `json:"account_id"`
	MaxPages  int    `json:"max_pages"`
}

func (server server) syncUsage(writer http.ResponseWriter, request *http.Request) {
	server.runUsageSync(writer, request, false)
}

func (server server) backfillUsage(writer http.ResponseWriter, request *http.Request) {
	server.runUsageSync(writer, request, true)
}

func (server server) runUsageSync(writer http.ResponseWriter, request *http.Request, backfill bool) {
	if server.usage == nil || server.accounts == nil {
		writeError(writer, http.StatusServiceUnavailable, "usage sync unavailable")
		return
	}
	var body usageSyncRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil || strings.TrimSpace(body.AccountID) == "" {
		writeError(writer, http.StatusBadRequest, "account_id is required")
		return
	}
	credential, err := server.accounts.GetCredential(request.Context(), quota.AccountID(body.AccountID))
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	if backfill {
		result, syncErr := server.usage.Backfill(request.Context(), body.AccountID, credential.WorkspaceID, credential.AuthCookie, body.MaxPages)
		if syncErr != nil {
			writeError(writer, http.StatusBadGateway, syncErr.Error())
			return
		}
		writeJSON(writer, http.StatusOK, result)
		return
	}
	result, syncErr := server.usage.SyncIncremental(request.Context(), body.AccountID, credential.WorkspaceID, credential.AuthCookie, body.MaxPages)
	if syncErr != nil {
		writeError(writer, http.StatusBadGateway, syncErr.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (server server) listLogs(writer http.ResponseWriter, request *http.Request) {
	if server.logs == nil {
		writeJSON(writer, http.StatusOK, logsResponse{Logs: []logDTO{}})
		return
	}
	limit, _ := strconv.Atoi(request.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(request.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 50
	}
	entries, err := server.logs.List(request.Context(), limit, offset)
	if err != nil {
		writeJSON(writer, http.StatusOK, mapLogs(server.logs.Recent(limit)))
		return
	}
	writeJSON(writer, http.StatusOK, mapLogs(entries))
}

func (server server) metrics(writer http.ResponseWriter, _ *http.Request) {
	if server.logs == nil {
		writeJSON(writer, http.StatusOK, map[string]any{"total_requests": 0})
		return
	}
	writeJSON(writer, http.StatusOK, server.logs.Snapshot())
}

func (server server) getVersion(writer http.ResponseWriter, request *http.Request) {
	force := request.URL.Query().Get("refresh") == "1" || request.URL.Query().Get("refresh") == "true"
	if server.versions == nil {
		writeJSON(writer, http.StatusOK, map[string]any{
			"current": "0.0.1", "latest": "0.0.1", "update_available": false,
			"image": "jovepoxy", "source": "local",
		})
		return
	}
	writeJSON(writer, http.StatusOK, server.versions.Get(request.Context(), force))
}

func (server server) getSettings(writer http.ResponseWriter, request *http.Request) {
	passwordCustom := false
	if server.auth != nil {
		passwordCustom = server.auth.PasswordIsCustom(request.Context())
	}
	loadPolicy := string(zenpool.LoadPolicySpread)
	maxAttempts := zenpool.DefaultMaxAttempts
	if server.pool != nil {
		loadPolicy = string(server.pool.LoadPolicy())
		maxAttempts = server.pool.MaxAttempts()
	}
	writeJSON(writer, http.StatusOK, settingsResponse{
		ModelCacheTTLSeconds: int(server.cfg.ModelCacheTTL.Seconds()),
		ShowAllModels:        server.cfg.ShowAllModels,
		OCVersion:            server.cfg.OCVersion,
		Listen:               server.cfg.Listen,
		CookieSecure:         server.cfg.CookieSecure,
		ZenBase:              server.cfg.ZenBase,
		DataDir:              server.cfg.DataDir,
		UpstreamTimeoutSec:   int(server.cfg.UpstreamTimeout.Seconds()),
		SessionTTLHours:      int(auth.DefaultSessionLifetime.Hours()),
		PasswordCustom:       passwordCustom,
		HTTPProxyConfigured:  server.cfg.HTTPProxy != nil,
		HTTPSProxyConfigured: server.cfg.HTTPSProxy != nil,
		LoadPolicy:           loadPolicy,
		MaxFailoverAttempts:  maxAttempts,
	})
}

func (server server) patchSettings(writer http.ResponseWriter, request *http.Request) {
	if server.pool == nil {
		writeError(writer, http.StatusServiceUnavailable, "zen pool unavailable")
		return
	}
	var body patchSettingsRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.LoadPolicy == nil && body.MaxFailoverAttempts == nil {
		writeError(writer, http.StatusBadRequest, "no settings fields to update")
		return
	}
	if body.LoadPolicy != nil {
		policy := zenpool.LoadPolicy(strings.TrimSpace(*body.LoadPolicy))
		if policy != zenpool.LoadPolicySpread && policy != zenpool.LoadPolicySticky {
			writeError(writer, http.StatusBadRequest, "load_policy must be spread or sticky")
			return
		}
		server.pool.SetLoadPolicy(policy)
	}
	if body.MaxFailoverAttempts != nil {
		n := *body.MaxFailoverAttempts
		if n < zenpool.MinMaxAttempts || n > zenpool.MaxMaxAttempts {
			writeError(writer, http.StatusBadRequest, "max_failover_attempts must be 2..4")
			return
		}
		server.pool.SetMaxAttempts(n)
	}
	// Return updated snapshot (same shape as GET).
	server.getSettings(writer, request)
}
