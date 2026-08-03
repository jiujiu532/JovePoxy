package anthropic

import (
	"encoding/json"
	"fmt"
)

type openAIResponse struct {
	Choices []openAIChoice `json:"choices"`
	Usage   *openAIUsage   `json:"usage"`
}

type openAIChoice struct {
	FinishReason string         `json:"finish_reason"`
	Message      *openAIMessage `json:"message"`
}

type openAIMessage struct {
	Content          string           `json:"content"`
	ReasoningContent string           `json:"reasoning_content"`
	Reasoning        string           `json:"reasoning"`
	ToolCalls        []openAIToolCall `json:"tool_calls"`
}

// reasoningText returns non-stream reasoning, accepting both field aliases
// used by upstreams (reasoning_content preferred, then reasoning).
func (m *openAIMessage) reasoningText() string {
	if m == nil {
		return ""
	}
	if m.ReasoningContent != "" {
		return m.ReasoningContent
	}
	return m.Reasoning
}

type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAIUsage struct {
	PromptTokens             int `json:"prompt_tokens"`
	CompletionTokens         int `json:"completion_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	PromptTokensDetails      *struct {
		CachedTokens     int `json:"cached_tokens"`
		CacheWriteTokens int `json:"cache_write_tokens"`
	} `json:"prompt_tokens_details"`
	InputTokensDetails *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
}

// Message is the Anthropic Messages API non-stream response shape.
type Message struct {
	ID         string           `json:"id"`
	Type       string           `json:"type"`
	Role       string           `json:"role"`
	Content    []map[string]any `json:"content"`
	Model      string           `json:"model"`
	StopReason string           `json:"stop_reason"`
	Usage      map[string]int   `json:"usage"`
}

// FromOpenAI converts a non-stream OpenAI chat.completion JSON body.
func FromOpenAI(body []byte, model string, inputTokens int) (Message, error) {
	var response openAIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return Message{}, fmt.Errorf("invalid openai response")
	}
	messageID, err := NewMessageID()
	if err != nil {
		return Message{}, err
	}
	if len(response.Choices) == 0 || response.Choices[0].Message == nil {
		return Message{
			ID: messageID, Type: "message", Role: "assistant",
			Content: []map[string]any{{"type": "text", "text": ""}},
			Model:   model, StopReason: "end_turn",
			Usage: usageMap(inputTokens, 0, response.Usage),
		}, nil
	}
	choice := response.Choices[0]
	content := make([]map[string]any, 0, 2+len(choice.Message.ToolCalls))
	if reasoning := choice.Message.reasoningText(); reasoning != "" {
		content = append(content, map[string]any{
			"type": "thinking", "thinking": reasoning,
		})
	}
	if choice.Message.Content != "" {
		content = append(content, map[string]any{"type": "text", "text": choice.Message.Content})
	}
	for _, toolCall := range choice.Message.ToolCalls {
		// Prefer parsed object when arguments are valid JSON object/array;
		// otherwise keep a raw string under input so illegal JSON is not silenced to {}.
		input := parseToolInput(toolCall.Function.Arguments)
		toolID := toolCall.ID
		if toolID == "" {
			toolID, err = NewToolUseID()
			if err != nil {
				return Message{}, err
			}
		}
		content = append(content, map[string]any{
			"type": "tool_use", "id": toolID, "name": toolCall.Function.Name, "input": input,
		})
	}
	if len(content) == 0 {
		content = append(content, map[string]any{"type": "text", "text": ""})
	}
	outputTokens := 0
	if response.Usage != nil {
		outputTokens = response.Usage.CompletionTokens
	}
	return Message{
		ID: messageID, Type: "message", Role: "assistant", Content: content,
		Model: model, StopReason: mapStopReason(choice.FinishReason),
		Usage: usageMap(inputTokens, outputTokens, response.Usage),
	}, nil
}

// parseToolInput maps OpenAI function.arguments (JSON string) to Anthropic tool_use.input.
// Valid JSON object/array → decoded value; empty → {}; invalid JSON → raw string (not {}).
func parseToolInput(arguments string) any {
	if arguments == "" {
		return map[string]any{}
	}
	var decoded any
	if err := json.Unmarshal([]byte(arguments), &decoded); err != nil {
		// Keep raw so callers can see the illegal payload instead of silent {}.
		return arguments
	}
	if decoded == nil {
		return map[string]any{}
	}
	return decoded
}

func mapStopReason(finishReason string) string {
	switch finishReason {
	case "tool_calls":
		return "tool_use"
	case "length":
		return "max_tokens"
	default:
		return "end_turn"
	}
}

func usageMap(inputTokens, outputTokens int, usage *openAIUsage) map[string]int {
	cacheRead, cacheCreation := 0, 0
	if usage != nil {
		if usage.PromptTokens > 0 {
			inputTokens = usage.PromptTokens
		}
		if usage.CompletionTokens > 0 {
			outputTokens = usage.CompletionTokens
		}
		cacheRead = usage.CacheReadInputTokens
		cacheCreation = usage.CacheCreationInputTokens
		if cacheRead == 0 && usage.PromptTokensDetails != nil && usage.PromptTokensDetails.CachedTokens > 0 {
			cacheRead = usage.PromptTokensDetails.CachedTokens
		}
		if cacheRead == 0 && usage.InputTokensDetails != nil && usage.InputTokensDetails.CachedTokens > 0 {
			cacheRead = usage.InputTokensDetails.CachedTokens
		}
		if cacheCreation == 0 && usage.PromptTokensDetails != nil && usage.PromptTokensDetails.CacheWriteTokens > 0 {
			cacheCreation = usage.PromptTokensDetails.CacheWriteTokens
		}
	}
	return map[string]int{
		"input_tokens": inputTokens, "output_tokens": outputTokens,
		"cache_creation_input_tokens": cacheCreation, "cache_read_input_tokens": cacheRead,
	}
}
