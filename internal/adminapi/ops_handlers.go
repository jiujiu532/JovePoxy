package adminapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"jovepoxy/internal/analytics"
	"jovepoxy/internal/auth"
	"jovepoxy/internal/quota"
	"jovepoxy/internal/reqlog"
	"jovepoxy/internal/usage"
	"jovepoxy/internal/version"
	"jovepoxy/internal/zenpool"
)

func (server server) overview(writer http.ResponseWriter, request *http.Request) {
	window := analytics.NormalizeWindow(request.URL.Query().Get("window"))
	opsKPIs, routingKPIs := server.buildWindowKPIs(request, window)
	if server.analytics == nil {
		resp := overviewResponse{ByModel: nil}
		resp.ZenPool = server.zenPoolSummary(request)
		resp.OpsKPIs = opsKPIs
		resp.RoutingKPIs = routingKPIs
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
	resp.OpsKPIs = opsKPIs
	resp.RoutingKPIs = routingKPIs
	writeJSON(writer, http.StatusOK, resp)
}

const overviewLogLimit = 5000

// loadOverviewLogEntries prefers persisted logs and falls back to in-memory recent logs.
func (server server) loadOverviewLogEntries(request *http.Request) []reqlog.Entry {
	if server.logs == nil {
		return nil
	}
	listed, err := server.logs.List(request.Context(), overviewLogLimit, 0)
	if err != nil {
		return server.logs.Recent(overviewLogLimit)
	}
	return listed
}

// buildWindowKPIs aggregates all overview request KPIs from one bounded log load.
func (server server) buildWindowKPIs(request *http.Request, window string) (*analytics.OpsKPIs, *analytics.RoutingKPIs) {
	now := time.Now().UTC()
	entries := server.loadOverviewLogEntries(request)
	ops := analytics.AggregateOpsKPIs(entries, window, now)
	routing := analytics.AggregateRoutingKPIs(entries, window, now)
	return &ops, &routing
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
	if limit <= 0 {
		limit = 50
	}
	accountID := strings.TrimSpace(request.URL.Query().Get("account_id"))
	from, err := parseTimeQuery(request.URL.Query().Get("from"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "invalid from")
		return
	}
	to, err := parseTimeQuery(request.URL.Query().Get("to"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "invalid to")
		return
	}
	records, err := server.usage.ListFiltered(request.Context(), usage.ListFilter{
		AccountID: accountID,
		From:      from,
		To:        to,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "list usage failed")
		return
	}
	resp := mapUsage(records)
	resp.Limit = limit
	resp.Truncated = len(records) >= limit
	writeJSON(writer, http.StatusOK, resp)
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
	from, err := parseTimeQuery(request.URL.Query().Get("from"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "invalid from")
		return
	}
	to, err := parseTimeQuery(request.URL.Query().Get("to"))
	if err != nil {
		writeError(writer, http.StatusBadRequest, "invalid to")
		return
	}
	filter := reqlog.ListFilter{From: from, To: to, Limit: limit, Offset: offset}
	entries, err := server.logs.ListFiltered(request.Context(), filter)
	if err != nil {
		// Fallback to in-memory ring (no time filter); still report limit/truncated.
		recent := server.logs.Recent(limit)
		resp := mapLogs(recent)
		resp.Limit = limit
		resp.Truncated = len(recent) >= limit
		writeJSON(writer, http.StatusOK, resp)
		return
	}
	resp := mapLogs(entries)
	resp.Limit = limit
	resp.Truncated = len(entries) >= limit
	writeJSON(writer, http.StatusOK, resp)
}

// parseTimeQuery parses optional RFC3339 / RFC3339Nano query times as UTC.
// Empty string yields zero time (open-ended filter bound).
func parseTimeQuery(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return ts.UTC(), nil
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts.UTC(), nil
	}
	return time.Time{}, errors.New("invalid time")
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
			"current": version.Current, "latest": version.Current, "update_available": false,
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
	benchMinutes := int(zenpool.DefaultBenchDuration / time.Minute)
	if server.pool != nil {
		loadPolicy = string(server.pool.LoadPolicy())
		maxAttempts = server.pool.MaxAttempts()
		benchMinutes = server.pool.BenchMinutes()
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
		BenchDurationMinutes: benchMinutes,
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
	if body.LoadPolicy == nil && body.MaxFailoverAttempts == nil && body.BenchDurationMinutes == nil {
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
	if body.BenchDurationMinutes != nil {
		n := *body.BenchDurationMinutes
		if n < zenpool.MinBenchMinutes || n > zenpool.MaxBenchMinutes {
			writeError(writer, http.StatusBadRequest, "bench_duration_minutes must be 1..60")
			return
		}
		server.pool.SetBenchMinutes(n)
	}
	// Return updated snapshot (same shape as GET).
	server.getSettings(writer, request)
}
