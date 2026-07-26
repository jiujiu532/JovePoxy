package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"jovepoxy/internal/proxypool"
	"jovepoxy/internal/zenpool"
)

func (server server) forwardChat(ctx context.Context, body json.RawMessage, stream bool, free bool) (*http.Response, error) {
	if free {
		// Free models are IP-limited; rotate egress proxies when configured.
		return proxypool.ProxyFree(ctx, server.proxies, server.zen, body, stream)
	}
	if server.pool == nil {
		return nil, zenpool.ErrNoHealthyKey
	}
	return zenpool.ProxyPaid(ctx, server.pool, server.zen, body, stream)
}

func paidRouteFailure(ctx context.Context, pool *zenpool.Service, err error) (int, string, bool) {
	if !errors.Is(err, zenpool.ErrNoHealthyKey) {
		return 0, "", false
	}
	message := "no healthy zen API key available for paid models"
	status := http.StatusServiceUnavailable
	if pool != nil {
		if list, listErr := pool.List(ctx); listErr == nil && len(list) == 0 {
			message = "no zen API keys configured for paid models"
			status = http.StatusBadRequest
		}
	}
	return status, message, true
}

func writePaidRouteOpenAIError(writer http.ResponseWriter, ctx context.Context, pool *zenpool.Service, err error) bool {
	status, message, ok := paidRouteFailure(ctx, pool, err)
	if !ok {
		return false
	}
	writeOpenAIError(writer, status, message, "invalid_request_error", "model", "zen_key_unavailable")
	return true
}

func writePaidRouteAnthropicError(writer http.ResponseWriter, ctx context.Context, pool *zenpool.Service, err error) bool {
	status, message, ok := paidRouteFailure(ctx, pool, err)
	if !ok {
		return false
	}
	writeAnthropicError(writer, status, "invalid_request_error", message)
	return true
}
