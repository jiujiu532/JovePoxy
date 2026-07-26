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
	rows, err := s.database.QueryContext(ctx, `
		SELECT id, name, prefix, enabled, revoked_at, rpm_limit, daily_limit
		FROM local_api_keys ORDER BY created_at ASC, id ASC`)
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
		var revokedAt sql.NullString
		if err := rows.Scan(
			&item.ID, &item.Label, &item.Prefix, &enabled, &revokedAt, &item.RPMLimit, &item.DailyLimit,
		); err != nil {
			return nil, fmt.Errorf("scan local API key: %w", err)
		}
		item.Enabled = enabled != 0
		item.Revoked = revokedAt.Valid
		metadata = append(metadata, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate local API keys: %w", err)
	}
	return metadata, nil
}

func requireAffectedRow(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read local API key update count: %w", err)
	}
	if affected == 0 {
		return ErrUnauthorized
	}
	return nil
}
