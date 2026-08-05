package zenpool

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"jovepoxy/internal/crypto"
	"jovepoxy/internal/idgen"
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
	// ForAttempt reserves controlled probe state for a real paid dial. Generic callers
	// such as model-catalog refreshes must not consume a probe slot.
	ForAttempt bool
}

// Service manages encrypted Zen API keys and weighted healthy selection.
type Service struct {
	store Store
	box   *crypto.Box
	clock Clock
	rr    atomic.Uint64

	loadPolicy     atomic.Value // LoadPolicy
	maxAttempts    atomic.Int32
	benchDurationN atomic.Int64 // nanoseconds; 401 process-memory bench window

	benchMu sync.Mutex
	benched map[KeyID]time.Time // until

	healthMu      sync.Mutex
	outcomeMu     sync.Mutex
	healthRuntime map[KeyID]*healthRuntime
	// healthDirty holds secret-free health not yet flushed (success throttle).
	// Overlay on List/Acquire so recovery and intermediate successes stay visible.
	healthDirty   map[KeyID]Health
	probeSequence uint64

	// providerWindows tracks short-window dial outcomes per provider for the
	// pool-wide 5xx storm guard (secret-free counts only).
	guardMu         sync.Mutex
	providerWindows map[Provider]*providerOutcomeWindow
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
		store:           store,
		box:             box,
		clock:           clock,
		benched:         make(map[KeyID]time.Time),
		healthRuntime:   make(map[KeyID]*healthRuntime),
		healthDirty:     make(map[KeyID]Health),
		providerWindows: make(map[Provider]*providerOutcomeWindow),
	}
	service.loadPolicy.Store(LoadPolicySpread)
	service.maxAttempts.Store(int32(DefaultMaxAttempts))
	service.benchDurationN.Store(int64(DefaultBenchDuration))
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

// BenchDuration returns the process-memory 401 isolation window.
func (service *Service) BenchDuration() time.Duration {
	if service == nil {
		return DefaultBenchDuration
	}
	n := service.benchDurationN.Load()
	if n <= 0 {
		return DefaultBenchDuration
	}
	return time.Duration(n)
}

// BenchMinutes returns BenchDuration as whole minutes (rounded, at least 1 when non-zero).
func (service *Service) BenchMinutes() int {
	d := service.BenchDuration()
	mins := int(d / time.Minute)
	if d%time.Minute != 0 {
		mins++
	}
	return clampBenchMinutes(mins)
}

// SetBenchDuration sets the 401 isolation window; values outside 1..60 minutes are clamped.
func (service *Service) SetBenchDuration(d time.Duration) {
	if service == nil {
		return
	}
	service.benchDurationN.Store(int64(clampBenchDuration(d)))
}

// SetBenchMinutes sets isolation from whole minutes (1..60).
func (service *Service) SetBenchMinutes(minutes int) {
	service.SetBenchDuration(time.Duration(clampBenchMinutes(minutes)) * time.Minute)
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

func clampBenchMinutes(n int) int {
	if n < MinBenchMinutes {
		return MinBenchMinutes
	}
	if n > MaxBenchMinutes {
		return MaxBenchMinutes
	}
	return n
}

func clampBenchDuration(d time.Duration) time.Duration {
	if d <= 0 {
		return DefaultBenchDuration
	}
	mins := int(d / time.Minute)
	if d%time.Minute != 0 {
		mins++
	}
	return time.Duration(clampBenchMinutes(mins)) * time.Minute
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
	prefix := maskPrefix(secret)
	record := storedKey{
		id: id, label: label, ciphertext: ciphertext, keyPrefix: prefix, weight: weight, enabled: true,
		provider: provider, createdAt: now.Format(time.RFC3339Nano),
	}
	if err := service.store.Insert(ctx, record); err != nil {
		return Metadata{}, err
	}
	return Metadata{
		ID: id, Label: label, Prefix: prefix, Weight: weight,
		Enabled: true, Provider: provider, CreatedAt: now,
		// Cold-start health so create DTO does not show raw zeros before first List join.
		HealthScore:    DefaultHealthScore,
		SelectionScore: SelectionScore(DefaultHealthScore, 0, 0),
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
		out = append(out, service.toMetadata(record))
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
		records, err := service.store.List(ctx)
		if err != nil {
			return Metadata{}, err
		}
		for _, record := range records {
			if record.id == id {
				weight = record.weight
				break
			}
		}
		if weight <= 0 {
			weight = 1
		}
	}
	secret := strings.TrimSpace(input.Secret)
	var ciphertext *string
	var keyPrefix *string
	if secret != "" {
		sealed, err := service.box.Seal(secret)
		if err != nil {
			return Metadata{}, fmt.Errorf("encrypt zen key: %w", err)
		}
		ciphertext = &sealed
		prefix := maskPrefix(secret)
		keyPrefix = &prefix
	}
	if err := service.store.Update(ctx, id, label, weight, ciphertext, keyPrefix); err != nil {
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
		return service.toMetadata(record), nil
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
// duration <= 0 uses the service BenchDuration() setting (default 10m).
func (service *Service) MarkBench(id KeyID, duration time.Duration) {
	if service == nil || id == "" {
		return
	}
	if duration <= 0 {
		duration = service.BenchDuration()
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
// Decrypt failures exclude that row and re-pick; a single bad ciphertext never fails the whole pool.
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
	type candidate struct {
		record storedKey
		score  int
	}
	active := make([]candidate, 0, len(records))
	probes := make([]candidate, 0, len(records))
	for _, record := range records {
		if _, skip := excluded[record.id]; skip || !record.enabled {
			continue
		}
		if until, ok := benched[record.id]; ok && now.Before(until) {
			continue
		}
		cooling, coolErr := isCooling(record.cooldownUntil, now)
		if coolErr != nil || cooling {
			continue
		}
		health := decayHealth(NormalizeHealth(service.overlayDirtyHealth(record.id, record.health)), now)
		requests, inflight, probeBusy := service.runtimeSnapshot(record.id, now)
		score := SelectionScore(health.HealthScore, requests, inflight)
		candidate := candidate{record: record, score: score}
		// A persisted cooldown reason whose deadline has expired means this key must
		// prove one request before returning to normal traffic.
		if health.CooldownReason != "" {
			if opts.ForAttempt && !probeBusy {
				probes = append(probes, candidate)
			}
			continue
		}
		active = append(active, candidate)
	}
	if len(active) == 0 && len(probes) == 0 {
		return Selected{}, ErrNoHealthyKey
	}

	service.healthMu.Lock()
	service.probeSequence++
	probeTurn := len(probes) > 0 && (len(active) == 0 || service.probeSequence%healthProbeEvery == 0)
	service.healthMu.Unlock()
	candidates := active
	probing := false
	if probeTurn {
		candidates = probes
		probing = true
	}

	policy := opts.Policy
	if policy == "" {
		policy = service.LoadPolicy()
	}
	// Keep one key from consuming the entire rolling window when alternatives exist.
	// Sticky sessions retain their healthy binding; the cap applies to spread only.
	if len(candidates) > 1 && !probing && policy != LoadPolicySticky {
		capped := make([]candidate, 0, len(candidates))
		for _, item := range candidates {
			ids := make([]KeyID, 0, len(candidates))
			for _, other := range candidates {
				ids = append(ids, other.record.id)
			}
			if !service.shareCapExceeded(item.record.id, ids, now) {
				capped = append(capped, item)
			}
		}
		if len(capped) > 0 {
			candidates = capped
		}
	}

	decryptSkipped := make(map[KeyID]struct{})
	for len(decryptSkipped) < len(candidates) {
		usable := make([]candidate, 0, len(candidates)-len(decryptSkipped))
		for _, item := range candidates {
			if _, skip := decryptSkipped[item.record.id]; !skip {
				usable = append(usable, item)
			}
		}
		if len(usable) == 0 {
			break
		}
		chosen := usable[0]
		if policy == LoadPolicySticky && strings.TrimSpace(opts.AffinityKey) != "" && !probing {
			// The affinity hash is intentionally unweighted so a health score change
			// cannot migrate an otherwise healthy sticky conversation.
			best := -1.0
			for _, item := range usable {
				value := rendezvousScore(opts.AffinityKey, string(item.record.id), 1)
				if value > best {
					best, chosen = value, item
				}
			}
		} else {
			total := 0
			equalScores := true
			for index, item := range usable {
				total += item.score
				if index > 0 && item.score != usable[0].score {
					equalScores = false
				}
			}
			rr := service.rr.Add(1)
			if equalScores {
				chosen = usable[int(rr%uint64(len(usable)))]
			} else {
				slot := int(rr % uint64(total))
				cumulative := 0
				for _, item := range usable {
					cumulative += item.score
					if slot < cumulative {
						chosen = item
						break
					}
				}
			}
		}
		secret, openErr := service.box.Open(chosen.record.ciphertext)
		if openErr != nil {
			decryptSkipped[chosen.record.id] = struct{}{}
			continue
		}
		return Selected{
			ID:       chosen.record.id,
			Secret:   secret,
			Label:    chosen.record.label,
			Probing:  probing,
			Provider: provider,
		}, nil
	}
	return Selected{}, ErrNoHealthyKey
}

// toMetadata builds secret-free admin metadata without failing List on bad ciphertext.
// Prefers the stored key_prefix column; optional Open is only a best-effort legacy backfill.
func (service *Service) toMetadata(record storedKey) Metadata {
	createdAt, err := time.Parse(time.RFC3339Nano, record.createdAt)
	if err != nil {
		createdAt, _ = time.Parse(time.RFC3339, record.createdAt)
	}
	provider := record.provider
	if provider == "" {
		provider = ProviderOpenCode
	}
	prefix := record.keyPrefix
	// Legacy rows may lack key_prefix; never fail List on decrypt — empty is fine.
	if prefix == "" && record.ciphertext != "" && service != nil && service.box != nil {
		if secret, openErr := service.box.Open(record.ciphertext); openErr == nil {
			prefix = maskPrefix(secret)
		}
	}
	meta := Metadata{
		ID: record.id, Label: record.label, Prefix: prefix,
		Weight: record.weight, Enabled: record.enabled, Provider: provider, CreatedAt: createdAt,
	}
	if record.cooldownUntil.Valid && record.cooldownUntil.String != "" {
		until, parseErr := time.Parse(time.RFC3339Nano, record.cooldownUntil.String)
		if parseErr != nil {
			until, parseErr = time.Parse(time.RFC3339, record.cooldownUntil.String)
		}
		if parseErr == nil {
			meta.CooldownUntil = &until
		}
	}
	health := NormalizeHealth(service.overlayDirtyHealth(record.id, record.health))
	health = decayHealth(health, service.clock.Now().UTC())
	meta.HealthScore = health.HealthScore
	meta.SuccessCount = health.SuccessCount
	meta.FailureCount = health.FailureCount
	meta.ConsecutiveFailures = health.ConsecutiveFailures
	meta.LastErrorClass = health.LastErrorClass
	meta.LastSuccessAt = health.LastSuccessAt
	meta.LastFailureAt = health.LastFailureAt
	meta.CooldownReason = health.CooldownReason
	// NeedsProbe after CooldownUntil is known: reason set and not currently cooling.
	meta.NeedsProbe = health.CooldownReason != "" && (meta.CooldownUntil == nil || !service.clock.Now().UTC().Before(*meta.CooldownUntil))
	if !health.ScoreUpdatedAt.IsZero() {
		updated := health.ScoreUpdatedAt
		meta.HealthUpdatedAt = &updated
	}
	requests, inflight, _ := service.runtimeSnapshot(record.id, service.clock.Now().UTC())
	meta.SelectionScore = SelectionScore(meta.HealthScore, requests, inflight)
	return meta
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
	id, err := idgen.Prefixed("zk_", 16)
	if err != nil {
		return "", fmt.Errorf("generate zen key id: %w", err)
	}
	return KeyID(id), nil
}
