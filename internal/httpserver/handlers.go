package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"jovepoxy/internal/effort"
	"jovepoxy/internal/keys"
	"jovepoxy/internal/models"
	"jovepoxy/internal/usageparse"
	"jovepoxy/internal/zen"
)

const maxChatRequestBytes = 4 << 20

type chatRequest struct {
	Model           string          `json:"model"`
	Messages        json.RawMessage `json:"messages"`
	Stream          bool            `json:"stream"`
	MaxTokens       int             `json:"max_tokens"`
	ReasoningEffort string          `json:"reasoning_effort"`
	Tools           json.RawMessage `json:"tools"`
	ToolChoice      json.RawMessage `json:"tool_choice"`
}

type modelsResponse struct {
	Object string        `json:"object"`
	Data   []modelObject `json:"data"`
}

type modelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

type healthResponse struct {
	Status     string         `json:"status"`
	Version    string         `json:"version"`
	ModelCount int            `json:"model_count"`
	Upstream   upstreamHealth `json:"upstream"`
}

type upstreamHealth struct {
	Status string `json:"status"`
}

func (server server) health(writer http.ResponseWriter, request *http.Request) {
	if server.catalog == nil {
		writeOpenAIError(writer, http.StatusBadGateway, "model catalog is unavailable", "api_error", "", "upstream_error")
		return
	}
	result, err := server.catalog.List(request.Context())
	if err != nil {
		writeCatalogError(writer, err)
		return
	}
	upstreamStatus := "ok"
	if result.Stale {
		upstreamStatus = "stale"
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(healthResponse{
		Status: "ok", Version: server.version, ModelCount: len(result.Models),
		Upstream: upstreamHealth{Status: upstreamStatus},
	})
}

func (server server) listModels(writer http.ResponseWriter, request *http.Request) {
	if server.catalog == nil {
		writeOpenAIError(writer, http.StatusBadGateway, "model catalog is unavailable", "api_error", "", "upstream_error")
		return
	}
	result, err := server.catalog.List(request.Context())
	if err != nil {
		writeCatalogError(writer, err)
		return
	}
	response := modelsResponse{Object: "list", Data: make([]modelObject, 0, len(result.Models))}
	for _, model := range result.Models {
		if model.Free || server.showAllModels {
			response.Data = append(response.Data, modelObject{ID: string(model.ID), Object: "model", OwnedBy: ownedBy(model.Provider)})
		}
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(response)
}

func (server server) chatCompletions(writer http.ResponseWriter, request *http.Request) {
	if !server.authorize(writer, request) {
		return
	}
	body, parsed, err := parseChatRequest(writer, request)
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
	// Clamp reasoning_effort to labels the target model accepts (no random xhigh/max).
	body, mappedEffort := applyReasoningEffort(body, parsed.Model)
	meta := requestMetaFrom(request.Context())
	meta.model = parsed.Model
	meta.stream = parsed.Stream
	meta.maxTokens = parsed.MaxTokens
	meta.reasoningEffort = mappedEffort
	// Stream usage is often omitted unless the client opts in; inject for logging.
	body = ensureStreamIncludeUsage(body, parsed.Stream)
	response, provider, selected, err := server.forwardChat(request.Context(), request, body, parsed.Stream, free, providers)
	meta.upstream = upstreamChannel(free, provider)
	meta.proxyID = string(selected.ID)
	meta.proxyLabel = selected.Label
	meta.proxyHost = selected.Host
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
		snap := server.copyStream(writer, response.Body)
		storeUsage(request, snap)
		return
	}
	server.copyJSON(writer, response, request)
}

func (server server) authorize(writer http.ResponseWriter, request *http.Request) bool {
	if server.keys == nil {
		writeOpenAIError(writer, http.StatusUnauthorized, "invalid API key", "authentication_error", "", "invalid_api_key")
		return false
	}
	verified, err := server.keys.Verify(request.Context(), keys.Credentials{
		Authorization: request.Header.Get("Authorization"), APIKey: request.Header.Get("x-api-key"),
	})
	if errors.Is(err, keys.ErrRateLimited) {
		writeOpenAIError(writer, http.StatusTooManyRequests, "local API key rate limit exceeded", "rate_limit_error", "", "rate_limit_exceeded")
		return false
	}
	if err != nil {
		writeOpenAIError(writer, http.StatusUnauthorized, "invalid API key", "authentication_error", "", "invalid_api_key")
		return false
	}
	*request = *request.WithContext(withRequestMeta(request.Context(), requestMeta{keyID: string(verified.ID)}))
	return true
}

// applyReasoningEffort rewrites top-level reasoning_effort (and nested reasoning.effort)
// to a model-safe label. Returns the possibly-modified body and the effort that will be sent
// (empty when the field is omitted). Bodies without effort fields are returned unchanged.
func applyReasoningEffort(body []byte, model string) ([]byte, string) {
	if len(bytes.TrimSpace(body)) == 0 {
		return body, ""
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil {
		return body, ""
	}
	_, hasTop := object["reasoning_effort"]
	_, hasNested := object["reasoning"]
	if !hasTop && !hasNested {
		return body, ""
	}
	rawEffort := ""
	if nested, ok := object["reasoning"]; ok {
		var reasoning struct {
			Effort string `json:"effort"`
		}
		if err := json.Unmarshal(nested, &reasoning); err == nil {
			rawEffort = reasoning.Effort
		}
	}
	if top, ok := object["reasoning_effort"]; ok {
		var s string
		if err := json.Unmarshal(top, &s); err == nil && strings.TrimSpace(s) != "" {
			rawEffort = s
		}
	}
	if strings.TrimSpace(rawEffort) == "" {
		// Nested reasoning without effort — leave body alone.
		return body, ""
	}
	mapped := effort.MapForModel(model, rawEffort)
	if mapped == "" {
		delete(object, "reasoning_effort")
		// Strip effort from nested reasoning object if present; keep other keys.
		if nested, ok := object["reasoning"]; ok {
			var reasoning map[string]json.RawMessage
			if err := json.Unmarshal(nested, &reasoning); err == nil {
				delete(reasoning, "effort")
				if len(reasoning) == 0 {
					delete(object, "reasoning")
				} else if encoded, err := json.Marshal(reasoning); err == nil {
					object["reasoning"] = encoded
				}
			}
		}
	} else {
		if encoded, err := json.Marshal(mapped); err == nil {
			object["reasoning_effort"] = encoded
		}
		if nested, ok := object["reasoning"]; ok {
			var reasoning map[string]json.RawMessage
			if err := json.Unmarshal(nested, &reasoning); err == nil {
				if encoded, err := json.Marshal(mapped); err == nil {
					reasoning["effort"] = encoded
					if out, err := json.Marshal(reasoning); err == nil {
						object["reasoning"] = out
					}
				}
			}
		}
	}
	encoded, err := json.Marshal(object)
	if err != nil {
		return body, mapped
	}
	return encoded, mapped
}

// ensureStreamIncludeUsage injects stream_options.include_usage when streaming and absent.
// Leaves non-stream bodies and already-configured clients untouched.
func ensureStreamIncludeUsage(body []byte, stream bool) []byte {
	if !stream || len(bytes.TrimSpace(body)) == 0 {
		return body
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil {
		return body
	}
	if _, exists := object["stream_options"]; exists {
		return body
	}
	object["stream_options"] = json.RawMessage(`{"include_usage":true}`)
	encoded, err := json.Marshal(object)
	if err != nil {
		return body
	}
	return encoded
}

func parseChatRequest(writer http.ResponseWriter, request *http.Request) (json.RawMessage, chatRequest, error) {
	defer request.Body.Close()
	body, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, maxChatRequestBytes))
	if err != nil {
		return nil, chatRequest{}, fmt.Errorf("invalid JSON request")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	var parsed chatRequest
	if err := decoder.Decode(&parsed); err != nil {
		return nil, chatRequest{}, fmt.Errorf("invalid JSON request")
	}
	var extra struct{}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, chatRequest{}, fmt.Errorf("invalid JSON request")
	}
	if strings.TrimSpace(parsed.Model) == "" {
		return nil, chatRequest{}, fmt.Errorf("model is required")
	}
	if messages := bytes.TrimSpace(parsed.Messages); len(messages) == 0 || messages[0] != '[' {
		return nil, chatRequest{}, fmt.Errorf("messages must be an array")
	}
	return json.RawMessage(body), parsed, nil
}

// classifyModel returns free flag and the full provider set for routing.
// Dual-provider IDs expose both OpenCode and Ollama so forwardChat can RR/failover.
func classifyModel(catalogModels []models.Model, requested string) (free bool, providers []models.Provider, found bool) {
	for _, model := range catalogModels {
		if string(model.ID) == requested {
			providers = models.ProvidersOf(model)
			if len(providers) == 0 {
				providers = []models.Provider{models.NormalizeProvider(model.Provider)}
			}
			free = model.Free
			// Ollama-only (or dual with ollama primary) never uses free/public path.
			if models.NormalizeProvider(model.Provider) == models.ProviderOllama {
				free = false
			}
			return free, providers, true
		}
	}
	return false, []models.Provider{models.ProviderOpenCode}, false
}

// upstreamChannel labels the data-plane path for request logs (not the HTTP API path).
func upstreamChannel(free bool, provider models.Provider) string {
	provider = models.NormalizeProvider(provider)
	if provider == models.ProviderOllama {
		return "ollama_paid"
	}
	if free {
		return "opencode_free"
	}
	return "opencode_paid"
}

func ownedBy(provider models.Provider) string {
	if models.NormalizeProvider(provider) == models.ProviderOllama {
		return "ollama"
	}
	return "zen"
}

func (server server) copyJSON(writer http.ResponseWriter, response *http.Response, request *http.Request) {
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
	if request != nil {
		storeUsage(request, usageparse.ParseOpenAIUsage(body))
	}
	writer.Header().Set("Content-Type", contentType(response.Header.Get("Content-Type"), "application/json"))
	writer.WriteHeader(response.StatusCode)
	_, _ = writer.Write(body)
}

func writeCatalogError(writer http.ResponseWriter, err error) {
	var timeout *zen.TimeoutError
	if errors.As(err, &timeout) {
		writeOpenAIError(writer, http.StatusGatewayTimeout, "model catalog timed out", "api_error", "", "upstream_timeout")
		return
	}
	writeOpenAIError(writer, http.StatusBadGateway, "model catalog is unavailable", "api_error", "", "upstream_error")
}

func writeUpstreamError(writer http.ResponseWriter, err error) {
	var timeout *zen.TimeoutError
	if errors.As(err, &timeout) {
		writeOpenAIError(writer, http.StatusGatewayTimeout, "upstream request timed out", "api_error", "", "upstream_timeout")
		return
	}
	var status *zen.StatusError
	if errors.As(err, &status) && status.StatusCode == http.StatusTooManyRequests {
		writeOpenAIError(writer, http.StatusTooManyRequests, "upstream rate limit exceeded", "rate_limit_error", "", "rate_limit_exceeded")
		return
	}
	writeOpenAIError(writer, http.StatusBadGateway, "upstream request failed", "api_error", "", "upstream_error")
}

func contentType(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
