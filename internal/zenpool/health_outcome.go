package zenpool

import (
	"context"
	"errors"
	"net/http"
	"time"

	"jovepoxy/internal/zen"
)

func classifyHealthFailure(err error) string {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ""
	}
	var status *zen.StatusError
	if errors.As(err, &status) {
		switch {
		case status.StatusCode == http.StatusUnauthorized:
			return healthErrorUnauthorized
		case status.StatusCode == http.StatusTooManyRequests:
			return healthErrorRateLimited
		case status.StatusCode >= http.StatusInternalServerError:
			return healthErrorUpstream5xx
		default:
			return ""
		}
	}
	return ""
}

// RecordPaidOutcome updates only secret-free health facts for one paid attempt.
// Context cancellation, client-side 4xx, and network failures do not punish Key identity.
// When the same provider hits a short-window 5xx storm, further 5xx outcomes do not
// lower personal health_score or stack long cooldowns (failover may still continue).
func (service *Service) RecordPaidOutcome(ctx context.Context, selected Selected, err error, latency time.Duration) {
	if service == nil || selected.ID == "" {
		return
	}
	defer service.noteRelease(selected.ID)
	if ctx.Err() != nil {
		return
	}
	service.outcomeMu.Lock()
	defer service.outcomeMu.Unlock()
	now := service.clock.Now().UTC()
	provider := selected.Provider
	if provider == "" {
		provider = ProviderOpenCode
	}
	health, loadErr := service.store.GetHealth(ctx, selected.ID)
	if loadErr != nil {
		return
	}
	// Apply any unflushed success updates before scoring this outcome.
	health = service.overlayDirtyHealth(selected.ID, health)
	health = decayHealth(health, now)
	if err == nil {
		service.noteProviderOutcome(provider, false, now)
		// Recovery / probe transitions must flush immediately; pure success may throttle.
		prevReason := health.CooldownReason
		prevConsecutive := health.ConsecutiveFailures
		// Componentized score: reliability + severity + latency (load is selection-only).
		// P0 latency uses neutral band (no pool p50 wired yet).
		health = updateHealthSuccess(health, latency, now)
		if selected.Probing && health.HealthScore < healthProbeFloor {
			health.HealthScore = healthProbeFloor
		}
		_ = service.store.SetCooldown(ctx, selected.ID, nil)
		forcePersist := selected.Probing || prevReason != "" || prevConsecutive > 0
		if forcePersist || service.shouldPersistSuccess(selected.ID, now) {
			service.persistHealth(ctx, health)
		} else {
			// Keep intermediate successes visible until the next flush window.
			service.rememberDirtyHealth(selected.ID, health)
		}
		return
	}

	class := classifyHealthFailure(err)
	if class == "" {
		// Network / non-identity errors still count as dial attempts for the storm ratio.
		service.noteProviderOutcome(provider, false, now)
		return
	}
	is5xx := class == healthErrorUpstream5xx
	// Guard uses prior samples only: the event that trips the threshold still punishes;
	// subsequent 5xx skip personal score/cooldown so the pool is not collectively zeroed.
	// Failover continues at the ProxyPaid layer.
	// Hook: provider5xxGuardActive is implemented in health_guard.go.
	if is5xx && service.provider5xxGuardActive(provider, now) {
		service.noteProviderOutcome(provider, true, now)
		return
	}
	service.noteProviderOutcome(provider, is5xx, now)
	health = updateHealthFailure(health, class, now)
	if class == healthErrorUnauthorized {
		// duration 0 → service BenchDuration() (admin/env configurable, default 10m)
		service.MarkBench(selected.ID, 0)
	} else {
		until := now.Add(cooldownForFailures(health.ConsecutiveFailures))
		if setErr := service.store.SetCooldown(ctx, selected.ID, &until); setErr != nil {
			// SQLite cooldown failed: still rest the key in-process so Acquire skips it.
			service.MarkBench(selected.ID, DefaultCooldown)
		}
	}
	// Failures always flush (or stay dirty if SQLite rejects the write).
	service.persistHealth(ctx, health)
}
