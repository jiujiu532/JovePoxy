package anthropic_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"jovepoxy/internal/anthropic"
)

func TestWriteStream_emits_message_and_text_events(t *testing.T) {
	// Given
	upstream := strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n")
	recorder := httptest.NewRecorder()

	// When
	anthropic.WriteStream(recorder, upstream, "demo-free", 4)

	// Then
	body := recorder.Body.String()
	for _, want := range []string{
		"event: message_start",
		"event: content_block_start",
		"event: content_block_delta",
		`"text":"Hi"`,
		"event: content_block_stop",
		"event: message_delta",
		`"stop_reason":"end_turn"`,
		"event: message_stop",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("stream body missing %q:\n%s", want, body)
		}
	}
	if recorder.Code != 200 {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestWriteStream_maps_first_event_rate_limit(t *testing.T) {
	// Given
	upstream := strings.NewReader("data: {\"type\":\"FreeUsageLimitError\"}\n\n")
	recorder := httptest.NewRecorder()

	// When
	anthropic.WriteStream(recorder, upstream, "demo-free", 1)

	// Then
	if recorder.Code != 429 {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "rate_limit_error") {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestWriteStream_maps_tool_call_deltas(t *testing.T) {
	// Given
	upstream := strings.NewReader(
		"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"name\":\"lookup\",\"arguments\":\"{\\\"q\\\"\"}}]}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\":1}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n",
	)
	recorder := httptest.NewRecorder()

	// When
	anthropic.WriteStream(recorder, upstream, "demo-free", 2)

	// Then
	body := recorder.Body.String()
	for _, want := range []string{
		`"type":"tool_use"`,
		`"partial_json":"{\"q\""`,
		`"stop_reason":"tool_use"`,
		"event: message_stop",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}
