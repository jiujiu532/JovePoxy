package responses

import (
	"encoding/json"
	"fmt"
	"time"
)

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Usage   *chatUsage   `json:"usage"`
}

type chatChoice struct {
	FinishReason string           `json:"finish_reason"`
	Message      *chatRespMessage `json:"message"`
}

type chatRespMessage struct {
	Content          string         `json:"content"`
	ReasoningContent string         `json:"reasoning_content"`
	Reasoning        string         `json:"reasoning"`
	ToolCalls        []chatToolCall `json:"tool_calls"`
}

// reasoningText accepts both reasoning_content and reasoning aliases (stream parity).
func (m *chatRespMessage) reasoningText() string {
	if m == nil {
		return ""
	}
	if m.ReasoningContent != "" {
		return m.ReasoningContent
	}
	return m.Reasoning
}

type chatToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatUsage struct {
	PromptTokens             int `json:"prompt_tokens"`
	CompletionTokens         int `json:"completion_tokens"`
	TotalTokens              int `json:"total_tokens"`
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

// Response is the Responses API non-stream response shape (subset).
type Response struct {
	ID                string           `json:"id"`
	Object            string           `json:"object"`
	CreatedAt         int64            `json:"created_at"`
	Status            string           `json:"status"`
	Model             string           `json:"model"`
	Output            []map[string]any `json:"output"`
	OutputText        string           `json:"output_text"`
	Usage             map[string]any   `json:"usage"`
	IncompleteDetails map[string]any   `json:"incomplete_details,omitempty"`
}

// FromOpenAI converts a non-stream chat.completion JSON body into a Responses object.
func FromOpenAI(body []byte, model string) (Response, error) {
	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Response{}, fmt.Errorf("invalid openai response")
	}
	responseID, err := NewResponseID()
	if err != nil {
		return Response{}, err
	}
	output := make([]map[string]any, 0, 3)
	outputText := ""
	finishReason := ""
	if len(parsed.Choices) > 0 && parsed.Choices[0].Message != nil {
		choice := parsed.Choices[0]
		finishReason = choice.FinishReason
		message := choice.Message
		// reasoning 优先于 message / function_call，便于 Codex 展示思考过程
		if reasoning := message.reasoningText(); reasoning != "" {
			reasoningID, idErr := NewReasoningID()
			if idErr != nil {
				return Response{}, idErr
			}
			output = append(output, map[string]any{
				"id": reasoningID, "type": "reasoning", "status": "completed",
				"summary": []map[string]any{{
					"type": "summary_text", "text": reasoning,
				}},
			})
		}
		if message.Content != "" {
			messageID, idErr := NewMessageID()
			if idErr != nil {
				return Response{}, idErr
			}
			outputText = message.Content
			output = append(output, map[string]any{
				"type": "message", "id": messageID, "status": "completed", "role": "assistant",
				"content": []map[string]any{{
					"type": "output_text", "text": message.Content, "annotations": []any{},
				}},
			})
		}
		for _, call := range message.ToolCalls {
			itemID, idErr := NewFunctionCallID()
			if idErr != nil {
				return Response{}, idErr
			}
			callID := call.ID
			if callID == "" {
				if callID, idErr = NewCallID(); idErr != nil {
					return Response{}, idErr
				}
			}
			// Keep raw arguments string (including illegal JSON) — never silence to {}.
			output = append(output, map[string]any{
				"type": "function_call", "id": itemID, "status": "completed",
				"call_id": callID, "name": call.Function.Name, "arguments": call.Function.Arguments,
			})
		}
	}
	status := "completed"
	var incomplete map[string]any
	if isIncompleteFinish(finishReason) {
		status = "incomplete"
		incomplete = map[string]any{"reason": incompleteReason(finishReason)}
	}
	return Response{
		ID: responseID, Object: "response", CreatedAt: time.Now().Unix(),
		Status: status, Model: model, Output: output, OutputText: outputText,
		Usage: usagePayload(parsed.Usage), IncompleteDetails: incomplete,
	}, nil
}

func usagePayload(usage *chatUsage) map[string]any {
	input, output, cached := 0, 0, 0
	if usage != nil {
		input = usage.PromptTokens
		output = usage.CompletionTokens
		cached = usage.CacheReadInputTokens
		if cached == 0 && usage.PromptTokensDetails != nil && usage.PromptTokensDetails.CachedTokens > 0 {
			cached = usage.PromptTokensDetails.CachedTokens
		}
		if cached == 0 && usage.InputTokensDetails != nil && usage.InputTokensDetails.CachedTokens > 0 {
			cached = usage.InputTokensDetails.CachedTokens
		}
	}
	return map[string]any{
		"input_tokens": input, "output_tokens": output, "total_tokens": input + output,
		"input_tokens_details":  map[string]int{"cached_tokens": cached},
		"output_tokens_details": map[string]int{"reasoning_tokens": 0},
	}
}
