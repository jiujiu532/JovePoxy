package zen

import (
	"fmt"
	"net/http"

	"jovepoxy/internal/idgen"
)

const userAgentSuffix = "ai-sdk/provider-utils/4.0.23 runtime/bun/1.3.13"

func compatibilityHeaders(auth Auth, openCodeVersion string) (http.Header, error) {
	requestID, err := newOpenCodeID("msg")
	if err != nil {
		return nil, fmt.Errorf("generate request ID: %w", err)
	}
	sessionID, err := newOpenCodeID("ses")
	if err != nil {
		return nil, fmt.Errorf("generate session ID: %w", err)
	}

	headers := make(http.Header, 7)
	headers.Set("Content-Type", "application/json")
	headers.Set("Authorization", auth.authorization())
	headers.Set("User-Agent", "opencode/"+openCodeVersion+" "+userAgentSuffix)
	headers.Set("x-opencode-client", "cli")
	headers.Set("x-opencode-project", "global")
	headers.Set("x-opencode-request", requestID)
	headers.Set("x-opencode-session", sessionID)
	return headers, nil
}

// plainAuthHeaders are minimal headers for OpenAI-compatible providers (e.g. Ollama Cloud).
func plainAuthHeaders(auth Auth) http.Header {
	headers := make(http.Header, 2)
	headers.Set("Content-Type", "application/json")
	headers.Set("Authorization", auth.authorization())
	return headers
}

func newOpenCodeID(prefix string) (string, error) {
	return idgen.OpenCode(prefix)
}
