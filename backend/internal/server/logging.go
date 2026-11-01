package server

import (
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// requestLogLevel picks the level of a request log line: errors for 5xx,
// warnings for 4xx except a 404 for a path that no route matches (scanner
// noise, info), info otherwise. Static files that load fine are debug.
func requestLogLevel(status int, path string, routeMatched bool) slog.Level {
	switch {
	case status >= http.StatusInternalServerError:
		return slog.LevelError
	case status == http.StatusNotFound && !routeMatched:
		return slog.LevelInfo
	case status >= http.StatusBadRequest:
		return slog.LevelWarn
	case !strings.HasPrefix(path, "/api/"):
		return slog.LevelDebug
	}
	return slog.LevelInfo
}

// requestLogger writes one line per request through slog, carrying the
// request ID. Health probes are not logged.
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.Request.URL.Path
		if path == "/healthz" || path == "/readyz" {
			return
		}
		status := c.Writer.Status()
		slog.Log(c.Request.Context(), requestLogLevel(status, path, c.FullPath() != ""), "http request",
			"method", c.Request.Method,
			"path", path,
			"route", c.FullPath(),
			"status", status,
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}

// recoveryMiddleware turns a handler panic into a 500 and logs it with the
// request ID and stack. gin's own Recovery dumps the request headers in debug
// mode, which would put the session cookie into the log.
func recoveryMiddleware() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, recovered any) {
		slog.ErrorContext(c.Request.Context(), "panic recovered",
			"method", c.Request.Method,
			"route", c.FullPath(),
			"panic", recovered,
			"stack", string(debug.Stack()),
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"ok": false})
	})
}
