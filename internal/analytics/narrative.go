package analytics

import "jovepoxy/internal/quota"

// BuildQuotaNarrative derives overview-level headroom story from live windows.
// Does not invent burn/days; worst used% is the max used_pct across windows with total>0.
func BuildQuotaNarrative(windows []quota.Window, effectiveRemaining float64) *QuotaNarrative {
	if len(windows) == 0 {
		return &QuotaNarrative{
			EffectiveRemaining: effectiveRemaining,
			Note:               "sample_insufficient",
		}
	}
	var worstUsed *float64
	var headroom *float64
	for _, window := range windows {
		derived := quota.DeriveWindowNarrative(window.Used, window.Remaining, window.Total)
		if derived.UsedPct == nil {
			continue
		}
		if worstUsed == nil || *derived.UsedPct > *worstUsed {
			usedCopy := *derived.UsedPct
			worstUsed = &usedCopy
			if derived.HeadroomPct != nil {
				headCopy := *derived.HeadroomPct
				headroom = &headCopy
			}
		}
	}
	if worstUsed == nil {
		return &QuotaNarrative{
			EffectiveRemaining: effectiveRemaining,
			Note:               "sample_insufficient",
		}
	}
	return &QuotaNarrative{
		EffectiveRemaining: effectiveRemaining,
		WorstUsedPct:       worstUsed,
		HeadroomPct:        headroom,
	}
}
