package usage

import (
	"context"
	"fmt"
	"time"
)

// PageFetcher loads one usage page from the OpenCode _server endpoint.
type PageFetcher interface {
	FetchPage(ctx context.Context, workspaceID, authCookie string, page int) ([]Record, error)
}

// SyncResult summarizes an incremental or backfill run.
type SyncResult struct {
	Inserted     int       `json:"inserted"`
	PagesFetched int       `json:"pages_fetched"`
	SyncAt       time.Time `json:"sync_at"`
	Error        string    `json:"error,omitempty"`
}

// Service runs incremental sync and backfill against a store + page fetcher.
type Service struct {
	store   Store
	fetcher PageFetcher
	now     func() time.Time
}

// NewService constructs a usage sync service.
func NewService(store Store, fetcher PageFetcher) *Service {
	return &Service{store: store, fetcher: fetcher, now: time.Now}
}

// SyncIncremental pages from 0 until no new inserts or a short page.
func (service *Service) SyncIncremental(ctx context.Context, accountID, workspaceID, authCookie string, maxPages int) (SyncResult, error) {
	if maxPages <= 0 {
		maxPages = 5
	}
	syncAt := service.now().UTC()
	insertedTotal := 0
	pagesFetched := 0
	deepest := -1
	for page := 0; page < maxPages; page++ {
		records, err := service.fetcher.FetchPage(ctx, workspaceID, authCookie, page)
		if err != nil {
			return SyncResult{Inserted: insertedTotal, PagesFetched: pagesFetched, SyncAt: syncAt, Error: err.Error()}, err
		}
		if len(records) == 0 {
			break
		}
		pagesFetched++
		inserted, err := service.store.InsertIgnore(ctx, accountID, records)
		if err != nil {
			return SyncResult{Inserted: insertedTotal, PagesFetched: pagesFetched, SyncAt: syncAt, Error: err.Error()}, err
		}
		insertedTotal += inserted
		deepest = page
		if inserted == 0 || inserted < len(records) || len(records) < PageSize {
			break
		}
	}
	if err := service.store.SetSyncState(ctx, SyncState{
		AccountID: accountID, DeepestPageFetched: deepest, UpdatedAt: syncAt, LastInsertedCount: insertedTotal, LastStatus: "ok",
	}); err != nil {
		return SyncResult{Inserted: insertedTotal, PagesFetched: pagesFetched, SyncAt: syncAt}, err
	}
	return SyncResult{Inserted: insertedTotal, PagesFetched: pagesFetched, SyncAt: syncAt}, nil
}

// Backfill continues from deepest_page_fetched + 1 for up to maxPages.
func (service *Service) Backfill(ctx context.Context, accountID, workspaceID, authCookie string, maxPages int) (SyncResult, error) {
	if maxPages <= 0 {
		maxPages = 5
	}
	if maxPages > 50 {
		maxPages = 50
	}
	state, err := service.store.GetSyncState(ctx, accountID)
	if err != nil {
		return SyncResult{}, err
	}
	startPage := 0
	if state.DeepestPageFetched >= 0 {
		startPage = state.DeepestPageFetched + 1
	}
	syncAt := service.now().UTC()
	insertedTotal := 0
	pagesFetched := 0
	page := startPage
	for range maxPages {
		records, err := service.fetcher.FetchPage(ctx, workspaceID, authCookie, page)
		if err != nil {
			return SyncResult{Inserted: insertedTotal, PagesFetched: pagesFetched, SyncAt: syncAt, Error: err.Error()}, err
		}
		if len(records) == 0 {
			break
		}
		pagesFetched++
		inserted, err := service.store.InsertIgnore(ctx, accountID, records)
		if err != nil {
			return SyncResult{Inserted: insertedTotal, PagesFetched: pagesFetched, SyncAt: syncAt, Error: err.Error()}, err
		}
		insertedTotal += inserted
		if err := service.store.SetSyncState(ctx, SyncState{
			AccountID: accountID, DeepestPageFetched: page, UpdatedAt: syncAt, LastInsertedCount: insertedTotal, LastStatus: "ok",
		}); err != nil {
			return SyncResult{Inserted: insertedTotal, PagesFetched: pagesFetched, SyncAt: syncAt}, err
		}
		if len(records) < PageSize {
			break
		}
		page++
	}
	return SyncResult{Inserted: insertedTotal, PagesFetched: pagesFetched, SyncAt: syncAt}, nil
}

// List is a thin store passthrough for admin listing later.
func (service *Service) List(ctx context.Context, accountID string, limit, offset int) ([]StoredRecord, error) {
	return service.store.List(ctx, accountID, limit, offset)
}

// Status returns the last sync cursor state.
func (service *Service) Status(ctx context.Context, accountID string) (SyncState, error) {
	if accountID == "" {
		return SyncState{}, fmt.Errorf("usage: account id is required")
	}
	return service.store.GetSyncState(ctx, accountID)
}
