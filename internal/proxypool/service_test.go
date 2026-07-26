package proxypool_test

import (
	"context"
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
	resp, err := proxypool.ProxyFree(ctx, service, dialer, json.RawMessage(`{}`), false)
	if err != nil {
		t.Fatalf("ProxyFree: %v", err)
	}
	defer resp.Body.Close()
	if dialer.calls != 2 {
		t.Fatalf("calls = %d", dialer.calls)
	}
}

func TestProxyFree_direct_when_pool_empty(t *testing.T) {
	ctx := context.Background()
	service := newPool(t)
	dialer := &scriptedFreeDialer{responses: []dialResult{
		{response: &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`))}},
	}}
	resp, err := proxypool.ProxyFree(ctx, service, dialer, json.RawMessage(`{}`), false)
	if err != nil {
		t.Fatalf("ProxyFree: %v", err)
	}
	defer resp.Body.Close()
	if dialer.directCalls != 1 {
		t.Fatalf("directCalls = %d", dialer.directCalls)
	}
}

func TestParseRejectsUnknownScheme(t *testing.T) {
	_, err := newPool(t).Create(context.Background(), proxypool.CreateInput{Label: "x", URL: "ftp://h:1"})
	if !errors.Is(err, proxypool.ErrInvalidURL) {
		t.Fatalf("err = %v", err)
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
	database, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	box, err := crypto.NewBox(strings.Repeat("p", 32))
	if err != nil {
		t.Fatalf("box: %v", err)
	}
	return proxypool.NewService(database, box, fixedClock{now: time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)})
}
