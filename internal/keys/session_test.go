package keys_test

import (
	"strings"
	"testing"
	"time"

	"jovepoxy/internal/keys"
)

func TestSessionStore_reuses_then_rotates_session_at_thirty_minute_boundary(t *testing.T) {
	// Given
	clock := &fakeClock{now: time.Date(2026, time.January, 2, 3, 4, 0, 0, time.UTC)}
	store := keys.NewSessionStore(clock)

	// When
	first, err := store.SessionFor(keys.KeyID("key-1"))
	if err != nil {
		t.Fatalf("first SessionFor() error = %v", err)
	}
	clock.Advance(29*time.Minute + 59*time.Second)
	reused, err := store.SessionFor(keys.KeyID("key-1"))
	if err != nil {
		t.Fatalf("reused SessionFor() error = %v", err)
	}
	clock.Advance(time.Second)
	rotated, err := store.SessionFor(keys.KeyID("key-1"))
	if err != nil {
		t.Fatalf("rotated SessionFor() error = %v", err)
	}
	other, err := store.SessionFor(keys.KeyID("key-2"))
	if err != nil {
		t.Fatalf("other SessionFor() error = %v", err)
	}

	// Then
	if !strings.HasPrefix(first, "ses_") {
		t.Fatal("session does not have ses_ prefix")
	}
	if reused != first {
		t.Fatal("session was not reused before rotation boundary")
	}
	if rotated == first {
		t.Fatal("session was not rotated at boundary")
	}
	if other == rotated {
		t.Fatal("different keys share a session")
	}
}
