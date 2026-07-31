package usage_test

import (
	"context"
	"testing"
	"time"

	"jovepoxy/internal/db"
	"jovepoxy/internal/usage"
)

func TestStore_list_filters_by_recorded_at(t *testing.T) {
	// Given
	ctx := context.Background()
	database, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if _, err := database.ExecContext(ctx, `
		INSERT INTO opencode_accounts (id, name, workspace_id, auth_cookie_ciphertext, enabled)
		VALUES ('acct_r', 'range', 'wrk_r', 'cipher', 1)
	`); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	store := usage.NewSQLiteStore(database)
	if _, err := store.InsertIgnore(ctx, "acct_r", []usage.Record{
		{USGID: "usg_early", CreatedAt: "2026-07-30T10:00:00.000Z", Model: "m1", InputTokens: 1, OutputTokens: 1},
		{USGID: "usg_mid", CreatedAt: "2026-07-30T12:00:00.000Z", Model: "m2", InputTokens: 2, OutputTokens: 2},
		{USGID: "usg_late", CreatedAt: "2026-07-30T14:00:00.000Z", Model: "m3", InputTokens: 3, OutputTokens: 3},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	from := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 30, 14, 0, 0, 0, time.UTC)

	// When: closed [from, to]
	list, err := store.List(ctx, usage.ListFilter{
		AccountID: "acct_r",
		From:      from,
		To:        to,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("list range: %v", err)
	}

	// Then
	if len(list) != 2 {
		t.Fatalf("range len = %d want 2; list=%+v", len(list), list)
	}
	ids := map[string]bool{}
	for _, row := range list {
		ids[row.USGID] = true
	}
	if !ids["usg_mid"] || !ids["usg_late"] || ids["usg_early"] {
		t.Fatalf("unexpected usg ids: %+v", ids)
	}

	// When: only upper bound
	toOnly, err := store.List(ctx, usage.ListFilter{
		AccountID: "acct_r",
		To:        time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC),
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("to-only: %v", err)
	}
	if len(toOnly) != 1 || toOnly[0].USGID != "usg_early" {
		t.Fatalf("to-only = %+v", toOnly)
	}

	// When: unfiltered still returns all
	all, err := store.List(ctx, usage.ListFilter{AccountID: "acct_r", Limit: 10})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("all len = %d", len(all))
	}
}
