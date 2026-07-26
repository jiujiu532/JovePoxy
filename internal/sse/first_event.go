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
// or error envelope that should map to HTTP 429 before streaming begins.
func IsRateLimitEvent(event []byte) bool {
	for _, line := range bytes.Split(event, []byte("\n")) {
		line = bytes.TrimSpace(bytes.TrimSuffix(line, []byte("\r")))
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if isRateLimitPayload(payload) {
			return true
		}
	}
	return isRateLimitPayload(bytes.TrimSpace(event))
}

func isEventBoundary(line []byte) bool {
	trimmed := bytes.TrimRight(line, "\r\n")
	return len(line) > 0 && len(trimmed) == 0
}

func isRateLimitPayload(payload []byte) bool {
	if len(payload) == 0 {
		return false
	}
	var envelope struct {
		Type  string          `json:"type"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return false
	}
	return strings.EqualFold(envelope.Type, "FreeUsageLimitError") || len(envelope.Error) > 0
}
