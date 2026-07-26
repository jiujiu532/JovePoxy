package proxypool

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"jovepoxy/internal/zen"
)

// FreeDialer is the Zen chat surface used for free-path proxy failover.
type FreeDialer interface {
	ChatCompletions(ctx context.Context, auth zen.Auth, body json.RawMessage, stream bool) (*http.Response, error)
	ChatCompletionsWithProxy(ctx context.Context, auth zen.Auth, body json.RawMessage, stream bool, proxyURL *url.URL) (*http.Response, error)
}

// ShouldFailover reports whether free-path should switch egress proxy.
func ShouldFailover(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var status *zen.StatusError
	if errors.As(err, &status) {
		return status.StatusCode == http.StatusTooManyRequests ||
			status.StatusCode == http.StatusForbidden ||
			status.StatusCode >= http.StatusInternalServerError
	}
	// Network / dial / proxy / timeout failures.
	return true
}

// CooldownFor maps failure class to proxy rest duration.
func CooldownFor(err error) time.Duration {
	var status *zen.StatusError
	if errors.As(err, &status) && status.StatusCode == http.StatusTooManyRequests {
		return RateLimitCooldown
	}
	return DefaultCooldown
}

// ProxyFree sends a free (public auth) chat request via egress proxy pool.
// If the pool is empty, it falls back to a direct Zen call.
// At most one failover to a different proxy is attempted.
func ProxyFree(ctx context.Context, pool *Service, dialer FreeDialer, body json.RawMessage, stream bool) (*http.Response, error) {
	if pool == nil {
		return dialer.ChatCompletions(ctx, zen.PublicAuth(), body, stream)
	}
	first, err := pool.Acquire(ctx)
	if errors.Is(err, ErrNoHealthyProxy) {
		return dialer.ChatCompletions(ctx, zen.PublicAuth(), body, stream)
	}
	if err != nil {
		return nil, err
	}
	response, err := dialer.ChatCompletionsWithProxy(ctx, zen.PublicAuth(), body, stream, first.URL)
	if err == nil || !ShouldFailover(err) {
		return response, err
	}
	// Do not failover cancellations from the client.
	if errors.Is(ctx.Err(), context.Canceled) {
		return response, err
	}
	_ = pool.MarkCooldown(ctx, first.ID, CooldownFor(err))
	second, acquireErr := pool.AcquireExcluding(ctx, first.ID)
	if acquireErr != nil {
		return nil, err
	}
	return dialer.ChatCompletionsWithProxy(ctx, zen.PublicAuth(), body, stream, second.URL)
}
