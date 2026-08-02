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
	ToolCalls        []openAIToolCall `json:"tool_calls"`
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
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
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
	if choice.Message.ReasoningContent != "" {
		content = append(content, map[string]any{
			"type": "thinking", "thinking": choice.Message.ReasoningContent,
		})
	}
	if choice.Message.Content != "" {
		content = append(content, map[string]any{"type": "text", "text": choice.Message.Content})
	}
	for _, toolCall := range choice.Message.ToolCalls {
		input := map[string]any{}
		if toolCall.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
				input = map[string]any{}
			}
		}
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
	if usage != nil && usage.PromptTokens > 0 {
		inputTokens = usage.PromptTokens
	}
	return map[string]int{
		"input_tokens": inputTokens, "output_tokens": outputTokens,
		"cache_creation_input_tokens": 0, "cache_read_input_tokens": 0,
	}
}
