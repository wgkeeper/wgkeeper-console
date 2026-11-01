package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// registerStatic serves the built frontend from staticRoot in production. In
// development the Vite dev server serves the UI, so unknown paths get a hint.
func registerStatic(r *gin.Engine, staticRoot string, isProd bool) {
	if !isProd {
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api") {
				c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
				return
			}
			c.JSON(http.StatusNotFound, gin.H{
				"error": "frontend_dev_server_only",
				"hint":  "Use Vite dev server at http://localhost:5173 in development.",
			})
		})
		return
	}

	indexPath := filepath.Join(staticRoot, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		return
	}
	if assetsPath := filepath.Join(staticRoot, "assets"); exists(assetsPath) {
		// Serve compiled assets under /assets to avoid conflicting with /api.
		r.Static("/assets", assetsPath)
	}
	// Serve the SPA entrypoint with no-cache: the hashed asset filenames let
	// /assets cache forever, but index.html must always be revalidated so a
	// deploy doesn't leave clients pinned to a stale build that calls removed APIs.
	serveIndex := func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, must-revalidate")
		c.File(indexPath)
	}
	r.GET("/", serveIndex)
	for _, name := range []string{"robots.txt", "theme-init.js", "favicon.svg"} {
		if path := filepath.Join(staticRoot, name); exists(path) {
			r.StaticFile("/"+name, path)
		}
	}
	r.NoRoute(func(c *gin.Context) {
		// Only page navigations get the SPA; API calls and assets get a 404.
		accept := c.GetHeader("Accept")
		if strings.HasPrefix(c.Request.URL.Path, "/api") || c.Request.Method != http.MethodGet ||
			(accept != "" && !strings.Contains(accept, "text/html")) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		serveIndex(c)
	})
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
