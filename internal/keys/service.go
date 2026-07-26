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
)

type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

type Service struct {
	store Store
	clock Clock
}

type Store interface {
	Create(context.Context, storedKey) error
	VerifyAndConsume(context.Context, string, time.Time) (VerifiedKey, error)
	Revoke(context.Context, KeyID) error
	SetEnabled(context.Context, KeyID, bool) error
	Update(context.Context, KeyID, UpdateInput) error
	List(context.Context) ([]KeyMetadata, error)
}

type storedKey struct {
	id         KeyID
	label      string
	prefix     string
	hash       string
	rpmLimit   int
	dailyLimit int
}

func NewService(database *sql.DB, clock Clock) *Service {
	return NewServiceWithStore(NewSQLiteStore(database), clock)
}

func NewServiceWithStore(store Store, clock Clock) *Service {
	if clock == nil {
		clock = systemClock{}
	}
	return &Service{store: store, clock: clock}
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
	record := storedKey{
		id: id, label: input.Label, prefix: secret[:14], hash: hex.EncodeToString(hash[:]),
		rpmLimit: input.RPMLimit, dailyLimit: input.DailyLimit,
	}
	if err := s.store.Create(ctx, record); err != nil {
		return Creation{}, fmt.Errorf("store local API key: %w", err)
	}
	return Creation{ID: id, Prefix: record.prefix, Secret: secret}, nil
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
	bytes := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", err
	}
	return KeyID("key_" + hex.EncodeToString(bytes)), nil
}
