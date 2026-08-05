package adminapi

import (
	"math"
	"time"

	"jovepoxy/internal/analytics"
	"jovepoxy/internal/effort"
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
	// Provider is the primary chat route (opencode | ollama).
	Provider string `json:"provider"`
	// Providers lists every source advertising this ID (OpenCode Go ∩ Ollama overlap).
	// Always non-empty when mapped from a live catalog entry.
	Providers []string `json:"providers"`
	// EffortLevels is the ordered reasoning_effort set this model accepts
	// after gateway clamp (includes none/auto when allowed).
	EffortLevels []string `json:"effort_levels"`
	// CacheUsage hints that request logs can surface cache counters for this model
	// when upstream includes them (gateway always parses when present).
	CacheUsage bool `json:"cache_usage"`
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

// coldStartHealthScore is the design default when no zen_key_health row / domain is present.
// Kept in adminapi until zenpool exposes Health on Metadata/KeyView.
const coldStartHealthScore = 70.0

type zenKeyDTO struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Prefix string `json:"prefix"`
	// Weight is legacy JSON for old clients; P0 selection ignores it (health_score drives share).
	Weight        int        `json:"weight"`
	Enabled       bool       `json:"enabled"`
	Provider      string     `json:"provider"`
	CooldownUntil *time.Time `json:"cooldown_until,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	// Status: active | cooling | benched | disabled | probing (probing when domain supports it).
	Status string `json:"status"`
	// TrafficPct is the estimated dynamic share within the same provider (not historical hits).
	TrafficPct           float64 `json:"traffic_pct"`
	CooldownRemainingSec int     `json:"cooldown_remaining_sec"`

	// Dynamic health (secret-free). Cold-start defaults apply when zenpool health domain is absent.
	HealthScore         float64    `json:"health_score"`
	SelectionScore      float64    `json:"selection_score"`
	SuccessCount        int        `json:"success_count"`
	FailureCount        int        `json:"failure_count"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	LastErrorClass      string     `json:"last_error_class"`
	LastSuccessAt       *time.Time `json:"last_success_at,omitempty"`
	LastFailureAt       *time.Time `json:"last_failure_at,omitempty"`
	HealthUpdatedAt     *time.Time `json:"health_updated_at,omitempty"`
	CooldownReason      string     `json:"cooldown_reason"`
}

type zenPoolProviderSummaryDTO struct {
	Total     int `json:"total"`
	Enabled   int `json:"enabled"`
	Healthy   int `json:"healthy"`
	Cooled    int `json:"cooled"`
	Benched   int `json:"benched"`
	Disabled  int `json:"disabled"`
	Probing   int `json:"probing"`
	Attention int `json:"attention"`
}

// zenPoolSummaryDTO is secret-free pool health for overview / key list.
type zenPoolSummaryDTO struct {
	Total     int `json:"total"`
	Enabled   int `json:"enabled"`
	Healthy   int `json:"healthy"`
	Cooled    int `json:"cooled"`
	Benched   int `json:"benched"`
	Disabled  int `json:"disabled"`
	Probing   int `json:"probing"`
	Attention int `json:"attention"`
	// ShareMode documents how traffic_pct is derived; not historical hit rate or cross-provider routing.
	ShareMode  string                               `json:"share_mode,omitempty"`
	ByProvider map[string]zenPoolProviderSummaryDTO `json:"by_provider,omitempty"`
}

type zenKeysResponse struct {
	Keys    []zenKeyDTO        `json:"keys"`
	Summary *zenPoolSummaryDTO `json:"summary,omitempty"`
}

type createZenKeyRequest struct {
	Label  string `json:"label"`
	Secret string `json:"secret"`
	// Weight is accepted for API compatibility but does not control P0 scheduling.
	Weight   int    `json:"weight"`
	Provider string `json:"provider"`
}

type updateZenKeyRequest struct {
	Label  string `json:"label"`
	Secret string `json:"secret"`
	// Weight is accepted for API compatibility but does not control P0 scheduling.
	Weight int `json:"weight"`
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
	ID    string `json:"id"`
	KeyID string `json:"key_id,omitempty"`
	Model string `json:"model"`
	Route string `json:"route"`
	// Upstream is the data-plane channel: opencode_free | opencode_paid | ollama_paid.
	Upstream            string    `json:"upstream,omitempty"`
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
	// BenchDurationMinutes is process-memory 401 isolation window (1..60, default 10).
	BenchDurationMinutes int `json:"bench_duration_minutes"`
}

type patchSettingsRequest struct {
	LoadPolicy           *string `json:"load_policy"`
	MaxFailoverAttempts  *int    `json:"max_failover_attempts"`
	BenchDurationMinutes *int    `json:"bench_duration_minutes"`
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
	OpsKPIs *analytics.OpsKPIs `json:"ops_kpis,omitempty"`
	// RoutingKPIs groups the same window's request metadata by final upstream channel.
	RoutingKPIs *analytics.RoutingKPIs `json:"routing_kpis,omitempty"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

func mapModels(result models.Result) modelsResponse {
	out := make([]modelDTO, 0, len(result.Models))
	for _, model := range result.Models {
		provider := string(models.NormalizeProvider(model.Provider))
		id := string(model.ID)
		providers := models.ProvidersOf(model)
		providerNames := make([]string, 0, len(providers))
		for _, p := range providers {
			providerNames = append(providerNames, string(p))
		}
		out = append(out, modelDTO{
			ID:           id,
			Free:         model.Free,
			Provider:     provider,
			Providers:    providerNames,
			EffortLevels: effort.LevelsForDisplay(id),
			CacheUsage:   true, // gateway parses cache fields whenever upstream emits them
		})
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
		dto := zenKeyDTO{
			ID: string(view.ID), Label: view.Label, Prefix: view.Prefix, Weight: view.Weight,
			Enabled: view.Enabled, Provider: provider,
			CooldownUntil: view.CooldownUntil, CreatedAt: view.CreatedAt,
			Status:               string(view.Status),
			CooldownRemainingSec: view.CooldownRemainingSec,
		}
		applyHealthView(&dto, view, now)
		out = append(out, dto)
	}
	// Dynamic share estimate from selection_score (not legacy weight, not historical hits).
	applyDynamicTrafficShares(out)
	summary := mapZenPoolSummaryFromKeys(out, zenpool.Summarize(list, now, benched))
	return zenKeysResponse{Keys: out, Summary: &summary}
}

// applyHealthView fills secret-free health fields from zenpool's persisted state.
// Explicit health_score 0 is preserved; only a fully empty cold-start view defaults to 70.
func applyHealthView(dto *zenKeyDTO, view zenpool.KeyView, _ time.Time) {
	if dto == nil {
		return
	}
	dto.HealthScore = view.HealthScore
	// Cold-start: no samples/state and score unset -> DefaultHealthScore.
	// Do not rewrite a deliberate 0 (e.g. after heavy failures with samples).
	empty := view.SuccessCount == 0 &&
		view.FailureCount == 0 &&
		view.ConsecutiveFailures == 0 &&
		view.LastErrorClass == "" &&
		view.LastSuccessAt == nil &&
		view.LastFailureAt == nil &&
		view.HealthUpdatedAt == nil &&
		view.CooldownReason == ""
	if empty && dto.HealthScore == 0 {
		dto.HealthScore = coldStartHealthScore
	}
	dto.SelectionScore = float64(view.SelectionScore)
	if dto.SelectionScore <= 0 {
		// Ephemeral mass when domain left SelectionScore unset; keep min 1 for real 0 health.
		dto.SelectionScore = math.Max(1, math.Round(dto.HealthScore))
	}
	dto.SuccessCount = view.SuccessCount
	dto.FailureCount = view.FailureCount
	dto.ConsecutiveFailures = view.ConsecutiveFailures
	dto.LastErrorClass = view.LastErrorClass
	dto.LastSuccessAt = view.LastSuccessAt
	dto.LastFailureAt = view.LastFailureAt
	dto.HealthUpdatedAt = view.HealthUpdatedAt
	dto.CooldownReason = view.CooldownReason
}

// applyDynamicTrafficShares sets traffic_pct from selection_score within each provider.
// Only status=active keys participate; others get 0. Values rounded to 1 decimal.
func applyDynamicTrafficShares(keys []zenKeyDTO) {
	if len(keys) == 0 {
		return
	}
	type shareAcc struct {
		total float64
		idxs  []int
	}
	byProvider := make(map[string]*shareAcc)
	for i, key := range keys {
		provider := key.Provider
		if provider == "" {
			provider = string(zenpool.ProviderOpenCode)
		}
		acc, ok := byProvider[provider]
		if !ok {
			acc = &shareAcc{}
			byProvider[provider] = acc
		}
		if key.Status == string(zenpool.StatusActive) && key.SelectionScore > 0 {
			acc.total += key.SelectionScore
			acc.idxs = append(acc.idxs, i)
		} else {
			keys[i].TrafficPct = 0
		}
	}
	for _, acc := range byProvider {
		if acc.total <= 0 {
			for _, idx := range acc.idxs {
				keys[idx].TrafficPct = 0
			}
			continue
		}
		for _, idx := range acc.idxs {
			pct := keys[idx].SelectionScore / acc.total * 100
			keys[idx].TrafficPct = math.Round(pct*10) / 10
		}
	}
}

func mapZenPoolSummary(sum zenpool.PoolSummary) zenPoolSummaryDTO {
	return mapZenPoolSummaryFromKeys(nil, sum)
}

// mapZenPoolSummaryFromKeys enriches pool counters with probing/attention and share mode.
// When key DTOs are present, probing is counted from status=probing rows.
// When keys is empty/nil (overview path), use domain PoolSummary.Probing so attention is not undercounted.
// Attention = cooled + benched + probing (keys needing operator awareness).
func mapZenPoolSummaryFromKeys(keys []zenKeyDTO, sum zenpool.PoolSummary) zenPoolSummaryDTO {
	probing := sum.Probing
	probingByProvider := map[string]int{}
	if len(sum.ByProvider) > 0 {
		for provider, item := range sum.ByProvider {
			probingByProvider[provider] = item.Probing
		}
	}
	if len(keys) > 0 {
		probing = 0
		probingByProvider = map[string]int{}
		for _, key := range keys {
			if key.Status != "probing" {
				continue
			}
			probing++
			provider := key.Provider
			if provider == "" {
				provider = string(zenpool.ProviderOpenCode)
			}
			probingByProvider[provider]++
		}
	}
	attention := sum.Cooled + sum.Benched + probing
	dto := zenPoolSummaryDTO{
		Total: sum.Total, Enabled: sum.Enabled, Healthy: sum.Healthy,
		Cooled: sum.Cooled, Benched: sum.Benched, Disabled: sum.Disabled,
		Probing: probing, Attention: attention,
		ShareMode: "dynamic_health_estimate",
	}
	if len(sum.ByProvider) > 0 {
		dto.ByProvider = make(map[string]zenPoolProviderSummaryDTO, len(sum.ByProvider))
		for provider, item := range sum.ByProvider {
			pProbing := probingByProvider[provider]
			dto.ByProvider[provider] = zenPoolProviderSummaryDTO{
				Total: item.Total, Enabled: item.Enabled, Healthy: item.Healthy,
				Cooled: item.Cooled, Benched: item.Benched, Disabled: item.Disabled,
				Probing: pProbing, Attention: item.Cooled + item.Benched + pProbing,
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
			Upstream: entry.Upstream,
			Status:   entry.Status, LatencyMS: entry.LatencyMS, TTFTMS: entry.TTFTMS,
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
