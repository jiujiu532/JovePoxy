package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"jovepoxy/internal/anthropic"
	"jovepoxy/internal/keys"
	"jovepoxy/internal/sse"
	"jovepoxy/internal/usageparse"
	"jovepoxy/internal/zen"
)

func (server server) messages(writer http.ResponseWriter, request *http.Request) {
	if !server.authorizeAnthropic(writer, request) {
		return
	}
	body, err := readRequestBody(writer, request)
	if err != nil {
		writeAnthropicError(writer, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	parsed, err := anthropic.ParseRequest(body)
	if err != nil {
		writeAnthropicError(writer, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if server.catalog == nil || server.zen == nil {
		writeAnthropicError(writer, http.StatusBadGateway, "api_error", "proxy is unavailable")
		return
	}
	result, err := server.catalog.List(request.Context())
	if err != nil {
		writeAnthropicCatalogError(writer, err)
		return
	}
	free, provider, found := classifyModel(result.Models, parsed.Model)
	if !found {
		writeAnthropicError(writer, http.StatusBadRequest, "invalid_request_error", "model is not available")
		return
	}
	meta := requestMetaFrom(request.Context())
	meta.model = parsed.Model
	meta.stream = parsed.Stream
	observability := parsed.Observability()
	meta.maxTokens = observability.MaxTokens
	meta.reasoningEffort = observability.ReasoningEffort
	meta.thinkingType = observability.ThinkingType
	meta.budgetTokens = observability.BudgetTokens
	*request = *request.WithContext(withRequestMeta(request.Context(), meta))
	openAIBody, inputTokens, err := anthropic.ToOpenAIChat(parsed)
	if err != nil {
		writeAnthropicError(writer, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	response, err := server.forwardChat(request.Context(), request, openAIBody, parsed.Stream, free, provider)
	if err != nil {
		if writePaidRouteAnthropicError(writer, request.Context(), server.pool, err, provider) {
			return
		}
		writeAnthropicUpstreamError(writer, err)
		return
	}
	defer response.Body.Close()
	if parsed.Stream {
		snap := anthropic.WriteStream(writer, response.Body, parsed.Model, inputTokens)
		storeUsage(request, snap)
		return
	}
	server.writeAnthropicJSON(writer, response, request, parsed.Model, inputTokens)
}

func (server server) authorizeAnthropic(writer http.ResponseWriter, request *http.Request) bool {
	if server.keys == nil {
		writeAnthropicError(writer, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return false
	}
	verified, err := server.keys.Verify(request.Context(), keys.Credentials{
		Authorization: request.Header.Get("Authorization"), APIKey: request.Header.Get("x-api-key"),
	})
	if errors.Is(err, keys.ErrRateLimited) {
		writeAnthropicError(writer, http.StatusTooManyRequests, "rate_limit_error", "local API key rate limit exceeded")
		return false
	}
	if err != nil {
		writeAnthropicError(writer, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return false
	}
	*request = *request.WithContext(withRequestMeta(request.Context(), requestMeta{keyID: string(verified.ID)}))
	return true
}

func (server server) writeAnthropicJSON(writer http.ResponseWriter, response *http.Response, request *http.Request, model string, inputTokens int) {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		writeAnthropicError(writer, http.StatusBadGateway, "upstream_error", "upstream response failed")
		return
	}
	if len(bytes.TrimSpace(body)) == 0 {
		writeAnthropicError(writer, http.StatusBadGateway, "upstream_error", "Empty response")
		return
	}
	if isOpenAIStyleRateLimit(body) {
		writeAnthropicError(writer, http.StatusTooManyRequests, "rate_limit_error", "upstream rate limit exceeded (free model rate limit)")
		return
	}
	// Parse usage from upstream OpenAI body before convert (single source for logs + client).
	snap := usageparse.ParseOpenAIUsage(body)
	if request != nil {
		storeUsage(request, snap)
	}
	message, err := anthropic.FromOpenAI(body, model, inputTokens)
	if err != nil {
		writeAnthropicError(writer, http.StatusBadGateway, "upstream_error", "Invalid upstream response")
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(message)
}

func readRequestBody(writer http.ResponseWriter, request *http.Request) ([]byte, error) {
	defer request.Body.Close()
	body, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, maxChatRequestBytes))
	if err != nil {
		return nil, errors.New("invalid JSON request")
	}
	return body, nil
}

func writeAnthropicError(writer http.ResponseWriter, status int, errorType, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"type": "error", "error": map[string]string{"type": errorType, "message": message},
	})
}

func writeAnthropicCatalogError(writer http.ResponseWriter, err error) {
	var timeout *zen.TimeoutError
	if errors.As(err, &timeout) {
		writeAnthropicError(writer, http.StatusGatewayTimeout, "timeout_error", "model catalog timed out")
		return
	}
	writeAnthropicError(writer, http.StatusBadGateway, "upstream_error", "model catalog is unavailable")
}

func writeAnthropicUpstreamError(writer http.ResponseWriter, err error) {
	var timeout *zen.TimeoutError
	if errors.As(err, &timeout) {
		writeAnthropicError(writer, http.StatusGatewayTimeout, "timeout_error", "Upstream timeout")
		return
	}
	var status *zen.StatusError
	if errors.As(err, &status) && status.StatusCode == http.StatusTooManyRequests {
		writeAnthropicError(writer, http.StatusTooManyRequests, "rate_limit_error", "upstream rate limit exceeded (free model rate limit)")
		return
	}
	writeAnthropicError(writer, http.StatusBadGateway, "upstream_error", "upstream request failed")
}

func isOpenAIStyleRateLimit(body []byte) bool {
	var envelope struct {
		Choices json.RawMessage `json:"choices"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return false
	}
	// Successful chat-style payloads must not be treated as rate-limit errors.
	if len(bytes.TrimSpace(envelope.Choices)) > 0 {
		return false
	}
	return sse.IsRateLimitPayload(body)
}
