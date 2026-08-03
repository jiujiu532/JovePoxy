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

func TestWriteStreamInterleavedDualToolCalls(t *testing.T) {
	// OpenAI 可在两个 tool 都 opened 后交错推送 arguments；args 不得串台。
	upstream := strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_a","function":{"name":"lookup","arguments":"{\"q\""}}]}}]}`,
		"",
		`data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_b","function":{"name":"search","arguments":"{\"k\""}}]}}]}`,
		"",
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\"alpha\"}"}}]}}]}`,
		"",
		`data: {"choices":[{"delta":{"tool_calls":[{"index":1,"function":{"arguments":":\"beta\"}"}}]}}]}`,
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
		`"call_id":"call_a"`,
		`"name":"lookup"`,
		`"arguments":"{\"q\":\"alpha\"}"`,
		`"call_id":"call_b"`,
		`"name":"search"`,
		`"arguments":"{\"k\":\"beta\"}"`,
		"event: response.completed",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in interleaved dual-tool stream:\n%s", expected, body)
		}
	}
	// 两个 function_call_arguments.done 都在，且 arguments 不得交叉拼接
	if strings.Contains(body, `"arguments":"{\"q\":\"alpha\"}{\"k\":\"beta\"}"`) ||
		strings.Contains(body, `"arguments":"{\"k\":\"beta\"}{\"q\":\"alpha\"}"`) ||
		strings.Contains(body, `"arguments":"{\"q\":\"alpha\",\"k\":\"beta\"}"`) {
		t.Fatalf("tool arguments appear concatenated/cross-contaminated:\n%s", body)
	}
	if strings.Count(body, "event: response.function_call_arguments.done") != 2 {
		t.Fatalf("expected 2 function_call_arguments.done events:\n%s", body)
	}
}

func TestWriteStreamTextReasoningTextResetsMessageBuffer(t *testing.T) {
	// text → late reasoning → text：第二段 message 不得拼接第一段。
	upstream := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"first"}}]}`,
		"",
		`data: {"choices":[{"delta":{"reasoning_content":"think"}}]}`,
		"",
		`data: {"choices":[{"delta":{"content":"second"}}]}`,
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
		`"delta":"first"`,
		`"text":"first"`,
		`"delta":"think"`,
		`"text":"think"`,
		`"delta":"second"`,
		`"text":"second"`,
		"event: response.completed",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in text/reasoning/text stream:\n%s", expected, body)
		}
	}
	if strings.Contains(body, `"text":"firstsecond"`) || strings.Contains(body, `"text":"firstsecond`) {
		t.Fatalf("second message text contaminated by first segment:\n%s", body)
	}
	// 两段 message 各有一次 output_text.done
	if strings.Count(body, "event: response.output_text.done") != 2 {
		t.Fatalf("expected 2 output_text.done events:\n%s", body)
	}
}


func TestWriteStreamFirstEventError(t *testing.T) {
	// Error envelope first event → JSON 502, never empty completed stream.
	upstream := "data: {\"error\":{\"type\":\"server_error\",\"message\":\"boom\"}}\n\n"
	recorder := httptest.NewRecorder()
	WriteStream(recorder, strings.NewReader(upstream), "demo")
	if recorder.Code != 502 {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, "response.created") || strings.Contains(body, "response.completed") {
		t.Fatalf("must not start success SSE on error first event:\n%s", body)
	}
	if !strings.Contains(body, "boom") {
		t.Fatalf("body = %s", body)
	}
}

func TestWriteStreamLengthFinishIncomplete(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"cut"}}]}`,
		"",
		`data: {"choices":[{"delta":{},"finish_reason":"length"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	recorder := httptest.NewRecorder()
	WriteStream(recorder, strings.NewReader(upstream), "demo")
	body := recorder.Body.String()
	if strings.Contains(body, "event: response.completed") {
		t.Fatalf("length finish must not emit completed:\n%s", body)
	}
	if !strings.Contains(body, "event: response.incomplete") {
		t.Fatalf("expected response.incomplete:\n%s", body)
	}
	if !strings.Contains(body, `"status":"incomplete"`) {
		t.Fatalf("expected incomplete status:\n%s", body)
	}
	if !strings.Contains(body, "max_output_tokens") {
		t.Fatalf("expected incomplete_details reason:\n%s", body)
	}
}

func TestWriteStreamLateToolName(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_x","function":{"arguments":"{\"a\""}}]}}]}`,
		"",
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"lookup","arguments":":1}"}}]}}]}`,
		"",
		`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	recorder := httptest.NewRecorder()
	WriteStream(recorder, strings.NewReader(upstream), "demo")
	body := recorder.Body.String()
	if !strings.Contains(body, `"name":"lookup"`) {
		t.Fatalf("late name should appear in done item:\n%s", body)
	}
	if !strings.Contains(body, `"arguments":"{\"a\":1}"`) {
		t.Fatalf("arguments should concatenate:\n%s", body)
	}
}

func TestWriteStreamTrailingUsageWithCache(t *testing.T) {
	// finish_reason first, usage frame later — terminal must wait for usage.
	upstream := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"ok"}}]}`,
		"",
		`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
		"",
		`data: {"choices":[],"usage":{"prompt_tokens":100,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":40}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	recorder := httptest.NewRecorder()
	snap := WriteStream(recorder, strings.NewReader(upstream), "demo")
	body := recorder.Body.String()
	if !strings.Contains(body, "event: response.completed") {
		t.Fatalf("expected completed:\n%s", body)
	}
	if !strings.Contains(body, `"cached_tokens":40`) {
		t.Fatalf("expected cached_tokens in completed usage:\n%s", body)
	}
	if !strings.Contains(body, `"input_tokens":100`) || !strings.Contains(body, `"output_tokens":5`) {
		t.Fatalf("expected input/output in completed usage:\n%s", body)
	}
	if snap.PromptTokens != 100 || snap.CompletionTokens != 5 || snap.CacheReadTokens != 40 {
		t.Fatalf("snap = %+v", snap)
	}
}
