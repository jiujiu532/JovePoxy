package db

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func TestOpen_initializes_versioned_schema_idempotently(t *testing.T) {
	// Given
	dataDir := t.TempDir()

	// When
	database, err := Open(context.Background(), dataDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})
	second, err := Open(context.Background(), dataDir)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := second.Close(); closeErr != nil {
			t.Errorf("second Close() error = %v", closeErr)
		}
	})

	// Then
	for _, table := range []string{
		"local_api_keys", "zen_keys", "opencode_accounts", "usage_records",
		"usage_sync_state", "request_logs", "settings", "admin_sessions", "local_key_usage",
	} {
		assertTableExists(t, database, table)
	}
	var migrations int
	if err := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM schema_migrations").Scan(&migrations); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if migrations != 12 {
		t.Fatalf("migration count = %d, want 12", migrations)
	}
	t.Logf("migration_version=12 migration_count=%d", migrations)
}

func TestOpen_configures_pragmas_on_second_connection(t *testing.T) {
	// Given
	database, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})
	database.SetMaxOpenConns(2)
	first, err := database.Conn(context.Background())
	if err != nil {
		t.Fatalf("first connection: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := first.Close(); closeErr != nil {
			t.Errorf("close first connection: %v", closeErr)
		}
	})

	// When
	second, err := database.Conn(context.Background())
	if err != nil {
		t.Fatalf("second connection: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := second.Close(); closeErr != nil {
			t.Errorf("close second connection: %v", closeErr)
		}
	})

	// Then
	assertPragmaValue(t, second, "foreign_keys", 1)
	assertPragmaValue(t, second, "busy_timeout", 5000)
	assertPragmaValue(t, second, "secure_delete", 1)
}

func TestOpen_observes_bounded_lock_timeout_between_handles(t *testing.T) {
	// Given
	dataDir := t.TempDir()
	first, err := Open(context.Background(), dataDir)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := first.Close(); closeErr != nil {
			t.Errorf("close first database: %v", closeErr)
		}
	})
	second, err := Open(context.Background(), dataDir)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := second.Close(); closeErr != nil {
			t.Errorf("close second database: %v", closeErr)
		}
	})
	tx, err := first.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin lock transaction: %v", err)
	}
	t.Cleanup(func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			t.Errorf("rollback lock transaction: %v", rollbackErr)
		}
	})
	if _, err := tx.ExecContext(context.Background(), "INSERT INTO settings (key, value) VALUES (?, ?)", "lock", "held"); err != nil {
		t.Fatalf("lock database: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	// When
	started := time.Now()
	_, err = second.ExecContext(ctx, "INSERT INTO settings (key, value) VALUES (?, ?)", "contended", "value")
	elapsed := time.Since(started)

	// Then
	if err == nil {
		t.Fatal("contended write error = nil")
	}
	if elapsed < 4*time.Second || elapsed >= 6*time.Second {
		t.Fatalf("contended write elapsed = %s, want bounded 5s timeout", elapsed)
	}
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		t.Fatalf("contended write error = %T %v, want *sqlite.Error", err, err)
	}
	primaryCode := sqliteErr.Code() & 0xff
	if primaryCode != sqlite3.SQLITE_BUSY && primaryCode != sqlite3.SQLITE_LOCKED {
		t.Fatalf("contended write SQLite code = %d, want SQLITE_BUSY (%d) or SQLITE_LOCKED (%d)", primaryCode, sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED)
	}
}

func TestMigrate_rejects_recorded_migration_with_missing_schema(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), databaseFile)
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open corrupt migration database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("close corrupt migration database: %v", closeErr)
		}
	})
	if _, err := database.Exec("CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)"); err != nil {
		t.Fatalf("create schema migration table: %v", err)
	}
	if _, err := database.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", 1, "test"); err != nil {
		t.Fatalf("record stale migration: %v", err)
	}

	// When
	err = Migrate(context.Background(), database)

	// Then
	if !errors.Is(err, ErrMigrationState) {
		t.Fatalf("Migrate() error = %v, want ErrMigrationState", err)
	}
}

func assertTableExists(t *testing.T, database *sql.DB, table string) {
	t.Helper()
	var name string
	err := database.QueryRowContext(
		context.Background(),
		"SELECT name FROM sqlite_master WHERE type = ? AND name = ?",
		"table",
		table,
	).Scan(&name)
	if err != nil {
		t.Fatalf("query table %s: %v", table, err)
	}
}

func assertPragmaValue(t *testing.T, connection *sql.Conn, name string, want int) {
	t.Helper()
	var got int
	if err := connection.QueryRowContext(context.Background(), "PRAGMA "+name).Scan(&got); err != nil {
		t.Fatalf("read pragma %s: %v", name, err)
	}
	if got != want {
		t.Fatalf("pragma %s = %d, want %d", name, got, want)
	}
}
