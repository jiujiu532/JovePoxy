package httpserver

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net/http"

	"jovepoxy/internal/sse"
	"jovepoxy/internal/usageparse"
)

// copyStream tees the upstream SSE body to the client while side-scanning usage.
// Scan failures never interrupt a successful stream; zero usage is returned.
func (server server) copyStream(writer http.ResponseWriter, body io.Reader) usageparse.UsageSnapshot {
	var snap usageparse.UsageSnapshot
	// First-event probe without idle wrapper so early JSON errors do not leave a
	// background reader holding the upstream body.
	reader := bufio.NewReader(body)
	firstEvent, err := sse.ReadFirstEvent(reader)
	if len(firstEvent) == 0 && errors.Is(err, io.EOF) {
		writeOpenAIError(writer, http.StatusBadGateway, "upstream returned an empty response", "api_error", "", "upstream_error")
		return snap
	}
	if err != nil && !errors.Is(err, io.EOF) && len(firstEvent) == 0 {
		writeOpenAIError(writer, http.StatusBadGateway, "upstream response failed", "api_error", "", "upstream_error")
		return snap
	}
	if sse.IsRateLimitEvent(firstEvent) {
		writeOpenAIError(writer, http.StatusTooManyRequests, "upstream rate limit exceeded", "rate_limit_error", "", "rate_limit_exceeded")
		return snap
	}
	if msg, isErr := sse.ErrorEventMessage(firstEvent); isErr {
		if msg == "" {
			msg = "upstream error"
		}
		writeOpenAIError(writer, http.StatusBadGateway, msg, "api_error", "", "upstream_error")
		return snap
	}
	if !sse.WriteHeaders(writer) {
		return snap
	}
	usageparse.ScanSSEEvent(firstEvent, &snap)
	if !writeAndFlush(writer, firstEvent) {
		return snap
	}
	if errors.Is(err, io.EOF) {
		return snap
	}
	// Idle timeout only for the remainder of the upstream body.
	reader = bufio.NewReader(sse.IdleReader(reader, sse.DefaultIdleTimeout))
	var lineBuf bytes.Buffer
	buffer := make([]byte, 32*1024)
	for {
		count, readErr := reader.Read(buffer)
		if count > 0 {
			chunk := buffer[:count]
			// Side-scan complete lines only; keep a small carry for partial lines.
			lineBuf.Write(chunk)
			for {
				line, ok := sse.ReadLine(&lineBuf)
				if !ok {
					break
				}
				usageparse.ScanSSEDataLine([]byte(line), &snap)
			}
			if !writeAndFlush(writer, chunk) {
				return snap
			}
		}
		if errors.Is(readErr, io.EOF) {
			if remainder := bytes.TrimSpace(lineBuf.Bytes()); len(remainder) > 0 {
				usageparse.ScanSSEDataLine(remainder, &snap)
			}
			return snap
		}
		if readErr != nil {
			return snap
		}
	}
}

func writeAndFlush(writer http.ResponseWriter, chunk []byte) bool {
	if _, err := writer.Write(chunk); err != nil {
		return false
	}
	return http.NewResponseController(writer).Flush() == nil
}

// storeUsage writes usage counters into the request context for observe.Record.
func storeUsage(request *http.Request, snap usageparse.UsageSnapshot) {
	meta := requestMetaFrom(request.Context())
	applyUsageSnapshot(&meta, snap.PromptTokens, snap.CompletionTokens, snap.CacheReadTokens, snap.CacheCreationTokens)
	*request = *request.WithContext(withRequestMeta(request.Context(), meta))
}
