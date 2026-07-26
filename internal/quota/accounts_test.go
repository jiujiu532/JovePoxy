package quota_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jovepoxy/internal/crypto"
	"jovepoxy/internal/db"
	"jovepoxy/internal/quota"
)

const testCookie = "auth=credential-for-tests-only"

func TestAccountService_crud_masks_cookie_and_encrypts_credential(t *testing.T) {
	// Given
	ctx := context.Background()
	service, database := newAccountService(t)
	created, err := service.Create(ctx, quota.CreateAccountInput{
		Name: "primary", WorkspaceID: "wrk_primary", AuthCookie: "Cookie: " + testCookie,
		ShowRolling: true, ShowWeekly: true, ShowMonthly: false, Enabled: true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// When
	accounts, err := service.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	var ciphertext string
	if err := database.QueryRowContext(ctx, "SELECT auth_cookie_ciphertext FROM opencode_accounts WHERE id = ?", created.ID).Scan(&ciphertext); err != nil {
		t.Fatalf("read ciphertext: %v", err)
	}
	updatedName := "renamed"
	updated, err := service.Update(ctx, created.ID, quota.UpdateAccountInput{Name: &updatedName})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := service.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Then
	if len(accounts) != 1 || accounts[0].MaskedCookie != "auth=***" {
		t.Fatalf("List() accounts = %+v, want exactly one masked cookie", accounts)
	}
	if strings.Contains(ciphertext, testCookie) || ciphertext == testCookie {
		t.Fatal("stored ciphertext contains plaintext cookie")
	}
	if updated.Name != updatedName || updated.MaskedCookie != "auth=***" {
		t.Fatalf("Update() account = %+v, want renamed account with preserved masked cookie", updated)
	}
	if _, err := service.GetCredential(ctx, created.ID); !errors.Is(err, quota.ErrAccountNotFound) {
		t.Fatalf("GetCredential() after Delete error = %v, want ErrAccountNotFound", err)
	}
	t.Logf("manual_qa_account_id=%s name=%s cookie=%s", created.ID, updated.Name, updated.MaskedCookie)
}

func TestAccountService_rejects_missing_or_malformed_cookie_without_leaking_it(t *testing.T) {
	// Given
	service, _ := newAccountService(t)

	// When / Then
	for _, rawCookie := range []string{"", "not-auth", "auth=", "auth=value; other=value", "Cookie: session=value", "auth=bad\r\nX-Test: injected"} {
		_, err := service.Create(context.Background(), quota.CreateAccountInput{
			Name: "invalid", WorkspaceID: "wrk_invalid", AuthCookie: rawCookie,
		})
		if !errors.Is(err, quota.ErrInvalidCookie) {
			t.Fatalf("Create(%q) error = %v, want ErrInvalidCookie", rawCookie, err)
		}
		if rawCookie != "" && strings.Contains(err.Error(), rawCookie) {
			t.Fatalf("Create(%q) error leaked cookie", rawCookie)
		}
	}
}

func TestAccountService_updates_visibility_and_enabled_state_without_replacing_secret(t *testing.T) {
	// Given
	ctx := context.Background()
	service, _ := newAccountService(t)
	created, err := service.Create(ctx, quota.CreateAccountInput{
		Name: "toggle", WorkspaceID: "wrk_toggle", AuthCookie: testCookie, Enabled: true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	showRolling := true
	enabled := false

	// When
	updated, err := service.Update(ctx, created.ID, quota.UpdateAccountInput{ShowRolling: &showRolling, Enabled: &enabled})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	credential, err := service.GetCredential(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetCredential() error = %v", err)
	}

	// Then
	if !updated.ShowRolling || updated.Enabled || credential.AuthCookie != testCookie {
		t.Fatalf("updated account = %+v credential=%+v, want enabled false and preserved credential", updated, credential)
	}
}

func TestWorkspaceResolver_sends_normalized_cookie_and_resolves_configured_workspace(t *testing.T) {
	// Given
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/workspace/wrk_connected/go" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if got := request.Header.Get("Cookie"); got != testCookie {
			t.Errorf("Cookie = %q, want normalized auth cookie", got)
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	resolver, err := quota.NewWorkspaceResolver(quota.WorkspaceResolverConfig{BaseURL: server.URL, HTTPClient: server.Client(), Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewWorkspaceResolver() error = %v", err)
	}

	// When
	workspaceID, err := resolver.Resolve(context.Background(), quota.WorkspaceTestInput{WorkspaceID: "wrk_connected", AuthCookie: "Cookie: " + testCookie})

	// Then
	if err != nil || workspaceID != "wrk_connected" {
		t.Fatalf("Resolve() = %q, %v", workspaceID, err)
	}
}

func TestWorkspaceResolver_returns_safe_errors_for_cancel_timeout_and_non_success_upstream(t *testing.T) {
	t.Run("cancelled context", func(t *testing.T) {
		// Given
		resolver := newResolver(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("handler must not run") }))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// When
		_, err := resolver.Resolve(ctx, quota.WorkspaceTestInput{WorkspaceID: "wrk_cancelled", AuthCookie: testCookie})

		// Then
		if !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), testCookie) {
			t.Fatalf("Resolve() error = %v, want safe cancellation", err)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		// Given
		started := make(chan struct{})
		release := make(chan struct{})
		resolver := newResolverWithTimeout(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			close(started)
			<-release
		}), 20*time.Millisecond)
		t.Cleanup(func() { close(release) })

		// When
		_, err := resolver.Resolve(context.Background(), quota.WorkspaceTestInput{WorkspaceID: "wrk_timeout", AuthCookie: testCookie})
		<-started

		// Then
		if !errors.Is(err, quota.ErrWorkspaceTimeout) || strings.Contains(err.Error(), testCookie) {
			t.Fatalf("Resolve() error = %v, want safe timeout", err)
		}
	})
	t.Run("non-success", func(t *testing.T) {
		// Given
		resolver := newResolver(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) { writer.WriteHeader(http.StatusUnauthorized) }))

		// When
		_, err := resolver.Resolve(context.Background(), quota.WorkspaceTestInput{WorkspaceID: "wrk_unauthorized", AuthCookie: testCookie})

		// Then
		var statusError *quota.WorkspaceStatusError
		if !errors.As(err, &statusError) || statusError.StatusCode != http.StatusUnauthorized || strings.Contains(err.Error(), testCookie) {
			t.Fatalf("Resolve() error = %v, want safe 401 WorkspaceStatusError", err)
		}
	})
}

func newAccountService(t *testing.T) (*quota.AccountService, *sql.DB) {
	t.Helper()
	database, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("close database: %v", closeErr)
		}
	})
	box, err := crypto.NewBox("test-admin-secret")
	if err != nil {
		t.Fatalf("new crypto box: %v", err)
	}
	service, err := quota.NewAccountService(database, box)
	if err != nil {
		t.Fatalf("NewAccountService() error = %v", err)
	}
	return service, database
}

func newResolver(t *testing.T, handler http.Handler) *quota.WorkspaceResolver {
	t.Helper()
	return newResolverWithTimeout(t, handler, time.Second)
}

func newResolverWithTimeout(t *testing.T, handler http.Handler, timeout time.Duration) *quota.WorkspaceResolver {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	resolver, err := quota.NewWorkspaceResolver(quota.WorkspaceResolverConfig{BaseURL: server.URL, HTTPClient: server.Client(), Timeout: timeout})
	if err != nil {
		t.Fatalf("NewWorkspaceResolver() error = %v", err)
	}
	return resolver
}
