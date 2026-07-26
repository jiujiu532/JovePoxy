package responses

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteStreamTextEvents(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"Hel"}}]}`,
		"",
		`data: {"choices":[{"delta":{"content":"lo"}}]}`,
		"",
		`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	recorder := httptest.NewRecorder()
	WriteStream(recorder, strings.NewReader(upstream), "demo")

	body := recorder.Body.String()
	for _, expected := range []string{
		"event: response.created",
		"event: response.output_item.added",
		`"delta":"Hel"`,
		`"delta":"lo"`,
		"event: response.output_text.done",
		`"text":"Hello"`,
		"event: response.completed",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in stream:\n%s", expected, body)
		}
	}
	if recorder.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("wrong content type: %s", recorder.Header().Get("Content-Type"))
	}
}

func TestWriteStreamToolCallEvents(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_x","function":{"name":"lookup","arguments":"{\"a\""}}]}}]}`,
		"",
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":":1}"}}]}}]}`,
		"",
		`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	recorder := httptest.NewRecorder()
	WriteStream(recorder, strings.NewReader(upstream), "demo")

	body := recorder.Body.String()
	for _, expected := range []string{
		"event: response.function_call_arguments.delta",
		"event: response.function_call_arguments.done",
		`"call_id":"call_x"`,
		`"name":"lookup"`,
		`"arguments":"{\"a\":1}"`,
		"event: response.completed",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in stream:\n%s", expected, body)
		}
	}
}

func TestWriteStreamEmptyUpstream(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteStream(recorder, strings.NewReader(""), "demo")
	if recorder.Code != 502 {
		t.Fatalf("expected 502 for empty upstream, got %d", recorder.Code)
	}
}
