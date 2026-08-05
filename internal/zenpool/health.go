package zenpool

import "time"

// DefaultHealthScore is the cold-start / missing-row score for paid keys.
// It is intentionally usable but not a perfect 100.
const DefaultHealthScore = 70.0

// Health is the secret-free, persisted dynamic health snapshot for one paid key.
// It never contains secrets, prompts, completions, cookies, or raw upstream bodies.
type Health struct {
	KeyID               KeyID
	HealthScore         float64
	SuccessCount        int
	FailureCount        int
	ConsecutiveFailures int
	LatencyEMAMs        *float64
	LastErrorClass      string
	LastSuccessAt       *time.Time
	LastFailureAt       *time.Time
	ScoreUpdatedAt      time.Time
	CooldownReason      string
}

// ColdStartHealth returns the default health row for a key with no persisted sample.
// ScoreUpdatedAt is left zero so callers can stamp it on first write.
func ColdStartHealth(id KeyID) Health {
	return Health{KeyID: id, HealthScore: DefaultHealthScore}
}

// NormalizeHealth fills cold-start defaults for a completely empty record.
// A deliberate persisted score of 0 is preserved when any sample/state field is set.
func NormalizeHealth(health Health) Health {
	empty := health.SuccessCount == 0 &&
		health.FailureCount == 0 &&
		health.ConsecutiveFailures == 0 &&
		health.LatencyEMAMs == nil &&
		health.LastErrorClass == "" &&
		health.LastSuccessAt == nil &&
		health.LastFailureAt == nil &&
		health.ScoreUpdatedAt.IsZero() &&
		health.CooldownReason == ""
	if empty && health.HealthScore == 0 {
		health.HealthScore = DefaultHealthScore
	}
	return health
}
