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
	healthCooldownBase    = time.Minute
	healthCooldownMaximum = 16 * time.Minute
	healthDecayInterval   = 5 * time.Minute
	healthDecayFraction   = 0.03
	// healthLatencyEMAAlpha is λ for latency EMA updates (score-model).
	healthLatencyEMAAlpha = 0.12
	healthProbeEvery      = 20

	// Component ranges (persisted health excludes load).
	healthReliabilityMax   = 55.0
	healthSeverityMin      = -25.0
	healthLatencyMax       = 20.0
	healthLatencyNeutral   = 15.0
	healthLatencyFast      = 20.0
	healthLatencyMid       = 12.0
	healthLatencySlow      = 4.0
	healthReliabilityPrior = 0.75 // unused once samples exist; cold-start uses DefaultHealthScore
	healthMinSamplesBeta   = 8
	healthUnauthorizedCap  = 10.0
	healthProbeFloor       = 55.0
)

const (
	healthErrorRateLimited  = "rate_limited"
	healthErrorUpstream5xx  = "upstream_5xx"
	healthErrorUnauthorized = "unauthorized"
)

// SelectionScore derives an ephemeral selection mass from persistent health and
// short-window load. Load is never persisted.
//
//	selection = max(1, round(persisted_health + loadPenalty))
//	loadPenalty ∈ [−15, 0]
//
// A real health_score of 0 is kept (then clamped to minSelectionScore after load
// penalty); cold-start defaults belong in NormalizeHealth, not here.
func SelectionScore(healthScore float64, requests5m, inflight int) int {
	if healthScore < 0 {
		healthScore = 0
	}
	score := int(math.Round(healthScore + loadPenalty(requests5m, inflight)))
	if score < minSelectionScore {
		return minSelectionScore
	}
	return score
}

// loadPenalty returns a non-positive load term for selection only.
//
//	−min(15, 8 × (requests5m/30 + inflight/4))
func loadPenalty(requests5m, inflight int) float64 {
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
	return -penalty
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

// reliabilityRate returns the success-rate prior used for the reliability component.
// samples < 8: Beta(s+2, f+2) mean (s+2)/(s+f+4); else observed s/(s+f).
func reliabilityRate(success, failure int) float64 {
	if success < 0 {
		success = 0
	}
	if failure < 0 {
		failure = 0
	}
	samples := success + failure
	if samples == 0 {
		return healthReliabilityPrior
	}
	if samples < healthMinSamplesBeta {
		return float64(success+2) / float64(success+failure+4)
	}
	return float64(success) / float64(samples)
}

// reliabilityPoints is the 0–55 reliability component.
func reliabilityPoints(success, failure int) float64 {
	return healthReliabilityMax * reliabilityRate(success, failure)
}

// severityPenaltyPoints returns the −25–0 severity component from last failure class
// and consecutive identity failures. Network / empty class → 0.
func severityPenaltyPoints(class string, consecutive int) float64 {
	if class == "" || consecutive < 1 {
		return 0
	}
	step := consecutive
	if step > 5 {
		step = 5
	}
	var magnitude float64
	switch class {
	case healthErrorRateLimited:
		// −(8 + 2×min(c,5))
		magnitude = 8 + 2*float64(step)
	case healthErrorUpstream5xx:
		// −(10 + 3×min(c,5))
		magnitude = 10 + 3*float64(step)
	case healthErrorUnauthorized:
		// Floor at severity minimum (−25).
		magnitude = -healthSeverityMin
	default:
		return 0
	}
	if magnitude > -healthSeverityMin {
		magnitude = -healthSeverityMin
	}
	return -magnitude
}

// latencyPoints is the 0–20 latency component.
// No sample / no pool p50 reference → neutral 15.
// With pool p50: ≤1.2× →20, ≤2× →12, ≤3× →4, else 0.
func latencyPoints(emaMs *float64, poolP50Ms *float64) float64 {
	if emaMs == nil {
		return healthLatencyNeutral
	}
	if poolP50Ms == nil || *poolP50Ms <= 0 {
		return healthLatencyNeutral
	}
	ratio := *emaMs / *poolP50Ms
	switch {
	case ratio <= 1.2:
		return healthLatencyFast
	case ratio <= 2.0:
		return healthLatencyMid
	case ratio <= 3.0:
		return healthLatencySlow
	default:
		return 0
	}
}

// composePersistedHealth builds
//
//	clamp(0, 100, reliability + severityPenalty + latency)
//
// Load is intentionally excluded (selection-only). Cold-start empty rows stay at 70.
// poolP50Ms may be nil → neutral latency band.
func composePersistedHealth(health Health, poolP50Ms *float64) float64 {
	if healthIsColdStart(health) {
		return DefaultHealthScore
	}
	rel := reliabilityPoints(health.SuccessCount, health.FailureCount)
	sev := severityPenaltyPoints(health.LastErrorClass, health.ConsecutiveFailures)
	lat := latencyPoints(health.LatencyEMAMs, poolP50Ms)
	return clampHealth(rel + sev + lat)
}

// healthIsColdStart reports a record with no samples or failure state yet.
func healthIsColdStart(health Health) bool {
	return health.SuccessCount == 0 &&
		health.FailureCount == 0 &&
		health.ConsecutiveFailures == 0 &&
		health.LatencyEMAMs == nil &&
		health.LastErrorClass == "" &&
		health.LastSuccessAt == nil &&
		health.LastFailureAt == nil &&
		health.CooldownReason == ""
}

func updateLatencyEMA(health Health, latency time.Duration) Health {
	if latency <= 0 {
		return health
	}
	value := float64(latency.Milliseconds())
	if value < 1 {
		value = 1
	}
	if health.LatencyEMAMs == nil {
		health.LatencyEMAMs = &value
		return health
	}
	next := (1-healthLatencyEMAAlpha)*(*health.LatencyEMAMs) + healthLatencyEMAAlpha*value
	health.LatencyEMAMs = &next
	return health
}

// updateHealthSuccess applies a 2xx outcome and recomposes persisted health.
// Latency uses neutral band when no pool p50 is available (P0: always neutral).
// Provider-wide 5xx freeze is handled in RecordPaidOutcome, not here.
func updateHealthSuccess(health Health, latency time.Duration, now time.Time) Health {
	return updateHealthSuccessWithP50(health, latency, now, nil)
}

// updateHealthSuccessWithP50 is the testable form with an optional same-provider p50.
func updateHealthSuccessWithP50(health Health, latency time.Duration, now time.Time, poolP50Ms *float64) Health {
	health = NormalizeHealth(health)
	health.SuccessCount++
	health.ConsecutiveFailures = 0
	health.LastErrorClass = ""
	health.CooldownReason = ""
	health = updateLatencyEMA(health, latency)
	timestamp := now.UTC()
	health.LastSuccessAt = &timestamp
	health.ScoreUpdatedAt = timestamp
	health.HealthScore = composePersistedHealth(health, poolP50Ms)
	return health
}

// updateHealthFailure applies an identity failure and recomposes persisted health.
// Pool-wide 5xx storm freeze (no further personal drop) is enforced in RecordPaidOutcome
// via provider5xxGuardActive before this function is called.
func updateHealthFailure(health Health, class string, now time.Time) Health {
	return updateHealthFailureWithP50(health, class, now, nil)
}

// updateHealthFailureWithP50 is the testable form with an optional same-provider p50.
func updateHealthFailureWithP50(health Health, class string, now time.Time, poolP50Ms *float64) Health {
	health = NormalizeHealth(health)
	health.FailureCount++
	health.ConsecutiveFailures++
	health.LastErrorClass = class
	health.CooldownReason = class
	timestamp := now.UTC()
	health.LastFailureAt = &timestamp
	health.ScoreUpdatedAt = timestamp
	health.HealthScore = composePersistedHealth(health, poolP50Ms)
	if class == healthErrorUnauthorized && health.HealthScore > healthUnauthorizedCap {
		health.HealthScore = healthUnauthorizedCap
	}
	return health
}

// decayHealth gently moves an idle score toward the cold-start neutral (70).
// It never jumps to 100. Component recompute on the next outcome overwrites this.
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
