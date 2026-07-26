package anthropic_test

import (
	"encoding/json"
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
