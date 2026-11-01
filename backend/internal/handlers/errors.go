package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"wg-keeper-backend/internal/services"

	"github.com/gin-gonic/gin"
)

// writeNodeError writes the response for a failed node operation. A missing
// node is 404; a node that rejects the key, answers with an error or can't be
// reached is 502 with unavailableCode (the node's status, if any, is in the
// body); anything else is a console-side failure: 500, logged.
func writeNodeError(c *gin.Context, err error, unavailableCode string) {
	nodeID := c.Param("id")
	var upstream *services.UpstreamError
	switch {
	case errors.Is(err, services.ErrNodeNotFound):
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "node_not_found"})
	case errors.As(err, &upstream):
		code := unavailableCode
		if upstream.Unauthorized() {
			code = "invalid_api_key"
		}
		slog.WarnContext(c.Request.Context(), "node request failed", "node_id", nodeID, "path", c.FullPath(), "node_status", upstream.Status)
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": code, "status": upstream.Status})
	case errors.Is(err, context.Canceled):
		// The browser went away; nobody is waiting for this response.
		c.AbortWithStatus(http.StatusServiceUnavailable)
	case errors.Is(err, services.ErrNodeUnreachable), errors.Is(err, context.DeadlineExceeded):
		slog.WarnContext(c.Request.Context(), "node unreachable", "node_id", nodeID, "path", c.FullPath(), "err", err)
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": unavailableCode})
	default:
		slog.ErrorContext(c.Request.Context(), "node operation failed", "node_id", nodeID, "path", c.FullPath(), "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false})
	}
}
