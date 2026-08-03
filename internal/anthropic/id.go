package anthropic

import (
	"fmt"

	"jovepoxy/internal/idgen"
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
	id, err := idgen.OpenCode(prefix)
	if err != nil {
		return "", fmt.Errorf("generate %s id: %w", prefix, err)
	}
	return id, nil
}
