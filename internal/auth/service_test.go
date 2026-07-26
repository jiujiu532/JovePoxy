package auth_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"jovepoxy/internal/auth"
	"jovepoxy/internal/db"
)

func TestService_Login_creates_verifiable_session_when_password_is_correct(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	service, database := newService(t, clock)
	defer database.Close()

	// When
	session, err := service.Login(context.Background(), auth.LoginInput{
		Password: "correct-password",
		Source:   "192.0.2.10",
	})

	// Then
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if session.Token == "" {
		t.Fatal("Login() returned an empty token")
	}
	if !session.ExpiresAt.Equal(clock.Now().Add(auth.DefaultSessionLifetime)) {
		t.Fatalf("Login() expiry = %s, want %s", session.ExpiresAt, clock.Now().Add(auth.DefaultSessionLifetime))
	}
	if err := service.Verify(context.Background(), session.Token); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestService_Login_rejects_wrong_or_missing_password_without_secret_text(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{name: "wrong password", password: "wrong-password"},
		{name: "missing password", password: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			clock := newFakeClock(time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
			service, database := newService(t, clock)
			defer database.Close()

			// When
			_, err := service.Login(context.Background(), auth.LoginInput{
				Password: test.password,
				Source:   "192.0.2.11",
			})

			// Then
			if !errors.Is(err, auth.ErrUnauthorized) {
				t.Fatalf("Login() error = %v, want ErrUnauthorized", err)
			}
			if strings.Contains(err.Error(), "correct-password") || (test.password != "" && strings.Contains(err.Error(), test.password)) {
				t.Fatalf("Login() error leaked a credential: %q", err)
			}
		})
	}
}

func TestService_Verify_rejects_expired_or_malformed_token(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	service, database := newService(t, clock)
	defer database.Close()
	session, err := service.Login(context.Background(), auth.LoginInput{Password: "correct-password", Source: "192.0.2.12"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	clock.Advance(auth.DefaultSessionLifetime + time.Second)

	// When / Then
	if err := service.Verify(context.Background(), session.Token); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("Verify(expired) error = %v, want ErrUnauthorized", err)
	}
	if err := service.Verify(context.Background(), "not-a-session-token"); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("Verify(malformed) error = %v, want ErrUnauthorized", err)
	}
}

func TestService_Logout_revokes_session(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	service, database := newService(t, clock)
	defer database.Close()
	session, err := service.Login(context.Background(), auth.LoginInput{Password: "correct-password", Source: "192.0.2.13"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	// When
	err = service.Logout(context.Background(), session.Token)

	// Then
	if err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if err := service.Verify(context.Background(), session.Token); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("Verify(revoked) error = %v, want ErrUnauthorized", err)
	}
}

func TestService_Authenticate_accepts_parsed_session_credential(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	service, database := newService(t, clock)
	defer database.Close()
	session, err := service.Login(context.Background(), auth.LoginInput{Password: "correct-password", Source: "192.0.2.15"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	// When
	err = service.Authenticate(context.Background(), auth.SessionCredential{Token: session.Token})

	// Then
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
}

func TestService_Login_stores_only_token_hash(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	service, database := newService(t, clock)
	defer database.Close()

	// When
	session, err := service.Login(context.Background(), auth.LoginInput{Password: "correct-password", Source: "192.0.2.14"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	// Then
	var tokenHash string
	if err := database.QueryRowContext(context.Background(), "SELECT token_hash FROM admin_sessions").Scan(&tokenHash); err != nil {
		t.Fatalf("query stored token hash: %v", err)
	}
	if tokenHash == session.Token || strings.Contains(tokenHash, session.Token) {
		t.Fatal("admin_sessions stored the plaintext session token")
	}
}

func newService(t *testing.T, clock *fakeClock) (*auth.Service, *sql.DB) {
	t.Helper()
	database, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	service, err := auth.NewService(auth.Config{
		Database: database,
		Password: "correct-password",
		Clock:    clock,
	})
	if err != nil {
		database.Close()
		t.Fatalf("new auth service: %v", err)
	}
	return service, database
}

type fakeClock struct{ now time.Time }

func newFakeClock(now time.Time) *fakeClock { return &fakeClock{now: now} }

func (clock *fakeClock) Now() time.Time { return clock.now }

func (clock *fakeClock) Advance(duration time.Duration) { clock.now = clock.now.Add(duration) }
