package responses

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"jovepoxy/internal/sse"
	"jovepoxy/internal/usageparse"
)

// WriteStream converts a chat.completion SSE body into Responses API SSE events.
// Returns last observed upstream usage (zero if absent); scan failures never break the stream.
func WriteStream(writer http.ResponseWriter, body io.Reader, model string) usageparse.UsageSnapshot {
	var snap usageparse.UsageSnapshot
	// First-event probe without idle wrapper so early JSON errors do not leave a
	// background reader holding the upstream body.
	reader := bufio.NewReader(body)
	firstEvent, err := sse.ReadFirstEvent(reader)
	if len(firstEvent) == 0 && errors.Is(err, io.EOF) {
		writeError(writer, http.StatusBadGateway, "upstream_error", "upstream returned an empty response")
		return snap
	}
	if err != nil && !errors.Is(err, io.EOF) && len(firstEvent) == 0 {
		writeError(writer, http.StatusBadGateway, "upstream_error", "upstream response failed")
		return snap
	}
	if sse.IsRateLimitEvent(firstEvent) {
		writeError(writer, http.StatusTooManyRequests, "rate_limit_error", "upstream rate limit exceeded")
		return snap
	}
	if msg, isErr := sse.ErrorEventMessage(firstEvent); isErr {
		if msg == "" {
			msg = "upstream error"
		}
		writeError(writer, http.StatusBadGateway, "upstream_error", msg)
		return snap
	}

	responseID, idErr := NewResponseID()
	if idErr != nil {
		writeError(writer, http.StatusInternalServerError, "api_error", "failed to allocate response id")
		return snap
	}
	if !sse.WriteHeaders(writer) {
		return snap
	}
	state := streamState{
		responseID: responseID,
		model:      model,
		createdAt:  time.Now().Unix(),
		tools:      make(map[int]*toolStreamState),
	}
	usageparse.ScanSSEEvent(firstEvent, &snap)
	if !state.emitCreated(writer) {
		return snap
	}
	if !state.consumeEvent(writer, firstEvent) {
		state.failIfNeeded(writer, "stream processing failed")
		return snap
	}
	if errors.Is(err, io.EOF) {
		state.finishIfNeeded(writer)
		return snap
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
				usageparse.ScanSSEDataLine([]byte(line), &snap)
				if !state.consumeLine(writer, line) {
					state.failIfNeeded(writer, "stream processing failed")
					return snap
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			if remainder := bytes.TrimSpace(buffer.Bytes()); len(remainder) > 0 {
				usageparse.ScanSSEDataLine(remainder, &snap)
				_ = state.consumeLine(writer, string(remainder))
			}
			state.finishIfNeeded(writer)
			return snap
		}
		if readErr != nil {
			// Mid-stream read failure: best-effort failed terminal if headers sent.
			state.failIfNeeded(writer, "upstream stream interrupted")
			return snap
		}
	}
}

// toolStreamState tracks one OpenAI tool_calls index through interleaved deltas.
type toolStreamState struct {
	itemID      string
	callID      string
	name        string
	args        strings.Builder
	outputIndex int
}

type streamState struct {
	responseID     string
	model          string
	createdAt      int64
	sequence       int
	outputIdx      int
	messageID      string
	messageOpen    bool
	textBuffer     strings.Builder
	tools          map[int]*toolStreamState // OpenAI tool index → open function_call
	toolsStarted   bool                     // once true, late reasoning is dropped
	reasoningOpen  bool
	reasoningID    string
	reasoningIndex int
	reasoningBuf   strings.Builder
	outputItems    []map[string]any
	finished       bool
	// lastFinishReason captures OpenAI finish_reason for completed vs incomplete.
	lastFinishReason string
}

func (state *streamState) next() int {
	state.sequence++
	return state.sequence
}

func (state *streamState) responseSnapshot(status string, extra map[string]any) map[string]any {
	output := state.outputItems
	if output == nil {
		output = []map[string]any{}
	}
	snap := map[string]any{
		"id": state.responseID, "object": "response", "created_at": state.createdAt,
		"status": status, "model": state.model, "output": output,
	}
	for k, v := range extra {
		snap[k] = v
	}
	return snap
}

func (state *streamState) emitCreated(writer http.ResponseWriter) bool {
	if !sse.WriteEvent(writer, "response.created", map[string]any{
		"type": "response.created", "sequence_number": state.next(),
		"response": state.responseSnapshot("in_progress", nil),
	}) {
		return false
	}
	return sse.WriteEvent(writer, "response.in_progress", map[string]any{
		"type": "response.in_progress", "sequence_number": state.next(),
		"response": state.responseSnapshot("in_progress", nil),
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
			Content          string           `json:"content"`
			ReasoningContent string           `json:"reasoning_content"`
			Reasoning        string           `json:"reasoning"`
			ToolCalls        []chatStreamTool `json:"tool_calls"`
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
	// Mid-stream OpenAI error: terminate with response.failed (not completed).
	if msg, isErr := sse.ErrorEventMessage([]byte("data: " + payload + "\n\n")); isErr {
		_ = state.fail(writer, msg)
		return false
	}
	var parsed chatStreamChunk
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil || len(parsed.Choices) == 0 {
		return true
	}
	choice := parsed.Choices[0]
	// reasoning 优先；切入 text/tool 前会 closeReasoning
	reasoning := choice.Delta.ReasoningContent
	if reasoning == "" {
		reasoning = choice.Delta.Reasoning
	}
	if reasoning != "" {
		if !state.emitReasoning(writer, reasoning) {
			return false
		}
	}
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
		state.lastFinishReason = choice.FinishReason
		return state.finish(writer)
	}
	return true
}

func (state *streamState) emitReasoning(writer http.ResponseWriter, delta string) bool {
	if delta == "" {
		return true
	}
	if !state.reasoningOpen {
		// Prefer reasoning before message/tool. Drop late reasoning once tools started
		// so output_index stays exclusive (matches anthropic stream policy).
		if state.toolsStarted {
			return true
		}
		// Rare interleave: text already streaming — close message first.
		if !state.closeMessage(writer) {
			return false
		}
		reasoningID, err := NewReasoningID()
		if err != nil {
			return false
		}
		state.reasoningID = reasoningID
		state.reasoningIndex = state.outputIdx
		state.reasoningOpen = true
		if !sse.WriteEvent(writer, "response.output_item.added", map[string]any{
			"type": "response.output_item.added", "sequence_number": state.next(),
			"output_index": state.reasoningIndex,
			"item": map[string]any{
				"id": reasoningID, "type": "reasoning", "status": "in_progress", "summary": []any{},
			},
		}) {
			return false
		}
		if !sse.WriteEvent(writer, "response.reasoning_summary_part.added", map[string]any{
			"type": "response.reasoning_summary_part.added", "sequence_number": state.next(),
			"item_id": reasoningID, "output_index": state.reasoningIndex, "summary_index": 0,
			"part": map[string]any{"type": "summary_text", "text": ""},
		}) {
			return false
		}
	}
	state.reasoningBuf.WriteString(delta)
	return sse.WriteEvent(writer, "response.reasoning_summary_text.delta", map[string]any{
		"type": "response.reasoning_summary_text.delta", "sequence_number": state.next(),
		"item_id": state.reasoningID, "output_index": state.reasoningIndex, "summary_index": 0,
		"delta": delta,
	})
}

func (state *streamState) closeReasoning(writer http.ResponseWriter) bool {
	if !state.reasoningOpen {
		return true
	}
	text := state.reasoningBuf.String()
	if !sse.WriteEvent(writer, "response.reasoning_summary_text.done", map[string]any{
		"type": "response.reasoning_summary_text.done", "sequence_number": state.next(),
		"item_id": state.reasoningID, "output_index": state.reasoningIndex, "summary_index": 0,
		"text": text,
	}) {
		return false
	}
	if !sse.WriteEvent(writer, "response.reasoning_summary_part.done", map[string]any{
		"type": "response.reasoning_summary_part.done", "sequence_number": state.next(),
		"item_id": state.reasoningID, "output_index": state.reasoningIndex, "summary_index": 0,
		"part": map[string]any{"type": "summary_text", "text": text},
	}) {
		return false
	}
	item := map[string]any{
		"id": state.reasoningID, "type": "reasoning", "status": "completed",
		"summary": []map[string]any{{"type": "summary_text", "text": text}},
	}
	if !sse.WriteEvent(writer, "response.output_item.done", map[string]any{
		"type": "response.output_item.done", "sequence_number": state.next(),
		"output_index": state.reasoningIndex, "item": item,
	}) {
		return false
	}
	state.outputItems = append(state.outputItems, item)
	state.reasoningOpen = false
	state.reasoningID = ""
	state.reasoningBuf.Reset()
	state.outputIdx++
	return true
}

func (state *streamState) emitText(writer http.ResponseWriter, content string) bool {
	if !state.closeReasoning(writer) {
		return false
	}
	// Prefer closing open tools before message so output_index stays exclusive.
	if !state.closeAllTools(writer) {
		return false
	}
	if !state.messageOpen {
		messageID, err := NewMessageID()
		if err != nil {
			return false
		}
		state.messageID = messageID
		state.messageOpen = true
		if !sse.WriteEvent(writer, "response.output_item.added", map[string]any{
			"type": "response.output_item.added", "sequence_number": state.next(),
			"output_index": state.outputIdx,
			"item": map[string]any{
				"type": "message", "id": messageID, "status": "in_progress",
				"role": "assistant", "content": []any{},
			},
		}) {
			return false
		}
		if !sse.WriteEvent(writer, "response.content_part.added", map[string]any{
			"type": "response.content_part.added", "sequence_number": state.next(),
			"item_id": messageID, "output_index": state.outputIdx, "content_index": 0,
			"part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}},
		}) {
			return false
		}
	}
	state.textBuffer.WriteString(content)
	return sse.WriteEvent(writer, "response.output_text.delta", map[string]any{
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
	if !sse.WriteEvent(writer, "response.output_text.done", map[string]any{
		"type": "response.output_text.done", "sequence_number": state.next(),
		"item_id": state.messageID, "output_index": state.outputIdx, "content_index": 0,
		"text": text,
	}) {
		return false
	}
	if !sse.WriteEvent(writer, "response.content_part.done", map[string]any{
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
	if !sse.WriteEvent(writer, "response.output_item.done", map[string]any{
		"type": "response.output_item.done", "sequence_number": state.next(),
		"output_index": state.outputIdx, "item": item,
	}) {
		return false
	}
	state.outputItems = append(state.outputItems, item)
	state.messageOpen = false
	state.messageID = ""
	state.textBuffer.Reset()
	state.outputIdx++
	return true
}

func (state *streamState) emitToolCalls(writer http.ResponseWriter, toolCalls []chatStreamTool) bool {
	for _, toolCall := range toolCalls {
		tool, known := state.tools[toolCall.Index]
		if !known {
			// 新的 function_call item：收尾 reasoning / message；已打开的其它 tool 保持 open 以支持交错
			if !state.closeReasoning(writer) {
				return false
			}
			if !state.closeMessage(writer) {
				return false
			}
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
			tool = &toolStreamState{
				itemID:      itemID,
				callID:      callID,
				name:        toolCall.Function.Name,
				outputIndex: state.outputIdx,
			}
			state.outputIdx++
			state.tools[toolCall.Index] = tool
			state.toolsStarted = true
			if !sse.WriteEvent(writer, "response.output_item.added", map[string]any{
				"type": "response.output_item.added", "sequence_number": state.next(),
				"output_index": tool.outputIndex,
				"item": map[string]any{
					"type": "function_call", "id": itemID, "status": "in_progress",
					"call_id": callID, "name": tool.name, "arguments": "",
				},
			}) {
				return false
			}
		} else {
			// 已知 index：可补全迟到的 id / name
			if toolCall.ID != "" && tool.callID == "" {
				tool.callID = toolCall.ID
			}
			if toolCall.Function.Name != "" && tool.name == "" {
				tool.name = toolCall.Function.Name
			}
		}
		if toolCall.Function.Arguments != "" {
			tool.args.WriteString(toolCall.Function.Arguments)
			if !sse.WriteEvent(writer, "response.function_call_arguments.delta", map[string]any{
				"type": "response.function_call_arguments.delta", "sequence_number": state.next(),
				"item_id": tool.itemID, "output_index": tool.outputIndex,
				"delta": toolCall.Function.Arguments,
			}) {
				return false
			}
		}
	}
	return true
}

func (state *streamState) closeTool(writer http.ResponseWriter, tool *toolStreamState) bool {
	if tool == nil || tool.itemID == "" {
		return true
	}
	args := tool.args.String()
	if !sse.WriteEvent(writer, "response.function_call_arguments.done", map[string]any{
		"type": "response.function_call_arguments.done", "sequence_number": state.next(),
		"item_id": tool.itemID, "output_index": tool.outputIndex, "arguments": args,
	}) {
		return false
	}
	item := map[string]any{
		"type": "function_call", "id": tool.itemID, "status": "completed",
		"call_id": tool.callID, "name": tool.name, "arguments": args,
	}
	if !sse.WriteEvent(writer, "response.output_item.done", map[string]any{
		"type": "response.output_item.done", "sequence_number": state.next(),
		"output_index": tool.outputIndex, "item": item,
	}) {
		return false
	}
	state.outputItems = append(state.outputItems, item)
	tool.itemID = ""
	return true
}

func (state *streamState) closeAllTools(writer http.ResponseWriter) bool {
	if len(state.tools) == 0 {
		return true
	}
	// Close by ascending output_index so completed output order is stable.
	indices := make([]int, 0, len(state.tools))
	for idx := range state.tools {
		indices = append(indices, idx)
	}
	sort.Slice(indices, func(i, j int) bool {
		return state.tools[indices[i]].outputIndex < state.tools[indices[j]].outputIndex
	})
	for _, idx := range indices {
		if !state.closeTool(writer, state.tools[idx]) {
			return false
		}
		delete(state.tools, idx)
	}
	return true
}

func (state *streamState) finishIfNeeded(writer http.ResponseWriter) {
	if !state.finished {
		_ = state.finish(writer)
	}
}

func (state *streamState) failIfNeeded(writer http.ResponseWriter, message string) {
	if !state.finished {
		_ = state.fail(writer, message)
	}
}

func (state *streamState) fail(writer http.ResponseWriter, message string) bool {
	if state.finished {
		return true
	}
	// Best-effort close open items, then emit failed (never fake completed).
	_ = state.closeReasoning(writer)
	_ = state.closeMessage(writer)
	_ = state.closeAllTools(writer)
	state.finished = true
	if message == "" {
		message = "upstream error"
	}
	return sse.WriteEvent(writer, "response.failed", map[string]any{
		"type": "response.failed", "sequence_number": state.next(),
		"response": state.responseSnapshot("failed", map[string]any{
			"error": map[string]string{"type": "api_error", "message": message},
		}),
	})
}

func (state *streamState) finish(writer http.ResponseWriter) bool {
	if state.finished {
		return true
	}
	if !state.closeReasoning(writer) {
		return false
	}
	if !state.closeMessage(writer) {
		return false
	}
	if !state.closeAllTools(writer) {
		return false
	}
	state.finished = true
	// length / content_filter truncation → incomplete rather than completed.
	if isIncompleteFinish(state.lastFinishReason) {
		return sse.WriteEvent(writer, "response.incomplete", map[string]any{
			"type": "response.incomplete", "sequence_number": state.next(),
			"response": state.responseSnapshot("incomplete", map[string]any{
				"incomplete_details": map[string]string{
					"reason": incompleteReason(state.lastFinishReason),
				},
			}),
		})
	}
	return sse.WriteEvent(writer, "response.completed", map[string]any{
		"type": "response.completed", "sequence_number": state.next(),
		"response": state.responseSnapshot("completed", nil),
	})
}

func isIncompleteFinish(finishReason string) bool {
	switch strings.ToLower(strings.TrimSpace(finishReason)) {
	case "length", "content_filter":
		return true
	default:
		return false
	}
}

func incompleteReason(finishReason string) string {
	switch strings.ToLower(strings.TrimSpace(finishReason)) {
	case "length":
		return "max_output_tokens"
	case "content_filter":
		return "content_filter"
	default:
		return finishReason
	}
}

func writeError(writer http.ResponseWriter, status int, code, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"error": map[string]string{"type": code, "message": message},
	})
}
