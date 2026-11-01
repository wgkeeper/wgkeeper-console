package middleware

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"wg-keeper-backend/internal/auth"
	"wg-keeper-backend/internal/database"
	"wg-keeper-backend/internal/models"
)

// AuthMiddleware requires a valid session cookie whose version matches the
// user's current SessionVersion, and refreshes the token when it is close to
// expiry.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(auth.SessionCookieName)
		if err != nil || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false})
			return
		}

		claims, err := auth.VerifyToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false})
			return
		}

		var user models.User
		if err := database.DB.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false})
				return
			}
			slog.Error("auth: load session user", "err", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"ok": false})
			return
		}

		// A logout or password change bumped the version: this token is revoked.
		if claims.Version != user.SessionVersion {
			auth.ClearSessionCookie(c)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false})
			return
		}

		if auth.ShouldRefresh(claims) {
			if newToken, expires, err := auth.RefreshToken(user, claims); err == nil {
				auth.SetSessionCookie(c, newToken, expires)
			}
		}

		c.Set("user", &user)
		c.Next()
	}
}

// RequirePasswordChanged blocks every route it wraps until the user has
// replaced the bootstrap password. Put it after AuthMiddleware and leave
// /me, /logout and /change-password outside it.
func RequirePasswordChanged() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := c.Value("user").(*models.User)
		if !ok || user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false})
			return
		}
		if user.MustChangePassword {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"ok": false, "error": "password_change_required"})
			return
		}
		c.Next()
	}
}

func HasSession(c *gin.Context) bool {
	_, exists := c.Get("user")
	return exists
}
