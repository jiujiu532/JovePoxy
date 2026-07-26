package db

import (
	"context"
	"database/sql"
	"testing"
)

func TestMigrate_adds_opencode_account_visibility_columns_idempotently(t *testing.T) {
	// Given
	database, err := sql.Open("sqlite", databaseDSN(t.TempDir()+"/accounts.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("close database: %v", closeErr)
		}
	})
	if _, err := database.Exec(initialSchema + localKeyUsageSchema + adminSessionsHashOnlySchema); err != nil {
		t.Fatalf("create version three schema: %v", err)
	}
	if _, err := database.Exec("CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP); INSERT INTO schema_migrations (version) VALUES (1), (2), (3)"); err != nil {
		t.Fatalf("record prior migrations: %v", err)
	}

	// When
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("migrate version four: %v", err)
	}
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("migrate version four a second time: %v", err)
	}

	// Then
	for _, column := range []string{"show_rolling", "show_weekly", "show_monthly"} {
		var count int
		if err := database.QueryRow("SELECT COUNT(*) FROM pragma_table_info('opencode_accounts') WHERE name = ?", column).Scan(&count); err != nil {
			t.Fatalf("count %s column: %v", column, err)
		}
		if count != 1 {
			t.Fatalf("%s columns = %d, want 1", column, count)
		}
	}
}
