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
	Content   string         `json:"content"`
	ToolCalls []chatToolCall `json:"tool_calls"`
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
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Response is the Responses API non-stream response shape (subset).
type Response struct {
	ID         string           `json:"id"`
	Object     string           `json:"object"`
	CreatedAt  int64            `json:"created_at"`
	Status     string           `json:"status"`
	Model      string           `json:"model"`
	Output     []map[string]any `json:"output"`
	OutputText string           `json:"output_text"`
	Usage      map[string]any   `json:"usage"`
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
	output := make([]map[string]any, 0, 2)
	outputText := ""
	if len(parsed.Choices) > 0 && parsed.Choices[0].Message != nil {
		message := parsed.Choices[0].Message
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
			output = append(output, map[string]any{
				"type": "function_call", "id": itemID, "status": "completed",
				"call_id": callID, "name": call.Function.Name, "arguments": call.Function.Arguments,
			})
		}
	}
	return Response{
		ID: responseID, Object: "response", CreatedAt: time.Now().Unix(),
		Status: "completed", Model: model, Output: output, OutputText: outputText,
		Usage: usagePayload(parsed.Usage),
	}, nil
}

func usagePayload(usage *chatUsage) map[string]any {
	input, output := 0, 0
	if usage != nil {
		input = usage.PromptTokens
		output = usage.CompletionTokens
	}
	return map[string]any{
		"input_tokens": input, "output_tokens": output, "total_tokens": input + output,
		"input_tokens_details":  map[string]int{"cached_tokens": 0},
		"output_tokens_details": map[string]int{"reasoning_tokens": 0},
	}
}
