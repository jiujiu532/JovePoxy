package sse_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jovepoxy/internal/sse"
)

func TestWriteHeaders(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	if !sse.WriteHeaders(recorder) {
		t.Fatal("WriteHeaders() = false, want true")
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-cache, no-transform" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := recorder.Header().Get("Connection"); got != "keep-alive" {
		t.Fatalf("Connection = %q", got)
	}
	if got := recorder.Header().Get("X-Accel-Buffering"); got != "no" {
		t.Fatalf("X-Accel-Buffering = %q", got)
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
}

func TestWriteEvent(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	if !sse.WriteHeaders(recorder) {
		t.Fatal("WriteHeaders failed")
	}
	payload := map[string]any{"type": "ping", "n": 1}
	if !sse.WriteEvent(recorder, "ping", payload) {
		t.Fatal("WriteEvent() = false, want true")
	}
	body := recorder.Body.String()
	if !strings.HasPrefix(body, "event: ping\ndata: ") {
		t.Fatalf("body prefix = %q", body)
	}
	if !strings.HasSuffix(body, "\n\n") {
		t.Fatalf("body should end with blank line: %q", body)
	}
	dataLine := strings.TrimPrefix(body, "event: ping\ndata: ")
	dataLine = strings.TrimSuffix(dataLine, "\n\n")
	var decoded map[string]any
	if err := json.Unmarshal([]byte(dataLine), &decoded); err != nil {
		t.Fatalf("unmarshal data: %v (raw %q)", err, dataLine)
	}
	if decoded["type"] != "ping" {
		t.Fatalf("type = %v", decoded["type"])
	}
}

func TestReadLine(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	buffer.WriteString("first\r\nsecond\npartial")

	line, ok := sse.ReadLine(&buffer)
	if !ok || line != "first" {
		t.Fatalf("first line = (%q, %v), want (\"first\", true)", line, ok)
	}
	line, ok = sse.ReadLine(&buffer)
	if !ok || line != "second" {
		t.Fatalf("second line = (%q, %v), want (\"second\", true)", line, ok)
	}
	line, ok = sse.ReadLine(&buffer)
	if ok || line != "" {
		t.Fatalf("partial line = (%q, %v), want (\"\", false)", line, ok)
	}
	if got := buffer.String(); got != "partial" {
		t.Fatalf("remainder = %q, want partial", got)
	}
}
