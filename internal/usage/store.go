package usage

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"jovepoxy/internal/idgen"
)

// ModelAggregate is one model row for overview dashboards.
type ModelAggregate struct {
	Model        string
	Requests     int64
	InputTokens  int64
	OutputTokens int64
}

// ListFilter bounds usage listing. Zero From/To means open-ended.
type ListFilter struct {
	AccountID string
	From      time.Time // inclusive if set (recorded_at)
	To        time.Time // inclusive if set (recorded_at)
	Limit     int
	Offset    int
}

// Store persists usage records and sync cursors.
type Store interface {
	InsertIgnore(ctx context.Context, accountID string, records []Record) (int, error)
	List(ctx context.Context, filter ListFilter) ([]StoredRecord, error)
	GetSyncState(ctx context.Context, accountID string) (SyncState, error)
	SetSyncState(ctx context.Context, state SyncState) error
	// AggregateTotals returns request count and token sum since the RFC3339 bound (inclusive).
	// Pass empty since for all-time totals.
	AggregateTotals(ctx context.Context, sinceRFC3339 string) (requests int64, tokens int64, err error)
	AggregateByModel(ctx context.Context, limit int) ([]ModelAggregate, error)
}

// StoredRecord is a row as returned to list APIs.
type StoredRecord struct {
	ID           string
	AccountID    string
	USGID        string
	Model        string
	InputTokens  int
	OutputTokens int
	RecordedAt   string
}

// SyncState tracks incremental/backfill progress per account.
type SyncState struct {
	AccountID           string
	DeepestPageFetched  int
	UpdatedAt           time.Time
	LastInsertedCount   int
	LastStatus          string
	LastError           string
}

type sqliteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a usage store on the shared SQLite database.
func NewSQLiteStore(database *sql.DB) Store {
	return &sqliteStore{db: database}
}

func (store *sqliteStore) InsertIgnore(ctx context.Context, accountID string, records []Record) (int, error) {
	if accountID == "" {
		return 0, fmt.Errorf("usage: account id is required")
	}
	inserted := 0
	for _, record := range records {
		id, err := newRowID()
		if err != nil {
			return inserted, err
		}
		// Normalize to fixed-ms RFC3339 so list bounds compare lexicographically.
		recordedAt := normalizeRecordedAt(record.CreatedAt)
		result, err := store.db.ExecContext(ctx, `
			INSERT OR IGNORE INTO usage_records (id, account_id, usg_id, model, input_tokens, output_tokens, recorded_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, id, accountID, record.USGID, record.Model, record.InputTokens, record.OutputTokens, recordedAt)
		if err != nil {
			return inserted, fmt.Errorf("insert usage record: %w", err)
		}
		affected, _ := result.RowsAffected()
		inserted += int(affected)
	}
	return inserted, nil
}

func (store *sqliteStore) List(ctx context.Context, filter ListFilter) ([]StoredRecord, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	fromBound := ""
	toBound := ""
	if !filter.From.IsZero() {
		fromBound = formatRecordedAtBound(filter.From)
	}
	if !filter.To.IsZero() {
		toBound = formatRecordedAtBound(filter.To)
	}
	// Closed interval [from, to] on recorded_at (fixed-ms RFC3339 strings).
	rows, err := store.db.QueryContext(ctx, `
		SELECT id, account_id, usg_id, model, input_tokens, output_tokens, recorded_at
		FROM usage_records
		WHERE (? = '' OR account_id = ?)
		  AND (? = '' OR recorded_at >= ?)
		  AND (? = '' OR recorded_at <= ?)
		ORDER BY recorded_at DESC, usg_id DESC
		LIMIT ? OFFSET ?
	`, filter.AccountID, filter.AccountID, fromBound, fromBound, toBound, toBound, filter.Limit, filter.Offset)
	if err != nil {
		return nil, fmt.Errorf("list usage records: %w", err)
	}
	defer rows.Close()
	out := make([]StoredRecord, 0)
	for rows.Next() {
		var record StoredRecord
		if err := rows.Scan(&record.ID, &record.AccountID, &record.USGID, &record.Model, &record.InputTokens, &record.OutputTokens, &record.RecordedAt); err != nil {
			return nil, fmt.Errorf("scan usage record: %w", err)
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

// recordedAtLayout is fixed-millisecond UTC so string compare matches chronological order
// for OpenCode-style timestamps (e.g. 2026-07-30T12:00:00.000Z).
const recordedAtLayout = "2006-01-02T15:04:05.000Z07:00"

func formatRecordedAtBound(t time.Time) string {
	return t.UTC().Format(recordedAtLayout)
}

func normalizeRecordedAt(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return ts.UTC().Format(recordedAtLayout)
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts.UTC().Format(recordedAtLayout)
	}
	return raw
}

func (store *sqliteStore) GetSyncState(ctx context.Context, accountID string) (SyncState, error) {
	var cursor sql.NullString
	var updatedAt string
	err := store.db.QueryRowContext(ctx, `
		SELECT cursor, updated_at FROM usage_sync_state WHERE account_id = ?
	`, accountID).Scan(&cursor, &updatedAt)
	if err == sql.ErrNoRows {
		return SyncState{AccountID: accountID, DeepestPageFetched: -1}, nil
	}
	if err != nil {
		return SyncState{}, fmt.Errorf("get usage sync state: %w", err)
	}
	state := SyncState{AccountID: accountID, DeepestPageFetched: -1}
	if cursor.Valid && cursor.String != "" {
		if page, err := strconv.Atoi(cursor.String); err == nil {
			state.DeepestPageFetched = page
		}
	}
	if ts, err := time.Parse(time.RFC3339Nano, updatedAt); err == nil {
		state.UpdatedAt = ts
	}
	return state, nil
}

func (store *sqliteStore) SetSyncState(ctx context.Context, state SyncState) error {
	if state.AccountID == "" {
		return fmt.Errorf("usage: account id is required")
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now().UTC()
	}
	cursor := strconv.Itoa(state.DeepestPageFetched)
	_, err := store.db.ExecContext(ctx, `
		INSERT INTO usage_sync_state (account_id, cursor, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(account_id) DO UPDATE SET cursor = excluded.cursor, updated_at = excluded.updated_at
	`, state.AccountID, cursor, state.UpdatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("set usage sync state: %w", err)
	}
	return nil
}

func (store *sqliteStore) AggregateTotals(ctx context.Context, sinceRFC3339 string) (int64, int64, error) {
	var requests, tokens int64
	var err error
	if sinceRFC3339 == "" {
		err = store.db.QueryRowContext(ctx, `
			SELECT COUNT(*), COALESCE(SUM(input_tokens + output_tokens), 0) FROM usage_records
		`).Scan(&requests, &tokens)
	} else {
		err = store.db.QueryRowContext(ctx, `
			SELECT COUNT(*), COALESCE(SUM(input_tokens + output_tokens), 0)
			FROM usage_records WHERE recorded_at >= ?
		`, sinceRFC3339).Scan(&requests, &tokens)
	}
	if err != nil {
		return 0, 0, fmt.Errorf("aggregate usage totals: %w", err)
	}
	return requests, tokens, nil
}

func (store *sqliteStore) AggregateByModel(ctx context.Context, limit int) ([]ModelAggregate, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := store.db.QueryContext(ctx, `
		SELECT model, COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0)
		FROM usage_records GROUP BY model ORDER BY COUNT(*) DESC, model ASC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("aggregate usage by model: %w", err)
	}
	defer rows.Close()
	out := make([]ModelAggregate, 0)
	for rows.Next() {
		var item ModelAggregate
		if err := rows.Scan(&item.Model, &item.Requests, &item.InputTokens, &item.OutputTokens); err != nil {
			return nil, fmt.Errorf("scan model aggregate: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func newRowID() (string, error) {
	id, err := idgen.Prefixed("ur_", 12)
	if err != nil {
		return "", fmt.Errorf("generate usage row id: %w", err)
	}
	return id, nil
}
