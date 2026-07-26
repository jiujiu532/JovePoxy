package db

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const legacyAdminSessionsSchema = `
	CREATE TABLE admin_sessions (
		token TEXT PRIMARY KEY,
		expires_at TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
`

func TestMigrate_replaces_legacy_admin_sessions_with_hash_only_schema_idempotently(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), databaseFile)
	database := openLegacySessionDatabase(t, path, "legacy-token")
	defer database.Close()

	// When
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("migrate legacy schema: %v", err)
	}
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("migrate legacy schema a second time: %v", err)
	}

	// Then
	assertHashedAdminSessionSchema(t, database)
	var storedSessions int
	if err := database.QueryRow("SELECT COUNT(*) FROM admin_sessions").Scan(&storedSessions); err != nil {
		t.Fatalf("count migrated sessions: %v", err)
	}
	if storedSessions != 0 {
		t.Fatalf("migrated sessions = %d, want 0 after security invalidation", storedSessions)
	}
}

func TestMigrate_rejects_recorded_version_three_with_legacy_admin_session_schema(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), databaseFile)
	database := openLegacySessionDatabase(t, path, "stale-token")
	defer database.Close()
	if _, err := database.Exec("INSERT INTO schema_migrations (version) VALUES (3)"); err != nil {
		t.Fatalf("record stale migration three: %v", err)
	}

	// When
	err := Migrate(context.Background(), database)

	// Then
	if !errors.Is(err, ErrMigrationState) {
		t.Fatalf("Migrate() error = %v, want ErrMigrationState", err)
	}
}

func TestOpen_removes_legacy_session_fixture_from_database_and_wal(t *testing.T) {
	// Given
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, databaseFile)
	const legacyTokenFixture = "legacy-session-fixture-do-not-reuse"
	legacy := openLegacySessionDatabase(t, path, legacyTokenFixture)
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	// When
	migrated, err := Open(context.Background(), dataDir)
	if err != nil {
		t.Fatalf("open migrated database: %v", err)
	}
	if err := migrated.Close(); err != nil {
		t.Fatalf("close migrated database: %v", err)
	}

	// Then
	assertFileDoesNotContain(t, path, legacyTokenFixture)
	walPath := path + "-wal"
	if _, err := os.Stat(walPath); errors.Is(err, os.ErrNotExist) {
		t.Log("WAL absent after checkpoint truncate, as expected")
		return
	} else if err != nil {
		t.Fatalf("stat WAL: %v", err)
	}
	assertFileDoesNotContain(t, walPath, legacyTokenFixture)
}

func openLegacySessionDatabase(t *testing.T, path, token string) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	if _, err := database.Exec(initialSchema + localKeyUsageSchema); err != nil {
		database.Close()
		t.Fatalf("create legacy base schema: %v", err)
	}
	if _, err := database.Exec("DROP TABLE admin_sessions;" + legacyAdminSessionsSchema); err != nil {
		database.Close()
		t.Fatalf("replace admin sessions with legacy schema: %v", err)
	}
	if _, err := database.Exec("CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)"); err != nil {
		database.Close()
		t.Fatalf("create migration table: %v", err)
	}
	if _, err := database.Exec("INSERT INTO schema_migrations (version) VALUES (1), (2)"); err != nil {
		database.Close()
		t.Fatalf("record legacy migrations: %v", err)
	}
	if _, err := database.Exec("INSERT INTO admin_sessions (token, expires_at) VALUES (?, ?)", token, "2026-07-16T12:00:00Z"); err != nil {
		database.Close()
		t.Fatalf("seed legacy session: %v", err)
	}
	return database
}

func assertHashedAdminSessionSchema(t *testing.T, database *sql.DB) {
	t.Helper()
	for _, column := range []string{"token_hash", "revoked_at"} {
		var count int
		if err := database.QueryRow("SELECT COUNT(*) FROM pragma_table_info('admin_sessions') WHERE name = ?", column).Scan(&count); err != nil {
			t.Fatalf("count %s column: %v", column, err)
		}
		if count != 1 {
			t.Fatalf("%s columns = %d, want 1", column, count)
		}
	}
	var legacyColumns int
	if err := database.QueryRow("SELECT COUNT(*) FROM pragma_table_info('admin_sessions') WHERE name = 'token'").Scan(&legacyColumns); err != nil {
		t.Fatalf("count legacy columns: %v", err)
	}
	if legacyColumns != 0 {
		t.Fatalf("legacy token columns = %d, want 0", legacyColumns)
	}
}

func assertFileDoesNotContain(t *testing.T, path, fixture string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Base(path), err)
	}
	if bytes.Contains(contents, []byte(fixture)) {
		t.Fatalf("legacy token fixture remains in %s", filepath.Base(path))
	}
}
