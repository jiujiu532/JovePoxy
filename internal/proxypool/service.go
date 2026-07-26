package proxypool

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"jovepoxy/internal/crypto"
)

// Clock supplies time for cooldown decisions.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Service manages encrypted egress proxy nodes and selection.
type Service struct {
	db    *sql.DB
	box   *crypto.Box
	clock Clock
	rr    atomic.Uint64
}

// NewService constructs a SQLite-backed proxy pool.
func NewService(database *sql.DB, box *crypto.Box, clock Clock) *Service {
	if clock == nil {
		clock = systemClock{}
	}
	return &Service{db: database, box: box, clock: clock}
}

// Create stores an encrypted proxy URL.
func (service *Service) Create(ctx context.Context, input CreateInput) (Metadata, error) {
	label := strings.TrimSpace(input.Label)
	if label == "" {
		return Metadata{}, ErrInvalidInput
	}
	parsed, err := parseProxyURL(input.URL)
	if err != nil {
		return Metadata{}, err
	}
	weight := input.Weight
	if weight <= 0 {
		weight = 1
	}
	ciphertext, err := service.box.Seal(parsed.String())
	if err != nil {
		return Metadata{}, fmt.Errorf("encrypt proxy url: %w", err)
	}
	id, err := newProxyID()
	if err != nil {
		return Metadata{}, err
	}
	now := service.clock.Now().UTC()
	if _, err := service.db.ExecContext(ctx, `
		INSERT INTO egress_proxies (id, label, url_ciphertext, scheme, host, weight, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?)
	`, string(id), label, ciphertext, parsed.Scheme, displayHost(parsed), weight, now.Format(time.RFC3339Nano)); err != nil {
		return Metadata{}, fmt.Errorf("insert proxy: %w", err)
	}
	return Metadata{
		ID: id, Label: label, Scheme: parsed.Scheme, Host: displayHost(parsed),
		Weight: weight, Enabled: true, CreatedAt: now,
	}, nil
}

// List returns masked proxy metadata.
func (service *Service) List(ctx context.Context) ([]Metadata, error) {
	rows, err := service.db.QueryContext(ctx, `
		SELECT id, label, scheme, host, weight, enabled, cooldown_until, created_at
		FROM egress_proxies ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list proxies: %w", err)
	}
	defer rows.Close()
	out := make([]Metadata, 0)
	for rows.Next() {
		var meta Metadata
		var enabled int
		var cooldown sql.NullString
		var created string
		if err := rows.Scan(&meta.ID, &meta.Label, &meta.Scheme, &meta.Host, &meta.Weight, &enabled, &cooldown, &created); err != nil {
			return nil, fmt.Errorf("scan proxy: %w", err)
		}
		meta.Enabled = enabled == 1
		if cooldown.Valid && cooldown.String != "" {
			if until, err := time.Parse(time.RFC3339Nano, cooldown.String); err == nil {
				meta.CooldownUntil = &until
			}
		}
		if ts, err := time.Parse(time.RFC3339Nano, created); err == nil {
			meta.CreatedAt = ts
		}
		out = append(out, meta)
	}
	return out, rows.Err()
}

// SetEnabled toggles a proxy node.
func (service *Service) SetEnabled(ctx context.Context, id ProxyID, enabled bool) error {
	value := 0
	if enabled {
		value = 1
	}
	result, err := service.db.ExecContext(ctx, `UPDATE egress_proxies SET enabled = ? WHERE id = ?`, value, string(id))
	if err != nil {
		return fmt.Errorf("set proxy enabled: %w", err)
	}
	return requireOne(result)
}

// Update patches label/weight; non-empty URL replaces the encrypted proxy endpoint.
func (service *Service) Update(ctx context.Context, id ProxyID, input UpdateInput) (Metadata, error) {
	label := strings.TrimSpace(input.Label)
	if id == "" || label == "" {
		return Metadata{}, ErrInvalidInput
	}
	weight := input.Weight
	if weight <= 0 {
		weight = 1
	}
	rawURL := strings.TrimSpace(input.URL)
	if rawURL != "" {
		parsed, err := parseProxyURL(rawURL)
		if err != nil {
			return Metadata{}, err
		}
		ciphertext, err := service.box.Seal(parsed.String())
		if err != nil {
			return Metadata{}, fmt.Errorf("encrypt proxy url: %w", err)
		}
		result, err := service.db.ExecContext(ctx, `
			UPDATE egress_proxies
			SET label = ?, weight = ?, url_ciphertext = ?, scheme = ?, host = ?
			WHERE id = ?
		`, label, weight, ciphertext, parsed.Scheme, displayHost(parsed), string(id))
		if err != nil {
			return Metadata{}, fmt.Errorf("update proxy: %w", err)
		}
		if err := requireOne(result); err != nil {
			return Metadata{}, err
		}
	} else {
		result, err := service.db.ExecContext(ctx, `
			UPDATE egress_proxies SET label = ?, weight = ? WHERE id = ?
		`, label, weight, string(id))
		if err != nil {
			return Metadata{}, fmt.Errorf("update proxy: %w", err)
		}
		if err := requireOne(result); err != nil {
			return Metadata{}, err
		}
	}
	list, err := service.List(ctx)
	if err != nil {
		return Metadata{}, err
	}
	for _, item := range list {
		if item.ID == id {
			return item, nil
		}
	}
	return Metadata{}, ErrNotFound
}

// Delete removes a proxy node.
func (service *Service) Delete(ctx context.Context, id ProxyID) error {
	result, err := service.db.ExecContext(ctx, `DELETE FROM egress_proxies WHERE id = ?`, string(id))
	if err != nil {
		return fmt.Errorf("delete proxy: %w", err)
	}
	return requireOne(result)
}

// MarkCooldown cools a proxy after failure / rate limit.
func (service *Service) MarkCooldown(ctx context.Context, id ProxyID, duration time.Duration) error {
	if duration <= 0 {
		duration = DefaultCooldown
	}
	until := service.clock.Now().UTC().Add(duration).Format(time.RFC3339Nano)
	result, err := service.db.ExecContext(ctx, `UPDATE egress_proxies SET cooldown_until = ? WHERE id = ?`, until, string(id))
	if err != nil {
		return fmt.Errorf("mark proxy cooldown: %w", err)
	}
	return requireOne(result)
}

// Acquire picks the next healthy proxy (weighted RR).
func (service *Service) Acquire(ctx context.Context) (Selected, error) {
	return service.AcquireExcluding(ctx, "")
}

// AcquireExcluding picks a healthy proxy other than excluded.
func (service *Service) AcquireExcluding(ctx context.Context, excluded ProxyID) (Selected, error) {
	rows, err := service.db.QueryContext(ctx, `
		SELECT id, url_ciphertext, weight, enabled, cooldown_until
		FROM egress_proxies ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return Selected{}, fmt.Errorf("list proxies for acquire: %w", err)
	}
	defer rows.Close()
	type candidate struct {
		id         ProxyID
		ciphertext string
		weight     int
	}
	now := service.clock.Now().UTC()
	candidates := make([]candidate, 0)
	totalWeight := 0
	for rows.Next() {
		var id ProxyID
		var ciphertext string
		var weight, enabled int
		var cooldown sql.NullString
		if err := rows.Scan(&id, &ciphertext, &weight, &enabled, &cooldown); err != nil {
			return Selected{}, err
		}
		if id == excluded || enabled != 1 || weight <= 0 {
			continue
		}
		if cooldown.Valid && cooldown.String != "" {
			until, err := time.Parse(time.RFC3339Nano, cooldown.String)
			if err == nil && now.Before(until) {
				continue
			}
		}
		candidates = append(candidates, candidate{id: id, ciphertext: ciphertext, weight: weight})
		totalWeight += weight
	}
	if totalWeight == 0 {
		return Selected{}, ErrNoHealthyProxy
	}
	slot := int(service.rr.Add(1) % uint64(totalWeight))
	cumulative := 0
	var chosen candidate
	for _, item := range candidates {
		cumulative += item.weight
		if slot < cumulative {
			chosen = item
			break
		}
	}
	raw, err := service.box.Open(chosen.ciphertext)
	if err != nil {
		return Selected{}, fmt.Errorf("decrypt proxy url: %w", err)
	}
	parsed, err := parseProxyURL(raw)
	if err != nil {
		return Selected{}, err
	}
	return Selected{ID: chosen.id, URL: parsed}, nil
}

// CountEnabled returns how many proxies exist and are enabled (ignores cooldown).
func (service *Service) CountEnabled(ctx context.Context) (int, error) {
	var count int
	err := service.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM egress_proxies WHERE enabled = 1`).Scan(&count)
	return count, err
}

func requireOne(result sql.Result) error {
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func newProxyID() (ProxyID, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return ProxyID("px_" + hex.EncodeToString(raw)), nil
}

// RedactURL returns scheme://host without credentials for logs/UI host field.
func RedactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

