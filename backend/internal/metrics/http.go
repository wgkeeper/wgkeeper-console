package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// pathUnmatched is the path label for requests that matched no route, so the
// attacker-controlled raw URL never becomes a label value.
const pathUnmatched = "unmatched"

// GinMiddleware records request count and duration. It labels by route
// template (c.FullPath(), e.g. "/api/nodes/:id"), never the raw path.
func (m *Metrics) GinMiddleware() gin.HandlerFunc {
	if m == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = pathUnmatched
		}
		method := methodLabel(c.Request.Method, path)
		m.HTTPRequests.WithLabelValues(method, path, classifyStatus(c.Writer.Status())).Inc()
		m.HTTPDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
	}
}

func classifyStatus(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500 && code < 600:
		return "5xx"
	}
	return "unknown_" + strconv.Itoa(code)
}

// methodLabel bounds the method label: unmatched routes accept any method
// token, and each distinct value would be a new series kept forever.
func methodLabel(method, path string) string {
	if path == pathUnmatched {
		return "other"
	}
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodHead, http.MethodOptions:
		return method
	}
	return "other"
}
