package anthropic

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"jovepoxy/internal/effort"
)

// Request is a parsed Anthropic Messages API request.
type Request struct {
	Model         string
	Stream        bool
	MaxTokens     int
	Messages      []json.RawMessage
	System        json.RawMessage
	Tools         []json.RawMessage
	Temperature   *float64
	TopP          *float64
	StopSequences []string
	ToolChoice    json.RawMessage
	Thinking      *thinkingConfig
	// OutputConfigEffort comes from output_config.effort (adaptive thinking).
	OutputConfigEffort string
}

type thinkingConfig struct {
	Type         string
	BudgetTokens int
	// HasBudget is true when budget_tokens was explicitly present in JSON.
	HasBudget bool
}

type openAIChatRequest struct {
	Model           string           `json:"model"`
	Messages        []map[string]any `json:"messages"`
	Stream          bool             `json:"stream"`
	MaxTokens       int              `json:"max_tokens,omitempty"`
	Tools           []map[string]any `json:"tools,omitempty"`
	ReasoningEffort string           `json:"reasoning_effort,omitempty"`
	Temperature     *float64         `json:"temperature,omitempty"`
	TopP            *float64         `json:"top_p,omitempty"`
	Stop            any              `json:"stop,omitempty"`
	ToolChoice      json.RawMessage  `json:"tool_choice,omitempty"`
}

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	// Thinking is the Anthropic official body for type=thinking blocks
	// (string, or rarely a nested object with text/thinking).
	Thinking  json.RawMessage `json:"thinking,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ParseRequest decodes and lightly validates an Anthropic Messages body.
func ParseRequest(body []byte) (Request, error) {
	var raw struct {
		Model         string            `json:"model"`
		Stream        bool              `json:"stream"`
		MaxTokens     int               `json:"max_tokens"`
		System        json.RawMessage   `json:"system"`
		Messages      []json.RawMessage `json:"messages"`
		Tools         []json.RawMessage `json:"tools"`
		Temperature   *float64          `json:"temperature"`
		TopP          *float64          `json:"top_p"`
		StopSequences json.RawMessage   `json:"stop_sequences"`
		ToolChoice    json.RawMessage   `json:"tool_choice"`
		Thinking      json.RawMessage   `json:"thinking"`
		OutputConfig  json.RawMessage   `json:"output_config"`
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
	if len(raw.Messages) == 0 {
		return Request{}, fmt.Errorf("messages is required")
	}

	req := Request{
		Model: raw.Model, Stream: raw.Stream, MaxTokens: raw.MaxTokens,
		Messages: raw.Messages, System: raw.System, Tools: raw.Tools,
		Temperature: raw.Temperature, TopP: raw.TopP,
		StopSequences: parseStopSequences(raw.StopSequences),
		ToolChoice:    raw.ToolChoice,
	}
	req.Thinking = parseThinking(raw.Thinking)
	req.OutputConfigEffort = parseOutputConfigEffort(raw.OutputConfig)
	return req, nil
}

// parseStopSequences maps Anthropic stop_sequences. Non-array values are ignored
// (do not fail the whole request).
func parseStopSequences(raw json.RawMessage) []string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	if trimmed[0] != '[' {
		return nil
	}
	var seqs []string
	if err := json.Unmarshal(trimmed, &seqs); err != nil {
		return nil
	}
	return seqs
}

func parseThinking(raw json.RawMessage) *thinkingConfig {
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return nil
	}
	// Use a map to detect whether budget_tokens was present.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil
	}
	var cfg struct {
		Type         string `json:"type"`
		BudgetTokens int    `json:"budget_tokens"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil
	}
	if strings.TrimSpace(cfg.Type) == "" {
		return nil
	}
	_, hasBudget := probe["budget_tokens"]
	return &thinkingConfig{
		Type:         strings.ToLower(strings.TrimSpace(cfg.Type)),
		BudgetTokens: cfg.BudgetTokens,
		HasBudget:    hasBudget,
	}
}

func parseOutputConfigEffort(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return ""
	}
	var cfg struct {
		Effort string `json:"effort"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return ""
	}
	return cfg.Effort
}

// ObservabilityMeta is secret-free generation params for request logging.
type ObservabilityMeta struct {
	MaxTokens       int
	ReasoningEffort string
	ThinkingType    string
	BudgetTokens    int
}

// Observability returns request-side generation metadata (no prompt/response bodies).
func (request Request) Observability() ObservabilityMeta {
	meta := ObservabilityMeta{
		MaxTokens:       request.MaxTokens,
		ReasoningEffort: mapReasoningEffort(request),
	}
	if request.Thinking != nil {
		meta.ThinkingType = request.Thinking.Type
		if request.Thinking.HasBudget {
			meta.BudgetTokens = request.Thinking.BudgetTokens
		}
	}
	return meta
}

// ToOpenAIChat converts an Anthropic request into an OpenAI chat.completions body.
// The second return value is a rough input-token estimate matching the reference proxy.
func ToOpenAIChat(request Request) ([]byte, int, error) {
	messages, err := convertMessages(request)
	if err != nil {
		return nil, 0, err
	}
	tools, err := convertTools(request.Tools)
	if err != nil {
		return nil, 0, err
	}
	toolChoice, err := mapToolChoice(request.ToolChoice)
	if err != nil {
		return nil, 0, err
	}
	payload := openAIChatRequest{
		Model:           request.Model,
		Messages:        messages,
		Stream:          request.Stream,
		MaxTokens:       request.MaxTokens,
		Tools:           tools,
		ReasoningEffort: mapReasoningEffort(request),
		Temperature:     request.Temperature,
		TopP:            request.TopP,
		Stop:            mapStopSequences(request.StopSequences),
		ToolChoice:      toolChoice,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("encode openai request: %w", err)
	}
	return encoded, estimateTokens(encoded), nil
}

// mapReasoningEffort maps Anthropic thinking / output_config to OpenAI reasoning_effort.
// Returns empty string when the field should be omitted.
func mapReasoningEffort(request Request) string {
	if request.Thinking == nil {
		return ""
	}
	switch request.Thinking.Type {
	case "disabled":
		return "none"
	case "enabled":
		if request.Thinking.HasBudget && request.Thinking.BudgetTokens > 0 {
			return effort.BudgetToLevel(request.Thinking.BudgetTokens)
		}
		// enabled without budget (or budget 0) → auto
		return "auto"
	case "adaptive", "auto":
		if level := effort.NormalizeLevel(request.OutputConfigEffort); level != "" {
			return level
		}
		return "auto"
	default:
		// Unknown type: ignore thinking config, do not 400.
		return ""
	}
}

// mapStopSequences maps Anthropic stop_sequences to OpenAI stop.
// One element → string; multiple → []string; empty → omit (nil).
func mapStopSequences(seqs []string) any {
	switch len(seqs) {
	case 0:
		return nil
	case 1:
		return seqs[0]
	default:
		return seqs
	}
}

// mapToolChoice maps Anthropic tool_choice to OpenAI chat tool_choice JSON.
// Missing/empty → omit (nil).
func mapToolChoice(raw json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, nil
	}
	// String form (rare but legal JSON for some clients).
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("invalid tool_choice")
		}
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "auto", "any", "none":
			// Map "any" to "required" for consistency if string form is used.
			if strings.EqualFold(s, "any") {
				return json.RawMessage(`"required"`), nil
			}
			encoded, err := json.Marshal(strings.ToLower(strings.TrimSpace(s)))
			if err != nil {
				return nil, fmt.Errorf("invalid tool_choice")
			}
			return encoded, nil
		default:
			// Unknown string: pass through lowercased.
			encoded, err := json.Marshal(effort.NormalizeLevel(s))
			if err != nil {
				return nil, fmt.Errorf("invalid tool_choice")
			}
			return encoded, nil
		}
	}
	var choice struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &choice); err != nil {
		return nil, fmt.Errorf("invalid tool_choice")
	}
	switch strings.ToLower(strings.TrimSpace(choice.Type)) {
	case "auto":
		return json.RawMessage(`"auto"`), nil
	case "any":
		return json.RawMessage(`"required"`), nil
	case "none":
		return json.RawMessage(`"none"`), nil
	case "tool":
		name := strings.TrimSpace(choice.Name)
		if name == "" {
			return nil, fmt.Errorf("invalid tool_choice")
		}
		encoded, err := json.Marshal(map[string]any{
			"type":     "function",
			"function": map[string]any{"name": name},
		})
		if err != nil {
			return nil, fmt.Errorf("encode tool_choice: %w", err)
		}
		return encoded, nil
	case "":
		return nil, nil
	default:
		// Unknown object type: omit rather than 400 (align with unknown thinking type).
		return nil, nil
	}
}

func convertMessages(request Request) ([]map[string]any, error) {
	messages := make([]map[string]any, 0, len(request.Messages)+1)
	if system := systemText(request.System); system != "" {
		messages = append(messages, map[string]any{"role": "system", "content": system})
	}
	for _, raw := range request.Messages {
		var message anthropicMessage
		if err := json.Unmarshal(raw, &message); err != nil {
			return nil, fmt.Errorf("invalid message")
		}
		converted, err := convertMessage(message)
		if err != nil {
			return nil, err
		}
		messages = append(messages, converted...)
	}
	return messages, nil
}

func systemText(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func convertMessage(message anthropicMessage) ([]map[string]any, error) {
	trimmed := bytes.TrimSpace(message.Content)
	if len(trimmed) == 0 {
		return []map[string]any{{"role": message.Role, "content": ""}}, nil
	}
	if trimmed[0] == '"' {
		var text string
		if err := json.Unmarshal(message.Content, &text); err != nil {
			return nil, fmt.Errorf("invalid message content")
		}
		return []map[string]any{{"role": message.Role, "content": text}}, nil
	}
	var blocks []contentBlock
	if err := json.Unmarshal(message.Content, &blocks); err != nil {
		return nil, fmt.Errorf("invalid message content blocks")
	}
	text := joinTextBlocks(blocks)
	// Only assistant messages may carry historical thinking → reasoning_content.
	// user/system thinking blocks are ignored to prevent prompt injection.
	reasoning := ""
	if message.Role == "assistant" {
		reasoning = joinThinkingBlocks(blocks)
	}
	toolUses := filterBlocks(blocks, "tool_use")
	if len(toolUses) > 0 && message.Role == "assistant" {
		toolCalls := make([]map[string]any, 0, len(toolUses))
		for _, tool := range toolUses {
			args := "{}"
			if len(tool.Input) > 0 {
				// Keep raw JSON text for OpenAI arguments (including invalid payloads).
				args = string(tool.Input)
			}
			toolID := tool.ID
			if toolID == "" {
				var idErr error
				toolID, idErr = NewToolUseID()
				if idErr != nil {
					return nil, idErr
				}
			}
			toolCalls = append(toolCalls, map[string]any{
				"id": toolID, "type": "function",
				"function": map[string]any{"name": tool.Name, "arguments": args},
			})
		}
		var content any
		if text != "" {
			content = text
		}
		msg := map[string]any{"role": "assistant", "content": content, "tool_calls": toolCalls}
		if reasoning != "" {
			msg["reasoning_content"] = reasoning
		}
		return []map[string]any{msg}, nil
	}
	if hasBlockType(blocks, "tool_result") {
		results := make([]map[string]any, 0)
		for _, block := range filterBlocks(blocks, "tool_result") {
			callID := block.ToolUseID
			if callID == "" {
				var idErr error
				callID, idErr = NewToolUseID()
				if idErr != nil {
					return nil, idErr
				}
			}
			results = append(results, map[string]any{
				"role": "tool", "tool_call_id": callID, "content": toolResultText(block),
			})
		}
		// 同条 message 里 tool_result 之外的文本 → 追加 user 消息
		if text != "" {
			results = append(results, map[string]any{"role": "user", "content": text})
		}
		return results, nil
	}
	msg := map[string]any{"role": message.Role, "content": text}
	if reasoning != "" {
		msg["reasoning_content"] = reasoning
	}
	return []map[string]any{msg}, nil
}

func convertTools(rawTools []json.RawMessage) ([]map[string]any, error) {
	if len(rawTools) == 0 {
		return nil, nil
	}
	tools := make([]map[string]any, 0, len(rawTools))
	for _, raw := range rawTools {
		var tool anthropicTool
		if err := json.Unmarshal(raw, &tool); err != nil {
			return nil, fmt.Errorf("invalid tool definition")
		}
		schema := json.RawMessage(`{}`)
		if len(tool.InputSchema) > 0 {
			schema = tool.InputSchema
		}
		var parameters any
		if err := json.Unmarshal(schema, &parameters); err != nil {
			return nil, fmt.Errorf("invalid tool input_schema")
		}
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": tool.Name, "description": tool.Description, "parameters": parameters,
			},
		})
	}
	return tools, nil
}

func joinTextBlocks(blocks []contentBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Type == "text" && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// joinThinkingBlocks accumulates non-empty thinking block text.
// redacted_thinking is never mapped.
func joinThinkingBlocks(blocks []contentBlock) string {
	parts := make([]string, 0)
	for _, block := range blocks {
		if block.Type != "thinking" {
			continue
		}
		text := thinkingBlockText(block)
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n")
}

// thinkingBlockText extracts text from an Anthropic thinking content block.
// Official field is "thinking" (string or nested object); "text" is a fallback
// used by some clients / Gemini-style shapes (aligns with CLIProxy GetThinkingText).
func thinkingBlockText(block contentBlock) string {
	if t := extractThinkingPayload(block.Thinking); t != "" {
		return t
	}
	return strings.TrimSpace(block.Text)
}

func extractThinkingPayload(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return ""
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return ""
		}
		return strings.TrimSpace(s)
	}
	if trimmed[0] == '{' {
		var obj struct {
			Text     string `json:"text"`
			Thinking string `json:"thinking"`
		}
		if err := json.Unmarshal(trimmed, &obj); err != nil {
			return ""
		}
		if t := strings.TrimSpace(obj.Text); t != "" {
			return t
		}
		return strings.TrimSpace(obj.Thinking)
	}
	return ""
}

func filterBlocks(blocks []contentBlock, blockType string) []contentBlock {
	filtered := make([]contentBlock, 0)
	for _, block := range blocks {
		if block.Type == blockType {
			filtered = append(filtered, block)
		}
	}
	return filtered
}

func hasBlockType(blocks []contentBlock, blockType string) bool {
	for _, block := range blocks {
		if block.Type == blockType {
			return true
		}
	}
	return false
}

func toolResultText(block contentBlock) string {
	if len(bytes.TrimSpace(block.Content)) == 0 {
		return ""
	}
	var asString string
	if err := json.Unmarshal(block.Content, &asString); err == nil {
		return asString
	}
	var nested []contentBlock
	if err := json.Unmarshal(block.Content, &nested); err == nil {
		return joinTextBlocks(nested)
	}
	return string(block.Content)
}

func estimateTokens(body []byte) int {
	return len(body) / 4
}
