package proxypool_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"jovepoxy/internal/crypto"
	"jovepoxy/internal/db"
	"jovepoxy/internal/proxypool"
	"jovepoxy/internal/zen"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func TestService_create_list_acquire_and_cooldown(t *testing.T) {
	ctx := context.Background()
	service := newPool(t)
	a, err := service.Create(ctx, proxypool.CreateInput{Label: "a", URL: "http://127.0.0.1:18080", Weight: 1})
	if err != nil {
		t.Fatalf("create a: %v", err)
	}
	b, err := service.Create(ctx, proxypool.CreateInput{Label: "b", URL: "socks5h://127.0.0.1:11080", Weight: 3})
	if err != nil {
		t.Fatalf("create b: %v", err)
	}
	list, err := service.List(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %+v err=%v", list, err)
	}
	if list[0].Host == "" || strings.Contains(list[0].Host, "password") {
		t.Fatalf("host leaked credentials: %+v", list[0])
	}

	counts := map[proxypool.ProxyID]int{}
	for range 40 {
		selected, err := service.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}
		counts[selected.ID]++
	}
	if counts[b.ID] <= counts[a.ID] {
		t.Fatalf("weighted counts = %+v", counts)
	}

	if err := service.MarkCooldown(ctx, b.ID, time.Minute); err != nil {
		t.Fatalf("cooldown: %v", err)
	}
	for range 5 {
		selected, err := service.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire after cooldown: %v", err)
		}
		if selected.ID != a.ID {
			t.Fatalf("selected %s, want only a", selected.ID)
		}
	}
}

func TestProxyFree_failsover_on_429(t *testing.T) {
	ctx := context.Background()
	service := newPool(t)
	if _, err := service.Create(ctx, proxypool.CreateInput{Label: "a", URL: "http://127.0.0.1:18081"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := service.Create(ctx, proxypool.CreateInput{Label: "b", URL: "http://127.0.0.1:18082"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	dialer := &scriptedFreeDialer{responses: []dialResult{
		{err: &zen.StatusError{StatusCode: http.StatusTooManyRequests}},
		{response: &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}},
	}}
	resp, selected, err := proxypool.ProxyFree(ctx, service, dialer, json.RawMessage(`{}`), false)
	if err != nil {
		t.Fatalf("ProxyFree: %v", err)
	}
	defer resp.Body.Close()
	if dialer.calls != 2 {
		t.Fatalf("calls = %d", dialer.calls)
	}
	if selected.ID == "" || selected.Host == "" {
		t.Fatalf("selected = %+v, want proxy metadata", selected)
	}
}

func TestProxyFree_direct_when_pool_empty(t *testing.T) {
	ctx := context.Background()
	service := newPool(t)
	dialer := &scriptedFreeDialer{responses: []dialResult{
		{response: &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`))}},
	}}
	resp, selected, err := proxypool.ProxyFree(ctx, service, dialer, json.RawMessage(`{}`), false)
	if err != nil {
		t.Fatalf("ProxyFree: %v", err)
	}
	defer resp.Body.Close()
	if dialer.directCalls != 1 {
		t.Fatalf("directCalls = %d", dialer.directCalls)
	}
	if selected.ID != "" {
		t.Fatalf("selected = %+v, want direct (empty)", selected)
	}
}

func TestParseRejectsUnknownScheme(t *testing.T) {
	_, err := newPool(t).Create(context.Background(), proxypool.CreateInput{Label: "x", URL: "ftp://h:1"})
	if !errors.Is(err, proxypool.ErrInvalidURL) {
		t.Fatalf("err = %v", err)
	}
}

func TestProxyFree_network_failover_without_cooldown(t *testing.T) {
	ctx := context.Background()
	service := newPool(t)
	if _, err := service.Create(ctx, proxypool.CreateInput{Label: "a", URL: "http://127.0.0.1:18091"}); err != nil {
		t.Fatalf("create a: %v", err)
	}
	if _, err := service.Create(ctx, proxypool.CreateInput{Label: "b", URL: "http://127.0.0.1:18092"}); err != nil {
		t.Fatalf("create b: %v", err)
	}
	dialer := &scriptedFreeDialer{responses: []dialResult{
		{err: errors.New("dial tcp: connection refused")},
		{response: &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}},
	}}
	resp, _, err := proxypool.ProxyFree(ctx, service, dialer, json.RawMessage(`{}`), false)
	if err != nil {
		t.Fatalf("ProxyFree: %v", err)
	}
	defer resp.Body.Close()
	if dialer.calls != 2 {
		t.Fatalf("calls = %d, want 2", dialer.calls)
	}
	list, listErr := service.List(ctx)
	if listErr != nil {
		t.Fatalf("list: %v", listErr)
	}
	for _, item := range list {
		if item.CooldownUntil != nil {
			t.Fatalf("network failover must not MarkCooldown; proxy %s until=%v", item.ID, item.CooldownUntil)
		}
	}
}

func TestProxyFree_no_failover_on_deadline(t *testing.T) {
	ctx := context.Background()
	service := newPool(t)
	if _, err := service.Create(ctx, proxypool.CreateInput{Label: "a", URL: "http://127.0.0.1:18093"}); err != nil {
		t.Fatalf("create a: %v", err)
	}
	if _, err := service.Create(ctx, proxypool.CreateInput{Label: "b", URL: "http://127.0.0.1:18094"}); err != nil {
		t.Fatalf("create b: %v", err)
	}
	dialer := &scriptedFreeDialer{responses: []dialResult{
		{err: context.DeadlineExceeded},
		{response: &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}},
	}}
	_, _, err := proxypool.ProxyFree(ctx, service, dialer, json.RawMessage(`{}`), false)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if dialer.calls != 1 {
		t.Fatalf("calls = %d, want 1", dialer.calls)
	}
	list, listErr := service.List(ctx)
	if listErr != nil {
		t.Fatalf("list: %v", listErr)
	}
	for _, item := range list {
		if item.CooldownUntil != nil {
			t.Fatalf("deadline must not cool proxy %s", item.ID)
		}
	}
}

func TestService_acquire_skips_dirty_cooldown(t *testing.T) {
	ctx := context.Background()
	database, service := newPoolWithDB(t)
	dirty, err := service.Create(ctx, proxypool.CreateInput{Label: "dirty", URL: "http://127.0.0.1:18101"})
	if err != nil {
		t.Fatalf("create dirty: %v", err)
	}
	clean, err := service.Create(ctx, proxypool.CreateInput{Label: "clean", URL: "http://127.0.0.1:18102"})
	if err != nil {
		t.Fatalf("create clean: %v", err)
	}
	if _, err := database.ExecContext(ctx, "UPDATE egress_proxies SET cooldown_until = ? WHERE id = ?", "not-a-timestamp", string(dirty.ID)); err != nil {
		t.Fatalf("poison cooldown: %v", err)
	}
	for range 6 {
		selected, err := service.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire with dirty cooldown: %v", err)
		}
		if selected.ID != clean.ID {
			t.Fatalf("selected %s, want clean %s", selected.ID, clean.ID)
		}
	}
}

func TestService_acquire_accepts_rfc3339_cooldown(t *testing.T) {
	ctx := context.Background()
	database, service := newPoolWithDB(t)
	cooling, err := service.Create(ctx, proxypool.CreateInput{Label: "cooling", URL: "http://127.0.0.1:18111"})
	if err != nil {
		t.Fatalf("create cooling: %v", err)
	}
	clean, err := service.Create(ctx, proxypool.CreateInput{Label: "clean", URL: "http://127.0.0.1:18112"})
	if err != nil {
		t.Fatalf("create clean: %v", err)
	}
	// Plain RFC3339 (no fractional seconds) must still be recognized as cooling.
	until := time.Date(2026, 7, 15, 12, 5, 0, 0, time.UTC).Format(time.RFC3339)
	if _, err := database.ExecContext(ctx, "UPDATE egress_proxies SET cooldown_until = ? WHERE id = ?", until, string(cooling.ID)); err != nil {
		t.Fatalf("set cooldown: %v", err)
	}
	for range 5 {
		selected, err := service.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}
		if selected.ID != clean.ID {
			t.Fatalf("selected %s, want clean %s (RFC3339 cooldown must apply)", selected.ID, clean.ID)
		}
	}
}

func TestService_acquire_skips_bad_ciphertext(t *testing.T) {
	ctx := context.Background()
	database, service := newPoolWithDB(t)
	good, err := service.Create(ctx, proxypool.CreateInput{Label: "good", URL: "http://127.0.0.1:18121"})
	if err != nil {
		t.Fatalf("create good: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO egress_proxies (id, label, url_ciphertext, scheme, host, weight, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, 1, 1, ?)
	`, "px_badcipher", "bad", "not-valid-ciphertext", "http", "127.0.0.1:9", "2026-07-15T12:00:00Z"); err != nil {
		t.Fatalf("insert bad ciphertext: %v", err)
	}
	for range 6 {
		selected, err := service.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire with bad ciphertext: %v", err)
		}
		if selected.ID != good.ID {
			t.Fatalf("selected %s, want good %s", selected.ID, good.ID)
		}
	}
}

type dialResult struct {
	response *http.Response
	err      error
}

type scriptedFreeDialer struct {
	responses   []dialResult
	calls       int
	directCalls int
}

func (d *scriptedFreeDialer) ChatCompletions(context.Context, zen.Auth, json.RawMessage, bool) (*http.Response, error) {
	d.directCalls++
	if d.calls >= len(d.responses) {
		return nil, &zen.StatusError{StatusCode: 500}
	}
	// When pool empty, ProxyFree uses ChatCompletions; reuse response list.
	item := d.responses[d.calls]
	d.calls++
	return item.response, item.err
}

func (d *scriptedFreeDialer) ChatCompletionsWithProxy(_ context.Context, _ zen.Auth, _ json.RawMessage, _ bool, _ *url.URL) (*http.Response, error) {
	if d.calls >= len(d.responses) {
		return nil, &zen.StatusError{StatusCode: 500}
	}
	item := d.responses[d.calls]
	d.calls++
	return item.response, item.err
}

func newPool(t *testing.T) *proxypool.Service {
	t.Helper()
	_, service := newPoolWithDB(t)
	return service
}

func newPoolWithDB(t *testing.T) (*sql.DB, *proxypool.Service) {
	t.Helper()
	database, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	box, err := crypto.NewBox(strings.Repeat("p", 32))
	if err != nil {
		t.Fatalf("box: %v", err)
	}
	return database, proxypool.NewService(database, box, fixedClock{now: time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)})
}
