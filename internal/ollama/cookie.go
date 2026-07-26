package ollama

import (
	"errors"
	"strings"
)

var ErrInvalidCookie = errors.New("invalid Ollama session cookie")

// NormalizeSessionCookie accepts raw token, Cookie: prefix, or full cookie header.
// Matches QuotaHub build_ollama_cookie_header behavior.
func NormalizeSessionCookie(raw string) (string, error) {
	cookie := strings.TrimSpace(raw)
	if len(cookie) >= len("Cookie:") && strings.EqualFold(cookie[:len("Cookie:")], "Cookie:") {
		cookie = strings.TrimSpace(cookie[len("Cookie:"):])
	}
	if cookie == "" {
		return "", ErrInvalidCookie
	}
	if !strings.Contains(cookie, "=") {
		return "__Secure-session=" + cookie, nil
	}
	return strings.TrimRight(cookie, ";"), nil
}
