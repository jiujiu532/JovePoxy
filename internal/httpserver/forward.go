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

func (server server) forwardChat(ctx context.Context, request *http.Request, body json.RawMessage, stream bool, free bool, provider models.Provider) (*http.Response, error) {
	provider = models.NormalizeProvider(provider)
	// Ollama never uses the Zen free/public path (Bearer public + egress proxy).
	if provider == models.ProviderOllama {
		free = false
	}
	if free {
		// Free models are IP-limited; rotate egress proxies when configured.
		return proxypool.ProxyFree(ctx, server.proxies, server.zen, body, stream)
	}
	if server.pool == nil {
		return nil, zenpool.ErrNoHealthyKey
	}
	affinity := zenpool.ConversationAffinityKey(request.Header, body)
	dialer := server.dialerFor(provider)
	if dialer == nil {
		// Misconfigured ollama dialer must not fall through to Zen with an Ollama key.
		return nil, zenpool.ErrNoHealthyKey
	}
	return zenpool.ProxyPaid(ctx, server.pool, dialer, body, stream, affinity, zenpool.Provider(provider))
}

func (server server) dialerFor(provider models.Provider) *zen.Client {
	if provider == models.ProviderOllama {
		// Never fall back to Zen: Ollama keys must not be sent to ZEN_BASE.
		return server.ollama
	}
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
