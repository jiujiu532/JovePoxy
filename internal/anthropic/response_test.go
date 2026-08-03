package anthropic_test

import (
	"testing"

	"jovepoxy/internal/anthropic"
)

func TestFromOpenAI_maps_tool_calls_to_tool_use(t *testing.T) {
	// Given
	body := []byte(`{
		"choices":[{
			"finish_reason":"tool_calls",
			"message":{
				"content":"",
				"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"x\"}"}}]
			}
		}],
		"usage":{"prompt_tokens":11,"completion_tokens":7}
	}`)

	// When
	message, err := anthropic.FromOpenAI(body, "demo-free", 3)

	// Then
	if err != nil {
		t.Fatalf("FromOpenAI: %v", err)
	}
	if message.StopReason != "tool_use" || message.Model != "demo-free" {
		t.Fatalf("message = %+v", message)
	}
	if len(message.Content) != 1 || message.Content[0]["type"] != "tool_use" {
		t.Fatalf("content = %+v", message.Content)
	}
	if message.Content[0]["name"] != "lookup" {
		t.Fatalf("tool name = %v", message.Content[0]["name"])
	}
	if message.Usage["input_tokens"] != 11 || message.Usage["output_tokens"] != 7 {
		t.Fatalf("usage = %+v", message.Usage)
	}
}

func TestFromOpenAI_maps_text_stop_reason(t *testing.T) {
	// Given
	body := []byte(`{"choices":[{"finish_reason":"stop","message":{"content":"hello"}}]}`)

	// When
	message, err := anthropic.FromOpenAI(body, "demo-free", 9)

	// Then
	if err != nil {
		t.Fatalf("FromOpenAI: %v", err)
	}
	if message.StopReason != "end_turn" || message.Content[0]["text"] != "hello" {
		t.Fatalf("message = %+v", message)
	}
	if message.Usage["input_tokens"] != 9 {
		t.Fatalf("usage = %+v", message.Usage)
	}
}

func TestFromOpenAI_prepends_thinking_before_text(t *testing.T) {
	// Given
	body := []byte(`{
		"choices":[{
			"finish_reason":"stop",
			"message":{
				"content":"answer",
				"reasoning_content":"step by step"
			}
		}]
	}`)

	// When
	message, err := anthropic.FromOpenAI(body, "demo-free", 1)

	// Then
	if err != nil {
		t.Fatalf("FromOpenAI: %v", err)
	}
	if len(message.Content) != 2 {
		t.Fatalf("content len = %d, content=%+v", len(message.Content), message.Content)
	}
	if message.Content[0]["type"] != "thinking" || message.Content[0]["thinking"] != "step by step" {
		t.Fatalf("thinking block = %+v", message.Content[0])
	}
	if message.Content[1]["type"] != "text" || message.Content[1]["text"] != "answer" {
		t.Fatalf("text block = %+v", message.Content[1])
	}
}

func TestFromOpenAI_prepends_thinking_before_tool_use(t *testing.T) {
	// Given
	body := []byte(`{
		"choices":[{
			"finish_reason":"tool_calls",
			"message":{
				"content":"",
				"reasoning_content":"need tool",
				"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]
			}
		}]
	}`)

	// When
	message, err := anthropic.FromOpenAI(body, "demo-free", 1)

	// Then
	if err != nil {
		t.Fatalf("FromOpenAI: %v", err)
	}
	if len(message.Content) != 2 {
		t.Fatalf("content = %+v", message.Content)
	}
	if message.Content[0]["type"] != "thinking" || message.Content[0]["thinking"] != "need tool" {
		t.Fatalf("thinking = %+v", message.Content[0])
	}
	if message.Content[1]["type"] != "tool_use" || message.Content[1]["name"] != "lookup" {
		t.Fatalf("tool = %+v", message.Content[1])
	}
}

func TestFromOpenAI_without_reasoning_has_no_thinking_block(t *testing.T) {
	// Given
	body := []byte(`{"choices":[{"finish_reason":"stop","message":{"content":"hello"}}]}`)

	// When
	message, err := anthropic.FromOpenAI(body, "demo-free", 1)

	// Then
	if err != nil {
		t.Fatalf("FromOpenAI: %v", err)
	}
	for _, block := range message.Content {
		if block["type"] == "thinking" {
			t.Fatalf("unexpected thinking block: %+v", message.Content)
		}
	}
	if len(message.Content) != 1 || message.Content[0]["type"] != "text" {
		t.Fatalf("content = %+v", message.Content)
	}
}

func TestFromOpenAI_empty_reasoning_content_skipped(t *testing.T) {
	// Given
	body := []byte(`{
		"choices":[{
			"finish_reason":"stop",
			"message":{"content":"only text","reasoning_content":""}
		}]
	}`)

	// When
	message, err := anthropic.FromOpenAI(body, "demo-free", 1)

	// Then
	if err != nil {
		t.Fatalf("FromOpenAI: %v", err)
	}
	if len(message.Content) != 1 || message.Content[0]["type"] != "text" {
		t.Fatalf("content = %+v", message.Content)
	}
}


func TestFromOpenAI_reasoning_field_alias(t *testing.T) {
	// Non-stream should accept message.reasoning like stream does.
	body := []byte(`{
		"choices":[{
			"finish_reason":"stop",
			"message":{"content":"answer","reasoning":"alias plan"}
		}]
	}`)
	message, err := anthropic.FromOpenAI(body, "demo-free", 1)
	if err != nil {
		t.Fatalf("FromOpenAI: %v", err)
	}
	if len(message.Content) < 2 {
		t.Fatalf("content = %+v", message.Content)
	}
	if message.Content[0]["type"] != "thinking" || message.Content[0]["thinking"] != "alias plan" {
		t.Fatalf("thinking = %+v", message.Content[0])
	}
}

func TestFromOpenAI_invalid_tool_arguments_not_empty_object(t *testing.T) {
	body := []byte(`{
		"choices":[{
			"finish_reason":"tool_calls",
			"message":{
				"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"not-json{"}}]
			}
		}]
	}`)
	message, err := anthropic.FromOpenAI(body, "demo-free", 1)
	if err != nil {
		t.Fatalf("FromOpenAI: %v", err)
	}
	if len(message.Content) != 1 || message.Content[0]["type"] != "tool_use" {
		t.Fatalf("content = %+v", message.Content)
	}
	input := message.Content[0]["input"]
	// Must not silently become empty object.
	if m, ok := input.(map[string]any); ok && len(m) == 0 {
		t.Fatalf("illegal arguments must not become empty map, got %#v", input)
	}
	if s, ok := input.(string); !ok || s != "not-json{" {
		t.Fatalf("expected raw string input, got %#v", input)
	}
}

func TestFromOpenAI_empty_tool_id_generated(t *testing.T) {
	body := []byte(`{
		"choices":[{
			"finish_reason":"tool_calls",
			"message":{
				"tool_calls":[{"id":"","type":"function","function":{"name":"lookup","arguments":"{}"}}]
			}
		}]
	}`)
	message, err := anthropic.FromOpenAI(body, "demo-free", 1)
	if err != nil {
		t.Fatalf("FromOpenAI: %v", err)
	}
	id, _ := message.Content[0]["id"].(string)
	if id == "" {
		t.Fatalf("expected generated tool id, content=%+v", message.Content)
	}
}
