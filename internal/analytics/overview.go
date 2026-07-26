package analytics

import (
	"context"
	"time"

	"jovepoxy/internal/quota"
	"jovepoxy/internal/usage"
)

// Overview is the control-plane dashboard payload.
type Overview struct {
	RequestsToday  int64            `json:"requests_today"`
	TokensToday    int64            `json:"tokens_today"`
	RequestsTotal  int64            `json:"requests_total"`
	TokensTotal    int64            `json:"tokens_total"`
	ByModel        []ModelBreakdown `json:"by_model"`
	QuotaEffective float64          `json:"quota_effective_remaining"`
	QuotaWindows   []CascadedWindow `json:"quota_windows,omitempty"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// ModelBreakdown is per-model usage aggregation.
type ModelBreakdown struct {
	Model        string `json:"model"`
	Requests     int64  `json:"requests"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
}

// UsageReader is the narrow persistence surface analytics needs.
type UsageReader interface {
	AggregateTotals(ctx context.Context, sinceRFC3339 string) (requests int64, tokens int64, err error)
	AggregateByModel(ctx context.Context, limit int) ([]usage.ModelAggregate, error)
}

// Service aggregates usage and optional live quota windows.
type Service struct {
	usage UsageReader
	now   func() time.Time
}

// NewService constructs analytics over a usage aggregation reader (not raw *sql.DB).
func NewService(reader UsageReader) *Service {
	return &Service{usage: reader, now: time.Now}
}

// Overview builds a stable overview document. Empty data returns zeros, never panics.
func (service *Service) Overview(ctx context.Context, windows []quota.Window) (Overview, error) {
	now := service.now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	overview := Overview{
		ByModel:        []ModelBreakdown{},
		QuotaWindows:   ApplyOpenCodeCascade(windows),
		QuotaEffective: EffectiveRemaining(windows),
		UpdatedAt:      now,
	}
	if service.usage == nil {
		return overview, nil
	}
	if requests, tokens, err := service.usage.AggregateTotals(ctx, dayStart); err == nil {
		overview.RequestsToday = requests
		overview.TokensToday = tokens
	}
	if requests, tokens, err := service.usage.AggregateTotals(ctx, ""); err == nil {
		overview.RequestsTotal = requests
		overview.TokensTotal = tokens
	}
	models, err := service.usage.AggregateByModel(ctx, 50)
	if err != nil {
		return overview, err
	}
	for _, item := range models {
		overview.ByModel = append(overview.ByModel, ModelBreakdown{
			Model: item.Model, Requests: item.Requests,
			InputTokens: item.InputTokens, OutputTokens: item.OutputTokens,
		})
	}
	return overview, nil
}
