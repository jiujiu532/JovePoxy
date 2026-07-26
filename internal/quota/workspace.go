package quota

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var (
	ErrInvalidWorkspaceResolverConfig = errors.New("invalid OpenCode workspace resolver configuration")
	ErrWorkspaceTimeout               = errors.New("OpenCode workspace test timed out")
)

type WorkspaceResolverConfig struct {
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
}

type WorkspaceTestInput struct {
	WorkspaceID string
	AuthCookie  string
}

type WorkspaceStatusError struct{ StatusCode int }

func (err *WorkspaceStatusError) Error() string {
	return fmt.Sprintf("OpenCode workspace test returned HTTP %d", err.StatusCode)
}

type WorkspaceResolver struct {
	baseURL    *url.URL
	httpClient *http.Client
	timeout    time.Duration
}

func NewWorkspaceResolver(config WorkspaceResolverConfig) (*WorkspaceResolver, error) {
	if config.HTTPClient == nil || config.Timeout <= 0 {
		return nil, ErrInvalidWorkspaceResolverConfig
	}
	baseURL, err := url.Parse(config.BaseURL)
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" || (baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		return nil, ErrInvalidWorkspaceResolverConfig
	}
	return &WorkspaceResolver{baseURL: baseURL, httpClient: config.HTTPClient, timeout: config.Timeout}, nil
}

// Resolve verifies the configured workspace without inspecting dashboard content.
func (resolver *WorkspaceResolver) Resolve(ctx context.Context, rawInput WorkspaceTestInput) (workspaceID string, err error) {
	workspaceID, err = parseWorkspaceID(rawInput.WorkspaceID)
	if err != nil {
		return "", err
	}
	cookie, err := normalizeAuthCookie(rawInput.AuthCookie)
	if err != nil {
		return "", err
	}
	requestContext, cancel := context.WithTimeout(ctx, resolver.timeout)
	defer cancel()
	endpoint := resolver.baseURL.JoinPath("workspace", workspaceID, "go")
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", fmt.Errorf("create OpenCode workspace test request: %w", err)
	}
	request.Header.Set("Cookie", cookie)
	response, err := resolver.httpClient.Do(request)
	if err != nil {
		if errors.Is(requestContext.Err(), context.Canceled) {
			return "", fmt.Errorf("OpenCode workspace test cancelled: %w", requestContext.Err())
		}
		if errors.Is(requestContext.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("%w: %v", ErrWorkspaceTimeout, requestContext.Err())
		}
		return "", fmt.Errorf("send OpenCode workspace test request: %w", err)
	}
	defer func() { err = errors.Join(err, response.Body.Close()) }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", &WorkspaceStatusError{StatusCode: response.StatusCode}
	}
	return workspaceID, nil
}
