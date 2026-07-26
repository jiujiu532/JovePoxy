package quota

import (
	"context"
	"sync"
	"time"
)

// SnapshotService builds account quota snapshots with a short process cache so
// control-plane overview/quotas handlers do not re-scrape on every request.
type SnapshotService struct {
	accounts *AccountService
	scraper  *Scraper
	ttl      time.Duration
	now      func() time.Time

	mu        sync.Mutex
	cachedAt  time.Time
	cached    []AccountQuota
	cachedErr error
}

// NewSnapshotService constructs a cached quota snapshotter.
func NewSnapshotService(accounts *AccountService, scraper *Scraper, ttl time.Duration) *SnapshotService {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &SnapshotService{accounts: accounts, scraper: scraper, ttl: ttl, now: time.Now}
}

// Snapshot returns enabled-account quotas, using cache when fresh.
func (service *SnapshotService) Snapshot(ctx context.Context) ([]AccountQuota, error) {
	if service == nil || service.accounts == nil || service.scraper == nil {
		return []AccountQuota{}, nil
	}
	now := service.now()
	service.mu.Lock()
	if !service.cachedAt.IsZero() && now.Sub(service.cachedAt) < service.ttl {
		out := append([]AccountQuota(nil), service.cached...)
		err := service.cachedErr
		service.mu.Unlock()
		return out, err
	}
	service.mu.Unlock()

	results, err := service.fetch(ctx)
	service.mu.Lock()
	service.cachedAt = now
	service.cached = append([]AccountQuota(nil), results...)
	service.cachedErr = err
	service.mu.Unlock()
	return results, err
}

// Windows flattens successful scrape windows for analytics cascade.
func (service *SnapshotService) Windows(ctx context.Context) []Window {
	results, err := service.Snapshot(ctx)
	if err != nil {
		return nil
	}
	windows := make([]Window, 0)
	for _, result := range results {
		if result.Success {
			windows = append(windows, result.Windows...)
		}
	}
	return windows
}

func (service *SnapshotService) fetch(ctx context.Context) ([]AccountQuota, error) {
	list, err := service.accounts.List(ctx)
	if err != nil {
		return nil, err
	}
	targets := make([]ScrapeTarget, 0, len(list))
	for _, account := range list {
		if !account.Enabled {
			continue
		}
		credential, err := service.accounts.GetCredential(ctx, account.ID)
		if err != nil {
			continue
		}
		targets = append(targets, ScrapeTarget{Account: account, AuthCookie: credential.AuthCookie})
	}
	return service.scraper.FetchAll(ctx, targets), nil
}
