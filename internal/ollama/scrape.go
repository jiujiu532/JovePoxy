package ollama

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	defaultSettingsURL = "https://ollama.com/settings"
	defaultUserAgent   = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	maxHTMLBytes       = 4 << 20
)

// AccountQuota is one account scrape result.
type AccountQuota struct {
	AccountID AccountID `json:"account_id"`
	Name      string    `json:"name"`
	Success   bool      `json:"success"`
	UpdatedAt time.Time `json:"updated_at"`
	Plan      string    `json:"plan,omitempty"`
	Windows   []Window  `json:"windows,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// Scraper fetches ollama.com settings pages.
type Scraper struct {
	httpClient  *http.Client
	settingsURL string
	timeout     time.Duration
	concurrency int
	now         func() time.Time
}

// ScraperConfig configures the scraper.
type ScraperConfig struct {
	HTTPClient  *http.Client
	SettingsURL string
	Timeout     time.Duration
	Concurrency int
	Now         func() time.Time
}

// NewScraper constructs an Ollama settings scraper.
func NewScraper(config ScraperConfig) (*Scraper, error) {
	if config.HTTPClient == nil {
		return nil, fmt.Errorf("ollama: http client is required")
	}
	if config.Timeout <= 0 {
		config.Timeout = 20 * time.Second
	}
	if config.Concurrency <= 0 {
		config.Concurrency = 4
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	url := config.SettingsURL
	if url == "" {
		url = defaultSettingsURL
	}
	return &Scraper{
		httpClient: config.HTTPClient, settingsURL: url,
		timeout: config.Timeout, concurrency: config.Concurrency, now: config.Now,
	}, nil
}

// FetchAll scrapes many accounts; one failure does not stop the batch.
func (scraper *Scraper) FetchAll(ctx context.Context, accounts []Account, cookies map[AccountID]string) []AccountQuota {
	results := make([]AccountQuota, len(accounts))
	if len(accounts) == 0 {
		return results
	}
	sem := make(chan struct{}, scraper.concurrency)
	var waitGroup sync.WaitGroup
	for index, account := range accounts {
		waitGroup.Add(1)
		go func(index int, account Account) {
			defer waitGroup.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[index] = AccountQuota{
					AccountID: account.ID, Name: account.Name, Success: false,
					UpdatedAt: scraper.now().UTC(), Error: ctx.Err().Error(),
				}
				return
			}
			results[index] = scraper.FetchAccount(ctx, account, cookies[account.ID])
		}(index, account)
	}
	waitGroup.Wait()
	return results
}

// FetchAccount scrapes one settings page.
func (scraper *Scraper) FetchAccount(ctx context.Context, account Account, sessionCookie string) AccountQuota {
	now := scraper.now().UTC()
	cookie, err := NormalizeSessionCookie(sessionCookie)
	if err != nil {
		return AccountQuota{AccountID: account.ID, Name: account.Name, Success: false, UpdatedAt: now, Error: "invalid session cookie"}
	}
	requestContext, cancel := context.WithTimeout(ctx, scraper.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, scraper.settingsURL, nil)
	if err != nil {
		return AccountQuota{AccountID: account.ID, Name: account.Name, Success: false, UpdatedAt: now, Error: "create request failed"}
	}
	request.Header.Set("Cookie", cookie)
	request.Header.Set("User-Agent", defaultUserAgent)
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	response, err := scraper.httpClient.Do(request)
	if err != nil {
		return AccountQuota{AccountID: account.ID, Name: account.Name, Success: false, UpdatedAt: now, Error: "request failed"}
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return AccountQuota{AccountID: account.ID, Name: account.Name, Success: false, UpdatedAt: now, Error: fmt.Sprintf("authentication failed (HTTP %d)", response.StatusCode)}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return AccountQuota{AccountID: account.ID, Name: account.Name, Success: false, UpdatedAt: now, Error: fmt.Sprintf("settings returned HTTP %d", response.StatusCode)}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxHTMLBytes))
	if err != nil {
		return AccountQuota{AccountID: account.ID, Name: account.Name, Success: false, UpdatedAt: now, Error: "read body failed"}
	}
	plan, windows, err := ParseQuotaHTML(string(body), now)
	if err != nil {
		return AccountQuota{AccountID: account.ID, Name: account.Name, Success: false, UpdatedAt: now, Error: err.Error()}
	}
	filtered := make([]Window, 0, len(windows))
	for _, window := range windows {
		if window.Label == LabelSession && !account.ShowSession {
			continue
		}
		if window.Label == LabelWeekly && !account.ShowWeekly {
			continue
		}
		filtered = append(filtered, window)
	}
	return AccountQuota{
		AccountID: account.ID, Name: account.Name, Success: true, UpdatedAt: now, Plan: plan, Windows: filtered,
	}
}
