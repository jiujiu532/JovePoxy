package zenpool_test

import (
	"context"
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
	response, err := zenpool.ProxyPaid(ctx, service, dialer, json.RawMessage(`{"model":"paid"}`), false)

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
	_, err := zenpool.ProxyPaid(context.Background(), service, &scriptedDialer{}, json.RawMessage(`{}`), false)

	// Then
	if !errors.Is(err, zenpool.ErrNoHealthyKey) {
		t.Fatalf("err = %v, want ErrNoHealthyKey", err)
	}
}

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
