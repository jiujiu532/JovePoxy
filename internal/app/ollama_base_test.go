package app

import "testing"

func TestOllamaAPIBase(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "", want: "https://ollama.com/v1"},
		{in: "https://ollama.com", want: "https://ollama.com/v1"},
		{in: "https://ollama.com/", want: "https://ollama.com/v1"},
		{in: "https://ollama.com/v1", want: "https://ollama.com/v1"},
		{in: "https://ollama.com/v1/", want: "https://ollama.com/v1"},
		{in: "https://example.com/custom", want: "https://example.com/custom/v1"},
	}
	for _, tc := range cases {
		if got := ollamaAPIBase(tc.in); got != tc.want {
			t.Fatalf("ollamaAPIBase(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
