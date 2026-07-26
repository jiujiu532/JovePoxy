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
