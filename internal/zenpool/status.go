package zenpool

import "time"

// KeyStatus is the admin-facing lifecycle of a pool key.
type KeyStatus string

const (
	// StatusActive is enabled and not in cooldown.
	StatusActive KeyStatus = "active"
	// StatusCooling is enabled but resting until CooldownUntil.
	StatusCooling KeyStatus = "cooling"
	// StatusDisabled is not eligible for selection.
	StatusDisabled KeyStatus = "disabled"
)

// ProviderSummary is healthy/cooled counts for one provider pool.
type ProviderSummary struct {
	Total    int `json:"total"`
	Enabled  int `json:"enabled"`
	Healthy  int `json:"healthy"`
	Cooled   int `json:"cooled"`
	Disabled int `json:"disabled"`
}

// PoolSummary aggregates secret-free pool health for overview / list.
type PoolSummary struct {
	Total      int                         `json:"total"`
	Enabled    int                         `json:"enabled"`
	Healthy    int                         `json:"healthy"`
	Cooled     int                         `json:"cooled"`
	Disabled   int                         `json:"disabled"`
	ByProvider map[string]ProviderSummary  `json:"by_provider,omitempty"`
}

// KeyView is Metadata plus pure display fields for admin DTOs.
type KeyView struct {
	Metadata
	Status               KeyStatus
	TrafficPct           float64
	CooldownRemainingSec int
}

// DeriveStatus classifies a key for admin surfaces.
func DeriveStatus(meta Metadata, now time.Time) KeyStatus {
	if !meta.Enabled {
		return StatusDisabled
	}
	if isCoolingMeta(meta.CooldownUntil, now) {
		return StatusCooling
	}
	return StatusActive
}

// CooldownRemainingSec returns max(0, until-now) in whole seconds.
func CooldownRemainingSec(meta Metadata, now time.Time) int {
	if meta.CooldownUntil == nil {
		return 0
	}
	remaining := meta.CooldownUntil.Sub(now)
	if remaining <= 0 {
		return 0
	}
	// Round up partial seconds so UI does not flash "0s" while still cooling.
	sec := int(remaining.Seconds())
	if remaining > time.Duration(sec)*time.Second {
		sec++
	}
	return sec
}

// TrafficShares returns per-index traffic percentages for one provider pool.
//
// Aligns with AcquireExcluding selection:
//   - eligible = enabled && not cooling && weight > 0
//   - traffic_pct(k) = weight(k)/totalWeight*100 for eligible, else 0
//   - if totalWeight == 0 (all disabled/cooling/zero-weight): all 0
//
// Callers that hold mixed providers must group first; shares are relative to
// the given list only.
func TrafficShares(list []Metadata, now time.Time) []float64 {
	out := make([]float64, len(list))
	if len(list) == 0 {
		return out
	}
	totalWeight := 0
	for _, meta := range list {
		if !isTrafficEligible(meta, now) {
			continue
		}
		totalWeight += meta.Weight
	}
	if totalWeight == 0 {
		return out
	}
	for i, meta := range list {
		if !isTrafficEligible(meta, now) {
			continue
		}
		out[i] = float64(meta.Weight) / float64(totalWeight) * 100
	}
	return out
}

// DeriveViews attaches status, remaining cooldown, and per-provider traffic %.
func DeriveViews(list []Metadata, now time.Time) []KeyView {
	views := make([]KeyView, len(list))
	// Group indices by provider so traffic % is within each pool.
	groups := make(map[Provider][]int, 2)
	for i, meta := range list {
		provider := meta.Provider
		if provider == "" {
			provider = ProviderOpenCode
		}
		groups[provider] = append(groups[provider], i)
		views[i] = KeyView{
			Metadata:             meta,
			Status:               DeriveStatus(meta, now),
			CooldownRemainingSec: CooldownRemainingSec(meta, now),
		}
		// Normalize empty provider for DTO consumers.
		if views[i].Provider == "" {
			views[i].Provider = ProviderOpenCode
		}
	}
	for _, indices := range groups {
		subset := make([]Metadata, len(indices))
		for j, idx := range indices {
			subset[j] = list[idx]
		}
		shares := TrafficShares(subset, now)
		for j, idx := range indices {
			views[idx].TrafficPct = shares[j]
		}
	}
	return views
}

// Summarize builds pool health counters (no secrets).
func Summarize(list []Metadata, now time.Time) PoolSummary {
	summary := PoolSummary{
		ByProvider: map[string]ProviderSummary{},
	}
	for _, meta := range list {
		provider := string(meta.Provider)
		if provider == "" {
			provider = string(ProviderOpenCode)
		}
		ps := summary.ByProvider[provider]
		ps.Total++
		summary.Total++

		status := DeriveStatus(meta, now)
		switch status {
		case StatusDisabled:
			ps.Disabled++
			summary.Disabled++
		case StatusCooling:
			ps.Enabled++
			ps.Cooled++
			summary.Enabled++
			summary.Cooled++
		default: // active
			ps.Enabled++
			ps.Healthy++
			summary.Enabled++
			summary.Healthy++
		}
		summary.ByProvider[provider] = ps
	}
	if len(summary.ByProvider) == 0 {
		summary.ByProvider = nil
	}
	return summary
}

func isTrafficEligible(meta Metadata, now time.Time) bool {
	if !meta.Enabled || meta.Weight <= 0 {
		return false
	}
	return !isCoolingMeta(meta.CooldownUntil, now)
}

func isCoolingMeta(until *time.Time, now time.Time) bool {
	if until == nil {
		return false
	}
	return now.Before(*until)
}
