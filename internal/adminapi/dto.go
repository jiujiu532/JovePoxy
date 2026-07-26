package adminapi

import (
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
	ID   string `json:"id"`
	Free bool   `json:"free"`
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
	ID            string     `json:"id"`
	Label         string     `json:"label"`
	Prefix        string     `json:"prefix"`
	Weight        int        `json:"weight"`
	Enabled       bool       `json:"enabled"`
	Provider      string     `json:"provider"`
	CooldownUntil *time.Time `json:"cooldown_until,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type zenKeysResponse struct {
	Keys []zenKeyDTO `json:"keys"`
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
	Label      string  `json:"label"`
	Used       float64 `json:"used"`
	Remaining  float64 `json:"remaining"`
	Total      float64 `json:"total"`
	Unit       string  `json:"unit"`
	ResetInSec int     `json:"reset_in_sec"`
}

type accountQuotaDTO struct {
	AccountID   string           `json:"account_id"`
	Name        string           `json:"name"`
	WorkspaceID string           `json:"workspace_id"`
	Success     bool             `json:"success"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Windows     []quotaWindowDTO `json:"windows,omitempty"`
	Error       string           `json:"error,omitempty"`
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
	Records []usageRecordDTO `json:"records"`
}

type logDTO struct {
	ID         string    `json:"id"`
	KeyID      string    `json:"key_id,omitempty"`
	Model      string    `json:"model"`
	Route      string    `json:"route"`
	Status     int       `json:"status"`
	LatencyMS  int64     `json:"latency_ms"`
	Stream     bool      `json:"stream"`
	ErrorClass string    `json:"error_class,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type logsResponse struct {
	Logs []logDTO `json:"logs"`
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
}

type overviewResponse struct {
	RequestsToday  int64                       `json:"requests_today"`
	TokensToday    int64                       `json:"tokens_today"`
	RequestsTotal  int64                       `json:"requests_total"`
	TokensTotal    int64                       `json:"tokens_total"`
	ByModel        []analytics.ModelBreakdown  `json:"by_model"`
	QuotaEffective float64                     `json:"quota_effective_remaining"`
	QuotaWindows   []analytics.CascadedWindow  `json:"quota_windows,omitempty"`
	UpdatedAt      time.Time                   `json:"updated_at"`
}

func mapModels(result models.Result) modelsResponse {
	out := make([]modelDTO, 0, len(result.Models))
	for _, model := range result.Models {
		out = append(out, modelDTO{ID: string(model.ID), Free: model.Free})
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

func mapZenKeys(list []zenpool.Metadata) zenKeysResponse {
	out := make([]zenKeyDTO, 0, len(list))
	for _, item := range list {
		provider := string(item.Provider)
		if provider == "" {
			provider = string(zenpool.ProviderOpenCode)
		}
		out = append(out, zenKeyDTO{
			ID: string(item.ID), Label: item.Label, Prefix: item.Prefix, Weight: item.Weight,
			Enabled: item.Enabled, Provider: provider,
			CooldownUntil: item.CooldownUntil, CreatedAt: item.CreatedAt,
		})
	}
	return zenKeysResponse{Keys: out}
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

func mapQuotas(list []quota.AccountQuota) quotasResponse {
	out := make([]accountQuotaDTO, 0, len(list))
	for _, item := range list {
		windows := make([]quotaWindowDTO, 0, len(item.Windows))
		for _, window := range item.Windows {
			windows = append(windows, quotaWindowDTO{
				Label: window.Label, Used: window.Used, Remaining: window.Remaining,
				Total: window.Total, Unit: window.Unit, ResetInSec: window.ResetInSec,
			})
		}
		out = append(out, accountQuotaDTO{
			AccountID: string(item.AccountID), Name: item.Name, WorkspaceID: item.WorkspaceID,
			Success: item.Success, UpdatedAt: item.UpdatedAt, Windows: windows, Error: item.Error,
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
			Status: entry.Status, LatencyMS: entry.LatencyMS, Stream: entry.Stream,
			ErrorClass: entry.ErrorClass, CreatedAt: entry.CreatedAt,
		})
	}
	return logsResponse{Logs: out}
}

func mapOverview(overview analytics.Overview) overviewResponse {
	return overviewResponse{
		RequestsToday: overview.RequestsToday, TokensToday: overview.TokensToday,
		RequestsTotal: overview.RequestsTotal, TokensTotal: overview.TokensTotal,
		ByModel: overview.ByModel, QuotaEffective: overview.QuotaEffective,
		QuotaWindows: overview.QuotaWindows, UpdatedAt: overview.UpdatedAt,
	}
}
