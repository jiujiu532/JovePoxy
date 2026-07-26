package keys

import (
	"errors"
	"strings"
)

var (
	ErrInvalidInput = errors.New("invalid local API key input")
	ErrUnauthorized = errors.New("local API key unauthorized")
	ErrRateLimited  = errors.New("local API key rate limited")
)

type KeyID string

type CreateInput struct {
	Label      string
	RPMLimit   int
	DailyLimit int
}

// UpdateInput patches local key metadata. Secret is never changeable.
type UpdateInput struct {
	Label      string
	RPMLimit   int
	DailyLimit int
}

type Creation struct {
	ID     KeyID
	Prefix string
	Secret string
}

type Credentials struct {
	Authorization string
	APIKey        string
}

type VerifiedKey struct {
	ID     KeyID
	Label  string
	Prefix string
}

type KeyMetadata struct {
	ID         KeyID
	Label      string
	Prefix     string
	Enabled    bool
	Revoked    bool
	RPMLimit   int
	DailyLimit int
}

type credential struct{ secret string }

func parseCreateInput(input CreateInput) (CreateInput, error) {
	input.Label = strings.TrimSpace(input.Label)
	if input.Label == "" || input.RPMLimit < 0 || input.DailyLimit < 0 {
		return CreateInput{}, ErrInvalidInput
	}
	return input, nil
}

func parseUpdateInput(input UpdateInput) (UpdateInput, error) {
	input.Label = strings.TrimSpace(input.Label)
	if input.Label == "" || input.RPMLimit < 0 || input.DailyLimit < 0 {
		return UpdateInput{}, ErrInvalidInput
	}
	return input, nil
}

func parseCredentials(input Credentials) (credential, error) {
	authorization := strings.TrimSpace(input.Authorization)
	apiKey := strings.TrimSpace(input.APIKey)
	if authorization != "" && apiKey != "" {
		return credential{}, ErrUnauthorized
	}
	if apiKey != "" {
		return parseSecret(apiKey)
	}
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return credential{}, ErrUnauthorized
	}
	return parseSecret(parts[1])
}

func parseSecret(secret string) (credential, error) {
	const prefix = "sk-oc-"
	if !strings.HasPrefix(secret, prefix) || len(secret) != len(prefix)+64 {
		return credential{}, ErrUnauthorized
	}
	for _, character := range secret[len(prefix):] {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return credential{}, ErrUnauthorized
		}
	}
	return credential{secret: secret}, nil
}
