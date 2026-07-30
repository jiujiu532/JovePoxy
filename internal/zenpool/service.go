package zenpool

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"jovepoxy/internal/crypto"
)

// Clock provides the service clock for cooldown decisions.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// AcquireOptions configures key selection for a single paid attempt.
type AcquireOptions struct {
	// Provider defaults to ProviderOpenCode (paid OpenCode routing).
	Provider Provider
	// Excluded key IDs already tried in this request (sticky + failover).
	Excluded []KeyID
	// AffinityKey is a hashed conversation key; only used when policy is sticky.
	AffinityKey string
	// Policy empty uses the service LoadPolicy().
	Policy LoadPolicy
}

// Service manages encrypted Zen API keys and weighted healthy selection.
type Service struct {
	store Store
	box   *crypto.Box
	clock Clock
	rr    atomic.Uint64

	loadPolicy  atomic.Value // LoadPolicy
	maxAttempts atomic.Int32

	benchMu sync.Mutex
	benched map[KeyID]time.Time // until
}

// NewService constructs a SQLite-backed pool service.
func NewService(database *sql.DB, box *crypto.Box, clock Clock) *Service {
	return NewServiceWithStore(NewSQLiteStore(database), box, clock)
}

// NewServiceWithStore constructs a pool service over an arbitrary store.
func NewServiceWithStore(store Store, box *crypto.Box, clock Clock) *Service {
	if clock == nil {
		clock = systemClock{}
	}
	service := &Service{
		store:   store,
		box:     box,
		clock:   clock,
		benched: make(map[KeyID]time.Time),
	}
	service.loadPolicy.Store(LoadPolicySpread)
	service.maxAttempts.Store(int32(DefaultMaxAttempts))
	return service
}

// LoadPolicy returns the current selection policy (spread|sticky).
func (service *Service) LoadPolicy() LoadPolicy {
	if service == nil {
		return LoadPolicySpread
	}
	value, _ := service.loadPolicy.Load().(LoadPolicy)
	if value != LoadPolicySticky && value != LoadPolicySpread {
		return LoadPolicySpread
	}
	return value
}

// SetLoadPolicy updates the selection policy. Unknown values fall back to spread.
func (service *Service) SetLoadPolicy(policy LoadPolicy) {
	if service == nil {
		return
	}
	switch policy {
	case LoadPolicySticky:
		service.loadPolicy.Store(LoadPolicySticky)
	default:
		service.loadPolicy.Store(LoadPolicySpread)
	}
}

// MaxAttempts returns how many different keys ProxyPaid may try (clamped 2..4).
func (service *Service) MaxAttempts() int {
	if service == nil {
		return DefaultMaxAttempts
	}
	return clampMaxAttempts(int(service.maxAttempts.Load()))
}

// SetMaxAttempts sets failover attempts; values outside 2..4 are clamped.
func (service *Service) SetMaxAttempts(n int) {
	if service == nil {
		return
	}
	service.maxAttempts.Store(int32(clampMaxAttempts(n)))
}

func clampMaxAttempts(n int) int {
	if n < MinMaxAttempts {
		return MinMaxAttempts
	}
	if n > MaxMaxAttempts {
		return MaxMaxAttempts
	}
	return n
}

// Create encrypts and stores an upstream API key. The secret is never returned again.
func (service *Service) Create(ctx context.Context, input CreateInput) (Metadata, error) {
	label := strings.TrimSpace(input.Label)
	secret := strings.TrimSpace(input.Secret)
	if label == "" {
		return Metadata{}, errors.New("label is required")
	}
	if secret == "" {
		return Metadata{}, errors.New("secret is required")
	}
	provider := input.Provider
	if provider == "" {
		provider = ProviderOpenCode
	}
	if provider != ProviderOpenCode && provider != ProviderOllama {
		return Metadata{}, errors.New("provider must be opencode or ollama")
	}
	weight := input.Weight
	if weight <= 0 {
		weight = 1
	}
	ciphertext, err := service.box.Seal(secret)
	if err != nil {
		return Metadata{}, fmt.Errorf("encrypt zen key: %w", err)
	}
	id, err := newKeyID()
	if err != nil {
		return Metadata{}, err
	}
	now := service.clock.Now().UTC()
	record := storedKey{
		id: id, label: label, ciphertext: ciphertext, weight: weight, enabled: true,
		provider: provider, createdAt: now.Format(time.RFC3339Nano),
	}
	if err := service.store.Insert(ctx, record); err != nil {
		return Metadata{}, err
	}
	return Metadata{
		ID: id, Label: label, Prefix: maskPrefix(secret), Weight: weight,
		Enabled: true, Provider: provider, CreatedAt: now,
	}, nil
}

// List returns secret-free metadata for admin surfaces.
func (service *Service) List(ctx context.Context) ([]Metadata, error) {
	return service.ListByProvider(ctx, "")
}

// ListByProvider returns keys for one pool (empty = all).
func (service *Service) ListByProvider(ctx context.Context, provider Provider) ([]Metadata, error) {
	var records []storedKey
	var err error
	if provider == "" {
		records, err = service.store.List(ctx)
	} else {
		records, err = service.store.ListByProvider(ctx, provider)
	}
	if err != nil {
		return nil, err
	}
	out := make([]Metadata, 0, len(records))
	for _, record := range records {
		meta, err := service.toMetadata(record)
		if err != nil {
			return nil, err
		}
		out = append(out, meta)
	}
	return out, nil
}

// SetEnabled toggles whether a key may be selected.
func (service *Service) SetEnabled(ctx context.Context, id KeyID, enabled bool) error {
	return service.store.SetEnabled(ctx, id, enabled)
}

// Update patches label/weight; non-empty secret replaces the encrypted material.
func (service *Service) Update(ctx context.Context, id KeyID, input UpdateInput) (Metadata, error) {
	label := strings.TrimSpace(input.Label)
	if id == "" || label == "" {
		return Metadata{}, errors.New("label is required")
	}
	weight := input.Weight
	if weight <= 0 {
		weight = 1
	}
	secret := strings.TrimSpace(input.Secret)
	var ciphertext *string
	if secret != "" {
		sealed, err := service.box.Seal(secret)
		if err != nil {
			return Metadata{}, fmt.Errorf("encrypt zen key: %w", err)
		}
		ciphertext = &sealed
	}
	if err := service.store.Update(ctx, id, label, weight, ciphertext); err != nil {
		return Metadata{}, err
	}
	records, err := service.store.List(ctx)
	if err != nil {
		return Metadata{}, err
	}
	for _, record := range records {
		if record.id != id {
			continue
		}
		return service.toMetadata(record)
	}
	return Metadata{}, sql.ErrNoRows
}

// Delete removes a Zen key permanently.
func (service *Service) Delete(ctx context.Context, id KeyID) error {
	return service.store.Delete(ctx, id)
}

// MarkCooldown puts a key into cooldown after an upstream failure.
func (service *Service) MarkCooldown(ctx context.Context, id KeyID, duration time.Duration) error {
	if duration <= 0 {
		duration = DefaultCooldown
	}
	until := service.clock.Now().UTC().Add(duration)
	return service.store.SetCooldown(ctx, id, &until)
}

// MarkBench puts a key into process-memory bench after 401. Auto-expires; never deletes the key.
func (service *Service) MarkBench(id KeyID, duration time.Duration) {
	if service == nil || id == "" {
		return
	}
	if duration <= 0 {
		duration = DefaultBenchDuration
	}
	until := service.clock.Now().UTC().Add(duration)
	service.benchMu.Lock()
	defer service.benchMu.Unlock()
	if service.benched == nil {
		service.benched = make(map[KeyID]time.Time)
	}
	service.benched[id] = until
}

// IsBenched reports whether id is still within its bench window at now.
func (service *Service) IsBenched(id KeyID, now time.Time) bool {
	if service == nil || id == "" {
		return false
	}
	service.benchMu.Lock()
	defer service.benchMu.Unlock()
	until, ok := service.benched[id]
	if !ok {
		return false
	}
	if !now.Before(until) {
		delete(service.benched, id)
		return false
	}
	return true
}

// BenchedSnapshot returns a copy of still-active bench deadlines (and purges expired entries).
func (service *Service) BenchedSnapshot(now time.Time) map[KeyID]time.Time {
	if service == nil {
		return nil
	}
	service.benchMu.Lock()
	defer service.benchMu.Unlock()
	if len(service.benched) == 0 {
		return nil
	}
	out := make(map[KeyID]time.Time, len(service.benched))
	for id, until := range service.benched {
		if now.Before(until) {
			out[id] = until
			continue
		}
		delete(service.benched, id)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Acquire selects the next healthy key using the service load policy (default spread RR).
func (service *Service) Acquire(ctx context.Context) (Selected, error) {
	return service.AcquireFor(ctx, AcquireOptions{})
}

// AcquireExcluding selects the next healthy OpenCode key that is not excluded.
// Paid OpenCode routing only draws from the opencode provider pool.
func (service *Service) AcquireExcluding(ctx context.Context, excluded KeyID) (Selected, error) {
	var list []KeyID
	if excluded != "" {
		list = []KeyID{excluded}
	}
	return service.AcquireFor(ctx, AcquireOptions{Excluded: list})
}

// AcquireFor selects a healthy key under the given options (provider, exclusions, affinity, policy).
func (service *Service) AcquireFor(ctx context.Context, opts AcquireOptions) (Selected, error) {
	provider := opts.Provider
	if provider == "" {
		provider = ProviderOpenCode
	}
	records, err := service.store.ListByProvider(ctx, provider)
	if err != nil {
		return Selected{}, err
	}
	now := service.clock.Now().UTC()
	excluded := make(map[KeyID]struct{}, len(opts.Excluded))
	for _, id := range opts.Excluded {
		if id != "" {
			excluded[id] = struct{}{}
		}
	}
	benched := service.BenchedSnapshot(now)
	candidates := make([]storedKey, 0, len(records))
	totalWeight := 0
	for _, record := range records {
		if _, skip := excluded[record.id]; skip {
			continue
		}
		if !record.enabled || record.weight <= 0 {
			continue
		}
		if benched != nil {
			if until, ok := benched[record.id]; ok && now.Before(until) {
				continue
			}
		}
		if cooling, err := isCooling(record.cooldownUntil, now); err != nil {
			return Selected{}, err
		} else if cooling {
			continue
		}
		candidates = append(candidates, record)
		totalWeight += record.weight
	}
	if totalWeight == 0 || len(candidates) == 0 {
		return Selected{}, ErrNoHealthyKey
	}

	policy := opts.Policy
	if policy == "" {
		policy = service.LoadPolicy()
	}
	var chosen storedKey
	if policy == LoadPolicySticky && strings.TrimSpace(opts.AffinityKey) != "" {
		chosen = weightedRendezvousPick(candidates, opts.AffinityKey)
	} else {
		// spread: weighted round-robin (also sticky fallback when affinity material is missing)
		slot := int(service.rr.Add(1) % uint64(totalWeight))
		cumulative := 0
		for _, candidate := range candidates {
			cumulative += candidate.weight
			if slot < cumulative {
				chosen = candidate
				break
			}
		}
	}
	secret, err := service.box.Open(chosen.ciphertext)
	if err != nil {
		return Selected{}, fmt.Errorf("decrypt zen key: %w", err)
	}
	return Selected{ID: chosen.id, Secret: secret, Label: chosen.label}, nil
}

func (service *Service) toMetadata(record storedKey) (Metadata, error) {
	secret, err := service.box.Open(record.ciphertext)
	if err != nil {
		return Metadata{}, fmt.Errorf("decrypt zen key for prefix: %w", err)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, record.createdAt)
	if err != nil {
		createdAt, _ = time.Parse(time.RFC3339, record.createdAt)
	}
	provider := record.provider
	if provider == "" {
		provider = ProviderOpenCode
	}
	meta := Metadata{
		ID: record.id, Label: record.label, Prefix: maskPrefix(secret),
		Weight: record.weight, Enabled: record.enabled, Provider: provider, CreatedAt: createdAt,
	}
	if record.cooldownUntil.Valid && record.cooldownUntil.String != "" {
		until, err := time.Parse(time.RFC3339Nano, record.cooldownUntil.String)
		if err != nil {
			until, err = time.Parse(time.RFC3339, record.cooldownUntil.String)
		}
		if err == nil {
			meta.CooldownUntil = &until
		}
	}
	return meta, nil
}

func isCooling(value sql.NullString, now time.Time) (bool, error) {
	if !value.Valid || value.String == "" {
		return false, nil
	}
	until, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		until, err = time.Parse(time.RFC3339, value.String)
		if err != nil {
			return false, fmt.Errorf("parse cooldown: %w", err)
		}
	}
	return now.Before(until), nil
}

func maskPrefix(secret string) string {
	if len(secret) <= 4 {
		return secret
	}
	if len(secret) <= 8 {
		return secret[:4] + "…"
	}
	return secret[:6] + "…"
}

func newKeyID() (KeyID, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate zen key id: %w", err)
	}
	return KeyID("zk_" + hex.EncodeToString(raw)), nil
}
