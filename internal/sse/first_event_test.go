package sse_test

import (
	"testing"

	"jovepoxy/internal/sse"
)

func TestIsRateLimitEvent_and_payload(t *testing.T) {
	t.Parallel()

	for _, scenario := range []struct {
		name  string
		event string
		want  bool
	}{
		{
			name:  "top-level FreeUsageLimitError",
			event: "data: {\"type\":\"FreeUsageLimitError\"}\n\n",
			want:  true,
		},
		{
			name:  "nested FreeUsageLimitError type",
			event: "data: {\"error\":{\"type\":\"FreeUsageLimitError\",\"message\":\"quota\"}}\n\n",
			want:  true,
		},
		{
			name:  "rate_limit_error type",
			event: "data: {\"error\":{\"type\":\"rate_limit_error\",\"message\":\"slow down\"}}\n\n",
			want:  true,
		},
		{
			name:  "rate_limit_exceeded code",
			event: "data: {\"error\":{\"type\":\"api_error\",\"code\":\"rate_limit_exceeded\"}}\n\n",
			want:  true,
		},
		{
			name:  "insufficient_quota code",
			event: "data: {\"error\":{\"code\":\"insufficient_quota\"}}\n\n",
			want:  true,
		},
		{
			name:  "server_error is not rate limit",
			event: "data: {\"error\":{\"type\":\"server_error\",\"message\":\"boom\"}}\n\n",
			want:  false,
		},
		{
			name:  "invalid_request_error is not rate limit",
			event: "data: {\"error\":{\"type\":\"invalid_request_error\",\"message\":\"bad\"}}\n\n",
			want:  false,
		},
		{
			name:  "empty error object is not rate limit",
			event: "data: {\"error\":{}}\n\n",
			want:  false,
		},
		{
			name:  "non-json data is not rate limit",
			event: "data: not-json\n\n",
			want:  false,
		},
	} {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			if got := sse.IsRateLimitEvent([]byte(scenario.event)); got != scenario.want {
				t.Fatalf("IsRateLimitEvent() = %v, want %v for %q", got, scenario.want, scenario.event)
			}
		})
	}
}

func TestIsRateLimitPayload(t *testing.T) {
	t.Parallel()

	for _, scenario := range []struct {
		name    string
		payload string
		want    bool
	}{
		{name: "FreeUsageLimitError", payload: `{"type":"FreeUsageLimitError"}`, want: true},
		{name: "server_error", payload: `{"error":{"type":"server_error"}}`, want: false},
		{name: "rate_limit_error", payload: `{"error":{"type":"rate_limit_error"}}`, want: true},
		{name: "case-insensitive free usage", payload: `{"type":"freeusagelimiterror"}`, want: true},
	} {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			if got := sse.IsRateLimitPayload([]byte(scenario.payload)); got != scenario.want {
				t.Fatalf("IsRateLimitPayload(%s) = %v, want %v", scenario.payload, got, scenario.want)
			}
		})
	}
}
