package zenpool

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type storedKey struct {
	id            KeyID
	label         string
	ciphertext    string
	weight        int
	enabled       bool
	provider      Provider
	cooldownUntil sql.NullString
	createdAt     string
}

type Store interface {
	Insert(context.Context, storedKey) error
	List(context.Context) ([]storedKey, error)
	ListByProvider(context.Context, Provider) ([]storedKey, error)
	SetEnabled(context.Context, KeyID, bool) error
	SetCooldown(context.Context, KeyID, *time.Time) error
	Update(context.Context, KeyID, string, int, *string) error
	Delete(context.Context, KeyID) error
	GetCiphertext(context.Context, KeyID) (string, error)
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
		INSERT INTO zen_keys (id, label, key_ciphertext, weight, enabled, cooldown_until, created_at, provider)
		VALUES (?, ?, ?, ?, ?, NULL, ?, ?)
	`, string(key.id), key.label, key.ciphertext, key.weight, boolToInt(key.enabled), key.createdAt, string(provider))
	if err != nil {
		return fmt.Errorf("insert zen key: %w", err)
	}
	return nil
}

func (store *sqliteStore) List(ctx context.Context) ([]storedKey, error) {
	return store.queryKeys(ctx, `
		SELECT id, label, key_ciphertext, weight, enabled, cooldown_until, created_at, COALESCE(provider, 'opencode')
		FROM zen_keys ORDER BY created_at ASC, id ASC
	`)
}

func (store *sqliteStore) ListByProvider(ctx context.Context, provider Provider) ([]storedKey, error) {
	if provider == "" {
		provider = ProviderOpenCode
	}
	return store.queryKeys(ctx, `
		SELECT id, label, key_ciphertext, weight, enabled, cooldown_until, created_at, COALESCE(provider, 'opencode')
		FROM zen_keys
		WHERE COALESCE(provider, 'opencode') = ?
		ORDER BY created_at ASC, id ASC
	`, string(provider))
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
		if err := rows.Scan(
			&key.id, &key.label, &key.ciphertext, &key.weight, &enabled,
			&key.cooldownUntil, &key.createdAt, &provider,
		); err != nil {
			return nil, fmt.Errorf("scan zen key: %w", err)
		}
		key.enabled = enabled == 1
		key.provider = Provider(provider)
		if key.provider == "" {
			key.provider = ProviderOpenCode
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

// Update patches label/weight; when ciphertext is non-nil, also replaces secret material.
func (store *sqliteStore) Update(ctx context.Context, id KeyID, label string, weight int, ciphertext *string) error {
	var result sql.Result
	var err error
	if ciphertext != nil {
		result, err = store.db.ExecContext(ctx, `
			UPDATE zen_keys SET label = ?, weight = ?, key_ciphertext = ? WHERE id = ?
		`, label, weight, *ciphertext, string(id))
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

func (store *sqliteStore) GetCiphertext(ctx context.Context, id KeyID) (string, error) {
	var ciphertext string
	err := store.db.QueryRowContext(ctx, `SELECT key_ciphertext FROM zen_keys WHERE id = ?`, string(id)).Scan(&ciphertext)
	if err != nil {
		return "", fmt.Errorf("get zen key ciphertext: %w", err)
	}
	return ciphertext, nil
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
