package httpserver

import (
	"bufio"
	"errors"
	"io"
	"net/http"

	"jovepoxy/internal/sse"
)

func (server server) copyStream(writer http.ResponseWriter, body io.Reader) {
	// First-event probe without idle wrapper so early JSON errors do not leave a
	// background reader holding the upstream body.
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
	if msg, isErr := sse.ErrorEventMessage(firstEvent); isErr {
		if msg == "" {
			msg = "upstream error"
		}
		writeOpenAIError(writer, http.StatusBadGateway, msg, "api_error", "", "upstream_error")
		return
	}
	if !sse.WriteHeaders(writer) {
		return
	}
	if !writeAndFlush(writer, firstEvent) {
		return
	}
	if errors.Is(err, io.EOF) {
		return
	}
	// Idle timeout only for the remainder of the upstream body.
	reader = bufio.NewReader(sse.IdleReader(reader, sse.DefaultIdleTimeout))
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
