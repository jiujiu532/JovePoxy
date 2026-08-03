package anthropic_test

import (
	"encoding/json"
	"strings"
	"testing"

	"jovepoxy/internal/anthropic"
)

func TestToOpenAIChat_converts_system_tools_and_tool_use(t *testing.T) {
	// Given
	body := []byte(`{
		"model":"demo-free",
		"max_tokens":128,
		"system":[{"type":"text","text":"be brief"}],
		"messages":[
			{"role":"user","content":"hi"},
			{"role":"assistant","content":[
				{"type":"text","text":"calling"},
				{"type":"tool_use","id":"toolu_1","name":"lookup","input":{"q":"x"}}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"toolu_1","content":"result text"}
			]}
		],
		"tools":[{"name":"lookup","description":"find things","input_schema":{"type":"object"}}]
	}`)
	request, err := anthropic.ParseRequest(body)
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}

	// When
	openAIBody, inputTokens, err := anthropic.ToOpenAIChat(request)

	// Then
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	if inputTokens <= 0 {
		t.Fatalf("inputTokens = %d, want > 0", inputTokens)
	}
	var payload struct {
		Model    string `json:"model"`
		Messages []struct {
			Role       string          `json:"role"`
			Content    json.RawMessage `json:"content"`
			ToolCalls  json.RawMessage `json:"tool_calls"`
			ToolCallID string          `json:"tool_call_id"`
		} `json:"messages"`
		Tools []struct {
			Type     string `json:"type"`
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Model != "demo-free" || len(payload.Tools) != 1 || payload.Tools[0].Function.Name != "lookup" {
		t.Fatalf("payload tools/model = %+v", payload)
	}
	if len(payload.Messages) != 4 {
		t.Fatalf("messages = %d, want 4 (system+user+assistant+tool)", len(payload.Messages))
	}
	if payload.Messages[0].Role != "system" || payload.Messages[2].Role != "assistant" || payload.Messages[3].Role != "tool" {
		t.Fatalf("roles = %+v", payload.Messages)
	}
	if len(payload.Messages[2].ToolCalls) == 0 || payload.Messages[3].ToolCallID != "toolu_1" {
		t.Fatalf("tool mapping failed: %+v", payload.Messages[2:])
	}
}

func TestToOpenAIChat_converts_string_system_and_user(t *testing.T) {
	// Given
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":16,
		"system":"sys","messages":[{"role":"user","content":"hello"}]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}

	// When
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}

	// Then
	var payload struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Messages) != 2 || payload.Messages[0].Content != "sys" || payload.Messages[1].Content != "hello" {
		t.Fatalf("messages = %+v", payload.Messages)
	}
}

func TestParseRequest_rejects_missing_model(t *testing.T) {
	// When
	_, err := anthropic.ParseRequest([]byte(`{"messages":[{"role":"user","content":"x"}]}`))

	// Then
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToOpenAIChat_thinking_enabled_with_budget(t *testing.T) {
	// budget 2000 → medium (1025–8192)
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":64,
		"thinking":{"type":"enabled","budget_tokens":2000},
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, _ := payload["reasoning_effort"].(string); got != "medium" {
		t.Fatalf("reasoning_effort = %v, want medium; body=%s", payload["reasoning_effort"], openAIBody)
	}
}

func TestToOpenAIChat_thinking_adaptive_with_effort(t *testing.T) {
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":64,
		"thinking":{"type":"adaptive"},
		"output_config":{"effort":"HIGH"},
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, _ := payload["reasoning_effort"].(string); got != "high" {
		t.Fatalf("reasoning_effort = %v, want high; body=%s", payload["reasoning_effort"], openAIBody)
	}
}

func TestToOpenAIChat_thinking_disabled(t *testing.T) {
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":64,
		"thinking":{"type":"disabled"},
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, _ := payload["reasoning_effort"].(string); got != "none" {
		t.Fatalf("reasoning_effort = %v, want none; body=%s", payload["reasoning_effort"], openAIBody)
	}
}

func TestToOpenAIChat_no_thinking_omits_reasoning_effort(t *testing.T) {
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":64,
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	if strings.Contains(string(openAIBody), "reasoning_effort") {
		t.Fatalf("body must not contain reasoning_effort: %s", openAIBody)
	}
	var payload map[string]any
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := payload["reasoning_effort"]; ok {
		t.Fatalf("reasoning_effort key present: %v", payload["reasoning_effort"])
	}
}

func TestToOpenAIChat_temperature_top_p_stop_sequences(t *testing.T) {
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":64,
		"temperature":0.7,
		"top_p":0.9,
		"stop_sequences":["END","STOP"],
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	var payload struct {
		Temperature *float64 `json:"temperature"`
		TopP        *float64 `json:"top_p"`
		Stop        any      `json:"stop"`
	}
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Temperature == nil || *payload.Temperature != 0.7 {
		t.Fatalf("temperature = %v, want 0.7", payload.Temperature)
	}
	if payload.TopP == nil || *payload.TopP != 0.9 {
		t.Fatalf("top_p = %v, want 0.9", payload.TopP)
	}
	stop, ok := payload.Stop.([]any)
	if !ok || len(stop) != 2 || stop[0] != "END" || stop[1] != "STOP" {
		t.Fatalf("stop = %#v, want [END STOP]", payload.Stop)
	}

	// single stop_sequences → string
	request1, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":64,
		"stop_sequences":["ONLY"],
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest single stop: %v", err)
	}
	body1, _, err := anthropic.ToOpenAIChat(request1)
	if err != nil {
		t.Fatalf("ToOpenAIChat single stop: %v", err)
	}
	var payload1 map[string]any
	if err := json.Unmarshal(body1, &payload1); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, _ := payload1["stop"].(string); got != "ONLY" {
		t.Fatalf("single stop = %#v, want string ONLY", payload1["stop"])
	}
}

func TestToOpenAIChat_tool_choice_any_to_required(t *testing.T) {
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":64,
		"tool_choice":{"type":"any"},
		"messages":[{"role":"user","content":"hi"}],
		"tools":[{"name":"lookup","description":"d","input_schema":{"type":"object"}}]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, _ := payload["tool_choice"].(string); got != "required" {
		t.Fatalf("tool_choice = %#v, want required; body=%s", payload["tool_choice"], openAIBody)
	}
}

func TestToOpenAIChat_tool_choice_tool_name(t *testing.T) {
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":64,
		"tool_choice":{"type":"tool","name":"lookup"},
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	var payload struct {
		ToolChoice struct {
			Type     string `json:"type"`
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		} `json:"tool_choice"`
	}
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.ToolChoice.Type != "function" || payload.ToolChoice.Function.Name != "lookup" {
		t.Fatalf("tool_choice = %+v, body=%s", payload.ToolChoice, openAIBody)
	}
}

func TestToOpenAIChat_assistant_history_thinking_to_reasoning_content(t *testing.T) {
	// Anthropic official thinking blocks use the "thinking" field (not "text").
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":64,
		"messages":[
			{"role":"user","content":"q"},
			{"role":"assistant","content":[
				{"type":"thinking","thinking":"step one"},
				{"type":"thinking","thinking":"step two"},
				{"type":"redacted_thinking","data":"secret"},
				{"type":"text","text":"answer"}
			]}
		]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	var payload struct {
		Messages []struct {
			Role             string `json:"role"`
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(payload.Messages))
	}
	asst := payload.Messages[1]
	if asst.Role != "assistant" {
		t.Fatalf("role = %s", asst.Role)
	}
	if asst.Content != "answer" {
		t.Fatalf("content = %q, want answer", asst.Content)
	}
	if asst.ReasoningContent != "step one\nstep two" {
		t.Fatalf("reasoning_content = %q, want step one\\nstep two", asst.ReasoningContent)
	}
	// redacted_thinking must never appear
	if strings.Contains(string(openAIBody), "secret") {
		t.Fatalf("redacted data leaked into body: %s", openAIBody)
	}
}

func TestToOpenAIChat_assistant_thinking_text_field_fallback(t *testing.T) {
	// Some clients / Gemini-style parts use "text" on thinking blocks.
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":64,
		"messages":[
			{"role":"assistant","content":[
				{"type":"thinking","text":"via text field"},
				{"type":"text","text":"ok"}
			]}
		]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	var payload struct {
		Messages []struct {
			ReasoningContent string `json:"reasoning_content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Messages) != 1 || payload.Messages[0].ReasoningContent != "via text field" {
		t.Fatalf("reasoning_content fallback failed: %+v body=%s", payload.Messages, openAIBody)
	}
}

func TestToOpenAIChat_user_thinking_not_injected(t *testing.T) {
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":64,
		"messages":[
			{"role":"user","content":[
				{"type":"thinking","thinking":"inject me"},
				{"type":"text","text":"real question"}
			]}
		]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	if strings.Contains(string(openAIBody), "reasoning_content") {
		t.Fatalf("user thinking must not produce reasoning_content: %s", openAIBody)
	}
	if strings.Contains(string(openAIBody), "inject me") {
		t.Fatalf("user thinking text leaked into body: %s", openAIBody)
	}
	var payload struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Messages) != 1 || payload.Messages[0].Content != "real question" {
		t.Fatalf("messages = %+v", payload.Messages)
	}
}

func TestToOpenAIChat_thinking_adaptive_without_effort_is_high(t *testing.T) {
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":64,
		"thinking":{"type":"adaptive"},
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, _ := payload["reasoning_effort"].(string); got != "high" {
		t.Fatalf("reasoning_effort = %v, want high", payload["reasoning_effort"])
	}
}

func TestParseRequest_invalid_stop_sequences_ignored(t *testing.T) {
	// Non-array stop_sequences must not fail ParseRequest (design: ignore stop).
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":64,
		"stop_sequences":"not-an-array",
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest should ignore bad stop_sequences: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	if strings.Contains(string(openAIBody), `"stop"`) {
		t.Fatalf("invalid stop_sequences must omit stop: %s", openAIBody)
	}
}

func TestToOpenAIChat_thinking_enabled_without_budget_is_high(t *testing.T) {
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":64,
		"thinking":{"type":"enabled"},
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, _ := payload["reasoning_effort"].(string); got != "high" {
		t.Fatalf("reasoning_effort = %v, want high", payload["reasoning_effort"])
	}
	meta := request.Observability()
	if meta.ReasoningEffort != "high" || meta.ThinkingType != "enabled" || meta.BudgetTokens != 0 {
		t.Fatalf("Observability = %+v, want high/enabled/0", meta)
	}
}

func TestToOpenAIChat_thinking_enabled_budget_zero_is_high(t *testing.T) {
	// budget_tokens=0 与缺省同等：不发 auto
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":64,
		"thinking":{"type":"enabled","budget_tokens":0},
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, _ := payload["reasoning_effort"].(string); got != "high" {
		t.Fatalf("reasoning_effort = %v, want high", payload["reasoning_effort"])
	}
}


func TestToOpenAIChat_enabled_with_output_config_effort_high(t *testing.T) {
	// Kelivo DeepSeek: enabled 无 budget，档位在 output_config.effort
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"deepseek-v4-flash-free","max_tokens":64000,
		"thinking":{"type":"enabled"},
		"output_config":{"effort":"high"},
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, _ := payload["reasoning_effort"].(string); got != "high" {
		t.Fatalf("reasoning_effort = %v, want high; body=%s", payload["reasoning_effort"], openAIBody)
	}
	meta := request.Observability()
	if meta.ReasoningEffort != "high" || meta.ThinkingType != "enabled" || meta.BudgetTokens != 0 {
		t.Fatalf("Observability = %+v", meta)
	}
}

func TestToOpenAIChat_enabled_with_output_config_effort_max_is_xhigh(t *testing.T) {
	// Kelivo 极限/全力 → output_config.effort=max；Zen 无 max，落成 xhigh
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"deepseek-v4-flash-free","max_tokens":64000,
		"thinking":{"type":"enabled"},
		"output_config":{"effort":"max"},
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, _ := payload["reasoning_effort"].(string); got != "xhigh" {
		t.Fatalf("reasoning_effort = %v, want xhigh; body=%s", payload["reasoning_effort"], openAIBody)
	}
	if request.Observability().ReasoningEffort != "xhigh" {
		t.Fatalf("Observability.ReasoningEffort = %q, want xhigh", request.Observability().ReasoningEffort)
	}
}

func TestToOpenAIChat_budget_beats_output_config_effort(t *testing.T) {
	// 有正 budget 时仍按 budget 映射，不被 output_config 覆盖
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":64,
		"thinking":{"type":"enabled","budget_tokens":2000},
		"output_config":{"effort":"max"},
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, _ := payload["reasoning_effort"].(string); got != "medium" {
		t.Fatalf("reasoning_effort = %v, want medium (budget wins); body=%s", payload["reasoning_effort"], openAIBody)
	}
}

func TestToOpenAIChat_unknown_thinking_type_omits_effort(t *testing.T) {
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":64,
		"thinking":{"type":"mystery","budget_tokens":999},
		"messages":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	if strings.Contains(string(openAIBody), "reasoning_effort") {
		t.Fatalf("unknown thinking type must omit reasoning_effort: %s", openAIBody)
	}
}

func TestToOpenAIChat_tool_result_with_text(t *testing.T) {
	// 同条 user message 含 tool_result + text → tool 消息后追加 user 文本
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":64,
		"messages":[
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"toolu_1","content":"result text"},
				{"type":"text","text":"please continue"}
			]}
		]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	var payload struct {
		Messages []struct {
			Role       string `json:"role"`
			Content    string `json:"content"`
			ToolCallID string `json:"tool_call_id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Messages) != 2 {
		t.Fatalf("messages = %d, want 2 (tool + user); body=%s", len(payload.Messages), openAIBody)
	}
	if payload.Messages[0].Role != "tool" || payload.Messages[0].ToolCallID != "toolu_1" || payload.Messages[0].Content != "result text" {
		t.Fatalf("tool msg = %+v", payload.Messages[0])
	}
	if payload.Messages[1].Role != "user" || payload.Messages[1].Content != "please continue" {
		t.Fatalf("user text msg = %+v", payload.Messages[1])
	}
}


func TestToOpenAIChat_empty_tool_use_id_generated(t *testing.T) {
	request, err := anthropic.ParseRequest([]byte(`{
		"model":"demo-free","max_tokens":16,
		"messages":[{"role":"assistant","content":[
			{"type":"tool_use","id":"","name":"lookup","input":{"q":1}}
		]}]
	}`))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	openAIBody, _, err := anthropic.ToOpenAIChat(request)
	if err != nil {
		t.Fatalf("ToOpenAIChat: %v", err)
	}
	var payload struct {
		Messages []struct {
			ToolCalls []struct {
				ID string `json:"id"`
			} `json:"tool_calls"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(openAIBody, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Messages) == 0 || len(payload.Messages[0].ToolCalls) == 0 {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.Messages[0].ToolCalls[0].ID == "" {
		t.Fatalf("expected generated tool call id")
	}
	if !strings.HasPrefix(payload.Messages[0].ToolCalls[0].ID, "toolu_") {
		t.Fatalf("id = %q", payload.Messages[0].ToolCalls[0].ID)
	}
}
