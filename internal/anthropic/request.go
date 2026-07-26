package anthropic

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Request is a parsed Anthropic Messages API request.
type Request struct {
	Model     string
	Stream    bool
	MaxTokens int
	Messages  []json.RawMessage
	System    json.RawMessage
	Tools     []json.RawMessage
}

type openAIChatRequest struct {
	Model     string           `json:"model"`
	Messages  []map[string]any `json:"messages"`
	Stream    bool             `json:"stream"`
	MaxTokens int              `json:"max_tokens,omitempty"`
	Tools     []map[string]any `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type contentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ParseRequest decodes and lightly validates an Anthropic Messages body.
func ParseRequest(body []byte) (Request, error) {
	var raw struct {
		Model     string            `json:"model"`
		Stream    bool              `json:"stream"`
		MaxTokens int               `json:"max_tokens"`
		System    json.RawMessage   `json:"system"`
		Messages  []json.RawMessage `json:"messages"`
		Tools     []json.RawMessage `json:"tools"`
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
	if len(raw.Messages) == 0 {
		return Request{}, fmt.Errorf("messages is required")
	}
	return Request{
		Model: raw.Model, Stream: raw.Stream, MaxTokens: raw.MaxTokens,
		Messages: raw.Messages, System: raw.System, Tools: raw.Tools,
	}, nil
}

// ToOpenAIChat converts an Anthropic request into an OpenAI chat.completions body.
// The second return value is a rough input-token estimate matching the reference proxy.
func ToOpenAIChat(request Request) ([]byte, int, error) {
	messages, err := convertMessages(request)
	if err != nil {
		return nil, 0, err
	}
	tools, err := convertTools(request.Tools)
	if err != nil {
		return nil, 0, err
	}
	payload := openAIChatRequest{
		Model: request.Model, Messages: messages, Stream: request.Stream, MaxTokens: request.MaxTokens, Tools: tools,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("encode openai request: %w", err)
	}
	return encoded, estimateTokens(encoded), nil
}

func convertMessages(request Request) ([]map[string]any, error) {
	messages := make([]map[string]any, 0, len(request.Messages)+1)
	if system := systemText(request.System); system != "" {
		messages = append(messages, map[string]any{"role": "system", "content": system})
	}
	for _, raw := range request.Messages {
		var message anthropicMessage
		if err := json.Unmarshal(raw, &message); err != nil {
			return nil, fmt.Errorf("invalid message")
		}
		converted, err := convertMessage(message)
		if err != nil {
			return nil, err
		}
		messages = append(messages, converted...)
	}
	return messages, nil
}

func systemText(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func convertMessage(message anthropicMessage) ([]map[string]any, error) {
	trimmed := bytes.TrimSpace(message.Content)
	if len(trimmed) == 0 {
		return []map[string]any{{"role": message.Role, "content": ""}}, nil
	}
	if trimmed[0] == '"' {
		var text string
		if err := json.Unmarshal(message.Content, &text); err != nil {
			return nil, fmt.Errorf("invalid message content")
		}
		return []map[string]any{{"role": message.Role, "content": text}}, nil
	}
	var blocks []contentBlock
	if err := json.Unmarshal(message.Content, &blocks); err != nil {
		return nil, fmt.Errorf("invalid message content blocks")
	}
	text := joinTextBlocks(blocks)
	toolUses := filterBlocks(blocks, "tool_use")
	if len(toolUses) > 0 && message.Role == "assistant" {
		toolCalls := make([]map[string]any, 0, len(toolUses))
		for _, tool := range toolUses {
			args := "{}"
			if len(tool.Input) > 0 {
				args = string(tool.Input)
			}
			toolCalls = append(toolCalls, map[string]any{
				"id": tool.ID, "type": "function",
				"function": map[string]any{"name": tool.Name, "arguments": args},
			})
		}
		var content any
		if text != "" {
			content = text
		}
		return []map[string]any{{"role": "assistant", "content": content, "tool_calls": toolCalls}}, nil
	}
	if hasBlockType(blocks, "tool_result") {
		results := make([]map[string]any, 0)
		for _, block := range filterBlocks(blocks, "tool_result") {
			results = append(results, map[string]any{
				"role": "tool", "tool_call_id": block.ToolUseID, "content": toolResultText(block),
			})
		}
		return results, nil
	}
	return []map[string]any{{"role": message.Role, "content": text}}, nil
}

func convertTools(rawTools []json.RawMessage) ([]map[string]any, error) {
	if len(rawTools) == 0 {
		return nil, nil
	}
	tools := make([]map[string]any, 0, len(rawTools))
	for _, raw := range rawTools {
		var tool anthropicTool
		if err := json.Unmarshal(raw, &tool); err != nil {
			return nil, fmt.Errorf("invalid tool definition")
		}
		schema := json.RawMessage(`{}`)
		if len(tool.InputSchema) > 0 {
			schema = tool.InputSchema
		}
		var parameters any
		if err := json.Unmarshal(schema, &parameters); err != nil {
			return nil, fmt.Errorf("invalid tool input_schema")
		}
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": tool.Name, "description": tool.Description, "parameters": parameters,
			},
		})
	}
	return tools, nil
}

func joinTextBlocks(blocks []contentBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Type == "text" && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func filterBlocks(blocks []contentBlock, blockType string) []contentBlock {
	filtered := make([]contentBlock, 0)
	for _, block := range blocks {
		if block.Type == blockType {
			filtered = append(filtered, block)
		}
	}
	return filtered
}

func hasBlockType(blocks []contentBlock, blockType string) bool {
	for _, block := range blocks {
		if block.Type == blockType {
			return true
		}
	}
	return false
}

func toolResultText(block contentBlock) string {
	if len(bytes.TrimSpace(block.Content)) == 0 {
		return ""
	}
	var asString string
	if err := json.Unmarshal(block.Content, &asString); err == nil {
		return asString
	}
	var nested []contentBlock
	if err := json.Unmarshal(block.Content, &nested); err == nil {
		return joinTextBlocks(nested)
	}
	return string(block.Content)
}

func estimateTokens(body []byte) int {
	return len(body) / 4
}
