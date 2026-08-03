// Package responses 实现 OpenAI Responses API 与上游 Chat Completions 的双向转换。
// 请求：/v1/responses 形状 → chat.completions 形状（复用 free/paid 转发链路）。
// 响应：chat.completion(.chunk) → Responses 对象 / Responses SSE 事件流。
package responses

import "jovepoxy/internal/idgen"

func newID(prefix string) (string, error) {
	return idgen.Prefixed(prefix, 12)
}

// NewResponseID returns a Responses API response id (resp_...).
func NewResponseID() (string, error) { return newID("resp_") }

// NewMessageID returns an output message item id (msg_...).
func NewMessageID() (string, error) { return newID("msg_") }

// NewFunctionCallID returns a function_call item id (fc_...).
func NewFunctionCallID() (string, error) { return newID("fc_") }

// NewCallID returns a tool call_id (call_...).
func NewCallID() (string, error) { return newID("call_") }

// NewReasoningID returns a reasoning output item id (rs_...).
func NewReasoningID() (string, error) { return newID("rs_") }
