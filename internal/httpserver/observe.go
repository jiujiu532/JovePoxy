package httpserver

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"jovepoxy/internal/reqlog"
)

// statusRecorder captures status and time-to-first-body-byte for request logs.
// TTFT is measured from handler start (started) to the first non-empty Write.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	started time.Time

	mu     sync.Mutex
	ttftMS int64
	// wrote marks that at least one body byte was written (ttft set once).
	wrote bool
}

func (recorder *statusRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

func (recorder *statusRecorder) Write(p []byte) (int, error) {
	if len(p) > 0 {
		recorder.mu.Lock()
		if !recorder.wrote {
			recorder.wrote = true
			recorder.ttftMS = time.Since(recorder.started).Milliseconds()
			if recorder.ttftMS <= 0 {
				recorder.ttftMS = 1
			}
		}
		recorder.mu.Unlock()
	}
	return recorder.ResponseWriter.Write(p)
}

func (recorder *statusRecorder) firstByteMS() int64 {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return recorder.ttftMS
}

// Unwrap lets http.ResponseController reach the real ResponseWriter (for Flush).
func (recorder *statusRecorder) Unwrap() http.ResponseWriter {
	return recorder.ResponseWriter
}

func (server server) observe(route string, handler http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if server.logs == nil {
			handler(writer, request)
			return
		}
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: writer, status: http.StatusOK, started: started}
		handler(recorder, request)
		meta := requestMetaFrom(request.Context())
		errorClass := meta.errorClass
		if errorClass == "" && recorder.status >= 400 {
			errorClass = http.StatusText(recorder.status)
		}
		latencyMS := time.Since(started).Milliseconds()
		ttftMS := recorder.firstByteMS()
		if latencyMS <= 0 {
			latencyMS = 1
		}
		if ttftMS > latencyMS {
			latencyMS = ttftMS
		}
		server.logs.Record(request.Context(), reqlog.Entry{
			KeyID: meta.keyID, Model: meta.model, Route: route, Status: recorder.status,
			LatencyMS: latencyMS, TTFTMS: ttftMS,
			Stream: meta.stream, ErrorClass: errorClass,
			MaxTokens: meta.maxTokens, ReasoningEffort: meta.reasoningEffort,
			ThinkingType: meta.thinkingType, BudgetTokens: meta.budgetTokens,
			InputTokens: meta.inputTokens, OutputTokens: meta.outputTokens,
			CacheReadTokens: meta.cacheReadTokens, CacheCreationTokens: meta.cacheCreationTokens,
		})
	}
}

func (server server) metrics(writer http.ResponseWriter, _ *http.Request) {
	if server.logs == nil {
		writeOpenAIError(writer, http.StatusServiceUnavailable, "metrics unavailable", "api_error", "", "metrics_unavailable")
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(server.logs.Snapshot())
}
