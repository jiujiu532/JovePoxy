package db

import (
	"context"
	"database/sql"
	"testing"
)

func TestMigrate_adds_zen_key_prefix_column_idempotently(t *testing.T) {
	// Given a schema through migration 7 (provider column present, no key_prefix).
	database, err := sql.Open("sqlite", databaseDSN(t.TempDir()+"/zen_prefix.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("close database: %v", closeErr)
		}
	})
	prior := initialSchema + localKeyUsageSchema + adminSessionsHashOnlySchema +
		opencodeAccountVisibilitySchema + ollamaAccountsSchema + egressProxiesSchema + zenKeyProviderSchema
	if _, err := database.Exec(prior); err != nil {
		t.Fatalf("create version seven schema: %v", err)
	}
	if _, err := database.Exec(`
		CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
		INSERT INTO schema_migrations (version) VALUES (1), (2), (3), (4), (5), (6), (7)
	`); err != nil {
		t.Fatalf("record prior migrations: %v", err)
	}

	// When
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("migrate version eight: %v", err)
	}
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("migrate version eight a second time: %v", err)
	}

	// Then
	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM pragma_table_info('zen_keys') WHERE name = ?", "key_prefix").Scan(&count); err != nil {
		t.Fatalf("count key_prefix column: %v", err)
	}
	if count != 1 {
		t.Fatalf("key_prefix columns = %d, want 1", count)
	}
	var migrations int
	if err := database.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&migrations); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if migrations != 12 {
		t.Fatalf("migration count = %d, want 12", migrations)
	}
}
