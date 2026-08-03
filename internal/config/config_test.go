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
	if cfg.ZenBase != "https://opencode.ai/zen/v1" {
		t.Fatalf("ZenBase = %q", cfg.ZenBase)
	}
	if cfg.ZenGoBase != "https://opencode.ai/zen/go" {
		t.Fatalf("ZenGoBase = %q", cfg.ZenGoBase)
	}
	if cfg.OllamaBase != "https://ollama.com" {
		t.Fatalf("OllamaBase = %q", cfg.OllamaBase)
	}
}

func TestLoad_parses_ollama_base(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", "admin")
	t.Setenv("ADMIN_SECRET", "01234567890123456789012345678901")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("OLLAMA_BASE", "https://ollama.example.com/v1/")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.OllamaBase != "https://ollama.example.com/v1" {
		t.Fatalf("OllamaBase = %q, want trimmed trailing slash", cfg.OllamaBase)
	}
}

func TestLoad_parses_zen_go_base(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", "admin")
	t.Setenv("ADMIN_SECRET", "01234567890123456789012345678901")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("ZEN_GO_BASE", "https://opencode.ai/zen/go/")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ZenGoBase != "https://opencode.ai/zen/go" {
		t.Fatalf("ZenGoBase = %q, want trimmed trailing slash", cfg.ZenGoBase)
	}
}
