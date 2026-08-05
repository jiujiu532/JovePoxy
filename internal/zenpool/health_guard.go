package zenpool

import (
	"time"
)

// Same-provider 5xx storm guard: when a short window shows enough upstream 5xx
// relative to dial attempts, further personal health_score punishment for 5xx is
// frozen so keys are not collectively zeroed (连坐). Failover may still continue.
//
// Threshold (documented constants):
//   - window: 60s
//   - active when 5xx count >= 5 AND 5xx/attempts >= 0.5
//
// 401/429 still punish individuals. Network errors never punish identity.
const (
	provider5xxWindow   = 60 * time.Second
	provider5xxMinCount = 5
	provider5xxMinRatio = 0.5
)

type providerOutcomeSample struct {
	at    time.Time
	is5xx bool
}

type providerOutcomeWindow struct {
	samples []providerOutcomeSample
}

func (window *providerOutcomeWindow) prune(now time.Time) {
	if window == nil || len(window.samples) == 0 {
		return
	}
	cutoff := now.Add(-provider5xxWindow)
	index := 0
	for index < len(window.samples) && window.samples[index].at.Before(cutoff) {
		index++
	}
	if index > 0 {
		window.samples = append([]providerOutcomeSample(nil), window.samples[index:]...)
	}
}

// noteProviderOutcome records one completed dial attempt for the provider window.
// is5xx should be true only for identity-class upstream 5xx outcomes.
// Caller may hold outcomeMu; this method takes its own guardMu.
func (service *Service) noteProviderOutcome(provider Provider, is5xx bool, now time.Time) {
	if service == nil {
		return
	}
	if provider == "" {
		provider = ProviderOpenCode
	}
	service.guardMu.Lock()
	defer service.guardMu.Unlock()
	if service.providerWindows == nil {
		service.providerWindows = make(map[Provider]*providerOutcomeWindow)
	}
	window := service.providerWindows[provider]
	if window == nil {
		window = &providerOutcomeWindow{}
		service.providerWindows[provider] = window
	}
	window.prune(now.UTC())
	window.samples = append(window.samples, providerOutcomeSample{
		at:    now.UTC(),
		is5xx: is5xx,
	})
}

// provider5xxGuardActive reports whether the provider is under a short-window 5xx storm.
// Uses only prior samples (call before noteProviderOutcome for "subsequent" semantics).
func (service *Service) provider5xxGuardActive(provider Provider, now time.Time) bool {
	if service == nil {
		return false
	}
	if provider == "" {
		provider = ProviderOpenCode
	}
	service.guardMu.Lock()
	defer service.guardMu.Unlock()
	window := service.providerWindows[provider]
	if window == nil {
		return false
	}
	window.prune(now.UTC())
	return isProvider5xxGuardActive(window.samples)
}

// isProvider5xxGuardActive evaluates the threshold on an already-pruned sample slice.
func isProvider5xxGuardActive(samples []providerOutcomeSample) bool {
	if len(samples) == 0 {
		return false
	}
	fives := 0
	for _, sample := range samples {
		if sample.is5xx {
			fives++
		}
	}
	if fives < provider5xxMinCount {
		return false
	}
	return float64(fives)/float64(len(samples)) >= provider5xxMinRatio
}
