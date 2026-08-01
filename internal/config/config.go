package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"
)

var ErrInvalidValue = errors.New("invalid configuration value")

// EnvironmentError identifies an invalid environment variable without exposing its value.
type EnvironmentError struct {
	Variable string
	Cause    error
}

func (e *EnvironmentError) Error() string {
	return fmt.Sprintf("config %s: %v", e.Variable, e.Cause)
}

func (e *EnvironmentError) Unwrap() error { return e.Cause }

// Config holds process configuration parsed from environment.
type Config struct {
	Listen          string
	DataDir         string
	AdminPassword   string
	AdminSecret     string
	ZenBase         string
	OllamaBase      string
	HTTPProxy       *url.URL
	HTTPSProxy      *url.URL
	ModelCacheTTL   time.Duration
	OCVersion       string
	ShowAllModels   bool
	CookieSecure    bool
	UpstreamTimeout time.Duration
}

// Load reads configuration from environment with defaults.
func Load() (Config, error) {
	adminPassword, err := requiredEnv("ADMIN_PASSWORD")
	if err != nil {
		return Config{}, err
	}
	listen, err := listenEnv()
	if err != nil {
		return Config{}, err
	}
	dataDir, err := requiredEnvOrDefault("DATA_DIR", "./data")
	if err != nil {
		return Config{}, err
	}
	zenBase, err := serviceURL("ZEN_BASE", envOr("ZEN_BASE", "https://opencode.ai/zen/v1"))
	if err != nil {
		return Config{}, err
	}
	ollamaBase, err := serviceURL("OLLAMA_BASE", envOr("OLLAMA_BASE", "https://ollama.com"))
	if err != nil {
		return Config{}, err
	}
	modelCacheTTL, err := durationEnv("MODEL_CACHE_TTL", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	upstreamTimeout, err := durationEnv("UPSTREAM_TIMEOUT", 120*time.Second)
	if err != nil {
		return Config{}, err
	}
	httpProxy, err := proxyEnv("HTTP_PROXY")
	if err != nil {
		return Config{}, err
	}
	httpsProxy, err := proxyEnv("HTTPS_PROXY")
	if err != nil {
		return Config{}, err
	}
	adminSecret, err := adminSecret()
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Listen:          listen,
		DataDir:         dataDir,
		AdminPassword:   adminPassword,
		AdminSecret:     adminSecret,
		ZenBase:         zenBase,
		OllamaBase:      ollamaBase,
		HTTPProxy:       httpProxy,
		HTTPSProxy:      httpsProxy,
		ModelCacheTTL:   modelCacheTTL,
		OCVersion:       envOr("OC_VERSION", "1.15.0"),
		ShowAllModels:   envOr("SHOW_ALL_MODELS", "false") == "true",
		CookieSecure:    envOr("COOKIE_SECURE", "false") == "true",
		UpstreamTimeout: upstreamTimeout,
	}
	return cfg, nil
}

func adminSecret() (string, error) {
	secret, isSet := os.LookupEnv("ADMIN_SECRET")
	if !isSet {
		return "", invalid("ADMIN_SECRET", errors.New("is required"))
	}
	if len(strings.TrimSpace(secret)) < 32 {
		return "", invalid("ADMIN_SECRET", errors.New("must be at least 32 characters"))
	}
	return secret, nil
}

func listenEnv() (string, error) {
	listen := envOr("LISTEN", "0.0.0.0:6446")
	if _, _, err := net.SplitHostPort(listen); err != nil {
		return "", invalid("LISTEN", errors.New("must be host:port"))
	}
	return listen, nil
}

func requiredEnv(key string) (string, error) {
	value, isSet := os.LookupEnv(key)
	if !isSet {
		return "", invalid(key, errors.New("is required"))
	}
	if strings.TrimSpace(value) == "" {
		return "", invalid(key, errors.New("is required"))
	}
	return value, nil
}

func requiredEnvOrDefault(key, fallback string) (string, error) {
	value, isSet := os.LookupEnv(key)
	if !isSet {
		return fallback, nil
	}
	if strings.TrimSpace(value) == "" {
		return "", invalid(key, errors.New("is required"))
	}
	return value, nil
}

func serviceURL(key, value string) (string, error) {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", invalid(key, errors.New("must be an HTTP(S) URL"))
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func proxyEnv(key string) (*url.URL, error) {
	value, isSet := os.LookupEnv(key)
	if !isSet || value == "" {
		return nil, nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, invalid(key, errors.New("must be an HTTP(S) URL"))
	}
	return parsed, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value, isSet := os.LookupEnv(key)
	if !isSet {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, invalid(key, errors.New("must be a positive duration"))
	}
	return duration, nil
}

func invalid(variable string, cause error) error {
	return &EnvironmentError{Variable: variable, Cause: fmt.Errorf("%w: %w", ErrInvalidValue, cause)}
}
