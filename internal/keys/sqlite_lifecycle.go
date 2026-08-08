package keys

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (s *SQLiteStore) SetEnabled(ctx context.Context, id KeyID, enabled bool) error {
	result, err := s.database.ExecContext(ctx, "UPDATE local_api_keys SET enabled = ? WHERE id = ?", enabled, id)
	if err != nil {
		return fmt.Errorf("update local API key enabled state: %w", err)
	}
	return requireAffectedRow(result)
}

func (s *SQLiteStore) Update(ctx context.Context, id KeyID, input UpdateInput) error {
	result, err := s.database.ExecContext(ctx, `
		UPDATE local_api_keys
		SET name = ?, rpm_limit = ?, daily_limit = ?
		WHERE id = ? AND revoked_at IS NULL
	`, input.Label, input.RPMLimit, input.DailyLimit, id)
	if err != nil {
		return fmt.Errorf("update local API key fields: %w", err)
	}
	return requireAffectedRow(result)
}

func (s *SQLiteStore) List(ctx context.Context) (metadata []KeyMetadata, err error) {
	// Drop legacy soft-revoked rows so older installs match hard-delete semantics.
	if err := s.purgeSoftRevoked(ctx); err != nil {
		return nil, err
	}
	rows, err := s.database.QueryContext(ctx, `
		SELECT id, name, prefix, enabled, rpm_limit, daily_limit
		FROM local_api_keys
		WHERE revoked_at IS NULL
		ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query local API keys: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	for rows.Next() {
		var item KeyMetadata
		var enabled int
		if err := rows.Scan(
			&item.ID, &item.Label, &item.Prefix, &enabled, &item.RPMLimit, &item.DailyLimit,
		); err != nil {
			return nil, fmt.Errorf("scan local API key: %w", err)
		}
		item.Enabled = enabled != 0
		item.Revoked = false
		metadata = append(metadata, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate local API keys: %w", err)
	}
	return metadata, nil
}

// SecretCiphertext returns the sealed secret for admin reveal (empty = legacy hash-only).
func (s *SQLiteStore) SecretCiphertext(ctx context.Context, id KeyID) (string, error) {
	var ciphertext string
	err := s.database.QueryRowContext(ctx, `
		SELECT COALESCE(secret_ciphertext, '')
		FROM local_api_keys
		WHERE id = ? AND revoked_at IS NULL`, id).Scan(&ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read local API key ciphertext: %w", err)
	}
	return ciphertext, nil
}

func (s *SQLiteStore) purgeSoftRevoked(ctx context.Context) error {
	if _, err := s.database.ExecContext(ctx, `
		UPDATE request_logs SET key_id = NULL
		WHERE key_id IN (SELECT id FROM local_api_keys WHERE revoked_at IS NOT NULL)`); err != nil {
		return fmt.Errorf("detach logs for soft-revoked local API keys: %w", err)
	}
	if _, err := s.database.ExecContext(ctx,
		"DELETE FROM local_api_keys WHERE revoked_at IS NOT NULL"); err != nil {
		return fmt.Errorf("purge soft-revoked local API keys: %w", err)
	}
	return nil
}

func requireAffectedRow(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read local API key update count: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
