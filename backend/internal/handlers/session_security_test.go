package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"wg-keeper-backend/internal/auth"
	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/database"
	"wg-keeper-backend/internal/middleware"
	"wg-keeper-backend/internal/models"
)

// securityRouter wires the auth routes the way cmd/server does.
func securityRouter(t *testing.T) *gin.Engine {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	t.Setenv("SECRET_KEY", strings.Repeat("12", 32))
	if _, err := config.Load(); err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	db := openHandlersTestDB(t)
	oldDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = oldDB })

	hash, err := bcrypt.GenerateFromPassword([]byte("correct-horse"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.User{ID: "user-1", Username: "admin", PasswordHash: string(hash), IsAdmin: true}).Error; err != nil {
		t.Fatal(err)
	}

	h := NewAuthHandler()
	r := gin.New()
	r.POST("/api/login", h.Login)
	protected := r.Group("/api")
	protected.Use(middleware.AuthMiddleware())
	protected.GET("/me", h.GetMe)
	protected.POST("/logout", h.Logout)
	protected.POST("/change-password", h.ChangePassword)
	return r
}

func loginToken(t *testing.T, r http.Handler) string {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, newJSONRequest(t, http.MethodPost, "/api/login", LoginRequest{Username: "admin", Password: "correct-horse"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("login: status = %d, want 200", rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			return c.Value
		}
	}
	t.Fatal("login set no session cookie")
	return ""
}

func call(r http.Handler, req *http.Request, token string) *httptest.ResponseRecorder {
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestLogoutRevokesEverySession(t *testing.T) {
	r := securityRouter(t)
	laptop := loginToken(t, r)
	stolen := loginToken(t, r)

	if rec := call(r, httptest.NewRequest(http.MethodPost, "/api/logout", nil), laptop); rec.Code != http.StatusOK {
		t.Fatalf("logout: status = %d", rec.Code)
	}
	for name, token := range map[string]string{"logged-out token": laptop, "other copy": stolen} {
		if rec := call(r, httptest.NewRequest(http.MethodGet, "/api/me", nil), token); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s after logout: status = %d, want 401", name, rec.Code)
		}
	}
}

func TestChangePasswordRevokesOtherSessions(t *testing.T) {
	r := securityRouter(t)
	current := loginToken(t, r)
	stolen := loginToken(t, r)

	req := newJSONRequest(t, http.MethodPost, "/api/change-password", ChangePasswordRequest{
		CurrentPassword: ptr("correct-horse"), NewPassword: "battery-staple",
	})
	rec := call(r, req, current)
	if rec.Code != http.StatusOK {
		t.Fatalf("change password: status = %d body = %s", rec.Code, rec.Body)
	}
	var fresh string
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			fresh = c.Value
		}
	}

	if rec := call(r, httptest.NewRequest(http.MethodGet, "/api/me", nil), stolen); rec.Code != http.StatusUnauthorized {
		t.Fatalf("session opened with the old password: status = %d, want 401", rec.Code)
	}
	if rec := call(r, httptest.NewRequest(http.MethodGet, "/api/me", nil), fresh); rec.Code != http.StatusOK {
		t.Fatalf("the client that changed the password lost its session: status = %d", rec.Code)
	}
}

func TestLoginUnknownUserAndDatabaseErrors(t *testing.T) {
	r := securityRouter(t)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, newJSONRequest(t, http.MethodPost, "/api/login", LoginRequest{Username: "nobody", Password: "x"}))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unknown user: status = %d, want 401", rec.Code)
	}

	sqlDB, err := database.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, newJSONRequest(t, http.MethodPost, "/api/login", LoginRequest{Username: "admin", Password: "correct-horse"}))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("database down: status = %d, want 500 (a 401 would count towards the lockout)", rec.Code)
	}
}

func TestParseNodeAddressRejectsCredentials(t *testing.T) {
	tests := []struct {
		raw  string
		want string
		ok   bool
	}{
		{"https://node.example:51821", "https://node.example:51821", true},
		{"http://10.0.0.5:51821/", "http://10.0.0.5:51821", true},
		{"https://node.example/some/path?x=1#frag", "https://node.example", true},
		{"https://admin:secret@node.example", "", false},
		{"https://token@node.example", "", false},
		{"ftp://node.example", "", false},
		{"node.example", "", false},
	}
	for _, tt := range tests {
		got, ok := parseNodeAddress(tt.raw)
		if ok != tt.ok || got != tt.want {
			t.Errorf("parseNodeAddress(%q) = (%q, %v), want (%q, %v)", tt.raw, got, ok, tt.want, tt.ok)
		}
	}
}
