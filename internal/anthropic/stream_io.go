package anthropic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"jovepoxy/internal/sse"
)

func writeStreamHeaders(writer http.ResponseWriter) bool {
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache, no-transform")
	writer.Header().Set("Connection", "keep-alive")
	writer.Header().Set("X-Accel-Buffering", "no")
	writer.WriteHeader(http.StatusOK)
	return http.NewResponseController(writer).Flush() == nil
}

func writeSSE(writer http.ResponseWriter, event string, data any) bool {
	payload, err := json.Marshal(data)
	if err != nil {
		return false
	}
	if _, err := writer.Write([]byte("event: " + event + "\ndata: ")); err != nil {
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

func writeError(writer http.ResponseWriter, status int, errorType, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"type": "error", "error": map[string]string{"type": errorType, "message": message},
	})
}

func readFirstSSEEvent(reader *bufio.Reader) ([]byte, error) {
	return sse.ReadFirstEvent(reader)
}

func isRateLimitEvent(event []byte) bool {
	return sse.IsRateLimitEvent(event)
}

func readLine(buffer *bytes.Buffer) (string, bool) {
	data := buffer.Bytes()
	index := bytes.IndexByte(data, '\n')
	if index < 0 {
		return "", false
	}
	line := string(data[:index])
	buffer.Next(index + 1)
	return strings.TrimRight(line, "\r"), true
}
