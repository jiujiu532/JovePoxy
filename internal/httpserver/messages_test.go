package httpserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jovepoxy/internal/zen"
)

func TestServer_messages_sync_converts_openai_upstream(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer public" {
			t.Errorf("authorization = %q", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"hello from zen"}}],"usage":{"prompt_tokens":5,"completion_tokens":3}}`))
	}))
	defer upstream.Close()
	server := newServer(t, upstream.URL, []zen.Model{{ID: "demo-free"}})
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{
		"model":"demo-free","max_tokens":32,"messages":[{"role":"user","content":"hi"}]
	}`))
	request.Header.Set("x-api-key", server.key)
	recorder := httptest.NewRecorder()

	// When
	server.handler.ServeHTTP(recorder, request)

	// Then
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Type    string `json:"type"`
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Type != "message" || payload.Role != "assistant" || payload.StopReason != "end_turn" {
		t.Fatalf("payload = %+v", payload)
	}
	if len(payload.Content) != 1 || payload.Content[0].Text != "hello from zen" {
		t.Fatalf("content = %+v", payload.Content)
	}
}

func TestServer_messages_rejects_invalid_key_with_anthropic_error(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer upstream.Close()
	server := newServer(t, upstream.URL, []zen.Model{{ID: "demo-free"}})
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{
		"model":"demo-free","max_tokens":8,"messages":[{"role":"user","content":"hi"}]
	}`))
	request.Header.Set("x-api-key", "sk-oc-"+strings.Repeat("0", 64))
	recorder := httptest.NewRecorder()

	// When
	server.handler.ServeHTTP(recorder, request)

	// Then
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"authentication_error"`) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestServer_messages_rejects_paid_model_without_zen_keys(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer upstream.Close()
	server := newServer(t, upstream.URL, []zen.Model{{ID: "paid-model"}})
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{
		"model":"paid-model","max_tokens":8,"messages":[{"role":"user","content":"hi"}]
	}`))
	request.Header.Set("Authorization", "Bearer "+server.key)
	recorder := httptest.NewRecorder()

	// When
	server.handler.ServeHTTP(recorder, request)

	// Then
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "no zen API keys") {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestServer_messages_streams_anthropic_events(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"A\"}}]}\n\n"))
		_, _ = writer.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
	}))
	defer upstream.Close()
	server := newServer(t, upstream.URL, []zen.Model{{ID: "demo-free"}})
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{
		"model":"demo-free","max_tokens":16,"stream":true,"messages":[{"role":"user","content":"hi"}]
	}`))
	request.Header.Set("x-api-key", server.key)
	recorder := httptest.NewRecorder()

	// When
	server.handler.ServeHTTP(recorder, request)

	// Then
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "event: message_start") || !strings.Contains(body, `"text":"A"`) || !strings.Contains(body, "event: message_stop") {
		t.Fatalf("stream body = %s", body)
	}
}
