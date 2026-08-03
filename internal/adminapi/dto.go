package adminapi

import (
	"math"
	"time"

	"jovepoxy/internal/analytics"
	"jovepoxy/internal/keys"
	"jovepoxy/internal/models"
	"jovepoxy/internal/quota"
	"jovepoxy/internal/reqlog"
	"jovepoxy/internal/usage"
	"jovepoxy/internal/zenpool"
)

// ErrorBody is the stable error envelope for admin APIs.
type ErrorBody struct {
	Error string `json:"error"`
}

type loginRequest struct {
	Password string `json:"password"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type okResponse struct {
	OK bool `json:"ok"`
}

type meResponse struct {
	OK   bool   `json:"ok"`
	Role string `json:"role"`
}

type loginResponse struct {
	OK        bool      `json:"ok"`
	ExpiresAt time.Time `json:"expires_at"`
}

type modelDTO struct {
	ID       string `json:"id"`
	Free     bool   `json:"free"`
	Provider string `json:"provider"`
}

type modelsResponse struct {
	Models []modelDTO `json:"models"`
	Stale  bool       `json:"stale"`
}

type localKeyDTO struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Prefix     string `json:"prefix"`
	Enabled    bool   `json:"enabled"`
	Revoked    bool   `json:"revoked"`
	RPMLimit   int    `json:"rpm_limit"`
	DailyLimit int    `json:"daily_limit"`
}

type localKeyCreatedDTO struct {
	ID     string `json:"id"`
	Prefix string `json:"prefix"`
	Secret string `json:"secret"`
}

type localKeysResponse struct {
	Keys []localKeyDTO `json:"keys"`
}

type createLocalKeyRequest struct {
	Label      string `json:"label"`
	RPMLimit   int    `json:"rpm_limit"`
	DailyLimit int    `json:"daily_limit"`
}

type updateLocalKeyRequest struct {
	Label      string `json:"label"`
	RPMLimit   int    `json:"rpm_limit"`
	DailyLimit int    `json:"daily_limit"`
}

type zenKeyDTO struct {
	ID                   string     `json:"id"`
	Label                string     `json:"label"`
	Prefix               string     `json:"prefix"`
	Weight               int        `json:"weight"`
	Enabled              bool       `json:"enabled"`
	Provider             string     `json:"provider"`
	CooldownUntil        *time.Time `json:"cooldown_until,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	Status               string     `json:"status"`
	TrafficPct           float64    `json:"traffic_pct"`
	CooldownRemainingSec int        `json:"cooldown_remaining_sec"`
}

type zenPoolProviderSummaryDTO struct {
	Total    int `json:"total"`
	Enabled  int `json:"enabled"`
	Healthy  int `json:"healthy"`
	Cooled   int `json:"cooled"`
	Benched  int `json:"benched"`
	Disabled int `json:"disabled"`
}

// zenPoolSummaryDTO is secret-free pool health for overview / key list.
type zenPoolSummaryDTO struct {
	Total      int                                  `json:"total"`
	Enabled    int                                  `json:"enabled"`
	Healthy    int                                  `json:"healthy"`
	Cooled     int                                  `json:"cooled"`
	Benched    int                                  `json:"benched"`
	Disabled   int                                  `json:"disabled"`
	ByProvider map[string]zenPoolProviderSummaryDTO `json:"by_provider,omitempty"`
}

type zenKeysResponse struct {
	Keys    []zenKeyDTO        `json:"keys"`
	Summary *zenPoolSummaryDTO `json:"summary,omitempty"`
}

type createZenKeyRequest struct {
	Label    string `json:"label"`
	Secret   string `json:"secret"`
	Weight   int    `json:"weight"`
	Provider string `json:"provider"`
}

type updateZenKeyRequest struct {
	Label  string `json:"label"`
	Secret string `json:"secret"`
	Weight int    `json:"weight"`
}

type accountDTO struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	WorkspaceID  string `json:"workspace_id"`
	MaskedCookie string `json:"masked_cookie"`
	ShowRolling  bool   `json:"show_rolling"`
	ShowWeekly   bool   `json:"show_weekly"`
	ShowMonthly  bool   `json:"show_monthly"`
	Enabled      bool   `json:"enabled"`
}

type accountsResponse struct {
	Accounts []accountDTO `json:"accounts"`
}

type createAccountRequest struct {
	Name        string `json:"name"`
	WorkspaceID string `json:"workspace_id"`
	AuthCookie  string `json:"auth_cookie"`
	ShowRolling bool   `json:"show_rolling"`
	ShowWeekly  bool   `json:"show_weekly"`
	ShowMonthly bool   `json:"show_monthly"`
	Enabled     bool   `json:"enabled"`
}

type quotaWindowDTO struct {
	Label       string   `json:"label"`
	Used        float64  `json:"used"`
	Remaining   float64  `json:"remaining"`
	Total       float64  `json:"total"`
	Unit        string   `json:"unit"`
	ResetInSec  int      `json:"reset_in_sec"`
	UsedPct     *float64 `json:"used_pct,omitempty"`
	HeadroomPct *float64 `json:"headroom_pct,omitempty"`
	BurnPerDay  *float64 `json:"burn_per_day,omitempty"`
	DaysToEmpty *float64 `json:"days_to_empty,omitempty"`
}

// accountQuotaNarrativeDTO is an optional account-level summary of the busiest window.
type accountQuotaNarrativeDTO struct {
	PrimaryLabel string   `json:"primary_label,omitempty"`
	UsedPct      *float64 `json:"used_pct,omitempty"`
	HeadroomPct  *float64 `json:"headroom_pct,omitempty"`
	DaysToEmpty  *float64 `json:"days_to_empty,omitempty"`
	Note         string   `json:"note,omitempty"`
}

type accountQuotaDTO struct {
	AccountID   string                    `json:"account_id"`
	Name        string                    `json:"name"`
	WorkspaceID string                    `json:"workspace_id"`
	Success     bool                      `json:"success"`
	UpdatedAt   time.Time                 `json:"updated_at"`
	Windows     []quotaWindowDTO          `json:"windows,omitempty"`
	Narrative   *accountQuotaNarrativeDTO `json:"narrative,omitempty"`
	Error       string                    `json:"error,omitempty"`
}

type quotasResponse struct {
	Quotas []accountQuotaDTO `json:"quotas"`
}

type usageRecordDTO struct {
	ID           string `json:"id"`
	AccountID    string `json:"account_id"`
	USGID        string `json:"usg_id"`
	Model        string `json:"model"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	RecordedAt   string `json:"recorded_at"`
}

type usageResponse struct {
	Records   []usageRecordDTO `json:"records"`
	Truncated bool             `json:"truncated,omitempty"`
	Limit     int              `json:"limit,omitempty"`
}

type logDTO struct {
	ID                  string    `json:"id"`
	KeyID               string    `json:"key_id,omitempty"`
	Model               string    `json:"model"`
	Route               string    `json:"route"`
	Status              int       `json:"status"`
	LatencyMS           int64     `json:"latency_ms"`
	TTFTMS              int64     `json:"ttft_ms"`
	Stream              bool      `json:"stream"`
	ErrorClass          string    `json:"error_class,omitempty"`
	MaxTokens           int       `json:"max_tokens,omitempty"`
	ReasoningEffort     string    `json:"reasoning_effort,omitempty"`
	ThinkingType        string    `json:"thinking_type,omitempty"`
	BudgetTokens        int       `json:"budget_tokens,omitempty"`
	InputTokens         int       `json:"input_tokens"`
	OutputTokens        int       `json:"output_tokens"`
	CacheReadTokens     int       `json:"cache_read_tokens"`
	CacheCreationTokens int       `json:"cache_creation_tokens"`
	CreatedAt           time.Time `json:"created_at"`
}

type logsResponse struct {
	Logs      []logDTO `json:"logs"`
	Truncated bool     `json:"truncated,omitempty"`
	Limit     int      `json:"limit,omitempty"`
}

type settingsResponse struct {
	ModelCacheTTLSeconds int    `json:"model_cache_ttl_seconds"`
	ShowAllModels        bool   `json:"show_all_models"`
	OCVersion            string `json:"oc_version"`
	Listen               string `json:"listen"`
	CookieSecure         bool   `json:"cookie_secure"`
	ZenBase              string `json:"zen_base"`
	DataDir              string `json:"data_dir"`
	UpstreamTimeoutSec   int    `json:"upstream_timeout_seconds"`
	SessionTTLHours      int    `json:"session_ttl_hours"`
	PasswordCustom       bool   `json:"password_custom"`
	HTTPProxyConfigured  bool   `json:"http_proxy_configured"`
	HTTPSProxyConfigured bool   `json:"https_proxy_configured"`
	// LoadPolicy is zenpool selection: spread (default) | sticky.
	LoadPolicy string `json:"load_policy"`
	// MaxFailoverAttempts is ProxyPaid attempts per request (2..4, default 2).
	MaxFailoverAttempts int `json:"max_failover_attempts"`
}

type patchSettingsRequest struct {
	LoadPolicy          *string `json:"load_policy"`
	MaxFailoverAttempts *int    `json:"max_failover_attempts"`
}

type overviewResponse struct {
	RequestsToday  int64                      `json:"requests_today"`
	TokensToday    int64                      `json:"tokens_today"`
	RequestsTotal  int64                      `json:"requests_total"`
	TokensTotal    int64                      `json:"tokens_total"`
	ByModel        []analytics.ModelBreakdown `json:"by_model"`
	QuotaEffective float64                    `json:"quota_effective_remaining"`
	QuotaWindows   []analytics.CascadedWindow `json:"quota_windows,omitempty"`
	// QuotaNarrative is owned by the quota burn/headroom surface.
	QuotaNarrative *analytics.QuotaNarrative `json:"quota_narrative,omitempty"`
	// ZenPool is owned by the zenpool status surface; do not merge quota narrative here.
	ZenPool *zenPoolSummaryDTO `json:"zen_pool,omitempty"`
	// OpsKPIs is owned by the overview-ops-kpis surface (reqlog time-window aggregates).
	OpsKPIs   *analytics.OpsKPIs `json:"ops_kpis,omitempty"`
	UpdatedAt time.Time          `json:"updated_at"`
}

func mapModels(result models.Result) modelsResponse {
	out := make([]modelDTO, 0, len(result.Models))
	for _, model := range result.Models {
		provider := string(models.NormalizeProvider(model.Provider))
		out = append(out, modelDTO{ID: string(model.ID), Free: model.Free, Provider: provider})
	}
	return modelsResponse{Models: out, Stale: result.Stale}
}

func mapLocalKeys(list []keys.KeyMetadata) localKeysResponse {
	out := make([]localKeyDTO, 0, len(list))
	for _, item := range list {
		out = append(out, localKeyDTO{
			ID: string(item.ID), Label: item.Label, Prefix: item.Prefix,
			Enabled: item.Enabled, Revoked: item.Revoked, RPMLimit: item.RPMLimit, DailyLimit: item.DailyLimit,
		})
	}
	return localKeysResponse{Keys: out}
}

func mapLocalKeyCreated(created keys.Creation) localKeyCreatedDTO {
	return localKeyCreatedDTO{ID: string(created.ID), Prefix: created.Prefix, Secret: created.Secret}
}

// mapZenKeys maps without benched state (create/update single-key DTOs).
func mapZenKeys(list []zenpool.Metadata) zenKeysResponse {
	return mapZenKeysAt(list, time.Now().UTC(), nil)
}

func mapZenKeysAt(list []zenpool.Metadata, now time.Time, benched map[zenpool.KeyID]time.Time) zenKeysResponse {
	views := zenpool.DeriveViews(list, now, benched)
	out := make([]zenKeyDTO, 0, len(views))
	for _, view := range views {
		provider := string(view.Provider)
		if provider == "" {
			provider = string(zenpool.ProviderOpenCode)
		}
		// Round to one decimal for stable JSON / UI display.
		pct := math.Round(view.TrafficPct*10) / 10
		out = append(out, zenKeyDTO{
			ID: string(view.ID), Label: view.Label, Prefix: view.Prefix, Weight: view.Weight,
			Enabled: view.Enabled, Provider: provider,
			CooldownUntil: view.CooldownUntil, CreatedAt: view.CreatedAt,
			Status: string(view.Status), TrafficPct: pct,
			CooldownRemainingSec: view.CooldownRemainingSec,
		})
	}
	summary := mapZenPoolSummary(zenpool.Summarize(list, now, benched))
	return zenKeysResponse{Keys: out, Summary: &summary}
}

func mapZenPoolSummary(sum zenpool.PoolSummary) zenPoolSummaryDTO {
	dto := zenPoolSummaryDTO{
		Total: sum.Total, Enabled: sum.Enabled, Healthy: sum.Healthy,
		Cooled: sum.Cooled, Benched: sum.Benched, Disabled: sum.Disabled,
	}
	if len(sum.ByProvider) > 0 {
		dto.ByProvider = make(map[string]zenPoolProviderSummaryDTO, len(sum.ByProvider))
		for provider, item := range sum.ByProvider {
			dto.ByProvider[provider] = zenPoolProviderSummaryDTO{
				Total: item.Total, Enabled: item.Enabled, Healthy: item.Healthy,
				Cooled: item.Cooled, Benched: item.Benched, Disabled: item.Disabled,
			}
		}
	}
	return dto
}

func mapAccounts(list []quota.Account) accountsResponse {
	out := make([]accountDTO, 0, len(list))
	for _, item := range list {
		out = append(out, accountDTO{
			ID: string(item.ID), Name: item.Name, WorkspaceID: item.WorkspaceID,
			MaskedCookie: item.MaskedCookie, ShowRolling: item.ShowRolling,
			ShowWeekly: item.ShowWeekly, ShowMonthly: item.ShowMonthly, Enabled: item.Enabled,
		})
	}
	return accountsResponse{Accounts: out}
}

func mapAccount(item quota.Account) accountDTO {
	return accountDTO{
		ID: string(item.ID), Name: item.Name, WorkspaceID: item.WorkspaceID,
		MaskedCookie: item.MaskedCookie, ShowRolling: item.ShowRolling,
		ShowWeekly: item.ShowWeekly, ShowMonthly: item.ShowMonthly, Enabled: item.Enabled,
	}
}

func mapQuotaWindow(window quota.Window) quotaWindowDTO {
	narrative := quota.DeriveWindowNarrative(window.Used, window.Remaining, window.Total)
	return quotaWindowDTO{
		Label: window.Label, Used: window.Used, Remaining: window.Remaining,
		Total: window.Total, Unit: window.Unit, ResetInSec: window.ResetInSec,
		UsedPct: narrative.UsedPct, HeadroomPct: narrative.HeadroomPct,
		BurnPerDay: narrative.BurnPerDay, DaysToEmpty: narrative.DaysToEmpty,
	}
}

func mapAccountNarrative(item quota.AccountQuota) *accountQuotaNarrativeDTO {
	if !item.Success {
		return nil
	}
	label, narrative, note := quota.PickPrimaryNarrative(item.Windows)
	if narrative.UsedPct == nil && note == "" {
		return nil
	}
	out := &accountQuotaNarrativeDTO{
		PrimaryLabel: label,
		UsedPct:      narrative.UsedPct,
		HeadroomPct:  narrative.HeadroomPct,
		DaysToEmpty:  narrative.DaysToEmpty,
		Note:         note,
	}
	return out
}

func mapQuotas(list []quota.AccountQuota) quotasResponse {
	out := make([]accountQuotaDTO, 0, len(list))
	for _, item := range list {
		windows := make([]quotaWindowDTO, 0, len(item.Windows))
		for _, window := range item.Windows {
			windows = append(windows, mapQuotaWindow(window))
		}
		out = append(out, accountQuotaDTO{
			AccountID: string(item.AccountID), Name: item.Name, WorkspaceID: item.WorkspaceID,
			Success: item.Success, UpdatedAt: item.UpdatedAt, Windows: windows,
			Narrative: mapAccountNarrative(item), Error: item.Error,
		})
	}
	return quotasResponse{Quotas: out}
}

func mapUsage(records []usage.StoredRecord) usageResponse {
	out := make([]usageRecordDTO, 0, len(records))
	for _, record := range records {
		out = append(out, usageRecordDTO{
			ID: record.ID, AccountID: record.AccountID, USGID: record.USGID, Model: record.Model,
			InputTokens: record.InputTokens, OutputTokens: record.OutputTokens, RecordedAt: record.RecordedAt,
		})
	}
	return usageResponse{Records: out}
}

func mapLogs(entries []reqlog.Entry) logsResponse {
	out := make([]logDTO, 0, len(entries))
	for _, entry := range entries {
		out = append(out, logDTO{
			ID: entry.ID, KeyID: entry.KeyID, Model: entry.Model, Route: entry.Route,
			Status: entry.Status, LatencyMS: entry.LatencyMS, TTFTMS: entry.TTFTMS,
			Stream: entry.Stream, ErrorClass: entry.ErrorClass, MaxTokens: entry.MaxTokens,
			ReasoningEffort: entry.ReasoningEffort, ThinkingType: entry.ThinkingType,
			BudgetTokens: entry.BudgetTokens,
			InputTokens:  entry.InputTokens, OutputTokens: entry.OutputTokens,
			CacheReadTokens: entry.CacheReadTokens, CacheCreationTokens: entry.CacheCreationTokens,
			CreatedAt: entry.CreatedAt,
		})
	}
	return logsResponse{Logs: out}
}

func mapOverview(overview analytics.Overview) overviewResponse {
	return overviewResponse{
		RequestsToday: overview.RequestsToday, TokensToday: overview.TokensToday,
		RequestsTotal: overview.RequestsTotal, TokensTotal: overview.TokensTotal,
		ByModel: overview.ByModel, QuotaEffective: overview.QuotaEffective,
		QuotaWindows: overview.QuotaWindows, QuotaNarrative: overview.QuotaNarrative,
		UpdatedAt: overview.UpdatedAt,
	}
}
