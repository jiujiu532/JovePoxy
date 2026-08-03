package auth_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"jovepoxy/internal/auth"
)

func TestService_Login_rate_limits_failed_attempts_and_success_resets_source(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	service, database := newService(t, clock)
	defer database.Close()
	input := auth.LoginInput{Password: "wrong-password", Source: "198.51.100.20"}

	// When
	for range auth.DefaultLoginAttemptLimit {
		if _, err := service.Login(context.Background(), input); !errors.Is(err, auth.ErrUnauthorized) {
			t.Fatalf("failed Login() error = %v, want ErrUnauthorized", err)
		}
	}
	_, limitedErr := service.Login(context.Background(), input)
	clock.Advance(auth.DefaultLoginAttemptWindow + time.Second)
	_, successErr := service.Login(context.Background(), auth.LoginInput{Password: "correct-password", Source: input.Source})
	_, laterFailureErr := service.Login(context.Background(), input)

	// Then
	if !errors.Is(limitedErr, auth.ErrRateLimited) {
		t.Fatalf("limited Login() error = %v, want ErrRateLimited", limitedErr)
	}
	if successErr != nil {
		t.Fatalf("successful Login() error = %v", successErr)
	}
	if !errors.Is(laterFailureErr, auth.ErrUnauthorized) {
		t.Fatalf("Login() after successful reset error = %v, want ErrUnauthorized", laterFailureErr)
	}
}

func TestService_Login_rate_limit_blocks_before_password_verify(t *testing.T) {
	// Given: exhaust the attempt budget with wrong passwords.
	clock := newFakeClock(time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	service, database := newService(t, clock)
	defer database.Close()
	source := "198.51.100.40"
	for range auth.DefaultLoginAttemptLimit {
		if _, err := service.Login(context.Background(), auth.LoginInput{
			Password: "wrong-password",
			Source:   source,
		}); !errors.Is(err, auth.ErrUnauthorized) {
			t.Fatalf("failed Login() error = %v, want ErrUnauthorized", err)
		}
	}

	// When: source is already limited — even the correct password must not pass.
	// A blocked Login must also skip bcrypt (cost=10 is tens of ms); map lookup is << 5ms.
	start := time.Now()
	_, correctWhileLimitedErr := service.Login(context.Background(), auth.LoginInput{
		Password: "correct-password",
		Source:   source,
	})
	blockedCorrectElapsed := time.Since(start)

	start = time.Now()
	_, wrongWhileLimitedErr := service.Login(context.Background(), auth.LoginInput{
		Password: "still-wrong",
		Source:   source,
	})
	blockedWrongElapsed := time.Since(start)

	// Baseline: one bcrypt verify for a wrong password on a fresh source.
	start = time.Now()
	_, _ = service.Login(context.Background(), auth.LoginInput{
		Password: "wrong-password",
		Source:   "198.51.100.41",
	})
	bcryptElapsed := time.Since(start)

	// Then
	if !errors.Is(correctWhileLimitedErr, auth.ErrRateLimited) {
		t.Fatalf("correct password while limited error = %v, want ErrRateLimited", correctWhileLimitedErr)
	}
	if !errors.Is(wrongWhileLimitedErr, auth.ErrRateLimited) {
		t.Fatalf("wrong password while limited error = %v, want ErrRateLimited", wrongWhileLimitedErr)
	}
	// Blocked path must be clearly cheaper than a real bcrypt CompareHashAndPassword.
	// Use a generous floor so CI noise does not flake, but still catch a full bcrypt.
	const maxBlocked = 5 * time.Millisecond
	if blockedCorrectElapsed > maxBlocked {
		t.Fatalf("blocked Login(correct) took %v (> %v); likely still running bcrypt", blockedCorrectElapsed, maxBlocked)
	}
	if blockedWrongElapsed > maxBlocked {
		t.Fatalf("blocked Login(wrong) took %v (> %v); likely still running bcrypt", blockedWrongElapsed, maxBlocked)
	}
	// Sanity: baseline bcrypt itself should be slower than the blocked path when the host is healthy.
	// Skip the relative check if bcrypt is unusually fast (e.g. tiny cost regression / very fast CPU).
	if bcryptElapsed > maxBlocked && blockedCorrectElapsed*10 > bcryptElapsed {
		t.Fatalf("blocked Login(correct)=%v not clearly faster than bcrypt baseline=%v", blockedCorrectElapsed, bcryptElapsed)
	}
}

func TestService_Login_rate_limits_same_ip_across_different_ports(t *testing.T) {
	// Given: each TCP connection presents RemoteAddr as ip:ephemeral-port
	clock := newFakeClock(time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	service, database := newService(t, clock)
	defer database.Close()
	sources := []string{
		"1.2.3.4:10000",
		"1.2.3.4:10001",
		"1.2.3.4:10002",
		"1.2.3.4:10003",
		"1.2.3.4:10004",
	}

	// When: fail once per distinct port (same IP)
	for _, source := range sources {
		if _, err := service.Login(context.Background(), auth.LoginInput{
			Password: "wrong-password",
			Source:   source,
		}); !errors.Is(err, auth.ErrUnauthorized) {
			t.Fatalf("Login(%q) error = %v, want ErrUnauthorized", source, err)
		}
	}
	// Same IP, yet another port — should already be rate limited
	_, limitedErr := service.Login(context.Background(), auth.LoginInput{
		Password: "wrong-password",
		Source:   "1.2.3.4:10005",
	})
	// Bare IP without port must share the same bucket
	_, bareLimitedErr := service.Login(context.Background(), auth.LoginInput{
		Password: "wrong-password",
		Source:   "1.2.3.4",
	})
	// A different IP must still be allowed ordinary failures
	_, otherIPErr := service.Login(context.Background(), auth.LoginInput{
		Password: "wrong-password",
		Source:   "1.2.3.5:20000",
	})

	// Then
	if !errors.Is(limitedErr, auth.ErrRateLimited) {
		t.Fatalf("Login after same-IP port rotation error = %v, want ErrRateLimited", limitedErr)
	}
	if !errors.Is(bareLimitedErr, auth.ErrRateLimited) {
		t.Fatalf("Login with bare IP error = %v, want ErrRateLimited", bareLimitedErr)
	}
	if !errors.Is(otherIPErr, auth.ErrUnauthorized) {
		t.Fatalf("Login for different IP error = %v, want ErrUnauthorized", otherIPErr)
	}
}

func TestService_Login_rate_limits_same_ipv6_across_different_ports(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	service, database := newService(t, clock)
	defer database.Close()

	// When
	for i := range auth.DefaultLoginAttemptLimit {
		source := "[2001:db8::1]:" + strconv.Itoa(10000+i)
		if _, err := service.Login(context.Background(), auth.LoginInput{
			Password: "wrong-password",
			Source:   source,
		}); !errors.Is(err, auth.ErrUnauthorized) {
			t.Fatalf("Login(%q) error = %v, want ErrUnauthorized", source, err)
		}
	}
	_, limitedErr := service.Login(context.Background(), auth.LoginInput{
		Password: "wrong-password",
		Source:   "[2001:db8::1]:65535",
	})

	// Then
	if !errors.Is(limitedErr, auth.ErrRateLimited) {
		t.Fatalf("IPv6 same-host port rotation error = %v, want ErrRateLimited", limitedErr)
	}
}

func TestService_Login_rate_limits_same_source_atomically_under_concurrent_failures(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	service, database := newService(t, clock)
	defer database.Close()
	const workers = 20
	start := make(chan struct{})
	errCh := make(chan error, workers)
	var group sync.WaitGroup

	for range workers {
		group.Go(func() {
			<-start
			_, err := service.Login(context.Background(), auth.LoginInput{
				Password: "wrong-password",
				Source:   "198.51.100.21",
			})
			errCh <- err
		})
	}

	// When
	close(start)
	group.Wait()
	close(errCh)

	// Then
	unauthorized := 0
	rateLimited := 0
	for err := range errCh {
		switch {
		case errors.Is(err, auth.ErrUnauthorized):
			unauthorized++
		case errors.Is(err, auth.ErrRateLimited):
			rateLimited++
		default:
			t.Fatalf("concurrent Login() error = %v", err)
		}
	}
	if unauthorized != auth.DefaultLoginAttemptLimit {
		t.Fatalf("ordinary unauthorized attempts = %d, want %d", unauthorized, auth.DefaultLoginAttemptLimit)
	}
	if rateLimited != workers-auth.DefaultLoginAttemptLimit {
		t.Fatalf("rate-limited attempts = %d, want %d", rateLimited, workers-auth.DefaultLoginAttemptLimit)
	}
}

func TestService_Login_and_Verify_are_safe_under_concurrency(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC))
	service, database := newService(t, clock)
	defer database.Close()
	const workers = 20
	errCh := make(chan error, workers)
	var group sync.WaitGroup

	// When
	for range workers {
		group.Go(func() {
			session, err := service.Login(context.Background(), auth.LoginInput{Password: "correct-password", Source: "203.0.113.30"})
			if err == nil {
				err = service.Verify(context.Background(), session.Token)
			}
			errCh <- err
		})
	}
	group.Wait()
	close(errCh)

	// Then
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent session operation error = %v", err)
		}
	}
}
