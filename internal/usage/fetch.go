package usage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"jovepoxy/internal/quota"
)

const (
	defaultUsageServerID = "def39973159c7f0483d8793a822b8dbb10d067e12c65455fcb4608459ba0234f"
	defaultUserAgent     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Gecko/20100101 Firefox/148.0"
	maxResponseBytes     = 4 << 20
)

// FetcherConfig configures the OpenCode usage _server client.
type FetcherConfig struct {
	BaseURL    string
	ServerID   string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// HTTPFetcher loads usage pages over HTTP.
type HTTPFetcher struct {
	baseURL    *url.URL
	serverID   string
	httpClient *http.Client
	timeout    time.Duration
}

// NewHTTPFetcher validates and constructs a usage page fetcher.
func NewHTTPFetcher(config FetcherConfig) (*HTTPFetcher, error) {
	if config.HTTPClient == nil {
		return nil, errors.New("usage: http client is required")
	}
	if config.Timeout <= 0 {
		config.Timeout = 15 * time.Second
	}
	base := strings.TrimSpace(config.BaseURL)
	if base == "" {
		base = "https://opencode.ai"
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("usage: invalid base URL")
	}
	serverID := strings.TrimSpace(config.ServerID)
	if serverID == "" {
		serverID = defaultUsageServerID
	}
	return &HTTPFetcher{baseURL: parsed, serverID: serverID, httpClient: config.HTTPClient, timeout: config.Timeout}, nil
}

// FetchPage requests one usage page and parses the JS payload.
func (fetcher *HTTPFetcher) FetchPage(ctx context.Context, workspaceID, authCookie string, page int) ([]Record, error) {
	cookie, err := quota.NormalizeAuthCookie(authCookie)
	if err != nil {
		return nil, errors.New("usage: auth cookie is required")
	}
	args := []any{workspaceID}
	if page > 0 {
		args = append(args, page)
	}
	encodedArgs, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("encode usage args: %w", err)
	}
	endpoint := fetcher.baseURL.JoinPath("_server")
	query := endpoint.Query()
	query.Set("id", fetcher.serverID)
	query.Set("args", string(encodedArgs))
	endpoint.RawQuery = query.Encode()

	requestContext, cancel := context.WithTimeout(ctx, fetcher.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create usage request: %w", err)
	}
	request.Header.Set("Cookie", cookie)
	request.Header.Set("User-Agent", defaultUserAgent)
	request.Header.Set("X-Server-Id", fetcher.serverID)
	request.Header.Set("Origin", fetcher.baseURL.Scheme+"://"+fetcher.baseURL.Host)
	request.Header.Set("Referer", fetcher.baseURL.String()+"/workspace/"+workspaceID+"/usage")
	request.Header.Set("Accept", "text/javascript, application/json;q=0.9, */*;q=0.8")

	response, err := fetcher.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send usage request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("usage authentication failed (HTTP %d)", response.StatusCode)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("usage request returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read usage response: %w", err)
	}
	return ParseUsageResponse(string(body)), nil
}
