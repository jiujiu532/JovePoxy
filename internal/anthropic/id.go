package anthropic

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

// NewMessageID returns an Anthropic-style message identifier (msg_...).
func NewMessageID() (string, error) {
	return newID("msg")
}

// NewToolUseID returns an Anthropic-style tool use identifier (toolu_...).
func NewToolUseID() (string, error) {
	return newID("toolu")
}

func newID(prefix string) (string, error) {
	randomBytes := make([]byte, 12)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate %s id: %w", prefix, err)
	}
	return fmt.Sprintf("%s_%x%s", prefix, time.Now().UnixMilli(), base64.RawURLEncoding.EncodeToString(randomBytes)), nil
}
