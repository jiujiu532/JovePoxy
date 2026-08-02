package anthropic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
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
	state := streamState{messageID: messageID, model: model, inputTokens: inputTokens}
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

// streamState tracks open content blocks and allocates Anthropic content indexes.
// thinking occupies an index when present; text/tool indexes follow via nextBlock.
type streamState struct {
	messageID    string
	model        string
	inputTokens  int
	outputTokens int
	finished     bool

	thinkingOpen  bool
	thinkingIndex int

	textOpen  bool
	textIndex int

	nextBlock int

	// openai tool call index -> anthropic content block index
	toolBlocks map[int]int
	// anthropic content block index still open (tool_use)
	toolOpen map[int]bool
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
	if !state.emitReasoning(writer, choice.Delta.reasoningText()) {
		return false
	}
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
		Delta        openAIStreamDelta `json:"delta"`
		FinishReason string            `json:"finish_reason"`
	} `json:"choices"`
}

type openAIStreamDelta struct {
	Content          string             `json:"content"`
	ReasoningContent string             `json:"reasoning_content"`
	Reasoning        string             `json:"reasoning"`
	ToolCalls        []openAIStreamTool `json:"tool_calls"`
}

func (d openAIStreamDelta) reasoningText() string {
	if d.ReasoningContent != "" {
		return d.ReasoningContent
	}
	return d.Reasoning
}

type openAIStreamTool struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (state *streamState) emitReasoning(writer http.ResponseWriter, text string) bool {
	if text == "" {
		return true
	}
	if !state.thinkingOpen {
		// Prefer thinking before text/tool. If text already started, close it first
		// (rare interleave); drop late reasoning once tools have started.
		if len(state.toolBlocks) > 0 {
			return true
		}
		if !state.closeText(writer) {
			return false
		}
		state.thinkingIndex = state.nextBlock
		state.nextBlock++
		if !writeSSE(writer, "content_block_start", map[string]any{
			"type": "content_block_start", "index": state.thinkingIndex,
			"content_block": map[string]any{"type": "thinking", "thinking": ""},
		}) {
			return false
		}
		state.thinkingOpen = true
	}
	if !writeSSE(writer, "content_block_delta", map[string]any{
		"type": "content_block_delta", "index": state.thinkingIndex,
		"delta": map[string]any{"type": "thinking_delta", "thinking": text},
	}) {
		return false
	}
	state.outputTokens += (len(text) + 3) / 4
	return true
}

func (state *streamState) emitText(writer http.ResponseWriter, content string) bool {
	if content == "" {
		return true
	}
	if !state.textOpen {
		if !state.closeThinking(writer) {
			return false
		}
		state.textIndex = state.nextBlock
		state.nextBlock++
		if !writeSSE(writer, "content_block_start", map[string]any{
			"type": "content_block_start", "index": state.textIndex,
			"content_block": map[string]any{"type": "text", "text": ""},
		}) {
			return false
		}
		state.textOpen = true
	}
	if !writeSSE(writer, "content_block_delta", map[string]any{
		"type": "content_block_delta", "index": state.textIndex,
		"delta": map[string]any{"type": "text_delta", "text": content},
	}) {
		return false
	}
	state.outputTokens += (len(content) + 3) / 4
	return true
}

func (state *streamState) emitToolCalls(writer http.ResponseWriter, toolCalls []openAIStreamTool) bool {
	for _, toolCall := range toolCalls {
		blockIdx, exists := state.toolBlocks[toolCall.Index]
		if !exists {
			if !state.closeThinking(writer) {
				return false
			}
			if !state.closeText(writer) {
				return false
			}
			if state.toolBlocks == nil {
				state.toolBlocks = make(map[int]int)
			}
			if state.toolOpen == nil {
				state.toolOpen = make(map[int]bool)
			}
			blockIdx = state.nextBlock
			state.nextBlock++
			state.toolBlocks[toolCall.Index] = blockIdx
			state.toolOpen[blockIdx] = true

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
				"type": "content_block_delta", "index": blockIdx,
				"delta": map[string]any{"type": "input_json_delta", "partial_json": toolCall.Function.Arguments},
			}) {
				return false
			}
			state.outputTokens += (len(toolCall.Function.Arguments) + 3) / 4
		}
	}
	return true
}

func (state *streamState) closeThinking(writer http.ResponseWriter) bool {
	if !state.thinkingOpen {
		return true
	}
	if !writeSSE(writer, "content_block_stop", map[string]any{
		"type": "content_block_stop", "index": state.thinkingIndex,
	}) {
		return false
	}
	state.thinkingOpen = false
	return true
}

func (state *streamState) closeText(writer http.ResponseWriter) bool {
	if !state.textOpen {
		return true
	}
	if !writeSSE(writer, "content_block_stop", map[string]any{
		"type": "content_block_stop", "index": state.textIndex,
	}) {
		return false
	}
	state.textOpen = false
	return true
}

func (state *streamState) closeOpenTools(writer http.ResponseWriter) bool {
	if len(state.toolOpen) == 0 {
		return true
	}
	indexes := make([]int, 0, len(state.toolOpen))
	for idx, open := range state.toolOpen {
		if open {
			indexes = append(indexes, idx)
		}
	}
	sort.Ints(indexes)
	for _, idx := range indexes {
		if !writeSSE(writer, "content_block_stop", map[string]any{
			"type": "content_block_stop", "index": idx,
		}) {
			return false
		}
		delete(state.toolOpen, idx)
	}
	return true
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
	if !state.closeThinking(writer) {
		return false
	}
	if !state.closeText(writer) {
		return false
	}
	if !state.closeOpenTools(writer) {
		return false
	}
	if !writeSSE(writer, "message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": mapStopReason(finishReason)},
		"usage": map[string]int{"output_tokens": state.outputTokens},
	}) {
		return false
	}
	return writeSSE(writer, "message_stop", map[string]any{"type": "message_stop"})
}
