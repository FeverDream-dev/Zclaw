package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Version mirrors the version declared in cmd/dockclawd/main.go
const Version = "0.1.0"

// Config holds runtime configuration loaded from environment variables
// and defaults. This struct is imported by cmd/dockclawd/main.go.
type Config struct {
	DataDir         string
	DBPath          string
	HTTPPort        int
	HealthPort      int
	LogLevel        string
	BrowserWSURL    string
	DockerSocket    string
	WorkerPoolSize  int
	BrowserPoolSize int
	HeartbeatJitter int
	DefaultProvider string
	DefaultModel    string

	// Auth related configuration
	JWTSecret      string
	AdminAPIKey    string
	SessionTTL     string
	APIKeyPrefix   string
	AuthEnabled    bool
	RateLimitRPS   int
	RateLimitBurst int
}

// Load reads environment variables and returns a Config filled with values
// matching the defaults used in loadConfig() in cmd/dockclawd/main.go.
func Load() Config {
	return Config{
		DataDir:         envOr("ZCLAW_DATA_DIR", "./data"),
		DBPath:          envOr("ZCLAW_DB_PATH", "./data/zclaw.db"),
		HTTPPort:        envIntOr("ZCLAW_HTTP_PORT", 8080),
		HealthPort:      envIntOr("ZCLAW_HEALTH_PORT", 8081),
		LogLevel:        envOr("ZCLAW_LOG_LEVEL", "info"),
		BrowserWSURL:    envOr("ZCLAW_BROWSER_WORKER_URL", "ws://browser-worker:9222"),
		DockerSocket:    envOr("ZCLAW_DOCKER_SOCKET", "/var/run/docker.sock"),
		WorkerPoolSize:  envIntOr("ZCLAW_WORKER_POOL_SIZE", 10),
		BrowserPoolSize: envIntOr("ZCLAW_BROWSER_POOL_SIZE", 5),
		HeartbeatJitter: envIntOr("ZCLAW_HEARTBEAT_JITTER_SECONDS", 30),
		DefaultProvider: envOr("ZCLAW_DEFAULT_MODEL_PROVIDER", "openai"),
		DefaultModel:    envOr("ZCLAW_DEFAULT_MODEL", "gpt-4o-mini"),

		JWTSecret:      envOr("ZCLAW_JWT_SECRET", ""),
		AdminAPIKey:    envOr("ZCLAW_ADMIN_API_KEY", ""),
		SessionTTL:     envOr("ZCLAW_SESSION_TTL", "24h"),
		APIKeyPrefix:   envOr("ZCLAW_API_KEY_PREFIX", "zclaw_"),
		AuthEnabled:    envBoolOr("ZCLAW_AUTH_ENABLED", false),
		RateLimitRPS:   envIntOr("ZCLAW_RATE_LIMIT_RPS", 100),
		RateLimitBurst: envIntOr("ZCLAW_RATE_LIMIT_BURST", 200),
	}
}

// SessionTTLSeconds returns the duration of the configured session TTL in seconds.
func (c Config) SessionTTLSeconds() int {
	if d, err := time.ParseDuration(c.SessionTTL); err == nil {
		return int(d.Seconds())
	}
	return 0
}

// Validate ensures the Config is coherent. Currently enforces that if
// authentication is enabled, a JWT secret must be provided.
func (c Config) Validate() error {
	if c.AuthEnabled && strings.TrimSpace(c.JWTSecret) == "" {
		return fmt.Errorf("auth is enabled but JWT secret is empty")
	}
	return nil
}

// envOr returns the environment value for key or the provided fallback.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envIntOr returns the integer value for key or the provided fallback.
func envIntOr(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if iv, err := strconv.Atoi(v); err == nil {
			return iv
		}
	}
	return fallback
}

// envBoolOr returns the boolean value for key or the provided fallback.
// Treats "true", "1", and "yes" (case-insensitive) as true.
func envBoolOr(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		s := strings.ToLower(strings.TrimSpace(v))
		if s == "true" || s == "1" || s == "yes" {
			return true
		}
		return false
	}
	return fallback
}
