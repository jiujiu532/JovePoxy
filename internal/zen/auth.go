package zen

import (
	"errors"
	"strings"
)

var ErrBlankAPIKey = errors.New("zen: API key must not be blank")

// Auth selects an upstream authorization scheme. It is sealed so callers must
// use PublicAuth or NewAPIKey instead of passing an arbitrary bearer string.
type Auth interface {
	authorization() string
	sealed()
}

type publicAuth struct{}

func (publicAuth) authorization() string { return "Bearer public" }
func (publicAuth) sealed()               {}

// PublicAuth authorizes a request against Zen's public model path.
func PublicAuth() Auth { return publicAuth{} }

// APIKey is a validated, non-blank Zen API key.
type APIKey struct {
	value string
}

func (APIKey) sealed() {}

func (key APIKey) authorization() string { return "Bearer " + key.value }

// NewAPIKey constructs paid upstream authorization without exposing a stringly
// typed authentication choice in the client API.
func NewAPIKey(value string) (APIKey, error) {
	if strings.TrimSpace(value) == "" {
		return APIKey{}, ErrBlankAPIKey
	}
	return APIKey{value: value}, nil
}
