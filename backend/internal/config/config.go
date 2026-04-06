package config

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/hkdf"
)

type Config struct {
	Environment         string
	Debug               bool
	DocsEnabled         bool
	Server              ServerConfig
	CORS                CORSConfig
	Auth                AuthConfig
	Database            DatabaseConfig
	Bootstrap           BootstrapConfig
	APIKeyEncryptionKey []byte // 32 bytes from API_KEY_ENCRYPTION_KEY (hex); nil = encryption disabled
}

type ServerConfig struct {
	Port              string
	TrustedProxies    []string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

type CORSConfig struct {
	AllowedOrigins []string
}

type AuthConfig struct {
	SessionTTL    time.Duration
	SessionSecret []byte
	Cookie        CookieConfig
}

type CookieConfig struct {
	Domain   string
	Secure   bool
	HTTPOnly bool
	SameSite http.SameSite
}

type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type BootstrapConfig struct {
	AdminUsername string
	AdminPassword string
}

var current Config

func Current() Config {
	return current
}

func Load() (Config, error) {
	environment := strings.TrimSpace(firstNonEmpty(os.Getenv("APP_ENV"), os.Getenv("NODE_ENV")))
	if environment == "" {
		environment = "production"
	}

	cfg := Config{
		Environment: environment,
		Debug:       parseBoolEnv("DEBUG", false),
		DocsEnabled: parseBoolEnv("DOCS", false),
		Server: ServerConfig{
			Port:              firstNonEmpty(strings.TrimSpace(os.Getenv("PORT")), "8000"),
			TrustedProxies:    parseCSVEnv("TRUSTED_PROXIES"),
			ReadTimeout:       parseDurationEnv("HTTP_READ_TIMEOUT", 10*time.Second),
			ReadHeaderTimeout: parseDurationEnv("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			WriteTimeout:      parseDurationEnv("HTTP_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:       parseDurationEnv("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout:   parseDurationEnv("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
		},
		CORS: CORSConfig{
			AllowedOrigins: defaultAllowedOrigins(environment),
		},
		Auth: AuthConfig{
			SessionTTL: parseDurationEnv("SESSION_TTL", 24*time.Hour),
			Cookie: CookieConfig{
				Domain:   strings.TrimSpace(os.Getenv("COOKIE_DOMAIN")),
				Secure:   parseBoolEnv("COOKIE_SECURE", environment == "production"),
				HTTPOnly: true,
				SameSite: parseSameSiteEnv("COOKIE_SAMESITE", http.SameSiteLaxMode),
			},
		},
		Database: DatabaseConfig{
			URL:             parseDatabaseURL(),
			MaxOpenConns:    parseIntEnv("DB_MAX_OPEN_CONNS", 20),
			MaxIdleConns:    parseIntEnv("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: parseDurationEnv("DB_CONN_MAX_LIFETIME", time.Hour),
			ConnMaxIdleTime: parseDurationEnv("DB_CONN_MAX_IDLE_TIME", 15*time.Minute),
		},
		Bootstrap: BootstrapConfig{
			AdminUsername: firstNonEmpty(strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_USERNAME")), "admin"),
			AdminPassword: os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
		},
	}

	if origins := parseCSVEnv("CORS_ALLOW_ORIGINS"); len(origins) > 0 {
		cfg.CORS.AllowedOrigins = origins
	}

	secretKey, err := parseHexKey("SECRET_KEY", 32)
	if err != nil {
		return Config{}, err
	}
	if len(secretKey) == 0 {
		if cfg.IsProduction() {
			return Config{}, fmt.Errorf("SECRET_KEY is required in production")
		}
		secretKey = make([]byte, 32)
		if _, err := rand.Read(secretKey); err != nil {
			return Config{}, fmt.Errorf("failed to generate ephemeral secret key: %w", err)
		}
		slog.Warn("SECRET_KEY not set, using ephemeral key — sessions will not survive restarts and API keys will not be encrypted")
	} else {
		apiKeyEncKey, err := deriveKey(secretKey, "api-key-encrypt")
		if err != nil {
			return Config{}, fmt.Errorf("failed to derive API key encryption key: %w", err)
		}
		cfg.APIKeyEncryptionKey = apiKeyEncKey
	}

	sessionSecret, err := deriveKey(secretKey, "session")
	if err != nil {
		return Config{}, fmt.Errorf("failed to derive session secret: %w", err)
	}
	cfg.Auth.SessionSecret = sessionSecret

	if len(cfg.Auth.Cookie.Domain) > 0 && strings.Contains(cfg.Auth.Cookie.Domain, "://") {
		return Config{}, fmt.Errorf("COOKIE_DOMAIN must be a hostname, not a URL")
	}

	if err := validateDatabaseURL(cfg.Database.URL); err != nil {
		return Config{}, err
	}

	if cfg.Auth.SessionTTL <= 0 {
		return Config{}, fmt.Errorf("SESSION_TTL must be greater than zero")
	}
	if cfg.Bootstrap.AdminUsername == "" {
		return Config{}, fmt.Errorf("BOOTSTRAP_ADMIN_USERNAME must not be empty")
	}

	current = cfg
	return cfg, nil
}

func (c Config) IsProduction() bool {
	return strings.EqualFold(c.Environment, "production")
}

func defaultAllowedOrigins(environment string) []string {
	if strings.EqualFold(environment, "production") {
		return nil
	}
	return []string{
		"http://localhost:5173",
		"http://127.0.0.1:5173",
		"http://localhost:4173",
		"http://127.0.0.1:4173",
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func parseCSVEnv(name string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseBoolEnv(name string, defaultValue bool) bool {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return defaultValue
	}

	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func parseIntEnv(name string, defaultValue int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func parseDurationEnv(name string, defaultValue time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return defaultValue
	}

	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func parseDatabaseURL() string {
	raw := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if raw == "" {
		return "file:./wgkeeper-console.db"
	}
	return raw
}

func validateDatabaseURL(url string) error {
	if strings.HasPrefix(url, "postgres://") ||
		strings.HasPrefix(url, "postgresql://") ||
		strings.HasPrefix(url, "file:") {
		return nil
	}
	return fmt.Errorf("DATABASE_URL must start with postgres://, postgresql://, or file: (got %q)", url)
}

// parseHexKey decodes a hex-encoded key from the named env var and checks its
// length. Returns nil without error when the variable is unset or empty.
func parseHexKey(name string, expectedBytes int) ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil, nil
	}
	key, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid hex: %w", name, err)
	}
	if len(key) != expectedBytes {
		return nil, fmt.Errorf("%s: must be %d bytes (%d hex chars), got %d bytes",
			name, expectedBytes, expectedBytes*2, len(key))
	}
	return key, nil
}

// deriveKey uses HKDF-SHA256 to derive a 32-byte purpose-specific key from master.
func deriveKey(master []byte, info string) ([]byte, error) {
	r := hkdf.New(sha256.New, master, nil, []byte(info))
	key := make([]byte, 32)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, err
	}
	return key, nil
}

func parseSameSiteEnv(name string, defaultValue http.SameSite) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "":
		return defaultValue
	case "lax":
		return http.SameSiteLaxMode
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return defaultValue
	}
}
