package zenpool

import (
	"errors"
	"time"
)

// KeyID identifies a stored upstream API key.
type KeyID string

// Provider scopes a key to an upstream product pool.
type Provider string

const (
	ProviderOpenCode Provider = "opencode"
	ProviderOllama   Provider = "ollama"
)

// ErrNoHealthyKey is returned when no enabled, non-cooling key is available.
var ErrNoHealthyKey = errors.New("zenpool: no healthy zen key available")

// CreateInput is the boundary input for adding an upstream key.
type CreateInput struct {
	Label    string
	Secret   string
	Weight   int
	Provider Provider
}

// UpdateInput patches Zen key metadata. Secret empty keeps the existing secret.
type UpdateInput struct {
	Label  string
	Weight int
	Secret string
}

// Metadata is a secret-free view of an upstream key.
type Metadata struct {
	ID            KeyID
	Label         string
	Prefix        string
	Weight        int
	Enabled       bool
	Provider      Provider
	CooldownUntil *time.Time
	CreatedAt     time.Time
}

// Selected is a decrypted key chosen for an upstream request.
type Selected struct {
	ID     KeyID
	Secret string
	Label  string
}

// Cooldown holds how long a key should rest after a failure class.
type Cooldown struct {
	Duration time.Duration
}

// Default cooldown policy from the locked plan: 60s general, 5m for 401.
const (
	DefaultCooldown   = 60 * time.Second
	UnauthorizedCooldown = 5 * time.Minute
)
