package keys

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"jovepoxy/internal/crypto"
	"jovepoxy/internal/idgen"
)

type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

type Service struct {
	store Store
	box   *crypto.Box
	clock Clock
}

type Store interface {
	Create(context.Context, storedKey) error
	VerifyAndConsume(context.Context, string, time.Time) (VerifiedKey, error)
	Revoke(context.Context, KeyID) error
	SetEnabled(context.Context, KeyID, bool) error
	Update(context.Context, KeyID, UpdateInput) error
	List(context.Context) ([]KeyMetadata, error)
	SecretCiphertext(context.Context, KeyID) (string, error)
}

type storedKey struct {
	id         KeyID
	label      string
	prefix     string
	hash       string
	ciphertext string
	rpmLimit   int
	dailyLimit int
}

// NewService builds a local-key service. box encrypts secrets at rest for admin reveal;
// when nil, Create still works (hash only) but Reveal is unavailable.
func NewService(database *sql.DB, box *crypto.Box, clock Clock) *Service {
	return NewServiceWithStore(NewSQLiteStore(database), box, clock)
}

func NewServiceWithStore(store Store, box *crypto.Box, clock Clock) *Service {
	if clock == nil {
		clock = systemClock{}
	}
	return &Service{store: store, box: box, clock: clock}
}

func (s *Service) Create(ctx context.Context, rawInput CreateInput) (Creation, error) {
	input, err := parseCreateInput(rawInput)
	if err != nil {
		return Creation{}, err
	}
	secret, err := randomSecret()
	if err != nil {
		return Creation{}, fmt.Errorf("generate local API key: %w", err)
	}
	id, err := randomID()
	if err != nil {
		return Creation{}, fmt.Errorf("generate local API key identifier: %w", err)
	}
	hash := sha256.Sum256([]byte(secret))
	ciphertext := ""
	if s.box != nil {
		sealed, sealErr := s.box.Seal(secret)
		if sealErr != nil {
			return Creation{}, fmt.Errorf("encrypt local API key: %w", sealErr)
		}
		ciphertext = sealed
	}
	record := storedKey{
		id: id, label: input.Label, prefix: secret[:14], hash: hex.EncodeToString(hash[:]),
		ciphertext: ciphertext, rpmLimit: input.RPMLimit, dailyLimit: input.DailyLimit,
	}
	if err := s.store.Create(ctx, record); err != nil {
		return Creation{}, fmt.Errorf("store local API key: %w", err)
	}
	return Creation{ID: id, Prefix: record.prefix, Secret: secret}, nil
}

// Reveal returns the plaintext secret for an existing key (admin only).
// Legacy hash-only rows and missing ciphertext return ErrSecretUnavailable.
func (s *Service) Reveal(ctx context.Context, id KeyID) (string, error) {
	if id == "" {
		return "", ErrInvalidInput
	}
	if s.box == nil {
		return "", ErrSecretUnavailable
	}
	ciphertext, err := s.store.SecretCiphertext(ctx, id)
	if err != nil {
		return "", err
	}
	if ciphertext == "" {
		return "", ErrSecretUnavailable
	}
	secret, err := s.box.Open(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decrypt local API key: %w", err)
	}
	return secret, nil
}

func (s *Service) Verify(ctx context.Context, input Credentials) (VerifiedKey, error) {
	credential, err := parseCredentials(input)
	if err != nil {
		return VerifiedKey{}, err
	}
	hash := sha256.Sum256([]byte(credential.secret))
	verified, err := s.store.VerifyAndConsume(ctx, hex.EncodeToString(hash[:]), s.clock.Now().UTC())
	if err != nil {
		return VerifiedKey{}, err
	}
	return verified, nil
}

func (s *Service) Revoke(ctx context.Context, id KeyID) error {
	if id == "" {
		return ErrInvalidInput
	}
	if err := s.store.Revoke(ctx, id); err != nil {
		return fmt.Errorf("revoke local API key: %w", err)
	}
	return nil
}

func (s *Service) SetEnabled(ctx context.Context, id KeyID, enabled bool) error {
	if id == "" {
		return ErrInvalidInput
	}
	if err := s.store.SetEnabled(ctx, id, enabled); err != nil {
		return fmt.Errorf("set local API key enabled state: %w", err)
	}
	return nil
}

// Update changes label and rate limits. Does not rotate the secret.
func (s *Service) Update(ctx context.Context, id KeyID, rawInput UpdateInput) error {
	if id == "" {
		return ErrInvalidInput
	}
	input, err := parseUpdateInput(rawInput)
	if err != nil {
		return err
	}
	if err := s.store.Update(ctx, id, input); err != nil {
		return fmt.Errorf("update local API key: %w", err)
	}
	return nil
}

func (s *Service) List(ctx context.Context) ([]KeyMetadata, error) {
	keys, err := s.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list local API keys: %w", err)
	}
	return keys, nil
}

func randomSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", err
	}
	return "sk-oc-" + hex.EncodeToString(bytes), nil
}

func randomID() (KeyID, error) {
	id, err := idgen.Prefixed("key_", 16)
	if err != nil {
		return "", err
	}
	return KeyID(id), nil
}
