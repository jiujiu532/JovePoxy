package auth

import (
	"database/sql"
	"errors"
	"time"
)

var (
	ErrUnauthorized  = errors.New("admin authentication failed")
	ErrRateLimited   = errors.New("admin authentication temporarily unavailable")
	ErrInvalidConfig = errors.New("invalid admin authentication configuration")
	ErrWeakPassword  = errors.New("new password is too weak")
)

const (
	DefaultSessionLifetime    = 24 * time.Hour
	DefaultLoginAttemptLimit  = 5
	DefaultLoginAttemptWindow = 15 * time.Minute
)

// Clock supplies wall time at the authentication boundary.
type Clock interface {
	Now() time.Time
}

// LoginInput contains credentials already parsed by an HTTP or RPC adapter.
type LoginInput struct {
	Password string
	Source   string
}

// SessionCredential is extracted by a transport adapter before middleware checks.
type SessionCredential struct {
	Token string
}

// Session is returned exactly once after a successful login.
type Session struct {
	Token     string
	ExpiresAt time.Time
}

// Config provides the dependencies for one administrator authentication service.
type Config struct {
	Database *sql.DB
	Password string
	Clock    Clock
}
