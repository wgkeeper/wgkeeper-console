package server

import (
	"wg-keeper-backend/internal/logging"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-Id"

// requestIDMiddleware gives every request an ID, returned in X-Request-Id and
// attached to every log line written while handling it. A client-supplied ID
// is kept only if it is a UUID v4, so crafted headers can't forge log lines.
func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if u, err := uuid.Parse(id); err != nil || u.Version() != 4 || u.String() != id {
			id = uuid.NewString()
		}
		c.Request = c.Request.WithContext(logging.WithRequestID(c.Request.Context(), id))
		c.Header(requestIDHeader, id)
		c.Next()
	}
}
