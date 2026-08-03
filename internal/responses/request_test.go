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

func TestToOpenAIChatReasoningEffortNested(t *testing.T) {
	parsed, err := ParseRequest([]byte(`{
		"model":"demo","input":"hi",
		"reasoning":{"effort":"HIGH"}
	}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	body, err := ToOpenAIChat(parsed)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	var chat map[string]any
	if err := json.Unmarshal(body, &chat); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if chat["reasoning_effort"] != "high" {
		t.Fatalf("expected reasoning_effort=high, got %v; body=%s", chat["reasoning_effort"], body)
	}
}

func TestToOpenAIChatReasoningEffortTopLevelFallback(t *testing.T) {
	parsed, err := ParseRequest([]byte(`{
		"model":"demo","input":"hi",
		"reasoning_effort":" medium "
	}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	body, err := ToOpenAIChat(parsed)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	var chat map[string]any
	if err := json.Unmarshal(body, &chat); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if chat["reasoning_effort"] != "medium" {
		t.Fatalf("expected reasoning_effort=medium, got %v", chat["reasoning_effort"])
	}
}

func TestToOpenAIChatNestedEffortPrefersOverTopLevel(t *testing.T) {
	parsed, err := ParseRequest([]byte(`{
		"model":"demo","input":"hi",
		"reasoning":{"effort":"low"},
		"reasoning_effort":"high"
	}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	body, err := ToOpenAIChat(parsed)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	var chat map[string]any
	if err := json.Unmarshal(body, &chat); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if chat["reasoning_effort"] != "low" {
		t.Fatalf("expected nested effort to win, got %v", chat["reasoning_effort"])
	}
}

func TestToOpenAIChatOmitsReasoningEffortWhenAbsent(t *testing.T) {
	parsed, err := ParseRequest([]byte(`{"model":"demo","input":"hi"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	body, err := ToOpenAIChat(parsed)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if strings.Contains(string(body), "reasoning_effort") {
		t.Fatalf("reasoning_effort should be omitted: %s", body)
	}
	if strings.Contains(string(body), "tool_choice") {
		t.Fatalf("tool_choice should be omitted: %s", body)
	}
	if strings.Contains(string(body), "parallel_tool_calls") {
		t.Fatalf("parallel_tool_calls should be omitted: %s", body)
	}
}

func TestToOpenAIChatToolChoiceAndParallel(t *testing.T) {
	parsed, err := ParseRequest([]byte(`{
		"model":"demo","input":"hi",
		"tool_choice":"required",
		"parallel_tool_calls":false
	}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	body, err := ToOpenAIChat(parsed)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	var chat struct {
		ToolChoice        json.RawMessage `json:"tool_choice"`
		ParallelToolCalls *bool           `json:"parallel_tool_calls"`
	}
	if err := json.Unmarshal(body, &chat); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(chat.ToolChoice) != `"required"` {
		t.Fatalf("tool_choice wrong: %s", chat.ToolChoice)
	}
	if chat.ParallelToolCalls == nil || *chat.ParallelToolCalls {
		t.Fatalf("parallel_tool_calls wrong: %v", chat.ParallelToolCalls)
	}
}

func TestToOpenAIChatToolChoiceObject(t *testing.T) {
	parsed, err := ParseRequest([]byte(`{
		"model":"demo","input":"hi",
		"tool_choice":{"type":"function","function":{"name":"lookup"}}
	}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	body, err := ToOpenAIChat(parsed)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	var chat map[string]any
	if err := json.Unmarshal(body, &chat); err != nil {
		t.Fatalf("decode: %v", err)
	}
	tc, ok := chat["tool_choice"].(map[string]any)
	if !ok {
		t.Fatalf("tool_choice not object: %v", chat["tool_choice"])
	}
	fn, _ := tc["function"].(map[string]any)
	if fn["name"] != "lookup" {
		t.Fatalf("tool_choice function name wrong: %+v", tc)
	}
}

func TestToOpenAIChatReasoningItemAttachesToAssistant(t *testing.T) {
	parsed, err := ParseRequest([]byte(`{
		"model":"demo",
		"input":[
			{"type":"reasoning","summary":[{"type":"summary_text","text":"think hard"}]},
			{"role":"user","content":"question"},
			{"role":"assistant","content":[{"type":"output_text","text":"answer"}]}
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
		Messages []struct {
			Role             string `json:"role"`
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &chat); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(chat.Messages) != 2 {
		t.Fatalf("expected 2 messages (user+assistant), got %d: %+v", len(chat.Messages), chat.Messages)
	}
	if chat.Messages[0].Role != "user" || chat.Messages[0].ReasoningContent != "" {
		t.Fatalf("user should not carry reasoning: %+v", chat.Messages[0])
	}
	if chat.Messages[1].Role != "assistant" || chat.Messages[1].Content != "answer" {
		t.Fatalf("assistant message wrong: %+v", chat.Messages[1])
	}
	if chat.Messages[1].ReasoningContent != "think hard" {
		t.Fatalf("expected reasoning_content on assistant, got %q", chat.Messages[1].ReasoningContent)
	}
	// Ensure key is present only when non-empty (user message must not include it).
	rawMsgs := struct {
		Messages []map[string]any `json:"messages"`
	}{}
	if err := json.Unmarshal(body, &rawMsgs); err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	if _, ok := rawMsgs.Messages[0]["reasoning_content"]; ok {
		t.Fatalf("user message should omit reasoning_content key: %+v", rawMsgs.Messages[0])
	}
}

func TestToOpenAIChatReasoningItemAttachesToFunctionCallBatch(t *testing.T) {
	parsed, err := ParseRequest([]byte(`{
		"model":"demo",
		"input":[
			{"type":"reasoning","content":[{"type":"text","text":"need tool"}]},
			{"type":"function_call","call_id":"c1","name":"lookup","arguments":"{}"},
			{"type":"function_call_output","call_id":"c1","output":"ok"}
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
		Messages []struct {
			Role             string `json:"role"`
			ReasoningContent string `json:"reasoning_content"`
			ToolCallID       string `json:"tool_call_id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &chat); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(chat.Messages) != 2 {
		t.Fatalf("expected assistant+tool, got %d: %+v", len(chat.Messages), chat.Messages)
	}
	if chat.Messages[0].Role != "assistant" || chat.Messages[0].ReasoningContent != "need tool" {
		t.Fatalf("assistant reasoning wrong: %+v", chat.Messages[0])
	}
	if chat.Messages[1].Role != "tool" || chat.Messages[1].ToolCallID != "c1" {
		t.Fatalf("tool message wrong: %+v", chat.Messages[1])
	}
}

func TestToOpenAIChatEmptyReasoningItemOmitsKey(t *testing.T) {
	parsed, err := ParseRequest([]byte(`{
		"model":"demo",
		"input":[
			{"type":"reasoning","summary":[]},
			{"role":"assistant","content":"hi"}
		]
	}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	body, err := ToOpenAIChat(parsed)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if strings.Contains(string(body), "reasoning_content") {
		t.Fatalf("empty reasoning should omit reasoning_content: %s", body)
	}
}

func TestToOpenAIChatMergesConsecutiveFunctionCallsAndDefers(t *testing.T) {
	// Two function_calls then outputs; a user message interleaved after first
	// output must be deferred until all tool outputs arrive so adjacency holds.
	parsed, err := ParseRequest([]byte(`{
		"model":"demo",
		"input":[
			{"role":"user","content":"use tools"},
			{"type":"function_call","call_id":"call_a","name":"alpha","arguments":"{\"x\":1}"},
			{"type":"function_call","call_id":"call_b","name":"beta","arguments":"{\"y\":2}"},
			{"type":"function_call_output","call_id":"call_a","output":"ra"},
			{"role":"user","content":"should defer"},
			{"type":"function_call_output","call_id":"call_b","output":"rb"}
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
	// Expected order:
	// 0 user "use tools"
	// 1 assistant with 2 tool_calls
	// 2 tool call_a
	// 3 tool call_b
	// 4 user "should defer"
	if len(chat.Messages) != 5 {
		t.Fatalf("expected 5 messages, got %d: %+v", len(chat.Messages), chat.Messages)
	}
	if chat.Messages[0]["role"] != "user" || chat.Messages[0]["content"] != "use tools" {
		t.Fatalf("msg0 wrong: %+v", chat.Messages[0])
	}
	if chat.Messages[1]["role"] != "assistant" {
		t.Fatalf("msg1 should be merged assistant: %+v", chat.Messages[1])
	}
	toolCalls, ok := chat.Messages[1]["tool_calls"].([]any)
	if !ok || len(toolCalls) != 2 {
		t.Fatalf("expected 2 tool_calls on single assistant, got: %+v", chat.Messages[1]["tool_calls"])
	}
	// Verify both call ids present in order.
	call0, _ := toolCalls[0].(map[string]any)
	call1, _ := toolCalls[1].(map[string]any)
	if call0["id"] != "call_a" || call1["id"] != "call_b" {
		t.Fatalf("tool_calls order/ids wrong: %+v", toolCalls)
	}
	if chat.Messages[2]["role"] != "tool" || chat.Messages[2]["tool_call_id"] != "call_a" {
		t.Fatalf("msg2 tool a wrong: %+v", chat.Messages[2])
	}
	if chat.Messages[3]["role"] != "tool" || chat.Messages[3]["tool_call_id"] != "call_b" {
		t.Fatalf("msg3 tool b wrong: %+v", chat.Messages[3])
	}
	if chat.Messages[4]["role"] != "user" || chat.Messages[4]["content"] != "should defer" {
		t.Fatalf("deferred user should be after tools: %+v", chat.Messages[4])
	}
	// No interleaved user between assistant tool_calls and tool messages.
	for i := 1; i <= 3; i++ {
		role, _ := chat.Messages[i]["role"].(string)
		if role == "user" {
			t.Fatalf("user interleaved at index %d: %+v", i, chat.Messages)
		}
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


func TestToOpenAIChatEmptyCallID(t *testing.T) {
	parsed, err := ParseRequest([]byte(`{
		"model":"demo",
		"input":[{"type":"function_call","call_id":"","name":"lookup","arguments":"{\"q\":1}"}]
	}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	body, err := ToOpenAIChat(parsed)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	var chat struct {
		Messages []struct {
			ToolCalls []struct {
				ID string `json:"id"`
			} `json:"tool_calls"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &chat); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(chat.Messages) == 0 || len(chat.Messages[0].ToolCalls) == 0 {
		t.Fatalf("messages = %+v", chat.Messages)
	}
	id := chat.Messages[0].ToolCalls[0].ID
	if id == "" || !strings.HasPrefix(id, "call_") {
		t.Fatalf("generated id = %q", id)
	}
}
