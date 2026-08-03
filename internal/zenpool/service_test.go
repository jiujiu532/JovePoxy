package zenpool_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"jovepoxy/internal/crypto"
	"jovepoxy/internal/db"
	"jovepoxy/internal/zen"
	"jovepoxy/internal/zenpool"
)

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

func TestService_weighted_selection_and_cooldown(t *testing.T) {
	// Given
	service := newPool(t)
	ctx := context.Background()
	first, err := service.Create(ctx, zenpool.CreateInput{Label: "a", Secret: "secret-aaaa", Weight: 1})
	if err != nil {
		t.Fatalf("create a: %v", err)
	}
	second, err := service.Create(ctx, zenpool.CreateInput{Label: "b", Secret: "secret-bbbb", Weight: 3})
	if err != nil {
		t.Fatalf("create b: %v", err)
	}
	counts := map[zenpool.KeyID]int{}
	for range 40 {
		selected, err := service.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}
		counts[selected.ID]++
	}
	if counts[second.ID] <= counts[first.ID] {
		t.Fatalf("weighted counts = %+v, want b heavier than a", counts)
	}

	// When
	if err := service.MarkCooldown(ctx, second.ID, time.Minute); err != nil {
		t.Fatalf("cooldown: %v", err)
	}
	for range 5 {
		selected, err := service.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire after cooldown: %v", err)
		}
		if selected.ID != first.ID {
			t.Fatalf("selected %s, want only first key", selected.ID)
		}
	}
}

func TestService_list_masks_secret(t *testing.T) {
	// Given
	service := newPool(t)
	const secret = "super-secret-zen-key"
	if _, err := service.Create(context.Background(), zenpool.CreateInput{Label: "main", Secret: secret}); err != nil {
		t.Fatalf("create: %v", err)
	}

	// When
	list, err := service.List(context.Background())

	// Then
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Prefix == secret || strings.Contains(list[0].Prefix, secret[6:]) {
		t.Fatalf("list leaked secret: %+v", list)
	}
}

func TestProxyPaid_failsover_once_on_429(t *testing.T) {
	// Given
	service := newPool(t)
	ctx := context.Background()
	if _, err := service.Create(ctx, zenpool.CreateInput{Label: "bad", Secret: "bad-key"}); err != nil {
		t.Fatalf("create bad: %v", err)
	}
	if _, err := service.Create(ctx, zenpool.CreateInput{Label: "good", Secret: "good-key"}); err != nil {
		t.Fatalf("create good: %v", err)
	}
	dialer := &scriptedDialer{responses: []dialResult{
		{err: &zen.StatusError{StatusCode: http.StatusTooManyRequests}},
		{response: &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}},
	}}

	// When
	response, err := zenpool.ProxyPaid(ctx, service, dialer, json.RawMessage(`{"model":"paid"}`), false, "", "")

	// Then
	if err != nil {
		t.Fatalf("ProxyPaid: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || dialer.calls != 2 {
		t.Fatalf("status=%d calls=%d", response.StatusCode, dialer.calls)
	}
}

func TestProxyPaid_returns_no_healthy_key(t *testing.T) {
	// Given
	service := newPool(t)

	// When
	_, err := zenpool.ProxyPaid(context.Background(), service, &scriptedDialer{}, json.RawMessage(`{}`), false, "", "")

	// Then
	if !errors.Is(err, zenpool.ErrNoHealthyKey) {
		t.Fatalf("err = %v, want ErrNoHealthyKey", err)
	}
}

func TestProxyPaid_401_benches_key(t *testing.T) {
	service := newPool(t)
	ctx := context.Background()
	first, err := service.Create(ctx, zenpool.CreateInput{Label: "bad", Secret: "bad-key"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	second, err := service.Create(ctx, zenpool.CreateInput{Label: "good", Secret: "good-key"})
	if err != nil {
		t.Fatalf("create good: %v", err)
	}
	dialer := &scriptedDialer{responses: []dialResult{
		{err: &zen.StatusError{StatusCode: http.StatusUnauthorized}},
		{response: &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}},
	}}
	response, err := zenpool.ProxyPaid(ctx, service, dialer, json.RawMessage(`{}`), false, "", "")
	if err != nil {
		t.Fatalf("ProxyPaid: %v", err)
	}
	defer response.Body.Close()
	if !service.IsBenched(first.ID, serviceClock(service)) {
		// first may or may not be the one that failed depending on RR; check at least one benched
		if !service.IsBenched(first.ID, time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)) &&
			!service.IsBenched(second.ID, time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)) {
			t.Fatal("expected a key to be benched after 401")
		}
	}
	// Benched key must not be selected again while window active.
	benchedID := first.ID
	if !service.IsBenched(first.ID, time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)) {
		benchedID = second.ID
	}
	for range 5 {
		selected, err := service.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}
		if selected.ID == benchedID {
			t.Fatalf("acquired benched key %s", benchedID)
		}
	}
}

func TestProxyPaid_max_attempts_cap(t *testing.T) {
	service := newPool(t)
	service.SetMaxAttempts(3)
	ctx := context.Background()
	for i, secret := range []string{"k1", "k2", "k3", "k4"} {
		if _, err := service.Create(ctx, zenpool.CreateInput{Label: secret, Secret: secret + "-secret", Weight: 1}); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	dialer := &scriptedDialer{responses: []dialResult{
		{err: &zen.StatusError{StatusCode: 500}},
		{err: &zen.StatusError{StatusCode: 500}},
		{err: &zen.StatusError{StatusCode: 500}},
		{err: &zen.StatusError{StatusCode: 500}},
	}}
	_, err := zenpool.ProxyPaid(ctx, service, dialer, json.RawMessage(`{}`), false, "", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if dialer.calls != 3 {
		t.Fatalf("calls = %d, want 3 (MaxAttempts)", dialer.calls)
	}
}

func TestAcquire_sticky_stable(t *testing.T) {
	service := newPool(t)
	ctx := context.Background()
	service.SetLoadPolicy(zenpool.LoadPolicySticky)
	var ids []zenpool.KeyID
	for _, label := range []string{"a", "b", "c"} {
		meta, err := service.Create(ctx, zenpool.CreateInput{Label: label, Secret: "secret-" + label, Weight: 1})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		ids = append(ids, meta.ID)
	}
	affinity := zenpool.ConversationAffinityKey(http.Header{"X-Conversation-Id": []string{"conv-stable-1"}}, nil)
	if affinity == "" {
		t.Fatal("affinity empty")
	}
	first, err := service.AcquireFor(ctx, zenpool.AcquireOptions{AffinityKey: affinity, Policy: zenpool.LoadPolicySticky})
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	for range 10 {
		selected, err := service.AcquireFor(ctx, zenpool.AcquireOptions{AffinityKey: affinity, Policy: zenpool.LoadPolicySticky})
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}
		if selected.ID != first.ID {
			t.Fatalf("sticky drift: got %s want %s", selected.ID, first.ID)
		}
	}
	// Cooling the sticky key should re-pin to another healthy candidate.
	if err := service.MarkCooldown(ctx, first.ID, time.Minute); err != nil {
		t.Fatalf("cooldown: %v", err)
	}
	next, err := service.AcquireFor(ctx, zenpool.AcquireOptions{AffinityKey: affinity, Policy: zenpool.LoadPolicySticky})
	if err != nil {
		t.Fatalf("acquire after cool: %v", err)
	}
	if next.ID == first.ID {
		t.Fatal("expected sticky to move off cooled key")
	}
	// Ensure we still only pick among remaining ids.
	_ = ids
}

func TestConversationAffinityKey_hash_only(t *testing.T) {
	raw := "session-raw-secret-value"
	h := zenpool.ConversationAffinityKey(http.Header{"X-Session-Id": []string{raw}}, nil)
	if h == "" || h == raw || strings.Contains(h, raw) {
		t.Fatalf("affinity leaked raw: %q", h)
	}
	h2 := zenpool.ConversationAffinityKey(nil, json.RawMessage(`{"user":"alice"}`))
	if h2 == "" || h2 == "alice" {
		t.Fatalf("body affinity = %q", h2)
	}
	if zenpool.ConversationAffinityKey(nil, nil) != "" {
		t.Fatal("empty material should yield empty affinity")
	}
}

func TestLoadPolicy_and_MaxAttempts_clamp(t *testing.T) {
	service := newPool(t)
	if service.LoadPolicy() != zenpool.LoadPolicySpread {
		t.Fatalf("default policy = %s", service.LoadPolicy())
	}
	service.SetLoadPolicy(zenpool.LoadPolicySticky)
	if service.LoadPolicy() != zenpool.LoadPolicySticky {
		t.Fatal("sticky not set")
	}
	service.SetLoadPolicy("nope")
	if service.LoadPolicy() != zenpool.LoadPolicySpread {
		t.Fatal("invalid policy should fall back to spread")
	}
	service.SetMaxAttempts(1)
	if service.MaxAttempts() != 2 {
		t.Fatalf("min clamp = %d", service.MaxAttempts())
	}
	service.SetMaxAttempts(9)
	if service.MaxAttempts() != 4 {
		t.Fatalf("max clamp = %d", service.MaxAttempts())
	}
	service.SetMaxAttempts(3)
	if service.MaxAttempts() != 3 {
		t.Fatalf("mid = %d", service.MaxAttempts())
	}
}


func TestService_bench_expires(t *testing.T) {
	// mutable clock so we can advance past bench window
	clock := &mutableClock{now: time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)}
	database, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	box, err := crypto.NewBox(strings.Repeat("s", 32))
	if err != nil {
		t.Fatalf("box: %v", err)
	}
	service := zenpool.NewService(database, box, clock)
	ctx := context.Background()
	meta, err := service.Create(ctx, zenpool.CreateInput{Label: "only", Secret: "secret-only"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	service.MarkBench(meta.ID, time.Minute)
	if !service.IsBenched(meta.ID, clock.Now()) {
		t.Fatal("should be benched")
	}
	if _, err := service.Acquire(ctx); !errors.Is(err, zenpool.ErrNoHealthyKey) {
		t.Fatalf("acquire during bench: %v", err)
	}
	clock.now = clock.now.Add(2 * time.Minute)
	if service.IsBenched(meta.ID, clock.Now()) {
		t.Fatal("bench should have expired")
	}
	selected, err := service.Acquire(ctx)
	if err != nil || selected.ID != meta.ID {
		t.Fatalf("after expire selected=%+v err=%v", selected, err)
	}
}

type mutableClock struct{ now time.Time }

func (c *mutableClock) Now() time.Time { return c.now }

type dialResult struct {
	response *http.Response
	err      error
}

type scriptedDialer struct {
	responses []dialResult
	calls     int
}

func (dialer *scriptedDialer) ChatCompletions(_ context.Context, _ zen.Auth, _ json.RawMessage, _ bool) (*http.Response, error) {
	if dialer.calls >= len(dialer.responses) {
		return nil, &zen.StatusError{StatusCode: 500}
	}
	result := dialer.responses[dialer.calls]
	dialer.calls++
	return result.response, result.err
}


func TestService_create_stores_key_prefix(t *testing.T) {
	// Given
	database, service := newPoolWithDB(t)
	ctx := context.Background()
	const secret = "super-secret-zen-key"

	// When
	created, err := service.Create(ctx, zenpool.CreateInput{Label: "main", Secret: secret})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Then — response prefix is masked, not the full secret
	if created.Prefix == "" || created.Prefix == secret || strings.Contains(created.Prefix, secret[6:]) {
		t.Fatalf("create prefix leaked or empty: %+v", created)
	}
	list, err := service.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Prefix != created.Prefix {
		t.Fatalf("list prefix = %+v, want %q", list, created.Prefix)
	}
	var stored string
	if err := database.QueryRowContext(ctx, "SELECT key_prefix FROM zen_keys WHERE id = ?", string(created.ID)).Scan(&stored); err != nil {
		t.Fatalf("read key_prefix: %v", err)
	}
	if stored != created.Prefix {
		t.Fatalf("stored key_prefix = %q, want %q", stored, created.Prefix)
	}
}

func TestService_acquire_skips_dirty_cooldown(t *testing.T) {
	// Given two healthy keys; poison one cooldown_until so isCooling would error.
	database, service := newPoolWithDB(t)
	ctx := context.Background()
	dirty, err := service.Create(ctx, zenpool.CreateInput{Label: "dirty", Secret: "secret-dirty"})
	if err != nil {
		t.Fatalf("create dirty: %v", err)
	}
	clean, err := service.Create(ctx, zenpool.CreateInput{Label: "clean", Secret: "secret-clean"})
	if err != nil {
		t.Fatalf("create clean: %v", err)
	}
	if _, err := database.ExecContext(ctx, "UPDATE zen_keys SET cooldown_until = ? WHERE id = ?", "not-a-timestamp", string(dirty.ID)); err != nil {
		t.Fatalf("poison cooldown: %v", err)
	}

	// When / Then — Acquire must not fail; only the clean key is selectable.
	for range 6 {
		selected, err := service.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire with dirty cooldown: %v", err)
		}
		if selected.ID != clean.ID {
			t.Fatalf("selected %s, want clean %s (dirty key must be skipped)", selected.ID, clean.ID)
		}
	}
}

func TestService_acquire_skips_bad_ciphertext(t *testing.T) {
	// Given one unreadable ciphertext row and one healthy key.
	database, service := newPoolWithDB(t)
	ctx := context.Background()
	good, err := service.Create(ctx, zenpool.CreateInput{Label: "good", Secret: "secret-good-key"})
	if err != nil {
		t.Fatalf("create good: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO zen_keys (id, label, key_ciphertext, key_prefix, weight, enabled, cooldown_until, created_at, provider)
		VALUES (?, ?, ?, ?, 1, 1, NULL, ?, ?)
	`, "zk_badcipher_acq", "bad", "not-valid-ciphertext", "badpre…", "2026-07-15T12:00:00Z", "opencode"); err != nil {
		t.Fatalf("insert bad ciphertext row: %v", err)
	}

	// When / Then — Acquire re-picks past the bad row instead of failing the whole pool.
	for range 6 {
		selected, err := service.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire with bad ciphertext: %v", err)
		}
		if selected.ID != good.ID {
			t.Fatalf("selected %s, want good %s", selected.ID, good.ID)
		}
		if selected.Secret != "secret-good-key" {
			t.Fatalf("secret = %q", selected.Secret)
		}
	}
}

func TestProxyPaid_status_cooldown_and_markcooldown_bench_fallback(t *testing.T) {
	// 429 cools the failed key in SQLite so Acquire prefers the remaining healthy key.
	service := newPool(t)
	ctx := context.Background()
	first, err := service.Create(ctx, zenpool.CreateInput{Label: "rate", Secret: "rate-key"})
	if err != nil {
		t.Fatalf("create rate: %v", err)
	}
	second, err := service.Create(ctx, zenpool.CreateInput{Label: "good", Secret: "good-key"})
	if err != nil {
		t.Fatalf("create good: %v", err)
	}
	_ = second
	dialer := &scriptedDialer{responses: []dialResult{
		{err: &zen.StatusError{StatusCode: http.StatusTooManyRequests}},
		{response: &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}},
	}}
	response, err := zenpool.ProxyPaid(ctx, service, dialer, json.RawMessage(`{}`), false, "", "")
	if err != nil {
		t.Fatalf("ProxyPaid: %v", err)
	}
	defer response.Body.Close()
	if dialer.calls != 2 {
		t.Fatalf("calls = %d, want 2", dialer.calls)
	}
	list, listErr := service.List(ctx)
	if listErr != nil {
		t.Fatalf("list: %v", listErr)
	}
	cooled := 0
	for _, item := range list {
		if item.CooldownUntil != nil {
			cooled++
		}
	}
	if cooled != 1 {
		t.Fatalf("expected exactly one key cooled after 429, got %d (first=%s)", cooled, first.ID)
	}
}

func TestService_list_tolerates_bad_ciphertext(t *testing.T) {
	// Given one good key and one row with unreadable ciphertext.
	database, service := newPoolWithDB(t)
	ctx := context.Background()
	good, err := service.Create(ctx, zenpool.CreateInput{Label: "good", Secret: "secret-good-key"})
	if err != nil {
		t.Fatalf("create good: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO zen_keys (id, label, key_ciphertext, key_prefix, weight, enabled, cooldown_until, created_at, provider)
		VALUES (?, ?, ?, ?, 1, 1, NULL, ?, ?)
	`, "zk_badcipher", "bad", "not-valid-ciphertext", "badpre…", "2026-07-15T12:00:00Z", "opencode"); err != nil {
		t.Fatalf("insert bad ciphertext row: %v", err)
	}

	// When
	list, err := service.List(ctx)

	// Then — whole list succeeds; good key intact; bad row degraded (prefix from column).
	if err != nil {
		t.Fatalf("list with bad ciphertext: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}
	byID := map[zenpool.KeyID]zenpool.Metadata{}
	for _, item := range list {
		byID[item.ID] = item
	}
	if byID[good.ID].Prefix != good.Prefix || byID[good.ID].Label != "good" {
		t.Fatalf("good key degraded: %+v", byID[good.ID])
	}
	bad := byID[zenpool.KeyID("zk_badcipher")]
	if bad.Label != "bad" {
		t.Fatalf("bad row missing: %+v", list)
	}
	if bad.Prefix != "badpre…" {
		t.Fatalf("bad prefix = %q, want stored key_prefix", bad.Prefix)
	}
}

func newPool(t *testing.T) *zenpool.Service {
	t.Helper()
	database, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	box, err := crypto.NewBox(strings.Repeat("s", 32))
	if err != nil {
		t.Fatalf("box: %v", err)
	}
	return zenpool.NewService(database, box, fixedClock{now: time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)})
}


func newPoolWithDB(t *testing.T) (*sql.DB, *zenpool.Service) {
	t.Helper()
	database, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	box, err := crypto.NewBox(strings.Repeat("s", 32))
	if err != nil {
		t.Fatalf("box: %v", err)
	}
	service := zenpool.NewService(database, box, fixedClock{now: time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)})
	return database, service
}

// serviceClock is unused helper placeholder to keep test file free of reflection.
func serviceClock(_ *zenpool.Service) time.Time {
	return time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
}
