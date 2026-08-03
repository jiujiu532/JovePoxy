package responses

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFromOpenAIWithReasoning(t *testing.T) {
	body := []byte(`{
		"choices":[{
			"finish_reason":"stop",
			"message":{
				"content":"answer",
				"reasoning_content":"think first"
			}
		}],
		"usage":{"prompt_tokens":3,"completion_tokens":5,"total_tokens":8}
	}`)
	resp, err := FromOpenAI(body, "demo")
	if err != nil {
		t.Fatalf("FromOpenAI: %v", err)
	}
	if resp.Status != "completed" || resp.Model != "demo" {
		t.Fatalf("unexpected response meta: %+v", resp)
	}
	if resp.OutputText != "answer" {
		t.Fatalf("output_text: %q", resp.OutputText)
	}
	if len(resp.Output) < 2 {
		t.Fatalf("expected reasoning + message, got %d items: %+v", len(resp.Output), resp.Output)
	}
	if resp.Output[0]["type"] != "reasoning" {
		t.Fatalf("first item should be reasoning, got %v", resp.Output[0]["type"])
	}
	id, _ := resp.Output[0]["id"].(string)
	if !strings.HasPrefix(id, "rs_") {
		t.Fatalf("reasoning id prefix: %q", id)
	}
	summary, ok := resp.Output[0]["summary"].([]map[string]any)
	if !ok || len(summary) != 1 || summary[0]["type"] != "summary_text" || summary[0]["text"] != "think first" {
		t.Fatalf("unexpected reasoning summary: %#v", resp.Output[0]["summary"])
	}
	if resp.Output[1]["type"] != "message" {
		t.Fatalf("second item should be message, got %v", resp.Output[1]["type"])
	}
}

func TestFromOpenAIWithoutReasoning(t *testing.T) {
	body := []byte(`{
		"choices":[{"finish_reason":"stop","message":{"content":"only text"}}]
	}`)
	resp, err := FromOpenAI(body, "demo")
	if err != nil {
		t.Fatalf("FromOpenAI: %v", err)
	}
	if len(resp.Output) != 1 || resp.Output[0]["type"] != "message" {
		t.Fatalf("expected single message item, got %+v", resp.Output)
	}
	for _, item := range resp.Output {
		if item["type"] == "reasoning" {
			t.Fatalf("unexpected reasoning item: %+v", item)
		}
	}
}

func TestFromOpenAIReasoningOnlyAndWithTool(t *testing.T) {
	body := []byte(`{
		"choices":[{
			"finish_reason":"tool_calls",
			"message":{
				"reasoning_content":"need tool",
				"tool_calls":[{
					"id":"call_1",
					"type":"function",
					"function":{"name":"lookup","arguments":"{}"}
				}]
			}
		}]
	}`)
	resp, err := FromOpenAI(body, "demo")
	if err != nil {
		t.Fatalf("FromOpenAI: %v", err)
	}
	if len(resp.Output) != 2 {
		t.Fatalf("expected reasoning + function_call, got %d: %+v", len(resp.Output), resp.Output)
	}
	if resp.Output[0]["type"] != "reasoning" || resp.Output[1]["type"] != "function_call" {
		t.Fatalf("order: %+v, %+v", resp.Output[0]["type"], resp.Output[1]["type"])
	}
	// 确保可 JSON 序列化（summary 切片类型）
	if _, err := json.Marshal(resp); err != nil {
		t.Fatalf("marshal response: %v", err)
	}
}

func TestFromOpenAIEmptyReasoningIgnored(t *testing.T) {
	body := []byte(`{
		"choices":[{"finish_reason":"stop","message":{"content":"hi","reasoning_content":""}}]
	}`)
	resp, err := FromOpenAI(body, "demo")
	if err != nil {
		t.Fatalf("FromOpenAI: %v", err)
	}
	if len(resp.Output) != 1 || resp.Output[0]["type"] != "message" {
		t.Fatalf("empty reasoning should not inject item: %+v", resp.Output)
	}
}


func TestFromOpenAIReasoningAlias(t *testing.T) {
	body := []byte(`{
		"choices":[{"finish_reason":"stop","message":{"content":"ok","reasoning":"alias"}}]
	}`)
	resp, err := FromOpenAI(body, "demo")
	if err != nil {
		t.Fatalf("FromOpenAI: %v", err)
	}
	if len(resp.Output) < 2 || resp.Output[0]["type"] != "reasoning" {
		t.Fatalf("expected reasoning from alias: %+v", resp.Output)
	}
	summary, _ := resp.Output[0]["summary"].([]map[string]any)
	if len(summary) != 1 || summary[0]["text"] != "alias" {
		t.Fatalf("summary = %#v", resp.Output[0]["summary"])
	}
}

func TestFromOpenAILengthIncomplete(t *testing.T) {
	body := []byte(`{
		"choices":[{"finish_reason":"length","message":{"content":"partial"}}]
	}`)
	resp, err := FromOpenAI(body, "demo")
	if err != nil {
		t.Fatalf("FromOpenAI: %v", err)
	}
	if resp.Status != "incomplete" {
		t.Fatalf("status = %q, want incomplete", resp.Status)
	}
	if resp.IncompleteDetails == nil || resp.IncompleteDetails["reason"] != "max_output_tokens" {
		t.Fatalf("incomplete_details = %#v", resp.IncompleteDetails)
	}
}

func TestFromOpenAIEmptyToolCallID(t *testing.T) {
	body := []byte(`{
		"choices":[{
			"finish_reason":"tool_calls",
			"message":{"tool_calls":[{"id":"","type":"function","function":{"name":"f","arguments":"{}"}}]}
		}]
	}`)
	resp, err := FromOpenAI(body, "demo")
	if err != nil {
		t.Fatalf("FromOpenAI: %v", err)
	}
	if len(resp.Output) != 1 {
		t.Fatalf("output = %+v", resp.Output)
	}
	callID, _ := resp.Output[0]["call_id"].(string)
	if callID == "" || !strings.HasPrefix(callID, "call_") {
		t.Fatalf("call_id = %q", callID)
	}
}
