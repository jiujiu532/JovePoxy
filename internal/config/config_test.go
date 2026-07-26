package config

import (
	"testing"
	"time"
)

func TestLoad_uses_existing_defaults_when_environment_is_empty(t *testing.T) {
	// Given
	t.Setenv("ADMIN_PASSWORD", "admin")
	t.Setenv("ADMIN_SECRET", "01234567890123456789012345678901")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")

	// When
	cfg, err := Load()

	// Then
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Listen != "0.0.0.0:6446" {
		t.Fatalf("Listen = %q", cfg.Listen)
	}
	if cfg.DataDir != "./data" {
		t.Fatalf("DataDir = %q", cfg.DataDir)
	}
	if cfg.ModelCacheTTL != 5*time.Minute {
		t.Fatalf("ModelCacheTTL = %s", cfg.ModelCacheTTL)
	}
}
