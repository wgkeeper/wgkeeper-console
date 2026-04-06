package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"wg-keeper-backend/internal/auth"
	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/database"
	"wg-keeper-backend/internal/models"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(auth.SessionCookieName)
		if err != nil || token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"ok": false})
			c.Abort()
			return
		}

		claims, err := auth.VerifyToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"ok": false})
			c.Abort()
			return
		}

		var user models.User
		if err := database.DB.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"ok": false})
			c.Abort()
			return
		}

		// Re-issue a fresh token when less than half the TTL remains.
		if auth.ShouldRefresh(claims) {
			if newToken, err := auth.CreateToken(user); err == nil {
				setSessionCookie(c, newToken)
			}
		}

		c.Set("user", &user)
		c.Next()
	}
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

func HasSession(c *gin.Context) bool {
	_, exists := c.Get("user")
	return exists
}
