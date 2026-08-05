package zenpool

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// healthTimeLayout is fixed-width UTC so lexicographic order matches chronology.
const healthTimeLayout = "2006-01-02T15:04:05.000000000Z07:00"

type storedKey struct {
	id            KeyID
	label         string
	ciphertext    string
	keyPrefix     string
	weight        int
	enabled       bool
	provider      Provider
	cooldownUntil sql.NullString
	createdAt     string
	// health is always populated on List/ListByProvider via LEFT JOIN COALESCE cold-start.
	health Health
}

// Store is the SQLite persistence boundary for zen keys and secret-free health rows.
// Health writes are intended to be called off the hot request path (or throttled);
// methods themselves are synchronous SQLite ops and never log secrets/bodies.
type Store interface {
	Insert(context.Context, storedKey) error
	List(context.Context) ([]storedKey, error)
	ListByProvider(context.Context, Provider) ([]storedKey, error)
	SetEnabled(context.Context, KeyID, bool) error
	SetCooldown(context.Context, KeyID, *time.Time) error
	// Update patches label/weight; when ciphertext is non-nil also replaces secret and key_prefix.
	Update(context.Context, KeyID, string, int, *string, *string) error
	Delete(context.Context, KeyID) error

	// GetHealth returns persisted health or a cold-start default when no row exists.
	GetHealth(context.Context, KeyID) (Health, error)
	// UpsertHealth inserts or replaces one key's health snapshot (no credentials).
	UpsertHealth(context.Context, Health) error
	// ListHealth returns all persisted health rows (keys without rows are omitted).
	ListHealth(context.Context) ([]Health, error)
}

type sqliteStore struct {
	db *sql.DB
}

func NewSQLiteStore(database *sql.DB) Store {
	return &sqliteStore{db: database}
}

func (store *sqliteStore) Insert(ctx context.Context, key storedKey) error {
	provider := key.provider
	if provider == "" {
		provider = ProviderOpenCode
	}
	_, err := store.db.ExecContext(ctx, `
		INSERT INTO zen_keys (id, label, key_ciphertext, key_prefix, weight, enabled, cooldown_until, created_at, provider)
		VALUES (?, ?, ?, ?, ?, ?, NULL, ?, ?)
	`, string(key.id), key.label, key.ciphertext, key.keyPrefix, key.weight, boolToInt(key.enabled), key.createdAt, string(provider))
	if err != nil {
		return fmt.Errorf("insert zen key: %w", err)
	}
	return nil
}

func (store *sqliteStore) List(ctx context.Context) ([]storedKey, error) {
	return store.queryKeys(ctx, `
		SELECT
			k.id, k.label, k.key_ciphertext, COALESCE(k.key_prefix, ''), k.weight, k.enabled,
			k.cooldown_until, k.created_at, COALESCE(k.provider, 'opencode'),
			COALESCE(h.health_score, ?),
			COALESCE(h.success_count, 0),
			COALESCE(h.failure_count, 0),
			COALESCE(h.consecutive_failures, 0),
			h.latency_ema_ms,
			COALESCE(h.last_error_class, ''),
			h.last_success_at,
			h.last_failure_at,
			h.score_updated_at,
			COALESCE(h.cooldown_reason, '')
		FROM zen_keys k
		LEFT JOIN zen_key_health h ON h.key_id = k.id
		ORDER BY k.created_at ASC, k.id ASC
	`, DefaultHealthScore)
}

func (store *sqliteStore) ListByProvider(ctx context.Context, provider Provider) ([]storedKey, error) {
	if provider == "" {
		provider = ProviderOpenCode
	}
	return store.queryKeys(ctx, `
		SELECT
			k.id, k.label, k.key_ciphertext, COALESCE(k.key_prefix, ''), k.weight, k.enabled,
			k.cooldown_until, k.created_at, COALESCE(k.provider, 'opencode'),
			COALESCE(h.health_score, ?),
			COALESCE(h.success_count, 0),
			COALESCE(h.failure_count, 0),
			COALESCE(h.consecutive_failures, 0),
			h.latency_ema_ms,
			COALESCE(h.last_error_class, ''),
			h.last_success_at,
			h.last_failure_at,
			h.score_updated_at,
			COALESCE(h.cooldown_reason, '')
		FROM zen_keys k
		LEFT JOIN zen_key_health h ON h.key_id = k.id
		WHERE COALESCE(k.provider, 'opencode') = ?
		ORDER BY k.created_at ASC, k.id ASC
	`, DefaultHealthScore, string(provider))
}

func (store *sqliteStore) queryKeys(ctx context.Context, query string, args ...any) ([]storedKey, error) {
	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list zen keys: %w", err)
	}
	defer rows.Close()
	keys := make([]storedKey, 0)
	for rows.Next() {
		var key storedKey
		var enabled int
		var provider string
		var latency sql.NullFloat64
		var lastSuccess, lastFailure, scoreUpdated sql.NullString
		if err := rows.Scan(
			&key.id, &key.label, &key.ciphertext, &key.keyPrefix, &key.weight, &enabled,
			&key.cooldownUntil, &key.createdAt, &provider,
			&key.health.HealthScore,
			&key.health.SuccessCount,
			&key.health.FailureCount,
			&key.health.ConsecutiveFailures,
			&latency,
			&key.health.LastErrorClass,
			&lastSuccess,
			&lastFailure,
			&scoreUpdated,
			&key.health.CooldownReason,
		); err != nil {
			return nil, fmt.Errorf("scan zen key: %w", err)
		}
		key.enabled = enabled == 1
		key.provider = Provider(provider)
		if key.provider == "" {
			key.provider = ProviderOpenCode
		}
		key.health.KeyID = key.id
		if latency.Valid {
			value := latency.Float64
			key.health.LatencyEMAMs = &value
		}
		key.health.LastSuccessAt = parseHealthTime(lastSuccess)
		key.health.LastFailureAt = parseHealthTime(lastFailure)
		if scoreUpdated.Valid {
			if ts, err := time.Parse(healthTimeLayout, scoreUpdated.String); err == nil {
				key.health.ScoreUpdatedAt = ts.UTC()
			} else if ts, err := time.Parse(time.RFC3339Nano, scoreUpdated.String); err == nil {
				key.health.ScoreUpdatedAt = ts.UTC()
			}
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (store *sqliteStore) SetEnabled(ctx context.Context, id KeyID, enabled bool) error {
	result, err := store.db.ExecContext(ctx, `UPDATE zen_keys SET enabled = ? WHERE id = ?`, boolToInt(enabled), string(id))
	if err != nil {
		return fmt.Errorf("set zen key enabled: %w", err)
	}
	return requireRows(result)
}

func (store *sqliteStore) SetCooldown(ctx context.Context, id KeyID, until *time.Time) error {
	var value any
	if until != nil {
		value = until.UTC().Format(time.RFC3339Nano)
	}
	result, err := store.db.ExecContext(ctx, `UPDATE zen_keys SET cooldown_until = ? WHERE id = ?`, value, string(id))
	if err != nil {
		return fmt.Errorf("set zen key cooldown: %w", err)
	}
	return requireRows(result)
}

// Update patches label/weight; when ciphertext is non-nil, also replaces secret material and key_prefix.
func (store *sqliteStore) Update(ctx context.Context, id KeyID, label string, weight int, ciphertext *string, keyPrefix *string) error {
	var result sql.Result
	var err error
	if ciphertext != nil {
		prefix := ""
		if keyPrefix != nil {
			prefix = *keyPrefix
		}
		result, err = store.db.ExecContext(ctx, `
			UPDATE zen_keys SET label = ?, weight = ?, key_ciphertext = ?, key_prefix = ? WHERE id = ?
		`, label, weight, *ciphertext, prefix, string(id))
	} else {
		result, err = store.db.ExecContext(ctx, `
			UPDATE zen_keys SET label = ?, weight = ? WHERE id = ?
		`, label, weight, string(id))
	}
	if err != nil {
		return fmt.Errorf("update zen key: %w", err)
	}
	return requireRows(result)
}

func (store *sqliteStore) Delete(ctx context.Context, id KeyID) error {
	result, err := store.db.ExecContext(ctx, `DELETE FROM zen_keys WHERE id = ?`, string(id))
	if err != nil {
		return fmt.Errorf("delete zen key: %w", err)
	}
	return requireRows(result)
}

func (store *sqliteStore) GetHealth(ctx context.Context, id KeyID) (Health, error) {
	if id == "" {
		return Health{}, fmt.Errorf("get zen key health: empty key id")
	}
	var health Health
	var latency sql.NullFloat64
	var lastSuccess, lastFailure, scoreUpdated sql.NullString
	err := store.db.QueryRowContext(ctx, `
		SELECT key_id, health_score, success_count, failure_count, consecutive_failures,
			latency_ema_ms, last_error_class, last_success_at, last_failure_at,
			score_updated_at, cooldown_reason
		FROM zen_key_health WHERE key_id = ?
	`, string(id)).Scan(
		&health.KeyID,
		&health.HealthScore,
		&health.SuccessCount,
		&health.FailureCount,
		&health.ConsecutiveFailures,
		&latency,
		&health.LastErrorClass,
		&lastSuccess,
		&lastFailure,
		&scoreUpdated,
		&health.CooldownReason,
	)
	if errorsIsNoRows(err) {
		return ColdStartHealth(id), nil
	}
	if err != nil {
		return Health{}, fmt.Errorf("get zen key health: %w", err)
	}
	if latency.Valid {
		value := latency.Float64
		health.LatencyEMAMs = &value
	}
	health.LastSuccessAt = parseHealthTime(lastSuccess)
	health.LastFailureAt = parseHealthTime(lastFailure)
	if scoreUpdated.Valid {
		if ts, parseErr := time.Parse(healthTimeLayout, scoreUpdated.String); parseErr == nil {
			health.ScoreUpdatedAt = ts.UTC()
		} else if ts, parseErr := time.Parse(time.RFC3339Nano, scoreUpdated.String); parseErr == nil {
			health.ScoreUpdatedAt = ts.UTC()
		}
	}
	return health, nil
}

func (store *sqliteStore) UpsertHealth(ctx context.Context, health Health) error {
	if health.KeyID == "" {
		return fmt.Errorf("upsert zen key health: empty key id")
	}
	scoreUpdated := health.ScoreUpdatedAt
	if scoreUpdated.IsZero() {
		scoreUpdated = time.Now().UTC()
	}
	var latency any
	if health.LatencyEMAMs != nil {
		latency = *health.LatencyEMAMs
	}
	var lastSuccess any
	if health.LastSuccessAt != nil {
		lastSuccess = health.LastSuccessAt.UTC().Format(healthTimeLayout)
	}
	var lastFailure any
	if health.LastFailureAt != nil {
		lastFailure = health.LastFailureAt.UTC().Format(healthTimeLayout)
	}
	_, err := store.db.ExecContext(ctx, `
		INSERT INTO zen_key_health (
			key_id, health_score, success_count, failure_count, consecutive_failures,
			latency_ema_ms, last_error_class, last_success_at, last_failure_at,
			score_updated_at, cooldown_reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(key_id) DO UPDATE SET
			health_score = excluded.health_score,
			success_count = excluded.success_count,
			failure_count = excluded.failure_count,
			consecutive_failures = excluded.consecutive_failures,
			latency_ema_ms = excluded.latency_ema_ms,
			last_error_class = excluded.last_error_class,
			last_success_at = excluded.last_success_at,
			last_failure_at = excluded.last_failure_at,
			score_updated_at = excluded.score_updated_at,
			cooldown_reason = excluded.cooldown_reason
	`,
		string(health.KeyID),
		health.HealthScore,
		health.SuccessCount,
		health.FailureCount,
		health.ConsecutiveFailures,
		latency,
		health.LastErrorClass,
		lastSuccess,
		lastFailure,
		scoreUpdated.UTC().Format(healthTimeLayout),
		health.CooldownReason,
	)
	if err != nil {
		return fmt.Errorf("upsert zen key health: %w", err)
	}
	return nil
}

func (store *sqliteStore) ListHealth(ctx context.Context) ([]Health, error) {
	rows, err := store.db.QueryContext(ctx, `
		SELECT key_id, health_score, success_count, failure_count, consecutive_failures,
			latency_ema_ms, last_error_class, last_success_at, last_failure_at,
			score_updated_at, cooldown_reason
		FROM zen_key_health
		ORDER BY key_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list zen key health: %w", err)
	}
	defer rows.Close()
	out := make([]Health, 0)
	for rows.Next() {
		var health Health
		var latency sql.NullFloat64
		var lastSuccess, lastFailure, scoreUpdated sql.NullString
		if err := rows.Scan(
			&health.KeyID,
			&health.HealthScore,
			&health.SuccessCount,
			&health.FailureCount,
			&health.ConsecutiveFailures,
			&latency,
			&health.LastErrorClass,
			&lastSuccess,
			&lastFailure,
			&scoreUpdated,
			&health.CooldownReason,
		); err != nil {
			return nil, fmt.Errorf("scan zen key health: %w", err)
		}
		if latency.Valid {
			value := latency.Float64
			health.LatencyEMAMs = &value
		}
		health.LastSuccessAt = parseHealthTime(lastSuccess)
		health.LastFailureAt = parseHealthTime(lastFailure)
		if scoreUpdated.Valid {
			if ts, parseErr := time.Parse(healthTimeLayout, scoreUpdated.String); parseErr == nil {
				health.ScoreUpdatedAt = ts.UTC()
			} else if ts, parseErr := time.Parse(time.RFC3339Nano, scoreUpdated.String); parseErr == nil {
				health.ScoreUpdatedAt = ts.UTC()
			}
		}
		out = append(out, health)
	}
	return out, rows.Err()
}

func parseHealthTime(value sql.NullString) *time.Time {
	if !value.Valid || value.String == "" {
		return nil
	}
	if ts, err := time.Parse(healthTimeLayout, value.String); err == nil {
		utc := ts.UTC()
		return &utc
	}
	if ts, err := time.Parse(time.RFC3339Nano, value.String); err == nil {
		utc := ts.UTC()
		return &utc
	}
	return nil
}

func errorsIsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func requireRows(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
