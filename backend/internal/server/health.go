package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Pinger reports whether the database is reachable.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// healthHandler is the liveness probe: the process is up and serving.
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// readinessHandler is the readiness probe: the database answers.
func readinessHandler(db Pinger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			slog.WarnContext(c.Request.Context(), "readiness: database unreachable", "err", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
