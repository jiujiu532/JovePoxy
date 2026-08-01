package config

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestLoad_rejects_malformed_zen_base(t *testing.T) {
	// Given
	t.Setenv("ADMIN_PASSWORD", "test-admin-password")
	t.Setenv("ADMIN_SECRET", "01234567890123456789012345678901")
	t.Setenv("ZEN_BASE", "not a URL")

	// When
	_, err := Load()

	// Then
	if err == nil {
		t.Fatal("Load() error = nil, want malformed ZEN_BASE rejection")
	}
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("Load() error = %v, want ErrInvalidValue", err)
	}
}

func TestLoad_rejects_malformed_ollama_base(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", "test-admin-password")
	t.Setenv("ADMIN_SECRET", "01234567890123456789012345678901")
	t.Setenv("OLLAMA_BASE", "not a URL")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want malformed OLLAMA_BASE rejection")
	}
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("Load() error = %v, want ErrInvalidValue", err)
	}
}

func TestLoad_rejects_short_explicit_admin_secret(t *testing.T) {
	// Given
	t.Setenv("ADMIN_PASSWORD", "test-admin-password")
	t.Setenv("ADMIN_SECRET", "too-short")

	// When
	_, err := Load()

	// Then
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("Load() error = %v, want ErrInvalidValue", err)
	}
}

func TestLoad_parses_standard_http_proxy_environment(t *testing.T) {
	// Given
	t.Setenv("ADMIN_PASSWORD", "test-admin-password")
	t.Setenv("ADMIN_SECRET", "01234567890123456789012345678901")
	t.Setenv("HTTP_PROXY", "http://proxy.internal:8080")
	t.Setenv("HTTPS_PROXY", "https://proxy.internal:8443")

	// When
	cfg, err := Load()

	// Then
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPProxy == nil || cfg.HTTPProxy.String() != "http://proxy.internal:8080" {
		t.Fatalf("HTTPProxy = %v", cfg.HTTPProxy)
	}
	if cfg.HTTPSProxy == nil || cfg.HTTPSProxy.String() != "https://proxy.internal:8443" {
		t.Fatalf("HTTPSProxy = %v", cfg.HTTPSProxy)
	}
}

func TestLoad_rejects_missing_admin_password_without_echoing_secret(t *testing.T) {
	// Given
	secret := "secret-value-that-must-not-appear-in-an-error"
	unsetenvForTest(t, "ADMIN_PASSWORD")
	t.Setenv("ADMIN_SECRET", secret)

	// When
	_, err := Load()

	// Then
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("Load() error = %v, want ErrInvalidValue", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("Load() error contains ADMIN_SECRET value")
	}
}

func TestLoad_rejects_missing_admin_secret(t *testing.T) {
	// Given
	t.Setenv("ADMIN_PASSWORD", "test-admin-password")
	unsetenvForTest(t, "ADMIN_SECRET")

	// When
	_, err := Load()

	// Then
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("Load() error = %v, want ErrInvalidValue", err)
	}
}

func TestLoad_rejects_invalid_proxy_url(t *testing.T) {
	// Given
	t.Setenv("ADMIN_PASSWORD", "test-admin-password")
	t.Setenv("ADMIN_SECRET", "01234567890123456789012345678901")
	t.Setenv("HTTP_PROXY", "proxy without a scheme")

	// When
	_, err := Load()

	// Then
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("Load() error = %v, want ErrInvalidValue", err)
	}
}

func TestLoad_rejects_empty_data_dir(t *testing.T) {
	// Given
	t.Setenv("ADMIN_PASSWORD", "test-admin-password")
	t.Setenv("ADMIN_SECRET", "01234567890123456789012345678901")
	t.Setenv("DATA_DIR", "")

	// When
	_, err := Load()

	// Then
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("Load() error = %v, want ErrInvalidValue", err)
	}
}

func unsetenvForTest(t *testing.T, key string) {
	t.Helper()
	value, isSet := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if !isSet {
			if err := os.Unsetenv(key); err != nil {
				t.Errorf("restore unset %s: %v", key, err)
			}
			return
		}
		if err := os.Setenv(key, value); err != nil {
			t.Errorf("restore %s: %v", key, err)
		}
	})
}
