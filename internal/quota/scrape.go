package quota

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultDashboardBase = "https://opencode.ai/workspace"
	defaultUserAgent     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Gecko/20100101 Firefox/148.0"
	maxHTMLBytes         = 4 << 20
)

// AccountQuota is one account's scrape result for the control plane.
type AccountQuota struct {
	AccountID   AccountID `json:"account_id"`
	Name        string    `json:"name"`
	WorkspaceID string    `json:"workspace_id"`
	Success     bool      `json:"success"`
	UpdatedAt   time.Time `json:"updated_at"`
	Windows     []Window  `json:"windows,omitempty"`
	Error       string    `json:"error,omitempty"`
}

// ScrapeTarget pairs a masked account view with a decrypted cookie for fetching.
type ScrapeTarget struct {
	Account    Account
	AuthCookie string
}

// ScraperConfig configures dashboard scraping (control plane only).
type ScraperConfig struct {
	DashboardBase string
	HTTPClient    *http.Client
	Timeout       time.Duration
	Concurrency   int
	Now           func() time.Time
}

// Scraper fetches and parses OpenCode dashboard quota windows.
type Scraper struct {
	dashboardBase *url.URL
	httpClient    *http.Client
	timeout       time.Duration
	concurrency   int
	now           func() time.Time
}

// NewScraper validates configuration for dashboard scraping.
func NewScraper(config ScraperConfig) (*Scraper, error) {
	if config.HTTPClient == nil {
		return nil, errors.New("quota: http client is required")
	}
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}
	if config.Concurrency <= 0 {
		config.Concurrency = 4
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	base := strings.TrimSpace(config.DashboardBase)
	if base == "" {
		base = defaultDashboardBase
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("quota: invalid dashboard base URL")
	}
	return &Scraper{
		dashboardBase: parsed, httpClient: config.HTTPClient, timeout: config.Timeout,
		concurrency: config.Concurrency, now: config.Now,
	}, nil
}

// FetchAll scrapes all targets; one failure does not stop the batch.
func (scraper *Scraper) FetchAll(ctx context.Context, targets []ScrapeTarget) []AccountQuota {
	results := make([]AccountQuota, len(targets))
	if len(targets) == 0 {
		return results
	}
	sem := make(chan struct{}, scraper.concurrency)
	var waitGroup sync.WaitGroup
	for index, target := range targets {
		waitGroup.Add(1)
		go func(index int, target ScrapeTarget) {
			defer waitGroup.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[index] = failedQuota(target, scraper.now().UTC(), ctx.Err().Error())
				return
			}
			results[index] = scraper.FetchAccount(ctx, target)
		}(index, target)
	}
	waitGroup.Wait()
	return results
}

// FetchAccount scrapes one dashboard page and returns a per-account result.
func (scraper *Scraper) FetchAccount(ctx context.Context, target ScrapeTarget) AccountQuota {
	now := scraper.now().UTC()
	cookie, err := normalizeAuthCookie(target.AuthCookie)
	if err != nil {
		return failedQuota(target, now, "invalid auth cookie")
	}
	workspaceID := strings.TrimSpace(target.Account.WorkspaceID)
	if workspaceID == "" {
		return failedQuota(target, now, "workspace_id is required")
	}
	requestContext, cancel := context.WithTimeout(ctx, scraper.timeout)
	defer cancel()
	endpoint := scraper.dashboardBase.JoinPath(workspaceID, "go")
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return failedQuota(target, now, "create dashboard request failed")
	}
	request.Header.Set("Cookie", cookie)
	request.Header.Set("User-Agent", defaultUserAgent)
	request.Header.Set("Accept", "text/html, application/xhtml+xml")
	response, err := scraper.httpClient.Do(request)
	if err != nil {
		if errors.Is(requestContext.Err(), context.DeadlineExceeded) {
			return failedQuota(target, now, "dashboard request timed out")
		}
		if errors.Is(requestContext.Err(), context.Canceled) {
			return failedQuota(target, now, "dashboard request cancelled")
		}
		return failedQuota(target, now, "dashboard request failed")
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		return failedQuota(target, now, fmt.Sprintf("dashboard redirect (HTTP %d)", response.StatusCode))
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return failedQuota(target, now, fmt.Sprintf("authentication failed (HTTP %d)", response.StatusCode))
	}
	if response.StatusCode == http.StatusNotFound {
		return failedQuota(target, now, "workspace not found (HTTP 404)")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return failedQuota(target, now, fmt.Sprintf("dashboard returned HTTP %d", response.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxHTMLBytes))
	if err != nil {
		return failedQuota(target, now, "read dashboard body failed")
	}
	windows := ParseQuotaHTML(string(body), now)
	if len(windows) == 0 {
		return failedQuota(target, now, "unable to parse quota windows from dashboard HTML")
	}
	return AccountQuota{
		AccountID: target.Account.ID, Name: target.Account.Name, WorkspaceID: workspaceID,
		Success: true, UpdatedAt: now, Windows: FilterWindows(windows, target.Account),
	}
}

func failedQuota(target ScrapeTarget, now time.Time, message string) AccountQuota {
	return AccountQuota{
		AccountID: target.Account.ID, Name: target.Account.Name,
		WorkspaceID: target.Account.WorkspaceID, Success: false, UpdatedAt: now, Error: message,
	}
}
