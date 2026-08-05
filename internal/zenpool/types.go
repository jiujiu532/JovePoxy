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

// LoadPolicy controls how healthy keys are chosen for paid traffic.
type LoadPolicy string

const (
	// LoadPolicySpread is weighted round-robin (default, matches legacy behavior).
	LoadPolicySpread LoadPolicy = "spread"
	// LoadPolicySticky pins a conversation affinity hash to one healthy key via weighted rendezvous.
	LoadPolicySticky LoadPolicy = "sticky"
)

// ErrNoHealthyKey is returned when no enabled, non-cooling, non-benched key is available.
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
	ID     KeyID
	Label  string
	Prefix string
	// Weight remains in the stored/API shape only for backwards compatibility.
	// Dynamic health selection ignores it.
	Weight        int
	Enabled       bool
	Provider      Provider
	CooldownUntil *time.Time
	CreatedAt     time.Time

	HealthScore         float64
	SelectionScore      int
	SuccessCount        int
	FailureCount        int
	ConsecutiveFailures int
	LastErrorClass      string
	LastSuccessAt       *time.Time
	LastFailureAt       *time.Time
	HealthUpdatedAt     *time.Time
	CooldownReason      string
	NeedsProbe          bool
}

// Selected is a decrypted key chosen for an upstream request.
type Selected struct {
	ID      KeyID
	Secret  string
	Label   string
	Probing bool
}

// Cooldown holds how long a key should rest after a failure class.
type Cooldown struct {
	Duration time.Duration
}

// Default cooldown / bench / failover policy.
const (
	DefaultCooldown      = 60 * time.Second
	UnauthorizedCooldown = 5 * time.Minute
	// DefaultBenchDuration is process-memory bench window after 401 (independent of SQLite cooldown).
	DefaultBenchDuration = 10 * time.Minute
	// DefaultMaxAttempts is primary + one failover (legacy ProxyPaid semantics).
	DefaultMaxAttempts = 2
	MinMaxAttempts     = 2
	MaxMaxAttempts     = 4
)
