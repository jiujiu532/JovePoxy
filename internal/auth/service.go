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
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const sessionTokenBytes = 32

// bcryptCost balances login latency with brute-force resistance for admin passwords.
const bcryptCost = 10

// passwordSettingKey is the settings row for the admin password hash.
// Value may be a bcrypt hash ($2a$/$2b$/$2y$) or a legacy hex-encoded SHA-256 digest.
const passwordSettingKey = "admin_password_sha256"

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

// Service authenticates one control-plane administrator without HTTP coupling.
type Service struct {
	database   *sql.DB
	// passwordHash is either a bcrypt hash string or a 64-char hex SHA-256 digest (legacy).
	passwordHash string
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
	hash, err := hashPassword(config.Password)
	if err != nil {
		return nil, fmt.Errorf("hash admin password: %w", err)
	}
	service := &Service{
		database:     config.Database,
		passwordHash: hash,
		clock:        clock,
		limiter:      loginLimiter{attempts: make(map[string][]time.Time)},
	}
	if err := service.loadPasswordOverride(context.Background()); err != nil {
		return nil, err
	}
	return service, nil
}

// Login verifies parsed credentials and issues a random, bounded-lifetime session.
// Rate-limit checks run before password verification so blocked sources skip bcrypt.
// A successful login clears prior failed attempts from that source.
// Legacy SHA-256 hashes are transparently upgraded to bcrypt after a successful match.
func (service *Service) Login(ctx context.Context, input LoginInput) (Session, error) {
	now := service.clock.Now().UTC()
	source := normalizedSource(input.Source)
	// Already at the attempt ceiling: reject before any password work (incl. bcrypt).
	if service.limiter.isBlocked(source, now) {
		return Session{}, ErrRateLimited
	}
	service.passwordMu.RLock()
	expected := service.passwordHash
	service.passwordMu.RUnlock()
	if !verifyPassword(expected, input.Password) {
		// Record the failure; concurrent racers that passed isBlocked may still hit the ceiling here.
		if !service.limiter.reserveFailure(source, now) {
			return Session{}, ErrRateLimited
		}
		return Session{}, ErrUnauthorized
	}
	// Transparent upgrade: re-hash legacy SHA-256 on successful login.
	if !isBcryptHash(expected) {
		if err := service.upgradePasswordHash(ctx, input.Password); err != nil {
			// Login still succeeds; upgrade failure is non-fatal for the session.
			// Next successful login will retry the upgrade.
			_ = err
		}
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

// ChangePassword verifies the current password, stores a new bcrypt hash, updates memory,
// and revokes every active admin session in the same DB transaction.
func (service *Service) ChangePassword(ctx context.Context, current, next string) error {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	if len(next) < 8 {
		return ErrWeakPassword
	}
	if current == next {
		return errors.New("new password must differ from current password")
	}
	service.passwordMu.RLock()
	expected := service.passwordHash
	service.passwordMu.RUnlock()
	if !verifyPassword(expected, current) {
		return ErrUnauthorized
	}
	nextHash, err := hashPassword(next)
	if err != nil {
		return fmt.Errorf("hash new admin password: %w", err)
	}

	tx, err := service.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin password change transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, passwordSettingKey, nextHash); err != nil {
		return fmt.Errorf("persist admin password: %w", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE admin_sessions SET revoked_at = ? WHERE revoked_at IS NULL`,
		service.clock.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("revoke all admin sessions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit password change: %w", err)
	}
	committed = true

	service.passwordMu.Lock()
	service.passwordHash = nextHash
	service.passwordMu.Unlock()
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
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if isBcryptHash(value) {
		service.passwordHash = value
		return nil
	}
	// Legacy hex SHA-256 (64 chars).
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return nil
	}
	service.passwordHash = value
	return nil
}

// upgradePasswordHash rewrites a legacy SHA-256 setting to bcrypt after a successful login.
func (service *Service) upgradePasswordHash(ctx context.Context, password string) error {
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	if _, err := service.database.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, passwordSettingKey, hash); err != nil {
		return fmt.Errorf("upgrade admin password hash: %w", err)
	}
	service.passwordMu.Lock()
	service.passwordHash = hash
	service.passwordMu.Unlock()
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

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func isBcryptHash(hash string) bool {
	return strings.HasPrefix(hash, "$2a$") ||
		strings.HasPrefix(hash, "$2b$") ||
		strings.HasPrefix(hash, "$2y$")
}

// verifyPassword accepts bcrypt hashes or legacy hex-encoded SHA-256 digests.
func verifyPassword(storedHash, password string) bool {
	if isBcryptHash(storedHash) {
		return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) == nil
	}
	// Legacy: hex SHA-256 of the password.
	provided := sha256.Sum256([]byte(password))
	expected, err := hex.DecodeString(strings.TrimSpace(storedHash))
	if err != nil || len(expected) != sha256.Size {
		return false
	}
	return subtle.ConstantTimeCompare(provided[:], expected) == 1
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

// normalizedSource turns a login source (typically request.RemoteAddr) into a
// stable rate-limit key: host/IP only, without the ephemeral client port.
// Bare IPs, hostnames, and IPv6 forms are accepted; X-Forwarded-For is never read here.
func normalizedSource(source string) string {
	const unknownSource = "unknown"
	trimmed := strings.TrimSpace(source)
	if trimmed == "" || len(trimmed) > 256 {
		return unknownSource
	}
	// RemoteAddr is usually "ip:port" or "[ipv6]:port"; strip port so concurrent
	// connections from the same client share one attempt bucket.
	if host, _, err := net.SplitHostPort(trimmed); err == nil {
		host = strings.TrimSpace(host)
		if host == "" || len(host) > 256 {
			return unknownSource
		}
		return host
	}
	return trimmed
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

// isBlocked reports whether source already has DefaultLoginAttemptLimit failures
// inside the current window. Callers use this before password verification.
func (limiter *loginLimiter) isBlocked(source string, now time.Time) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	limiter.prune(source, now)
	return len(limiter.attempts[source]) >= DefaultLoginAttemptLimit
}

// reserveFailure records one failed attempt when the source is still under the limit.
// Returns false when the source is already at the ceiling (no additional record).
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
