// Package models maintains the cached, dynamically classified model catalog.
package models

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"jovepoxy/internal/zen"
)

var (
	ErrInvalidSource = errors.New("models: source is required")
	ErrInvalidTTL    = errors.New("models: cache TTL must be positive")
)

// ModelID identifies an upstream model.
type ModelID string

// Provider identifies which upstream product owns a model.
type Provider string

const (
	ProviderOpenCode Provider = "opencode"
	ProviderOllama   Provider = "ollama"
)

// Model is the catalog's classified model value.
type Model struct {
	ID       ModelID
	Free     bool
	Provider Provider // empty/zero treated as opencode
}

// Result exposes the current model snapshot and whether an upstream refresh
// failed after that snapshot was obtained.
type Result struct {
	Models []Model
	Stale  bool
}

// Settings configures cache duration and free-model overrides. Denylist wins.
type Settings struct {
	TTL           time.Duration
	FreeAllowlist []ModelID
	FreeDenylist  []ModelID
	// OpenCodePaid is optional; when non-nil, Refresh merges its models as paid OpenCode (Go suite).
	OpenCodePaid Source
	// Ollama is optional; when non-nil, Refresh merges its models (paid only).
	Ollama Source
}

// Source is the typed boundary that supplies upstream model values.
type Source interface {
	Models(context.Context) ([]zen.Model, error)
}

// RefreshError is returned only when no successful catalog snapshot exists.
type RefreshError struct {
	cause error
}

func (err *RefreshError) Error() string { return "models: refresh failed" }

func (err *RefreshError) Unwrap() error { return err.cause }

// Catalog serializes refreshes and serves immutable copies of its last snapshot.
type Catalog struct {
	source       Source
	openCodePaid Source
	ollama       Source
	ttl          time.Duration
	allow        map[ModelID]struct{}
	deny         map[ModelID]struct{}
	now          func() time.Time

	mu                sync.Mutex
	models            []Model
	fetchedAt         time.Time
	lastRefreshFailed bool
	lastRefreshError  error
	inFlight          chan struct{}
}

// NewCatalog constructs an in-memory catalog. Settings are parsed once here so
// classification code only receives typed model IDs.
// source is the public Zen free-tier catalog (Bearer public).
func NewCatalog(source Source, settings Settings) (*Catalog, error) {
	if source == nil {
		return nil, ErrInvalidSource
	}
	if settings.TTL <= 0 {
		return nil, ErrInvalidTTL
	}
	return &Catalog{
		source:       source,
		openCodePaid: settings.OpenCodePaid,
		ollama:       settings.Ollama,
		ttl:          settings.TTL,
		allow:        toSet(settings.FreeAllowlist),
		deny:         toSet(settings.FreeDenylist),
		now:          time.Now,
	}, nil
}

// List returns the fresh cache when available, otherwise refreshes it.
func (catalog *Catalog) List(ctx context.Context) (Result, error) {
	catalog.mu.Lock()
	fresh := catalog.hasSnapshotLocked() && catalog.now().Before(catalog.fetchedAt.Add(catalog.ttl))
	result := catalog.resultLocked()
	catalog.mu.Unlock()
	if fresh {
		return result, nil
	}
	return catalog.Refresh(ctx)
}

// Refresh forces one shared upstream fetch. Waiting callers retain their own
// cancellation semantics while the caller that started the refresh owns its I/O.
func (catalog *Catalog) Refresh(ctx context.Context) (Result, error) {
	catalog.mu.Lock()
	if done := catalog.inFlight; done != nil {
		catalog.mu.Unlock()
		select {
		case <-done:
			return catalog.currentResult()
		case <-ctx.Done():
			return Result{}, ctx.Err()
		}
	}
	done := make(chan struct{})
	catalog.inFlight = done
	catalog.mu.Unlock()

	merged, partial, refreshErr := catalog.fetchAndMerge(ctx)
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	defer close(done)
	catalog.inFlight = nil
	if refreshErr != nil {
		catalog.lastRefreshFailed = true
		catalog.lastRefreshError = refreshErr
		return catalog.resultOrErrorLocked(refreshErr)
	}
	catalog.models = merged
	catalog.fetchedAt = catalog.now()
	catalog.lastRefreshFailed = partial
	if partial {
		catalog.lastRefreshError = errors.New("models: secondary catalog source failed")
	} else {
		catalog.lastRefreshError = nil
	}
	return catalog.resultLocked(), nil
}

func (catalog *Catalog) fetchAndMerge(ctx context.Context) (merged []Model, partial bool, err error) {
	// Public Zen is free-tier only: keep free models, drop public paid IDs
	// (Claude/Gemini/etc. are not usable with OpenCode Go pool keys).
	upstreamModels, refreshErr := catalog.source.Models(ctx)
	if refreshErr != nil {
		return nil, false, refreshErr
	}
	merged = catalog.classifyOpenCodeFree(upstreamModels)

	if catalog.openCodePaid != nil {
		paidModels, paidErr := catalog.openCodePaid.Models(ctx)
		if paidErr != nil {
			// Keep free snapshot; mark partial so operators can observe the failure.
			partial = true
		} else {
			merged = mergePaidModels(merged, paidModels, ProviderOpenCode)
		}
	}

	if catalog.ollama == nil {
		return merged, partial, nil
	}
	ollamaModels, ollamaErr := catalog.ollama.Models(ctx)
	if ollamaErr != nil {
		return merged, true, nil
	}
	if len(ollamaModels) == 0 {
		return merged, partial, nil
	}
	merged = mergePaidModels(merged, ollamaModels, ProviderOllama)
	return merged, partial, nil
}

func mergePaidModels(merged []Model, upstream []zen.Model, provider Provider) []Model {
	seen := make(map[ModelID]struct{}, len(merged))
	for _, model := range merged {
		seen[model.ID] = struct{}{}
	}
	for _, upstreamModel := range upstream {
		id := ModelID(strings.TrimSpace(upstreamModel.ID))
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		merged = append(merged, Model{ID: id, Free: false, Provider: provider})
		seen[id] = struct{}{}
	}
	return merged
}

func (catalog *Catalog) currentResult() (Result, error) {
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	return catalog.resultOrErrorLocked(nil)
}

func (catalog *Catalog) resultOrErrorLocked(refreshErr error) (Result, error) {
	if catalog.hasSnapshotLocked() {
		return catalog.resultLocked(), nil
	}
	if refreshErr == nil {
		refreshErr = catalog.lastRefreshError
	}
	return Result{}, &RefreshError{cause: refreshErr}
}

func (catalog *Catalog) hasSnapshotLocked() bool {
	return !catalog.fetchedAt.IsZero()
}

func (catalog *Catalog) resultLocked() Result {
	return Result{Models: append([]Model(nil), catalog.models...), Stale: catalog.lastRefreshFailed}
}

// classifyOpenCodeFree keeps only free-tier OpenCode models from the public Zen list.
func (catalog *Catalog) classifyOpenCodeFree(upstreamModels []zen.Model) []Model {
	classified := make([]Model, 0, len(upstreamModels))
	for _, upstreamModel := range upstreamModels {
		id := ModelID(upstreamModel.ID)
		_, allowed := catalog.allow[id]
		_, denied := catalog.deny[id]
		free := allowed || hasFreeDefault(id)
		if denied {
			free = false
		}
		if !free {
			// Public paid IDs are not part of the Go-suite catalog.
			continue
		}
		classified = append(classified, Model{ID: id, Free: true, Provider: ProviderOpenCode})
	}
	return classified
}

func hasFreeDefault(id ModelID) bool {
	return id == "big-pickle" || len(id) > len("-free") && string(id[len(id)-len("-free"):]) == "-free"
}

func toSet(ids []ModelID) map[ModelID]struct{} {
	set := make(map[ModelID]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

// NormalizeProvider clamps unknown/empty providers to OpenCode for routing safety.
// Only ProviderOllama is treated as a distinct paid pool + dialer.
func NormalizeProvider(provider Provider) Provider {
	if provider == ProviderOllama {
		return ProviderOllama
	}
	return ProviderOpenCode
}
