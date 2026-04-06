package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"wg-keeper-backend/internal/auth"
	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/database"
	"wg-keeper-backend/internal/models"
)

func TestAuthHandlerLogin(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	_, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	db := openHandlersTestDB(t)
	oldDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = oldDB })

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("secret-pass"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword() error = %v", err)
	}
	if err := db.Create(&models.User{
		ID:                 "user-1",
		Username:           "admin",
		PasswordHash:       string(passwordHash),
		IsAdmin:            true,
		MustChangePassword: true,
	}).Error; err != nil {
		t.Fatalf("db.Create() error = %v", err)
	}

	tests := []struct {
		name       string
		body       any
		wantStatus int
		wantCookie bool
	}{
		{name: "success", body: LoginRequest{Username: "admin", Password: "secret-pass"}, wantStatus: http.StatusOK, wantCookie: true},
		{name: "bad password", body: LoginRequest{Username: "admin", Password: "wrong"}, wantStatus: http.StatusUnauthorized},
		{name: "empty username", body: LoginRequest{Username: " ", Password: "secret-pass"}, wantStatus: http.StatusUnauthorized},
	}

	handler := NewAuthHandler()
	t.Run("bad json body", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString("{"))
		ctx.Request.Header.Set("Content-Type", "application/json")
		handler.Login(ctx)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = newJSONRequest(t, http.MethodPost, "/api/login", tt.body)

			handler.Login(ctx)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantCookie {
				if cookie := rec.Header().Get("Set-Cookie"); cookie == "" || !bytes.Contains([]byte(cookie), []byte(auth.SessionCookieName+"=")) {
					t.Fatalf("expected session cookie to be set")
				}
			}
		})
	}
}

func TestAuthHandlerChangePassword(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	_, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	db := openHandlersTestDB(t)
	oldDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = oldDB })

	currentHash, err := bcrypt.GenerateFromPassword([]byte("old-secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword() error = %v", err)
	}

	handler := NewAuthHandler()
	tests := []struct {
		name            string
		user            models.User
		body            ChangePasswordRequest
		wantStatus      int
		wantError       string
		wantMustChange  bool
		checkDBPassword bool
	}{
		{
			name:       "password not allowed",
			user:       models.User{ID: "u0", Username: "admin", PasswordHash: string(currentHash)},
			body:       ChangePasswordRequest{CurrentPassword: ptr("old-secret"), NewPassword: "admin"},
			wantStatus: http.StatusBadRequest, wantError: "password_not_allowed",
		},
		{
			name:       "too short",
			user:       models.User{ID: "u1", Username: "admin", PasswordHash: string(currentHash)},
			body:       ChangePasswordRequest{CurrentPassword: ptr("old-secret"), NewPassword: "short"},
			wantStatus: http.StatusBadRequest, wantError: "password_too_short",
		},
		{
			name:       "invalid current password",
			user:       models.User{ID: "u1x", Username: "admin", PasswordHash: string(currentHash)},
			body:       ChangePasswordRequest{CurrentPassword: ptr("wrong"), NewPassword: "new-secret-123"},
			wantStatus: http.StatusBadRequest, wantError: "invalid_current_password",
		},
		{
			name:       "current required",
			user:       models.User{ID: "u2", Username: "admin", PasswordHash: string(currentHash)},
			body:       ChangePasswordRequest{NewPassword: "new-secret-123"},
			wantStatus: http.StatusBadRequest, wantError: "current_password_required",
		},
		{
			name:       "first login without current password",
			user:       models.User{ID: "u3", Username: "admin", PasswordHash: string(currentHash), MustChangePassword: true},
			body:       ChangePasswordRequest{NewPassword: "new-secret-123"},
			wantStatus: http.StatusOK, checkDBPassword: true,
		},
		{
			name:       "success with current password",
			user:       models.User{ID: "u4", Username: "admin", PasswordHash: string(currentHash)},
			body:       ChangePasswordRequest{CurrentPassword: ptr("old-secret"), NewPassword: "new-secret-123"},
			wantStatus: http.StatusOK, checkDBPassword: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := db.Where("1 = 1").Delete(&models.User{}).Error; err != nil {
				t.Fatalf("cleanup users: %v", err)
			}
			user := tt.user
			if err := db.Create(&user).Error; err != nil {
				t.Fatalf("db.Create() error = %v", err)
			}

			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = newJSONRequest(t, http.MethodPost, "/api/change-password", tt.body)
			ctx.Set("user", &user)

			handler.ChangePassword(ctx)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantError != "" {
				var body map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("json.Unmarshal(): %v", err)
				}
				if body["error"] != tt.wantError {
					t.Fatalf("error = %v, want %q", body["error"], tt.wantError)
				}
			}
			if tt.checkDBPassword {
				var stored models.User
				if err := db.Where("id = ?", user.ID).First(&stored).Error; err != nil {
					t.Fatalf("db.First() error = %v", err)
				}
				if err := bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(tt.body.NewPassword)); err != nil {
					t.Fatalf("new password hash mismatch: %v", err)
				}
				if stored.MustChangePassword {
					t.Fatalf("expected MustChangePassword=false after update")
				}
				if cookie := rec.Header().Get("Set-Cookie"); cookie == "" || !bytes.Contains([]byte(cookie), []byte(auth.SessionCookieName+"=")) {
					t.Fatalf("expected rotated session cookie to be set")
				}
			}
		})
	}
}

func TestAuthHandlerChangePasswordUpdatesLoginPassword(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	_, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	db := openHandlersTestDB(t)
	oldDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = oldDB })

	currentHash, err := bcrypt.GenerateFromPassword([]byte("bootstrap-secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword() error = %v", err)
	}

	user := models.User{
		ID:                 "u-password-flow",
		Username:           "admin",
		PasswordHash:       string(currentHash),
		IsAdmin:            true,
		MustChangePassword: true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("db.Create() error = %v", err)
	}

	handler := NewAuthHandler()
	changeRec := httptest.NewRecorder()
	changeCtx, _ := gin.CreateTestContext(changeRec)
	changeCtx.Request = newJSONRequest(t, http.MethodPost, "/api/change-password", ChangePasswordRequest{
		NewPassword: "new-secret-123",
	})
	changeCtx.Set("user", &user)

	handler.ChangePassword(changeCtx)

	if changeRec.Code != http.StatusOK {
		t.Fatalf("change status = %d, want %d", changeRec.Code, http.StatusOK)
	}

	oldLoginRec := httptest.NewRecorder()
	oldLoginCtx, _ := gin.CreateTestContext(oldLoginRec)
	oldLoginCtx.Request = newJSONRequest(t, http.MethodPost, "/api/login", LoginRequest{
		Username: "admin",
		Password: "bootstrap-secret",
	})
	handler.Login(oldLoginCtx)
	if oldLoginRec.Code != http.StatusUnauthorized {
		t.Fatalf("old password login status = %d, want %d", oldLoginRec.Code, http.StatusUnauthorized)
	}

	newLoginRec := httptest.NewRecorder()
	newLoginCtx, _ := gin.CreateTestContext(newLoginRec)
	newLoginCtx.Request = newJSONRequest(t, http.MethodPost, "/api/login", LoginRequest{
		Username: "admin",
		Password: "new-secret-123",
	})
	handler.Login(newLoginCtx)
	if newLoginRec.Code != http.StatusOK {
		t.Fatalf("new password login status = %d, want %d", newLoginRec.Code, http.StatusOK)
	}
}

func TestAuthHandlerGetMeAndLogout(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	_, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	handler := NewAuthHandler()

	t.Run("get me unauthorized", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/me", nil)
		handler.GetMe(ctx)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("get me success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/me", nil)
		ctx.Set("user", &models.User{Username: "admin", IsAdmin: true})
		handler.GetMe(ctx)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("get me invalid user type", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/me", nil)
		ctx.Set("user", "bad-user")
		handler.GetMe(ctx)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("logout clears cookie", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/logout", nil)
		handler.Logout(ctx)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		cookie := rec.Header().Get("Set-Cookie")
		if !bytes.Contains([]byte(cookie), []byte("Max-Age=0")) && !bytes.Contains([]byte(cookie), []byte("Max-Age=-1")) {
			t.Fatalf("expected cookie clearing header, got %q", cookie)
		}
	})
}

func openHandlersTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return db
}

func newJSONRequest(t *testing.T, method string, path string, body any) *http.Request {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func ptr[T any](value T) *T {
	return &value
}
