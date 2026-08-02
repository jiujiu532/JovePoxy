package httpserver

import "testing"

func TestIsOpenAIStyleRateLimit(t *testing.T) {
	t.Parallel()

	for _, scenario := range []struct {
		name string
		body string
		want bool
	}{
		{
			name: "FreeUsageLimitError",
			body: `{"type":"FreeUsageLimitError","message":"quota"}`,
			want: true,
		},
		{
			name: "error.type server_error",
			body: `{"error":{"type":"server_error","message":"boom"}}`,
			want: false,
		},
		{
			name: "error.type rate_limit_error",
			body: `{"error":{"type":"rate_limit_error","message":"slow down"}}`,
			want: true,
		},
		{
			name: "error.code rate_limit_exceeded",
			body: `{"error":{"type":"api_error","code":"rate_limit_exceeded"}}`,
			want: true,
		},
		{
			name: "error.code insufficient_quota",
			body: `{"error":{"code":"insufficient_quota"}}`,
			want: true,
		},
		{
			name: "invalid_request_error",
			body: `{"error":{"type":"invalid_request_error","message":"bad"}}`,
			want: false,
		},
		{
			name: "successful choices payload",
			body: `{"choices":[{"message":{"content":"ok"}}],"error":{"type":"rate_limit_error"}}`,
			want: false,
		},
		{
			name: "empty error object",
			body: `{"error":{}}`,
			want: false,
		},
	} {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			if got := isOpenAIStyleRateLimit([]byte(scenario.body)); got != scenario.want {
				t.Fatalf("isOpenAIStyleRateLimit(%s) = %v, want %v", scenario.body, got, scenario.want)
			}
		})
	}
}
