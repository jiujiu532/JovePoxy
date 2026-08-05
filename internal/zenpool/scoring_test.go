package zenpool

import (
	"math"
	"testing"
	"time"
)

func TestComposePersistedHealth_coldStart(t *testing.T) {
	t.Parallel()
	h := ColdStartHealth("zk_new")
	if got := composePersistedHealth(h, nil); got != DefaultHealthScore {
		t.Fatalf("cold-start score = %v, want %v", got, DefaultHealthScore)
	}
	if got := composePersistedHealth(NormalizeHealth(Health{}), nil); got != DefaultHealthScore {
		t.Fatalf("empty normalized = %v, want %v", got, DefaultHealthScore)
	}
}

func TestReliability_fewSamplesBetaNotFullAfterOneSuccess(t *testing.T) {
	t.Parallel()
	// One success, zero failures: Beta (1+2)/(1+0+4) = 0.6 → reliability 33.
	rate := reliabilityRate(1, 0)
	if math.Abs(rate-0.6) > 1e-9 {
		t.Fatalf("rate = %v, want 0.6", rate)
	}
	rel := reliabilityPoints(1, 0)
	if math.Abs(rel-33) > 1e-9 {
		t.Fatalf("reliability = %v, want 33", rel)
	}
	// Composed with neutral latency 15, severity 0 → 48, never 100.
	h := Health{KeyID: "k", SuccessCount: 1, FailureCount: 0}
	score := composePersistedHealth(h, nil)
	if score >= 100 || score > 55+healthLatencyNeutral {
		t.Fatalf("one-success score = %v, must not be near 100", score)
	}
	if math.Abs(score-(33+healthLatencyNeutral)) > 1e-9 {
		t.Fatalf("one-success score = %v, want 48", score)
	}
}

func TestReliability_observedAfterMinSamples(t *testing.T) {
	t.Parallel()
	// 19/20 successes, samples ≥ 8 → observed 0.95 → 52.25
	rate := reliabilityRate(19, 1)
	if math.Abs(rate-0.95) > 1e-9 {
		t.Fatalf("rate = %v, want 0.95", rate)
	}
	rel := reliabilityPoints(19, 1)
	if math.Abs(rel-52.25) > 1e-9 {
		t.Fatalf("reliability = %v, want 52.25", rel)
	}
}

func TestSeverity_three429sAnd401Floor(t *testing.T) {
	t.Parallel()
	// Three 429s: magnitude 8+2*3 = 14 → −14
	if got := severityPenaltyPoints(healthErrorRateLimited, 3); got != -14 {
		t.Fatalf("429×3 severity = %v, want -14", got)
	}
	// Cap at −25: 5xx with consecutive 5 → 10+3*5 = 25 → −25
	if got := severityPenaltyPoints(healthErrorUpstream5xx, 5); got != -25 {
		t.Fatalf("5xx×5 severity = %v, want -25", got)
	}
	// 401 floors severity at −25
	if got := severityPenaltyPoints(healthErrorUnauthorized, 1); got != -25 {
		t.Fatalf("401 severity = %v, want -25", got)
	}
	// Network / empty → 0
	if got := severityPenaltyPoints("", 3); got != 0 {
		t.Fatalf("empty class severity = %v", got)
	}
}

func TestUpdateHealthFailure_three429sLowerScoreSubstantially(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	h := ColdStartHealth("zk_c")
	for range 3 {
		h = updateHealthFailure(h, healthErrorRateLimited, now)
		now = now.Add(time.Second)
	}
	// s=0,f=3 → Beta 2/7 ≈ 0.2857 → rel ≈ 15.71; sev −14; lat 15 → ≈ 16.71
	if h.HealthScore >= DefaultHealthScore {
		t.Fatalf("after 3×429 score = %v, want well below cold-start", h.HealthScore)
	}
	if h.HealthScore > 25 {
		t.Fatalf("after 3×429 score = %v, want roughly ~17 (score-model example)", h.HealthScore)
	}
	if h.ConsecutiveFailures != 3 || h.FailureCount != 3 {
		t.Fatalf("counters failure=%d consecutive=%d", h.FailureCount, h.ConsecutiveFailures)
	}
	if h.LastErrorClass != healthErrorRateLimited || h.CooldownReason != healthErrorRateLimited {
		t.Fatalf("class/reason = %q/%q", h.LastErrorClass, h.CooldownReason)
	}
	// Cooldown duration for c=3: 60 * 2^2 = 240s
	if d := cooldownForFailures(3); d != 4*time.Minute {
		t.Fatalf("cooldown = %v, want 4m", d)
	}
}

func TestUpdateHealthFailure_401CapsAtTen(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	h := updateHealthFailure(ColdStartHealth("zk_u"), healthErrorUnauthorized, now)
	if h.HealthScore > healthUnauthorizedCap {
		t.Fatalf("401 score = %v, want ≤ %v", h.HealthScore, healthUnauthorizedCap)
	}
	if h.LastErrorClass != healthErrorUnauthorized {
		t.Fatalf("class = %q", h.LastErrorClass)
	}
}

func TestLatencyPoints_bandsAndNeutral(t *testing.T) {
	t.Parallel()
	if got := latencyPoints(nil, nil); got != healthLatencyNeutral {
		t.Fatalf("nil ema = %v, want neutral %v", got, healthLatencyNeutral)
	}
	ema := 100.0
	if got := latencyPoints(&ema, nil); got != healthLatencyNeutral {
		t.Fatalf("no p50 = %v, want neutral", got)
	}
	p50 := 100.0
	cases := []struct {
		ema  float64
		want float64
	}{
		{100, healthLatencyFast}, // 1.0×
		{120, healthLatencyFast}, // 1.2×
		{121, healthLatencyMid},  // just over 1.2×
		{200, healthLatencyMid},  // 2.0×
		{201, healthLatencySlow}, // just over 2×
		{300, healthLatencySlow}, // 3.0×
		{301, 0},                 // >3×
	}
	for _, tc := range cases {
		ema := tc.ema
		if got := latencyPoints(&ema, &p50); got != tc.want {
			t.Fatalf("ema=%v p50=%v → %v, want %v", tc.ema, p50, got, tc.want)
		}
	}
}

func TestComposePersistedHealth_latencyBandWithP50(t *testing.T) {
	t.Parallel()
	// Perfect reliability observed: 16 success / 0 fail → rate 1 → rel 55
	// Fast latency vs p50 → +20 → score 75
	ema := 50.0
	p50 := 100.0
	h := Health{KeyID: "k", SuccessCount: 16, FailureCount: 0, LatencyEMAMs: &ema}
	got := composePersistedHealth(h, &p50)
	if math.Abs(got-75) > 1e-9 {
		t.Fatalf("score = %v, want 75 (55+20)", got)
	}
	// Same key without p50 → neutral 15 → 70
	got = composePersistedHealth(h, nil)
	if math.Abs(got-70) > 1e-9 {
		t.Fatalf("score without p50 = %v, want 70", got)
	}
}

func TestSelectionScore_loadPenaltyOnly(t *testing.T) {
	t.Parallel()
	// No load: selection == round(health)
	if got := SelectionScore(70, 0, 0); got != 70 {
		t.Fatalf("no load = %d, want 70", got)
	}
	// requests5m=30, inflight=0 → penalty 8*(1+0)=8 → 70-8=62
	if got := SelectionScore(70, 30, 0); got != 62 {
		t.Fatalf("rpm load = %d, want 62", got)
	}
	// requests=0, inflight=4 → penalty 8 → 62
	if got := SelectionScore(70, 0, 4); got != 62 {
		t.Fatalf("inflight load = %d, want 62", got)
	}
	// Cap at 15: large load → 70-15=55
	if got := SelectionScore(70, 1000, 100); got != 55 {
		t.Fatalf("capped load = %d, want 55", got)
	}
	// Explicit health 0 + load still ≥ 1
	if got := SelectionScore(0, 30, 4); got != 1 {
		t.Fatalf("zero health = %d, want 1", got)
	}
	// loadPenalty helper is non-positive
	if p := loadPenalty(30, 0); p != -8 {
		t.Fatalf("loadPenalty = %v, want -8", p)
	}
}

func TestUpdateHealthSuccess_raisesAfterFailuresAndClearsSeverity(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	h := ColdStartHealth("zk_r")
	h = updateHealthFailure(h, healthErrorRateLimited, now)
	h = updateHealthFailure(h, healthErrorRateLimited, now.Add(time.Second))
	failed := h.HealthScore
	if failed >= DefaultHealthScore {
		t.Fatalf("failed score = %v", failed)
	}
	h = updateHealthSuccess(h, 25*time.Millisecond, now.Add(2*time.Second))
	if h.HealthScore <= failed {
		t.Fatalf("success did not raise score: before=%v after=%v", failed, h.HealthScore)
	}
	if h.ConsecutiveFailures != 0 || h.LastErrorClass != "" || h.CooldownReason != "" {
		t.Fatalf("failure state not cleared: %+v", h)
	}
	if h.SuccessCount != 1 || h.FailureCount != 2 {
		t.Fatalf("counters success=%d failure=%d", h.SuccessCount, h.FailureCount)
	}
	if h.LatencyEMAMs == nil {
		t.Fatal("expected latency EMA after success")
	}
}

func TestUpdateHealthSuccessWithP50_usesLatencyBand(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	// Build enough samples so reliability is high, then a slow latency vs p50.
	h := Health{KeyID: "zk_lat", SuccessCount: 15, FailureCount: 0}
	p50 := 50.0
	// One more success at 200ms (~4× p50) → slow/zero latency band.
	h = updateHealthSuccessWithP50(h, 200*time.Millisecond, now, &p50)
	// s=16 → rel 55; ema≈200; ratio 4 → lat 0 → score 55
	if math.Abs(h.HealthScore-55) > 1e-6 {
		t.Fatalf("slow latency score = %v, want 55", h.HealthScore)
	}
	// Neutral (nil p50) path on same counts/ema should be higher.
	neutral := composePersistedHealth(h, nil)
	if neutral <= h.HealthScore {
		t.Fatalf("neutral latency should score higher than slow: neutral=%v slow=%v", neutral, h.HealthScore)
	}
}

func TestDecayHealth_movesTowardNeutralNotHundred(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	h := Health{
		KeyID: "zk_d", HealthScore: 20, SuccessCount: 1, FailureCount: 5,
		ScoreUpdatedAt: now,
	}
	later := now.Add(healthDecayInterval * 2)
	h = decayHealth(h, later)
	if h.HealthScore <= 20 {
		t.Fatalf("decay should rise toward 70: got %v", h.HealthScore)
	}
	if h.HealthScore >= 100 {
		t.Fatalf("decay must never jump to 100: got %v", h.HealthScore)
	}
	if h.HealthScore >= DefaultHealthScore {
		t.Fatalf("two decay steps from 20 should stay below 70: got %v", h.HealthScore)
	}
}
