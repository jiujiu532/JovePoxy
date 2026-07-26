package httpserver

import (
	"bufio"
	"errors"
	"io"
	"net/http"

	"jovepoxy/internal/sse"
)

func (server server) copyStream(writer http.ResponseWriter, body io.Reader) {
	reader := bufio.NewReader(body)
	firstEvent, err := sse.ReadFirstEvent(reader)
	if len(firstEvent) == 0 && errors.Is(err, io.EOF) {
		writeOpenAIError(writer, http.StatusBadGateway, "upstream returned an empty response", "api_error", "", "upstream_error")
		return
	}
	if err != nil && !errors.Is(err, io.EOF) && len(firstEvent) == 0 {
		writeOpenAIError(writer, http.StatusBadGateway, "upstream response failed", "api_error", "", "upstream_error")
		return
	}
	if sse.IsRateLimitEvent(firstEvent) {
		writeOpenAIError(writer, http.StatusTooManyRequests, "upstream rate limit exceeded", "rate_limit_error", "", "rate_limit_exceeded")
		return
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.WriteHeader(http.StatusOK)
	if !writeAndFlush(writer, firstEvent) {
		return
	}
	if errors.Is(err, io.EOF) {
		return
	}
	buffer := make([]byte, 32*1024)
	for {
		count, readErr := reader.Read(buffer)
		if count > 0 && !writeAndFlush(writer, buffer[:count]) {
			return
		}
		if errors.Is(readErr, io.EOF) {
			return
		}
		if readErr != nil {
			return
		}
	}
}

func writeAndFlush(writer http.ResponseWriter, chunk []byte) bool {
	if _, err := writer.Write(chunk); err != nil {
		return false
	}
	return http.NewResponseController(writer).Flush() == nil
}
