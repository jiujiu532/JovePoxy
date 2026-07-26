package responses

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Request is a parsed OpenAI Responses API request.
type Request struct {
	Model           string
	Stream          bool
	MaxOutputTokens int
	Instructions    string
	Input           json.RawMessage
	Tools           []json.RawMessage
}

// ParseRequest decodes and lightly validates a /v1/responses body.
func ParseRequest(body []byte) (Request, error) {
	var raw struct {
		Model           string            `json:"model"`
		Stream          bool              `json:"stream"`
		MaxOutputTokens int               `json:"max_output_tokens"`
		Instructions    string            `json:"instructions"`
		Input           json.RawMessage   `json:"input"`
		Tools           []json.RawMessage `json:"tools"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&raw); err != nil {
		return Request{}, fmt.Errorf("invalid JSON request")
	}
	var extra struct{}
	if err := decoder.Decode(&extra); err == nil {
		return Request{}, fmt.Errorf("invalid JSON request")
	} else if !errors.Is(err, io.EOF) {
		return Request{}, fmt.Errorf("invalid JSON request")
	}
	if strings.TrimSpace(raw.Model) == "" {
		return Request{}, fmt.Errorf("model is required")
	}
	if len(bytes.TrimSpace(raw.Input)) == 0 && strings.TrimSpace(raw.Instructions) == "" {
		return Request{}, fmt.Errorf("input is required")
	}
	return Request{
		Model: raw.Model, Stream: raw.Stream, MaxOutputTokens: raw.MaxOutputTokens,
		Instructions: raw.Instructions, Input: raw.Input, Tools: raw.Tools,
	}, nil
}

type chatMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content"`
	ToolCalls  []map[string]any `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type chatRequest struct {
	Model     string           `json:"model"`
	Messages  []chatMessage    `json:"messages"`
	Stream    bool             `json:"stream"`
	MaxTokens int              `json:"max_tokens,omitempty"`
	Tools     []map[string]any `json:"tools,omitempty"`
}

// ToOpenAIChat converts a Responses request into a chat.completions body.
func ToOpenAIChat(request Request) ([]byte, error) {
	messages, err := convertInput(request)
	if err != nil {
		return nil, err
	}
	tools, err := convertTools(request.Tools)
	if err != nil {
		return nil, err
	}
	payload := chatRequest{
		Model: request.Model, Messages: messages, Stream: request.Stream,
		MaxTokens: request.MaxOutputTokens, Tools: tools,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode openai request: %w", err)
	}
	return encoded, nil
}

func convertInput(request Request) ([]chatMessage, error) {
	messages := make([]chatMessage, 0, 4)
	if system := strings.TrimSpace(request.Instructions); system != "" {
		messages = append(messages, chatMessage{Role: "system", Content: system})
	}
	input := bytes.TrimSpace(request.Input)
	if len(input) == 0 {
		return messages, nil
	}
	// input 可以是纯字符串或 item 数组
	if input[0] == '"' {
		var text string
		if err := json.Unmarshal(input, &text); err != nil {
			return nil, fmt.Errorf("invalid input")
		}
		messages = append(messages, chatMessage{Role: "user", Content: text})
		return messages, nil
	}
	if input[0] != '[' {
		return nil, fmt.Errorf("input must be a string or an array")
	}
	var items []json.RawMessage
	if err := json.Unmarshal(input, &items); err != nil {
		return nil, fmt.Errorf("invalid input")
	}
	for _, item := range items {
		converted, err := convertItem(item)
		if err != nil {
			return nil, err
		}
		if converted != nil {
			messages = append(messages, *converted)
		}
	}
	return messages, nil
}

type inputItem struct {
	Type      string          `json:"type"`
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	CallID    string          `json:"call_id"`
	Name      string          `json:"name"`
	Arguments string          `json:"arguments"`
	Output    json.RawMessage `json:"output"`
}

func convertItem(raw json.RawMessage) (*chatMessage, error) {
	var item inputItem
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, fmt.Errorf("invalid input item")
	}
	// 缺省 type：带 role 视作 message
	kind := item.Type
	if kind == "" && item.Role != "" {
		kind = "message"
	}
	switch kind {
	case "message":
		content, err := flattenContent(item.Content)
		if err != nil {
			return nil, err
		}
		role := item.Role
		if role == "" {
			role = "user"
		}
		if role == "developer" {
			role = "system"
		}
		return &chatMessage{Role: role, Content: content}, nil
	case "function_call":
		call := map[string]any{
			"id":   item.CallID,
			"type": "function",
			"function": map[string]any{
				"name":      item.Name,
				"arguments": item.Arguments,
			},
		}
		return &chatMessage{Role: "assistant", Content: "", ToolCalls: []map[string]any{call}}, nil
	case "function_call_output":
		output := strings.TrimSpace(string(item.Output))
		if len(output) > 1 && output[0] == '"' {
			var text string
			if err := json.Unmarshal(item.Output, &text); err == nil {
				output = text
			}
		}
		return &chatMessage{Role: "tool", Content: output, ToolCallID: item.CallID}, nil
	case "reasoning":
		// 推理 item 上游不需要，静默跳过
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported input item type: %s", kind)
	}
}

// flattenContent 把 Responses 的 content（字符串或 parts 数组）拍平成纯文本。
func flattenContent(raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return "", nil
	}
	if trimmed[0] == '"' {
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return "", fmt.Errorf("invalid message content")
		}
		return text, nil
	}
	if trimmed[0] != '[' {
		return "", fmt.Errorf("invalid message content")
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(trimmed, &parts); err != nil {
		return "", fmt.Errorf("invalid message content")
	}
	var builder strings.Builder
	for _, part := range parts {
		switch part.Type {
		case "input_text", "output_text", "text", "summary_text":
			builder.WriteString(part.Text)
		default:
			return "", fmt.Errorf("unsupported content part type: %s", part.Type)
		}
	}
	return builder.String(), nil
}

// convertTools 把 Responses 工具（扁平 name/parameters）转成 chat.completions 的 function 工具。
func convertTools(tools []json.RawMessage) ([]map[string]any, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	converted := make([]map[string]any, 0, len(tools))
	for _, raw := range tools {
		var tool struct {
			Type        string          `json:"type"`
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Parameters  json.RawMessage `json:"parameters"`
			Function    json.RawMessage `json:"function"`
		}
		if err := json.Unmarshal(raw, &tool); err != nil {
			return nil, fmt.Errorf("invalid tool")
		}
		// 已是 chat.completions 形状则透传
		if len(tool.Function) > 0 {
			var fn map[string]any
			if err := json.Unmarshal(tool.Function, &fn); err != nil {
				return nil, fmt.Errorf("invalid tool")
			}
			converted = append(converted, map[string]any{"type": "function", "function": fn})
			continue
		}
		if tool.Type != "function" || tool.Name == "" {
			return nil, fmt.Errorf("unsupported tool type: %s", tool.Type)
		}
		fn := map[string]any{"name": tool.Name}
		if tool.Description != "" {
			fn["description"] = tool.Description
		}
		if len(tool.Parameters) > 0 {
			var params any
			if err := json.Unmarshal(tool.Parameters, &params); err != nil {
				return nil, fmt.Errorf("invalid tool parameters")
			}
			fn["parameters"] = params
		}
		converted = append(converted, map[string]any{"type": "function", "function": fn})
	}
	return converted, nil
}
