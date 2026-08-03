package usageparse_test

import (
	"testing"

	"jovepoxy/internal/usageparse"
)

func TestParseOpenAIUsage_basic(t *testing.T) {
	body := []byte(`{"id":"x","choices":[],"usage":{"prompt_tokens":12,"completion_tokens":4}}`)
	snap := usageparse.ParseOpenAIUsage(body)
	if snap.PromptTokens != 12 || snap.CompletionTokens != 4 {
		t.Fatalf("snap = %+v", snap)
	}
	if snap.CacheReadTokens != 0 || snap.CacheCreationTokens != 0 {
		t.Fatalf("cache should be 0: %+v", snap)
	}
}

func TestParseOpenAIUsage_cachedTokens(t *testing.T) {
	body := []byte(`{
		"usage":{
			"prompt_tokens":100,
			"completion_tokens":20,
			"prompt_tokens_details":{"cached_tokens":40,"cache_write_tokens":5}
		}
	}`)
	snap := usageparse.ParseOpenAIUsage(body)
	if snap.PromptTokens != 100 || snap.CompletionTokens != 20 {
		t.Fatalf("tokens = %+v", snap)
	}
	if snap.CacheReadTokens != 40 || snap.CacheCreationTokens != 5 {
		t.Fatalf("cache = %+v", snap)
	}
}

func TestParseOpenAIUsage_inputTokensDetails(t *testing.T) {
	body := []byte(`{"usage":{"prompt_tokens":50,"completion_tokens":3,"input_tokens_details":{"cached_tokens":15}}}`)
	snap := usageparse.ParseOpenAIUsage(body)
	if snap.CacheReadTokens != 15 {
		t.Fatalf("cache read = %d", snap.CacheReadTokens)
	}
}

func TestParseOpenAIUsage_topLevelCacheFields(t *testing.T) {
	body := []byte(`{"usage":{"prompt_tokens":10,"completion_tokens":2,"cache_read_input_tokens":7,"cache_creation_input_tokens":1}}`)
	snap := usageparse.ParseOpenAIUsage(body)
	if snap.CacheReadTokens != 7 || snap.CacheCreationTokens != 1 {
		t.Fatalf("snap = %+v", snap)
	}
}

func TestParseOpenAIUsage_deepseekPromptCacheHit(t *testing.T) {
	body := []byte(`{
		"usage":{
			"prompt_tokens":120,
			"completion_tokens":8,
			"prompt_cache_hit_tokens":90,
			"prompt_cache_miss_tokens":30
		}
	}`)
	snap := usageparse.ParseOpenAIUsage(body)
	if snap.PromptTokens != 120 || snap.CompletionTokens != 8 {
		t.Fatalf("tokens = %+v", snap)
	}
	if snap.CacheReadTokens != 90 {
		t.Fatalf("cache read from prompt_cache_hit_tokens = %d", snap.CacheReadTokens)
	}
}

func TestParseOpenAIUsage_missingOrMalformed(t *testing.T) {
	if !usageparse.ParseOpenAIUsage(nil).IsZero() {
		t.Fatal("nil body")
	}
	if !usageparse.ParseOpenAIUsage([]byte(`{"choices":[]}`)).IsZero() {
		t.Fatal("no usage")
	}
	if !usageparse.ParseOpenAIUsage([]byte(`not-json`)).IsZero() {
		t.Fatal("malformed")
	}
	if !usageparse.ParseOpenAIUsage([]byte(`{"usage":null}`)).IsZero() {
		t.Fatal("null usage")
	}
}

func TestScanSSEDataLine_lastUsageWins(t *testing.T) {
	var snap usageparse.UsageSnapshot
	usageparse.ScanSSEDataLine([]byte(`data: {"choices":[{"delta":{"content":"a"}}]}`), &snap)
	if !snap.IsZero() {
		t.Fatalf("mid chunk should not set usage: %+v", snap)
	}
	usageparse.ScanSSEDataLine([]byte(`data: {"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":9,"completion_tokens":3,"prompt_tokens_details":{"cached_tokens":2}}}`), &snap)
	if snap.PromptTokens != 9 || snap.CompletionTokens != 3 || snap.CacheReadTokens != 2 {
		t.Fatalf("snap = %+v", snap)
	}
	// later frame with only usage
	usageparse.ScanSSEDataLine([]byte(`data: {"usage":{"prompt_tokens":11,"completion_tokens":5}}`), &snap)
	if snap.PromptTokens != 11 || snap.CompletionTokens != 5 {
		t.Fatalf("last usage = %+v", snap)
	}
	usageparse.ScanSSEDataLine([]byte(`data: [DONE]`), &snap)
	if snap.PromptTokens != 11 {
		t.Fatalf("DONE should not clear: %+v", snap)
	}
}

func TestScanSSEEvent(t *testing.T) {
	event := []byte("data: {\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2}}\n\n")
	var snap usageparse.UsageSnapshot
	usageparse.ScanSSEEvent(event, &snap)
	if snap.PromptTokens != 1 || snap.CompletionTokens != 2 {
		t.Fatalf("snap = %+v", snap)
	}
}
