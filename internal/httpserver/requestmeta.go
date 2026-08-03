package httpserver

import "context"

type metaKey struct{}

type requestMeta struct {
	model           string
	keyID           string
	stream          bool
	errorClass      string
	maxTokens       int
	reasoningEffort string
	thinkingType    string
	budgetTokens    int
}

func withRequestMeta(ctx context.Context, meta requestMeta) context.Context {
	return context.WithValue(ctx, metaKey{}, meta)
}

func requestMetaFrom(ctx context.Context) requestMeta {
	meta, _ := ctx.Value(metaKey{}).(requestMeta)
	return meta
}
