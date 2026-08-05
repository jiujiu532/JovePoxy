package zenpool_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"jovepoxy/internal/zen"
	"jovepoxy/internal/zenpool"
)

// TestRecordPaidOutcome_success_throttle_keeps_dirty_visible ensures intermediate
// successes within the 30s write throttle still update List via dirty overlay,
// and recovery after failures always flushes CooldownReason immediately.
func TestRecordPaidOutcome_success_throttle_keeps_dirty_visible(t *testing.T) {
	t.Parallel()
	database, service, _ := newPoolWithDBClock(t)
	ctx := context.Background()
	meta, err := service.Create(ctx, zenpool.CreateInput{Label: "throttle", Secret: "secret-throttle-key"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	selected, err := service.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	// First success flushes (empty lastPersist).
	service.RecordPaidOutcome(ctx, selected, nil, 10*time.Millisecond)
	first := mustListKey(t, service, meta.ID)
	if first.SuccessCount != 1 {
		t.Fatalf("after first success: success_count=%d", first.SuccessCount)
	}
	var persisted int
	if err := database.QueryRowContext(ctx, `SELECT success_count FROM zen_key_health WHERE key_id = ?`, string(meta.ID)).Scan(&persisted); err != nil {
		t.Fatalf("read health: %v", err)
	}
	if persisted != 1 {
		t.Fatalf("persisted success_count=%d, want 1", persisted)
	}

	// Second success within throttle stays dirty in memory but visible on List.
	service.RecordPaidOutcome(ctx, selected, nil, 11*time.Millisecond)
	second := mustListKey(t, service, meta.ID)
	if second.SuccessCount != 2 {
		t.Fatalf("list success_count=%d, want 2 from dirty overlay", second.SuccessCount)
	}
	if err := database.QueryRowContext(ctx, `SELECT success_count FROM zen_key_health WHERE key_id = ?`, string(meta.ID)).Scan(&persisted); err != nil {
		t.Fatalf("read health after dirty: %v", err)
	}
	if persisted != 1 {
		t.Fatalf("sqlite success_count=%d, want still 1 while throttled", persisted)
	}
	if second.HealthScore <= first.HealthScore {
		t.Fatalf("dirty health_score did not rise: first=%v second=%v", first.HealthScore, second.HealthScore)
	}

	// Identity failure after dirty success must merge dirty counters then flush.
	service.RecordPaidOutcome(ctx, selected, &zen.StatusError{StatusCode: http.StatusTooManyRequests}, 5*time.Millisecond)
	failed := mustListKey(t, service, meta.ID)
	if failed.SuccessCount != 2 || failed.FailureCount != 1 {
		t.Fatalf("after fail: success=%d failure=%d", failed.SuccessCount, failed.FailureCount)
	}
	if failed.CooldownReason == "" || failed.CooldownUntil == nil {
		t.Fatalf("expected cooling after 429: reason=%q until=%v", failed.CooldownReason, failed.CooldownUntil)
	}

	// Clear cooldown so recovery can clear reason; recovery must flush even inside throttle window.
	if _, err := database.ExecContext(ctx, `UPDATE zen_keys SET cooldown_until = NULL WHERE id = ?`, string(meta.ID)); err != nil {
		t.Fatalf("clear cooldown: %v", err)
	}
	service.RecordPaidOutcome(ctx, selected, nil, 20*time.Millisecond)
	recovered := mustListKey(t, service, meta.ID)
	if recovered.CooldownReason != "" || recovered.ConsecutiveFailures != 0 {
		t.Fatalf("recovery not visible: reason=%q consecutive=%d", recovered.CooldownReason, recovered.ConsecutiveFailures)
	}
	if recovered.SuccessCount != 3 {
		t.Fatalf("success_count=%d, want 3", recovered.SuccessCount)
	}
	var reason string
	var consecutive int
	if err := database.QueryRowContext(ctx, `
		SELECT cooldown_reason, consecutive_failures FROM zen_key_health WHERE key_id = ?
	`, string(meta.ID)).Scan(&reason, &consecutive); err != nil {
		t.Fatalf("read recovery health: %v", err)
	}
	if reason != "" || consecutive != 0 {
		t.Fatalf("recovery not flushed: reason=%q consecutive=%d", reason, consecutive)
	}
}
