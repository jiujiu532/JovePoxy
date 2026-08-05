package zenpool_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"jovepoxy/internal/crypto"
	"jovepoxy/internal/db"
	"jovepoxy/internal/zen"
	"jovepoxy/internal/zenpool"
)

func newPoolWithMutableClock(t *testing.T) (*zenpool.Service, *mutableClock) {
	t.Helper()
	clock := &mutableClock{now: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}
	database, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	box, err := crypto.NewBox("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("box: %v", err)
	}
	return zenpool.NewService(database, box, clock), clock
}

// TestProvider5xxGuard_freezes_personal_score_after_storm locks the pool-wide
// 5xx storm threshold: after ≥5 fivesxx with ratio ≥0.5 in 60s, further 5xx must
// not lower health_score. 429 still punishes; window expiry lifts the guard.
//
// Two keys share the provider window so personal scores stay above the 0 floor
// while the pool still trips the count threshold (single-key consecutive 5xx
// floors health_score before the 5th sample).
func TestProvider5xxGuard_freezes_personal_score_after_storm(t *testing.T) {
	t.Parallel()

	service, clock := newPoolWithMutableClock(t)
	ctx := context.Background()
	keyA, err := service.Create(ctx, zenpool.CreateInput{Label: "storm-a", Secret: "secret-storm-a"})
	if err != nil {
		t.Fatalf("create a: %v", err)
	}
	keyB, err := service.Create(ctx, zenpool.CreateInput{Label: "storm-b", Secret: "secret-storm-b"})
	if err != nil {
		t.Fatalf("create b: %v", err)
	}
	// Hand-built Selected rows (Acquire RR is nondeterministic with two healthy keys).
	// RecordPaidOutcome only needs ID + Provider for health/guard wiring.
	selA := zenpool.Selected{ID: keyA.ID, Label: "storm-a", Provider: zenpool.ProviderOpenCode}
	selB := zenpool.Selected{ID: keyB.ID, Label: "storm-b", Provider: zenpool.ProviderOpenCode}

	upstream5xx := &zen.StatusError{StatusCode: http.StatusServiceUnavailable}
	// Provider samples: A,A,A,B,B → 5×5xx; personal consecutive stays small.
	for range 3 {
		service.RecordPaidOutcome(ctx, selA, upstream5xx, 10*time.Millisecond)
	}
	for range 2 {
		service.RecordPaidOutcome(ctx, selB, upstream5xx, 10*time.Millisecond)
	}
	itemA := mustListKey(t, service, keyA.ID)
	itemB := mustListKey(t, service, keyB.ID)
	if itemA.HealthScore >= zenpool.DefaultHealthScore || itemB.HealthScore >= zenpool.DefaultHealthScore {
		t.Fatalf("pre-guard 5xx should lower both keys: a=%v b=%v", itemA.HealthScore, itemB.HealthScore)
	}
	if itemA.FailureCount != 3 || itemB.FailureCount != 2 {
		t.Fatalf("pre-guard failures: a=%d b=%d, want 3/2", itemA.FailureCount, itemB.FailureCount)
	}
	frozenA := itemA.HealthScore
	frozenB := itemB.HealthScore

	// Subsequent 5xx: personal score/counters freeze (no 连坐).
	for i := 0; i < 3; i++ {
		service.RecordPaidOutcome(ctx, selA, upstream5xx, 11*time.Millisecond)
		service.RecordPaidOutcome(ctx, selB, upstream5xx, 11*time.Millisecond)
		afterA := mustListKey(t, service, keyA.ID)
		afterB := mustListKey(t, service, keyB.ID)
		if afterA.HealthScore != frozenA || afterA.FailureCount != 3 {
			t.Fatalf("guard-active 5xx on A changed health: score=%v failures=%d (iter %d)", afterA.HealthScore, afterA.FailureCount, i)
		}
		if afterB.HealthScore != frozenB || afterB.FailureCount != 2 {
			t.Fatalf("guard-active 5xx on B changed health: score=%v failures=%d (iter %d)", afterB.HealthScore, afterB.FailureCount, i)
		}
	}

	// 429 still punishes individuals while 5xx guard is active (counters/class/cooldown).
	// Component severity for 429 can be milder than 5xx, so score may not fall further;
	// identity path must still run (unlike guard-frozen 5xx).
	service.RecordPaidOutcome(ctx, selA, &zen.StatusError{StatusCode: http.StatusTooManyRequests}, 12*time.Millisecond)
	after429 := mustListKey(t, service, keyA.ID)
	if after429.FailureCount != 4 {
		t.Fatalf("429 under 5xx guard failure_count = %d, want 4", after429.FailureCount)
	}
	if after429.LastErrorClass != "rate_limited" {
		t.Fatalf("last_error_class = %q, want rate_limited", after429.LastErrorClass)
	}
	if after429.CooldownUntil == nil {
		t.Fatal("429 under 5xx guard should still set cooldown")
	}
	scoreAfter429 := after429.HealthScore

	// Advance past the 60s window so samples expire and the guard lifts.
	clock.now = clock.now.Add(61 * time.Second)
	service.RecordPaidOutcome(ctx, selA, upstream5xx, 13*time.Millisecond)
	afterLift := mustListKey(t, service, keyA.ID)
	if afterLift.FailureCount != 5 {
		t.Fatalf("after window expiry 5xx should punish again: failure_count=%d want 5", afterLift.FailureCount)
	}
	if afterLift.LastErrorClass != "upstream_5xx" {
		t.Fatalf("last_error_class = %q, want upstream_5xx after guard lift", afterLift.LastErrorClass)
	}
	// Score must recompose under 5xx severity (not stay frozen as during guard).
	_ = scoreAfter429
}

// TestProvider5xxGuard_isolated_per_provider ensures OpenCode 5xx storm does not
// freeze Ollama personal scoring (and vice versa).
func TestProvider5xxGuard_isolated_per_provider(t *testing.T) {
	t.Parallel()

	service, _ := newPoolWithMutableClock(t)
	ctx := context.Background()
	oc, err := service.Create(ctx, zenpool.CreateInput{
		Label: "oc", Secret: "secret-oc", Provider: zenpool.ProviderOpenCode,
	})
	if err != nil {
		t.Fatalf("create oc: %v", err)
	}
	ol, err := service.Create(ctx, zenpool.CreateInput{
		Label: "ol", Secret: "secret-ol", Provider: zenpool.ProviderOllama,
	})
	if err != nil {
		t.Fatalf("create ol: %v", err)
	}

	ocSel, err := service.AcquireFor(ctx, zenpool.AcquireOptions{Provider: zenpool.ProviderOpenCode, ForAttempt: true})
	if err != nil {
		t.Fatalf("acquire oc: %v", err)
	}
	olSel, err := service.AcquireFor(ctx, zenpool.AcquireOptions{Provider: zenpool.ProviderOllama, ForAttempt: true})
	if err != nil {
		t.Fatalf("acquire ol: %v", err)
	}
	if ocSel.Provider != zenpool.ProviderOpenCode || olSel.Provider != zenpool.ProviderOllama {
		t.Fatalf("providers: oc=%q ol=%q", ocSel.Provider, olSel.Provider)
	}

	upstream5xx := &zen.StatusError{StatusCode: http.StatusBadGateway}
	// Trip OpenCode guard only.
	for i := 0; i < 5; i++ {
		service.RecordPaidOutcome(ctx, ocSel, upstream5xx, 5*time.Millisecond)
	}
	ocFrozen := mustListKey(t, service, oc.ID).HealthScore
	service.RecordPaidOutcome(ctx, ocSel, upstream5xx, 5*time.Millisecond)
	if mustListKey(t, service, oc.ID).HealthScore != ocFrozen {
		t.Fatal("opencode guard should freeze further 5xx score drops")
	}

	// Ollama has no storm history: first 5xx must still lower its score.
	beforeOL := mustListKey(t, service, ol.ID).HealthScore
	service.RecordPaidOutcome(ctx, olSel, upstream5xx, 5*time.Millisecond)
	afterOL := mustListKey(t, service, ol.ID).HealthScore
	if afterOL >= beforeOL {
		t.Fatalf("ollama 5xx must not inherit opencode guard: before=%v after=%v", beforeOL, afterOL)
	}
}

// TestProvider5xxGuard_ratio_requires_half_attempts ensures sparse 5xx among many
// non-5xx attempts do not activate the freeze (need ratio ≥ 0.5 and count ≥ 5).
func TestProvider5xxGuard_ratio_requires_half_attempts(t *testing.T) {
	t.Parallel()

	service, _ := newPoolWithMutableClock(t)
	ctx := context.Background()
	meta, err := service.Create(ctx, zenpool.CreateInput{Label: "ratio", Secret: "secret-ratio"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	selected, err := service.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	// 5×5xx interleaved with 6 successes → ratio 5/11 < 0.5, guard stays off.
	// Pattern: 5xx, ok, 5xx, ok, ... ending with enough successes.
	upstream5xx := &zen.StatusError{StatusCode: http.StatusInternalServerError}
	for i := 0; i < 5; i++ {
		service.RecordPaidOutcome(ctx, selected, upstream5xx, 4*time.Millisecond)
		service.RecordPaidOutcome(ctx, selected, nil, 4*time.Millisecond)
	}
	// Extra success to keep ratio low if any residual.
	service.RecordPaidOutcome(ctx, selected, nil, 4*time.Millisecond)

	before := mustListKey(t, service, meta.ID).HealthScore
	service.RecordPaidOutcome(ctx, selected, upstream5xx, 4*time.Millisecond)
	after := mustListKey(t, service, meta.ID).HealthScore
	if after >= before {
		t.Fatalf("below-ratio window must still punish 5xx: before=%v after=%v", before, after)
	}
}
