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
// Status 401/429/5xx and temporary network failures retry; client cancel and parent
// deadline exceeded do not. Network failover does not imply key cooldown (see ProxyPaid).
func ShouldFailover(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
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

// shouldMarkCooldown reports whether a failed attempt should cool the key in SQLite.
// Only upstream HTTP status failures (401/429/5xx path already filtered by ShouldFailover)
// mark cooldown; pure network errors failover without cooling the key identity.
func shouldMarkCooldown(err error) bool {
	var status *zen.StatusError
	return errors.As(err, &status)
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
//
// Failover policy:
//   - parent ctx cancel/deadline → stop, never cool the key
//   - StatusError 401 → process-memory MarkBench + try next key
//   - StatusError 429/5xx → MarkCooldown (MarkBench fallback if store fails) + try next
//   - pure network/timeout → try next key without MarkCooldown
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
		// Parent cancel/deadline: no failover and no cooldown (even if dialer returned another err).
		if ctx.Err() != nil || !ShouldFailover(err) {
			return nil, err
		}
		// 401 → process-memory bench (do not Delete, do not SQLite cooldown by default).
		if isUnauthorized(err) {
			service.MarkBench(selected.ID, DefaultBenchDuration)
			continue
		}
		// Status failures cool the key; network-only failures just try the next key.
		if shouldMarkCooldown(err) {
			if markErr := service.MarkCooldown(ctx, selected.ID, CooldownFor(err)); markErr != nil {
				// Persist failed: keep the key out of rotation in this process.
				service.MarkBench(selected.ID, DefaultBenchDuration)
			}
		}
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
