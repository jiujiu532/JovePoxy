package usage_test

import (
	"context"
	"testing"

	"jovepoxy/internal/db"
	"jovepoxy/internal/usage"
)

type scriptedFetcher struct {
	pages map[int][]usage.Record
}

func (fetcher scriptedFetcher) FetchPage(_ context.Context, _, _ string, page int) ([]usage.Record, error) {
	return fetcher.pages[page], nil
}

func TestService_sync_incremental_inserts_and_ignores_duplicates(t *testing.T) {
	// Given
	ctx := context.Background()
	database, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	// seed account FK
	if _, err := database.ExecContext(ctx, `
		INSERT INTO opencode_accounts (id, name, workspace_id, auth_cookie_ciphertext, enabled)
		VALUES ('acct_1', 'primary', 'wrk_1', 'cipher', 1)
	`); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	store := usage.NewSQLiteStore(database)
	fetcher := scriptedFetcher{pages: map[int][]usage.Record{
		0: {
			{USGID: "usg_a", CreatedAt: "2026-07-09T08:16:06.000Z", Model: "m1", InputTokens: 1, OutputTokens: 2},
			{USGID: "usg_b", CreatedAt: "2026-07-09T08:17:06.000Z", Model: "m2", InputTokens: 3, OutputTokens: 4},
		},
	}}
	service := usage.NewService(store, fetcher)

	// When
	first, err := service.SyncIncremental(ctx, "acct_1", "wrk_1", "auth=token", 3)
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}
	second, err := service.SyncIncremental(ctx, "acct_1", "wrk_1", "auth=token", 3)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	list, err := service.List(ctx, "acct_1", 10, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	// Then
	if first.Inserted != 2 || second.Inserted != 0 {
		t.Fatalf("inserted first=%d second=%d", first.Inserted, second.Inserted)
	}
	if len(list) != 2 {
		t.Fatalf("list = %+v", list)
	}
	state, err := service.Status(ctx, "acct_1")
	if err != nil || state.DeepestPageFetched != 0 {
		t.Fatalf("state = %+v err=%v", state, err)
	}
}

func TestService_backfill_continues_from_deepest_page(t *testing.T) {
	// Given
	ctx := context.Background()
	database, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if _, err := database.ExecContext(ctx, `
		INSERT INTO opencode_accounts (id, name, workspace_id, auth_cookie_ciphertext, enabled)
		VALUES ('acct_2', 'primary', 'wrk_2', 'cipher', 1)
	`); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	store := usage.NewSQLiteStore(database)
	_ = store.SetSyncState(ctx, usage.SyncState{AccountID: "acct_2", DeepestPageFetched: 0})
	page1 := make([]usage.Record, usage.PageSize)
	for i := range page1 {
		page1[i] = usage.Record{USGID: "usg_p1_" + string(rune('a'+i%26)) + string(rune('0'+i/26)), CreatedAt: "2026-07-09T00:00:00Z", Model: "m"}
	}
	// unique usg ids
	for i := range page1 {
		page1[i].USGID = "usg_page1_" + itoa(i)
	}
	fetcher := scriptedFetcher{pages: map[int][]usage.Record{
		1: page1[:2],
	}}
	service := usage.NewService(store, fetcher)

	// When
	result, err := service.Backfill(ctx, "acct_2", "wrk_2", "auth=token", 3)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	state, err := service.Status(ctx, "acct_2")

	// Then
	if result.Inserted != 2 || result.PagesFetched != 1 {
		t.Fatalf("result = %+v", result)
	}
	if err != nil || state.DeepestPageFetched != 1 {
		t.Fatalf("state = %+v err=%v", state, err)
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
