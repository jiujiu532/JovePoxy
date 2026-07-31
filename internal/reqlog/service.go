package reqlog

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Entry is one observed data-plane request without prompt/response bodies.
type Entry struct {
	ID         string
	KeyID      string
	Model      string
	Route      string
	Status     int
	LatencyMS  int64
	Stream     bool
	ErrorClass string
	CreatedAt  time.Time
}

// Snapshot is the in-memory counter view for dashboards.
type Snapshot struct {
	TotalRequests uint64 `json:"total_requests"`
	Status429     uint64 `json:"status_429"`
	Status5xx     uint64 `json:"status_5xx"`
	Status2xx     uint64 `json:"status_2xx"`
	StreamRequests uint64 `json:"stream_requests"`
}

// Service records request logs asynchronously and keeps lightweight counters.
type Service struct {
	store   Store
	clock   Clock
	ring    []Entry
	ringMu  sync.RWMutex
	ringPos int
	ringCap int

	total   atomic.Uint64
	s429    atomic.Uint64
	s5xx    atomic.Uint64
	s2xx    atomic.Uint64
	streams atomic.Uint64
}

// Clock provides timestamps for log rows.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// NewService constructs a recorder with an in-memory ring (default 256).
func NewService(database *sql.DB, clock Clock) *Service {
	return NewServiceWithStore(NewSQLiteStore(database), clock, 256)
}

// NewServiceWithStore constructs a recorder over a custom store and ring size.
func NewServiceWithStore(store Store, clock Clock, ringCapacity int) *Service {
	if clock == nil {
		clock = systemClock{}
	}
	if ringCapacity <= 0 {
		ringCapacity = 256
	}
	return &Service{store: store, clock: clock, ring: make([]Entry, ringCapacity), ringCap: ringCapacity}
}

// Record updates counters, the ring buffer, and best-effort SQLite persistence.
// Persistence failures never surface to the request path.
func (service *Service) Record(ctx context.Context, entry Entry) {
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = service.clock.Now().UTC()
	}
	if entry.ID == "" {
		entry.ID = newID()
	}
	service.total.Add(1)
	if entry.Stream {
		service.streams.Add(1)
	}
	switch {
	case entry.Status == 429:
		service.s429.Add(1)
	case entry.Status >= 500:
		service.s5xx.Add(1)
	case entry.Status >= 200 && entry.Status < 300:
		service.s2xx.Add(1)
	}
	service.pushRing(entry)
	_ = service.store.Insert(ctx, entry)
}

// Snapshot returns process-local counters.
func (service *Service) Snapshot() Snapshot {
	return Snapshot{
		TotalRequests:  service.total.Load(),
		Status429:      service.s429.Load(),
		Status5xx:      service.s5xx.Load(),
		Status2xx:      service.s2xx.Load(),
		StreamRequests: service.streams.Load(),
	}
}

// Recent returns the newest ring entries (newest first), capped by limit.
func (service *Service) Recent(limit int) []Entry {
	service.ringMu.RLock()
	defer service.ringMu.RUnlock()
	if limit <= 0 || limit > service.ringCap {
		limit = service.ringCap
	}
	out := make([]Entry, 0, limit)
	for i := 0; i < service.ringCap && len(out) < limit; i++ {
		index := (service.ringPos - 1 - i + service.ringCap) % service.ringCap
		entry := service.ring[index]
		if entry.ID == "" {
			continue
		}
		out = append(out, entry)
	}
	return out
}

// ListFilter bounds persisted log listing. Zero From/To means open-ended.
type ListFilter struct {
	From   time.Time // inclusive if set
	To     time.Time // inclusive if set
	Limit  int
	Offset int
}

// List reads persisted logs newest first (no time filter).
func (service *Service) List(ctx context.Context, limit, offset int) ([]Entry, error) {
	return service.ListFiltered(ctx, ListFilter{Limit: limit, Offset: offset})
}

// ListFiltered reads persisted logs with optional created_at bounds.
func (service *Service) ListFiltered(ctx context.Context, filter ListFilter) ([]Entry, error) {
	return service.store.List(ctx, filter)
}

func (service *Service) pushRing(entry Entry) {
	service.ringMu.Lock()
	defer service.ringMu.Unlock()
	service.ring[service.ringPos%service.ringCap] = entry
	service.ringPos++
}

func newID() string {
	raw := make([]byte, 12)
	_, _ = rand.Read(raw)
	return "rl_" + hex.EncodeToString(raw)
}

// Store is the persistence boundary for request logs.
type Store interface {
	Insert(context.Context, Entry) error
	List(context.Context, ListFilter) ([]Entry, error)
}

type sqliteStore struct{ db *sql.DB }

// NewSQLiteStore persists request logs in SQLite.
func NewSQLiteStore(database *sql.DB) Store {
	return &sqliteStore{db: database}
}

// createdAtLayout pads fractional seconds to 9 digits so lexicographic order matches
// chronological order. time.RFC3339Nano strips trailing zeros and breaks string compares
// (e.g. "...00.5Z" < "...00Z").
const createdAtLayout = "2006-01-02T15:04:05.000000000Z07:00"

func formatCreatedAt(t time.Time) string {
	return t.UTC().Format(createdAtLayout)
}

func (store *sqliteStore) Insert(ctx context.Context, entry Entry) error {
	var keyID any
	if entry.KeyID != "" {
		keyID = entry.KeyID
	}
	_, err := store.db.ExecContext(ctx, `
		INSERT INTO request_logs (id, key_id, model, route, status, latency_ms, stream, error_class, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, entry.ID, keyID, entry.Model, entry.Route, entry.Status, entry.LatencyMS, boolToInt(entry.Stream), nullString(entry.ErrorClass), formatCreatedAt(entry.CreatedAt))
	if err != nil {
		return fmt.Errorf("insert request log: %w", err)
	}
	return nil
}

func (store *sqliteStore) List(ctx context.Context, filter ListFilter) ([]Entry, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	fromBound := ""
	toBound := ""
	if !filter.From.IsZero() {
		fromBound = formatCreatedAt(filter.From)
	}
	if !filter.To.IsZero() {
		toBound = formatCreatedAt(filter.To)
	}
	// Closed interval [from, to] on fixed-width RFC3339Nano UTC strings.
	rows, err := store.db.QueryContext(ctx, `
		SELECT id, key_id, model, route, status, latency_ms, stream, error_class, created_at
		FROM request_logs
		WHERE (? = '' OR created_at >= ?)
		  AND (? = '' OR created_at <= ?)
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, fromBound, fromBound, toBound, toBound, filter.Limit, filter.Offset)
	if err != nil {
		return nil, fmt.Errorf("list request logs: %w", err)
	}
	defer rows.Close()
	out := make([]Entry, 0)
	for rows.Next() {
		var entry Entry
		var keyID sql.NullString
		var errorClass sql.NullString
		var stream int
		var created string
		if err := rows.Scan(&entry.ID, &keyID, &entry.Model, &entry.Route, &entry.Status, &entry.LatencyMS, &stream, &errorClass, &created); err != nil {
			return nil, fmt.Errorf("scan request log: %w", err)
		}
		entry.KeyID = keyID.String
		entry.ErrorClass = errorClass.String
		entry.Stream = stream == 1
		if ts, err := time.Parse(time.RFC3339Nano, created); err == nil {
			entry.CreatedAt = ts
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
