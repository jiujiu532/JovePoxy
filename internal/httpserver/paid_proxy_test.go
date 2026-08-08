package httpserver

import (
	"context"
	"encoding/json"
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
	"jovepoxy/internal/zenpool"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type paidDialResult struct {
	response *http.Response
	err      error
}

// scriptedPaidDialer records direct vs proxy dials for paid egress tests.
type scriptedPaidDialer struct {
	// proxyResponses are consumed by ChatCompletionsWithProxy only.
	proxyResponses []paidDialResult
	// directResponses are consumed by ChatCompletions only.
	directResponses []paidDialResult
	proxyCalls      int
	directCalls     int
	lastProxy       *url.URL
}

func (d *scriptedPaidDialer) ChatCompletions(context.Context, zen.Auth, json.RawMessage, bool) (*http.Response, error) {
	d.directCalls++
	if d.directCalls-1 >= len(d.directResponses) {
		return nil, &zen.StatusError{StatusCode: 500}
	}
	item := d.directResponses[d.directCalls-1]
	return item.response, item.err
}

func (d *scriptedPaidDialer) ChatCompletionsWithProxy(_ context.Context, _ zen.Auth, _ json.RawMessage, _ bool, proxyURL *url.URL) (*http.Response, error) {
	d.proxyCalls++
	d.lastProxy = proxyURL
	if d.proxyCalls-1 >= len(d.proxyResponses) {
		return nil, &zen.StatusError{StatusCode: 500}
	}
	item := d.proxyResponses[d.proxyCalls-1]
	return item.response, item.err
}

func okBody() *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}
}

func newPaidTestPools(t *testing.T) (*proxypool.Service, *zenpool.Service) {
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
	clock := fixedClock{now: time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)}
	proxies := proxypool.NewService(database, box, clock)
	keys := zenpool.NewService(database, box, clock)
	return proxies, keys
}

func seedZenKey(t *testing.T, keys *zenpool.Service) {
	t.Helper()
	if _, err := keys.Create(context.Background(), zenpool.CreateInput{Label: "k", Secret: "paid-secret-value"}); err != nil {
		t.Fatalf("create zen key: %v", err)
	}
}

func TestDialPaid_flag_off_skips_proxy_acquire(t *testing.T) {
	ctx := context.Background()
	proxies, keys := newPaidTestPools(t)
	// Proxy exists but flag is off — must not use it.
	if _, err := proxies.Create(ctx, proxypool.CreateInput{Label: "px", URL: "http://127.0.0.1:18080"}); err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	seedZenKey(t, keys)
	dialer := &scriptedPaidDialer{directResponses: []paidDialResult{{response: okBody()}}}

	resp, selected, err := dialPaidWithOptionalProxy(ctx, proxies, keys, dialer, json.RawMessage(`{}`), false, "", zenpool.ProviderOpenCode)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer resp.Body.Close()
	if dialer.proxyCalls != 0 {
		t.Fatalf("proxyCalls = %d, want 0 when flag off", dialer.proxyCalls)
	}
	if dialer.directCalls != 1 {
		t.Fatalf("directCalls = %d, want 1", dialer.directCalls)
	}
	if selected.ID != "" {
		t.Fatalf("selected = %+v, want empty (direct)", selected)
	}
}

func TestDialPaid_flag_on_healthy_proxy_returns_selected(t *testing.T) {
	ctx := context.Background()
	proxies, keys := newPaidTestPools(t)
	proxies.SetPaidUseProxyPool(true)
	meta, err := proxies.Create(ctx, proxypool.CreateInput{Label: "egress-a", URL: "http://127.0.0.1:18081"})
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	seedZenKey(t, keys)
	dialer := &scriptedPaidDialer{proxyResponses: []paidDialResult{{response: okBody()}}}

	resp, selected, err := dialPaidWithOptionalProxy(ctx, proxies, keys, dialer, json.RawMessage(`{}`), false, "", zenpool.ProviderOpenCode)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer resp.Body.Close()
	if dialer.proxyCalls != 1 {
		t.Fatalf("proxyCalls = %d, want 1", dialer.proxyCalls)
	}
	if dialer.directCalls != 0 {
		t.Fatalf("directCalls = %d, want 0", dialer.directCalls)
	}
	if selected.ID != meta.ID || selected.Label != "egress-a" {
		t.Fatalf("selected = %+v, want id=%s label=egress-a", selected, meta.ID)
	}
	if dialer.lastProxy == nil || dialer.lastProxy.Host == "" {
		t.Fatalf("lastProxy = %v", dialer.lastProxy)
	}
}

func TestDialPaid_flag_on_empty_pool_direct(t *testing.T) {
	ctx := context.Background()
	proxies, keys := newPaidTestPools(t)
	proxies.SetPaidUseProxyPool(true)
	seedZenKey(t, keys)
	dialer := &scriptedPaidDialer{directResponses: []paidDialResult{{response: okBody()}}}

	resp, selected, err := dialPaidWithOptionalProxy(ctx, proxies, keys, dialer, json.RawMessage(`{}`), false, "", zenpool.ProviderOpenCode)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer resp.Body.Close()
	if dialer.proxyCalls != 0 {
		t.Fatalf("proxyCalls = %d, want 0", dialer.proxyCalls)
	}
	if dialer.directCalls != 1 {
		t.Fatalf("directCalls = %d, want 1", dialer.directCalls)
	}
	if selected.ID != "" {
		t.Fatalf("selected = %+v, want empty", selected)
	}
}

func TestDialPaid_flag_on_proxy_status_cools_and_falls_back_direct(t *testing.T) {
	ctx := context.Background()
	proxies, keys := newPaidTestPools(t)
	proxies.SetPaidUseProxyPool(true)
	meta, err := proxies.Create(ctx, proxypool.CreateInput{Label: "bad-px", URL: "http://127.0.0.1:18091"})
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	seedZenKey(t, keys)
	// 403: proxypool cools egress; zenpool does not punish key identity (unlike 429).
	dialer := &scriptedPaidDialer{
		proxyResponses:  []paidDialResult{{err: &zen.StatusError{StatusCode: http.StatusForbidden}}},
		directResponses: []paidDialResult{{response: okBody()}},
	}

	resp, selected, err := dialPaidWithOptionalProxy(ctx, proxies, keys, dialer, json.RawMessage(`{}`), false, "", zenpool.ProviderOpenCode)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer resp.Body.Close()
	if dialer.proxyCalls != 1 {
		t.Fatalf("proxyCalls = %d, want 1", dialer.proxyCalls)
	}
	if dialer.directCalls != 1 {
		t.Fatalf("directCalls = %d, want 1 (direct fallback)", dialer.directCalls)
	}
	if selected.ID != "" {
		t.Fatalf("selected after direct fallback = %+v, want empty", selected)
	}
	list, listErr := proxies.List(ctx)
	if listErr != nil {
		t.Fatalf("list: %v", listErr)
	}
	var cooled bool
	for _, item := range list {
		if item.ID == meta.ID && item.CooldownUntil != nil {
			cooled = true
		}
	}
	if !cooled {
		t.Fatalf("proxy %s should be cooled after 403; list=%+v", meta.ID, list)
	}
}

func TestDialPaid_flag_on_proxy_429_cools_proxy_not_key(t *testing.T) {
	ctx := context.Background()
	proxies, keys := newPaidTestPools(t)
	proxies.SetPaidUseProxyPool(true)
	proxyMeta, err := proxies.Create(ctx, proxypool.CreateInput{Label: "rate-px", URL: "http://127.0.0.1:18092"})
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	keyMeta, err := keys.Create(ctx, zenpool.CreateInput{Label: "k", Secret: "paid-secret-value"})
	if err != nil {
		t.Fatalf("create zen key: %v", err)
	}
	// 429 via egress: cool proxy, do not cool key identity (ProxyPaidEgress), then direct succeeds.
	dialer := &scriptedPaidDialer{
		proxyResponses:  []paidDialResult{{err: &zen.StatusError{StatusCode: http.StatusTooManyRequests}}},
		directResponses: []paidDialResult{{response: okBody()}},
	}

	resp, selected, err := dialPaidWithOptionalProxy(ctx, proxies, keys, dialer, json.RawMessage(`{}`), false, "", zenpool.ProviderOpenCode)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer resp.Body.Close()
	if dialer.proxyCalls != 1 || dialer.directCalls != 1 {
		t.Fatalf("proxyCalls=%d directCalls=%d, want 1/1", dialer.proxyCalls, dialer.directCalls)
	}
	if selected.ID != "" {
		t.Fatalf("selected after direct = %+v, want empty", selected)
	}
	proxyList, listErr := proxies.List(ctx)
	if listErr != nil {
		t.Fatalf("list proxies: %v", listErr)
	}
	var proxyCooled bool
	for _, item := range proxyList {
		if item.ID == proxyMeta.ID && item.CooldownUntil != nil {
			proxyCooled = true
		}
	}
	if !proxyCooled {
		t.Fatalf("proxy should cool after 429; list=%+v", proxyList)
	}
	keyList, keyErr := keys.List(ctx)
	if keyErr != nil {
		t.Fatalf("list keys: %v", keyErr)
	}
	for _, item := range keyList {
		if item.ID != keyMeta.ID {
			continue
		}
		if item.CooldownUntil != nil {
			t.Fatalf("key should not cool after proxy-path 429; got until=%v", item.CooldownUntil)
		}
		if item.FailureCount != 0 || item.LastErrorClass != "" {
			t.Fatalf("key identity punished after proxy 429: failures=%d class=%q", item.FailureCount, item.LastErrorClass)
		}
	}
}

func TestDialPaid_flag_on_proxy_status_switches_to_second_proxy(t *testing.T) {
	ctx := context.Background()
	proxies, keys := newPaidTestPools(t)
	proxies.SetPaidUseProxyPool(true)
	if _, err := proxies.Create(ctx, proxypool.CreateInput{Label: "a", URL: "http://127.0.0.1:18101"}); err != nil {
		t.Fatalf("create a: %v", err)
	}
	if _, err := proxies.Create(ctx, proxypool.CreateInput{Label: "b", URL: "http://127.0.0.1:18102"}); err != nil {
		t.Fatalf("create b: %v", err)
	}
	seedZenKey(t, keys)
	dialer := &scriptedPaidDialer{
		proxyResponses: []paidDialResult{
			{err: &zen.StatusError{StatusCode: http.StatusForbidden}},
			{response: okBody()},
		},
	}

	resp, selected, err := dialPaidWithOptionalProxy(ctx, proxies, keys, dialer, json.RawMessage(`{}`), false, "", zenpool.ProviderOpenCode)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer resp.Body.Close()
	if dialer.proxyCalls != 2 {
		t.Fatalf("proxyCalls = %d, want 2", dialer.proxyCalls)
	}
	if dialer.directCalls != 0 {
		t.Fatalf("directCalls = %d, want 0 when second proxy succeeds", dialer.directCalls)
	}
	if selected.ID == "" || selected.Label == "" {
		t.Fatalf("selected = %+v, want second proxy metadata", selected)
	}
}
