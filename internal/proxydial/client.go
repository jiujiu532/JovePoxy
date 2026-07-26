// Package proxydial builds per-request HTTP clients for HTTP/SOCKS egress.
package proxydial

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	xproxy "golang.org/x/net/proxy"
)

// ErrInvalidURL is returned for unsupported proxy schemes.
var ErrInvalidURL = errors.New("proxydial: URL must be http, https, socks5, or socks5h")

// NewHTTPClient builds a client that egresses through proxyURL.
func NewHTTPClient(proxyURL *url.URL, timeout time.Duration) (*http.Client, error) {
	if proxyURL == nil {
		return nil, ErrInvalidURL
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	transport, err := newTransport(proxyURL, timeout)
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}

func newTransport(proxyURL *url.URL, timeout time.Duration) (*http.Transport, error) {
	headerTimeout := 30 * time.Second
	if timeout > 0 && timeout < headerTimeout {
		headerTimeout = timeout
	}
	baseDialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: headerTimeout,
		ExpectContinueTimeout: time.Second,
	}

	scheme := strings.ToLower(proxyURL.Scheme)
	switch scheme {
	case "http", "https":
		transport.Proxy = http.ProxyURL(proxyURL)
		transport.DialContext = baseDialer.DialContext
		return transport, nil
	case "socks5", "socks5h":
		var auth *xproxy.Auth
		if proxyURL.User != nil {
			password, _ := proxyURL.User.Password()
			auth = &xproxy.Auth{User: proxyURL.User.Username(), Password: password}
		}
		socksDialer, err := xproxy.SOCKS5("tcp", proxyURL.Host, auth, baseDialer)
		if err != nil {
			return nil, fmt.Errorf("create socks dialer: %w", err)
		}
		contextDialer, ok := socksDialer.(xproxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("socks dialer missing context support")
		}
		if scheme == "socks5h" {
			transport.DialContext = contextDialer.DialContext
			return transport, nil
		}
		transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("no IPs for %s", host)
			}
			return contextDialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		}
		return transport, nil
	default:
		return nil, ErrInvalidURL
	}
}
