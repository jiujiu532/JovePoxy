package keys

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type SQLiteStore struct{ database *sql.DB }

func NewSQLiteStore(database *sql.DB) *SQLiteStore { return &SQLiteStore{database: database} }

func (s *SQLiteStore) Create(ctx context.Context, key storedKey) error {
	_, err := s.database.ExecContext(ctx, `
		INSERT INTO local_api_keys (id, name, key_hash, prefix, rpm_limit, daily_limit)
		VALUES (?, ?, ?, ?, ?, ?)`, key.id, key.label, key.hash, key.prefix, key.rpmLimit, key.dailyLimit)
	if err != nil {
		return fmt.Errorf("insert local API key: %w", err)
	}
	return nil
}

func (s *SQLiteStore) VerifyAndConsume(ctx context.Context, candidateHash string, now time.Time) (verified VerifiedKey, err error) {
	connection, err := s.database.Conn(ctx)
	if err != nil {
		return VerifiedKey{}, fmt.Errorf("acquire local key connection: %w", err)
	}
	defer func() { err = errors.Join(err, connection.Close()) }()
	if _, err := connection.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return VerifiedKey{}, fmt.Errorf("lock local key usage: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, rollbackErr := connection.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
			err = errors.Join(err, rollbackErr)
		}
	}()
	record, err := loadKey(ctx, connection, candidateHash)
	if err != nil {
		return VerifiedKey{}, err
	}
	if err := consumeLimits(ctx, connection, record, now); err != nil {
		return VerifiedKey{}, err
	}
	if _, err := connection.ExecContext(ctx, "COMMIT"); err != nil {
		return VerifiedKey{}, fmt.Errorf("commit local key usage: %w", err)
	}
	committed = true
	return VerifiedKey{ID: record.id, Label: record.label, Prefix: record.prefix}, nil
}

// Revoke hard-deletes the key. local_key_usage cascades; request_logs.key_id is nulled.
func (s *SQLiteStore) Revoke(ctx context.Context, id KeyID) error {
	if _, err := s.database.ExecContext(ctx,
		"UPDATE request_logs SET key_id = NULL WHERE key_id = ?", id,
	); err != nil {
		return fmt.Errorf("detach request logs for local API key: %w", err)
	}
	result, err := s.database.ExecContext(ctx, "DELETE FROM local_api_keys WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete local API key: %w", err)
	}
	return requireAffectedRow(result)
}

type keyRecord struct {
	id         KeyID
	label      string
	prefix     string
	hash       string
	rpmLimit   int
	dailyLimit int
}

func loadKey(ctx context.Context, connection *sql.Conn, candidateHash string) (keyRecord, error) {
	var record keyRecord
	var enabled int
	var revokedAt sql.NullString
	err := connection.QueryRowContext(ctx, `
		SELECT id, name, prefix, key_hash, rpm_limit, daily_limit, enabled, revoked_at
		FROM local_api_keys WHERE key_hash = ?`, candidateHash).Scan(
		&record.id, &record.label, &record.prefix, &record.hash, &record.rpmLimit, &record.dailyLimit, &enabled, &revokedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return keyRecord{}, ErrUnauthorized
	}
	if err != nil {
		return keyRecord{}, fmt.Errorf("load local API key: %w", err)
	}
	storedHash, decodeErr := hex.DecodeString(record.hash)
	candidate, candidateErr := hex.DecodeString(candidateHash)
	if decodeErr != nil || candidateErr != nil || subtle.ConstantTimeCompare(storedHash, candidate) != 1 || enabled == 0 || revokedAt.Valid {
		return keyRecord{}, ErrUnauthorized
	}
	return record, nil
}

func consumeLimits(ctx context.Context, connection *sql.Conn, record keyRecord, now time.Time) error {
	if err := consumeWindow(ctx, connection, record.id, "day", dayStart(now), record.dailyLimit); err != nil {
		return err
	}
	return consumeWindow(ctx, connection, record.id, "minute", minuteStart(now), record.rpmLimit)
}

func consumeWindow(ctx context.Context, connection *sql.Conn, id KeyID, kind string, windowStart time.Time, limit int) error {
	if limit == 0 {
		return nil
	}
	var storedStart string
	var count int
	err := connection.QueryRowContext(ctx,
		"SELECT window_started_at, request_count FROM local_key_usage WHERE key_id = ? AND window_kind = ?", id, kind,
	).Scan(&storedStart, &count)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = connection.ExecContext(ctx,
			"INSERT INTO local_key_usage (key_id, window_kind, window_started_at, request_count) VALUES (?, ?, ?, 1)",
			id, kind, windowStart.Format(time.RFC3339),
		)
		if err != nil {
			return fmt.Errorf("create local key usage: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("load local key usage: %w", err)
	}
	if storedStart != windowStart.Format(time.RFC3339) {
		_, err = connection.ExecContext(ctx,
			"UPDATE local_key_usage SET window_started_at = ?, request_count = 1 WHERE key_id = ? AND window_kind = ?",
			windowStart.Format(time.RFC3339), id, kind,
		)
		if err != nil {
			return fmt.Errorf("reset local key usage: %w", err)
		}
		return nil
	}
	if count >= limit {
		return ErrRateLimited
	}
	if _, err := connection.ExecContext(ctx,
		"UPDATE local_key_usage SET request_count = request_count + 1 WHERE key_id = ? AND window_kind = ?", id, kind,
	); err != nil {
		return fmt.Errorf("increment local key usage: %w", err)
	}
	return nil
}

func minuteStart(now time.Time) time.Time { return now.Truncate(time.Minute) }

func dayStart(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}
