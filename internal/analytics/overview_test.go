package analytics_test

import (
	"context"
	"testing"

	"jovepoxy/internal/analytics"
	"jovepoxy/internal/db"
	"jovepoxy/internal/usage"
)

func TestOverview_empty_store_returns_zeros(t *testing.T) {
	database, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	service := analytics.NewService(usage.NewSQLiteStore(database))

	overview, err := service.Overview(context.Background(), nil)
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	if overview.RequestsToday != 0 || overview.TokensTotal != 0 || len(overview.ByModel) != 0 {
		t.Fatalf("overview = %+v", overview)
	}
}

func TestOverview_aggregates_seeded_usage(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if _, err := database.ExecContext(ctx, `
		INSERT INTO opencode_accounts (id, name, workspace_id, auth_cookie_ciphertext, enabled)
		VALUES ('acct_1', 'a', 'wrk_1', 'c', 1)
	`); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO usage_records (id, account_id, usg_id, model, input_tokens, output_tokens, recorded_at)
		VALUES
		('1', 'acct_1', 'usg_1', 'm1', 10, 5, '2099-01-01T10:00:00Z'),
		('2', 'acct_1', 'usg_2', 'm1', 20, 5, '2099-01-01T11:00:00Z'),
		('3', 'acct_1', 'usg_3', 'm2', 1, 1, '2099-01-01T12:00:00Z')
	`); err != nil {
		t.Fatalf("seed usage: %v", err)
	}
	service := analytics.NewService(usage.NewSQLiteStore(database))

	overview, err := service.Overview(ctx, nil)
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	if overview.RequestsTotal != 3 || overview.TokensTotal != 42 {
		t.Fatalf("totals = %+v", overview)
	}
	if len(overview.ByModel) != 2 {
		t.Fatalf("by_model = %+v", overview.ByModel)
	}
}
