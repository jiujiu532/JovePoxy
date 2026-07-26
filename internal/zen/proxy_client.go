package zen

import (
	"net/http"
	"net/url"
	"time"

	"jovepoxy/internal/proxydial"
)

func newClientForProxy(proxyURL *url.URL, timeout time.Duration) (*http.Client, error) {
	return proxydial.NewHTTPClient(proxyURL, timeout)
}
