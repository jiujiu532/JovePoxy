package keys_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"jovepoxy/internal/crypto"
	"jovepoxy/internal/db"
	"jovepoxy/internal/keys"
)

func TestService_creates_hashed_key_and_verifies_bearer_credential(t *testing.T) {
	// Given
	ctx := context.Background()
	service, database := newService(t)
	created, err := service.Create(ctx, keys.CreateInput{Label: "integration"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// When
	verified, err := service.Verify(ctx, keys.Credentials{Authorization: "Bearer " + created.Secret})

	// Then
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if verified.ID != created.ID {
		t.Fatalf("verified ID = %q, want %q", verified.ID, created.ID)
	}
	if !strings.HasPrefix(created.Secret, "sk-oc-") {
		t.Fatalf("created secret has unexpected format")
	}
	var storedHash string
	if err := database.QueryRowContext(ctx, "SELECT key_hash FROM local_api_keys WHERE id = ?", created.ID).Scan(&storedHash); err != nil {
		t.Fatalf("read stored hash: %v", err)
	}
	if storedHash == created.Secret || strings.Contains(storedHash, created.Secret) {
		t.Fatal("stored hash contains plaintext secret")
	}
	t.Logf("manual_qa_key_id=%s key_prefix=%s", created.ID, created.Prefix)
}

func TestService_verifies_x_api_key_and_rejects_ambiguous_or_malformed_credentials(t *testing.T) {
	// Given
	ctx := context.Background()
	service, _ := newService(t)
	created, err := service.Create(ctx, keys.CreateInput{Label: "headers"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// When / Then
	if _, err := service.Verify(ctx, keys.Credentials{APIKey: created.Secret}); err != nil {
		t.Fatalf("Verify(x-api-key) error = %v", err)
	}
	for _, credentials := range []keys.Credentials{
		{Authorization: "Basic ignored"},
		{Authorization: "Bearer"},
		{Authorization: "Bearer "},
		{Authorization: "Bearer " + created.Secret, APIKey: created.Secret},
		{Authorization: "Bearer wrong"},
	} {
		_, err := service.Verify(ctx, credentials)
		if !errors.Is(err, keys.ErrUnauthorized) {
			t.Fatalf("Verify(%+v) error = %v, want ErrUnauthorized", credentials, err)
		}
		if strings.Contains(err.Error(), created.Secret) {
			t.Fatal("credential error contains a secret")
		}
	}
}

func TestService_rejects_disabled_or_revoked_keys(t *testing.T) {
	// Given
	ctx := context.Background()
	service, _ := newService(t)
	disabled, err := service.Create(ctx, keys.CreateInput{Label: "disabled"})
	if err != nil {
		t.Fatalf("Create(disabled) error = %v", err)
	}
	if err := service.SetEnabled(ctx, disabled.ID, false); err != nil {
		t.Fatalf("SetEnabled(false) error = %v", err)
	}
	deleted, err := service.Create(ctx, keys.CreateInput{Label: "deleted"})
	if err != nil {
		t.Fatalf("Create(deleted) error = %v", err)
	}
	if err := service.Revoke(ctx, deleted.ID); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	listed, err := service.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 1 || !containsDisabledKey(listed, disabled.ID) || containsKeyID(listed, deleted.ID) {
		t.Fatalf("List() metadata = %+v, want only disabled key (deleted gone)", listed)
	}

	// When / Then
	for _, secret := range []string{disabled.Secret, deleted.Secret} {
		_, err := service.Verify(ctx, keys.Credentials{APIKey: secret})
		if !errors.Is(err, keys.ErrUnauthorized) {
			t.Fatalf("Verify(disabled or deleted) error = %v, want ErrUnauthorized", err)
		}
	}
}

func TestService_lifecycle_missing_id_returns_ErrNotFound(t *testing.T) {
	// Given
	ctx := context.Background()
	service, _ := newService(t)
	const missing keys.KeyID = "key_does_not_exist_000000000000"

	// When / Then
	if err := service.Revoke(ctx, missing); !errors.Is(err, keys.ErrNotFound) {
		t.Fatalf("Revoke(missing) error = %v, want ErrNotFound", err)
	}
	if err := service.SetEnabled(ctx, missing, false); !errors.Is(err, keys.ErrNotFound) {
		t.Fatalf("SetEnabled(missing) error = %v, want ErrNotFound", err)
	}
	if err := service.Update(ctx, missing, keys.UpdateInput{Label: "ghost"}); !errors.Is(err, keys.ErrNotFound) {
		t.Fatalf("Update(missing) error = %v, want ErrNotFound", err)
	}
}

func TestService_enforces_optional_rpm_and_daily_limits(t *testing.T) {
	// Given
	ctx := context.Background()
	clock := &fakeClock{now: time.Date(2026, time.January, 2, 3, 4, 0, 0, time.UTC)}
	service, _ := newServiceWithClock(t, clock)
	created, err := service.Create(ctx, keys.CreateInput{Label: "limited", RPMLimit: 2, DailyLimit: 3})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// When / Then
	for range 2 {
		if _, err := service.Verify(ctx, keys.Credentials{APIKey: created.Secret}); err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
	}
	if _, err := service.Verify(ctx, keys.Credentials{APIKey: created.Secret}); !errors.Is(err, keys.ErrRateLimited) {
		t.Fatalf("Verify() error = %v, want ErrRateLimited", err)
	}
	clock.Advance(time.Minute)
	if _, err := service.Verify(ctx, keys.Credentials{APIKey: created.Secret}); err != nil {
		t.Fatalf("Verify() after RPM window error = %v", err)
	}
	if _, err := service.Verify(ctx, keys.Credentials{APIKey: created.Secret}); !errors.Is(err, keys.ErrRateLimited) {
		t.Fatalf("Verify() daily limit error = %v, want ErrRateLimited", err)
	}
}

func newService(t *testing.T) (*keys.Service, *sql.DB) {
	t.Helper()
	return newServiceWithClock(t, &fakeClock{now: time.Date(2026, time.January, 2, 3, 4, 0, 0, time.UTC)})
}

func newServiceWithClock(t *testing.T, clock *fakeClock) (*keys.Service, *sql.DB) {
	t.Helper()
	database, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("close database: %v", closeErr)
		}
	})
	box, err := crypto.NewBox("test-admin-secret-32-bytes-minimum!!")
	if err != nil {
		t.Fatalf("NewBox: %v", err)
	}
	return keys.NewService(database, box, clock), database
}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) Advance(duration time.Duration) { c.now = c.now.Add(duration) }

func containsKeyID(metadata []keys.KeyMetadata, id keys.KeyID) bool {
	for _, item := range metadata {
		if item.ID == id {
			return true
		}
	}
	return false
}

func containsDisabledKey(metadata []keys.KeyMetadata, id keys.KeyID) bool {
	for _, item := range metadata {
		if item.ID == id {
			return !item.Enabled && item.Prefix != ""
		}
	}
	return false
}

func TestService_reveals_encrypted_secret(t *testing.T) {
	ctx := context.Background()
	service, database := newService(t)
	created, err := service.Create(ctx, keys.CreateInput{Label: "reveal-me"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	var ciphertext string
	if err := database.QueryRowContext(ctx, "SELECT secret_ciphertext FROM local_api_keys WHERE id = ?", created.ID).Scan(&ciphertext); err != nil {
		t.Fatalf("read ciphertext: %v", err)
	}
	if ciphertext == "" || ciphertext == created.Secret {
		t.Fatalf("ciphertext not sealed: %q", ciphertext)
	}
	secret, err := service.Reveal(ctx, created.ID)
	if err != nil {
		t.Fatalf("Reveal() error = %v", err)
	}
	if secret != created.Secret {
		t.Fatalf("Reveal() = %q, want %q", secret, created.Secret)
	}
}

func TestService_reveal_legacy_hash_only_unavailable(t *testing.T) {
	ctx := context.Background()
	service, database := newService(t)
	created, err := service.Create(ctx, keys.CreateInput{Label: "legacy"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := database.ExecContext(ctx, "UPDATE local_api_keys SET secret_ciphertext = '' WHERE id = ?", created.ID); err != nil {
		t.Fatalf("clear ciphertext: %v", err)
	}
	if _, err := service.Reveal(ctx, created.ID); !errors.Is(err, keys.ErrSecretUnavailable) {
		t.Fatalf("Reveal() error = %v, want ErrSecretUnavailable", err)
	}
}
