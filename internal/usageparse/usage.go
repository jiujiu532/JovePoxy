// Package usageparse extracts token/cache usage from OpenAI-shaped JSON and SSE.
// It never retains prompt/response bodies — only integer counters.
package usageparse

import (
	"bytes"
	"encoding/json"
)

// UsageSnapshot is a privacy-safe token/cache summary from an upstream response.
type UsageSnapshot struct {
	PromptTokens        int // input / prompt_tokens
	CompletionTokens    int // output / completion_tokens
	CacheReadTokens     int // cached_tokens / cache_read_input_tokens
	CacheCreationTokens int // cache write / cache_creation_input_tokens
}

// IsZero reports whether all counters are unset.
func (s UsageSnapshot) IsZero() bool {
	return s.PromptTokens == 0 && s.CompletionTokens == 0 && s.CacheReadTokens == 0 && s.CacheCreationTokens == 0
}

// ParseOpenAIUsage extracts usage from a non-stream chat.completion (or similar) JSON body.
// Missing or malformed usage yields a zero snapshot; failures never panic.
func ParseOpenAIUsage(body []byte) UsageSnapshot {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return UsageSnapshot{}
	}
	var envelope struct {
		Usage json.RawMessage `json:"usage"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return UsageSnapshot{}
	}
	raw := bytes.TrimSpace(envelope.Usage)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return UsageSnapshot{}
	}
	return parseUsageObject(raw)
}

// ScanSSEDataLine updates snap when a single SSE line carries a usage object.
// Only the last non-zero snapshot is kept (typical final-chunk usage).
func ScanSSEDataLine(line []byte, snap *UsageSnapshot) {
	if snap == nil {
		return
	}
	line = bytes.TrimSpace(bytes.TrimSuffix(line, []byte("\r")))
	if !bytes.HasPrefix(line, []byte("data:")) {
		return
	}
	payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		return
	}
	// Chunk itself may be the usage-bearing object (has top-level "usage").
	parsed := ParseOpenAIUsage(payload)
	if !parsed.IsZero() {
		*snap = parsed
	}
}

// ScanSSEEvent scans every data: line inside a multi-line first SSE event.
func ScanSSEEvent(event []byte, snap *UsageSnapshot) {
	if snap == nil || len(event) == 0 {
		return
	}
	for _, line := range bytes.Split(event, []byte("\n")) {
		ScanSSEDataLine(line, snap)
	}
}

func parseUsageObject(raw []byte) UsageSnapshot {
	var u struct {
		PromptTokens             int `json:"prompt_tokens"`
		CompletionTokens         int `json:"completion_tokens"`
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		// DeepSeek OpenAI-compat cache fields (top-level usage).
		PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
		PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
		PromptTokensDetails   *struct {
			CachedTokens      int `json:"cached_tokens"`
			CacheWriteTokens  int `json:"cache_write_tokens"`
			CachedTokensWrite int `json:"cached_tokens_write"`
		} `json:"prompt_tokens_details"`
		InputTokensDetails *struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"input_tokens_details"`
	}
	if err := json.Unmarshal(raw, &u); err != nil {
		return UsageSnapshot{}
	}
	snap := UsageSnapshot{
		PromptTokens:        u.PromptTokens,
		CompletionTokens:    u.CompletionTokens,
		CacheReadTokens:     u.CacheReadInputTokens,
		CacheCreationTokens: u.CacheCreationInputTokens,
	}
	// Anthropic / Responses-style aliases when OpenAI fields are absent.
	if snap.PromptTokens == 0 && u.InputTokens > 0 {
		snap.PromptTokens = u.InputTokens
	}
	if snap.CompletionTokens == 0 && u.OutputTokens > 0 {
		snap.CompletionTokens = u.OutputTokens
	}
	// Cache read priority: top-level Anthropic → OpenAI details → DeepSeek hit.
	if snap.CacheReadTokens == 0 {
		if u.PromptTokensDetails != nil && u.PromptTokensDetails.CachedTokens > 0 {
			snap.CacheReadTokens = u.PromptTokensDetails.CachedTokens
		} else if u.InputTokensDetails != nil && u.InputTokensDetails.CachedTokens > 0 {
			snap.CacheReadTokens = u.InputTokensDetails.CachedTokens
		} else if u.PromptCacheHitTokens > 0 {
			snap.CacheReadTokens = u.PromptCacheHitTokens
		}
	}
	if snap.CacheCreationTokens == 0 && u.PromptTokensDetails != nil {
		if u.PromptTokensDetails.CacheWriteTokens > 0 {
			snap.CacheCreationTokens = u.PromptTokensDetails.CacheWriteTokens
		} else if u.PromptTokensDetails.CachedTokensWrite > 0 {
			snap.CacheCreationTokens = u.PromptTokensDetails.CachedTokensWrite
		}
	}
	// DeepSeek reports miss tokens; when hit is known and prompt total is known,
	// miss is informational only — do not invent cache creation from miss.
	_ = u.PromptCacheMissTokens
	return snap
}
