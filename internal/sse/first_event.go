// Package sse provides shared Server-Sent Event stream helpers for data-plane adapters.
package sse

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
)

// ReadFirstEvent reads through the first blank line so the complete first SSE
// event is available before response headers are committed.
func ReadFirstEvent(reader *bufio.Reader) ([]byte, error) {
	var event bytes.Buffer
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			_, _ = event.Write(line)
		}
		if isEventBoundary(line) {
			if err != nil {
				return event.Bytes(), err
			}
			return event.Bytes(), nil
		}
		if err != nil {
			return event.Bytes(), err
		}
	}
}

// IsRateLimitEvent reports whether the first SSE event is an upstream free-usage
// or explicit rate-limit envelope that should map to HTTP 429 before streaming begins.
func IsRateLimitEvent(event []byte) bool {
	for _, line := range bytes.Split(event, []byte("\n")) {
		line = bytes.TrimSpace(bytes.TrimSuffix(line, []byte("\r")))
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if IsRateLimitPayload(payload) {
			return true
		}
	}
	return IsRateLimitPayload(bytes.TrimSpace(event))
}

// IsRateLimitPayload reports whether a JSON body (or SSE data payload) is a free-usage
// or explicit rate-limit error. Generic server/invalid-request errors are not treated as 429.
func IsRateLimitPayload(payload []byte) bool {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 || bytes.Equal(payload, []byte("null")) {
		return false
	}
	var envelope struct {
		Type  string          `json:"type"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(envelope.Type), "FreeUsageLimitError") {
		return true
	}
	errorPayload := bytes.TrimSpace(envelope.Error)
	if len(errorPayload) == 0 || bytes.Equal(errorPayload, []byte("null")) {
		return false
	}
	var detail struct {
		Type string `json:"type"`
		Code string `json:"code"`
	}
	if err := json.Unmarshal(errorPayload, &detail); err != nil {
		// Non-object error values are not treated as rate limits.
		return false
	}
	return isRateLimitToken(detail.Type) || isRateLimitToken(detail.Code)
}

func isEventBoundary(line []byte) bool {
	trimmed := bytes.TrimRight(line, "\r\n")
	return len(line) > 0 && len(trimmed) == 0
}

func isRateLimitToken(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false
	}
	if value == "freeusagelimiterror" || value == "insufficient_quota" {
		return true
	}
	return strings.Contains(value, "rate_limit")
}
