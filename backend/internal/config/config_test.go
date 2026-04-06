package config

import (
	"net/http"
	"testing"
	"time"
)

func TestLoadDevelopmentDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("CORS_ALLOW_ORIGINS", "")
	t.Setenv("COOKIE_SAMESITE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != "8000" {
		t.Fatalf("expected default port 8000, got %q", cfg.Server.Port)
	}
	if len(cfg.CORS.AllowedOrigins) == 0 {
		t.Fatalf("expected default dev CORS origins")
	}
	if cfg.Auth.Cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("expected lax same-site, got %v", cfg.Auth.Cookie.SameSite)
	}
}

func TestLoadProductionCookieDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_SECURE", "")
	t.Setenv("CORS_ALLOW_ORIGINS", "")
	t.Setenv("SECRET_KEY", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.IsProduction() {
		t.Fatalf("expected production config")
	}
	if !cfg.Auth.Cookie.Secure {
		t.Fatalf("expected secure cookies in production")
	}
	if len(cfg.CORS.AllowedOrigins) != 0 {
		t.Fatalf("expected no default production CORS origins, got %v", cfg.CORS.AllowedOrigins)
	}
}

func TestLoadRejectsInvalidCookieDomain(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_DOMAIN", "https://example.com")

	if _, err := Load(); err == nil {
		t.Fatalf("expected invalid COOKIE_DOMAIN error")
	}
}

func TestLoadAppliesExplicitOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("PORT", "9000")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://console.example.com,https://admin.example.com")
	t.Setenv("COOKIE_SAMESITE", "strict")
	t.Setenv("SESSION_TTL", "720h")
	t.Setenv("DB_MAX_OPEN_CONNS", "50")
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "30m")
	t.Setenv("SECRET_KEY", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != "9000" {
		t.Fatalf("Port = %q, want 9000", cfg.Server.Port)
	}
	if len(cfg.CORS.AllowedOrigins) != 2 {
		t.Fatalf("AllowedOrigins = %v", cfg.CORS.AllowedOrigins)
	}
	if cfg.Auth.Cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("SameSite = %v, want strict", cfg.Auth.Cookie.SameSite)
	}
	if cfg.Auth.SessionTTL != 720*time.Hour {
		t.Fatalf("SessionTTL = %v", cfg.Auth.SessionTTL)
	}
	if cfg.Database.MaxOpenConns != 50 {
		t.Fatalf("MaxOpenConns = %d", cfg.Database.MaxOpenConns)
	}
	if cfg.Database.ConnMaxIdleTime != 30*time.Minute {
		t.Fatalf("ConnMaxIdleTime = %v", cfg.Database.ConnMaxIdleTime)
	}
}

func TestLoadProductionRequiresSecretKey(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SECRET_KEY", "")

	if _, err := Load(); err == nil {
		t.Fatalf("expected error when SECRET_KEY is missing in production")
	}
}

func BenchmarkLoad(b *testing.B) {
	b.Setenv("APP_ENV", "production")
	b.Setenv("CORS_ALLOW_ORIGINS", "https://console.example.com")
	b.Setenv("SECRET_KEY", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")

	for i := 0; i < b.N; i++ {
		if _, err := Load(); err != nil {
			b.Fatalf("Load() error = %v", err)
		}
	}
}
