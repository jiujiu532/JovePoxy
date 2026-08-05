package zenpool_test

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"jovepoxy/internal/crypto"
	"jovepoxy/internal/db"
	"jovepoxy/internal/zen"
	"jovepoxy/internal/zenpool"
)

// TestRecordPaidOutcome_429_and_5xx_lower_score_and_persist_cooldown locks the
// identity-failure path: rate-limit / upstream 5xx must drop health_score,
// increment failure counters, stamp error class, and persist cooldown_until.
func TestRecordPaidOutcome_429_and_5xx_lower_score_and_persist_cooldown(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		status    int
		wantClass string
	}{
		{name: "429", status: http.StatusTooManyRequests, wantClass: "rate_limited"},
		{name: "503", status: http.StatusServiceUnavailable, wantClass: "upstream_5xx"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			database, service, clock := newPoolWithDBClock(t)
			ctx := context.Background()
			meta, err := service.Create(ctx, zenpool.CreateInput{Label: "paid", Secret: "secret-paid-key"})
			if err != nil {
				t.Fatalf("create: %v", err)
			}

			selected, err := service.Acquire(ctx)
			if err != nil {
				t.Fatalf("acquire: %v", err)
			}
			if selected.ID != meta.ID {
				t.Fatalf("selected %s, want %s", selected.ID, meta.ID)
			}

			// Act: one identity failure of the given class.
			service.RecordPaidOutcome(ctx, selected, &zen.StatusError{StatusCode: tc.status}, 12*time.Millisecond)

			// Assert via admin-facing list (secret-free).
			item := mustListKey(t, service, meta.ID)
			if item.HealthScore >= zenpool.DefaultHealthScore {
				t.Fatalf("health_score = %v, want below cold-start %v", item.HealthScore, zenpool.DefaultHealthScore)
			}
			if item.FailureCount != 1 {
				t.Fatalf("failure_count = %d, want 1", item.FailureCount)
			}
			if item.ConsecutiveFailures != 1 {
				t.Fatalf("consecutive_failures = %d, want 1", item.ConsecutiveFailures)
			}
			if item.SuccessCount != 0 {
				t.Fatalf("success_count = %d, want 0", item.SuccessCount)
			}
			if item.LastErrorClass != tc.wantClass {
				t.Fatalf("last_error_class = %q, want %q", item.LastErrorClass, tc.wantClass)
			}
			if item.CooldownReason != tc.wantClass {
				t.Fatalf("cooldown_reason = %q, want %q", item.CooldownReason, tc.wantClass)
			}
			if item.CooldownUntil == nil {
				t.Fatal("cooldown_until is nil, want persisted cooling window")
			}
			if !item.CooldownUntil.After(clock.Now()) {
				t.Fatalf("cooldown_until %v not after now %v", item.CooldownUntil, clock.Now())
			}
			if item.LastFailureAt == nil {
				t.Fatal("last_failure_at is nil")
			}

			// Assert persistence in zen_key_health (not just memory).
			var score float64
			var failures, consecutive int
			var class, reason string
			err = database.QueryRowContext(ctx, `
				SELECT health_score, failure_count, consecutive_failures, last_error_class, cooldown_reason
				FROM zen_key_health WHERE key_id = ?
			`, string(meta.ID)).Scan(&score, &failures, &consecutive, &class, &reason)
			if err != nil {
				t.Fatalf("read zen_key_health: %v", err)
			}
			if score != item.HealthScore || failures != 1 || consecutive != 1 || class != tc.wantClass || reason != tc.wantClass {
				t.Fatalf("persisted health mismatch: score=%v failures=%d consecutive=%d class=%q reason=%q",
					score, failures, consecutive, class, reason)
			}

			var cooldown sql.NullString
			if err := database.QueryRowContext(ctx, `SELECT cooldown_until FROM zen_keys WHERE id = ?`, string(meta.ID)).Scan(&cooldown); err != nil {
				t.Fatalf("read cooldown_until: %v", err)
			}
			if !cooldown.Valid || cooldown.String == "" {
				t.Fatal("zen_keys.cooldown_until not persisted")
			}
		})
	}
}

// TestRecordPaidOutcome_success_raises_score_and_clears_failure_state verifies
// recovery after identity failures: score climbs, consecutive/error reason clear,
// and SQLite cooldown is lifted.
func TestRecordPaidOutcome_success_raises_score_and_clears_failure_state(t *testing.T) {
	t.Parallel()

	database, service, _ := newPoolWithDBClock(t)
	ctx := context.Background()
	meta, err := service.Create(ctx, zenpool.CreateInput{Label: "recover", Secret: "secret-recover-key"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	selected, err := service.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	// Arrange: two 429s so score and consecutive failures are clearly non-default.
	service.RecordPaidOutcome(ctx, selected, &zen.StatusError{StatusCode: http.StatusTooManyRequests}, 8*time.Millisecond)
	// Second outcome without re-acquire (key may already be cooling).
	service.RecordPaidOutcome(ctx, selected, &zen.StatusError{StatusCode: http.StatusTooManyRequests}, 9*time.Millisecond)

	afterFail := mustListKey(t, service, meta.ID)
	if afterFail.FailureCount != 2 || afterFail.ConsecutiveFailures != 2 {
		t.Fatalf("after fails: failure=%d consecutive=%d", afterFail.FailureCount, afterFail.ConsecutiveFailures)
	}
	if afterFail.HealthScore >= zenpool.DefaultHealthScore {
		t.Fatalf("after fails health_score=%v, want < %v", afterFail.HealthScore, zenpool.DefaultHealthScore)
	}
	if afterFail.CooldownUntil == nil || afterFail.LastErrorClass == "" {
		t.Fatalf("after fails expected cooling + error class, got until=%v class=%q", afterFail.CooldownUntil, afterFail.LastErrorClass)
	}
	failedScore := afterFail.HealthScore

	// Act: one successful paid attempt.
	service.RecordPaidOutcome(ctx, selected, nil, 25*time.Millisecond)

	afterOK := mustListKey(t, service, meta.ID)
	if afterOK.SuccessCount != 1 {
		t.Fatalf("success_count = %d, want 1", afterOK.SuccessCount)
	}
	if afterOK.ConsecutiveFailures != 0 {
		t.Fatalf("consecutive_failures = %d, want 0", afterOK.ConsecutiveFailures)
	}
	if afterOK.LastErrorClass != "" {
		t.Fatalf("last_error_class = %q, want empty", afterOK.LastErrorClass)
	}
	if afterOK.CooldownReason != "" {
		t.Fatalf("cooldown_reason = %q, want empty", afterOK.CooldownReason)
	}
	if afterOK.CooldownUntil != nil {
		t.Fatalf("cooldown_until = %v, want nil after success", afterOK.CooldownUntil)
	}
	if afterOK.HealthScore <= failedScore {
		t.Fatalf("health_score did not rise: before=%v after=%v", failedScore, afterOK.HealthScore)
	}
	if afterOK.LastSuccessAt == nil {
		t.Fatal("last_success_at is nil")
	}
	// Failure history is retained (not wiped); only consecutive/error/cooldown clear.
	if afterOK.FailureCount != 2 {
		t.Fatalf("failure_count = %d, want retained 2", afterOK.FailureCount)
	}

	var consecutive int
	var class, reason string
	var score float64
	err = database.QueryRowContext(ctx, `
		SELECT health_score, consecutive_failures, last_error_class, cooldown_reason
		FROM zen_key_health WHERE key_id = ?
	`, string(meta.ID)).Scan(&score, &consecutive, &class, &reason)
	if err != nil {
		t.Fatalf("read zen_key_health: %v", err)
	}
	if consecutive != 0 || class != "" || reason != "" {
		t.Fatalf("persisted clear state: consecutive=%d class=%q reason=%q", consecutive, class, reason)
	}
	if score != afterOK.HealthScore {
		t.Fatalf("persisted score=%v list score=%v", score, afterOK.HealthScore)
	}
}

// TestRecordPaidOutcome_401_benches_and_network_skips_identity_penalty ensures
// 401 goes through bench/unauthorized scoring, while network errors neither
// punish health counters nor cool/bench the key.
func TestRecordPaidOutcome_401_benches_and_network_skips_identity_penalty(t *testing.T) {
	t.Parallel()

	t.Run("401_benches_and_marks_unauthorized", func(t *testing.T) {
		t.Parallel()
		_, service, clock := newPoolWithDBClock(t)
		ctx := context.Background()
		meta, err := service.Create(ctx, zenpool.CreateInput{Label: "bad-auth", Secret: "secret-bad-auth"})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		selected, err := service.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}

		service.RecordPaidOutcome(ctx, selected, &zen.StatusError{StatusCode: http.StatusUnauthorized}, 5*time.Millisecond)

		if !service.IsBenched(meta.ID, clock.Now()) {
			t.Fatal("expected key benched after 401")
		}
		item := mustListKey(t, service, meta.ID)
		if item.LastErrorClass != "unauthorized" {
			t.Fatalf("last_error_class = %q, want unauthorized", item.LastErrorClass)
		}
		if item.FailureCount != 1 || item.ConsecutiveFailures != 1 {
			t.Fatalf("failure=%d consecutive=%d, want 1/1 for 401 identity failure", item.FailureCount, item.ConsecutiveFailures)
		}
		if item.HealthScore > 10 {
			t.Fatalf("health_score = %v, want <= 10 after 401", item.HealthScore)
		}
		// 401 uses process bench, not necessarily SQLite cooldown.
		if _, err := service.Acquire(ctx); !errors.Is(err, zenpool.ErrNoHealthyKey) {
			t.Fatalf("acquire while benched: %v", err)
		}
	})

	t.Run("network_error_no_identity_penalty", func(t *testing.T) {
		t.Parallel()
		database, service, clock := newPoolWithDBClock(t)
		ctx := context.Background()
		meta, err := service.Create(ctx, zenpool.CreateInput{Label: "net", Secret: "secret-net-key"})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		selected, err := service.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}

		netErr := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
		service.RecordPaidOutcome(ctx, selected, netErr, 3*time.Millisecond)

		item := mustListKey(t, service, meta.ID)
		if item.FailureCount != 0 || item.SuccessCount != 0 || item.ConsecutiveFailures != 0 {
			t.Fatalf("network must not change counters: success=%d failure=%d consecutive=%d",
				item.SuccessCount, item.FailureCount, item.ConsecutiveFailures)
		}
		if item.HealthScore != zenpool.DefaultHealthScore {
			t.Fatalf("health_score = %v, want cold-start %v", item.HealthScore, zenpool.DefaultHealthScore)
		}
		if item.LastErrorClass != "" || item.CooldownReason != "" {
			t.Fatalf("error class/reason set after network: class=%q reason=%q", item.LastErrorClass, item.CooldownReason)
		}
		if item.CooldownUntil != nil {
			t.Fatalf("cooldown_until set after network: %v", item.CooldownUntil)
		}
		if service.IsBenched(meta.ID, clock.Now()) {
			t.Fatal("network must not bench key")
		}

		// No health row should be forced by a pure network miss (GetHealth cold-start only).
		var n int
		if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM zen_key_health WHERE key_id = ?`, string(meta.ID)).Scan(&n); err != nil {
			t.Fatalf("count health: %v", err)
		}
		if n != 0 {
			t.Fatalf("zen_key_health rows = %d, want 0 after network-only outcome", n)
		}
	})

	t.Run("proxy_paid_network_failover_preserves_health", func(t *testing.T) {
		t.Parallel()
		service := newPool(t)
		ctx := context.Background()
		first, err := service.Create(ctx, zenpool.CreateInput{Label: "down", Secret: "down-key"})
		if err != nil {
			t.Fatalf("create down: %v", err)
		}
		second, err := service.Create(ctx, zenpool.CreateInput{Label: "up", Secret: "up-key"})
		if err != nil {
			t.Fatalf("create up: %v", err)
		}
		_ = first
		_ = second
		dialer := &scriptedDialer{responses: []dialResult{
			{err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}},
			{response: &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}},
		}}
		response, err := zenpool.ProxyPaid(ctx, service, dialer, nil, false, "", "")
		if err != nil {
			t.Fatalf("ProxyPaid: %v", err)
		}
		defer response.Body.Close()

		list, err := service.List(ctx)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		for _, item := range list {
			if item.FailureCount != 0 {
				t.Fatalf("key %s failure_count=%d after network failover", item.ID, item.FailureCount)
			}
			if item.CooldownUntil != nil {
				t.Fatalf("key %s cooled after network failover", item.ID)
			}
			if service.IsBenched(item.ID, time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)) {
				t.Fatalf("key %s benched after network failover", item.ID)
			}
			// Exactly one success (the healthy dial); the network attempt must not invent failures.
			if item.FailureCount != 0 {
				t.Fatalf("unexpected failure on %s", item.ID)
			}
		}
		// One of the keys should have recorded the successful dial.
		successes := 0
		for _, item := range list {
			successes += item.SuccessCount
		}
		if successes != 1 {
			t.Fatalf("total success_count = %d, want 1", successes)
		}
	})
}

// TestAcquire_cold_start_ignores_legacy_weight ensures dynamic selection uses
// health (cold-start equal scores), not stored weight, when samples are empty.
func TestAcquire_cold_start_ignores_legacy_weight(t *testing.T) {
	t.Parallel()

	service := newPool(t)
	ctx := context.Background()
	light, err := service.Create(ctx, zenpool.CreateInput{Label: "light", Secret: "secret-light", Weight: 1})
	if err != nil {
		t.Fatalf("create light: %v", err)
	}
	heavy, err := service.Create(ctx, zenpool.CreateInput{Label: "heavy", Secret: "secret-heavy", Weight: 99})
	if err != nil {
		t.Fatalf("create heavy: %v", err)
	}

	// Sanity: list still surfaces legacy weight for compatibility, but selection ignores it.
	list, err := service.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	byID := map[zenpool.KeyID]zenpool.Metadata{}
	for _, item := range list {
		byID[item.ID] = item
		if item.HealthScore != zenpool.DefaultHealthScore {
			t.Fatalf("key %s health_score=%v, want cold-start %v", item.ID, item.HealthScore, zenpool.DefaultHealthScore)
		}
		if item.SelectionScore < 1 {
			t.Fatalf("key %s selection_score=%d", item.ID, item.SelectionScore)
		}
	}
	if byID[light.ID].Weight != 1 || byID[heavy.ID].Weight != 99 {
		t.Fatalf("legacy weights not stored: light=%d heavy=%d", byID[light.ID].Weight, byID[heavy.ID].Weight)
	}

	counts := map[zenpool.KeyID]int{}
	const rounds = 80
	for range rounds {
		selected, err := service.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}
		counts[selected.ID]++
		// Release inflight without scoring (network-class errors are non-identity).
		service.RecordPaidOutcome(ctx, selected, errors.New("release-only"), 0)
	}

	// With equal cold-start scores, RR/weighted mass must stay near even.
	// Weight 99 vs 1 would yield ~99% heavy if legacy weight still drove selection.
	delta := counts[light.ID] - counts[heavy.ID]
	if delta < 0 {
		delta = -delta
	}
	if delta > rounds/4 {
		t.Fatalf("cold-start selection skewed by legacy weight: counts=%+v (delta=%d)", counts, delta)
	}
	if counts[heavy.ID] > int(float64(rounds)*0.7) {
		t.Fatalf("heavy weight key took %.0f%% of traffic; legacy weight must not dominate cold start: %+v",
			100*float64(counts[heavy.ID])/float64(rounds), counts)
	}
}

func newPoolWithDBClock(t *testing.T) (*sql.DB, *zenpool.Service, fixedClock) {
	t.Helper()
	clock := fixedClock{now: time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)}
	database, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	box, err := crypto.NewBox(strings.Repeat("s", 32))
	if err != nil {
		t.Fatalf("box: %v", err)
	}
	return database, zenpool.NewService(database, box, clock), clock
}

func mustListKey(t *testing.T, service *zenpool.Service, id zenpool.KeyID) zenpool.Metadata {
	t.Helper()
	list, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, item := range list {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("key %s not in list", id)
	return zenpool.Metadata{}
}
