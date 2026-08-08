package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"jovepoxy/internal/models"
	"jovepoxy/internal/proxypool"
	"jovepoxy/internal/zen"
	"jovepoxy/internal/zenpool"
)

// forwardChat routes free traffic to public Zen, and paid traffic through key pools.
// providers lists every paid source advertising the model ID (primary + overlap).
// Dual-provider IDs are rotated request-by-request; on pool/upstream failure the next
// provider is tried. The returned provider is the one that produced the response
// (or the last attempted provider when all fail).
func (server server) forwardChat(ctx context.Context, request *http.Request, body json.RawMessage, stream bool, free bool, providers []models.Provider) (*http.Response, models.Provider, proxypool.Selected, error) {
	if free {
		// Free models are IP-limited; rotate egress proxies when configured.
		resp, selected, err := proxypool.ProxyFree(ctx, server.proxies, server.zen, body, stream)
		return resp, models.ProviderOpenCode, selected, err
	}
	if server.pool == nil {
		return nil, firstProvider(providers), proxypool.Selected{}, zenpool.ErrNoHealthyKey
	}
	order := server.rotateProviders(providers)
	affinity := zenpool.ConversationAffinityKey(request.Header, body)
	var lastErr error
	lastProvider := firstProvider(order)
	for _, provider := range order {
		provider = models.NormalizeProvider(provider)
		lastProvider = provider
		dialer := server.dialerFor(provider)
		if dialer == nil {
			lastErr = zenpool.ErrNoHealthyKey
			continue
		}
		response, err := zenpool.ProxyPaid(ctx, server.pool, dialer, body, stream, affinity, zenpool.Provider(provider))
		if err == nil {
			return response, provider, proxypool.Selected{}, nil
		}
		lastErr = err
		// Client gone → stop; do not burn the next pool.
		if ctx.Err() != nil {
			return nil, provider, proxypool.Selected{}, err
		}
		// Cross-provider: always try the next dual source. ProxyPaid already
		// exhausted in-pool key failover. Intra-pool ShouldFailover is intentionally
		// narrower (401/429/5xx); here 4xx like Ollama 404/400 must still hand off
		// so the sibling pool can serve the same model ID.
		continue
	}
	if lastErr == nil {
		lastErr = zenpool.ErrNoHealthyKey
	}
	return nil, lastProvider, proxypool.Selected{}, lastErr
}

func firstProvider(providers []models.Provider) models.Provider {
	if len(providers) == 0 {
		return models.ProviderOpenCode
	}
	return models.NormalizeProvider(providers[0])
}

// rotateProviders returns providers starting at the next RR slot.
// Single-provider lists are returned as-is (no counter bump needed for fairness).
func (server server) rotateProviders(providers []models.Provider) []models.Provider {
	normalized := make([]models.Provider, 0, len(providers))
	seen := make(map[models.Provider]struct{}, len(providers))
	for _, p := range providers {
		p = models.NormalizeProvider(p)
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		normalized = append(normalized, p)
	}
	if len(normalized) == 0 {
		return []models.Provider{models.ProviderOpenCode}
	}
	if len(normalized) == 1 {
		return normalized
	}
	var start uint64
	if server.providerRR != nil {
		start = server.providerRR.Add(1) - 1
	}
	out := make([]models.Provider, len(normalized))
	n := uint64(len(normalized))
	for i := uint64(0); i < n; i++ {
		out[i] = normalized[(start+i)%n]
	}
	return out
}

func (server server) dialerFor(provider models.Provider) *zen.Client {
	if provider == models.ProviderOllama {
		// Never fall back to Zen: Ollama keys must not be sent to ZEN_BASE.
		return server.ollama
	}
	// Paid OpenCode Go suite uses /zen/go, not public /zen/v1.
	if server.zenGo != nil {
		return server.zenGo
	}
	// Fallback for tests that only wire Zen (free base); production always sets ZenGo.
	return server.zen
}

func paidRouteFailure(ctx context.Context, pool *zenpool.Service, err error, provider models.Provider) (int, string, bool) {
	if !errors.Is(err, zenpool.ErrNoHealthyKey) {
		return 0, "", false
	}
	provider = models.NormalizeProvider(provider)
	message := "no healthy zen API key available for paid models"
	status := http.StatusServiceUnavailable
	if provider == models.ProviderOllama {
		message = "no healthy ollama API key available for paid models"
	}
	if pool != nil {
		list, listErr := pool.ListByProvider(ctx, zenpool.Provider(provider))
		if listErr == nil && len(list) == 0 {
			if provider == models.ProviderOllama {
				message = "no ollama API keys configured for paid models"
			} else {
				message = "no zen API keys configured for paid models"
			}
			status = http.StatusBadRequest
		}
	}
	return status, message, true
}

func writePaidRouteOpenAIError(writer http.ResponseWriter, ctx context.Context, pool *zenpool.Service, err error, provider models.Provider) bool {
	status, message, ok := paidRouteFailure(ctx, pool, err, provider)
	if !ok {
		return false
	}
	writeOpenAIError(writer, status, message, "invalid_request_error", "model", "zen_key_unavailable")
	return true
}

func writePaidRouteAnthropicError(writer http.ResponseWriter, ctx context.Context, pool *zenpool.Service, err error, provider models.Provider) bool {
	status, message, ok := paidRouteFailure(ctx, pool, err, provider)
	if !ok {
		return false
	}
	writeAnthropicError(writer, status, "invalid_request_error", message)
	return true
}
