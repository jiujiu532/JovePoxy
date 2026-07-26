package httpserver

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"jovepoxy/internal/keys"
	"jovepoxy/internal/models"
	"jovepoxy/internal/zen"
)

const maxChatRequestBytes = 4 << 20

type chatRequest struct {
	Model      string          `json:"model"`
	Messages   json.RawMessage `json:"messages"`
	Stream     bool            `json:"stream"`
	Tools      json.RawMessage `json:"tools"`
	ToolChoice json.RawMessage `json:"tool_choice"`
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
			response.Data = append(response.Data, modelObject{ID: string(model.ID), Object: "model", OwnedBy: "zen"})
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
	free, found := classifyModel(result.Models, parsed.Model)
	if !found {
		writeOpenAIError(writer, http.StatusBadRequest, "model is not available", "invalid_request_error", "model", "model_not_available")
		return
	}
	meta := requestMetaFrom(request.Context())
	meta.model = parsed.Model
	meta.stream = parsed.Stream
	*request = *request.WithContext(withRequestMeta(request.Context(), meta))
	response, err := server.forwardChat(request.Context(), body, parsed.Stream, free)
	if err != nil {
		if writePaidRouteOpenAIError(writer, request.Context(), server.pool, err) {
			return
		}
		writeUpstreamError(writer, err)
		return
	}
	defer response.Body.Close()
	if parsed.Stream {
		server.copyStream(writer, response.Body)
		return
	}
	server.copyJSON(writer, response)
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

func classifyModel(catalogModels []models.Model, requested string) (free bool, found bool) {
	for _, model := range catalogModels {
		if string(model.ID) == requested {
			return model.Free, true
		}
	}
	return false, false
}

func (server server) copyJSON(writer http.ResponseWriter, response *http.Response) {
	reader := bufio.NewReader(response.Body)
	firstByte, err := reader.ReadByte()
	if errors.Is(err, io.EOF) {
		writeOpenAIError(writer, http.StatusBadGateway, "upstream returned an empty response", "api_error", "", "upstream_error")
		return
	}
	if err != nil {
		writeOpenAIError(writer, http.StatusBadGateway, "upstream response failed", "api_error", "", "upstream_error")
		return
	}
	writer.Header().Set("Content-Type", contentType(response.Header.Get("Content-Type"), "application/json"))
	writer.WriteHeader(response.StatusCode)
	_, _ = writer.Write([]byte{firstByte})
	_, _ = io.Copy(writer, reader)
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
