package middleware

import (
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"wg-keeper-backend/internal/config"
)

// RequestSizeLimit rejects requests whose body exceeds maxBytes.
func RequestSizeLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// NoStore keeps API responses out of browser and proxy caches. Some of them
// carry peer private keys and preshared keys.
func NoStore() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}

// SameOriginGuard rejects state-changing requests that a browser sent from
// another site, on top of the SameSite cookie. It trusts Sec-Fetch-Site when
// the browser sends it and falls back to comparing Origin with Host. Requests
// with neither header (curl, scripts) are not browser cross-site requests and
// pass. Bodies must be JSON, which a plain HTML form cannot send.
func SameOriginGuard(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}

	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}

		origin := c.GetHeader("Origin")
		_, originAllowed := allowed[origin]
		switch c.GetHeader("Sec-Fetch-Site") {
		case "same-origin", "none":
		case "":
			if origin != "" && !originAllowed && !sameHost(origin, c.Request.Host) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"ok": false, "error": "cross_origin_request"})
				return
			}
		default: // same-site, cross-site
			if !originAllowed {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"ok": false, "error": "cross_origin_request"})
				return
			}
		}

		if c.Request.ContentLength != 0 {
			mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
			if err != nil || mediaType != "application/json" {
				c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{"ok": false, "error": "json_body_required"})
				return
			}
		}
		c.Next()
	}
}

func sameHost(origin, host string) bool {
	u, err := url.Parse(origin)
	return err == nil && u.Host != "" && strings.EqualFold(u.Host, host)
}

const cspPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob:; " +
	"font-src 'self' data:; " +
	"connect-src 'self'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'"

func SecurityHeaders(isProd bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		c.Header("Content-Security-Policy", cspPolicy)
		if isProd {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

func CORSMiddleware(cfg config.CORSConfig) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, origin := range cfg.AllowedOrigins {
		allowed[origin] = struct{}{}
	}
	enforceCORS := len(allowed) > 0

	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin == "" || !enforceCORS {
			c.Next()
			return
		}

		if _, ok := allowed[origin]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"ok": false, "error": "cors_origin_denied"})
			return
		}

		c.Header("Vary", "Origin")
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
