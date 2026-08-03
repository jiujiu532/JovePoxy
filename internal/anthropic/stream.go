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

	"jovepoxy/internal/sse"
)

// WriteStream converts an OpenAI chat.completion SSE body into Anthropic Messages SSE events.
func WriteStream(writer http.ResponseWriter, body io.Reader, model string, inputTokens int) {
	// First-event probe without idle wrapper so early JSON errors do not leave a
	// background reader holding the upstream body.
	reader := bufio.NewReader(body)
	firstEvent, err := sse.ReadFirstEvent(reader)
	if len(firstEvent) == 0 && errors.Is(err, io.EOF) {
		writeError(writer, http.StatusBadGateway, "upstream_error", "Empty response")
		return
	}
	if err != nil && !errors.Is(err, io.EOF) && len(firstEvent) == 0 {
		writeError(writer, http.StatusBadGateway, "upstream_error", "upstream response failed")
		return
	}
	if sse.IsRateLimitEvent(firstEvent) {
		writeError(writer, http.StatusTooManyRequests, "rate_limit_error", "upstream rate limit exceeded (free model rate limit)")
		return
	}
	if msg, isErr := sse.ErrorEventMessage(firstEvent); isErr {
		if msg == "" {
			msg = "upstream error"
		}
		writeError(writer, http.StatusBadGateway, "upstream_error", msg)
		return
	}

	messageID, idErr := NewMessageID()
	if idErr != nil {
		writeError(writer, http.StatusInternalServerError, "api_error", "failed to allocate message id")
		return
	}
	if !sse.WriteHeaders(writer) {
		return
	}
	state := streamState{messageID: messageID, model: model, inputTokens: inputTokens}
	if !state.emitMessageStart(writer) {
		return
	}
	if !state.consumeEvent(writer, firstEvent) {
		// Headers already sent: best-effort terminal frame.
		state.finishIfNeeded(writer)
		return
	}
	if errors.Is(err, io.EOF) {
		state.finishIfNeeded(writer)
		return
	}

	// Idle timeout only for the remainder of the upstream body.
	reader = bufio.NewReader(sse.IdleReader(reader, sse.DefaultIdleTimeout))
	var buffer bytes.Buffer
	chunk := make([]byte, 32*1024)
	for {
		count, readErr := reader.Read(chunk)
		if count > 0 {
			buffer.Write(chunk[:count])
			for {
				line, ok := sse.ReadLine(&buffer)
				if !ok {
					break
				}
				if !state.consumeLine(writer, line) {
					state.finishIfNeeded(writer)
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
			// Mid-stream read failure: best-effort message_stop if not finished.
			state.finishIfNeeded(writer)
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
	// anthropic block index -> emitted name (for late name backfill tracking)
	toolNames map[int]string
	// anthropic block index -> emitted id
	toolIDs map[int]string
}

func (state *streamState) emitMessageStart(writer http.ResponseWriter) bool {
	return sse.WriteEvent(writer, "message_start", map[string]any{
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
	// Mid-stream OpenAI error object: stop without fake success (headers already sent).
	if msg, isErr := sse.ErrorEventMessage([]byte("data: " + payload + "\n\n")); isErr {
		_ = state.emitErrorAndStop(writer, msg)
		return false
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
		if !sse.WriteEvent(writer, "content_block_start", map[string]any{
			"type": "content_block_start", "index": state.thinkingIndex,
			"content_block": map[string]any{"type": "thinking", "thinking": ""},
		}) {
			return false
		}
		state.thinkingOpen = true
	}
	if !sse.WriteEvent(writer, "content_block_delta", map[string]any{
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
		// tool→text：先关闭已 open 的 tool blocks（与 finish 路径一致）
		if !state.closeOpenTools(writer) {
			return false
		}
		if !state.closeThinking(writer) {
			return false
		}
		state.textIndex = state.nextBlock
		state.nextBlock++
		if !sse.WriteEvent(writer, "content_block_start", map[string]any{
			"type": "content_block_start", "index": state.textIndex,
			"content_block": map[string]any{"type": "text", "text": ""},
		}) {
			return false
		}
		state.textOpen = true
	}
	if !sse.WriteEvent(writer, "content_block_delta", map[string]any{
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
			if state.toolNames == nil {
				state.toolNames = make(map[int]string)
			}
			if state.toolIDs == nil {
				state.toolIDs = make(map[int]string)
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
			name := toolCall.Function.Name
			state.toolIDs[blockIdx] = toolID
			state.toolNames[blockIdx] = name
			if !sse.WriteEvent(writer, "content_block_start", map[string]any{
				"type": "content_block_start", "index": blockIdx,
				"content_block": map[string]any{"type": "tool_use", "id": toolID, "name": name},
			}) {
				return false
			}
		} else {
			// Late name/id backfill on an already-open tool block.
			if toolCall.ID != "" && state.toolIDs != nil {
				if cur := state.toolIDs[blockIdx]; cur == "" {
					state.toolIDs[blockIdx] = toolCall.ID
				}
			}
			if toolCall.Function.Name != "" && state.toolNames != nil {
				if cur := state.toolNames[blockIdx]; cur == "" {
					state.toolNames[blockIdx] = toolCall.Function.Name
					// Re-announce block start with filled name so clients that
					// only saw empty name can recover (mirrors responses late fill).
					toolID := ""
					if state.toolIDs != nil {
						toolID = state.toolIDs[blockIdx]
					}
					if toolID == "" {
						toolID = toolCall.ID
					}
					if !sse.WriteEvent(writer, "content_block_start", map[string]any{
						"type": "content_block_start", "index": blockIdx,
						"content_block": map[string]any{
							"type": "tool_use", "id": toolID, "name": toolCall.Function.Name,
						},
					}) {
						return false
					}
				}
			}
		}
		if toolCall.Function.Arguments != "" {
			if !sse.WriteEvent(writer, "content_block_delta", map[string]any{
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
	if !sse.WriteEvent(writer, "content_block_stop", map[string]any{
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
	if !sse.WriteEvent(writer, "content_block_stop", map[string]any{
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
		if !sse.WriteEvent(writer, "content_block_stop", map[string]any{
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

func (state *streamState) emitErrorAndStop(writer http.ResponseWriter, message string) bool {
	if state.finished {
		return true
	}
	// Close open blocks then emit error + message_stop (no fake end_turn success).
	_ = state.closeThinking(writer)
	_ = state.closeText(writer)
	_ = state.closeOpenTools(writer)
	state.finished = true
	if message == "" {
		message = "upstream error"
	}
	if !sse.WriteEvent(writer, "error", map[string]any{
		"type": "error",
		"error": map[string]string{
			"type": "api_error", "message": message,
		},
	}) {
		return false
	}
	return sse.WriteEvent(writer, "message_stop", map[string]any{"type": "message_stop"})
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
	if !sse.WriteEvent(writer, "message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": mapStopReason(finishReason)},
		"usage": map[string]int{"output_tokens": state.outputTokens},
	}) {
		return false
	}
	return sse.WriteEvent(writer, "message_stop", map[string]any{"type": "message_stop"})
}
