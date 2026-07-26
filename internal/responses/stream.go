package responses

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"jovepoxy/internal/sse"
)

// WriteStream converts a chat.completion SSE body into Responses API SSE events.
func WriteStream(writer http.ResponseWriter, body io.Reader, model string) {
	reader := bufio.NewReader(body)
	firstEvent, err := sse.ReadFirstEvent(reader)
	if len(firstEvent) == 0 && errors.Is(err, io.EOF) {
		writeError(writer, http.StatusBadGateway, "upstream_error", "upstream returned an empty response")
		return
	}
	if err != nil && !errors.Is(err, io.EOF) && len(firstEvent) == 0 {
		writeError(writer, http.StatusBadGateway, "upstream_error", "upstream response failed")
		return
	}
	if sse.IsRateLimitEvent(firstEvent) {
		writeError(writer, http.StatusTooManyRequests, "rate_limit_error", "upstream rate limit exceeded")
		return
	}

	responseID, idErr := NewResponseID()
	if idErr != nil {
		writeError(writer, http.StatusInternalServerError, "api_error", "failed to allocate response id")
		return
	}
	if !writeStreamHeaders(writer) {
		return
	}
	state := streamState{
		responseID: responseID,
		model:      model,
		createdAt:  time.Now().Unix(),
		toolIdx:    -1,
	}
	if !state.emitCreated(writer) {
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
	responseID  string
	model       string
	createdAt   int64
	sequence    int
	outputIdx   int
	messageID   string
	messageOpen bool
	textBuffer  strings.Builder
	toolIdx     int
	toolItemID  string
	toolCallID  string
	toolName    string
	toolArgs    strings.Builder
	outputItems []map[string]any
	finished    bool
}

func (state *streamState) next() int {
	state.sequence++
	return state.sequence
}

func (state *streamState) responseSnapshot(status string) map[string]any {
	output := state.outputItems
	if output == nil {
		output = []map[string]any{}
	}
	return map[string]any{
		"id": state.responseID, "object": "response", "created_at": state.createdAt,
		"status": status, "model": state.model, "output": output,
	}
}

func (state *streamState) emitCreated(writer http.ResponseWriter) bool {
	if !writeSSE(writer, "response.created", map[string]any{
		"type": "response.created", "sequence_number": state.next(),
		"response": state.responseSnapshot("in_progress"),
	}) {
		return false
	}
	return writeSSE(writer, "response.in_progress", map[string]any{
		"type": "response.in_progress", "sequence_number": state.next(),
		"response": state.responseSnapshot("in_progress"),
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

type chatStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string           `json:"content"`
			ToolCalls []chatStreamTool `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

type chatStreamTool struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (state *streamState) consumeLine(writer http.ResponseWriter, line string) bool {
	if state.finished || !strings.HasPrefix(line, "data:") {
		return true
	}
	payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if payload == "" || payload == "[DONE]" {
		return true
	}
	var parsed chatStreamChunk
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil || len(parsed.Choices) == 0 {
		return true
	}
	choice := parsed.Choices[0]
	if choice.Delta.Content != "" {
		if !state.emitText(writer, choice.Delta.Content) {
			return false
		}
	}
	if len(choice.Delta.ToolCalls) > 0 {
		if !state.emitToolCalls(writer, choice.Delta.ToolCalls) {
			return false
		}
	}
	if choice.FinishReason != "" {
		return state.finish(writer)
	}
	return true
}

func (state *streamState) emitText(writer http.ResponseWriter, content string) bool {
	if !state.messageOpen {
		messageID, err := NewMessageID()
		if err != nil {
			return false
		}
		state.messageID = messageID
		state.messageOpen = true
		if !writeSSE(writer, "response.output_item.added", map[string]any{
			"type": "response.output_item.added", "sequence_number": state.next(),
			"output_index": state.outputIdx,
			"item": map[string]any{
				"type": "message", "id": messageID, "status": "in_progress",
				"role": "assistant", "content": []any{},
			},
		}) {
			return false
		}
		if !writeSSE(writer, "response.content_part.added", map[string]any{
			"type": "response.content_part.added", "sequence_number": state.next(),
			"item_id": messageID, "output_index": state.outputIdx, "content_index": 0,
			"part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}},
		}) {
			return false
		}
	}
	state.textBuffer.WriteString(content)
	return writeSSE(writer, "response.output_text.delta", map[string]any{
		"type": "response.output_text.delta", "sequence_number": state.next(),
		"item_id": state.messageID, "output_index": state.outputIdx, "content_index": 0,
		"delta": content,
	})
}

func (state *streamState) closeMessage(writer http.ResponseWriter) bool {
	if !state.messageOpen {
		return true
	}
	text := state.textBuffer.String()
	if !writeSSE(writer, "response.output_text.done", map[string]any{
		"type": "response.output_text.done", "sequence_number": state.next(),
		"item_id": state.messageID, "output_index": state.outputIdx, "content_index": 0,
		"text": text,
	}) {
		return false
	}
	if !writeSSE(writer, "response.content_part.done", map[string]any{
		"type": "response.content_part.done", "sequence_number": state.next(),
		"item_id": state.messageID, "output_index": state.outputIdx, "content_index": 0,
		"part": map[string]any{"type": "output_text", "text": text, "annotations": []any{}},
	}) {
		return false
	}
	item := map[string]any{
		"type": "message", "id": state.messageID, "status": "completed", "role": "assistant",
		"content": []map[string]any{{"type": "output_text", "text": text, "annotations": []any{}}},
	}
	if !writeSSE(writer, "response.output_item.done", map[string]any{
		"type": "response.output_item.done", "sequence_number": state.next(),
		"output_index": state.outputIdx, "item": item,
	}) {
		return false
	}
	state.outputItems = append(state.outputItems, item)
	state.messageOpen = false
	state.outputIdx++
	return true
}

func (state *streamState) emitToolCalls(writer http.ResponseWriter, toolCalls []chatStreamTool) bool {
	for _, toolCall := range toolCalls {
		if toolCall.Index > state.toolIdx {
			// 新的 function_call item：先收尾文本 message 与上一个工具
			if !state.closeMessage(writer) {
				return false
			}
			if !state.closeTool(writer) {
				return false
			}
			state.toolIdx = toolCall.Index
			itemID, err := NewFunctionCallID()
			if err != nil {
				return false
			}
			callID := toolCall.ID
			if callID == "" {
				if callID, err = NewCallID(); err != nil {
					return false
				}
			}
			state.toolItemID = itemID
			state.toolCallID = callID
			state.toolName = toolCall.Function.Name
			state.toolArgs.Reset()
			if !writeSSE(writer, "response.output_item.added", map[string]any{
				"type": "response.output_item.added", "sequence_number": state.next(),
				"output_index": state.outputIdx,
				"item": map[string]any{
					"type": "function_call", "id": itemID, "status": "in_progress",
					"call_id": callID, "name": state.toolName, "arguments": "",
				},
			}) {
				return false
			}
		}
		if toolCall.Function.Arguments != "" {
			state.toolArgs.WriteString(toolCall.Function.Arguments)
			if !writeSSE(writer, "response.function_call_arguments.delta", map[string]any{
				"type": "response.function_call_arguments.delta", "sequence_number": state.next(),
				"item_id": state.toolItemID, "output_index": state.outputIdx,
				"delta": toolCall.Function.Arguments,
			}) {
				return false
			}
		}
	}
	return true
}

func (state *streamState) closeTool(writer http.ResponseWriter) bool {
	if state.toolItemID == "" {
		return true
	}
	args := state.toolArgs.String()
	if !writeSSE(writer, "response.function_call_arguments.done", map[string]any{
		"type": "response.function_call_arguments.done", "sequence_number": state.next(),
		"item_id": state.toolItemID, "output_index": state.outputIdx, "arguments": args,
	}) {
		return false
	}
	item := map[string]any{
		"type": "function_call", "id": state.toolItemID, "status": "completed",
		"call_id": state.toolCallID, "name": state.toolName, "arguments": args,
	}
	if !writeSSE(writer, "response.output_item.done", map[string]any{
		"type": "response.output_item.done", "sequence_number": state.next(),
		"output_index": state.outputIdx, "item": item,
	}) {
		return false
	}
	state.outputItems = append(state.outputItems, item)
	state.toolItemID = ""
	state.outputIdx++
	return true
}

func (state *streamState) finishIfNeeded(writer http.ResponseWriter) {
	if !state.finished {
		_ = state.finish(writer)
	}
}

func (state *streamState) finish(writer http.ResponseWriter) bool {
	if state.finished {
		return true
	}
	if !state.closeMessage(writer) {
		return false
	}
	if !state.closeTool(writer) {
		return false
	}
	state.finished = true
	return writeSSE(writer, "response.completed", map[string]any{
		"type": "response.completed", "sequence_number": state.next(),
		"response": state.responseSnapshot("completed"),
	})
}

func writeStreamHeaders(writer http.ResponseWriter) bool {
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache, no-transform")
	writer.Header().Set("Connection", "keep-alive")
	writer.Header().Set("X-Accel-Buffering", "no")
	writer.WriteHeader(http.StatusOK)
	return http.NewResponseController(writer).Flush() == nil
}

func writeSSE(writer http.ResponseWriter, event string, data any) bool {
	payload, err := json.Marshal(data)
	if err != nil {
		return false
	}
	if _, err := writer.Write([]byte("event: " + event + "\ndata: ")); err != nil {
		return false
	}
	if _, err := writer.Write(payload); err != nil {
		return false
	}
	if _, err := writer.Write([]byte("\n\n")); err != nil {
		return false
	}
	return http.NewResponseController(writer).Flush() == nil
}

func writeError(writer http.ResponseWriter, status int, code, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"error": map[string]string{"type": code, "message": message},
	})
}

func readLine(buffer *bytes.Buffer) (string, bool) {
	data := buffer.Bytes()
	index := bytes.IndexByte(data, '\n')
	if index < 0 {
		return "", false
	}
	line := string(data[:index])
	buffer.Next(index + 1)
	return strings.TrimRight(line, "\r"), true
}
