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
	health, loadErr := service.store.GetHealth(ctx, selected.ID)
	if loadErr != nil {
		return
	}
	// Apply any unflushed success updates before scoring this outcome.
	health = service.overlayDirtyHealth(selected.ID, health)
	health = decayHealth(health, now)
	if err == nil {
		// Recovery / probe transitions must flush immediately; pure success may throttle.
		prevReason := health.CooldownReason
		prevConsecutive := health.ConsecutiveFailures
		health = updateHealthSuccess(health, latency, now)
		if selected.Probing && health.HealthScore < 55 {
			health.HealthScore = 55
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
		return
	}
	health = updateHealthFailure(health, class, now)
	if class == healthErrorUnauthorized {
		service.MarkBench(selected.ID, DefaultBenchDuration)
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
