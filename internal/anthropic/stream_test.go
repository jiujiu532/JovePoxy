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
	if strings.Contains(body, `"type":"thinking"`) || strings.Contains(body, "thinking_delta") {
		t.Fatalf("unexpected thinking events without reasoning:\n%s", body)
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

func TestWriteStream_reasoning_then_text(t *testing.T) {
	// Given
	upstream := strings.NewReader(
		"data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"think\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n",
	)
	recorder := httptest.NewRecorder()

	// When
	anthropic.WriteStream(recorder, upstream, "demo-free", 1)

	// Then
	body := recorder.Body.String()
	for _, want := range []string{
		`"type":"thinking"`,
		`"type":"thinking_delta"`,
		`"thinking":"think"`,
		`"type":"text_delta"`,
		`"text":"Hi"`,
		"event: message_stop",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	thinkStart := strings.Index(body, `"type":"thinking"`)
	thinkDelta := strings.Index(body, `"type":"thinking_delta"`)
	textDelta := strings.Index(body, `"type":"text_delta"`)
	if thinkStart < 0 || thinkDelta < 0 || textDelta < 0 || !(thinkStart < thinkDelta && thinkDelta < textDelta) {
		t.Fatalf("expected thinking before text order, body:\n%s", body)
	}
	// thinking index 0, text index 1
	if !strings.Contains(body, `"index":0,"content_block":{"thinking":"","type":"thinking"}`) &&
		!strings.Contains(body, `"index":0`) {
		// still require text block starts at index 1 after thinking
	}
	if !strings.Contains(body, `"index":1`) {
		t.Fatalf("expected text block at index 1 after thinking:\n%s", body)
	}
}

func TestWriteStream_reasoning_field_alias(t *testing.T) {
	// Given — some upstreams use delta.reasoning instead of reasoning_content
	upstream := strings.NewReader(
		"data: {\"choices\":[{\"delta\":{\"reasoning\":\"plan\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n",
	)
	recorder := httptest.NewRecorder()

	// When
	anthropic.WriteStream(recorder, upstream, "demo-free", 1)

	// Then
	body := recorder.Body.String()
	if !strings.Contains(body, `"type":"thinking_delta"`) || !strings.Contains(body, `"thinking":"plan"`) {
		t.Fatalf("expected reasoning alias mapped to thinking:\n%s", body)
	}
}

func TestWriteStream_reasoning_then_tool(t *testing.T) {
	// Given
	upstream := strings.NewReader(
		"data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"use tool\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"name\":\"lookup\",\"arguments\":\"{}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n",
	)
	recorder := httptest.NewRecorder()

	// When
	anthropic.WriteStream(recorder, upstream, "demo-free", 1)

	// Then
	body := recorder.Body.String()
	for _, want := range []string{
		`"type":"thinking"`,
		`"thinking":"use tool"`,
		`"type":"tool_use"`,
		`"stop_reason":"tool_use"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	thinkPos := strings.Index(body, `"type":"thinking"`)
	toolPos := strings.Index(body, `"type":"tool_use"`)
	if thinkPos < 0 || toolPos < 0 || thinkPos > toolPos {
		t.Fatalf("expected thinking before tool_use:\n%s", body)
	}
}

func TestWriteStream_drops_late_reasoning_after_tool(t *testing.T) {
	// Given — tools 已占用 block 后迟到的 reasoning 应丢弃
	upstream := strings.NewReader(
		"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"name\":\"lookup\",\"arguments\":\"{}\"}}]}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"too late\"},\"finish_reason\":\"tool_calls\"}]}\n\n",
	)
	recorder := httptest.NewRecorder()

	// When
	anthropic.WriteStream(recorder, upstream, "demo-free", 1)

	// Then
	body := recorder.Body.String()
	if strings.Contains(body, "too late") || strings.Contains(body, `"type":"thinking"`) {
		t.Fatalf("late reasoning after tool must be dropped:\n%s", body)
	}
	if !strings.Contains(body, `"type":"tool_use"`) || !strings.Contains(body, `"stop_reason":"tool_use"`) {
		t.Fatalf("tool path should still complete:\n%s", body)
	}
}

func TestWriteStream_reasoning_only_closes_thinking(t *testing.T) {
	// Given — 仅 reasoning 无 text 也应正常收尾
	upstream := strings.NewReader(
		"data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"solo\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n",
	)
	recorder := httptest.NewRecorder()

	// When
	anthropic.WriteStream(recorder, upstream, "demo-free", 1)

	// Then
	body := recorder.Body.String()
	for _, want := range []string{
		`"type":"thinking"`,
		`"thinking":"solo"`,
		"event: content_block_stop",
		"event: message_stop",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	if strings.Contains(body, `"type":"text_delta"`) {
		t.Fatalf("reasoning-only must not invent text deltas:\n%s", body)
	}
}

func TestWriteStream_tool_then_text_closes_tools(t *testing.T) {
	// tool_use 后再 content：应先 content_block_stop 关闭 tool，再开 text
	upstream := strings.NewReader(
		"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"name\":\"lookup\",\"arguments\":\"{}\"}}]}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"after tool\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n",
	)
	recorder := httptest.NewRecorder()

	anthropic.WriteStream(recorder, upstream, "demo-free", 1)

	body := recorder.Body.String()
	for _, want := range []string{
		`"type":"tool_use"`,
		`"type":"text_delta"`,
		`"text":"after tool"`,
		"event: message_stop",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	// tool start 后、text start 前必须有 content_block_stop（关闭 tool）
	toolStart := strings.Index(body, `"type":"tool_use"`)
	textStart := strings.Index(body, `"type":"text"`)
	if toolStart < 0 || textStart < 0 || toolStart > textStart {
		t.Fatalf("expected tool_use before text block start:\n%s", body)
	}
	// 在 tool_use 与 text content_block 之间应出现 content_block_stop
	middle := body[toolStart:textStart]
	if !strings.Contains(middle, "content_block_stop") {
		t.Fatalf("expected content_block_stop between tool and text:\n%s", body)
	}
}


func TestWriteStream_first_event_error_not_completed(t *testing.T) {
	// OpenAI-style error first event must not become empty completed/stop stream.
	upstream := strings.NewReader("data: {\"error\":{\"type\":\"server_error\",\"message\":\"upstream boom\"}}\n\n")
	recorder := httptest.NewRecorder()
	anthropic.WriteStream(recorder, upstream, "demo-free", 1)
	if recorder.Code != 502 {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, "message_start") || strings.Contains(body, "event: message_stop") {
		t.Fatalf("error first event must not open SSE success stream:\n%s", body)
	}
	if !strings.Contains(body, "upstream boom") {
		t.Fatalf("expected error message in body: %s", body)
	}
}

func TestWriteStream_late_tool_name_backfill(t *testing.T) {
	// First delta opens tool without name; second fills name on same index.
	upstream := strings.NewReader(
		"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"arguments\":\"{\\\"q\\\"\"}}]}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"name\":\"lookup\",\"arguments\":\":1}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n",
	)
	recorder := httptest.NewRecorder()
	anthropic.WriteStream(recorder, upstream, "demo-free", 1)
	body := recorder.Body.String()
	if !strings.Contains(body, `"name":"lookup"`) {
		t.Fatalf("expected late tool name backfill:\n%s", body)
	}
	if !strings.Contains(body, `"stop_reason":"tool_use"`) {
		t.Fatalf("expected tool_use stop:\n%s", body)
	}
}
