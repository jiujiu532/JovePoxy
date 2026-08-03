package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrMigrationState = errors.New("invalid schema migration state")

type migration struct {
	version int
	sql     string
}

var migrations = []migration{
	{version: 1, sql: initialSchema},
	{version: 2, sql: localKeyUsageSchema},
	{version: 3, sql: adminSessionsHashOnlySchema},
	{version: 4, sql: opencodeAccountVisibilitySchema},
	{version: 5, sql: ollamaAccountsSchema},
	{version: 6, sql: egressProxiesSchema},
	{version: 7, sql: zenKeyProviderSchema},
	{version: 8, sql: zenKeyPrefixSchema},
	{version: 9, sql: requestLogMetaSchema},
	{version: 10, sql: requestLogUsageSchema},
	{version: 11, sql: requestLogTTFTSchema},
}

// Migrate records and applies each pending schema migration transactionally.
func Migrate(ctx context.Context, database *sql.DB) error {
	if _, err := database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		return fmt.Errorf("create schema migration table: %w", err)
	}
	for _, item := range migrations {
		applied, err := applyMigration(ctx, database, item)
		if err != nil {
			return err
		}
		if item.version == 3 && applied {
			if err := purgeLegacySessionResidue(ctx, database); err != nil {
				return err
			}
		}
	}
	return nil
}

func applyMigration(ctx context.Context, database *sql.DB, item migration) (bool, error) {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin migration %d: %w", item.version, err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return
		}
	}()
	var version int
	err = tx.QueryRowContext(ctx, "SELECT version FROM schema_migrations WHERE version = ?", item.version).Scan(&version)
	if err == nil {
		if validationErr := validateRecordedMigration(ctx, tx, item); validationErr != nil {
			return false, validationErr
		}
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("check migration %d: %w", item.version, err)
	}
	if _, err := tx.ExecContext(ctx, item.sql); err != nil {
		return false, fmt.Errorf("apply migration %d: %w", item.version, err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES (?)", item.version); err != nil {
		return false, fmt.Errorf("record migration %d: %w", item.version, err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit migration %d: %w", item.version, err)
	}
	return true, nil
}

func validateRecordedMigration(ctx context.Context, tx *sql.Tx, item migration) error {
	var tables []string
	switch item.version {
	case 1:
		tables = initialSchemaTables
	case 2:
		tables = usageSchemaTables
	case 3:
		tables = []string{"admin_sessions"}
	case 4:
		return validateOpenCodeAccountVisibilityColumns(ctx, tx)
	case 5:
		tables = []string{"ollama_accounts"}
	case 6:
		tables = []string{"egress_proxies"}
	case 7:
		return validateZenKeyProviderColumn(ctx, tx)
	case 8:
		return validateZenKeyPrefixColumn(ctx, tx)
	case 9:
		return validateRequestLogMetaColumns(ctx, tx)
	case 10:
		return validateRequestLogUsageColumns(ctx, tx)
	case 11:
		return validateRequestLogTTFTColumn(ctx, tx)
	default:
		return nil
	}
	for _, table := range tables {
		var name string
		err := tx.QueryRowContext(
			ctx,
			"SELECT name FROM sqlite_master WHERE type = ? AND name = ?",
			"table",
			table,
		).Scan(&name)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("migration %d missing table %s: %w", item.version, table, ErrMigrationState)
		}
		if err != nil {
			return fmt.Errorf("validate migration %d table %s: %w", item.version, table, err)
		}
	}
	if item.version == 3 {
		return validateHashedAdminSessionColumns(ctx, tx)
	}
	return nil
}

func validateZenKeyProviderColumn(ctx context.Context, tx *sql.Tx) error {
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('zen_keys') WHERE name = ?", "provider").Scan(&count); err != nil {
		return fmt.Errorf("validate migration 7 provider column: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("migration 7 missing provider column: %w", ErrMigrationState)
	}
	return nil
}

func validateZenKeyPrefixColumn(ctx context.Context, tx *sql.Tx) error {
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('zen_keys') WHERE name = ?", "key_prefix").Scan(&count); err != nil {
		return fmt.Errorf("validate migration 8 key_prefix column: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("migration 8 missing key_prefix column: %w", ErrMigrationState)
	}
	return nil
}

func validateRequestLogMetaColumns(ctx context.Context, tx *sql.Tx) error {
	for _, column := range []string{"max_tokens", "reasoning_effort", "thinking_type", "budget_tokens"} {
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('request_logs') WHERE name = ?", column).Scan(&count); err != nil {
			return fmt.Errorf("validate migration 9 column %s: %w", column, err)
		}
		if count != 1 {
			return fmt.Errorf("migration 9 missing column %s: %w", column, ErrMigrationState)
		}
	}
	return nil
}

func validateRequestLogUsageColumns(ctx context.Context, tx *sql.Tx) error {
	for _, column := range []string{"input_tokens", "output_tokens", "cache_read_tokens", "cache_creation_tokens"} {
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('request_logs') WHERE name = ?", column).Scan(&count); err != nil {
			return fmt.Errorf("validate migration 10 column %s: %w", column, err)
		}
		if count != 1 {
			return fmt.Errorf("migration 10 missing column %s: %w", column, ErrMigrationState)
		}
	}
	return nil
}

func validateRequestLogTTFTColumn(ctx context.Context, tx *sql.Tx) error {
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('request_logs') WHERE name = ?", "ttft_ms").Scan(&count); err != nil {
		return fmt.Errorf("validate migration 11 column ttft_ms: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("migration 11 missing column ttft_ms: %w", ErrMigrationState)
	}
	return nil
}

func validateOpenCodeAccountVisibilityColumns(ctx context.Context, tx *sql.Tx) error {
	for _, column := range opencodeAccountVisibilityColumns {
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('opencode_accounts') WHERE name = ?", column).Scan(&count); err != nil {
			return fmt.Errorf("validate migration 4 column %s: %w", column, err)
		}
		if count != 1 {
			return fmt.Errorf("migration 4 missing column %s: %w", column, ErrMigrationState)
		}
	}
	return nil
}

func validateHashedAdminSessionColumns(ctx context.Context, tx *sql.Tx) error {
	for _, column := range []string{"token_hash", "revoked_at"} {
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('admin_sessions') WHERE name = ?", column).Scan(&count); err != nil {
			return fmt.Errorf("validate migration 3 column %s: %w", column, err)
		}
		if count != 1 {
			return fmt.Errorf("migration 3 missing column %s: %w", column, ErrMigrationState)
		}
	}
	var legacyTokenColumns int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('admin_sessions') WHERE name = 'token'").Scan(&legacyTokenColumns); err != nil {
		return fmt.Errorf("validate migration 3 legacy token column: %w", err)
	}
	if legacyTokenColumns != 0 {
		return fmt.Errorf("migration 3 retains legacy token column: %w", ErrMigrationState)
	}
	return nil
}

// purgeLegacySessionResidue persists secure deletions and removes WAL frames.
// It cannot guarantee erasure from filesystem snapshots or storage wear-leveling.
func purgeLegacySessionResidue(ctx context.Context, database *sql.DB) error {
	if _, err := database.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("checkpoint legacy session deletion: %w", err)
	}
	if _, err := database.ExecContext(ctx, "VACUUM"); err != nil {
		return fmt.Errorf("vacuum legacy session deletion: %w", err)
	}
	if _, err := database.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("truncate post-vacuum WAL: %w", err)
	}
	return nil
}
