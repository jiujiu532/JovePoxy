// Package idgen centralizes random identifier generation.
// Callers keep package-local thin wrappers so public prefixes and formats stay unchanged.
package idgen

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// Prefixed returns prefix concatenated with the hex encoding of n random bytes.
// n must be positive. Existing callers use n=12 (24 hex chars) or n=16 (32 hex chars).
func Prefixed(prefix string, n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("idgen: random byte length must be positive")
	}
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("idgen: read randomness: %w", err)
	}
	return prefix + hex.EncodeToString(raw), nil
}

// OpenCode returns an OpenCode-style id: prefix_unixMilli_base64url(12 random bytes).
// Matches anthropic message/tool ids and zen x-opencode-* request/session ids.
func OpenCode(prefix string) (string, error) {
	randomBytes := make([]byte, 12)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("idgen: read randomness: %w", err)
	}
	return fmt.Sprintf("%s_%x%s", prefix, time.Now().UnixMilli(), base64.RawURLEncoding.EncodeToString(randomBytes)), nil
}
