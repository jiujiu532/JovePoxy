package responses

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseRequestStringInput(t *testing.T) {
	parsed, err := ParseRequest([]byte(`{"model":"demo","input":"hello"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Model != "demo" || parsed.Stream {
		t.Fatalf("unexpected request: %+v", parsed)
	}
	body, err := ToOpenAIChat(parsed)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	var chat struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &chat); err != nil {
		t.Fatalf("decode chat body: %v", err)
	}
	if len(chat.Messages) != 1 || chat.Messages[0].Role != "user" || chat.Messages[0].Content != "hello" {
		t.Fatalf("unexpected messages: %+v", chat.Messages)
	}
}

func TestParseRequestRejectsMissingModel(t *testing.T) {
	if _, err := ParseRequest([]byte(`{"input":"hi"}`)); err == nil {
		t.Fatal("expected error for missing model")
	}
}

func TestParseRequestRejectsMissingInput(t *testing.T) {
	if _, err := ParseRequest([]byte(`{"model":"demo"}`)); err == nil {
		t.Fatal("expected error for missing input")
	}
}

func TestToOpenAIChatInstructionsAndItems(t *testing.T) {
	parsed, err := ParseRequest([]byte(`{
		"model":"demo","instructions":"be brief",
		"input":[
			{"role":"user","content":[{"type":"input_text","text":"question"}]},
			{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"q\":1}"},
			{"type":"function_call_output","call_id":"call_1","output":"42"}
		]
	}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	body, err := ToOpenAIChat(parsed)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	var chat struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal(body, &chat); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(chat.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %d: %+v", len(chat.Messages), chat.Messages)
	}
	if chat.Messages[0]["role"] != "system" || chat.Messages[0]["content"] != "be brief" {
		t.Fatalf("system message wrong: %+v", chat.Messages[0])
	}
	if chat.Messages[3]["role"] != "tool" || chat.Messages[3]["tool_call_id"] != "call_1" {
		t.Fatalf("tool message wrong: %+v", chat.Messages[3])
	}
}

func TestToOpenAIChatConvertsFlatTools(t *testing.T) {
	parsed, err := ParseRequest([]byte(`{
		"model":"demo","input":"hi",
		"tools":[{"type":"function","name":"get_weather","description":"d","parameters":{"type":"object"}}]
	}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	body, err := ToOpenAIChat(parsed)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if !strings.Contains(string(body), `"function":{"description":"d","name":"get_weather"`) &&
		!strings.Contains(string(body), `"name":"get_weather"`) {
		t.Fatalf("tools not converted: %s", body)
	}
}

func TestFromOpenAITextAndToolCall(t *testing.T) {
	body := []byte(`{
		"choices":[{"finish_reason":"tool_calls","message":{
			"content":"partial answer",
			"tool_calls":[{"id":"call_9","type":"function","function":{"name":"lookup","arguments":"{}"}}]
		}}],
		"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}
	}`)
	converted, err := FromOpenAI(body, "demo")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if converted.Object != "response" || converted.Status != "completed" {
		t.Fatalf("unexpected envelope: %+v", converted)
	}
	if converted.OutputText != "partial answer" {
		t.Fatalf("output_text wrong: %q", converted.OutputText)
	}
	if len(converted.Output) != 2 {
		t.Fatalf("expected 2 output items, got %d", len(converted.Output))
	}
	if converted.Output[0]["type"] != "message" {
		t.Fatalf("first item should be message: %+v", converted.Output[0])
	}
	call := converted.Output[1]
	if call["type"] != "function_call" || call["call_id"] != "call_9" || call["name"] != "lookup" {
		t.Fatalf("function_call item wrong: %+v", call)
	}
	usage, _ := converted.Usage["total_tokens"].(int)
	if usage != 15 {
		t.Fatalf("usage total wrong: %+v", converted.Usage)
	}
}

func TestFromOpenAIEmptyChoices(t *testing.T) {
	converted, err := FromOpenAI([]byte(`{"choices":[]}`), "demo")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if converted.Status != "completed" || len(converted.Output) != 0 {
		t.Fatalf("unexpected: %+v", converted)
	}
}
