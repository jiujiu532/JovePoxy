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
	return true
}

// shouldMarkCooldown and CooldownFor are retained for compatibility with callers;
// ProxyPaid now delegates health-aware cooling to RecordPaidOutcome.
func shouldMarkCooldown(err error) bool {
	var status *zen.StatusError
	return errors.As(err, &status)
}

func CooldownFor(err error) time.Duration {
	var status *zen.StatusError
	if errors.As(err, &status) && status.StatusCode == http.StatusUnauthorized {
		return UnauthorizedCooldown
	}
	return DefaultCooldown
}

func isUnauthorized(err error) bool {
	var status *zen.StatusError
	return errors.As(err, &status) && status.StatusCode == http.StatusUnauthorized
}

// ChatDialer is the Zen chat surface used by paid-path failover.
type ChatDialer interface {
	ChatCompletions(ctx context.Context, auth zen.Auth, body json.RawMessage, stream bool) (*http.Response, error)
}

// errEgressAttributed is recorded instead of 429/403/5xx when dialing through an
// egress proxy so key health is not punished for IP/proxy limits. Callers still
// see the original StatusError for failover decisions.
var errEgressAttributed = errors.New("zenpool: egress-attributed upstream failure")

// isEgressAttributableStatus reports status codes that usually reflect the
// egress IP/proxy rather than key identity (401 remains a key fault).
func isEgressAttributableStatus(err error) bool {
	var status *zen.StatusError
	if !errors.As(err, &status) {
		return false
	}
	code := status.StatusCode
	return code == http.StatusTooManyRequests ||
		code == http.StatusForbidden ||
		code >= http.StatusInternalServerError
}

// ProxyPaid sends a chat request with up to MaxAttempts keys. Each completed
// dial records only secret-free health metadata for the selected key.
func ProxyPaid(ctx context.Context, service *Service, dialer ChatDialer, body json.RawMessage, stream bool, affinityKey string, provider Provider) (*http.Response, error) {
	return proxyPaid(ctx, service, dialer, body, stream, affinityKey, provider, false)
}

// ProxyPaidEgress is like ProxyPaid but attributes 429/403/5xx to the egress path:
// those outcomes do not cool or score-punish keys (401 still does). Used when
// paid traffic is dialed through the admin proxy pool so direct fallback can still
// acquire the same keys.
func ProxyPaidEgress(ctx context.Context, service *Service, dialer ChatDialer, body json.RawMessage, stream bool, affinityKey string, provider Provider) (*http.Response, error) {
	return proxyPaid(ctx, service, dialer, body, stream, affinityKey, provider, true)
}

func proxyPaid(ctx context.Context, service *Service, dialer ChatDialer, body json.RawMessage, stream bool, affinityKey string, provider Provider, egressAttribution bool) (*http.Response, error) {
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
			Provider: provider, Excluded: tried, AffinityKey: affinityKey, Policy: policy, ForAttempt: true,
		})
		if err != nil {
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, err
		}
		started := service.clock.Now()
		service.noteAcquire(selected.ID, selected.Probing, started)
		response, err := dialWithKey(ctx, dialer, selected, body, stream)
		recordErr := err
		if err != nil && egressAttribution && isEgressAttributableStatus(err) {
			// Keep original err for caller failover; do not punish key identity.
			recordErr = errEgressAttributed
		}
		service.RecordPaidOutcome(ctx, selected, recordErr, service.clock.Now().Sub(started))
		if err == nil {
			return response, nil
		}
		lastErr = err
		tried = append(tried, selected.ID)
		if ctx.Err() != nil || !ShouldFailover(err) {
			return nil, err
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
