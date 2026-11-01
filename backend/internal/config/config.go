package config

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/hkdf"
)

type Config struct {
	Environment         string
	Debug               bool
	DocsEnabled         bool
	LogFormat           string // "text" or "json"
	Server              ServerConfig
	Metrics             MetricsConfig
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

// MetricsConfig enables the Prometheus endpoint on its own port. Port 0 keeps
// it off; a token of at least 32 characters is required when it is on.
type MetricsConfig struct {
	Port  int
	Token string
}

// Enabled reports whether the metrics endpoint should run.
func (m MetricsConfig) Enabled() bool { return m.Port != 0 }

// minMetricsTokenLength matches wgkeeper-node's requirement.
const minMetricsTokenLength = 32

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

	// Invalid values are collected and reported together instead of silently
	// falling back to defaults: SESSION_TTL=7d must not quietly become 24h.
	p := &envParser{}
	cfg := Config{
		Environment: environment,
		Debug:       p.bool("DEBUG", false),
		DocsEnabled: p.bool("DOCS", false),
		LogFormat:   p.oneOf("LOG_FORMAT", "text", "text", "json"),
		Server: ServerConfig{
			Port:              firstNonEmpty(strings.TrimSpace(os.Getenv("PORT")), "8000"),
			TrustedProxies:    parseCSVEnv("TRUSTED_PROXIES"),
			ReadTimeout:       p.duration("HTTP_READ_TIMEOUT", 10*time.Second),
			ReadHeaderTimeout: p.duration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			WriteTimeout:      p.duration("HTTP_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:       p.duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout:   p.duration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
		},
		Metrics: MetricsConfig{
			Port:  p.int("METRICS_PORT", 0),
			Token: strings.TrimSpace(os.Getenv("METRICS_TOKEN")),
		},
		CORS: CORSConfig{
			AllowedOrigins: defaultAllowedOrigins(environment),
		},
		Auth: AuthConfig{
			SessionTTL: p.duration("SESSION_TTL", 24*time.Hour),
			Cookie: CookieConfig{
				Domain:   strings.TrimSpace(os.Getenv("COOKIE_DOMAIN")),
				Secure:   p.bool("COOKIE_SECURE", environment == "production"),
				HTTPOnly: true,
				SameSite: p.sameSite("COOKIE_SAMESITE", http.SameSiteLaxMode),
			},
		},
		Database: DatabaseConfig{
			URL:             parseDatabaseURL(),
			MaxOpenConns:    p.int("DB_MAX_OPEN_CONNS", 20),
			MaxIdleConns:    p.int("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: p.duration("DB_CONN_MAX_LIFETIME", time.Hour),
			ConnMaxIdleTime: p.duration("DB_CONN_MAX_IDLE_TIME", 15*time.Minute),
		},
		Bootstrap: BootstrapConfig{
			AdminUsername: firstNonEmpty(strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_USERNAME")), "admin"),
			AdminPassword: os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
		},
	}

	if origins := parseCSVEnv("CORS_ALLOW_ORIGINS"); len(origins) > 0 {
		cfg.CORS.AllowedOrigins = origins
	}
	if cfg.Metrics.Enabled() {
		switch {
		case cfg.Metrics.Port < 1 || cfg.Metrics.Port > 65535:
			p.errs = append(p.errs, fmt.Errorf("METRICS_PORT must be between 1 and 65535, got %d", cfg.Metrics.Port))
		case strconv.Itoa(cfg.Metrics.Port) == cfg.Server.Port:
			p.errs = append(p.errs, fmt.Errorf("METRICS_PORT must differ from PORT"))
		}
		if len(cfg.Metrics.Token) < minMetricsTokenLength {
			p.errs = append(p.errs, fmt.Errorf("METRICS_TOKEN must be at least %d characters when METRICS_PORT is set", minMetricsTokenLength))
		}
	}
	if err := errors.Join(p.errs...); err != nil {
		return Config{}, err
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
	// Browsers reject SameSite=None cookies without Secure, and without SameSite
	// the session cookie would also ride along on cross-site requests.
	if cfg.Auth.Cookie.SameSite == http.SameSiteNoneMode && !cfg.Auth.Cookie.Secure {
		return Config{}, fmt.Errorf("COOKIE_SAMESITE=none requires COOKIE_SECURE=true")
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

// envParser reads typed environment variables and records every invalid
// value, so Load can report them all at once.
type envParser struct {
	errs []error
}

func (p *envParser) fail(name, raw, want string) {
	p.errs = append(p.errs, fmt.Errorf("%s: %q is not %s", name, raw, want))
}

func (p *envParser) bool(name string, defaultValue bool) bool {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		p.fail(name, raw, "a boolean (true or false)")
		return defaultValue
	}
	return parsed
}

func (p *envParser) int(name string, defaultValue int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		p.fail(name, raw, "an integer")
		return defaultValue
	}
	return parsed
}

func (p *envParser) duration(name string, defaultValue time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return defaultValue
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		p.fail(name, raw, "a duration such as 30s, 15m or 24h")
		return defaultValue
	}
	return parsed
}

func (p *envParser) oneOf(name, defaultValue string, allowed ...string) string {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	if raw == "" {
		return defaultValue
	}
	if slices.Contains(allowed, raw) {
		return raw
	}
	p.fail(name, raw, "one of "+strings.Join(allowed, ", "))
	return defaultValue
}

func (p *envParser) sameSite(name string, defaultValue http.SameSite) http.SameSite {
	switch p.oneOf(name, "", "lax", "strict", "none") {
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
