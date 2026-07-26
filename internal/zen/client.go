// Package zen provides the cancellable HTTP boundary for Zen Chat Completions.
package zen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"jovepoxy/internal/config"
)

const chatCompletionsPath = "chat/completions"

var (
	ErrInvalidBaseURL = errors.New("zen: invalid base URL")
	ErrInvalidTimeout = errors.New("zen: upstream timeout must be positive")
)

// Client reuses one tuned HTTP transport for Zen upstream requests.
type Client struct {
	baseURL         *url.URL
	httpClient      *http.Client
	openCodeVersion string
}

// TimeoutError reports a request that exceeded the configured upstream timeout.
type TimeoutError struct {
	cause error
}

func (err *TimeoutError) Error() string { return "zen: upstream request timed out" }

func (err *TimeoutError) Unwrap() error { return err.cause }

// StatusError preserves a non-success upstream status without retaining its
// potentially sensitive response body.
type StatusError struct {
	StatusCode int
}

func (err *StatusError) Error() string {
	return fmt.Sprintf("zen: upstream returned HTTP %d", err.StatusCode)
}

// NewClient creates a client from the already-validated application config.
func NewClient(cfg config.Config) (*Client, error) {
	if cfg.UpstreamTimeout <= 0 {
		return nil, ErrInvalidTimeout
	}
	baseURL, err := url.Parse(cfg.ZenBase)
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, ErrInvalidBaseURL
	}
	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return nil, ErrInvalidBaseURL
	}

	return &Client{
		baseURL:         baseURL,
		httpClient:      &http.Client{Transport: newTransport(cfg), Timeout: cfg.UpstreamTimeout},
		openCodeVersion: cfg.OCVersion,
	}, nil
}

func newTransport(cfg config.Config) *http.Transport {
	headerTimeout := 30 * time.Second
	if cfg.UpstreamTimeout > 0 && cfg.UpstreamTimeout < headerTimeout {
		headerTimeout = cfg.UpstreamTimeout
	}
	return &http.Transport{
		Proxy:                 configuredProxy(cfg.HTTPProxy, cfg.HTTPSProxy),
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: headerTimeout,
		ExpectContinueTimeout: time.Second,
	}
}

func configuredProxy(httpProxy, httpsProxy *url.URL) func(*http.Request) (*url.URL, error) {
	return func(request *http.Request) (*url.URL, error) {
		if request.URL.Scheme == "https" {
			return httpsProxy, nil
		}
		return httpProxy, nil
	}
}

// newClientForProxy is implemented in proxy_client.go to keep SOCKS wiring isolated.

// ChatCompletions sends the raw JSON body unchanged. On success, the caller
// owns response.Body and must close it after streaming or reading completes.
func (client *Client) ChatCompletions(ctx context.Context, auth Auth, body json.RawMessage, stream bool) (*http.Response, error) {
	return client.doChatCompletions(ctx, auth, body, stream, client.httpClient)
}

// ChatCompletionsWithProxy is like ChatCompletions but forces a single egress proxy.
func (client *Client) ChatCompletionsWithProxy(ctx context.Context, auth Auth, body json.RawMessage, stream bool, proxyURL *url.URL) (*http.Response, error) {
	if proxyURL == nil {
		return client.ChatCompletions(ctx, auth, body, stream)
	}
	timeout := client.httpClient.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	httpClient, err := newClientForProxy(proxyURL, timeout)
	if err != nil {
		return nil, err
	}
	return client.doChatCompletions(ctx, auth, body, stream, httpClient)
}

func (client *Client) doChatCompletions(ctx context.Context, auth Auth, body json.RawMessage, stream bool, httpClient *http.Client) (*http.Response, error) {
	headers, err := compatibilityHeaders(auth, client.openCodeVersion)
	if err != nil {
		return nil, fmt.Errorf("build compatibility headers: %w", err)
	}
	endpoint := client.baseURL.JoinPath(chatCompletionsPath)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(bytes.Clone(body)))
	if err != nil {
		return nil, fmt.Errorf("create Zen Chat Completions request: %w", err)
	}
	request.Header = headers
	if stream {
		request.Header.Set("Accept", "text/event-stream")
	} else {
		request.Header.Set("Accept", "application/json")
	}

	response, err := httpClient.Do(request)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, fmt.Errorf("Zen Chat Completions request cancelled: %w", ctx.Err())
		}
		if isTimeoutError(err, ctx) {
			return nil, &TimeoutError{cause: err}
		}
		return nil, fmt.Errorf("send Zen Chat Completions request: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		statusError := &StatusError{StatusCode: response.StatusCode}
		if closeErr := response.Body.Close(); closeErr != nil {
			return nil, errors.Join(statusError, fmt.Errorf("close upstream error response: %w", closeErr))
		}
		return nil, statusError
	}
	return response, nil
}

func isTimeoutError(err error, ctx context.Context) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	// net/http wraps Client.Timeout without always exposing DeadlineExceeded.
	return strings.Contains(err.Error(), "Client.Timeout exceeded")
}
