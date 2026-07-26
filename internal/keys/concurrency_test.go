package keys_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"jovepoxy/internal/keys"
)

func TestService_enforces_daily_limit_under_parallel_verification(t *testing.T) {
	// Given
	ctx := context.Background()
	service, _ := newService(t)
	created, err := service.Create(ctx, keys.CreateInput{Label: "parallel", DailyLimit: 10})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	var successes, limited int
	var lock sync.Mutex
	var workers sync.WaitGroup

	// When
	for range 30 {
		workers.Go(func() {
			_, verifyErr := service.Verify(ctx, keys.Credentials{APIKey: created.Secret})
			lock.Lock()
			defer lock.Unlock()
			switch {
			case verifyErr == nil:
				successes++
			case errors.Is(verifyErr, keys.ErrRateLimited):
				limited++
			default:
				t.Errorf("Verify() error = %v", verifyErr)
			}
		})
	}
	workers.Wait()

	// Then
	if successes != 10 {
		t.Fatalf("successful verifications = %d, want 10", successes)
	}
	if limited != 20 {
		t.Fatalf("limited verifications = %d, want 20", limited)
	}
}
