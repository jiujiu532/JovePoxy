package proxypool

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func parseProxyURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrInvalidURL
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return nil, ErrInvalidURL
	}
	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, ErrInvalidURL
	}
	// Require host:port for dial reliability.
	if _, _, err := net.SplitHostPort(parsed.Host); err != nil {
		// allow host without port for http(s) default ports only if Host is set
		if scheme == "http" || scheme == "https" {
			if !strings.Contains(parsed.Host, ":") {
				return parsed, nil
			}
		}
		return nil, fmt.Errorf("%w: host must include port for %s", ErrInvalidURL, scheme)
	}
	parsed.Scheme = scheme
	return parsed, nil
}

func displayHost(u *url.URL) string {
	return u.Host
}
