package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"jovepoxy/internal/proxypool"
	"jovepoxy/internal/zen"
	"jovepoxy/internal/zenpool"
)

// proxyCapableDialer is a chat dialer that can force a single egress proxy URL.
// *zen.Client implements this; tests may supply fakes with both methods.
type proxyCapableDialer interface {
	zenpool.ChatDialer
	ChatCompletionsWithProxy(ctx context.Context, auth zen.Auth, body json.RawMessage, stream bool, proxyURL *url.URL) (*http.Response, error)
}

// proxyBoundDialer pins every ChatCompletions call to a fixed egress proxy.
type proxyBoundDialer struct {
	inner    proxyCapableDialer
	proxyURL *url.URL
}

func (d proxyBoundDialer) ChatCompletions(ctx context.Context, auth zen.Auth, body json.RawMessage, stream bool) (*http.Response, error) {
	return d.inner.ChatCompletionsWithProxy(ctx, auth, body, stream, d.proxyURL)
}

// dialPaidWithOptionalProxy runs paid key-pool failover optionally via egress proxies.
// Flag off / nil proxies / no healthy proxy → direct ProxyPaid and empty Selected.
// On proxy-side failure: cool status failures, try one alternate proxy, then fall back to direct.
func dialPaidWithOptionalProxy(
	ctx context.Context,
	proxies *proxypool.Service,
	keys *zenpool.Service,
	dialer proxyCapableDialer,
	body json.RawMessage,
	stream bool,
	affinity string,
	provider zenpool.Provider,
) (*http.Response, proxypool.Selected, error) {
	if keys == nil {
		return nil, proxypool.Selected{}, zenpool.ErrNoHealthyKey
	}
	if dialer == nil {
		return nil, proxypool.Selected{}, zenpool.ErrNoHealthyKey
	}
	// Default / disabled: paid direct (historical behavior).
	if proxies == nil || !proxies.PaidUseProxyPool() {
		resp, err := zenpool.ProxyPaid(ctx, keys, dialer, body, stream, affinity, provider)
		return resp, proxypool.Selected{}, err
	}

	first, err := proxies.Acquire(ctx)
	if errors.Is(err, proxypool.ErrNoHealthyProxy) {
		resp, dialErr := zenpool.ProxyPaid(ctx, keys, dialer, body, stream, affinity, provider)
		return resp, proxypool.Selected{}, dialErr
	}
	if err != nil {
		return nil, proxypool.Selected{}, err
	}

	resp, err := zenpool.ProxyPaidEgress(ctx, keys, proxyBoundDialer{inner: dialer, proxyURL: first.URL}, body, stream, affinity, provider)
	if err == nil {
		return resp, first, nil
	}
	// Parent cancel/deadline or non-failover class: stop without cooling / direct fallback.
	if ctx.Err() != nil || !proxypool.ShouldFailover(err) {
		return resp, first, err
	}
	// Status failures cool the proxy; pure network errors may still switch egress.
	if proxyShouldMarkCooldown(err) {
		_ = proxies.MarkCooldown(ctx, first.ID, proxypool.CooldownFor(err))
	}

	second, acquireErr := proxies.AcquireExcluding(ctx, first.ID)
	if acquireErr == nil {
		resp, err = zenpool.ProxyPaidEgress(ctx, keys, proxyBoundDialer{inner: dialer, proxyURL: second.URL}, body, stream, affinity, provider)
		if err == nil {
			return resp, second, nil
		}
		if ctx.Err() != nil || !proxypool.ShouldFailover(err) {
			return resp, second, err
		}
		if proxyShouldMarkCooldown(err) {
			_ = proxies.MarkCooldown(ctx, second.ID, proxypool.CooldownFor(err))
		}
	}

	// Egress exhausted → same request falls back to direct (empty Selected).
	resp, err = zenpool.ProxyPaid(ctx, keys, dialer, body, stream, affinity, provider)
	return resp, proxypool.Selected{}, err
}

// proxyShouldMarkCooldown mirrors free-path: only HTTP StatusError cools the proxy.
func proxyShouldMarkCooldown(err error) bool {
	var status *zen.StatusError
	return errors.As(err, &status)
}
