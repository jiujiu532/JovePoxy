package analytics

import "jovepoxy/internal/quota"

// CascadedWindow is a quota window with cascade display fields.
type CascadedWindow struct {
	Label              string  `json:"label"`
	Used               float64 `json:"used"`
	Remaining          float64 `json:"remaining"`
	Total              float64 `json:"total"`
	Unit               string  `json:"unit"`
	ResetInSec         int     `json:"reset_in_sec"`
	Blocked            bool    `json:"blocked"`
	BlockedBy          string  `json:"blocked_by,omitempty"`
	EffectiveRemaining float64 `json:"effective_remaining"`
}

// ApplyOpenCodeCascade marks lower windows blocked when higher windows are full.
func ApplyOpenCodeCascade(windows []quota.Window) []CascadedWindow {
	monthly := windowByLabel(windows, quota.LabelMonthly)
	weekly := windowByLabel(windows, quota.LabelWeekly)
	monthlyFull := monthly != nil && monthly.Used >= 100
	weeklyFull := weekly != nil && weekly.Used >= 100

	out := make([]CascadedWindow, 0, len(windows))
	for _, window := range windows {
		item := CascadedWindow{
			Label: window.Label, Used: window.Used, Remaining: window.Remaining,
			Total: window.Total, Unit: window.Unit, ResetInSec: window.ResetInSec,
			EffectiveRemaining: window.Remaining,
		}
		switch window.Label {
		case quota.LabelWeekly:
			if monthlyFull {
				item.Blocked = true
				item.BlockedBy = quota.LabelMonthly
				item.EffectiveRemaining = 0
			}
		case quota.LabelRolling:
			if monthlyFull || weeklyFull {
				item.Blocked = true
				item.BlockedBy = quota.LabelMonthly
				if !monthlyFull {
					item.BlockedBy = quota.LabelWeekly
				}
				item.EffectiveRemaining = 0
			}
		}
		out = append(out, item)
	}
	return out
}

// EffectiveRemaining returns the display remaining after cascade priority.
func EffectiveRemaining(windows []quota.Window) float64 {
	cascaded := ApplyOpenCodeCascade(windows)
	for _, label := range []string{quota.LabelRolling, quota.LabelWeekly, quota.LabelMonthly} {
		for _, window := range cascaded {
			if window.Label == label {
				return window.EffectiveRemaining
			}
		}
	}
	return 0
}

func windowByLabel(windows []quota.Window, label string) *quota.Window {
	for index := range windows {
		if windows[index].Label == label {
			return &windows[index]
		}
	}
	return nil
}
