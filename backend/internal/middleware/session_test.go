package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wg-keeper-backend/internal/auth"
	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/database"
	"wg-keeper-backend/internal/models"
)

// setupSessionTest loads a config with a fixed SECRET_KEY, points database.DB
// at a private in-memory database and creates one user.
func setupSessionTest(t *testing.T, user models.User) *gorm.DB {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	t.Setenv("SECRET_KEY", strings.Repeat("ab", 32))
	t.Setenv("SESSION_TTL", "24h")
	if _, err := config.Load(); err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("db.Create() error = %v", err)
	}
	oldDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = oldDB })
	return db
}

func protectedRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware())
	r.GET("/me", func(c *gin.Context) { c.Status(http.StatusOK) })
	nodes := r.Group("/")
	nodes.Use(RequirePasswordChanged())
	nodes.GET("/nodes", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func requestWithToken(r http.Handler, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func sessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			return c
		}
	}
	return nil
}

func TestAuthMiddlewareRejectsRevokedToken(t *testing.T) {
	user := models.User{ID: "user-1", Username: "admin"}
	db := setupSessionTest(t, user)

	token, _, err := auth.CreateToken(user)
	if err != nil {
		t.Fatal(err)
	}
	if rec := requestWithToken(protectedRouter(), "/nodes", token); rec.Code != http.StatusOK {
		t.Fatalf("before revocation: status = %d, want 200", rec.Code)
	}

	// What logout and password change do.
	if err := db.Model(&models.User{}).Where("id = ?", user.ID).
		Update("session_version", gorm.Expr("session_version + 1")).Error; err != nil {
		t.Fatal(err)
	}

	rec := requestWithToken(protectedRouter(), "/nodes", token)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("after revocation: status = %d, want 401", rec.Code)
	}
	if c := sessionCookie(rec); c == nil || c.MaxAge >= 0 {
		t.Fatalf("revoked token should clear the cookie, got %+v", c)
	}
}

func TestAuthMiddlewareRejectsDeletedUser(t *testing.T) {
	user := models.User{ID: "user-1", Username: "admin"}
	db := setupSessionTest(t, user)
	token, _, err := auth.CreateToken(user)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&models.User{}, "id = ?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if rec := requestWithToken(protectedRouter(), "/me", token); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAuthMiddlewareRefreshKeepsSession(t *testing.T) {
	user := models.User{ID: "user-1", Username: "admin"}
	setupSessionTest(t, user)

	now := time.Now()
	authTime := now.Add(-20 * time.Hour)
	old := signTestClaims(t, auth.TokenClaims{
		UserID:    user.ID,
		IssuedAt:  authTime.Unix(),
		ExpiresAt: now.Add(4 * time.Hour).Unix(), // under half of the 24h TTL left
		AuthTime:  authTime.Unix(),
	})

	rec := requestWithToken(protectedRouter(), "/me", old)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	c := sessionCookie(rec)
	if c == nil {
		t.Fatal("expected a refreshed session cookie")
	}
	if !c.HttpOnly || c.Path != "/api" || c.MaxAge <= 0 {
		t.Fatalf("refreshed cookie attributes: %+v", c)
	}
	claims, err := auth.VerifyToken(c.Value)
	if err != nil {
		t.Fatalf("refreshed token: %v", err)
	}
	if claims.AuthTime != authTime.Unix() {
		t.Fatalf("refresh moved the login time: got %d, want %d", claims.AuthTime, authTime.Unix())
	}

	fresh, _, err := auth.CreateToken(user)
	if err != nil {
		t.Fatal(err)
	}
	if c := sessionCookie(requestWithToken(protectedRouter(), "/me", fresh)); c != nil {
		t.Fatalf("a fresh token should not be refreshed, got cookie %+v", c)
	}
}

func TestRequirePasswordChanged(t *testing.T) {
	user := models.User{ID: "user-1", Username: "admin", MustChangePassword: true}
	db := setupSessionTest(t, user)
	token, _, err := auth.CreateToken(user)
	if err != nil {
		t.Fatal(err)
	}

	r := protectedRouter()
	if rec := requestWithToken(r, "/nodes", token); rec.Code != http.StatusForbidden ||
		!strings.Contains(rec.Body.String(), "password_change_required") {
		t.Fatalf("bootstrap password: status = %d body = %s, want 403 password_change_required", rec.Code, rec.Body)
	}
	if rec := requestWithToken(r, "/me", token); rec.Code != http.StatusOK {
		t.Fatalf("/me must stay reachable to show the change-password screen, got %d", rec.Code)
	}

	if err := db.Model(&models.User{}).Where("id = ?", user.ID).Update("must_change_password", false).Error; err != nil {
		t.Fatal(err)
	}
	if rec := requestWithToken(r, "/nodes", token); rec.Code != http.StatusOK {
		t.Fatalf("after password change: status = %d, want 200", rec.Code)
	}
}

func signTestClaims(t *testing.T, claims auth.TokenClaims) string {
	t.Helper()
	// Same format as auth.CreateToken: base64url(payload) "." hex(HMAC-SHA256).
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, config.Current().Auth.SessionSecret)
	mac.Write([]byte(encoded))
	return encoded + "." + hex.EncodeToString(mac.Sum(nil))
}
