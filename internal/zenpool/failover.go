package zenpool

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"jovepoxy/internal/zen"
)

// ShouldFailover reports whether an upstream error should trigger one key retry.
func ShouldFailover(err error) bool {
	var status *zen.StatusError
	if !errors.As(err, &status) {
		return false
	}
	code := status.StatusCode
	return code == http.StatusUnauthorized ||
		code == http.StatusTooManyRequests ||
		code >= http.StatusInternalServerError
}

// CooldownFor returns the cooldown duration for a failed upstream status.
func CooldownFor(err error) time.Duration {
	var status *zen.StatusError
	if errors.As(err, &status) && status.StatusCode == http.StatusUnauthorized {
		return UnauthorizedCooldown
	}
	return DefaultCooldown
}

// ChatDialer is the Zen chat surface used by paid-path failover.
type ChatDialer interface {
	ChatCompletions(ctx context.Context, auth zen.Auth, body json.RawMessage, stream bool) (*http.Response, error)
}

// ProxyPaid sends a chat request with at most one failover to a different key.
func ProxyPaid(ctx context.Context, service *Service, dialer ChatDialer, body json.RawMessage, stream bool) (*http.Response, error) {
	first, err := service.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	response, err := dialWithKey(ctx, dialer, first, body, stream)
	if err == nil || !ShouldFailover(err) {
		return response, err
	}
	_ = service.MarkCooldown(ctx, first.ID, CooldownFor(err))
	second, acquireErr := service.AcquireExcluding(ctx, first.ID)
	if acquireErr != nil {
		return nil, err
	}
	return dialWithKey(ctx, dialer, second, body, stream)
}

func dialWithKey(ctx context.Context, dialer ChatDialer, selected Selected, body json.RawMessage, stream bool) (*http.Response, error) {
	auth, err := zen.NewAPIKey(selected.Secret)
	if err != nil {
		return nil, err
	}
	return dialer.ChatCompletions(ctx, auth, body, stream)
}
