package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"wg-keeper-backend/internal/config"
)

// SetSessionCookie stores the session token in the response cookie. The cookie
// expires with the token, so a browser drops it once the session is over.
func SetSessionCookie(c *gin.Context, token string, expiresAt time.Time) {
	setCookie(c, token, max(int(time.Until(expiresAt).Seconds()), 1))
}

// ClearSessionCookie removes the session cookie from the browser.
func ClearSessionCookie(c *gin.Context) {
	setCookie(c, "", -1)
}

func setCookie(c *gin.Context, value string, maxAge int) {
	cfg := config.Current()
	c.SetSameSite(cfg.Auth.Cookie.SameSite)
	// Secure and SameSite come from COOKIE_SECURE / COOKIE_SAMESITE; config.Load
	// rejects SameSite=None without Secure.
	http.SetCookie(c.Writer, &http.Cookie{ //nolint:gosec // G124: attributes are configurable and validated in config.Load
		Name:     SessionCookieName,
		Value:    value,
		Path:     "/api",
		Domain:   cfg.Auth.Cookie.Domain,
		MaxAge:   maxAge,
		Secure:   cfg.Auth.Cookie.Secure,
		HttpOnly: cfg.Auth.Cookie.HTTPOnly,
		SameSite: cfg.Auth.Cookie.SameSite,
	})
}
