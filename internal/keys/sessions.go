package keys

import (
	"sync"
	"time"

	"jovepoxy/internal/idgen"
)

// SessionStore rotates outbound Zen-compatible session IDs per local API key.
// Used by the data plane header builder, not the admin UI session cookie.
type SessionStore struct {
	clock    Clock
	mu       sync.Mutex
	sessions map[KeyID]sessionEntry
}

type sessionEntry struct {
	id        string
	createdAt time.Time
}

// NewSessionStore creates an in-memory session rotator with a 30-minute TTL.
func NewSessionStore(clock Clock) *SessionStore {
	if clock == nil {
		clock = systemClock{}
	}
	return &SessionStore{clock: clock, sessions: make(map[KeyID]sessionEntry)}
}

// SessionFor returns a stable ses_ ID for thirty minutes, then rotates.
func (store *SessionStore) SessionFor(id KeyID) (string, error) {
	if id == "" {
		return "", ErrInvalidInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	now := store.clock.Now().UTC()
	if entry, ok := store.sessions[id]; ok && now.Sub(entry.createdAt) < 30*time.Minute {
		return entry.id, nil
	}
	sessionID, err := newSessionID()
	if err != nil {
		return "", err
	}
	store.sessions[id] = sessionEntry{id: sessionID, createdAt: now}
	return sessionID, nil
}

func newSessionID() (string, error) {
	return idgen.Prefixed("ses_", 12)
}
