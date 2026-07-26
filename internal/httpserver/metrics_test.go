package httpserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jovepoxy/internal/zen"
)

func TestServer_records_request_log_and_metrics(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer upstream.Close()
	server := newServer(t, upstream.URL, []zen.Model{{ID: "demo-free"}})
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"demo-free","messages":[{"role":"user","content":"hi"}]}`))
	request.Header.Set("Authorization", "Bearer "+server.key)
	recorder := httptest.NewRecorder()

	// When
	server.handler.ServeHTTP(recorder, request)
	metricsRecorder := httptest.NewRecorder()
	server.handler.ServeHTTP(metricsRecorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	// Then
	if recorder.Code != http.StatusOK {
		t.Fatalf("chat status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if metricsRecorder.Code != http.StatusOK {
		t.Fatalf("metrics status = %d body=%s", metricsRecorder.Code, metricsRecorder.Body.String())
	}
	var snapshot struct {
		TotalRequests uint64 `json:"total_requests"`
		Status2xx     uint64 `json:"status_2xx"`
	}
	if err := json.Unmarshal(metricsRecorder.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("decode metrics: %v", err)
	}
	if snapshot.TotalRequests < 1 || snapshot.Status2xx < 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}
