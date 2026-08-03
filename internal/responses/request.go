package responses

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"jovepoxy/internal/effort"
)

// Request is a parsed OpenAI Responses API request.
type Request struct {
	Model             string
	Stream            bool
	MaxOutputTokens   int
	Instructions      string
	Input             json.RawMessage
	Tools             []json.RawMessage
	ReasoningEffort   string
	ToolChoice        json.RawMessage
	ParallelToolCalls *bool
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
		Reasoning       *struct {
			Effort string `json:"effort"`
		} `json:"reasoning"`
		ReasoningEffort   string          `json:"reasoning_effort"`
		ToolChoice        json.RawMessage `json:"tool_choice"`
		ParallelToolCalls *bool           `json:"parallel_tool_calls"`
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

	// Prefer nested reasoning.effort; fall back to top-level reasoning_effort.
	reasoningEffort := ""
	if raw.Reasoning != nil {
		reasoningEffort = raw.Reasoning.Effort
	}
	if strings.TrimSpace(reasoningEffort) == "" {
		reasoningEffort = raw.ReasoningEffort
	}

	toolChoice := normalizeRawJSON(raw.ToolChoice)

	return Request{
		Model:             raw.Model,
		Stream:            raw.Stream,
		MaxOutputTokens:   raw.MaxOutputTokens,
		Instructions:      raw.Instructions,
		Input:             raw.Input,
		Tools:             raw.Tools,
		ReasoningEffort:   reasoningEffort,
		ToolChoice:        toolChoice,
		ParallelToolCalls: raw.ParallelToolCalls,
	}, nil
}

// normalizeRawJSON returns nil for empty / null JSON so omitempty works.
func normalizeRawJSON(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	return trimmed
}

type chatMessage struct {
	Role             string           `json:"role"`
	Content          any              `json:"content"`
	ToolCalls        []map[string]any `json:"tool_calls,omitempty"`
	ToolCallID       string           `json:"tool_call_id,omitempty"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
}

type chatRequest struct {
	Model             string           `json:"model"`
	Messages          []chatMessage    `json:"messages"`
	Stream            bool             `json:"stream"`
	MaxTokens         int              `json:"max_tokens,omitempty"`
	Tools             []map[string]any `json:"tools,omitempty"`
	ReasoningEffort   string           `json:"reasoning_effort,omitempty"`
	ToolChoice        json.RawMessage  `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool            `json:"parallel_tool_calls,omitempty"`
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
		Model:             request.Model,
		Messages:          messages,
		Stream:            request.Stream,
		MaxTokens:         request.MaxOutputTokens,
		Tools:             tools,
		ReasoningEffort:   effort.NormalizeLevel(request.ReasoningEffort),
		ToolChoice:        request.ToolChoice,
		ParallelToolCalls: request.ParallelToolCalls,
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

	pendingReasoning := ""
	awaitingToolOutputs := make(map[string]struct{})
	deferred := make([]chatMessage, 0)
	var pendingToolCalls []map[string]any
	var pendingToolCallIDs []string

	appendMessage := func(msg chatMessage) {
		messages = append(messages, msg)
	}
	takePendingReasoning := func() string {
		r := pendingReasoning
		pendingReasoning = ""
		return r
	}
	flushPendingToolCalls := func() {
		if len(pendingToolCalls) == 0 {
			return
		}
		msg := chatMessage{
			Role:      "assistant",
			Content:   "",
			ToolCalls: pendingToolCalls,
		}
		if r := takePendingReasoning(); r != "" {
			msg.ReasoningContent = r
		}
		appendMessage(msg)
		for _, id := range pendingToolCallIDs {
			if strings.TrimSpace(id) == "" {
				continue
			}
			awaitingToolOutputs[id] = struct{}{}
		}
		pendingToolCalls = nil
		pendingToolCallIDs = nil
	}
	flushDeferred := func() {
		for _, msg := range deferred {
			appendMessage(msg)
		}
		deferred = deferred[:0]
	}
	// Keep tool-call adjacency: assistant(tool_calls) → tool messages with no
	// other roles interleaved while outputs are still outstanding.
	appendRegular := func(msg chatMessage) {
		if len(awaitingToolOutputs) > 0 {
			deferred = append(deferred, msg)
			return
		}
		appendMessage(msg)
	}
	appendPendingReasoning := func(text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		if pendingReasoning == "" {
			pendingReasoning = text
			return
		}
		pendingReasoning += "\n" + text
	}

	for _, raw := range items {
		var item inputItem
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, fmt.Errorf("invalid input item")
		}
		// 缺省 type：带 role 视作 message
		kind := item.Type
		if kind == "" && item.Role != "" {
			kind = "message"
		}

		// Buffer consecutive function_calls; flush before any other item type.
		if kind != "function_call" {
			flushPendingToolCalls()
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
			msg := chatMessage{Role: role, Content: content}
			if role == "assistant" {
				if r := takePendingReasoning(); r != "" {
					msg.ReasoningContent = r
				}
			}
			appendRegular(msg)

		case "reasoning":
			// Accumulate for the next assistant (message or merged function_call batch).
			appendPendingReasoning(extractReasoningText(item))

		case "function_call":
			callID := item.CallID
			if callID == "" {
				generated, idErr := NewCallID()
				if idErr != nil {
					return nil, idErr
				}
				callID = generated
			}
			// Keep arguments as raw string; empty becomes "{}" for chat shape.
			args := item.Arguments
			if args == "" {
				args = "{}"
			}
			call := map[string]any{
				"id":   callID,
				"type": "function",
				"function": map[string]any{
					"name":      item.Name,
					"arguments": args,
				},
			}
			pendingToolCalls = append(pendingToolCalls, call)
			pendingToolCallIDs = append(pendingToolCallIDs, callID)

		case "function_call_output":
			output := strings.TrimSpace(string(item.Output))
			if len(output) > 1 && output[0] == '"' {
				var text string
				if err := json.Unmarshal(item.Output, &text); err == nil {
					output = text
				}
			}
			appendMessage(chatMessage{Role: "tool", Content: output, ToolCallID: item.CallID})
			if callID := strings.TrimSpace(item.CallID); callID != "" {
				delete(awaitingToolOutputs, callID)
			}
			if len(awaitingToolOutputs) == 0 && len(deferred) > 0 {
				flushDeferred()
			}

		default:
			return nil, fmt.Errorf("unsupported input item type: %s", kind)
		}
	}

	flushPendingToolCalls()
	// Orphan reasoning with no following assistant: emit as empty-content assistant
	// only when there is text (omit key entirely if empty — already handled).
	if r := takePendingReasoning(); r != "" {
		appendRegular(chatMessage{Role: "assistant", Content: "", ReasoningContent: r})
	}
	flushDeferred()
	return messages, nil
}

type inputItem struct {
	Type      string          `json:"type"`
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	Summary   json.RawMessage `json:"summary"`
	CallID    string          `json:"call_id"`
	Name      string          `json:"name"`
	Arguments string          `json:"arguments"`
	Output    json.RawMessage `json:"output"`
}

// extractReasoningText collects text from reasoning item summary/content parts.
// Unknown part types are skipped (no error) so cold fields do not break conversion.
// Empty result means the caller should omit reasoning_content.
func extractReasoningText(item inputItem) string {
	var builder strings.Builder
	appendField := func(raw json.RawMessage) {
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) == 0 {
			return
		}
		if trimmed[0] == '"' {
			var text string
			if err := json.Unmarshal(trimmed, &text); err == nil {
				builder.WriteString(text)
			}
			return
		}
		if trimmed[0] != '[' {
			return
		}
		var parts []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(trimmed, &parts); err != nil {
			return
		}
		for _, part := range parts {
			switch part.Type {
			case "input_text", "output_text", "text", "summary_text", "":
				builder.WriteString(part.Text)
			}
		}
	}
	appendField(item.Summary)
	appendField(item.Content)
	return builder.String()
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
