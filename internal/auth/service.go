package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const sessionTokenBytes = 32

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

const passwordSettingKey = "admin_password_sha256"

// Service authenticates one control-plane administrator without HTTP coupling.
type Service struct {
	database     *sql.DB
	passwordHash [sha256.Size]byte
	passwordMu   sync.RWMutex
	clock        Clock
	limiter      loginLimiter
}

// NewService creates an administrator session service from validated configuration.
// If a password hash was previously saved in settings, it overrides the env password.
func NewService(config Config) (*Service, error) {
	if config.Database == nil || strings.TrimSpace(config.Password) == "" {
		return nil, ErrInvalidConfig
	}
	clock := config.Clock
	if clock == nil {
		clock = realClock{}
	}
	service := &Service{
		database:     config.Database,
		passwordHash: sha256.Sum256([]byte(config.Password)),
		clock:        clock,
		limiter:      loginLimiter{attempts: make(map[string][]time.Time)},
	}
	if err := service.loadPasswordOverride(context.Background()); err != nil {
		return nil, err
	}
	return service, nil
}

// Login verifies parsed credentials and issues a random, bounded-lifetime session.
// A successful login clears prior failed attempts from that source.
func (service *Service) Login(ctx context.Context, input LoginInput) (Session, error) {
	now := service.clock.Now().UTC()
	source := normalizedSource(input.Source)
	providedHash := sha256.Sum256([]byte(input.Password))
	service.passwordMu.RLock()
	expected := service.passwordHash
	service.passwordMu.RUnlock()
	if subtle.ConstantTimeCompare(providedHash[:], expected[:]) != 1 {
		if !service.limiter.reserveFailure(source, now) {
			return Session{}, ErrRateLimited
		}
		return Session{}, ErrUnauthorized
	}
	service.limiter.clear(source)
	token, err := newSessionToken()
	if err != nil {
		return Session{}, fmt.Errorf("generate admin session token: %w", err)
	}
	expiresAt := now.Add(DefaultSessionLifetime)
	if _, err := service.database.ExecContext(
		ctx,
		"INSERT INTO admin_sessions (token_hash, expires_at) VALUES (?, ?)",
		tokenHash(token),
		expiresAt.Format(time.RFC3339Nano),
	); err != nil {
		return Session{}, fmt.Errorf("store admin session: %w", err)
	}
	return Session{Token: token, ExpiresAt: expiresAt}, nil
}

// Verify validates a parsed session token for a framework middleware adapter.
func (service *Service) Verify(ctx context.Context, token string) error {
	if !isSessionToken(token) {
		return ErrUnauthorized
	}
	var expiresAt string
	var revokedAt sql.NullString
	err := service.database.QueryRowContext(
		ctx,
		"SELECT expires_at, revoked_at FROM admin_sessions WHERE token_hash = ?",
		tokenHash(token),
	).Scan(&expiresAt, &revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUnauthorized
	}
	if err != nil {
		return fmt.Errorf("load admin session: %w", err)
	}
	expires, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return fmt.Errorf("parse admin session expiry: %w", err)
	}
	if revokedAt.Valid || !service.clock.Now().UTC().Before(expires) {
		return ErrUnauthorized
	}
	return nil
}

// Authenticate is a framework-neutral middleware primitive for parsed credentials.
func (service *Service) Authenticate(ctx context.Context, credential SessionCredential) error {
	return service.Verify(ctx, credential.Token)
}

// ChangePassword verifies the current password, stores a new hash, updates memory,
// and revokes every active admin session so clients must log in again.
func (service *Service) ChangePassword(ctx context.Context, current, next string) error {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	if len(next) < 8 {
		return ErrWeakPassword
	}
	if current == next {
		return errors.New("new password must differ from current password")
	}
	currentHash := sha256.Sum256([]byte(current))
	service.passwordMu.RLock()
	expected := service.passwordHash
	service.passwordMu.RUnlock()
	if subtle.ConstantTimeCompare(currentHash[:], expected[:]) != 1 {
		return ErrUnauthorized
	}
	nextHash := sha256.Sum256([]byte(next))
	hexHash := hex.EncodeToString(nextHash[:])
	if _, err := service.database.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, passwordSettingKey, hexHash); err != nil {
		return fmt.Errorf("persist admin password: %w", err)
	}
	service.passwordMu.Lock()
	service.passwordHash = nextHash
	service.passwordMu.Unlock()
	if err := service.RevokeAllSessions(ctx); err != nil {
		return err
	}
	return nil
}

// RevokeAllSessions invalidates every non-revoked admin session.
func (service *Service) RevokeAllSessions(ctx context.Context) error {
	if _, err := service.database.ExecContext(
		ctx,
		`UPDATE admin_sessions SET revoked_at = ? WHERE revoked_at IS NULL`,
		service.clock.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("revoke all admin sessions: %w", err)
	}
	return nil
}

// PasswordIsCustom reports whether the active password came from settings (not env alone).
func (service *Service) PasswordIsCustom(ctx context.Context) bool {
	var value string
	err := service.database.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, passwordSettingKey).Scan(&value)
	return err == nil && value != ""
}

func (service *Service) loadPasswordOverride(ctx context.Context) error {
	var value string
	err := service.database.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, passwordSettingKey).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load admin password override: %w", err)
	}
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	if err != nil || len(decoded) != sha256.Size {
		return nil
	}
	var hash [sha256.Size]byte
	copy(hash[:], decoded)
	service.passwordHash = hash
	return nil
}

// Logout revokes a parsed session token. Missing or malformed tokens are safe no-ops.
func (service *Service) Logout(ctx context.Context, token string) error {
	if !isSessionToken(token) {
		return nil
	}
	if _, err := service.database.ExecContext(
		ctx,
		"UPDATE admin_sessions SET revoked_at = ? WHERE token_hash = ? AND revoked_at IS NULL",
		service.clock.Now().UTC().Format(time.RFC3339Nano),
		tokenHash(token),
	); err != nil {
		return fmt.Errorf("revoke admin session: %w", err)
	}
	return nil
}

func newSessionToken() (string, error) {
	bytes := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func isSessionToken(token string) bool {
	decoded, err := hex.DecodeString(token)
	return err == nil && len(decoded) == sessionTokenBytes
}

func normalizedSource(source string) string {
	const unknownSource = "unknown"
	trimmed := strings.TrimSpace(source)
	if trimmed == "" || len(trimmed) > 256 {
		return unknownSource
	}
	return trimmed
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func (limiter *loginLimiter) reserveFailure(source string, now time.Time) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	limiter.prune(source, now)
	if len(limiter.attempts[source]) >= DefaultLoginAttemptLimit {
		return false
	}
	limiter.attempts[source] = append(limiter.attempts[source], now)
	return true
}

func (limiter *loginLimiter) clear(source string) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	delete(limiter.attempts, source)
}

func (limiter *loginLimiter) prune(source string, now time.Time) {
	attempts := limiter.attempts[source]
	cutoff := now.Add(-DefaultLoginAttemptWindow)
	firstValid := 0
	for firstValid < len(attempts) && !attempts[firstValid].After(cutoff) {
		firstValid++
	}
	if firstValid == len(attempts) {
		delete(limiter.attempts, source)
		return
	}
	limiter.attempts[source] = attempts[firstValid:]
}
