package adminapi

import (
	"time"

	"jovepoxy/internal/ollama"
	"jovepoxy/internal/quota"
)

// ollamaQuotaDTO is the control-plane shape for Ollama account scrape results.
// Additive narrative fields mirror OpenCode windows for UI reuse.
type ollamaQuotaDTO struct {
	AccountID string                    `json:"account_id"`
	Name      string                    `json:"name"`
	Success   bool                      `json:"success"`
	UpdatedAt time.Time                 `json:"updated_at"`
	Plan      string                    `json:"plan,omitempty"`
	Windows   []ollamaQuotaWindowDTO    `json:"windows,omitempty"`
	Narrative *accountQuotaNarrativeDTO `json:"narrative,omitempty"`
	Error     string                    `json:"error,omitempty"`
}

type ollamaQuotaWindowDTO struct {
	Label       string              `json:"label"`
	Used        float64             `json:"used"`
	Remaining   float64             `json:"remaining"`
	Total       float64             `json:"total"`
	Unit        string              `json:"unit"`
	ResetAt     string              `json:"reset_at,omitempty"`
	ResetInSec  int                 `json:"reset_in_sec"`
	StatusText  string              `json:"status_text,omitempty"`
	Models      []ollama.ModelUsage `json:"models,omitempty"`
	UsedPct     *float64            `json:"used_pct,omitempty"`
	HeadroomPct *float64            `json:"headroom_pct,omitempty"`
	BurnPerDay  *float64            `json:"burn_per_day,omitempty"`
	DaysToEmpty *float64            `json:"days_to_empty,omitempty"`
}

func mapOllamaQuotas(list []ollama.AccountQuota) []ollamaQuotaDTO {
	out := make([]ollamaQuotaDTO, 0, len(list))
	for _, item := range list {
		windows := make([]ollamaQuotaWindowDTO, 0, len(item.Windows))
		quotaWindows := make([]quota.Window, 0, len(item.Windows))
		for _, window := range item.Windows {
			narrative := quota.DeriveWindowNarrative(window.Used, window.Remaining, window.Total)
			windows = append(windows, ollamaQuotaWindowDTO{
				Label: window.Label, Used: window.Used, Remaining: window.Remaining,
				Total: window.Total, Unit: window.Unit, ResetAt: window.ResetAt,
				ResetInSec: window.ResetInSec, StatusText: window.StatusText, Models: window.Models,
				UsedPct: narrative.UsedPct, HeadroomPct: narrative.HeadroomPct,
				BurnPerDay: narrative.BurnPerDay, DaysToEmpty: narrative.DaysToEmpty,
			})
			quotaWindows = append(quotaWindows, quota.Window{
				Label: window.Label, Used: window.Used, Remaining: window.Remaining,
				Total: window.Total, Unit: window.Unit, ResetInSec: window.ResetInSec,
			})
		}
		dto := ollamaQuotaDTO{
			AccountID: string(item.AccountID), Name: item.Name, Success: item.Success,
			UpdatedAt: item.UpdatedAt, Plan: item.Plan, Windows: windows, Error: item.Error,
		}
		if item.Success {
			label, narrative, note := quota.PickPrimaryNarrative(quotaWindows)
			if narrative.UsedPct != nil || note != "" {
				dto.Narrative = &accountQuotaNarrativeDTO{
					PrimaryLabel: label,
					UsedPct:      narrative.UsedPct,
					HeadroomPct:  narrative.HeadroomPct,
					DaysToEmpty:  narrative.DaysToEmpty,
					Note:         note,
				}
			}
		}
		out = append(out, dto)
	}
	return out
}
