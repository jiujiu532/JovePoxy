package proxypool

import (
	"errors"
	"net/url"
	"time"
)

// ProxyID identifies an egress proxy node.
type ProxyID string

var (
	ErrNoHealthyProxy = errors.New("proxypool: no healthy egress proxy available")
	ErrInvalidInput   = errors.New("proxypool: invalid proxy input")
	ErrNotFound       = errors.New("proxypool: proxy not found")
	ErrInvalidURL     = errors.New("proxypool: URL must be http, https, socks5, or socks5h")
)

const (
	DefaultCooldown = 60 * time.Second
	RateLimitCooldown = 2 * time.Minute
)

// CreateInput adds a proxy node. URL examples:
//   - http://user:pass@host:8080
//   - socks5://host:1080
//   - socks5h://user:pass@host:1080  (remote DNS)
type CreateInput struct {
	Label  string
	URL    string
	Weight int
}

// UpdateInput patches proxy metadata. URL empty keeps the existing encrypted URL.
type UpdateInput struct {
	Label  string
	Weight int
	URL    string
}

// Metadata is the secret-safe list view.
type Metadata struct {
	ID            ProxyID    `json:"id"`
	Label         string     `json:"label"`
	Scheme        string     `json:"scheme"`
	Host          string     `json:"host"`
	Weight        int        `json:"weight"`
	Enabled       bool       `json:"enabled"`
	CooldownUntil *time.Time `json:"cooldown_until,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// Selected is a decrypted proxy chosen for one upstream attempt.
// Label/Host are secret-safe display fields for request logs (no credentials).
type Selected struct {
	ID    ProxyID
	Label string
	Host  string
	URL   *url.URL
}
