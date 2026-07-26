package adminapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"jovepoxy/internal/auth"
	"jovepoxy/internal/quota"
)

func (server server) overview(writer http.ResponseWriter, request *http.Request) {
	if server.analytics == nil {
		writeJSON(writer, http.StatusOK, overviewResponse{ByModel: nil})
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
	writeJSON(writer, http.StatusOK, mapOverview(overview))
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
	})
}
