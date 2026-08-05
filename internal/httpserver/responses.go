package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"jovepoxy/internal/responses"
	"jovepoxy/internal/usageparse"
)

// responsesHandler 实现 OpenAI Responses API（POST /v1/responses）。
// 入站 Responses 形状转 chat.completions 后复用 free/paid 转发链路，
// 出站再转回 Responses 对象 / SSE 事件流。
func (server server) responsesHandler(writer http.ResponseWriter, request *http.Request) {
	if !server.authorize(writer, request) {
		return
	}
	body, err := readRequestBody(writer, request)
	if err != nil {
		writeOpenAIError(writer, http.StatusBadRequest, err.Error(), "invalid_request_error", "", "invalid_request_error")
		return
	}
	parsed, err := responses.ParseRequest(body)
	if err != nil {
		writeOpenAIError(writer, http.StatusBadRequest, err.Error(), "invalid_request_error", "", "invalid_request_error")
		return
	}
	if server.catalog == nil || server.zen == nil {
		writeOpenAIError(writer, http.StatusBadGateway, "proxy is unavailable", "api_error", "", "upstream_error")
		return
	}
	result, err := server.catalog.List(request.Context())
	if err != nil {
		writeCatalogError(writer, err)
		return
	}
	free, providers, found := classifyModel(result.Models, parsed.Model)
	if !found {
		writeOpenAIError(writer, http.StatusBadRequest, "model is not available", "invalid_request_error", "model", "model_not_available")
		return
	}
	meta := requestMetaFrom(request.Context())
	meta.model = parsed.Model
	meta.stream = parsed.Stream
	observability := parsed.Observability()
	meta.maxTokens = observability.MaxTokens
	meta.reasoningEffort = observability.ReasoningEffort
	chatBody, err := responses.ToOpenAIChat(parsed)
	if err != nil {
		writeOpenAIError(writer, http.StatusBadRequest, err.Error(), "invalid_request_error", "", "invalid_request_error")
		return
	}
	response, provider, err := server.forwardChat(request.Context(), request, chatBody, parsed.Stream, free, providers)
	meta.upstream = upstreamChannel(free, provider)
	*request = *request.WithContext(withRequestMeta(request.Context(), meta))
	if err != nil {
		if writePaidRouteOpenAIError(writer, request.Context(), server.pool, err, provider) {
			return
		}
		writeUpstreamError(writer, err)
		return
	}
	defer response.Body.Close()
	if parsed.Stream {
		snap := responses.WriteStream(writer, response.Body, parsed.Model)
		storeUsage(request, snap)
		return
	}
	server.writeResponsesJSON(writer, response, request, parsed.Model)
}

func (server server) writeResponsesJSON(writer http.ResponseWriter, response *http.Response, request *http.Request, model string) {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		writeOpenAIError(writer, http.StatusBadGateway, "upstream response failed", "api_error", "", "upstream_error")
		return
	}
	if len(bytes.TrimSpace(body)) == 0 {
		writeOpenAIError(writer, http.StatusBadGateway, "upstream returned an empty response", "api_error", "", "upstream_error")
		return
	}
	if isOpenAIStyleRateLimit(body) {
		writeOpenAIError(writer, http.StatusTooManyRequests, "upstream rate limit exceeded", "rate_limit_error", "", "rate_limit_exceeded")
		return
	}
	snap := usageparse.ParseOpenAIUsage(body)
	if request != nil {
		storeUsage(request, snap)
	}
	converted, err := responses.FromOpenAI(body, model)
	if err != nil {
		writeOpenAIError(writer, http.StatusBadGateway, "invalid upstream response", "api_error", "", "upstream_error")
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(converted)
}
