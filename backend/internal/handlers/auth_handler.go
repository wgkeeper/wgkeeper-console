package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"wg-keeper-backend/internal/auth"
	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/database"
	"wg-keeper-backend/internal/models"
)

// LoginRequest represents login request body
type LoginRequest struct {
	Username string `json:"username" example:"admin" binding:"required"`
	Password string `json:"password" example:"admin" binding:"required"`
}

// LoginResponse represents login response
type LoginResponse struct {
	OK                 bool   `json:"ok" example:"true"`
	MustChangePassword bool   `json:"mustChangePassword" example:"true"`
	Username           string `json:"username,omitempty" example:"admin"`
}

// MeResponse represents me endpoint response
type MeResponse struct {
	Authenticated      bool   `json:"authenticated" example:"true"`
	Username           string `json:"username" example:"admin"`
	MustChangePassword bool   `json:"mustChangePassword" example:"false"`
	IsAdmin            bool   `json:"isAdmin" example:"true"`
}

// ChangePasswordRequest represents password change request body
type ChangePasswordRequest struct {
	CurrentPassword *string `json:"currentPassword" example:"old-password"`
	NewPassword     string  `json:"newPassword" example:"new-strong-password" binding:"required"`
}

type AuthHandler struct{}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{}
}

// GetMe godoc
// @Summary      Get current user info
// @Description  Check if user is authenticated
// @Tags         auth
// @Security     CookieAuth
// @Success      200  {object}  MeResponse
// @Failure      401  {object}  map[string]interface{}  "Unauthorized"
// @Router       /me [get]
func (h *AuthHandler) GetMe(c *gin.Context) {
	userAny, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"authenticated": false})
		return
	}
	user, ok := userAny.(*models.User)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"authenticated": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"authenticated":      true,
		"username":           user.Username,
		"mustChangePassword": user.MustChangePassword,
		"isAdmin":            user.IsAdmin,
	})
}

// Login godoc
// @Summary      Login
// @Description  Authenticate user and issue a signed session token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      LoginRequest  true  "Login credentials"
// @Success      200      {object}  LoginResponse
// @Failure      401      {object}  map[string]interface{}  "Invalid credentials"
// @Router       /login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var body LoginRequest

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false})
		return
	}

	username := strings.TrimSpace(body.Username)
	password := body.Password

	if username == "" || password == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false})
		return
	}

	var user models.User
	if err := database.DB.Where("username = ?", username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false})
		return
	}

	token, err := auth.CreateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false})
		return
	}

	setSessionCookie(c, token)

	c.JSON(http.StatusOK, gin.H{
		"ok":                 true,
		"mustChangePassword": user.MustChangePassword,
		"username":           user.Username,
	})
}

// Logout godoc
// @Summary      Logout
// @Description  Clear the session cookie
// @Tags         auth
// @Produce      json
// @Success      200  {object}  LoginResponse
// @Router       /logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func setSessionCookie(c *gin.Context, token string) {
	cfg := config.Current()
	c.SetSameSite(cfg.Auth.Cookie.SameSite)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/api",
		Domain:   cfg.Auth.Cookie.Domain,
		MaxAge:   int(cfg.Auth.SessionTTL.Seconds()),
		Secure:   cfg.Auth.Cookie.Secure,
		HttpOnly: cfg.Auth.Cookie.HTTPOnly,
		SameSite: cfg.Auth.Cookie.SameSite,
	})
}

func clearSessionCookie(c *gin.Context) {
	cfg := config.Current()
	c.SetSameSite(cfg.Auth.Cookie.SameSite)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/api",
		Domain:   cfg.Auth.Cookie.Domain,
		MaxAge:   -1,
		Secure:   cfg.Auth.Cookie.Secure,
		HttpOnly: cfg.Auth.Cookie.HTTPOnly,
		SameSite: cfg.Auth.Cookie.SameSite,
	})
}

// ChangePassword godoc
// @Summary      Change password
// @Description  Change current user's password. If the user is required to change password on first login, currentPassword may be omitted.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      ChangePasswordRequest  true  "Password change payload"
// @Success      200      {object}  LoginResponse
// @Failure      400      {object}  map[string]interface{}  "Bad request"
// @Failure      401      {object}  map[string]interface{}  "Unauthorized"
// @Router       /change-password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userAny, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false})
		return
	}
	tokenUser, ok := userAny.(*models.User)
	if !ok || tokenUser == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false})
		return
	}

	var body ChangePasswordRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false})
		return
	}

	newPassword := strings.TrimSpace(body.NewPassword)
	if strings.EqualFold(newPassword, "admin") {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "password_not_allowed"})
		return
	}
	if len(newPassword) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "password_too_short"})
		return
	}

	// Reload user from DB to get the password hash for verification.
	var user models.User
	if err := database.DB.Where("id = ?", tokenUser.ID).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false})
		return
	}

	// Require current password unless this is the first login and password must be changed.
	if !user.MustChangePassword {
		if body.CurrentPassword == nil || strings.TrimSpace(*body.CurrentPassword) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "current_password_required"})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(*body.CurrentPassword)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid_current_password"})
			return
		}
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false})
		return
	}

	updates := map[string]interface{}{
		"password_hash":        string(hashed),
		"must_change_password": false,
	}
	if err := database.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false})
		return
	}

	user.MustChangePassword = false
	token, err := auth.CreateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false})
		return
	}
	setSessionCookie(c, token)

	c.JSON(http.StatusOK, gin.H{
		"ok":                 true,
		"mustChangePassword": false,
	})
}
