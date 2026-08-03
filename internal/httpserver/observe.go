package httpserver

import (
	"encoding/json"
	"net/http"
	"time"

	"jovepoxy/internal/reqlog"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *statusRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
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
		recorder := &statusRecorder{ResponseWriter: writer, status: http.StatusOK}
		handler(recorder, request)
		meta := requestMetaFrom(request.Context())
		errorClass := meta.errorClass
		if errorClass == "" && recorder.status >= 400 {
			errorClass = http.StatusText(recorder.status)
		}
		server.logs.Record(request.Context(), reqlog.Entry{
			KeyID: meta.keyID, Model: meta.model, Route: route, Status: recorder.status,
			LatencyMS: time.Since(started).Milliseconds(), Stream: meta.stream, ErrorClass: errorClass,
			MaxTokens: meta.maxTokens, ReasoningEffort: meta.reasoningEffort,
			ThinkingType: meta.thinkingType, BudgetTokens: meta.budgetTokens,
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
