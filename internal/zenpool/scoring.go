package zenpool

import (
	"math"
	"time"
)

const (
	minSelectionScore     = 1
	maxSelectionShare     = 0.70
	healthLoadWindow      = 5 * time.Minute
	healthLoadRefRequests = 30
	healthLoadRefInflight = 4
	healthMaxLoadPenalty  = 15.0
	healthSuccessStep     = 2.0
	healthCooldownBase    = time.Minute
	healthCooldownMaximum = 16 * time.Minute
	healthDecayInterval   = 5 * time.Minute
	healthDecayFraction   = 0.03
	healthLatencyEMAAlpha = 0.12
	healthProbeEvery      = 20
)

const (
	healthErrorRateLimited  = "rate_limited"
	healthErrorUpstream5xx  = "upstream_5xx"
	healthErrorUnauthorized = "unauthorized"
)

// SelectionScore derives an ephemeral selection mass from persistent health and
// short-window load. It is never persisted because load is process-local.
// A real health_score of 0 is kept (then clamped to minSelectionScore after load
// penalty); cold-start defaults belong in NormalizeHealth, not here.
func SelectionScore(healthScore float64, requests5m, inflight int) int {
	if healthScore < 0 {
		healthScore = 0
	}
	if requests5m < 0 {
		requests5m = 0
	}
	if inflight < 0 {
		inflight = 0
	}
	penalty := 8 * (float64(requests5m)/healthLoadRefRequests + float64(inflight)/healthLoadRefInflight)
	if penalty > healthMaxLoadPenalty {
		penalty = healthMaxLoadPenalty
	}
	score := int(math.Round(healthScore - penalty))
	if score < minSelectionScore {
		return minSelectionScore
	}
	return score
}

func cooldownForFailures(consecutive int) time.Duration {
	if consecutive < 1 {
		consecutive = 1
	}
	exponent := consecutive - 1
	if exponent > 4 {
		exponent = 4
	}
	cooldown := healthCooldownBase * time.Duration(1<<uint(exponent))
	if cooldown > healthCooldownMaximum {
		return healthCooldownMaximum
	}
	return cooldown
}

func failurePenalty(class string, consecutive int) float64 {
	if consecutive < 1 {
		consecutive = 1
	}
	step := consecutive
	if step > 5 {
		step = 5
	}
	switch class {
	case healthErrorRateLimited:
		return 8 + 2*float64(step)
	case healthErrorUpstream5xx:
		return 10 + 3*float64(step)
	case healthErrorUnauthorized:
		return 90
	default:
		return 0
	}
}

func updateHealthSuccess(health Health, latency time.Duration, now time.Time) Health {
	health = NormalizeHealth(health)
	health.SuccessCount++
	health.ConsecutiveFailures = 0
	health.LastErrorClass = ""
	health.CooldownReason = ""
	health.HealthScore = clampHealth(health.HealthScore + healthSuccessStep)
	if latency > 0 {
		value := float64(latency.Milliseconds())
		if value < 1 {
			value = 1
		}
		if health.LatencyEMAMs == nil {
			health.LatencyEMAMs = &value
		} else {
			next := (1-healthLatencyEMAAlpha)*(*health.LatencyEMAMs) + healthLatencyEMAAlpha*value
			health.LatencyEMAMs = &next
		}
	}
	timestamp := now.UTC()
	health.LastSuccessAt = &timestamp
	health.ScoreUpdatedAt = timestamp
	return health
}

func updateHealthFailure(health Health, class string, now time.Time) Health {
	health = NormalizeHealth(health)
	health.FailureCount++
	health.ConsecutiveFailures++
	health.LastErrorClass = class
	health.CooldownReason = class
	health.HealthScore = clampHealth(health.HealthScore - failurePenalty(class, health.ConsecutiveFailures))
	if class == healthErrorUnauthorized && health.HealthScore > 10 {
		health.HealthScore = 10
	}
	timestamp := now.UTC()
	health.LastFailureAt = &timestamp
	health.ScoreUpdatedAt = timestamp
	return health
}

func decayHealth(health Health, now time.Time) Health {
	health = NormalizeHealth(health)
	if health.ScoreUpdatedAt.IsZero() {
		return health
	}
	elapsed := now.UTC().Sub(health.ScoreUpdatedAt)
	if elapsed < healthDecayInterval {
		return health
	}
	steps := int(elapsed / healthDecayInterval)
	for range steps {
		health.HealthScore += (DefaultHealthScore - health.HealthScore) * healthDecayFraction
	}
	health.HealthScore = clampHealth(health.HealthScore)
	health.ScoreUpdatedAt = health.ScoreUpdatedAt.Add(time.Duration(steps) * healthDecayInterval)
	return health
}

func clampHealth(score float64) float64 {
	return math.Max(0, math.Min(100, score))
}
