package httpserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jovepoxy/internal/zen"
)

func TestResponses_endpoint_converts_chat_completion(t *testing.T) {
	// Given：上游返回一个普通 chat.completion
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/chat/completions" && !strings.HasSuffix(request.URL.Path, "/chat/completions") {
			t.Errorf("unexpected upstream path: %s", request.URL.Path)
		}
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(request.Body).Decode(&body)
		if len(body.Messages) == 0 || body.Messages[0].Role != "user" || body.Messages[0].Content != "hello" {
			t.Errorf("input not converted to chat messages: %+v", body.Messages)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"world"}}],"usage":{"prompt_tokens":2,"completion_tokens":1}}`))
	}))
	defer upstream.Close()
	server := newServer(t, upstream.URL, []zen.Model{{ID: "demo-free"}})

	request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"demo-free","input":"hello"}`))
	request.Header.Set("Authorization", "Bearer "+server.key)
	recorder := httptest.NewRecorder()

	// When
	server.handler.ServeHTTP(recorder, request)

	// Then
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Object     string           `json:"object"`
		Status     string           `json:"status"`
		OutputText string           `json:"output_text"`
		Output     []map[string]any `json:"output"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Object != "response" || response.Status != "completed" || response.OutputText != "world" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if len(response.Output) != 1 || response.Output[0]["type"] != "message" {
		t.Fatalf("unexpected output items: %+v", response.Output)
	}
}

func TestResponses_endpoint_streams_events(t *testing.T) {
	// Given：上游返回 chat.completion.chunk SSE
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte(
			"data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
				"data: [DONE]\n\n"))
	}))
	defer upstream.Close()
	server := newServer(t, upstream.URL, []zen.Model{{ID: "demo-free"}})

	request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"demo-free","input":"hello","stream":true}`))
	request.Header.Set("Authorization", "Bearer "+server.key)
	recorder := httptest.NewRecorder()

	// When
	server.handler.ServeHTTP(recorder, request)

	// Then
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, expected := range []string{
		"event: response.created",
		"event: response.output_text.delta",
		"event: response.completed",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in stream:\n%s", expected, body)
		}
	}
}

func TestResponses_endpoint_rejects_unknown_model_and_bad_body(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer upstream.Close()
	server := newServer(t, upstream.URL, []zen.Model{{ID: "demo-free"}})

	cases := []struct {
		name string
		body string
		want int
	}{
		{"unknown model", `{"model":"nope","input":"hi"}`, http.StatusBadRequest},
		{"missing input", `{"model":"demo-free"}`, http.StatusBadRequest},
		{"invalid json", `{`, http.StatusBadRequest},
	}
	for _, testCase := range cases {
		request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(testCase.body))
		request.Header.Set("Authorization", "Bearer "+server.key)
		recorder := httptest.NewRecorder()
		server.handler.ServeHTTP(recorder, request)
		if recorder.Code != testCase.want {
			t.Fatalf("%s: status = %d, want %d", testCase.name, recorder.Code, testCase.want)
		}
	}
}
