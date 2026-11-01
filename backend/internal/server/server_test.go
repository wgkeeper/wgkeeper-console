package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/database"
	"wg-keeper-backend/internal/logging"
	"wg-keeper-backend/internal/metrics"
	"wg-keeper-backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRequestIDMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(requestIDMiddleware())
	var seen string
	r.GET("/", func(c *gin.Context) { seen = logging.RequestID(c.Request.Context()) })

	valid := uuid.NewString()
	for _, tt := range []struct {
		name, sent string
		keep       bool
	}{
		{"valid v4 kept", valid, true},
		{"missing", "", false},
		{"not a UUID", "abc\nlevel=ERROR msg=forged", false},
		{"upper-case form replaced", strings.ToUpper(valid), false},
		{"version 1 replaced", "550e8400-e29b-11d4-a716-446655440000", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.sent != "" {
				req.Header.Set(requestIDHeader, tt.sent)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			got := rec.Header().Get(requestIDHeader)
			if got != seen {
				t.Fatalf("header %q and context %q differ", got, seen)
			}
			if tt.keep && got != tt.sent {
				t.Fatalf("got %q, want the client's ID kept", got)
			}
			if !tt.keep {
				if got == tt.sent {
					t.Fatalf("kept an invalid client ID %q", got)
				}
				if u, err := uuid.Parse(got); err != nil || u.Version() != 4 {
					t.Fatalf("generated ID %q is not a UUID v4", got)
				}
			}
		})
	}
}

func TestRequestLogLevel(t *testing.T) {
	tests := []struct {
		status  int
		path    string
		matched bool
		want    slog.Level
	}{
		{500, "/api/nodes", true, slog.LevelError},
		{502, "/api/nodes/x/peers", true, slog.LevelError},
		{404, "/wp-login.php", false, slog.LevelInfo},
		{404, "/api/nodes/x", true, slog.LevelWarn},
		{401, "/api/me", true, slog.LevelWarn},
		{200, "/api/nodes", true, slog.LevelInfo},
		{200, "/assets/app.js", true, slog.LevelDebug},
	}
	for _, tt := range tests {
		if got := requestLogLevel(tt.status, tt.path, tt.matched); got != tt.want {
			t.Errorf("requestLogLevel(%d, %q, %v) = %v, want %v", tt.status, tt.path, tt.matched, got, tt.want)
		}
	}
}

func TestRecoveryLogsWithRequestIDAndNoHeaders(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(logging.New(&buf, "text", slog.LevelDebug))
	t.Cleanup(func() { slog.SetDefault(prev) })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(requestIDMiddleware(), recoveryMiddleware())
	r.GET("/boom", func(*gin.Context) { panic("boom") })

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	req.AddCookie(&http.Cookie{Name: "wg_session", Value: "secret-session-token"})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	logged := buf.String()
	if !strings.Contains(logged, "panic recovered") || !strings.Contains(logged, "request_id="+rec.Header().Get(requestIDHeader)) {
		t.Fatalf("log lacks the panic or the request ID:\n%s", logged)
	}
	if strings.Contains(logged, "secret-session-token") {
		t.Fatalf("the session cookie leaked into the log:\n%s", logged)
	}
}

type fakePinger struct{ err error }

func (f fakePinger) PingContext(context.Context) error { return f.err }

func TestHealthProbes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name string
		ping error
		want int
	}{
		{"database up", nil, http.StatusOK},
		{"database down", errors.New("connection refused"), http.StatusServiceUnavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/healthz", healthHandler)
			r.GET("/readyz", readinessHandler(fakePinger{tt.ping}))

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("/healthz = %d, want 200 regardless of the database", rec.Code)
			}
			rec = httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if rec.Code != tt.want {
				t.Fatalf("/readyz = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

// newTestRouter builds the real router over a private SQLite database.
func newTestRouter(t *testing.T, m *metrics.Metrics) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "test")
	t.Setenv("SECRET_KEY", strings.Repeat("ab", 32))
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	oldDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = oldDB })
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}

	r, stop, err := NewRouter(cfg, Deps{DB: sqlDB, Metrics: m, StaticRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
	return r
}

func TestRouterWiring(t *testing.T) {
	m := metrics.New()
	r := newTestRouter(t, m)
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}

	if rec := get("/healthz"); rec.Code != http.StatusOK {
		t.Fatalf("/healthz = %d", rec.Code)
	}
	if rec := get("/readyz"); rec.Code != http.StatusOK {
		t.Fatalf("/readyz = %d", rec.Code)
	}
	rec := get("/api/nodes")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("/api/nodes without a session = %d, want 401", rec.Code)
	}
	if rec.Header().Get(requestIDHeader) == "" || rec.Header().Get("Cache-Control") != "no-store" ||
		rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("missing request ID, no-store or security headers: %v", rec.Header())
	}
	if rec := get("/api/does-not-exist"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown API path = %d, want 404", rec.Code)
	}

	if got := testutil.ToFloat64(m.HTTPRequests.WithLabelValues("GET", "/api/nodes", "4xx")); got != 1 {
		t.Fatalf("http_requests_total for /api/nodes 4xx = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.HTTPRequests.WithLabelValues("other", "unmatched", "4xx")); got != 1 {
		t.Fatalf("unmatched paths must share one series, got %v", got)
	}
}
