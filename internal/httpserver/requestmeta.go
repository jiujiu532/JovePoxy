package httpserver

import "context"

type metaKey struct{}

type requestMeta struct {
	model  string
	keyID  string
	// upstream channel: opencode_free | opencode_paid | ollama_paid
	upstream string
	// Egress proxy for this request (secret-safe host/label only).
	// Free path may always set these; paid only when paid_use_proxy_pool is on
	// and the attempt used a pool proxy. Empty = direct.
	proxyID    string
	proxyLabel string
	proxyHost  string
	stream     bool
	errorClass string
	maxTokens  int
	reasoningEffort     string
	thinkingType        string
	budgetTokens        int
	inputTokens         int
	outputTokens        int
	cacheReadTokens     int
	cacheCreationTokens int
}

func withRequestMeta(ctx context.Context, meta requestMeta) context.Context {
	return context.WithValue(ctx, metaKey{}, meta)
}

func requestMetaFrom(ctx context.Context) requestMeta {
	meta, _ := ctx.Value(metaKey{}).(requestMeta)
	return meta
}

func applyUsageSnapshot(meta *requestMeta, prompt, completion, cacheRead, cacheCreation int) {
	if meta == nil {
		return
	}
	meta.inputTokens = prompt
	meta.outputTokens = completion
	meta.cacheReadTokens = cacheRead
	meta.cacheCreationTokens = cacheCreation
}
