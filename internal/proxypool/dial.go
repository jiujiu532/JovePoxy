package proxypool

import (
	"net/http"
	"net/url"
	"time"

	"jovepoxy/internal/proxydial"
)

// NewHTTPClient builds a one-shot HTTP client that egresses through proxyURL.
func NewHTTPClient(proxyURL *url.URL, timeout time.Duration) (*http.Client, error) {
	return proxydial.NewHTTPClient(proxyURL, timeout)
}
