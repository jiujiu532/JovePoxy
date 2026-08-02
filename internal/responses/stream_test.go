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

func TestWriteStreamReasoningThenText(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"choices":[{"delta":{"reasoning_content":"step "}}]}`,
		"",
		`data: {"choices":[{"delta":{"reasoning_content":"one"}}]}`,
		"",
		`data: {"choices":[{"delta":{"content":"Hi"}}]}`,
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
		`"type":"reasoning"`,
		"event: response.reasoning_summary_part.added",
		"event: response.reasoning_summary_text.delta",
		`"delta":"step "`,
		`"delta":"one"`,
		"event: response.reasoning_summary_text.done",
		`"text":"step one"`,
		"event: response.reasoning_summary_part.done",
		"event: response.output_item.done",
		`"delta":"Hi"`,
		"event: response.output_text.done",
		"event: response.completed",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in stream:\n%s", expected, body)
		}
	}
	// reasoning 事件应出现在首个文本 delta 之前
	reasonIdx := strings.Index(body, `"delta":"step "`)
	textIdx := strings.Index(body, `"delta":"Hi"`)
	if reasonIdx < 0 || textIdx < 0 || reasonIdx > textIdx {
		t.Fatalf("reasoning should precede text; reason=%d text=%d\n%s", reasonIdx, textIdx, body)
	}
	// completed.output 含 reasoning 与 message
	if !strings.Contains(body, `"type":"reasoning"`) || !strings.Contains(body, `"type":"message"`) {
		t.Fatalf("completed output should retain reasoning and message:\n%s", body)
	}
}

func TestWriteStreamReasoningFieldAlias(t *testing.T) {
	// 兼容仅提供 delta.reasoning（无 _content 后缀）
	upstream := strings.Join([]string{
		`data: {"choices":[{"delta":{"reasoning":"alias"}}]}`,
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
		"event: response.reasoning_summary_text.delta",
		`"delta":"alias"`,
		"event: response.reasoning_summary_text.done",
		`"text":"alias"`,
		"event: response.completed",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in stream:\n%s", expected, body)
		}
	}
}

func TestWriteStreamReasoningThenTool(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"choices":[{"delta":{"reasoning_content":"plan"}}]}`,
		"",
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_x","function":{"name":"lookup","arguments":"{}"}}]}}]}`,
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
		"event: response.reasoning_summary_text.delta",
		"event: response.reasoning_summary_text.done",
		"event: response.function_call_arguments.delta",
		`"name":"lookup"`,
		"event: response.completed",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in stream:\n%s", expected, body)
		}
	}
	reasonDone := strings.Index(body, "event: response.reasoning_summary_text.done")
	toolDelta := strings.Index(body, "event: response.function_call_arguments.delta")
	if reasonDone < 0 || toolDelta < 0 || reasonDone > toolDelta {
		t.Fatalf("reasoning must close before tool; reasonDone=%d tool=%d\n%s", reasonDone, toolDelta, body)
	}
}

func TestWriteStreamDropsLateReasoningAfterTool(t *testing.T) {
	// tools 已开始后迟到的 reasoning 应丢弃，避免与 function_call 争用 output_index
	upstream := strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_x","function":{"name":"lookup","arguments":"{}"}}]}}]}`,
		"",
		`data: {"choices":[{"delta":{"reasoning_content":"too late"}}]}`,
		"",
		`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	recorder := httptest.NewRecorder()
	WriteStream(recorder, strings.NewReader(upstream), "demo")
	body := recorder.Body.String()
	if strings.Contains(body, "too late") || strings.Contains(body, "response.reasoning_summary_text.delta") {
		t.Fatalf("late reasoning after tool must be dropped:\n%s", body)
	}
	if !strings.Contains(body, `"name":"lookup"`) || !strings.Contains(body, "event: response.completed") {
		t.Fatalf("tool path should still complete:\n%s", body)
	}
}

func TestWriteStreamNoReasoningHasNoReasoningEvents(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"Hi"}}]}`,
		"",
		`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	recorder := httptest.NewRecorder()
	WriteStream(recorder, strings.NewReader(upstream), "demo")
	body := recorder.Body.String()
	for _, banned := range []string{
		`"type":"reasoning"`,
		"response.reasoning_summary",
	} {
		if strings.Contains(body, banned) {
			t.Fatalf("unexpected reasoning marker %q:\n%s", banned, body)
		}
	}
}
