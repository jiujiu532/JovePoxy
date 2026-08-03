package httpserver

import "context"

type metaKey struct{}

type requestMeta struct {
	model               string
	keyID               string
	stream              bool
	errorClass          string
	maxTokens           int
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
