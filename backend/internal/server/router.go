// Package server wires the console's HTTP API: middleware, routes, health
// probes and the built frontend.
package server

import (
	"fmt"
	"log/slog"

	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/handlers"
	"wg-keeper-backend/internal/metrics"
	"wg-keeper-backend/internal/middleware"
	"wg-keeper-backend/internal/services"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// maxRequestBody caps every request body; the API only takes small JSON.
const maxRequestBody = 1 << 20

// Deps are the collaborators the router needs.
type Deps struct {
	// DB backs the readiness probe.
	DB Pinger
	// Service talks to nodes; nil builds a default one.
	Service *services.DashboardService
	// Metrics is optional; nil disables recording.
	Metrics *metrics.Metrics
	// StaticRoot holds the built frontend, served in production.
	StaticRoot string
}

// NewRouter builds the API router. The returned function stops the rate
// limiters' background work; call it on shutdown.
func NewRouter(cfg config.Config, deps Deps) (*gin.Engine, func(), error) {
	r := gin.New()
	if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		return nil, nil, fmt.Errorf("set trusted proxies: %w", err)
	}
	r.Use(
		requestIDMiddleware(),
		recoveryMiddleware(),
		requestLogger(),
		deps.Metrics.GinMiddleware(),
		middleware.RequestSizeLimit(maxRequestBody),
		middleware.SecurityHeaders(cfg.IsProduction()),
		middleware.CORSMiddleware(cfg.CORS),
	)

	// Probes sit outside auth, rate limits and the same-origin guard.
	r.GET("/healthz", healthHandler)
	r.GET("/readyz", readinessHandler(deps.DB))

	if cfg.DocsEnabled {
		r.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/docs/doc.json")))
		slog.Info("swagger UI available", "path", "/docs/index.html")
	}

	svc := deps.Service
	if svc == nil {
		svc = services.NewDashboardService()
	}
	authHandler := handlers.NewAuthHandler()
	dashboardHandler := handlers.NewDashboardHandlerWithService(svc)
	loginRL := middleware.NewLoginRateLimiter()
	apiRL := middleware.NewAPIRateLimiter()

	api := r.Group("/api")
	api.Use(middleware.NoStore(), middleware.SameOriginGuard(cfg.CORS.AllowedOrigins))
	{
		api.POST("/login", middleware.LoginRateLimit(loginRL), authHandler.Login)

		// Protected routes (require an active session).
		protected := api.Group("/")
		protected.Use(middleware.AuthMiddleware(), middleware.APIRateLimit(apiRL))
		protected.GET("/me", authHandler.GetMe)
		protected.POST("/logout", authHandler.Logout)
		protected.POST("/change-password", authHandler.ChangePassword)

		// Dashboard routes: closed until the bootstrap password is changed.
		nodes := protected.Group("/")
		nodes.Use(middleware.RequirePasswordChanged())
		nodes.GET("/nodes", dashboardHandler.GetNodes)
		nodes.POST("/nodes", dashboardHandler.CreateNode)
		nodes.POST("/nodes/check", dashboardHandler.CheckNode)
		nodes.GET("/nodes/:id", dashboardHandler.GetNode)
		nodes.GET("/nodes/:id/stats", dashboardHandler.GetNodeStats)
		// POST, not GET: generating a config creates or rotates the peer on the node.
		nodes.POST("/nodes/:id/config", dashboardHandler.GetNodeConfig)
		nodes.GET("/nodes/:id/peers", dashboardHandler.GetNodePeers)
		nodes.GET("/nodes/:id/peers/:peerId", dashboardHandler.GetNodePeerDetail)
		nodes.DELETE("/nodes/:id/peers", dashboardHandler.DeletePeer)
		nodes.DELETE("/nodes/:id", dashboardHandler.DeleteNode)
		nodes.POST("/nodes/refresh", dashboardHandler.RefreshNodes)
	}

	registerStatic(r, deps.StaticRoot, cfg.IsProduction())

	stop := func() {
		loginRL.Close()
		apiRL.Close()
	}
	return r, stop, nil
}
