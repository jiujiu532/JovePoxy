package anthropic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// WriteStream converts an OpenAI chat.completion SSE body into Anthropic Messages SSE events.
func WriteStream(writer http.ResponseWriter, body io.Reader, model string, inputTokens int) {
	reader := bufio.NewReader(body)
	firstEvent, err := readFirstSSEEvent(reader)
	if len(firstEvent) == 0 && errors.Is(err, io.EOF) {
		writeError(writer, http.StatusBadGateway, "upstream_error", "Empty response")
		return
	}
	if err != nil && !errors.Is(err, io.EOF) && len(firstEvent) == 0 {
		writeError(writer, http.StatusBadGateway, "upstream_error", "upstream response failed")
		return
	}
	if isRateLimitEvent(firstEvent) {
		writeError(writer, http.StatusTooManyRequests, "rate_limit_error", "upstream rate limit exceeded (free model rate limit)")
		return
	}

	messageID, idErr := NewMessageID()
	if idErr != nil {
		writeError(writer, http.StatusInternalServerError, "api_error", "failed to allocate message id")
		return
	}
	if !writeStreamHeaders(writer) {
		return
	}
	state := streamState{messageID: messageID, model: model, inputTokens: inputTokens, toolIdx: -1}
	if !state.emitMessageStart(writer) {
		return
	}
	if !state.consumeEvent(writer, firstEvent) {
		return
	}
	if errors.Is(err, io.EOF) {
		state.finishIfNeeded(writer)
		return
	}

	var buffer bytes.Buffer
	chunk := make([]byte, 32*1024)
	for {
		count, readErr := reader.Read(chunk)
		if count > 0 {
			buffer.Write(chunk[:count])
			for {
				line, ok := readLine(&buffer)
				if !ok {
					break
				}
				if !state.consumeLine(writer, line) {
					return
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			if remainder := bytes.TrimSpace(buffer.Bytes()); len(remainder) > 0 {
				_ = state.consumeLine(writer, string(remainder))
			}
			state.finishIfNeeded(writer)
			return
		}
		if readErr != nil {
			return
		}
	}
}

type streamState struct {
	messageID    string
	model        string
	inputTokens  int
	outputTokens int
	contentIdx   int
	toolIdx      int
	finished     bool
}

func (state *streamState) emitMessageStart(writer http.ResponseWriter) bool {
	return writeSSE(writer, "message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id": state.messageID, "type": "message", "role": "assistant", "content": []any{},
			"model": state.model, "stop_reason": nil,
			"usage": map[string]int{
				"input_tokens": state.inputTokens, "output_tokens": 0,
				"cache_creation_input_tokens": 0, "cache_read_input_tokens": 0,
			},
		},
	})
}

func (state *streamState) consumeEvent(writer http.ResponseWriter, event []byte) bool {
	for _, line := range strings.Split(string(event), "\n") {
		if !state.consumeLine(writer, strings.TrimRight(line, "\r")) {
			return false
		}
	}
	return true
}

func (state *streamState) consumeLine(writer http.ResponseWriter, line string) bool {
	if state.finished || !strings.HasPrefix(line, "data:") {
		return true
	}
	payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if payload == "" || payload == "[DONE]" {
		return true
	}
	var parsed openAIStreamChunk
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil || len(parsed.Choices) == 0 {
		return true
	}
	choice := parsed.Choices[0]
	if !state.emitText(writer, choice.Delta.Content) {
		return false
	}
	if !state.emitToolCalls(writer, choice.Delta.ToolCalls) {
		return false
	}
	if choice.FinishReason != "" {
		return state.finish(writer, choice.FinishReason)
	}
	return true
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string            `json:"content"`
			ToolCalls []openAIStreamTool `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

type openAIStreamTool struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (state *streamState) emitText(writer http.ResponseWriter, content string) bool {
	if content == "" {
		return true
	}
	if state.contentIdx == 0 && state.toolIdx == -1 {
		if !writeSSE(writer, "content_block_start", map[string]any{
			"type": "content_block_start", "index": 0,
			"content_block": map[string]any{"type": "text", "text": ""},
		}) {
			return false
		}
		state.contentIdx = 1
	}
	if !writeSSE(writer, "content_block_delta", map[string]any{
		"type": "content_block_delta", "index": 0,
		"delta": map[string]any{"type": "text_delta", "text": content},
	}) {
		return false
	}
	state.outputTokens += (len(content) + 3) / 4
	return true
}

func (state *streamState) emitToolCalls(writer http.ResponseWriter, toolCalls []openAIStreamTool) bool {
	for _, toolCall := range toolCalls {
		idx := toolCall.Index
		if idx > state.toolIdx {
			if state.toolIdx == -1 && state.contentIdx > 0 {
				if !writeSSE(writer, "content_block_stop", map[string]any{"type": "content_block_stop", "index": 0}) {
					return false
				}
			}
			state.toolIdx = idx
			blockIdx := state.toolBlockIndex(idx)
			toolID := toolCall.ID
			if toolID == "" {
				var err error
				toolID, err = NewToolUseID()
				if err != nil {
					return false
				}
			}
			if !writeSSE(writer, "content_block_start", map[string]any{
				"type": "content_block_start", "index": blockIdx,
				"content_block": map[string]any{"type": "tool_use", "id": toolID, "name": toolCall.Function.Name},
			}) {
				return false
			}
		}
		if toolCall.Function.Arguments != "" {
			if !writeSSE(writer, "content_block_delta", map[string]any{
				"type": "content_block_delta", "index": state.toolBlockIndex(toolCall.Index),
				"delta": map[string]any{"type": "input_json_delta", "partial_json": toolCall.Function.Arguments},
			}) {
				return false
			}
			state.outputTokens += (len(toolCall.Function.Arguments) + 3) / 4
		}
	}
	return true
}

func (state *streamState) toolBlockIndex(idx int) int {
	if state.contentIdx > 0 {
		return idx + 1
	}
	return idx
}

func (state *streamState) finishIfNeeded(writer http.ResponseWriter) {
	if !state.finished {
		_ = state.finish(writer, "stop")
	}
}

func (state *streamState) finish(writer http.ResponseWriter, finishReason string) bool {
	if state.finished {
		return true
	}
	state.finished = true
	totalBlocks := 0
	if state.contentIdx > 0 {
		totalBlocks++
	}
	if state.toolIdx >= 0 {
		totalBlocks += state.toolIdx + 1
	}
	for index := 0; index < totalBlocks; index++ {
		if !writeSSE(writer, "content_block_stop", map[string]any{"type": "content_block_stop", "index": index}) {
			return false
		}
	}
	if !writeSSE(writer, "message_delta", map[string]any{
		"type": "message_delta",
		"delta": map[string]any{"stop_reason": mapStopReason(finishReason)},
		"usage": map[string]int{"output_tokens": state.outputTokens},
	}) {
		return false
	}
	return writeSSE(writer, "message_stop", map[string]any{"type": "message_stop"})
}
