package anthropic

import (
	"encoding/json"
	"net/http"
)

// writeError writes an Anthropic-shaped JSON error body (not SSE).
// Shape differs from responses package; keep package-local.
func writeError(writer http.ResponseWriter, status int, errorType, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"type": "error", "error": map[string]string{"type": errorType, "message": message},
	})
}
