package zenpool

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"jovepoxy/internal/zen"
)

// ShouldFailover reports whether an upstream error should trigger another key attempt.
// Status 401/429/5xx, timeouts, and temporary network failures retry; client cancel does not.
func ShouldFailover(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var status *zen.StatusError
	if errors.As(err, &status) {
		code := status.StatusCode
		return code == http.StatusUnauthorized ||
			code == http.StatusTooManyRequests ||
			code >= http.StatusInternalServerError
	}
	// Network / dial / TLS / timeout failures (same spirit as free-path proxypool).
	return true
}

// CooldownFor returns the SQLite cooldown duration for a failed upstream status.
// 401 is handled via process-memory MarkBench in ProxyPaid; this still documents the legacy 5m value.
func CooldownFor(err error) time.Duration {
	var status *zen.StatusError
	if errors.As(err, &status) && status.StatusCode == http.StatusUnauthorized {
		return UnauthorizedCooldown
	}
	return DefaultCooldown
}

// isUnauthorized reports a 401 StatusError.
func isUnauthorized(err error) bool {
	var status *zen.StatusError
	return errors.As(err, &status) && status.StatusCode == http.StatusUnauthorized
}

// ChatDialer is the Zen chat surface used by paid-path failover.
type ChatDialer interface {
	ChatCompletions(ctx context.Context, auth zen.Auth, body json.RawMessage, stream bool) (*http.Response, error)
}

// ProxyPaid sends a chat request with up to MaxAttempts keys (default 2 = one failover).
// affinityKey is optional hashed conversation material for sticky policy; empty falls back to spread.
// provider empty defaults to ProviderOpenCode (AcquireFor compatibility).
func ProxyPaid(ctx context.Context, service *Service, dialer ChatDialer, body json.RawMessage, stream bool, affinityKey string, provider Provider) (*http.Response, error) {
	if service == nil {
		return nil, ErrNoHealthyKey
	}
	if provider == "" {
		provider = ProviderOpenCode
	}
	maxAttempts := service.MaxAttempts()
	policy := service.LoadPolicy()
	var lastErr error
	tried := make([]KeyID, 0, maxAttempts)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		selected, err := service.AcquireFor(ctx, AcquireOptions{
			Provider:    provider,
			Excluded:    tried,
			AffinityKey: affinityKey,
			Policy:      policy,
		})
		if err != nil {
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, err
		}
		response, err := dialWithKey(ctx, dialer, selected, body, stream)
		if err == nil {
			return response, nil
		}
		lastErr = err
		tried = append(tried, selected.ID)
		if !ShouldFailover(err) {
			return nil, err
		}
		// 401 → process-memory bench (do not Delete, do not SQLite cooldown by default).
		if isUnauthorized(err) {
			service.MarkBench(selected.ID, DefaultBenchDuration)
			continue
		}
		_ = service.MarkCooldown(ctx, selected.ID, CooldownFor(err))
	}
	return nil, lastErr
}

func dialWithKey(ctx context.Context, dialer ChatDialer, selected Selected, body json.RawMessage, stream bool) (*http.Response, error) {
	auth, err := zen.NewAPIKey(selected.Secret)
	if err != nil {
		return nil, err
	}
	return dialer.ChatCompletions(ctx, auth, body, stream)
}
