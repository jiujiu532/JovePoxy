package sse

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
)

// WriteHeaders commits standard SSE response headers and flushes once so the
// client sees the stream open immediately.
func WriteHeaders(writer http.ResponseWriter) bool {
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache, no-transform")
	writer.Header().Set("Connection", "keep-alive")
	writer.Header().Set("X-Accel-Buffering", "no")
	writer.WriteHeader(http.StatusOK)
	return http.NewResponseController(writer).Flush() == nil
}

// WriteEvent marshals data as JSON and writes one SSE event frame:
//
//	event: <eventType>
//	data: <json>
//
// followed by a blank line and a flush.
func WriteEvent(writer http.ResponseWriter, eventType string, data any) bool {
	payload, err := json.Marshal(data)
	if err != nil {
		return false
	}
	if _, err := writer.Write([]byte("event: " + eventType + "\ndata: ")); err != nil {
		return false
	}
	if _, err := writer.Write(payload); err != nil {
		return false
	}
	if _, err := writer.Write([]byte("\n\n")); err != nil {
		return false
	}
	return http.NewResponseController(writer).Flush() == nil
}

// ReadLine pulls the next complete line (without trailing \r/\n) from buffer.
// Returns ok=false when no newline is available yet.
func ReadLine(buffer *bytes.Buffer) (string, bool) {
	data := buffer.Bytes()
	index := bytes.IndexByte(data, '\n')
	if index < 0 {
		return "", false
	}
	line := string(data[:index])
	buffer.Next(index + 1)
	return strings.TrimRight(line, "\r"), true
}
