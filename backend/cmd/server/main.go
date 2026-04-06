package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/database"
	"wg-keeper-backend/internal/handlers"
	"wg-keeper-backend/internal/middleware"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "wg-keeper-backend/docs"
)

// @title           WG Keeper Console API
// @version         1.0
// @description     WireGuard management console API
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8000
// @BasePath  /api

// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name wg_session

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	// Initialize database
	if err := database.Init(); err != nil {
		slog.Error("failed to initialize database", "err", err)
		os.Exit(1)
	}

	// Set Gin mode
	if cfg.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	slog.Info("starting", "debug", cfg.Debug, "docs", cfg.DocsEnabled, "env", cfg.Environment)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), middleware.RequestSizeLimit(1<<20), middleware.SecurityHeaders(cfg.IsProduction()), middleware.CORSMiddleware(cfg.CORS))
	if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		slog.Error("failed to set trusted proxies", "err", err)
		os.Exit(1)
	}

	// Initialize handlers
	authHandler := handlers.NewAuthHandler()
	dashboardHandler := handlers.NewDashboardHandler()
	loginRL := middleware.NewLoginRateLimiter()
	apiRL := middleware.NewAPIRateLimiter()

	// Swagger documentation (only when DOCS=true)
	if cfg.DocsEnabled {
		r.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/docs/doc.json")))
		slog.Info("swagger UI available", "path", "/docs/index.html")
	}

	// API routes
	api := r.Group("/api")
	{
		// Public auth routes
		api.POST("/login", middleware.LoginRateLimit(loginRL), authHandler.Login)

		// Protected routes (require active session)
		protected := api.Group("/")
		protected.Use(middleware.AuthMiddleware(), middleware.APIRateLimit(apiRL))

		// Auth routes (single admin user for now)
		protected.GET("/me", authHandler.GetMe)
		protected.POST("/logout", authHandler.Logout)
		protected.POST("/change-password", authHandler.ChangePassword)

		// Dashboard routes
		protected.GET("/nodes", dashboardHandler.GetNodes)
		protected.POST("/nodes", dashboardHandler.CreateNode)
		protected.POST("/nodes/check", dashboardHandler.CheckNode)
		protected.GET("/nodes/:id/stats", dashboardHandler.GetNodeStats)
		protected.GET("/nodes/:id/config", dashboardHandler.GetNodeConfig)
		protected.GET("/nodes/:id/peers", dashboardHandler.GetNodePeers)
		protected.GET("/nodes/:id/peers/:peerId", dashboardHandler.GetNodePeerDetail)
		protected.DELETE("/nodes/:id/peers", dashboardHandler.DeletePeer)
		protected.DELETE("/nodes/:id", dashboardHandler.DeleteNode)
		protected.POST("/nodes/refresh", dashboardHandler.RefreshNodes)
	}

	// Serve static files in production
	isProd := cfg.IsProduction()
	staticRoot := "./public"
	if isProd {
		indexPath := filepath.Join(staticRoot, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			assetsPath := filepath.Join(staticRoot, "assets")
			if _, err := os.Stat(assetsPath); err == nil {
				// Serve compiled assets under /assets to avoid conflicting
				// with API routes (e.g. /api)
				r.Static("/assets", assetsPath)
			}
			// Serve the SPA entrypoint at /
			r.StaticFile("/", indexPath)
			robotsPath := filepath.Join(staticRoot, "robots.txt")
			if _, err := os.Stat(robotsPath); err == nil {
				r.StaticFile("/robots.txt", robotsPath)
			}
			themeInitPath := filepath.Join(staticRoot, "theme-init.js")
			if _, err := os.Stat(themeInitPath); err == nil {
				r.StaticFile("/theme-init.js", themeInitPath)
			}
			r.NoRoute(func(c *gin.Context) {
				// Skip API routes
				if strings.HasPrefix(c.Request.URL.Path, "/api") {
					c.JSON(404, gin.H{"error": "not_found"})
					return
				}
				// Only serve HTML for GET requests with HTML accept header
				if c.Request.Method != "GET" {
					c.JSON(404, gin.H{"error": "not_found"})
					return
				}
				accept := c.GetHeader("Accept")
				if accept != "" && !strings.Contains(accept, "text/html") {
					c.JSON(404, gin.H{"error": "not_found"})
					return
				}
				c.File(indexPath)
			})
		}
	} else {
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
	}

	server := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           r,
		ReadTimeout:       cfg.Server.ReadTimeout,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		MaxHeaderBytes:    1 << 13, // 8KB
	}

	go func() {
		slog.Info("server starting", "port", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("failed to shut down server", "err", err)
		os.Exit(1)
	}
}
